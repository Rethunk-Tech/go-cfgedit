# Changelog

Notable changes to `go-cfgedit`. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `ParseBepInEx`: read a BepInEx `.cfg` into typed entries (type, default, range, acceptable values, description, plugin name and GUID) and `Set` one value, validated the way BepInEx parses it, with every other byte kept.
- `ParseMelon`: read a MelonLoader `MelonPreferences.cfg` into entries typed from their TOML values and `Set` one value as TOML, refusing an edit that would leave an unparsable file.
- `ValueError`, `Problem` and `ErrNoSetting`: a refused value carries a stable problem code, the bound broken and the acceptable values; a missing setting is told apart with `errors.Is`.

[Unreleased]: https://github.com/Rethunk-Tech/go-cfgedit/commits/main
