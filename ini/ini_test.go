package ini

import (
	"errors"
	"strings"
	"testing"
)

func TestSet(t *testing.T) {
	for _, c := range []struct{ name, in, section, want string }{
		{"rewrites case-insensitively", "[o]\r\nModsDisabled = 1\r\nx = 2\r\n", "", "[o]\r\nModsDisabled = 0\r\nx = 2\r\n"},
		{"ignores comments", "; modsdisabled = 1\n[o]\nmodsdisabled=1", "", "; modsdisabled = 1\n[o]\nmodsdisabled = 0"},
		{"appends when absent", "[o]\nx = 2", "", "[o]\nx = 2\nmodsdisabled = 0\n"},
		{"empty file", "", "", "modsdisabled = 0\n"},
		{"byte order mark on the first line", "\xef\xbb\xbfmodsdisabled = 1\n", "", "\xef\xbb\xbfmodsdisabled = 0\n"},
		{"byte order mark before a section", "\xef\xbb\xbf[Options]\r\nmodsdisabled = 1\r\n", "options", "\xef\xbb\xbf[Options]\r\nmodsdisabled = 0\r\n"},
		{"edits only inside its section", "[a]\nmodsdisabled = 1\n[b]\nmodsdisabled = 1\n", "B", "[a]\nmodsdisabled = 1\n[b]\nmodsdisabled = 0\n"},
		{"appends inside its section", "[a]\nx = 1\n\n[b]\ny = 2\n", "a", "[a]\nx = 1\nmodsdisabled = 0\n\n[b]\ny = 2\n"},
		{"appends inside the last section keeping CRLF", "[a]\r\nx = 1", "A", "[a]\r\nx = 1\r\nmodsdisabled = 0\r\n"},
		{"creates the section", "[a]\r\nx = 1\r\n", "b", "[a]\r\nx = 1\r\n[b]\r\nmodsdisabled = 0\r\n"},
		{"a header with a comment", "[Options] ; game\nmodsdisabled = 1\n", "options", "[Options] ; game\nmodsdisabled = 0\n"},
		{"fills an empty section", "[a]\n[b]\ny = 2\n", "a", "[a]\nmodsdisabled = 0\n[b]\ny = 2\n"},
		{"keeps each other line's ending", "[a]\nx = 1\r\nmodsdisabled = 1\ny = 2", "", "[a]\nx = 1\r\nmodsdisabled = 0\ny = 2"},
		{"drops indentation and an inline comment", "[a]\n  modsdisabled = 1 ; why\n", "", "[a]\nmodsdisabled = 0\n"},
	} {
		got, err := Set(c.in, c.section, "modsdisabled", "0")
		if err != nil || got != c.want {
			t.Errorf("%s: got %q (%v) want %q", c.name, got, err, c.want)
		}
	}
}

func TestSetRefusesNonTextUntouched(t *testing.T) {
	for _, in := range []string{"\xff\xfem\x00o\x00", "\xfe\xff\x00m\x00o", "a\x00b\x00"} {
		got, err := Set(in, "", "k", "v")
		if !errors.Is(err, ErrNotText) || got != in {
			t.Errorf("%q: got %q, %v", in, got, err)
		}
	}
}

func TestRefusalDoesNotCallANULInUTF8TextUTF16(t *testing.T) {
	err := Check("[o]\nk = a\x00b\n")
	if !errors.Is(err, ErrNotText) || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("got %v", err)
	}
}

func TestSetRefusesNamesThatWouldBreakTheLines(t *testing.T) {
	const in = "[a]\nx = 1\n"
	for _, c := range [][3]string{{"a", "k\nz", "v"}, {"a", "k", "v\r\nw = 2"}, {"a", "k", "v\n"}, {"a]\n[b", "k", "v"}, {"a\n", "k", "v"}, {"a", "k=z", "v"}, {"a", "#k", "v"}, {"a", " ", "v"}, {"a", "k", "v\x00"}} {
		got, err := Set(in, c[0], c[1], c[2])
		if !errors.Is(err, ErrBadName) || got != in {
			t.Errorf("%q: got %q, %v", c, got, err)
		}
	}
}

// Setting a value twice gives the same text as setting it once, and a byte order mark stays.
func FuzzSet(f *testing.F) {
	f.Add("[o]\r\nk = 1\r\n", "o", "k", "v")
	f.Add("\xef\xbb\xbf[a]\nx=1", "", "x", "2")
	f.Add("", "s", "k", "")
	f.Add("a\r", "", "a", "1")
	f.Fuzz(func(t *testing.T, content, section, key, value string) {
		once, err := Set(content, section, key, value)
		if err != nil {
			if once != content {
				t.Fatalf("a refusal changed the text: %q -> %q", content, once)
			}
			return
		}
		if strings.HasPrefix(content, "\xef\xbb\xbf") != strings.HasPrefix(once, "\xef\xbb\xbf") {
			t.Fatalf("byte order mark lost or added: %q -> %q", content, once)
		}
		twice, err := Set(once, section, key, value)
		if err != nil || twice != once {
			t.Fatalf("not idempotent: %q -> %q (%v)", once, twice, err)
		}
	})
}
