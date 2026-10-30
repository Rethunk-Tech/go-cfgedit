# HUMANS.md

Run and use `go-cfgedit`. Internals: [AGENTS.md](AGENTS.md).

## Quick start

```bash
go get github.com/Rethunk-Tech/go-cfgedit
```

```go
package main

import (
	"errors"
	"fmt"

	"github.com/Rethunk-Tech/go-cfgedit"
	"github.com/Rethunk-Tech/go-cfgedit/ini"
)

func main() {
	cfg := "## Plugin GUID: com.example.hud\n\n[General]\n\n# Setting type: Int32\n# Acceptable value range: From 10 to 300\nDelay = 120\n"
	doc := cfgedit.ParseBepInEx(cfg)
	for _, e := range doc.Entries() {
		fmt.Println(e.Section, e.Key, e.Type, e.Value)
	}

	out, err := doc.Set("General", "Delay", "150")
	fmt.Printf("%q %v\n", out, err)

	_, err = doc.Set("General", "Delay", "5")
	var ve *cfgedit.ValueError
	if errors.As(err, &ve) && ve.Problem == cfgedit.OutOfRange {
		fmt.Println("allowed:", *ve.Min, "to", *ve.Max)
	}
	_, err = doc.Set("General", "Gone", "1")
	fmt.Println(errors.Is(err, cfgedit.ErrNoSetting))

	text, err := ini.Set("[Options]\r\nmods = 1\r\n", "Options", "mods", "0")
	fmt.Printf("%q %v\n", text, err)
}
```

## Contracts

| Behaviour | Contract |
| --- | --- |
| `Set` on any editor | Returns the new text; the input is never modified and no file is touched. Every byte outside the edited value is unchanged, and each line keeps its own ending. |
| `ParseBepInEx`, `ParseMelon` | Never fail; a line they do not understand is skipped. `Entries` is in file order. |
| `BepInEx.Set` | Refuses a value BepInEx would reset to the default at start-up: wrong type, outside the file's range or the .NET type's, not an enum member, a line break. Enum names are written in the member's own spelling. |
| `BepInEx` `Entry.Value` | As the file holds it, string escapes included. |
| `Melon.Set` | Writes the value as TOML and refuses one that would turn a parsable file unparsable (`BreaksTOML`). A file already unparsable is edited without that check. |
| `Melon` `Entry.Value` | A string is unquoted and unescaped; a `TypeRaw` value (inline table, date, hex integer) is its TOML text, written as typed. |
| Missing key | The error wraps `ErrNoSetting`; the message is `no setting <key> in [<section>]`. |
| Refused value | `*ValueError`; `Problem` strings are stable, `Error()` is an English sentence for logs, `Min`, `Max` and `Values` carry the detail. |
| Duplicate key | `Set` edits the first one. |
| `ini.Set` | Names compared without case, comments ignored; a missing key is appended inside its section, a missing section at the end. The edited line becomes `key = value`, dropping its indentation and inline comment. |
| `ini.Set` refusals | `ErrNotText` for UTF-16 or NUL content, `ErrBadName` for a key that is empty, holds `=` or starts with `;`, `#` or `[`, a line break or NUL in a key or value, or `]` in a section. The text is returned untouched. |
| `msbuild` | `Parse` finds each element's byte span; `SetProperty`, `SetRawProperty`, `RemoveProperty`, `SetItem`, `Remove` and `SetAttr` change only those bytes, so comments, indentation and attribute order stay. Values go through `Escape`/`EscapeXML`; `CheckValue` refuses a character XML cannot carry (`ErrBadChar`). |
| Known gaps | TOML dotted keys and multi-line inline tables are not parsed as entries; the TOML check is `BurntSushi/toml`'s TOML 1.0 reader, not Tomlet; INI has no continuation lines. |

## Requirements

Go 1.27.3 or newer, plus `lefthook` and `gitleaks` on PATH if you want the git hooks.

```bash
lefthook install
```

That is per clone; git does not commit hooks. Pre-commit scans staged changes for secrets, and
pre-push runs `gate`.

## Verify

```bash
gate
```
