package cfgedit

import (
	"strings"
	"testing"
	"time"
)

// A file of unclosed arrays must parse in linear time, not rescan to the end from every line.
func TestParseMelonUnclosedArraysLinear(t *testing.T) {
	text := strings.Repeat("k=[\n", 40000)
	start := time.Now()
	ParseMelon(text)
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("ParseMelon took %v on 160 KB of unclosed arrays", d)
	}
}

func TestParseMelonArrayStillCloses(t *testing.T) {
	m := ParseMelon("a=[\n1,\n2]\nb=[\n[3],\n]\nc=[1\n")
	if got := len(m.Entries()); got != 3 {
		t.Fatalf("entries = %d, want 3", got)
	}
}
