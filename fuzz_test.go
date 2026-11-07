package cfgedit

import (
	"slices"
	"strings"
	"testing"

	"github.com/Rethunk-Tech/go-cfgedit/internal/textdoc"
)

// FuzzBepInExSet holds that rewriting one setting with its own value never panics, never moves or changes any other
// line (lines as the parser cuts them, a lone "\r" included), and leaves a line already written as "Key = value" byte for byte as it was, each line keeping its own ending.
func FuzzBepInExSet(f *testing.F) {
	f.Add("## Settings file was created by plugin P v1.0\n## Plugin GUID: a.b\n\n[General]\n\n## Hi\n# Setting type: Boolean\n# Default value: true\nEnabled = true\n")
	f.Add("[A]\nKey=1\r\nOther =  two\n")
	f.Add("[x]\n# Setting type: Int32\n# Acceptable value range: From 1 to 5\nN = 3\n=\n[\n")
	f.Add("\xef\xbb\xbf[A]\r\n# Setting type: Int32\nK = 1\nL = 2\r\nM = 3")
	f.Fuzz(func(t *testing.T, text string) {
		d := ParseBepInEx(text)
		for _, listed := range d.entries {
			// A repeated key edits its first line, which is the entry Set targets.
			e, _ := d.find(listed.Section, listed.Key)
			out, err := d.Set(e.Section, e.Key, e.Value)
			if err != nil {
				continue
			}
			// Lines are the parser's own (a lone "\r" ends one too), which is what e.line indexes.
			got, want := textdoc.SplitCR(out), textdoc.SplitCR(text)
			if len(got.Lines) != len(want.Lines) {
				t.Fatalf("line count %d, want %d", len(got.Lines), len(want.Lines))
			}
			for i := range got.Lines {
				if i != e.line && got.Lines[i] != want.Lines[i] {
					t.Fatalf("line %d changed: %q -> %q", i, want.Lines[i], got.Lines[i])
				}
				if got.EOLs[i] != want.EOLs[i] {
					t.Fatalf("line %d ending %q, was %q", i, got.EOLs[i], want.EOLs[i])
				}
			}
			// Validation canonicalises a value ("truE" becomes "true"), which is a change; anything it keeps must not move.
			if canon, _ := e.validate(e.Value); canon == e.Value && d.doc.Lines[e.line] == e.Key+" = "+e.Value && out != text {
				t.Fatalf("an unchanged value rewrote the file:\n%q\n%q", text, out)
			}
		}
	})
}

// FuzzMelonSet holds that setting an entry leaves every line before it and after its value's end unchanged, and the
// line count drops by exactly the value's extra lines.
func FuzzMelonSet(f *testing.F) {
	f.Add(melonSample)
	f.Add("[M]\r\nA = 1\nB = [\r\n 1,\r\n]\nC = 3")
	f.Add("[Mod]\nNote = '''\nsay \"a=b\"\nC:\\x'''\nGrid = [\n\t[ \"[\", 1, ],\n]\nAfter = 1\n")
	f.Add("\xef\xbb\xbf[A]\nK = \"x\" # c\n[[B]]\nV = { a = 1 }\n")
	f.Fuzz(func(t *testing.T, text string) {
		d := ParseMelon(text)
		before := strings.SplitAfter(text, "\n")
		for _, listed := range d.entries {
			// A repeated key edits its first entry.
			e := d.entries[slices.IndexFunc(d.entries, func(o melonEntry) bool { return o.Section == listed.Section && o.Key == listed.Key })]
			out, err := d.Set(e.Section, e.Key, e.Value)
			if err != nil {
				continue
			}
			got := strings.SplitAfter(out, "\n")
			if len(got) != len(before)-(e.end-e.line) {
				t.Fatalf("line count %d, want %d", len(got), len(before)-(e.end-e.line))
			}
			for i := range e.line {
				if got[i] != before[i] {
					t.Fatalf("line %d changed: %q -> %q", i, before[i], got[i])
				}
			}
			for i := e.end + 1; i < len(before); i++ {
				if got[i-(e.end-e.line)] != before[i] {
					t.Fatalf("line %d changed: %q -> %q", i, before[i], got[i-(e.end-e.line)])
				}
			}
		}
	})
}
