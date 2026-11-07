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
func Split(text string) Doc { return split(text, false) }

// SplitCR is Split for a reader that also ends a line at a lone "\r", as .NET's ReadLine does: such a line's ending
// is "\r", and a trailing "\r" ends a line too.
func SplitCR(text string) Doc { return split(text, true) }

func split(text string, loneCR bool) Doc {
	text, bom := strings.CutPrefix(text, utf8BOM)
	d := Doc{BOM: bom}
	start := 0
	for i := 0; i < len(text); i++ {
		var eol string
		switch {
		case text[i] == '\n' && i > start && text[i-1] == '\r':
			eol = "\r\n"
		case text[i] == '\n':
			eol = "\n"
		case text[i] == '\r' && loneCR && (i+1 == len(text) || text[i+1] != '\n'):
			eol = "\r"
		default:
			continue
		}
		end := i
		if eol == "\r\n" {
			end = i - 1
		}
		d.Lines, d.EOLs = append(d.Lines, text[start:end]), append(d.EOLs, eol)
		start = i + 1
	}
	d.Lines, d.EOLs = append(d.Lines, text[start:]), append(d.EOLs, "")
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
