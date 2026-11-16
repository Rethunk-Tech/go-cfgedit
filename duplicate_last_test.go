package cfgedit

import "testing"

func TestBepInExDuplicateKeyLast(t *testing.T) {
	const in = "[Cat]\nK = 1\nK = 2\n"
	d := ParseBepInEx(in)
	var last string
	n := 0
	for _, e := range d.Entries() {
		if e.Section == "Cat" && e.Key == "K" {
			last = e.Value
			n++
		}
	}
	if n != 2 || last != "2" {
		t.Fatalf("last value: n=%d last=%q", n, last)
	}
	out, err := d.Set("Cat", "K", "3")
	if err != nil {
		t.Fatal(err)
	}
	if out != "[Cat]\nK = 1\nK = 3\n" {
		t.Fatalf("Set last duplicate: got %q", out)
	}
}

func TestMelonDuplicateKeyLast(t *testing.T) {
	const in = "[Cat]\nK = 1\nK = 2\n"
	d := ParseMelon(in)
	var last string
	n := 0
	for _, e := range d.Entries() {
		if e.Section == "Cat" && e.Key == "K" {
			last = e.Value
			n++
		}
	}
	if n != 2 || last != "2" {
		t.Fatalf("last value: n=%d last=%q", n, last)
	}
	out, err := d.Set("Cat", "K", "3")
	if err != nil {
		t.Fatal(err)
	}
	if out != "[Cat]\nK = 1\nK = 3\n" {
		t.Fatalf("Set last duplicate: got %q", out)
	}
}
