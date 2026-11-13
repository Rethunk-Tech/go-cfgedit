// Package msbuild edits MSBuild files (.csproj, .props, .targets) in place: it parses them to find each element's exact
// bytes, then changes only those, so the developer's formatting, comments and attribute order stay as they wrote them.
package msbuild

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// Elem is one element of the file: its name, attributes, and where its bytes are.
type Elem struct {
	Name  string
	Attrs map[string]string
	// Start and End span the whole element; InnerStart and InnerEnd its content, equal for a self-closing one.
	Start, End           int
	InnerStart, InnerEnd int
	// Parent is the index of the enclosing element in the parse, -1 for the root.
	Parent int
	text   string
}

// Text is the element's content with XML entities resolved, empty when it holds child elements.
func (e Elem) Text() string {
	inner := e.text[e.InnerStart:e.InnerEnd]
	var out strings.Builder
	d := xml.NewDecoder(strings.NewReader("<x>" + inner + "</x>"))
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.CharData:
			out.Write(t)
		case xml.StartElement:
			if t.Name.Local != "x" {
				return ""
			}
		}
	}
	return out.String()
}

// Parse lists the elements of an MSBuild file in document order; comments and processing instructions are skipped.
func Parse(text string) ([]Elem, error) {
	d := xml.NewDecoder(strings.NewReader(text))
	var out []Elem
	var open []int
	var names []xml.Name
	for {
		start := int(d.InputOffset())
		tok, err := d.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			attrs := make(map[string]string, len(t.Attr))
			for _, a := range t.Attr {
				attrs[a.Name.Local] = a.Value
			}
			parent := -1
			if len(open) > 0 {
				parent = open[len(open)-1]
			}
			at := int(d.InputOffset())
			out = append(out, Elem{Name: t.Name.Local, Attrs: attrs, Start: start, InnerStart: at, InnerEnd: at, Parent: parent, text: text})
			open = append(open, len(out)-1)
			names = append(names, t.Name)
		case xml.EndElement:
			if len(open) == 0 {
				return nil, fmt.Errorf("unexpected </%s>", t.Name.Local)
			}
			// RawToken does not pair tags; <a><b></a></b> would otherwise parse.
			if want := names[len(names)-1]; want != t.Name {
				return nil, fmt.Errorf("</%s> closes <%s>", t.Name.Local, want.Local)
			}
			e := &out[open[len(open)-1]]
			open = open[:len(open)-1]
			names = names[:len(names)-1]
			e.End = int(d.InputOffset())
			// A self-closing element's end comes with no bytes of its own.
			if e.End != e.InnerStart {
				e.InnerEnd = start
			}
		}
	}
	if len(open) > 0 {
		return nil, errors.New("the file ends inside an element")
	}
	return out, nil
}

// Property is the effective property called name (the last unconditional one), unescaped; empty when the file has none or does not parse.
func Property(text, name string) string {
	elems, err := Parse(text)
	if err != nil {
		return ""
	}
	if e, ok := property(elems, name); ok {
		return unescape.Replace(strings.TrimSpace(e.Text()))
	}
	return ""
}

// property finds the definition MSBuild's evaluation ends with: the last one in document order. Conditions are not
// evaluated, so a property or group carrying a Condition is skipped while any unconditional definition exists; a name
// defined only conditionally resolves to its last definition, since that is all there is to read or edit.
func property(elems []Elem, name string) (Elem, bool) {
	var plain, last Elem
	havePlain, haveLast := false, false
	for _, e := range elems {
		if e.Parent < 0 || !strings.EqualFold(e.Name, name) || !strings.EqualFold(elems[e.Parent].Name, "PropertyGroup") {
			continue
		}
		last, haveLast = e, true
		if attrValue(e.Attrs, "Condition") == "" && attrValue(elems[e.Parent].Attrs, "Condition") == "" {
			plain, havePlain = e, true
		}
	}
	if havePlain {
		return plain, true
	}
	return last, haveLast
}

// SetProperty sets the effective property called name (the last unconditional one) to value, escaped for MSBuild and XML; a missing one is added to the
// first PropertyGroup, or to a new one.
func SetProperty(text, name, value string) (string, error) {
	return SetRawProperty(text, name, Escape(value))
}

// SetRawProperty is SetProperty for text already escaped, such as a list whose ; MSBuild must split on. raw is XML
// character data: markup in it (a tag, comment or CDATA section) is refused with ErrMarkup, because it would add
// elements to a project that dotnet builds.
func SetRawProperty(text, name, raw string) (string, error) {
	if err := checkName(name); err != nil {
		return text, err
	}
	if err := CheckValue(raw); err != nil {
		return text, fmt.Errorf("the value of %s: %w", name, err)
	}
	if err := checkText(raw); err != nil {
		return text, fmt.Errorf("the value of %s: %w", name, err)
	}
	elems, err := Parse(text)
	if err != nil {
		return text, err
	}
	if e, ok := property(elems, name); ok {
		if e.InnerStart == e.InnerEnd && e.End == e.InnerStart {
			open := strings.TrimSuffix(strings.TrimRight(text[e.Start:e.End], " \t\r\n"), "/>")
			open = strings.TrimRight(open, " \t")
			return text[:e.Start] + open + ">" + raw + "</" + e.Name + ">" + text[e.End:], nil
		}
		return text[:e.InnerStart] + raw + text[e.InnerEnd:], nil
	}
	line := "<" + name + ">" + raw + "</" + name + ">"
	for _, e := range elems {
		if strings.EqualFold(e.Name, "PropertyGroup") && e.Parent == 0 {
			return appendChild(text, elems, e, line), nil
		}
	}
	return appendGroup(text, elems, "PropertyGroup", line)
}

// RemoveProperty drops every property called name, with its line.
func RemoveProperty(text, name string) (string, error) {
	return Remove(text, func(e Elem, parent string) bool {
		return strings.EqualFold(e.Name, name) && strings.EqualFold(parent, "PropertyGroup")
	})
}

// Item is an item element: its type, its Include and its other attributes.
type Item struct {
	Type    string
	Include string
	Attrs   map[string]string
}

// Items lists the items of the given types in document order.
func Items(text string, types ...string) ([]Item, error) {
	elems, err := Parse(text)
	if err != nil {
		return nil, err
	}
	var out []Item
	for _, e := range elems {
		if hasType(types, e.Name) && e.Parent >= 0 && strings.EqualFold(elems[e.Parent].Name, "ItemGroup") {
			out = append(out, Item{Type: e.Name, Include: attrValue(e.Attrs, "Include"), Attrs: e.Attrs})
		}
	}
	return out, nil
}

// SetItem puts item in place of the first item whose type is one of same and whose Include matches; otherwise after the
// last item after picks, or in a new ItemGroup.
func SetItem(text string, item Item, same []string, after func(Item) bool) (string, error) {
	if err := checkName(item.Type); err != nil {
		return text, err
	}
	for k := range item.Attrs {
		if err := checkName(k); err != nil {
			return text, err
		}
	}
	for _, v := range append([]string{item.Include}, slices.Collect(maps.Values(item.Attrs))...) {
		if err := CheckValue(v); err != nil {
			return text, fmt.Errorf("the %s item %q: %w", item.Type, item.Include, err)
		}
	}
	elems, err := Parse(text)
	if err != nil {
		return text, err
	}
	xmlText := item.xml()
	var last *Elem
	for i, e := range elems {
		if e.Parent < 0 || !strings.EqualFold(elems[e.Parent].Name, "ItemGroup") {
			continue
		}
		if hasType(same, e.Name) && strings.EqualFold(attrValue(e.Attrs, "Include"), item.Include) {
			return text[:e.Start] + xmlText + text[e.End:], nil
		}
		if after(Item{Type: e.Name, Include: attrValue(e.Attrs, "Include"), Attrs: e.Attrs}) {
			last = &elems[i]
		}
	}
	if last != nil {
		// After the rest of its line, such as a trailing comment, unless that line also closes the group.
		at := last.End
		if le := strings.IndexByte(text[at:], '\n'); le >= 0 && !strings.Contains(text[at:at+le], "</") {
			at += le
			if text[at-1] == '\r' {
				at--
			}
		}
		return text[:at] + lineEnding(text) + indentOf(text, last.Start) + xmlText + text[at:], nil
	}
	return appendGroup(text, elems, "ItemGroup", xmlText)
}

// hasType reports whether typ is one of types; MSBuild compares item types case-insensitively.
func hasType(types []string, typ string) bool {
	return slices.ContainsFunc(types, func(t string) bool { return strings.EqualFold(t, typ) })
}

func attrValue(attrs map[string]string, name string) string {
	for k, v := range attrs {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	return ""
}

// Remove drops every element match picks, given its parent's name, with the line it stands on when nothing else
// shares that line.
func Remove(text string, match func(e Elem, parent string) bool) (string, error) {
	elems, err := Parse(text)
	if err != nil {
		return text, err
	}
	var spans [][2]int
	for _, e := range elems {
		parent := ""
		if e.Parent >= 0 {
			parent = elems[e.Parent].Name
		}
		// Elements come in document order, so a match inside an already removed one is gone with it.
		if match(e, parent) && (len(spans) == 0 || e.Start >= spans[len(spans)-1][1]) {
			spans = append(spans, lineSpan(text, e.Start, e.End))
		}
	}
	for _, span := range slices.Backward(spans) {
		text = text[:span[0]] + text[span[1]:]
	}
	return text, nil
}

// SetAttr sets attr to value on every element match picks, in place: an existing value's bytes are replaced, a missing
// attribute is added before the tag's end. It also serves .slnx solutions, which are XML.
func SetAttr(text string, match func(e Elem, parent string) bool, attr, value string) (string, error) {
	if err := checkName(attr); err != nil {
		return text, err
	}
	if err := CheckValue(value); err != nil {
		return text, fmt.Errorf("the %s attribute: %w", attr, err)
	}
	elems, err := Parse(text)
	if err != nil {
		return text, err
	}
	quoted := attr + `="` + escapeAttr(value) + `"`
	for _, e := range slices.Backward(elems) {
		parent := ""
		if e.Parent >= 0 {
			parent = elems[e.Parent].Name
		}
		if !match(e, parent) {
			continue
		}
		tag := text[e.Start:e.InnerStart]
		if from, to, ok := attrSpan(tag, attr); ok {
			tag = tag[:from] + quoted + tag[to:]
		} else {
			end := len(tag) - 1
			if strings.HasSuffix(tag, "/>") {
				end = len(tag) - 2
			}
			tag = strings.TrimRight(tag[:end], " \t") + " " + quoted + tag[end:]
			if strings.HasSuffix(tag, quoted+"/>") {
				tag = strings.TrimSuffix(tag, "/>") + " />"
			}
		}
		text = text[:e.Start] + tag + text[e.InnerStart:]
	}
	return text, nil
}

// attrSpan is the byte range of attr="value" (the name through the closing quote) in a start tag, found by walking
// its attributes so a quoted value that holds " attr='x'" is never mistaken for the attribute.
func attrSpan(tag, attr string) (from, to int, ok bool) {
	p := 1
	for p < len(tag) && !strings.ContainsRune(" \t\r\n/>", rune(tag[p])) {
		p++
	}
	for p < len(tag) {
		for p < len(tag) && strings.ContainsRune(" \t\r\n", rune(tag[p])) {
			p++
		}
		start := p
		for p < len(tag) && !strings.ContainsRune(" \t\r\n=/>", rune(tag[p])) {
			p++
		}
		name := tag[start:p]
		for p < len(tag) && strings.ContainsRune(" \t\r\n", rune(tag[p])) {
			p++
		}
		if name == "" || p >= len(tag) || tag[p] != '=' {
			return 0, 0, false
		}
		p++
		for p < len(tag) && strings.ContainsRune(" \t\r\n", rune(tag[p])) {
			p++
		}
		if p >= len(tag) || (tag[p] != '"' && tag[p] != '\'') {
			return 0, 0, false
		}
		closing := strings.IndexByte(tag[p+1:], tag[p])
		if closing < 0 {
			return 0, 0, false
		}
		p += closing + 2
		if strings.EqualFold(name, attr) {
			return start, p, true
		}
	}
	return 0, 0, false
}

func (it Item) xml() string {
	var b strings.Builder
	b.WriteString("<" + it.Type + ` Include="` + escapeAttr(it.Include) + `"`)
	keys := make([]string, 0, len(it.Attrs))
	for k := range it.Attrs {
		if k != "Include" {
			keys = append(keys, k)
		}
	}
	// Version first, as the template writes it, then the rest in a fixed order.
	slices.SortFunc(keys, func(a, b string) int {
		if a == "Version" || b == "Version" {
			return map[bool]int{true: -1, false: 1}[a == "Version"]
		}
		return strings.Compare(a, b)
	})
	for _, k := range keys {
		b.WriteString(" " + k + `="` + escapeAttr(it.Attrs[k]) + `"`)
	}
	b.WriteString(" />")
	return b.String()
}

// lineSpan widens [start,end) to its whole line, break included, when only whitespace shares that line.
func lineSpan(text string, start, end int) [2]int {
	ls := strings.LastIndexByte(text[:start], '\n') + 1
	le := strings.IndexByte(text[end:], '\n')
	if le < 0 {
		le = len(text)
	} else {
		le += end + 1
	}
	if strings.TrimSpace(text[ls:start]) == "" && strings.TrimSpace(text[end:le]) == "" {
		return [2]int{ls, le}
	}
	return [2]int{start, end}
}

func indentOf(text string, at int) string {
	ls := strings.LastIndexByte(text[:at], '\n') + 1
	return text[ls : ls+len(text[ls:at])-len(strings.TrimLeft(text[ls:at], " \t"))]
}

// lineEnding is the file's own break, taken from its first line.
func lineEnding(text string) string {
	if i := strings.IndexByte(text, '\n'); i > 0 && text[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

// indentStep is one level of the file's indent, read from the root's first child.
func indentStep(text string, elems []Elem) string {
	for _, e := range elems {
		if e.Parent == 0 {
			if in := indentOf(text, e.Start); in != "" {
				return in
			}
			break
		}
	}
	return "  "
}

// appendChild adds line as the last child of e, indented like its other children or one step past e.
func appendChild(text string, elems []Elem, e Elem, line string) string {
	nl := lineEnding(text)
	indent := indentOf(text, e.Start) + indentStep(text, elems)
	for _, c := range elems {
		if c.Start > e.Start && c.End <= e.End && elems[c.Parent].Start == e.Start {
			indent = indentOf(text, c.Start)
		}
	}
	at := e.InnerEnd
	ls := strings.LastIndexByte(text[:at], '\n') + 1
	if strings.TrimSpace(text[ls:at]) == "" && ls > e.InnerStart {
		return text[:ls] + indent + line + nl + text[ls:]
	}
	return text[:at] + nl + indent + line + nl + indentOf(text, e.Start) + text[at:]
}

// appendGroup adds a new group of kind holding line at the end of the root element.
func appendGroup(text string, elems []Elem, kind, line string) (string, error) {
	if len(elems) == 0 {
		return text, errors.New("the file has no root element")
	}
	root := elems[0]
	at := root.InnerEnd
	ls := strings.LastIndexByte(text[:at], '\n') + 1
	nl, step := lineEnding(text), indentStep(text, elems)
	group := step + "<" + kind + ">" + nl + step + step + line + nl + step + "</" + kind + ">" + nl
	if strings.TrimSpace(text[ls:at]) == "" && ls > root.InnerStart {
		return text[:ls] + group + text[ls:], nil
	}
	return text[:at] + nl + group + text[at:], nil
}

// MSBuild expands $(), @(), %XX and splits on ; in a property's text, so the characters it reads specially are written
// as %XX. The XML escape runs last: escaping first would turn the ; of &lt; into %3B and break the entity.
var (
	escapeMSBuild = strings.NewReplacer("%", "%25", "$", "%24", "@", "%40", ";", "%3B", "'", "%27", "*", "%2A", "?", "%3F")
	unescape      = strings.NewReplacer("%25", "%", "%24", "$", "%40", "@", "%3B", ";", "%27", "'", "%2A", "*", "%3F", "?")
	escapeXML     = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
)

// ErrBadName is an element, item type or attribute name that is not a plain XML name.
var ErrBadName = errors.New("msbuild: not a plain XML name")

// ErrMarkup is raw property text holding markup or a malformed entity.
var ErrMarkup = errors.New("msbuild: the raw text holds markup")

var plainName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

func checkName(name string) error {
	if !plainName.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrBadName, name)
	}
	return nil
}

// checkText accepts only character data: no tag, comment, CDATA section or processing instruction, and entity
// references that XML resolves.
func checkText(raw string) error {
	if strings.Contains(raw, "<") {
		return ErrMarkup
	}
	dec := xml.NewDecoder(strings.NewReader("<x>" + raw + "</x>"))
	dec.Strict = true
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: %w", ErrMarkup, err)
		}
		switch tok.(type) {
		case xml.StartElement, xml.EndElement, xml.CharData:
		default:
			return ErrMarkup
		}
	}
}

// ErrBadChar is a value holding invalid UTF-8 or a character XML 1.0 cannot carry.
var ErrBadChar = errors.New("holds a character XML cannot carry")

// CheckValue refuses a value that would make the file one XML cannot parse.
func CheckValue(v string) error {
	if !utf8.ValidString(v) || strings.ContainsFunc(v, func(r rune) bool { return !xmlChar(r) }) {
		return ErrBadChar
	}
	return nil
}

// xmlChar reports whether XML 1.0 allows r in text.
func xmlChar(r rune) bool {
	return r == '\t' || r == '\n' || r == '\r' || r >= 0x20 && r <= 0xD7FF || r >= 0xE000 && r <= 0xFFFD && r != 0xFFFE && r != 0xFFFF || r >= 0x10000
}

// Escape writes value as a property's text: MSBuild's special characters as %XX, then XML's.
func Escape(value string) string { return escapeXML.Replace(escapeMSBuild.Replace(value)) }

// EscapeXML escapes only what XML needs, for a value MSBuild should still read specially.
func EscapeXML(value string) string { return escapeXML.Replace(value) }

func escapeAttr(v string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(v)
}
