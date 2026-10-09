# Qualified Named Types, PR 2 of ADR 0002 (#100) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A named non-struct type declared in **another package of the same module** (`b.Cents`, `b.Tags`, `kinds.Labels`, `money.Cents` through an import alias) resolves in its declaring package and emits exactly what the same declaration made locally emits after PR 1, in every field position and as a parameter. Today (PR 1 head) every such field is `object` plus PR 1's "resolves to no schema" warning. Closes #100.

**Architecture:** No new carrier and no consumer change. PR 1's resolver (`internal/analyzer/resolution.go`) gains one lookup, `inModuleTypeSite`, that maps a qualified name to the file declaring it in the imported in-module package. Wherever the resolver meets a qualified named leaf that is not a struct and not a stdlib/well-known name (`resolveQualified`, identity mode; `resolveUnderlying`'s qualified branch, underlying mode), it hands the **short** name to the existing `resolveLocal` **in that declaring file's context** (`c.in(site.file, site.path)`). Everything else falls out of PR 1's code: `localTypeDecls` collects every build-tagged declaration of the name in that package, `marshalerSet` scans that package, `resolveTypeSpecChain` finds that package's structs, `recordRef` records the declaring file as the ref site so `registerRefLeaf` registers b's `User` (not the route package's `User`), and `c.st.param` keeps the Marshaler-guard exemption for parameters. Three analyzer-private state changes make the reuse correct: the recursion stack is keyed by `(package dir, name)` (`typeKey`), warnings name another package's types clause-qualified (`display`, so `b.Tree contains itself`), and the #97 byte-slice rule is decided inside `resolveLocal` (where the element's own methods are known) via a `byteElem` context flag instead of a second `marshalerSet` call in the route file's context. The generator, `models`, the constraint module and every command are untouched.

**Tech Stack:** Go 1.25 language floor; `go/ast` static analysis only; goldens in `internal/spectest`.

**Spec (authoritative, in this order):** #100's triage brief (the only comment on #100, 2026-09-29; there are no dated amendments). The original issue body is superseded where they disagree (its "route map values through slice-item typing" direction is dead: PR 1 types maps from the resolved form). Glossary: `CONTEXT.md` (**Shape**, **Resolution**, **Marshaler type**). Design: `docs/adr/0002-named-type-resolution.md`. PR 1: `docs/superpowers/plans/2026_10_08_named_type_resolution_pr1.md` (read its **Execution notes**) and the commit body of #128's squash commit `feeafaa` on `main` (identical to PR 1's branch head `deedfa9` apart from the `(#128)` subject suffix and the trailer's casing).

**Baseline for line anchors:** `main` at `feeafaa` (PR 1, #128, squash-merged 2026-10-09T00:27Z; unreleased, latest tag `v0.3.1`). `feeafaa` and PR 1's branch head `deedfa9` share tree `54516b4`, so every anchor, every "Before (`deedfa9`)" cell and every prototype result below (all taken on a `deedfa9` export) holds unchanged on `feeafaa`; `deedfa9` is kept below as that provenance. Re-check every anchor with `grep -n` before editing.

**Prototype evidence (planner, 2026-10-08):** every code and test block below was applied to a scratch export of `deedfa9` (`git archive`, outside the repo) and run: `go test ./...` green, the pinned golangci-lint v2.12.2 `0 issues` on `./...`, `gocognit -over 15` clean on every touched file, every function in `resolution.go` at 100% statement coverage in the analyzer package's own profile, and the mutation checks in Task 8 each failing the tests named there (lists re-run 2026-10-08 with the `require.Contains` guards and the `b.MStamp`/`LStamp` rows in place, so no panic masks a failure). The behavior table's "After" cells are that prototype's output; its "Before" cells are a `deedfa9` binary's output on the same fixture.

---

## Stale points in the brief (checked against `deedfa9`)

- **`resolveUnderlyingBuiltin`, `underlyingScalarBuiltin`, `localTypeUnderlyings`, `resolveNamedScalars` no longer exist.** PR 1 deleted them. The "second dotted-name gate" the brief names is now `resolveUnderlying`'s qualified branch (`resolution.go:366–377`); the first is `resolveQualified`'s unresolvable tail (`resolution.go:284–287`).
- **"PR 1's Resolution field on `models.FieldInfo`/`models.TypeInfo`":** `TypeInfo.Resolution` was removed late in PR 1 (commit body: "Payloads are not resolved yet; #110 adds that, and its TypeInfo field with it"). Only `FieldInfo.Resolution` exists; this PR fills it for fields only, as the brief says.
- **"Today … none warns":** true on `main 72f68f1`; on PR 1's head every body/param row warns "resolves to no schema" (or "is a defined type over b.Cents"). The Before column below is PR 1's head.
- **Fixture constraint "declare the structs that use `b`, `kinds` or `money` in the handlers' file":** written against `main`. PR 1's `structDeclSite` fixed that seam (#123), and `TestQualifiedInSiblingFile` (Task 1) pins it for qualified non-struct types. The golden fixture still follows the brief's layout (harmless).
- **"The only expected move is PR 1's `named_resolution` golden, if it carries PR 1's `b.Cents` row":** it does not. PR 1 put that row in **`named_fallback`** (`api.go:90`, `Cents types.Cents`, package `named_fallback/types`). That golden moves; see "Goldens" (open decision 1).
- **`TestResolveUnderlyingBuiltinBuildTaggedVariants`** was renamed `TestResolveLocalBuildTaggedVariants` (`analyzer_test.go:6740`) by PR 1.

---

## Global Constraints

- **Branch:** `fix/qualified-named-types` in the worktree `/Users/gaborage/Projects/gaborage/code/go-bricks-openapi-pr2`, moved onto `origin/main` (`feeafaa`) by Task 0 Step 1; the PR targets `main` directly (the remote `fix/named-type-resolution` is deleted, so there is no stack). Never touch the main checkout at `/Users/gaborage/Projects/gaborage/code/go-bricks-openapi`. Never push, open PRs or touch GitHub state from the implementation task.
- **Goldens:** only `go test ./internal/spectest -update` regenerates them, never `make update`. After `-update`, `git status --porcelain internal/spectest/testdata` may list **only** the new `qualified_named_types/` directory and the `named_fallback/` edits (`api.go`, `expected.yaml`, deleted `types/cents.go`). The `named_fallback/expected.yaml` diff must be exactly the removal of the two-line `cents:` / `type: object` property. Any other moved `expected.yaml` is a bug: fix the code, don't accept the diff. The golden test is `TestGoldenFixtures` (a `-run 'TestFixtures/…'` pattern matches nothing and exits 0).
- **Cognitive complexity ≤15 per function** (SonarCloud `go:S3776`; `make lint` checks only cyclomatic). Check with `go run github.com/uudashr/gocognit/cmd/gocognit@latest -over 15 <touched files>`. Pre-existing, untouched, do **not** "fix": `yamlNodeToJSONValue` (`internal/commands/generate.go:358`, 16, gocognit counts its recursion; Sonar does not) and the test `TestMarshalerGuardPositions` (16; Sonar excludes tests). A tree-wide `gocognit -over 15 internal cmd` therefore exits 1 before and after this PR.
- **Coverage ≥80% on new code**, from each package's **own** tests (no `-coverpkg`). All new production code is in `internal/analyzer`, so analyzer tests must cover it; spectest/commands tests and fixtures add nothing. `internal/models` is untouched.
- **`goconst` counts `_test.go` literals** (min-len 4, min-occurrences 3). The new tests use `"uint64"`, which pushes `builtins.go`'s literal over the threshold: add `goTypeUint64` (Task 2 Step 1). Reproduced with the pinned v2.12.2: `string \`uint64\` has 3 occurrences`.
- **No `t.Parallel()`.** Go 1.25 floor (`strings.Cut`, `slices.Sorted`, `maps.Keys` are all ≤1.23).
- **Settled invariants (CLAUDE.md), untouched:** `lookupStructTag`/`unquoteLiteral` stay the only tag/literal readers (the new lookup reads imports only through `fileImports`, which already uses `unquoteLiteral`); the Constraint set stays in `internal/generator/constraints.go`; example coercion stays in the `fieldInfoToProperty` wrapper; `referencedSchemaNames` is untouched; the uint `minimum: 0` pre-stamp is untouched (it is what gives `b.Big` its `minimum: 0`); `NOSONAR` and `//nolint` are not interchangeable (this PR adds neither).
- **Out of scope (do not implement):**
  - Payloads `Result[b.Cents]`, `Result[b.Tags]`: #110. Pinned unchanged by `TestQualifiedPayloadUnchanged`.
  - Plain struct fields promoted from another package whose refs resolve in the embedding package: #101 (PR 1 closes it).
  - Out-of-module types and well-known types under an aliased import (`t.Time`): stay a warned fallback by design. More stdlib types: #113.
  - Generic instantiations (`b.Box[int]`): #114 (they decode as `ShapeUnknown` and never reach the lookup).
  - `json.Number`/`time.Month`/`time.Weekday`: #89 (done).
  - The text-marshaler string rule, struct Marshaler types, embedded method promotion: #111.
  - Named non-struct request types: #102. Unbindable-param warnings (query `many []b.Cents`): #121.
  - `unaliasedImportName`'s first-sorted-file clause: #122. Do not fix it; the fixture avoids it (`gen.go` sorts after `b.go`), and the unit test that puts a `package main` generator first uses an **aliased** import.
  - Sibling-file qualified field types: #123 (PR 1).
  - `resolveQualifiedStruct` does not skip `package main` files (brief: "resolveQualifiedStruct does not"). Leave it; only the new lookup skips them.
- **Gates before the PR:** `make check` with the pinned golangci-lint (`make dev-deps`; `lint` refuses a mismatched binary); `go test ./internal/spectest`; gocognit on touched files; the coverage spot-check and mutation checks (Task 8).
- **Squash commit = the one commit.** Its body stands alone (Task 9). Personal repo: no `Refs:` line.

---

## Behavior rows: expected emitted YAML

Flow style for readability; goldens are block style with `OpenAPIProperty` key order (`type, allOf, properties, additionalProperties, format, …, $ref, items, minLength, maxLength, …, minimum, maximum, …, nullable`). "Before" is `deedfa9`; "After" is the prototype. Every "After" cell equals what the same declaration made in the route package emits after PR 1 (the brief's "as local"), apart from a struct leaf's component name.

| # | Field (declared in `b` unless noted) | Before (`deedfa9`) | After | Warns after? |
|---|---|---|---|---|
| Q1 | `b.Cents` (`int64`), `money.Cents` | `{type: object}` | `{type: integer, format: int64}` | no |
| Q1 | `b.Big` (`uint64`) | `{type: object}` | `{type: integer, format: int64, minimum: 0}` | no |
| Q1 | `b.Flag` (`bool`) | `{type: object}` | `{type: boolean}` | no |
| Q2 | `b.Code` (`string`) beside a local `type Code int32` | `{type: object}` | `{type: string}`; the local `Code` field stays `{type: integer, format: int32}` | no |
| Q3 | `b.Wrapped` (`Cents`); `b.Grade` (`c.Level`, `int32`); local `type Discount b.Cents`; local `Cents2 = b.Cents` | `{type: object}` | `b.Wrapped`, `Discount`, `Cents2`: `{type: integer, format: int64}`; `b.Grade`: `{type: integer, format: int32}` | no |
| Q3 | promoted from embedded `b.Base`: `Grade Grade` and `Level c.Level` | `{type: object}` each | `{type: integer, format: int32}` each | no |
| Q4 | `b.Cents validate:"min=1,max=100"` | `{type: object}` | `{type: integer, format: int64, minimum: 1, maximum: 100}` | no |
| Q4 | `b.Code validate:"min=2,max=8"` | `{type: object}` | `{type: string, minLength: 2, maxLength: 8}` | no |
| Q5 | `*b.Cents` | `{type: object, nullable: true}` | `{type: integer, format: int64, nullable: true}` | no |
| Q5 | `[]b.Cents` | `{type: array, items: {type: object}}` | `{type: array, items: {type: integer, format: int64}}` | no |
| Q5 | `[][]b.Cents` | `items: {type: array, items: {type: object}}` | `{type: array, items: {type: array, items: {type: integer, format: int64}}}` | no |
| Q5 | `map[string]b.Cents`, `map[string]*b.Cents`, `*map[string]b.Cents` | `{type: object, additionalProperties: {type: object}}` | `{type: object, additionalProperties: {type: integer, format: int64}}` | no |
| Q5 | `map[string][]b.Cents` | `additionalProperties: {type: array, items: {type: object}}` | `{type: object, additionalProperties: {type: array, items: {type: integer, format: int64}}}` | no |
| Q5 | `[]map[string]b.Cents` | `items: {type: object, additionalProperties: {type: object}}` | `{type: array, items: {type: object, additionalProperties: {type: integer, format: int64}}}` | no |
| Q6 | `b.Tags` (`[]string`), `money.Tags` | `{type: object}` | `{type: array, items: {type: string}}` | no |
| Q6 | `*b.Tags` | `{type: object, nullable: true}` | `{type: array, items: {type: string}}` (no `nullable`, = `*[]string`) | no |
| Q6 | `[]b.Tags` | `{type: array, items: {type: object}}` | `{type: array, items: {type: array, items: {type: string}}}` | no |
| Q6 | `map[string]b.Tags` | `additionalProperties: {type: object}` | `{type: object, additionalProperties: {type: array, items: {type: string}}}` | no |
| Q6 | `kinds.Labels` (`map[string]string`) | `{type: object}` | `{type: object, additionalProperties: {type: string}}` | no |
| Q7 | `b.Codes` (`[]Code`, b's `Code` is `string`) | `{type: object}` | `{type: array, items: {type: string}}` | no |
| Q7 | local `type Tags []b.Tags` | `{type: array, items: {type: object}}` | `{type: array, items: {type: array, items: {type: string}}}`, **no recursion warning** | no |
| Q8 | `b.UserList` (`[]User`) beside a local `User` struct | `{type: object}`; b's `User` registered only via a direct `b.User` field | `{type: array, items: {$ref: '#/components/schemas/User'}}`, the same component a direct `b.User` field references (b's); the local `User` keeps its own component, but **whichever `User` registers first takes the bare name** and the other becomes `<Pkg><Name>`. So when `b.UserList` registers b's `User` ahead of the local one, the local `User` is **renamed** from `User` (`deedfa9`, where only a direct `b.User` field registered b's) to `QualifiednamedtypesUser`, moving its `$ref`s; with the local field first, b's becomes `BUser` instead (both reproduced on the prototype, `scratchpad/pr2/disp/probe2`, `probe3`). The golden shows the resulting names (its direct `BUser b.User` field already ordered them that way on `deedfa9`); `TestQualifiedShadowing` is the no-direct-field case | no |
| Q8 | `b.Items` (`[]c.Item`; the route package never imports `c`) | `{type: object}`; c's `Item` never registered | `{type: array, items: {$ref: '#/components/schemas/Item'}}`; `Item` (c's) emitted | no |
| Q9 | `b.Stamp` (`= time.Time`) | `{type: object}` | `{type: string, format: date-time}` | no |
| Q9 | `b.Blob` (`[]byte`) | `{type: object}` | `{type: string, format: byte}` | no |
| Q9 | `b.Tree` (`map[string]Tree`) | `{type: object}` | `{type: object, additionalProperties: {}}` | **yes** (`b.Tree contains itself`) |
| Q9 | `b.Addr` (`uintptr`) | `{type: object}` | `{type: object}` | **yes** (uintptr warning only) |
| Q10 | `b.Status` (`int`; `MarshalText` in `b/status_text.go`), local `StatusA = b.Status` | `{type: object}` | `{}` | **yes** (`b.Status has its own MarshalText method`) |
| Q10 | `*b.PStatus` (`func (*PStatus) UnmarshalJSON`) | `{type: object, nullable: true}` | `{}` | **yes** (`b.PStatus has its own UnmarshalJSON method`) |
| Q10 | local `type StatusD b.Status` | `{type: object}` | `{type: integer, format: int64}` | no |
| Q11 | `b.Width` (`int64`/`int32` in build-tagged files of b) | `{type: object}` | `{type: integer}` (no `format`) | no |
| Q11 | `b.Cents` beside `b/gen.go` (`//go:build ignore`, `package main`, `type Cents int32`) | `{type: object}` | `{type: integer, format: int64}` | no |
| Q12 | path `code b.Code` | `{type: object}` | `{type: string}` | no |
| Q12 | query `min b.Cents` / `many []b.Cents` / `tags b.Tags` | `{type: object}` / `{type: array, items: {type: object}}` / `{type: object}` | `{type: integer, format: int64}` / `{type: array, items: {type: integer, format: int64}}` / `{type: array, items: {type: string}}` | no |
| Q12 | query `st b.Status` | `{type: object}` | `{type: integer, format: int64}` (parameters bind by kind) | no |
| Q13 | `scheduler.ScheduleType` (go-bricks), `decimal.Decimal`, `t.Time` (`t "time"`) | `{type: object}` | unchanged `{type: object}` | **yes**, unchanged (new text, see Warnings) |
| Q13 | payload `server.Result[b.Cents]` | `data: {type: object, description: Response data}` + `response type b.Cents resolves to no schema component` | unchanged | unchanged |

Rows that fall out of the mechanism and are pinned beyond the brief's table:

| # | Field | Before | After | Warns after? |
|---|---|---|---|---|
| Q14 | `[]b.FlagU` (`type FlagU byte`, only `func (*FlagU) UnmarshalJSON`), local `FU = b.FlagU` as `[]FU` | `{type: array, items: {type: object}}` / object | `{type: string, format: byte}` (byte-slice rule, as local) | no |
| Q14 | `[]b.FlagV` (`MarshalJSON`), local `FV = b.FlagV` as `[]FV` | object leaves | `{type: array, items: {}}` | **yes** |
| Q14 | `b.FlagU` direct; `[2]b.FlagU` | object | `{}`; `{type: array, items: {}}` (the rule is slices only) | **yes** |
| Q15 | `b.UA` (`= User`), `b.UD` (`type UD User`), local `type D b.UA` | `{type: object}` + warning | `{$ref: …/UA}`, `{$ref: …/UD}` (components registered in b under those names); `D` references `UA` (the local twin `type D UA2` would be its own `D` component: the allowed component-name difference) | no |
| Q16 | `b.Missing` (b declares no `Missing`); `zz.X` (no such import); `nope.X` (in-module path with no directory); `g.OnlyMain` (only a `package main` file in `g/` declares it) | `{type: object}` + warning | unchanged `{type: object}` | **yes**, naming `b.Missing` / `zz.X` / `nope.X` / `g.OnlyMain` |
| Q17 | local `type W b.Tree`; `b.T2` (`type T2 time.Time` in b); local `type D2 b.T2` | object + warning | `W`: `{type: object, additionalProperties: {}}`; `b.T2`, `D2`: `{type: object}` | **yes**: `has type W: b.Tree contains itself`; `has type b.T2: b.T2 is a defined type over time.Time`; `has type D2: D2 is a defined type over time.Time` |

`json:"-"` fields are never resolved and never warn (unchanged). `ShapeUnknown` leaves never reach the lookup.

---

## Data model

**`internal/models`: no change.** The Resolution vocabulary is unchanged: a resolved qualified type produces the same `TypeShape` tree a local one does. Leaf `Name`s stay **short** (`ShapeMarshaler{Name: "Status"}`, `ShapeRecursive{Name: "Tree"}`, `ShapeRef{Name: "User"}`); the generator never reads a Marshaler or recursive leaf's name, and a ref leaf's name is overwritten by registration with the final component name.

**Analyzer-private state (`internal/analyzer/resolution.go`, anchors at `deedfa9`):**

| Item (anchor) | Change | Why |
|---|---|---|
| `resolveState` (80–85) | add `home string`: `filepath.Dir` of the field's own file; `stack` now holds `typeKey`s, not names | `display` needs the field's package; the stack must tell a local `Tags` from b's `Tags` |
| `cut` (94–106) | parameter renamed `name` → `key`; body unchanged | it now receives a `typeKey` |
| `resolveCtx` (117–130) | add `byteElem bool` before `st` | carries "this is a slice element" into `resolveLocal`, where the element's methods are known |
| `newResolveCtx` (145–147) | set `home: filepath.Dir(path)` | |
| `quiet` (155–164) | copy `home` into the private state; rewrite its doc comment (text under "Byte-slice rule") | a quiet run still displays names relative to the field; `marshalerLeaf` makes the isolation load-bearing |
| new `typeKey`, `display` (methods on `resolveCtx`) | see code | |

No `TypeShape` field, no `models` accessor, no new consumer path.

---

## Resolver changes, with every helper's degenerate input checked

### Helpers reused as is (behavior verified at `deedfa9`)

| Helper (anchor) | Degenerate behavior the plan relies on |
|---|---|
| `fileImports` (`analyzer.go:3515`) | Skips `_` and `.` imports. An unaliased in-module import maps under `unaliasedImportName` (the first sorted file's clause, #122); an aliased one under its alias. A missing key reads as `""` from the map. |
| `inModuleDir` (`analyzer.go:3488`) | `false` when `a.modulePath == ""` (no `go.mod` read: every direct `New(dir)` unit test that never runs `AnalyzeProject`), for `""`, for stdlib/third-party paths, and for a path escaping the project root. A nonexistent in-module dir is still `true` (lexical `withinProjectRoot`). |
| `parsePackageDir` (`analyzer.go:3557`) | Cached per dir; skips `_test.go`, unparsable and out-of-root files; errors only on an unreadable dir. Returns **every** clause in the dir, `package main` generators included. |
| `typeSpecInFile` (`analyzer.go:3838`) | First `TypeSpec` of that name in one file, any RHS (struct, alias, defined). `nil` when absent. |
| `samePackageFiles` / `localTypeDecls` (`resolution.go:478`, `496`) | The given file first, then same-dir files **with the given file's clause** in sorted order (a `package main` generator is skipped because its clause differs). So `localTypeDecls(name, siteFile, sitePath)` returns every build-tagged declaration of the name in the imported package and never a generator's. |
| `resolveTypeSpecChain` (`analyzer.go:3295`) | Called by `resolveLocal` first; in b's context it finds b's structs and alias/defined chains to them (`b.UA`, `b.UD`), so those become `ShapeRef`s recorded at b's file (Q15). |
| `marshalerSet` (`marshaler.go:65`) | Scans `samePackageFiles(c.file, c.path)`: in b's context, every file of b (so `MarshalText` in `b/status_text.go` counts), never the route package. |
| `registerRefLeaf` (`resolution.go:579`) | `registerTypeAt(leaf.Name, site.file.Name.Name, site.file, site.path, depth)`. For a leaf recorded at b's file, `pkg == site.file.Name.Name`, so it takes the local branch **in b** (`structDeclSite` → b's `User`) and keys the component `schemaKey("User", "b")`: the same key `registerQualifiedTypeAt` uses for a direct `b.User` field (`q.pkg = file.Name.Name`). A `c.Item` leaf recorded at b's file goes through `registerQualifiedTypeAt("c.Item", bFile)`, which resolves `c` through **b's** imports. |
| `resolveQualifiedStruct` (`analyzer.go:3406`) | Unchanged and still first in `resolveQualified`: a struct target stays the existing `ShapeRef{"b.User"}` path. |
| `knownUnderlyingBuiltins`, `models.WellKnownTypeNames`, `kindBackedUnderlying` | Unchanged and checked **before** the new lookup (open decision 2). They match by qualified string, so they only ever fire for names that would also fail the in-module lookup unless a project has its own in-module package named `time`/`uuid`/`json`. |

### New lookup: `inModuleTypeSite`

```go
// inModuleTypeSite finds the declaring file of a qualified named type
// (q.T) in another package of the module: q maps through file's imports to an
// in-module directory, and the first file in sorted path order that declares
// T, among files of the imported package (never a package main file, such as
// a //go:build ignore generator: main cannot be imported), is the site. ok is
// false for stdlib and third-party packages, an unknown qualifier, an
// unreadable directory, or a name the package does not declare.
func (a *ProjectAnalyzer) inModuleTypeSite(qualified string, file *ast.File) (site pkgFile, name string, ok bool) {
	// Callers pass dotted names only; an undotted one maps to no import.
	qual, name, _ := strings.Cut(qualified, ".")
	dir, ok := a.inModuleDir(a.fileImports(file)[qual])
	if !ok {
		return pkgFile{}, "", false
	}
	files, err := a.parsePackageDir(dir)
	if err != nil {
		return pkgFile{}, "", false
	}
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if f := files[p]; f.Name.Name != mainPackageName && typeSpecInFile(f, name) != nil {
			return pkgFile{file: f, path: p}, name, true
		}
	}
	return pkgFile{}, "", false
}
```

- **No `!found` guard on `strings.Cut`.** Both callers pass dotted names only (`resolveShape` and `resolveUnderlying` test `strings.Contains(…, ".")` first); a guard would be dead, uncovered code. An undotted name gives `qual = qualified`, `name = ""`, no import, `false`.
- **Why "not `main`" and not the qualifier or `unaliasedImportName`:** an importable package is never `main`, so skipping `main` is correct regardless of sort order and of whether the import is aliased (`money`). `unaliasedImportName` returns the first sorted file's clause and would pick a generator that sorts first (brief). A directory holding two non-`main` clauses (not a buildable package) resolves to the first sorted declaring file; accepted residual (open decision 3). **Superseded in review:** a `//go:build ignore` tool of any clause leaves the directory buildable, so the site is now taken only among files sharing `importableClause`'s clause (the first sorted file that is neither `main` nor build-ignored); see Review dispositions.
- After the site is found, `resolveLocal` → `localTypeDecls(name, site.file, site.path)` collects every declaration of `name` with the **site file's** clause, which is the imported package's.

### Call sites

1. **`resolveQualified` (`resolution.go:273–288`)**, after the struct, `knownUnderlyingBuiltins` and well-known checks, before the unresolvable note:

```go
	if models.WellKnownTypeNames[leaf.Name] {
		return leaf
	}
	if site, name, ok := a.inModuleTypeSite(leaf.Name, c.file); ok {
		return a.resolveLocal(models.TypeShape{Kind: models.ShapeNamed, Name: name}, c.in(site.file, site.path), identityMode)
	}
	c.st.note(fieldFallback{kind: fallbackUnresolvable, typeName: leaf.Name})
	return leaf
```

   Doc comment: "…a well-known type stays as is, an in-module named type resolves in its declaring package (inModuleTypeSite), and anything else is unresolvable (it emits object)."

2. **`resolveUnderlying` (`resolution.go:355–378`)**, after `kindBackedUnderlying`, before the defined-over-qualified note (this is what makes `type Discount b.Cents`, b's `type Grade c.Level` and `type StatusD b.Status` work; underlying mode drops `b.Status`'s methods, so `StatusD` is `int`):

```go
	if u, ok := kindBackedUnderlying(s.Name); ok {
		return u
	}
	if site, target, ok := a.inModuleTypeSite(s.Name, c.file); ok {
		return a.resolveLocal(models.TypeShape{Kind: models.ShapeNamed, Name: target}, c.in(site.file, site.path), underlyingMode)
	}
	definer := c.definer
	if definer == "" {
		definer = c.display(name)
	}
```

   Doc comment: "…a qualified name when kind-backed or declared in another package of the module (by its own underlying, dropping its methods) — any other qualified underlying (time.Time, uuid.UUID, decimal.Decimal) leaves name unresolvable…".

`c.in(...)` keeps `st` (one per field), `definer` and `byteElem`. So `st.param` (Marshaler exemption for parameters), the first-fallback note, the ref sites and the recursion stack are shared across packages, and every step after the lookup runs in the declaring file (its imports, its local names, its alias/defined distinction, its Marshaler scan, its recursion cut).

### Recursion key and display names

```go
// typeKey identifies name declared in c's package: the directory keeps a
// local Tags apart from another package's Tags on the recursion stack.
func (c resolveCtx) typeKey(name string) string {
	return filepath.Dir(c.path) + "\x00" + name
}

// display is name as a warning shows it: bare in the field's own package,
// qualified by its package clause in any other.
func (c resolveCtx) display(name string) string {
	if filepath.Dir(c.path) == c.st.home {
		return name
	}
	return c.file.Name.Name + "." + name
}
```

- `resolveLocal` (290–318): `c.st.cut(c.typeKey(name))`; the cut note and the no-declaration note use `typeName: c.display(name)`. The `ShapeRecursive` leaf keeps `Name: name`.
- `resolveDecls` (320–338): push `c.typeKey(name)` (line 324); the disagree note uses `c.display(name)` (line 336).
- `resolveDecl` (340–353): `dc.definer = dc.display(name)` (line 350). The definer is displayed **when it is set**, in its own package's context, because the note that reads it may fire in another package (`type D2 b.T2` → `T2 time.Time` notes `D2`, not `b.D2`).
- `resolveUnderlying`: the fallback definer is `c.display(name)` (line 374).
- `typeKey` is per **directory**: in-memory unit tests use nonexistent paths, which still give distinct dirs. Depth-cap semantics are unchanged (stack length).
- Without the key, local `type Tags []b.Tags` would be cut as recursive (pinned by `TestQualifiedShadowing` and the golden's `localTags`; Mutation 3).

### Byte-slice rule moves into `resolveLocal` (required, not cosmetic)

PR 1's `resolveSliceElem` (248–266) resolved a **local** named element, then re-ran `a.marshalerSet(r.Name, c)` in the **slice's** file context to see whether the Marshaler leaf was decode-only. Once qualified types resolve, that second scan looks in the wrong package. Reproduced on the prototype with PR 1's `resolveSliceElem` kept:

- `[]FV` with local `FV = b.FlagV` (b's `FlagV` has `MarshalJSON`) emits `{type: string, format: byte}` **silently** (wrong: `encoding/json` calls `MarshalJSON`), because the route package declares no `FlagV` methods;
- `[]b.FlagU` (decode-only, byte) emits `{type: array, items: {}}` plus a warning, while its local twin is base64.

The fix decides the rule where the methods are known. `resolveSliceElem` becomes:

```go
// resolveSliceElem resolves a slice element with byteElem set, so the
// byte-slice rule (#97) is decided where the element's methods are known.
func (a *ProjectAnalyzer) resolveSliceElem(e *models.TypeShape, c resolveCtx) *models.TypeShape {
	c.byteElem = true
	return a.resolveElem(e, c)
}
```

`resolveShape`'s container arms clear the flag before descending (`c.byteElem = false` as the first statement of both the pointer/array/map arm and the slice arm), so only the slice's **direct** element, reached through any chain of aliases or named types, sees it. `resolveLocal`'s Marshaler branch becomes a call to a new helper (this also keeps `resolveLocal` at 7):

```go
	if mode == identityMode && !c.st.param {
		if m := a.marshalerSet(name, c); m.found() {
			return a.marshalerLeaf(name, decls, c, m)
		}
	}
```

```go
// marshalerLeaf resolves name, a Marshaler type with methods m met in
// identity mode: a Marshaler leaf over its quietly resolved underlying type,
// noted, except as a slice element (c.byteElem) of a byte-kind type with only
// decode-side methods, which is that plain byte, unnoted, so the slice stays
// base64 (#97). The rule is decided here, in the declaring package's context,
// because only here are the type's own methods known.
func (a *ProjectAnalyzer) marshalerLeaf(name string, decls []typeDecl, c resolveCtx, m marshalerMethods) models.TypeShape {
	under := a.resolveDecls(name, decls, c.quiet(), underlyingMode)
	if c.byteElem && !m.encode && isBytePrimitive(&under) {
		return under
	}
	c.st.note(fieldFallback{kind: fallbackMarshaler, typeName: c.display(name), detail: m.first})
	return models.TypeShape{Kind: models.ShapeMarshaler, Name: name, Elem: &under}
}
```

Equivalence with PR 1 for local types (checked case by case, and every PR 1 test passes unchanged on the prototype): PR 1 noted the Marshaler fallback, then restored `st.first` from a snapshot when the rule applied; here the note is simply not made. Moving the note after the quiet run changes nothing **only because `quiet()` isolates `st.first`**: on PR 1 that isolation was unobservable (the note came first and `note` keeps the first), here it is required, since otherwise a note from the underlying run (a Marshaler type over `time.Time`, `type S time.Time` with `MarshalJSON`, notes the defined-over-qualified fallback there) wins and the field warns "is a defined type over time.Time … emitting an untyped object" while it renders `{}`. Pinned by `TestQualifiedWarnedRowsInEveryPosition`'s `b.MStamp`/`LStamp` rows and mutation 7; `quiet()`'s doc comment, which today calls the isolation unobservable and forward-looking, becomes:

```go
// quiet is c with a private copy of the state: what it notes and the ref
// sites it records are discarded. The stack is cloned so the quiet run's
// pushes cannot overwrite the caller's backing array. The isolation is
// required: marshalerLeaf notes the Marshaler fallback after its quiet run of
// the type's underlying form, so a note from that run (a defined type over
// an unresolvable qualified type, or a recursion cut) must not become the
// field's warning. The discarded ref sites matter once a consumer
// descends into a Marshaler's Elem (#111).
func (c resolveCtx) quiet() resolveCtx {
	c.st = &resolveState{param: c.st.param, home: c.st.home, stack: slices.Clone(c.st.stack), refSites: map[string]pkgFile{}}
	return c
}
```

The rest, unchanged from PR 1: an alias chain (`[]FlagUA`, `FlagUA = FlagU`) keeps the flag through `resolveDecl`'s alias path; a defined chain (`[]D`, `type D FlagU`) reaches `FlagU` in underlying mode, which never checks methods (base64, as before); `[]Arr` with `Arr = [2]FlagU` or `Ptr = *FlagU` clears the flag in the container arm (`{}` plus a warning, as before). `isBytePrimitive` is unchanged and still used.

### Builtins (`internal/analyzer/builtins.go`)

- Add `goTypeUint64 = "uint64"` to the const block (after `goTypeInt64`, line 16) and use it in `isIntegerType`'s case list (line 50, replacing the `"uint64"` literal). Reason: goconst (Global Constraints).
- Add `mainPackageName = "main" // a program's package clause: never importable` after `unknownTypeName` (line 27). No `"main"` constant exists in the package today (grepped).

### Warnings (texts and names)

The kinds and dedupe are PR 1's, unchanged. Two texts change because "a named type from another package" is no longer a cause:

```go
	unresolvableFieldWarning = "field %s at %s has type %s: %s resolves to no schema (a type from outside the module, " +
		"a well-known type under an aliased import, or a name with no declaration in its package) — emitting an untyped object"
	definedOverQualifiedFieldWarning = "field %s at %s has type %s: %s is a defined type over %s, which resolves to no schema " +
		"(a defined type drops its target's methods, and a type from outside the module is not resolved) — emitting an untyped object"
```

No test or doc asserts the removed wording (`git grep 'another package'` in tests/docs finds only unrelated comments); assertions use the stable prefixes "resolves to no schema" / "is a defined type over". Names of types from another package now appear clause-qualified: `b.Status has its own MarshalText method`, `b.Tree contains itself`, `b.Width's build-tagged declarations disagree`, `b.T2 is a defined type over time.Time`. Local names are unchanged, so no PR 1 warning text moves.

Also update `resolution.go`'s header comment (lines 15–19): "every local named non-struct type" → "every named non-struct type of the module (local, or declared in another in-module package and resolved in that package)", and `resolveLocal`'s doc comment (290–294): "resolves a local named type" → "resolves a named type declared in c's package (the field's own, or the package inModuleTypeSite found)", and `quiet`'s doc comment (155–160) to the text under "Byte-slice rule" (the old text's "Today the isolation is unobservable … forward-looking for #111" becomes false once `marshalerLeaf` notes after the quiet run).

---

## Goldens

- **New:** `internal/spectest/testdata/qualified_named_types/` (Task 5). Strict-unclean on purpose (it carries the warned rows Q9, Q10, Q13).
- **Moves (named in the PR):** `named_fallback/expected.yaml` loses its `cents` property (two lines), because the fixture's `Cents types.Cents` row is deleted together with `named_fallback/types/` (open decision 1). The fixture's premise ("every field raises one analyzer warning", `api.go:65`) stays true.
- **Must not move:** every other golden, including `crosspkg`, `pkgname_mismatch`, `collision`, `delegation` (their sub-packages declare only structs: grepped) and PR 1's `named_resolution` (its `domain` sub-package declares only structs). Confirmed on the prototype: the only golden failures before regeneration were `named_fallback` and the new fixture.

---

## Cognitive-complexity budget (gocognit, measured on `deedfa9`; after = prototype)

| Function (file:line at `deedfa9`) | Now | After | Note |
|---|---|---|---|
| `resolveLocal` (resolution.go:295) | 7 | 7 | Marshaler branch moves to `marshalerLeaf`; without the split it measured 11 |
| new `marshalerLeaf` | — | 2 | |
| `resolveQualified` (276) | 3 | 4 | |
| new `inModuleTypeSite` | — | 6 | |
| `resolveUnderlying` (361) | 4 | 5 | |
| `resolveSliceElem` (251) | 5 | 0 | |
| `resolveShape` (212) | 3 | 3 | two assignments |
| `resolveDecls` (323) / `resolveDecl` (344) | 3 / 3 | 3 / 3 | |
| `cut` (98) | 2 | 2 | |
| new `typeKey` / `display` | — | 0 / 1 | |
| `newResolveCtx` / `quiet` | 0 / 0 | 0 / 0 | |
| `isIntegerType` (builtins.go:47) | 1 | 1 | literal → constant |
| new tests | — | ≤5 each | all `Test…` bodies are loops of asserts |

Nothing touched exceeds 7.

---

## Task 0: Branch and baseline

- [x] **Step 1: move the branch onto `main`.** In `/Users/gaborage/Projects/gaborage/code/go-bricks-openapi-pr2`: `git fetch -q origin`, then check the preconditions: `git branch --show-current` is `fix/qualified-named-types`, `git rev-parse HEAD` is `deedfa9…`, `git status --porcelain` lists only `?? docs/superpowers/plans/2026_10_08_qualified_named_types_pr2.md` (this plan, untracked), and `git rev-parse 'deedfa9^{tree}' 'origin/main^{tree}'` prints the same tree twice (`54516b4…`) unless `main` has moved. Then `git rebase --onto origin/main deedfa9 fix/qualified-named-types`: the branch has no commits past `deedfa9`, so this replays nothing and leaves `HEAD` at `origin/main` (same as `git checkout -B fix/qualified-named-types origin/main`; the untracked plan file is untouched). Expected `git rev-parse HEAD` → `feeafaa…`, and `git merge-base HEAD origin/main` equals `HEAD`. If `origin/main` is past `feeafaa`, the trees differ: re-check every line anchor with `grep -n` and treat Step 2 as the new baseline.
- [x] **Step 2:** Baseline at `feeafaa` (or the newer `origin/main`): `make check` (pinned golangci-lint) and `go test ./internal/spectest` green. Record `go run github.com/uudashr/gocognit/cmd/gocognit@latest internal/analyzer/resolution.go | sort -rn | head` (expect `resolveLocal` 7 at the top).

## Task 1: Failing analyzer tests (TDD red)

**Files:** create `internal/analyzer/qualified_test.go`. It reuses PR 1's helpers from `resolution_test.go` (`resolveGoMod`, `resolveModuleHead`, `rowsModule`, `rowsBody`, `resolveRow`, `renders`, `fieldWarnings`), `analyzeDirectiveProject` (`directive_test.go:685`) and `parseInDir` (`marshaler_test.go:16`). Every test runs through `analyzeDirectiveProject` (it reads `go.mod`, so `a.modulePath` is set); a bare `New(dir)` has no module path and the lookup never fires (pinned as a degenerate case).

- [x] **Step 1:** Write the file exactly as below. `goTypeUint64` does not exist yet, so the package does not compile: that is the red state for Step 2.

```go
package analyzer

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// qualifiedB is package b of the qualified-resolution tests: one declaration
// per row of #100's table. Status's MarshalText lives in b/status_text.go.
const qualifiedB = `package b

import (
	"time"

	"github.com/example/app/c"
)

type Cents int64
type Big uint64
type Flag bool
type Code string
type Codes []Code
type Wrapped Cents
type Grade c.Level
type Base struct {
	Grade Grade   ` + "`json:\"grade\"`" + `
	Level c.Level ` + "`json:\"level\"`" + `
}
type Tags []string
type User struct {
	Name string ` + "`json:\"name\"`" + `
}
type UserList []User
type Items []c.Item
type Stamp = time.Time
type Blob []byte
type Tree map[string]Tree
type Addr uintptr
type Status int
type PStatus int

func (p *PStatus) UnmarshalJSON(b []byte) error { return nil }

type FlagU byte

func (f *FlagU) UnmarshalJSON(b []byte) error { return nil }

type FlagV byte

func (f *FlagV) MarshalJSON() ([]byte, error) { return nil, nil }

type T2 time.Time
type MStamp time.Time

func (MStamp) MarshalJSON() ([]byte, error) { return nil, nil }
`

// qualifiedImports are the import lines of every qualified rows module.
const qualifiedImports = "\t\"github.com/example/app/b\"\n\t\"github.com/example/app/kinds\"\n\tmoney \"github.com/example/app/b\"\n"

// analyzeQualified analyzes a project with packages b, c and kinds beside a
// rows module (mod/module.go) declaring decls and the Rows fields rows, plus
// any extra files; it returns the analyzer and Rows' rendered resolutions.
func analyzeQualified(t *testing.T, decls, rows string, extra map[string]string) (a *ProjectAnalyzer, got map[string]string) {
	t.Helper()
	files := map[string]string{
		"go.mod":                             resolveGoMod,
		filepath.Join("b", "b.go"):           qualifiedB,
		filepath.Join("b", "status_text.go"): "package b\n\nfunc (s Status) MarshalText() ([]byte, error) { return nil, nil }\n",
		filepath.Join("c", "c.go"):           "package c\n\ntype Level int32\n\ntype Item struct {\n\tID int `json:\"id\"`\n}\n",
		filepath.Join("kinds", "kinds.go"):   "package kinds\n\ntype Labels map[string]string\n",
		filepath.Join("mod", "module.go"):    rowsModule(qualifiedImports, "var _ kinds.Labels\nvar _ money.Cents\n"+decls, rows),
	}
	for k, v := range extra {
		files[k] = v
	}
	a = analyzeDirectiveProject(t, files)
	return a, renders(t, a.typeRegistry["Rows"])
}

// TestQualifiedNamedRows pins #100's resolvable rows: each in-module
// qualified named type resolves in its declaring package, in every position,
// and none of them warns.
func TestQualifiedNamedRows(t *testing.T) {
	rows := []resolveRow{
		{"cents", "b.Cents", goTypeInt64},
		{"moneyCents", "money.Cents", goTypeInt64},
		{"big", "b.Big", goTypeUint64},
		{"flag", "b.Flag", goTypeBool},
		{"code", "b.Code", goTypeString},
		{"wrapped", "b.Wrapped", goTypeInt64},
		{"grade", "b.Grade", goTypeInt32},
		{"discount", "Discount", goTypeInt64},
		{"cents2", "Cents2", goTypeInt64},
		{"pCents", "*b.Cents", "*int64"},
		{"centsList", "[]b.Cents", "[]int64"},
		{"centsGrid", "[][]b.Cents", "[][]int64"},
		{"centsMap", "map[string]b.Cents", "map[string]int64"},
		{"centsMapL", "map[string][]b.Cents", "map[string][]int64"},
		{"centsMapP", "map[string]*b.Cents", "map[string]*int64"},
		{"pCentsMap", "*map[string]b.Cents", "*map[string]int64"},
		{"centsMapArr", "[]map[string]b.Cents", "[]map[string]int64"},
		{"tags", "b.Tags", "[]string"},
		{"pTags", "*b.Tags", "*[]string"},
		{"tagsList", "[]b.Tags", "[][]string"},
		{"tagsMap", "map[string]b.Tags", "map[string][]string"},
		{"moneyTags", "money.Tags", "[]string"},
		{"labels", "kinds.Labels", "map[string]string"},
		{"codes", "b.Codes", "[]string"},
		{"localTags", "Tags", "[][]string"},
		{"userList", "b.UserList", "[]$User"},
		{"items", "b.Items", "[]$Item"},
		{"stamp", "b.Stamp", "time.Time"},
		{"blob", "b.Blob", "[]byte"},
		{"statusD", "StatusD", "int"},
		{"us", "[]b.FlagU", "[]byte"},
	}
	decls := "type Discount b.Cents\ntype Cents2 = b.Cents\ntype StatusD b.Status\ntype Tags []b.Tags\n"
	a, got := analyzeQualified(t, decls, rowsBody(rows), nil)
	for _, r := range rows {
		assert.Equal(t, r.want, got[r.json], r.json)
	}
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestQualifiedShadowing pins that every step after the lookup runs in the
// declaring package: b's Code, Codes and User win over the route package's
// own Code int32 and User struct, b's Tags is not the local Tags, and
// b.Items registers c's Item although the route package never imports c.
func TestQualifiedShadowing(t *testing.T) {
	decls := "type Code int32\ntype User struct{ ID int `json:\"id\"` }\ntype Tags []b.Tags\n"
	rows := "\tCode b.Code `json:\"code\"`\n\tLocalCode Code `json:\"localCode\"`\n\tCodes b.Codes `json:\"codes\"`\n" +
		"\tUserList b.UserList `json:\"userList\"`\n\tLocalUser User `json:\"localUser\"`\n\tItems b.Items `json:\"items\"`\n" +
		"\tTags Tags `json:\"tags\"`\n"
	a, got := analyzeQualified(t, decls, rows, nil)
	assert.Equal(t, goTypeString, got["code"])
	assert.Equal(t, goTypeInt32, got["localCode"])
	assert.Equal(t, "[]string", got["codes"])
	assert.Equal(t, "[][]string", got["tags"], "a local Tags over b.Tags is not recursive")

	bUser := strings.TrimPrefix(got["userList"], "[]$")
	require.Contains(t, a.typeRegistry, bUser)
	assert.Equal(t, "b", a.typeRegistry[bUser].Package, "b.UserList references b's User")
	localUser := strings.TrimPrefix(got["localUser"], "$")
	assert.NotEqual(t, bUser, localUser)
	require.Contains(t, a.typeRegistry, localUser)
	assert.Equal(t, "mod", a.typeRegistry[localUser].Package)
	assert.Equal(t, "[]$Item", got["items"])
	require.Contains(t, a.typeRegistry, "Item")
	assert.Equal(t, "c", a.typeRegistry["Item"].Package)
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestQualifiedWarnedRowsInEveryPosition pins #100's warned rows in all seven
// positions: the render is the position around the direct render, and
// exactly one warning of the row's kind names the field.
func TestQualifiedWarnedRowsInEveryPosition(t *testing.T) {
	reps := []struct{ goType, direct, warning string }{
		{"b.Status", "marshal:Status(int)", "b.Status has its own MarshalText method"},
		{"StatusA", "marshal:Status(int)", "b.Status has its own MarshalText method"},
		{"b.PStatus", "marshal:PStatus(int)", "b.PStatus has its own UnmarshalJSON method"},
		{"b.Tree", "map[string]cycle:Tree", "b.Tree contains itself"},
		{"b.Addr", "uintptr", "holds a uintptr"},
		{"b.Missing", "b.Missing", "b.Missing resolves to no schema"},
		{"decimal.Decimal", "decimal.Decimal", "decimal.Decimal resolves to no schema"},
		{"b.T2", "T2", "b.T2 is a defined type over time.Time"},
		{"b.MStamp", "marshal:MStamp(MStamp)", "b.MStamp has its own MarshalJSON method"},
		{"LStamp", "marshal:LStamp(LStamp)", "LStamp has its own MarshalJSON method"},
	}
	positions := []string{"%s", "*%s", "[]%s", "map[string]%s", "[][]%s", "map[string][]%s", "[]map[string]%s"}
	var body strings.Builder
	for i, r := range reps {
		for j, p := range positions {
			fmt.Fprintf(&body, "\tF%dP%d %s\n", i, j, fmt.Sprintf(p, r.goType))
		}
	}
	files := map[string]string{
		filepath.Join("mod", "module.go"): rowsModule(qualifiedImports+"\t\"github.com/shopspring/decimal\"\n\t\"time\"\n",
			"var _ kinds.Labels\nvar _ money.Cents\ntype StatusA = b.Status\ntype LStamp time.Time\n\nfunc (LStamp) MarshalJSON() ([]byte, error) { return nil, nil }\n", body.String()),
	}
	a, got := analyzeQualified(t, "", "", files)
	for i, r := range reps {
		for j, p := range positions {
			name := fmt.Sprintf("F%dP%d", i, j)
			assert.Equal(t, fmt.Sprintf(p, r.direct), got[name], name)
			if w := fieldWarnings(a, name); assert.Len(t, w, 1, name) {
				assert.Contains(t, w[0], r.warning, name)
			}
		}
	}
}

// TestQualifiedByteSliceRule pins the byte-slice rule (#97) across packages:
// it is decided by the methods of the element's declaring package, so a
// decode-only b.FlagU keeps []b.FlagU base64, while a local alias of b's
// encode-side FlagV makes its slice an array of {} with a warning.
func TestQualifiedByteSliceRule(t *testing.T) {
	decls := "type FU = b.FlagU\ntype FV = b.FlagV\n"
	rows := "\tUs []b.FlagU `json:\"us\"`\n\tFUs []FU `json:\"fus\"`\n\tVs []b.FlagV `json:\"vs\"`\n\tFVs []FV `json:\"fvs\"`\n" +
		"\tU b.FlagU `json:\"u\"`\n\tUArr [2]b.FlagU `json:\"uArr\"`\n"
	a, got := analyzeQualified(t, decls, rows, nil)
	assert.Equal(t, "[]byte", got["us"])
	assert.Equal(t, "[]byte", got["fus"])
	assert.Equal(t, "[]marshal:FlagV(byte)", got["vs"])
	assert.Equal(t, "[]marshal:FlagV(byte)", got["fvs"])
	assert.Equal(t, "marshal:FlagU(byte)", got["u"], "outside a slice a decode-only type is still a Marshaler type")
	assert.Equal(t, "[N]marshal:FlagU(byte)", got["uArr"], "the rule applies to slices only")
	for _, name := range []string{"Us", "FUs"} {
		assert.Empty(t, fieldWarnings(a, name), name)
	}
	for _, name := range []string{"Vs", "FVs", "U", "UArr"} {
		assert.Len(t, fieldWarnings(a, name), 1, name)
	}
}

// TestQualifiedLookupDegenerate pins inModuleTypeSite's misses: an unknown
// qualifier, a stdlib or third-party package, an in-module path with no
// directory, and a name only a package main file declares all leave the
// leaf unresolvable, warned; a qualified alias or defined type over a struct
// in b becomes a $ref registered in b.
func TestQualifiedLookupDegenerate(t *testing.T) {
	files := map[string]string{
		filepath.Join("g", "kind.go"):   "package g\n\ntype Other int\n",
		filepath.Join("g", "zz_gen.go"): "//go:build ignore\n\npackage main\n\ntype OnlyMain int64\n\nfunc main() {}\n",
		filepath.Join("b", "alias.go"):  "package b\n\ntype UA = User\ntype UD User\n",
		filepath.Join("mod", "module.go"): rowsModule(qualifiedImports+"\t\"github.com/example/app/nope\"\n\t\"github.com/example/app/g\"\n\t\"github.com/shopspring/decimal\"\n",
			"var _ kinds.Labels\nvar _ money.Cents\n",
			"\tZ zz.X `json:\"z\"`\n\tN nope.X `json:\"n\"`\n\tM g.OnlyMain `json:\"m\"`\n\tD decimal.Decimal `json:\"d\"`\n"+
				"\tUA b.UA `json:\"ua\"`\n\tUD b.UD `json:\"ud\"`\n"),
	}
	a, got := analyzeQualified(t, "", "", files)
	for name, rendered := range map[string]string{"Z": "zz.X", "N": "nope.X", "M": "g.OnlyMain", "D": "decimal.Decimal"} {
		assert.Equal(t, rendered, got[strings.ToLower(name)], name)
		if w := fieldWarnings(a, name); assert.Len(t, w, 1, name) {
			assert.Contains(t, w[0], rendered+" resolves to no schema", name)
		}
	}
	assert.Equal(t, "$UA", got["ua"])
	assert.Equal(t, "$UD", got["ud"])
	require.Contains(t, a.typeRegistry, "UA")
	assert.Equal(t, "b", a.typeRegistry["UA"].Package)
	assert.Empty(t, fieldWarnings(a, "UA"))
	assert.Empty(t, fieldWarnings(a, "UD"))

	// With no module path (no go.mod was read), nothing is in-module.
	bare := New(t.TempDir())
	file, _ := parseInDir(t, bare, bare.projectRoot, "x.go", "package x\n\nimport \"github.com/example/app/b\"\n")
	_, _, ok := bare.inModuleTypeSite("b.Cents", file)
	assert.False(t, ok)
}

// TestQualifiedBuildTaggedVariants pins that every declaration of a
// qualified name in its package is merged, as for a local name (#92): b.Width
// in two build-tagged files of b, reached directly, as slice items and as a
// map value. A //go:build ignore generator in package main that sorts FIRST
// in b's directory (aaa_gen.go) never contributes a declaration; the import is
// aliased so fileImports' first-sorted-file clause (#122) is not exercised.
func TestQualifiedBuildTaggedVariants(t *testing.T) {
	variant := func(constraint, decl string) string {
		return "//go:build " + constraint + "\n\npackage w\n\n" + decl + "\n"
	}
	cases := []struct {
		name  string
		files map[string]string
		leaf  string
	}{
		{"widths disagree", map[string]string{"width_amd64.go": variant("amd64", "type Width int64"), "width_386.go": variant("386", "type Width int32")}, "kind:" + kindInteger},
		{"widths agree", map[string]string{"width_amd64.go": variant("amd64", "type Width int64"), "width_arm64.go": variant("arm64", "type Width int64")}, goTypeInt64},
		{"a package main generator sorts first", map[string]string{
			"aaa_gen.go": "//go:build ignore\n\npackage main\n\ntype Width string\n\nfunc main() {}\n",
			"width.go":   "package w\n\ntype Width int64\n",
		}, goTypeInt64},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := map[string]string{
				"go.mod": resolveGoMod,
				filepath.Join("mod", "module.go"): rowsModule("\tww \"github.com/example/app/w\"\n", "",
					"\tW ww.Width `json:\"w\"`\n\tWS []ww.Width `json:\"ws\"`\n\tWM map[string]ww.Width `json:\"wm\"`\n"),
			}
			for name, src := range c.files {
				files[filepath.Join("w", name)] = src
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

// TestQualifiedPayloadUnchanged pins the out-of-scope payload half (#110): a
// Result[b.Cents] payload keeps its untyped data schema and its warning.
func TestQualifiedPayloadUnchanged(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod":                   resolveGoMod,
		filepath.Join("b", "b.go"): "package b\n\ntype Cents int64\n",
		filepath.Join("mod", "module.go"): strings.Replace(resolveModuleHead, "import (\n", "import (\n\t\"github.com/example/app/b\"\n", 1) + `
func (m *Module) get(ctx server.HandlerContext) (server.Result[b.Cents], server.IAPIError) {
	return server.Result[b.Cents]{}, nil
}
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/cents", m.get)
}
`,
	})
	assert.Contains(t, strings.Join(a.Warnings(t.Context()), "\n"), "response type b.Cents resolves to no schema component")
}

// TestQualifiedInSiblingFile pins that a struct declared in a sibling of the
// handlers' file resolves its qualified named types against its own imports
// (PR 1's structDeclSite), so the handlers' file need not import b.
func TestQualifiedInSiblingFile(t *testing.T) {
	files := map[string]string{
		filepath.Join("mod", "module.go"): rowsModule("", "", "\tSib Sib `json:\"sib\"`\n"),
		filepath.Join("mod", "sib.go"): "package mod\n\nimport \"github.com/example/app/b\"\n\n" +
			"type Sib struct {\n\tCents b.Cents `json:\"cents\"`\n\tUsers b.UserList `json:\"users\"`\n}\n",
	}
	a, got := analyzeQualified(t, "", "", files)
	assert.Equal(t, "$Sib", got["sib"])
	sib := renders(t, a.typeRegistry["Sib"])
	assert.Equal(t, goTypeInt64, sib["cents"])
	assert.Equal(t, "[]$User", sib["users"])
	require.Contains(t, a.typeRegistry, "User")
	assert.Equal(t, "b", a.typeRegistry["User"].Package)
	assert.Empty(t, a.Warnings(t.Context()))
}
```

Notes on the tests:
- `TestQualifiedWarnedRowsInEveryPosition` asserts that `b.Addr` warns **only** with the uintptr warning (exactly one warning, containing "holds a uintptr"), that the Marshaler and recursion warnings name the type clause-qualified, and that `b.T2` names its definer clause-qualified.
- Its `b.MStamp` and `LStamp` rows (a Marshaler type defined over `time.Time`, whose quiet underlying run notes the defined-over-qualified fallback) pin that `quiet()` keeps that note out of the field's warning now that `marshalerLeaf` notes after the quiet run (mutation 7). At red, the `b.MStamp` rows fail with the rest (the stub leaves `b.MStamp` unresolvable) and the `LStamp` rows already pass: a PR 1 pin, like `TestQualifiedPayloadUnchanged`.
- `TestQualifiedLookupDegenerate`'s `N nope.X` covers `inModuleTypeSite`'s `parsePackageDir` error branch (in-module path, no directory); `M g.OnlyMain` covers the `main` filter (only `g/zz_gen.go` declares it); `Z zz.X` covers the missing-qualifier branch; `D decimal.Decimal` the out-of-module branch.
- `TestQualifiedPayloadUnchanged` passes **before** the change as well (it is a pin, not a red test).

- [x] **Step 2: make it compile, still red.** The file references two names that do not exist yet: `goTypeUint64` and `inModuleTypeSite`. Do Task 2 Step 1 (the `builtins.go` constants) now, and add a temporary stub in `resolution.go` that Task 2 Step 3 replaces:

```go
func (a *ProjectAnalyzer) inModuleTypeSite(_ string, _ *ast.File) (site pkgFile, name string, ok bool) {
	return pkgFile{}, "", false // stub: replaced in Task 2 Step 3
}
```

- [x] **Step 3:** `go test ./internal/analyzer -run TestQualified` → `TestQualifiedNamedRows`, `TestQualifiedShadowing`, `TestQualifiedWarnedRowsInEveryPosition`, `TestQualifiedByteSliceRule`, `TestQualifiedLookupDegenerate` (its `UA`/`UD` rows), `TestQualifiedBuildTaggedVariants` and `TestQualifiedInSiblingFile` **fail**; `TestQualifiedPayloadUnchanged` passes (it is a pin). Run `go test ./internal/analyzer` too: every PR 1 test still passes (the stub changes nothing). Each `a.typeRegistry[...]` dereference is preceded by `require.Contains` for this reason: unguarded, the stub leaves `UA` unregistered and `TestQualifiedLookupDegenerate` SIGSEGVs, which aborts the analyzer test binary and hides every later test (including `TestQualifiedBuildTaggedVariants`, `TestQualifiedInSiblingFile` and all of `resolution_test.go`). Verified on a `deedfa9` export with this file and the stub: exactly the seven tests above fail and nothing panics.

## Task 2: Implementation (TDD green)

**Files:** modify `internal/analyzer/builtins.go`, `internal/analyzer/resolution.go`.

- [x] **Step 1: `builtins.go`** (done in Task 1 Step 2; listed for completeness). Add `goTypeUint64 = "uint64"` after `goTypeInt64` (line 16) and use it in `isIntegerType` (line 50); add `mainPackageName` after `unknownTypeName` (line 27). `gofmt` realigns the const block.
- [x] **Step 2: state.** Apply the "Analyzer-private state" table: `home` and `stack` comment in `resolveState`; `cut(key string)`; `byteElem` in `resolveCtx` with the comment "byteElem is set while a slice's element is resolved: a byte-kind Marshaler type with only decode-side methods is then a plain byte, so the slice stays base64 (#97). Container nodes clear it."; `home` in `newResolveCtx` and `quiet`, and `quiet`'s new doc comment ("Byte-slice rule"; the isolation is now required); add `typeKey` and `display` after `newResolveCtx`.
- [x] **Step 3: lookup.** Replace the stub with `inModuleTypeSite` (placed after `resolveQualified`) and wire both call sites (`resolveQualified`, `resolveUnderlying`), with the doc-comment updates.
- [x] **Step 4: recursion key and display names.** `resolveLocal`, `resolveDecls`, `resolveDecl`, `resolveUnderlying` as in "Recursion key and display names".
- [x] **Step 5: byte-slice rule.** Replace `resolveSliceElem`'s body; add the two `c.byteElem = false` lines in `resolveShape`; replace `resolveLocal`'s Marshaler branch with the `marshalerLeaf` call and add `marshalerLeaf` after `resolveLocal`. `isBytePrimitive` stays.
- [x] **Step 6: warning texts and comments** (see "Warnings": the two texts, the header comment, `resolveLocal`'s and `quiet`'s doc comments).
- [x] **Step 7:** `go build ./... && go test ./internal/analyzer`. Expected: every `TestQualified*` passes; every PR 1 test passes **except** `TestUnresolvableFieldsWarn` (Task 3). `go test ./internal/spectest` fails only `TestGoldenFixtures/named_fallback` (the `cents` row).

## Task 3: The one deliberate analyzer-test update

**File:** `internal/analyzer/resolution_test.go`, `TestUnresolvableFieldsWarn` (740–781). Its `Cents types.Cents` row asserted PR 1's interim fallback for an in-module qualified type, which this PR removes on purpose.

- [x] **Step 1:** In the Rows body (line 748) add a `TMissing types.Missing` field after `Cents`: `"\tCents types.Cents `json:\"cents\"`\n\tTMissing types.Missing `json:\"tMissing\"`\n\tStampD StampD …"` (keeps an in-module qualified name with no declaration in the unresolvable set).
- [x] **Step 2:** In the unresolvable loop (line 753) replace `"Cents"` with `"TMissing"`.
- [x] **Step 3:** Replace line 777 (`assert.Equal(t, "types.Cents", got["cents"])`) with:

```go
	assert.Equal(t, goTypeInt64, got["cents"], "an in-module qualified named type resolves (#100)")
	assert.Empty(t, fieldWarnings(a, "Cents"))
	assert.Equal(t, "types.Missing", got["tMissing"])
```

- [x] **Step 4:** `go test ./internal/analyzer` green. No other existing test changes (verified on the prototype: the whole repo's suite passes with only this edit and the `named_fallback` fixture edit).

## Task 4: Spectest parity oracle

**File:** create `internal/spectest/qualified_oracle_test.go`. It reuses PR 1's `oracleDecls`, `oracleModule`, `oraclePositions`, `oracleCases` and `writeOracleProject` (`oracle_test.go`) and writes the oracle's declarations a second time as package `q`, so **every** Marshaler-free oracle case (`FlagU` in its three slice positions, via Task 2's byte-slice fix) is compared as `q.<Named>` against its local twin `<Named>`, in every position and as a query parameter. `$ref`s are dereferenced once before comparing: q's `User`/`Address` collide with the oracle's own and become `QUser`/`QAddress`, which is the brief's allowed difference ("apart from a struct leaf's component name"). `q.Members` (`[]domain.Member`) references the **same** `Member` component as the local one.

- [x] **Step 1:** Write the file:

```go
package spectest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/gaborage/go-bricks-openapi/internal/analyzer"
)

// qualifiedOracleModule adds the /qualified route to the oracle project.
const qualifiedOracleModule = `package oracle

import (
	"net/http"

	"github.com/gaborage/go-bricks/server"
)

func (m *Module) getQualified(req QualifiedQuery, ctx server.HandlerContext) (server.Result[Qualified], server.IAPIError) {
	return server.NewResult(http.StatusOK, Qualified{}), nil
}
`

// writeQualifiedOracleProject writes the oracle project plus a copy of its
// declarations as package q, and a Qualified struct holding each oracle case
// as q.<named> (Q<i>P<j>) beside its local twin (L<i>P<j>) in each of the
// case's positions, and once each as a query parameter.
func writeQualifiedOracleProject(t *testing.T, dir string) {
	t.Helper()
	writeOracleProject(t, dir)
	var body, query strings.Builder
	for i, c := range oracleCases {
		positions := c.positions
		if positions == nil {
			positions = oraclePositions
		}
		for j, p := range positions {
			fmt.Fprintf(&body, "\tQ%dP%d %s `json:\"q%dp%d\"%s`\n", i, j, fmt.Sprintf(p, "q."+c.named), i, j, c.tag)
			fmt.Fprintf(&body, "\tL%dP%d %s `json:\"l%dp%d\"%s`\n", i, j, fmt.Sprintf(p, c.named), i, j, c.tag)
		}
		fmt.Fprintf(&query, "\tQ%d q.%s `query:\"q%d\"`\n\tL%d %s `query:\"l%d\"`\n", i, c.named, i, i, c.named, i)
	}
	files := map[string]string{
		filepath.Join("q", "types.go"): strings.Replace(oracleDecls, "package oracle", "package q", 1),
		"qualified.go": "package oracle\n\nimport \"github.com/example/oracle/q\"\n\n" +
			"type Qualified struct {\n" + body.String() + "}\n\ntype QualifiedQuery struct {\n" + query.String() + "}\n",
		"qualified_module.go": qualifiedOracleModule,
	}
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "q"), 0o750))
	for name, src := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600), name)
	}
	mod := filepath.Join(dir, "module.go")
	src, err := os.ReadFile(mod)
	require.NoError(t, err)
	const get = `server.GET(hr, r, "/oracle", m.get)`
	patched := strings.Replace(string(src), get, get+"\n\tserver.GET(hr, r, \"/qualified\", m.getQualified)", 1)
	require.NotEqual(t, string(src), patched, "the oracle module registers /oracle")
	require.NoError(t, os.WriteFile(mod, []byte(patched), 0o600))
}

// derefOnce replaces every {$ref: X} in v with component X, so fields that
// reference equal components under different names (q's User is QUser beside
// the oracle's own User) compare equal: a struct leaf's component name is the
// one allowed difference between a qualified type and its local twin.
func derefOnce(v any, schemas map[string]any) any {
	switch t := v.(type) {
	case map[string]any:
		if ref, ok := t["$ref"].(string); ok {
			return schemas[strings.TrimPrefix(ref, "#/components/schemas/")]
		}
		out := make(map[string]any, len(t))
		for k, e := range t {
			out[k] = derefOnce(e, schemas)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = derefOnce(e, schemas)
		}
		return out
	default:
		return v
	}
}

// TestQualifiedNamedResolutionOracle is #100's parity guard: each
// Marshaler-free named type declared in another package of the module (q, a
// copy of the oracle's declarations) emits what the same declaration made
// locally emits, in every position and as a query parameter, apart from a
// struct leaf's component name, with no warning.
func TestQualifiedNamedResolutionOracle(t *testing.T) {
	dir := t.TempDir()
	writeQualifiedOracleProject(t, dir)

	spec, err := Generate(t.Context(), dir)
	require.NoError(t, err)
	require.NoError(t, Validate(t.Context(), []byte(spec)))

	var doc struct {
		Paths map[string]map[string]struct {
			Parameters []struct {
				Name   string `yaml:"name"`
				Schema any    `yaml:"schema"`
			} `yaml:"parameters"`
		} `yaml:"paths"`
		Components struct {
			Schemas map[string]any `yaml:"schemas"`
		} `yaml:"components"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &doc))
	schemas := doc.Components.Schemas
	qualified, ok := schemas["Qualified"].(map[string]any)
	require.True(t, ok, "Qualified component missing:\n%s", spec)
	props, ok := qualified["properties"].(map[string]any)
	require.True(t, ok)
	params := map[string]any{}
	for _, p := range doc.Paths["/qualified"]["get"].Parameters {
		params[p.Name] = p.Schema
	}

	for i, c := range oracleCases {
		positions := c.positions
		if positions == nil {
			positions = oraclePositions
		}
		for j, p := range positions {
			q, l := fmt.Sprintf("q%dp%d", i, j), fmt.Sprintf("l%dp%d", i, j)
			require.Contains(t, props, q)
			require.Contains(t, props, l)
			assert.Equal(t, derefOnce(props[l], schemas), derefOnce(props[q], schemas), "%s%s in %s", c.named, c.tag, p)
		}
		q, l := fmt.Sprintf("q%d", i), fmt.Sprintf("l%d", i)
		require.Contains(t, params, q)
		assert.Equal(t, derefOnce(params[l], schemas), derefOnce(params[q], schemas), "%s as a query parameter", c.named)
	}

	a := analyzer.New(dir)
	_, err = a.AnalyzeProject()
	require.NoError(t, err)
	assert.Empty(t, a.Warnings(t.Context()))
}
```

- [x] **Step 2:** `go test ./internal/spectest -run TestQualifiedNamedResolutionOracle` green (it fails on `deedfa9`: every `q.` field is `object`).

## Task 5: Golden fixtures

**Fixture rules (CLAUDE.md):** `go.mod` with a module under `github.com/example/`, `go 1.25`, `require github.com/gaborage/go-bricks v0.53.0`, no `go.sum`, no `replace`; one `.go` file implementing a go-bricks module; the underscore directory takes an underscore-free package name. Fixture Go is only AST-parsed (the build-tagged and `//go:build ignore` files are never compiled; `testdata` is skipped by Go tooling).

- [x] **Step 1: create `internal/spectest/testdata/qualified_named_types/`** with exactly these files.

`go.mod`:

```
module github.com/example/qualifiednamedtypes

go 1.25

require github.com/gaborage/go-bricks v0.53.0
```

`b/b.go`:

```go
// Package b declares the named non-struct types the route package uses
// qualified (b.Cents) and through an import alias (money.Cents).
package b

import (
	"time"

	"github.com/example/qualifiednamedtypes/c"
)

type Cents int64
type Big uint64
type Flag bool

// Code is a string here; the route package declares its own Code int32.
type Code string
type Codes []Code

type Wrapped Cents
type Grade c.Level

// Base is embedded by the route package: its fields are promoted and resolve
// in b.
type Base struct {
	Grade Grade   `json:"grade"`
	Level c.Level `json:"level"`
}

type Tags []string

// User shares its name with the route package's own User struct.
type User struct {
	Name string `json:"name"`
}

type UserList []User
type Items []c.Item

type Stamp = time.Time
type Blob []byte
type Tree map[string]Tree
type Addr uintptr

// Status is a Marshaler type: its MarshalText is in status_text.go.
type Status int

// PStatus decodes through a method on its pointer.
type PStatus int

func (p *PStatus) UnmarshalJSON(b []byte) error { return nil }
```

`b/status_text.go`:

```go
package b

func (s Status) MarshalText() ([]byte, error) { return nil, nil }
```

`b/width_amd64.go`:

```go
//go:build amd64

package b

type Width int64
```

`b/width_386.go`:

```go
//go:build 386

package b

type Width int32
```

`b/gen.go` (sorts **after** `b.go`, per the brief, so #122's first-sorted-file clause is not tripped):

```go
//go:build ignore

// A generator program: package main never contributes declarations to b.
package main

type Cents int32

func main() {}
```

`c/c.go`:

```go
// Package c is imported by b only; the route package never imports it.
package c

type Level int32

type Item struct {
	ID int `json:"id"`
}
```

`kinds/kinds.go`:

```go
package kinds

type Labels map[string]string
```

`module.go` (structs using `b`, `kinds` and `money` sit in the handlers' file, per the brief):

```go
package qualifiednamedtypes

import (
	"net/http"
	t "time"

	"github.com/example/qualifiednamedtypes/b"
	"github.com/example/qualifiednamedtypes/kinds"
	money "github.com/example/qualifiednamedtypes/b"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/scheduler"
	"github.com/gaborage/go-bricks/server"
	"github.com/shopspring/decimal"
)

type Module struct{}

func (m *Module) Name() string                    { return "qualified" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Code and User shadow b's own Code and User.
type Code int32

type User struct {
	ID int `json:"id"`
}

type Discount b.Cents
type Cents2 = b.Cents
type StatusA = b.Status
type StatusD b.Status

// Tags is a local slice of b's Tags: no recursion.
type Tags []b.Tags

// Qualified holds one field per row.
type Qualified struct {
	b.Base

	Cents      b.Cents     `json:"cents"`
	MoneyCents money.Cents `json:"moneyCents"`
	Big        b.Big       `json:"big"`
	Flag       b.Flag      `json:"flag"`
	Code       b.Code      `json:"code"`
	LocalCode  Code        `json:"localCode"`
	Wrapped    b.Wrapped   `json:"wrapped"`
	Grade      b.Grade     `json:"grade2"`
	Discount   Discount    `json:"discount"`
	Cents2     Cents2      `json:"cents2"`

	Bounded     b.Cents `json:"bounded" validate:"min=1,max=100"`
	BoundedCode b.Code  `json:"boundedCode" validate:"min=2,max=8"`

	PCents      *b.Cents             `json:"pCents"`
	CentsList   []b.Cents            `json:"centsList"`
	CentsGrid   [][]b.Cents          `json:"centsGrid"`
	CentsMap    map[string]b.Cents   `json:"centsMap"`
	CentsMapL   map[string][]b.Cents `json:"centsMapL"`
	CentsMapP   map[string]*b.Cents  `json:"centsMapP"`
	PCentsMap   *map[string]b.Cents  `json:"pCentsMap"`
	CentsMapArr []map[string]b.Cents `json:"centsMapArr"`

	BTags     b.Tags            `json:"bTags"`
	PBTags    *b.Tags           `json:"pBTags"`
	BTagsList []b.Tags          `json:"bTagsList"`
	BTagsMap  map[string]b.Tags `json:"bTagsMap"`
	MoneyTags money.Tags        `json:"moneyTags"`
	Labels    kinds.Labels      `json:"labels"`
	Codes     b.Codes           `json:"codes"`
	LocalTags Tags              `json:"localTags"`

	UserList  b.UserList `json:"userList"`
	BUser     b.User     `json:"bUser"`
	LocalUser User       `json:"localUser"`
	Items     b.Items    `json:"items"`

	Stamp b.Stamp `json:"stamp"`
	Blob  b.Blob  `json:"blob"`
	Tree  b.Tree  `json:"tree"`
	Addr  b.Addr  `json:"addr"`

	Status  b.Status   `json:"status"`
	PStatus *b.PStatus `json:"pStatus"`
	StatusA StatusA    `json:"statusA"`
	StatusD StatusD    `json:"statusD"`

	Width b.Width `json:"width"`

	Schedule    scheduler.ScheduleType `json:"schedule"`
	Decimal     decimal.Decimal        `json:"decimal"`
	AliasedTime t.Time                 `json:"aliasedTime"`
}

// QualifiedParams binds qualified named types as parameters.
type QualifiedParams struct {
	Code b.Code    `param:"code"`
	Min  b.Cents   `query:"min"`
	Many []b.Cents `query:"many"`
	Tags b.Tags    `query:"tags"`
	St   b.Status  `query:"st"`
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/qualified/:code", m.get)
	server.GET(hr, r, "/cents", m.cents)
}

func (m *Module) get(req QualifiedParams, ctx server.HandlerContext) (server.Result[Qualified], server.IAPIError) {
	return server.NewResult(http.StatusOK, Qualified{}), nil
}

// cents keeps today's payload fallback: payloads are resolved by #110.
func (m *Module) cents(ctx server.HandlerContext) (server.Result[b.Cents], server.IAPIError) {
	return server.NewResult(http.StatusOK, b.Cents(0)), nil
}
```

- [x] **Step 2: `named_fallback`.** Delete `internal/spectest/testdata/named_fallback/types/cents.go` (and the now-empty `types/`), remove the import line `"github.com/example/namedfallback/types"` (`api.go:9`) and the field line `Cents       types.Cents        …` (`api.go:90`). The remaining struct's alignment is unchanged.
- [x] **Step 3:** `go test ./internal/spectest -update`, then `git status --porcelain internal/spectest/testdata` → only `?? …/qualified_named_types/` and the `named_fallback/` edits. `git diff internal/spectest/testdata/named_fallback/expected.yaml` → exactly the two removed lines `cents:` / `type: object`.
- [x] **Step 4: check the new golden cell by cell** against the behavior table (Q1–Q13). Its components are exactly `ErrorResponse`, `Item`, `Qualified`, `QualifiednamedtypesUser`, `User` (b's `User` registers first; the route package's `User` takes the qualified name; nothing is orphaned). `/cents`'s `data` is `{type: object, description: Response data}`. Then `go test ./internal/spectest` (no `-update`) green.

## Task 6: Commands: strict-clean qualified project

**File:** create `internal/commands/qualified_named_types_test.go`. The golden fixture carries warned rows, so the brief's strict criterion gets its own temp project whose only non-local field types are resolvable in-module qualified types. It reuses `writeProject` (`doctor_test.go:727`), `minGoBricksVer`, `outputFileName`, `runGenerate` and `testutil.CaptureStdout`.

- [x] **Step 1:** Write the file:

```go
package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/testutil"
)

// qualifiedStrictModSrc is a module whose only non-local field types are
// named types from other packages of the module: b (also imported as money),
// b's import c, and kinds.
const qualifiedStrictModSrc = `package svc

import (
	"github.com/example/svc/b"
	"github.com/example/svc/kinds"
	money "github.com/example/svc/b"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Code int32
type User struct {
	ID int ` + "`json:\"id\"`" + `
}
type Discount b.Cents
type Tags []b.Tags

type Body struct {
	b.Base
	Cents    b.Cents            ` + "`json:\"cents\" validate:\"min=1\"`" + `
	Money    money.Cents        ` + "`json:\"money\"`" + `
	Code     b.Code             ` + "`json:\"code\"`" + `
	Local    Code               ` + "`json:\"local\"`" + `
	Discount Discount           ` + "`json:\"discount\"`" + `
	ByKey    map[string]b.Cents ` + "`json:\"byKey\"`" + `
	Tags     Tags               ` + "`json:\"tags\"`" + `
	Labels   kinds.Labels       ` + "`json:\"labels\"`" + `
	Users    b.UserList         ` + "`json:\"users\"`" + `
	Owner    User               ` + "`json:\"owner\"`" + `
	Items    b.Items            ` + "`json:\"items\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/body", m.body, server.WithTags("svc"))
}

func (m *Module) body(ctx server.HandlerContext) (server.Result[Body], server.IAPIError) {
	return server.Result[Body]{}, nil
}
`

// TestRunGenerateQualifiedNamedTypesStrictClean pins that a project whose
// only non-local field types are resolvable named types from other packages
// of the module passes --strict --validate with no warning (#100).
func TestRunGenerateQualifiedNamedTypesStrictClean(t *testing.T) {
	goMod := "module github.com/example/svc\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks " + minGoBricksVer + "\n"
	dir := writeProject(t, goMod, qualifiedStrictModSrc)
	for rel, src := range map[string]string{
		filepath.Join("b", "b.go"): "package b\n\nimport \"github.com/example/svc/c\"\n\n" +
			"type Cents int64\ntype Code string\ntype Tags []string\ntype Grade c.Level\n" +
			"type Base struct {\n\tGrade Grade `json:\"grade\"`\n}\n" +
			"type User struct {\n\tName string `json:\"name\"`\n}\ntype UserList []User\ntype Items []c.Item\n",
		filepath.Join("c", "c.go"):         "package c\n\ntype Level int32\n\ntype Item struct {\n\tID int `json:\"id\"`\n}\n",
		filepath.Join("kinds", "kinds.go"): "package kinds\n\ntype Labels map[string]string\n",
	} {
		full := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
		require.NoError(t, os.WriteFile(full, []byte(src), 0o600))
	}

	out := filepath.Join(t.TempDir(), outputFileName)
	var runErr error
	stdout := testutil.CaptureStdout(t, func() {
		runErr = runGenerate(context.Background(), &GenerateOptions{ProjectRoot: dir, OutputFile: out, Strict: true, Validate: true})
	})
	require.NoError(t, runErr, stdout)
	assert.Contains(t, stdout, "Warnings: 0\n")
}
```

- [x] **Step 2:** `go test ./internal/commands -run TestRunGenerateQualifiedNamedTypesStrictClean` green (on `deedfa9` it fails: every qualified field warns).

## Task 7: Docs

- [x] **Step 1: README Known limitations** (`README.md:256–262`). Replace the unresolvable-field bullet's cause list so it names only out-of-module types and well-known types under an aliased import, and drop "a named non-struct type from another package of the project (`b.Cents`)":

```markdown
- A field type that resolves to no schema — a type from outside the module
  (`decimal.Decimal`, a go-bricks type such as `scheduler.ScheduleType`), a
  well-known type under an aliased import (`t.Time`, `j.RawMessage`), or a
  defined type over a well-known struct (`type Stamp time.Time`,
  `type ID uuid.UUID`) — is documented as an untyped object with a warning (so
  `--strict` fails on it). Named types from other packages of the project,
  under any import name, are resolved in their own package like local ones.
```

  No other README text names other-package named types as a fallback (`grep -n -i 'another package\|other package' README.md`: lines 297 and 338 are about route constants and directives, unrelated).
- [x] **Step 2: `CONTEXT.md`: no change.** **Resolution** already covers "what a named non-struct type stands for" with no local-only wording. **`docs/adr/0002`: no change** (accepted ADRs are not amended; its staging bullet names PR 1, #110 and #111, and this PR is within its decision). See open decision 6.
- [x] **Step 3: code comments** (Task 2 Step 6) and the deleted `named_fallback/types/cents.go` (its "not resolved yet (#100)" comment goes with it).

## Task 8: Lint, complexity, coverage, mutation

- [x] **Step 1:** `make check` (pinned golangci-lint; it runs `go fmt` first). Expected `0 issues`. The only goconst trap (`"uint64"`) is handled in Task 2 Step 1.
- [x] **Step 2:** `go run github.com/uudashr/gocognit/cmd/gocognit@latest -over 15 internal/analyzer/resolution.go internal/analyzer/builtins.go internal/analyzer/qualified_test.go internal/spectest/qualified_oracle_test.go internal/commands/qualified_named_types_test.go` → no output, exit 0. Compare against the budget table.
- [x] **Step 3: coverage.** `go test ./internal/analyzer -coverprofile=/tmp/cov.out && go tool cover -func=/tmp/cov.out | grep resolution.go | awk '$3!="100.0%"'` → no output (every function in `resolution.go` at 100%, as on the prototype). `inModuleTypeSite`'s branches are covered by `TestQualifiedLookupDegenerate` (`nope.X`: unreadable dir; `g.OnlyMain`: `main` skip and not-declared; `zz.X` and `decimal.Decimal`: not in-module) and `TestQualifiedNamedRows` (found).
- [x] **Step 4: mutation checks.** Apply each, run `go test ./internal/analyzer ./internal/spectest ./internal/commands`, confirm the named tests fail, restore with `git checkout -- internal/analyzer/resolution.go` (empty `git diff` after each). Prototype results:
  1. **Lookup reports "not found"** (`inModuleTypeSite` returns `pkgFile{}, "", false` first) → `TestQualifiedNamedRows`, `TestQualifiedShadowing`, `TestQualifiedWarnedRowsInEveryPosition`, `TestQualifiedByteSliceRule`, `TestQualifiedLookupDegenerate`, `TestQualifiedBuildTaggedVariants` (all three subtests), `TestQualifiedInSiblingFile`, `TestUnresolvableFieldsWarn` (Task 3's `cents` assertion), `TestQualifiedNamedResolutionOracle`, `TestGoldenFixtures/qualified_named_types`, `TestRunGenerateQualifiedNamedTypesStrictClean` fail, with no panic.
  2. **Post-lookup steps run in the route file** (`resolveQualified`'s call becomes `_ = site` then `return a.resolveLocal(…, c.in(c.file, c.path), identityMode)`; without the `_ = site` line, `c.in(c.file, c.path)` and a bare `c` alike leave `site` unused and the package does not compile (`declared and not used: site`), which is not the mutation failing) → `TestQualifiedShadowing` (b.Code becomes the local `int32`; `b.UserList`, `b.Items`, `b.Codes` fall back), `TestQualifiedNamedRows`, `TestQualifiedWarnedRowsInEveryPosition`, `TestQualifiedByteSliceRule`, `TestQualifiedLookupDegenerate`, `TestQualifiedBuildTaggedVariants`, `TestQualifiedInSiblingFile`, `TestUnresolvableFieldsWarn`, the golden and the strict test fail. The same mutation in `resolveUnderlying` (same `_ = site` plus `c.in(c.file, c.path)` form) fails `TestQualifiedNamedRows`, the golden and the strict test. (The oracle alone cannot catch it: q's names equal the local ones; that is what the shadowing rows are for.)
  3. **Recursion key by name only** (`typeKey` returns `name`) → `TestQualifiedShadowing` and `TestQualifiedNamedRows` (`localTags` cut as recursive), the golden and the strict test fail.
  4. **Byte-slice rule off** (`marshalerLeaf`'s condition prefixed with `false &&`) → `TestQualifiedByteSliceRule`, `TestQualifiedNamedRows` (`us`), both oracles, PR 1's `TestByteSliceElementRule`/`TestResolveFieldRows`, `TestGoldenFixtures/named_resolution` and `TestRunGenerateNamedResolutionFixtureStrictClean` fail. **4b:** restoring PR 1's `resolveSliceElem` body (and dropping `marshalerLeaf`'s rule) must fail `TestQualifiedByteSliceRule`: on the prototype binary that variant emitted `fvs` as `{type: string, format: byte}` and `us` as `{type: array, items: {}}` (verified on the binary, not yet through this test).
  5. **No `main` filter** (drop `f.Name.Name != mainPackageName &&`) → `TestQualifiedBuildTaggedVariants/a_package_main_generator_sorts_first` and `TestQualifiedLookupDegenerate` (`g.OnlyMain`) fail. The golden does not catch it, by design (`gen.go` sorts after `b.go`).
  6. **Display never qualifies** (`display` always returns `name`) → `TestQualifiedWarnedRowsInEveryPosition` fails.
  7. **`quiet()` shares state** (its body becomes `return c`) → `TestQualifiedWarnedRowsInEveryPosition` fails, on its `b.MStamp` and `LStamp` rows in all seven positions (each warns "is a defined type over time.Time … emitting an untyped object" instead of "has its own MarshalJSON method"; the render stays `marshal:…`). Only that test fails: spectest and commands pass, because no fixture holds a Marshaler type whose underlying run notes anything (`b.Status` over `int` notes nothing), so do not expect a golden to catch it.

## Task 9: Commit

- [x] One commit (squash-ready), signed (1Password must be unlocked; never disable signing). Write the message to a file and run `git commit -F <file>`: the commit hook blocks heredoc `-m`. Subject (Conventional Commit; `fix` per the brief; changelog "Fixed", PATCH pre-1.0; after Task 0's rebase the branch carries exactly this one commit on top of `main`, so the PR squashes under this subject, not the PR title. Before committing, `git fetch -q origin`; if `origin/main` moved, `git rebase origin/main` and rerun Task 8 Step 1, since the ruleset requires the branch to be up to date):

```
fix(analyzer): resolve named types from other in-module packages
```

Body (standalone):

```
A named non-struct type declared in another package of the module
(b.Cents, b.Tags, kinds.Labels, money.Cents through an import alias)
now documents exactly what the same declaration made locally documents,
in every field position and as a parameter. Before, every such field was
an untyped object with a "resolves to no schema" warning, so --strict
failed on valid code.

The resolver gains one lookup, inModuleTypeSite: the qualifier maps
through the file's imports to an in-module directory, and the first
sorted file there that declares the name, among files of the importable
package, is the site. That package's clause is the clause of the first
sorted file that is neither package main nor build-ignored, so a
//go:build ignore generator or tool of another clause never picks it; a
build-ignored file sharing the package's clause is merged like any
build-tagged declaration (build constraints are not evaluated).
Both qualified paths use it after the stdlib checks: a qualified field
leaf (identity mode) and a defined type over a qualified type
(underlying mode, so type StatusD b.Status drops b.Status's methods).
The short name then resolves through the existing local path in the
declaring file's context: every build-tagged declaration of it in that
package is merged, its Marshaler methods are read there, its own imports
and local names apply (b's Code is a string beside a local Code int32;
b.Items reaches c's Item though the route package never imports c), and
a struct leaf registers in that package, as the same component a direct
b.User field references. A qualified alias or defined type over a struct
(b.UA, b.UD) now becomes a $ref to a component registered in b.

That registration can rename an existing component. Components are keyed
by short name and the first registration takes the bare name, so when
b.UserList registers b's User before the route package's own User, the
local struct becomes QualifiednamedtypesUser, which moves its $refs and
generated client type names; consumers must regenerate clients.

Components are now keyed by declaring directory as well as package
clause and type name. Before, two packages sharing a clause
(orders/model and users/model, or v2/api beside a root package api)
shared one component per type name, so a users/model.Item field was
documented as orders/model.Item's schema with no warning; that already
happened for a direct struct field, and resolving qualified named types
would have extended it, silently, to every named type over such a struct
(type Items []Item). The second package's struct now gets its own
component under the usual collision name (ModelItem), which moves $refs
and client type names in projects with such packages. A named type over
a struct is keyed by its own declaring site, not the struct's: a local
type User b.Record and b's own type User RecordV2 used to share one
component, documented with whichever struct registered first, and are
now two (User, BUser). An alias re-exporting a same-named struct of
another package (type User = dto.User) is that struct, so it keeps the
key a direct dto.User reference takes and the two share one component,
as before; an alias under another name (type UA = dto.User) keeps its
own.

Three resolver-state changes make that reuse correct:
- The recursion stack is keyed by package directory and name, so a local
  type Tags []b.Tags is not cut as recursive.
- Warnings name another package's types by package clause
  (b.Tree contains itself, b.Status has its own MarshalText method).
- The byte-slice rule (#97) is decided inside resolveLocal, where the
  element's own methods are known, through a byteElem context flag.
  #128 re-scanned the methods in the slice's file, which once qualified
  types resolve would document []FV (FV = b.FlagV, which has
  MarshalJSON) as base64 silently and []b.FlagU (decode-only) as {}.
  The Marshaler warning is now noted after the quiet run of the type's
  underlying form, so that run's private state is required: a Marshaler
  type over time.Time must not warn as a defined type over time.Time.

Build-tagged declarations are merged leaf by leaf, and a struct leaf is
a ref by short name; each declaration now records its ref sites
separately, so variants reaching two structs of one short name in
different packages (type W dw.Items beside type W lx.Items) disagree
and warn instead of documenting the first one's struct for both.

Other-package Marshaler types (b.Status, *b.PStatus) become {} and still
fail --strict, now with the Marshaler warning; qualified uintptr
wrappers (b.Addr) stay object with the uintptr warning instead.

The unresolvable warning's causes no longer include other-package types:
it now names a type from outside the module, a well-known type under an
aliased import, an unaliased in-module import whose directory's first
file has another package clause (#122), or an undeclared name. The
defined-over-qualified warning says "a type from outside the module".
Payloads are unchanged: Result[b.Cents] keeps its untyped data schema
and warning (#110).

Residuals: an unaliased import of a directory whose first sorted file
has another package clause (a package main generator or another
build-ignored file) still maps under that clause (#122), named in the
README; a directory holding two non-main, non-ignored package clauses
resolves through the first such file's clause.

Tests: analyzer row, shadowing, every-position warning, byte-slice,
lookup-miss, build-tag, package-main, build-ignored-tool,
same-clause-package, named-over-struct site keying, build-tagged ref
clash and buildIgnored tests; a spectest oracle comparing each
Marshaler-free oracle type declared in another package with its local
twin in seven positions and as a query parameter; a --strict clean
project. TestUnresolvableFieldsWarn's types.Cents row now asserts the
resolution, with an undeclared types.Missing keeping the warned
in-module case. New golden qualified_named_types. The named_fallback
golden loses its cents row, deleted with its types package because it
no longer falls back.

BREAKING CHANGE: generated component names can change. A route-package
struct can lose its bare name to a same-named struct now registered
through another package's named type (User becomes
QualifiednamedtypesUser), and a second package sharing a clause, or a
named type over a struct sharing a name with one in the struct's
package, gets <Pkg><Name> (ModelItem, BUser). $refs and generated
client type names move; regenerate clients.

Closes #100

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

PR body (global CLAUDE.md: three headings, ≤3 sentences each, under 150 words, final attribution line; personal repo, no `Refs:`):

```markdown
## What
Named non-struct types from another package of the module (`b.Cents`, `money.Tags`, `kinds.Labels`) fell back to `object` with a warning; they now resolve in their declaring package and emit what the same local declaration emits, in every position. Closes #100.

## Impact
These fields change schema and lose their warnings, so `--strict` passes; other-package Marshaler types become `{}` and still fail it. Struct leaves reached through them now register as components (b's `User`, c's `Item`), and components are keyed by declaring site, so a same-named struct can become `<Pkg><Name>` (`QualifiednamedtypesUser`, `ModelItem`, `BUser`), changing its `$ref`s and generated client types: regenerate clients. Warnings now say "a type from outside the module"; update log matchers.

## Verification
New `qualified_named_types` golden; `named_fallback` moved by one row. Mutation checks bit: a failing lookup, route-file resolution, a name-only recursion key, the byte-slice rule, shared quiet-run state, struct-site keying and ref-clash merging fail the new tests.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

---

## Impact (user-visible behavior changes; SemVer surfaces: generated-output shape, doctor/validation)

Relative to PR 1 (#128, `feeafaa` on `main`, unreleased; tree-identical to `deedfa9`). If #128 and this PR ship in one release, the net change for these fields relative to the previous release is "untyped `object`, no warning" → resolved schema.

1. In-module qualified named non-struct types (`b.Cents`, `money.Cents`, `b.Tags`, `kinds.Labels`, chains such as `b.Wrapped`, `b.Grade` over `c.Level`, local `type Discount b.Cents` and `Cents2 = b.Cents`) move from `object` to their resolved schema in every position (pointer, slice, nested slice, map value, pointer-to-map, slice of maps), with `validate` keywords applied (`minimum`/`maximum`, `minLength`/`maxLength`), and `*b.Tags` has no `nullable`.
2. Struct leaves reached through them now emit `$ref`s and register their components in the declaring package (b's `User` via `b.UserList`, c's `Item` via `b.Items`), including components that were never emitted before. A qualified alias or defined type over a struct (`b.UA`, `b.UD`) becomes a `$ref`. **This can rename an existing component** (generated-output-shape SemVer surface): components are keyed by short name and the first registration takes the bare name, so a route-package struct sharing a short name with a struct leaf now reached through a qualified named type loses the bare name when that field registers first, and becomes `<Pkg><Name>` (Q8: the local `User` becomes `QualifiednamedtypesUser`, as in the `qualified_named_types` golden's component list). Its `$ref` paths and generated client type names change even if no field names `b.User` directly; consumers must regenerate clients.
3. PR 1's unresolvable warnings for these fields disappear: **`--strict` passes on them again**, and `doctor` stops reporting them as caveats.
4. Marshaler types from other packages (`b.Status`, `*b.PStatus`, `StatusA = b.Status`) move from `object` to `{}`, and **still fail `--strict`**, now with the Marshaler-type warning instead of the unresolvable one. `type StatusD b.Status` becomes `{integer, int64}`.
5. Qualified recursive types (`b.Tree`) become `{type: object, additionalProperties: {}}` with the recursion warning; qualified `uintptr` wrappers (`b.Addr`) stay `object` with the **uintptr warning** instead of the unresolvable one.
6. Parameters of qualified named types are typed like local ones (`path code b.Code` → string, `query many []b.Cents` → integer array); a Marshaler parameter (`st b.Status`) is typed by kind (`{integer, int64}`), unwarned.
7. Build-tagged qualified types merge like local ones (`b.Width` → `{type: integer}`). `package main` generators and `//go:build ignore` files of another clause never pick the imported directory's package; a build-ignored file sharing the package's clause is merged like any build-tagged declaration (build constraints are not evaluated). Variants reaching two structs of one short name in different packages (`type W dw.Items` beside `type W lx.Items`) disagree and warn instead of merging into the first one's struct.
7b. **Components are keyed by declaring directory** (generated-output-shape SemVer surface): two packages sharing a clause (`orders/model`, `users/model`) no longer share one component per type name; the second gets `<Pkg><Name>` (`ModelItem`), moving its `$ref`s and client type names. This also fixes direct struct fields (`v2.User` beside a root package `api`), which documented the wrong package's struct with no warning. A named type over a struct is keyed by its own declaring site, not the struct's (added in review round 3): before that, a local `type User b.Record` and b's `type User RecordV2` shared one component, so directory keying alone did not close the same-name collision for named types over structs.
8. Byte-slice rule across packages: `[]b.FlagU` (decode-only byte) is base64 like its local twin; a slice of a local alias of another package's encode-side byte Marshaler stays `{type: array, items: {}}` with a warning.
9. Warning text: the unresolvable and defined-over-qualified warnings now say "a type from outside the module" (the "named type from another package" cause is gone); other-package types are named clause-qualified (`b.Tree contains itself`). Consumers matching the old text must update.
10. Out-of-module types (`decimal.Decimal`, `scheduler.ScheduleType`), well-known types under an aliased import (`t.Time`) and `Result[b.Cents]` payloads are unchanged.

---

## Open decisions (recommended default first)

1. **`named_fallback`'s `Cents types.Cents` row. Default: delete the row and `named_fallback/types/`**, so the fixture keeps its premise (every field warns once) and the golden diff is two removed lines, named in the PR. Alternative: keep the row; the golden's `cents` moves `object` → `{integer, int64}` and the fixture's header comment must then carve out an exception. The brief expected no move here (it guessed the row lived in `named_resolution`), so the move is the user's to accept.
2. **Lookup order in `resolveQualified`/`resolveUnderlying`. Default: after the stdlib checks** (struct, `knownUnderlyingBuiltins`, well-known / `kindBackedUnderlying`). Zero golden risk, and a project that vendors an in-module `uuid` package with `type UUID [16]byte` keeps today's `{string, uuid}`. Alternative: in-module first, which documents such a vendored type from its own declaration (an integer array, or `{}` if it has `MarshalText`).
3. **Which files form "the imported package". Default: every non-`main` file; the site is the first sorted one declaring the name, and `localTypeDecls` then filters by that file's clause.** A directory with two non-`main` clauses (not buildable) resolves to the first sorted declaring file. **Revised in review:** the premise fails for build-ignored files (Go excludes them and the directory builds), so build-ignored files are skipped too and the site must share the first remaining file's clause. Alternative: require the clause to equal the qualifier for unaliased imports, which still needs this rule for aliased ones (`money`).
4. **Clause-qualified names in warnings (`b.Tree contains itself`). Default: yes**; it disambiguates a local `Tree` from b's when the field's declared type is a local wrapper (`has type W: b.Tree contains itself`). Leaf names in the Resolution stay short. Alternative: bare names (smaller diff, ambiguous text).
5. **Byte-slice rule refactor (`byteElem`). Not open in substance:** without it, qualified resolution introduces a silent misdocumentation (`[]FV` as base64) and a parity break (`[]b.FlagU`). Flagged because it rewrites PR 1 code (`resolveSliceElem`, the Marshaler branch); every PR 1 test passes unchanged.
6. **No ADR 0002 or CONTEXT.md edit. Default: none.** Alternative: append "and other in-module packages' named types (#100)" to ADR 0002's staging bullet.
7. **`Closes #100`. Default: yes.** Every table row is fixed; the out-of-scope items each have their own issue (#110, #111, #113, #114, #121, #122, #123).
8. **Commit type for a change that can rename an existing component (Q8, Impact item 2). Decided in review round 3: `fix(analyzer):` with a `BREAKING CHANGE:` footer** (CLAUDE.md and RELEASING.md require the marker; pre-1.0 caps the bump to MINOR). The original default, kept here for the record, was plain `fix(analyzer):` with no footer. Every prior output-shape change on `main` shipped as an unmarked `fix` (#90, #92, #105, #106, #108, #124–#128; `git log --grep=BREAKING` finds only `feat(commands)!` #26, a CLI-surface change), and the rename is a collision side effect of fixing previously untyped fields. Alternative: add a `BREAKING CHANGE:` footer (CLAUDE.md: mark breaking changes even though pre-1.0 caps the bump to MINOR), which turns this release's bump MINOR.

## Review dispositions

- **Major, Impact item 2 / PR-body `## Impact` / Q8: struct-leaf registration can rename an existing component. Applied.** Reproduced on `probe2` (the golden fixture minus its direct `BUser b.User` field) with the reviewer's binary and the planner's `proto-bin`: `deedfa9` gives `[ErrorResponse, Qualified, User]`, `localUser → User`; the prototype gives `[ErrorResponse, Item, Qualified, QualifiednamedtypesUser, User]`, `localUser → QualifiednamedtypesUser`. Swapping the two fields (`probe3`) keeps the local `User` bare and names b's `BUser`, so the rename is order-dependent; the Q8 row, Impact item 2, the PR body's `## Impact` and the commit body now say so, and open decision 8 records the commit-type question. One correction: the golden is cited as showing the resulting names only, since its direct `b.User` field gave the same names on `deedfa9`; `TestQualifiedShadowing` is the no-direct-field case.

- **Major (two findings, one issue), Global Constraints Branch/Stacked PR, Baseline, Task 0 Step 1, Task 9: the stacked base is gone. Applied.** Verified: #128 is `MERGED` (2026-10-09T00:27:04Z) as `feeafaa` on `origin/main`; `git ls-remote --heads origin fix/named-type-resolution` is empty; `deedfa9`, `feeafaa` and `origin/main` share tree `54516b4`; `git merge-base HEAD origin/main` is `514e417`. The branch now moves onto `origin/main` in Task 0 Step 1 and targets `main`; the stacked-PR/retarget/`-X ours` paragraph is deleted; Baseline, Spec and Impact name `feeafaa`; Task 9 adds a re-fetch/rebase before committing so the one-commit squash subject and the strict up-to-date rule both hold. `deedfa9` stays as prototype provenance (same tree). Beyond the finding: Task 0's "clean `git status`" was false (this plan file is untracked); the precondition now expects exactly that one line.
- **Major, `qualified_test.go` unguarded `typeRegistry` dereferences masking Task 1 Step 3 and mutation 1. Applied.** Reproduced on the `gates_rev` prototype under mutation 1: `TestQualifiedLookupDegenerate` SIGSEGVs at the `UA` dereference and the run lists five failures. Added `require.Contains` before the `Item`, `UA`, `User` and (beyond the finding) `localUser` dereferences; `renders` callers were already safe via its `require.NotNil`. Re-ran Task 1's red state (`deedfa9` export + stub + guarded file): exactly the seven listed tests fail, no panic, every PR 1 test passes. Re-ran all six mutations with the guards: mutation 1 adds `TestQualifiedBuildTaggedVariants`, `TestQualifiedInSiblingFile` and (beyond the finding) `TestUnresolvableFieldsWarn`; mutation 2 as written (`passes c`) did not compile (unused `site`), so it is restated with `c.in(c.file, c.path)` (corrected in the next round: that form needs a preceding `_ = site` too, see the `quiet()` disposition) and its list gains the same three tests; mutations 2b, 3, 4, 5 and 6 match the plan unchanged.
- **Major, `quiet()` isolation becomes load-bearing once `marshalerLeaf` notes after the quiet run; its doc comment says the opposite and nothing pins it. Applied.** Verified: `deedfa9`'s `quiet()` comment (resolution.go 155–160) calls the isolation "unobservable" because the note came first, and Step 6 did not list it. Reproduced on `qi` (the plan applied to a `deedfa9` export): with the plan's test file as it stood, `quiet()` → `return c` passes analyzer, spectest and commands. Added `b.MStamp` (b) and local `LStamp`, each `time.Time` with `MarshalJSON`, as rows of `TestQualifiedWarnedRowsInEveryPosition` (stricter than the suggested "contains MarshalJSON": the clause-qualified name and seven positions); mutation 7 now fails exactly that test (all 14 fields warn "is a defined type over time.Time … emitting an untyped object"). The comment's recursion case was probed too (throwaway `type RT map[string]RT` with `MarshalJSON` in b: "has its own MarshalJSON method" as planned, "b.RT contains itself" under mutation 7); the depth cap was not probed, so the comment does not name it. `quiet()`'s new comment is in "Byte-slice rule", wired into the data-model row and Task 2 Steps 2 and 6; the Equivalence paragraph, commit body, PR `## Verification` (145 words) and self-review name the invariant. Re-checked: red state still exactly the seven tests, no panic (`LStamp` rows pass at red); mutations 1–6 fail the listed tests unchanged; full suite green; pinned golangci-lint v2.12.2 `0 issues`; gocognit clean. Beyond the finding: mutation 2's restated `c.in(c.file, c.path)` form does not compile either (`declared and not used: site`); items 2 and 2b now prefix `_ = site`.

- **Major, `inModuleTypeSite` skips only `package main`, so a first-sorted `//go:build ignore` tool of another clause becomes the site. Applied.** Reproduced (aliased import, `al/a_tool.go` `package tool` `type Cents string` beside `al/al.go` `type Cents int64`: `alCents: {type: string}`, no warning; Go builds the directory and marshals a number). New `importableClause` takes the clause of the first sorted file that is neither `main` nor build-ignored (`buildIgnored`: the `//go:build`/`// +build` line holds with every tag on but not with `ignore` alone off, so `!linux` is not ignored); the site must share it. Pinned by a `TestQualifiedBuildTaggedVariants` case and `TestBuildIgnored`. `unaliasedImportName` is untouched (#122 stays out of scope).
- **Major, components keyed by package clause, so `orders/model.Item` and `users/model.Item` share one component; this PR widened the collision to every qualified named type over a struct and removed its warning. Applied.** `schemaKey` now also keys on the declaring directory (`registerStructAt` passes `filepath.Dir(filePath)`); the display rule is unchanged, so the second package's struct is `ModelItem`. This also fixes the pre-existing direct-struct case (`Other v2.User`). No golden moved. Pinned by `TestQualifiedSameClausePackages` and `TestSchemaKeyCollision`.
- **Minor, README claims every in-module named type resolves. Applied:** the sentence now names the #122 exception. Warning text unchanged.
- **Minor, dead `c.byteElem = false` in `resolveShape`'s slice arm. Applied:** deleted (`resolveSliceElem` sets the flag before any read).
- **Minor, commit body and a test comment say "PR 1". Applied:** both now say #128.

- **Review round 3 (fixer).** Applied: (major) a named type over a struct (`registerViaTypeSpecAt`) is keyed by its own declaring site, so a local `type User r.Record` and r's `type User RecordV2` are two components (`TestQualifiedNamedOverStructKeyedBySite`); (minor) build-tagged declarations record ref sites per declaration (`resolveEachDecl`/`foldRefSites`), so variants reaching dw.Item and lx.Item disagree and warn (`TestQualifiedBuildTaggedRefClash`); the README and the unresolvable warning name the #122 cause as any first-sorted file of another clause; the commit body's build-ignored sentence is corrected; `BREAKING CHANGE:` footer added (open decision 8); `schemaKey`'s doc says (dir, pkg, typeName). Declined as out of scope: making `unaliasedImportName` use `importableClause` (that is #122, with its own open points) and skipping same-clause build-ignored files (constraints are not evaluated anywhere else). No golden moved; both new tests fail with their fix reverted.

- **Review round 4 (fixer).** Applied: (major) round 3's site keying split the re-export idiom: `type User = dto.User` beside a direct `dto.User` registered two identical components (`User`, `DtoUser`) by discovery order, with no warning; base (feeafaa) shared one. `registerViaTypeSpecAt` now keys an alias of a same-named qualified struct (`isSameNamedReExport`) at that struct's site, the key a direct reference takes; an alias under another name and every defined type keep their own site. Pinned by `TestQualifiedAliasReExportSharesComponent` (two of five subtests fail at round 3; the other three already passed and guard). (minor) The README's build-constraints bullet names the `ignore` exception. Declined: the one-commit check's second commit is the stale local `fix/named-type-resolution` ref (pre-rebase); `main..HEAD` is one commit. No golden moved.

## Self-review: acceptance criteria → steps

- **Every table row has unit or golden coverage:** Q1–Q13 in `TestQualifiedNamedRows`/`TestQualifiedShadowing`/`TestQualifiedWarnedRowsInEveryPosition`/`TestQualifiedBuildTaggedVariants` (Task 1) and the golden (Task 5); params (Q12) in the golden and the oracle's query parameters.
- **Warning rows warn once per field** (Marshaler, `b.Tree`, `b.Addr`, out-of-module): `TestQualifiedWarnedRowsInEveryPosition`, seven positions each, `assert.Len(w, 1)`.
- **Parity test** (each in-module row equals its local twin apart from component names): `TestQualifiedNamedResolutionOracle` (Task 4).
- **Shadowing rows** (`b.Code`, `b.Codes`, local `Tags`, `b.UserList`) in the style of `TestPromotedNamedScalarResolvesInDeclaringPackage`: `TestQualifiedShadowing`; `b.Items` registers c's `Item`, and the golden's component list shows nothing orphaned (`Validate` would also reject a dangling `$ref`).
- **`--strict` exits 0 with `Warnings: 0`** on a non-golden project: `TestRunGenerateQualifiedNamedTypesStrictClean` (Task 6).
- **Out-of-module names keep the fallback and warn; `Result[b.Cents]` unchanged:** `TestQualifiedWarnedRowsInEveryPosition` (`decimal.Decimal`), `TestQualifiedLookupDegenerate`, `TestQualifiedPayloadUnchanged`, the golden's `/cents`.
- **Temp-dir tests for `b.Width` and the `package main` row:** `TestQualifiedBuildTaggedVariants` (three cases, aliased import, generator sorting first).
- **New golden** `qualified_named_types` with `b`, `c`, `kinds` and a `go.mod` per the fixture rules: Task 5.
- **No golden on main moves except the named one:** Task 5 Step 3 gate; the moved one is `named_fallback` (open decision 1), not `named_resolution`.
- **README Known limitations:** Task 7 Step 1.
- **Mutation checks** (lookup "not found"; post-lookup steps in the route file): Task 8 Step 4, items 1 and 2, plus five more (item 7 pins `quiet()`'s isolation).
- **`make check`, gocognit ≤15, ≥80% new-code coverage:** Task 8 Steps 1–3.
- **Commit type `fix(analyzer):` and the `## Impact` bullets:** Task 9 and the Impact list (items 1–5 cover the brief's four bullets).

## Execution notes

- **Branch:** the executor's task text cut `fix/qualified-named-types` from `fix/named-type-resolution` (`deedfa9`); Task 0 Step 1's `git rebase --onto origin/main deedfa9` then moved it to `feeafaa` with nothing replayed (trees identical, `54516b4`). Baseline `make check` was green there and `resolveLocal` (7) topped gocognit, as recorded. #100 still has exactly one comment (the 2026-09-29 brief), so no amendment applied.
- **Order within the TDD loop:** the oracle (Task 4) and strict-clean (Task 6) test files were written with Task 1's, before any implementation, and failed red with it (`TestQualifiedNamedResolutionOracle`, `TestRunGenerateQualifiedNamedTypesStrictClean`). Task 1 Step 3's red state matched the plan exactly: the seven named analyzer tests failed, nothing panicked, every PR 1 test passed. Task 2 Step 7's state matched too: only `TestUnresolvableFieldsWarn` and `TestGoldenFixtures/named_fallback` failed.
- **Goldens:** the single `-update` run touched only the new `qualified_named_types/` and `named_fallback/` (`api.go`, `expected.yaml`, deleted `types/cents.go`); the `named_fallback/expected.yaml` diff is exactly the removed `cents:` / `type: object`. The new golden was checked cell by cell against Q1–Q13; components are exactly `ErrorResponse`, `Item`, `Qualified`, `QualifiednamedtypesUser`, `User`, and `/cents`'s `data` stays `{type: object, description: Response data}`. The fixture's `module.go` was run through `gofmt -w` (Go tooling skips `testdata/`, so `make fmt` does not): it sorts the plan's `kinds` import after `money`, with no golden change, so `gofmt -l` keeps its single known hit (`raw_add/api.go`).
- **Mutation restore (deviation):** Task 8 Step 4's `git checkout -- internal/analyzer/resolution.go` would have reverted the uncommitted implementation to `feeafaa`, so each mutation was restored from a scratchpad copy of `resolution.go` instead, with an empty `diff` against it after every run. Mutation 1 was applied as an `if true { return pkgFile{}, "", false }` prefix (same effect as a bare early return). Every listed mutation (1, 2, 2b, 3, 4, 5, 6, 7) failed exactly the tests the plan names, with no panic. **4b** (PR 1's `resolveSliceElem` body restored, `marshalerLeaf`'s rule disabled), previously verified only on a binary, fails `TestQualifiedByteSliceRule`, `TestQualifiedNamedRows` and `TestQualifiedNamedResolutionOracle`.
- **Gates:** `make check` (pinned golangci-lint v2.12.2, `0 issues`), `go test -race -count=1 ./...` green. gocognit `-over 15` on the touched files reports only the pre-existing, untouched `TestMarshalerGuardPositions` (16, in `resolution_test.go`); the largest touched production function is `fileBuildConstraint` at 13, then `resolveLocal` and `inModuleTypeSite` at 7. Every function in `resolution.go` and `builtins.go` is at 100% statement coverage, both in the analyzer package's own profile and in the `./...` profile.
