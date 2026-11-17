package jsonfile

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestRefusesNestingPastMaxDepth holds that a small file cannot amplify into a huge output: each indented line carries
// its ancestors' indentation, so output grows with the square of the nesting.
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

// TestRefusesIndentAmplification holds that the 1000-deep document whose every element line carries 2000 bytes of
// indentation is refused rather than written out as megabytes.
func TestRefusesIndentAmplification(t *testing.T) {
	t.Parallel()
	in := []byte(strings.Repeat("[", 1000) + "1," + strings.Repeat("1,", 3000) + "1" + strings.Repeat("]", 1000))
	if _, err := Parse(in); !errors.Is(err, ErrTooDeep) {
		t.Errorf("Parse of %d bytes: %v, want ErrTooDeep", len(in), err)
	}
}

// TestMaxDepthRoundTrip holds that a document nesting exactly MaxDepth levels is still read and written unchanged.
func TestMaxDepthRoundTrip(t *testing.T) {
	t.Parallel()
	in := []byte(strings.Repeat(`{"a":`, MaxDepth) + "1" + strings.Repeat("}", MaxDepth))
	n, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := n.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	again, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse of Marshal output: %v", err)
	}
	if !reflect.DeepEqual(again, n) {
		t.Error("Marshal then Parse changed the document")
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
