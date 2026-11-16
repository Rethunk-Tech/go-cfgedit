package ini

import "testing"

func TestSetDuplicateKeyLast(t *testing.T) {
	in := "[S]\nkey = first\nkey = second\n"
	out, err := Set(in, "S", "key", "third")
	if err != nil {
		t.Fatal(err)
	}
	if out != "[S]\nkey = first\nkey = third\n" {
		t.Fatalf("Set last duplicate: got %q", out)
	}
}
