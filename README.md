<h1 align="center">go-cfgedit</h1>

<div align="center">

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.27.3+-00ADD8?logo=go&logoColor=white)](go.mod)

</div>

---

Byte-preserving single-setting editors for BepInEx .cfg, MelonLoader MelonPreferences.cfg and INI files. Package `cfgedit`, module `github.com/Rethunk-Tech/go-cfgedit`.

Each editor takes the file's text, changes one setting and returns the new text. Every other byte stays as it was, including a byte order mark and each line's own line ending, so a hand-edited config keeps its comments, order and spacing. Nothing here reads or writes a file.

## Quick start

```bash
go get github.com/Rethunk-Tech/go-cfgedit
```

A worked example and the contracts: [HUMANS.md](HUMANS.md).

## Features

- BepInEx `.cfg`: typed entries read from the `#` comment lines, values validated the way BepInEx parses them.
- MelonLoader `MelonPreferences.cfg`: types read off the TOML values, edits written as TOML and re-checked as a whole.
- INI: `ini.Set` edits one `key = value` in a section, adding the key or section when missing.
- Refusals are typed: `ValueError` carries a stable `Problem` code, the bound broken and the acceptable values.
- Per-line endings and the BOM survive every edit.

## Documentation

| Doc | Audience |
| --- | --- |
| [HUMANS.md](HUMANS.md) | Running and using the package |
| [AGENTS.md](AGENTS.md) | Internals, invariants, and why |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Process — commits, tests, docs |
| [SECURITY.md](SECURITY.md) | Vulnerability disclosure |
| [CHANGELOG.md](CHANGELOG.md) | Release notes |

## License

MIT — see [LICENSE](LICENSE). © 2026 Rethunk.Tech, LLC.
