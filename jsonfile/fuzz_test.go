package jsonfile

import (
	"errors"
	"strings"
	"testing"
)

func FuzzSet(f *testing.F) {
	f.Add(`{"other":42, "ENABLED":false}`, "enabled", byte(Bool), "true")
	f.Add(`{"z":1,"nested":{"foo":1.50}}`, "nested.foo", byte(Number), "2.25")
	f.Add("{\n  // the window size\n  \"Width\" :   1920,\n}", "width", byte(Number), "1280")
	f.Add("\ufeff{\r\n\t\"a\": 1,\r\n\t\"b\": 2\r\n}\r\n", "B", byte(Number), "3")
	f.Add("{a:'b', // c\n s:[1,],}", "a", byte(String), "z")
	f.Add(`{"a":1}`, "", byte(String), "x")
	f.Add("{}", "New.Key", byte(String), "v")
	f.Fuzz(func(t *testing.T, content, field string, kind byte, raw string) {
		if len(content) > 1<<16 || len(raw) > 1<<12 {
			t.Skip()
		}
		path, err := SplitPath(field)
		if err != nil {
			if field != "" {
				return
			}
			path = nil
		}
		v, err := Coerce(raw, Kind(kind))
		if err != nil {
			return
		}
		out, err := Set(content, path, v)
		if err != nil {
			if out != content {
				t.Fatalf("a refusal changed the text")
			}
			return
		}
		if path != nil && strings.HasPrefix(content, "\ufeff") && !strings.HasPrefix(out, "\ufeff") {
			t.Fatalf("byte order mark lost: %q -> %q", content, out)
		}
		n, err := Parse([]byte(out))
		if err != nil {
			t.Fatalf("Set wrote a file that does not parse: %v\n%s", err, out)
		}
		if len(out) > len(content)+len(raw)*8+len(field)+4096 {
			t.Fatalf("unbounded write: %d from %d", len(out), len(content))
		}
		got, ok := n.Lookup(path)
		if !ok || !same(got, v) {
			t.Fatalf("Lookup after Set: ok=%v got=%+v want=%+v", ok, got, v)
		}
	})
}

func FuzzSplitPath(f *testing.F) {
	f.Add(` Keys.Open\.Menu `)
	f.Add(`a\\b`)
	f.Add("")
	f.Add(".")
	f.Add("a..b")
	f.Add("a. .b")
	f.Fuzz(func(t *testing.T, field string) {
		if len(field) > 1<<16 {
			t.Skip()
		}
		parts, err := SplitPath(field)
		if err != nil {
			if !errors.Is(err, ErrBadPath) {
				t.Fatalf("unexpected error %v", err)
			}
			return
		}
		if len(parts) > len(field)+1 {
			t.Fatalf("parts=%d len=%d", len(parts), len(field))
		}
		for i, p := range parts {
			if p == "" {
				t.Fatalf("empty part %d in %q", i, field)
			}
		}
	})
}

func FuzzCoerce(f *testing.F) {
	f.Add(" True ", byte(Bool))
	f.Add(" 2.50 ", byte(Number))
	f.Add(" x ", byte(String))
	f.Add("yes", byte(Bool))
	f.Add("1,5", byte(Number))
	f.Add("1e309", byte(Number))
	f.Fuzz(func(t *testing.T, raw string, kind byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		n, err := Coerce(raw, Kind(kind))
		switch Kind(kind) {
		case Bool:
			if err != nil {
				if !errors.Is(err, ErrNotBool) {
					t.Fatalf("unexpected error %v", err)
				}
				return
			}
			if n.Kind != Bool {
				t.Fatalf("kind %v, want Bool", n.Kind)
			}
		case Number:
			if err != nil {
				if !errors.Is(err, ErrNotNumber) {
					t.Fatalf("unexpected error %v", err)
				}
				return
			}
			if n.Kind != Number || n.Num.String() != strings.TrimSpace(raw) {
				t.Fatalf("got %+v, want trimmed %q", n, raw)
			}
		default:
			if err != nil {
				t.Fatalf("String refused: %v", err)
			}
			if n.Kind != String || n.Str != raw {
				t.Fatalf("got %+v, want String %q", n, raw)
			}
		}
	})
}
