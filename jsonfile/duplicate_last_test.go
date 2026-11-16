package jsonfile

import (
	"strings"
	"testing"
)

func TestDuplicateKeyLast(t *testing.T) {
	n, err := Parse([]byte(`{"a":1,"a":2}`))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := n.Lookup([]string{"a"})
	if !ok || got.Text() != "2" {
		t.Fatalf("Lookup last: ok=%v got=%q", ok, got.Text())
	}
	n, err = Set(n, []string{"a"}, Node{Kind: Number, Num: "3"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := n.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"a": 1`) || !strings.Contains(s, `"a": 3`) || strings.Contains(s, `"a": 2`) {
		t.Fatalf("Set last duplicate: got %s", s)
	}
}
