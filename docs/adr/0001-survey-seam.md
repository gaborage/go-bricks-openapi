---
status: proposed
date: 2026-09-04
---

Status flips to accepted when the survey package lands (PR B of the 2026-09-04 series).

# The Survey is the seam between analysis and the commands

`generate` and `doctor` each re-run analyze → stats → content warnings → analyzer warnings → go-bricks status → warned verdict, reporting every stage through global stdout/stderr and keeping two divergent verdict switches. We will put that shared reading behind one package, `internal/survey`, that returns a value and performs no output; the commands will become renderers writing to their cobra-provided writers. Two callers (`commands`, `spectest`) will make the seam real.

## Considered options

- Keep the run inside `internal/commands` as an unexported module — one caller, untestable except through command tests, `commands` keeps importing `analyzer`.
- Put the run inside `internal/analyzer` as a higher-level entry point — drags command vocabulary (stats, strict) into the AST walker.
- Widen the run to include spec rendering, `--validate`, and file writing — all `generate`-only; nothing else needs them.

## Consequences

- The go-bricks dependency resolver will live in `survey`, not `commands`, because the Survey's warned bit will depend on it. Doctor's pre-flight check will call it there. The Go-version and directory-layout checks will stay in `doctor` — they are Pre-flight checks (see `CONTEXT.md`), not part of any Survey.
- The Survey will carry its diagnostics as separate slots (analyzer warnings, content warnings, untyped routes, go-bricks status) rather than one merged list, because the two commands render them in different orders and with different prefixes. Merging them would be a rendering change, not a refactor.
- `generate` and `doctor` still disagree on `verdictUnreadable` (generate passes, doctor fails). The refactor will preserve that on purpose; aligning it will be a separate `fix`.
- `testutil.CaptureStdout` will no longer be needed by `commands` tests. It will stay for `main_test.go`, which still exercises `main()` against the real `os.Stdout`.
