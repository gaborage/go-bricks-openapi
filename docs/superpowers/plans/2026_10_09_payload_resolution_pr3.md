# Payload Resolution, PR 3 of ADR 0002 (#110) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A response payload of type `T`, either the type argument of `server.Result[T]` / `server.ResultWithMeta[T]` or a handler's bare first result, emits exactly the schema a struct field of type `T` emits after PR 1 and PR 2, and warns exactly when that field would. The root is the one exception: it sheds one pointer level of its **resolved** form and is never `nullable` (`Result[PC]`, with `type PC *int64`, is `{integer, int64}`). That is the brief's rule; `setTypeAndFormat` already strips any further level (`stripPointers`, #128), so `Result[*PC]` emits what `Result[PC]` emits (Review dispositions, finding 2). A **written** `**T` root (`Result[**int64]`) is not touched: it is #120's open payload question and stays unresolved here. Named non-struct payloads (`Cents`, `Tags`, `UserList`, `b.Cents`), unnamed composites (`map[string]int64`, `[][]Cents`, `*[]string`) and pointer-element slices (`[]*byte`) stop being an untyped `object`. Closes #110.

**Architecture:** `models.TypeInfo` gains the **Resolution** carrier PR 1 deliberately left out (`Resolution *TypeShape` plus `ResolvedShape()`). A new analyzer file, `internal/analyzer/payload.go`, runs PR 1/PR 2's resolver (`resolveShape` in a fresh `newResolveCtx`) on every payload that carries a Shape and did **not** register as a project struct, in `populateTypeFields`, before the two older screens (the local "named non-struct type" warning and `dropUnresolvablePayloadName`). Those screens keep handling a payload whose root resolves to nothing. Extraction (`payloadTypeInfo`) learns to carry a nameless TypeInfo for maps, nested slices and pointers to a slice, array, map or `interface{}` (the wrapper/bare path only; request extraction is untouched; a written `**T` root stays nil, #120). Because a request-less handler with such a payload now has a non-nil response, `handlerAnalysis.found()` turns true for it, so its success status and inferred error responses are documented instead of dropped (Impact 12). `slicePayloadTypeInfo` keeps the element pointer. The generator types a Resolution with the same `setTypeAndFormat` walk fields use (it becomes a plain function), and `inlinePayloadSchema`, `setElemTypeAndFormat` and most of `sliceResponsePayloadSchema` go away. `referencedSchemaNames` walks a response's Resolution for `$ref` leaves; request types stay unscanned. Doctor and `generate`'s route count classify payloads through one exported analyzer predicate, `IsTypedPayload`. A pre-existing context bug is fixed first, at two seams: `extractRoutesFromPackage` walked every `RegisterRoutes` file with the **Module-struct file's** path, and `extractHandlerSignature`'s sibling loop populated a handler found in another file of the package with that same path. Either mismatch made the resolver skip the file whose path it was given, so it searched the wrong files.

**Tech Stack:** Go 1.25 language floor; `go/ast` static analysis only; goldens in `internal/spectest`.

**Spec (authoritative, in this order):** #110's triage brief (its only comment, 2026-09-29; there are no dated amendments). The issue body's "stamp the payload with the equivalent scalar Shape" direction is superseded by ADR 0002 (Shape stays syntactic). Glossary: `CONTEXT.md` (**Shape**, **Resolution**, **Marshaler type**). Design: `docs/adr/0002-named-type-resolution.md`. Read PR 1's plan (`docs/superpowers/plans/2026_10_08_named_type_resolution_pr1.md`, **Execution notes**), PR 2's plan (`docs/superpowers/plans/2026_10_08_qualified_named_types_pr2.md`, **Execution notes**), and the bodies of `feeafaa` (#128) and `9d2b78a` (`git log -2 fix/qualified-named-types`).

**Base:** branch `fix/payload-resolution`, cut from `fix/qualified-named-types` at `9d2b78a` (PR 2, #129, **open, not on `main`**). Every anchor below is on `9d2b78a`. Because the base is unmerged, every anchor is given with a `grep -n` recipe as well as a line number. Re-run the recipe before editing. Rebasing onto `main` after #129 merges is the orchestrator's job.

**Prototype evidence (planner, 2026-10-09):** the analyzer, generator, doctor and fixture changes below were applied to a scratch copy of `9d2b78a` (outside the repo) and run:
- The pinned golangci-lint gave `0 issues` once the two gocritic findings named in Task 3 were fixed.
- `go test ./internal/spectest -update` changed exactly three existing goldens: `named_types`, `qualified_named_types` and `slice_result`. The `-update` run used the brief's two new fixtures. Retiring `inlinePayloadSchema` afterwards changed no golden.
- `generate --strict --validate` on the resolved-rows fixture reported `Warnings: 0`. The fallback fixture raised exactly one warning per route.
- The "After" cells below are that prototype's output. The "Today" cells are a `9d2b78a` binary's output on the same scratch project (`scratchpad/pr3/np`, built by `scratchpad/pr3/gen/mk.py`).
- The full list of existing tests the change breaks is in Task 8. Each one is a deliberate update.
- The `scratchpad/pr3/...` paths cited below (`np`, `sib`, `review3_split`, `review3_t1*_bin`, `gates_par`, `chain`, `ren`, `np_fixture.yaml`, `npf_fixture.yaml`) are session-local evidence only. Executing the plan does not need them: every expected cell is in the tables.

---

## Stale points in the brief (checked against `9d2b78a`)

1. **Bare returns now carry a Shape.** The brief puts bare returns (#119) out of scope and says "only the wrapper path stamps [a Shape]". #124 (`2fe498d`; #119 is now closed by it) routes a bare first result through the same `payloadTypeInfo`, so bare returns **do** carry a Shape and resolve exactly like `Result[T]`. #124's own body hands maps to #110 ("A bare map[string]T stays the untyped object that Result[map[string]T] gives today (#110 owns maps)"). Parity of bare and wrapped payloads is pinned by `TestRunGenerateBareReturnMatchesResult`, which keeps asserting it (three of its rows flip to typed, Task 8).
2. **Request types are not touched.** `populateRequestType` (#127) screens every non-struct request, and requests never carry a Shape, so nothing in this PR can reach them. `req Cents` keeps #127's `request type Cents is not a struct …` warning, not the old (A) text the brief quotes.
3. **Three goldens move, not two.** PR 2 added `qualified_named_types`, whose `/cents` route (`server.Result[b.Cents]`) PR 2's execution notes pin as `{type: object, description: Response data}`. It becomes `{type: integer, format: int64}`. The brief allowed for this ("A golden PR 1 or PR 2 added moves only if it has a non-struct or unnamed-composite payload; name each in the PR").
4. **A field bug surfaces on the payload path** (Task 1). The `(AST, path)` pair handed to the resolver is mismatched at two seams, and both carry the path of the file that declares the **Module struct** (`analyzeFile` passes its own `filePath` into `extractRoutesFromPackage`, `grep -n 'module.Routes = a.extractRoutesFromPackage' internal/analyzer/analyzer.go`, 498):
   - **Seam A, `extractRoutesFromPackage`** (686–690): every file holding a `RegisterRoutes` is walked as `collectRoutesFromFile(file, filePath, …)`, so `walkSetup` pairs the routes file's AST with the Module-struct file's path. `extractHandlerSignature`'s **first** branch (handler in the routes file) then calls `populateRequestType`/`populateTypeFields` with that pair. Repro: `module.go` holds the Module struct and `type Tags []string`; `routes.go` holds `RegisterRoutes`, `type Resp struct{ T Tags }` and the handler. `9d2b78a` warns `field T at routes.go:10:2 has type Tags: Tags resolves to no schema` and documents `t: {type: object}` (`scratchpad/pr3/review3_split`).
   - **Seam B, `extractHandlerSignature`'s sibling loop** (3075–3078): a handler found in another file than the walked one is populated with that file's AST and the walked file's path. A field `T Tags` in `handlers.go`, with `type Tags []string` in `module.go`, warns `Tags resolves to no schema` (`scratchpad/pr3/sib`).

   `samePackageFiles` skips the file at `p == path`, so in both cases the file whose path was passed is invisible. Fixing only seam B leaves seam A's repro warning (`scratchpad/pr3/review3_t1_bin`); fixing both gives `Warnings: 0` and `t: {type: array, items: {type: string}}` with no golden diff (`review3_t1b_bin`). Payload resolution uses the same pair, so both must be fixed first.
5. **`TestQualifiedPayloadUnchanged`** (`internal/analyzer/qualified_test.go`, PR 2) pins the old payload fallback for `Result[b.Cents]`. This PR updates it deliberately.

---

## Global Constraints

- **Worktree:** `/Users/gaborage/Projects/gaborage/code/go-bricks-openapi-pr3` only. Never touch `../go-bricks-openapi` or `../go-bricks-openapi-pr2`. Scratch projects and binaries go under `/private/tmp/claude-501/-Users-gaborage-Projects-gaborage-code-go-bricks-openapi/7fea5cb8-6b07-4be5-b097-b746a0737ef9/scratchpad/pr3/` only. Never push, open PRs or touch GitHub state.
- **Goldens:** regenerate only with `go test ./internal/spectest -update`, never `make update`. `-update` is package-scoped. The golden test is `TestGoldenFixtures`, and a `-run 'TestFixtures/…'` pattern matches nothing. After `-update`, `git status --porcelain internal/spectest/testdata` may list only:
  - the two new fixture directories;
  - `expected.yaml` of `named_types`, `qualified_named_types` and `slice_result`, each with exactly the diff in **Goldens** below;
  - the comment-only edits to those three fixtures' `.go` files.

  Any other moved `expected.yaml` is a bug: fix the code, don't accept the diff.
- **Cognitive complexity ≤15 per function** (SonarCloud `go:S3776`, server-side only; `make lint` checks only cyclomatic). Check with `go run github.com/uudashr/gocognit/cmd/gocognit@latest -over 15 <files>`. The budget table is binding. Baseline: the only production function over 15 is the untouched `yamlNodeToJSONValue` (`internal/commands/generate.go`, 16), plus `TestMarshalerGuardPositions` (16, test). Do not touch either.
- **Coverage ≥80% on new code** in `cmd/` and `internal/`. A package's coverage comes **only from its own tests** (no `-coverpkg`). Spectest and commands tests cover no analyzer or generator lines, so every new analyzer and generator branch needs an in-package test. `internal/models` stays test-free, so never add a `_test.go` there. Its new accessor is trivial.
- **`goconst`** counts `_test.go` literals (min-len 4, min-occurrences 3). Put new production strings in `const`s. The payload warning texts are a `const` block in `payload.go`. A **new test literal equal to an existing raw production literal of the same package also counts** (CLAUDE.md, the PR #58 lesson): `"Response data"` is already two raw literals in `openapi.go`, so one more in a test assignment or table row trips the pinned v2.12.2 on `openapi.go:864` and fails the required `Lint` check. A bare call argument (`assert.Equal(t, "Response data", …)`) is not counted under goconst's default `ignore-calls`, but do not lean on that: use the const Task 4 Step 2a introduces.
- **`exhaustive`** (`default-signifies-exhaustive: true`): every new `switch` on a `ShapeKind` or `fallbackKind` has a `default:` arm.
- **gocritic** (all tags): reproduced in the prototype:
  - Adding a field to `handlerAnalysis` makes `found()`'s value receiver `hugeParam` (80 bytes). Make it a pointer receiver.
  - A two-result `extractResponseType` trips `unnamedResult`. Name the results.
- **No `t.Parallel()`.** Go 1.25 floor.
- **Settled invariants (CLAUDE.md), untouched:**
  - `lookupStructTag` and `unquoteLiteral` stay the only tag and literal readers. This PR reads neither.
  - The Constraint set stays in `internal/generator/constraints.go`. A payload has no validate tag, so `applyValidationConstraints` is not called for payloads.
  - Example coercion stays only in `fieldInfoToProperty`. Payloads have no example, and `buildFieldProperty` is untouched.
  - `referencedSchemaNames` still scans no non-JOSE request type. Only the **response** Resolution walk is added. Requests never carry a Shape or Resolution.
  - The `schema == nil` check in `generateSchemasFromTypes` is untouched. `json_excluded_request` and `empty_request` must not move.
  - The uint `minimum: 0` pre-stamp in `setBasicTypeAndFormat` stays frozen. Payload typing reaches it through `setTypeAndFormat` exactly as fields do (`Result[Flag]` → `minimum: 0`).
  - `NOSONAR` and `//nolint` are not interchangeable. This PR adds neither.
- **Out of scope (do not implement):**
  - #111: text-marshaler strings, struct Marshaler types, embedded promotion. Every Marshaler payload is `{}` with a warning.
  - #117: overrides.
  - #114: generic instantiations (`Result[Page[User]]` stays nil).
  - #118: anonymous structs (`Result[struct{ A int }]` and `[]struct{…}` stay untyped).
  - #102: request types.
  - The `[]uuid.UUID`-as-project-struct residual. A **registered** struct slice payload keeps today's path (open decision 4).
  - A JOSE struct reached only through a composite payload (`Result[map[string]Sealed]`) is documented as plain JSON with a `$ref`. Today it is an untyped object. Not modelled further.
- **Gates before the commit:**
  - `make check` with the pinned golangci-lint (`make dev-deps`).
  - `go test ./internal/spectest`.
  - gocognit `-over 15` on every touched file.
  - The coverage spot-check (Task 10).
  - The mutation checks (Task 10).
- **One commit** (squash). Its body stands alone (template in Task 11). Personal repo: no `Refs:` ticket line. The only closing keyword anywhere is the `Closes #110` trailer. Every other issue is cited as `Refs #N`.

---

## Behavior rows: expected emitted `data` schema

Flow style for readability. Goldens are block style, and key order follows `OpenAPIProperty`. "(A)" is `request/response type X is a named non-struct type …`. "(B)" is `response type X resolves to no schema component …`. "(R)" is only the aggregate `route(s) have no resolved request/response type`. "Typed" is doctor's verdict and `generate`'s route count. Every row behaves the same under `ResultWithMeta[T]`, `WithRawResponse()` (the whole body instead of `data`) and a bare `T` return. The fixtures pin representatives of each.

| # | Payload `T` | Today (`9d2b78a`) | After | Warning after | Typed after |
|---|---|---|---|---|---|
| P1 | `Cents` (`int64`), `*Cents`, `Chain` (`type Chain Cents`), `Alias = int64`, `Dur` (`time.Duration`), `PC` (`*int64`), `*PC` | `{type: object, description: Response data}`, (A) | `{type: integer, format: int64}`, no `nullable` | none | yes |
| P2 | `Flag` (`byte`) / `Status` (`string`) / `Ratio` (`float64`) / `Enabled` (`bool`) | object, (A) | `{integer, int32, minimum: 0}` / `{type: string}` / `{number, double}` / `{type: boolean}` | none | yes |
| P3 | `Width` (`int32` under `//go:build linux`, `int64` otherwise) | object, (A) | `{type: integer}` (kind-only, #92) | none | yes |
| P4 | `[]Cents`, `[]*Cents`, `[]Enabled`, `[2]Cents` | array of object, (A) | array of the element's schema | none | yes |
| P5 | `Blob` (`[]byte`), `[]Flag`, `Raw` (`type Raw json.RawMessage`), `*[]byte` | object / array of object, (A) or (R) | `{type: string, format: byte}` | none | yes |
| P6 | `Tags` (`[]string`), `*Tags`, `b.Tags` | object, (A) / (B) | `{type: array, items: {type: string}}` | none | yes |
| P7 | `[]Tags`, `[][]string` | array of object / object | array of arrays of string | none | yes |
| P8 | `UserList` (`[]User`), `*UserList`, also under `ResultWithMeta`, `WithRawResponse()`, bare | object, (A); `User` not emitted | `{type: array, items: {$ref: User}}`; `User` emitted and referenced | none | yes |
| P9 | `Stamp = time.Time` / `RawA = json.RawMessage` / `AnyD any` / `Pair [2]int` | object, (A) | `{string, date-time}` / `{}` / `{}` / `{type: array, items: {integer, int64}}` | none | yes |
| P10 | `b.Cents` | object, (B) | `{integer, int64}` | none | yes |
| P11 | `map[string]int64`, `map[string]Cents`, `map[string][]Cents`, `[]map[string]int64`, `[][]Cents`, `*[]string`, `*interface{}` (`{}`) (also `ResultWithMeta`, raw, bare) | object, (R) | the field's schema (`{type: object, additionalProperties: {integer, int64}}`, …); **no** `description` | none | yes |
| P12 | `map[string]Address` | object, (R); `Address` not emitted | `{type: object, additionalProperties: {$ref: Address}}`; `Address` emitted | none | yes |
| P14 | `[]*byte`, `[]*Flag`, `[4]Flag`, `*[4]byte` | `{string, byte}` silent (**wrong**: `encoding/json` writes numbers) / array of object (A) / array of object (A) / object (R) | `{type: array, items: {integer, int32, minimum: 0}}` (never base64) | none | yes |
| F1 | `Tier` (`MarshalText`), `*Tier`, `PTier` (pointer `MarshalText`), `b.Tier` | object, (A) / (B) | `{}` | `response type Tier: Tier has its own MarshalText method, which encoding/json uses instead of its underlying type — emitting an untyped schema ({})` (`b.Tier` names `b.Tier`) | no |
| F2 | `[]Tier`, `[]FlagV` (byte, own `MarshalJSON`), `map[string]Tier` | array of object (A) / object (R) | `{type: array, items: {}}` / `{type: object, additionalProperties: {}}` | the Marshaler warning, first `%s` the payload as written (`[]FlagV`, `map[string]Tier`) | no |
| F3 | `Tree` (`map[string]Tree`) | object, (A) | `{type: object, additionalProperties: {}}` | `response type Tree: Tree contains itself — the recursion is cut to an untyped schema ({})` | no |
| F4 | `Addr` (`uintptr`), `uintptr` | object (A) / object **silent, typed** | `{type: object, description: Response data}` | `response type Addr holds a uintptr, a machine address with no meaningful API contract — emitting an untyped schema (use a sized integer type)` | no |
| F5 | `map[string]uintptr` / `[]uintptr` | object (R) / array of object **silent, typed** | `{type: object, additionalProperties: {type: object}}` / `{type: array, items: {type: object}}` | the uintptr warning | no |
| F6 | `decimal.Decimal`, `t.Time` (aliased import), `*Missing`, `[]decimal.Decimal` | object (array of object), (B) | unchanged (`[]decimal.Decimal` by the resolved path; see Notes) | unchanged (B) | no |
| F7 | `map[string]decimal.Decimal` | object, (R) only | `{type: object, additionalProperties: {type: object}}` | (B) naming `decimal.Decimal` | no |
| F8 | `StampD` (`type StampD time.Time`) | object, (A) | unchanged | unchanged (A): the root resolves to nothing | no |
| F9 | `[]StampD` | array of object, (A) | unchanged schema | `response type []StampD: StampD is a defined type over time.Time, which resolves to no schema (…) — emitting an untyped object` | no |
| F10 | `complex128`, `Cx` (`type Cx complex128`) | object **silent, typed** / object (A) | `{type: object, description: Response data}` | `response type complex128 holds the builtin complex128, a Go type with no JSON schema — emitting an untyped object` | no |
| U1 | `Fn` (`type Fn func()`), `[]Fn`, `[]Ch` (`type Ch chan int`) | object (A) / array of object (A) | unchanged | unchanged (A) | no |
| U2 | `struct{ A int }`, `[]struct{…}`, `map[string]func()`, `map[string]chan int`, `Page[User]` (a **literal** unmodelled leaf: `ShapeUnknown` at extraction) | nil response, (R) | unchanged | unchanged (R) | no |
| U4 | nameless composite over a **named** func/chan type: `map[string]Fn`, `[][]Fn`, `*[]Fn`, `map[string]Ch` (`type Ch chan int`) | `{type: object, description: Response data}`, (R) only; nil response | the field's schema: `{type: object, additionalProperties: {type: object}}` / `{type: array, items: {type: array, items: {type: object}}}` / `{type: array, items: {type: object}}` / `{type: object, additionalProperties: {type: object}}`; **no** `description`; Resolution nil, typed from the Shape | (R) only, no per-type warning (field parity: a `map[string]Fn` field is silent and emits the same schema) | no |
| U3 | written `**T` root: `**int64`, `**Address`, `**[]string` | nil response, (R) | unchanged (#120's open payload half) | unchanged (R) | no |
| C1 | `Item`, `*Item`, `[]Item`, `[]*Item`, `[2]*Address`, `time.Time`, `[]*time.Time`, `int64`, `[]string`, `any`, `NoContentResult` | — | unchanged byte for byte | unchanged | unchanged |
| Q1 | request `req Cents`, `req []Address`, … | #127 warning, no body | unchanged | unchanged | unchanged |

Notes:
- F6's `[]decimal.Decimal` reaches F6's output by the resolved path, not the screens. Its root is a slice, and `resolveQualified` returns the leaf as `named decimal.Decimal` (noting `fallbackUnresolvable`), not `ShapeUnknown`. So `payloadResolves` is true: Resolution `slice(named decimal.Decimal)` is stamped, Name is cleared, and `warnPayload` raises the (B) text naming `decimal.Decimal`. The schema (`array of object`) and the warning are byte-identical to `9d2b78a`. Only the root F6 payloads (`decimal.Decimal`, `t.Time`, `*Missing`) keep a nil Resolution and fall to `dropUnresolvablePayloadName`.
- `description: Response data` stays on a **bare** `{type: object}` only (F4, F6, F8, F10). A typed map (P11, F3, F5, F7) gets none: `successEnvelopeSchema`'s test gains `AdditionalProperties == nil`.
- The order of precedence for a payload's one warning is: uintptr first (as for fields), then the first fallback the resolution noted, then a ref leaf that could not register. A payload never raises two warnings.
- `Result[[]server.IAPIError]` keeps resolving to nil (framework element). A framework type inside a composite (`map[string]server.IAPIError`) now resolves and warns (B) naming `server.IAPIError`. This is an accepted degenerate case.
- A U2 or U3 payload keeps a nil response, so a request-less handler returning one is still "not found" and still loses its success and error statuses (documented 200, no inferred error responses). Every P/F/U4 payload that was nil before (P11, P12, P7's `[][]string`, F2/F5/F7's maps, P14's `*[4]byte`, P5's `*[]byte`, every U4 row) is now found and keeps them (Impact 12).
- U4's path: extraction carries the payload because its leaf is `ShapeNamed` (`Fn`), not `ShapeUnknown` (U2's literal `func()`). The resolver turns the leaf into `ShapeUnknown`, so `payloadResolves` is false and nothing is stamped; `screenUnresolvedPayload` short-circuits on `Name == ""`, so no per-type warning; `responsePayloadSchema` types the Shape (`Name == "" && Shape != nil`), whose named leaf emits `{type: object}`, which is byte-identical to the field's schema (verified on `scratchpad/pr3/gates_par`, `base_bin` vs `proto_bin`: the `Fields` component's `f1`/`f2`/`f8` match the U4 cells and raise no field warning). `IsTypedPayload` sees the non-well-known named leaf and returns false, so the aggregate (R) still lists a request-less U4 route and `--strict` still fails on it. `[]Fn`/`[]Ch` are U1, not U4: the slice arm's `ShapeNamed` element names the TypeInfo `Fn`, and the screens raise (A), an existing asymmetry this PR does not touch.
- `Result[error]` stays nil (`isFrameworkType("error")`). `Result[[]error]` keeps its nameless Shape and now warns as an untyped builtin (F10's text). Today it is a silent typed `{type: array, items: {type: object}}`.

---

## Data model (`internal/models/models.go`)

Anchor: `grep -n 'Shape \*TypeShape' internal/models/models.go` (126, inside `TypeInfo` 89–127).

```go
	Shape *TypeShape
	// Resolution is the payload's Shape with every named non-struct type of the
	// module substituted by what it stands for and every struct leaf a ShapeRef
	// (CONTEXT.md, "Resolution"). Stamped by the analyzer on a payload that does
	// not register as a struct; nil for a request, a registered struct payload,
	// the NoContentResult marker, a payload whose root resolves to nothing, and
	// a nameless composite whose resolution has an unmodelled leaf
	// (map[string]Fn), which is typed from its Shape like the field of its type.
	// Read it through ResolvedShape.
	Resolution *TypeShape
}

// ResolvedShape returns the payload's Resolution, else its Shape (nil when it
// has neither).
func (t *TypeInfo) ResolvedShape() *TypeShape {
	if t.Resolution != nil {
		return t.Resolution
	}
	return t.Shape
}
```

- It returns a pointer, unlike `FieldInfo.ResolvedShape`: a payload's Shape is itself optional.
- Rewrite the `Shape` doc's third paragraph ("For any other non-slice payload … falls back to an untyped object"). Say instead that a payload that does not register is typed from its Resolution exactly as a field of its type. Say that only a root that resolves to nothing (an unresolvable or undeclared name, a defined type over a well-known struct, disagreeing build-tagged declarations, or an unmodelled leaf such as `func()`) keeps the name-cleared, warned, untyped-object fallback. Add that a nameless composite over a named func or chan type (`map[string]Fn`) gets no Resolution and is typed from its Shape (a container of untyped objects) with no per-type warning, exactly as a field of its type. Keep the sentence that `Shape` is nil for requests and registered non-slice structs.
- Also rewrite the `TypeShape` doc line "The registry outcome lives in FieldInfo.Resolution". It should say `FieldInfo.Resolution and TypeInfo.Resolution`.

---

## Analyzer design, with every helper's degenerate input checked

### Helpers reused as is (checked at `9d2b78a`)

| Helper (recipe) | Degenerate behavior relied on |
|---|---|
| `resolveShape` (`grep -n 'func (a \*ProjectAnalyzer) resolveShape' internal/analyzer/resolution.go`, 236) | Returns a fresh tree. It records ref sites in `c.st.refSites` and notes the first fallback in `c.st.first`, but **neither registers nor warns**. A call whose result is discarded (open decision 1's root-resolves-to-nothing case) therefore has no side effect beyond `parsePackageDir`'s cache. A nil `Elem` stays nil. `ShapeUnknown` and zero-Kind leaves pass through with no note. |
| `newResolveCtx(file, path, param)` (152) | `home = filepath.Dir(path)`. **The path must be the file's own path**: `samePackageFiles` skips `p == path`. That is Task 1's bug (both seams). Pass `param=false` for payloads: a body is never a parameter, so the Marshaler guard applies. |
| `walkResolvedLeaves` (650) | Never descends into a map's `Key` or a Marshaler leaf's `Elem`. A nil root is a no-op. A container with nil `Elem` yields no leaf. |
| `holdsUintptr` (663) | True for any emitted `uintptr` primitive leaf at any depth. |
| `registerTypeAt` (`grep -n 'func (a \*ProjectAnalyzer) registerTypeAt' internal/analyzer/analyzer.go`, 3238) | Returns the cached TypeInfo on a repeat key, and nil for a non-struct. A depth-0 call never hits the cap. |
| `registerPayloadType` (3121) | On a nameless TypeInfo (composite payloads, builtins), `registerType("", pkg, …)` and then `registerQualifiedType(pkg+".")` both return nil. Nameless builtin payloads already take this path on `9d2b78a`. |
| `PayloadBaseShape` (2423) | Sheds one sequence level, then one pointer level. A `ShapePointer` with nil `Elem` is returned as is. **Kept exported**: `TestUnresolvablePayloadFallsBackWithWarning` calls it and must pass unchanged. Its doc ("Exported for doctor's isTypedPayload") is rewritten (Task 9), since doctor now calls `IsTypedPayload`. **Superseded in review:** unexported as `payloadBaseShape`, since every remaining caller, tests included, is in package `analyzer`. |
| `typeInfoFromExpr` (2345) | Handles `Ident`, `*Ident`, `*q.T`, `q.T` and generics only. It is nil for a framework type (`HandlerContext`, `IAPIError`, `error`, qualified or not), for `ArrayType`/`MapType`/`StructType`/`FuncType`, and for `**T`/`*[]T`. |
| `typeShape` (`grep -n 'func (a \*ProjectAnalyzer) typeShape' internal/analyzer/analyzer.go`) | `StructType`, `FuncType`, `ChanType`, `IndexExpr` → `ShapeUnknown`. `error`/`complex128` → `ShapePrimitive`. A `*ast.Ident` builtin is primitive. |

### Task 1's fix: every `(AST, path)` pair names one file (two seams)

`grep -n 'for _, file := range files {' internal/analyzer/analyzer.go` hits both loops (686 and 3075; the 685 constants loop also matches and stays as is). Both iterate `parsePackage`'s `map[path]*ast.File` with `_` and pass the **Module-struct file's** `filePath`.

- **Seam A, `extractRoutesFromPackage`** (`grep -n 'a.collectRoutesFromFile(file, filePath' internal/analyzer/analyzer.go`, 690): change the route loop (686) to `for path, file := range files` and pass `path` to `collectRoutesFromFile`. `walkSetup.filePath` then always matches `walkSetup.astFile`, which fixes `extractHandlerSignature`'s first branch and every other `walkSetup` consumer (`extractHandlerInfo`, `resolveHandler`, `resolveDelegateContext`, the self-helper `delegateContext`, `findPackageFuncDecl`, the walk-stack key). The parse-failure fallback above the loop already passes a matching pair.
- **Seam B, the sibling loop** (`grep -n 'a.populateTypeFields(h.response, file, filePath)' internal/analyzer/analyzer.go`, 3078): change 3075 to `for path, file := range files` and pass `path` to both `populateRequestType` and `populateTypeFields`.

`parsePackage` keys are absolute paths of files of that package, so each file's own path is exact. `path` shadows no import (`analyzer.go` imports `path/filepath` only; `for path, astFile := range all` at 2067 is precedent). Child walkers already take their pair from one source (`recurseInto`'s `target`, `findPackageFuncDecl`'s `declFile, declPath`), and no other caller of the populate functions mixes files and paths.

### Extraction (`analyzer.go`)

1. **`payloadTypeInfo`** (2382). Dispatch on the expression:

   ```go
   func (a *ProjectAnalyzer) payloadTypeInfo(expr ast.Expr, packageName string, serverAliases map[string]struct{}) *models.TypeInfo {
   	switch e := expr.(type) {
   	case *ast.ArrayType:
   		return a.slicePayloadTypeInfo(e, packageName, serverAliases)
   	case *ast.MapType:
   		return a.compositePayloadTypeInfo(e, packageName)
   	case *ast.StarExpr:
   		if isCompositePointee(e.X) { // *[]T, *[N]T, *map[K]V, *interface{}
   			return a.compositePayloadTypeInfo(e, packageName)
   		}
   	}
   	return a.scalarPayloadTypeInfo(expr, packageName, serverAliases)
   }
   ```

   Its doc gains this sentence: a map, a nested slice and a pointer to a slice, array, map or `interface{}` are carried nameless with their Shape. The rule stays on the payload path only. A slice or map handler **parameter** keeps resolving to nil, through `typeInfoFromExpr`, which is unchanged. A written `**T` root falls through to `scalarPayloadTypeInfo`, where `typeInfoFromExpr` returns nil exactly as on `9d2b78a` (U3): whether `Result[**T]` should resolve is #120's open question, not #110's.
2. **New `isCompositePointee(x ast.Expr) bool`**: true for `*ast.ArrayType`, `*ast.MapType` and `*ast.InterfaceType`; `default:` false. So `*Named`/`*q.T` keep the scalar path, `**T` stays nil, and `*struct{…}`/`*func()` take exactly the `9d2b78a` scalar path, unchanged whatever it returns. The prototype used `!isNamedTypeExpr(e.X)` instead, which also let `**T` through; that is the only behavioral difference, confined to U3.
3. **New `compositePayloadTypeInfo(expr ast.Expr, packageName string) *models.TypeInfo`**: `shape := a.typeShape(expr)`. It returns nil when `hasUnknownLeaf(&shape)`, which keeps U2 (#118, #114). Otherwise it returns `&models.TypeInfo{Package: packageName, Shape: &shape}` with no Name.
4. **New `hasUnknownLeaf(s *models.TypeShape) bool`**: uses `walkResolvedLeaves` and is true on a `ShapeUnknown` or zero-Kind leaf. It is shared with `payloadResolves`.
5. **`slicePayloadTypeInfo`** (2450): delete the `star.X` shed (2453–2456). Decode `elemShape := a.typeShape(arr.Elt)`, keeping the element pointer (P14). Switch on `PayloadBaseShape(shape).Kind`, which sheds the sequence and one pointer:
   - `ShapePrimitive`: nameless TypeInfo with the Shape. This is today's arm, now reached for `[]*byte`.
   - `ShapeNamed`: `elem := a.typeInfoFromExpr(arr.Elt, …)`. `handleStarExprType` names `*Item` as `Item`, so `[]*Item` still registers. It is nil for a framework element, which keeps `[]server.IAPIError` nil.
   - `default`: `return a.compositePayloadTypeInfo(arr, packageName)`. This covers `[][]T`, `[]map`, `[]**T` and `[]*[]T`.

   Rewrite the doc. Drop "a pointer element is shed first" and "Anything else … returns nil". Say the element pointer is kept, because `encoding/json` writes `[]*byte` as numbers, and that other element shapes are carried nameless.
6. **`extractResponseType`** (2612): return `(ti *models.TypeInfo, written string)`. `written` is the payload type as written: `types.ExprString(index)` of the wrapper's type argument (`IndexExpr.Index`, `IndexListExpr.Indices[0]`, guarded by `len > 0` like `typeInfoFromExpr`), `""` for `NoContentResult`, and `types.ExprString(first)` for a bare return.
7. **`handlerAnalysis`** (1533): add `responseType string // the payload type as written, for payload warnings`. Set it in `findHandlerInFile` (2631). Change `func (h handlerAnalysis) found()` (1545) to a pointer receiver. Every call site holds an addressable variable, and `go vet` flags none. Its body (`request != nil || requestType != "" || response != nil`) is unchanged, but its verdict changes for a request-less handler whose payload is newly carried (P11, P12, P7's `[][]string`, `*[]byte`, `*[4]byte`, F2/F5/F7 maps, `[]**T`, `[]*[]T`): it was false, so `extractHandlerInfo` (`grep -n 'return handlerName, handlerAnalysis{}' internal/analyzer/analyzer.go`) dropped `successStatus` and `errorStatuses`; it is now true and both are kept. This is wanted (Impact 12) and pinned by `TestRequestlessCompositePayloadKeepsStatuses` (Task 3).
8. **`populateTypeFields`** (3149): new signature `populateTypeFields(typeInfo *models.TypeInfo, written string, astFile *ast.File, filePath string)`. Its two callers pass `h.responseType`. Restructure it so the function stays at or below its budget:

   ```go
   func (a *ProjectAnalyzer) populateTypeFields(typeInfo *models.TypeInfo, written string, astFile *ast.File, filePath string) {
   	if typeInfo == nil {
   		return
   	}
   	if registered := a.registerPayloadType(typeInfo, astFile, filePath); registered != nil {
   		adoptRegisteredType(typeInfo, registered)
   		return
   	}
   	if a.resolvePayload(typeInfo, written, astFile, filePath) {
   		return
   	}
   	a.screenUnresolvedPayload(typeInfo, astFile, filePath) // today's (A) block + dropUnresolvablePayloadName, moved verbatim
   }
   ```

   `screenUnresolvedPayload` is today's `registered == nil` body (3152–3175), moved unchanged with its long comment. Update the `populateTypeFields` doc to describe the four outcomes: registered struct; resolved (stamped, maybe warned); root resolves to nothing (screened, warned); and a nameless payload whose resolution has an unmodelled leaf (U4), which `resolvePayload` leaves unstamped and the screens pass over silently on `Name == ""`, so it is typed from its Shape.
9. **`dropUnresolvablePayloadName`** (3207): replace the inline literal with the new `unresolvablePayloadWarning` const (the text is identical). Rewrite its doc. It now sees only a payload whose root resolves to nothing. A composite with an unresolvable leaf is warned by `warnPayload` with the same text.

### Payload resolution (`internal/analyzer/payload.go`, new)

```go
// Payload fallback warnings. Each payload raises at most one. The first
// argument is the payload type as written; the unresolvable one keeps its
// pre-#110 text, naming the type that resolved to nothing.
const (
	unresolvablePayloadWarning = "response type %s resolves to no schema component — emitting an untyped schema " +
		"(declare it as a struct in the project for a typed spec)"
	marshalerPayloadWarning = "response type %s: %s has its own %s method, which encoding/json uses instead of its " +
		"underlying type — emitting an untyped schema ({})"
	recursivePayloadWarning = "response type %s: %s contains itself — the recursion is cut to an untyped schema ({})"
	depthCapPayloadWarning  = "response type %s: its named types nest more than %d deep, so %s is cut to an untyped " +
		"schema ({}) as if it were recursive"
	definedOverQualifiedPayloadWarning = "response type %s: %s is a defined type over %s, which resolves to no schema " +
		"(a defined type drops its target's methods, and a type from outside the module is not resolved) — emitting an untyped object"
	declsDisagreePayloadWarning = "response type %s: %s's build-tagged declarations disagree on shape (no common " +
		"container or scalar kind; build constraints are not evaluated) — emitting an untyped object"
	untypedBuiltinPayloadWarning = "response type %s holds the builtin %s, a Go type with no JSON schema " +
		"— emitting an untyped object"
	uintptrPayloadWarning = "response type %s holds a uintptr, a machine address with no meaningful API contract " +
		"— emitting an untyped schema (use a sized integer type)"
	unregisteredRefPayloadWarning = "response type %s: struct %s could not be registered as a component " +
		"— emitting an untyped object"
)
```

Functions, with their budget:

| Function | Contract | Degenerate inputs |
|---|---|---|
| `(a) resolvePayload(ti, written, astFile, filePath) bool` | If `ti.Shape == nil`, return false. Otherwise `c := newResolveCtx(astFile, filePath, false)` and `r := a.resolveShape(*ti.Shape, c)`. If `!payloadResolves(&r)`, return false, changing nothing. Otherwise stamp `ti.Resolution = &r`. Unless `isWellKnownLeaf(PayloadBaseShape(*ti.Shape))`, clear `ti.Name` and `ti.Fields`: a named non-struct never names a component, while `time.Time`, `[]time.Weekday` and `*json.Number` keep the name `TestUnresolvablePayloadFallsBackWithWarning` pins. Then `warned := a.warnPayload(written, &r, c.st.first)` and `a.registerPayloadRefs(&r, c.st.refSites, pkgFile{astFile, filePath}, written, warned)`. Return true. | A nameless builtin (`int64`) resolves to itself, and `Name` is already "". The Shape is never rewritten. |
| `payloadResolves(r *models.TypeShape) bool` | Shed every pointer at the root. False when the root is a `ShapeNamed` not in `models.WellKnownTypeNames`, or when `hasUnknownLeaf(r)`. Otherwise true. | A non-well-known named leaf **below a container root** resolves (true), and `warnPayload` warns from the noted fallback: `[]decimal.Decimal` → true, (B); `[]StampD` → true, defined-over-qualified. Only a named root falls to the screens. Never widen the rejection to every non-well-known named leaf: a nameless composite (`map[string]decimal.Decimal`, `[]StampD`, `map[string]Missing`) would reach `screenUnresolvedPayload`, where `Name == ""` short-circuits both screens, leaving the route silently untyped with no per-type warning. That caveat is about **named unresolvable** leaves, whose field does warn. An Unknown leaf below a nameless root (U4, `map[string]Fn`) is the one accepted no-per-type-warning case, because the field of that type is silent too; the aggregate (R) still fires through `IsTypedPayload`. `*Missing` → false, which gives (B). `StampD` → false, which gives (A). `Fn` and `[]Fn` → `ShapeUnknown` leaf → false, which gives (A). `ptr(ptr(prim int64))` (`*PC`'s resolved form) → true. A root `ShapeRef` → true. A pointer with nil `Elem` is the root itself → true. `setTypeAndFormat` emits `object` for it, and only a hand-built shape can produce it. |
| `isWellKnownLeaf(s models.TypeShape) bool` | `s.Kind == ShapeNamed && WellKnownTypeNames[s.Name]`. | |
| `(a) warnPayload(written string, r *models.TypeShape, fb fieldFallback) bool` | `holdsUintptr(r)` → uintptr warning. Otherwise `switch fb.kind`: `fallbackNone` returns false. Each other kind raises its const, with `default:` taking the untyped builtin (`fb.detail`). Unresolvable passes `fb.typeName` only, to keep the (B) text. Returns true when it warned. | The uintptr check runs before the fallback check, exactly as `warnResolvedField` does for fields. |
| `(a) registerPayloadRefs(r, sites map[string]pkgFile, def pkgFile, written string, warned bool)` | For each `ShapeRef` leaf (`walkResolvedLeaves`), site = `sites[leaf.Name]` else `def`. If `!a.registerLeafAt(leaf, site, 0) && !warned`, raise `unregisteredRefPayloadWarning` once and set `warned`. | Capture `leaf.Name` **before** `registerLeafAt`, because it demotes the leaf in place. |
| `IsTypedPayload(ti *models.TypeInfo) bool` (exported, for doctor and generate) | nil → false. `ti.Resolution == nil && ti.Name != ""` → true: a registered struct, or a hand-built well-known named payload. Otherwise `s := ti.ResolvedShape()`: nil → false, else true iff no leaf `isFallbackLeaf`. | Hand-built `{Shape: slice(named Status)}` → untyped. Doctor's "named-scalar slice element is untyped" case keeps passing. Hand-built nameless `primitive uintptr` is now untyped (it was typed). |
| `isFallbackLeaf(leaf *models.TypeShape) bool` | `ShapeNamed` → not well-known. `ShapePrimitive` → `uintptr` or in `UntypedBuiltinNames`. `ShapeRef`, `ShapeKindOnly` → false. `default:` (marshaler, recursive, unknown, zero) → true. | A demoted ref (`ShapeNamed`) counts as a fallback. |

`resolution.go`: split `registerRefLeaf` (`grep -n 'func (a \*ProjectAnalyzer) registerRefLeaf' internal/analyzer/resolution.go`, 720).

```go
// registerLeafAt registers one ShapeRef leaf in the file its name was
// written in and stamps its final component name, reporting success. A leaf
// that cannot register is demoted to a named leaf (it emits object); the
// caller warns.
func (a *ProjectAnalyzer) registerLeafAt(leaf *models.TypeShape, site pkgFile, depth int) bool {
	if reg := a.registerTypeAt(leaf.Name, site.file.Name.Name, site.file, site.path, depth); reg != nil {
		leaf.Name = reg.Name
		return true
	}
	leaf.Kind = models.ShapeNamed
	return false
}
```

`registerRefLeaf`'s head then becomes `if a.registerLeafAt(leaf, site, depth) || depth > maxTypeRegistrationDepth { return }`. The rest is unchanged, so field behavior is byte-identical.

### Doctor (`internal/commands/doctor.go`)

`isTypedPayload` (569) becomes `return analyzer.IsTypedPayload(ti)`. Rewrite its doc: a payload is typed exactly when its schema has no fallback leaf, so `Result[AnyD]` (`{}`) is typed and `Result[Tier]` (`{}`) is not. Requests reach it too and are unchanged, since a request has no Shape: a request with a Name is typed, one without is not. Drop the `models` import if it becomes unused. `classifyRoute` (534) also drives `generate`'s route count (`internal/commands/generate.go`, `grep -n 'have no resolved' internal/commands/generate.go`), so both move together with no further edit.

---

## Generator design (`internal/generator/openapi.go`)

1. **`setTypeAndFormat` becomes a plain function** (`grep -n 'func (g \*OpenAPIGenerator) setTypeAndFormat' internal/generator/openapi.go`, 1706). It never reads `g`. Rewrite its recursive calls and the callers in `buildFieldProperty` as `setTypeAndFormat(...)`. Test edits are mechanical:
   - Replace `gen.setTypeAndFormat(` with `setTypeAndFormat(` (about 18 sites in `openapi_test.go` and `resolution_test.go`).
   - Delete each `gen := New(...)` / `var gen OpenAPIGenerator` this leaves unused (8 in the prototype).
   - Remove the blank first line `whitespace` then flags in `TestSetTypeAndFormat`, `TestSetTypeAndFormatMaps` and `TestSetTypeAndFormatPredeclaredAliases`.

   Rejected: `(&OpenAPIGenerator{}).setTypeAndFormat` from a free function (open decision 8).
2. **`responsePayloadSchema`** (891):

   ```go
   // responsePayloadSchema returns the bare schema of a response payload. A
   // payload the analyzer resolved (Resolution) is typed exactly as a struct
   // field of its type is, every pointer level shed and never nullable. A
   // registered project struct is a $ref to its component, an array of them for
   // a registered struct slice ([]Item). Anything else, meaning no payload or
   // the warned fallback whose name the analyzer cleared, is typed from its Shape,
   // which for a name that resolved to nothing is the untyped object.
   func responsePayloadSchema(response *models.TypeInfo) *OpenAPIProperty {
   	switch {
   	case response == nil:
   		return &OpenAPIProperty{Type: typeObject}
   	case response.Resolution != nil:
   		return payloadShapeSchema(*response.Resolution)
   	case response.Name == "" && response.Shape != nil:
   		return payloadShapeSchema(*response.Shape)
   	case response.Name == "":
   		return &OpenAPIProperty{Type: typeObject}
   	case response.Shape != nil && isSequence(*response.Shape):
   		return structSlicePayloadSchema(response)
   	default:
   		return &OpenAPIProperty{Ref: refPath(schemaName(response))}
   	}
   }

   // payloadShapeSchema types a payload shape as setTypeAndFormat types a field.
   func payloadShapeSchema(s models.TypeShape) *OpenAPIProperty {
   	prop := &OpenAPIProperty{}
   	setTypeAndFormat(prop, s)
   	return prop
   }
   ```

   `nullable` is added only by `buildFieldProperty`, which payloads never call, so "never nullable" holds by construction. `setTypeAndFormat` sheds every pointer (`stripPointers`, #128), so `Result[*PC]` (resolved `ptr(ptr(prim int64))`) emits what `Result[PC]` emits. A written `Result[**int64]` never reaches it: extraction leaves it nil (U3, #120).
3. **`structSlicePayloadSchema`** replaces `sliceResponsePayloadSchema` (941). It is reached only for a **registered** struct slice (Name = element component):

   ```go
   // structSlicePayloadSchema documents a slice or array payload of a registered
   // project struct ([]Item, []*Item, [2]Address): Name is the element's
   // component. A project struct named like a well-known type ([]uuid.UUID) is
   // still typed inline as that type, a documented residual.
   func structSlicePayloadSchema(response *models.TypeInfo) *OpenAPIProperty {
   	items := &OpenAPIProperty{}
   	if elem := response.Shape.Elem; elem != nil && isWellKnownElem(*elem) {
   		setTypeAndFormat(items, *elem)
   	} else {
   		items.Ref = refPath(schemaName(response))
   	}
   	return &OpenAPIProperty{Type: typeArray, Items: items}
   }
   ```

   **Delete** `inlinePayloadSchema` (917) and `setElemTypeAndFormat` (984). **Keep** `isWellKnownElem` (973). This is the brief's "delegate or go away". The prototype confirmed it changes no golden and gives identical scratch output.
4. **`successEnvelopeSchema`** (859): the annotation test becomes `data.Ref == "" && data.Type == typeObject && data.AdditionalProperties == nil`. Update the comment: only the bare untyped object is annotated, so a typed map (`Result[Tree]`, `Result[map[string]int64]`) is not.
5. **`referencedSchemaNames`** (1333): inside the route loop, after the `payloadNamesComponent` block, add:

   ```go
   		// A resolved payload names no component itself; the structs its
   		// Resolution reaches are $ref'd from its schema.
   		if r.Response != nil && r.Response.Resolution != nil {
   			addRefNames(out, *r.Response.Resolution)
   		}
   ```

   The request block and the long invariant comment are untouched. Add one sentence to the doc: response Resolutions are walked like field Resolutions, and request types still are not.
6. **`payloadNamesComponent`** (1368) now mirrors `responsePayloadSchema`:

   ```go
   func payloadNamesComponent(ti *models.TypeInfo) bool {
   	switch {
   	case ti.Name == "":
   		return false
   	case ti.JOSE:
   		return true
   	case ti.Resolution != nil:
   		return false
   	case ti.Shape != nil && isSequence(*ti.Shape):
   		return ti.Shape.Elem == nil || !isWellKnownElem(*ti.Shape.Elem)
   	default:
   		return true
   	}
   }
   ```

   Rewrite its doc accordingly. A resolved payload with a kept well-known name (`time.Time`) marks nothing. Before, `inlinePayloadSchema` decided that.
7. `shapeAfterPointer`'s doc ("Payload typing still uses it (until #120's payload half)") becomes "`isWellKnownElem` uses it". It is still used there and by `wellKnownShape` callers.

---

## Goldens

Exactly three existing goldens move (prototype `-update` diff):

```diff
# named_types/expected.yaml, GET /users data (server.Result[UserList])
-                    type: object
-                    description: Response data
+                    type: array
+                    items:
+                      $ref: '#/components/schemas/User'
# components gains User (email, id int64, name), between ErrorResponse and UserResp

# qualified_named_types/expected.yaml, GET /cents data (server.Result[b.Cents])
-                    type: object
-                    description: Response data
+                    type: integer
+                    format: int64

# slice_result/expected.yaml, GET /statuses data items (server.Result[[]Status])
-                      type: object
+                      type: string
```

Fixture comments to rewrite (comment-only, so no golden byte depends on them):
- `named_types/api.go` 32–35. `UserList` now documents as an array of `$ref: User`, exactly as a `[]User` field does.
- `qualified_named_types/module.go` 113. `cents` is now `{integer, int64}`, since payloads resolve like fields (#110).
- `slice_result/api.go`, the `Status` comment (~17) and `listStatuses` (~64). `[]Status` items are strings.

The goldens that must **not** move were checked against the prototype:
- `bare_return`: its payloads are builtin, well-known, `[]string`, `[]Address`, `interface{}` and `Address`.
- `byte_format_example`: `Blob` is a struct there.
- `crosspkg`: `types.Money` is a struct.
- `fixed_array`: `[4]*byte` was already an integer array. The element pointer is now kept, and items are still `{integer, int32, minimum: 0}`.
- `well_known`, `raw_message`, `named_scalar_builtins`, `named_resolution`, `named_fallback`, `nonstruct_request`, `json_excluded_request`, `empty_request`, `collision`, `jose*`.

### New fixture `internal/spectest/testdata/named_payloads/` (`package namedpayloads`)

Rules (CLAUDE.md):
- The `go.mod` is `module github.com/example/named_payloads`, `go 1.25`, `require github.com/gaborage/go-bricks v0.53.0`, with no `go.sum` and no `replace`.
- The code is only AST-parsed, never compiled.
- Run `gofmt -w` on the fixture `.go` files by hand, because Go tooling skips `testdata/`.

Files:
- `types.go`: `Cents`, `Chain`, `Alias`, `Dur`, `Flag`, `Status`, `Ratio`, `Enabled`, `Blob`, `Raw`, `Tags`, `UserList`, `Stamp`, `RawA`, `AnyD`, `PC`, `Pair`, structs `User{ID int64 json:"id"}` and `Address{City string json:"city"}`. Each declaration gets a short doc comment saying what it documents.
- `width_linux.go`: `//go:build linux`, `type Width int32`.
- `width_other.go`: `//go:build !linux`, `type Width int64`.
- `b/b.go`: `package b`, `type Cents int64`, `type Tags []string`.
- `module.go`: one route per resolved row, with handler bodies `var v T; return v, nil`. Routes:
  - `/cents`, `/pcents`, `/chain`, `/alias`, `/dur`, `/cents-meta` (`ResultWithMeta`), `/cents-raw` (`WithRawResponse()`), `/flag`, `/status`, `/ratio`, `/enabled`, `/width`;
  - `/cents-list`, `/pcents-list`, `/enabled-list`, `/blob`, `/flag-list`, `/raw`, `/tags`, `/ptags`, `/tags-list`;
  - `/users`, `/pusers`, `/users-meta`, `/users-raw`, `/stamp`, `/rawa`, `/anyd`, `/pc`, `/pair`, `/b-cents`, `/b-tags`;
  - `/map-int`, `/map-cents`, `/map-address`, `/grid`, `/cents-grid`, `/pstrings`, `/maps`, `/map-cents-list`, `/map-int-meta`, `/map-int-raw`;
  - `/pbytes` (`[]*byte`), `/pflags`, `/flag-array` (`[4]Flag`), `/pbyte-slice` (`*[]byte`), `/pbyte-array` (`*[4]byte`);
  - `/bare-cents`, `/bare-users` (bare returns), `/piface` (`*interface{}`);
  - two status routes, the only handlers whose body is not `var v T; return v, nil`:
    - `POST /map-created`: `create(ctx server.HandlerContext) (server.Result[map[string]int64], server.IAPIError)`, returning `server.Result[map[string]int64]{}, server.NewNotFoundError("x")` on an `if ctx.Echo == nil` branch and `server.Created(map[string]int64{}), nil` otherwise;
    - `GET /bare-map`: `bareMap(ctx server.HandlerContext) (map[string]int64, server.IAPIError)`, returning `nil, server.NewConflictError("x")`.

  No `**T` route: U3 is pinned in `TestPayloadResolutionRows`, not in a golden.

Expected: every `data` cell as in rows P1–P14. `/map-created` documents `"201"` (`Resource created successfully`) and `"404"`, with no `"200"`; `/bare-map` documents `"409"` (the planner's probe, `scratchpad/pr3/cr_go/st`, shows both on the prototype and `"200"` with neither on `9d2b78a`). Components are exactly `Address`, `ErrorResponse`, `RawErrorResponse` and `User`. `generate --strict --validate` gives `Warnings: 0`. `scratchpad/pr3/np_fixture.yaml` is the prototype run of the **pre-review** fixture: it still has `/pptr-int` and `/pptr-address` (now dropped, U3) and lacks `/piface`, `/map-created` and `/bare-map` (added). Check those five routes against the rows and the status sentence above, not against that file.

### New fixture `internal/spectest/testdata/named_payload_fallback/` (`package namedpayloadfallback`)

Same `go.mod` rules (`module github.com/example/named_payload_fallback`). None of its Marshaler types has both `MarshalText` and `UnmarshalText`, so #111 will not move this golden.
- `types.go`:
  - `Tier int` with value `MarshalText` only;
  - `PTier int` with pointer `MarshalText` only;
  - `FlagV byte` with value `MarshalJSON`;
  - `Tree map[string]Tree`, `Addr uintptr`, `StampD time.Time`, `Cx complex128`.
- `b/b.go`: `Tier int` with `MarshalText`.
- `module.go`:
  - imports `t "time"`, `github.com/shopspring/decimal` and the in-module `b`;
  - routes `/tier`, `/ptier-ptr` (`*Tier`), `/ptier`, `/tiers`, `/flagvs`, `/b-tier`, `/tier-map`, `/tree`, `/addr`, `/addr-map`, `/uintptr`, `/uintptrs`, `/decimal`, `/aliased-time`, `/missing` (`*Missing`, undeclared), `/decimal-map`, `/stampd`, `/stampds`, `/cx`, `/complex`.

Expected: rows F1–F10. Components are `ErrorResponse` only. There is exactly one analyzer warning per route (20), and `generate` prints `Warnings: 21`: the 20 analyzer warnings plus the aggregate route count, as the prototype showed. The prototype output is `scratchpad/pr3/npf_fixture.yaml`.

---

## Cognitive-complexity budget (gocognit, measured on `9d2b78a`; "After" = prototype)

| Function (file) | Now | After (cap) |
|---|---|---|
| `populateTypeFields` (analyzer.go) | 8 | ≤5 (restructured; prototype without the split: 10) |
| new `screenUnresolvedPayload` | — | ≤8 (today's block, moved) |
| `extractHandlerSignature` | 8 | 8 (loop variable only) |
| `extractRoutesFromPackage` | 6 | 6 (loop variable only; Task 1 seam A) |
| `findHandlerInFile` | 6 | 6 |
| `extractResponseType` | 4 | ≤5 |
| `payloadTypeInfo` | 1 | ≤4 (prototype 3; `isCompositePointee` keeps the same nesting) |
| `slicePayloadTypeInfo` | 4 | ≤4 (prototype 3) |
| `scalarPayloadTypeInfo` | 3 | 3 (doc only) |
| `dropUnresolvablePayloadName` | 4 | 4 |
| new `compositePayloadTypeInfo` / `isCompositePointee` / `hasUnknownLeaf` | — | 1 / 1 / 1 |
| `registerRefLeaf` (resolution.go) | 4 | ≤4 |
| new `registerLeafAt` | — | 1 |
| new `resolvePayload` / `payloadResolves` / `isWellKnownLeaf` | — | 3 / 4 / 1 |
| new `warnPayload` / `registerPayloadRefs` | — | 2 / 7 |
| new `IsTypedPayload` / `isFallbackLeaf` | — | 5 / 2 |
| `isTypedPayload` (doctor.go) | 3 | 0 |
| `responsePayloadSchema` (openapi.go) | 5 | ≤3 |
| new `payloadShapeSchema` / `structSlicePayloadSchema` | — | 0 / 3 |
| `inlinePayloadSchema` / `sliceResponsePayloadSchema` / `setElemTypeAndFormat` | 3 / 3 / 1 | deleted |
| `successEnvelopeSchema` | 2 | ≤3 |
| `referencedSchemaNames` | 7 | ≤10 |
| `payloadNamesComponent` | 7 | ≤3 |
| `setTypeAndFormat` | 7 | 7 (receiver dropped) |

---

## Task 0: Branch and baseline

- [x] **Step 1:** `cd /Users/gaborage/Projects/gaborage/code/go-bricks-openapi-pr3 && git fetch -q origin && git status`. Confirm the branch `fix/payload-resolution` is at `9d2b78a` (`git log -1 --format=%h`). If it is not, run `git checkout -B fix/payload-resolution fix/qualified-named-types`.
- [x] **Step 2:** Baseline: `make check` (pinned golangci-lint; `make dev-deps` first if `lint` refuses), `go test ./internal/spectest`, and `go run github.com/uudashr/gocognit/cmd/gocognit@latest -over 15 ./internal ./cmd`. Expect only `yamlNodeToJSONValue` (16) and `TestMarshalerGuardPositions` (16).
- [x] **Step 3:** Re-read #110 (`GH_TOKEN=$(gh auth token -u gaborage) gh issue view 110 --repo gaborage/go-bricks-openapi --comments`) and confirm there is still exactly one comment (the 2026-09-29 brief). If an amendment has appeared, stop and report.

## Task 1: Handler and route file context, two seams (TDD)

**Files:** `internal/analyzer/analyzer.go` (`extractRoutesFromPackage`, `extractHandlerSignature`), new tests in `internal/analyzer/payload_test.go`.

- [x] **Step 1 (red), seam B:** `TestHandlerInSiblingFileResolvesInItsOwnFile`. Use `analyzeDirectiveProject` (or a temp dir with `go.mod` and `analyzer.New`) to write a two-file package:
  - `module.go` declares the `Module`, `RegisterRoutes` registering `/resp`, and `type Tags []string`.
  - `handlers.go` declares `type Resp struct { T Tags \`json:"t"\` }` and `func (m *Module) get(ctx server.HandlerContext) (server.Result[Resp], server.IAPIError)`.

  Assert: no warnings, and the registered `Resp`'s field `T` renders (`renderShape`, see Task 3 Step 1) `ResolvedShape()` as `[]string`. On `9d2b78a` this fails with `field T at handlers.go:… has type Tags: Tags resolves to no schema`.
- [x] **Step 1b (red), seam A:** `TestHandlerInRouteFileResolvesModuleFileTypes`. A two-file package where `RegisterRoutes` is **not** in the Module-struct file:
  - `module.go` declares the `Module` struct with `Name`/`Init`/`Shutdown`, `type Tags []string` and `type Cents int64`.
  - `routes.go` declares `RegisterRoutes` (routes `/resp` and `/cents`), `type Resp struct { T Tags \`json:"t"\` }`, `get` returning `server.Result[Resp]` and `cents` returning `server.Result[Cents]`.

  Assert: no warnings except, until Task 3 lands, `/cents`'s pre-#110 (A) payload warning (filter by the `field ` prefix); `Resp`'s field `T` renders `ResolvedShape()` as `[]string`. Task 3 Step 1 extends this test with the payload half: no warnings at all, and `/cents`'s `Response.Resolution` renders `int64` with Name "" (nothing stamps a Resolution before Task 3). On `9d2b78a`, and with only seam B fixed, this fails with `field T at routes.go:… has type Tags: Tags resolves to no schema` (`scratchpad/pr3/review3_split`, `review3_t1_bin`).
- [x] **Step 2 (green):** apply **Task 1's fix** (both seams): in `extractRoutesFromPackage`'s route loop and in `extractHandlerSignature`'s sibling loop, change `for _, file := range files` to `for path, file := range files` and pass `path` (to `collectRoutesFromFile`; to `populateRequestType` and `populateTypeFields`). Leave the constants loop alone.
- [x] **Step 3:** `go test ./internal/... && go test ./internal/spectest`. All green, with zero golden diff (the reviewer's `review3_t1b` run of both seams showed none). Each Step 1 test must go red on its own when only its seam is reverted (mutations 4a/4b, Task 10).

## Task 2: Carrier in `models`

- [x] Add `TypeInfo.Resolution` and `ResolvedShape()`, and update both docs as in **Data model**. `go build ./...`. Nothing reads them yet.

## Task 3: Analyzer extraction and payload resolution (TDD)

**Files:** `internal/analyzer/analyzer.go`, `internal/analyzer/resolution.go`, new `internal/analyzer/payload.go`, new `internal/analyzer/payload_test.go`.

- [x] **Step 1 (red): `TestPayloadResolutionRows`.** One module (`analyzeSingleModule`-style source plus a `b` package through `analyzeDirectiveProject` for the qualified rows). It has one route per row of P1–P14, F1–F10 and U1–U4 (bare and `ResultWithMeta` variants for `Cents`, `UserList` and `map[string]int64`). For each route assert:
  - `route.Response.Resolution` rendered by the **existing** `renderShape` (`internal/analyzer/shape_test.go:13`, same package; never declare a second one, it would not compile). Its notation: `*` + elem for a pointer, `[]` + elem for a slice, `[N]` + elem for any array, `map[K]V`, the bare name for a primitive or named leaf, `$Name` for a ref, `kind:Name` for kind-only, `marshal:Name(elem)` for a Marshaler, `cycle:Name` for recursion, `unknown` otherwise. Examples: `UserList` → `[]$User`; `PC` → `*int64`; `*PC` → `**int64`; `Width` → `kind:` + `kindInteger` (the existing test const); `Tier` → `marshal:Tier(int)`; `[]*byte` → `[]*byte`; `map[string]Address` → `map[string]$Address`; F6's `[]decimal.Decimal` → `[]decimal.Decimal`. Plan prose elsewhere (`slice(ref User)`, `ptr(prim int64)`, …) is descriptive only; every asserted string uses this notation.
  - **Nil is asserted with `assert.Nil`, never rendered**: `renderShapePtr(nil)` returns `"unknown"`, identical to a `ShapeUnknown` leaf. A nil Resolution is expected only for the **root** F6 rows (`decimal.Decimal`, `t.Time`, `*Missing`), F8, U1 and U4; a nil `Response` for U2 and U3 (`**int64`, `**Address`). F6's `[]decimal.Decimal` expects a rendered `[]decimal.Decimal`, Name "" and exactly the (B) warning naming `decimal.Decimal` (Notes under **Behavior rows**).
  - U4 (`map[string]Fn`, `map[string]Ch`, `*[]Fn`): `Response` non-nil, Name "", Resolution nil, `renderShape(*Shape)` is `map[string]Fn` / `map[string]Ch` / `*[]Fn`, and **no** `response type ` or `request/response type ` warning.
  - **goconst:** an expected string used three or more times across the package's files (`[]$User`, `$Address`, …) is counted; follow the existing `"kind:" + kindInteger` pattern or hoist repeats into test-file consts.
  - `route.Response.Name`: "" except well-known payloads.
  - The exact payload warnings, matched by the `response type ` and `request/response type ` prefixes. The resolved rows have none. Every F row has exactly one, with its text from the table.
  - `a.typeRegistry` contains `User` and `Address`. Its keys contain no `UserList`, `Tags` or `Cents`.

  Expected red on `9d2b78a`: P rows have nil Resolution and (A)/(B) warnings.

  Also extend `TestHandlerInRouteFileResolvesModuleFileTypes` (Task 1 Step 1b) with its payload half: no warnings at all, and `/cents`'s Resolution renders `int64` with Name "".
- [x] **Step 2 (red): unit tests for the new helpers.**
  - `TestPayloadResolvesRejects`. Hand-built `models.TypeShape` literals (the analyzer package has no shape builders; `internal/generator/shape_builders_test.go` is another package); name rows by their `renderShape` form, e.g. `*Cents`, `*time.Time`, `[]unknown`, `[]decimal.Decimal`): a named non-well-known root; a pointer to it; a pointer to a well-known `time.Time` (true); `ShapeUnknown` inside a slice; a zero-Kind leaf; a pointer with nil `Elem` (true); `slice(named decimal.Decimal)` (true: a non-well-known named leaf below a container root resolves, which pins out the over-wide rejection).
  - `TestRequestlessCompositePayloadKeepsStatuses` (finding 1). One module, no request parameters, the two status handlers of the `named_payloads` fixture (`/map-created`, `/bare-map`), a U4 route `GET /fn-map` returning `server.Result[map[string]Fn]` with a `NewNotFoundError` branch, plus a U2 control `GET /anon` returning `server.Result[struct{ A int }]` with `server.Created(...)` and a `NewNotFoundError` branch. Assert: `/map-created` has `SuccessStatus == 201` and `ErrorStatuses` containing 404; `/bare-map` has `ErrorStatuses` containing 409; `/fn-map` has a non-nil `Response` and `ErrorStatuses` containing 404 (U4, decision 10); `/anon` has a nil `Response`, `SuccessStatus == 0` and no `ErrorStatuses` (U2 still loses them). On `9d2b78a` the first three fail with `SuccessStatus == 0` and nil `ErrorStatuses`.
  - `TestCompositePayloadTypeInfo` (through `payloadTypeInfo`):
    - `map[string]func()` → nil;
    - `map[string]Fn` (`type Fn func()` declared in the file) → nameless TypeInfo with Shape `map[string]Fn` (U4: the leaf is `ShapeNamed` at extraction);
    - `*[]string` → nameless TypeInfo with `ptr(slice(prim))`;
    - `*interface{}` → nameless TypeInfo with `ptr(prim interface{})`;
    - `**int64` and `**Address` → nil (U3);
    - `[]struct{A int}` (via `slicePayloadTypeInfo`) → nil;
    - `[]server.IAPIError` → nil, and `[]*Item` → Name `Item` with Shape `slice(ptr(named Item))`.
  - `TestIsTypedPayload` covers every arm:
    - nil;
    - registered name with no Shape;
    - hand-built well-known name plus Shape;
    - Resolution with a ref, a kind-only, a marshaler, a recursive, a uintptr, a `complex128`, a non-well-known named and an unknown leaf;
    - nameless `{Shape: slice(named Status)}`, untyped (doctor's case);
    - nameless Shape primitive `int64`, typed;
    - a TypeInfo with neither Shape nor Name, untyped.
  - `TestWarnPayloadEveryKind` drives `warnPayload` directly with each `fallbackKind` (none, Marshaler, recursive, depth cap, unresolvable, defined-over-qualified, decls-disagree, untyped builtin) and a uintptr Resolution that outranks a noted Marshaler fallback. Assert the exact text and the bool.
  - `TestPayloadDepthCapAndDeclsDisagree` drives the two kinds that need a composite root to reach `warnPayload` end to end:
    - `[]D0` with an 11-deep chain `type D0 []D1 … type D10 []int` → depth-cap text;
    - `[]W` with `type W []string` in `w_a.go` and `type W []int` in `w_b.go` → decls-disagree text.
  - `TestRegisterPayloadRefsDemotes` calls `registerPayloadRefs` directly with a hand-built `ShapeRef{Name: "Nope"}` and a site file that declares no `Nope`. Assert the leaf is demoted to `ShapeNamed`, there is exactly one `unregisteredRefPayloadWarning`, and none when `warned` is true.
  - `TestExtractResponseTypeWritten` asserts `written` for `server.Result[map[string]Tier]` (`map[string]Tier`), a bare `[]uintptr`, `server.NoContentResult` (""), and `srv.ResultWithMeta[*Cents]` under an aliased server import (`*Cents`).
- [x] **Step 3 (green):** implement **Extraction** items 1–9 and **Payload resolution** exactly as designed. Order: `registerLeafAt` split (field tests stay green), extraction, `payload.go`, then the `populateTypeFields` restructure.
- [x] **Step 4:** `go test ./internal/analyzer`. The new tests are green. The pre-existing analyzer failures must be exactly the analyzer rows of Task 8's list, and nothing else. Do not fix them yet if you want a clean red→green record; otherwise apply Task 8's analyzer updates now.
- [x] **Step 5:** `make lint`. Expect and fix exactly the two prototype findings: `found()` must use a pointer receiver (`hugeParam`), and `extractResponseType` needs named results (`unnamedResult`).

## Task 4: Generator (TDD)

**Files:** `internal/generator/openapi.go`, `internal/generator/openapi_test.go`, `internal/generator/resolution_test.go`, and a new `internal/generator/payload_test.go` for the new tests (keeps `openapi_test.go` from growing; PR 1 precedent).

- [x] **Step 1 (red):**
  - `TestResponsePayloadSchemaResolution`. A table of hand-built `TypeInfo{Shape, Resolution}` → expected property:
    - `prim int64` → `{integer, int64}`;
    - `ptr(ptr(prim int64))` → the same, with no `Nullable`;
    - `slice(ref User)` → array of `$ref`;
    - `ptr(ref Address)` → bare `$ref`, no `allOf`;
    - `map(prim string → marshaler)` → `{type: object, additionalProperties: {}}`;
    - `kind-only integer` → `{type: integer}`;
    - `slice(prim byte)` → `{string, byte}`;
    - `array(ptr(prim byte))` → integer array with `minimum: 0`;
    - `prim uintptr` → `{type: object}`.
  - `TestSuccessEnvelopeAnnotatesOnlyBareObject`. The `responseDataDescription` const (Step 2a; never a `"Response data"` literal in a test) appears on `Resolution: prim uintptr` and on a nameless `Shape: named decimal.Decimal`. It is absent on `Resolution: map(prim int64)`, `map(recursive)` and `slice(prim string)`.
  - `TestReferencedSchemaNamesWalksResponseResolution`. A route with `Response{Resolution: map(ref Address)}` marks `Address`. A route with `Response{Name: "Time", Resolution: named time.Time}` marks nothing. A **request** with `Resolution` set by hand marks nothing, which pins that requests are never walked.
  - `TestPayloadNamesComponentArms` covers each `switch` arm.
- [x] **Step 2a (goconst):** in `openapi.go` add `const responseDataDescription = "Response data"` (next to the other generator string consts) and use it at both production sites: `successEnvelopeSchema` (`grep -n 'data.Description = "Response data"' internal/generator/openapi.go`, 864) and `successResponseSchema` (`grep -n 'Description: "Response data"' internal/generator/openapi.go`, 1199). Replace the existing test literal (`grep -n '"Response data", successEnvelopeSchema' internal/generator/openapi_test.go`, 3049) with the const, and use the const in every new generator test. Afterwards `grep -rn '"Response data"' internal/generator` must print only the const declaration. No golden moves (the text is identical).
- [x] **Step 2 (green):** implement **Generator design** items 1–7.
- [x] **Step 3:** migrate the hand-built generator tests listed in Task 8 (generator rows) and run `go test ./internal/generator`.
- [x] **Step 4:** `go test ./internal/spectest`. Only `named_types`, `qualified_named_types` and `slice_result` fail, each with exactly the **Goldens** diff. Do **not** `-update` yet. This is the checkpoint for retiring `inlinePayloadSchema`, `setElemTypeAndFormat` and `sliceResponsePayloadSchema`: a fourth failing golden means the retirement changed output.

## Task 5: Doctor

- [x] **Step 1 (red), commands:** `TestDoctorClassifiesPayloads`. Use `writeProject` with one route per payload (or use the two new fixtures from Task 6 directly) and `calculateProjectStats`:
  - typed: `Cents`, `UserList`, `map[string]int64`, `AnyD`, `Width`, `*[]string`;
  - untyped: `Tier`, `Tree`, `uintptr`, `[]uintptr`, `decimal.Decimal`, `Fn`, `complex128`, `**Address` (U3, still nil: #120), `map[string]Fn` (U4: carried, but its named `Fn` leaf is a fallback leaf).
- [x] **Step 2 (green):** `isTypedPayload` delegates to `analyzer.IsTypedPayload`, and its doc is rewritten.

## Task 6: Fixtures and goldens

- [x] **Step 1:** Create `named_payloads/` and `named_payload_fallback/` exactly as in **Goldens**, with no `expected.yaml`. `gofmt -w` their `.go` files.
- [x] **Step 2:** Rewrite the three fixture comments (**Goldens**).
- [x] **Step 3:** `go test ./internal/spectest -update`, then `git status --porcelain internal/spectest/testdata`. It must list only the two new directories, the three moved `expected.yaml` and the three comment-only `.go` edits.
- [x] **Step 4:** Check the three diffs line by line against **Goldens**. Check both new goldens cell by cell against P1–P14 and F1–F10 (data schema, components, no `description` on typed maps), plus `/map-created`'s `"201"`/`"404"` (no `"200"`) and `/bare-map`'s `"409"`.
- [x] **Step 5:** `go test ./internal/spectest` green.

## Task 7: Parity oracle and CLI tests

- [x] **Step 1: `TestPayloadFieldParity`** (`internal/spectest/payload_parity_test.go`), the brief's acceptance test.
  - Write one project with the declarations of both new fixtures, a `Fields` struct with one field `F<i>` per row, and a route `/fields` returning `server.Result[Fields]`.
  - Add one route `/p<i>` per row, returning the row's payload expression. Rows cover every P and F cell, plus `ResultWithMeta[Cents]`, `ResultWithMeta[UserList]`, `ResultWithMeta[map[string]int64]`, the `WithRawResponse()` variants of `Cents`, `UserList` and `map[string]int64`, `Width` (kind-only), and bare `Cents`, `UserList` and `map[string]int64`.
  - Each row's field type is `T`, or `T`'s pointed-to type when `T` or its resolved form is a pointer: `*Cents`→`Cents`, `PC`→`int64`, `*PC`→`int64`, `*Tags`→`[]string`, `*UserList`→`UserList`, `*[]string`→`[]string`, `*[]byte`→`[]byte`, `*[4]byte`→`[4]byte`, `*interface{}`→`interface{}`, `*Tier`→`Tier`, `*Missing`→`Missing`, `*[]Fn`→`[]Fn`. U4 rows `map[string]Fn`, `[][]Fn`, `*[]Fn` and `map[string]Ch` are included (field parity is what decision 10 rests on). There is no `**T` row (U3 stays nil; #120).
  - Assert, per row: the payload schema (`data`, or the whole body for raw routes) with its `description` key removed equals `Fields.properties["f<i>"]`. The document validates (`Validate`).
  - Also assert that `components` holds no named non-struct entry (no `Cents`, `Tags`, `UserList`, `Tier`, `Tree` …) and does hold `User` and `Address`.
- [x] **Step 2: `TestRunGenerateNamedPayloadsFixtureStrictClean`** (`internal/commands/named_payloads_test.go`). Model it on `TestRunGenerateNamedResolutionFixtureStrictClean`: `ProjectRoot: ../spectest/testdata/named_payloads`, `Strict: true`, `Validate: true`. Expect no error and `Warnings: 0`.
- [x] **Step 3: `TestRunGeneratePayloadFallbacksFailStrict`** in the same file. Use one temp project per F row, with a `fieldFallbackModSrc`-style template holding `DECLS` and `PAYLOAD`.
  - Base the template on `unresolvablePayloadModSrc` (`internal/commands/generate_test.go`), whose handler takes a typed `PriceReq` request. The route therefore keeps a typed request and raises no aggregate route-count warning, which **is** counted in `Warnings: N`: the fallback fixture prints `Warnings: 21` for 20 payload warnings.
  - `--strict` must fail with no artifact, and stdout must contain `Warnings: 1\n`.
  - A non-strict `--validate` run must succeed.
  - Include `uintptr` and `[]uintptr`. Those two flip from strict-clean to strict-failing.
- [x] **Step 3b:** the `named_payloads` golden (Task 6) pins `/map-created`'s `"201"`/`"404"` and `/bare-map`'s `"409"` end to end; `TestPayloadFieldParity` compares only `data` and cannot catch a status regression, which is why Task 3 adds `TestRequestlessCompositePayloadKeepsStatuses`.
- [x] **Step 4:** `TestUnresolvablePayloadFallsBackWithWarning` and `TestRunGenerateUnresolvablePayloadFallsBack` pass **unchanged** (check with `git diff --stat` that neither test file changed in their hunks).

## Task 8: Deliberate updates to existing tests (each named in the commit body)

These are the complete prototype failure lists. Each changes on purpose. Every rendered expectation below is in `renderShape` notation (Task 3 Step 1); a test that today compares `Kind`/`Name` fields directly may keep doing so for the same shape. **Analyzer:**

1. `TestNamedSliceWarnsAndClears` (`analyzer_test.go` 4058). Rename it `TestNamedSlicePayloadResolves` and assert:
   - no warning;
   - `Name == ""`;
   - `renderShape(*Resolution)` is `[]$User`;
   - `a.typeRegistry` contains `User` and not `UserList`.

   This is the brief's named update.
2. `TestNamedScalarSliceElementClearsNameKeepsShape` (4103). Rename it `TestNamedScalarSliceElementResolves` and assert:
   - Name "";
   - `renderShape(*Shape)` is still `[]Status`;
   - `renderShape(*Resolution)` is `[]string`;
   - no warning;
   - `Status` not registered.
3. `TestAliasChainDepthCapped` (`grep -n 'func TestAliasChainDepthCapped'`). `Result[T0]` with a ten-alias chain to struct `T9` now resolves exactly as a `T0` field does: a `$ref` to component `T1`, with no warning (prototype: `scratchpad/pr3/chain`). Assert:
   - `renderShape(*Resolution)` is `$T1`;
   - `Name == ""`;
   - there are no warnings;
   - `T1` is registered.

   Rewrite its doc. The cap still stops `registerViaTypeSpec`, but the resolver opens one alias and resolves the rest. The old "Step 3 warning" is gone because the payload now matches the field.
4. `TestTypeInfoFromExprResultWrappers` (3593–3618), three rows:
   - `result_slice_of_pointer_is_treated_as_slice_of_value`: the Shape now renders `[]*User` (Elem is `ShapePointer` over named `User`), and Name is still `User`. Rename it `…keeps_the_element_pointer`.
   - `result_nested_slice_is_nil` becomes `…is_carried_nameless`, with Shape `[][]User`.
   - `result_map_is_nil` becomes `…is_carried_nameless`, with Shape `map[string]User`.
5. `TestFixedArrayPayloadShapes` (`fixed_array_test.go` ~60): the `[2]*Address` and `[4]*byte` rows (×3 forms). The expected Shape becomes `[N]*Address` / `[N]*byte` (Elem a `ShapePointer`). The emitted output is unchanged (the `fixed_array` golden does not move).
6. `TestBareReturnPayloadShapes/map_stays_untyped` (`bare_return_test.go` 145). Rename it `map_resolves`: `ti` is non-nil and nameless, `renderShape(*ti.Shape)` is `map[string]Cents`, `renderShape(*ti.Resolution)` is `map[string]int64` (`Cents` is `int64` in that file), and there are no warnings.
7. `TestNonStructRequestWithUnresolvedResponseStillWarns` (`nonstruct_request_test.go` ~172). The response `map[string]Status` now resolves. Change it to `map[string]chan int`, which still resolves to nothing, so the test keeps proving that a handler is found when neither side resolves.
8. `TestNonStructRequestLeavesResponseWarningsAlone/named_non_struct_response` (~180). Keep `Status` and assert only the request warning (rename the subtest `named_non_struct_response_resolves`). Add a subtest `unresolvable_response` with `server.Result[decimal.Decimal]`, asserting the request warning plus the (B) text, if that file's fixture imports `decimal`. Otherwise use `server.Result[*Missing]`.
9. `TestQualifiedPayloadUnchanged` (`qualified_test.go` 371). Rename it `TestQualifiedPayloadResolves` and assert no warning plus `renderShape(*Resolution)` == `int64`. Rewrite its doc.

**Commands:**

10. `TestRunGenerateBareReturnMatchesResult` (`bare_return_test.go`): rows `Cents`, `Tags` and `map[string]Cents` become `typed: true`. The bare/wrapped parity assertion is unchanged and still holds.
11. `TestDoctorCountsNonStructRequestUntyped/non_struct_request_untyped_response` (`nonstruct_request_test.go` 141). The response `map[string]string` is now typed. Change it to `map[string]chan int` so the route stays fully untyped.

**Generator** (all hand-built payloads with no Resolution; the analyzer now always stamps one for these):

12. `TestResponsePayloadSchemaInlineScalars`, `TestBuildResponsesInlineScalarPayloads`, `TestReferencedSchemaNamesSkipsInlinePayloads` and `TestGenerateNoOrphanForWellKnownPayloadName`. Each hand-built well-known or builtin payload gains `Resolution` equal to its Shape (a well-known leaf resolves to itself). A test builder `resolvedPayload(s models.TypeShape) (shape, resolution *models.TypeShape)` may be added to `shape_builders_test.go`.
13. `TestInlinePayloadSchemaRejects`: delete it with `inlinePayloadSchema`.
14. `TestResponsePayloadSchemaSliceWithoutElement`: a nameless slice Shape with nil `Elem` now gives `{type: array, items: {}}` (`setTypeAndFormat`), still a valid array. Update the items assertion to empty.
15. The `setTypeAndFormat` receiver edits (generator design item 1).

## Task 9: Docs

- [x] `README.md` Known limitations, the `server.Result[T]` bullet (`grep -n 'A \`server.Result\[T\]\` payload is typed inline' README.md`, 272–290). Rewrite it as:
  - A payload (wrapper type argument or bare return) is documented exactly as a struct field of its type: named non-struct types (local or from another package of the module), maps, nested slices and pointers to containers included. The root is never `nullable`.
  - A project struct is referenced, even one whose short name collides with a well-known type. A slice payload of such a colliding struct (`server.Result[[]uuid.UUID]`) is still typed inline (keep this sentence).
  - A payload that ends in a fallback (a Marshaler type, recursion, `uintptr`, `error`/`complex64`/`complex128`, or a type that resolves to no schema) raises one warning, so `--strict` fails. A named func or chan type inside a container (`map[string]Fn`) is documented as an untyped object silently, as for a field.
  - Drop "(except `uintptr`, which stays an untyped object with no warning)".
  - Keep the `[]byte`/`[N]byte` and bare-return sentences. Keep the anonymous-struct and generic exceptions as their own short sentence (#118, #114).
- [x] `CONTEXT.md` **Shape** entry: replace "so a well-known or builtin payload is typed from it rather than `$ref`'d" with "so a payload that does not register as a project struct is resolved (see **Resolution**) and typed like a field of its type rather than `$ref`'d". **Resolution** already names payloads, so it needs no edit.
- [x] `docs/adr/0002-named-type-resolution.md`: no edit. It is accepted, and its staging bullet already assigns payload resolution to #110.
- [x] Doc comments updated in code (each listed in **Analyzer/Generator design**):
  - `models.TypeInfo.Shape` and `TypeShape`;
  - `payloadTypeInfo`, `scalarPayloadTypeInfo` (its "interface{} is resolved here" paragraph stays; drop "shedRegisteredPayloadShape" only if renamed), `slicePayloadTypeInfo` and `PayloadBaseShape`;
  - `populateTypeFields`, `dropUnresolvablePayloadName`, `isTypedPayload`, `classifyRoute` (it says "the same gate the generator uses (see responsePayloadSchema)", which is still true);
  - `responsePayloadSchema`, `successEnvelopeSchema`, `payloadNamesComponent`, `referencedSchemaNames`, `shapeAfterPointer`;
  - `wellKnownFormats`' "payloads, until #110" clause becomes "registered struct slice elements".

## Task 10: Lint, complexity, coverage, mutation

- [x] `make check` (pinned golangci-lint, `0 issues`). `go test -race -count=1 ./...`.
- [ ] Watch SonarCloud's new-code duplication (≤3%, server-side only) on the PR. `warnPayload` and its const block parallel `warnFieldFallback` and `resolution.go`'s field consts. `dupl` (100 tokens) passed in the prototype. If Sonar's CPD trips, share a small formatter for the fallback tail between the two warn functions, and keep every field warning's text byte-identical (the existing field-warning tests pin it).
- [x] `go run github.com/uudashr/gocognit/cmd/gocognit@latest -over 15 internal/analyzer internal/generator internal/commands`. The only allowed hits are the two baseline ones. Check every touched function against the budget table.
- [x] Coverage: `go test ./internal/analyzer -coverprofile=/tmp/…/a.out && go tool cover -func=… | grep -E 'payload.go|resolvePayload|registerLeafAt|compositePayloadTypeInfo|hasUnknownLeaf|isCompositePointee|extractResponseType|slicePayloadTypeInfo|payloadTypeInfo|populateTypeFields|screenUnresolvedPayload'`. Do the same for the generator (`responsePayloadSchema`, `payloadShapeSchema`, `structSlicePayloadSchema`, `successEnvelopeSchema`, `referencedSchemaNames`, `payloadNamesComponent`). Target 100% statement coverage on every new function and ≥80% overall on new lines. `isTypedPayload` is a one-line delegate covered by commands tests.
- [x] Mutation checks (each restored to an empty diff from a scratch copy of the edited file; never `git checkout --` uncommitted work):
  1. `resolvePayload` returns false on entry. Fails: `TestPayloadResolutionRows`, `TestPayloadFieldParity`, `TestGoldenFixtures/named_payloads`, `TestRunGenerateNamedPayloadsFixtureStrictClean`, `TestDoctorClassifiesPayloads`.
  2. `IsTypedPayload` reverts to `Name != "" || base primitive`. Fails: `TestIsTypedPayload` and `TestDoctorClassifiesPayloads` (the uintptr, Cents and map rows).
  3. `slicePayloadTypeInfo` sheds the element pointer again. Fails: the `[]*byte` rows of `TestPayloadResolutionRows` and `TestPayloadFieldParity` (base64), and `TestGoldenFixtures/named_payloads`.
  4a. Seam A: `extractRoutesFromPackage`'s `collectRoutesFromFile(file, path, …)` reverts to `filePath`. Fails: `TestHandlerInRouteFileResolvesModuleFileTypes` (field and `/cents` payload halves). `TestHandlerInSiblingFileResolvesInItsOwnFile` stays green.
  4b. Seam B: the sibling-loop `path` reverts to `filePath`. Fails: `TestHandlerInSiblingFileResolvesInItsOwnFile`. `TestHandlerInRouteFileResolvesModuleFileTypes` stays green.
  5. `successEnvelopeSchema` drops `AdditionalProperties == nil`. Fails: `TestSuccessEnvelopeAnnotatesOnlyBareObject` and `TestGoldenFixtures/named_payloads` (typed maps gain `description`).
  6. `referencedSchemaNames` drops the Resolution walk. Fails: `TestReferencedSchemaNamesWalksResponseResolution`. `named_types` still emits `User`, because `generateSchemasFromTypes` emits every registered non-params-only type, so the golden alone would not catch it. That is why the unit test exists.
  7. `warnPayload` skips the `holdsUintptr` check. Fails: the uintptr rows of `TestPayloadResolutionRows` and `TestRunGeneratePayloadFallbacksFailStrict`.
  8. `payloadTypeInfo`'s `*ast.MapType` arm returns nil. Fails: `TestRequestlessCompositePayloadKeepsStatuses` (201/404/409 lost), the map rows of `TestPayloadResolutionRows`, and `TestGoldenFixtures/named_payloads`.
  9. `isCompositePointee` also returns true for `*ast.StarExpr` (the prototype's `**T` behavior). Fails: the U3 rows of `TestPayloadResolutionRows` and `TestCompositePayloadTypeInfo`, and `TestDoctorClassifiesPayloads`'s `**Address` row.

## Task 11: Commit

Subject (Conventional Commit, `fix` → Fixed; most of the change is analyzer-side). Count it: ≤72.

```
fix(analyzer): type named and composite response payloads like fields
```

Body (write to a file, then `git commit -F <file>`; the commit hook blocks heredoc `-m`). Do not commit until the orchestrator says so.

```
A response payload, either the T of server.Result[T] / ResultWithMeta[T] or
a handler's bare first result, now documents exactly the schema a struct
field of type T documents and warns exactly when that field would. Before,
every named non-struct payload (Cents, Tags, UserList, b.Cents) and every
unnamed composite (map[string]int64, [][]Cents, *[]string) was
an untyped object with a warning or the "no resolved request/response
type" aggregate, so --strict failed on valid code and structs reached only
through such a payload (UserList's User, map[string]Address's Address)
were never emitted. The root is the one exception: it is never nullable
and sheds its resolved pointer (Result[PC], type PC *int64, is an int64).
A request-less handler whose payload is a map, a nested slice or a pointer
to a container was not found at all, so its success status (201 for
server.Created, 202 for Accepted, s for NewResult(s, ...)) and the error
responses inferred from its body were silently dropped; it is now found
and they are documented.

models.TypeInfo gains the Resolution PR #128 left out, read through
ResolvedShape. populateTypeFields runs the field resolver on every payload
that carries a Shape and does not register as a struct, before the two
older screens, which keep handling a payload whose root resolves to
nothing (an undeclared or out-of-module name, an aliased well-known
import, a defined type over a well-known struct, an unmodelled leaf such
as func()). A resolved payload names no component; the structs it reaches
register in the file their names were written in and are $ref'd from its
schema, and referencedSchemaNames walks response Resolutions (request
types are still never scanned). Payload extraction carries maps, nested
slices and pointers to a slice, array, map or interface{} nameless, on the
payload path only (a slice or map handler parameter still resolves to nil,
and a written **T root stays nil, leaving #120's open question alone), and
keeps a slice payload's element pointer: Result[[]*byte] was documented
as base64 while encoding/json writes a number array.

The generator types a Resolution with the walk fields use
(setTypeAndFormat, now a plain function); inlinePayloadSchema and
setElemTypeAndFormat are gone, and sliceResponsePayloadSchema shrinks to
structSlicePayloadSchema for registered struct slices, which keep their
path ([]uuid.UUID of a colliding project struct is still typed inline).
"description: Response data" now marks only a bare untyped object, not a
typed map. doctor and generate's route count classify payloads through
analyzer.IsTypedPayload: a payload is typed exactly when its schema has no
fallback leaf, so Result[AnyD] ({}) is typed and Result[Tier] ({}) is not.

Warnings: a payload that ends in a fallback raises exactly one warning,
naming the payload as written: a Marshaler type (now {} instead of
object), recursion (Tree), uintptr (Result[uintptr] and Result[[]uintptr]
were silent and counted typed; they now warn, fail --strict and count
untyped, reversing README's Result[T] note), error/complex64/complex128,
a defined type over a qualified struct inside a composite, disagreeing
build-tagged declarations, or a struct leaf that cannot register. An
unresolvable leaf keeps the "response type X resolves to no schema
component" text, now also for composites such as
map[string]decimal.Decimal, which had only the aggregate warning.

Also fixed: the resolver was handed one file's AST with another file's
path, always the path of the file declaring the Module struct, at two
seams. extractRoutesFromPackage walked a RegisterRoutes kept in another
file with that path, and extractHandlerSignature populated a handler found
outside the walked file with it. The resolver skips the file whose path it is
given, so a field or payload typed with a named type declared in the
Module-struct file (T Tags, Result[Cents]) warned "resolves to no schema"
on valid code whenever routes or handlers lived in their own file.

Result[**int64] and Result[**Address] stay unresolved, as before: whether
a written double-pointer payload resolves is still open on #120. Marshaler
payloads stay {} under #111's pending string rule.

A map, nested slice or pointer to a slice over a named func or chan type
(map[string]Fn, [][]Fn, *[]Fn) now takes its field's schema, a container
of untyped objects, and keeps its statuses, but stays untyped with only
the aggregate warning, which does not fire when the route has a typed
request: the field of that type is silent too.

Goldens: named_types (/users is an array of $ref User; components gain
User), qualified_named_types (/cents is an int64) and slice_result
(listStatuses items are strings) move; their fixture comments are
rewritten. New fixtures named_payloads (every resolved row;
generate --strict --validate is clean) and named_payload_fallback (every
fallback row, one warning each).

Tests: payload rows, extraction, warning-kind, demotion, typedness and
two file-context analyzer tests (one per seam); generator Resolution, envelope-annotation and
reference tests; a request-less status test (201/404 on a map payload,
409 on a bare map, a struct{...} payload still losing them); a spectest
parity oracle comparing each payload's schema
with the field of its type (pointer roots against the pointed-to type),
including ResultWithMeta, WithRawResponse and bare returns; CLI strict and
doctor tests. Renamed because they now resolve:
TestNamedSliceWarnsAndClears -> TestNamedSlicePayloadResolves,
TestNamedScalarSliceElementClearsNameKeepsShape ->
TestNamedScalarSliceElementResolves and TestQualifiedPayloadUnchanged ->
TestQualifiedPayloadResolves. Removed: TestInlinePayloadSchemaRejects,
with inlinePayloadSchema. Updated on purpose:
TestAliasChainDepthCapped (Result[T0] now $refs T1 like a T0 field),
three TestTypeInfoFromExprResultWrappers rows, TestFixedArrayPayloadShapes'
pointer-element rows, TestBareReturnPayloadShapes' map row, two
non-struct-request tests that used a now-resolving response, three
TestRunGenerateBareReturnMatchesResult rows (now typed),
TestDoctorCountsNonStructRequestUntyped's untyped-response row, and the
hand-built generator payload tests (they now carry a Resolution).
TestUnresolvablePayloadFallsBackWithWarning and
TestRunGenerateUnresolvablePayloadFallsBack pass unchanged.

BREAKING CHANGE: payload data schemas move from object to typed schemas,
and specs gain components for structs reached only through a payload. A
struct registered through a payload can now take a short name first, so
a same-named struct of another package registered later is renamed
(User becomes <Pkg>User), moving $refs and generated client type names;
regenerate clients. Request-less routes with a map, nested-slice or
pointer-to-container payload gain their 201/202/NewResult success code
and inferred error responses.

Closes #110
Refs #120
Refs #111

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

PR body (if the orchestrator opens one, per the user's global three-heading rule, ≤150 words):

```markdown
## What
Response payloads (`Result[T]`, `ResultWithMeta[T]`, bare returns) were an untyped `object` for named non-struct types and unnamed composites, though a field of the same type was typed. They now emit exactly the field's schema, the root never `nullable`, and warn exactly when that field would. A file-context bug that hid types declared in the Module-struct file from routes and handlers in other files is fixed too.

## Impact
`data` schemas move to typed schemas; such routes pass `--strict`, count as typed, and request-less ones gain their 201/202 and error responses. `uintptr` payloads newly warn and fail `--strict`. A struct now registered through a payload can rename a same-named struct of another package; regenerate clients.

## Verification
Three goldens move (`named_types`, `qualified_named_types`, `slice_result`); mutation checks bit.

Closes #110

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

---

## Impact (user-visible behavior changes; SemVer surfaces: generated-output shape, doctor/validation)

1. Named non-struct payloads (rows P1–P10), local or from another package of the module, move from `{type: object, description: Response data}` to their field schema. Their (A)/(B) warning disappears, so such routes **newly pass `--strict`** and **count as typed in `doctor`**.
2. Unnamed composite payloads (P11–P12: `map[string]T`, `[][]T`, `[]map[K]V`, `*[]T`, `*map[K]V`, `*interface{}`) move from `object` plus the aggregate "no resolved type" warning to typed schemas, also counted typed. A written `**T` root (U3) does not move: #120's payload question stays open. A nameless composite over a named func or chan type (U4: `map[string]Fn`, `[][]Fn`, `*[]Fn`, `map[string]Ch`) moves from `{type: object, description: Response data}` to its field's container-of-`{type: object}` schema, but stays untyped: it keeps only the aggregate warning (no per-type warning, as for the field) and still fails `--strict` when the route has no typed request.
3. Specs **gain components** for structs reached only through a payload (`UserList`'s `User`, `map[string]Address`'s `Address`). That can **rename** a same-named struct of another package registered later (`User` → `<Pkg>User`), moving `$ref`s and generated client type names (BREAKING CHANGE footer; reproduced in `scratchpad/pr3/ren`).
4. `Result[[]*byte]` stops being documented as base64 and becomes an integer array, matching `encoding/json`. `Result[*[]byte]` becomes base64, and `Result[*[4]byte]` an integer array.
5. Marshaler payloads (`Tier`, `[]FlagV`, `map[string]Tier`) become `{}` (any JSON value) instead of `object` and **still warn**, now with the Marshaler-type text.
6. **`Result[uintptr]` and `Result[[]uintptr]` newly warn, fail `--strict` and count untyped.** Before, they were silent and typed. This reverses README's note.
7. **`Result[complex128]` (and `complex64`, and `[]error`) newly warn, fail `--strict` and count untyped**, matching fields. `type Cx complex128` changes from the (A) text to the untyped-builtin text.
8. Composites with a fallback leaf gain a per-type warning (`map[string]decimal.Decimal` → (B) naming `decimal.Decimal`; `map[string]Tier` → Marshaler; `map[string]uintptr` → uintptr). Before, they had only the aggregate warning. `[]StampD` changes from the (A) text to the defined-over-qualified text.
9. `description: Response data` no longer marks typed maps. It stays on bare untyped objects.
10. Pre-existing context bug fixed at two seams: a `RegisterRoutes` kept outside the Module-struct file (seam A, `extractRoutesFromPackage`), and a handler kept in another file than its `RegisterRoutes` (seam B, `extractHandlerSignature`'s sibling loop), now resolve named field and payload types declared in the Module-struct file. Before, both were resolved with the Module-struct file's path, so those types warned "resolves to no schema" and documented `object`.
11. Unchanged: `decimal.Decimal`, `t.Time` and `*Missing` payloads (same (B) text), `StampD`/`Fn` roots (same (A) text), anonymous-struct and generic payloads and written `**T` roots (U2, U3: still a nil response, so a request-less handler returning one still loses its success and error statuses), every struct, well-known and builtin payload, and every request.
12. **Request-less handlers whose payload is newly carried now keep their statuses.** A handler with no request parameter returning a map, nested slice or pointer-to-container payload (P11, P12, P7's `[][]string`, `*[]byte`, `*[4]byte`, the F2/F5/F7 maps, every U4 row; wrapped or bare) used to be "not found" (`handlerAnalysis.found()` false), so its success status and inferred error responses were dropped: it documented `200` and no `404`/`409`/…. It now documents its constructor-derived success code (`server.Created` → `201`, `Accepted` → `202`, `NewResult(s, …)` → `s`; a `NoContentResult` handler was already found, through its marker) and its body-inferred error responses. Response keys are generated-output shape, so this rides the BREAKING CHANGE footer. Pinned by `TestRequestlessCompositePayloadKeepsStatuses` and the `named_payloads` golden (`/map-created`, `/bare-map`).

---

## Open decisions (recommended default first)

1. **A payload whose root resolves to nothing keeps today's (A)/(B) text.** **Default: keep** (the brief: "the local non-struct warning and dropUnresolvablePayloadName … keep handling whatever resolves to nothing"). So `StampD` warns (A) while `[]StampD` gets the field-class text. Alternative: route root fallbacks through `warnPayload` too (field-class texts everywhere, (B) kept only for unresolvable). That retires (A) except for `Fn` roots and changes more warning texts.
2. **Payload warnings name the payload as written** (`response type map[string]Tier: Tier has …`), carried in `handlerAnalysis.responseType`. **Default: yes.** It costs one struct field (hence `found()`'s pointer receiver) and a two-result `extractResponseType`. Alternative: name only the type where resolution stopped, as (A)/(B) do. Then the uintptr warning has no name to show for `Result[[]uintptr]` without a shape renderer.
3. ~~`Result[**T]` resolves~~ **Settled by review (finding 2): a written `**T` root stays nil** (`isCompositePointee` excludes `*ast.StarExpr`), and #120 keeps its open payload question (`Refs #120` only). Reopening it means a user decision on #120, then a one-line change to `isCompositePointee` plus a `Closes #120` trailer.
4. **Registered struct slice payloads keep today's path** (`Resolution == nil`, `structSlicePayloadSchema`). **Default: yes**, because the brief says struct payloads keep today's output. Alternative: resolve them too. That fixes README's `[]uuid.UUID` colliding-struct residual (it would `$ref` the project struct) and retires `isWellKnownElem`. It moves no golden in this repo, but it changes documented behavior.
5. **Fix both file-path seams in this PR** (`extractRoutesFromPackage`'s route loop and `extractHandlerSignature`'s sibling loop). **Default: yes**: payload resolution is wrong without them (a `RegisterRoutes` outside the Module-struct file is a common layout), and each is a two-token change with its own red test and mutation. They also fix fields (Impact 10). The planner touches no GitHub state, so the user may file a tracking issue if they want the field half recorded separately.
6. **Untyped builtins (`complex128`, `[]error`) warn and count untyped as payloads.** **Default: yes** (field parity; the brief: "of the class its field would raise"). Alternative: leave builtin payloads silent and typed as today.
7. **Composites with a literal unmodelled leaf (`[]struct{…}`, `map[string]func()`, `map[string]chan int`) stay nil.** **Default: yes** (#118, #114). This is decided at extraction, where the leaf is already `ShapeUnknown`. A **named** func/chan leaf is only unknown after resolution, so it is carried: that is U4, decision 10.
8. **`setTypeAndFormat` loses its receiver.** **Default: yes** (it never read `g`; about 18 mechanical test edits). Alternative: call `(&OpenAPIGenerator{}).setTypeAndFormat` from `payloadShapeSchema`, with no test churn and an awkward call site.
9. **BREAKING CHANGE footer** for the component-rename side effect (Impact 3), following PR 2's precedent. **Default: include it.** Pre-1.0 caps the bump to MINOR. Alternative: omit it, since the rename needs two same-named structs across packages and a payload route registered first.
10. **U4 (a nameless composite over a named func/chan type, `map[string]Fn`) takes the field's schema, keeps statuses, and stays untyped with only the aggregate warning.** **Default: (b) accept** (Impact 2, 12): it is exactly the brief's parity rule, since the field `F map[string]Fn` emits the same `{type: object, additionalProperties: {type: object}}` with no warning, and `IsTypedPayload` keeps the route untyped. Alternative (a), keep today's nil response: it cannot be done at extraction (the leaf is `ShapeNamed` there; only the resolver, which needs the file context, sees it is a func), and dropping the response inside `populateTypeFields` comes after `found()` has already been decided, so the route would keep its statuses anyway unless both `extractHandlerSignature` branches re-check `found()`. That is a third behavior that matches neither today nor the field. Alternative (c), add a per-type warning naming the func/chan leaf: payload-only, so it breaks field parity unless fields warn too (out of scope).

---

## Review dispositions

### First review (2026-10-09), both findings verified against `9d2b78a`, the prototype and the issues

1. **Finding 1 (major), request-less composite payloads newly emit success/error statuses: applied.** Verified: `found()` (`analyzer.go:1545`) is `request != nil || requestType != "" || response != nil`, and `extractHandlerInfo` returns `handlerAnalysis{}` on its error; the reviewer's probe (`scratchpad/pr3/cr_go/st`) shows `200` → `201` plus new `404`/`409` keys between the `9d2b78a` and prototype binaries. The planner's own probe (`scratchpad/pr3/cr_go/iface`, `base.yaml` from a fresh `9d2b78a` build vs `proto.yaml`) shows `*interface{}` also flips: `{type: object, description: Response data}` with the aggregate warning and no `404` before, `{}` typed with its `404` after, so it joins P11. Applied: Architecture sentence, Extraction item 7 note, Behavior-rows note, Impact 12 and Impact 11's U2/U3 clause, commit-body sentence and BREAKING footer clause, `TestRequestlessCompositePayloadKeepsStatuses` (Task 3, with a U2 control), `/map-created` and `/bare-map` in the `named_payloads` golden, mutation 8, PR-body Impact clause.
2. **Finding 2 (major), the plan settled #120's open `Result[**T]` question: applied via option (a).** Verified: #120 is OPEN, its body ends "Open: should `PayloadBaseShape` also shed every level, so `Result[**T]` emits what `Result[*T]` emits?", and its reopen comment says the payload half is still open; #110's brief says "sheds one pointer level" and lists no `**T` cell. Option (b) needs the user's sign-off, which a planner cannot get. Applied: `isCompositePointee` replaces `isNamedTypeExpr` so a written `**T` root keeps `9d2b78a`'s nil (new row U3, pinned in `TestPayloadResolutionRows`, `TestCompositePayloadTypeInfo`, doctor's untyped list and mutation 9); P13, `**int64` in P1, `/pptr-int`, `/pptr-address`, the `**` parity rows and Impact 2's "settles #120" are gone; the commit body says `Result[**T]` stays unresolved and keeps only `Refs #120`; open decision 3 is closed.
   - **Goal wording, recorded deviation:** the Goal now uses the brief's "one pointer level of its resolved form". The emitted schema is the same either way, because `setTypeAndFormat` strips every pointer (`stripPointers`, `openapi.go:1707`, added by #128 after the brief). So `Result[*PC]` (written single pointer, resolved `**int64`) still emits `{integer, int64}` and stays in P1. The gate is on the **written** root, not on the resolved form's depth. `payloadResolves` still sheds every root pointer, but only to find the root's kind, not to choose the schema.
   - **Kept, not #120's question:** `[]**T` and `[]*[]T` elements go through `slicePayloadTypeInfo`'s composite arm. They are element positions, which #128 already settled for fields (`[]**int64` types like `[]*int64`). #120's open question is only about the payload root.

### Second review (2026-10-09), both findings verified against `9d2b78a`

3. **Finding 3 (major), Task 3 expected a nil Resolution for `[]decimal.Decimal`, contradicting `payloadResolves`: applied.** Verified: `resolveQualified` (`resolution.go:289`) returns the leaf unchanged as `ShapeNamed decimal.Decimal` after noting `fallbackUnresolvable` (line 303), never `ShapeUnknown`, so `payloadResolves` is true for `slice(named decimal.Decimal)`. Extraction item 9 and the `Resolution` doc already said "root resolves to nothing", so only the test expectation and the F6 row contradicted the design. Applied: Task 3 Step 1 expects `slice(named:decimal.Decimal)` with nil only for root F6, F8 and U1; the `payloadResolves` row states that a named leaf below a container resolves and forbids the over-wide fix; a Behavior-rows note and the F6 cell explain the path; `TestPayloadResolvesRejects` gains the positive `slice(named decimal.Decimal)` case. Task 7 Step 4 still holds: `TestUnresolvablePayloadFallsBackWithWarning`'s `/decimals` asserts `Name == ""` (cleared by `resolvePayload`), `PayloadBaseShape(*Shape).Kind == ShapeNamed` (Shape is never rewritten) and the (B) substring naming `decimal.Decimal` (`warnPayload` passes `fb.typeName`).
4. **Finding 4 (major), a new `"Response data"` test literal trips goconst on `openapi.go`: applied, with refined evidence.** Reproduced with the pinned v2.12.2 on `scratchpad/pr3/review_goconst`: a test with `want := "Response data"` gives `openapi.go:864:22: string \`Response data\` has 3 occurrences` (864 assignment, 1199 composite-literal value, the new one). The same literal as a bare call argument (`assert.Equal(t, "Response data", …)`) gives `0 issues`, because of goconst's default `ignore-calls` (`.golangci.yml` does not set it), which is also why the existing call-argument literal at `openapi_test.go:3049` is not counted today. The finding's "already at threshold" is therefore true only for non-call positions, but a table-driven envelope test row is a composite literal and would trip. Applied: new Task 4 Step 2a (`responseDataDescription` const at both production sites, the 3049 test site and every new test) and the extended Global Constraints goconst bullet.

### Third review (2026-10-09), all three findings verified against `9d2b78a` and the scratch binaries; none rejected

5. **Finding 5 (blocker), Task 1 fixed only one of two mismatched `(AST, path)` seams: applied.** Verified: `extractRoutesFromPackage` (`analyzer.go:686–690`) walks every `RegisterRoutes` file with the `filePath` `analyzeFile` passed at 498 (the Module-struct file's), so `extractHandlerSignature`'s first branch (3066–3068) pairs the routes file's AST with that path. Reproduced on `scratchpad/pr3/review3_split` (Module + `type Tags []string` in `module.go`; `RegisterRoutes`, `Resp{T Tags}` and the handler in `routes.go`): `base_bin` and `review3_t1_bin` (seam B only) both warn `field T at routes.go:10:2 has type Tags: Tags resolves to no schema` and emit `t: {type: object}`; `review3_t1b_bin` (both seams; the scratch diff is exactly the four lines at 685/690/3075/3077–3078) gives `Warnings: 0` and `t: {type: array, items: {type: string}}`. Checked that `path` shadows no import and that child walkers (`recurseInto`, `findPackageFuncDecl`) already pair from one source. Applied: Architecture, Stale point 4 (both seams, the Module-struct file's path), **Task 1's fix** rewritten for both loops with `grep` recipes, `newResolveCtx` row, complexity row for `extractRoutesFromPackage` (6, unchanged), Task 1 Step 1b `TestHandlerInRouteFileResolvesModuleFileTypes` (field half in Task 1, `Result[Cents]` payload half in Task 3), mutation 4 split into 4a/4b (each seam's test goes red alone), Impact 10, open decision 5, commit-body "Also fixed" paragraph and test list, PR-body What.
6. **Finding 6 (major), U4, nameless composites over a named func/chan leaf were unclassified: applied with disposition (b).** Verified on `scratchpad/pr3/gates_par` with `base_bin` and `proto_bin`: `map[string]Fn`, `[][]Fn`, `*[]Fn` and `map[string]Ch` move from `{type: object, description: Response data}` with statuses 200/400/500 to containers of `{type: object}` with 404 added; only the aggregate warning lists them; the `Fields` component's `f1`/`f2`/`f8` are byte-identical to the payload schemas and raise no field warning. (b) is the brief's parity rule, and `IsTypedPayload` keeps the routes untyped. (a) was rejected: the leaf is `ShapeNamed` at extraction, and dropping the response after resolution comes after `found()`, so statuses would be kept anyway (open decision 10). Applied: the `Resolution`/`Shape` doc text (Data model), the `populateTypeFields` doc (four outcomes), the Task 9 README bullet, row U4 (U1 gains `[]Ch`, U2 gains `map[string]chan int` and the literal-leaf qualifier), a Behavior-rows note on the U4 path, the `payloadResolves` caveat narrowed to named unresolvable leaves, Impact 2 and 12, open decision 7 clarified and decision 10 added, commit-body paragraph, and pins in `TestPayloadResolutionRows`, `TestCompositePayloadTypeInfo` (`map[string]Fn` carried), `TestRequestlessCompositePayloadKeepsStatuses` (`/fn-map` keeps 404), `TestDoctorClassifiesPayloads` (untyped) and `TestPayloadFieldParity` (four U4 rows). Not added to the `named_payload_fallback` golden, whose "one per-type warning per route" contract U4 would break.
7. **Finding 7 (major), a second `renderShape` would not compile and the plan's notation matched no helper: applied (reuse, not rename).** Verified: `internal/analyzer/shape_test.go:13` is `package analyzer` and renders `*`/`[]`/`[N]`/`map[K]V`/bare name/`$Ref`/`kind:`/`marshal:Name(elem)`/`cycle:`/`unknown`. Applied: Task 3 Step 1 reuses it with a notation key and examples; every asserted expectation in Task 1, Task 3 and Task 8 items 1–6 and 9 is rewritten in that notation (`[]$User`, `[]string`, `$T1`, `[]*User`, `[][]User`, `map[string]User`, `[N]*Address`, `map[string]Cents`/`map[string]int64`, `int64`); prose elsewhere is marked descriptive. Two traps added: a nil Resolution is asserted with `assert.Nil` (`renderShapePtr(nil)` returns `"unknown"`, same as a `ShapeUnknown` leaf), and repeated expected strings count toward goconst (`"kind:" + kindInteger` pattern or test consts).

---

## Self-review: acceptance criteria → steps

- **Parity test over every Desired cell, including the ResultWithMeta, WithRawResponse and kind-only rows**: Task 7 Step 1, with analyzer-level rows in Task 3 Step 1.
- **Resolved rows silent and `--strict` exit 0; each fallback row exactly one warning and failing `--strict` (uintptr included)**: Task 7 Steps 2–3, Task 3 Step 1, and the `named_payload_fallback` golden.
- **`req Cents` unchanged, and the two unresolvable tests unchanged**: Q1 and Task 7 Step 4. The `nonstruct_request` golden and tests stay green.
- **No named non-struct component; `User`/`Address` emitted and referenced; `json_excluded_request`/`empty_request` unmoved; doctor typed/untyped**: Task 7 Step 1 (components), Tasks 4–5, Task 6 Step 3.
- **Goldens only via `-update`, exactly the expected movers, named**: Task 6. This is three movers, not two (stale point 3).
- **New fixture with no MarshalText+UnmarshalText type**: `named_payload_fallback`.
- **Updated deliberately: the two named tests, the docs, the README bullet; doctor's "named-scalar slice element is untyped" keeps passing**: Tasks 7 and 9, and `IsTypedPayload`'s degenerate row.
- **Mutation check (revert payload resolution → parity and doctor fail)**: Task 10, mutation 1.
- **`make check`, gocognit ≤15, ≥80% coverage**: Task 10.
- **Commit type and Impact**: Task 11 and **Impact**.
- **No scope taken from other issues**: a written `**T` root is left to #120 (U3); request-less status recovery is a consequence of #110's own extraction, documented as Impact 12.

---

## Execution notes

- **Branch and baseline:** `fix/payload-resolution` was cut from `fix/qualified-named-types` at `9d2b78a`, with the untracked plan carried over. Baseline `make check` was green, and gocognit `-over 15` reported only `yamlNodeToJSONValue` (16) and `TestMarshalerGuardPositions` (16). #110 still has exactly one comment (the 2026-09-29 brief).
- **Toolchain (environment only, no repo change):** the machine's Homebrew Go moved from go1.27.1 to go1.27.2 during the session. The pinned golangci-lint v2.12.2, built with go1.27.1, then fails `typecheck` on the standard library ("export data version 5 is greater than maximum supported version 4"). The final `make check` therefore ran under `GOTOOLCHAIN=go1.27.1`, the toolchain the baseline used, and gave `0 issues`. `go test -race -count=1 ./...` and the coverage run used the default go1.27.2 and were green.
- **TDD order (deviation):** both Task 1 tests went red on `9d2b78a` before the seam fix. The seam-A test failed with the field warning plus the `/cents` (A) warning, and the field half went green once both seams were fixed. The Task 3–7 tests were written after their implementation. Their red state was established by the Task 10 mutations, every one of which bit (below), not by a pre-implementation run. Task 3 Step 4's state matched the plan exactly: the nine analyzer rows of Task 8, and nothing else, failed before they were updated. Task 4 Step 3 matched as well: only the Task 8 generator rows failed (items 12 and 14).
- **Lint:** the two prototype findings (`found()`'s `hugeParam`, `extractResponseType`'s `unnamedResult`) were avoided while writing the code, so lint never raised them. The only lint finding was a `prealloc` in the new `TestDoctorClassifiesPayloads`, which was fixed.
- **Test helpers (deviation):** `analyzeDirectiveProject` returns no routes, so `payload_test.go` adds `analyzeProjectRoutes` and `routeByPath`. The generator builder is `resolvedPayload(ti *models.TypeInfo) *models.TypeInfo`, which copies the Shape into the Resolution, rather than the plan's suggested `(shape, resolution)` pair. The new generator tests' nameless builder is `resolvedPayloadOf`, because `resolvedTo` already names a field builder. `payloadRowsModule` is reused by `TestPayloadDepthCapAndDeclsDisagree`.
- **Test scope (deviation):**
  - `TestDoctorClassifiesPayloads` drops the plan's `Width` row: `writeProject` writes a single file, so it cannot hold build-tagged declarations. `TestPayloadResolutionRows` and the `named_payloads` golden pin `Width`.
  - `TestRunGeneratePayloadFallbacksFailStrict` drops `b.Tier`, which needs a second package. That leaves 19 rows. `TestPayloadResolutionRows` and the `named_payload_fallback` golden pin `b.Tier`.
  - Additions: `TestCompositePayloadTypeInfo` gains `*[4]byte`, `*map[string]A`, `[]*[]string` and `[]**int64`. A generator `TestStructSlicePayloadSchema` covers both arms of `structSlicePayloadSchema`. `TestPayloadResolutionRows` also covers `*PC`, `[2]Cents`, `[]error` and two C1 controls (`time.Time` and `[]*time.Time` keep the name `Time`).
- **Goldens:** before `-update`, `go test ./internal/spectest` failed on exactly `named_types`, `qualified_named_types` and `slice_result` plus the two new fixtures. The single `-update` run touched only the two new directories, the three `expected.yaml` files (each diff exactly as in **Goldens**) and the three comment-only `.go` edits. Both new goldens were checked cell by cell against P1–P14 and F1–F10. `named_payloads` has components `Address`, `ErrorResponse`, `RawErrorResponse` and `User`; `/map-created` documents 201 and 404 with no 200, and `/bare-map` documents 409. `named_payload_fallback` has `ErrorResponse` only, and its typed maps carry no `description`. Through the CLI, `generate --strict --validate` on `named_payloads` gives `Warnings: 0`. `named_payload_fallback` gives `Warnings: 21`: 20 analyzer warnings plus the aggregate. The new fixture `.go` files were `gofmt -w`'d by hand.
- **Mutations:** each mutation was restored from a scratchpad copy (`cmp`-verified after every run); none used `git checkout`.
  - 1 failed `TestPayloadResolutionRows`, `TestPayloadFieldParity`, `TestGoldenFixtures`, `TestRunGenerateNamedPayloadsFixtureStrictClean` and `TestDoctorClassifiesPayloads`, plus the updated Task 8 tests.
  - 2 failed `TestIsTypedPayload`, `TestDoctorClassifiesPayloads`, `TestRunGenerateNamedPayloadsFixtureStrictClean` and `TestRunGenerateBareReturnMatchesResult`.
  - 3 failed `TestPayloadResolutionRows`, `TestPayloadFieldParity` and `TestGoldenFixtures`, plus the element-pointer rows.
  - 4a failed only `TestHandlerInRouteFileResolvesModuleFileTypes`, and 4b only `TestHandlerInSiblingFileResolvesInItsOwnFile`. Both mutations also restore `for _, file`: reverting the argument alone leaves `path` unused and fails the build, which the first attempt mistook for "no failures".
  - 5 failed `TestSuccessEnvelopeAnnotatesOnlyBareObject` and `TestGoldenFixtures`.
  - 6 failed `TestReferencedSchemaNamesWalksResponseResolution` only.
  - 7 failed `TestPayloadResolutionRows`, `TestRunGeneratePayloadFallbacksFailStrict` and `TestWarnPayloadEveryKind`.
  - 8 failed `TestRequestlessCompositePayloadKeepsStatuses`, `TestPayloadResolutionRows`, `TestGoldenFixtures` and others.
  - 9 failed `TestPayloadResolutionRows`, `TestCompositePayloadTypeInfo` and `TestDoctorClassifiesPayloads`.
- **Gates:**
  - `make check` gave `0 issues`, with every package green and CLI validation passing.
  - gocognit `-over 15` on every changed production file reports nothing. Every touched function is within the budget table: `referencedSchemaNames` 10, `extractHandlerSignature` 8, `registerPayloadRefs` 7, `populateTypeFields` 3, `responsePayloadSchema` 3.
  - Statement coverage is 100% on every new or changed analyzer, generator and doctor function except two. `extractRoutesFromPackage` (92.3%) and `dropUnresolvablePayloadName` (87.5%) miss pre-existing lines this change did not add; the latter's well-known guard is now unreachable but kept as a defensive guard. `TypeInfo.ResolvedShape` is uncovered because `internal/models` is test-free by design, like `FieldInfo.ResolvedShape`.
  - `TestUnresolvablePayloadFallsBackWithWarning` and `TestRunGenerateUnresolvablePayloadFallsBack` are unchanged: no diff hunk touches either.
