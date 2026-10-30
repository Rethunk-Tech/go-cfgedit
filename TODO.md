# TODO

Work that is decided but not built. `AGENTS.md` and `README.md` describe what
exists; this file is the only place that describes what does not.

## Complexity check on the split functions

The plan asks for a check that `validate`, `literal`, `valueEnd` and `ini.Set` (and the functions split from them)
stay at gocognit 15 or below. Canon enables no gocognit linter, so adding one here is a canon decision. Today the
highest production function is `ini.scan` at 14 (gocognit v1.2.0). Done when the gate fails on a production function
above 15 and test functions are exempt.
