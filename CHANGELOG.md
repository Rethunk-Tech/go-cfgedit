# Changelog

Notable changes to `go-cfgedit`. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.5.0] - 2026-11-21

### Changed

- Breaking: `jsonfile.Set` now has the signature `func(string, []string, Node) (string, error)` and returns an error. `jsonfile.MaxDepth` (64) and `jsonfile.ErrTooDeep` are new: nesting past 64 levels is refused.

### Added

- `jsonfile` splices one value into the file's text, keeping every other byte.

### Fixed

- `msbuild` reads and edits only the project's own properties, not a `Choose` or `Target` branch, and adds a missing property to an unconditional `PropertyGroup`.
- `msbuild.SetItem` treats a nil `after` as matching no item.
- Melon and INI split on a lone CR, so `Set` keeps the next key.
- A Melon leading-zero integer is typed as raw, matching TOML.
- `ParseMelon` runs in linear time on unclosed arrays.
- `jsonfile` keeps a UTF-8 byte order mark, refuses invalid UTF-8 when encoding a string, and refuses nesting past 64 levels.
- A duplicate key edits the last one.

## [0.4.2] - 2026-11-12

### Fixed

- `msbuild` reads and edits the last unconditional property, as MSBuild evaluates it; compares parent names, `Include` and attributes without case; keeps attributes when filling a self-closing property; and `Remove` no longer panics when a match nests inside another.
- Melon: strings close after an even backslash run, a key splits at the first `=` outside quotes, and a table header with an inline comment no longer hides its entries (a value comment keeps its spacing).
- `jsonfile`: `Marshal` no longer escapes `&`, `<` and `>` in strings it was not asked to change, `Coerce` refuses Inf, NaN and non-JSON numbers, and `Set` and `Lookup` use the last case-insensitive key.
- `bepinex.Set` edits the last duplicate key, matching BepInEx.

## [0.4.1] - 2026-11-12

### Fixed

- `ini.Check` detects a UTF-16 byte-order mark from the first two bytes.

## [0.4.0] - 2026-11-08

### Added

- `jsonfile`: parse a mod's lenient JSON config (`Parse`), read a value by dotted path (`Lookup`), type raw text as the setting's kind (`Coerce`) and set one value (`Set`), keeping key order and every other value; `SplitPath` unescapes a `.` or `\` in a key, and `Node.Marshal` writes the two-space-indented file.

## [0.3.0] - 2026-11-07

### Changed

- `msbuild.SetRawProperty` refuses text holding markup or a malformed entity (`ErrMarkup`), and `SetProperty`, `SetRawProperty`, `SetItem` and `SetAttr` refuse a property, item type or attribute name that is not a plain XML name (`ErrBadName`).

### Fixed

- `msbuild.SetAttr` no longer mistakes an attribute name written inside another attribute's value for the attribute.
- `ParseBepInEx` ends a line at a lone CR, as BepInEx's reader does, so `Set` no longer drops the setting after it.
- `Melon.Set` refuses a multi-line value in a file that does not parse, instead of deleting the lines after an unclosed string.
- `ini.Set` refuses a name that would make the result read as UTF-16.

## [0.2.0] - 2026-10-30

### Added

- `msbuild`: read and edit the properties, items and attributes of a `.csproj`, `.props` or `.targets` in place, changing only the edited element's bytes.

## [0.1.0] - 2026-10-29

### Added

- `ParseBepInEx`: read a BepInEx `.cfg` into typed entries (type, default, range, acceptable values, description, plugin name and GUID) and `Set` one value, validated the way BepInEx parses it, with every other byte kept.
- `ParseMelon`: read a MelonLoader `MelonPreferences.cfg` into entries typed from their TOML values and `Set` one value as TOML, refusing an edit that would leave an unparsable file.
- `ValueError`, `Problem` and `ErrNoSetting`: a refused value carries a stable problem code, the bound broken and the acceptable values; a missing setting is told apart with `errors.Is`.
- `ini.Set` and `ini.Check`: set `key = value` in an INI section, adding the key or the section when missing, refusing UTF-16 or NUL content and names that would break the lines.

[Unreleased]: https://github.com/Rethunk-Tech/go-cfgedit/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/Rethunk-Tech/go-cfgedit/compare/v0.4.2...v0.5.0
[0.4.2]: https://github.com/Rethunk-Tech/go-cfgedit/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/Rethunk-Tech/go-cfgedit/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/Rethunk-Tech/go-cfgedit/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/Rethunk-Tech/go-cfgedit/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/Rethunk-Tech/go-cfgedit/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Rethunk-Tech/go-cfgedit/releases/tag/v0.1.0
