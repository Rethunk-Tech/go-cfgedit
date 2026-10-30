// Package textdoc splits text into lines that remember their own ending, so an edit rewrites one line and every other
// byte comes back as it was.
package textdoc

import (
	"slices"
	"strings"
)

// utf8BOM is the UTF-8 byte order mark.
const utf8BOM = "\xef\xbb\xbf"

// Doc is text cut at "\n". Lines holds each line without its ending, EOLs the ending it had: "\r\n", "\n", or ""
// for an unterminated last line. Text ending in a newline therefore has a final empty line.
type Doc struct {
	BOM   bool
	Lines []string
	EOLs  []string
}

// Split cuts text into a Doc, taking a leading UTF-8 byte order mark off the first line.
func Split(text string) Doc {
	text, bom := strings.CutPrefix(text, utf8BOM)
	d := Doc{BOM: bom}
	parts := strings.Split(text, "\n")
	for i, l := range parts {
		switch cut, ok := strings.CutSuffix(l, "\r"); {
		case i == len(parts)-1:
			// No newline follows the last piece, so a trailing "\r" is content, not an ending.
			d.Lines, d.EOLs = append(d.Lines, l), append(d.EOLs, "")
		case ok:
			d.Lines, d.EOLs = append(d.Lines, cut), append(d.EOLs, "\r\n")
		default:
			d.Lines, d.EOLs = append(d.Lines, l), append(d.EOLs, "\n")
		}
	}
	return d
}

// Join is the text the Doc was split from, or the edited text when Lines and EOLs were changed in step.
func (d Doc) Join() string {
	var b strings.Builder
	if d.BOM {
		b.WriteString(utf8BOM)
	}
	for i, l := range d.Lines {
		b.WriteString(l)
		b.WriteString(d.EOLs[i])
	}
	return b.String()
}

// EOL is the ending for a line added to the Doc: "\r\n" when any line has one, else "\n".
func (d Doc) EOL() string {
	if slices.Contains(d.EOLs, "\r\n") {
		return "\r\n"
	}
	return "\n"
}
