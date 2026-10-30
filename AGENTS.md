# AGENTS.md

Package `cfgedit` edits one setting of a BepInEx `.cfg` or a MelonLoader `MelonPreferences.cfg`, and `cfgedit/ini` does
the same for INI. Applications that let a person change a game's or plugin's config import it; it imports no consumer
package and does no file I/O.

## Gate

`gate` is the command. Each override in `.gate.toml` is there because a runner
cannot resolve that check from PATH.

## Shape

- Text in, text out. No function opens a file, so permissions and atomic replacement stay with the caller.
- Every edit goes through `internal/textdoc`, which keeps each line's own ending: a file mixing `\r\n` and `\n` comes back
  with the same mix. Only the edited line (or, for Melon, its value's lines) differs.
- Validation follows the loader's own reader, because a value the editor accepts and the game rejects is silently reset:
  BepInEx's `TomlTypeConverter` for `.cfg`, TOML for Melon. The .NET type bounds and Decimal's exact bound travel in
  `ValueError.Min`/`Max`, never in `Entry.Min`/`Max`, which hold only what the file states.
- `Problem` strings are wire codes callers switch on; changing one breaks them. Every `switch` over `Type` or `Problem`
  lists every constant (the `exhaustive` lint has no default-covers-all).
- `Melon.Set` re-parses the whole file with `BurntSushi/toml` as a validator only; it never writes. One unparsable line
  makes MelonLoader run on defaults with saving disabled.
- An unclassified Melon value (inline table, date, hex integer) is `TypeRaw`, editable as typed rather than read-only,
  because MelonLoader reads it. `Set` refuses it empty or when it would break the file.
- `ini.Set` trims the key and section before comparing, because lines are compared trimmed; checks run on the raw names
  first so trimming cannot hide a line break.
- Fixtures under `testdata/` are bytes (`-text`): they hold a BOM and mixed endings on purpose.

## Files

| File | Holds |
| --- | --- |
| `types.go` | `Type`, `Entry`, `ValueError`, `Problem`, `ErrNoSetting`, number parsing shared by both editors |
| `bepinex.go` | `ParseBepInEx` and `BepInEx.Set`: comment-metadata parser and .NET-style validation |
| `melon.go` | `ParseMelon` and `Melon.Set`: TOML value typing, literals and the whole-file TOML check |
| `ini/ini.go` | `ini.Set`, `ini.Check` and the two refusals |
| `internal/textdoc/textdoc.go` | Lines with their own endings and the BOM |
| `doc.go` | Package comment |
