package jsonfile

import (
	"errors"
	"strings"
	"testing"
)

// TestRefusesNestingPastMaxDepth holds that a small file cannot amplify into a huge output: each indented level writes
// its ancestors' indentation on its own line, so 10,000 nested arrays in a 20 KB file wrote 200 MB.
func TestRefusesNestingPastMaxDepth(t *testing.T) {
	t.Parallel()
	arrays := func(depth int) []byte {
		return []byte(strings.Repeat("[", depth) + strings.Repeat("]", depth))
	}
	objects := func(depth int) []byte {
		return []byte(strings.Repeat(`{"a":`, depth) + "1" + strings.Repeat("}", depth))
	}
	for _, tc := range []struct {
		name string
		in   []byte
	}{
		{"arrays", arrays(MaxDepth + 1)},
		{"objects", objects(MaxDepth + 1)},
		{"far past", arrays(100 * MaxDepth)},
	} {
		if _, err := Parse(tc.in); !errors.Is(err, ErrTooDeep) {
			t.Errorf("%s at depth %d: %v, want ErrTooDeep", tc.name, MaxDepth+1, err)
		}
	}
	for _, tc := range []struct {
		name string
		in   []byte
		max  int
	}{
		{"arrays", arrays(MaxDepth), 4 * MaxDepth * MaxDepth},
		{"objects", objects(MaxDepth), 16 * MaxDepth * MaxDepth},
	} {
		n, err := Parse(tc.in)
		if err != nil {
			t.Fatalf("%s at depth %d: %v", tc.name, MaxDepth, err)
		}
		out, err := n.Marshal()
		if err != nil {
			t.Fatalf("%s at depth %d: %v", tc.name, MaxDepth, err)
		}
		if len(out) > tc.max {
			t.Errorf("%s at depth %d: Marshal wrote %d bytes, more than %d", tc.name, MaxDepth, len(out), tc.max)
		}
		if text := n.Text(); len(text) > tc.max {
			t.Errorf("%s at depth %d: Text wrote %d bytes, more than %d", tc.name, MaxDepth, len(text), tc.max)
		}
	}
}

// TestMarshalAndTextRefuseDeepNodes holds that a Node built by hand cannot write far more than a file of the same shape
// would, since the depth guard is in the writers as well as the reader.
func TestMarshalAndTextRefuseDeepNodes(t *testing.T) {
	t.Parallel()
	n := Node{Kind: Number, Num: "1"}
	for range MaxDepth + 1 {
		n = Node{Kind: Array, Arr: []Node{n}}
	}
	if _, err := n.Marshal(); !errors.Is(err, ErrTooDeep) {
		t.Errorf("Marshal: %v, want ErrTooDeep", err)
	}
	if text := n.Text(); text != "" {
		t.Errorf("Text wrote %d bytes of a value past MaxDepth, want none", len(text))
	}
}
