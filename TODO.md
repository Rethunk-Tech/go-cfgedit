# TODO

Work that is decided but not built. `AGENTS.md` and `README.md` describe what
exists; this file is the only place that describes what does not.

## Complexity check

`validate`, `literal`, `valueEnd` and `ini.Set`, and the functions split from them, stay at gocognit 15 or below.
Canon enables no gocognit linter, so adding one here is a canon decision. Done when the gate fails on a production
function above 15, with test functions exempt.
