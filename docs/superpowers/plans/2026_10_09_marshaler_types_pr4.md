# Marshaler Types, PR 4 of ADR 0002 (#111) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Marshaler type is documented from its **method set**, as Go's selector rules and `encoding/json` see it, instead of from its fields or underlying type. A type that `encoding/json` writes and reads as text on every toolchain (`MarshalText` with no `MarshalJSON`/`MarshalJSONTo`, and `UnmarshalText` with no `UnmarshalJSON`/`UnmarshalJSONFrom`, declared or promoted) becomes `{type: string}` with no `validate` keyword, its `example:` kept as a string. Every other Marshaler type stays `{}` with one warning. A **struct** Marshaler type, whether its methods are its own or promoted through an embedded field, is never registered as a component, never `$ref`'d, and nothing reachable only through it registers either. Parameters keep typing by kind. A jose-tagged struct Marshaler payload keeps its JOSE wire contract (`application/jose`, the JOSE error set) and only stops naming a plaintext component; a struct-typed parameter of a Marshaler request keeps its component; a params-only Marshaler request (at least one field, every one a parameter) is documented exactly as before, while a zero-field one gains a body like any other. Closes #111.

**Architecture:** One new Resolution-only kind, `models.ShapeText`, carries the string outcome. The analyzer's Marshaler guard (`internal/analyzer/marshaler.go`) gains per-method bits on `marshalerMethods` (the string rule), and a method-set walker, `methodSetOf`, that follows alias and defined chains, promotes embedded fields' methods (shallowest depth wins, a same-depth tie cancels, a declared method of any signature shadows), and reads `time.Time`, `uuid.UUID` and `json.RawMessage` from a small well-known table. The resolver consults it in the two places a struct becomes a `ShapeRef` today (`resolveLocal`, `resolveQualified`), through `structMarshalerLeaf`. `registerPayloadType` refuses a Marshaler type (recording its struct's `jose:` tag as `JOSE`, which only registration set before), so a payload falls through to PR 3's `resolvePayload`, and a request reaches a new `populateMarshalerRequest`, which keeps its path/query/header fields and gives the request TypeInfo a Resolution (a params-only, non-JOSE one, with at least one field and every field a parameter, keeps its Name instead and gets no body, as on the base; a zero-field one is not params-only and gets a body). The generator types `ShapeText` as `string`, drops validate keywords on it (`isMarshalerLeaf`), emits a request body from a request Resolution, gives `joseDescription` a nameless variant, and marks the `$ref` targets of a request's own Fields (`addRequestFieldRefs`). No golden on the base moves.

**Tech Stack:** Go 1.25 language floor; `go/ast` static analysis only; goldens in `internal/spectest`.

**Spec (authoritative, in this order):** #111's triage brief (first comment) **as amended by** its 2026-10-03 comment ("text encoding and jsonv2 methods": the string rule needs `MarshalText`, the JSON-specific methods win, and four extra rows). Glossary: `CONTEXT.md` (**Resolution**, **Marshaler type**: already says "promoted from a struct it embeds"). Design: `docs/adr/0002-named-type-resolution.md`. Prior plans' **Execution notes**: `docs/superpowers/plans/2026_10_08_named_type_resolution_pr1.md`, `2026_10_08_qualified_named_types_pr2.md`, `2026_10_09_payload_resolution_pr3.md`; commit bodies `git log -3 fix/payload-resolution`.

**Base:** `fix/marshaler-types`, cut from `fix/payload-resolution` at `533353f` (PR 3, #130, **open, not on `main`**). Every line anchor below is on `533353f` and comes with a `grep -n` recipe; re-run the recipe before editing. Rebasing onto `main` after #130 merges is the orchestrator's job.

**Prototype evidence (planner, 2026-10-09):** every code change in **Appendix A** was applied to a `git archive` copy of `533353f` (`scratchpad/pr4/proto`, outside the repo) and run:
- `go test ./...` passed **with no existing test edited** and **no golden moved** (before and after adding the new fixture, `diff -rq` of `internal/spectest/testdata` against the base lists only `marshaler_types/`).
- `GOTOOLCHAIN=go1.27.1 make lint` (pinned v2.12.2) gave `0 issues` after one fix already folded into Appendix A (gocritic `importShadow`: `marshaler.go` must not import `go/types`, its `matchTypes` has a parameter named `types`).
- gocognit after: the largest touched function is `resolveLocal` at 9 (budget table below); after review round 1 it is `referencedSchemaNames` at 10, unchanged from the base.
- `generate --strict --validate` on a scratch project whose only Marshaler types are string both ways (`MDW`, `Excl`, `Near`, `[]Level`, a text request, plus the non-Marshaler `Node`, `A`, `GenEmb`, `TagEmb`) exits 0 with `Warnings: 0`, which is Task 7's expected outcome.
- The **Today** cells in the behavior tables are a `533353f` binary's output on the fixture of Task 6; the **After** cells are the prototype binary's output on the same project (`generate --validate` clean). Session-local paths (`scratchpad/pr4/...`) are evidence only; executing the plan needs none of them.
- **Review round 1 (2026-10-09), revised prototype `scratchpad/pr4/proto2`** (`proto` plus the three fixes of **Review dispositions**; its `payload.go`, `analyzer.go` and `openapi.go` hunks replace Appendix A's, and its `marshaler.go`/`resolution.go` hunks are byte-identical to the first prototype's):
  - `go test ./...` green with no existing test edited; `diff -rq` of testdata against `proto` was empty before the fixture grew (no golden moved, `marshaler_types` included); `GOTOOLCHAIN=go1.27.1 make lint` gave `0 issues`; gocognit numbers are in the budget table.
  - `generate --validate` passes on all six reviewer projects (`cr_go/e1`, `e2`, `e4`, `rv_get`, `rv_req`, `rv_wrap`). The first prototype failed kin on `e2`/`e4` (dangling `Opaque`/`Filter`); now both keep that component, and `rv_get`'s output is byte-identical to the base.
  - Every JOSE route (`Result[JoseResp]`, `Result[*JoseResp]`, `Result[[]JoseResp]`, a JOSE text request, a JOSE params-only request) keeps `application/jose`, `JOSEErrorEnvelope`, 401 and 415 (`scratchpad/pr4/rev_edge`, diffed against the base binary).
  - Mutation: dropping the `addRequestFieldRefs` call makes `rev_edge` and `e4` fail kin again (`Empty`, `Filter` not found).
  - The grown fixture (Appendix B; rows M16, M17, U5) regenerates **additively** against the first prototype's golden: five inserted blocks, no changed or deleted line, 831 non-empty lines.
- **Review round 2 (2026-10-09), prototype `scratchpad/pr4/proto3`** (`proto2` with only the fixture's `Fields` grown by the twelve positions of **Review dispositions, round 2**; no code hunk changed, so Appendix A stands):
  - `go test ./...` green; the golden regenerates **additively** against `proto2`'s: four inserted blocks inside `Fields`, no changed or deleted line, **859** non-empty lines; every new `*T` is `{}` with no `nullable`, every `[]T` `items: {}`, every `map[string]T` `additionalProperties: {}`, and no component is added.
  - `generate --validate` on the grown fixture: **42** warnings after (41 per-type + the aggregate), **28** on three independently built `533353f` binaries (the twelve fields are `$ref`s there).
  - Warning loss confirmed (`scratchpad/pr4/r2w`): a text struct `MoneyT` reached only as a field, with a malformed-tag field, a `uintptr` field, an undeclared-type field and a `StatusV` field, warns four times on the base and **zero** times after (`--strict` 1 → 0); a text request `Local` with a `param` field and two malformed-tag fields (hiding `validate` and `query`) still prints both tag warnings after, the second reading `query dropped from the spec`. Making `MoneyT` the request as well brings its tag warning back (`scratchpad/pr4/r2w2`, `Warnings: 1`).
- **Review round 3 (2026-10-09), no code change** (Appendix A stands; binaries are the reviewer's `scratchpad/pr4/cr4/{base_bin,after_bin}`, built by the reviewer from `533353f` and `533353f` + Appendix A; independently checked only that `cr4/src`'s `marshaler.go`, `resolution.go`, `payload.go`, `analyzer.go` and `openapi.go` are byte-identical to `proto3`'s; probes in `scratchpad/pr4/rv6`):
  - Zero-field struct Marshaler requests (`rv6/zf`): `GET /ping`, request `Ping struct{}` with pointer `UnmarshalJSON`, gains `requestBody {schema: {}}` and one `request type Ping …` warning; `generate --strict` goes 0 → 1, and doctor prints the warning and "Ready with caveats" (the route stays typed there only because its response is). `POST /blank`, request `Blank struct{}` with the text pair, gains `requestBody {type: string}` with no warning. A plain `Plain struct{}` request is unchanged. The base emits no body for any of the three. `rv5probe`'s `Hidden{ID param; secret string}` (one counted field, a parameter) stays byte-identical to the base.
  - Short-name hand-off (`cr4/p1`): `a.Money` (value `MarshalJSON`) and a plain `b.Money{Cents}`, both reached from `Out{A a.Money; B b.Money}` and `Result[b.Money]`. The base names them `Money` (`amount`) and `BMoney` (`cents`). After, `b.Money` takes `Money` (now `cents`), `BMoney` is gone, `a` is `{}`, and the only warning is `a.Money`'s field warning.

---

## Scope re-derived against what PR 1–3 shipped (checked at `533353f`)

Already shipped, **do not redo**:
- The exact-signature guard for all seven methods (`marshalerSignatures`, `marshaler.go:50`), on `T` or `*T`, through alias chains (resolver recursion), never through a defined type (`type LD Level` resolves in underlying mode).
- `ShapeMarshaler` → `{}` plus one warning per field declaration and per payload; param exemption (`resolveState.param`); the byte-slice rule (`marshalerLeaf`, decode-only byte kind in a slice stays base64); validate keywords dropped on a Marshaler leaf and its `dive` element (`applyValidationConstraints` via `isMarshalerLeaf`, `constraints.go:245–268`); `example:` kept (coerceExample's `""` arm keeps a string on `{}`).
- Payload resolution (PR 3): a payload that does not register goes through `resolvePayload`, warns once, and is classified by `IsTypedPayload`. So the † rows (`Result[Level]`, `Result[[]Level]`) apply here, and so do the ‡ rows (`domain.Tag`, PR 2).

Not shipped, verified in code, **this PR**:
1. `resolveLocal` returns `ShapeRef` for any chain ending at a struct **before** the Marshaler check (`resolution.go:388` vs `401`); `resolveQualified` likewise (`290`). Struct Marshaler types still register and `$ref`.
2. `registerPayloadType` (`analyzer.go:3193`) never consults methods, so `Result[Money]` and request `CustomBody` register as structs.
3. `marshalerMethods` is two bools (`marshaler.go:25`); the string rule needs per-method presence.
4. No promotion, no well-known method table.

Out of scope (brief): #117 (override), #113 (`*big.Int`, `net.IP`, `netip.*`), third-party types (`decimal.Decimal` stays PR 1's fallback, and an embedded one goes undetected), #114 (generics: a `Box[T]` embed contributes no methods), #102 (non-struct requests: a non-struct Marshaler request keeps #127's "not a struct" warning), #116 (dropped non-Marshaler embeds: `Pair`'s fields stay dropped; **`embeddedFields` is not touched**), #115 (`json:",string"`), #123 (sibling-file import bug, footnote ¹).

---

## Global Constraints

- **Worktree:** `/Users/gaborage/Projects/gaborage/code/go-bricks-openapi-pr4` only. Never touch the other `go-bricks-openapi*` checkouts. Scratch projects/binaries under `/private/tmp/claude-501/-Users-gaborage-Projects-gaborage-code-go-bricks-openapi/7fea5cb8-6b07-4be5-b097-b746a0737ef9/scratchpad/pr4/` only. Never push, open PRs or touch GitHub state. Read issues only with `GH_TOKEN=$(gh auth token -u gaborage) gh issue view <N> --repo gaborage/go-bricks-openapi --comments`; never `gh auth switch`.
- **Toolchain (environment, not code):** local Go is 1.27.2, whose export data the pinned golangci-lint v2.12.2 cannot read. Run `GOTOOLCHAIN=go1.27.1 make check` / `make lint`; `go test` may use the default toolchain.
- **Goldens:** regenerate only with `go test ./internal/spectest -update` (package-scoped; never `make update`; the test is `TestGoldenFixtures`). After `-update`, `git status --porcelain internal/spectest/testdata` may list **only** the new `marshaler_types/` directory. Any other moved file is a bug: fix the code, don't accept the diff. Reason no existing golden moves: no fixture on the base declares a string-both-ways type (every Marshaler type in `named_resolution`, `named_fallback`, `named_payload_fallback`, `qualified_named_types` has one side only, a JSON method, or a wrong signature), none declares a struct Marshaler type, and none embeds a Marshaler type, `time.Time`, `uuid.UUID` or `json.RawMessage` (checked by grep and confirmed by both prototype runs).
- **Cognitive complexity ≤15** per function (SonarCloud `go:S3776`, server-side; `make lint` checks only cyclomatic). Check: `go run github.com/uudashr/gocognit/cmd/gocognit@latest -over 15 <files>`. Baseline over 15: only `yamlNodeToJSONValue` (16, untouched) and `TestMarshalerGuardPositions` (16, test, **do not grow it**: add sibling tests). The budget table is binding.
- **Coverage ≥80% on new code** (`cmd/`, `internal/`). A package's coverage comes only from its own tests; spectest/commands tests cover no analyzer or generator line. Every new branch needs an in-package unit test. `internal/models` stays test-free (its change is one constant).
- **goconst** counts `_test.go` literals (min-len 4, min-occurrences 3): new warning texts are `const`s; tests compare against `fmt.Sprintf(<const>, …)`, not new raw copies of production literals. Type names in test sources (`"Money"`, `"Level"`) are test-only literals; goconst counts across the whole **package** (test files included, the PR #58 lesson), so if a name repeats ≥3 times in the package, hoist it into a test `const` (PR 1 hit this with `complex64`).
- **exhaustive** (`default-signifies-exhaustive: true`): every new `switch` on a kind has a `default:` arm (Appendix A has them).
- **gocritic** (all tags): `marshaler.go` must not import `go/types` (`importShadow`, reproduced). Keep `embedMethods`/`methodEntry` passed by value as in Appendix A (lint clean at those sizes).
- **No `t.Parallel()`.** Go 1.25 floor (`slices.Clip`, `slices.Contains` are fine).
- **Settled invariants (CLAUDE.md), untouched:** `lookupStructTag`/`unquoteLiteral` (not read here); the Constraint set stays in `constraints.go` (only `isMarshalerLeaf` there grows one kind); example coercion stays only in `fieldInfoToProperty` (`buildFieldProperty` untouched); `referencedSchemaNames` still never marks a non-JOSE request's **own** name. It now walks every request's **Fields** for `$ref` leaves (`addRequestFieldRefs`). For a registered request that repeats exactly what `addFieldSchemaRefs` marks (`adoptRegisteredType` copies the registry entry's Fields). For a struct Marshaler request, which is not in the registry, it marks its parameters' struct targets, which nothing else would. The invariant's rationale (no orphan component for a params-only request type) holds because the request's own name is never marked; its doc comment's "reachable only when `len(bodyFields) > 0`" paragraph gains this case (Task 8); the `schema == nil` check, `isParamsOnlyType` and the uint `minimum: 0` pre-stamp are untouched; no `NOSONAR`/`//nolint` added.
- **One commit** (squash). Body stands alone (Task 10). The only closing keyword anywhere is the `Closes #111` trailer; every other issue is cited as `Refs #N` or "#N should …", never after close/fix/resolve wording.

---

## Behavior rows: expected emitted YAML (fixture `marshaler_types`, Task 6)

Field rows are properties of the `Fields` component (`GET /fields` response, `POST /fields` request body: the same component, so every field row holds in a request body too). "warn" = exactly one warning per field declaration (deduped by position) or per payload.

### String both ways (no warning, typed)

| # | Field / payload | Today | After |
|---|---|---|---|
| S1 | `Level Level validate:"required,min=1,max=3" example:"high"` | `{example: high}` + warn | `{type: string, example: high}`, `level` in `required` |
| S2 | `LevelPtr *Level` | `{}` + warn | `{type: string, nullable: true}` |
| S3 | `LevelA LA` (`LA = Level`) | `{}` + warn | `{type: string}` |
| S4 | `LevelP LevelP` (pair on `*T` only) | `{}` + warn | `{type: string}` **(superseded: `{}` + warn; see post-execution review)** |
| S5 | `LevelOne Level validate:"oneof=1 2 3" example:"2"` | `{example: "2"}` + warn | `{type: string, example: "2"}` (no `enum`) |
| S6 | `Levels []Level validate:"min=1,dive,max=3"` | `{type: array, items: {}, minItems: 1}` + warn | `{type: array, items: {type: string}, minItems: 1}` |
| S7 | `Grid [][]Level` | items `{type: array, items: {}}` + warn | `{type: array, items: {type: array, items: {type: string}}}` |
| S8 | `LevelMap map[string]Level`; `LevelPMap map[string]LevelP` | `additionalProperties: {}` + warn | `{type: object, additionalProperties: {type: string}}` **(superseded for `LevelPMap`: `additionalProperties: {}` + warn; see post-execution review)** |
| S9 | `Flags []FlagT2` (byte kind, text pair) | `items: {}` + warn | `{type: array, items: {type: string}}` (never base64) |
| S10 | `Code Code validate:"min=2,max=5,email"` (string kind) | `{}` + warn | `{type: string}` (no `format`, `minLength`, `maxLength`) |
| S11 | `TextApp` (`MarshalText`+`AppendText`+`UnmarshalText`) | `{}` + warn | `{type: string}` |
| S12 | `MoneyT`, `*MoneyT`, `[]MoneyT` (struct, text pair) | `$ref MoneyT`; `allOf`+`nullable`; items `$ref` | `{type: string}`; `{type: string, nullable: true}`; `items: {type: string}`; **no `MoneyT` component** |
| S13 | `Amount domain.Amount` (struct in `domain`, text pair) | `$ref Amount` | `{type: string}`; no component |
| S14 | `EmbP{LevelP; N}` | `$ref EmbP` (`{n}`) | `{type: string}`; no component **(superseded: `{}` + warn, no component; `PtrEmbP{*LevelP; N}` is the string row; see post-execution review)** |
| S15 | `WrapLevel{Level; Note}`, `PtrEmb{*Level; Note}`, `TaggedEmb{Level json:"lvl"; Note}`, `Deep{WrapLevel; X}`, `WithID{uuid.UUID; Note}`, `WithCode{domain.Tag; X}` | `$ref` to components `{note}`, `{note}`, `{lvl: {}, note}` (+ a warning on `lvl`), `{note, x}`, `{note}`, `{x}` | each `{type: string}` (no `format` on `WithID`); none registered |
| S16 | `Tag domain.Tag` (‡) | `{}` + warn | `{type: string}` |
| S17 | `GET /money-t` `Result[MoneyT]`; `/wrap-level` `Result[WrapLevel]` | `data: $ref` | `data: {type: string}`; typed |
| S18 | † `/level` `Result[Level]`; `/levels` `Result[[]Level]` | `data: {}` / `items: {}` + warn; both counted in the aggregate | `data: {type: string}` / `{type: array, items: {type: string}}`; typed; no warning |

### Other Marshaler types: `{}` plus one warning, no component

| # | Field / payload | Today | After |
|---|---|---|---|
| M1 | `LD LD` (`type LD Level`) | `{type: integer, format: int64}` | unchanged |
| M2 | `StatusV` (MarshalText only), `LevelU` (UnmarshalText only), `Both` (pair + MarshalJSON), `TextJ` (pair + UnmarshalJSON) | `{}` + warn | unchanged (baseline) |
| M3 | amendment: `AppOnly` (AppendText + UnmarshalText), `ToText` (MarshalJSONTo + pair), `FromOnly` (UnmarshalJSONFrom only) | `{}` + warn | unchanged: `AppOnly` warns `AppendText`, `ToText` warns `MarshalJSONTo`, `FromOnly` warns `UnmarshalJSONFrom` |
| M4 | `Money`, `*Money`, `[]Money`, `map[string]Money` (value MarshalJSON) | `$ref`; `allOf`+`nullable`; items/values `$ref` | `{}`; `{}` (**no `nullable`**); `items: {}`; `additionalProperties: {}`; each warns `Money has its own MarshalJSON method` |
| M5 | `MoneyP` (pointer MarshalJSON), `MoneyU` (UnmarshalJSON only), each as `T`, `*T`, `[]T`, `map[string]T` | `$ref`; `allOf`+`nullable`; items/values `$ref` | as M4: `{}`; `{}` (**no `nullable`**, the pointer-receiver case included); `items: {}`; `additionalProperties: {}`; each warns `MoneyP has its own MarshalJSON method` / `MoneyU has its own UnmarshalJSON method` |
| M6 | `MA` (`MA = Money`) as `MA`, `*MA`, `[]MA`, `map[string]MA` | `$ref MA` (own component); `allOf`+`nullable`; items/values `$ref MA` | as M4; each warns `… has type <as written>: Money has its own MarshalJSON method …` |
| M7 | `domain.Price` as `T`, `*T`, `[]T`, `map[string]T` | `$ref Price`; `allOf`+`nullable`; items/values `$ref Price` | as M4; each warns `domain.Price has its own MarshalJSON method` |
| M8 | `Wrapper{StatusV; Name}` | `$ref` `{name}` | `{}`, warns `Wrapper has the MarshalText method promoted from its embedded StatusV` |
| M9 | `Stamped{time.Time; Note}` | `$ref` `{note}` | `{}`, warns `Stamped has the MarshalJSON method promoted from its embedded time.Time` |
| M10 | `Twin{Level; StatusV}` | `$ref` (empty) | `{}`, warns `Twin has the UnmarshalText method promoted from its embedded Level` (the two `MarshalText` cancel) |
| M11 | `Shadow{Level; Note}` + own `MarshalJSON` | `$ref` `{note}` | `{}`, warns `Shadow has its own MarshalJSON method` |
| M12 | `WithInner MoneyWithInner` (own MarshalJSON; field `Inner OnlyInner`) | `$ref`; `OnlyInner` registered | `{}` + warn; **neither** component exists |
| M13 | `Raw{json.RawMessage; N}` | (row added by this plan) | `{}`, warns `Raw has the MarshalJSON method promoted from its embedded json.RawMessage` |
| M14 | `/money` `Result[Money]`, `/money-ptr` `Result[*Money]`, `/moneys` `Result[[]Money]`, `/stamped` `Result[Stamped]` | `data: $ref` (array of `$ref`) | `data: {}` / `{type: array, items: {}}`; one `response type …` warning each; the four routes are untyped (aggregate warning names them); no `Money`/`Stamped` component |
| M15 | `POST /custom/:id`, request `CustomBody` (pointer `UnmarshalJSON`, `ID string param:"id"`, `Name string json:"name"`) | body `$ref CustomBody {name}`; `id` path param | `requestBody: {required: true, content: {application/json: {schema: {}}}}`; `id` stays `{name: id, in: path, required: true, schema: {type: string}}`; no component; warns `request type CustomBody: CustomBody has its own UnmarshalJSON method …`; doctor counts the request untyped |
| M16 | `POST /cmd`, request `CmdBody{F Filter query:"f"; E Empty query:"e"; Name string json:"name"}` + pointer `UnmarshalJSON` (`Filter{A string query:"a"}` is params-only, `Empty{}` has no property) | body `$ref CmdBody`; parameters `$ref Filter`, `$ref Empty`; components `CmdBody`, `Filter`, `Empty` | body `{}`; parameters **keep** `$ref Filter` and `$ref Empty`, and `Filter: {type: object}`, `Empty: {type: object}` stay (marked by `addRequestFieldRefs`); no `CmdBody`; one request warning |
| M17 | `POST /jose`, request `JoseReq{_ struct{} jose:"decrypt=k1,verify=k2"; Name}` + pointer `UnmarshalJSON`, response `Result[JoseResp]` (`_ struct{} jose:"sign=k1,encrypt=k2"`, value `MarshalJSON`) | `application/jose` both ways, descriptions name `JoseReq`/`JoseResp`; 400/500 `JOSEErrorEnvelope`, 401, 415; components `JoseReq`, `JoseResp` | **still** `application/jose` `{type: string, format: jose}` both ways, `JOSEErrorEnvelope`, 401, 415; the request description is `joseUndocumentedDescription` (names no component); the response description names `SuccessResponse` (the existing nameless-JOSE fallback, emitted by `createStandardSchemas`); no `JoseReq`/`JoseResp`; one request and one response warning; route untyped (`jose` in the aggregate) |

### Unchanged (not Marshaler types, or parameters)

| # | Row | Both |
|---|---|---|
| U1 | `MoneyOdd` (`func (MoneyOdd) MarshalJSON() string`) | `$ref MoneyOdd {amount}` |
| U2 | `MD MD` (`type MD Money`: declared methods dropped, nothing promoted) | `$ref MD {amount}` |
| U3 | `Pair{Level; Code}` (every method cancels) | `$ref Pair`, `{type: object}` (fields dropped: #116) |
| U4 | `GET /params` request `Params{Level Level query:"level"; HLevel Level header:"X-Level"}` | `{type: integer, format: int64}` for both parameters; no warning |
| U5 | `GET /items/:id` request `ListReq{Defaults; Page int query:"page"; ID string param:"id"}`, `Defaults{}` with pointer `UnmarshalJSON` (a params-only struct Marshaler request) | parameters `page`, `id`; **no** `requestBody`; no warning; no component; doctor typed |

Final components of the golden: exactly `Empty`, `ErrorResponse`, `Fields`, `Filter`, `JOSEErrorEnvelope`, `MD`, `MoneyOdd`, `Pair`, `SuccessResponse`. `generate --validate` on it: 42 warnings (41 per-type + the aggregate naming `money, moneyPtr, moneys, stamped, jose`). The base binary on the same fixture prints 28 (aggregate `level, levels`).

### Extra rows pinned by unit tests only (prototype-verified on a scratch project)

| Row | After |
|---|---|
| `type MDW WrapLevel` (defined over a promoting struct) | `{type: string}`: a defined type drops its target's **declared** methods but keeps what the struct's embedded fields promote |
| `Excl{Level json:"-"; N}` | `{type: string}`: method promotion ignores json tags, `json:"-"` included |
| `OddShadow{Level; N}` + own `MarshalText() string` | `{}`, warns `OddShadow has the UnmarshalText method promoted from its embedded Level`: the wrong-signature own `MarshalText` shadows the promoted one |
| `Near{StatusV; WrapLevel}` | `{type: string}`: `MarshalText` from `StatusV` (depth 1) beats `WrapLevel`'s (depth 2); `UnmarshalText` comes from depth 2 |
| `StampA{TA}`, `type TA = time.Time` | `{}`, warns `… promoted from its embedded TA` |
| `Node{*Node; Name}`; `A{*B; X}`, `B{*A; Y}` | terminate; `$ref` as today |
| `GenEmb{Box[int]; N}` | `$ref` as today (a generic embed contributes nothing, #114) |
| `TagEmb{Tags; N}`, `type Tags []string` | `$ref` as today (a named slice embed has no methods; covers `rhsMethodSet`'s `default:` arm) |
| request `MoneyT` (struct, text pair) | `requestBody` schema `{type: string}`; no warning; doctor typed |
| `Q{M MoneyT query:"m"}` | parameter `m` keeps `$ref MoneyT` and its component (params are exempt; unchanged) |
| `Result[*JoseResp]`, `Result[[]JoseResp]` | `JOSE` kept (the gate keys on `Name`/`Package`, which a pointer or slice payload keeps); `application/jose`, description names `SuccessResponse` |
| request `JoseText` (jose sentinel, text pair, no exported field) | `application/jose`, `joseUndocumentedDescription`; no warning; doctor typed |
| request `JoseParams{_ jose; ID string param:"id"}` + `UnmarshalJSON` (params-only **and** JOSE) | body emitted (`application/jose`, nameless description), `id` parameter, one request warning: the params-only arm skips JOSE, as `isParamsOnlyType` does |
| request `TextParams{ID string param:"id"}` with the text pair (params-only, text) | as on the base: `id` parameter, no body, no warning, Name kept |
| request `Ping struct{}` + pointer `UnmarshalJSON`, on a GET (zero fields, not params-only) | Today: no body, no warning, request typed. After: `requestBody {required: true, content: {application/json: {schema: {}}}}`, one `request type Ping …` warning, `Name ""`, request untyped (`IsTypedPayload` false), `generate --strict` fails (review round 3, finding 1) |
| request `Blank struct{}` with the text pair (zero fields, text) | Today: no body, no warning, typed. After: `requestBody` schema `{type: string}`, no warning, typed |
| request `Hidden{ID string param:"id"; secret string}` + `UnmarshalJSON` (an unexported field is not extracted, so it is params-only) | as on the base: `id` parameter, no body, no warning, Name kept |
| `a.Money` (struct, value `MarshalJSON`) and a plain `b.Money{Cents}` in another package, both reached | Today: components `Money` (`a`'s) and `BMoney` (`b`'s). After: `b.Money` registers as `Money` (`schemaKey`'s short name is no longer claimed by `a.Money`), `BMoney` is gone, `$ref`s to it move (review round 3, finding 2) |

---

## Data model (`internal/models/models.go`)

- Add, after `ShapeRecursive` (`grep -n 'ShapeRecursive ShapeKind' internal/models/models.go`, 220):

  ```go
  // ShapeText is a Marshaler type encoding/json writes and reads as a JSON
  // string on every toolchain (#111): MarshalText with no MarshalJSON or
  // MarshalJSONTo, and UnmarshalText with no UnmarshalJSON or
  // UnmarshalJSONFrom, declared or promoted. Name is the type. It emits
  // {type: string} and takes no validate keyword.
  ShapeText ShapeKind = "text"
  ```
- Update `TypeShape`'s doc ("plus the four Resolution-only kinds") to five, listing `ShapeText`.
- `ShapeMarshaler`'s comment (214–217, "kept for #111"): Elem is the quietly resolved underlying form of a **non-struct** Marshaler type, used only for the byte-slice decision; a struct Marshaler leaf has no Elem; nothing emits or registers under it. Field stays (open decision 4).
- `TypeInfo.Resolution` doc (132–140): "nil for a request" becomes "nil for a request, except one whose type is a struct Marshaler type (#111) with a body (not params-only, or JOSE): its Resolution is its text or Marshaler leaf, its Name is cleared and its Fields are only its parameters. A params-only (at least one field, every one a parameter), non-JOSE one keeps its Name and parameter Fields and has no Resolution; a zero-field one is not params-only". `TypeInfo.JOSE`'s doc: also set for a struct Marshaler payload whose struct carries a `jose:` tag, although it is not registered. `TypeInfo.Shape`'s doc is unchanged (requests still carry no Shape).

No new field: `ShapeText` reuses `Kind`/`Name`. A distinct kind (not `ShapePrimitive{string}`) is required, because `constraintsFor` would apply string keywords to a primitive string and the brief drops every `validate` keyword at the Marshaler leaf.

---

## Analyzer design, with every helper's degenerate input checked

### Helpers reused as is (checked at `533353f`)

| Helper | Degenerate input → behavior relied on |
|---|---|
| `marshalerCandidate` (`marshaler.go:95`) | non-FuncDecl, plain function, generic receiver (`Box[T]` is an IndexExpr), unknown name → nil |
| `isMethodOnStruct` (`analyzer.go:628`) | nil/empty `Recv` → false; matches `T` and `*T` only |
| `samePackageFiles` (`resolution.go:619`) | unreadable dir → just the given file; other-clause files skipped |
| `resolveLocalTypeSpec` (`analyzer.go:3943`) | name with no declaration (including a builtin `int`) → ok=false; returns the **first** declaration only (build-tagged twins: methods are still unioned by `declaredMethods`, which scans all files) |
| `inModuleTypeSite` (`resolution.go:314`) | undotted, stdlib/third-party, unknown qualifier, unreadable dir, undeclared → ok=false |
| `resolveQualifiedStruct` (`analyzer.go:3526`) | non-struct or alias in the target package → ok=false (so `q.UA` aliases stay on the non-struct path, as today) |
| `resolveTypeSpecChain` (`analyzer.go:3406`) | follows `type X = Y`, `type X Y` and `type X q.T` (alias **or** defined) to a struct; >8 hops or non-struct → false |
| `typeShape` (`analyzer.go:4208`) | selector with a non-ident X, generics, func → `ShapeUnknown` (Name "") |
| `c.display` (`resolution.go:164`) | bare in the field's own package, `pkg.T` elsewhere |
| `extractStructFields` (`analyzer.go:3982`) | registers nothing itself; tag warnings fire at extraction (same as registering would) |
| `warnResolvedField` / `registerFieldRefAt` | key off `a.fieldSites[f.Resolution]`; a copied `FieldInfo` keeps the pointer |
| go/parser | `StructType.Fields` is never nil (so `promotedMethods` needs no nil guard; a hand-built AST is not an input) |

### The string rule (`marshaler.go`)

`marshalerMethods` gains `has methodBits` (one bit per method; `methodSignature` gains `bit`, filled in the one table). `with()` ORs the bit. `textBothWays()`: `MarshalText` and `UnmarshalText` set, and none of `MarshalJSON|MarshalJSONTo|UnmarshalJSON|UnmarshalJSONFrom`. `AppendText` neither helps nor hurts (amendment). PR 1's `marshalerSet` keeps its scan order and `first` semantics (`methodsInFile` now delegates the loop to `visitMarshalerMethods`, shared with `declaredMethods`); existing `TestMarshalerSet*` tests assert fields, not struct equality, so the new field breaks none.

Non-struct path: `marshalerLeaf` (`resolution.go:415`) returns `ShapeText{Name: name}`, unnoted, **before** its quiet underlying run when `m.textBothWays()`. Equivalent to putting it after the byte-elem rule, since a text type always has an encode side (`[]FlagT2` → string items, never base64).

### Method sets with promotion (`marshaler.go`, new)

- `methodEntry{depth, exact, ambiguous, owner, via}` and `methodSet map[string]methodEntry`, holding only the seven names.
- `declaredMethods(name, c)`: every method of `name`/`*name` under one of the seven names in every file of c's package, **exact or not**: a wrong-signature declared method is in the method set, so it shadows a promoted one (`OddShadow`), and doesn't count itself (`MoneyOdd`). `owner` = `c.display(name)`.
- `methodSetOf(name, c, keep, open)`: own (declared, when `keep`) `over` the right-hand side's set. `keep` passes on only through an alias (`ts.Assign.IsValid()`); a defined type's target contributes only what its struct promotes. `open` (typeKeys, cloned on append with `slices.Clip`) cuts a revisited type (`S{*S}`, mutual `A{*B}`/`B{*A}`) and caps the walk at `maxNamedResolutionDepth` (9).
- `rhsMethodSet`: struct literal → `promotedMethods`; ident → `methodSetOf`; selector → `qualifiedMethodSet(a.typeShape(t).Name, …)`; anything else → empty.
- `qualifiedMethodSet`: in-module (`inModuleTypeSite`) → `methodSetOf` in the declaring package; else, only when `keep`, `wellKnownMethodSets[qualified]` (`time.Time`: MarshalJSON, MarshalText, UnmarshalJSON, UnmarshalText; `uuid.UUID`: the text pair; `json.RawMessage`: the JSON pair), keyed by the `models.WellKnown*` constants and matched by written name, like every other well-known lookup (an aliased `t "time"` import is not recognised). `models.WellKnownTypeNames` and the generator's `wellKnownFormats` are **not touched** (`TestWellKnownFormatsMatchModels` stays green). Used directly, those types keep their well-known schema: the table is read only through promotion.
- `promotedMethods(st, c, open)`: one `embedMethodSet` per field with no names, **whatever its json tag** (Go's method sets ignore tags), then `promote`.
- `embedMethodSet`: `typeShape(f.Type)`, one pointer shed (pointer-receiver methods count in every position, per the brief and ADR 0002); a non-named embed (generic, builtin `int`, `error`) contributes nothing; `via` = the embed's type as written (`StatusV`, `time.Time`, `domain.Tag`, `TA`).
- `promote(embeds)`: per name, candidate depth = inner depth + 1; the strictly shallowest wins; a tie at the shallowest depth becomes `{ambiguous: true}` at that depth, which also hides deeper candidates (Go's rule) and propagates outward as ambiguous.
- `methodSet.marshaler(typeName)`: reads entries in the fixed `marshalerMethodOrder` (MarshalJSON, MarshalJSONTo, MarshalText, AppendText, UnmarshalJSON, UnmarshalJSONFrom, UnmarshalText); only `exact && !ambiguous` entries count; returns the `marshalerMethods` and the fallback of the first counted entry: depth 0 → `fallbackMarshaler{typeName: owner}` (so `MA` names `Money`, as PR 1 names `Level` for `LA`); depth > 0 → new `fallbackMarshalerPromoted{typeName: the struct, detail: method, via}`.

Not modelled (residuals, recorded in the commit body, not README): a struct **field** named like one of the seven methods also shadows a promoted method in Go; an embedded **interface** (`json.Marshaler`, or a local interface) promotes its methods. Both are absent from every row and rare.

### Resolver call sites (`resolution.go`)

- `resolveLocal` (386, `grep -n 'func (a \*ProjectAnalyzer) resolveLocal' internal/analyzer/resolution.go`): inside the `resolveTypeSpecChain` ok branch (388), before `c.recordRef(name)`, `if r, isMarshaler := a.structMarshalerLeaf(name, c, mode); isMarshaler { return r }`.
- `resolveQualified` (289): `if q, ok := a.resolveQualifiedStruct(...)` (was `_`), then `a.structMarshalerLeaf(q.typeName, c.in(q.file, q.filePath), identityMode)` before `recordRef`: methods are read in the struct's own package, display `domain.Price`.
- New `structMarshalerLeaf(name, c, mode)`: param → not a Marshaler (stays a ref, as today); otherwise `methodSetOf(name, c, mode == identityMode, nil).marshaler(c.display(name))`; none → stays a ref; text → `ShapeText{Name}` unnoted; else note the fallback and return `ShapeMarshaler{Name}` with **no Elem**. Because the leaf is not a `ShapeRef`, `registerResolutionRefs`/`registerPayloadRefs`/`addRefNames` never see the struct, and its fields are never extracted, so nothing reachable only through it registers (`OnlyInner`), and **no warning those fields raised on the base fires** (malformed tag, `uintptr`, unresolvable type, Marshaler field, embed promotion): Impact item 7.
- Underlying mode reaches the struct branch only through a qualified alias chain (`type D q.UA`); `keep=false` there drops the chain's declared methods and keeps the promoted ones, matching Go.
- Warnings: `fallbackMarshalerPromoted` (new `fallbackKind`, appended after `fallbackUntypedBuiltin` at 68), `fieldFallback.via` (new field), `marshalerPromotedFieldWarning` const (after `unknownFieldPosition`, 48), one new `case` in `warnFieldFallback` (689). Existing Marshaler warning text is unchanged byte for byte (struct own-method rows reuse `marshalerFieldWarning`, "instead of its underlying type": open decision 3).
- `quiet()`'s comment (177–188): drop "The discarded ref sites matter once a consumer descends into a Marshaler's Elem (#111)"; nothing descends.

### Payloads and requests

- `payload.go`: `marshalerPromotedPayloadWarning`, `marshalerRequestWarning`, `marshalerPromotedRequestWarning` consts; one new `case` in `warnPayload` (90); `isFallbackLeaf` (163): `ShapeText` joins `ShapeRef, ShapeKindOnly` → not a fallback (its `default:` arm returns true, so forgetting this makes `Result[Level]` untyped). New helpers (Appendix A): `namedShape(ti, file)` (bare when `Package` is "" or the file's own package, else `pkg.Name`, i.e. `registerPayloadType`'s rule), `marshalerNamed(ti, file, path)` (Name "" → false; else resolve `namedShape` in a fresh `newResolveCtx(…, false)` and report a `ShapeMarshaler`/`ShapeText` root, with the noted fallback), `structSite`, `payloadStructSite` (renamed from the first prototype's `requestStructSite`: responses use it too), `structMarshalerJOSE`, `populateMarshalerRequest`, `requestParamFields`, `warnMarshalerRequest`.
- `analyzer.go`, `registerPayloadType` (3193): first statement `if _, _, isMarshaler := a.marshalerNamed(typeInfo, astFile, filePath); isMarshaler { typeInfo.JOSE = a.structMarshalerJOSE(typeInfo, astFile, filePath); return nil }`. For a non-struct Marshaler name this returns what `registerType` returned anyway (nil), and `JOSE` stays false (no struct site). `structMarshalerJOSE` is `payloadStructSite` plus the existing `hasJOSETag` (the analyzer's one jose detector, mirroring go-bricks' `jose.ScanType`), so `JOSE` is exactly what `registerType` (analyzer.go:3463) would have recorded. It is set at the gate because requests and responses both pass through it, and because registration (3211/3463) was the only place that set `JOSE`. It is keyed on `Name`/`Package` like `registerType`, so `Result[*JoseResp]` and `Result[[]JoseResp]` keep it (prototype-verified). Responses then reach PR 3's `resolvePayload` unchanged: root `ShapeMarshaler`/`ShapeText` passes `payloadResolves`, the name is cleared (not well-known), `warnPayload` warns from the note. A colliding project struct `uuid.UUID` with a text pair keeps `Name` (well-known spelling) but `payloadNamesComponent` returns false on its Resolution: no orphan.
- `populateRequestType` (3173): after the `registerPayloadType` branch, `if a.populateMarshalerRequest(h, astFile, filePath) { return }`, before the non-struct warning.
- `populateMarshalerRequest` (**order differs from the first prototype, on purpose**; Appendix A has the final order): `marshalerNamed` first (not a Marshaler → false: unchanged path), then `payloadStructSite` (not a struct, e.g. request `Level` → false: #127's "not a struct" warning, unchanged). In the first prototype the two checks ran the other way round, which left the `!isMarshaler` branch unreachable (a non-Marshaler struct always registers first) and uncovered. Then `params, paramsOnly := requestParamFields(site, Name)` and `Fields = params`.
  - **If `paramsOnly && !ti.JOSE`, return true here**, with Name kept, no Resolution and no warning (finding 4). A request every one of whose fields is a parameter (at least one field, `isParamsOnlyType`'s sense) got no body on the base and was typed in doctor; it keeps both. `isParamsOnlyType` itself is not consulted, because the request is not in the registry. Not gated on "has any non-param field", so a JOSE params-only request keeps its `application/jose` body.
  - **Zero-field requests are not params-only (review round 3, finding 1; kept on purpose).** `requestParamFields` counts only the fields `extractStructFields` returns (an unexported field is not among them; `rv5probe`'s `Hidden` counts one), so `Ping struct{}`, or a struct whose only fields are unexported, has `total == 0` and takes the body arm. On the base it had **no** body (`bodyFields == 0`, not JOSE), no warning, and was typed. After, a non-text one gets `requestBody {schema: {}}` (also on a GET), one `request type …` warning, `Name ""`, an untyped request in doctor and a `generate --strict` failure; a text one (`Blank struct{}` with the pair) gets a `{type: string}` body with no warning and stays typed. This is the brief's rule (a struct with `UnmarshalJSON` is `{}`): a zero-field struct that declares `UnmarshalJSON` or the text pair does read the body through it, while round 1's carve-out rests on every field being a parameter, which a zero-field struct cannot show. It changes a declared SemVer surface (doctor, `--strict`), so it is stated in Impact item 5, the README, the commit body and the PR `## Impact`, and pinned by Task 4's zero-field rows, Task 7's strict test and mutation 13. Base parity instead is open decision 6. (The round-1 text of this bullet said a zero-field text request "keeps its string body"; the base had no body there, so it gains one.)
  - Otherwise `Name = ""`, `Resolution = &leaf`, `warnMarshalerRequest(h.requestType, fb)`. `JOSE` is already set by the gate.
- `requestParamFields`: `extractStructFields(st, pkg, file, path, {name}, 0)`, keep `ParamType != ""` fields, then the two loops `registerStructKeyed` runs (warn each, then `registerFieldRefAt(…, depth 1)`), so a param's own warning and refs behave as they did when the struct registered (`Q{M MoneyT query:"m"}` keeps `$ref MoneyT`). Returns `(params, total > 0 && len(params) == total)`. Registering a parameter's struct is not enough on its own: `generateSchemasFromTypes` drops a params-only or zero-property struct that nothing marks, so the generator's `addRequestFieldRefs` marks it (findings 1 and 5). **Accepted residual (review round 2):** `extractStructFields` warns on a malformed tag while it builds each field, so a non-parameter field's tag warning still fires (and still fails `--strict`) although that field is in no schema any more. Not suppressed: at extraction time a body field cannot be told from a parameter whose mangled tag hid its `param`/`query`/`header` key, and that is the one case where the warning still matters (`Q string json:"q", query:"q"` reads `query dropped from the spec`). Suppressing it would also need a new seam through `buildFieldInfo`/`warnUnreadableTagKeys`. Pinned by Task 4's `TestMarshalerRequestTagWarnings`.

### Doctor (`internal/commands/doctor.go`)

No code change. `isTypedPayload`'s comment (550–559, "Requests reach it as well and are unchanged: a request carries no Shape …") becomes: a request with a Name is typed and one without is not, except a struct Marshaler request with a body, which carries a Resolution and is typed exactly when it is a string both ways (a params-only, non-JOSE one keeps its Name, so it stays typed; a zero-field one is not params-only and is untyped unless it is a text type).

## Generator design

- `setTypeAndFormat` (`openapi.go:1663`): `case models.ShapeText: prop.Type = typeString` after the `ShapeKindOnly` case. Pointer `nullable` follows from `buildFieldProperty`'s existing `prop.Type != ""` test (`*Level`, `*MoneyT`); a `ShapeMarshaler` leaf stays typeless, so `*Money` is never nullable. A payload root is never nullable (PR 3). Update the doc comment's leaf list.
- `isMarshalerLeaf` (`constraints.go:265`): true for `ShapeMarshaler` **or** `ShapeText`, so `applyValidationConstraints` drops collection rules on the leaf and `dive` rules on its element, while an enclosing slice/map keeps its cardinality (`minItems: 1` on S6). Update its comment and `applyValidationConstraints`' "No rule lands on a Marshaler leaf" paragraph (text leaves too).
- `applyExample` untouched: `{type: string}` keeps `example: high` and `example: "2"` as strings.
- `buildOperation` (658): emit a body when `len(bodyFields) > 0 || route.Request.JOSE || route.Request.Resolution != nil` (a Marshaler request's Fields hold only parameters, and a text request struct may have no exported field at all). Update the comment above it.
- `buildRequestBody` (2020): new case after `isJOSE`: `case reqType != nil && reqType.Resolution != nil:` → `application/json` with `payloadShapeSchema(*reqType.Resolution)`. Update its doc comment.
- `joseDescription` (695): `if plaintextSchema == "" { return joseUndocumentedDescription }`, with the new const after the function. Only a JOSE struct Marshaler **request** reaches it with `""` (`buildRequestBody` passes `reqType.Name`); without the guard the description would read `see #/components/schemas/.`. The `isJOSE` arm precedes the new Resolution arm, so a JOSE Marshaler request keeps `application/jose`. On the base `JOSE` was set only with a registered Name, so the guard never fires there and no golden moves.
- `referencedSchemaNames` (1288): one call, `addRequestFieldRefs(out, r.Request)`, at the end of the route loop. New helper `addRequestFieldRefs`: nil guard, then `addRefNames` over each `Fields[j].ResolvedShape()`. Inlined, the loop would push `referencedSchemaNames` from 10 to 16, so it is a helper. Update the doc comment as **Global Constraints** says.
- Unchanged and why: `addRefNames` (a text or Marshaler leaf references nothing), `payloadNamesComponent` (Name cleared; its `Name == ""` arm precedes `JOSE`), `successPlaintextSchema` (a nameless JOSE response already falls back to `SuccessResponse`, which `createStandardSchemas` (1133) emits), `errorSchemaName` (keys on `JOSE`, which is now kept), `isParamsOnlyType` (the requests in question are not in the registry), `extractParameters` (params typed by kind), `generateSchemasFromTypes` (no registry entry).

---

## Cognitive-complexity budget (gocognit; Before = `533353f`, After = prototype)

| Function | File | Before | After (cap) |
|---|---|---|---|
| `resolveLocal` | resolution.go | 7 | 9 |
| `resolveQualified` | resolution.go | 4 | 6 |
| `structMarshalerLeaf` (new) | resolution.go | – | 3 |
| `marshalerLeaf` | resolution.go | 2 | 3 |
| `warnFieldFallback` | resolution.go | 3 | 3 |
| `methodsInFile` | marshaler.go | 7 | 2 |
| `visitMarshalerMethods` (new) | marshaler.go | – | 5 |
| `marshalerMethods.with` | marshaler.go | 3 | 3 |
| `textBothWays` (new) | marshaler.go | – | 1 |
| `promote` (new) | marshaler.go | – | 7 |
| `methodSet.marshaler` (new) | marshaler.go | – | 6 |
| `methodSetOf` (new) | marshaler.go | – | 5 |
| `declaredMethods`, `over`, `qualifiedMethodSet`, `promotedMethods`, `embedMethodSet`, `rhsMethodSet`, `methodEntry.fallback` (new) | marshaler.go | – | ≤3 each |
| `populateRequestType` | analyzer.go | 5 | 7 |
| `registerPayloadType` | analyzer.go | 2 | 3 |
| `populateMarshalerRequest` (new) | payload.go | – | 4 |
| `payloadStructSite`, `structMarshalerJOSE`, `marshalerNamed`, `namedShape`, `warnMarshalerRequest` (new) | payload.go | – | ≤2 each |
| `requestParamFields` (new) | payload.go | – | 6 |
| `warnPayload` | payload.go | 2 | 2 |
| `isFallbackLeaf` | payload.go | 2 | 2 |
| `setTypeAndFormat` | openapi.go | 7 | 7 |
| `buildOperation` | openapi.go | 5 | 5 |
| `buildRequestBody` | openapi.go | 2 | 3 |
| `referencedSchemaNames` | openapi.go | 10 | 10 |
| `addRequestFieldRefs` (new) | openapi.go | – | 2 |
| `joseDescription` | openapi.go | 0 | 1 |
| `isMarshalerLeaf` | constraints.go | 0 | 1 |

New tests must not push any test function over 15 (split tables into sibling tests rather than growing `TestMarshalerGuardPositions`).

---

## Task 0: Branch and baseline

- [ ] **Step 1:** `cd /Users/gaborage/Projects/gaborage/code/go-bricks-openapi-pr4 && git fetch -q origin && git status` — on `fix/marshaler-types`, clean, `HEAD` = `533353f` (if not: `git checkout -B fix/marshaler-types fix/payload-resolution`). This plan file is untracked; leave it untracked unless the orchestrator says otherwise (check `git check-ignore -v docs/superpowers/plans/2026_10_09_marshaler_types_pr4.md`; PR 1–3's plans are tracked, so commit it with the change only if instructed).
- [ ] **Step 2:** `GOTOOLCHAIN=go1.27.1 make check` green; `go run github.com/uudashr/gocognit/cmd/gocognit@latest -over 15 internal cmd` lists only `yamlNodeToJSONValue` and `TestMarshalerGuardPositions`.
- [ ] **Step 3:** Re-read #111's comments (`gh issue view 111 … --comments`); if a comment newer than 2026-10-03 exists, it supersedes this plan where they differ: stop and report.

## Task 1: Carrier (`models`)

- [ ] **Step 1:** Add `ShapeText` and the four doc edits of **Data model**. `go build ./...` green (nothing reads it yet).

## Task 2: String rule and method sets, test first (`internal/analyzer/marshaler_test.go`)

- [ ] **Step 1 (red):** add tests (same file, `package analyzer`, reuse `parseInDir`/`marshalerSetOf`):
  - `TestTextBothWays`: `marshalerSetOf` on `type T int` with method combos → `textBothWays()`: pair (value MarshalText, pointer UnmarshalText) → true; pair on `*T` only → true; pair + AppendText → true; AppendText + UnmarshalText → false; MarshalJSONTo + pair → false; MarshalJSON + pair → false; UnmarshalJSON + pair → false; UnmarshalJSONFrom + pair → false; UnmarshalJSONFrom only → false; MarshalText only → false; UnmarshalText only → false. Also assert `has` bits for one row.
  - `TestPromote` (pure, hand-built `[]embedMethods`): unique depth-1 entry promoted at depth 2 with `via` = its embed; two embeds tying → `ambiguous`; depth 1 beats depth 2 regardless of order; an ambiguous shallow entry hides a deeper exact one; `exact: false` propagates.
  - `TestMethodSetOverAndMarshaler`: `over` keeps the shallower entry; `marshaler` skips non-exact and ambiguous entries, reads `marshalerMethodOrder` (an own `UnmarshalJSON` plus a promoted `MarshalText` names `MarshalText`), returns `fallbackMarshaler{typeName: owner}` for depth 0 and `fallbackMarshalerPromoted{typeName, detail, via}` for depth > 0; an empty set → `!found()`.
  - `TestMethodSetOf` (temp project via `analyzeProjectRoutes`-style files, or `parseInDir` with a real dir so sibling files resolve; one table, each row a type name → expected `found`, `textBothWays`, fallback kind/typeName/via): `Money`, `MoneyP`, `MoneyU`, `MoneyOdd` (none), `MA` (owner `Money`), `MD` (none), `MDW` (text), `MoneyT`, `EmbP`, `WrapLevel`, `PtrEmb`, `TaggedEmb`, `Excl`, `Deep`, `Near`, `Twin` (decode only, via `Level`), `Pair` (none), `Shadow` (own MarshalJSON), `OddShadow` (UnmarshalText promoted), `Stamped` (via `time.Time`), `StampA` (via `TA`), embed of `type StampD time.Time` (none), `WithID` (text), `Raw` (MarshalJSON via `json.RawMessage`), `GenEmb` (none), `TagEmb{Tags; N}` with `type Tags []string` (none: `rhsMethodSet`'s `default:` arm), `struct{ int; N int }` (none), `Node`/`A` (none, terminates), `WithCode{domain.Tag; X}` with a `domain/` subpackage and `go.mod` (text, cross-package), an embedded `decimal.Decimal` (none: third-party), a 10-deep chain `E0{E1}`…`E9{Level}` (the cap: `E0` and `E1` see nothing, because `Level` is cut at nine open names; `E2` sees its methods; prototype-verified). Split into two or three test functions if one would exceed 15. **The cross-package rows need `a.modulePath`:** `inModuleTypeSite` → `inModuleDir` returns false when it is empty, so a bare `New(dir)` sees `WithCode` as "none". Either run `AnalyzeProject` on the temp project first (it reads `go.mod`), or route those rows through `analyzeProjectRoutes` and assert on the field Resolution instead.
- [ ] **Step 2 (green):** apply Appendix A's `marshaler.go` hunks (bits, table, `visitMarshalerMethods`, `methodsInFile`, `with`, `textBothWays`, and the method-set block), plus `fallbackMarshalerPromoted` and `fieldFallback.via` in `resolution.go` (needed to compile). Existing `TestMarshalerSet*` stay green unchanged.

## Task 3: Resolver, test first

- [ ] **Step 1 (red):** new file `internal/analyzer/marshaler_types_test.go`:
  - `TestMarshalerTypeFieldResolution`: one project (types of Appendix B's `types.go`, `domain/domain.go`, and a module whose `Fields` struct holds S1–S16 and M1–M13, **every M4–M7 type in all four positions** `T`, `*T`, `[]T`, `map[string]T`, as Appendix B's `Fields` does), analyze, and for each `Fields` field assert the Resolution root kind after pointers/containers (`ShapeText` / `ShapeMarshaler` / `ShapeRef` / primitive) and that the `ShapeMarshaler` leaf of a struct has `Elem == nil`. Rows are table entries, so the sixteen M4–M7 rows cost no complexity; if the function nears 15, split by row group.
  - `TestStructMarshalerRegistersNoComponent`: same project; `project.Types` has `Fields`, `MD`, `MoneyOdd`, `Pair` (+ nothing else besides what U-rows need) and **lacks** `Money`, `MoneyP`, `MoneyU`, `MA`, `Price`, `MoneyT`, `Amount`, `EmbP`, `Wrapper`, `Stamped`, `Twin`, `Shadow`, `MoneyWithInner`, `OnlyInner`, `WrapLevel`, `PtrEmb`, `TaggedEmb`, `Deep`, `WithID`, `WithCode`, `Raw`, and the review-round-1 types `JoseReq`, `JoseResp`, `CmdBody`, `Defaults`, `ListReq`, `Filter`, `Empty` (this project has no `/jose`, `/cmd` or `/items` route; with `/cmd` present, `Filter` and `Empty` **do** register through `requestParamFields`, which Task 4 asserts).
  - `TestMarshalerTypeFieldWarnings`: `a.Warnings()` contains exactly one warning per M-row field (the sixteen M4–M7 positions included, each naming its field and its type as written, e.g. `*MoneyP`, `map[string]domain.Price`), with the exact texts of M4–M13 (`fmt.Sprintf(marshalerFieldWarning, …)` / `marshalerPromotedFieldWarning`), and none mentioning an S-row field or `lvl`.
  - `TestMarshalerTypeParamsByKind`: `Params{Level query; HLevel header; M MoneyT query}` → int64 resolutions for the two `Level`s, `ShapeRef` for `M`, `MoneyT` registered, no warning.
  - `TestStructMarshalerUnderlyingMode`: `type D q.UA` with `q.UA = q.User` and `q.User{Level}` → `ShapeText` (promoted kept, declared dropped); with `q.User` declaring its own `MarshalJSON` and no embed → `ShapeRef` (dropped).
  - `TestStructMarshalerFreesShortName` (review round 3, finding 2): a module with `go.mod` (reuse `TestMethodSetOf`'s `WithCode`/`domain` setup), packages `a` (`type Money struct{Amount int64}` + value `MarshalJSON`) and `b` (plain `type Money struct{Cents int64}`), and `Out{A a.Money; B b.Money}` in the handlers' file as the payload of one route, `a` declared first. After `AnalyzeProject`: `Out.Fields[B]`'s Resolution is `ShapeRef{Name: "Money"}`, `project.Types["Money"]` has the field `Cents`, and `project.Types` has no `BMoney` and no entry for `a.Money`. On the base the same project yields `Money` (`a`'s) and `BMoney`; that is the behavior change Impact item 3 names. Hoist `"Money"` into a test `const` if the package then repeats it ≥3 times (goconst).
- [ ] **Step 2 (green):** apply Appendix A's `resolution.go` hunks (`resolveLocal`, `resolveQualified`, `marshalerLeaf`, `structMarshalerLeaf`, the warning const and `warnFieldFallback` case) and the `quiet()` comment edit. Run `go test ./internal/analyzer` — every pre-existing test stays green (prototype: none changes).

## Task 4: Payloads and requests, test first (`internal/analyzer/payload_test.go` or a new `marshaler_payload_test.go`)

- [ ] **Step 1 (red):**
  - `TestMarshalerPayloadRows` (`analyzeProjectRoutes` + `routeByPath`): `/money`, `/money-ptr`, `/moneys`, `/stamped` → `Response.Name == ""`, Resolution root (after pointer/slice) `ShapeMarshaler`, `IsTypedPayload` false, one warning each with the exact `marshalerPayloadWarning`/`marshalerPromotedPayloadWarning` text, no `Money`/`Stamped` in `Types`; `/money-t`, `/wrap-level`, `/level`, `/levels` → `ShapeText` root (or slice of it), `IsTypedPayload` true, no warning.
  - `TestWarnPayloadEveryKind`: add the `fallbackMarshalerPromoted` row (extend the table; keep the function ≤15).
  - `TestIsTypedPayload`/`isFallbackLeaf`: add a `ShapeText` row (typed).
  - `TestMarshalerRequest`: request `CustomBody` → `Name ""`, `Resolution.Kind == ShapeMarshaler`, `Fields` = only `ID` (`ParamType: "path"`), warning `fmt.Sprintf(marshalerRequestWarning, "CustomBody", "CustomBody", methodUnmarshalJSON)`, no `CustomBody` component; request `*Wrapper` → promoted request warning naming `StatusV`; request `MoneyT` → `ShapeText`, no warning; request `domain.Price` (qualified) → `ShapeMarshaler`, warning names `domain.Price`; request `Level` (non-struct Marshaler) → unchanged `nonStructRequestWarning`, no Resolution; request `Cents` (non-Marshaler non-struct) → unchanged.
  - `TestMarshalerRequestEdgeRows` (sibling, keeps both ≤15): a JOSE-tagged (`_ struct{} jose:"…"`) request with own `UnmarshalJSON`, and a JOSE-tagged response with own `MarshalJSON` as `Result[T]`, `Result[*T]` and `Result[[]T]` → `JOSE == true`, `Name == ""`, no `JoseReq`/`JoseResp` in `Types`; a JOSE params-only request (`JoseParams`) → `JOSE == true`, `Resolution != nil`, `Fields` = `id` only, one warning; a params-only request `ListReq{Defaults; Page query; ID param}` → `Name == "ListReq"`, `Resolution == nil`, `Fields` = `Page`, `ID`, no warning, `IsTypedPayload` true, no `ListReq` in `Types`; a params-only text request `TextParams` → the same, no warning; a JOSE text request `JoseText` (sentinel, text pair, no exported field) → `JOSE == true`, `Resolution.Kind == ShapeText`, no warning, `IsTypedPayload` true; `CmdBody{F Filter query; E Empty query; Name}` → `Fields` = `F`, `E` with `ShapeRef` resolutions, and `Filter`, `Empty` registered.
  - `TestZeroFieldMarshalerRequest` (sibling; review round 3, finding 1): on a `GET`, request `Ping struct{}` with pointer `UnmarshalJSON` → `Name == ""`, `Resolution.Kind == ShapeMarshaler`, `Fields` empty, exactly one warning `fmt.Sprintf(marshalerRequestWarning, "Ping", "Ping", methodUnmarshalJSON)`, `IsTypedPayload(route.Request)` false; request `Blank struct{}` with the text pair → `Name == ""`, `Resolution.Kind == ShapeText`, no warning, `IsTypedPayload` true; request `Hidden{ID string param:"id"; secret string}` with pointer `UnmarshalJSON` → params-only: `Name == "Hidden"`, `Resolution == nil`, `Fields` = `ID`, no warning; a plain `Plain struct{}` request → unchanged (`Name == "Plain"`, registered as on the base).
  - `TestMarshalerRequestTagWarnings` (sibling; pins the round-2 residual): request `Local{ID string param:"id"; Name string json:"name", validate:"x"; Q string json:"q", query:"q"}` with the text pair → `Name == ""`, `Resolution.Kind == ShapeText`, `Fields` = `ID` only, no request warning, and exactly two tag warnings, one naming `Name` (`validate`) and one naming `Q` (`query`). Match on a test `const` for the shared `is not readable by reflect.StructTag` fragment (goconst). Then, in a **second project** (warnings dedupe by tag position, so the request path would fire them first in the same one), the same struct reached only as a **field** (`Out{L Local}`) → no warning at all.
  - `TestStructMarshalerJOSE`: jose-tagged struct → true; untagged struct → false; non-struct Marshaler (`Level`) → false (no site).
  - `TestWarnMarshalerRequestEveryKind`: own, promoted, and `fallbackNone` (no warning).
  - `TestNamedShapeAndMarshalerNamed`: `Package ""`, own package, other package; `marshalerNamed` with `Name ""` → false.
- [ ] **Step 2 (green):** apply Appendix A's `payload.go` and `analyzer.go` hunks. They already have the swapped check order of **Payloads and requests**, the JOSE flag at the gate and the params-only arm.

## Task 5: Generator, test first (`internal/generator/resolution_test.go`)

- [ ] **Step 1 (red):**
  - `TestSetTypeAndFormatResolutionLeaves`: add `ShapeText` → `{type: string}` (no format) and `[]ShapeText` → string items.
  - `TestTextLeafTakesNoKeywords` (sibling of `TestMarshalerLeafTakesNoKeywords`): a `FieldInfo` resolved to `ShapeText` with `Constraints{min, max, email, oneof}` → `{type: string}` only; `Pointer{ShapeText}` → `nullable: true`; `Slice{ShapeText}` with `Constraints{min: 1}`, `ElementConstraints{max: 3}` → `minItems: 1`, items `{type: string}`; through `fieldInfoToProperty` with `Example: "2"` → `example: "2"` (a string) and `"high"` → `"high"`.
  - `TestIsMarshalerLeaf`: `ShapeText`, `Pointer{ShapeText}` true; primitive false.
  - `TestMarshalerRequestBody`: a route whose `Request` is `{Resolution: &ShapeMarshaler, Fields: [id path param]}` → `requestBody` `{}` under `application/json`, `id` parameter present; `{Resolution: &ShapeText}` with no Fields → body `{type: string}`; `buildRequestBody(nil)` unchanged (inline object).
  - `TestAddFieldSchemaRefsWalksResolution`: add a row whose field resolves to `ShapeText` (no name marked).
  - `TestReferencedSchemaNamesMarshalerRequestParams`: a route whose `Request` is `{Resolution: &ShapeMarshaler, Fields: [f query param resolved to ShapeRef Filter, e query param resolved to ShapeRef Empty]}`, with `types` holding a params-only `Filter` and a field-less `Empty` → both marked, `""` not marked; then `generateSchemasFromTypes` emits both (`{type: object}`). Plus `addRequestFieldRefs(out, nil)` → no panic, nothing marked; a registered request's Fields → the same names `addFieldSchemaRefs` marks, and never the request's own name.
  - `TestJOSEMarshalerRequestBody`: `{JOSE: true, Name: "", Resolution: &ShapeMarshaler}` → `buildRequestBody` gives `application/jose` `{type: string, format: jose}` with `Description == joseUndocumentedDescription` (no `#/components/schemas/` substring); `buildOperation` emits it; a JOSE response with `Name == ""` → description names `SuccessResponse`, and `createStandardSchemas` includes `SuccessResponse` and `JOSEErrorEnvelope`. `joseDescription("X")` is unchanged byte for byte.
- [ ] **Step 2 (green):** apply Appendix A's `openapi.go` and `constraints.go` hunks plus the comment updates listed in **Generator design**.

## Task 6: Golden fixture `internal/spectest/testdata/marshaler_types/`

- [ ] **Step 1:** create the four files of **Appendix B** verbatim (`go.mod`, `domain/domain.go`, `types.go`, `module.go`). `package marshalertypes`; the structs that use `domain` (`WithCode`, `Fields`) live in `module.go`, the handlers' file (footnote ¹, #123). Run `gofmt -w` on the `.go` files by hand (Go tooling skips `testdata/`).
- [ ] **Step 2:** `go test ./internal/spectest` fails only on `TestGoldenFixtures/marshaler_types` (missing `expected.yaml`).
- [ ] **Step 3:** `go test ./internal/spectest -update`, then `git status --porcelain internal/spectest/testdata` lists only `marshaler_types/`.
- [ ] **Step 4:** check `expected.yaml` cell by cell against S1–S18, M1–M17, U1–U5 (components exactly `Empty`, `ErrorResponse`, `Fields`, `Filter`, `JOSEErrorEnvelope`, `MD`, `MoneyOdd`, `Pair`, `SuccessResponse`; `required: [level]` on `Fields`; `/custom/{id}` body `{}` and the `id` path parameter; `/params` integer parameters; `/cmd` parameters `$ref Filter`/`$ref Empty`; `/jose` `application/jose` both ways with 401/415; `/items/{id}` no `requestBody`). The round-2 prototype's `expected.yaml` had 859 non-empty lines (`*MoneyP`, `*MoneyU`, `*MA`, `*domain.Price` each `{}` with no `nullable`; their slices `items: {}`, their maps `additionalProperties: {}`).
- [ ] **Step 5 (parity oracle, `internal/spectest/payload_parity_test.go`):** add `Level` (pair), `MoneyT` (struct pair), `WrapLevel{Level; Note}`, `Money` (struct MarshalJSON) and `Stamped{time.Time; Note}` to `parityDecls` (add the `time` import if missing; it is already there), and rows `same("Level", "[]Level", "MoneyT", "WrapLevel", "Money", "[]Money", "Stamped")` plus pointer rows `"*MoneyT": "MoneyT"`, `"*Money": "Money"`. The payload schema must equal the field's (pointer roots compared with the pointee, as the existing rows do).

## Task 7: Commands (`internal/commands/marshaler_types_test.go`, new)

- [ ] `TestRunGenerateTextMarshalersStrictClean`: `writeProject` with one file declaring `Level`, `LevelP`, `MoneyT`, `WrapLevel`, `WithID{uuid.UUID; Note}`, a struct with fields of each (pointer and slice forms included), and routes `Result[MoneyT]`, `Result[Level]`, `Result[[]Level]` and a POST with request `MoneyT` → `generate --strict --validate` exits 0 with `Warnings: 0`. The project also declares `MoneyF`, a text struct used **only** as a field and as `Result[MoneyF]`, never as a request, with fields `A int json:"a", validate:"min=1"` (malformed tag) and `P uintptr`: both warn on the base and must not after, which pins Impact item 7 (a text struct's fields are never read). Do not put them on `MoneyT`: it is the POST request here, and a request's fields are still extracted, so its malformed tag would warn (the round-2 residual; prototype-checked, `Warnings: 1`) (this fixture has non-struct payloads, which is fine because #110 has landed on the base).
- [ ] `TestRunGenerateStructMarshalerFailsStrict`: the same plus one `Money` field → `--strict` exits non-zero, the Money warning printed.
- [ ] `TestDoctorClassifiesMarshalerTypes`: routes `Result[Money]` (untyped), `Result[MoneyT]` (typed), request `CustomBody` with a typed response (request untyped), request `MoneyT` (typed), request `ListReq` (params-only, typed); assert the typed count doctor prints, following `TestDoctorClassifiesPayloads`' pattern.
- [ ] `TestRunGenerateStructMarshalerEdgeRoutesValidate`: `writeProject` with M16's `CmdBody`/`Filter`/`Empty`, M17's `JoseReq`/`JoseResp` and U5's `ListReq` → `generate --validate` (no `--strict`) exits 0 (kin passes: no `$ref` dangles), and the written spec has `application/jose` under `/jose`, `Filter` and `Empty` components, and no `requestBody` under `/items/{id}`. Then `--strict` on the `ListReq` route alone exits 0 with `Warnings: 0`.
- [ ] `TestRunGenerateZeroFieldMarshalerRequest` (review round 3, finding 1; the declared doctor/`--strict` surface): a project whose only route is `GET /ping` with request `Ping struct{}` + pointer `UnmarshalJSON` and a typed response → `generate --strict` exits non-zero and prints the `request type Ping` warning; without `--strict` the written spec has `requestBody` under `/ping` with schema `{}`. A second project with `POST /blank`, request `Blank struct{}` + the text pair → `generate --strict --validate` exits 0, `Warnings: 0`, body `{type: string}`. Both outcomes differ from the base (no body, `Warnings: 0` for `Ping`), prototype-checked in `rv6/zf`.

## Task 8: Docs

- [ ] **README.md, Known limitations**, replace the Marshaler bullet (`grep -n 'A field whose type has its own .MarshalJSON' README.md`, 232–245) with one that says, keeping the existing sentences about defined types, parameters, the byte slice rule and the alias-receiver residual:
  - a type `encoding/json` writes and reads as text, through `MarshalText` and `UnmarshalText` declared or promoted, with no `MarshalJSON`/`MarshalJSONTo`/`UnmarshalJSON`/`UnmarshalJSONFrom` (`AppendText` alone does not count), is documented as `{type: string}`, without its `validate` keywords (an enclosing slice's or map's cardinality is kept) and with its `example:` as a string;
  - every other Marshaler type, **including a struct and a struct that embeds one** (method promotion follows Go: the shallowest embedding wins, two at the same depth cancel, the type's own method shadows a promoted one; json tags do not matter), is documented as `{}` with a warning, gets no component, and makes the route untyped when it is the payload; a request of such a type keeps its path/query/header fields as parameters and gets an untyped body; there is no override yet (#117);
  - pointer-receiver-only methods count in every position, although `encoding/json` skips them on non-addressable values (a map value, a payload returned by value);
  - out-of-module types are known only from a table (`time.Time`, `uuid.UUID`, `json.RawMessage`), so an embedded `decimal.Decimal` goes undetected and its embedding struct is documented from its fields;
  - methods in build-tagged files count (build constraints are not evaluated);
  - a Marshaler type's own fields are not read, so their tags and types are not checked and raise no warning; a Marshaler **request**'s fields are still read to find its parameters, so a malformed tag on any of them still warns.
- [ ] **README.md payload bullet** (285): "A payload that ends in a fallback — a Marshaler type, …" becomes "— a Marshaler type other than a text one, …".
- [ ] **CONTEXT.md:** no change (the **Marshaler type** entry already covers promotion). **ADR 0002:** no change (it already records that #111 lands the string rule, struct Marshaler types and promotion).
- [ ] Comment edits listed in **Data model**, **Resolver call sites**, **Doctor**, **Generator design** and **Global Constraints** (`referencedSchemaNames`' invariant paragraph: a request's own name is still never marked; its Fields' ref targets are, which for a struct Marshaler request is the only marking they get). `buildRequestBody`'s doc: a JOSE request with no Name gets the nameless description.
- [ ] **CLAUDE.md, Settled invariants:** supersede in place (the Doc rot convention: `> **SUPERSEDED (date).**` banner, old text kept) the bullet "`referencedSchemaNames` deliberately does not scan non-JOSE request types". New text: it never marks a non-JOSE request's **own** name (that still orphans a component per params-only request type); `addRequestFieldRefs` marks the `$ref` targets of every request's Fields, which for a registered request is exactly what `addFieldSchemaRefs` marks, and for a struct Marshaler request (#111, not in the registry) is the only marking its struct parameters get, or their `$ref`s dangle. Without this, the next agent reads `addRequestFieldRefs` as a violation of the old bullet.
- [ ] **README.md, Known limitations** (same Marshaler bullet): a jose-tagged struct Marshaler type keeps its route `application/jose`; its plaintext is not documented (the request description says so; the response description names the generic `SuccessResponse`). A request whose every field is a path/query/header parameter is documented without a body even when it is a Marshaler type; a Marshaler request with no field at all is not such a request and gets a body (`{}` with a warning, or `{type: string}`), on a GET too. Because a struct Marshaler type gets no component, it no longer holds its short component name: a same-named struct of another package that used to be qualified (`BMoney`) can now take the short one (`Money`), so its `$ref`s and generated client type name move.

## Task 9: Lint, complexity, coverage, mutations

- [ ] `GOTOOLCHAIN=go1.27.1 make check` → `0 issues`, all green; `go test -race ./...` green.
- [ ] gocognit `-over 15` on every touched file reports only the two baseline entries; touched functions within the budget table.
- [ ] Coverage: `go test -coverprofile=/tmp/a.out ./internal/analyzer && go tool cover -func=/tmp/a.out | grep -E 'marshaler.go|payload.go|resolution.go'` and the same for `./internal/generator`: every new or changed function 100% (prototype order swap in Task 4 makes `populateMarshalerRequest` fully reachable).
- [ ] Mutations, each restored from a scratchpad copy of the file (`cmp` after every run; **never** `git checkout`, which would discard the uncommitted change):
  1. `textBothWays` returns `false` → fails `TestTextBothWays`, `TestMarshalerTypeFieldResolution` (Level rows), `TestMarshalerPayloadRows`, `TestGoldenFixtures/marshaler_types`, `TestRunGenerateTextMarshalersStrictClean`.
  2. `structMarshalerLeaf` returns `models.TypeShape{}, false` first → fails `TestStructMarshalerRegistersNoComponent` (Money), `TestStructMarshalerFreesShortName` (`b.Money` back to `BMoney`), `TestMarshalerTypeFieldResolution`, `TestMarshalerRequest`, the golden.
  3. `promotedMethods` returns `methodSet{}` → fails `TestMethodSetOf` (WrapLevel), `TestStructMarshalerRegistersNoComponent` (WrapLevel registered), the golden.
  4. `registerPayloadType`'s gate removed → fails `TestMarshalerPayloadRows` (`/money` registers `Money`) and the golden.
  5. `isFallbackLeaf` without `ShapeText` → fails the `IsTypedPayload` text row and `TestDoctorClassifiesMarshalerTypes`.
  6. `isMarshalerLeaf` without `ShapeText` → fails `TestTextLeafTakesNoKeywords` and the golden (`minLength`/`format: email` on `code`).
  7. `buildOperation` without `|| route.Request.Resolution != nil` → fails `TestMarshalerRequestBody` and the golden (`/custom/{id}` loses its body).
  8. `promote`'s tie arm deleted (last writer wins) → fails `TestPromote` and the `Twin`/`Pair` rows.
  9. The gate's `typeInfo.JOSE = …` line deleted → fails `TestMarshalerRequestEdgeRows` (JOSE rows), the golden (`/jose` turns `application/json`, loses 401/415) and `TestRunGenerateStructMarshalerEdgeRoutesValidate`.
  10. The `addRequestFieldRefs` call deleted → fails `TestReferencedSchemaNamesMarshalerRequestParams`, the golden's in-process validation (`Filter`/`Empty` not found; prototype-verified on scratch projects) and the edge-routes CLI test.
  11. The params-only arm deleted (`if paramsOnly && !ti.JOSE`) → fails the `ListReq` rows of `TestMarshalerRequestEdgeRows` and `TestDoctorClassifiesMarshalerTypes`, and the golden (`/items/{id}` gains a body).
  12. `joseDescription`'s `""` guard deleted → fails `TestJOSEMarshalerRequestBody` and the golden.
  13. `requestParamFields` returns `len(params) == total` (the `total > 0 &&` guard dropped, so zero fields reads as params-only) → fails `TestZeroFieldMarshalerRequest` (`Ping`, `Blank` keep their Name and get no Resolution) and `TestRunGenerateZeroFieldMarshalerRequest` (`--strict` exits 0, no `requestBody`). The golden is not expected to move (no zero-field request in it); if it does, record why.

## Task 10: Commit

- [ ] Stage the code, tests, fixture, README, CLAUDE.md and comment edits (and this plan only if instructed in Task 0). One commit, `git commit -F <file>` (a commit hook rejects heredoc `-m` messages; signing may need 1Password unlocked: retry, never disable signing). Message:

```
fix(analyzer): type text Marshaler types as strings, never $ref one

A Marshaler type is now documented from its method set as Go and
encoding/json see it, not from its fields or underlying type. A type
encoding/json writes and reads as text on every toolchain (MarshalText
with no MarshalJSON or MarshalJSONTo, and UnmarshalText with no
UnmarshalJSON or UnmarshalJSONFrom; AppendText alone does not count)
is {type: string}: Level, *Level (nullable), LA = Level, LevelP (pair on
the pointer), []Level, map[string]Level, a byte-kind []FlagT2 (never
base64), a struct with the text pair (MoneyT, domain.Amount) and a
struct that promotes it (WrapLevel, {*Level; Note}, a json-tagged or
json:"-" embed, Deep{WrapLevel; X}, WithID{uuid.UUID; Note},
WithCode{domain.Tag; X}). Its validate keywords are dropped (an
enclosing slice's or map's cardinality is kept) and its example is kept
as a string. Result[Level], Result[[]Level], Result[MoneyT] and
Result[WrapLevel] are typed with no warning.

Every other Marshaler type stays {} with one warning, now including
struct types, which used to register a component and be $ref'd: Money
(own MarshalJSON), MoneyP, MoneyU, MA = Money, domain.Price, Wrapper
(promoted MarshalText only), Stamped{time.Time; Note}, Twin{Level;
StatusV} (the two MarshalText cancel, UnmarshalText remains), Shadow
(own MarshalJSON over an embedded Level), Raw{json.RawMessage; N} and
MoneyWithInner. None registers a component, nothing reachable only
through one does (OnlyInner), *Money is not nullable, and Result[Money],
Result[*Money], Result[[]Money] and Result[Stamped] count as untyped. A
request of such a type (CustomBody with UnmarshalJSON) gets an untyped
body with a warning, keeps its path, query and header fields as
parameters, and counts as untyped; a text request struct gets a string
body. A request whose every field is a parameter gets no body and no
warning, as before; one with no field at all is not such a request, so
Ping struct{} with UnmarshalJSON, which had no body, now gets {} and a
warning (on a GET too) and a text Blank struct{} a string body. A struct
parameter of such a request keeps its component (the generator now marks
the $ref targets of a request's own fields, never the request itself),
and a jose-tagged struct Marshaler request or response keeps
application/jose and the JOSE error set; only its plaintext goes
undocumented (no component is named). A promoted method's warning names
the embed it comes through.

Method sets follow Go's selector rules: declared methods (on T or *T,
any signature: a wrong-signature own method shadows a promoted one
without counting) over promoted ones, the shallowest embedding wins and
a same-depth tie cancels and hides deeper ones; an alias keeps its
target's methods, a defined type keeps only what its struct promotes
(type MD Money stays a struct; type MDW WrapLevel is a string).
Promotion ignores json tags and reads value, pointer, local, in-module
and well-known embeds; time.Time, uuid.UUID and json.RawMessage come
from a fixed table. Parameters are bound by kind and unchanged.

Warnings follow the fields that are read. A struct Marshaler type's own
fields, and those of structs reachable only through it, are never
extracted, so the malformed-tag, uintptr, unresolvable-type, Marshaler
and embed-promotion warnings they raised are gone; a text struct can
therefore newly pass generate --strict. A struct Marshaler request's
fields are still extracted to find its parameters, so a malformed tag
on a body field still warns and fails --strict, although that field is
in no schema: at that point a body field cannot be told from a parameter
whose broken tag hid its param, query or header key.

Unchanged: LD (type LD Level), MoneyOdd (MarshalJSON() string), MD,
Pair{Level; Code} (every method cancels; its dropped fields are #116's)
and every parameter. No existing golden moves.

Residuals: an embedded third-party type (decimal.Decimal) is not read;
a struct field named like one of the seven methods, and an embedded
interface, are not modelled. Generics stay #114's and overrides #117's.

New: models.ShapeText; the method-set walk and well-known table in
marshaler.go; structMarshalerLeaf in the resolver; a Marshaler gate in
registerPayloadType (which records a jose tag) and
populateMarshalerRequest for requests; the generator types ShapeText as
string, drops its keywords, emits a request body from a request
Resolution, describes a nameless JOSE request body, and marks a
request's field refs (a request's own name is still never marked;
CLAUDE.md's referencedSchemaNames invariant is superseded in place to
say so). New golden marshaler_types;
unit tests per package, payload/field parity rows, and CLI strict and
doctor tests.

BREAKING CHANGE: struct Marshaler types and structs that promote a
Marshaler method lose their components; their $refs become {} or
{type: string}, which moves generated client types, and routes whose
payload or request is a non-text Marshaler type newly count as untyped
and fail generate --strict, a zero-field request included. Such a type
no longer holds its short component name, so a same-named struct of
another package that was qualified is renamed to it (BMoney becomes
Money, and Money now holds that struct), moving $refs and generated
client type names; regenerate clients.

Closes #111
Refs #116
Refs #117
Refs #113
Refs #114
Refs #123

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

Check before committing: subject ≤72 chars (it is 67); `Closes` appears once; no other issue number follows close/fix/resolve wording anywhere in the body.

PR body (orchestrator's, per the global three-heading rule; ≤150 words):

```
## What
Text Marshaler types (MarshalText + UnmarshalText, no JSON methods, declared or promoted) were `{}` with a warning and struct ones were `$ref`'d components; text types are now `{type: string}` without validate keywords, and every other Marshaler type, structs and embedders included, is `{}` with a warning and no component.

## Impact
Struct Marshaler types and structs promoting a Marshaler method lose their components, so `$ref`s and client types move, and a same-named struct elsewhere can take the freed short name (`BMoney` becomes `Money`); JOSE routes stay `application/jose`. Non-text ones, zero-field requests included, newly fail `generate --strict` and count as untyped in doctor; text ones now pass, and their own fields no longer raise warnings.

## Verification
New golden `marshaler_types`; no existing golden moved. Mutation checks on the string rule, struct inlining and promotion each failed their tests.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

---

## Impact (user-visible behavior changes; SemVer surfaces: generated-output shape, doctor/validation)

1. String-both-ways types (`Level`, `LevelP`, `Code`, `[]FlagT2`, `TextApp`), and `MoneyT`-style structs and promoting structs (`WrapLevel`, `WithID`, `WithCode`, `EmbP`), become `{type: string}` without `validate` keywords; their fields, payloads and requests **stop warning and now pass `--strict`**; their routes become typed in doctor.
2. Struct Marshaler types and structs that promote a non-text Marshaler method (`Money`, `MA`, `domain.Price`, `Wrapper`, `Stamped`, `Twin`, `Shadow`, `MoneyWithInner`, `Raw`) go from a `$ref` to `{}` with **one new warning** each, so `generate --strict` **newly fails**, and a route whose payload or request is one counts as **untyped** (aggregate warning, doctor).
3. Components disappear: every struct Marshaler type's, and every struct's reachable only through one (`OnlyInner`). `$ref`s and generated client type names move (BREAKING, capped to MINOR pre-1.0). Those types also stop claiming their short name in `schemaKey` (first come, first served), so a same-named plain struct of another package that was collision-qualified (`BMoney`) now takes the short name (`Money`): its component is renamed, every `$ref` to it moves, and a component of an existing name (`Money`) now holds a different struct, with no warning. This touches types that are not Marshaler types at all; regenerate clients. No existing golden is affected (no base fixture declares a struct Marshaler type, Global Constraints). Pinned by `TestStructMarshalerFreesShortName`.
4. `*Money`-style pointer fields lose `allOf` + `nullable` (now `{}`); `*MoneyT` becomes `{type: string, nullable: true}`.
5. A struct Marshaler request: `requestBody` becomes `{}` (or `{type: string}`) under `application/json`, path/query/header fields stay parameters (their struct components stay), new `request type …` warning for non-text ones. A params-only one (at least one field, every one a parameter) is unchanged (no body, no warning, typed). A **zero-field** one (`Ping struct{}` with `UnmarshalJSON`, or only unexported fields) is not params-only: it had no body on the base and now gets `{}` (also on a GET), a new warning, an untyped request in doctor and a newly failing `generate --strict`; a zero-field text one gets a `{type: string}` body and stays typed.
6. A jose-tagged struct Marshaler request or response stays `application/jose` with `JOSEErrorEnvelope`, 401 and 415. Its description stops naming the plaintext type: a request's says the plaintext is not documented, a response's names `SuccessResponse`, a component the document now carries even when no other route needs it. The plaintext component (`JoseReq`, `JoseResp`) disappears like any other struct Marshaler type's, and the route counts as untyped unless the type is a text one.
7. A struct Marshaler type's own fields, and those of any struct reachable only through it, are never extracted, so **every warning they raised on the base disappears**: malformed tag (`hiddenTagKeys`), `uintptr`, unresolvable type, Marshaler field, embed promotion (`TaggedEmb`'s `lvl`). A text struct (`MoneyT`-style) whose fields warned therefore flips `generate --strict` from 1 to 0 with no diagnostic; that is correct for the wire (encoding/json never reads those tags) but it is a visible change. The opposite holds for a struct Marshaler **request**: its fields are still extracted to find parameters, so a malformed tag on a body field still warns and still fails `--strict`, with text saying the key was dropped from a spec that no longer holds the field (accepted residual, Payloads and requests).
8. Parameters, defined types over Marshaler types (`LD`, `MD`), wrong-signature methods (`MoneyOdd`) and fully cancelling embeds (`Pair`) are unchanged.

## Open decisions (recommended default first)

1. **JOSE-tagged struct Marshaler type: resolved in review round 1** (adopted: `JOSE` kept, nameless request description, row M17). Still open: the request/response warning texts say "emitting an untyped requestBody schema ({})" / "untyped schema ({})" also on a JOSE route, where the wire schema is the JOSE token and only the plaintext is untyped. Default: keep one text (fewer consts; the warning is still true of the plaintext). Alternative: JOSE-specific variants (two more consts and a branch in `warnMarshalerRequest`/`warnPayload`).
2. **`time.Time`'s table entry.** Default: the brief's four methods. Go ≥1.24's `time.Time.AppendText` would matter only for a struct whose other same-depth embed cancels all four; outcome unchanged in every brief row.
3. **Own-method warning text for structs** reuses `marshalerFieldWarning`/`marshalerPayloadWarning` ("instead of its underlying type"). Default: keep (no existing text changes). Alternative: a struct-specific "instead of its fields" variant (two more consts).
4. **`ShapeMarshaler.Elem`.** Default: keep the field (the byte-slice rule still needs the quiet run), fix its comment; struct leaves carry none.
5. **Method-set cost.** Default: no cache; every struct ref now triggers one method scan of its package (parsed files are cached). Add a per-typeKey cache only if a real project shows a slowdown.
6. **Zero-field struct Marshaler request (review round 3, finding 1).** Default: keep Appendix A's behavior and state it (a body, `{}` plus a warning or `{type: string}`; untyped and `--strict`-failing unless text). Reason: the brief makes a struct with `UnmarshalJSON` `{}`, and a zero-field struct declaring a decode method does read the body through it; round 1's carve-out needs every field to be a parameter, which zero fields cannot show. Alternative (base parity): drop `total > 0 &&` in `requestParamFields` (or add a `total == 0 && !JOSE` arm, optionally only for a non-text leaf), so such a request keeps its Name, gets no body and no warning. That is a one-token code change but needs a prototype re-run, the `TextParams`-style rows for `Ping`/`Blank` flipped, and mutation 13 inverted.
7. **Short-name hand-off (review round 3, finding 2).** Default: accept and document it (Impact item 3, BREAKING CHANGE footer), as PR 3 did for the mirror case. Alternative: have a struct Marshaler type still reserve its short name in `usedNames` without registering; rejected as default because it would keep a qualified `BMoney` beside no `Money` component, a gap in the names for a type the spec never shows, and would need a new seam in `structMarshalerLeaf`.

## Self-review: acceptance criteria → steps

| Criterion (brief + amendment) | Where |
|---|---|
| Every table row covered in each position it lists | S/M/U rows → Task 3 (fields), Task 4 (payloads, request), Task 5 (generator), Task 6 (golden + parity), Task 7 (CLI). `Money`, `MoneyP`, `MoneyU`, `MA`, `domain.Price` each as `T`, `*T`, `[]T`, `map[string]T` (rows M4–M7, Appendix B `Fields`). Round 1 claimed this row while only `Money` had all four positions (review round 2, finding 2). |
| Four amendment rows | S11, M3, `TestTextBothWays` |
| No component/`$ref`/JOSE description names a Marshaler type; none unreferenced; `MD`, `MoneyOdd`, `Pair` keep theirs; kin passes | `TestStructMarshalerRegistersNoComponent`, golden components list (M16, M17), `TestMarshalerRequestEdgeRows`, `TestJOSEMarshalerRequestBody`, `TestReferencedSchemaNamesMarshalerRequestParams`, spectest validation, `TestRunGenerateStructMarshalerEdgeRoutesValidate`. (The first plan claimed this row and it was false: a struct parameter of a Marshaler request dangled, review findings 1/5.) |
| One per-type warning per field position and payload; `--strict` 1 on any, 0 on a text-only fixture | `TestMarshalerTypeFieldWarnings`, `TestMarshalerPayloadRows`, Task 7 |
| README revision (five points, plus the JOSE and params-only sentences of review round 1) | Task 8 |
| New golden with `domain` sub-package, structs using `domain` in the handlers' file | Task 6, Appendix B |
| No golden on main moves; PR 1/#100/#110 goldens move only if they declare a text or struct Marshaler type (none does) | Global Constraints, Task 6 Step 3 |
| ≥80% new-code coverage | Task 9 |
| Mutation: string rule → Level, inlining → Money, promotion → WrapLevel | Task 9, mutations 1–3 |
| `make check`, cognitive ≤15 | Task 9, budget table |
| `fix(analyzer):`, Impact covers strings-pass and structs-fail | Task 10, Impact |
| Zero-field struct Marshaler requests (body, warning, untyped, `--strict`) and the short-name hand-off (`BMoney` → `Money`) are stated and pinned (review round 3) | Impact items 3 and 5, README (Task 8), commit body, PR `## Impact`; `TestZeroFieldMarshalerRequest`, `TestRunGenerateZeroFieldMarshalerRequest`, `TestStructMarshalerFreesShortName`, mutations 2 and 13 |
| Warnings lost with a struct Marshaler type's fields, and the request-field residual, are stated and pinned | Impact item 7, commit body, PR Impact, README; `TestRunGenerateTextMarshalersStrictClean`, `TestMarshalerRequestTagWarnings` |

## Review dispositions (round 1, 2026-10-09)

Six findings, three distinct issues; all accepted after checking each against `533353f` and reproducing it on the revised prototype.

| Findings | Verdict | Disposition |
|---|---|---|
| 1 (blocker), 5 (major): a struct parameter of a struct Marshaler request dangles (`Filter`, `Opaque`, `Empty`) | **Accepted** | Confirmed: `routeTypeRegistry` and `addFieldSchemaRefs` only see registry types, the request is no longer one, and `generateSchemasFromTypes` drops a params-only or nil-schema struct nobody marks. Fix: `addRequestFieldRefs` in `referencedSchemaNames`, called for every request (for a registered one it repeats `addFieldSchemaRefs`; it never marks the request's own name, so the CLAUDE.md invariant's rationale holds). Called unconditionally rather than only on `Resolution != nil`, because the params-only arm (finding 4) keeps a request out of the registry without a Resolution. Row M16, `TestReferencedSchemaNamesMarshalerRequestParams`, edge-routes CLI test, mutation 10. |
| 2 (major), 3 (blocker), 6 (major): a jose-tagged struct Marshaler request or response loses `application/jose`, `JOSEErrorEnvelope`, 401 and 415 | **Accepted**; open decision 1's default flipped | Confirmed: `JOSE` is set only at registration (analyzer.go:3211/3463), which the gate skips; `hasJOSETag`'s own doc says a jose-tagged type is never plain JSON. Fix: the gate sets `JOSE` through `structMarshalerJOSE` (reusing `hasJOSETag`, not a new `lookupStructTag` scan, so there stays one jose detector) for requests and responses alike; `joseDescription("")` returns a nameless const; responses reuse the existing `SuccessResponse` fallback. Task 4's JOSE assertion is now `JOSE == true, Name == ""`; the commit-body residual is deleted; Impact item 8 lists what still changes. Row M17, mutations 9 and 12. |
| 4 (major): a params-only request whose method set holds a Marshaler method gains a body and a warning (on a GET too) | **Accepted**, option (a) | Confirmed on `rv_get`. Fix: `populateMarshalerRequest` returns early for a params-only (≥1 field, all parameters), non-JOSE request with Name and parameter Fields kept, no Resolution, no warning. This is byte-identical to the base on `rv_get` and keeps doctor's typed count. It is not gated on "has a non-param field", so the zero-field text request keeps its string body, and a JOSE one keeps its JOSE body. *(Corrected in round 3: the base had no body for a zero-field request, so it gains one; see round 3, finding 1.)* Row U5, mutation 11. Not adopted: letting it register normally (the advisor's variant), which would put a struct Marshaler type in the registry against the brief's "never registered", and would need the params-only test (a field extraction) to run before the gate. |


## Review dispositions (round 2, 2026-10-09)

Two findings; both accepted after reproducing them on the reviewer's binaries (`scratchpad/pr4/cr3/{base,after}`, projects `e7`, `e1`) and on `proto3`.

| Finding | Verdict | Disposition |
|---|---|---|
| 1 (major): the warning and `--strict` impact is only partly listed | **Accepted**; request side kept as a residual | Confirmed on `e7`, `e1` and `r2w`: the base's malformed-tag, `uintptr`, unresolvable-type and Marshaler-field warnings inside a field-reached struct Marshaler type are gone after, and a text one flips `--strict` 1 → 0; a struct Marshaler request still warns on its body fields' tags. Impact item 7, the commit body, the PR `## Impact`, the README bullet and the `structMarshalerLeaf`/`requestParamFields` prose now say both. Request side **not suppressed**: at extraction a body field cannot be told from a parameter whose mangled tag hid its `param`/`query`/`header` key, and `r2w`'s `Q` shows that warning is the only signal a parameter vanished. Pinned by `TestMarshalerRequestTagWarnings` (residual) and the malformed-tag/`uintptr` `MoneyT` in `TestRunGenerateTextMarshalersStrictClean` (loss). |
| 2 (major): `MoneyP`, `MoneyU`, `MA`, `domain.Price` covered only as bare `T` | **Accepted** | Confirmed against Appendix B (only `Money` had four positions). `Fields` gains `*T`, `[]T`, `map[string]T` for each of the four (gofmt realigned the whole block, so Appendix B's `Fields` is replaced in full); rows M5–M7, Task 3's two tests, the counts (30 → 42 warnings, 831 → 859 golden lines) and the self-review row are updated. `proto3`: golden additive, no `nullable`, no new component, `go test ./...` green. |

## Review dispositions (round 3, 2026-10-09)

Two findings; both accepted after reproducing them with the reviewer's binaries (`scratchpad/pr4/cr4/base_bin` = `533353f`, `after_bin` = `533353f` + Appendix A) on `rv5probe`, `cr4/p1` and a new `rv6/zf` (`Ping`, `Blank`, `Plain`). Neither needs a code change; Appendix A stands.

| Finding | Verdict | Disposition |
|---|---|---|
| 1 (major): a zero-field struct Marshaler request gains a body, a warning and an untyped classification, and the plan says it "keeps its string body" | **Accepted**, option (b) (keep and state it); base parity recorded as open decision 6 | Confirmed: `Ping struct{}` + `UnmarshalJSON` on a GET gains `requestBody {schema: {}}` and one request warning, `--strict` 0 → 1, doctor "Ready with caveats" (the route stays typed in doctor only through its typed response; the request is untyped); `Blank struct{}` with the text pair gains a `{type: string}` body, no warning; `Plain struct{}` and `Hidden{ID param; secret string}` are unchanged. The "keeps its string body" sentence was wrong: the base had no body there. Corrected in **Payloads and requests** (and flagged in round 1's disposition), the Goal, Architecture, Data model and Doctor prose; stated in Impact item 5, README (Task 8), commit body and PR `## Impact`; extra rows `Ping`, `Blank`, `Hidden`; pinned by `TestZeroFieldMarshalerRequest`, `TestRunGenerateZeroFieldMarshalerRequest` and mutation 13. Not base parity, because the brief makes a struct with `UnmarshalJSON` `{}` and a zero-field decoder does read the body, while round 1's carve-out rests on every field being a parameter. |
| 2 (major): a freed short name moves a same-named plain struct of another package (`BMoney` → `Money`) and nothing says so | **Accepted** | Confirmed on `cr4/p1`: `BMoney` disappears, `Money` now holds `b.Money` (`cents`), its `$ref`s move, `a` is `{}`, one warning (for `a.Money` only). Cause: `schemaKey` (`analyzer.go:3492`) gives the short name to the first registrant, and a struct Marshaler type no longer registers. No base golden moves (no fixture declares a struct Marshaler type). Stated in Impact item 3, the BREAKING CHANGE footer, PR `## Impact` (now 133 words) and the README bullet, in PR 3's wording; pinned by `TestStructMarshalerFreesShortName` (Task 3) and added to mutation 2. Reserving the name without registering is open decision 7's rejected alternative. |

---

## Execution notes (2026-10-09)

Executed on `fix/marshaler-types`, cut from `fix/payload-resolution` at `533353f`; one commit, this plan included (the orchestrator asked for it, overriding Task 0 Step 1's "leave it untracked").

- **Spec currency (Task 0 Step 3).** #111 has two comments; the newer, `2026-10-04T04:47Z`, is the "Brief amendment (2026-10-03)" itself (UTC vs. local date). Nothing supersedes this plan.
- **Applying Appendix A.** `git apply --recount` misread the `--- a/` boundaries; the appendix was split per file and applied with `patch -p1`, in task order (the two small `resolution.go` hunks, `fallbackMarshalerPromoted` and `fieldFallback.via`, by hand at Task 2, the rest at Task 3). **Plan defect:** the last `payload.go` hunk's header says `+175,122` but carries 124 new lines, so `patch` dropped the closing `\t}` and `}` of `warnMarshalerRequest`; restored by hand. Every other hunk's counts match. No other code deviates from Appendix A.
- **Doc edits** (absent from Appendix A) written fresh: `models.go` (`ShapeText` in `TypeShape`'s list, `ShapeMarshaler.Elem`, `TypeInfo.Resolution`, `TypeInfo.JOSE`), `quiet()`, doctor's `isTypedPayload`, `setTypeAndFormat`, `buildOperation`, `buildRequestBody`, `referencedSchemaNames` (new paragraph on `addRequestFieldRefs`), `applyValidationConstraints`/`isMarshalerLeaf`, README (Marshaler bullet rewritten, payload bullet), CLAUDE.md (bullet superseded in place).
- **Test layout.** Task 2's tests are in `marshaler_test.go`; `TestMethodSetOf` is split into `TestMethodSetOfOwnAndAliased` and `TestMethodSetOfPromoted`, over a written project (`writeMTProject`) with `a.modulePath` set directly so `WithCode`'s cross-package row resolves. Task 3's tests and the shared sources (`mtTypesSrc`, `mtExtraSrc`, `mtFieldsSrc`) are in `marshaler_types_test.go`; Task 4's in a new `marshaler_payload_test.go`; Task 5's in a new `internal/generator/marshaler_types_test.go`, plus table rows in `TestSetTypeAndFormatResolutionLeaves`, `TestAddFieldSchemaRefsWalksResolution`, `TestWarnPayloadEveryKind` and `TestIsTypedPayload`. The test helper `renderShape` gained a `text:` case.
- **`TestStructMarshalerUnderlyingMode`:** the leaf of `type D q.UA` is named `UA` (the qualified alias, as on the base: the control row is `$UA`), so the assertions are `text:UA` and `$UA`; the kinds are what the plan pins.
- **Task 7:** generate prints warnings to stderr, which `testutil.CaptureStdout` does not capture, so the tests read warning texts from `analyzer.Warnings` and only the `Warnings: N` summary from stdout. `TestDoctorClassifiesMarshalerTypes` asserts `calculateProjectStats` (untyped `money` and `custom` routes, 2 typed requests, 1 typed response), as `TestDoctorClassifiesPayloads` does.
- **Lint fixes:** a test `HandlerName: "jose"` pushed the package's `"jose"` literal over goconst (renamed `joseRoute`); prealloc and revive (error-return order) in test helpers.
- **Mutation hardening:** under mutation 3 `TestMarshalerRequest` dereferenced a nil Resolution and the panic hid every later analyzer test; it now asserts per row without panicking.
- **Results:** `git status --porcelain internal/spectest/testdata` after `-update` listed only `marshaler_types/`; `expected.yaml` matches every S/M/U row, 859 non-empty lines, components exactly `Empty`, `ErrorResponse`, `Fields`, `Filter`, `JOSEErrorEnvelope`, `MD`, `MoneyOdd`, `Pair`, `SuccessResponse`; `generate --validate` on it prints 42 warnings, the aggregate naming `money, moneyPtr, moneys, stamped, jose`. gocognit: every touched function within the budget table, only the two baseline entries over 15. Coverage: every new or changed function 100% from its own package's tests. All 13 mutations fail the tests the plan names (mutation 13 moves no golden, as expected).

## Review dispositions (post-execution review, 2026-10-09)

Six findings on the executed commit; five accepted, the sixth (commit-body wrapping) fixed in the amend.

| Finding | Verdict | Disposition |
|---|---|---|
| Major: `methodSetOf`'s nine-deep cut is silent and order-dependent (a memo entry computed on a shorter path escapes it) | **Accepted**, option (a) | The depth cap is gone; `open` still cuts cycles, which is exact (a re-entered type only re-promotes shadowed methods). Pinned by `TestMethodSetOfUncappedAndOrderFree` (12-deep chain, both walk orders) and `TestMethodSetOfCycleCutNotMemoized`; `E0`/`E1` are now text rows. |
| Major: a pointer-only `MarshalText` (`LevelP`, `EmbP{LevelP}`) is documented as a string, but `encoding/json` writes the underlying type for a non-addressable value (map value, `Result[T]` by value) | **Accepted**; reverses rows S4, S8 (`LevelPMap`) and S14 and the brief's matching rows, per ADR 0002 ("text in both directions"; "`{}` accepts both forms") | Method-set entries record `ptrOnly`; a pointer embed lifts it, a value embed keeps it; `textBothWays` requires `MarshalText` in the value method set (`UnmarshalText` may stay on the pointer). `LevelP`, `LevelPMap`, `EmbP` are `{}` + warn; new fixture row `PtrEmbP{*LevelP}` is a string. A `*LevelP` field stays `{}` too (conservative; position-aware typing would be a follow-up). |
| Minor: a JOSE text-Marshaler response's description claims the `SuccessResponse` schema | **Accepted** | `successPlaintextSchema` returns `""` for a response carrying a Resolution, so the description is `joseUndocumentedDescription`, and `createStandardSchemas` (now keyed on `successPlaintextSchema`) stops emitting `SuccessResponse` for it. Supersedes row M17's response description and Impact item 6's `SuccessResponse` clause. |
| Minor: `time.Time`'s table entry lacks `AppendText` (Go >= 1.24) | **Accepted**; supersedes open decision 2 | Added; `TimeTie{time.Time; AllFive; Note}` pins that the tie cancels all five. |
| Minor: `ShapeMarshaler.Elem` comment says it is "used for the byte-slice decision" | **Accepted** (reworded) | Elem is still set; the comment now says it is kept for diagnostics and tests, and that the byte-slice decision reads the local value before the leaf is built. |

## Appendix A: prototype diff (applied to `533353f`, lint-clean, `go test ./...` green; revised after review round 1)

Apply hunk by hunk; anchors may drift by a few lines. The comment edits of Task 8 are not included. The `payload.go`, `analyzer.go` and `openapi.go` hunks are the **revised** prototype's (`scratchpad/pr4/proto2`, review round 1): they already carry the swapped check order, the JOSE flag, the params-only arm and `addRequestFieldRefs`.

```diff
--- a/internal/models/models.go
+++ b/internal/models/models.go
@@ -218,6 +218,12 @@
 	// ShapeRecursive is a named type met again inside its own resolution, or
 	// past maxNamedResolutionDepth. Name is that type. The cut point emits {}.
 	ShapeRecursive ShapeKind = "recursive"
+	// ShapeText is a Marshaler type encoding/json writes and reads as a JSON
+	// string on every toolchain (#111): MarshalText with no MarshalJSON or
+	// MarshalJSONTo, and UnmarshalText with no UnmarshalJSON or
+	// UnmarshalJSONFrom, declared or promoted. Name is the type. It emits
+	// {type: string} and takes no validate keyword.
+	ShapeText ShapeKind = "text"
 )
 
 // TypeShape is the syntactic container structure of a field's declared type,
--- a/internal/analyzer/marshaler.go
+++ b/internal/analyzer/marshaler.go
@@ -1,7 +1,13 @@
 package analyzer
 
-import "go/ast"
+import (
+	"go/ast"
+	"slices"
+	"strings"
 
+	"github.com/gaborage/go-bricks-openapi/internal/models"
+)
+
 // The Marshaler guard (CONTEXT.md, "Marshaler type"): a named type with any of
 // encoding/json's seven own-format methods is written through that method, not
 // through its underlying type, so its underlying type documents nothing about
@@ -21,15 +27,39 @@
 	methodUnmarshalJSONFrom = "UnmarshalJSONFrom"
 )
 
+// methodBits is a set of the seven methods, one bit each.
+type methodBits uint8
+
+const (
+	bitMarshalJSON methodBits = 1 << iota
+	bitMarshalText
+	bitMarshalJSONTo
+	bitAppendText
+	bitUnmarshalJSON
+	bitUnmarshalText
+	bitUnmarshalJSONFrom
+)
+
 // marshalerMethods is what a type's own method set says about its JSON form.
 type marshalerMethods struct {
-	encode bool   // MarshalJSON, MarshalJSONTo, MarshalText or AppendText
-	decode bool   // UnmarshalJSON, UnmarshalJSONFrom or UnmarshalText
-	first  string // the first matching method name met, for the warning
+	encode bool       // MarshalJSON, MarshalJSONTo, MarshalText or AppendText
+	decode bool       // UnmarshalJSON, UnmarshalJSONFrom or UnmarshalText
+	first  string     // the first matching method name met, for the warning
+	has    methodBits // every matching method
 }
 
 func (m marshalerMethods) found() bool { return m.encode || m.decode }
 
+// textBothWays reports whether encoding/json writes and reads the type as a
+// JSON string on every toolchain (ADR 0002): MarshalText with no MarshalJSON
+// or MarshalJSONTo, and UnmarshalText with no UnmarshalJSON or
+// UnmarshalJSONFrom. AppendText alone does not count: the classic
+// encoding/json ignores it and writes the underlying type.
+func (m marshalerMethods) textBothWays() bool {
+	const jsonSpecific = bitMarshalJSON | bitMarshalJSONTo | bitUnmarshalJSON | bitUnmarshalJSONFrom
+	return m.has&bitMarshalText != 0 && m.has&bitUnmarshalText != 0 && m.has&jsonSpecific == 0
+}
+
 // typePred tests one parameter or result type; imports maps the method's own
 // file's import names to paths.
 type typePred func(e ast.Expr, imports map[string]string) bool
@@ -40,6 +70,7 @@
 	params  []typePred
 	results []typePred
 	encode  bool
+	bit     methodBits
 }
 
 var (
@@ -48,14 +79,30 @@
 
 	// marshalerSignatures is the single table of the seven methods.
 	marshalerSignatures = map[string]methodSignature{
-		methodMarshalJSON:       {results: []typePred{isByteSliceExpr, isErrorIdent}, encode: true},
-		methodMarshalText:       {results: []typePred{isByteSliceExpr, isErrorIdent}, encode: true},
-		methodMarshalJSONTo:     {params: []typePred{jsontextEncoderPtr}, results: []typePred{isErrorIdent}, encode: true},
-		methodAppendText:        {params: []typePred{isByteSliceExpr}, results: []typePred{isByteSliceExpr, isErrorIdent}, encode: true},
-		methodUnmarshalJSON:     {params: []typePred{isByteSliceExpr}, results: []typePred{isErrorIdent}},
-		methodUnmarshalText:     {params: []typePred{isByteSliceExpr}, results: []typePred{isErrorIdent}},
-		methodUnmarshalJSONFrom: {params: []typePred{jsontextDecoderPtr}, results: []typePred{isErrorIdent}},
+		methodMarshalJSON:       {results: []typePred{isByteSliceExpr, isErrorIdent}, encode: true, bit: bitMarshalJSON},
+		methodMarshalText:       {results: []typePred{isByteSliceExpr, isErrorIdent}, encode: true, bit: bitMarshalText},
+		methodMarshalJSONTo:     {params: []typePred{jsontextEncoderPtr}, results: []typePred{isErrorIdent}, encode: true, bit: bitMarshalJSONTo},
+		methodAppendText:        {params: []typePred{isByteSliceExpr}, results: []typePred{isByteSliceExpr, isErrorIdent}, encode: true, bit: bitAppendText},
+		methodUnmarshalJSON:     {params: []typePred{isByteSliceExpr}, results: []typePred{isErrorIdent}, bit: bitUnmarshalJSON},
+		methodUnmarshalText:     {params: []typePred{isByteSliceExpr}, results: []typePred{isErrorIdent}, bit: bitUnmarshalText},
+		methodUnmarshalJSONFrom: {params: []typePred{jsontextDecoderPtr}, results: []typePred{isErrorIdent}, bit: bitUnmarshalJSONFrom},
 	}
+
+	// marshalerMethodOrder fixes the order a method set is read in, so the
+	// method a warning names is deterministic.
+	marshalerMethodOrder = []string{
+		methodMarshalJSON, methodMarshalJSONTo, methodMarshalText, methodAppendText,
+		methodUnmarshalJSON, methodUnmarshalJSONFrom, methodUnmarshalText,
+	}
+
+	// wellKnownMethodSets are the out-of-module types whose methods are known
+	// without their source, read only when one is embedded (promotion): used
+	// directly, each keeps its well-known schema.
+	wellKnownMethodSets = map[string][]string{
+		models.WellKnownTimeTime:   {methodMarshalJSON, methodMarshalText, methodUnmarshalJSON, methodUnmarshalText},
+		models.WellKnownUUID:       {methodMarshalText, methodUnmarshalText},
+		models.WellKnownRawMessage: {methodMarshalJSON, methodUnmarshalJSON},
+	}
 )
 
 // marshalerSet scans every file of c's package for methods of name (receiver
@@ -73,6 +120,19 @@
 // methodsInFile folds the matching methods of name declared in file into m.
 // The file's imports are resolved once, and only when a candidate appears.
 func (a *ProjectAnalyzer) methodsInFile(file *ast.File, name string, m marshalerMethods) marshalerMethods {
+	a.visitMarshalerMethods(file, name, func(method string, exact bool) {
+		if exact {
+			m = m.with(method, marshalerSignatures[method].encode)
+		}
+	})
+	return m
+}
+
+// visitMarshalerMethods calls visit for every method of name (receiver name
+// or *name) declared in file under one of the seven names, with whether its
+// signature is exact. The file's imports are resolved once, and only when a
+// candidate appears.
+func (a *ProjectAnalyzer) visitMarshalerMethods(file *ast.File, name string, visit func(method string, exact bool)) {
 	var imports map[string]string
 	for _, decl := range file.Decls {
 		fd, sig := a.marshalerCandidate(decl, name)
@@ -82,11 +142,8 @@
 		if imports == nil {
 			imports = a.fileImports(file)
 		}
-		if matchMarshalerSignature(fd, sig, imports) {
-			m = m.with(fd.Name.Name, sig.encode)
-		}
+		visit(fd.Name.Name, matchMarshalerSignature(fd, sig, imports))
 	}
-	return m
 }
 
 // marshalerCandidate returns decl and the signature it must have when decl is
@@ -111,6 +168,7 @@
 	}
 	m.encode = m.encode || encode
 	m.decode = m.decode || !encode
+	m.has |= marshalerSignatures[method].bit
 	return m
 }
 
@@ -182,5 +240,187 @@
 		}
 		pkg, ok := sel.X.(*ast.Ident)
 		return ok && imports[pkg.Name] == jsontextImportPath
+	}
+}
+
+// methodEntry is one of the seven method names in a type's method set, as
+// Go's selector rules see it (#111).
+type methodEntry struct {
+	depth     int    // 0: declared on the type or a type it aliases; n: promoted through n embeddings
+	exact     bool   // the selected method has the exact interface signature
+	ambiguous bool   // two embeds tie at the shallowest depth: in no method set, and it hides deeper ones
+	owner     string // depth 0: the declaring type, as a warning names it
+	via       string // depth > 0: the embedded field it is promoted through, as written
+}
+
+// methodSet maps each of the seven names a type's method set holds to its
+// entry. A name absent from the map is not in the set.
+type methodSet map[string]methodEntry
+
+// over adds to ms every name of deeper that ms lacks: a shallower name
+// shadows a deeper one.
+func (ms methodSet) over(deeper methodSet) methodSet {
+	for name, e := range deeper {
+		if _, ok := ms[name]; !ok {
+			ms[name] = e
+		}
+	}
+	return ms
+}
+
+// marshaler reads ms in marshalerMethodOrder: what its exact, unambiguous
+// entries say about the type's JSON form, and the fallback the first one
+// names (typeName is the struct a promoted method belongs to).
+func (ms methodSet) marshaler(typeName string) (m marshalerMethods, fb fieldFallback) {
+	for _, method := range marshalerMethodOrder {
+		e, ok := ms[method]
+		if !ok || !e.exact || e.ambiguous {
+			continue
+		}
+		if !m.found() {
+			fb = e.fallback(typeName, method)
+		}
+		m = m.with(method, marshalerSignatures[method].encode)
+	}
+	return m, fb
+}
+
+// fallback is the warning an entry raises when its method makes the type {}.
+func (e methodEntry) fallback(typeName, method string) fieldFallback {
+	if e.depth == 0 {
+		return fieldFallback{kind: fallbackMarshaler, typeName: e.owner, detail: method}
 	}
+	return fieldFallback{kind: fallbackMarshalerPromoted, typeName: typeName, detail: method, via: e.via}
 }
+
+// embedMethods is the method set of one embedded field, and the field's type
+// as written (its pointer shed), which its promoted entries name.
+type embedMethods struct {
+	via string
+	set methodSet
+}
+
+// promote folds the method sets of a struct's embedded fields into the set
+// they promote into the struct (Go's selector rules): each name at its
+// shallowest depth plus one, from the one embed providing it there; two
+// embeds tying at that depth make it ambiguous.
+func promote(embeds []embedMethods) methodSet {
+	out := methodSet{}
+	for _, em := range embeds {
+		for name, e := range em.set {
+			cand := methodEntry{depth: e.depth + 1, exact: e.exact, ambiguous: e.ambiguous, via: em.via}
+			prev, seen := out[name]
+			switch {
+			case !seen || cand.depth < prev.depth:
+				out[name] = cand
+			case cand.depth == prev.depth:
+				out[name] = methodEntry{depth: cand.depth, ambiguous: true}
+			}
+		}
+	}
+	return out
+}
+
+// declaredMethods returns the methods declared on name (receiver name or
+// *name) in every file of c's package under one of the seven names, exact or
+// not: a non-exact one is in the method set, so it shadows a promoted one.
+func (a *ProjectAnalyzer) declaredMethods(name string, c resolveCtx) methodSet {
+	ms := methodSet{}
+	owner := c.display(name)
+	for _, pf := range a.samePackageFiles(c.file, c.path) {
+		a.visitMarshalerMethods(pf.file, name, func(method string, exact bool) {
+			e := ms[method]
+			ms[method] = methodEntry{exact: e.exact || exact, owner: owner}
+		})
+	}
+	return ms
+}
+
+// methodSetOf returns the method set of the named type name declared in c's
+// package: the methods declared on it (when keep) and on every type it
+// aliases, over what the struct at the end of its chain promotes. A defined
+// type drops its target's declared methods but keeps what the target's struct
+// promotes. open holds the typeKeys being walked: a type met again (S{*S})
+// adds nothing, and the walk stops maxNamedResolutionDepth names deep.
+func (a *ProjectAnalyzer) methodSetOf(name string, c resolveCtx, keep bool, open []string) methodSet {
+	key := c.typeKey(name)
+	if slices.Contains(open, key) || len(open) >= maxNamedResolutionDepth {
+		return methodSet{}
+	}
+	own := methodSet{}
+	if keep {
+		own = a.declaredMethods(name, c)
+	}
+	ts, file, path, ok := a.resolveLocalTypeSpec(c.file, c.path, name)
+	if !ok {
+		return own
+	}
+	open = append(slices.Clip(open), key)
+	return own.over(a.rhsMethodSet(ts.Type, c.in(file, path), keep && ts.Assign.IsValid(), open))
+}
+
+// rhsMethodSet returns the method set a declaration's right-hand side
+// contributes: what a struct literal promotes, or the set of the named type
+// it names (keep false for a defined type's target).
+func (a *ProjectAnalyzer) rhsMethodSet(rhs ast.Expr, c resolveCtx, keep bool, open []string) methodSet {
+	switch t := rhs.(type) {
+	case *ast.StructType:
+		return a.promotedMethods(t, c, open)
+	case *ast.Ident:
+		return a.methodSetOf(t.Name, c, keep, open)
+	case *ast.SelectorExpr:
+		return a.qualifiedMethodSet(a.typeShape(t).Name, c, keep, open)
+	default:
+		return methodSet{}
+	}
+}
+
+// qualifiedMethodSet returns the method set of q.T: an in-module type's, in
+// its declaring package, else a well-known type's (wellKnownMethodSets), else
+// none (a third-party type is not read).
+func (a *ProjectAnalyzer) qualifiedMethodSet(qualified string, c resolveCtx, keep bool, open []string) methodSet {
+	if site, name, ok := a.inModuleTypeSite(qualified, c.file); ok {
+		return a.methodSetOf(name, c.in(site.file, site.path), keep, open)
+	}
+	ms := methodSet{}
+	if !keep {
+		return ms
+	}
+	for _, method := range wellKnownMethodSets[qualified] {
+		ms[method] = methodEntry{exact: true, owner: qualified}
+	}
+	return ms
+}
+
+// promotedMethods returns the method set a struct literal's embedded fields
+// promote, whatever their json tag: by value, by pointer, local, from another
+// package of the module, or well-known.
+func (a *ProjectAnalyzer) promotedMethods(st *ast.StructType, c resolveCtx, open []string) methodSet {
+	var embeds []embedMethods
+	for _, f := range st.Fields.List {
+		if len(f.Names) == 0 {
+			embeds = append(embeds, a.embedMethodSet(f, c, open))
+		}
+	}
+	return promote(embeds)
+}
+
+// embedMethodSet returns one embedded field's method set: a pointer embed's
+// is its target's (a pointer receiver counts in every position). A generic
+// or builtin embed contributes nothing.
+func (a *ProjectAnalyzer) embedMethodSet(f *ast.Field, c resolveCtx, open []string) embedMethods {
+	s := a.typeShape(f.Type)
+	if s.Kind == models.ShapePointer && s.Elem != nil {
+		s = *s.Elem
+	}
+	em := embedMethods{via: s.Name}
+	switch {
+	case s.Kind != models.ShapeNamed:
+		em.set = methodSet{}
+	case strings.Contains(s.Name, "."):
+		em.set = a.qualifiedMethodSet(s.Name, c, true, open)
+	default:
+		em.set = a.methodSetOf(s.Name, c, true, open)
+	}
+	return em
+}
--- a/internal/analyzer/resolution.go
+++ b/internal/analyzer/resolution.go
@@ -45,7 +45,9 @@
 		"— emitting an untyped object"
 	unregisteredRefFieldWarning = "field %s at %s has type %s: struct %s could not be registered as a component " +
 		"— emitting an untyped object"
-	unknownFieldPosition = "unknown position"
+	unknownFieldPosition          = "unknown position"
+	marshalerPromotedFieldWarning = "field %s at %s has type %s: %s has the %s method promoted from its embedded %s, which " +
+		"encoding/json uses instead of its fields — emitting an untyped schema ({}); parameters are typed by kind and unaffected"
 )
 
 type resolveMode int
@@ -66,6 +68,7 @@
 	fallbackDefinedOverQualified
 	fallbackDeclsDisagree
 	fallbackUntypedBuiltin
+	fallbackMarshalerPromoted
 )
 
 // fieldFallback is the first fallback a field's resolution met.
@@ -76,6 +79,9 @@
 	// underlying (fallbackDefinedOverQualified), or the builtin
 	// (fallbackUntypedBuiltin).
 	detail string
+	// via is the embedded field a promoted Marshaler method comes through
+	// (fallbackMarshalerPromoted), as written.
+	via string
 }
 
 // resolveState is the state shared by one field's resolution.
@@ -287,7 +293,10 @@
 // an in-module named type resolves in its declaring package (inModuleTypeSite),
 // and anything else is unresolvable (it emits object).
 func (a *ProjectAnalyzer) resolveQualified(leaf models.TypeShape, c resolveCtx) models.TypeShape {
-	if _, ok := a.resolveQualifiedStruct(leaf.Name, c.file); ok {
+	if q, ok := a.resolveQualifiedStruct(leaf.Name, c.file); ok {
+		if r, isMarshaler := a.structMarshalerLeaf(q.typeName, c.in(q.file, q.filePath), identityMode); isMarshaler {
+			return r
+		}
 		c.recordRef(leaf.Name)
 		return models.TypeShape{Kind: models.ShapeRef, Name: leaf.Name}
 	}
@@ -386,6 +395,9 @@
 func (a *ProjectAnalyzer) resolveLocal(leaf models.TypeShape, c resolveCtx, mode resolveMode) models.TypeShape {
 	name := leaf.Name
 	if _, _, _, _, ok := a.resolveTypeSpecChain(c.file, c.path, name, 0); ok {
+		if r, isMarshaler := a.structMarshalerLeaf(name, c, mode); isMarshaler {
+			return r
+		}
 		c.recordRef(name)
 		return models.TypeShape{Kind: models.ShapeRef, Name: name}
 	}
@@ -413,12 +425,36 @@
 // base64 (#97). The rule is decided here, in the declaring package's context,
 // because only here are the type's own methods known.
 func (a *ProjectAnalyzer) marshalerLeaf(name string, decls []typeDecl, c resolveCtx, m marshalerMethods) models.TypeShape {
+	if m.textBothWays() {
+		return models.TypeShape{Kind: models.ShapeText, Name: name}
+	}
 	under := a.resolveDecls(name, decls, c.quiet(), underlyingMode)
 	if c.byteElem && !m.encode && isBytePrimitive(&under) {
 		return under
 	}
 	c.st.note(fieldFallback{kind: fallbackMarshaler, typeName: c.display(name), detail: m.first})
 	return models.TypeShape{Kind: models.ShapeMarshaler, Name: name, Elem: &under}
+}
+
+// structMarshalerLeaf resolves name, a named type whose chain ends at a
+// struct, in a body position: a text leaf when its method set (declared and
+// promoted: methodSetOf) makes it a string both ways, a noted Marshaler leaf
+// when it holds any other of the seven methods, else ok is false and it stays
+// a ref. In underlying mode name's declared methods are dropped. A parameter
+// is bound by kind, so it is never a Marshaler leaf.
+func (a *ProjectAnalyzer) structMarshalerLeaf(name string, c resolveCtx, mode resolveMode) (models.TypeShape, bool) {
+	if c.st.param {
+		return models.TypeShape{}, false
+	}
+	m, fb := a.methodSetOf(name, c, mode == identityMode, nil).marshaler(c.display(name))
+	if !m.found() {
+		return models.TypeShape{}, false
+	}
+	if m.textBothWays() {
+		return models.TypeShape{Kind: models.ShapeText, Name: name}, true
+	}
+	c.st.note(fb)
+	return models.TypeShape{Kind: models.ShapeMarshaler, Name: name}, true
 }
 
 // resolveDecls resolves every declaration of name with name open, then merges
@@ -708,6 +744,8 @@
 		a.addWarningf(definedOverQualifiedFieldWarning, append(head, fb.typeName, fb.detail)...)
 	case fallbackDeclsDisagree:
 		a.addWarningf(declsDisagreeFieldWarning, append(head, fb.typeName)...)
+	case fallbackMarshalerPromoted:
+		a.addWarningf(marshalerPromotedFieldWarning, append(head, fb.typeName, fb.detail, fb.via)...)
 	default:
 		a.addWarningf(untypedBuiltinFieldWarning, append(head, fb.detail)...)
 	}
--- a/internal/analyzer/payload.go
+++ b/internal/analyzer/payload.go
@@ -32,6 +32,13 @@
 		"— emitting an untyped schema (use a sized integer type)"
 	unregisteredRefPayloadWarning = "response type %s: struct %s could not be registered as a component " +
 		"— emitting an untyped object"
+	marshalerPromotedPayloadWarning = "response type %s: %s has the %s method promoted from its embedded %s, which " +
+		"encoding/json uses instead of its fields — emitting an untyped schema ({})"
+	marshalerRequestWarning = "request type %s: %s has its own %s method, which encoding/json uses instead of its " +
+		"fields — emitting an untyped requestBody schema ({}); its path, query and header fields stay parameters"
+	marshalerPromotedRequestWarning = "request type %s: %s has the %s method promoted from its embedded %s, which " +
+		"encoding/json uses instead of its fields — emitting an untyped requestBody schema ({}); its path, query and " +
+		"header fields stay parameters"
 )
 
 // resolvePayload resolves a payload that did not register as a project struct
@@ -107,6 +114,8 @@
 		a.addWarningf(definedOverQualifiedPayloadWarning, written, fb.typeName, fb.detail)
 	case fallbackDeclsDisagree:
 		a.addWarningf(declsDisagreePayloadWarning, written, fb.typeName)
+	case fallbackMarshalerPromoted:
+		a.addWarningf(marshalerPromotedPayloadWarning, written, fb.typeName, fb.detail, fb.via)
 	default:
 		a.addWarningf(untypedBuiltinPayloadWarning, written, fb.detail)
 	}
@@ -166,9 +175,122 @@
 		return !models.WellKnownTypeNames[leaf.Name]
 	case models.ShapePrimitive:
 		return leaf.Name == goTypeUintptr || models.UntypedBuiltinNames[leaf.Name]
-	case models.ShapeRef, models.ShapeKindOnly:
+	case models.ShapeRef, models.ShapeKindOnly, models.ShapeText:
 		return false
 	default:
 		return true
 	}
 }
+
+// namedShape is the type a request or response TypeInfo names, spelled as the
+// Shape decoder would: bare when declared in astFile's package, qualified by
+// its import name otherwise (registerPayloadType's rule).
+func namedShape(ti *models.TypeInfo, astFile *ast.File) models.TypeShape {
+	name := ti.Name
+	if ti.Package != "" && ti.Package != astFile.Name.Name {
+		name = ti.Package + "." + name
+	}
+	return models.TypeShape{Kind: models.ShapeNamed, Name: name}
+}
+
+// marshalerNamed resolves the type ti names in a body position and reports
+// whether it is a Marshaler type (a text or Marshaler leaf), with the leaf and
+// the fallback it noted. A struct Marshaler type never registers (#111).
+func (a *ProjectAnalyzer) marshalerNamed(ti *models.TypeInfo, astFile *ast.File, filePath string) (models.TypeShape, fieldFallback, bool) {
+	if ti.Name == "" {
+		return models.TypeShape{}, fieldFallback{}, false
+	}
+	c := newResolveCtx(astFile, filePath, false)
+	r := a.resolveShape(namedShape(ti, astFile), c)
+	return r, c.st.first, r.Kind == models.ShapeMarshaler || r.Kind == models.ShapeText
+}
+
+// structSite is a struct declaration and the package and file declaring it.
+type structSite struct {
+	st   *ast.StructType
+	pkg  string
+	file *ast.File
+	path string
+}
+
+// payloadStructSite finds the struct a request TypeInfo names, as
+// registerPayloadType would: a local chain, or a struct of another package
+// of the module.
+func (a *ProjectAnalyzer) payloadStructSite(ti *models.TypeInfo, astFile *ast.File, filePath string) (structSite, bool) {
+	if ti.Package != "" && ti.Package != astFile.Name.Name {
+		q, ok := a.resolveQualifiedStruct(ti.Package+"."+ti.Name, astFile)
+		return structSite{st: q.st, pkg: q.pkg, file: q.file, path: q.filePath}, ok
+	}
+	st, pkg, file, path, ok := a.resolveTypeSpecChain(astFile, filePath, ti.Name, 0)
+	return structSite{st: st, pkg: pkg, file: file, path: path}, ok
+}
+
+// populateMarshalerRequest documents a request whose type is a struct
+// Marshaler type (#111): its body is the Marshaler leaf's schema ({} with one
+// warning, or a string), it names no component, and only its path, query and
+// header fields are kept, as parameters. It reports false for any other
+// request.
+func (a *ProjectAnalyzer) populateMarshalerRequest(h *handlerAnalysis, astFile *ast.File, filePath string) bool {
+	ti := h.request
+	leaf, fb, isMarshaler := a.marshalerNamed(ti, astFile, filePath)
+	if !isMarshaler {
+		return false
+	}
+	site, ok := a.payloadStructSite(ti, astFile, filePath)
+	if !ok {
+		return false
+	}
+	params, paramsOnly := a.requestParamFields(site, ti.Name)
+	ti.Fields = params
+	if paramsOnly && !ti.JOSE {
+		// Bound from parameters only (isParamsOnlyType's sense): no body is
+		// documented, as before #111; Name is kept so doctor counts it typed.
+		return true
+	}
+	ti.Name = ""
+	ti.Resolution = &leaf
+	a.warnMarshalerRequest(h.requestType, fb)
+	return true
+}
+
+// requestParamFields extracts a struct Marshaler request's fields without
+// registering it, and keeps its parameters: each warns and registers its refs
+// as registerStructKeyed would.
+func (a *ProjectAnalyzer) requestParamFields(s structSite, name string) ([]models.FieldInfo, bool) {
+	fields := a.extractStructFields(s.st, s.pkg, s.file, s.path, map[string]struct{}{name: {}}, 0)
+	total := len(fields)
+	params := fields[:0]
+	for i := range fields {
+		if fields[i].ParamType != "" {
+			params = append(params, fields[i])
+		}
+	}
+	for i := range params {
+		a.warnResolvedField(&params[i])
+	}
+	for i := range params {
+		a.registerFieldRefAt(&params[i], s.file, s.path, 1)
+	}
+	// Zero fields is not params-only: such a request reads its body through
+	// its own method, so it gets a body (review round 3, finding 1).
+	return params, total > 0 && len(params) == total
+}
+
+// structMarshalerJOSE reports whether the struct a Marshaler payload names
+// carries a jose: tag, as registerType would have recorded: the route stays
+// JOSE on the wire whatever the plaintext's methods.
+func (a *ProjectAnalyzer) structMarshalerJOSE(ti *models.TypeInfo, astFile *ast.File, filePath string) bool {
+	site, ok := a.payloadStructSite(ti, astFile, filePath)
+	return ok && hasJOSETag(site.st)
+}
+
+// warnMarshalerRequest raises a struct Marshaler request's one warning, or
+// none for a string-both-ways type. written is the request type as written.
+func (a *ProjectAnalyzer) warnMarshalerRequest(written string, fb fieldFallback) {
+	switch fb.kind {
+	case fallbackMarshaler:
+		a.addWarningf(marshalerRequestWarning, written, fb.typeName, fb.detail)
+	case fallbackMarshalerPromoted:
+		a.addWarningf(marshalerPromotedRequestWarning, written, fb.typeName, fb.detail, fb.via)
+	default:
+		// A string both ways: documented, no warning.
+	}
+}
--- a/internal/analyzer/analyzer.go
+++ b/internal/analyzer/analyzer.go
@@ -3176,6 +3176,9 @@
 			adoptRegisteredType(h.request, registered)
 			return
 		}
+		if a.populateMarshalerRequest(h, astFile, filePath) {
+			return
+		}
 	}
 	if h.requestType == "" {
 		return
@@ -3191,6 +3194,10 @@
 // returns its registered TypeInfo, or nil when the name is not a resolvable
 // struct.
 func (a *ProjectAnalyzer) registerPayloadType(typeInfo *models.TypeInfo, astFile *ast.File, filePath string) *models.TypeInfo {
+	if _, _, isMarshaler := a.marshalerNamed(typeInfo, astFile, filePath); isMarshaler {
+		typeInfo.JOSE = a.structMarshalerJOSE(typeInfo, astFile, filePath)
+		return nil
+	}
 	registered := a.registerType(typeInfo.Name, typeInfo.Package, astFile, filePath)
 	if registered == nil && typeInfo.Package != "" {
 		// A qualified request/response type (e.g. server.Result[types.Money]) arrives
--- a/internal/generator/openapi.go
+++ b/internal/generator/openapi.go
@@ -655,7 +655,7 @@
 	// JOSE-tagged (a JOSE request type may have only the sentinel field, with all
 	// "plaintext" fields filtered into header/path/query params or absent — but
 	// the route still expects an application/jose payload on the wire).
-	if route.Request != nil && (len(bodyFields) > 0 || route.Request.JOSE) {
+	if route.Request != nil && (len(bodyFields) > 0 || route.Request.JOSE || route.Request.Resolution != nil) {
 		op.RequestBody = g.buildRequestBody(route.Request)
 	}
 
@@ -693,6 +693,9 @@
 // objects do not), so this is attached at the parent level. It names the
 // plaintext component schema the decrypted payload conforms to.
 func joseDescription(plaintextSchema string) string {
+	if plaintextSchema == "" {
+		return joseUndocumentedDescription
+	}
 	return fmt.Sprintf(
 		"JOSE compact serialization (signed-then-encrypted). The wire payload\n"+
 			"is a base64url-encoded JWE compact form whose plaintext, after\n"+
@@ -701,6 +704,12 @@
 		plaintextSchema, refPath(plaintextSchema))
 }
 
+// joseUndocumentedDescription is joseDescription for a JOSE request whose
+// plaintext type is a struct Marshaler type (#111): it names no component.
+const joseUndocumentedDescription = "JOSE compact serialization (signed-then-encrypted). The wire payload\n" +
+	"is a base64url-encoded JWE compact form whose plaintext, after\n" +
+	"decrypt+verify, is its type's own JSON encoding, which is not documented.\n"
+
 // jsonMediaRef builds a single-entry application/json content map whose schema is
 // a $ref to the named component.
 func jsonMediaRef(name string) map[string]*OpenAPIMediaType {
@@ -1310,9 +1319,24 @@
 		if r.Request != nil && r.Request.JOSE && r.Request.Name != "" {
 			out[schemaName(r.Request)] = true
 		}
+		addRequestFieldRefs(out, r.Request)
 	}
 	addFieldSchemaRefs(out, types)
 	return out
+}
+
+// addRequestFieldRefs marks the $ref leaves of a request's own Fields. A
+// registered request's Fields are its registry entry's, so this repeats what
+// addFieldSchemaRefs marks; a struct Marshaler request (#111) is not in the
+// registry and keeps only parameters, whose struct targets would otherwise go
+// unmarked. It never marks the request's own name.
+func addRequestFieldRefs(out map[string]bool, req *models.TypeInfo) {
+	if req == nil {
+		return
+	}
+	for j := range req.Fields {
+		addRefNames(out, req.Fields[j].ResolvedShape())
+	}
 }
 
 // payloadNamesComponent reports whether a response payload points at the
@@ -1688,6 +1712,8 @@
 		prop.Ref = refPath(s.Name)
 	case models.ShapeKindOnly:
 		prop.Type = s.Name
+	case models.ShapeText:
+		prop.Type = typeString
 	case models.ShapeMarshaler, models.ShapeRecursive:
 		return // {}: any JSON value
 	default:
@@ -2032,6 +2058,8 @@
 		// schema; the Media Type schema describes the JOSE string-token wire shape.
 		rb.Description = joseDescription(schemaName)
 		rb.Content = map[string]*OpenAPIMediaType{mediaJOSE: {Schema: joseTokenSchema()}}
+	case reqType != nil && reqType.Resolution != nil:
+		rb.Content = map[string]*OpenAPIMediaType{mediaJSON: {Schema: payloadShapeSchema(*reqType.Resolution)}}
 	case schemaName != "":
 		rb.Content = jsonMediaRef(schemaName)
 	default:
--- a/internal/generator/constraints.go
+++ b/internal/generator/constraints.go
@@ -263,7 +263,8 @@
 
 // isMarshalerLeaf reports whether s, after every pointer, is a Marshaler leaf.
 func isMarshalerLeaf(s models.TypeShape) bool {
-	return stripPointers(s).Kind == models.ShapeMarshaler
+	k := stripPointers(s).Kind
+	return k == models.ShapeMarshaler || k == models.ShapeText
 }
 
 // sortedKeys returns the keys of m in lexicographic order so callers can iterate
```

## Appendix B: fixture `internal/spectest/testdata/marshaler_types/`

`go.mod`:

```
module github.com/example/marshaler_types

go 1.25

require github.com/gaborage/go-bricks v0.53.0
```

`domain/domain.go`:

```go
// Package domain declares Marshaler types another package of the module uses.
package domain

// Tag is an int written and read as text: a string both ways.
type Tag int

// MarshalText writes the tag name.
func (t Tag) MarshalText() ([]byte, error) { return []byte("t"), nil }

// UnmarshalText reads the tag name.
func (t *Tag) UnmarshalText(b []byte) error { return nil }

// Price is a struct written through its own MarshalJSON.
type Price struct {
	Cents int64 `json:"cents"`
}

// MarshalJSON writes the price.
func (p Price) MarshalJSON() ([]byte, error) { return []byte(`1`), nil }

// Amount is a struct written and read as text: a string both ways.
type Amount struct {
	Value int64 `json:"value"`
}

// MarshalText writes the amount.
func (a Amount) MarshalText() ([]byte, error) { return []byte("1 USD"), nil }

// UnmarshalText reads the amount.
func (a *Amount) UnmarshalText(b []byte) error { return nil }
```

`types.go`:

```go
package marshalertypes

import (
	"encoding/json"
	"encoding/json/jsontext"
	"time"

	"github.com/google/uuid"
)

// Level is written through a value MarshalText and read through a pointer
// UnmarshalText: a string both ways.
type Level int

// MarshalText writes the level name.
func (l Level) MarshalText() ([]byte, error) { return []byte("high"), nil }

// UnmarshalText reads the level name.
func (l *Level) UnmarshalText(b []byte) error { return nil }

// LA is an alias of Level: it keeps Level's methods.
type LA = Level

// LD is a defined type over Level: it drops Level's methods.
type LD Level

// LevelP has the text pair on its pointer only.
type LevelP int

// MarshalText writes the level name.
func (l *LevelP) MarshalText() ([]byte, error) { return []byte("p"), nil }

// UnmarshalText reads the level name.
func (l *LevelP) UnmarshalText(b []byte) error { return nil }

// StatusV only encodes as text: it decodes from a number.
type StatusV int

// MarshalText writes the status name.
func (s StatusV) MarshalText() ([]byte, error) { return []byte("active"), nil }

// LevelU only decodes as text.
type LevelU int

// UnmarshalText reads the level name.
func (l *LevelU) UnmarshalText(b []byte) error { return nil }

// Both has the text pair and a MarshalJSON, which wins on encode.
type Both int

// MarshalText writes the name.
func (b Both) MarshalText() ([]byte, error) { return nil, nil }

// UnmarshalText reads the name.
func (b *Both) UnmarshalText(p []byte) error { return nil }

// MarshalJSON wins over MarshalText.
func (b Both) MarshalJSON() ([]byte, error) { return nil, nil }

// TextJ has the text pair and an UnmarshalJSON, which wins on decode.
type TextJ int

// MarshalText writes the name.
func (x TextJ) MarshalText() ([]byte, error) { return nil, nil }

// UnmarshalText reads the name.
func (x *TextJ) UnmarshalText(p []byte) error { return nil }

// UnmarshalJSON wins over UnmarshalText.
func (x *TextJ) UnmarshalJSON(p []byte) error { return nil }

// Code is a string kind with the text pair.
type Code string

// MarshalText writes the code.
func (c Code) MarshalText() ([]byte, error) { return []byte(c), nil }

// UnmarshalText reads the code.
func (c *Code) UnmarshalText(b []byte) error { return nil }

// FlagT2 is a byte kind with the text pair: never base64.
type FlagT2 byte

// MarshalText writes the flag.
func (f FlagT2) MarshalText() ([]byte, error) { return nil, nil }

// UnmarshalText reads the flag.
func (f *FlagT2) UnmarshalText(b []byte) error { return nil }

// TextApp has MarshalText, AppendText and UnmarshalText: a string.
type TextApp int

// MarshalText writes the value.
func (x TextApp) MarshalText() ([]byte, error) { return nil, nil }

// AppendText appends the value.
func (x TextApp) AppendText(b []byte) ([]byte, error) { return b, nil }

// UnmarshalText reads the value.
func (x *TextApp) UnmarshalText(b []byte) error { return nil }

// AppOnly has AppendText and UnmarshalText but no MarshalText.
type AppOnly int

// AppendText appends the value.
func (x AppOnly) AppendText(b []byte) ([]byte, error) { return b, nil }

// UnmarshalText reads the value.
func (x *AppOnly) UnmarshalText(b []byte) error { return nil }

// ToText has MarshalJSONTo beside the text pair: the JSON method wins.
type ToText int

// MarshalJSONTo writes the value.
func (x ToText) MarshalJSONTo(enc *jsontext.Encoder) error { return nil }

// MarshalText writes the value.
func (x ToText) MarshalText() ([]byte, error) { return nil, nil }

// UnmarshalText reads the value.
func (x *ToText) UnmarshalText(b []byte) error { return nil }

// FromOnly has only UnmarshalJSONFrom.
type FromOnly int

// UnmarshalJSONFrom reads the value.
func (x *FromOnly) UnmarshalJSONFrom(dec *jsontext.Decoder) error { return nil }

// Money is a struct written through a value MarshalJSON.
type Money struct {
	Amount int64 `json:"amount"`
}

// MarshalJSON writes the money.
func (m Money) MarshalJSON() ([]byte, error) { return nil, nil }

// MoneyP is a struct written through a pointer MarshalJSON.
type MoneyP struct {
	Amount int64 `json:"amount"`
}

// MarshalJSON writes the money.
func (m *MoneyP) MarshalJSON() ([]byte, error) { return nil, nil }

// MoneyU is a struct read through its own UnmarshalJSON only.
type MoneyU struct {
	Amount int64 `json:"amount"`
}

// UnmarshalJSON reads the money.
func (m *MoneyU) UnmarshalJSON(b []byte) error { return nil }

// MA is an alias of Money: it keeps Money's methods.
type MA = Money

// MD is a defined type over Money: it drops Money's declared methods.
type MD Money

// MoneyOdd has a MarshalJSON with the wrong signature: not a Marshaler type.
type MoneyOdd struct {
	Amount int64 `json:"amount"`
}

// MarshalJSON returns a string, so encoding/json ignores it.
func (MoneyOdd) MarshalJSON() string { return "" }

// MoneyT is a struct written and read as text: a string both ways.
type MoneyT struct {
	Amount int64 `json:"amount"`
}

// MarshalText writes the money.
func (m MoneyT) MarshalText() ([]byte, error) { return []byte("1 USD"), nil }

// UnmarshalText reads the money.
func (m *MoneyT) UnmarshalText(b []byte) error { return nil }

// EmbP promotes LevelP's pointer text pair: a string both ways.
type EmbP struct {
	LevelP
	N int `json:"n"`
}

// Pair's embedded Level and Code cancel every method: an object both ways.
type Pair struct {
	Level
	Code
}

// Wrapper promotes StatusV's MarshalText only.
type Wrapper struct {
	StatusV
	Name string `json:"name"`
}

// Stamped promotes time.Time's four methods.
type Stamped struct {
	time.Time
	Note string `json:"note"`
}

// Twin's two MarshalText methods cancel; Level's UnmarshalText is promoted.
type Twin struct {
	Level
	StatusV
}

// Shadow's own MarshalJSON outranks the text pair promoted from Level.
type Shadow struct {
	Level
	Note string `json:"note"`
}

// MarshalJSON writes the shadow.
func (s Shadow) MarshalJSON() ([]byte, error) { return nil, nil }

// OnlyInner is reached only through MoneyWithInner.
type OnlyInner struct {
	In string `json:"in"`
}

// MoneyWithInner is written through its own MarshalJSON.
type MoneyWithInner struct {
	Inner OnlyInner `json:"inner"`
}

// MarshalJSON writes the money.
func (m MoneyWithInner) MarshalJSON() ([]byte, error) { return nil, nil }

// WrapLevel promotes Level's text pair: a string both ways.
type WrapLevel struct {
	Level
	Note string `json:"note"`
}

// PtrEmb promotes the text pair through an embedded pointer.
type PtrEmb struct {
	*Level
	Note string `json:"note"`
}

// TaggedEmb promotes the text pair through a json-tagged embed.
type TaggedEmb struct {
	Level `json:"lvl"`
	Note  string `json:"note"`
}

// Deep promotes the text pair two levels down.
type Deep struct {
	WrapLevel
	X int `json:"x"`
}

// WithID promotes uuid.UUID's text pair.
type WithID struct {
	uuid.UUID
	Note string `json:"note"`
}

// CustomBody is a request read through its own UnmarshalJSON; its id is
// still a path parameter.
type CustomBody struct {
	ID   string `param:"id"`
	Name string `json:"name"`
}

// UnmarshalJSON reads the body.
func (c *CustomBody) UnmarshalJSON(b []byte) error { return nil }

// Raw promotes json.RawMessage's JSON pair.
type Raw struct {
	json.RawMessage
	N int `json:"n"`
}

// Params types a Level query and header parameter by kind.
type Params struct {
	Level  Level `query:"level"`
	HLevel Level `header:"X-Level"`
}

// JoseReq is a JOSE request whose plaintext is read through its own
// UnmarshalJSON: the route stays application/jose, naming no component.
type JoseReq struct {
	_    struct{} `jose:"decrypt=k1,verify=k2"`
	Name string   `json:"name"`
}

// UnmarshalJSON reads the plaintext.
func (j *JoseReq) UnmarshalJSON(b []byte) error { return nil }

// JoseResp is a JOSE response whose plaintext is written through its own
// MarshalJSON.
type JoseResp struct {
	_ struct{} `jose:"sign=k1,encrypt=k2"`
	A int64    `json:"a"`
}

// MarshalJSON writes the plaintext.
func (j JoseResp) MarshalJSON() ([]byte, error) { return nil, nil }

// Filter is params-only: reached only as a parameter of CmdBody.
type Filter struct {
	A string `query:"a"`
}

// Empty has no serializable property: reached only as a parameter.
type Empty struct{}

// CmdBody is read through its own UnmarshalJSON; its struct-typed
// parameters keep their components.
type CmdBody struct {
	F    Filter `query:"f"`
	E    Empty  `query:"e"`
	Name string `json:"name"`
}

// UnmarshalJSON reads the body.
func (c *CmdBody) UnmarshalJSON(b []byte) error { return nil }

// Defaults promotes a pointer UnmarshalJSON.
type Defaults struct{}

// UnmarshalJSON reads the defaults.
func (d *Defaults) UnmarshalJSON(b []byte) error { return nil }

// ListReq is params-only: no body is documented, although it promotes
// Defaults' UnmarshalJSON.
type ListReq struct {
	Defaults
	Page int    `query:"page"`
	ID   string `param:"id"`
}
```

`module.go`:

```go
// Package marshalertypes pins Marshaler types (#111): string-both-ways types
// are strings, every other Marshaler type is {} with a warning, and no
// struct Marshaler type registers a component.
package marshalertypes

import (
	"github.com/example/marshaler_types/domain"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "marshalertypes" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// WithCode promotes domain.Tag's text pair across packages.
type WithCode struct {
	domain.Tag
	X int `json:"x"`
}

// Fields holds one field per row.
type Fields struct {
	Level     Level             `json:"level" validate:"required,min=1,max=3" example:"high"`
	LevelPtr  *Level            `json:"levelPtr"`
	LevelA    LA                `json:"levelA"`
	LevelP    LevelP            `json:"levelP"`
	LevelOne  Level             `json:"levelOne" validate:"oneof=1 2 3" example:"2"`
	Levels    []Level           `json:"levels" validate:"min=1,dive,max=3"`
	Grid      [][]Level         `json:"grid"`
	LevelMap  map[string]Level  `json:"levelMap"`
	LevelPMap map[string]LevelP `json:"levelPMap"`
	Flags     []FlagT2          `json:"flags"`
	Code      Code              `json:"code" validate:"min=2,max=5,email"`
	LD        LD                `json:"ld"`
	StatusV   StatusV           `json:"statusV"`
	LevelU    LevelU            `json:"levelU"`
	Both      Both              `json:"both"`
	TextJ     TextJ             `json:"textJ"`
	TextApp   TextApp           `json:"textApp"`
	AppOnly   AppOnly           `json:"appOnly"`
	ToText    ToText            `json:"toText"`
	FromOnly  FromOnly          `json:"fromOnly"`

	Money     Money                   `json:"money"`
	MoneyPtr  *Money                  `json:"moneyPtr"`
	Moneys    []Money                 `json:"moneys"`
	MoneyMap  map[string]Money        `json:"moneyMap"`
	MoneyP    MoneyP                  `json:"moneyP"`
	MoneyU    MoneyU                  `json:"moneyU"`
	MA        MA                      `json:"ma"`
	Price     domain.Price            `json:"price"`
	MoneyPPtr *MoneyP                 `json:"moneyPPtr"`
	MoneyPs   []MoneyP                `json:"moneyPs"`
	MoneyPMap map[string]MoneyP       `json:"moneyPMap"`
	MoneyUPtr *MoneyU                 `json:"moneyUPtr"`
	MoneyUs   []MoneyU                `json:"moneyUs"`
	MoneyUMap map[string]MoneyU       `json:"moneyUMap"`
	MAPtr     *MA                     `json:"maPtr"`
	MAs       []MA                    `json:"mas"`
	MAMap     map[string]MA           `json:"maMap"`
	PricePtr  *domain.Price           `json:"pricePtr"`
	Prices    []domain.Price          `json:"prices"`
	PriceMap  map[string]domain.Price `json:"priceMap"`
	MoneyT    MoneyT                  `json:"moneyT"`
	MoneyTPtr *MoneyT                 `json:"moneyTPtr"`
	MoneyTs   []MoneyT                `json:"moneyTs"`
	Amount    domain.Amount           `json:"amount"`
	EmbP      EmbP                    `json:"embP"`
	MoneyOdd  MoneyOdd                `json:"moneyOdd"`
	MD        MD                      `json:"md"`
	Pair      Pair                    `json:"pair"`
	Wrapper   Wrapper                 `json:"wrapper"`
	Stamped   Stamped                 `json:"stamped"`
	Twin      Twin                    `json:"twin"`
	Shadow    Shadow                  `json:"shadow"`
	WithInner MoneyWithInner          `json:"withInner"`
	WrapLevel WrapLevel               `json:"wrapLevel"`
	PtrEmb    PtrEmb                  `json:"ptrEmb"`
	TaggedEmb TaggedEmb               `json:"taggedEmb"`
	Deep      Deep                    `json:"deep"`
	WithID    WithID                  `json:"withID"`
	WithCode  WithCode                `json:"withCode"`
	Tag       domain.Tag              `json:"tag"`
	Raw       Raw                     `json:"raw"`
}

// RegisterRoutes registers one route per payload row.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/fields", m.getFields)
	server.POST(hr, r, "/fields", m.postFields)
	server.GET(hr, r, "/params", m.params)
	server.POST(hr, r, "/custom/:id", m.custom)
	server.GET(hr, r, "/money", m.money)
	server.GET(hr, r, "/money-ptr", m.moneyPtr)
	server.GET(hr, r, "/moneys", m.moneys)
	server.GET(hr, r, "/stamped", m.stamped)
	server.GET(hr, r, "/money-t", m.moneyT)
	server.GET(hr, r, "/wrap-level", m.wrapLevel)
	server.GET(hr, r, "/level", m.level)
	server.GET(hr, r, "/levels", m.levels)
	server.POST(hr, r, "/jose", m.jose)
	server.POST(hr, r, "/cmd", m.cmd)
	server.GET(hr, r, "/items/:id", m.items)
}

func (m *Module) getFields(ctx server.HandlerContext) (server.Result[Fields], server.IAPIError) {
	var v server.Result[Fields]
	return v, nil
}

func (m *Module) postFields(req Fields, ctx server.HandlerContext) (server.Result[Fields], server.IAPIError) {
	var v server.Result[Fields]
	return v, nil
}

func (m *Module) params(req Params, ctx server.HandlerContext) (server.Result[Level], server.IAPIError) {
	var v server.Result[Level]
	return v, nil
}

func (m *Module) custom(req CustomBody, ctx server.HandlerContext) (server.Result[Level], server.IAPIError) {
	var v server.Result[Level]
	return v, nil
}

func (m *Module) money(ctx server.HandlerContext) (server.Result[Money], server.IAPIError) {
	var v server.Result[Money]
	return v, nil
}

func (m *Module) moneyPtr(ctx server.HandlerContext) (server.Result[*Money], server.IAPIError) {
	var v server.Result[*Money]
	return v, nil
}

func (m *Module) moneys(ctx server.HandlerContext) (server.Result[[]Money], server.IAPIError) {
	var v server.Result[[]Money]
	return v, nil
}

func (m *Module) stamped(ctx server.HandlerContext) (server.Result[Stamped], server.IAPIError) {
	var v server.Result[Stamped]
	return v, nil
}

func (m *Module) moneyT(ctx server.HandlerContext) (server.Result[MoneyT], server.IAPIError) {
	var v server.Result[MoneyT]
	return v, nil
}

func (m *Module) wrapLevel(ctx server.HandlerContext) (server.Result[WrapLevel], server.IAPIError) {
	var v server.Result[WrapLevel]
	return v, nil
}

func (m *Module) level(ctx server.HandlerContext) (server.Result[Level], server.IAPIError) {
	var v server.Result[Level]
	return v, nil
}

func (m *Module) levels(ctx server.HandlerContext) (server.Result[[]Level], server.IAPIError) {
	var v server.Result[[]Level]
	return v, nil
}

func (m *Module) jose(req JoseReq, ctx server.HandlerContext) (server.Result[JoseResp], server.IAPIError) {
	var v server.Result[JoseResp]
	return v, nil
}

func (m *Module) cmd(req CmdBody, ctx server.HandlerContext) (server.Result[Level], server.IAPIError) {
	var v server.Result[Level]
	return v, nil
}

func (m *Module) items(req ListReq, ctx server.HandlerContext) (server.Result[Level], server.IAPIError) {
	var v server.Result[Level]
	return v, nil
}
```
