package jsonfile

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestParseMarshalKeepsOrderAndTypes(t *testing.T) {
	t.Parallel()
	in := []byte(`{"z":1,"a":{"n":1.5,"flag":true,"s":"x","xs":["p","q"],"extra":null},"mixed":[1,"a"]}`)
	n, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := n.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(`{
  "z": 1,
  "a": {
    "n": 1.5,
    "flag": true,
    "s": "x",
    "xs": [
      "p",
      "q"
    ],
    "extra": null
  },
  "mixed": [
    1,
    "a"
  ]
}
`)
	if !bytes.Equal(got, want) {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	again, err := Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := again.Marshal(); err != nil || !bytes.Equal(out, want) {
		t.Fatalf("second pass:\n%s %v", out, err)
	}
}

func TestParseAcceptsCommentsAndTrailingCommas(t *testing.T) {
	t.Parallel()
	n, err := Parse([]byte("{\n  \"other\":42, // retained\n  /* block */\n  \"s\":\"x\",\n}"))
	if err != nil {
		t.Fatal(err)
	}
	want := Node{Kind: Object, Obj: []Pair{
		{Key: "other", Value: Node{Kind: Number, Num: "42"}},
		{Key: "s", Value: Node{Kind: String, Str: "x"}},
	}}
	if !equal(n, want) {
		t.Fatalf("got %+v", n)
	}
}

func TestParseRefusesTrailingData(t *testing.T) {
	t.Parallel()
	for _, in := range []string{`{"a":1} {}`, "1 2", `{} x`} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("%q: accepted", in)
		}
	}
}

func TestSplitPathKeepsADottedKeyWhole(t *testing.T) {
	t.Parallel()
	parts, err := SplitPath(` Keys.Open\.Menu `)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Keys", "Open.Menu"}; !slices.Equal(parts, want) {
		t.Fatalf("parts = %q, want %q", parts, want)
	}
	parts, err = SplitPath(`a\\b`)
	if err != nil || !slices.Equal(parts, []string{`a\b`}) {
		t.Fatalf("parts = %q %v", parts, err)
	}
	for _, field := range []string{"", ".", "a..b", "a. .b"} {
		if _, err := SplitPath(field); !errors.Is(err, ErrBadPath) {
			t.Errorf("%q: %v, want ErrBadPath", field, err)
		}
	}
}

func TestSetKeepsTheFilesKeySpelling(t *testing.T) {
	t.Parallel()
	n, err := Parse([]byte(`{"other":42, "ENABLED":false}`))
	if err != nil {
		t.Fatal(err)
	}
	n, err = Set(n, []string{"enabled"}, Node{Kind: Bool, Bool: true})
	if err != nil {
		t.Fatal(err)
	}
	out, err := n.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"other\": 42,\n  \"ENABLED\": true\n}\n"
	if string(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestSetKeepsANumberAsWritten(t *testing.T) {
	t.Parallel()
	n, err := Parse([]byte(`{"z":1,"nested":{"foo":1.50}}`))
	if err != nil {
		t.Fatal(err)
	}
	n, err = Set(n, []string{"nested", "foo"}, Node{Kind: Number, Num: "2.25"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := n.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "2.25") || strings.Index(string(out), `"z"`) > strings.Index(string(out), `"nested"`) {
		t.Fatalf("got:\n%s", out)
	}
}

func TestSetAppendsAMissingLastKey(t *testing.T) {
	t.Parallel()
	n, err := Parse([]byte(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	n, err = Set(n, []string{"New.Key"}, Node{Kind: String, Str: "v"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := n.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"a\": 1,\n  \"New.Key\": \"v\"\n}\n"
	if string(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestSetRefusesAStrayPath(t *testing.T) {
	t.Parallel()
	n, err := Parse([]byte(`{"a":{"b":1},"s":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Set(n, []string{"a", "missing", "c"}, Node{Kind: Null}); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("got %v, want ErrUnknownKey", err)
	}
	if _, err := Set(n, []string{"s", "c"}, Node{Kind: Null}); !errors.Is(err, ErrNotObject) {
		t.Errorf("got %v, want ErrNotObject", err)
	}
}

func TestCoerceTypesTheThreeKinds(t *testing.T) {
	t.Parallel()
	got, err := Coerce(" True ", Bool)
	if err != nil || got.Kind != Bool || !got.Bool {
		t.Fatalf("bool: %+v %v", got, err)
	}
	got, err = Coerce(" 2.50 ", Number)
	if err != nil || got.Kind != Number || got.Num != "2.50" {
		t.Fatalf("number: %+v %v", got, err)
	}
	got, err = Coerce(" x ", String)
	if err != nil || got.Kind != String || got.Str != " x " {
		t.Fatalf("string: %+v %v", got, err)
	}
	if _, err := Coerce("yes", Bool); !errors.Is(err, ErrNotBool) {
		t.Errorf("bool: %v, want ErrNotBool", err)
	}
	if _, err := Coerce("1,5", Number); !errors.Is(err, ErrNotNumber) {
		t.Errorf("number: %v, want ErrNotNumber", err)
	}
}

func TestNodeText(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		node Node
		want string
	}{
		{Node{Kind: Null}, "null"},
		{Node{Kind: Bool, Bool: true}, "true"},
		{Node{Kind: Number, Num: "1.50"}, "1.50"},
		{Node{Kind: String, Str: "x"}, "x"},
		{Node{Kind: Array, Arr: []Node{{Kind: Number, Num: "1"}}}, "[\n  1\n]"},
		{Node{Kind: Object, Obj: []Pair{{Key: "a", Value: Node{Kind: Null}}}}, "{\n  \"a\": null\n}"},
	} {
		if got := tc.node.Text(); got != tc.want {
			t.Errorf("%+v: got %q, want %q", tc.node, got, tc.want)
		}
	}
}

func TestLookupFollowsThePathWithoutCase(t *testing.T) {
	t.Parallel()
	n, err := Parse([]byte(`{"a":{"B":1.50}}`))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := n.Lookup([]string{"A", "b"})
	if !ok || got.Kind != Number || got.Num != "1.50" {
		t.Fatalf("got %+v %v", got, ok)
	}
	if _, ok := n.Lookup([]string{"a", "c"}); ok {
		t.Error("a missing key was found")
	}
	if _, ok := n.Lookup([]string{"a", "B", "c"}); ok {
		t.Error("a path through a number was found")
	}
}

// FuzzParseMarshal holds that a value this package parses comes back from Marshal as itself: parsing the output again
// gives the same node, and Marshal never panics.
func FuzzParseMarshal(f *testing.F) {
	f.Add([]byte(`{"z":1,"a":{"n":1.5,"s":"x"},"xs":[true,null]}`))
	f.Add([]byte("{a:'b', // c\n s:[1,],}"))
	f.Add([]byte(`"str"`))
	f.Add([]byte(`{"a":1} {}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		n, err := Parse(data)
		if err != nil {
			return
		}
		out, err := n.Marshal()
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		again, err := Parse(out)
		if err != nil {
			t.Fatalf("Parse(Marshal(n)): %v\n%s", err, out)
		}
		if !equal(again, n) {
			t.Fatalf("%q: got %+v, want %+v", data, again, n)
		}
	})
}

// equal compares two nodes by value, since slices are not comparable.
func equal(a, b Node) bool {
	if a.Kind != b.Kind || a.Num != b.Num || a.Str != b.Str || a.Bool != b.Bool || len(a.Arr) != len(b.Arr) || len(a.Obj) != len(b.Obj) {
		return false
	}
	for i := range a.Arr {
		if !equal(a.Arr[i], b.Arr[i]) {
			return false
		}
	}
	for i := range a.Obj {
		if a.Obj[i].Key != b.Obj[i].Key || !equal(a.Obj[i].Value, b.Obj[i].Value) {
			return false
		}
	}
	return true
}
