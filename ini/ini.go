// Package ini sets `key = value` lines in an ini-style file's text, leaving every other byte as it was.
package ini

import (
	"errors"
	"strings"

	"github.com/Rethunk-Tech/go-cfgedit/internal/textdoc"
)

// ErrNotText refuses content that is UTF-16 or holds NUL bytes (BOM-less UTF-16 shows as NULs): its bytes are not
// lines of text, and an edit would mangle it.
var ErrNotText = errors.New("ini: UTF-16 or NUL bytes, not editable as text")

// ErrBadName refuses a key that is empty, holds "=" or a line break, or starts with ";", "#" or "[", a value with a line
// break, a section with "]" or a line break, and a NUL in any of them: written as given they would become extra lines,
// a comment or a different section.
var ErrBadName = errors.New("ini: a key that is empty, holds = or starts with ; # [, a NUL or line break in a key or value, or ] in a section")

// Check reports why content cannot be edited: ErrNotText for such a file, nil otherwise.
func Check(content string) error {
	if strings.HasPrefix(content, "\xff\xfe") || strings.HasPrefix(content, "\xfe\xff") || strings.ContainsRune(content, 0) {
		return ErrNotText
	}
	return nil
}

// Set gives key the value inside section ("" for any section, taking the first match). The first line holding key in
// the section (names compared without case, comments ignored) is rewritten as `key = value`, which drops its
// indentation and any inline comment; a missing key is added at the end of its section, and a missing section is
// created at the end of the file. A leading UTF-8 byte order mark and every other line's ending are kept. Names are
// trimmed of surrounding space. It returns ErrNotText for UTF-16 or NUL content and ErrBadName for a name or value that
// would break the line structure, in both cases with content untouched.
func Set(content, section, key, value string) (string, error) {
	if err := Check(content); err != nil {
		return content, err
	}
	if strings.ContainsAny(key+value, "\r\n\x00") || strings.ContainsAny(section, "]\r\n\x00") {
		return content, ErrBadName
	}
	// Names are compared trimmed, so a padded one would never match the line it wrote.
	section, key = strings.TrimSpace(section), strings.TrimSpace(key)
	if badKey(key) {
		return content, ErrBadName
	}
	d := textdoc.Split(content)
	// Text ending in a newline splits to a final empty line, which is not a line of the file.
	if n := len(d.Lines); d.Lines[n-1] == "" {
		d.Lines, d.EOLs = d.Lines[:n-1], d.EOLs[:n-1]
	}
	at, last, sectionSeen := scan(d.Lines, section, key)
	if at >= 0 {
		k, _, _ := strings.Cut(d.Lines[at], "=")
		d.Lines[at] = strings.TrimSpace(k) + " = " + value
		return d.Join(), nil
	}
	return insert(d, section, key, value, last, sectionSeen).Join(), nil
}

// scan finds the line holding key in section, or -1. last is the index after the final non-blank line of the section,
// where a missing key goes (-1 when it has none), and sectionSeen says whether the section exists.
func scan(lines []string, section, key string) (at, last int, sectionSeen bool) {
	inWanted := section == ""
	last, sectionSeen = -1, section == ""
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case line == "" || line[0] == ';' || line[0] == '#':
			continue
		case line[0] == '[':
			cur, _, _ := strings.Cut(line[1:], "]")
			inWanted = section == "" || strings.EqualFold(strings.TrimSpace(cur), section)
			sectionSeen = sectionSeen || inWanted
			if inWanted {
				last = i + 1
			}
			continue
		}
		if !inWanted {
			continue
		}
		last = i + 1
		if k, _, ok := strings.Cut(line, "="); ok && strings.EqualFold(strings.TrimSpace(k), key) {
			return i, last, sectionSeen
		}
	}
	return -1, last, sectionSeen
}

// insert adds the missing key, and its section when that is missing too, ending every added line with d's line ending.
func insert(d textdoc.Doc, section, key, value string, last int, sectionSeen bool) textdoc.Doc {
	eol := d.EOL()
	if n := len(d.Lines); n > 0 && d.EOLs[n-1] == "" {
		d.EOLs[n-1] = eol
	}
	add := key + " = " + value
	switch {
	case section == "" || (sectionSeen && last < 0):
		d.Lines, d.EOLs = append(d.Lines, add), append(d.EOLs, eol)
	case sectionSeen:
		d.Lines = append(d.Lines[:last], append([]string{add}, d.Lines[last:]...)...)
		d.EOLs = append(d.EOLs[:last], append([]string{eol}, d.EOLs[last:]...)...)
	default:
		d.Lines, d.EOLs = append(d.Lines, "["+section+"]", add), append(d.EOLs, eol, eol)
	}
	return d
}

// badKey is a key no line could hold: empty, containing the separator, or opening like a comment or a header.
func badKey(key string) bool {
	return key == "" || strings.Contains(key, "=") || strings.ContainsRune(";#[", rune(key[0]))
}
