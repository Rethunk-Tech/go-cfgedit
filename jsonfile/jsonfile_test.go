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

func TestSetSplicesInlineArraysQuotedKeysAndComments(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in, key, want string
		v             Node
	}{
		{`{"a":[1,2]}`, "a", `{"a":[3, 4]}`, Node{Kind: Array, Arr: []Node{{Kind: Number, Num: "3"}, {Kind: Number, Num: "4"}}}},
		{`{"a:b":1}`, "a:b", `{"a:b":2}`, Node{Kind: Number, Num: "2"}},
		{`{'a:b':1}`, "a:b", `{'a:b':2}`, Node{Kind: Number, Num: "2"}},
		{`{"a":1, /*c*/ "b":2}`, "b", `{"a":1, /*c*/ "b":3}`, Node{Kind: Number, Num: "3"}},
		{`{"a":1, /**/ "b":2}`, "b", `{"a":1, /**/ "b":3}`, Node{Kind: Number, Num: "3"}},
	} {
		out, err := Set(tc.in, []string{tc.key}, tc.v)
		if err != nil || out != tc.want {
			t.Errorf("%q: got %q (%v) want %q", tc.in, out, err, tc.want)
		}
	}
}

func TestSetKeepsTheFilesKeySpelling(t *testing.T) {
	t.Parallel()
	in := `{"other":42, "ENABLED":false}`
	out, err := Set(in, []string{"enabled"}, Node{Kind: Bool, Bool: true})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"other":42, "ENABLED":true}`
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestSetKeepsANumberAsWritten(t *testing.T) {
	t.Parallel()
	in := `{"z":1,"nested":{"foo":1.50}}`
	out, err := Set(in, []string{"nested", "foo"}, Node{Kind: Number, Num: "2.25"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"z":1,"nested":{"foo":2.25}}`; out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

// TestSetChangesOnlyTheValue holds the package promise: comments, spacing and key order survive an edit, and the one
// byte range that differs is the old value's.
func TestSetChangesOnlyTheValue(t *testing.T) {
	t.Parallel()
	in := "{\n" +
		"  // the window size\n" +
		"  \"Width\" :   1920,\n" +
		"\t\"Height\":1080,   /* kept */\n" +
		"  \"Title\": \"a  b\"\n" +
		"}\n"
	out, err := Set(in, []string{"width"}, Node{Kind: Number, Num: "1280"})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(in, "1920", "1280", 1)
	if out != want {
		t.Fatalf("got:\n%q\nwant:\n%q", out, want)
	}
}

func TestSetLeavesAnUnchangedFileByteIdentical(t *testing.T) {
	t.Parallel()
	in := "{\n\t'a': [1, 2,], // c\n\t\"b\": {\"c\": true},\n}\n"
	for _, tc := range []struct {
		path []string
		v    Node
	}{
		{[]string{"a"}, Node{Kind: Array, Arr: []Node{{Kind: Number, Num: "1"}, {Kind: Number, Num: "2"}}}},
		{[]string{"b", "c"}, Node{Kind: Bool, Bool: true}},
	} {
		out, err := Set(in, tc.path, tc.v)
		if err != nil {
			t.Fatal(err)
		}
		if out != in {
			t.Errorf("%q: got\n%q\nwant it unchanged", tc.path, out)
		}
	}
}

func TestSetAppendsAMissingLastKey(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{"{\n  \"a\": 1\n}\n", "{\n  \"a\": 1,\n  \"New.Key\": \"v\"\n}\n"},
		{
			"{\r\n    \"a\": {\r\n        \"b\": true\r\n    }\r\n}",
			"{\r\n    \"a\": {\r\n        \"b\": true,\r\n        \"New.Key\": \"v\"\r\n    }\r\n}",
		},
		{"{\"a\":1}", `{"a":1, "New.Key": "v"}`},
		{"{}", `{"New.Key": "v"}`},
	} {
		path := []string{"New.Key"}
		if strings.Contains(tc.in, `"a": {`) {
			path = []string{"a", "New.Key"}
		}
		out, err := Set(tc.in, path, Node{Kind: String, Str: "v"})
		if err != nil {
			t.Fatal(err)
		}
		if out != tc.want {
			t.Errorf("got:\n%q\nwant:\n%q", out, tc.want)
		}
	}
}

func TestSetKeepsAByteOrderMarkAndCRLF(t *testing.T) {
	t.Parallel()
	in := "\ufeff{\r\n\t\"a\": 1,\r\n\t\"b\": 2\r\n}\r\n"
	out, err := Set(in, []string{"B"}, Node{Kind: Number, Num: "3"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "\ufeff{\r\n\t\"a\": 1,\r\n\t\"b\": 3\r\n}\r\n"; out != want {
		t.Fatalf("got:\n%q\nwant:\n%q", out, want)
	}
}

func TestSetRefusesAStrayPath(t *testing.T) {
	t.Parallel()
	in := `{"a":{"b":1},"s":"x"}`
	if _, err := Set(in, []string{"a", "missing", "c"}, Node{Kind: Null}); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("got %v, want ErrUnknownKey", err)
	}
	if _, err := Set(in, []string{"s", "c"}, Node{Kind: Null}); !errors.Is(err, ErrNotObject) {
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
		{Node{Kind: Array, Arr: []Node{{Kind: Array, Arr: []Node{{Kind: Number, Num: "1"}}}}}, "[\n  [\n    1\n  ]\n]"},
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

func TestSetAndLookupUseTheLastMatchingKey(t *testing.T) {
	t.Parallel()
	in := `{"Enabled": false, "enabled": true}`
	n, err := Parse([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := n.Lookup([]string{"enabled"})
	if !ok || got.Kind != Bool || !got.Bool {
		t.Fatalf("Lookup = %+v %v", got, ok)
	}
	out, err := Set(in, []string{"enabled"}, Node{Kind: Bool, Bool: false})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"Enabled": false, "enabled": false}`; out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestCoerceRefusesNonJSONNumbers(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"NaN", "Inf", "+1", "01"} {
		if _, err := Coerce(in, Number); !errors.Is(err, ErrNotNumber) {
			t.Errorf("%q: %v, want ErrNotNumber", in, err)
		}
	}
	got, err := Coerce("2.50", Number)
	if err != nil || got.Kind != Number || got.Num != "2.50" {
		t.Fatalf("2.50: %+v %v", got, err)
	}
}

func TestMarshalKeepsHTMLCharactersUnescaped(t *testing.T) {
	n, err := Parse([]byte(`{"url": "http://x/?a=1&b=<2>"}`))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := n.Marshal()
	if want := "{\n  \"url\": \"http://x/?a=1&b=<2>\"\n}\n"; string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestSetSplicesAnObjectValueAndAMemberAfterATrailingComma(t *testing.T) {
	t.Parallel()
	obj := Node{Kind: Object, Obj: []Pair{
		{Key: "k", Value: Node{Kind: Number, Num: "2"}},
		{Key: "s", Value: Node{Kind: String, Str: "x"}},
	}}
	for _, tc := range []struct {
		in, want string
		path     []string
		v        Node
	}{
		{`{"a":1}`, `{"a":{"k": 2, "s": "x"}}`, []string{"a"}, obj},
		{`{"a":1,}`, `{"a":1, "b": {"k": 2, "s": "x"}}`, []string{"b"}, obj},
		{"{\n  \"a\": 1,\n}\n", "{\n  \"a\": 1,\n  \"b\": {\"k\": 2, \"s\": \"x\"}\n}\n", []string{"b"}, obj},
		{`{"a\"b":{"c":1}}`, `{"a\"b":{"c":2}}`, []string{`a"b`, "c"}, Node{Kind: Number, Num: "2"}},
		{"{\n  /* keep */ \"a\": 1\n}\n", "{\n  /* keep */ \"a\": 1,\n  \"b\": true\n}\n", []string{"b"}, Node{Kind: Bool, Bool: true}},
	} {
		out, err := Set(tc.in, tc.path, tc.v)
		if err != nil || out != tc.want {
			t.Errorf("%q: got %q (%v) want %q", tc.in, out, err, tc.want)
		}
	}
	in := `{"x":{"a":1}}`
	out, err := Set(in, []string{"x"}, Node{Kind: Object, Obj: []Pair{{Key: "b", Value: Node{Kind: Number, Num: "1"}}}})
	if err != nil || out == in {
		t.Fatalf("same-shaped object with different keys: %q %v", out, err)
	}
	parsed, err := Parse([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	out, err = Set(in, nil, parsed)
	if err != nil || out != in {
		t.Fatalf("empty path same value: %q %v", out, err)
	}
	out, err = Set(in, nil, Node{Kind: Object})
	if err != nil || out != "{}\n" {
		t.Fatalf("empty path empty object: %q %v", out, err)
	}
}

func TestSetLeavesUnparseableAndInvalidUTF8FilesUntouched(t *testing.T) {
	t.Parallel()
	in := `{"a":1}`
	if out, err := Set(`[`, []string{"a"}, Node{Kind: Null}); err == nil || out != `[` {
		t.Errorf("unparseable: %q %v", out, err)
	}
	if out, err := Set(in, []string{"a"}, Node{Kind: String, Str: "\xff"}); err == nil || out != in {
		t.Errorf("invalid UTF-8 value: %q %v", out, err)
	}
	if out, err := Set(in, []string{"\xff"}, Node{Kind: Number, Num: "1"}); err == nil || out != in {
		t.Errorf("invalid UTF-8 key: %q %v", out, err)
	}
	if out, err := Set(in, []string{"a"}, Node{Kind: Array, Arr: []Node{{Kind: String, Str: "\xff"}}}); err == nil || out != in {
		t.Errorf("invalid UTF-8 array item: %q %v", out, err)
	}
	if out, err := Set(in, []string{"a"}, Node{Kind: Object, Obj: []Pair{{Key: "\xff", Value: Node{Kind: Null}}}}); err == nil || out != in {
		t.Errorf("invalid UTF-8 object key: %q %v", out, err)
	}
	if out, err := Set(in, []string{"a"}, Node{Kind: Object, Obj: []Pair{{Key: "k", Value: Node{Kind: String, Str: "\xff"}}}}); err == nil || out != in {
		t.Errorf("invalid UTF-8 object value: %q %v", out, err)
	}
	deep := Node{Kind: Number, Num: "1"}
	for range MaxDepth + 1 {
		deep = Node{Kind: Array, Arr: []Node{deep}}
	}
	if out, err := Set(`1`, nil, deep); !errors.Is(err, ErrTooDeep) || out != `1` {
		t.Errorf("too-deep empty path: %q %v", out, err)
	}
	for _, raw := range []string{`{]`, `[}`, `[,]}`} {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Errorf("%q parsed", raw)
		}
	}
}

func TestEmptyObjectMarshalsToBracesAndUnknownKindHasNoText(t *testing.T) {
	t.Parallel()
	got, err := (Node{Kind: Object}).Marshal()
	if err != nil || string(got) != "{}\n" {
		t.Fatalf("empty object Marshal: %q %v", got, err)
	}
	if text := (Node{Kind: Object}).Text(); text != "{}" {
		t.Fatalf("empty object Text: %q", text)
	}
	if _, err := (Node{Kind: Object, Obj: []Pair{{Key: "\xff", Value: Node{Kind: Null}}}}).Marshal(); err == nil {
		t.Fatal("invalid UTF-8 key marshaled")
	}
	if (Node{Kind: 99}).Text() != "" {
		t.Fatal("unknown kind had text")
	}
	if _, err := (Node{Kind: Object, Obj: []Pair{{Key: "k", Value: Node{Kind: String, Str: "\xff"}}}}).Marshal(); err == nil {
		t.Fatal("invalid UTF-8 value marshaled")
	}
	deep := Node{Kind: Number, Num: "1"}
	for range MaxDepth + 1 {
		deep = Node{Kind: Object, Obj: []Pair{{Key: "a", Value: deep}}}
	}
	if _, err := deep.Marshal(); !errors.Is(err, ErrTooDeep) {
		t.Fatalf("too-deep object Marshal: %v", err)
	}
}
