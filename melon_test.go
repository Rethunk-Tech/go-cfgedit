package cfgedit

import (
	"errors"
	"strings"
	"testing"
)

// The layout is what MelonPreferences_Entry.Save writes through Tomlet: the description as a preceding comment, the
// optional inline comment after the value.
const melonSample = `[MelonLoader]
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
`

func melonByKey(d *Melon, section string) map[string]Entry {
	got := map[string]Entry{}
	for _, e := range d.Entries() {
		if e.Section == section {
			got[e.Key] = e
		}
	}
	return got
}

func TestMelonPreferencesAreTypedByTheirValuesAndEditedInPlace(t *testing.T) {
	d := ParseMelon(melonSample)
	got := melonByKey(d, "BetterHands")
	for key, want := range map[string]Type{"Enabled": TypeBool, "Speed": TypeFloat, "Count": TypeInt, "Names": TypeList, "Label": TypeString} {
		if got[key].Type != want {
			t.Errorf("%s type %q, want %q", key, got[key].Type, want)
		}
	}
	if got["Enabled"].Description != "Turns the feature on" || got["Label"].Value != "Hello # not a comment" {
		t.Errorf("entries %+v", got)
	}
	out, err := d.Set("BetterHands", "Speed", "3")
	if err != nil || !strings.Contains(out, "Speed = 3.0\n") || strings.Count(out, "\n") != strings.Count(melonSample, "\n") {
		t.Fatalf("set float: %q %v", out, err)
	}
	out, err = d.Set("MelonLoader", "load_order", "custom")
	if err != nil || !strings.Contains(out, `load_order = "custom" # inline`+"\n") {
		t.Fatalf("set string keeps the inline comment: %q %v", out, err)
	}
	if _, err := d.Set("BetterHands", "Count", "x"); err == nil {
		t.Fatal("a non-number was accepted for an int")
	}
	if _, err := d.Set("BetterHands", "Gone", "1"); !errors.Is(err, ErrNoSetting) {
		t.Fatalf("a missing setting: %v", err)
	}
}

// Tomlet reads a float only as lowercase TOML ("1e5", "inf"); "1E5", "Infinity" or "0x1p3" fails the whole file.
func TestMelonFloatIsWrittenAsTOML(t *testing.T) {
	d := ParseMelon(melonSample)
	for in, want := range map[string]string{"1E5": "Speed = 100000.0\n", "0x1p3": "Speed = 8.0\n", ".5": "Speed = 0.5\n"} {
		if out, err := d.Set("BetterHands", "Speed", in); err != nil || !strings.Contains(out, want) {
			t.Errorf("%s: %q %v", in, out, err)
		}
	}
	for _, in := range []string{"Infinity", "NaN"} {
		if _, err := d.Set("BetterHands", "Speed", in); err == nil {
			t.Errorf("%s accepted", in)
		}
	}
}

func TestMelonStringIsWrittenWithTOMLEscapes(t *testing.T) {
	d := ParseMelon("[Mod]\nName = \"x\"\n")
	out, err := d.Set("Mod", "Name", "a\a\v\x01é\"\\\tz")
	if err != nil || out != "[Mod]\nName = \"a\\u0007\\u000B\\u0001é\\\"\\\\\\tz\"\n" {
		t.Fatalf("%q %v", out, err)
	}
}

func TestMelonQuotedCategoryIsNamedWithoutItsQuotes(t *testing.T) {
	d := ParseMelon("[\"My Mod\"]\nOn = true\n['Lit.Mod']\nOn = true\n")
	es := d.Entries()
	if es[0].Section != "My Mod" || es[1].Section != "Lit.Mod" {
		t.Fatalf("sections %q, %q", es[0].Section, es[1].Section)
	}
	if out, err := d.Set("My Mod", "On", "false"); err != nil || out != "[\"My Mod\"]\nOn = false\n['Lit.Mod']\nOn = true\n" {
		t.Fatalf("%q %v", out, err)
	}
}

// Tomlet writes a string holding a newline and a backslash or double quote as a multi-line literal, and an array of
// arrays one element per line; an edit replaces the whole value, and no line inside it reads as a setting.
func TestMelonMultiLineValuesAreEditedWhole(t *testing.T) {
	const text = "[Mod]\nNote = '''\nsay \"a=b\"\nC:\\x'''\nGrid = [\n\t[ \"[\", 1, ],\n\t[ 2, ],\n]\nAfter = 1\n"
	d := ParseMelon(text)
	var keys []string
	for _, e := range d.Entries() {
		keys = append(keys, e.Key)
	}
	if strings.Join(keys, ",") != "Note,Grid,After" || d.Entries()[0].Value != "say \"a=b\"\nC:\\x" {
		t.Fatalf("entries %+v", d.Entries())
	}
	out, err := d.Set("Mod", "Note", "short")
	if err != nil || out != "[Mod]\nNote = \"short\"\nGrid = [\n\t[ \"[\", 1, ],\n\t[ 2, ],\n]\nAfter = 1\n" {
		t.Fatalf("set string: %q %v", out, err)
	}
	out, err = d.Set("Mod", "Grid", "[]")
	if err != nil || out != "[Mod]\nNote = '''\nsay \"a=b\"\nC:\\x'''\nGrid = []\nAfter = 1\n" {
		t.Fatalf("set array: %q %v", out, err)
	}
}

func TestMelonSetKeepsEachLinesOwnEnding(t *testing.T) {
	const text = "[M]\r\nA = 1\nB = [\r\n 1,\r\n]\nC = 3"
	d := ParseMelon(text)
	if out, err := d.Set("M", "A", "2"); err != nil || out != "[M]\r\nA = 2\nB = [\r\n 1,\r\n]\nC = 3" {
		t.Fatalf("%q %v", out, err)
	}
	if out, err := d.Set("M", "B", "[]"); err != nil || out != "[M]\r\nA = 1\nB = []\nC = 3" {
		t.Fatalf("%q %v", out, err)
	}
}

// Values the parser cannot type keep their TOML text when edited; integers take every TOML spelling.
func TestMelonRawAndTOMLIntegers(t *testing.T) {
	const text = "[M]\nV = { x = 1.0, y = 2.0 }\nH = 0x1F\nT = 1979-05-27\nN = 1_000\nI = 5\n"
	d := ParseMelon(text)
	got := melonByKey(d, "M")
	for _, k := range []string{"V", "H", "T", "N"} {
		if got[k].Type != TypeRaw {
			t.Errorf("%s type %q", k, got[k].Type)
		}
		if out, err := d.Set("M", k, got[k].Value); err != nil || out != text {
			t.Errorf("%s: untouched value changed: %q %v", k, out, err)
		}
	}
	if out, err := d.Set("M", "V", "{ x = 3.0 }"); err != nil || !strings.Contains(out, "V = { x = 3.0 }\n") {
		t.Errorf("raw edit: %q %v", out, err)
	}
	for _, in := range []string{"1_000", "0x1F", "0o17", "0b101", "-7"} {
		if out, err := d.Set("M", "I", in); err != nil || !strings.Contains(out, "I = "+in+"\n") {
			t.Errorf("int %s: %q %v", in, out, err)
		}
	}
	for _, in := range []string{"1__0", "_1", "0x", "-0x1", "1.5"} {
		if _, err := d.Set("M", "I", in); err == nil {
			t.Errorf("int %s accepted", in)
		}
	}
}

func TestMelonRefusesAValueThatBreaksTheFile(t *testing.T) {
	d := ParseMelon("[A]\nT = 2024-01-02\nL = [1]\nN = 5\nR = 1.5\n")
	for key, value := range map[string]string{"T": "hello world", "L": "[a b]", "N": "007"} {
		if out, err := d.Set("A", key, value); err == nil {
			t.Errorf("%s=%q accepted: %q", key, value, out)
		}
	}
	d = ParseMelon("[A]\nT = { a = 1 }\nR = 1.5\nL = [1]\n")
	_, err := d.Set("A", "T", "{ x = ")
	var ve *ValueError
	if !errors.As(err, &ve) || ve.Problem != BreaksTOML || ve.Error() != "T: that is not a TOML value" {
		t.Errorf("raw: %v", err)
	}
	if _, err := d.Set("A", "T", " "); !errors.As(err, &ve) || ve.Problem != Empty {
		t.Errorf("blank raw: %v", err)
	}
	if _, err := d.Set("A", "L", "1"); !errors.As(err, &ve) || ve.Problem != NotList {
		t.Errorf("list: %v", err)
	}
	if out, err := ParseMelon("[A]\nN = 5\n").Set("A", "N", "0"); err != nil || out != "[A]\nN = 0\n" {
		t.Errorf("zero: %q %v", out, err)
	}
}

// A file that is already unparsable is not made the edit's fault: the edit goes through.
func TestMelonEditsAFileThatWasAlreadyBroken(t *testing.T) {
	if out, err := ParseMelon("[A]\nN = 5\nbad line = = =\n").Set("A", "N", "6"); err != nil || !strings.HasPrefix(out, "[A]\nN = 6\n") {
		t.Errorf("%q %v", out, err)
	}
}
