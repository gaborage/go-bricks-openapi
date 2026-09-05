# Survey seam and Constraint set collapse — design

Record of the 2026-09-04 architecture review and grilling session. Three PRs, in this order, each branched from `main`. Domain terms are in `/CONTEXT.md` (Shape, Resolution, Constraint set, Survey, Pre-flight check); the seam decision is `docs/adr/0001-survey-seam.md`.

Vocabulary: **module** (interface + implementation), **seam** (where a module's interface lives), **adapter** (what satisfies an interface at a seam), **depth**/**leverage**/**locality** as in the codebase-design skill.

## PR A — `refactor(generator): typed constraint set replaces pair-list applicators`

**Friction.** After PR #58 (the relocation, "2a" of `docs/superpowers/plans/2026_09_01_constraint_module.md`) the Constraint set lives in `internal/generator/constraints.go` but is still carried as a stringly `[]openAPIConstraint{Name, Value any}` list: a placeholder-and-partner pass (`resolveMostRestrictive`, `boundPlaceholder`, `readBound`, `materializeBounds`) resolves most-restrictive bounds, and 13 six-line `apply*Constraint` funcs behind a `constraintApplicators` name→func map type-assert `Value` back out. Two callers (`applyConstraints`, `applyElementConstraints`) orchestrate the `dive` element path themselves.

**Decisions.** This PR is the "2b" section (Tasks 3–7) of that plan, grilled on 2026-09-01 and re-affirmed on 2026-09-04 (the session first drafted a rules-write-directly-onto-the-property variant, then adopted 2b because it keeps int64 bound precision, preserves the uint `minimum: 0` pre-stamp overwrite semantics structurally, and guards `$ref` items).
- `constraintSet` — typed fields (`format`, `pattern`, `enum`, the six integer keywords as `*int`, `minimum`/`maximum` as `*numericBound`) — replaces the list. Bounds reconcile at set time: largest floor, smallest ceiling; on an equal numeric value exclusive beats inclusive; a strictly tighter inclusive bound replaces an exclusive one and drops its flag. `numericBound` keeps int64 values exact so two distinct bounds above 2^53 never collapse when compared. `format`/`pattern`/`enum` are last-set-wins in sorted validator-key order (the retired applicators' last-writer-wins).
- `applyTo(prop)` writes only set fields, so a pre-stamped value (the uint `minimum: 0` from `setTypeAndFormat`) survives an empty set and is overwritten only by an explicit bound. Folding that floor into the module is issue #54, not this PR.
- `constraintsFor(shape, underlyingKind, constraints)` replaces `mapConstraintToOpenAPI` and returns `*constraintSet`; handlers keep their current parameter lists (three kind-agnostic ones take no `effKind`), dispatch through closures in the same order as before; the two cardinality functions return nothing.
- `applyValidationConstraints(prop, field)` is the single entry point owning both scopes; element rules apply only when `prop.Items` is an inline schema — `Items.Ref != ""` drops them, encoding the rule `refProperty` used to enforce by not calling the element path.
- `analyzer/builtins.go` (landed in #58) keeps the analyzer's own kind predicates; the module keeps verbatim private copies — accepted one-list-per-package.
- The mapper corpus (`constraints_test.go`) is re-asserted on the emitted `OpenAPIProperty`; precedence for repeated keywords and the uint-floor ordering are pinned by new tests.
- Goldens byte-identical (`go test ./internal/spectest`, never `-update`). Invariants kept: `example:` coercion only in `fieldInfoToProperty`; one-pointer unwrap for the scalar base, one-pointer-then-one-slice for the element shape.

## PR B — `refactor(commands): survey the project once, render per command`

**Friction.** `runGenerate` and `runDoctor` each re-ran analyze → stats → content warnings → analyzer warnings → go-bricks status → warned verdict, printing through global `os.Stdout`/`os.Stderr`, with two divergent verdict switches. `testutil.CaptureStdout` (swaps the global) exists to test that, and is why `t.Parallel()` is banned.

**Decisions.**
- New package `internal/survey`. `Run(ctx context.Context, projectRoot string) (*Survey, error)`. `Survey` carries separate slots — `Project *models.Project`, `Stats` (incl. `UntypedRoutes`), `ContentWarnings []string`, `AnalyzerWarnings []string`, `GoBricks GoBricksStatus`, `Warned bool` — because the commands render them in different orders and with different prefixes. It prints nothing and writes nothing. `Run` takes no `verbose`/`strict`: `verbose` is rendering; `--strict` is generate-only and applies to `Warned`.
- Moves in: `ProjectStats` (as `Stats`), `calculateProjectStats`, `classifyRoute`, `contentWarnings`; the go-bricks resolver (`resolveGoBricksStatus`, `parseGoBricksVersion`, the verdict enum, `minGoBricksVer`/`verifiedGoBricksVer`), exported so doctor's pre-flight can call it before any Survey. The `readFileFn` test seam becomes an unexported package var in `survey`.
- `runGenerate`/`runDoctor` gain `out, errw io.Writer` parameters wired from `cmd.OutOrStdout()`/`cmd.ErrOrStderr()` (the convention `validate.go` already uses). Rendering order in `generate` is unchanged: version warning → verbose module lines → analyzer warnings → content warnings → untyped-route line; `doctor` renders the stats block with untyped inside, then content, then analyzer warnings.
- The `verdictUnreadable` divergence (generate passes, doctor fails) is preserved; aligning it is a separate `fix`.
- `spectest.Generate` calls `survey.Run` and returns the spec plus the Survey's diagnostic lines (analyzer → content → untyped, no prefix).
- Tests: `survey` unit tests through `Run` with temp-dir module fixtures (same idiom as the analyzer's `analyzeModuleProject` helper); command tests keep calling `runGenerate`/`runDoctor` directly with `bytes.Buffer`s, replacing the three `CaptureStdout` uses in `doctor_test.go`; one `Execute()` test per command asserts flag → writer wiring. `testutil` stays for `main_test.go`. No `t.Parallel()` added.
- Docs: CLAUDE.md's test-package section notes the capture helper now serves only `main_test.go`.
- Verification claim: goldens untouched; command output byte-identical.

## PR C — `test(spectest): lock per-fixture diagnostics`

- `expected.warnings` beside `expected.yaml`: one diagnostic per line, no prefix, order analyzer warnings → content warnings → untyped-route line. Absent file = no diagnostics expected. `-update` writes the file only when non-empty and deletes a stale one when a fixture stops warning.
- `.gitattributes` gains `internal/spectest/testdata/**/expected.warnings text eol=lf`; the harness normalizes EOL on read.
- Three fixtures carry files today: `delegation`, `named_types`, `raw_add`.
- CLAUDE.md's fixture section gains the deletion-on-update rule.
