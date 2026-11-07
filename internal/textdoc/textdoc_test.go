package textdoc

import "testing"

func TestSplitJoinRoundTripsEveryEnding(t *testing.T) {
	for _, in := range []string{"", "a", "a\n", "a\r\nb\nc", "\xef\xbb\xbfa\r\n\r\n", "\r\n", "a\r\r\n", "a\r", "a\nb\r"} {
		if got := Split(in).Join(); got != in {
			t.Errorf("%q came back as %q", in, got)
		}
	}
}

func TestEOLIsCRLFWhenAnyLineHasOne(t *testing.T) {
	for in, want := range map[string]string{"a\nb": "\n", "a\nb\r\nc": "\r\n", "": "\n"} {
		if got := Split(in).EOL(); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestSplitCRBreaksAtALoneCR(t *testing.T) {
	d := SplitCR("a\rb\r\nc\r")
	if got := d.Lines; len(got) != 4 || got[0] != "a" || got[1] != "b" || got[2] != "c" || got[3] != "" {
		t.Fatalf("lines = %q", got)
	}
	if d.EOLs[0] != "\r" || d.EOLs[1] != "\r\n" || d.EOLs[2] != "\r" {
		t.Fatalf("eols = %q", d.EOLs)
	}
	for _, in := range []string{"", "a\rb", "a\r\r\n", "\r\r", "a\n\rb"} {
		if got := SplitCR(in).Join(); got != in {
			t.Errorf("%q came back as %q", in, got)
		}
	}
}
