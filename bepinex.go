package cfgedit

import (
	"math"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Rethunk-Tech/go-cfgedit/internal/textdoc"
)

// The layout parsed here is what BepInEx's ConfigFile.Save and ConfigEntryBase.WriteDescription write
// (BepInEx.Core/Configuration/ConfigFile.cs and ConfigEntryBase.cs, BepInEx 5.4.x/6):
//
//	## Settings file was created by plugin <Name> v<Version>
//	## Plugin GUID: <GUID>
//
//	[Section]
//
//	## <description, one "## " line per line of text>
//	# Setting type: <.NET type name>
//	# Default value: <value>
//	# Acceptable values: a, b, c               (AcceptableValueList, or the names of an enum)
//	# Acceptable value range: From x to y      (AcceptableValueRange)
//	# Multiple values can be set at the same time by separating them with , (e.g. Debug, Warning)   (a [Flags] enum)
//	Key = value
//
// The reader skips lines that start with '#', and splits "Key = value" at the first '='.

var (
	settingType = regexp.MustCompile(`^# Setting type: (.+)$`)
	defaultVal  = regexp.MustCompile(`^# Default value: ?(.*)$`)
	acceptList  = regexp.MustCompile(`^# Acceptable values: (.*)$`)
	acceptRange = regexp.MustCompile(`^# Acceptable value range: From (.+) to (.+)$`)
	flagsLine   = "# Multiple values can be set at the same time by separating them with ,"
	sectionLine = regexp.MustCompile(`^\[(.+)\]$`)
	pluginName  = regexp.MustCompile(`^## Settings file was created by plugin (.+) v\S+$`)
	pluginGUID  = regexp.MustCompile(`^## Plugin GUID: (.+)$`)
)

// netLimits bound the .NET number types narrower than what validation parses; BepInEx replaces a value that overflows
// its type with the default when the game starts.
var netLimits = map[string][2]float64{
	"Byte": {0, math.MaxUint8}, "SByte": {math.MinInt8, math.MaxInt8},
	"Int16": {math.MinInt16, math.MaxInt16}, "UInt16": {0, math.MaxUint16},
	"Int32": {math.MinInt32, math.MaxInt32}, "UInt32": {0, math.MaxUint32}, "UInt64": {0, math.MaxUint64},
	"Single": {-math.MaxFloat32, math.MaxFloat32},
}

// decimalMax is .NET's decimal.MaxValue, which a float64 cannot hold exactly; decimalBound is it for the refusal's detail.
var (
	decimalMax, _ = new(big.Rat).SetString("79228162514264337593543950335")
	decimalBound  = [2]float64{-79228162514264337593543950335, 79228162514264337593543950335}
)

// hexColor is what Unity's ColorUtility.TryParseHtmlString reads after BepInEx adds the "#": RGB, RGBA, RRGGBB or RRGGBBAA.
var hexColor = regexp.MustCompile(`^#?([0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

var (
	unsignedTypes = []string{"Byte", "UInt16", "UInt32", "UInt64"}
	// plainDecimal is what .NET's decimal.Parse reads: digits with an optional point, no exponent.
	plainDecimal = regexp.MustCompile(`^[+-]?(\d+(\.\d*)?|\.\d+)$`)
)

// BepInEx is a parsed BepInEx .cfg. It keeps the original lines, so a Set changes only the edited line.
type BepInEx struct {
	doc          textdoc.Doc
	plugin, guid string
	entries      []bepEntry
}

type bepEntry struct {
	Entry
	line int
	// netType is the .NET type BepInEx parses the value into.
	netType string
}

// ParseBepInEx reads a BepInEx .cfg. It never fails: a line it does not understand is skipped.
func ParseBepInEx(text string) *BepInEx {
	d := &BepInEx{doc: textdoc.Split(text)}
	var section string
	var pending bepEntry
	var desc []string
	reset := func() { pending, desc = bepEntry{}, nil }
	for i, raw := range d.doc.Lines {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			reset()
		case pluginName.MatchString(line):
			d.plugin = pluginName.FindStringSubmatch(line)[1]
		case pluginGUID.MatchString(line):
			d.guid = pluginGUID.FindStringSubmatch(line)[1]
		case strings.HasPrefix(line, "## "):
			desc = append(desc, strings.TrimPrefix(line, "## "))
		case strings.HasPrefix(line, "##"):
			desc = append(desc, strings.TrimPrefix(line, "##"))
		case strings.HasPrefix(line, "#"):
			readMeta(&pending, line)
		case sectionLine.MatchString(line):
			section = sectionLine.FindStringSubmatch(line)[1]
			reset()
		default:
			key, val, ok := strings.Cut(line, "=")
			if !ok {
				reset()
				continue
			}
			e := pending
			e.Section, e.Key, e.Value = section, strings.TrimSpace(key), strings.TrimSpace(val)
			e.Description = strings.Join(desc, "\n")
			e.line = i
			finishEntry(&e)
			d.entries = append(d.entries, e)
			reset()
		}
	}
	return d
}

// readMeta reads one "# ..." metadata line; the .NET type name waits in netType until finishEntry.
func readMeta(e *bepEntry, line string) {
	switch {
	case settingType.MatchString(line):
		e.netType = settingType.FindStringSubmatch(line)[1]
	case defaultVal.MatchString(line):
		e.Default, e.HasDefault = defaultVal.FindStringSubmatch(line)[1], true
	case acceptList.MatchString(line):
		for v := range strings.SplitSeq(acceptList.FindStringSubmatch(line)[1], ", ") {
			e.Values = append(e.Values, strings.TrimSpace(v))
		}
	case acceptRange.MatchString(line):
		m := acceptRange.FindStringSubmatch(line)
		if lo, err := strconv.ParseFloat(strings.TrimSpace(m[1]), 64); err == nil {
			e.Min = &lo
		}
		if hi, err := strconv.ParseFloat(strings.TrimSpace(m[2]), 64); err == nil {
			e.Max = &hi
		}
	case strings.HasPrefix(line, flagsLine):
		e.Flags = true
	}
}

// finishEntry turns the .NET type name into one of the entry types.
func finishEntry(e *bepEntry) {
	switch name := e.netType; {
	case len(e.Values) > 0:
		e.Type = TypeEnum
	case name == "Boolean":
		e.Type = TypeBool
	case slices.Contains([]string{"Int32", "Int64", "Int16", "UInt32", "UInt64", "UInt16", "Byte", "SByte"}, name):
		e.Type = TypeInt
	case slices.Contains([]string{"Single", "Double", "Decimal"}, name):
		e.Type = TypeFloat
	case name == "Color":
		e.Type = TypeColor
	case strings.HasSuffix(name, "[]"):
		e.Type = TypeList
	default:
		e.Type = TypeString
	}
}

// Plugin is the plugin name from the "Settings file was created by plugin <Name> v<Version>" header, or "".
func (d *BepInEx) Plugin() string { return d.plugin }

// GUID is the plugin GUID from the "Plugin GUID: <GUID>" header, or "".
func (d *BepInEx) GUID() string { return d.guid }

// Entries lists the settings in file order.
func (d *BepInEx) Entries() []Entry {
	out := make([]Entry, len(d.entries))
	for i, e := range d.entries {
		out[i] = e.Entry
	}
	return out
}

func (d *BepInEx) find(section, key string) (bepEntry, bool) {
	for _, e := range d.entries {
		if e.Section == section && e.Key == key {
			return e, true
		}
	}
	return bepEntry{}, false
}

// Set returns the file text with one value replaced and every other byte as it was, or an *ValueError when BepInEx
// would not read the value back as typed, or an error wrapping ErrNoSetting.
func (d *BepInEx) Set(section, key, value string) (string, error) {
	e, ok := d.find(section, key)
	if !ok {
		return "", noSetting(section, key)
	}
	val, err := e.validate(value)
	if err != nil {
		return "", err
	}
	if err := e.checkNetType(val); err != nil {
		return "", err
	}
	nd := d.doc
	nd.Lines = slices.Clone(d.doc.Lines)
	old := nd.Lines[e.line]
	eq := strings.Index(old, "=")
	nd.Lines[e.line] = strings.TrimRight(old[:eq+1], " ") + " " + val
	return nd.Join(), nil
}

// checkNetType refuses a value outside what the .NET type holds.
func (e bepEntry) checkNetType(val string) error {
	if e.netType == "Decimal" {
		if r, ok := new(big.Rat).SetString(val); ok && r.Abs(r).Cmp(decimalMax) > 0 {
			return refuseRange(e.Key, &decimalBound[0], &decimalBound[1])
		}
	}
	if lim, ok := netLimits[e.netType]; ok {
		if n, _ := strconv.ParseFloat(val, 64); n < lim[0] || n > lim[1] {
			return refuseRange(e.Key, &lim[0], &lim[1])
		}
	}
	return nil
}

// validate checks value against the entry's type, range and acceptable values and returns the text to write.
func (e bepEntry) validate(value string) (string, error) {
	if strings.ContainsAny(value, "\r\n") {
		return "", refuse(e.Key, MultiLine)
	}
	value = strings.TrimSpace(value)
	switch e.Type {
	case TypeBool:
		return e.validateBool(value)
	case TypeInt:
		return e.validateInt(value)
	case TypeFloat:
		return e.validateFloat(value)
	case TypeColor:
		if !hexColor.MatchString(value) {
			return "", refuse(e.Key, NotColor)
		}
	case TypeEnum:
		return e.validateEnum(value)
	case TypeString, TypeList, TypeRaw:
	}
	return value, nil
}

func (e bepEntry) validateBool(value string) (string, error) {
	switch v := strings.ToLower(value); v {
	case "true", "false":
		return v, nil
	}
	return "", refuse(e.Key, NotBool)
}

func (e bepEntry) validateInt(value string) (string, error) {
	var n float64
	if slices.Contains(unsignedTypes, e.netType) {
		u, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return "", refuse(e.Key, NotInteger)
		}
		n = float64(u)
	} else {
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return "", refuse(e.Key, NotInteger)
		}
		n = float64(i)
	}
	return value, e.inRange(n)
}

func (e bepEntry) validateFloat(value string) (string, error) {
	n, text, err := finite(e.Key, value)
	if err != nil {
		return "", err
	}
	if e.netType == "Decimal" {
		// decimal.Parse takes no exponent: keep plain digits as typed, else spell the number out.
		text = strconv.FormatFloat(n, 'f', -1, 64)
		if plainDecimal.MatchString(value) {
			text = value
		}
	}
	return text, e.inRange(n)
}

func (e bepEntry) validateEnum(value string) (string, error) {
	parts := []string{value}
	if e.Flags {
		parts = strings.Split(value, ",")
	}
	// Enum.Parse ignores case; the file gets the member's own spelling.
	for i, p := range parts {
		trimmed := strings.TrimSpace(p)
		j := slices.IndexFunc(e.Values, func(v string) bool { return strings.EqualFold(v, trimmed) })
		if j < 0 {
			return "", &ValueError{Key: e.Key, Problem: NotAllowed, Values: e.Values}
		}
		parts[i] = strings.Replace(p, trimmed, e.Values[j], 1)
	}
	return strings.Join(parts, ","), nil
}

func (e bepEntry) inRange(n float64) error {
	if (e.Min != nil && n < *e.Min) || (e.Max != nil && n > *e.Max) {
		return refuseRange(e.Key, e.Min, e.Max)
	}
	return nil
}
