package cfgedit

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Type is how an entry's value is written and validated.
type Type string

// The entry types. TypeRaw is a Melon value the parser cannot type.
const (
	TypeBool   Type = "bool"
	TypeInt    Type = "int"
	TypeFloat  Type = "float"
	TypeEnum   Type = "enum"
	TypeString Type = "string"
	TypeColor  Type = "color"
	TypeList   Type = "list"
	// TypeRaw is a Melon value the parser cannot type (an inline table, a date, a hex integer): its TOML text is
	// shown and written as typed, never quoted.
	TypeRaw Type = "raw"
)

// Entry is one setting, in file order.
type Entry struct {
	Section, Key string
	Type         Type
	// Value is the BepInEx value as the file holds it, escapes included; a Melon string is unquoted and unescaped.
	Value       string
	Default     string // BepInEx only.
	HasDefault  bool
	Description string
	Min, Max    *float64 // An acceptable range the file states.
	Values      []string // The acceptable values of an enum.
	Flags       bool     // An enum whose values combine with commas.
}

// ErrNoSetting is wrapped by Set when the section has no such key. The wrapped text is "no setting <key> in [<section>]".
var ErrNoSetting = errors.New("no setting")

func noSetting(section, key string) error {
	return fmt.Errorf("%w %s in [%s]", ErrNoSetting, key, section)
}

// Problem is why a value was refused. Its string is stable and safe to put on a wire.
type Problem string

// The reasons Set refuses a value.
const (
	MultiLine  Problem = "line"         // A value holding a line break.
	NotBool    Problem = "bool"         // Neither true nor false.
	NotInteger Problem = "whole_number" // Also a TOML integer with a leading zero.
	NotNumber  Problem = "number"       // Also NaN and infinities.
	NotAllowed Problem = "enum"         // Not one of Values.
	OutOfRange Problem = "range"        // The file's range, the .NET type's, or Decimal's exact bound.
	NotColor   Problem = "color"        // Not 3, 4, 6 or 8 hex digits.
	NotList    Problem = "list"         // A TOML array without its brackets.
	Empty      Problem = "empty"        // A TypeRaw value left blank.
	BreaksTOML Problem = "toml"         // Melon: the edit would leave a file that parsed unparsable.
)

// ValueError is a value Set refuses. Error is a plain English sentence naming the key, for logs and CLIs; callers
// word their own sentence for people from Problem and the bounds.
type ValueError struct {
	Key     string
	Problem Problem
	// Min and Max are the bound broken for OutOfRange, from the file or the .NET type; either may be nil.
	Min, Max *float64
	Values   []string // The acceptable values for NotAllowed.
}

func (e *ValueError) Error() string {
	switch e.Problem {
	case MultiLine:
		return e.Key + ": a value is one line"
	case NotBool:
		return e.Key + " expects true or false"
	case NotInteger:
		return e.Key + " expects a whole number"
	case NotNumber:
		return e.Key + " expects a number"
	case NotAllowed:
		return e.Key + " must be one of " + strings.Join(e.Values, ", ")
	case OutOfRange:
		return e.rangeSentence()
	case NotColor:
		return e.Key + " expects a hex color such as FF8800 or FF8800CC"
	case NotList:
		return e.Key + " expects a list in [brackets]"
	case Empty:
		return e.Key + " expects a value"
	case BreaksTOML:
		return e.Key + ": that is not a TOML value"
	}
	return e.Key + ": refused (" + string(e.Problem) + ")"
}

func (e *ValueError) rangeSentence() string {
	f := func(p *float64) string { return strconv.FormatFloat(*p, 'f', -1, 64) }
	switch {
	case e.Min == nil && e.Max == nil:
		return e.Key + " is out of range"
	case e.Min == nil:
		return e.Key + " must be at most " + f(e.Max)
	case e.Max == nil:
		return e.Key + " must be at least " + f(e.Min)
	}
	return e.Key + " must be between " + f(e.Min) + " and " + f(e.Max)
}

func refuse(key string, p Problem) *ValueError { return &ValueError{Key: key, Problem: p} }

func refuseRange(key string, lo, hi *float64) *ValueError {
	return &ValueError{Key: key, Problem: OutOfRange, Min: lo, Max: hi}
}

// finite parses a number and writes it the way both .NET and TOML read it: Go also takes "1_000", hex and "Inf", which
// neither does.
func finite(key, value string) (float64, string, error) {
	n, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
		return 0, "", refuse(key, NotNumber)
	}
	return n, strconv.FormatFloat(n, 'g', -1, 64), nil
}
