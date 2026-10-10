# Importable Package Clause (#122) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An in-module package is named, and its structs and named types are searched, by the package clause of the files the go command builds into it. Some files sort first in the directory but are never built into the package. These are a `//go:build ignore` generator in `package main`, a build-ignored tool of another clause, a `//go:build tools` file or a GOOS/GOARCH-suffixed file (`a_windows.go`) of another clause next to an unconstrained file, a file named `_…` or `.…`, and a `package documentation` file. None of them may rename an unaliased import or stand in for one of the package's types. Today a sorted-first generator renames the import to `main`, so every `b.X` reference falls back (fields, embeds, payloads, request bodies, route delegates). Under any import, a same-named struct in such a file replaces the real one. Closes #122. On valid code, one measured layout regresses against `main` (row R16, found in the revision 4 behavior review), and every other measured layout matches or improves on it (rows R11-R15, R17). Decision 6 decides whether that ships as `fix!` or waits for a changed fallback.

**Blocking: answer open decision 6 before Task 1.** Task 1's tests already encode the answer. The body changes two of the brief's adopted defaults:
- **The tie-break "as in #129".** A file with no build constraint (no `//go:build` line and no GOOS/GOARCH file-name suffix) now names the package, and files go never builds are skipped.
- **The unconditional clause filter on `resolveQualifiedStruct`.** It no longer applies when the clause came from a constrained file.

Taken as written, the defaults break valid code that `main` resolves (Appendix B). The body breaks less, but not nothing: R16. Changing an adopted default is the maintainer's call, so the implementer must not go past Task 1 Step 0 without a written answer. No agent message counts as that answer. The orchestrator relays this one line to the user and waits for the user's own reply:

> Issue 122, decision 6: (a) the plan's body, measured: fixes every reported row, but regresses one valid layout (R16: every file of the package build-constrained, and a `_`- or `.`-named file, or a `//go:build ignore` file, of the package's own clause sorts before a file of another clause under another constraint (`//go:build tools`, or a GOOS/GOARCH-suffixed name such as `a_windows.go`); unaliased references to it stop resolving, and so do those to a real package of that other clause imported earlier in the same file), so it ships as `fix!`, MINOR. Or (a') the same with a changed fallback that keeps `main`'s answer for R16, ships as `fix`, PATCH, but needs another plan revision with a prototype first. Or (b) the triage comment's defaults as written: R16 regresses too, and so do aliased struct references behind a sorted-first `//go:build tools` file, `fix!`, MINOR. Which one? Also decision 8, smaller: the triage comment drops the unresolvable-type warning's cause "an unaliased in-module import whose directory's first file has another package clause", but the R13 residual still reaches that cause under every option, and R16 does under (a) and (b), so either keep it reworded to "an in-module package whose buildable files all carry a build constraint and whose first one by name has another package clause" (recommended), or drop it as the triage comment says?

A reply of "go with (a), keep the warning" is enough. A reply that names only decision 6 answers decision 6, and decision 8 then takes the triage comment's default (drop the cause), because rewording it amends an adopted default too. Revision 4's question claimed "(a1+): no measured regression on valid code"; R16 makes that false, so an answer given to that wording does not count. An answer given to revision 5, 6 or 7's wording still counts. Revision 8 changed no option and no outcome; it only widened the R16 layouts the question names.

**Architecture:** `importableClause` (`internal/analyzer/resolution.go:350`) already decides which file of a directory names the package for named non-struct lookups (`inModuleTypeSite`, PR #129).
- **New `packageClause`.** `importableClause`'s loop moves into it, and it also reports whether the clause is *sure*, meaning an unconstrained file named it. A file is unconstrained when it has no `//go:build` line and no GOOS/GOARCH file-name suffix. `importableClause` becomes its two-value wrapper. `inModuleTypeSite` is unedited but inherits the new rule.
- **New `neverImportable`.** It matches the files go/build never builds into a package: `package main`, `package documentation`, and names starting with `_` or `.`.
- **New `osArchSuffixed`**, with local copies of go/build's GOOS and GOARCH lists (`knownOS`, `knownArch`). It reports whether a file name carries go/build's implicit platform constraint (`x_windows.go`, `x_amd64.go`). Only the suffix's presence is read.
- **New `inPackage`.** It decides which files the struct search reads.
- **`unaliasedImportName`** (`analyzer.go:3669`) returns `importableClause`'s clause, falling back to `filepath.Base(path)`.
- **`resolveQualifiedStruct`** (`analyzer.go:3539`) searches only `inPackage` files.

Every consumer of `fileImports` (`analyzer.go:3545`, `resolution.go:326`, `marshaler.go:152`) and of `resolveQualifiedStruct` (field refs, embeds, payloads, request bodies, delegates, alias chains) inherits the fix without changing. `internal/generator`, `internal/models`, the constraint module and every command are untouched.

**Tech Stack:** Go; `go/ast` static analysis only; goldens in `internal/spectest`. The language floor is Go 1.25 (CLAUDE.md: "The 1.25 floor is intentional — don't rely on language features newer than that"). The change uses nothing newer than Go 1.23 (`slices.Sorted`, `maps.Keys`; `strings.Cut` is Go 1.18).

**Spec (authoritative, in this order):** the 2026-10-09 re-triage comment on #122 (on `main 7ff8391`), section "Adopted defaults (ready-for-agent)". It supersedes the issue body where they disagree. Decision 6 asks to amend two of its defaults. Glossary: `CONTEXT.md` (**Resolution**, **Marshaler type**). Design: `docs/adr/0002-named-type-resolution.md`. Precedent: `docs/superpowers/plans/2026_10_08_qualified_named_types_pr2.md` (PR #129, which introduced `importableClause`). This plan has no doc fixes outside #122; they go in a separate change (decision 3, Appendix C).

**Baseline for line anchors:** `origin/main` at `033a9c9` (`build(release): stop publishing empty GitHub Release bodies (#137)`). Nothing under `internal/`, `cmd/`, `README.md`, `CONTEXT.md` or `docs/adr/` changed between the brief's `7ff8391` and `033a9c9`. `git diff --stat 7ff8391 033a9c9` touches only `.github/workflows/{ci,release}.yml`, `.goreleaser.yaml` and `CLAUDE.md`, so every anchor the brief gives still holds, and each is re-checked below. Re-check every anchor with `grep -n` before editing.

**Prototype evidence (revision 4, planner, 2026-10-09):** every code and test block below was applied to a scratch export of `033a9c9` (`git archive`, outside the repo, under `scratchpad/issue122/rev4/repo`; the full diff is `rev4/rev4.diff`) and run:
- `go test ./...` and `go test -race ./internal/analyzer` green.
- The pinned golangci-lint v2.14.0 reports `0 issues` on `./...`.
- `make sec` scans 22 files with 0 issues, and `make validate-cli` is green.
- `gocognit -over 15` is clean on every touched file.
- In the analyzer package's own profile, `resolveQualifiedStruct`, `unaliasedImportName`, `importableClause`, `packageClause`, `neverImportable`, `osArchSuffixed`, `inPackage` and `inModuleTypeSite` are at 100.0%.
- Every existing golden is byte-identical, and the new golden is byte-identical to revision 3's (`diff -rq` of `testdata` against the revision 3 prototype).
- Mutations M1-M20 behave as tabled in Task 7. Only M4, a documented equivalent mutant, survives.
- The red runs of Task 1 Step 3 and Task 2 Step 2 were repeated for the new stand-in test (`rev4/red`, `rev4/t2`).

In the behavior table, "Before" cells are a `033a9c9` binary's output on the scratch fixtures, and "After" cells are the revision 4 binary's (`rev4/scripts/measure.sh`, outputs in `rev4/out`).

**Prototype evidence (revision 6, fixer, 2026-10-10):** the revision 4 prototype, copied to `scratchpad/issue122/rev6/repo` (its `analyzer.go` and `resolution.go` are byte-identical to `rev4/repo`'s), with the revision 6 blocks applied in `rev6/fixed` (the `_test` strip in `osArchSuffixed` and its `TestOSArchSuffixed` row) and `rev6/fixed8` (also decision 8's default wording and its pin):
- `go test ./...` and `go test -race ./internal/analyzer` green in `rev6/fixed`, and `go test ./...` green in `rev6/fixed8`; no golden moved.
- The pinned golangci-lint v2.14.0 reports `0 issues` on both.
- `gocognit -over 15` is clean on `analyzer.go`, `resolution.go` and `qualified_test.go`; `osArchSuffixed` stays at 2.
- `packageClause`, `osArchSuffixed` and `inPackage` stay at 100.0% in the analyzer package's own profile.
- A brute-force check over 209 generated non-`_test.go` file names (`rev6/brute/main.go`, against `build.Context.MatchFile` on 16 GOOS/GOARCH pairs) finds 32 names where revision 5's `osArchSuffixed` disagrees with go/build, every one of the shape `…_<GOOS|GOARCH>_test.<x>.go`, and 0 with the `_test` strip.
- Mutations M21 and M22 fail as tabled in Task 7 (`rev6/m21`, `rev6/m22`). M1-M20 were not re-run: no line they mutate changed.

**Revisions:**
- **Revision 2:** R11 was re-measured (12 → 14, not "unchanged"), the R11b regression was found, `TestQualifiedToolsFileSortsFirst` was added, and Appendix A1 was prototyped.
- **Revision 3:**
  - Appendix A1 moved into the body.
  - Files that go/build never builds are skipped (review finding 1).
  - The struct search keeps `main`'s behavior when the clause came from a constrained file. This closes a narrow regression that A1 itself had (R13), found while verifying findings 2 and 3.
  - The doc bundle was split out (finding 4).
- **Revision 4:**
  - A GOOS/GOARCH file-name suffix now counts as a build constraint (`osArchSuffixed`). Revision 3 took a suffixed file of another clause for the package and regressed valid code (R15).
  - The new `TestQualifiedAllConstrainedStandIns` pins the three guards that no test caught before (`inPackage`'s `!buildIgnored` and `!neverImportable`, and `neverImportable`'s `package main` term), plus the suffix rule. `TestOSArchSuffixed` pins the suffix reader. These are mutations M13-M20.
  - Decision 6 carries a one-line question for the orchestrator to relay to the user.

  Details are in "Review dispositions (revision 4)".
- **Revision 5 (fixer, after the behavior and adversarial reviews of revision 4):** plan text only; no code or test block changed, and nothing was implemented, because decision 6 is unanswered.
  - R16 is a measured regression of the body on valid code (`advrev/x1`, `x2`). The Goal, decision 6 and its one-line question, Impact, Residuals, the README block and the commit message now say so, and the commit becomes `fix(analyzer)!:` under (a). Task 1 gains a step that pins R16 under whichever option is chosen.
  - The README block, R13 and the commit's "Residual:" paragraph no longer claim that only files of the clause are searched when every file is constrained (`behav/fx/r16`, `r16w`).
  - Impact and the behavior table now cover a generator sorted between two real files (R17, `behav/fx/mid`).

  Details are in "Review dispositions (revision 5)".
- **Revision 6 (fixer, after the gate, behavior and adversarial reviews of revision 5):** nothing was implemented or committed, because decision 6 is still unanswered. One code block changed, plus its test:
  - `osArchSuffixed` drops a trailing `_test` element before reading the suffix, as go/build's `goodOSArchFile` does. `parsePackageDir` reads non-test names such as `z_windows_test.pb.go`, and without the strip such a file of another clause became the sure clause and brought R15 back (new R15 row, `rev6/fx_suffix_test`). `TestOSArchSuffixed` gains the row, and mutation M21 pins it.
  - New open decision 8: the triage comment drops the unresolvable-type warning's first-file cause, but R13 and R16 still reach it. The recommended default keeps it, reworded, and the relayed question now asks it. Task 4, Impact item 4, the commit message and the self-review follow it. Mutation M22 pins the wording.
  - The commit message's first body paragraph is re-wrapped to 72 columns.

  Details are in "Review dispositions (revision 6)".
- **Revision 7 (fixer, after the gate, behavior and regression reviews of revision 6):** plan text only; nothing was implemented or committed, because decisions 6 and 8 are still unanswered. Impact item 2, the README block (Task 6 Step 2) and the commit's second paragraph now disclose that a newly resolving struct can take a bare component name from a same-named struct of another package (`regrev/fx/swap`). Details are in "Review dispositions (revision 7)".
- **Revision 8 (fixer, after the gate, behavior and regression reviews of revision 7):** plan text only; nothing was implemented or committed, because decisions 6 and 8 are still unanswered. No code or test block changed. The R16 disclosure now covers what the body's code already does:
  - The skipped own-clause file can also be named with a leading `.` (`beh8/fx/x1dot`), and the other-clause file can be a GOOS/GOARCH-suffixed file with no build line (`a_windows.go`, `beh8/fx/x1win`); both measured `Warnings: 0` → `4`, as `x1`. Decision 6's one-line question, the R16 rows, Impact item 7, decision 6's measured result, the commit's "Regression:" paragraph, its `BREAKING CHANGE:` footer and the PR body's Impact say so.
  - Under R16, an earlier unaliased import of a real in-module package of the stand-in's clause also stops resolving, because the misnamed import replaces its `fileImports` entry (`regrev8/fx/coll`: `Warnings: 0` → `3`). The same places say so; the overwrite rule is unchanged.

  Details are in "Review dispositions (revision 8)".
- **Revision 9 (fixer, after the post-commit review of the implementation):** Impact item 7 now notes that under R16 an aliased named non-struct type takes the other clause's declaration silently when that file declares the same name, instead of falling back; the stand-in rows of `TestQualifiedAllConstrainedOwnClauseFirst` and `TestQualifiedAllConstrainedToolsFirst` pin it, and the README and commit message say so. No code changed.

---

## Stale points in the brief (checked against `033a9c9`)

- **Anchors.** All hold:
  - `unaliasedImportName` is `analyzer.go:3669-3686`. The brief's `:3680` is its first-file loop, `3679-3683`.
  - `resolveQualifiedStruct` is `analyzer.go:3539-3566`, with the unfiltered loop at `3559-3564` (brief: `:3559`).
  - The `fileImports` consumers are `analyzer.go:3545`, `resolution.go:326` and `marshaler.go:152`.
  - `importableClause` is `resolution.go:346-357` (doc from `:346`), `buildIgnored` is `:362-368`, and `fileBuildConstraint` is from `:372`.
  - `samePackageFiles` is `resolution.go:655`.
  - `TestQualifiedBuildTaggedVariants` is `qualified_test.go:266`, with its doc comment from `:259`.
  - The README caveats are `README.md:305-307`, inside the sentence starting at `:302`.
  - The build-constraints bullet is `README.md:394-399`, with its `:396` parenthetical and its `:399` exception.
  - The warning clause is `resolution.go:38-39`.
- **"Maintainer: please confirm #129 skipped this only to keep scope small."** The PR 2 plan says so verbatim, under "Out of scope (do not implement)": "`resolveQualifiedStruct` does not skip `package main` files … Leave it; only the new lookup skips them." It also lists "`unaliasedImportName`'s first-sorted-file clause: #122. Do not fix it". Both were deferred for scope, not rejected.

  The brief did not weigh what the filter does together with the tie-break "as in #129". In two layouts, PR 129's rule returns the clause of a file of another clause that go never builds into the package, and the filter then skips every file of the real package, even under an aliased import where `main` resolves:
  - a sorted-first `//go:build tools` file;
  - a sorted-first `_…`, `.…` or `package documentation` file (review finding 1).

  Appendix B has the numbers. The body therefore changes the tie-break and makes the filter conditional (decision 6).
- **Rows the brief does not list, found by the prototype:**
  - **R9.** A route **delegate** in such a package (`m.handler.RegisterRoutes(hr, r)`, `handler *handlers.PaymentHandler`) is not found today, so its routes vanish.
  - **R10.** A struct declared **only** in a generator (no importable declaration) resolves today to the generator's struct, and stops resolving after this PR.
  - **R12.** A named type behind a sorted-first `_…`/`doc.go` file is already wrong on `main` through PR 129's `inModuleTypeSite`.
  - **R14.** In a package whose every file is constrained, a never-built file of another clause that declares a same-named struct stands in for it on `main`.
  - **R15.** A GOOS/GOARCH file-name suffix is an implicit build constraint, and a suffixed file of another clause is valid Go beside a real file that excludes that platform.
- **`#127`'s request warning** ("request type b.Addr is not a struct … panics at request time"): after this PR it no longer fires for the brief's row. Its wording ("is not a struct" for any unresolvable request type, "panics at request time") is out of scope: it belongs to #120's `**T` PR, as the brief says.

---

## Global Constraints

- **Branch:** `fix/importable-package-clause`, cut from `origin/main` (`033a9c9`) in Task 0. Never push, open PRs or touch GitHub state from the implementation task. Commit only in Task 8.
- **Goldens:**
  - Only `go test ./internal/spectest -update` regenerates them, never `make update`.
  - `-update` is package-scoped: `go test ./... -update` fails every other package.
  - After `-update`, `git status --porcelain internal/spectest/testdata` may list **only** the new `?? internal/spectest/testdata/importable_clause/` directory. A moved existing `expected.yaml` is a bug: fix the code, don't accept the diff.
  - The golden test is `TestGoldenFixtures`. A `-run 'TestFixtures/…'` pattern matches nothing and exits 0.
- **Cognitive complexity ≤15 per function** (SonarCloud `go:S3776`; `make lint` checks only cyclomatic). Check with `gocognit -over 15 <touched files>` (installed at `~/go/bin/gocognit`; else `go run github.com/uudashr/gocognit/cmd/gocognit@latest`). Two functions are already over and stay untouched; do **not** "fix" them: `yamlNodeToJSONValue` (`internal/commands/generate.go:358`, 18) and the test `TestMarshalerGuardPositions` (`resolution_test.go:382`, 16). A tree-wide `gocognit -over 15 internal cmd` therefore exits 1 before and after this PR.
- **Coverage ≥80% on new code** from each package's **own** tests (no `-coverpkg`). All changed production lines are in `internal/analyzer`, and the analyzer tests cover them (see "Coverage strategy"). Fixtures add nothing to Sonar's gate (`**/testdata/**` is excluded).
- **`goconst` counts `_test.go` literals** (min-len 4, min-occurrences 3). The prototype's tests pass the pinned v2.14.0 with `0 issues`. If a literal tips over, add a constant or local rather than a `//nolint`.
- **No `t.Parallel()`.** No language features newer than Go 1.25.
- **Settled invariants (CLAUDE.md), untouched:**
  - `lookupStructTag` and `unquoteLiteral` stay the only tag and literal readers (`fileImports` keeps calling `unquoteLiteral`).
  - The Constraint set stays in `internal/generator/constraints.go`.
  - Example coercion stays in the `fieldInfoToProperty` wrapper.
  - `referencedSchemaNames` and `addRequestFieldRefs` are untouched.
  - The uint `minimum: 0` pre-stamp is untouched.
  - `NOSONAR` and `//nolint` are not interchangeable; this PR adds neither.
- **Out of scope (do not implement):**
  - The "panics at request time" wording of the non-struct request warning: #120's `**T` PR.
  - #123's leftovers: their own PR.
  - Evaluating build constraints other than `ignore`: the brief adopts ignore-only detection. The body only asks whether a file *has* a constraint (a `//go:build` line, or a GOOS/GOARCH file-name suffix through `osArchSuffixed`), never what it selects. Reading the suffix is the same presence test as reading the line, so it stays inside ignore-only detection. R13 is the residual this leaves.
  - Skipping `_…`/`.…` files in `parsePackageDir` itself (the broader option in review finding 1). It would also change `samePackageFiles`, `structDeclSite` and `findMethodDecl`, and needs its own goldens review. A `_…`/`.…` file that shares the package's clause therefore stays searched, as on `main` (see Residuals).
  - `findMethodDecl` and `findPackageFuncDecl` (`analyzer.go:1461`, `:1495`) still scan every file of a directory for a registration method or helper function. This is unchanged, and documented in the README sentence that replaces "struct lookups still scan every file".
  - Doc fixes outside #122 belong to a separate `docs:` change (decision 3; texts in Appendix C). They are README "Go 1.26+", CLAUDE.md:96 and :99, the release-please bullet, the release.yml comment, `internal/commands/doctor.go:34-41` (`minGoVersion = "go1.25"`) and `marshaler.go:109`. Do not touch `CLAUDE.md`, `.github/`, `internal/commands/` or `marshaler.go` here.
- **Gates before the commit:**
  - `make check` with the pinned golangci-lint. Run `make dev-deps` if `lint` reports a missing or mismatched binary, and `golangci-lint cache clean` if it reports files that do not exist.
  - `make sec`.
  - `go test ./internal/spectest`.
  - gocognit on the touched files.
  - The coverage spot-check and the mutation checks (Task 7).
- **One commit.** Its body stands alone (Task 8). This is a personal repo, so there is no `Refs:` line. The **only** `#N` token allowed in the commit message is the `Closes #122` trailer, for two reasons:
  - GitHub closes any issue named after a closing keyword anywhere in the message.
  - release-please copies every `#N` into the Release PR body as `closes [#N]`.

  Refer to every other issue or PR without the hash (`issue 116`, `PR 129`).

---

## Behavior rows: expected emitted YAML

Rows are in flow style for readability; goldens are block style. The fixture is scratch, identical to the new golden in Task 5:
- `b/aaa_gen.go` (`//go:build ignore`, `package main`) declares `type Addr struct{ Wrong int }`. It sorts before `b/b.go`.
- `b/b.go` (`package b`) declares `Addr{Street}`, `Base{Created}`, `Cents int64`, `Tags []string`, `Level int` with a value `MarshalText` and a pointer `UnmarshalText`, and a `Money` struct with `MarshalJSON`.
- The module imports `…/b` unaliased **and** as `bb`.

"Warns?" is after this PR.

| # | Row | Before (`033a9c9`) | After | Warns? |
|---|---|---|---|---|
| R1 | field `Addr b.Addr` | `{type: object}` + "b.Addr resolves to no schema" | `{$ref: '#/components/schemas/Addr'}` | no |
| R1 | field `[]b.Addr` | `{type: array, items: {type: object}}` + warning | `{type: array, items: {$ref: …/Addr}}` | no |
| R1 | field `*b.Addr` | `{type: object, nullable: true}` + warning | `{type: object, allOf: [{$ref: …/Addr}], nullable: true}` | no |
| R1 | field `map[string]b.Addr` | `{type: object, additionalProperties: {type: object}}` + warning | `{type: object, additionalProperties: {$ref: …/Addr}}` | no |
| R2 | field `b.Cents` (`int64`) | `{type: object}` + warning | `{type: integer, format: int64}` | no |
| R2 | field `b.Tags` (`[]string`) | `{type: object}` + warning | `{type: array, items: {type: string}}` | no |
| R2 | field `b.Level` (text pair) | `{type: object}` + warning | `{type: string}` | no |
| R2 | field `b.Money` (own `MarshalJSON`) | `{type: object}` + unresolvable warning | `{}` | **yes**: `b.Money has its own MarshalJSON method` (correct, ADR 0002) |
| R3 | payload `Result[b.Addr]` (`GET /addr`) | `data: {type: object, description: Response data}` + "response type b.Addr resolves to no schema component" | `data: {$ref: …/Addr}` | no |
| R3 | payload `Result[b.Cents]` (`POST /addr`) | `data: {type: object, description: Response data}` + warning | `data: {type: integer, format: int64}` | no |
| R4 | request `req b.Addr` (`POST /addr`) | no `requestBody` + "request type b.Addr is not a struct: … no requestBody emitted" | `requestBody: {required: true, content: {application/json: {schema: {$ref: …/Addr}}}}` | no |
| R5 | `type Embeds struct{ b.Base; *b.Addr; Note string }` | `Embeds: {properties: {note}}` (silent) | `Embeds: {properties: {created: string, note: string, street: string}}` | no |
| R6 | `type WrapL struct{ b.Level }` as field `Wrap WrapL` and payload `Result[WrapL]` | `$ref: …/WrapL`, component `WrapL: {type: object}` (empty; silent) | `{type: string}` both; no `WrapL` component | no |
| R7 | aliased `bb.Addr` field and `Result[bb.Addr]` (`GET /aliased`) | `$ref: …/Addr` with `Addr: {properties: {wrong: {type: integer, format: int64}}}` (the generator's struct; silent) | `$ref: …/Addr` with `Addr: {properties: {street: {type: string}}}` | no |
| R8 | aggregate on the fixture | "2 route(s) have no resolved request/response type: getAddr, createAddr"; `Warnings: 12`; doctor `Typed routes: 3/5` | no aggregate warning; `Warnings: 1` (R2's `b.Money`); doctor `5/5`. Without the `Money` field: `--strict` exit 1 → 0 | — |
| R9 | delegate: the `delegation` golden's project plus `internal/handlers/aaa_gen.go` (`//go:build ignore`, `package main`) | 0 operations; `Warnings: 3`: "skipping m.handler.RegisterRoutes(...)" and "…RegisterAdmin(...)" ("delegate type or method not found") + "modules discovered but no routes"; `--strict` exit 1 | 3 operations, as the `delegation` golden; `Warnings: 0`; `--strict` exit 0; doctor 3/3 typed | no |
| R10 | a struct declared **only** in a generator (scratch: `g/gen.go` alone in `g/` under alias `gg`; `b/aaa_gen.go` alone in `b/` under alias `bb`): field `gg.Addr` / `bb.Addr` | `$ref` to the generator's struct, registered under clause `main` (`Addr`, or `MainAddr` when `Addr` is taken); silent | `{type: object}` + "gg.Addr resolves to no schema (a type from outside the module, a well-known type under an aliased import, an in-module package whose buildable files all carry a build constraint and whose first one by name has another package clause, or a name with no declaration in its package)" | **yes** |
| R10 | same, payload `Result[bb.Addr]` | `data: {$ref: …/Addr}` | untyped `data` + "response type bb.Addr resolves to no schema component" | **yes** |
| R10 | same, request `req bb.Addr` | `requestBody` `$ref` to the generator's struct | no `requestBody` + "request type bb.Addr is not a struct: …" | **yes** |
| R11 | tools-first, **unaliased**: the R1-R7 fixture with `b/aaa_tools.go` (`//go:build tools`, `package tools`) in place of the generator | every `b.X` falls back; `Warnings: 12` | every row as in R1-R7; `Warnings: 1` (R2's `b.Money`); the spec body is identical to the `importable_clause` golden's | R2 only |
| R11 | same layout, **aliased** struct references (scratch aggregate: `b/` and `h/` both tools-first under aliases; field, embed, request, payload, delegate `m.hd.RegisterRoutes`) | all resolve; `Warnings: 0`; `--strict` exit 0; doctor 3 routes, 3/3 typed | unchanged: byte-identical output | no |
| R11 | same layout, **aliased** named non-struct `bb.Cents` (scratch: field, embed, request, payload, `bb.Cents`) | `{type: object}` + "bb.Cents resolves to no schema"; `Warnings: 1`; `--strict` exit 1 | `{type: integer, format: int64}`; `Warnings: 0`; `--strict` exit 0 | no |
| R12 | `b1/_tools.go` (`package tools`, declares `type Cents string`) sorts first; aliased `x1.Cents` | `{type: string}`: the `_` file's declaration, silent | `{type: integer, format: int64}` | no |
| R12 | `b2/doc.go` (`package documentation`) sorts first; aliased `x2.Cents`, unaliased `b2.Addr2` and `b2.Cents` | `{type: object}` + "resolves to no schema" each (with the row above: `Warnings: 3`, `--strict` exit 1) | `{type: integer, format: int64}`, `$ref: …/Addr2`, `{type: integer, format: int64}`; `Warnings: 0`; `--strict` exit 0 | no |
| R12 | aliased struct references behind a sorted-first `_tools.go`, `.tools.go` or `doc.go` of another clause (field, embed, request, payload) | resolve; `Warnings: 0` | unchanged: byte-identical output | no |
| R12 | the same files declaring their own `Addr` (`TestQualifiedToolsFileSortsFirst`, also for `aaa_tools.go`) | unaliased `b.Addr` falls back + warning; aliased `bb.Addr` is `$ref` to the stand-in's `{wrong: int}` (silent) | both `$ref: …/Addr` with `{street: string}`; no warning | no |
| R13 | residual, every file constrained: `b/b.go` under `//go:build !plan9`, `b/aaa_tools.go` (`//go:build tools`, `package tools`) sorts first; aliased field, embed and request `bb.Addr`, field `bb.Cents` | the structs resolve; `bb.Cents` falls back + warning; `Warnings: 1` | unchanged: byte-identical output. A1 alone gave `Warnings: 3` (requestBody and `created` lost) | `bb.Cents` (as before) |
| R13 | residual, constrained by name: `b/aaa_tools.go` (`//go:build tools`, `package tools`) sorts first, `b/b_linux.go` (no build line) and `b/b_other.go` (`//go:build !linux`) are `package b` (scratch `rev4/fx/e1e`; module as in R15) | unaliased `b.Addr` and `bb.Cents` fall back + warning; aliased structs resolve; `Warnings: 2` | unchanged: byte-identical output. Revision 3 resolved all of it (`Warnings: 0`), because it took `b_linux.go` for an unconstrained file | as before |
| R13 | residual, constrained by name: `b/a_windows.go` (`package bwin`, no build line, its own `Addr{Wrong}`) sorts first, `b/z.go` (`//go:build !windows`, `package b`) (scratch `rev4/fx/e1b`) | unaliased `b.Addr` and `bb.Cents` fall back + warning; aliased `bb.Addr` is the stand-in's `{wrong}` (silent); `Warnings: 2` | unchanged: byte-identical output. Revision 3 also lost the promoted `created` | as before |
| R13 | residual, every file constrained, the stand-in sorted in the middle: `b/a.go` (`//go:build !plan9`, `package b`; `Base`, `Cents`) and `b/z.go` (same constraint and clause; `Addr{Street}`) with `b/m_tools.go` (`//go:build tools`, `package tools`, its own `Addr{Wrong}`) between them. Same with `a.go`/`z.go` under `//go:build !windows` and `b/m_windows.go` (`package bwin`, no build line) (scratch `behav/fx/r16`, `r16w`; valid Go, `behav/valid`) | `Addr: {wrong: int}`, the stand-in's (silent); `Warnings: 0`; `--strict` exit 0 | unchanged: byte-identical output. The clause is `b` but not sure, so `m_tools.go`/`m_windows.go` is searched for structs wherever it sorts | as before (silent) |
| R14 | every real file constrained, a stand-in beside it: `b/types.go` under `//go:build !plan9` declares `Addr{Street}`, `Base`, `Cents`. A sorted-first file declares its own `Addr{Wrong}`: `_old.go` (`package old`), `doc.go` (`package documentation`), `aaa_tool.go` (`//go:build ignore`, `package tool`) or `aaa_gen.go` (`//go:build generate`, `package main`). Fields: unaliased `b.Addr`, aliased `bb.Addr`, embed `bb.Base`, `bb.Cents` (scratch `gosem4/fx/e2`-`e4` for `aaa_gen.go`, `aaa_tool.go` and `_tool.go`; `doc.go` from the red run of `TestQualifiedAllConstrainedStandIns`) | aliased `bb.Addr` is the stand-in's `{wrong: int}` (silent). Unaliased `b.Addr` falls back + warning, and so does `bb.Cents` behind `_old.go`/`doc.go`. `Warnings: 1`-`2` | `$ref: …/Addr` with `{street: string}` for both imports, `bb.Cents` `{type: integer, format: int64}`; `Warnings: 0` | no |
| R15 | file-name suffix, real file first: `b/a.go` (`//go:build !windows`, `package b`) and `b/z_windows.go` (`package bwin`, no build line, its own `Addr{Wrong}`). Valid Go: `package b` everywhere but Windows. Fields: unaliased `b.Addr`, aliased `bb.Addr`, embed `bb.Base`, `bb.Cents`, request `bb.Addr` (scratch `gosem4/fx/e1`) | everything resolves; `Warnings: 0` | unchanged: byte-identical output. Revision 3 regressed: `Warnings: 2`, aliased `bb.Addr` and the requestBody became the stand-in's `{wrong}`, and `created` was lost silently | no |
| R15 | same, the suffix before a trailing `_test` element: `b/a.go` (`//go:build !windows`, `package b`) and `b/z_windows_test.pb.go` (`package bwin`, no build line, its own `Addr{Wrong}`), which `parsePackageDir` reads. Valid Go: `GOOS=linux`/`darwin go list` gives `b go=[a.go] ign=[z_windows_test.pb.go]`, `GOOS=windows` gives `bwin go=[z_windows_test.pb.go]` (scratch `rev6/fx_suffix_test`, `rev6/valid`) | everything resolves; `Warnings: 0`; `--strict` exit 0 | unchanged: byte-identical output (`rev6/fixed`). Revision 5's helper regressed it: `Warnings: 2`, `--strict` exit 1, aliased `bb.Addr` and the requestBody became `{wrong}`, `created` was lost silently, and unaliased `b.Addr` and `bb.Cents` fell back (mutation M21) | no |
| R15 | file-name suffix, stand-in first: `b/a_windows.go` (`package bwin`, no build line) sorts before an unconstrained `b/z.go` (`package b`), with or without its own `Addr{Wrong}` (scratch `rev4/fx/e1c`, `e1d`) | unaliased `b.Addr` and `bb.Cents` fall back + warning, `Warnings: 2`. With the stand-in `Addr`, aliased `bb.Addr` is `{wrong}` (silent) | everything resolves to `b`'s declarations; `Warnings: 0`. Revision 3 regressed `e1d` to `Warnings: 4` (aliased structs lost) | no |
| R16 | **regression of the body**, every importable file constrained, a never-built file of the package's own clause sorted first: `b/_old.go` (`package b`, no declarations), `b/aaa_tools.go` (`//go:build tools`, `package tools`), `b/b.go` (`//go:build !plan9`, `package b`; `Addr`, `Base`, `Cents`). Module: embed `b.Base`, fields `b.Addr`, `b.Cents`, `bb.Addr`, `bb.Cents`, request `req b.Addr` (scratch `advrev/x1`; `go list`: `b [b.go] ign=[aaa_tools.go]`) | everything resolves: requestBody `$ref: …/Addr`, `created` promoted, `ucents`/`cents` `{type: integer, format: int64}`; `Warnings: 0`; `--strict` exit 0 | `packageClause` skips `_old.go` and takes `tools`, not sure. The unaliased import is named `tools`, so no requestBody (non-struct request warning), `created` lost silently, `unaliased` and `ucents` `{type: object}` + warning; `inModuleTypeSite` takes `tools` too, so aliased `cents` `{type: object}` + warning. Aliased `bb.Addr` still `$ref`. `Warnings: 4`; `--strict` exit 1. Appendix B's (b): `Warnings: 5`, aliased `bb.Addr` lost too | **yes** (new) |
| R16 | same, with `b/a_gen.go` (`//go:build ignore`, `package b`) in place of `_old.go` and `b/tools.go`, `b/types.go` in place of `aaa_tools.go`, `b.go` (scratch `advrev/x2`; `go list`: `b [b.go] ign=[a_gen.go tools.go]`) | structs, requestBody and `created` resolve; `ucents` and `cents` already fall back through PR 129's `inModuleTypeSite`; `Warnings: 2`; `--strict` exit 1 | requestBody and `created` lost (silent), `unaliased` `{type: object}` + warning; `Warnings: 4`. Appendix B's (b): `Warnings: 5` | **yes** (new) |
| R16 | same as `x1`, with `b/.old.go` in place of `_old.go` (scratch `beh8/fx/x1dot`), or with `_old.go`, then `b/a_windows.go` (`package bwin`, no build line) in place of `aaa_tools.go`, then `b/b.go` under `//go:build !windows` (scratch `beh8/fx/x1win`). Valid Go (`beh8/valid`): `x1dot` gives `b go=[b.go] ign=[aaa_tools.go]` on linux, windows and darwin; `x1win` gives `b go=[b.go]` on linux and darwin, `bwin go=[a_windows.go]` on windows | everything resolves; `Warnings: 0`; `--strict` exit 0 | as `x1`: no requestBody (non-struct request warning), unaliased `b.Addr` and `b.Cents` and aliased `bb.Cents` `{type: object}` + warning; `Warnings: 4`; `--strict` exit 1 (revision 8, `beh8/run.sh`) | **yes** (new) |
| R16 | the `x1` layout under `pkg/b`, plus a real, unconstrained `internal/tools` (`package tools`, `Widget{Name}`) imported unaliased **before** `pkg/b` in the same file (goimports order). Fields `W tools.Widget`, `A b.Addr`, request `req tools.Widget` (scratch `regrev8/fx/coll`; valid Go, `regrev8/valid_coll`: `…/internal/tools tools go=[tools.go]`, `…/pkg/b b go=[b.go] ign=[aaa_tools.go]`) | everything resolves: requestBody `$ref: …/Widget`, components `Addr{street}` and `Widget{name}`; `Warnings: 0`; `--strict` exit 0 | `fileImports` keys `pkg/b` as `tools` too, and that later write replaces `internal/tools`'s entry (`analyzer.go:3659`, the overwrite rule is unchanged), so `tools.Widget` is looked up in `pkg/b`: no requestBody (non-struct request warning), `W` and `A` `{type: object}` + warning, both components gone; `Warnings: 3`; `--strict` exit 1. With `pkg/b` imported first, only `A b.Addr` breaks (`Warnings: 1`, `rev8/coll_rev`). The same collision already happens on `main` in the R13 layout: without `_old.go`, `033a9c9` and the plan both give these three warnings (`rev8/coll_r13`). R16 adds one more layout that reaches it | **yes** (new) |
| R17 | generator sorted between two real files: `b/api.go` (`package b`, `Base`, `Cents`), `b/gen.go` (`//go:build ignore`, `package main`, its own `Addr{Wrong}`), `b/types.go` (`package b`, `Addr{Street}`). The unaliased import is already named `b` on `main`, because `api.go` sorts first (scratch `behav/fx/mid`) | `Addr: {wrong: int}`, the generator's (silent); `Warnings: 0` | `Addr: {street: string}`; `Warnings: 0`. A silent output change with no sorted-first stray file | no |

Under the adopted defaults as written (revision 2's body), the R11-R13 and R16 layouts regress. Appendix B gives the numbers. Under the body, only R16 does.

Unchanged and pinned:
- The `pkgname_mismatch` golden: an unaliased import of `transport/httpapi` declaring `package http` is keyed `http`.
- `qualified_named_types/b/gen.go`: it is `package main`, sorts **after** `b.go`, and declares only a scalar.
- A `//go:build ignore` file sharing the package's clause is still merged (README says so; the same as `inModuleTypeSite`).

`marshaler.go:152` (`visitMarshalerMethods` → `matchMarshalerSignature`) reads `fileImports` only to match stdlib types in method signatures (`*jsontext.Encoder`, …). Those are not in-module, so its behavior is unchanged in practice.

---

## Data model

**None.** `internal/models` is untouched, and no analyzer-private state changes. `qualifiedStruct.pkg` stays `file.Name.Name`, the matched file's clause, as on `main`. When the clause is sure, it always equals that clause, because only files of that clause are searched.

---

## Design, with every helper's degenerate input checked

### Helpers reused as is (checked at `033a9c9`)

| Helper (anchor) | Behavior relied on | Degenerate inputs (read in code) |
|---|---|---|
| `buildIgnored(f)` (`resolution.go:362`) | true iff the file's first `//go:build`/`// +build` line before `package` holds with every tag set but not with `ignore` alone unset | no constraint → false; unparsable constraint → false (`fileBuildConstraint` returns nil); `ignore \|\| linux` → false; `ignore && linux` → true (pinned by `TestBuildIgnored`) |
| `fileBuildConstraint(f)` (`resolution.go:372`) | the first `//go:build` (or `// +build`) line before the package clause, parsed | none → nil. An unparsable line also gives nil, so `packageClause` counts that file as unconstrained unless its name is suffixed; on valid code this cannot happen, because `go build` rejects such a line. It never reads a GOOS/GOARCH file-name suffix (`x_linux.go`); `packageClause` reads that through `osArchSuffixed`. A suffixed file of another clause is valid Go when every file of the real clause excludes that platform (R15) |
| `parsePackageDir(dir)` (`analyzer.go:3690`) | cached map path→file of the directory's non-test `.go` files; the keys are full paths in one directory, so their sorted order is the file-name order, and `filepath.Base` gives the name | unreadable/missing dir → `(nil, err)`; empty dir → empty map, no error; `_test.go` never read; `_…` and `.…` files **are** read (only `neverImportable` skips them); files escaping containment or failing to parse skipped. Every kept file has a non-nil `Name` (`go/parser` sets it on any file that parses) |
| `inModuleDir(path)` (`analyzer.go:3621`) | in-module import path → directory under the project root | no module path, external path, or a traversal escaping the root → `("", false)`; an in-module path with no directory → `(dir, true)` (then `parsePackageDir` errors) |

### `importableClause`, `packageClause`, `neverImportable`, `osArchSuffixed`, `inPackage` (`resolution.go:346-357`, replaced)

Replace `importableClause`'s doc comment and body (`:346-357`) with:

```go
// importableClause returns the package clause of the package a directory's
// files build (packageClause, without its certainty). ok is false when no
// file qualifies.
func importableClause(files map[string]*ast.File) (string, bool) {
	clause, ok, _ := packageClause(files)
	return clause, ok
}

// packageClause returns the package clause of the package a directory's files
// build. Files the go command never builds into an importable package
// (neverImportable) are skipped. Of the rest, a file with no build constraint
// (no //go:build line and no GOOS/GOARCH file-name suffix, osArchSuffixed) is
// in every build, so on valid code its clause is the package's: the first
// such file in sorted path order names it, and sure is true. When every
// remaining file is constrained, the first that is not build-ignored names it
// (a //go:build ignore generator or tool, whatever its clause, never does),
// and sure is false: a file of another clause under a constraint other than
// ignore (//go:build tools) may have been taken for the package. ok is false
// when no file qualifies.
func packageClause(files map[string]*ast.File) (clause string, ok, sure bool) {
	for _, p := range slices.Sorted(maps.Keys(files)) {
		f := files[p]
		if neverImportable(p, f) {
			continue
		}
		if fileBuildConstraint(f) == nil && !osArchSuffixed(filepath.Base(p)) {
			return f.Name.Name, true, true
		}
		if !ok && !buildIgnored(f) {
			clause, ok = f.Name.Name, true
		}
	}
	return clause, ok, false
}

// documentationPackageName is the package clause go/build never builds,
// whatever the tags: a file declaring it documents a directory and is no
// part of its package.
const documentationPackageName = "documentation"

// neverImportable reports whether the go command never builds the file at
// path into an importable package, whatever the tags: package main, package
// documentation, or a file whose name starts with _ or . (go/build skips
// both as editor or temporary files).
func neverImportable(path string, f *ast.File) bool {
	base := filepath.Base(path)
	return f.Name.Name == mainPackageName || f.Name.Name == documentationPackageName ||
		strings.HasPrefix(base, "_") || strings.HasPrefix(base, ".")
}

// knownOS and knownArch are go/build's GOOS and GOARCH values for file-name
// matching (internal/syslist, which is not importable): past, present and
// future ports, never removed. unix is a build tag only, not a file suffix.
var (
	knownOS = map[string]bool{
		"aix": true, "android": true, "darwin": true, "dragonfly": true, "freebsd": true, "hurd": true,
		"illumos": true, "ios": true, "js": true, "linux": true, "nacl": true, "netbsd": true,
		"openbsd": true, "plan9": true, "solaris": true, "wasip1": true, "windows": true, "zos": true,
	}
	knownArch = map[string]bool{
		"386": true, "amd64": true, "amd64p32": true, "arm": true, "armbe": true, "arm64": true,
		"arm64be": true, "loong64": true, "mips": true, "mipsle": true, "mips64": true, "mips64le": true,
		"mips64p32": true, "mips64p32le": true, "ppc": true, "ppc64": true, "ppc64le": true, "riscv": true,
		"riscv64": true, "s390": true, "s390x": true, "sparc": true, "sparc64": true, "wasm": true,
	}
)

// osArchSuffixed reports whether the file name base carries go/build's
// implicit build constraint (goodOSArchFile): cut at its first dot and with a
// trailing _test element dropped, the element after its last underscore is a
// known GOOS or GOARCH (x_linux.go, x_amd64.go, x_linux_amd64.go,
// x_windows_test.pb.go; not linux.go or x_unix.go). Only its presence is read,
// never which platforms it selects. parsePackageDir skips only names ending in
// _test.go, so it does read a name such as x_windows_test.pb.go, which
// go/build constrains.
func osArchSuffixed(base string) bool {
	name, _, _ := strings.Cut(base, ".")
	name = strings.TrimSuffix(name, "_test")
	i := strings.LastIndex(name, "_")
	if i < 0 {
		return false
	}
	suffix := name[i+1:]
	return knownOS[suffix] || knownArch[suffix]
}

// inPackage reports whether resolveQualifiedStruct searches the file at path
// for a struct of the package packageClause named. A file of that clause is
// always searched. When the clause is sure (an unconstrained file named it),
// no other file is. When it is not, any other file the go command can build
// (not neverImportable, not build-ignored) is searched too: a sorted-first
// file of another clause under a constraint other than ignore may have named
// the package.
func inPackage(path string, f *ast.File, clause string, sure bool) bool {
	if f.Name.Name == clause {
		return true
	}
	return !sure && !neverImportable(path, f) && !buildIgnored(f)
}
```

`resolution.go` already imports `path/filepath` and `strings`. The two maps copy `$GOROOT/src/internal/syslist/syslist.go` (`KnownOS`, `KnownArch`) at Go 1.27.2. go/build exports no way to read them, and `internal/syslist` cannot be imported. A port added to Go after 1.27.2 is missing from the copy, so a file suffixed with it counts as unconstrained, and the R15 layout can recur for that port alone (Residuals). `osArchSuffixed` matches go/build's `goodOSArchFile` (`go/build/build.go:1995`) on every name `parsePackageDir` returns, because it drops a trailing `_test` element first, as `goodOSArchFile` does (`build.go:2012-2014`). `parsePackageDir` skips only names ending in `_test.go`, so it reads `z_windows_test.pb.go`, which go/build builds only on Windows; revision 5's helper read no suffix there and brought R15 back (`rev6/fx_suffix_test`). Measured: 0 disagreements with `build.Context.MatchFile` over 209 generated names, against 32 without the strip (`rev6/brute`). When the last two elements are a GOOS and a GOARCH, the last one is a GOARCH anyway, so checking only the last element is enough.

| Directory (sorted) | `importableClause` before | `packageClause` after |
|---|---|---|
| empty | `("", false)` | `("", false, false)` |
| only `package main`, `package documentation`, `_…` or `.…` files | `("", false)` for `package main`; else that file's clause | `("", false, false)` |
| only build-ignored files | `("", false)` | `("", false, false)` |
| `aaa_gen.go` (`package main`), `width.go` (`package w`) | `w` | `w`, sure |
| `aaa_tools.go` (`//go:build tools`, `package tools`), `b.go` (`package b`) | `tools` | `b`, sure |
| `_tools.go` / `.tools.go` (`package tools`) or `doc.go` (`package documentation`), then `types.go` (`package b`) | `tools` / `documentation` | `b`, sure |
| only tagged variants (`width_386.go`, `width_amd64.go`, both `package w`) | `w` | `w`, not sure |
| `aaa_tools.go` (`//go:build tools`, `package tools`), `b.go` (`//go:build !plan9`, `package b`) | `tools` | `tools`, not sure (R13 residual) |
| `a.go` (`//go:build !windows`, `package b`), `z_windows.go` (`package bwin`, no build line) | `b` | `b`, not sure (R15; revision 3: `bwin`, sure) |
| `a_windows.go` (`package bwin`, no build line), `z.go` (`package b`) | `bwin` | `b`, sure (R15) |
| `aaa_tools.go` (`//go:build tools`, `package tools`), `b_linux.go` (`package b`, no build line), `b_other.go` (`//go:build !linux`, `package b`) | `tools` | `tools`, not sure (R13 residual; revision 3: `b`, sure) |
| `_old.go` (`package b`), `aaa_tools.go` (`//go:build tools`, `package tools`), `b.go` (`//go:build !plan9`, `package b`) | `b` | `tools`, not sure (**R16 regression**; `unaliasedImportName` on `main` also said `b`) |
| `a_gen.go` (`//go:build ignore`, `package b`), `tools.go` (`//go:build tools`, `package tools`), `types.go` (`//go:build !plan9`, `package b`) | `tools` | `tools`, not sure (**R16 regression** through `unaliasedImportName`, which on `main` said `b`) |

`inPackage(path, f, clause, sure)`:

| File | sure | Searched |
|---|---|---|
| of the clause (including a same-clause build-ignored or `_…` file) | either | yes |
| of another clause | yes | no |
| of another clause, neither `neverImportable` nor build-ignored (`//go:build tools`) | no | yes, as on `main` |
| of another clause, `neverImportable` or build-ignored | no | no (R14; pinned by M13, M14) |

Cost: the loop sorts the keys, as the old one did. It runs `fileBuildConstraint` and `osArchSuffixed` (a string cut and two map lookups) on files up to the first unconstrained one, normally the first file, and `fileBuildConstraint` stops at the package clause. `inPackage` is a string compare in the common, sure case. There is no measurable change.

### `unaliasedImportName` (`analyzer.go:3664-3686`)

Replace the first-file loop with `importableClause`, and keep every fallback.

| Input | Before | After |
|---|---|---|
| external/stdlib (`encoding/json`) | `json` | `json` |
| in-module, no directory (`…/nope`) | `nope` | `nope` |
| first file `package main` generator, then `package w` (dir `width`) | `main` | `w` |
| first file `//go:build ignore` `package tool`, then `package x` | `tool` | `x` |
| first file `//go:build tools` `package tools`, then unconstrained `package y` | `tools` | `y` |
| `.a.go` (`package dot`), `_b.go` (`package under`), `doc.go` (`package documentation`), `types.go` (`package real`) | `dot` | `real` |
| `//go:build tools` `package tools`, then `//go:build linux` `package z` | `tools` | `tools` (R13 residual) |
| `_old.go` (`package b`), then `//go:build tools` `package tools`, then `//go:build !plan9` `package b` | `b` | `tools` (**R16 regression**, not in `TestUnaliasedImportName`; pinned by Task 1 Step 1b) |
| `//go:build ignore` `package b`, then `//go:build tools` `package tools`, then `//go:build !plan9` `package b` | `b` | `tools` (**R16 regression**; Task 1 Step 1b) |
| only a `package main` generator (`genonly`) | `main` | `genonly` |
| only a `//go:build ignore` file of clause `s` (`ignored`) | `s` | `ignored` |
| `transport/httpapi` declaring `package http` | `http` | `http` |

The `genonly` and `ignored` fallbacks name a directory that Go itself cannot import (no buildable file), so they are not observable on valid code. Every row is in `TestUnaliasedImportName` (Task 1).

```go
// unaliasedImportName returns the local name an unaliased import is referenced by:
// for an in-module package, the package clause of its importable files
// (importableClause), so a package main generator or a build-ignored tool that
// sorts first in the directory never renames it; else filepath.Base(path), for
// external/stdlib imports, an unreadable directory, or one with no importable
// file. Uses the same cached helpers as resolveQualifiedStruct (inModuleDir +
// parsePackageDir), so it adds no new parse cost on the hot path.
func (a *ProjectAnalyzer) unaliasedImportName(path string) string {
	dir, ok := a.inModuleDir(path)
	if !ok {
		return filepath.Base(path)
	}
	files, err := a.parsePackageDir(dir)
	if err != nil {
		return filepath.Base(path)
	}
	if clause, ok := importableClause(files); ok {
		return clause
	}
	return filepath.Base(path)
}
```

`slices`/`maps` stay imported in `analyzer.go`, because other users remain.

Also reword `fileImports`'s doc comment (`analyzer.go:3640-3647`) so it names `unaliasedImportName` instead of "We resolve the declared name":

```go
// fileImports maps each import's local alias to its import path. An explicit alias
// (e.g. `foo "path/bar"`) is used verbatim. For an unaliased import, Go references
// the package by its DECLARED `package` clause name, which is not always the path's
// last segment (e.g. `import "transport/httpapi"` whose files say `package http`).
// unaliasedImportName resolves that name from the already-parsed in-module package
// (cached, no new parse on the hot path) and falls back to the path base for
// external/stdlib imports whose declared name is unknowable here. Blank (_) and dot
// (.) imports are skipped.
```

### `resolveQualifiedStruct` (`analyzer.go:3535-3566`)

When the clause is sure, the loop reads only the files of that clause, exactly as `inModuleTypeSite` (`resolution.go:334-343`) does. That fixes R7, R10, and R12's stand-ins. When the clause is not sure, the loop reads every file the go command can build, as `main` reads every file, so R13 cannot regress, while a never-built stand-in of another clause is skipped (R14). A same-clause build-ignored file stays searchable either way (consistent with `inModuleTypeSite` and the README). The returned `pkg` stays the matched file's clause.

```go
// resolveQualifiedStruct resolves a pkg.Type reference against astFile's imports.
// It parses the in-module target package (with a per-dir cache) and finds the
// named struct among the files of its package (packageClause, inPackage).
// Returns ok=false for stdlib/third-party imports, unknown aliases, a directory
// with no importable file, or names that are not a struct in the target package.
func (a *ProjectAnalyzer) resolveQualifiedStruct(qualified string, astFile *ast.File) (qualifiedStruct, bool) {
	// ... unchanged through `files, err := a.parsePackageDir(dir)` and its err check ...
	clause, ok, sure := packageClause(files)
	if !ok {
		return qualifiedStruct{}, false // no importable file: only generators and tools
	}
	// Search only the package's files (inPackage: a package main generator's
	// struct is never the package's), in a stable order so the resolved
	// definition is deterministic even when several files in the dir could
	// match (e.g. build-tagged variants).
	for _, path := range slices.Sorted(maps.Keys(files)) {
		file := files[path]
		if !inPackage(path, file, clause, sure) {
			continue
		}
		if st := a.findStructInFile(file, typeName); st != nil {
			return qualifiedStruct{typeName: typeName, pkg: file.Name.Name, st: st, file: file, filePath: path}, true
		}
	}
	return qualifiedStruct{}, false // not a struct in the target package (alias/interface)
}
```

The `!ok` early return is **equivalent by construction**. When `ok` is false, every file is `neverImportable` or build-ignored, `clause` is `""` and `sure` is false, so `inPackage` rejects every file. Mutation M4 therefore survives (Task 7). The return is kept to mirror `inModuleTypeSite` and to state the case; see open decision 4.

Consumers that inherit the fix with no edit:
- `registerQualifiedTypeAt` (`analyzer.go:3577`), which covers field refs, `registerTypeAt`'s qualified branches at `:3346`/`:3354`, and `registerPayloadType` at `:3212`;
- `resolveTypeSpecChain`'s `type X = q.T` branch (`:3440`);
- embedded-struct promotion (`:4115`);
- `resolveDelegateContext` (`:1674`, row R9);
- `payloadStructSite` (`payload.go:221`);
- `resolveQualified` (`resolution.go:296`).

### Warning text (`resolution.go:36-39`)

This section follows open decision 8. No existing test asserts the first-file clause (every test matches `"… resolves to no schema"`), and no README line quotes it. The fix does not remove its cause: under (a), R13 and R16 still name the package after a file of another clause, so their unaliased references and their named non-struct types (aliased `bb.Cents` included) still emit this warning. Measured on `gates5/r13` (R13): `main`'s binary and `rev6/fixed8` both print `Warnings: 4`, and the three field warnings (`Unaliased`, `UCents`, `Cents`) carry this text. With the clause dropped, none of the causes the warning lists is true for them.

**Decision 8 default (recommended, needs the user's word): reword the clause to the residual.** The pin goes in Task 4.

```go
	unresolvableFieldWarning = "field %s at %s has type %s: %s resolves to no schema (a type from outside the module, " +
		"a well-known type under an aliased import, an in-module package whose buildable files all carry a build constraint " +
		"and whose first one by name has another package clause, or a name with no declaration in its package) — emitting an untyped object"
```

**Decision 8 alternative, and the default when the user's reply does not answer decision 8: drop the clause**, as the triage comment's adopted default says. No pin is added.

```go
	unresolvableFieldWarning = "field %s at %s has type %s: %s resolves to no schema (a type from outside the module, " +
		"a well-known type under an aliased import, or a name with no declaration in its package) — emitting an untyped object"
```

Under decision 6 (b), the default's clause loses "whose buildable files all carry a build constraint and" (the first buildable file names the package whether or not it is constrained). Under (a'), the later plan revision that prototypes (a') supplies it.

---

## Goldens

**No existing golden moves.** Only one fixture has a non-importable file in an imported directory: `qualified_named_types/b/gen.go`. It sorts after `b.go` and declares only `type Cents int32`, which `inModuleTypeSite` already skipped. No other fixture has a `//go:build` line in an imported directory, a `package main` or `package documentation` file, or a `_…`/`.…` Go file (`grep -rln 'go:build\|^package main\|^package documentation' internal/spectest/testdata`; `find internal/spectest/testdata -name '_*.go' -o -name '.*.go'`). `pkgname_mismatch` stays green, because its only file is importable.

### New fixture `internal/spectest/testdata/importable_clause/` (`package importableclause`)

The fixture follows CLAUDE.md:
- `go.mod` declares a `github.com/example/` module, `go 1.25`, and `require github.com/gaborage/go-bricks v0.53.0`, with no `go.sum` and no `replace`.
- The underscore directory takes an underscore-free package name.

The fixture is intentionally not `--strict`-clean. `b.Money`'s Marshaler warning is the correct outcome, and it proves the Marshaler path is reached through the unaliased import (open decision 5).

`go.mod`:

```
module github.com/example/importableclause

go 1.25

require github.com/gaborage/go-bricks v0.53.0
```

`b/aaa_gen.go`:

```go
//go:build ignore

// A generator program that sorts first in b. Its package main clause neither
// names b's import nor contributes a declaration to b: its Addr is not b's.
package main

// Addr is the generator's own type.
type Addr struct {
	Wrong int `json:"wrong"`
}

func main() {}
```

`b/b.go`:

```go
// Package b is imported unaliased (and once aliased) by the module. A
// //go:build ignore generator in package main sorts first in this directory.
package b

// Addr is a struct: a component.
type Addr struct {
	Street string `json:"street"`
}

// Base is embedded by value: its fields are promoted.
type Base struct {
	Created string `json:"created"`
}

// Cents is a named scalar.
type Cents int64

// Tags is a named slice.
type Tags []string

// Level is written and read as text: a string.
type Level int

// MarshalText writes the level.
func (l Level) MarshalText() ([]byte, error) { return []byte("high"), nil }

// UnmarshalText reads the level.
func (l *Level) UnmarshalText(b []byte) error { return nil }

// Money is written through its own MarshalJSON: untyped.
type Money struct {
	Amount int64 `json:"amount"`
}

// MarshalJSON writes the money.
func (m Money) MarshalJSON() ([]byte, error) { return nil, nil }
```

`module.go`:

```go
// Package importableclause pins that an in-module package is named after its
// importable files' package clause: b's first file by name is a //go:build
// ignore generator in package main, which neither renames the unaliased import
// nor lends b a declaration.
package importableclause

import (
	"github.com/example/importableclause/b"
	bb "github.com/example/importableclause/b"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "importable" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// WrapL promotes b.Level's text pair: a string both ways.
type WrapL struct {
	b.Level
}

// Embeds promotes b.Base's and b.Addr's fields.
type Embeds struct {
	b.Base
	*b.Addr
	Note string `json:"note"`
}

// Rows holds one field per qualified position.
type Rows struct {
	Addr     b.Addr            `json:"addr"`
	AddrList []b.Addr          `json:"addrList"`
	AddrPtr  *b.Addr           `json:"addrPtr"`
	AddrMap  map[string]b.Addr `json:"addrMap"`
	Cents    b.Cents           `json:"cents"`
	Tags     b.Tags            `json:"tags"`
	Level    b.Level           `json:"level"`
	Money    b.Money           `json:"money"`
	Wrap     WrapL             `json:"wrap"`
	Embeds   Embeds            `json:"embeds"`
	Aliased  bb.Addr           `json:"aliased"`
}

func (m *Module) rows(ctx server.HandlerContext) (server.Result[Rows], server.IAPIError) {
	return server.Result[Rows]{}, nil
}

func (m *Module) getAddr(ctx server.HandlerContext) (server.Result[b.Addr], server.IAPIError) {
	return server.Result[b.Addr]{}, nil
}

func (m *Module) createAddr(req b.Addr, ctx server.HandlerContext) (server.Result[b.Cents], server.IAPIError) {
	return server.Result[b.Cents]{}, nil
}

func (m *Module) wrap(ctx server.HandlerContext) (server.Result[WrapL], server.IAPIError) {
	return server.Result[WrapL]{}, nil
}

func (m *Module) aliased(ctx server.HandlerContext) (server.Result[bb.Addr], server.IAPIError) {
	return server.Result[bb.Addr]{}, nil
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/rows", m.rows)
	server.GET(hr, r, "/addr", m.getAddr)
	server.POST(hr, r, "/addr", m.createAddr)
	server.GET(hr, r, "/wrap", m.wrap)
	server.GET(hr, r, "/aliased", m.aliased)
}
```

The expected golden is 297 lines, generated by `-update`; review it against this description:
- Five operations on four paths: `/addr` (get and post), `/aliased`, `/rows`, `/wrap`.
- `GET /addr` and `GET /aliased`: `data: {$ref: '#/components/schemas/Addr'}`.
- `POST /addr`: `requestBody: {required: true, content: {application/json: {schema: {$ref: …/Addr}}}}` and `data: {type: integer, format: int64}`.
- `GET /wrap`: `data: {type: string}`.
- `GET /rows`: `data: {$ref: …/Rows}`.
- Components exactly `Addr`, `Embeds`, `ErrorResponse`, `Rows` (no `WrapL`):

```yaml
    Addr:
      type: object
      properties:
        street:
          type: string
    Embeds:
      type: object
      properties:
        created:
          type: string
        note:
          type: string
        street:
          type: string
    # ErrorResponse: unchanged boilerplate
    Rows:
      type: object
      properties:
        addr:
          $ref: '#/components/schemas/Addr'
        addrList:
          type: array
          items:
            $ref: '#/components/schemas/Addr'
        addrMap:
          type: object
          additionalProperties:
            $ref: '#/components/schemas/Addr'
        addrPtr:
          type: object
          allOf:
            - $ref: '#/components/schemas/Addr'
          nullable: true
        aliased:
          $ref: '#/components/schemas/Addr'
        cents:
          type: integer
          format: int64
        embeds:
          $ref: '#/components/schemas/Embeds'
        level:
          type: string
        money: {}
        tags:
          type: array
          items:
            type: string
        wrap:
          type: string
```

Any of these means the fix is incomplete: a `wrong:` property, a `WrapL` component, a `type: object` leaf under `Rows` (other than `addrPtr`'s wrapper), or a missing `requestBody`.

---

## Cognitive-complexity budget (gocognit, measured on `033a9c9`; "After" = prototype)

| Function (anchor at `033a9c9`) | Before | After |
|---|---|---|
| `unaliasedImportName` (`analyzer.go:3669`) | 6 | 3 |
| `resolveQualifiedStruct` (`analyzer.go:3539`) | 7 | 10 |
| `fileImports` (`analyzer.go:3648`, doc comment only) | 7 | 7 |
| `importableClause` (`resolution.go:350`) | 4 | 0 (wrapper) |
| `packageClause` (new) | — | 9 |
| `osArchSuffixed` (new) | — | 2 |
| `inPackage` (new) | — | 2 |
| `neverImportable` (new) | — | 1 |
| `inModuleTypeSite` (`resolution.go:323`, untouched) | 7 | 7 |
| `parsePackageDir` (`analyzer.go:3690`, untouched) | 12 | 12 |
| test helper `runWidthCases` (new) | — | 4 |
| `TestUnaliasedImportName`, `TestQualifiedStructImportablePackage`, `TestQualifiedToolsFileSortsFirst`, `TestQualifiedAllConstrainedToolsFirst`, `TestQualifiedAllConstrainedStandIns`, `TestOSArchSuffixed` (new) | — | 1 each |

All are ≤15. `gocognit -over 15 internal/analyzer/analyzer.go internal/analyzer/resolution.go internal/analyzer/qualified_test.go` exits 0.

## Coverage strategy

The changed production lines are:
- `unaliasedImportName`'s `importableClause` branch and its final fallback;
- `resolveQualifiedStruct`'s `!ok` return, `continue` and return;
- every branch of `packageClause` (skip, sure return, fallback), `neverImportable`, `osArchSuffixed` (no underscore, suffix read) and `inPackage` (same clause, other clause sure, other clause not sure).

The new tests execute each of them. Line coverage alone did not pin `inPackage`'s not-sure branch: in revision 3 only `TestQualifiedAllConstrainedToolsFirst` reached it, and its tools file declares nothing, so neither guard decided a result. `TestQualifiedAllConstrainedStandIns` gives each guard a row whose stand-in declares its own `Addr` (M13-M15). The prototype's analyzer profile reports 100.0% for `resolveQualifiedStruct`, `unaliasedImportName`, `importableClause`, `packageClause`, `neverImportable`, `osArchSuffixed`, `inPackage` and `inModuleTypeSite`. Spot-check in Task 7 with the analyzer package's own profile, no `-coverpkg`.

---

## Task 0: Branch and baseline

- [ ] **Step 1:** `cd /Users/gaborage/Projects/gaborage/code/go-bricks-openapi && git fetch -q origin && git switch -c fix/importable-package-clause origin/main` (or the worktree the orchestrator designates). Confirm `git rev-parse --short HEAD` is `033a9c9`, or re-anchor every line below if `main` moved.
- [ ] **Step 2:** Baseline: `go test ./internal/analyzer ./internal/spectest` green. `gocognit internal/analyzer/analyzer.go internal/analyzer/resolution.go | grep -E 'unaliasedImportName|resolveQualifiedStruct|importableClause'` shows 6, 7 and 4.

## Task 1: Failing tests first (TDD red)

**Files:** Modify `internal/analyzer/qualified_test.go`. Replace `TestQualifiedBuildTaggedVariants`, from its doc comment through its closing brace (`:259-304`), with the block below. It sits right before `TestBuildIgnored`.

- [ ] **Step 0 (gate):** open decision 6 must have the maintainer's **written** answer. Do not proceed without one, because these tests encode it.
  - For (a), the body, continue as written, with Step 1b's (a) assertions and Task 8's `fix(analyzer)!:` message.
  - For (a'), stop. The plan needs a later revision (revision 6 did not touch (a')) that prototypes the changed fallback in the scratchpad, measures it on every scratch fixture (`behav/fx`, `advrev/x1`, `x2`, `rev4/fx`, `gosem4/fx`) against both of `main`'s rules (`unaliasedImportName`'s first file and PR 129's `importableClause`, which differ), and gets a review round.
  - For (b), apply Appendix B's test deltas to the block below here, its code deltas in Task 3, and Step 1b's (b) assertions.
  - For anything else, including an answer to revision 4's "(a1+): no measured regression" wording, stop and ask for a revised plan.
  - Record decision 8 from the same reply for Task 4: keep the clause reworded (the default) only if the reply says so; otherwise drop it (the alternative). Decision 8 never blocks.
- [ ] **Step 1:** Replace `TestQualifiedBuildTaggedVariants` and its doc comment with this block. The aliased test keeps its original four cases through `runWidthCases` and gains four, and its doc no longer mentions "#122 is not exercised":

```go
// widthCase is one layout of package w's files for the build-tagged variant
// tests, with the rendered leaf every w.Width position resolves to.
type widthCase struct {
	name  string
	files map[string]string
	leaf  string
}

// widthCases are the layouts of the build-tagged variant tests: Width in two
// build-tagged files of w, and a file of another clause that w never builds
// sorting FIRST beside it: a //go:build ignore generator in package main, a
// //go:build ignore or //go:build tools file, a file named _ or . first, or a
// package documentation file.
func widthCases() []widthCase {
	variant := func(constraint, decl string) string {
		return "//go:build " + constraint + "\n\npackage w\n\n" + decl + "\n"
	}
	return []widthCase{
		{"widths disagree", map[string]string{"width_amd64.go": variant("amd64", "type Width int64"), "width_386.go": variant("386", "type Width int32")}, "kind:" + kindInteger},
		{"widths agree", map[string]string{"width_amd64.go": variant("amd64", "type Width int64"), "width_arm64.go": variant("arm64", "type Width int64")}, goTypeInt64},
		{"a package main generator sorts first", map[string]string{
			"aaa_gen.go": "//go:build ignore\n\npackage main\n\ntype Width string\n\nfunc main() {}\n",
			"width.go":   "package w\n\ntype Width int64\n",
		}, goTypeInt64},
		{"a build-ignored tool of another clause sorts first", map[string]string{
			"aaa_tool.go": "//go:build ignore\n\npackage tool\n\ntype Width string\n",
			"width.go":    "package w\n\ntype Width int64\n",
		}, goTypeInt64},
		{"a tools-tagged file of another clause sorts first", map[string]string{
			"aaa_tools.go": "//go:build tools\n\npackage tools\n\ntype Width string\n",
			"width.go":     "package w\n\ntype Width int64\n",
		}, goTypeInt64},
		{"an underscore-named file of another clause sorts first", map[string]string{
			"_tool.go": "package tool\n\ntype Width string\n",
			"width.go": "package w\n\ntype Width int64\n",
		}, goTypeInt64},
		{"a dot-named file of another clause sorts first", map[string]string{
			".tool.go": "package tool\n\ntype Width string\n",
			"width.go": "package w\n\ntype Width int64\n",
		}, goTypeInt64},
		{"a package documentation file sorts first", map[string]string{
			"doc.go":   "package documentation\n\ntype Width string\n",
			"width.go": "package w\n\ntype Width int64\n",
		}, goTypeInt64},
	}
}

// runWidthCases analyzes each widthCase with w's files in dir, imported by
// importLine and referenced as qual.Width directly, as slice items and as a
// map value.
func runWidthCases(t *testing.T, dir, importLine, qual string) {
	t.Helper()
	for _, c := range widthCases() {
		t.Run(c.name, func(t *testing.T) {
			files := map[string]string{
				"go.mod": resolveGoMod,
				filepath.Join("mod", "module.go"): rowsModule(importLine, "",
					"\tW "+qual+".Width `json:\"w\"`\n\tWS []"+qual+".Width `json:\"ws\"`\n\tWM map[string]"+qual+".Width `json:\"wm\"`\n"),
			}
			for name, src := range c.files {
				files[filepath.Join(dir, name)] = src
			}
			a := analyzeDirectiveProject(t, files)
			got := renders(t, a.typeRegistry["Rows"])
			assert.Equal(t, c.leaf, got["w"])
			assert.Equal(t, "[]"+c.leaf, got["ws"])
			assert.Equal(t, "map[string]"+c.leaf, got["wm"])
			assert.Empty(t, a.Warnings(t.Context()))
		})
	}
}

// TestQualifiedBuildTaggedVariants pins that every declaration of a
// qualified name in its package is merged, as for a local name (#92), and
// that a generator or tool sorting first never contributes one, under an
// aliased import.
func TestQualifiedBuildTaggedVariants(t *testing.T) {
	runWidthCases(t, "w", "\tww \"github.com/example/app/w\"\n", "ww")
}

// TestQualifiedBuildTaggedVariantsUnaliased is the unaliased twin (#122): an
// unaliased import of a directory (width) whose base is not its package
// clause (w) is named after its importable files' clause, so a generator or
// tool sorting first neither renames the import nor lends a declaration.
func TestQualifiedBuildTaggedVariantsUnaliased(t *testing.T) {
	runWidthCases(t, "width", "\t\"github.com/example/app/width\"\n", "w")
}

// TestUnaliasedImportName pins the name of an unaliased import: its
// directory's importable clause, else the import path's last element.
func TestUnaliasedImportName(t *testing.T) {
	gen := "//go:build ignore\n\npackage main\n\nfunc main() {}\n"
	tagged := "//go:build tools\n\npackage tools\n"
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("toolsfirst", "aaa_tools.go"): tagged,
		filepath.Join("toolsfirst", "y.go"):         "package y\n",
		filepath.Join("tagonly", "aaa_tools.go"):    tagged,
		filepath.Join("tagonly", "z.go"):            "//go:build linux\n\npackage z\n",
		filepath.Join("skipped", ".a.go"):           "package dot\n",
		filepath.Join("skipped", "_b.go"):           "package under\n",
		filepath.Join("skipped", "doc.go"):          "package documentation\n",
		filepath.Join("skipped", "types.go"):        "package real\n",
		filepath.Join("width", "aaa_gen.go"):        gen,
		filepath.Join("width", "width.go"):          "package w\n",
		filepath.Join("tools", "aaa_tool.go"):       "//go:build ignore\n\npackage tool\n",
		filepath.Join("tools", "x.go"):              "package x\n",
		filepath.Join("genonly", "gen.go"):          gen,
		filepath.Join("ignored", "s.go"):            "//go:build ignore\n\npackage s\n",
	})
	for path, want := range map[string]string{
		"github.com/example/app/width":      "w",
		"github.com/example/app/tools":      "x",
		"github.com/example/app/genonly":    "genonly", // no importable file
		"github.com/example/app/ignored":    "ignored", // its only file is build-ignored
		"github.com/example/app/nope":       "nope",    // no such directory
		"github.com/example/app/toolsfirst": "y",       // an unconstrained file names it
		"github.com/example/app/tagonly":    "tools",   // every file constrained: residual
		"github.com/example/app/skipped":    "real",    // _, . and documentation files never build
		"encoding/json":                     "json",    // not in the module
	} {
		assert.Equal(t, want, a.unaliasedImportName(path), path)
	}
}

// TestQualifiedStructImportablePackage pins that a struct reference resolves
// among its package's importable files only (#122): b's first file is a
// package main generator declaring its own Addr, which neither renames b's
// unaliased import nor replaces b's Addr under an aliased one, and a
// directory holding only such a generator resolves no struct.
func TestQualifiedStructImportablePackage(t *testing.T) {
	gen := "//go:build ignore\n\npackage main\n\ntype Addr struct {\n\tWrong int `json:\"wrong\"`\n}\n\nfunc main() {}\n"
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod":                         resolveGoMod,
		filepath.Join("b", "aaa_gen.go"): gen,
		filepath.Join("b", "b.go"): "package b\n\ntype Addr struct {\n\tStreet string `json:\"street\"`\n}\n\n" +
			"type Base struct {\n\tCreated string `json:\"created\"`\n}\n",
		filepath.Join("g", "gen.go"): gen,
		filepath.Join("mod", "module.go"): rowsModule(
			"\t\"github.com/example/app/b\"\n\tbb \"github.com/example/app/b\"\n\tgg \"github.com/example/app/g\"\n", "",
			"\tb.Base\n\tAddr b.Addr `json:\"addr\"`\n\tAliased bb.Addr `json:\"aliased\"`\n\tOnlyGen gg.Addr `json:\"onlyGen\"`\n"),
	})
	got := renders(t, a.typeRegistry["Rows"])
	assert.Equal(t, "$Addr", got["addr"])
	assert.Equal(t, "$Addr", got["aliased"])
	assert.Equal(t, goTypeString, got["created"])
	assert.Equal(t, "gg.Addr", got["onlyGen"])
	require.Contains(t, a.typeRegistry, "Addr")
	assert.Equal(t, map[string]string{"street": goTypeString}, renders(t, a.typeRegistry["Addr"]))
	assert.Equal(t, "b", a.typeRegistry["Addr"].Package)
	if w := fieldWarnings(a, "OnlyGen"); assert.Len(t, w, 1) {
		assert.Contains(t, w[0], "gg.Addr resolves to no schema")
	}
	assert.Empty(t, fieldWarnings(a, "Addr"))
	assert.Empty(t, fieldWarnings(a, "Aliased"))
}

// TestQualifiedToolsFileSortsFirst pins that a file of another clause that b
// never builds, sorting first in b, neither names b nor stands in for b's
// structs under either import, although it declares its own Addr: a
// //go:build tools file (types.go has no build constraint, so its clause is
// b's), a file named _ or . first, and a package documentation file (go/build
// skips the last three whatever the tags).
func TestQualifiedToolsFileSortsFirst(t *testing.T) {
	standIn := "\n\ntype Addr struct {\n\tWrong int `json:\"wrong\"`\n}\n"
	for _, first := range []struct{ name, src string }{
		{"aaa_tools.go", "//go:build tools\n\npackage tools" + standIn},
		{"_tools.go", "package tools" + standIn},
		{".tools.go", "package tools" + standIn},
		{"doc.go", "package documentation" + standIn},
	} {
		t.Run(first.name, func(t *testing.T) {
			a := analyzeDirectiveProject(t, map[string]string{
				"go.mod":                       resolveGoMod,
				filepath.Join("b", first.name): first.src,
				filepath.Join("b", "types.go"): "package b\n\ntype Addr struct {\n\tStreet string `json:\"street\"`\n}\n\n" +
					"type Base struct {\n\tCreated string `json:\"created\"`\n}\n",
				filepath.Join("mod", "module.go"): rowsModule(
					"\t\"github.com/example/app/b\"\n\tbb \"github.com/example/app/b\"\n", "",
					"\tbb.Base\n\tAliased bb.Addr `json:\"aliased\"`\n\tUnaliased b.Addr `json:\"unaliased\"`\n"),
			})
			got := renders(t, a.typeRegistry["Rows"])
			assert.Equal(t, "$Addr", got["aliased"])
			assert.Equal(t, "$Addr", got["unaliased"])
			assert.Equal(t, goTypeString, got["created"])
			require.Contains(t, a.typeRegistry, "Addr")
			assert.Equal(t, map[string]string{"street": goTypeString}, renders(t, a.typeRegistry["Addr"]))
			assert.Empty(t, a.Warnings(t.Context()))
		})
	}
}

// TestQualifiedAllConstrainedToolsFirst pins that when every file of b
// carries a build constraint, a //go:build tools file of package tools that
// sorts first still cannot hide b's structs under an aliased import: they are
// searched in every file the go command can build, as before. Named
// non-struct types still take that file's clause and fall back, as before,
// and an unaliased import of b is still named tools.
func TestQualifiedAllConstrainedToolsFirst(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod":                           resolveGoMod,
		filepath.Join("b", "aaa_tools.go"): "//go:build tools\n\npackage tools\n",
		filepath.Join("b", "b.go"): "//go:build !plan9\n\npackage b\n\ntype Addr struct {\n\tStreet string `json:\"street\"`\n}\n\n" +
			"type Base struct {\n\tCreated string `json:\"created\"`\n}\n\ntype Cents int64\n",
		filepath.Join("mod", "module.go"): rowsModule("\tbb \"github.com/example/app/b\"\n", "",
			"\tbb.Base\n\tAliased bb.Addr `json:\"aliased\"`\n\tCents bb.Cents `json:\"cents\"`\n"),
	})
	got := renders(t, a.typeRegistry["Rows"])
	assert.Equal(t, "$Addr", got["aliased"])
	assert.Equal(t, goTypeString, got["created"])
	require.Contains(t, a.typeRegistry, "Addr")
	assert.Equal(t, "b", a.typeRegistry["Addr"].Package)
	assert.Equal(t, "bb.Cents", got["cents"])
	assert.Empty(t, fieldWarnings(a, "Aliased"))
	if w := fieldWarnings(a, "Cents"); assert.Len(t, w, 1) {
		assert.Contains(t, w[0], "bb.Cents resolves to no schema")
	}
	assert.Equal(t, "tools", a.unaliasedImportName("github.com/example/app/b"))
}

// TestQualifiedAllConstrainedStandIns pins that when b's only real file
// carries a build constraint, a file beside it that b never builds, or one of
// another clause built only on a platform b's file excludes, neither names b
// nor stands in for b's types, although it declares its own Addr: a file named
// _ first, a package documentation file, a //go:build ignore tool, a package
// main generator under a tag other than ignore, and a z_plan9.go of another
// clause with no //go:build line. Its name constrains z_plan9.go, so it never
// names b as an unconstrained file would; it is still searched for structs,
// after types.go.
func TestQualifiedAllConstrainedStandIns(t *testing.T) {
	standIn := "\n\ntype Addr struct {\n\tWrong int `json:\"wrong\"`\n}\n"
	for _, other := range []struct{ name, src string }{
		{"_old.go", "package old" + standIn},
		{"doc.go", "package documentation" + standIn},
		{"aaa_tool.go", "//go:build ignore\n\npackage tool" + standIn},
		{"aaa_gen.go", "//go:build generate\n\npackage main" + standIn + "\nfunc main() {}\n"},
		{"z_plan9.go", "package bplan9" + standIn},
	} {
		t.Run(other.name, func(t *testing.T) {
			a := analyzeDirectiveProject(t, map[string]string{
				"go.mod":                       resolveGoMod,
				filepath.Join("b", other.name): other.src,
				filepath.Join("b", "types.go"): "//go:build !plan9\n\npackage b\n\ntype Addr struct {\n\tStreet string `json:\"street\"`\n}\n\n" +
					"type Base struct {\n\tCreated string `json:\"created\"`\n}\n\ntype Cents int64\n",
				filepath.Join("mod", "module.go"): rowsModule(
					"\t\"github.com/example/app/b\"\n\tbb \"github.com/example/app/b\"\n", "",
					"\tbb.Base\n\tAliased bb.Addr `json:\"aliased\"`\n\tUnaliased b.Addr `json:\"unaliased\"`\n\tCents bb.Cents `json:\"cents\"`\n"),
			})
			got := renders(t, a.typeRegistry["Rows"])
			assert.Equal(t, "$Addr", got["aliased"])
			assert.Equal(t, "$Addr", got["unaliased"])
			assert.Equal(t, goTypeString, got["created"])
			assert.Equal(t, goTypeInt64, got["cents"])
			require.Contains(t, a.typeRegistry, "Addr")
			assert.Equal(t, map[string]string{"street": goTypeString}, renders(t, a.typeRegistry["Addr"]))
			assert.Equal(t, "b", a.typeRegistry["Addr"].Package)
			assert.Equal(t, "b", a.unaliasedImportName("github.com/example/app/b"))
			assert.Empty(t, a.Warnings(t.Context()))
		})
	}
}

// TestDelegateBehindGenerator pins that a route delegate in another package
// is found through an unaliased import when a package main generator sorts
// first in its directory (#122): its routes are discovered and typed.
func TestDelegateBehindGenerator(t *testing.T) {
	_, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":                         resolveGoMod,
		filepath.Join("h", "aaa_gen.go"): "//go:build ignore\n\npackage main\n\nfunc main() {}\n",
		filepath.Join("h", "h.go"): `package h

import "github.com/gaborage/go-bricks/server"

type Handler struct{}
type Thing struct{ ID int64 ` + "`json:\"id\"`" + ` }

func (x *Handler) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/things", x.list)
}
func (x *Handler) list(ctx server.HandlerContext) (server.Result[Thing], server.IAPIError) {
	return server.Result[Thing]{}, nil
}
`,
		filepath.Join("mod", "module.go"): `package mod

import (
	"github.com/example/app/h"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{ hd *h.Handler }

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	m.hd.RegisterRoutes(hr, r)
}
`,
	})
	list := routeByPath(t, routes, "/things")
	require.NotNil(t, list.Response)
	assert.Equal(t, "Thing", list.Response.Name)
}
```

- [ ] **Step 1b (R16; revision 5, not prototyped):** Add `TestQualifiedAllConstrainedOwnClauseFirst` right after `TestQualifiedAllConstrainedStandIns`. Build it like `TestQualifiedAllConstrainedToolsFirst` (`analyzeDirectiveProject`, `resolveGoMod`, `rowsModule`), importing `"github.com/example/app/b"` and `bb "github.com/example/app/b"`, with fields `b.Base` (embedded), `Unaliased b.Addr`, `UCents b.Cents`, `Aliased bb.Addr` and `Cents bb.Cents`. Table rows:
  - `_old.go`: `b/_old.go` is `package b\n\nfunc deprecated() {}\n`, `b/aaa_tools.go` is `//go:build tools\n\npackage tools\n`, and `b/b.go` is `TestQualifiedAllConstrainedToolsFirst`'s `b.go` (`//go:build !plan9`; `Addr`, `Base`, `Cents`).
  - `a_gen.go`: `b/a_gen.go` is `//go:build ignore\n\npackage b\n`, `b/tools.go` is `//go:build tools\n\npackage tools\n`, and `b/types.go` has the same source as that `b.go`.

  Assertions per decision 6's answer:
  - **(a)** pins the regression, so a later fix flips it deliberately. These values are inferred from the `rev4` binary's output on `advrev/x1` and `x2`, not read from a test run; `unaliasedImportName` returning `tools` is inferred too (a `filepath.Base` fallback to `b` would have found `b.go`'s `Addr`). In both rows: `aliased` is `$Addr`, and `Addr` renders `{"street": string}` in package `b`. `unaliased` is `b.Addr` and `ucents` is `b.Cents`, each with one "resolves to no schema" warning. `created` is absent, `unaliasedImportName` is `tools`, and `cents` is `bb.Cents` with its warning (in the `a_gen.go` row, `ucents` and `cents` fall back on `main` too).
  - **(a')** asserts at least `main`'s output. In both rows: `unaliased` and `aliased` are `$Addr`, `created` is a string, and `unaliasedImportName` is `b`. In the `_old.go` row, `ucents` and `cents` are int64 and there are no warnings.
  - **(b)** asserts (a)'s output, except that `aliased` is `bb.Addr` with its warning and there is no `Addr` component.

  At `033a9c9` this test is red under (a) and (b), by design (it pins outputs that differ from `main`'s), and green under (a').
- [ ] **Step 2:** `gofmt -w internal/analyzer/qualified_test.go`. The block above is already gofmt output, so expect no change.
- [ ] **Step 3: Run red.** `go test ./internal/analyzer -count=1 -run 'TestQualifiedBuildTaggedVariants|TestUnaliasedImportName|TestQualifiedStructImportablePackage|TestQualifiedToolsFileSortsFirst|TestQualifiedAllConstrainedToolsFirst|TestQualifiedAllConstrainedStandIns|TestDelegateBehindGenerator'`. These results at `033a9c9` were observed on the prototype's red runs (`rev3/red`, `rev4/red`):
  - `TestQualifiedBuildTaggedVariants`: the original four cases PASS (the refactor is behavior-neutral). The four new "sorts first" cases FAIL. PR 129's `inModuleTypeSite` takes the stand-in file's clause and its `type Width string`, so the run reports `expected "int64"`, `actual "string"` (and `[]string`, `map[string]string`) silently.
  - `TestQualifiedBuildTaggedVariantsUnaliased`: the two "widths" cases PASS. All six "sorts first" cases FAIL (`w.Width` falls back, with the old "first file has another package clause" warnings).
  - `TestUnaliasedImportName`: FAIL on `width` (`main`), `tools` (`tool`), `skipped` (`dot`), `genonly` (`main`), `ignored` (`s`) and `toolsfirst` (`tools`). The `tagonly` row passes; it is the R13 residual, unchanged.
  - `TestQualifiedStructImportablePackage`: FAIL. `addr` is `b.Addr`, `created` is missing, `onlyGen` is `$MainAddr`, and `Addr` is `{"wrong":"int"}` in package `main`.
  - `TestQualifiedToolsFileSortsFirst`: all four subtests FAIL. `unaliased` is `b.Addr`, with its warning. `aliased` is `$Addr`, but `Addr` is the stand-in's `{"wrong":"int"}`.
  - `TestQualifiedAllConstrainedToolsFirst`: **PASS**. It guards against a regression (R13) and is green on `main` and after the change. A1 without `inPackage` fails it (mutation M10).
  - `TestQualifiedAllConstrainedStandIns`: `_old.go`, `doc.go`, `aaa_tool.go` and `aaa_gen.go` FAIL. In each, `Addr` is the stand-in's `{"wrong":"int"}` in the stand-in's package, and `unaliased` is `b.Addr` with its warning. The unaliased import is named after the first file: `old`, `documentation`, `tool` or `main`. `cents` is `bb.Cents` behind `_old.go` and `doc.go`. `z_plan9.go` **PASSES**: `main` resolves it, and revision 3's code without `osArchSuffixed` fails it (R15, mutation M16).
  - `TestDelegateBehindGenerator`: FAIL (`route not found`).
  - `TestQualifiedAllConstrainedOwnClauseFirst` (Step 1b; add it to the `-run` pattern): FAIL under (a) and (b), because `main` resolves `unaliased`, `created` and (in the `_old.go` row) `cents`. Not observed on a prototype.

## Task 2: Name unaliased imports by the importable clause (green, part 1)

**Files:** Modify `internal/analyzer/analyzer.go:3640-3686`.

- [ ] **Step 1:** Replace `unaliasedImportName`'s doc and body with the block in "Design → `unaliasedImportName`", and replace `fileImports`'s doc comment with the one given there.
- [ ] **Step 2:** Re-run Task 1 Step 3's command plus `TestFileImportsUsesDeclaredPackageName`. Observed with only this task applied:
  - **PASS:** `TestDelegateBehindGenerator`, `TestFileImportsUsesDeclaredPackageName`, `TestQualifiedAllConstrainedToolsFirst`, `TestQualifiedAllConstrainedStandIns/z_plan9.go`, and the first four cases of both variant tests.
  - **Still FAIL:** the four new cases of both variant tests (tools-tagged, underscore, dot, documentation).
  - **Still FAIL:** `TestUnaliasedImportName`, on `toolsfirst` (`tools`) and `skipped` (`dot`) only.
  - **Still FAIL:** `TestQualifiedStructImportablePackage`, all four `TestQualifiedToolsFileSortsFirst` subtests, and the other four `TestQualifiedAllConstrainedStandIns` subtests (each still registers the stand-in's `Addr`).

  Task 3's `packageClause` and `inPackage` fix the rest.

## Task 3: Package clause and struct search (green, part 2)

**Files:** Modify `internal/analyzer/resolution.go:346-357` and `internal/analyzer/analyzer.go:3535-3566`.

- [ ] **Step 0 (reminder):** this task implements decision 6's answer from Task 1 Step 0. Under (b), use Appendix B's code instead of Steps 1-2.
- [ ] **Step 1a (red):** Add this test to `internal/analyzer/qualified_test.go`, right after `TestQualifiedAllConstrainedStandIns`. It names `osArchSuffixed`, so the package does not compile until Step 1. That is why it is not in Task 1, whose red run must compile against `033a9c9`.

```go
// TestOSArchSuffixed pins go/build's file-name constraint as osArchSuffixed
// reads it: the element after the last underscore, cut at the first dot.
func TestOSArchSuffixed(t *testing.T) {
	for name, want := range map[string]bool{
		"z_windows.go":         true,
		"x_amd64.go":           true,
		"x_linux_arm64.go":     true,
		"linux_amd64.go":       true,  // the part before the first _ is a prefix, amd64 a suffix
		"x_linux.pb.go":        true,  // cut at the first dot
		"x_windows_test.pb.go": true,  // a trailing _test element is dropped first
		"linux.go":             false, // no underscore: no suffix
		"x_unix.go":            false, // unix is a build tag only
		"x_tools.go":           false,
		"types.go":             false,
	} {
		assert.Equal(t, want, osArchSuffixed(name), name)
	}
}
```

- [ ] **Step 1:** Replace `importableClause`'s doc comment and body with the block in "Design → `importableClause`, `packageClause`, `neverImportable`, `osArchSuffixed`, `inPackage`".
- [ ] **Step 2:** Replace `resolveQualifiedStruct`'s doc comment, and the block after its `parsePackageDir` error check, with the code in "Design → `resolveQualifiedStruct`".
- [ ] **Step 3:** `go test ./internal/analyzer -count=1`: everything passes.

## Task 4: Warning text

**Files:** Modify `internal/analyzer/resolution.go:36-39`; under decision 8's default, also `internal/analyzer/qualified_test.go`.

This task implements decision 8's answer from Task 1 Step 0. A reply that does not answer decision 8 means the alternative (drop the clause).

- [ ] **Step 0a (decision 8 default only, red):** in `TestQualifiedAllConstrainedToolsFirst`, inside the `if w := fieldWarnings(a, "Cents"); assert.Len(t, w, 1) {` block and after its `assert.Contains(t, w[0], "bb.Cents resolves to no schema")` line, add:

  ```go
  		assert.Contains(t, w[0], "an in-module package whose buildable files all carry a build constraint and whose first one by name has another package clause")
  ```

  `go test ./internal/analyzer -count=1 -run TestQualifiedAllConstrainedToolsFirst` FAILS: the warning still carries `main`'s wording.
- [ ] **Step 1:** Replace `unresolvableFieldWarning` with decision 8's constant from "Design → Warning text": the three-line default, or the two-line alternative.
- [ ] **Step 2:** `grep -rn "first file has another" internal cmd README.md` prints nothing. Under the default, `grep -rn "first one by name has another package clause" internal` prints the constant and the Step 0a line. `go test ./internal/analyzer ./internal/commands -count=1` PASS.

## Task 5: Golden fixture

**Files:** Create `internal/spectest/testdata/importable_clause/{go.mod,module.go,b/aaa_gen.go,b/b.go}` from "Goldens". `expected.yaml` is generated.

- [ ] **Step 1:** Write the four files verbatim. `gofmt -l internal/spectest/testdata/importable_clause` prints nothing.
- [ ] **Step 2:** `go test ./internal/spectest -run 'TestGoldenFixtures/importable_clause' -update -count=1`, then `go test ./internal/spectest -count=1` (no `-update`) PASS.
- [ ] **Step 3:** `git status --porcelain internal/spectest/testdata` lists only `?? internal/spectest/testdata/importable_clause/`. Review `expected.yaml` against the excerpt and checks in "Goldens".

## Task 6: README

**Files:** `README.md` only.
- `CONTEXT.md` and `docs/adr/` need nothing: neither mentions import naming, build constraints or first files (checked with `grep -n -i 'ignore\|package main\|first file\|clause\|unaliased' CONTEXT.md docs/adr/*.md`).
- The tracked PR 129 plan (`docs/superpowers/plans/2026_10_08_qualified_named_types_pr2.md`) describes the old `unaliasedImportName` (`:139`) and `importableClause`'s tie-break (`:181`, `:1487`). It is an executed plan and a historical record, not a design doc under `docs/superpowers/specs/`, so it gets no SUPERSEDED banner. Leave it untouched.

- [ ] **Step 1 (#122 caveat, README.md:302-307).** Replace

```
  `--strict` fails on it). Named types from other packages of the project,
  under a default or aliased import (not a dot import), are resolved in their
  own package like local ones —
  except through an unaliased import of a directory whose first file by name
  has another package clause (a `package main` generator or another
  build-ignored file), which falls back with that warning (#122).
```

with

```
  `--strict` fails on it). Named types from other packages of the project,
  under a default or aliased import (not a dot import), are resolved in their
  own package like local ones.
```

- [ ] **Step 2 (build constraints, README.md:394-399).** Replace the first six lines of that bullet

```
- Build constraints (`//go:build`) are ignored, except `ignore`, which only
  decides which files are searched for an imported package's named non-struct
  types (struct lookups still scan every file): a build-ignored file of
  another package clause (a generator or tool) is skipped, while one sharing
  the package's clause is still merged (an unaliased import of such a
  directory is the #122 exception above). When a registration helper is
```

with the following. The bullet continues unchanged from "declared in several build-tagged files".

```
- Build constraints (`//go:build` lines and GOOS/GOARCH file-name suffixes
  such as `_linux.go`) are not evaluated, except `ignore`. An imported
  package's clause is read from its directory, skipping the files the go
  command never builds into a package (`package main`, `package
  documentation`, and names starting with `_` or `.`): the first remaining
  file by name with no constraint (no `//go:build` line and no such suffix)
  names it, or, when every remaining file is constrained, the first that is
  not build-ignored. That clause names an unaliased import, and only files
  with it are searched for the package's named types. When a file with no
  constraint named it, only those files are searched for its structs too,
  so a generator or tool of another clause (`package main`, or under
  `//go:build ignore` or `//go:build tools`) is skipped, while a file
  sharing the package's clause is still merged, even when build-ignored or
  named `_…`. A struct found this way competes for its short component
  name like any other: reached before a same-named struct of another
  package, it takes the short one (`Addr`), and that struct's component is
  qualified (`ModAddr`), so its `$ref`s and generated client type name
  move. When every file is constrained, the first remaining file can be
  one of another clause under another constraint (`//go:build tools`, or a
  name such as `x_windows.go`), even when a skipped file of the package's
  own clause sorts before it. It then names the package: an unaliased
  import of it and its named types fall back with a warning. Its structs
  are searched in every file the go command can build, whichever clause
  named the package, so a same-named struct in a file of another clause
  stands in for the package's when that file sorts before the one
  declaring the package's own.
  A registration method or helper function is looked up in every file of
  its directory.
  When a registration helper is
```

Under (a'), drop ", even when a skipped file of the package's own clause sorts before it" (R16 keeps `main`'s answer there). Under (b), use Appendix B's README text.

- [ ] **Step 3:** `grep -n '#122\|struct lookups still scan' README.md` prints nothing.

## Task 7: Lint, complexity, coverage, mutation

- [ ] **Step 1:** Run `make check` (it runs `fmt lint test validate-cli`). `fmt` may rewrite files, so re-check `git status` afterwards. Run `make sec`: expect 22 files scanned, but trust CI's `Security (gosec)` count.
- [ ] **Step 2:** `gocognit -over 15 internal/analyzer/analyzer.go internal/analyzer/resolution.go internal/analyzer/qualified_test.go` exits 0, with numbers as in the budget table.
- [ ] **Step 3: Coverage.** With `SCRATCH` set to the session scratchpad (never the repo), run `go test ./internal/analyzer -count=1 -coverprofile=$SCRATCH/c.out && go tool cover -func=$SCRATCH/c.out | grep -E 'unaliasedImportName|resolveQualifiedStruct|importableClause|packageClause|neverImportable|osArchSuffixed|inPackage|inModuleTypeSite'`. Every line must show 100.0%.
- [ ] **Step 4: Mutation checks.** Apply each mutation to `analyzer.go` or `resolution.go`, run `go test ./internal/analyzer ./internal/spectest -count=1`, and record the failures. Revert with `git checkout -p` or the editor, and never commit a mutant. The prototype's results (`rev4/scripts/mut.py`). `StandIns` is `TestQualifiedAllConstrainedStandIns`:

| # | Mutation | Must fail (prototype) |
|---|---|---|
| M1 | `unaliasedImportName` back to the first-file loop | `…Unaliased` (all six "sorts first"), `TestUnaliasedImportName`, `TestQualifiedStructImportablePackage`, `TestQualifiedToolsFileSortsFirst` (all four), `StandIns` (all but `z_plan9.go`), `TestDelegateBehindGenerator`, `TestGoldenFixtures/importable_clause`; under (a) also `TestQualifiedAllConstrainedOwnClauseFirst` (both rows; revision 5, not run) |
| M2 | disable the struct filter (`if !inPackage(path, file, clause, sure) && false {`) | `TestQualifiedStructImportablePackage`, `TestQualifiedToolsFileSortsFirst` (all four), `StandIns` (all but `z_plan9.go`), `TestGoldenFixtures/importable_clause` |
| M3 | `unaliasedImportName` always returns `filepath.Base(path)` | `TestFileImportsUsesDeclaredPackageName`, `…Unaliased` (all eight), `TestUnaliasedImportName`, `TestQualifiedAllConstrainedToolsFirst`, `TestGoldenFixtures/pkgname_mismatch` |
| M4 | `clause, _, sure := packageClause(files)` without the `!ok` return | nothing: an **equivalent mutant**, expected to survive |
| M5 | drop `if fileBuildConstraint(f) == nil && !osArchSuffixed(filepath.Base(p)) { return f.Name.Name, true, true }` (the new tie-break) | both variant tests ("a tools-tagged file…"), `TestUnaliasedImportName`, `TestQualifiedToolsFileSortsFirst/aaa_tools.go` |
| M6 | drop the `!ok && !buildIgnored(f)` fallback | both variant tests ("widths disagree", "widths agree"), `TestUnaliasedImportName`, `TestQualifiedAllConstrainedToolsFirst`, `StandIns` (all five) |
| M7 | drop `strings.HasPrefix(base, "_")` from `neverImportable` | both variant tests ("an underscore-named file…"), `TestUnaliasedImportName`, `TestQualifiedToolsFileSortsFirst/_tools.go`, `StandIns/_old.go` |
| M8 | drop `strings.HasPrefix(base, ".")` | both variant tests ("a dot-named file…"), `TestUnaliasedImportName`, `TestQualifiedToolsFileSortsFirst/.tools.go` |
| M9 | drop `f.Name.Name == documentationPackageName` | both variant tests ("a package documentation file…"), `TestUnaliasedImportName`, `TestQualifiedToolsFileSortsFirst/doc.go`, `StandIns/doc.go` |
| M10 | `inPackage` never searches another clause (`return false && …`), which is A1 alone | `TestQualifiedAllConstrainedToolsFirst` |
| M11 | `inPackage` ignores `sure` (`return !neverImportable(path, f) && !buildIgnored(f)`) | `TestQualifiedToolsFileSortsFirst/aaa_tools.go` |
| M12 | `pkg: clause` instead of `pkg: file.Name.Name` | `TestQualifiedAllConstrainedToolsFirst` |
| M13 | `inPackage` drops `!buildIgnored(f)` (`return !sure && !neverImportable(path, f)`) | `StandIns/aaa_tool.go` |
| M14 | `inPackage` drops `!neverImportable(path, f)` (`return !sure && !buildIgnored(f)`) | `StandIns/_old.go`, `StandIns/doc.go`, `StandIns/aaa_gen.go` |
| M15 | drop `f.Name.Name == mainPackageName` from `neverImportable` | `StandIns/aaa_gen.go` (the only `package main` file under a tag other than `ignore`) |
| M16 | drop `&& !osArchSuffixed(filepath.Base(p))` (revision 3's tie-break) | `StandIns/z_plan9.go` |
| M17 | `osArchSuffixed` returns `knownOS[suffix]` only | `TestOSArchSuffixed` |
| M18 | `osArchSuffixed` returns `knownArch[suffix]` only | `TestOSArchSuffixed`, `StandIns/z_plan9.go` |
| M19 | `name := strings.TrimSuffix(base, ".go")` instead of the cut at the first dot | `TestOSArchSuffixed` |
| M20 | `strings.Index` instead of `strings.LastIndex` | `TestOSArchSuffixed` |
| M21 | drop `name = strings.TrimSuffix(name, "_test")` from `osArchSuffixed` (revision 5's helper) | `TestOSArchSuffixed` (`x_windows_test.pb.go`; revision 6, `rev6/m21`) |
| M22 | decision 8 default only: the two-line alternative constant in place of the reworded one | `TestQualifiedAllConstrainedToolsFirst` (revision 6, `rev6/m22`) |

## Task 8: Commit

- [ ] **Step 1:** Stage exactly these paths:
  - `internal/analyzer/analyzer.go`
  - `internal/analyzer/resolution.go`
  - `internal/analyzer/qualified_test.go`
  - `internal/spectest/testdata/importable_clause/` (5 files)
  - `README.md`
  - this plan file

  `git status --porcelain` shows nothing else. There is no `go-bricks-openapi` binary, because it is gitignored. `git check-ignore -v` on this plan's path exits 1, so the plan is tracked once committed.
- [ ] **Step 2:** Write the message below to a file in the session scratchpad and check it: `grep -n '#' <msgfile>` must print **only** the `Closes #122` line. Commit with `git commit -F <msgfile>`. Commits are signed via 1Password: if it is locked, retry, and never disable signing.

```
fix(analyzer)!: resolve imported packages from their importable files

An unaliased import of an in-module package was named after the package
clause of the first file in its directory by name. A //go:build ignore
generator in package main that sorted first (aaa_gen.go, or gen.go
beside types.go) therefore renamed the import to main, and nothing
qualified by it resolved. The clause is now read from the files the go
command can build into the package. Files declaring package main or
package documentation, and files whose names start with _ or ., are
skipped. Of the rest, the first file by name with no build constraint
names the package, since such a file is in every build. A file is
constrained when it has a //go:build line or a GOOS/GOARCH file-name
suffix as go/build reads it, such as _windows.go or _windows_test.pb.go;
only the ignore tag is evaluated. When every file is constrained, the
first that is not build-ignored names it. That clause names an unaliased
import (falling back to the import path's last element), and lookups of
named non-struct types use it as before. Struct lookups in an imported
package search only files of that clause too, so a generator's struct of
the same name no longer stands in for the package's own, wherever the
generator sorts and under an aliased import as well. When the clause
came from a constrained file, struct lookups still search every file the
go command can build, as they did, but no longer a file of another
clause that it never builds.

Through such an import, struct fields are $refs again: b.Addr direct, in
a slice, behind a pointer and as a map value. b.Cents, b.Tags and
b.Level are integer, array and string, and b.Money (own MarshalJSON) is
{} with its Marshaler warning. Result[b.Addr] and Result[b.Cents]
payloads are typed. A b.Addr request gets its requestBody instead of a
"not a struct" warning. Embedded b.Base and *b.Addr promote their
fields, and a struct embedding b.Level is a string instead of an empty
component. A route delegate declared in such a package is found, so its
routes are discovered. Warnings drop accordingly, so generate --strict
can newly pass, and doctor counts more routes as typed. A struct
declared only in a generator or build-ignored tool of the imported
directory no longer resolves. It is now an untyped object with the
unresolvable-type warning (an untyped payload, no requestBody), where it
used to be the generator's struct. A struct that now resolves through
such an import takes part in component naming: when it is reached before
a same-named struct of another package, it takes the bare component name
and that struct's component becomes `<Pkg><Name>` (e.g. Addr -> ModAddr),
silently. The unresolvable-type warning's first-file cause now names the
layout that still reaches it: an in-module package whose buildable files
all carry a build constraint, the first by name of another clause.

The same rule fixes a package that has an unconstrained file and a
sorted-first file of another clause that is under a build constraint
other than ignore (//go:build tools, package tools, or a name such as
a_windows.go), is named with a leading _ or ., or declares package
documentation. Its unaliased import and its named non-struct types now
resolve, and none of that file's declarations stands in for the
package's. Struct references to it through an aliased import resolved
before and still do. In a package whose every file is constrained, a
file it never builds (package main under any tag, package
documentation, a _ or . name, or a build-ignored file) no longer names
it or stands in for its types either. Residual: in such a package, the
first remaining file by name can be one of another clause under a
constraint other than ignore (//go:build tools, or a name such as
x_windows.go). It still names the package, and its unaliased import and
named non-struct types fall back. Its structs are still searched in
every file the go command can build, whichever clause named the package,
so a same-named struct declared in a file of another clause still stands
in for the package's when that file sorts before the one declaring the
package's own.

Regression: that residual now also applies when a file of the package's
own clause that the go command never builds (a leading _ or . in its
name, or //go:build ignore) sorts before the file of another clause
(under //go:build tools, or a GOOS/GOARCH-suffixed name such as
a_windows.go). That file used to name the unaliased import correctly.
Now the other clause does, so references through that import fall back:
fields and named types are untyped objects with a warning, embedded
structs lose their promoted fields silently, and a request gets no
requestBody. An unaliased import of a real package of that other clause,
placed earlier in the same file, falls back the same way, because the
misnamed import replaces its entry. Behind a sorted-first _ or . file,
named non-struct types fall back under an aliased import too. Structs
under an aliased import still resolve.

No existing golden moves. The new importable_clause golden covers the
fields, embeds, payloads, request, embedded text type and aliased
stand-in above. New analyzer tests add an unaliased twin of the
build-tagged variant test, in a directory whose base is not its clause.
They also pin the import-name fallbacks, the struct filter, the
delegate, each skipped first file under both imports, the
all-constrained layout with each kind of stand-in, the regressed
own-clause-first layout, and the file-name suffix reader.

BREAKING CHANGE: in an imported in-module directory whose every file the go command can build carries a build constraint, where a file of the package's own clause that it never builds (a leading _ or . in its name, or //go:build ignore) sorts before a file of another clause under another constraint (//go:build tools, or a GOOS/GOARCH-suffixed name such as a_windows.go), an unaliased import of the package now falls back: fields and named types are untyped objects with a warning, embedded structs lose their promoted fields silently, requests get no requestBody, and generate --strict fails. An unaliased import of a real package of that other clause, placed earlier in the same file, falls back the same way, because the misnamed import replaces its entry. Behind a leading-_ or -. file, named non-struct types fall back under an aliased import too.

Closes #122

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

This is the message for decision 6 (a). The subject is 69 characters. It carries `!` and a `BREAKING CHANGE:` footer because R16 regresses valid code, and RELEASING.md marks breaking changes so; pre-1.0, that caps the release at MINOR. After the Release PR opens, check that the breaking note in its body stops before `Closes` and the co-author trailer. Under decision 6 (b), use Appendix B's message instead, adding this message's "Regression:" paragraph. Under (a'), the later plan revision that prototypes (a') supplies the message. The second paragraph's last sentence follows decision 8's default; under its alternative it reads "The unresolvable-type warning no longer lists the first-file case." instead (re-wrap that paragraph to 72 columns). A one-commit PR squashes under this subject (`COMMIT_OR_PR_TITLE`) and this body (`COMMIT_MESSAGES`).

PR body, for whoever opens the PR (≤150 words; three headings):

```markdown
## What
An unaliased in-module import was named after its directory's first file, so a sorted-first `package main` generator broke every reference through it; struct lookups could also pick a generator's struct. The clause now comes from files Go builds: `package main`, `package documentation` and `_`/`.` names are skipped, and an unconstrained file wins. Struct lookups search only that clause unless it came from a constrained file.

## Impact
Through such an import, fields, embeds, payloads, requests and delegated routes resolve; a struct declared only in a generator now warns. A sorted-first `//go:build tools`, `_`/`.` or `package documentation` file no longer hides its package's types. Breaking: in an all-constrained package, a never-built own-clause file sorting before a constrained other-clause file (`//go:build tools`, `a_windows.go`) misnames the unaliased import, breaking references through it and through an earlier same-named import.

## Verification
New golden `importable_clause`; no existing golden moved. Mutation-checked: every behavior-changing revert fails a test.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

---

## Impact (user-visible behavior changes; SemVer surfaces: generated-output shape, doctor/validation)

Only projects with an imported in-module directory that holds one of these files are affected: a `package main` file, a `package documentation` file, a `_…`/`.…` file, or a file of another clause under a build constraint (a `//go:build` line or a GOOS/GOARCH file-name suffix). The import name changes only when such a file sorts first. Through the struct filter, a same-named struct in such a file that the go command never builds stops standing in wherever that file sorts, not only when it sorts first (R14, R17): a project with no sorted-first stray file can still see its spec change, silently, to the real struct. Item 7 is the one regression (R16).

1. **Generated output, when such a file sorts first and the import is unaliased (rows R1-R6, R9, R11, R12, R14, R15):**
   - `$ref`s and typed scalars, slices and strings;
   - `{}` for a Marshaler struct;
   - typed `Result[T]` payloads and a `requestBody`;
   - promoted embed fields;
   - a string for a struct promoting a text pair (its empty component disappears);
   - a delegate's routes, which were missing.
2. **Generated output under any import (R7, R12, R14, R15, R17):** a same-named declaration in such a file no longer replaces the package's own. That includes a generator sorted between two real files (R17: `wrong` → `street`, no warning before or after), a package whose every file is constrained, when the file is one go never builds (R14), and a sorted-first `_GOOS` file of another clause beside an unconstrained file (R15). Struct components change to the real properties (`wrong` → `street`), and named types to the real type (`x1.Cents`: `string` → `integer, int64`). Named non-struct types behind a sorted-first `//go:build tools`, `_…`, `.…` or `package documentation` file resolve under an aliased import too. They fell back on `main` through PR 129's `inModuleTypeSite` (`bb.Cents`: unresolvable-type warning → `{type: integer, format: int64}`). A struct that now resolves through such an import takes part in component naming (`schemaKey`, first registered wins): when it is reached before a same-named struct of another package, it takes the bare component name, and that struct's component becomes `<Pkg><Name>` (e.g. `Addr` → `ModAddr`), silently (`regrev/fx/swap`: `033a9c9` `Addr: {city}` with `Warnings: 1`; plan `Addr: {street}`, `ModAddr: {city}`, `Warnings: 0`; reached in the other order, `swaprev` keeps `Addr: {city}` and adds `BAddr`).
3. **Newly unresolved (R10):** some structs used to resolve to a never-built file's struct (a component under clause `main`, e.g. `MainAddr`), although Go would not compile the reference. These are structs declared **only** in files of the imported directory that go never builds. That covers a generator or build-ignored tool, and a `_…`/`.…`/`package documentation` file of another clause. It also covers a `//go:build tools` file of another clause when the package has an unconstrained file (mutation M11 pins that skip). Now:
   - their field is `{type: object}` with the unresolvable-type warning;
   - their payload is untyped with "resolves to no schema component";
   - their request gets no `requestBody` and the non-struct request warning.

   `generate --strict` newly fails there.
4. **Warnings:** several warnings that these rows raised disappear: unresolvable-type, untyped-payload, non-struct-request, aggregate untyped-route, "skipping m.<field>.<Method>(...)" and "modules discovered but no routes". `generate --strict` therefore newly passes where they were the only warnings (R8 without `Money`, R9, R11's `bb.Cents`, R12). The real types can raise their own warnings instead: R2's Marshaler warning, or a `uintptr` or a malformed tag in the real struct. Item 3 raises these same warnings where none fired before. The unresolvable-type warning's "an unaliased in-module import whose directory's first file has another package clause" cause is reworded to "an in-module package whose buildable files all carry a build constraint and whose first one by name has another package clause" under decision 8's default, or dropped under its alternative. The warning fires in the same places either way; only its text changes.
5. **doctor:** more routes are discovered and typed (R8: 3/5 → 5/5; R9: 0 → 3 routes), and "Ready with caveats" can become ready.
6. **Not observable on valid code:** an unaliased import of a directory with no importable file is now keyed by the path's last element instead of `main` or an ignored file's clause.
7. **Regression on valid code (R16), under decision 6 (a):** take a package whose every file the go command can build is constrained. A file of the package's own clause that go never builds (`_old.go`, `.old.go`, or a `//go:build ignore` file) sorts first, and a file of another clause under another constraint (`//go:build tools`, `package tools`, or a GOOS/GOARCH-suffixed name such as `a_windows.go` with no build line) comes next. `main` named the unaliased import after the first file, which was right. `packageClause` skips that file and takes the other clause, not sure. So through the unaliased import:
   - fields and named types become `{type: object}` with the unresolvable-type warning;
   - embedded structs lose their promoted fields silently;
   - a request gets no `requestBody`, with the non-struct request warning.

   Another unaliased import in the same file, of a real in-module package whose clause is that other clause (e.g. an actual `internal/tools`), stops resolving too when it comes earlier in the import list: the misnamed import replaces its `fileImports` entry (later write wins, `analyzer.go:3659`; the plan does not change that rule). That is one more layout reaching a collision `main` already has in R13 layouts, not a new class (`regrev8/fx/coll`: `Warnings: 0` → `3`, requestBody and both components lost; imported in the other order, only `b.Addr` breaks, as above).

   When the skipped file is a `_…` or `.…` file, PR 129's rule had taken its clause for named non-struct types too, so they also fall back under an aliased import (`bb.Cents`). Struct references through an aliased import still resolve. `generate --strict` newly fails (`advrev/x1`: `Warnings: 0` → `4`, and the same for its `.old.go` and `a_windows.go` variants `beh8/fx/x1dot`, `x1win`; `x2`: `2` → `4`). Under (b) it is worse (`5` each, aliased structs lost too). (a') would keep `main`'s output here. When the other-clause file declares the same name (`type Cents string` in `aaa_tools.go`), aliased `bb.Cents` takes that declaration instead of falling back: silently, so `--strict` passes on a wrong schema (post-commit review, scratch `issue122/probe_standin`: `integer`/`int64` → `string`, `Warnings: 0` on both sides; pinned by the stand-in row of `TestQualifiedAllConstrainedOwnClauseFirst`).

Apart from R16, no measured layout regresses against `main` on valid code (R11-R15, R17). Struct references through an aliased import resolve wherever they did, R16 included. The aliased-struct fixtures of R11 and R12, all four R13 layouts, R15's `e1` and its `_test`-suffixed variant (`rev6/fx_suffix_test`) are byte-identical to `main`'s output. No rule that never evaluates build constraints wins every all-constrained layout: R16's never-built same-clause file is evidence of the package's clause that the body discards.

## Residuals (documented, not fixed)

- **R13.** Take a package whose every file is constrained, by a `//go:build` line or a GOOS/GOARCH file-name suffix. When the first file by name that `packageClause` does not skip is of another clause under a constraint other than `ignore` (`//go:build tools`, or a name such as `a_windows.go`), it still names the package. Its unaliased import and named non-struct types then fall back with a warning, as on `main` when that file also sorts first. When a skipped file of the package's own clause sorts before it, that is the R16 regression. In such a package the clause is never sure, so its structs are searched in every file the go command can build, as on `main`, and they resolve under an aliased import. A file of another clause under a constraint other than `ignore` is searched wherever it sorts, even when a file of the real clause named the package: a same-named struct in it still stands in for the package's, silently, as on `main`. That holds when it sorts first (`e1b`) and when it sorts between the real files (`behav/fx/r16`: `m_tools.go`; `r16w`: `m_windows.go`). Pinned by `TestQualifiedAllConstrainedToolsFirst` and by `TestUnaliasedImportName`'s `tagonly` row; the mid-sorted stand-in is not pinned by a test.
- **R16 (regression, under decision 6 (a)).** See Impact item 7. Pinned by Task 1 Step 1b's `TestQualifiedAllConstrainedOwnClauseFirst`.
  - Revision 3 resolved the `e1e` layout, a tools file before `b_linux.go` and `b_other.go`, because it took `b_linux.go` for unconstrained. Reading the suffix gives that up, back to `main`'s output, in exchange for fixing R15. Telling the two apart would mean evaluating constraints, which is out of scope.
- **Same-clause files are still read.** A `//go:build ignore` file sharing the package's clause is still merged and searched (deliberate, as in PR #129). So is a `_…`/`.…` file sharing the clause, because the broader `parsePackageDir` option is out of scope. A stale `_old.go` that declares one of the package's structs and sorts first can therefore still stand in for it, as on `main`.
- **The GOOS/GOARCH lists are a copy.** `knownOS` and `knownArch` copy Go 1.27.2's `internal/syslist`, which go/build exposes no way to read. A port Go adds later is missing from them, so a file suffixed with it counts as unconstrained, and R15's regression can recur for that port alone until the copy is updated.
- Registration methods and helper functions (`findMethodDecl`, `findPackageFuncDecl`) are still looked up in every file of their directory.
- The non-struct request warning says "is not a struct" for any unresolvable request type. This is pre-existing, and its wording belongs to #120's PR.

## Open decisions (recommended default first)

1. **Pin the delegate row (R9) with an analyzer unit test only, not in the golden.**
   - Default: a unit test (`TestDelegateBehindGenerator`). The brief scopes the golden to field, embed, payload, request and `WrapL`, and a delegate fixture would need a second handler package.
   - Alternative: add a `b.Handler` delegate to `importable_clause`.
2. **Replace the removed README parenthetical "(struct lookups still scan every file)" with the precise statement of what is still unfiltered.** That statement is the R13 sentence plus "A registration method or helper function is looked up in every file of its directory" (from `findMethodDecl`/`findPackageFuncDecl`). Both go beyond the brief's literal ask.
   - Default: yes (Task 6 Step 2), so the Known-limitations list stays truthful.
   - Alternative: delete only the parenthetical and the #122 exception, dropping both sentences.
3. **Doc fixes outside #122 (review finding 4): ship them as a separate `docs:` change.** No user message asks to bundle them with this fix, and neither the issue nor its adopted defaults mention them. The texts are in Appendix C.
   - Default: a separate `docs:` commit or PR after this one. In it:
     - CLAUDE.md:99's 1.25-floor rule stays until the maintainer signs off on replacing it, because it is a policy change, not doc rot.
     - Doctor's `minGoVersion = "go1.25"`, its comment, `doctor_test.go:55`'s comment and `marshaler.go:109` move with README's Go line, so README and doctor never disagree. Otherwise README's Go line is left alone.
   - Alternative: the maintainer explicitly asks for the bundle here. Then re-add Appendix C as Task 6 Steps 4-7 and a commit "Docs:" paragraph, and still get sign-off for CLAUDE.md:99.
4. **Keep the equivalent `if !ok` early return in `resolveQualifiedStruct`.**
   - Default: keep it. It mirrors `inModuleTypeSite` and states the no-importable-file case, and M4 is documented as equivalent.
   - Alternative: `clause, _, sure := packageClause(files)`, which lowers complexity from 10 to 9.
5. **Keep `b.Money` in the golden although it warns.**
   - Default: keep it. The brief lists it, and it shows the Marshaler path is reached through the fixed import.
   - Alternative: drop it so the fixture is `--strict`-clean.
6. **BLOCKING, answer in writing before Task 1: amend two adopted defaults.** The brief adopts the tie-break "as in #129" and an unconditional `importableClause` filter on `resolveQualifiedStruct`. Taken as written, the two regress valid code that `main` resolves, on the generated-output and doctor/validation SemVer surfaces (Appendix B). The brief's unanswered "Maintainer: please confirm" about the filter is exactly this. Revision 5: the body regresses valid code too, in one narrower layout (R16), so no option is both measured and regression-free.
   - **(a) Default: the plan body (revision 4 called it (a1+)).**
     - What it changes:
       - `packageClause` skips the files go never builds (`neverImportable`) and prefers a file with no build constraint (no `//go:build` line, no GOOS/GOARCH suffix: `osArchSuffixed`), which changes PR 129's tie-break.
       - Because `importableClause` wraps it, `inModuleTypeSite` follows the new rule. On valid code that improves results (Impact item 2), except behind a sorted-first `_…` or `.…` file of the package's own clause in R16 (Impact item 7).
       - The struct filter applies only when the clause is sure (`inPackage`).
     - Measured result: R16 regresses (`advrev/x1`: `Warnings: 0` → `4`, requestBody and `created` lost, and the same for its `.old.go` and `a_windows.go` variants; `x2`: `2` → `4`; an earlier import of a real package of the stand-in's clause collides, `regrev8/fx/coll`: `0` → `3`). Every other measured layout is byte-identical to `main` where `main` resolves (R11-R15) or improves (R17), and every gate is green on the prototype. No test pins R16 yet (Task 1 Step 1b adds one).
     - Release: by RELEASING.md, a regression on valid code is breaking, so the commit is `fix(analyzer)!:` with a `BREAKING CHANGE:` footer (Task 8), and the release is MINOR while on 0.x.
     - Cost: it amends both adopted defaults, adds four small functions and a copy of go/build's GOOS/GOARCH lists, and lets the struct loop differ from `inModuleTypeSite` in the not-sure case.
   - **(a') The body with a changed not-sure fallback, not prototyped.** The goal is to keep `main`'s answer in R16 and ship as `fix`, PATCH. `main` has two rules that differ: `unaliasedImportName` takes the first file's clause, and PR 129's `importableClause` takes the first file that is neither `package main` nor build-ignored. One candidate, unmeasured: when the clause is not sure, take the clause of the first file whose clause is also that of a file go can build. `unaliasedImportName` would read it from any file and `inModuleTypeSite` would skip `package main` and build-ignored files first, so each consumer keeps its `main` answer whenever that answer names a buildable file. Reasoned on paper only: a single rule for both consumers loses some all-constrained layout to one of them (an ignored `package tools` file before the real file and a `//go:build tools` file). The plan's history (revisions 2-4 each regressed a layout found later) means this needs a later plan revision with a scratch prototype measured on every fixture before Task 1.
   - **(b) The adopted defaults as written, plus `neverImportable`.** Review finding 1's fix applies under every option. This option ships a regression on valid code: aliased struct references behind a sorted-first `//go:build tools` file stop resolving (R11 and R13 layouts), and R16 regresses further than under (a) (`Warnings: 5` each, measured on the `rev4/fixb` prototype rebuilt in revision 5). Per RELEASING.md it is marked breaking (`fix(analyzer)!:` with a footer), so the release is MINOR. It needs the maintainer's written sign-off. Deltas are in Appendix B.
   - **Rejected variants, recorded here:**
     - **(a1), revision 2's Appendix A1 without `inPackage`.** It regresses R13 (scratch `allcons`: `Warnings: 1` → `3`, requestBody and `created` lost), as mutation M10 shows.
     - **(a2'), a struct filter that only ever skips files.** It never clause-filters: it skips `neverImportable` files and build-ignored files of another clause. Its struct filter regresses nothing, but it keeps the body's tie-break (so R16 regresses as under (a)), drops the adopted filter everywhere, and a same-named struct in a sorted-first `//go:build tools` file would still stand in for the package's. Not prototyped.
     - **Revision 3's body, (a1+) without `osArchSuffixed`.** It regresses R15 (`e1`: `Warnings: 0` → `2`, aliased `bb.Addr` and the requestBody become the stand-in, `created` lost silently; `e1d`: `2` → `4`), as mutation M16 shows.
7. **Read GOOS/GOARCH file-name suffixes rather than document R15 (review finding 2, revision 4).**
   - Default: read them (`osArchSuffixed`). This avoids the R15 regression. (Revision 4 said it kept the no-regression claim true and the commit a plain `fix`; R16 has since made both false.) It costs about 30 lines: the helper, a copy of go/build's GOOS/GOARCH lists that can go stale (Residuals), and `TestOSArchSuffixed`. It also gives up revision 3's accidental `e1e` improvement (R13).
   - Alternative: drop `osArchSuffixed`, its maps, `TestOSArchSuffixed` and the `z_plan9.go` row of `TestQualifiedAllConstrainedStandIns`. The R15 layouts then regress against `main` (`e1`, `e1d`). By the plan's own rule that makes the commit `fix(analyzer)!:` with a footer, a MINOR release, and needs the carve-out in the Goal, the Impact closing line and the README.
8. **Keep the unresolvable-type warning's first-file cause, reworded, rather than drop it (review finding 5, revision 6).** The triage comment's adopted default removes "an unaliased in-module import whose directory's first file has another package clause" from `unresolvableFieldWarning` (`resolution.go:38-39`). Under every option of decision 6, R13 still names the package after a file of another clause, and so does R16 under (a) and (b) ((a') aims to keep `main`'s answer there). Their unaliased references and their named non-struct types under either import still emit this warning, and once the clause is gone none of the causes it lists is true (`gates5/r13`, `advrev/log_x1_rev4.txt`). On `main`, R13's unaliased references got a warning that named the real cause (R16's resolved). The relayed question at the top asks this next to decision 6. It never blocks: a reply that does not answer it means the alternative.
   - Recommended default (needs the user's word, because it amends an adopted default): reword the clause to "an in-module package whose buildable files all carry a build constraint and whose first one by name has another package clause". It covers aliased named non-struct types too (`bb.Cents` in R13 and R16), which `main`'s "an unaliased in-module import" wording never did. Task 4 Step 0a pins it in `TestQualifiedAllConstrainedToolsFirst` (M22). Prototyped in `rev6/fixed8`: tests green, lint `0 issues`, and the R13 fixture prints the new text with `Warnings: 4`, as on `main`. Under (b) the clause loses "whose buildable files all carry a build constraint and".
   - Alternative, and the outcome when the reply is silent on it: drop the clause as adopted (the two-line constant in "Design → Warning text"). No pin.

## Self-review: acceptance criteria → steps

| Brief item | Where |
|---|---|
| Reuse `importableClause`/`buildIgnored` in `unaliasedImportName`, `filepath.Base` fallback | Task 2; `TestUnaliasedImportName` |
| Tie-break: first sorted qualifying file | amended (decision 6): the first unconstrained file (no `//go:build` line, no GOOS/GOARCH suffix), else PR 129's rule; files go never builds are skipped. Task 3; M5-M9, M15-M21 |
| Filter `resolveQualifiedStruct` by `importableClause` | amended (decision 6): filtered when the clause is sure. Task 3; `TestQualifiedStructImportablePackage`, `TestQualifiedToolsFileSortsFirst`, `TestQualifiedAllConstrainedToolsFirst`, `TestQualifiedAllConstrainedStandIns`; M2, M10, M11, M13, M14 |
| Unaliased twin of `TestQualifiedBuildTaggedVariants` | Task 1 (`…Unaliased`, dir `width` ≠ clause `w`) |
| Golden covering field, embed, payload, request, `WrapL` | Task 5 (`importable_clause`) |
| Keep `pkgname_mismatch` green | Task 7 (M3 shows it guards the declared-name path) |
| Remove README.md:305-307, :399 and the "struct lookups still scan every file" note at :396 | Task 6 Steps 1-2 |
| Remove the warning clause at `resolution.go:38-39` | amended only if the user says so (decision 8): reworded to the residual R13 and R16 still reach; otherwise removed as adopted. Task 4; M22 |
| Out of scope: request-time wording, #123's leftovers | Global Constraints |
| No doc fixes outside #122 | Global Constraints; decision 3; Appendix C |

## Review dispositions (revision 8)

The gate, behavior and regression reviews of revision 7 ran with nothing on the branch, for the fourth cycle in a row: `fix/importable-package-clause`, `main` and `origin/main` were all `033a9c9`, with this plan untracked. The fixer re-checked that state and changed plan text only. No gate was re-run: with no code diff, `make check`, `go test -race ./...` and `gocognit` would only re-validate `main`. The commit message was extracted from Task 8 and checked: `grep -n '#'` prints only `Closes #122`, the subject is 69 characters, and every body line but the single-line `BREAKING CHANGE:` footer is at most 72 columns. The PR body is 147 words without its headings and attribution line.

| # | Finding | Disposition |
|---|---|---|
| 1, 3, 5 (blocker) | The branch has no implementation and no commit; the loop cannot progress until the user answers decisions 6 and 8 | **Valid, not fixable by the fixer.** Same as revision 7's findings 1, 3 and 5: the relayed request picks none of (a), (a') or (b), and no agent message counts. No `--amend` (it would rewrite `origin/main`'s `033a9c9`) and no plan-only commit. The plan stays untracked; the orchestrator relays the question at the top word for word and pauses the loop |
| 2 (minor) | The R16 disclosure names only `_` and `//go:build ignore` for the skipped file and `//go:build tools` for the other one | **Applied, disclosure only.** Reproduced with the reviewer's binaries (`beh8/run.sh x1dot`, `x1win`): `033a9c9` `Warnings: 0`, `--strict` exit 0; plan `Warnings: 4`, `--strict` exit 1, with aliased `bb.Cents` falling back behind `.old.go` too. Follows from `neverImportable` (skips `.` names) and `osArchSuffixed` (`a_windows.go` is constrained). New R16 row; the question, Impact item 7, decision 6, the commit's "Regression:" paragraph and footer, and the PR body updated |
| 4 (minor) | Under R16, an earlier unaliased import of a real package whose clause is the stand-in's also stops resolving (`fileImports` later write wins) | **Applied, disclosure only; code unchanged.** Reproduced (`regrev8/bin/{base,plan}` on `regrev8/fx/coll`): `Warnings: 0` → `3`, requestBody and both components lost. Import order reversed (`rev8/coll_rev`): only `b.Addr` breaks, `Warnings: 1`. Without `_old.go` (`rev8/coll_r13`), `033a9c9` and the plan both give the three warnings, so this is one more layout reaching an existing collision, not a new class. New R16 row; Impact item 7, the commit's "Regression:" paragraph and footer, and the question updated. The optional Step 1b test row was not added: it is not prototyped, and Step 1b itself still awaits decision 6 |

## Review dispositions (revision 7)

The gate, behavior and regression reviews of revision 6 again ran with nothing on the branch: `fix/importable-package-clause`, `main` and `origin/main` were all `033a9c9`, with this plan untracked. The fixer re-checked that state (`git rev-parse HEAD origin/main`, `git log origin/main..HEAD` empty, `git status --short` showing only this plan) and changed plan text only. No gate was re-run: with no code diff, `make check` and `go test -race ./...` would only re-validate `main`.

| # | Finding | Disposition |
|---|---|---|
| 1, 3, 5 (blocker) | The branch has no implementation and no commit; the review/fix loop cannot progress until the user answers decisions 6 and 8 | **Not fixable by the fixer.** Same as revision 6's findings 1 and 4: the relayed request ("start 122 and let's /triage the ones that need it") picks none of (a), (a') or (b), and no agent message counts. No `--amend` (it would rewrite `origin/main`'s `033a9c9`) and no plan-only commit (it would carry `Closes #122` without a fix or break the one-commit rule). The plan stays untracked; the orchestrator relays the question at the top word for word |
| 2 (major) | R16: the body regresses an all-constrained package whose never-built own-clause file sorts before a `//go:build tools` file | **No change.** Real, and already documented (Impact item 7, Residuals, the `BREAKING CHANGE:` footer) and pinned under every option by Task 1 Step 1b. The reviewer's `regrev/repo/internal/analyzer/zz_r16_test.go` observed Step 1b's (a) assertions passing on `rev6/fixed8`; the "inferred" caveat is dropped when Step 1b is implemented. Decision 6 settles it |
| 4 (minor) | A struct that newly resolves through a fixed import can take a bare component name, silently renaming another package's same-named struct (`Addr` → `ModAddr`) | **Applied, disclosure only; code unchanged.** Confirmed against `schemaKey` (`analyzer.go:3505`, first registered wins) and `regrev/fx/swap`: `033a9c9` `Addr: {city}`, `Warnings: 1`; plan `Addr: {street}`, `ModAddr: {city}`, `Warnings: 0`; `swaprev` (other order) keeps `Addr: {city}` and adds `BAddr`. One sentence each in Impact item 2, the README block of Task 6 Step 2 (matching the README's existing `BMoney` → `Money` precedent) and the commit's second paragraph, inserted before decision 8's sentence so that sentence stays last; re-wrapped to 72 columns, and `grep '#'` on the message still prints only `Closes #122`. The PR body is unchanged (near its word cap; the commit body is the tracked home) |

## Review dispositions (revision 6)

The gate, behavior and adversarial reviews of revision 5 again ran with nothing on the branch: `fix/importable-package-clause`, `main` and `origin/main` were all `033a9c9`, with this plan untracked. The fixer re-ran each finding in `scratchpad/issue122/rev6` before applying it: fresh `033a9c9` (`rev6/base`) and plan (`rev6/repo`) binaries, the fix applied in `rev6/fixed` and `rev6/fixed8`, `go list`/`go vet`/`go build` on `rev6/valid`, and `rev6/brute`. One code block and its test changed; nothing was implemented or committed.

| # | Finding | Disposition |
|---|---|---|
| 1, 4 (blocker) | The branch has no implementation and no commit; Task 1 Step 0 waits on decision 6 | **Not fixable by the fixer.** The user's relayed request ("start 122 and let's /triage the ones that need it") does not pick (a), (a') or (b), and no agent message counts. The plan stays untracked. There is no commit to amend: `git commit --amend` on this branch would rewrite `033a9c9`, `origin/main`'s own commit, and a plan-only commit would carry `Closes #122` without a fix or break the one-commit rule. The orchestrator relays the question at the top, which now also asks decision 8 |
| 2 (major), 3 (minor) | `osArchSuffixed` does not drop a trailing `_test` element as `goodOSArchFile` does, so a read file such as `z_windows_test.pb.go` of another clause becomes the sure clause: R15 again, and the parity claim, the helper comment and Impact's "no other regression" are false | **Applied.** Reproduced on `rev6/fx_suffix_test`: `033a9c9` `Warnings: 0`, strict exit 0; plan `Warnings: 2`, strict exit 1, `Addr` the stand-in's `{wrong}` behind the aliased field and the requestBody, `created` lost. `go list` confirms the file is Windows-only. With `name = strings.TrimSuffix(name, "_test")` the output is byte-identical to `033a9c9`'s, tests and lint are green, `osArchSuffixed` stays at gocognit 2, and the brute-force disagreement with `MatchFile` drops from 32 of 209 names to 0. New `TestOSArchSuffixed` row, M21, an R15 row; the helper comment and the parity paragraph are corrected. With the fix, Impact's closing line holds as written |
| 5 (minor) | Dropping the warning's first-file cause leaves R13 and R16 with a warning whose listed causes are all false | **Raised as open decision 8, default prototyped.** Reproduced on `gates5/r13`: `033a9c9` names the cause, the plan's text names none, `Warnings: 4` both. The finding's wording said "an unaliased in-module import", but aliased `bb.Cents` hits the same cause, so the proposed wording names the package instead. Not edited in unilaterally: Task 4 implements whichever answer the user gives, and silence means the adopted default (drop). The relayed question, Task 1 Step 0, Task 4, the Warning text section, Impact item 4, the commit's second paragraph and the self-review follow it |
| 6 (minor) | A splice artifact left a 29-character line in the commit body's first paragraph | **Applied.** The first paragraph is re-wrapped to 72 columns (the second too, after decision 8's sentence). `grep '#'` on the message still prints only `Closes #122`, and no body line exceeds 72 columns except the `BREAKING CHANGE:` footer, which stays one line |

## Review dispositions (revision 5)

The gate, behavior and adversarial reviews ran on the branch before anything was implemented: `fix/importable-package-clause` was at `033a9c9` with only this plan, untracked, because Task 1 Step 0 waits on decision 6. They tested revision 4's prototype (`rev4/repo`) instead. Each finding was re-run before it was applied: `advrev/base` and `advrev/rev4` on `advrev/x1`, `x2` (`go list` and `go vet` on `advrev/valid_x1`, `valid_x2`), `behav/run.sh` on `behav/fx/r16`, `r16w`, `mid`, and a rebuilt `rev4/fixb` binary for (b)'s R16 numbers (`scratchpad/issue122/verify`). No code or test block changed; nothing was implemented or committed.

| # | Finding | Disposition |
|---|---|---|
| 1 (minor) | The branch has no implementation, so the behavior pass tested the prototype, not the branch | **No action.** Correct, and not a defect of the plan: Task 1 Step 0 blocks until decision 6 is answered. It restates finding 5 |
| 2 (minor) | The README block, R13 and the commit's "Residual:" paragraph claim only files of the clause are searched, but when every file is constrained a `//go:build tools` or `x_windows.go` file of another clause is searched wherever it sorts, and its same-named struct stands in | **Applied.** Reproduced: `r16` and `r16w` emit `Addr: {wrong}` before and after, `Warnings: 0`, `--strict` exit 0. The README block now says the clause filters structs only when an unconstrained file named it, and that a stand-in of another clause wins wherever it sorts otherwise. R13 and the commit paragraph match, and a fourth R13 row records the layout |
| 3 (minor) | Impact says only projects with a sorted-first stray file are affected, but the struct filter also changes a generator sorted between two real files | **Applied.** Reproduced: `mid` goes from `Addr: {wrong}` to `{street}`, `Warnings: 0` both. Impact's scope line and item 2 now say so, and row R17 records it |
| 4 (major) | A never-built file of the package's own clause sorted before a `//go:build tools` file, in an all-constrained package, regresses valid code; this falsifies "no measured regression", Impact item 2, R13, and decision 6's one-line question | **Applied, option (a) of the finding, as plan text.** Reproduced: `x1` `Warnings: 0` → `4` (requestBody and `created` lost, aliased `bb.Cents` falls back), `x2` `2` → `4`; (b) gives `5` each. R16 rows, Impact item 7, an R16 residual, the corrected R13, Goal, decision 6 and its one-line question, and a `fix(analyzer)!:` commit with a `BREAKING CHANGE:` footer. Task 1 Step 1b pins R16 under every option. The finding's option (b), a changed fallback, is recorded as decision 6 (a'), **not prototyped**: revisions 2-4 each shipped an untested rule that a later fixture broke |
| 5 (blocker) | Nothing to review: no commit, no source change | **Not fixable by the fixer.** Implementing needs the user's written answer to decision 6, which no agent message supplies, and the question the user would have answered was false until this revision. The plan stays untracked: committing it alone would either carry `Closes #122` without a fix or break the one-commit rule |

## Review dispositions (revision 4)

Each finding was checked against the rev3 prototype (`rev3/repo`, a copy of the plan's code on `033a9c9`) before it was applied:
- The mutation runner (`gosem4/mut.py`) was re-run on a fresh copy.
- Binaries built from `033a9c9`, revision 3 and revision 4 were run on the reviewers' fixtures (`gosem4/fx/e1`-`e5`) and on new ones (`rev4/fx/e1b`-`e1e`).
- `goodOSArchFile` and `internal/syslist` were read in `$GOROOT` (Go 1.27.2).
- The adopted defaults were re-read in issue 122's re-triage comment.

None is rejected.

| # | Finding | Disposition |
|---|---|---|
| 1 (major) | `neverImportable`'s `package main` term, and `inPackage`'s `!buildIgnored` and `!neverImportable` terms in its not-sure branch, can each be removed with no test failing, so the PR's "every revert fails a test" claim is false | **Applied.** Reproduced: all three mutants survived `go test ./internal/analyzer ./internal/spectest` on revision 3. The new `TestQualifiedAllConstrainedStandIns` gives each a row where a stand-in declares its own `Addr` beside a constrained real file. `aaa_gen.go` (`//go:build generate`, `package main`) is the only layout where the `package main` term decides anything; every other `package main` file in the tests is also `//go:build ignore`. The mutants are M15, M13 and M14, and each now fails. The real file is `types.go`, not `b.go`, so `doc.go` sorts first. The Coverage strategy now says line coverage alone did not pin these guards. The PR's Verification line reads "every behavior-changing revert", because M4 is a documented equivalent mutant |
| 2 (major) | `packageClause` ignores GOOS/GOARCH file-name suffixes, so a suffixed file of another clause sorting after a constrained real file becomes the sure clause. Under every import this regresses valid code, which `main` gets right, and it contradicts "no measured layout regresses" and the `fix`-not-`fix!` argument | **Applied, option (a).** Reproduced: `e1` gave `Warnings: 0` on `033a9c9` and `2` on revision 3 (aliased `bb.Addr` and the requestBody became the stand-in, `created` was lost). `osArchSuffixed` mirrors `goodOSArchFile` over a local copy of `internal/syslist`. With it, `e1` is byte-identical to `033a9c9`'s output (R15), pinned by the `z_plan9.go` row and `TestOSArchSuffixed` (M16-M20). The planner found that revision 3 also regressed `e1b` (`created` lost) and `e1d` (`Warnings: 2` → `4`). Revision 4 matches `main` on `e1b` and improves `e1c` and `e1d` to `Warnings: 0`. It gives up revision 3's improvement on `e1e`, which is back to `main`'s output (R13). The residual is rewritten, and the README, Goal, Impact and commit text now name the suffix. Option (b) is kept as open decision 7's alternative |
| 3 (major) | The same three mutants as finding 1, with a table-test fix | **Applied**, merged with finding 1. The suggested rows (`_old.go`, `aaa_tool.go`, `aaa_gen.go`) are used as given, plus `doc.go` and `z_plan9.go`. The test asserts `$Addr`, `{street}`, `Package == "b"`, `unaliasedImportName == "b"` and no warnings. MA, MB and MC are M14, M13 and M15 |
| 4 (major) | The same two `inPackage` mutants, with a `doc.go` and `_tools.go` variant | **Applied**, merged with finding 1. The `doc.go` row is in the table, and `_old.go` covers the `_` case. A behavior row (R14) records that the all-constrained stand-in case is fixed |
| 5 (major) | Decision 6 changes two adopted defaults, and no agent message counts as the user's answer, so the implementer either stalls at Task 1 Step 0 or ships a default the user never approved | **Applied.** Checked: the re-triage comment's "Adopted defaults (ready-for-agent)" names the tie-break "as in #129" and the unconditional filter. The header now carries the one-line question for the orchestrator to relay, and the planner's return value lists it first. The planner cannot answer it; the gate in Task 1 Step 0 is unchanged. The question was framed after finding 2 was fixed, because "(a1+): no regression" was false until then |

## Review dispositions (revision 3, history)

Each finding was checked before being applied:
- against `$GOROOT/src/go/build/build.go` (`:568-572`, `:906`, `:942`);
- against `parsePackageDir` (`analyzer.go:3690`, which reads `_…`/`.…` files);
- against issue 122's body and comments (no doc item in the adopted defaults);
- against `origin/main` `go.mod`, `CLAUDE.md:96`/`:99`, `README.md:199-201` and `doctor.go:34-41`.

The reviewer's fixtures were also re-run with binaries built from `033a9c9`, from revision 2 and from revision 3. None is rejected. Two are applied with a correction, and one alternative is not taken.

| # | Finding | Disposition |
|---|---|---|
| 1 (blocker) | go/build never builds `_…`, `.…` and `package documentation` files; with the struct filter they hide a valid package's structs under an aliased import, and every disclosure understated the scope | **Applied.** Reproduced: `fx/under` went from `Warnings: 0` to `5` under revision 2 and `fx/dot` from `0` to `2`. Both are byte-identical to `033a9c9`'s output under revision 3. `neverImportable` sits in the clause loop, so `inModuleTypeSite` inherits it. It is pinned by three `widthCases`, the `skipped` row of `TestUnaliasedImportName` and the `TestQualifiedToolsFileSortsFirst` table (M7-M9). The README, commit, Impact and decision 6 texts are rewritten, and the false A1.1 premise is gone. The broader `parsePackageDir` option is **not taken** (scope; see Out of scope and Residuals) |
| 2 (blocker) | Put Appendix A1 in the body, or require written sign-off on (b) | **Applied, with a correction.** A1 itself regresses when every file of the package is constrained. Scratch `allcons`: `033a9c9` `Warnings: 1`, A1 `3`, with the requestBody and the promoted `created` lost. The body is therefore A1 + `neverImportable` + clause certainty (`packageClause`/`inPackage`), which keeps `allcons` byte-identical to `033a9c9` (R13, M10). It uses the A1.4-style message: no `!`, no footer, PATCH. The R11b rows are reclassified rather than dropped, because their "Before" column still documents `main`. (b) needs written sign-off (Task 1 Step 0) |
| 3 (blocker) | Decision 6 is a choice about the brief; don't let (b) ship by inertia | **Applied.** Decision 6 is reframed as amending two adopted defaults, and Task 1 Step 0 requires a written answer. (b) moves to Appendix B. The same correction as finding 2 applies: "A1 is the only option with no regression" was false; (a1+) is |
| 4 (major) | Four doc edits unrelated to #122 ride in the fix, including a policy change to CLAUDE.md:99, and README would disagree with doctor | **Applied.** The facts were verified (`go.mod` `go 1.26.0` since `354b696`; CLAUDE.md:99 verbatim; no tracking issue). Task 6 Steps 3-6 and the commit's "Docs:" paragraph are removed, and the texts are kept in Appendix C for a separate `docs:` change. Global Constraints and Tech Stack are restored to Go 1.25, and decision 3 now covers the floor policy and the README/doctor agreement |

The planner added the following; no finding raised them:
- the `allcons` regression and its fix;
- the R12 rows: named types behind `_…`/`doc.go` files were already wrong on `main`, measured on scratch `rev3/fx/r12` (`Warnings: 3` → `0`);
- mutations M10-M12.

### Revision 2 dispositions (history)

All three findings described the R11b regression, and none was rejected. Revision 2 corrected the R11 count (12 → 14), added the delegate row and the `--strict`/doctor effects, added `TestQualifiedToolsFileSortsFirst`, and prototyped Appendix A1 as decision 6 (a1). Revision 3 supersedes its default (b) and folds A1 into the body.

## Appendix B: decision 6 (b), only with the maintainer's written sign-off

Deltas to the body:
- **Code.**
  - `importableClause` keeps PR 129's loop, with `neverImportable(p, f)` in place of the `f.Name.Name != mainPackageName` test. This is the reviewer's `coldsem/fixb` prototype: its `resolution.go` differs from revision 2's only by that helper and its use.
  - There is no `packageClause`, no `inPackage` and no `osArchSuffixed` (with its `knownOS`/`knownArch` maps). The tie-break is the first sorted qualifying file, so whether a file is constrained never matters.
  - `resolveQualifiedStruct` uses `clause, ok := importableClause(files)`, filters with `if file.Name.Name != clause { continue }` and returns `pkg: clause` (revision 2's code).
- **Tests.**
  - Drop the "a tools-tagged file of another clause sorts first" `widthCase`.
  - In `TestUnaliasedImportName`, the `toolsfirst` row expects `tools`.
  - In `TestQualifiedToolsFileSortsFirst`, the `aaa_tools.go` subtest takes revision 2's file (no stand-in) and revision 2's regression assertions: `aliased` is `bb.Addr`, `unaliased` is `b.Addr`, there is no `created` and no `Addr`, and each field has one warning. The other three subtests stay as in the body.
  - Drop `TestQualifiedAllConstrainedToolsFirst`, or invert it to pin the regression.
  - Drop `TestOSArchSuffixed`. Keep `TestQualifiedAllConstrainedStandIns` unchanged: all five rows pass on the (b) prototype (`coldsem/fixb` plus the test, run as `rev4/fixb`).
  - Mutations M5, M6, M10-M14 and M16-M20 do not apply. M15 still applies to `neverImportable` (named `goToolIgnored` in `coldsem/fixb`), and `StandIns/aaa_gen.go` is expected to fail it; that mutant was not run under (b).
- **README Step 2.** Use revision 2's text: a file of another clause under any other constraint is not skipped, and when it sorts first its clause is taken for the package's, so the package's own structs and named types do not resolve under any import. Add `package documentation` and `_`/`.` names to the skipped files.
- **Commit.** Subject `fix(analyzer)!: resolve imported packages from their importable files`. The body gets revision 2's "Regression:" paragraph, scoped to files under a `//go:build` constraint other than `ignore`, and this footer:

  ```
  BREAKING CHANGE: in an imported in-module directory where a file of another package clause under a build constraint other than ignore (//go:build tools, package tools) sorts first by name, struct references through an aliased import no longer resolve: fields are untyped objects with a warning, embedded structs lose their promoted fields silently, payloads are untyped, requests get no requestBody, and route delegates declared there are not found, so generate --strict fails and doctor counts fewer routes.
  ```

  The release is MINOR, with a ⚠ note. After the Release PR opens, check that the breaking note in its body stops before `Closes` and the co-author trailer.
- **Impact.** Add the regression item:
  - a field becomes `{type: object}` with a warning;
  - an embed loses its promoted fields silently;
  - a payload becomes untyped;
  - a request loses its `requestBody`;
  - a delegate's routes vanish;
  - `--strict` newly fails, and doctor counts fewer routes.
- **Measured.** These numbers come from the revision 2 binary, which is (b) without `neverImportable`. None of these fixtures has a `_…`, `.…` or `package documentation` file, so (b) with it gives the same numbers:

  | Fixture | `033a9c9` | (b) |
  |---|---|---|
  | R1-R7 tools fixture | `Warnings: 12` | `Warnings: 14` |
  | R11 aggregate (`gosem/r11al`) | `Warnings: 0`; `--strict` exit 0; doctor 3 routes, 3/3 typed | `Warnings: 5`; `--strict` exit 1; doctor 2 routes, 1/2 typed; `Addr` and `Thing` components gone |
  | `coldtools/toolsfx2` (with `bb.Cents`) | `Warnings: 1` | `Warnings: 5` |
  | `review/tf` | `Warnings: 0` | `Warnings: 4` |
  | R13 `allcons` | `Warnings: 1` | `Warnings: 3` |

## Appendix C: doc fixes for a separate `docs:` change (not in this commit; decision 3)

All of these were verified at `033a9c9`:
- `go.mod` has declared `go 1.26.0` since `354b696` (2026-09-05, PR 62, `chore: update deps`). That is forced by `github.com/go-openapi/jsonpointer` v1.0.1, an indirect dependency via kin-openapi whose own `go` directive is `1.26.0`.
- `3aea87e`'s CHANGELOG line for `7ff8391` reads `closes [#116] [#117] [#113] [#114] [#123] [#111]`. Issue 116 was closed at 2026-10-09T17:26:41Z and reopened.

- **README.md:199-201.** Replace

  ```
  - **Go 1.25+** to build/run the tool. (Target projects on GoBricks v0.45.0+
    require Go 1.26+ to build *themselves*, but the tool only parses their
    source, so it doesn't inherit that requirement.)
  ```

  with

  ```
  - **Go 1.26+** to build/run the tool, as its `go.mod` requires (a
    dependency, `github.com/go-openapi/jsonpointer`, needs Go 1.26). The tool
    only parses a target project's source, so a target's own Go requirement
    never applies to it.
  ```

  In the same change, update `internal/commands/doctor.go:34-41` (`minGoVersion = "go1.25"` and its "single-sourced to match go.mod (go 1.25)" comment), `doctor_test.go:55`'s comment and `marshaler.go:109` ("below the go 1.25 floor"), so README and doctor agree.
- **CLAUDE.md:96, superseded in place.** Leave the fixture rule `go 1.25` at `:81` untouched.

  ```
  - > **SUPERSEDED (2026-10-09).** `go.mod` used to declare `go 1.25.0`. Since the 2026-09-05 dependency update it declares `go 1.26.0`, forced by `github.com/go-openapi/jsonpointer` v1.0.1 (an indirect dependency via kin-openapi) whose own `go` directive is 1.26.0, so building the tool needs Go 1.26+. There is still no `toolchain` directive, while CI's setup-go steps pin a newer Go.
  ```

- **CLAUDE.md:99.** This is a policy change: apply it only with the maintainer's sign-off.

  ```
  - > **SUPERSEDED (2026-10-09).** The 1.25 floor is gone: the effective floor to build is the `go.mod` `go` directive (1.26, above). Don't rely on language features newer than that directive.
  ```

- **CLAUDE.md "Commits & releases".** Insert after "Never merge a Release PR you are not ready to tag immediately.":

  ```
  - release-please copies every `#N` anywhere in a squash commit message into that commit's CHANGELOG line and the Release PR body as `closes [#N] [#M]…`.
  - When the Release PR merges, GitHub closes the first issue after each `closes`: merging the v0.4.0 Release PR closed issue 116, which a commit had only mentioned, and it had to be reopened.
  - So refer to an issue you are not closing without the hash (`issue 116`), and read a Release PR's body for `closes [#N]` before merging it.
  ```

- **.github/workflows/release.yml:135-136.** Replace

  ```
            # --release-notes makes the CHANGELOG.md section (release-please-owned) the
            # GitHub Release body; GoReleaser's own changelog is disabled in .goreleaser.yaml.
  ```

  with

  ```
            # --release-notes makes the CHANGELOG.md section (release-please-owned) the
            # GitHub Release body: with it, the changelog pipe uses that section verbatim.
  ```

  This matches `.goreleaser.yaml:54-58`, which forbids `changelog.disable`. Run `actionlint` if it is installed.

## Execution notes

Executed 2026-10-10 on `fix/importable-package-clause`, cut from `origin/main` at `033a9c9` (no anchor moved).

**Decisions (the user's written reply, "as suggested", to the relayed question and its stated recommendations):**
- Decision 6: **(a)**, the plan body as written; the commit is `fix(analyzer)!:` with the R16 `BREAKING CHANGE:` footer.
- Decision 8: **keep the cause, reworded** ("an in-module package whose buildable files all carry a build constraint and whose first one by name has another package clause"), with Task 4 Step 0a's pin.
- Decision 3: the doc fixes of Appendix C are **not** bundled; they go in a separate `docs:` change. README's Go-version line, `CLAUDE.md`, `.github/`, `internal/commands/` and `marshaler.go` are untouched.
- Minor decisions 1, 2, 4 and 5: as the plan states them.

**Deviations:**
- **Task 1 Step 1b** was not prototyped by the planner. `TestQualifiedAllConstrainedOwnClauseFirst` is the regression reviewer's `regrev/repo/internal/analyzer/zz_r16_test.go`, with its `t.Logf` lines dropped, a doc comment added, and the three fall-back warnings checked for "resolves to no schema" instead of only counted. Its (a) assertions are now observed, not inferred: red on both rows at `033a9c9`, green on both after Task 3. After Task 2 alone the `a_gen.go` row already passes (PR 129's `importableClause` takes `tools` there too) and `_old.go` still fails; Task 2 Step 2's list does not mention this test.
- **Placement:** Step 1b and Task 3 Step 1a both say "right after `TestQualifiedAllConstrainedStandIns`". The order is `…StandIns`, `…OwnClauseFirst`, `TestOSArchSuffixed`, `TestDelegateBehindGenerator`.
- **goconst:** no hoist was needed; the pinned golangci-lint v2.14.0 reports `0 issues` with the Step 1b test's literals inline.
- **Mutations** ran on an `rsync` copy of the working tree in the session scratchpad (`exec/mut`), with `rev4/scripts/mut.py` plus M21 and M22, so no mutant ever touched the repo. Every row fails as tabled and M4 survives. `TestQualifiedAllConstrainedOwnClauseFirst` additionally fails M3, M6, M7 (`_old.go`), M10 and M12, beside M1 (both rows) as predicted.

**Gates** (local Go 1.27.2): `make fmt` changed nothing; `make check` exit 0 (lint `0 issues`); `go test -race -count=1 ./...` green; `make sec` scanned 22 files, 0 issues; `gocognit -over 15` on `analyzer.go`, `resolution.go` and `qualified_test.go` exits 0, with the budget table's numbers plus `TestQualifiedAllConstrainedOwnClauseFirst` at 11. `go test -coverprofile ./...` and the analyzer package's own profile both give 100.0% for `resolveQualifiedStruct`, `unaliasedImportName`, `importableClause`, `packageClause`, `neverImportable`, `osArchSuffixed`, `inPackage` and `inModuleTypeSite`. `analyzer.go` and `resolution.go` are byte-identical to the `rev6/fixed8` prototype's. Goldens: `-update` added only `importable_clause/expected.yaml` (297 lines, byte-identical to the prototype's); no existing golden moved. On the new fixture the CLI prints `Warnings: 1` (`b.Money`'s Marshaler warning) and doctor counts 5 routes, 5/5 typed (R8).

**Post-review edits (orchestrator, after the verify loop):** three minor findings applied, text only. Row R10's "After" cell now quotes the warning as decision 8 rewords it. The README "Build constraints" bullet, its copy in Task 6 and the commit's "Residual:" sentence now say a same-named struct of another clause stands in only when its file sorts before the one declaring the package's own, whichever clause named the package (probe `issue122/fx_standin`: `a.go` own clause, `m_tools.go` stand-in, `z.go` real struct gives `$ref ToolsAddr`; `z.go` sorting first resolves the real one; `origin/main` behaves the same). The commit body and `BREAKING CHANGE:` footer backtick `<Pkg><Name>`, which GitHub's renderer otherwise strips from release notes (as it did in the 0.4.0 CHANGELOG section).
