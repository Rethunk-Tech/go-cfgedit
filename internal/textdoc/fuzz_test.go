package textdoc

import (
	"strings"
	"testing"
)

func FuzzSplit(f *testing.F) {
	for _, s := range []string{"", "a", "a\n", "a\r\nb\nc", "\xef\xbb\xbfa\r\n\r\n", "\r\n", "a\r\r\n", "a\r", "a\nb\r"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 1<<16 {
			t.Skip()
		}
		roundTrip(t, Split(text), text, false)
	})
}

func FuzzSplitCR(f *testing.F) {
	for _, s := range []string{"", "a\rb", "a\r\r\n", "\r\r", "a\n\rb", "a\rb\r\nc\r", "\xef\xbb\xbfa\r\n"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 1<<16 {
			t.Skip()
		}
		roundTrip(t, SplitCR(text), text, true)
	})
}

func roundTrip(t *testing.T, d Doc, text string, loneCR bool) {
	t.Helper()
	if got := d.Join(); got != text {
		t.Fatalf("Join came back as %q, want %q", got, text)
	}
	if len(d.Lines) != len(d.EOLs) {
		t.Fatalf("lines=%d eols=%d", len(d.Lines), len(d.EOLs))
	}
	if n := len(d.Lines); n > len(text)+1 {
		t.Fatalf("lines=%d len=%d", n, len(text))
	}
	if d.BOM != strings.HasPrefix(text, utf8BOM) {
		t.Fatalf("BOM=%v prefix=%q", d.BOM, text)
	}
	for i, line := range d.Lines {
		if strings.Contains(line, "\n") {
			t.Fatalf("line %d holds a newline: %q", i, line)
		}
		if loneCR && strings.Contains(line, "\r") {
			t.Fatalf("line %d holds a CR: %q", i, line)
		}
		switch d.EOLs[i] {
		case "\n", "\r\n", "":
		case "\r":
			if !loneCR {
				t.Fatalf("line %d ending %q without SplitCR", i, d.EOLs[i])
			}
		default:
			t.Fatalf("line %d ending %q", i, d.EOLs[i])
		}
	}
}
