package cfgedit

import (
	"os"
	"testing"
)

func seedCfg(f *testing.F) {
	if b, err := os.ReadFile("testdata/bepinex.cfg"); err == nil {
		f.Add(string(b))
	}
	f.Add("[General]\n## Setting type: Boolean\n# Default value: false\nEnabled = true\n")
	f.Add("\r0=")
}

func FuzzParseBepInEx(f *testing.F) {
	seedCfg(f)
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 1<<16 {
			t.Skip()
		}
		d := ParseBepInEx(text)
		es := d.Entries()
		if len(es) > len(text)+1 {
			t.Fatalf("entries=%d len=%d", len(es), len(text))
		}
	})
}

func FuzzBepInExSet(f *testing.F) {
	seedCfg(f)
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 1<<16 {
			t.Skip()
		}
		d := ParseBepInEx(text)
		for _, e := range d.Entries() {
			out, err := d.Set(e.Section, e.Key, e.Value)
			if err != nil {
				continue
			}
			if len(out) > len(text)+len(e.Value)+64 {
				t.Fatalf("unbounded write: in=%d out=%d", len(text), len(out))
			}
		}
	})
}

func FuzzParseMelon(f *testing.F) {
	f.Add(`[MelonLoader]
# Mod load order
load_order = "default" # inline

[BetterHands]
# Turns the feature on
Enabled = true
# Multiplier
Speed = 2.5
Count = 4
Names = ["a", "b"]
Label = "Hello # not a comment"
`)
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 1<<16 {
			t.Skip()
		}
		d := ParseMelon(text)
		es := d.Entries()
		if len(es) > len(text)+1 {
			t.Fatalf("entries=%d len=%d", len(es), len(text))
		}
		for _, e := range es {
			out, err := d.Set(e.Section, e.Key, e.Value)
			if err != nil {
				continue
			}
			// Melon re-encodes as TOML, so control bytes become \uXXXX.
			if len(out) > 8*len(text)+256 {
				t.Fatalf("unbounded write: in=%d out=%d", len(text), len(out))
			}
		}
	})
}
