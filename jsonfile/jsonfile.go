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
		return writeString(buf, n.Str)
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
			if err := writeString(buf, p.Key); err != nil {
				return err
			}
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

// writeString writes s as a JSON string without the HTML escaping json.Marshal adds, so "&" and "<" stay as written.
func writeString(buf *bytes.Buffer, s string) error {
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return err
	}
	buf.Truncate(buf.Len() - 1) // Encode ends with a newline
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

// Set replaces the value at path in content, matching keys case-insensitively; the file's own key spelling is kept,
// and every byte outside the replaced value stays as written. A missing last key is appended to its object under the
// spelling in path. A missing intermediate key wraps ErrUnknownKey, and an intermediate value that is not an object
// wraps ErrNotObject. An empty path returns v as the whole content.
func Set(content string, path []string, v Node) (string, error) {
	n, err := Parse([]byte(content))
	if err != nil {
		return content, err
	}
	enc, err := encodeInline(v)
	if err != nil {
		return content, err
	}
	if len(path) == 0 {
		if same(n, v) {
			return content, nil
		}
		out, err := v.Marshal()
		if err != nil {
			return content, err
		}
		return string(out), nil
	}
	scan := &scanner{text: content}
	for i, key := range path {
		if n.Kind != Object {
			return content, fmt.Errorf("%s %w", key, ErrNotObject)
		}
		obj, err := scan.object()
		if err != nil {
			return content, err
		}
		at := -1
		for j, p := range n.Obj {
			if strings.EqualFold(p.Key, key) {
				at = j
			}
		}
		if at < 0 {
			if i < len(path)-1 {
				return content, fmt.Errorf("%w %s", ErrUnknownKey, key)
			}
			return insertMember(content, obj, key, enc)
		}
		if i == len(path)-1 {
			if same(n.Obj[at].Value, v) {
				return content, nil
			}
			m := obj.members[at]
			return content[:m.start] + string(enc) + content[m.end:], nil
		}
		// The scanner walks the file in step with the parsed keys, so the nth pair is the nth member.
		n = n.Obj[at].Value
		scan.at = obj.members[at].start
	}
	return content, nil
}

// same reports whether two nodes hold the same value, so a Set that changes nothing returns the file untouched.
func same(a, b Node) bool {
	if a.Kind != b.Kind || a.Num != b.Num || a.Str != b.Str || a.Bool != b.Bool || len(a.Arr) != len(b.Arr) || len(a.Obj) != len(b.Obj) {
		return false
	}
	for i := range a.Arr {
		if !same(a.Arr[i], b.Arr[i]) {
			return false
		}
	}
	for i := range a.Obj {
		if a.Obj[i].Key != b.Obj[i].Key || !same(a.Obj[i].Value, b.Obj[i].Value) {
			return false
		}
	}
	return true
}

// insertMember adds `"key": value` at the end of obj, on a line of its own indented like the members already there.
func insertMember(content string, obj objectAt, key string, enc []byte) (string, error) {
	var buf bytes.Buffer
	if err := writeString(&buf, key); err != nil {
		return content, err
	}
	buf.WriteString(": ")
	buf.Write(enc)
	member := buf.String()
	if len(obj.members) == 0 {
		return content[:obj.open+1] + member + content[obj.open+1:], nil
	}
	first := obj.members[0]
	last := obj.members[len(obj.members)-1]
	prevNL := strings.LastIndexByte(content[:first.keyStart], '\n')
	if prevNL < obj.open {
		// The object sits on one line, so a member added on a line of its own would be the only break in it.
		at, sep := last.end, ", "
		if last.comma >= 0 {
			at, sep = last.comma+1, " "
		}
		return content[:at] + sep + member + content[at:], nil
	}
	eol, indent := "\n", content[prevNL+1:first.keyStart]
	if prevNL > 0 && content[prevNL-1] == '\r' {
		eol = "\r\n"
	}
	if strings.Trim(indent, " \t") != "" {
		indent = "  "
	}
	at, sep := last.end, ","+eol+indent
	if last.comma >= 0 {
		at, sep = last.comma+1, eol+indent
	}
	return content[:at] + sep + member + content[at:], nil
}

// encodeInline writes n on one line, the form Set splices where the file's own value stood: a value split over lines
// would move bytes the edit is meant to leave alone.
func encodeInline(n Node) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeInline(&buf, n); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeInline(buf *bytes.Buffer, n Node) error {
	switch n.Kind {
	case Null:
		buf.WriteString("null")
	case Bool:
		buf.WriteString(strconv.FormatBool(n.Bool))
	case Number:
		buf.WriteString(n.Num.String())
	case String:
		return writeString(buf, n.Str)
	case Array:
		buf.WriteByte('[')
		for i, item := range n.Arr {
			if i > 0 {
				buf.WriteString(", ")
			}
			if err := writeInline(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case Object:
		buf.WriteByte('{')
		for i, p := range n.Obj {
			if i > 0 {
				buf.WriteString(", ")
			}
			if err := writeString(buf, p.Key); err != nil {
				return err
			}
			buf.WriteString(": ")
			if err := writeInline(buf, p.Value); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	}
	return nil
}

// scanner finds where the values of the file's text sit, so an edit can splice one without rewriting the rest. It
// only ever runs on text Parse has accepted, so it needs no error of its own for bad syntax.
type scanner struct {
	text string
	at   int
}

// objectAt is where an object's opening brace and its members sit in the text.
type objectAt struct {
	open    int
	members []memberAt
}

// memberAt is one member's key and value offsets; comma is the offset of the comma after the value, or -1.
type memberAt struct {
	keyStart int
	start    int
	end      int
	comma    int
}

// object reads the object whose "{" is next and returns where its parts sit.
func (p *scanner) object() (objectAt, error) {
	p.skip()
	var obj objectAt
	if p.at >= len(p.text) || p.text[p.at] != '{' {
		return obj, errors.New("jsonfile: expected an object")
	}
	obj.open = p.at
	p.at++
	for {
		p.skip()
		if p.at >= len(p.text) {
			return obj, errors.New("jsonfile: unexpected end of the value")
		}
		if p.text[p.at] == '}' {
			p.at++
			return obj, nil
		}
		m := memberAt{keyStart: p.at, comma: -1}
		if err := p.key(); err != nil {
			return obj, err
		}
		p.skip()
		if p.at >= len(p.text) || p.text[p.at] != ':' {
			return obj, errors.New("jsonfile: expected a colon")
		}
		p.at++
		p.skip()
		m.start = p.at
		if err := p.value(); err != nil {
			return obj, err
		}
		m.end = p.at
		p.skip()
		if p.at < len(p.text) && p.text[p.at] == ',' {
			m.comma = p.at
			p.at++
		}
		obj.members = append(obj.members, m)
	}
}

// value reads one value and leaves p.at just after it.
func (p *scanner) value() error {
	switch p.text[p.at] {
	case '{':
		_, err := p.object()
		return err
	case '[':
		p.at++
		for {
			p.skip()
			if p.at >= len(p.text) {
				return errors.New("jsonfile: unexpected end of the value")
			}
			if p.text[p.at] == ']' {
				p.at++
				return nil
			}
			if err := p.value(); err != nil {
				return err
			}
			p.skip()
			if p.at < len(p.text) && p.text[p.at] == ',' {
				p.at++
			}
		}
	case '"', '\'':
		return p.quoted()
	}
	for p.at < len(p.text) && !bareEnd(p.text[p.at]) {
		p.at++
	}
	return nil
}

// quoted skips a single- or double-quoted string.
func (p *scanner) quoted() error {
	quote := p.text[p.at]
	if quote != '"' && quote != '\'' {
		return errors.New("jsonfile: expected a string")
	}
	for p.at++; p.at < len(p.text); {
		switch p.text[p.at] {
		case '\\':
			p.at += 2
		case quote:
			p.at++
			return nil
		default:
			p.at++
		}
	}
	return errors.New("jsonfile: unterminated string")
}

// key skips an object key, which a lenient file may leave unquoted.
func (p *scanner) key() error {
	if c := p.text[p.at]; c == '"' || c == '\'' {
		return p.quoted()
	}
	for p.at < len(p.text) && p.text[p.at] != ':' && !bareEnd(p.text[p.at]) {
		p.at++
	}
	return nil
}

// skip passes whitespace and the comments a lenient reader accepts.
func (p *scanner) skip() {
	for p.at < len(p.text) {
		switch c := p.text[p.at]; {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			p.at++
		case c == '/' && p.at+1 < len(p.text) && p.text[p.at+1] == '/':
			for p.at < len(p.text) && p.text[p.at] != '\n' {
				p.at++
			}
		case c == '/' && p.at+1 < len(p.text) && p.text[p.at+1] == '*':
			block := strings.Index(p.text[p.at+2:], "*/")
			if block < 0 {
				p.at = len(p.text)
				return
			}
			p.at += block + 4
		default:
			return
		}
	}
}

// bareEnd reports the bytes that end a number, true, false or null.
func bareEnd(c byte) bool {
	switch c {
	case ' ', '\t', '\r', '\n', ',', '}', ']', '/':
		return true
	}
	return false
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
