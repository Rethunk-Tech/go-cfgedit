// Package jsonfile edits one value in a mod's JSON config file while keeping its key order and every other value as
// written. Keys match without case; a key the file spells differently is never renamed.
package jsonfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Rethunk-Tech/go-lenientjson"
)

// Kind is the JSON type of a Node.
type Kind byte

// The JSON types a Node can hold.
const (
	Null Kind = iota
	Bool
	Number
	String
	Array
	Object
)

// Pair is one key and its value, in the order the object holds them.
type Pair struct {
	Key   string
	Value Node
}

// Node is one JSON value; an object keeps its keys in file order.
type Node struct {
	Kind Kind
	Num  json.Number // Number
	Str  string      // String
	Bool bool        // Bool
	Arr  []Node      // Array
	Obj  []Pair      // Object
}

var (
	// ErrBadPath refuses a path with an empty part, so a stray "." cannot edit an unnamed key.
	ErrBadPath = errors.New("jsonfile: a path part is empty")
	// ErrUnknownKey refuses a path whose intermediate key the object does not hold.
	ErrUnknownKey = errors.New("unknown config field")
	// ErrNotObject refuses a path that descends into a value that is not an object.
	ErrNotObject = errors.New("is not an object")
	// ErrNotBool refuses text that strconv.ParseBool cannot read as a boolean.
	ErrNotBool = errors.New("jsonfile: the text is not a boolean")
	// ErrNotNumber refuses text that is not a finite JSON number.
	ErrNotNumber = errors.New("jsonfile: the text is not a number")
)

var jsonNumber = regexp.MustCompile(`^-?(0|[1-9]\d*)(\.\d+)?([eE][+-]?\d+)?$`)

// Parse reads a lenient JSON config, as Json.NET writes one: comments, trailing commas, single-quoted strings and
// unquoted keys are accepted. Key order and each number's written form are kept, and trailing data after the value is
// an error.
func Parse(data []byte) (Node, error) {
	dec := json.NewDecoder(bytes.NewReader(lenientjson.Clean(data)))
	dec.UseNumber()
	n, err := decode(dec)
	if err != nil {
		return Node{}, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return Node{}, errors.New("trailing data after the JSON value")
	}
	return n, nil
}

func decode(dec *json.Decoder) (Node, error) {
	tok, err := dec.Token()
	if err != nil {
		return Node{}, err
	}
	switch v := tok.(type) {
	case nil:
		return Node{Kind: Null}, nil
	case bool:
		return Node{Kind: Bool, Bool: v}, nil
	case json.Number:
		return Node{Kind: Number, Num: v}, nil
	case string:
		return Node{Kind: String, Str: v}, nil
	case json.Delim:
		switch v {
		case '[':
			return decodeArray(dec)
		case '{':
			return decodeObject(dec)
		case ']', '}':
			return Node{}, fmt.Errorf("unexpected %c", v)
		}
	}
	return Node{}, errors.New("unexpected JSON token")
}

func decodeArray(dec *json.Decoder) (Node, error) {
	var arr []Node
	for dec.More() {
		n, err := decode(dec)
		if err != nil {
			return Node{}, err
		}
		arr = append(arr, n)
	}
	if err := closing(dec, ']'); err != nil {
		return Node{}, err
	}
	return Node{Kind: Array, Arr: arr}, nil
}

func decodeObject(dec *json.Decoder) (Node, error) {
	var obj []Pair
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return Node{}, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return Node{}, errors.New("expected an object key")
		}
		val, err := decode(dec)
		if err != nil {
			return Node{}, err
		}
		obj = append(obj, Pair{Key: key, Value: val})
	}
	if err := closing(dec, '}'); err != nil {
		return Node{}, err
	}
	return Node{Kind: Object, Obj: obj}, nil
}

// closing reads the delimiter that ends an array or object, which must be want.
func closing(dec *json.Decoder, want json.Delim) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if got, ok := tok.(json.Delim); !ok || got != want {
		return fmt.Errorf("expected %c", want)
	}
	return nil
}

// Marshal writes n with two-space indentation and a trailing newline.
func (n Node) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	if err := encode(&buf, n, 0); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func encode(buf *bytes.Buffer, n Node, depth int) error {
	switch n.Kind {
	case Null:
		buf.WriteString("null")
	case Bool:
		buf.WriteString(strconv.FormatBool(n.Bool))
	case Number:
		buf.WriteString(n.Num.String())
	case String:
		enc, err := json.Marshal(n.Str)
		if err != nil {
			return err
		}
		buf.Write(enc)
	case Array:
		if len(n.Arr) == 0 {
			buf.WriteString("[]")
			return nil
		}
		buf.WriteString("[\n")
		pad := strings.Repeat("  ", depth+1)
		for i, item := range n.Arr {
			buf.WriteString(pad)
			if err := encode(buf, item, depth+1); err != nil {
				return err
			}
			if i < len(n.Arr)-1 {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
		}
		buf.WriteString(strings.Repeat("  ", depth))
		buf.WriteByte(']')
	case Object:
		if len(n.Obj) == 0 {
			buf.WriteString("{}")
			return nil
		}
		buf.WriteString("{\n")
		pad := strings.Repeat("  ", depth+1)
		for i, p := range n.Obj {
			buf.WriteString(pad)
			key, err := json.Marshal(p.Key)
			if err != nil {
				return err
			}
			buf.Write(key)
			buf.WriteString(": ")
			if err := encode(buf, p.Value, depth+1); err != nil {
				return err
			}
			if i < len(n.Obj)-1 {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
		}
		buf.WriteString(strings.Repeat("  ", depth))
		buf.WriteByte('}')
	}
	return nil
}

// Text is the value as a settings field shows it: scalars as they read, an array or object as its JSON.
func (n Node) Text() string {
	switch n.Kind {
	case Null:
		return "null"
	case Bool:
		return strconv.FormatBool(n.Bool)
	case Number:
		return n.Num.String()
	case String:
		return n.Str
	case Array, Object:
		var buf bytes.Buffer
		// No error is possible: encode only reports a kind it does not know, and both are known here.
		_ = encode(&buf, n, 0)
		return buf.String()
	}
	return ""
}

// SplitPath splits a dotted path into its keys; a backslash escapes a "." or "\" inside one. Parts are trimmed, and an
// empty part returns ErrBadPath.
func SplitPath(field string) ([]string, error) {
	var parts []string
	var cur strings.Builder
	for i := 0; i < len(field); i++ {
		switch c := field[i]; {
		case c == '\\' && i+1 < len(field) && (field[i+1] == '.' || field[i+1] == '\\'):
			i++
			cur.WriteByte(field[i])
		case c == '.':
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	parts = append(parts, cur.String())
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
		if parts[i] == "" {
			return nil, fmt.Errorf("%w: %q", ErrBadPath, field)
		}
	}
	return parts, nil
}

// Lookup returns the value at path, matching keys case-insensitively. An empty path is n itself.
func (n Node) Lookup(path []string) (Node, bool) {
	for _, key := range path {
		if n.Kind != Object {
			return Node{}, false
		}
		i := -1
		for j, p := range n.Obj {
			if strings.EqualFold(p.Key, key) {
				i = j
			}
		}
		if i < 0 {
			return Node{}, false
		}
		n = n.Obj[i].Value
	}
	return n, true
}

// Set replaces the value at path, matching keys case-insensitively; the file's own key spelling is kept. A missing last
// key is appended to its object under the spelling in path. A missing intermediate key wraps ErrUnknownKey, and an
// intermediate value that is not an object wraps ErrNotObject.
func Set(n Node, path []string, v Node) (Node, error) {
	head, rest, ok := cutPath(path)
	if !ok {
		return v, nil
	}
	if n.Kind != Object {
		return n, fmt.Errorf("%s %w", head, ErrNotObject)
	}
	for i := len(n.Obj) - 1; i >= 0; i-- {
		p := n.Obj[i]
		if !strings.EqualFold(p.Key, head) {
			continue
		}
		child := v
		if len(rest) > 0 {
			var err error
			child, err = Set(p.Value, rest, v)
			if err != nil {
				return n, err
			}
		}
		// The caller's object is shared with its Node, so the edit goes into a copy.
		out := slices.Clone(n.Obj)
		out[i].Value = child
		n.Obj = out
		return n, nil
	}
	if len(rest) > 0 {
		return n, fmt.Errorf("%w %s", ErrUnknownKey, head)
	}
	out := make([]Pair, len(n.Obj)+1)
	copy(out, n.Obj)
	out[len(n.Obj)] = Pair{Key: head, Value: v}
	n.Obj = out
	return n, nil
}

// cutPath splits path into its first key and the rest; ok is false for an empty path.
func cutPath(path []string) (head string, rest []string, ok bool) {
	if len(path) == 0 {
		return "", nil, false
	}
	return path[0], path[1:], true
}

// Coerce types raw text as kind: Bool parses it with strconv.ParseBool, Number must be a finite JSON number and keeps
// the trimmed text, and anything else is a String holding raw unchanged.
func Coerce(raw string, kind Kind) (Node, error) {
	if kind == Bool {
		parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return Node{}, ErrNotBool
		}
		return Node{Kind: Bool, Bool: parsed}, nil
	}
	if kind == Number {
		text := strings.TrimSpace(raw)
		n, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsInf(n, 0) || math.IsNaN(n) || !jsonNumber.MatchString(text) {
			return Node{}, ErrNotNumber
		}
		return Node{Kind: Number, Num: json.Number(text)}, nil
	}
	return Node{Kind: String, Str: raw}, nil
}
