package cfgedit

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/Rethunk-Tech/go-cfgedit/internal/textdoc"
)

// MelonLoader keeps every melon's settings in UserData/MelonPreferences.cfg, a TOML file (MelonLoader/Preferences,
// Tomlet): one [Category] table per melon or settings group, and per entry its description as the comment lines above
// `Key = value` and an optional inline `# comment`. The file names no types, ranges or defaults, so the type of an entry
// is read off its value.

var (
	// leadingZero is a decimal integer TOML forbids: 007, but not 0, 0x1F or 0.
	leadingZero = regexp.MustCompile(`^[+-]?0\d`)
	tomlInt     = regexp.MustCompile(`^[+-]?\d+$`)
	// tomlIntForms are the integers TOML spells: underscores between digits, and 0x, 0o and 0b without a sign.
	tomlIntForms = regexp.MustCompile(`^([+-]?\d+(_\d+)*|0x[0-9a-fA-F]+(_[0-9a-fA-F]+)*|0o[0-7]+(_[0-7]+)*|0b[01]+(_[01]+)*)$`)
	tomlFloat    = regexp.MustCompile(`^[+-]?(\d+(\.\d+)?([eE][+-]?\d+)?|inf|nan)$`)
)

// Melon is a parsed MelonPreferences.cfg. It keeps the original lines, so a Set changes only the edited value.
type Melon struct {
	doc     textdoc.Doc
	entries []melonEntry
}

type melonEntry struct {
	Entry
	// line is where the entry starts and end where its value ends: Tomlet writes some values over several lines.
	line, end int
	// prefix is the line up to and including "=" and its following space; suffix an inline comment with its spaces.
	prefix, suffix string
}

// ParseMelon reads a MelonPreferences.cfg. It never fails: a line it does not understand is skipped.
func ParseMelon(text string) *Melon {
	d := &Melon{doc: textdoc.Split(text)}
	lines := d.doc.Lines
	var section string
	var desc []string
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			desc = nil
		case strings.HasPrefix(line, "#"):
			desc = append(desc, strings.TrimSpace(strings.TrimPrefix(line, "#")))
		case strings.HasPrefix(line, "["):
			section, desc = tableName(line), nil
		default:
			eq := keyEnd(raw)
			if eq < 0 {
				desc = nil
				continue
			}
			value, suffix := splitInline(strings.TrimSpace(raw[eq+1:]))
			end := valueEnd(lines, i, value)
			if end > i {
				value, suffix = strings.Join(append([]string{value}, lines[i+1:end+1]...), "\n"), ""
			}
			typ, val := melonType(value)
			d.entries = append(d.entries, melonEntry{
				Section: section, Key: strings.Trim(strings.TrimSpace(raw[:eq]), `"'`),
				Description: strings.Join(desc, "\n"), Type: typ, Value: val,
				line: i, end: end, prefix: raw[:eq+1] + " ", suffix: suffix,
			})
			desc = nil
			i = end
		}
	}
	return d
}

// keyEnd is the index of the `=` that ends a key, skipping one inside a quoted key; -1 when there is none.
func keyEnd(raw string) int {
	var quote byte
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case quote != 0:
			if c == '\\' && quote == '"' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '=':
			return i
		}
	}
	return -1
}

// tableName is a table header's name, a quoted one without its quotes.
func tableName(line string) string {
	line, _ = splitInline(line)
	name := strings.Trim(line, "[] \t")
	if strings.HasPrefix(name, `"`) {
		if u, err := strconv.Unquote(name); err == nil {
			return u
		}
	}
	if len(name) >= 2 && name[0] == '\'' && name[len(name)-1] == '\'' {
		return name[1 : len(name)-1]
	}
	return name
}

// splitInline separates a value from a trailing `# comment`, which only counts outside a quoted string.
func splitInline(v string) (value, comment string) {
	var quote rune
	for i, r := range v {
		switch {
		case quote != 0:
			if r == quote && (quote == '\'' || !escaped(v, i)) {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '#':
			value = strings.TrimRight(v[:i], " \t")
			return value, v[len(value):]
		}
	}
	return v, ""
}

// escaped reports whether the byte at v[i] follows an odd run of backslashes, so a basic string's quote there is text.
func escaped(v string, i int) bool {
	n := 0
	for j := i - 1; j >= 0 && v[j] == '\\'; j-- {
		n++
	}
	return n%2 == 1
}

// valueEnd is the line a value opening on line i ends on: a multi-line string runs to its closing quotes, an array
// until its brackets close.
func valueEnd(lines []string, i int, value string) int {
	if end, ok := multiLineStringEnd(lines, i, value); ok {
		return end
	}
	return arrayEnd(lines, i, value)
}

func multiLineStringEnd(lines []string, i int, value string) (int, bool) {
	for _, q := range []string{"'''", `"""`} {
		if strings.HasPrefix(value, q) && !strings.Contains(value[len(q):], q) {
			for j := i + 1; j < len(lines); j++ {
				if strings.Contains(lines[j], q) {
					return j, true
				}
			}
			return len(lines) - 1, true
		}
	}
	return 0, false
}

func arrayEnd(lines []string, i int, value string) int {
	depth := 0
	for j := i; j < len(lines) && strings.HasPrefix(value, "["); j++ {
		text := lines[j]
		if j == i {
			text = value
		}
		if depth += bracketDepth(text); depth <= 0 {
			return j
		}
	}
	return i
}

// bracketDepth is how many more brackets a line opens than it closes, outside strings and comments.
func bracketDepth(line string) int {
	value, _ := splitInline(strings.TrimSpace(line))
	var quote rune
	depth := 0
	for i, r := range value {
		switch {
		case quote != 0:
			if r == quote && (quote == '\'' || !escaped(value, i)) {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '[':
			depth++
		case r == ']':
			depth--
		}
	}
	return depth
}

// melonType is the entry type of a TOML value and the text to show for it.
func melonType(v string) (typ Type, shown string) {
	for _, q := range []string{"'''", `"""`} {
		if inner, ok := strings.CutPrefix(v, q); ok {
			inner = strings.TrimPrefix(strings.TrimSuffix(inner, q), "\n")
			return TypeString, inner
		}
	}
	switch {
	case v == "true" || v == "false":
		return TypeBool, v
	case tomlInt.MatchString(v):
		return TypeInt, v
	case tomlFloat.MatchString(v):
		return TypeFloat, v
	case strings.HasPrefix(v, `"`):
		if u, err := strconv.Unquote(v); err == nil {
			return TypeString, u
		}
	case strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'") && len(v) >= 2:
		return TypeString, v[1 : len(v)-1]
	case strings.HasPrefix(v, "["):
		return TypeList, v
	}
	return TypeRaw, v
}

// literal is the TOML text for a value of the entry's type, or an error when the text is not one.
func (e melonEntry) literal(value string) (string, error) {
	if strings.ContainsAny(value, "\r\n") {
		return "", refuse(e.Key, MultiLine)
	}
	value = strings.TrimSpace(value)
	switch e.Type {
	case TypeBool:
		if v := strings.ToLower(value); v == "true" || v == "false" {
			return v, nil
		}
		return "", refuse(e.Key, NotBool)
	case TypeInt:
		return e.intLiteral(value)
	case TypeRaw:
		if value == "" {
			return "", refuse(e.Key, Empty)
		}
		return value, nil
	case TypeFloat:
		return e.floatLiteral(value)
	case TypeList:
		if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
			return "", refuse(e.Key, NotList)
		}
		return value, nil
	case TypeString, TypeEnum, TypeColor:
	}
	return tomlString(value), nil
}

func (e melonEntry) intLiteral(value string) (string, error) {
	if !tomlIntForms.MatchString(value) || leadingZero.MatchString(value) {
		return "", refuse(e.Key, NotInteger)
	}
	clean := strings.ReplaceAll(value, "_", "")
	base := 10
	if len(clean) > 1 && clean[0] == '0' && clean[1] >= 'a' {
		base = 0
	}
	if _, err := strconv.ParseInt(clean, base, 64); err != nil {
		return "", refuse(e.Key, NotInteger)
	}
	return value, nil
}

func (e melonEntry) floatLiteral(value string) (string, error) {
	_, value, err := finite(e.Key, value)
	if err != nil {
		return "", err
	}
	if tomlInt.MatchString(value) {
		value += ".0"
	}
	return value, nil
}

// tomlString is value as a TOML basic string; strconv.Quote's \a, \v and \x escapes are not TOML.
func tomlString(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func parsesAsTOML(lines []string) bool {
	var sink map[string]any
	_, err := toml.Decode(strings.Join(lines, "\n"), &sink)
	return err == nil
}

// Entries lists the settings in file order.
func (d *Melon) Entries() []Entry {
	out := make([]Entry, len(d.entries))
	for i, e := range d.entries {
		out[i] = e.Entry
	}
	return out
}

// Set returns the file text with one value replaced, written as TOML, and every other byte as it was. It refuses a
// value of the wrong type and one that would leave a file that parsed unparsable (BreaksTOML); a missing setting wraps
// ErrNoSetting.
func (d *Melon) Set(section, key, value string) (string, error) {
	for _, e := range d.entries {
		if e.Section != section || e.Key != key {
			continue
		}
		lit, err := e.literal(value)
		if err != nil {
			return "", err
		}
		nd := d.doc
		nd.Lines = slices.Delete(slices.Clone(d.doc.Lines), e.line+1, e.end+1)
		nd.Lines[e.line] = e.prefix + lit + e.suffix
		// The value now ends where its last line did.
		nd.EOLs = slices.Delete(slices.Clone(d.doc.EOLs), e.line+1, e.end+1)
		nd.EOLs[e.line] = d.doc.EOLs[e.end]
		// One unparsable line makes MelonLoader run every melon on defaults and disable saving.
		parsed := parsesAsTOML(d.doc.Lines)
		if parsed && !parsesAsTOML(nd.Lines) {
			return "", refuse(e.Key, BreaksTOML)
		}
		// In a file that does not parse, a value over several lines may be an unclosed string or array running to
		// the end, and replacing its span would delete whatever follows.
		if !parsed && e.end > e.line {
			return "", refuse(e.Key, BreaksTOML)
		}
		return nd.Join(), nil
	}
	return "", noSetting(section, key)
}
