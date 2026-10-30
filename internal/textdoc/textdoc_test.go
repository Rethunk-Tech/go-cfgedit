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
