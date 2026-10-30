# Security

## Supported versions

The latest tagged release. There are no long-lived release branches, so a fix ships in the next tag
rather than being backported.

## Reporting a vulnerability

Use GitHub's **private vulnerability reporting** on this repository (Security → Report a
vulnerability). It stays private until a fix ships.

Please do not open a public issue for a suspected vulnerability first.

## What is in scope

- Config text passed to `ParseBepInEx`, `ParseMelon`, `ini.Set` and `ini.Check`: arbitrary bytes must not cause a
  panic, an unbounded allocation beyond the input size, or a change to any line other than the edited one.
- The `key`, `section` and `value` passed to `Set`: a value holding a line break must not become extra lines
  (`ini.Set` returns `ErrBadName`, the typed editors return `MultiLine`), and a Melon value must not leave a
  parsable file unparsable.

## What is out of scope

- Reading and writing the file, its permissions and atomic replacement: the caller does both.
- Deciding which settings a person may change: `Set` edits any setting that exists.
- Values the game itself rejects but the format allows beyond what the package checks (a plugin's own validation).
