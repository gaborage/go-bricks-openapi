---
status: accepted
date: 2026-09-04
---

# The Survey is the seam between analysis and the commands

`generate` and `doctor` each re-ran analyze → stats → content warnings → analyzer warnings → go-bricks status → warned verdict, reporting every stage through global stdout/stderr and keeping two divergent verdict switches. We put that shared reading behind one package, `internal/survey`, that returns a value and performs no output; the commands become renderers writing to their cobra-provided writers. Two callers (`commands`, `spectest`) make the seam real.

## Considered options

- Keep the run inside `internal/commands` as an unexported module — one caller, untestable except through command tests, `commands` keeps importing `analyzer`.
- Put the run inside `internal/analyzer` as a higher-level entry point — drags command vocabulary (stats, strict) into the AST walker.
- Widen the run to include spec rendering, `--validate`, and file writing — all `generate`-only; nothing else needs them.

## Consequences

- The go-bricks dependency resolver lives in `survey`, not `commands`, because the Survey's warned bit depends on it. Doctor's pre-flight check calls it there. The Go-version and directory-layout checks stay in `doctor` — they are Pre-flight checks (see `CONTEXT.md`), not part of any Survey.
- The Survey carries its diagnostics as separate slots (analyzer warnings, content warnings, untyped routes, go-bricks status) rather than one merged list, because the two commands render them in different orders and with different prefixes. Merging them is a rendering change, not a refactor.
- `generate` and `doctor` still disagree on `verdictUnreadable` (generate passes, doctor fails). The refactor preserved that on purpose; aligning it is a separate `fix`.
- `testutil.CaptureStdout` is no longer needed by `commands` tests. It stays for `main_test.go`, which still exercises `main()` against the real `os.Stdout`.
