package jsonfile

import "testing"

func TestDuplicateKeyLast(t *testing.T) {
	n, err := Parse([]byte(`{"a":1,"a":2}`))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := n.Lookup([]string{"a"})
	if !ok || got.Text() != "2" {
		t.Fatalf("Lookup last: ok=%v got=%q", ok, got.Text())
	}
	out, err := Set(`{"a":1,"a":2}`, []string{"a"}, Node{Kind: Number, Num: "3"})
	if err != nil {
		t.Fatal(err)
	}
	if out != `{"a":1,"a":3}` {
		t.Fatalf("Set last duplicate: got %s", out)
	}
}
