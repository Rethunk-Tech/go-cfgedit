package cfgedit

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/bepinex.cfg")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func entryOf(t *testing.T, d *BepInEx, section, key string) Entry {
	t.Helper()
	for _, e := range d.Entries() {
		if e.Section == section && e.Key == key {
			return e
		}
	}
	t.Fatalf("no %s.%s", section, key)
	return Entry{}
}

func TestParseReadsTypesRangesAndDescriptions(t *testing.T) {
	d := ParseBepInEx(fixture(t))
	if d.Plugin() != "Better Stuff" || d.GUID() != "com.example.betterstuff" {
		t.Fatalf("header %q %q", d.Plugin(), d.GUID())
	}
	cases := []struct {
		section, key string
		typ          Type
		def, value   string
	}{
		{"General", "ShowHud", TypeBool, "true", "true"},
		{"General", "LeaveDelay", TypeInt, "90", "120"},
		{"General", "ScrapMultiplier", TypeFloat, "1", "1.25"},
		{"Advanced", "Mode", TypeEnum, "Normal", "Hard"},
		{"Advanced", "Label", TypeString, "Crew", "Night Shift"},
		{"Advanced", "Tint", TypeColor, "FFFFFFFF", "FF8800FF"},
	}
	for _, c := range cases {
		e := entryOf(t, d, c.section, c.key)
		if e.Type != c.typ || e.Default != c.def || e.Value != c.value || !e.HasDefault {
			t.Fatalf("%s.%s = %+v", c.section, c.key, e)
		}
	}
	delay := entryOf(t, d, "General", "LeaveDelay")
	if delay.Min == nil || *delay.Min != 10 || *delay.Max != 300 {
		t.Fatalf("range %+v", delay)
	}
	if delay.Description != "How many seconds before the ship leaves.\nLonger waits are safer." {
		t.Fatalf("description %q", delay.Description)
	}
	if mode := entryOf(t, d, "Advanced", "Mode"); strings.Join(mode.Values, ",") != "Easy,Normal,Hard" {
		t.Fatalf("values %v", mode.Values)
	}
}

func TestSetChangesOnlyTheEditedLine(t *testing.T) {
	text := fixture(t)
	d := ParseBepInEx(text)
	got, err := d.Set("General", "LeaveDelay", "150")
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Replace(text, "LeaveDelay = 120", "LeaveDelay = 150", 1); got != want {
		t.Fatalf("a write must keep comments and order:\n%s", got)
	}
	for _, bad := range [][3]string{{"General", "LeaveDelay", "5"}, {"General", "ShowHud", "maybe"}, {"Advanced", "Mode", "Nightmare"}, {"General", "LeaveDelay", "1.5"}} {
		if _, err := d.Set(bad[0], bad[1], bad[2]); err == nil {
			t.Fatalf("%v must be refused", bad)
		}
	}
	_, err = d.Set("General", "Gone", "1")
	if !errors.Is(err, ErrNoSetting) || err.Error() != "no setting Gone in [General]" {
		t.Fatalf("a missing setting: %v", err)
	}
}

func TestSetKeepsBOMAndCRLF(t *testing.T) {
	text := "\xef\xbb\xbf" + strings.ReplaceAll(fixture(t), "\n", "\r\n")
	got, err := ParseBepInEx(text).Set("General", "LeaveDelay", "150")
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Replace(text, "LeaveDelay = 120", "LeaveDelay = 150", 1); got != want {
		t.Fatalf("BOM and CRLF must survive a write:\n%q", got)
	}
}

// BepInEx parses a value into its .NET type and drops one that does not fit for the default when the game starts.
func TestSetRefusesWhatTheDotNetTypeCannotHold(t *testing.T) {
	d := ParseBepInEx("[A]\n# Setting type: Int32\nN = 1\n# Setting type: Byte\nB = 1\n# Setting type: Single\nF = 1\n")
	for _, bad := range [][2]string{{"N", "3000000000"}, {"B", "256"}, {"B", "-1"}, {"F", "Inf"}, {"F", "1e39"}} {
		if out, err := d.Set("A", bad[0], bad[1]); err == nil {
			t.Errorf("%v accepted: %q", bad, out)
		}
	}
	if out, err := d.Set("A", "B", "255"); err != nil || !strings.Contains(out, "B = 255\n") {
		t.Fatalf("%q %v", out, err)
	}
	// Go reads "1_000" and hex floats, .NET does not: the value is written in a form both read.
	if out, err := d.Set("A", "F", "1_000"); err != nil || !strings.Contains(out, "F = 1000\n") {
		t.Fatalf("%q %v", out, err)
	}
}

func TestSetKeepsEachLinesOwnEnding(t *testing.T) {
	const text = "[A]\r\n# Setting type: Int32\nK = 1\nL = 2\r\n# Setting type: Int32\nM = 3"
	got, err := ParseBepInEx(text).Set("A", "L", "5")
	if want := strings.Replace(text, "L = 2", "L = 5", 1); err != nil || got != want {
		t.Fatalf("%q %v", got, err)
	}
}

func TestSetWritesDecimalsPlainAndAcceptsWhatDotNetReads(t *testing.T) {
	d := ParseBepInEx("[A]\n# Setting type: Decimal\nD = 1\n# Setting type: UInt64\nU = 1\n# Setting type: Int32\n# Acceptable values: Easy, Hard\nMode = Easy\n# Setting type: Mode\n# Acceptable values: Easy, Hard, Wild\n# Multiple values can be set at the same time by separating them with ,\nM = Easy\n")
	for in, want := range map[string]string{"0.00001": "D = 0.00001\n", "1e-5": "D = 0.00001\n", "123456789012345678901234": "D = 123456789012345678901234\n"} {
		if out, err := d.Set("A", "D", in); err != nil || !strings.Contains(out, want) {
			t.Errorf("%s: %q %v", in, out, err)
		}
	}
	if out, err := d.Set("A", "U", "18446744073709551615"); err != nil || !strings.Contains(out, "U = 18446744073709551615\n") {
		t.Errorf("uint64: %q %v", out, err)
	}
	if _, err := d.Set("A", "U", "-1"); err == nil {
		t.Error("negative UInt64 accepted")
	}
	if out, err := d.Set("A", "Mode", "hARD"); err != nil || !strings.Contains(out, "Mode = Hard\n") {
		t.Errorf("enum case: %q %v", out, err)
	}
	if out, err := d.Set("A", "M", "easy,WILD"); err != nil || !strings.Contains(out, "M = Easy,Wild\n") {
		t.Errorf("flags case: %q %v", out, err)
	}
}

func TestDecimalAndColorAreCheckedTheWayBepInExParsesThem(t *testing.T) {
	d := ParseBepInEx("[A]\n# Setting type: Decimal\nD = 1\n# Setting type: Color\nC = FF0000FF\n")
	const top = "79228162514264337593543950335"
	if out, err := d.Set("A", "D", top); err != nil || !strings.Contains(out, "D = "+top+"\n") {
		t.Errorf("decimal max: %q %v", out, err)
	}
	for _, v := range []string{"79228162514264337593543950336", "-79228162514264337593543950336", "1e30"} {
		if _, err := d.Set("A", "D", v); err == nil {
			t.Errorf("decimal %s accepted", v)
		}
	}
	for _, v := range []string{"FF8800", "#ff880080", "f80"} {
		if _, err := d.Set("A", "C", v); err != nil {
			t.Errorf("color %s: %v", v, err)
		}
	}
	for _, v := range []string{"zzz", "FF880", "FF88000", ""} {
		if _, err := d.Set("A", "C", v); err == nil {
			t.Errorf("color %q accepted", v)
		}
	}
}

func TestRefusedValueIsTypedAndWordedInEnglish(t *testing.T) {
	d := ParseBepInEx(fixture(t) + "\n[B]\n# Setting type: Single\n# Acceptable value range: From 0.5 to 3\nS = 1\n")
	for _, c := range []struct {
		section, key, value string
		want                Problem
		sentence            string
	}{
		{"General", "LeaveDelay", "5", OutOfRange, "LeaveDelay must be between 10 and 300"},
		{"General", "LeaveDelay", "1.5", NotInteger, "LeaveDelay expects a whole number"},
		{"General", "ShowHud", "maybe", NotBool, "ShowHud expects true or false"},
		{"Advanced", "Mode", "Nightmare", NotAllowed, "Mode must be one of Easy, Normal, Hard"},
		{"General", "ScrapMultiplier", "x", NotNumber, "ScrapMultiplier expects a number"},
		{"Advanced", "Tint", "zz", NotColor, "Tint expects a hex color such as FF8800 or FF8800CC"},
		{"General", "ShowHud", "a\nb", MultiLine, "ShowHud: a value is one line"},
	} {
		_, err := d.Set(c.section, c.key, c.value)
		var ve *ValueError
		if !errors.As(err, &ve) || ve.Problem != c.want || ve.Key != c.key || ve.Error() != c.sentence {
			t.Errorf("%s=%q: %v %+v", c.key, c.value, err, ve)
		}
	}
	_, err := d.Set("General", "LeaveDelay", "5")
	var ve *ValueError
	if !errors.As(err, &ve) || *ve.Min != 10 || *ve.Max != 300 {
		t.Errorf("range bounds: %+v", ve)
	}
	_, err = d.Set("Advanced", "Mode", "x")
	if !errors.As(err, &ve) || strings.Join(ve.Values, ",") != "Easy,Normal,Hard" {
		t.Errorf("enum values: %+v", ve)
	}
	// The .NET type's limit travels in the error, not in Entry.
	d = ParseBepInEx("[A]\n# Setting type: UInt32\nN = 1\n")
	_, err = d.Set("A", "N", "4294967296")
	if !errors.As(err, &ve) || ve.Problem != OutOfRange || *ve.Min != 0 || *ve.Max != 4294967295 || ve.Error() != "N must be between 0 and 4294967295" {
		t.Errorf("net limit: %v %+v", err, ve)
	}
	if e := entryOf(t, d, "A", "N"); e.Min != nil || e.Max != nil {
		t.Errorf("net limit leaked into Entry: %+v", e)
	}
}

func TestValueErrorSentencesForOpenRanges(t *testing.T) {
	lo, hi := 2.0, 9.0
	for _, c := range []struct {
		e    ValueError
		want string
	}{
		{ValueError{Key: "K", Problem: OutOfRange, Min: &lo}, "K must be at least 2"},
		{ValueError{Key: "K", Problem: OutOfRange, Max: &hi}, "K must be at most 9"},
		{ValueError{Key: "K", Problem: Empty}, "K expects a value"},
		{ValueError{Key: "K", Problem: NotList}, "K expects a list in [brackets]"},
		{ValueError{Key: "K", Problem: BreaksTOML}, "K: that is not a TOML value"},
	} {
		if got := c.e.Error(); got != c.want {
			t.Errorf("%q, want %q", got, c.want)
		}
	}
}

func TestBepInExLoneCRIsALineBreak(t *testing.T) {
	d := ParseBepInEx("[S]\na = 1\rb = 2\n")
	if len(d.Entries()) != 2 {
		t.Fatalf("entries = %+v", d.Entries())
	}
	out, err := d.Set("S", "a", "9")
	if err != nil || out != "[S]\na = 9\rb = 2\n" {
		t.Fatalf("Set = %q, %v", out, err)
	}
}
