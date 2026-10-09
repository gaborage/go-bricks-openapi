# Named Type Resolution, PR 1 (#109 + #88 + #97) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every **local** named non-struct type gets a **Resolution**: in every field position (direct, `*T`, `[]T`, `[N]T`, `map[K]T`, nested to any depth) it emits exactly what its hand-expanded type emits. A **Marshaler type** in a body never resolves (`{}` plus a warning); every field that still falls back warns. One PR closes #109, #88 and #97, and, through its two context fixes, #123 and #101. It fixes #100's map-value/nested half and #120's field rows without closing either.

**Architecture:** A new analyzer resolver (`internal/analyzer/resolution.go`, guard in `marshaler.go`) decodes each local `TypeSpec.Type` with `typeShape` and substitutes it at every named leaf, writing the result into a new `FieldInfo.Resolution *models.TypeShape`. The tree reuses the Shape vocabulary plus four Resolution-only leaf kinds (`ShapeRef`, `ShapeKindOnly`, `ShapeMarshaler`, `ShapeRecursive`). Every field consumer reads `FieldInfo.ResolvedShape()` (the Resolution, else the syntactic Shape). The generator's one recursive typing (`setTypeAndFormat`) learns the four leaf kinds, which retires the one-level named-scalar branch, `namedScalarItems`, `setNamedScalarType` and `refProperty`'s array branch. `RefName`, `MapValueRefName`, `UnderlyingKind` and `UnderlyingBuiltin` are deleted from `models.FieldInfo` (open decision 1). Two context fixes make "resolved in the declaring file" true: a struct's fields are extracted in the file that **declares** the struct (`structDeclSite`, not the referencing file), and every `ShapeRef` leaf is registered in the file it was **resolved** in (a per-field site record), so a qualified leaf (`domain.User`) never re-resolves its alias against another file's imports.

**Tech Stack:** Go 1.25 language floor; `go/ast` static analysis only; goldens in `internal/spectest`.

**Spec (authoritative, in this order):** #109's triage brief as amended 2026-10-03 (seven methods, not four) and 2026-10-07 (#125/#124 already landed; flip ADR 0002 to accepted). #88's and #97's own briefs are **superseded** where they disagree; specifically the README line "documented from their underlying type" is **not** added. Glossary: `CONTEXT.md` (**Shape**, **Resolution**, **Marshaler type**). Design: `docs/adr/0002-named-type-resolution.md`. #98 (closed via #126) and #89 (closed via #125) are done, so the "Blocked by" lines are stale.

**Overlapping open issues this PR also settles (name them; never leave the overlap implicit):**
- **#123** (sibling-file qualified field types): Task 3 Step 0 (`structDeclSite` at the three call sites #123's Direction names) plus rule 9 (per-field ref sites, #123's promoted-field seam) are exactly its two seams. **Closed.** Its residuals stay out: the `schemaKey` package-name shadow (two packages both named `domain` share one component; unfiled) and whether a dropped embed warns (#116).
- **#101** (promoted cross-package struct refs resolve in the embedding package): rule 9 plus Task 6's deletion of the old `shapeBaseName` stamping. **Closed**, pinned by `TestPromotedFieldIgnoresEmbeddingPackageNames` (Task 6).
- **#120** (double pointers): `stripPointers` (open decision 5) fixes its **field rows**, params included, pinned by `TestMultiPointerMatchesSinglePointer` (Task 4). **Not closed:** its `Result[**T]` / `PayloadBaseShape` payload half stays open.

**Baseline for line anchors:** `origin/main` at `514e417` (after #127). Re-check every anchor with `grep -n` before editing.

---

## Global Constraints

- **Branch:** `fix/named-type-resolution`, cut from `origin/main` in a fresh worktree: `git fetch -q origin && git worktree add ../go-bricks-openapi-ntr -b fix/named-type-resolution origin/main`.
- **Goldens:** only `go test ./internal/spectest -update` regenerates them, and never `make update`. **No existing `expected.yaml` may change.** After `-update`, `git status --porcelain internal/spectest/testdata` may list only the two new fixture directories plus the comment-only edit to `named_scalar_builtins/handler.go`. Any modified existing `expected.yaml` is a bug: fix the code, don't accept the diff. The golden test is `TestGoldenFixtures`, and a `-run 'TestFixtures/…'` pattern matches nothing.
- **Cognitive complexity ≤15 per function** (SonarCloud `go:S3776`, server-side only; `make lint` checks only cyclomatic). Check with `go run github.com/uudashr/gocognit/cmd/gocognit@latest -over 15 <files>`. The budget table below is binding.
- **Coverage ≥80% on new code** in `cmd/` and `internal/`. The profile has no `-coverpkg`, so a package's coverage comes **only from its own tests**. Spectest and commands tests do not cover analyzer or generator lines, so every new analyzer or generator branch needs an in-package test. `internal/models` is filtered out of `TEST_PACKAGES` and stays test-free: never add a `_test.go` there.
- **`goconst` counts `_test.go` literals** (min-len 4, min-occurrences 3) even though findings in tests are suppressed. New production string literals repeated 3+ times across the package, test files included, must be constants. New code goes in **new files** (`resolution.go`, `marshaler.go`), not the 4.4k-line `analyzer.go`.
- **`exhaustive` lint** (`default-signifies-exhaustive: true`): adding four `ShapeKind` constants means **every new `switch` on a `ShapeKind` needs a `default:` arm**. The three existing switches (`analyzer.go` 2453, 3837; `commands/generate.go` 359, a yaml switch) already have defaults.
- **No `t.Parallel()`**. The Go 1.25 floor applies, so use only `slices`, `maps` and `strings` APIs available in 1.25.
- **Settled invariants (CLAUDE.md), untouched:**
  - `lookupStructTag` and `unquoteLiteral` stay the only tag and literal readers.
  - The Constraint set lives only in `internal/generator/constraints.go`. `applyValidationConstraints` stays the single entry point.
  - Example coercion happens only in the `fieldInfoToProperty` wrapper. `buildFieldProperty` must not stamp examples, and this PR deletes the one place it did: `refProperty`'s array branch called `applyExample`.
  - `referencedSchemaNames` must not scan non-JOSE request types. Only `addFieldSchemaRefs` changes.
  - The `schema == nil` check in `generateSchemasFromTypes` is untouched.
  - The uint `minimum: 0` pre-stamp in `setBasicTypeAndFormat` stays frozen (#54). `applyTo` still writes only set keywords.
  - `NOSONAR` and `//nolint` are not interchangeable.
- **Out of scope (do not implement):**
  - Other-package named types (`b.Cents`): #100. Here they warn as unresolvable.
  - Payload resolution: #110. `TypeInfo.Resolution` is declared but left nil, and `TestNamedSliceWarnsAndClears` and `TestNamedScalarSliceElementClearsNameKeepsShape` stay unchanged.
  - Request types: #102.
  - The text-marshaler string rule, struct Marshaler types and embedded method promotion: #111.
  - Overrides: #117.
  - Generic instantiations: #114.
  - Anonymous struct fields: #118. They stay `ShapeUnknown` and never warn.
  - `json:",string"`: #115.
  - More stdlib types: #113.
  - Dropped embeds: #116 (including #123's open point on whether a dropped embed warns).
  - #120's payload half (`Result[**T]`, `PayloadBaseShape` shedding one pointer level): stays with #120/#110.
  - #123's `schemaKey` residual: components are keyed by package **name**, so two packages both named `domain` that each register `Price` share one component. Separate, unfiled bug (open decision 10).
  - Unbindable-param warnings: #121.
  - `registerTypeAt` and `resolveTypeSpecChain` do **not** learn composites. (They do change which file they extract a struct in: Task 3 Step 0.)
- **Gates before the PR:**
  - `make check`, with the pinned golangci-lint from `make dev-deps`; `lint` refuses a mismatched binary.
  - `go test ./internal/spectest`.
  - gocognit `-over 15` on every touched file.
  - The coverage spot-check (Task 9).
  - The four mutation checks (Task 9).
- **Squash commit = the one commit.** Its body stands alone (template at the end). Personal repo: no `Refs:` line.

---

## Behavior rows: expected emitted YAML

Flow style is used for readability. Goldens are block style, and key order follows `OpenAPIProperty` field order (`type, allOf, properties, additionalProperties, format, description, example, $ref, items, minLength, maxLength, minItems, maxItems, minProperties, maxProperties, minimum, …, nullable`).

**Desired = hand-expanded.** These cells were produced by running the current `origin/main` binary over the hand-expanded types in `scratchpad/pr1/expanded`, then checked against the brief. Other positions emit what the hand-expanded type emits there. A pointer at the top of a slice or map adds no `nullable`; a pointer behind a container never does.

| # | Case (field type) | Emitted (direct field) | Warns? |
|---|---|---|---|
| R1 | `type Tags []string`, `TagsA = []string`, `type Chain Tags`, `*Tags` | `{type: array, items: {type: string}}` (`*Tags` identical, no `nullable`) | no |
| R1 | `type Attrs map[string]string` | `{type: object, additionalProperties: {type: string}}` | no |
| R1 | `type Stamps []time.Time` | `{type: array, items: {type: string, format: date-time}}` | no |
| R1 | `type Ids []Cents` | `{type: array, items: {type: integer, format: int64}}` | no |
| R2 | `Tags validate:"min=1,dive,max=5"` | `{type: array, items: {type: string, maxLength: 5}, minItems: 1}` | no |
| R2 | `Attrs validate:"min=1"` | `{type: object, additionalProperties: {type: string}, minProperties: 1}` | no |
| R3 | `type UserList []User` | `{type: array, items: {$ref: '#/components/schemas/User'}}`; `User` component emitted | no |
| R3 | `type PAddr *Address` | `{type: object, allOf: [{$ref: '#/components/schemas/Address'}], nullable: true}`; `Address` emitted | no |
| R4 | `type PC *int64` | `{type: integer, format: int64, nullable: true}`; `[]PC` → items `{type: integer, format: int64}` | no |
| R4 | `type FlagB bool` | `{type: boolean}` | no |
| R4 | `type AnyD any`, `type Shaper interface{ Area() float64 }` | `{}` | no |
| R5 | `type Pair [2]int` | `{type: array, items: {type: integer, format: int64}}` | no |
| R5 | `type Key [4]byte`, `type Arr [3]byte`, `[3]Flag`, `[]*Flag` | `{type: array, items: {type: integer, format: int32, minimum: 0}}` (never base64) | no |
| R6 | `[][]Cents` | `{type: array, items: {type: array, items: {type: integer, format: int64}}}` | no |
| R6 | `map[string]Cents` | `{type: object, additionalProperties: {type: integer, format: int64}}` | no |
| R6 | `[]map[string]Cents` | `{type: array, items: {type: object, additionalProperties: {type: integer, format: int64}}}` | no |
| R7 | plain `[][]User`, `*[][]User` | `{type: array, items: {type: array, items: {$ref: …/User}}}` (today: one level lost) | no |
| R7 | plain `[]map[string]User` | `{type: array, items: {type: object, additionalProperties: {$ref: …/User}}}`; `User` emitted (today: `object` leaf, `User` unemitted) | no |
| R8 | `type Blob []byte`, `Blob = []byte`, `type B2 Blob`, `type Flags []Flag`, `type Raw json.RawMessage`, `type RawOfRaw Raw`, `[]Flag` (`type Flag byte`), `[]U8` (`U8 = uint8`), `[]FlagU` | `{type: string, format: byte}`; `*Blob`/`*[]Flag` identical (no `nullable`) | no |
| R8 | `[][]Flag` | `{type: array, items: {type: string, format: byte}}` | no |
| R8 | `map[string]Flag` | `{type: object, additionalProperties: {type: integer, format: int32, minimum: 0}}` | no |
| R9 | `[]Flag validate:"min=1,dive,max=3" example:"AQI="` | `{type: string, format: byte, example: AQI=}`, no length or items keywords | no |
| R10 | `Stamp = time.Time` | `{type: string, format: date-time}` | no |
| R10 | `IDA = uuid.UUID` | `{type: string, format: uuid}` | no |
| R10 | `RawA = json.RawMessage` | `{}` | no |
| R10 | `type Amount json.Number` | `{type: string}` | no |
| R11 | Marshaler: `type Status int` + `MarshalText`, `StatusA = Status`, `StatusP` (`func (*StatusP) UnmarshalJSON([]byte) error`), `type TagsM []string` + `MarshalJSON`, `FlagV` (`func (*FlagV) MarshalJSON`) | `{}` | **yes** |
| R11 | `*Status` | `{}` (no type, so no `nullable`) | **yes** |
| R11 | `[]FlagV` | `{type: array, items: {}}` (not base64) | **yes** |
| R11 | `type StatusD Status`, `Odd` (`func (Odd) MarshalText() string`) | `{type: integer, format: int64}` | no |
| R12 | `Status validate:"min=1,max=3"` / `validate:"oneof=1 2"` | `{}` | **yes** |
| R12 | `Status example:"active"` | `{example: active}` | **yes** |
| R12 | `[]Status validate:"min=1,dive,oneof=1 2"` | `{type: array, items: {}, minItems: 1}` | **yes** |
| R13 | `type Tree map[string]Tree` | `{type: object, additionalProperties: {}}` | **yes** |
| R13 | `type List []List` | `{type: array, items: {}}` | **yes** |
| R13 | mutual `type Left []Right; type Right map[string]Left` | `{type: array, items: {type: object, additionalProperties: {}}}` | **yes** |
| R14 | `decimal.Decimal`, `t.Time` (`import t "time"`), `j.RawMessage` (`import j "encoding/json"`), `b.Cents` (other package), `type StampD time.Time`, `type IDD uuid.UUID` | `{type: object}` | **yes** |
| R14 | builtins `error`, `complex64`, `complex128`, and `type C complex128` | `{type: object}` (`setBasicTypeAndFormat`'s `default:`) | **yes** (untyped builtin) |
| R14 | `type Addr uintptr` | `{type: object}` | **uintptr warning only** |
| R14 | `map[string]uintptr` | `{type: object, additionalProperties: {type: object}}` | **uintptr warning only** |
| R15 | query `Tags` / `*Tags` | `{type: array, items: {type: string}}` | no |
| R15 | query `Ids` | `{type: array, items: {type: integer, format: int64}}` | no |
| R15 | query `FlagB` | `{type: boolean}` | no |
| R15 | query `Amount` | `{type: string}` | no |
| R15 | query `TagsM` (Marshaler, exempt) | `{type: array, items: {type: string}}` | no |
| R15 | query `Status` (Marshaler, exempt) | `{type: integer, format: int64}` | no |
| R16 | kind-only (#92) at depth: `map[string]Word`, `[][]Word` with `Word` int64/int32 across build tags | `additionalProperties: {type: integer}`; `items: {type: array, items: {type: integer}}` | no |
| R17 | plain `map[string]User validate:"min=1"` | `{type: object, additionalProperties: {$ref: …/User}, minProperties: 1}` (today drops `minProperties`; open decision 2) | no |
| R18 | sibling-file layout: `types.go` imports `…/domain` and declares `type Members []domain.Member`, `type PMember *domain.Member`, `type MemberMap map[string]domain.Member` and `struct Holder { Item domain.Item; Items []domain.Item }`; the field struct (`api.go`) and the handler (`module.go`) do **not** import `domain` | `members` → `{type: array, items: {$ref: …/Member}}`; `pMember` → `{type: object, allOf: [{$ref: …/Member}], nullable: true}`; `memberMap` → `{type: object, additionalProperties: {$ref: …/Member}}`; `Holder.item` → `{$ref: …/Item}`; `Holder.items` → `{type: array, items: {$ref: …/Item}}`; `Member`, `Item` emitted (today: `object`, silently, no component) | no |
| R18 | promoted cross-package embed: `Receipt { other.Base }` where `other.Base { In Inner }` and `Inner` is a struct local to `other` | `in` → `{$ref: …/Inner}`; `Inner` emitted (today: `object`, silently) | no |

`json:"-"` fields are never resolved and never warn. `ShapeUnknown` leaves (generics, anonymous structs, `chan`, `func`) are unchanged and never warn.

---

## Data model (`internal/models/models.go`)

Current anchors: `TypeInfo` 89–127, `ShapeKind` consts 160–174, `TypeShape` 176–191, `FieldInfo` 193–238.

```go
const (
	ShapePointer ShapeKind = "pointer"
	// … existing kinds unchanged …
	ShapeUnknown   ShapeKind = "unknown"

	// Resolution-only kinds. The analyzer's resolver writes them into a
	// Resolution; the Shape decoder never produces them, so a Shape stays
	// purely syntactic.

	// ShapeRef is a named struct that registers as a component. Name is its
	// component name: the declared name until registerFieldRefAt stamps the
	// final, collision-qualified one.
	ShapeRef ShapeKind = "ref"
	// ShapeKindOnly is a named scalar whose build-tagged declarations disagree
	// on width (#92). Name is the OpenAPI kind: integer, number, string or boolean.
	ShapeKindOnly ShapeKind = "kind-only"
	// ShapeMarshaler is a Marshaler type (CONTEXT.md) in a body position. Name
	// is the type that declares the method. Elem is its underlying resolution,
	// kept for #111. Nothing emits or registers what is under Elem.
	ShapeMarshaler ShapeKind = "marshaler"
	// ShapeRecursive is a named type met again inside its own resolution, or
	// past maxNamedResolutionDepth. Name is that type. The cut point emits {}.
	ShapeRecursive ShapeKind = "recursive"
)
```

- Update the `TypeShape` doc (176–180): drop "(that is Resolution: RefName/MapValueRefName/UnderlyingBuiltin/UnderlyingKind)" and say the registry outcome lives in `FieldInfo.Resolution` / `TypeInfo.Resolution`, which reuse this vocabulary plus the four Resolution-only kinds.
- `FieldInfo`: **delete** `RefName`, `MapValueRefName`, `UnderlyingKind` and `UnderlyingBuiltin` (lines 211–237). Add:

```go
	// Resolution is the field's type with every local named non-struct type
	// substituted by what it stands for, at every depth, and every struct leaf
	// marked ShapeRef (CONTEXT.md, "Resolution"). Stamped by the analyzer for
	// every field that reaches the document (never for a json:"-" body field).
	// Read it only through ResolvedShape.
	Resolution *TypeShape
}

// ResolvedShape returns the field's Resolution, or its syntactic Shape when
// the field has none (a hand-built FieldInfo, or a json:"-" field).
func (f *FieldInfo) ResolvedShape() TypeShape {
	if f.Resolution != nil {
		return *f.Resolution
	}
	return f.Shape
}
```

- `TypeInfo`: add `Resolution *TypeShape` (doc: "the payload's Resolution; filled by #110, nil until then") and `func (t *TypeInfo) ResolvedShape() *TypeShape` (returns `Resolution` if non-nil, else `Shape`). Nothing reads it in this PR.
- Add the single source for the untyped-builtin warning (R14), beside `WellKnownTypeNames`:

```go
// UntypedBuiltinNames are the Go builtins with no JSON Schema form: the
// generator types them as a bare object (setBasicTypeAndFormat's default),
// and the analyzer warns on any field leaf that is one. uintptr is not here:
// it has its own warning.
var UntypedBuiltinNames = map[string]bool{"error": true, "complex64": true, "complex128": true}
```

  It is a map literal with no branches, so `models` stays test-free. The analyzer reads it; the generator's parity test (Task 4 Step 2) pins that each name, and `uintptr`, falls to `object` and that every other builtin does not.
- Fix the misplaced comment at 158 ("FieldInfo represents a struct field…" sits above `ShapeKind`). Move it onto `type FieldInfo`, since revive will see the doc on the exported type.

**Rejected alternative (record in the commit body):** a distinct `Resolution` struct type. It is cleaner, but the brief's "one accessor that returns it or else Shape" forces Shape's type, and a second type would duplicate the generator's container recursion.

**Coverage note:** `models` is excluded from `TEST_PACKAGES`, so its two accessors (~8 lines) never appear in `coverage.out`. That is accepted, and is small against the PR's new-line count.

---

## Resolver semantics (analyzer), with every helper's degenerate input checked

### Helpers reused as is (behavior verified on `origin/main`)

| Helper (anchor) | Degenerate behavior the resolver relies on |
|---|---|
| `typeShape` (4247) | `InterfaceType`, even with methods, gives `ShapePrimitive "interface{}"`. `StructType`, `FuncType`, `ChanType`, `IndexExpr` (generics) and `ParenExpr` give `ShapeUnknown`. A selector with a non-ident base gives `ShapeUnknown`. `[N]T` gives `ShapeArray`. |
| `resolveTypeSpecChain` (3285) | Base case is `findStructDefinition`. It follows `type X = Y` / `type X Y` idents and `type X = q.T` selectors. It returns `ok=false` at `depth > 8`, for any non-ident/selector RHS, and when the name is undeclared. It never errors. |
| `findStructDefinition` (3917) | Searches the current file, then every same-package file (unsorted map, but it only answers "is there a struct"). An unreadable dir gives an error, which reads as "not a struct". **It drops the declaring file**, so three of its callers move to the new `structDeclSite` (Task 3 Step 0): `registerTypeAt` (3247), `resolveTypeSpecChain`'s base case (3290) and `embeddedFields`' local branch (4125). `fieldTypeExprOf` (1621) only returns `field.Type` and keeps it. |
| `registerTypeAt` (3231) | A dotted name goes to `registerQualifiedTypeAt(name, astFile, depth)`, which resolves the alias through `fileImports(astFile)`: **the file passed in must be the file the name was written in**. It keeps the depth cap (`depth > maxTypeRegistrationDepth` → `warnDepthExceeded`, nil), so every registration in this PR goes through it, never straight to `registerStructAt`. |
| `resolveQualifiedStruct` (3389) | `ok=false` for no dot, an unknown alias (dot/blank imports), stdlib/third-party (`inModuleDir` false), an unparsable dir, or a non-struct target. |
| `parsePackageDir` (3540) | Caches per dir. It skips `_test.go`, unparsable and out-of-root files, and returns an error only when the dir is unreadable. The in-memory files of unit tests use nonexistent paths, so callers must still scan the current file themselves. |
| `fileImports` (3498) | Maps local name → path. It skips `_` and `.`; an unaliased import maps by package clause (in-module) or `filepath.Base` (`"encoding/json/jsontext"` → `jsontext`). |
| `isMethodOnStruct` (621) | Despite the name, it matches any ident receiver: `T` or `*T`. A nil or empty `recv` gives false. Generic (`T[K]`) and parenthesized receivers are not matched (accepted residual). |
| `isJSONExcluded` (3633) | `JSONName == "-" && ParamType == ""`. |
| `sameBuiltin` (3717) | Canonicalizes `byte`↔`uint8` and `rune`↔`int32`. **Kept**, and used by the declaration merge. |
| `primitiveKind` (3748) | Today `bool` → `""`. **Changed:** `bool` → `"boolean"` (new const `kindBoolean` in `builtins.go`), as the brief asks for (`TestPrimitiveKind`'s bool row). |
| `knownUnderlyingBuiltins` (3648) | Duration → int64, Month/Weekday → int; `json.Number` deliberately absent. **Unchanged**. |

### Resolution rules

The resolution of field `f` happens at extraction, in the file that declares `f`'s struct. On `origin/main` that is **not** true for a struct declared in a sibling file of the referencing file: `registerTypeAt` and `embeddedFields` extract it with the referencing file, so `Items []m.Item` in `types.go` behind a handler in `module.go` (which does not import `m`) emits `object` silently today, and under this plan's rule 2.4 would warn as unresolvable on valid code. Task 3 Step 0 fixes this first (`structDeclSite`). Promoted cross-package fields already resolve in their own package (`embeddedFields`' qualified branch).

1. **Containers.** For pointer, array and map, copy the node and resolve `Elem`; a map's `Key` is kept as is and never resolved or warned about. For a slice, the element goes through the byte-element rule (rule 6). A nil `Elem` stays nil.
2. **Qualified name** (`q.T`), in this order, which mirrors `registerFieldRefAt`, where a registered struct won:
   1. `resolveQualifiedStruct` ok gives `ShapeRef{q.T}`. This keeps a project `uuid.UUID` struct a `$ref`, as today.
   2. `knownUnderlyingBuiltins` gives `ShapePrimitive{builtin}`. A direct `time.Duration` emits `{integer, int64}` exactly as `wellKnownFormats` did, and bounds still apply.
   3. `WellKnownTypeNames` keeps the `ShapeNamed` leaf (`time.Time`, `uuid.UUID`, `json.RawMessage`, `json.Number`).
   4. Otherwise the type is **unresolvable**: keep the leaf (it emits `object`) and note it.
3. **Local name `N`** (`resolveLocal`, mode = identity or underlying):
   1. If `resolveTypeSpecChain(file, path, N, 0)` is ok, return `ShapeRef{N}`. Checking struct first keeps the "a struct variant is a $ref" build-tag case.
   2. If `N` is already on the stack, or the stack holds `maxNamedResolutionDepth` (8, parity with `resolveTypeSpecChain`) names, return `ShapeRecursive{N}` and note it.
   3. Collect the declarations: the current file first, then same-package siblings in sorted path order, skipping the current path and files of another package. If there are none, `N` is **unresolvable**: keep the leaf and note it.
   4. In **identity** mode, and only when the field is not a param: if `marshalerSet(N)` finds any of the seven methods, note it and return `ShapeMarshaler{N, Elem: underlying}`. The underlying is resolved in underlying mode on a **quiet** copy of the state, which notes nothing.
   5. Otherwise push `N`, resolve each declaration, pop, and merge (rule 5).
4. **One declaration** `type N <rhs>`:
   - In identity mode an **alias** (`spec.Assign.IsValid()`) resolves `typeShape(rhs)` normally: an alias is its target, methods included. `StatusA = Status` therefore reaches `Status`'s guard.
   - A **defined type**, or any declaration in underlying mode, resolves the **underlying** of `rhs`:
     - A builtin or composite resolves normally, **through `resolveShape`**, so rule 8's untyped-builtin note fires for `type C complex128` exactly as for a direct `complex128`. Nested leaves keep identity mode, so `[]Status` elements stay Marshaler.
     - A local ident `M` resolves through `resolveLocal(M, underlying mode)`: the defined type drops `M`'s methods, so `type StatusD Status` gives `int`.
     - A qualified name goes through `kindBackedUnderlying`: `knownUnderlyingBuiltins`, plus `json.RawMessage` → `[]byte` (`ShapeSlice{ShapePrimitive byte}`) and `json.Number` → `string`. **These two apply only at the end of a defined chain**, never in rule 2. On a miss (`time.Time`, `uuid.UUID`, `decimal.Decimal`, `b.Cents`), note `N` as unresolvable and return `ShapeNamed{N}`. Do **not** return the qualified leaf: `type StampD time.Time` would then be typed as a date-time.
5. **Merge** (build-tagged variants; keeps README's precedence):
   - Fold the declarations pairwise with `mergeShapes`. Containers must match in kind and merge their `Elem`s. Leaves merge when they are the same leaf (`sameBuiltin` for primitives, `Kind`+`Name` otherwise). Primitive and kind-only leaves of the same `leafKind` merge to `ShapeKindOnly{kind}`, at any depth.
   - On failure, use the first declaration whose resolution is a scalar leaf (`leafKind != ""`): `ShapeKindOnly{thatKind}`.
   - Otherwise `N` is unresolvable: note it and return `ShapeNamed{N}`.
6. **Byte-slice element** (#97, the brief's "base64 rule checks only Marshal\*"). For a slice whose `Elem` is a local `ShapeNamed`:
   - Snapshot `st.first`, then resolve the element in identity mode.
   - If the result is `ShapeMarshaler` whose `Elem` is `ShapePrimitive byte|uint8`, and `marshalerSet(result.Name).encode` is false (only `UnmarshalJSON`, `UnmarshalText` or `UnmarshalJSONFrom`), restore `st.first` and use `*result.Elem`. The slice is then a base64 `[]byte`, and nothing warns.
   - Otherwise keep the result: `[]FlagV` is an array of `{}`.
   - The rule applies to slices only, never arrays (`[3]FlagU` items are `{}` and warn) or pointer elements (`[]*FlagU` items are `{}` and warn).
7. **Params** (`ParamType != ""`) skip rule 3.4, so a Marshaler param resolves to its underlying type and is unwarned. All other rules, unresolvable warnings included, apply to params (open decision 3).
8. **Untyped builtin leaf** (R14, the brief's "any leaf that would reach `setBasicTypeAndFormat`'s unrecognised-name `object`"). In `resolveShape`'s `ShapePrimitive` arm, a name in `models.UntypedBuiltinNames` notes `fallbackUntypedBuiltin{typeName: outermost named type on the stack, else the builtin; detail: the builtin}`. `uintptr` is not in the set and keeps its own warning. Quiet runs (a Marshaler's `Elem`) note nothing, as for every note.
9. **Ref sites.** Every time the resolver produces a `ShapeRef` leaf (rule 2.1 qualified, rule 3.1 local), it records `c.st.refSites[leaf.Name] = pkgFile{c.file, c.path}` (first wins): the file in whose scope that name was written. Quiet runs record into their own discarded map, which is correct because a Marshaler's `Elem` is never registered. `resolveField` stores `fieldSite{loc, refSites}` in `a.fieldSites[f.Resolution]`. `registerFieldRefAt` registers each leaf with that site's file, path and package (`site.file.Name.Name`), never the struct's own file. This covers:
   - a qualified leaf from a sibling-file declaration (`type Users []domain.User` in `types.go`, field in `api.go`);
   - a local leaf of a promoted cross-package embed (`Inner` local to `other`, field promoted into `shop.Receipt`), which was open decision 7.
   - **Why a per-field record keyed by the `Resolution` root and not a per-leaf pointer:** `resolveShape` returns values, so a leaf has no stable address until its parent allocates it, while `f.Resolution` is allocated once and survives every `FieldInfo` copy. **Why not a new `TypeShape` field:** it would put analyzer context into `models`.
   - **Accepted residual:** two leaves in one field with the same written name resolved in two files where it means two different types (`m.User` with `m` bound to two different packages in two files that one field's chain spans). First wins, and the second registers in the first's context. Recorded in the commit body.

### Marshaler guard (`marshaler.go`)

`marshalerSet(name, c)` scans `samePackageFiles` for every `*ast.FuncDecl` whose receiver satisfies `isMethodOnStruct(recv, name)` and whose name and signature match one of these exactly. Results may be named. `fieldTypes` flattens a `*ast.FieldList` into one type per name, or one type for an unnamed field, and a nil list into an empty slice.

| Method | Params | Results | Side |
|---|---|---|---|
| `MarshalJSON` | none | `[]byte`, `error` | encode |
| `MarshalText` | none | `[]byte`, `error` | encode |
| `MarshalJSONTo` | `*jsontext.Encoder` | `error` | encode |
| `AppendText` | `[]byte` | `[]byte`, `error` | encode |
| `UnmarshalJSON` | `[]byte` | `error` | decode |
| `UnmarshalText` | `[]byte` | `error` | decode |
| `UnmarshalJSONFrom` | `*jsontext.Decoder` | `error` | decode |

- `[]byte` is `*ast.ArrayType{Len: nil, Elt: Ident byte|uint8}`. `error` is `Ident "error"`.
- `*jsontext.Encoder`/`Decoder` is a `StarExpr` over a `SelectorExpr` whose `X` ident maps through `fileImports(file)` (**the method's own file**) to exactly `encoding/json/jsontext`. An aliased import (`jt "encoding/json/jsontext"`) counts.
- Anything else does not count: `func (Odd) MarshalText() string`, `MarshalJSON() []byte`, `UnmarshalJSON(string) error`, `MarshalJSONTo(*json.Encoder) error`.
- Returns `marshalerMethods{encode, decode bool; first string}`, where `first` is the first matching method name in file order, used in the warning.
- `structDeclaresMethod` (605) is **not** reused (it matches names only).
- Residual: a method declared on an alias receiver name (`func (FlagUA) MarshalJSON`) counts only where that alias name is used.

### Warnings (one per field declaration, deduped by source position like `warnUintptrField`)

- **uintptr wins.** If any leaf of the resolution is `ShapePrimitive uintptr`, outside Marshaler and recursive leaves, emit only the existing uintptr warning. That now covers `type Addr uintptr` and `map[string]uintptr`. Change its text from "is a uintptr" to "holds a uintptr" (accurate for a map value or wrapper). **Two tests read that text:** `TestUintptrFieldWarns` asserts only "uintptr", the field name and "no meaningful API contract", and stays unchanged; `TestRunGenerateFixedArrayFields` (`internal/commands/fixed_array_test.go:155`) asserts `"is a uintptr"` and is updated to `"holds a uintptr"` in Task 3 Step 5, recorded in the commit body.
- **Otherwise** the first noted fallback, in depth-first order, picks the message. Each is a package `const`:
  - `marshalerFieldWarning`: `"field %s at %s has type %s, whose own %s method encoding/json uses instead of its underlying type — emitting an untyped schema ({}); parameters are typed by kind and unaffected"`
  - `recursiveFieldWarning`: `"field %s at %s has type %s, which contains itself (or nests named types too deeply) — the recursion is cut to an untyped schema ({})"`
  - `unresolvableFieldWarning`: `"field %s at %s has type %s, which resolves to no schema (a third-party type, a well-known type under an aliased import, a defined type over a well-known struct, or a named type from another package) — emitting an untyped object"`
  - `untypedBuiltinFieldWarning`: `"field %s at %s has type %s, which holds the builtin %s, a Go type with no JSON schema — emitting an untyped object"` (rule 8; `fieldFallback.detail` carries the builtin).
  - `unregisteredRefFieldWarning`: `"field %s at %s has type %s, whose struct could not be registered as a component — emitting an untyped object"`. Raised by `registerRefLeaf` (Task 3 Step 4) when it demotes a `ShapeRef` leaf to `ShapeNamed` for any reason **other than the depth cap** (which already raised `warnDepthExceeded`). It uses the `loc` stored in `a.fieldSites` and the same `fieldWarned` dedupe, so a field that already warned at extraction does not warn twice. No demotion is ever silent.
- New dedupe map `fieldWarned map[string]struct{}` and site record `fieldSites map[*models.TypeShape]fieldSite` on `ProjectAnalyzer` (beside `uintptrWarned`, line 130). **All three maps are allocated eagerly:** add `uintptrWarned`, `fieldWarned` and `fieldSites` (each `make(...)`) to the `New()` literal (line 142), and in `AnalyzeProject` (line 246) change `a.uintptrWarned = nil` to `make(...)` and reset the two new maps with `make(...)` too. `warnUintptrField`'s own lazy block (4176–4177) goes. Extract `func (a *ProjectAnalyzer) fieldLoc(field *ast.Field) string` and `func markOnce(seen map[string]struct{}, loc string) bool` (a plain map, never `*map`), so the uintptr and fallback paths share them and `dupl` stays quiet.
  - **Why eager, not lazy:** a lazy `markOnce` needs `*map[string]struct{}`, which gocritic's `ptrToRefParam` rejects (`.golangci.yml` enables all five tags and does not disable it; reproduced with the pinned v2.12.2 on a scratch module: `ptrToRefParam: consider 'seen' to be of non-pointer type`). Eager is safe: `New()` is the package's only constructor, and no test builds `&ProjectAnalyzer{}`, `new(ProjectAnalyzer)` or a zero value (grepped on `514e417`), so an analyzer that never runs `AnalyzeProject` still has the maps.
  - **Leave `tagWarned` alone:** it stays `nil` in `AnalyzeProject` with its own lazy block in `tagcheck.go:160`. Do not "harmonize" it into `markOnce` here.
  - `resolveState.refSites` is made by `newResolveCtx` and again by `quiet()`. A missing `fieldSites` entry or `refSites` key just misses.
- Every warning feeds `--strict` and `doctor` ("Ready with caveats") through the existing `Warnings()` path; no command code changes.

---

## Cognitive-complexity budget (measured on `origin/main` with gocognit; nothing in `internal/analyzer` or `internal/generator` is over 15 today)

| Function (file:line) | Now | After (cap) | Note |
|---|---|---|---|
| `buildFieldProperty` (openapi.go:1522) | 12 | ≤7 | two branches retire |
| `refProperty` (openapi.go:1603) | 2 | ≤2 | array branch deleted |
| `setTypeAndFormat` (openapi.go:1731) | 7 | ≤10 | +3 leaf cases in a `switch` with `default` |
| `isPointerField` (openapi.go:1639) | 1 | 1 | |
| `addFieldSchemaRefs` (openapi.go:1393) | 11 | ≤6 | extract `addRefNames` (≤6) |
| `extractParameters` (openapi.go:2042) | 11 | 11 | untouched; it reads through `fieldInfoToProperty` |
| `applyValidationConstraints` (constraints.go:245) | 7 | ≤10 | + Marshaler skips |
| `constraintsFor` (constraints.go:180) | 10 | ≤10 | signature loses `underlyingKind` |
| `effectiveKind` (constraints.go:301) | 2 | ≤3 | |
| `namedScalarItems` (1773) / `setNamedScalarType` (1792) | 4 / 1 | deleted | |
| `registerFieldRefAt` (analyzer.go:3588) | 5 | ≤4 | walk + site lookup only; the per-leaf work moves to `registerRefLeaf` |
| `registerTypeAt` (3231) / `resolveTypeSpecChain` (3285) | — | unchanged ±0 | `findStructDefinition` → `structDeclSite`, same branch count |
| new: `structDeclSite` | — | ≤5 | own file, then sorted siblings |
| new: `registerRefLeaf` | — | ≤6 | depth-cap check, register, stamp or demote + warn |
| `extractStructFields` (4009) | 9 | ≤9 | drop the `resolveNamedScalars` call |
| `namedFields` (4045) / `embeddedFields` (4095) / `buildFieldInfo` (4143) | 3 / 7 / 1 | 3 / 7 / ≤2 | signatures gain `astFile, filePath` |
| `warnUintptrField` (4167) | 5 | ≤4 | shape test moves to caller |
| `resolveNamedScalars` (6), `resolveUnderlyingBuiltin` (3), `namedScalarBuiltin` (7), `underlyingScalarBuiltin` (3), `localTypeUnderlyings` (7), `namedTypeUnderlyingInFile` (10), `underlyingIdentString` (3), `shapeMapValueBase` (4) | — | deleted | |
| new: `resolveField` | — | ≤4 | |
| new: `resolveShape` | — | ≤8 | `switch` + `default` |
| new: `resolveSliceElem` | — | ≤6 | |
| new: `resolveQualified` | — | ≤5 | |
| new: `resolveLocal` | — | ≤9 | |
| new: `resolveDecls` | — | ≤6 | |
| new: `resolveDecl` | — | ≤3 | |
| new: `resolveUnderlying` | — | ≤6 | |
| new: `kindBackedUnderlying` | — | ≤4 | |
| new: `mergeShapes` | — | ≤8 | split `mergeElems` if needed |
| new: `mergeDecls`, `firstScalarKind`, `leafKind` | — | ≤5 each | |
| new: `samePackageFiles`, `localTypeDecls` | — | ≤8 each | |
| new: `walkResolvedLeaves`, `holdsUintptr` | — | ≤5 each | |
| new: `warnFieldFallback` | — | ≤6 | |
| new: `marshalerSet` | — | ≤10 | split into `methodsInFile` if over |
| new: `matchMarshalerSignature` | — | ≤8 | table of signature predicates, not a big `switch` |
| new: `fieldTypes`, `isByteSliceExpr`, `isErrorIdent`, `isJSONTextPtr` | — | ≤5 each | |

---

## Task 0: Worktree and baseline

- [ ] **Step 1:** `git fetch -q origin && git worktree add ../go-bricks-openapi-ntr -b fix/named-type-resolution origin/main`, then `cd` there.
- [ ] **Step 2:** `make dev-deps` if `make lint` reports a missing or mismatched golangci-lint. Then run `make check` and `go test ./internal/spectest`; both must be green before any edit. Undo `make fmt`'s rewrites if any appear (there should be none).
- [ ] **Step 3:** Record the baseline: `go run github.com/uudashr/gocognit/cmd/gocognit@latest -over 15 internal/analyzer internal/generator` must print nothing.

## Task 1: Carrier in `models` (additive first, so the build stays green)

**Files:** `internal/models/models.go`

- [ ] **Step 1:** Add the four `ShapeKind` consts, `FieldInfo.Resolution` with `ResolvedShape()`, and `TypeInfo.Resolution` with `ResolvedShape()`, exactly as in **Data model**. **Do not delete the four old fields yet** (Task 6 does it), so the analyzer and generator keep compiling.
- [ ] **Step 2:** `go build ./... && go vet ./internal/models`. There are no tests (`models` is test-free by design).

## Task 2: Marshaler guard, test first

**Files:** create `internal/analyzer/marshaler.go` and `internal/analyzer/marshaler_test.go`

**Interfaces (produced):**

```go
const jsontextImportPath = "encoding/json/jsontext"

// marshalerMethods is what a type's own method set says about its JSON form.
type marshalerMethods struct {
	encode bool   // MarshalJSON, MarshalJSONTo, MarshalText or AppendText
	decode bool   // UnmarshalJSON, UnmarshalJSONFrom or UnmarshalText
	first  string // the first matching method name met, for the warning
}

func (m marshalerMethods) found() bool { return m.encode || m.decode }

func (a *ProjectAnalyzer) marshalerSet(name string, c resolveCtx) marshalerMethods
func matchMarshalerSignature(fd *ast.FuncDecl, imports map[string]string) (encode, ok bool)
func fieldTypes(fl *ast.FieldList) []ast.Expr
```

`resolveCtx` and `samePackageFiles` come from Task 3. Write `samePackageFiles` here first if Task 2 lands alone. It returns `[]pkgFile{file *ast.File; path string}`: the current file first, then `parsePackageDir(filepath.Dir(path))` entries in `slices.Sorted(maps.Keys(…))` order, skipping `p == path` and files whose `Name.Name != file.Name.Name`. A dir error yields just the current file.

- [ ] **Step 1: Failing tests** (`marshaler_test.go`, parsing in-memory sources with `parser.ParseFile(a.fileSet, path, src, 0)` against a `t.TempDir()` path):
  - `TestMarshalerSetSevenMethods`: a table over the seven methods × receivers `T`/`*T`. Assert `found()` and the right `encode`/`decode` side. MarshalJSONTo and UnmarshalJSONFrom need `import "encoding/json/jsontext"`; also run them once with `jt "encoding/json/jsontext"`.
  - `TestMarshalerSetWrongSignatures`: none of these count:
    - `MarshalText() string`
    - `MarshalJSON() []byte`
    - `MarshalJSON() ([]byte, string)`
    - `MarshalJSON(x int) ([]byte, error)`
    - `UnmarshalJSON(s string) error`
    - `UnmarshalJSON(b []byte)` (no result)
    - `AppendText() ([]byte, error)`
    - `MarshalJSONTo(enc *json.Encoder) error` with `import "encoding/json"`
    - `MarshalJSONTo(enc *jsontext.Encoder) error` where `jsontext` is imported from `github.com/go-json-experiment/json/jsontext`
    - a correct method on another type
    - a plain function (nil `Recv`)
  - `TestMarshalerSetNamedResults`: `func (T) MarshalJSON() (b []byte, err error)` counts. `func (T) UnmarshalJSON(a, b []byte) error` (two params via one field) does not.
  - `TestMarshalerSetSiblingFileAndOtherPackage`: a method in a sibling same-package file counts. A method in a `//go:build ignore` `package main` sibling does not. An unreadable dir still scans the current file.
  - `TestFieldTypes`: nil list gives `[]`; `(a, b []byte)` gives 2; `([]byte, error)` gives 2.
- [ ] **Step 2:** Run `go test ./internal/analyzer -run 'TestMarshalerSet|TestFieldTypes'`. It must FAIL (undefined).
- [ ] **Step 3: Implement** `marshaler.go`. Use one table `map[string]signature{params, results []func(ast.Expr, map[string]string) bool; encode bool}` so `matchMarshalerSignature` stays ≤8. Import resolution uses `a.fileImports(file)`, built once per file with matching decls only, not per decl.
- [ ] **Step 4:** The tests PASS. gocognit `-over 15 internal/analyzer/marshaler.go` prints nothing.

## Task 3: Resolver, test first (fills `Resolution` alongside the old fields)

**Files:**
- create `internal/analyzer/resolution.go` and `internal/analyzer/resolution_test.go`
- modify `internal/analyzer/analyzer.go`:
  - `registerTypeAt` 3247–3251, `resolveTypeSpecChain` 3290–3291, `embeddedFields` 4125–4127 (Step 0, `structDeclSite`)
  - `buildFieldInfo` 4143, `namedFields` 4045, the `embeddedFields` call at 4116, `extractStructFields` 4033–4038
  - `registerFieldRefAt` 3588
  - `warnUintptrField` 4167
  - struct field at 130, `New()` at 142 (eager maps), reset at 246
  - `primitiveKind` 3748
- modify `internal/analyzer/builtins.go` (`kindBoolean`, and the comment at line 9)
- modify `internal/analyzer/shape_test.go` (`renderShape`)
- modify `internal/commands/fixed_array_test.go:155` (uintptr text, Step 5)

**Interfaces (produced, analyzer-private):**

```go
const maxNamedResolutionDepth = 8 // parity with resolveTypeSpecChain's chain cap

type resolveMode int
const (
	identityMode   resolveMode = iota // methods count: a direct use, an alias, a nested element
	underlyingMode                    // a defined type's underlying: methods are dropped
)

type fallbackKind int
const (
	fallbackNone fallbackKind = iota
	fallbackMarshaler
	fallbackRecursive
	fallbackUnresolvable
	fallbackUntypedBuiltin
)

type fieldFallback struct {
	kind     fallbackKind
	typeName string
	detail   string // fallbackMarshaler: the method; fallbackUntypedBuiltin: the builtin
}

type resolveState struct {
	param    bool               // path/query/header param: exempt from the Marshaler guard
	stack    []string           // local names being resolved
	first    fieldFallback
	refSites map[string]pkgFile // ShapeRef leaf name -> file it was written in (rule 9), first wins
}

// fieldSite is what registration needs from extraction, keyed by the field's
// Resolution root pointer in a.fieldSites.
type fieldSite struct {
	loc      string             // fieldLoc(field), for the demotion warning
	refSites map[string]pkgFile // from resolveState.refSites
}

type resolveCtx struct {
	file *ast.File
	path string
	st   *resolveState
}

type pkgFile struct {
	file *ast.File
	path string
}

type typeDecl struct {
	spec *ast.TypeSpec
	file *ast.File
	path string
}

func (a *ProjectAnalyzer) structDeclSite(astFile *ast.File, filePath, name string) (*ast.StructType, *ast.File, string, bool) // Step 0
func (a *ProjectAnalyzer) registerRefLeaf(leaf *models.TypeShape, f *models.FieldInfo, site pkgFile, depth int)

func newResolveCtx(file *ast.File, path string, param bool) resolveCtx
func (c resolveCtx) in(file *ast.File, path string) resolveCtx // same state, other file
func (c resolveCtx) quiet() resolveCtx                         // cloned stack (slices.Clone), fresh first, fresh refSites
func (s *resolveState) note(fb fieldFallback)                  // first wins
func (s *resolveState) cut(name string) bool                   // on stack, or len(stack) >= maxNamedResolutionDepth

func (a *ProjectAnalyzer) resolveField(f *models.FieldInfo, field *ast.Field, astFile *ast.File, filePath string)
func (a *ProjectAnalyzer) resolveShape(s models.TypeShape, c resolveCtx) models.TypeShape
func (a *ProjectAnalyzer) resolveSliceElem(e *models.TypeShape, c resolveCtx) *models.TypeShape
func (a *ProjectAnalyzer) resolveQualified(leaf models.TypeShape, c resolveCtx) models.TypeShape
func (a *ProjectAnalyzer) resolveLocal(leaf models.TypeShape, c resolveCtx, mode resolveMode) models.TypeShape
func (a *ProjectAnalyzer) resolveDecls(name string, decls []typeDecl, c resolveCtx, mode resolveMode) models.TypeShape
func (a *ProjectAnalyzer) resolveDecl(name string, d typeDecl, c resolveCtx, mode resolveMode) models.TypeShape
func (a *ProjectAnalyzer) resolveUnderlying(name string, rhs ast.Expr, c resolveCtx) models.TypeShape
func kindBackedUnderlying(qualified string) (models.TypeShape, bool)
func mergeDecls(results []models.TypeShape) (models.TypeShape, bool)
func mergeShapes(x, y models.TypeShape) (models.TypeShape, bool)
func leafKind(s models.TypeShape) string // primitiveKind(Name) for primitives, Name for kind-only, else ""
func firstScalarKind(results []models.TypeShape) string
func (a *ProjectAnalyzer) samePackageFiles(file *ast.File, path string) []pkgFile
func (a *ProjectAnalyzer) localTypeDecls(name string, file *ast.File, path string) []typeDecl
func walkResolvedLeaves(s *models.TypeShape, fn func(leaf *models.TypeShape)) // pointer/slice/array/map Elem only; never Key, never a Marshaler's Elem
func holdsUintptr(s *models.TypeShape) bool
func (a *ProjectAnalyzer) fieldLoc(field *ast.Field) string
func markOnce(seen map[string]struct{}, loc string) bool
func (a *ProjectAnalyzer) warnFieldFallback(f *models.FieldInfo, field *ast.Field, fb fieldFallback)
```

**Go-semantics traps to honour (they caught past plans):**
- `resolveDecls` pushes with `c.st.stack = append(c.st.stack, name)` and pops in a `defer` by reslicing. That is safe only because `st` is a shared pointer and push/pop are strictly nested. `quiet()` must `slices.Clone` the stack, or the quiet run's appends can overwrite the caller's backing array.
- Every container node in a Resolution is freshly allocated, since `registerFieldRefAt` mutates leaves in place. The only shared pointer allowed is a map `Key`, which is never mutated. Never return `&someField.Shape` or a sub-pointer of the Shape.
- `resolveSliceElem` restores `c.st.first` to its snapshot **only** when it substitutes the byte element. The Marshaler's underlying was already resolved quiet, so the snapshot covers exactly the one note made.

- [ ] **Step 0: Extract structs in their declaring file (`structDeclSite`), its own checkpoint.** This lands and is verified before any resolver code, so a golden move later in Task 3 can be attributed.
  - `structDeclSite(astFile, filePath, name)`: `findStructInFile` on the current file (returns it with `astFile, filePath`), then `parsePackage(filePath, astFile.Name.Name)` entries in `slices.Sorted(maps.Keys(files))` order, returning the first hit with **its** file and path. A dir error or no hit gives `ok=false`.
  - Use it at all three `findStructDefinition` callers that drop the file:
    - `registerTypeAt` (3247): `registerStructAt(name, pkg, st, declFile, declPath, depth)`; on `!ok`, `registerViaTypeSpecAt` as today;
    - `resolveTypeSpecChain`'s base case (3290): return `s, declFile.Name.Name, declFile, declPath, true`, so `registerViaTypeSpecAt` extracts `type UA = Holder` (with `Holder` in a sibling file) in `Holder`'s file;
    - `embeddedFields`' local branch (4125): `extractStructFields(embedded, pkg, eFile, ePath, visited, depth+1)`.
  - Leave `findStructDefinition` for `fieldTypeExprOf` (1621); if `unused` or `dupl` objects, make `findStructDefinition` a thin wrapper over `structDeclSite`.
  - Failing test first, `TestStructExtractedInDeclaringFile` (`analyzeDirectiveProject`, real files):
    - `models/item.go` declares `type Item struct{ ID int }` and `type Base struct{ Created string }`.
    - `mod/types.go` has `import m "<module>/models"` and `type Resp struct { m.Base; Items []m.Item; One m.Item }`.
    - `mod/module.go` declares `type RespA = Resp` (**here, not in `types.go`**: with the alias beside `Resp`, `resolveTypeSpecChain` already recurses into `types.go` on `origin/main` and the base-case fix goes untested) and holds both handlers, `server.Result[Resp]` and `server.Result[RespA]`. It does **not** import `m`.
    - Assert **per component**, never only "`typeRegistry` holds `Item`" (either route registering `Item` would satisfy that): `Resp`'s and `RespA`'s own `items` and `one` fields each carry `RefName == "Item"` (the old carrier; `Resolution` does not exist yet), each component has the promoted `created` field (#123's embed row), and the warnings are empty.
    - On `origin/main` both legs fail: `Resp` through `registerTypeAt`, `RespA` through `resolveTypeSpecChain`'s base case (`RespA` resolves in `module.go`, recurses with `module.go`, and the base case finds `Resp` in `types.go` but returns `module.go`).
    - Task 3 Step 2 adds `renderShape(f.ResolvedShape())` assertions (`items` → `[]$Item`, `one` → `$Item`) to the same test; Task 6 drops the `RefName` ones. Do **not** add an embedding variant here (`type Outer struct { Resp }` in `module.go`): its promoted `m.Item` fields are registered from `Outer`'s file, which Step 0 does not fix; rule 9's ref sites do, and `TestSiblingFileRefSites` (Step 2) pins it.
  - Run `go test ./... && go test ./internal/spectest`, then `git status --porcelain internal/spectest/testdata` must be empty. A scratch prototype of the `registerTypeAt` and `embeddedFields` changes on `514e417` kept every test and golden green with no diff and turned the scratch layout's `object` fields into `$ref: Item`. The zero-diff check shows the three changes are neutral for existing output; `TestStructExtractedInDeclaringFile` shows the first two work, and Mutation 4 (Task 9) checks each call site separately.
  - Before Step 0 this layout emitted `object` silently; after it, `$ref` plus a component. That is a pre-existing output-shape bug fixed here, listed in Impact.
- [ ] **Step 1: Extend `renderShape`** (shape_test.go:13) with `ShapeRef` → `"$"+Name`, `ShapeKindOnly` → `"kind:"+Name`, `ShapeMarshaler` → `"marshal:"+Name+"("+renderShapePtr(Elem)+")"`, `ShapeRecursive` → `"cycle:"+Name`. Keep `default: "unknown"`.
- [ ] **Step 2: Failing tests** (`resolution_test.go`). Most go through `analyzeSingleModule` (analyzer_test.go:4516) and read `renderShape(f.ResolvedShape())` per JSON name from `a.typeRegistry[...]`:
  - `TestResolveFieldRows`: one module declaring every R1–R14 type; table `field → rendered resolution`. Examples:
    - `tags` → `[]string`; `tagsPtr` → `*[]string`; `attrs` → `map[string]string`
    - `users` → `[]$User`; `addr` → `*$Address`; `pc` → `*int64`; `flagB` → `bool`; `anyD` → `any`; `shaper` → `interface{}`
    - `pair` → `[N]int`; `arr` → `[N]byte`; `flags3` → `[N]byte`; `flagPtrs` → `[]*byte`
    - `grid` → `[][]int64`; `centsMap` → `map[string]int64`; `usm` → `[]map[string]$User`
    - `blob`/`blobA`/`b2`/`flags`/`raw`/`rawOfRaw` → `[]byte`; `u8s` → `[]uint8`; `flagUs` → `[]byte`
    - `stamp` → `time.Time`; `ida` → `uuid.UUID`; `rawA` → `json.RawMessage`; `amount` → `string`
    - `statusD` → `int`; `odd` → `int`
    - `status` → `marshal:Status(int)`; `statusA` → `marshal:Status(int)`; `statusPtr` → `*marshal:Status(int)`; `tagsM` → `marshal:TagsM([]string)`; `flagVs` → `[]marshal:FlagV(byte)`
    - `tree` → `map[string]cycle:Tree`; `list` → `[]cycle:List`; `left` → `[]map[string]cycle:Left`
    - `decimal` → `decimal.Decimal`; `stampD` → `StampD`; `idd` → `IDD`; `addr2` (`type Addr uintptr`) → `uintptr`
  - `TestResolveRawAndStampAliasesVersusDefined`: `Raw`/`RawA` and `StampD`/`Stamp` resolve differently. Direct `json.RawMessage` and `json.Number` stay well-known leaves, and `*json.RawMessage` gives `*json.RawMessage`.
  - `TestResolveChains`: `B2` and `RawOfRaw` reach `[]byte`. `type X RawA` (defined over an alias of RawMessage) reaches `[]byte`. `Arr [3]byte` is `[N]byte`, never `[]byte`.
  - `TestResolveRecursionTerminates`: `Tree`, `List`, the mutual `Left`/`Right`, and a 9-deep distinct chain `type A0 []A1 … type A8 []A9; type A9 int` (field `F A0`) all terminate, and each gives one recursive warning.
    - Expected renders: `tree` → `map[string]cycle:Tree`; `list` → `[]cycle:List`; `left` → `[]map[string]cycle:Left`.
    - The chain's render is `[][][][][][][][]cycle:A8`. `cut()` checks `len(stack) >= 8` **before** pushing: A0…A7 are pushed (8 names), so A8 is the cut. That gives eight `[]` levels from A0…A7's declarations, then `cycle:A8`.
    - Pin that exact string. If the implementation pushes before checking, the render gains or loses a level, so fix the code, not the expectation.
  - `TestMarshalerGuardPositions`: for each of the seven methods on `T` and on `*T`, field `F T` gives `marshal:T(int)` and one warning. Through `TA = T` the field is Marshaler too. Through `TD T` it is `int` and unwarned. `Odd` is `int`, unwarned. `F T` with `query:"f"` is `int`, unwarned. `F T` with `json:"-"` has a nil Resolution and no warning.
  - `TestByteSliceElementRule`:
    - `[]FlagU` (only `UnmarshalJSON` on `*FlagU`) gives `[]byte` and no warning.
    - `[]FlagT` (only `UnmarshalText`) gives `[]byte`.
    - `[]FlagV` (`MarshalJSON`) gives `[]marshal:FlagV(byte)` plus a warning, and the same goes for `AppendText`, `MarshalText` and `MarshalJSONTo`.
    - `[3]FlagU` gives `[N]marshal:FlagU(byte)` plus a warning. `[]*FlagU` gives `[]*marshal:FlagU(byte)` plus a warning.
    - `[]FlagUA` (`FlagUA = FlagU`) gives `[]byte`.
  - `TestResolveRegistersRefLeavesAtDepth`: fields `[][]User`, `*[][]User`, `[]map[string]User`, `map[string][][]User`, `UserList`, `PAddr`. `User` and `Address` are in `typeRegistry`, and every `$` leaf carries the final name. With a colliding second package, assert a qualified name like `OrdersUser`; reuse the `collision` fixture layout through `analyzeDirectiveProject`, directive_test.go:685.
    - **Sibling-file case (finding: qualified leaves lose their file):** `mod/types.go` imports `<module>/domain` and declares `type Users []domain.User`, `type PAddr2 *domain.Address`, `type UserMap map[string]domain.User`; `mod/api.go` declares `Resp { Users Users; P PAddr2; M UserMap }` and does **not** import `domain`; `mod/module.go` holds the handler. Assert `users` → `[]$User`, `p` → `*$Address`, `m` → `map[string]$User`, `User` and `Address` in `typeRegistry`, and no warnings. Without rule 9 these leaves demote (and, with the demotion warning, warn); this case is what makes rule 9 necessary.
    - **Alias-shadow case:** the same layout, but `api.go` imports a *different* in-module package under the name `domain` that also declares `User`. Assert the registered component is the one from `types.go`'s `domain` (by its fields), not `api.go`'s.
  - `TestSiblingFileRefSites`: rule 9 beyond direct sibling declarations.
    - Promoted cross-package embed (was open decision 7): `shop/receipt.go` has `type Receipt struct { other.Base }`; `other/base.go` has `type Base struct { In Inner; Ins []Inner }` and `type Inner struct{ X int }`. Assert `in` → `$Inner`, `ins` → `[]$Inner`, `Inner` registered, no warnings.
    - Embedding of a sibling-file struct: `type Outer struct { Resp }` in `module.go`, with `Resp { Items []m.Item }` in `types.go` (which alone imports `m`). Assert `items` → `[]$Item`, `Item` registered, no warnings.
  - Extend `TestStructExtractedInDeclaringFile` (Step 0) with resolution assertions: in both `Resp` and `RespA`, `items` → `[]$Item`, `one` → `$Item`.
  - `TestRegisterRefLeafDemotion` (in-package, covers both demotion branches; the analyzer comes from `New()` and never runs `AnalyzeProject`, which pins that `New()` allocates `fieldWarned`/`fieldSites`):
    - A hand-built `FieldInfo` whose `Resolution` is `[]$Missing`, with an `a.fieldSites` entry that has a `loc` but no ref site for `Missing`, passed to `registerFieldRefAt` at depth 1 with an in-memory file: the leaf becomes `ShapeNamed Missing`, and exactly one `unregisteredRefFieldWarning` names the field at that `loc`. A second call for another field sharing the `loc` adds no warning.
    - The same with **no** `a.fieldSites` entry at all: the leaf is demoted and the warning says `unknown position`.
    - The same field at `depth = maxTypeRegistrationDepth + 1`: the leaf is demoted, the depth-exceeded warning fires, and **no** `unregisteredRefFieldWarning` does.
  - `TestMarshalerUnderlyingNotRegistered`: `type Users []User` + `func (Users) MarshalJSON() ([]byte, error)` as the only use of `User`. `User` is **not** in `typeRegistry` (no orphan component).
  - `TestFieldFallbackWarnsOncePerDeclaration`:
    - The same struct is reached from two routes, and promoted through two embedding structs; `A, B Status` shares one declaration. Exactly one Marshaler warning per declaration.
    - The warnings name the case and the field (`Status`, `MarshalText`).
    - A field holding both a uintptr leaf and a Marshaler leaf gets only the uintptr warning.
  - `TestUintptrWrapperAndMapValueWarn`: `type Addr uintptr` field, `map[string]uintptr`, `map[string]Addr` and `[]Addr` each give exactly one uintptr warning and no fallback warning. `TestUintptrFieldWarns` (6933) **stays unchanged and green** (exactly two hits).
  - `TestUnresolvableFieldsWarn`: `decimal.Decimal`, `t.Time` (aliased), `j.RawMessage` (aliased), `types.Cents` (an in-module non-struct; write a `types/` package through `analyzeDirectiveProject`), `StampD`, `IDD`, an undeclared `Missing`, and an unresolvable query param each warn once. A `ShapeUnknown` field (`chan int`, `struct{ X int }`, `Page[int]`) does not warn.
    - Untyped builtins (rule 8): `Err error`, `C64 complex64`, `C128 complex128`, `CW C` (`type C complex128`), `CM map[string]complex128` each give exactly one `untypedBuiltinFieldWarning` naming the builtin (`CW`'s names type `C`). Renders: `error`, `complex64`, `complex128`, `complex128`, `map[string]complex128`.
  - `TestWarnedRowsInEveryPosition` (the brief's "every row in each of its positions" for warned rows). Representatives: `Status` (Marshaler scalar), `TagsM` (Marshaler composite), `Tree` (recursive), `decimal.Decimal` (third-party), `t.Time` (aliased import), `StampD` (defined over a well-known struct), `C` (untyped builtin wrapper) and `Addr` (uintptr wrapper). Positions: `%s`, `*%s`, `[]%s`, `map[string]%s`, `[][]%s`, `map[string][]%s`, `[]map[string]%s`. One generated struct holds all 56 fields (`F<i>P<j>`). Assert per field:
    - the render is the position wrapped around the direct render (`Status` → `marshal:Status(int)`, so `[]map[string]%s` → `[]map[string]marshal:Status(int)`; `Tree` direct is `map[string]cycle:Tree`, so `[]%s` → `[]map[string]cycle:Tree`);
    - exactly one warning contains `"field F<i>P<j> at "`, and its kind matches the row (`Addr` gets only the uintptr warning).
    The emitted schemas for these positions are pinned on the generator side by `TestFallbackLeavesInEveryPosition` (Task 4 Step 2).
  - `TestMergeShapes` (pure): equal, `sameBuiltin` pairs, same-kind leaves giving kind-only, kind-only at depth (`[]int64` vs `[]int32` gives `[]kind:integer`), container-kind mismatch failing, different kinds failing, nil `Elem` pairs.
  - `TestKindBackedUnderlying` (pure): Duration → `int64`; Month/Weekday → `int`; RawMessage → `[]byte`; Number → `string`; `time.Time` and `uuid.UUID` → `false`.
- [ ] **Step 3:** Run `go test ./internal/analyzer -run 'TestResolve|TestMarshalerGuard|TestByteSlice|TestFieldFallback|TestUintptrWrapper|TestUnresolvable|TestMergeShapes|TestKindBacked|TestSiblingFileRefSites|TestRegisterRefLeafDemotion|TestWarnedRowsInEveryPosition'`. It must FAIL to compile.
- [ ] **Step 4: Implement** `resolution.go` per **Resolver semantics**. Then wire it in:
  - `buildFieldInfo(name string, field *ast.Field, astFile *ast.File, filePath string)`: after `parseFieldTags`, call `a.resolveField(&fieldInfo, field, astFile, filePath)`, which replaces the `warnUintptrField` call. Update its comment: tag parsing comes first because `json:"-"` skips resolution and its diagnostics, and a param tag exempts Marshaler types.
  - `namedFields(field, astFile, filePath)`. In `embeddedFields` the json-named branch (4116) passes `astFile, filePath`.
  - `extractStructFields`: **keep** the `a.resolveNamedScalars(shallow, …)` call for now. It is transitional, and Task 6 deletes it and its comment (4035–4038). The new per-field resolution runs in `buildFieldInfo`, already in the declaring file's context.
  - `resolveField`: if excluded, return. Otherwise run `resolveShape` on `newResolveCtx(astFile, filePath, f.ParamType != "")`, set `f.Resolution = &r`, and record `a.fieldSites[f.Resolution] = fieldSite{loc: a.fieldLoc(field), refSites: c.st.refSites}`. Then: `if holdsUintptr(f.Resolution) { a.warnUintptrField(f, field); return }; a.warnFieldFallback(f, field, c.st.first)`.
  - `warnUintptrField`: drop the `shapeBaseName(...) != goTypeUintptr` test (the caller decides). Keep the nil and excluded guards, use `fieldLoc` and `markOnce(a.uintptrWarned, loc)` (its lazy `make` block goes; `New()` allocates the map), and change the text to "holds a uintptr".
  - `registerFieldRefAt`: replace the body after the `isJSONExcluded` guard with a `walkResolvedLeaves(f.Resolution, …)` loop that calls `registerRefLeaf(leaf, f, site, depth)` for each `ShapeRef` leaf, where `site` is `a.fieldSites[f.Resolution].refSites[leaf.Name]`, defaulting to `pkgFile{astFile, filePath}` (a hand-built field, or a leaf with no record).
  - `registerRefLeaf` (rule 9):
    - `capped := depth > maxTypeRegistrationDepth`;
    - `reg := a.registerTypeAt(leaf.Name, site.file.Name.Name, site.file, site.path, depth)`. Always through `registerTypeAt`, never `registerStructAt` directly, so the depth cap (Cycle A's bound) and the qualified/alias routes stay in one place;
    - on success `leaf.Name = reg.Name`;
    - on failure `leaf.Kind = models.ShapeNamed` (emits `object`), and unless `capped` (already warned by `warnDepthExceeded`), raise `unregisteredRefFieldWarning` at the site record's `loc` through `markOnce(a.fieldWarned, loc)`. With no `loc` (a hand-built field), warn with the field name and `"unknown position"`.
  - The `pkg` argument of `registerFieldRefAt` is then unused by the new walk; keep it while the transitional stamping below reads it, and drop it in Task 6 if `unparam` flags it.
  - **Transition only (deleted in Task 6):** keep stamping `RefName`/`MapValueRefName` exactly as today *in addition*, and keep `resolveNamedScalars`, so the generator still reads the old fields and every golden stays byte-identical in this task. Simplest: leave the old function bodies and their calls in place, and add the new walk after them.
  - Doc invariant on `registerFieldRefAt`: each Resolution is registered exactly once, because `registerStructAt` returns a cached `TypeInfo` before re-extracting.
  - `primitiveKind`: add `case s == goTypeBool: return kindBoolean`.
- [ ] **Step 5:** `go test ./internal/analyzer`. The new tests pass.
  - Update `TestPrimitiveKind`'s `bool` row (analyzer_test.go:5635) to `kindBoolean` **in this step**, since Step 4 changed `primitiveKind`.
  - Known transitional side effect, not a bug to chase: the old `underlyingScalarBuiltin` gates on `primitiveKind(u) != ""`, so until Task 6 it also classifies `type X bool` fields as `UnderlyingKind: "boolean"`. No fixture has one, and the old generator would emit `{type: boolean}` from it.
  - Update `internal/commands/fixed_array_test.go:155` from `assert.Contains(t, warnings[0], "is a uintptr")` to `"holds a uintptr"` **in this step** (the text changed in Step 4). It is the only other reader of that text (`git grep -n 'is a uintptr'` on `514e417` finds just it and `analyzer.go:4183`); record it in the commit body.
  - Fix every other **existing** test that now fails *only* because a new warning fires, and record each in the commit body. Expected: none outside `TestResolveUnderlyingBuiltinPredeclaredAliases`'s `Ptr`, which gains a uintptr warning, but that test does not count warnings. No existing fixture or test source declares an `error`/`complex64`/`complex128` field (grepped on `514e417`), so rule 8 adds no collateral warning.
- [ ] **Step 6:** `go test ./... && go test ./internal/spectest`, all green with **no golden change** (the generator still reads the old fields; Step 0's extraction fix and rule 9's sites are golden-neutral because no existing fixture has a sibling-file qualified ref or a promoted cross-package struct ref: `crosspkg`'s embedded `types.Money` holds only scalars).
- [ ] **Step 7:** gocognit `-over 15 internal/analyzer/resolution.go internal/analyzer/marshaler.go internal/analyzer/analyzer.go` prints nothing.

## Task 4: Generator reads only the accessor

**Files:**
- `internal/generator/openapi.go`:
  - `buildFieldProperty` 1522, `refProperty` 1603, `isPointerField` 1639
  - `shapeAfterPointer` 1646 (keep it; payload helpers use it)
  - `setTypeAndFormat` 1731, `namedScalarItems` 1773, `setNamedScalarType` 1792
  - `addFieldSchemaRefs` 1393
  - the `wellKnownFormats` doc 1663–1697
- `internal/generator/constraints.go`: `constraintsFor` 180, `applyValidationConstraints` 245, `effectiveKind` 301
- Tests: `openapi_test.go`, `constraints_test.go`, `fixed_array_test.go`, `shape_builders_test.go`

**Interfaces:**

```go
// stripPointers sheds every pointer level: JSON has no pointer, and the
// retired named-scalar path and shapeBaseName's $ref lookup both ignored
// pointer depth, so **Cents and **Address keep their schema (open decision 5).
func stripPointers(s models.TypeShape) models.TypeShape

func (g *OpenAPIGenerator) setTypeAndFormat(prop *OpenAPIProperty, shape models.TypeShape) // + ShapeRef/KindOnly/Marshaler/Recursive leaves
func refProperty(field *models.FieldInfo, name string) *OpenAPIProperty                     // bare $ref, or allOf+nullable for a body pointer
func addRefNames(out map[string]bool, s models.TypeShape)
func constraintsFor(shape models.TypeShape, constraints map[string]string) *constraintSet  // kind from the leaf
func effectiveKind(base models.TypeShape) string                                            // ShapeKindOnly -> Name; else the classifiers on Name
func isMarshalerLeaf(s models.TypeShape) bool                                               // stripPointers(s).Kind == ShapeMarshaler
```

Bodies (sketches; verify against the live code):

```go
func (g *OpenAPIGenerator) buildFieldProperty(field *models.FieldInfo) *OpenAPIProperty {
	resolved := field.ResolvedShape()
	target := stripPointers(resolved)
	if target.Kind == models.ShapeRef {
		return refProperty(field, target.Name) // a $ref stands alone
	}
	prop := &OpenAPIProperty{Description: field.Description}
	g.setTypeAndFormat(prop, resolved)
	applyValidationConstraints(prop, field)
	// JSON null only for a pointer to a scalar / well-known type; never to a slice
	// or map, and never on a typeless ({}) schema it could not extend.
	if isPointerField(field) && !isSequence(target) && target.Kind != models.ShapeMap && prop.Type != "" {
		prop.Nullable = true
	}
	return prop
}

func (g *OpenAPIGenerator) setTypeAndFormat(prop *OpenAPIProperty, shape models.TypeShape) {
	s := stripPointers(shape)
	if wk, ok := wellKnownShape(s); ok {
		setWellKnown(prop, wk)
		return
	}
	switch s.Kind {
	case models.ShapeSlice, models.ShapeArray:
		prop.Type = typeArray
		prop.Items = &OpenAPIProperty{}
		if s.Elem != nil {
			g.setTypeAndFormat(prop.Items, *s.Elem)
		}
	case models.ShapeMap:
		prop.Type = typeObject
		prop.AdditionalProperties = &OpenAPIProperty{}
		if s.Elem != nil {
			g.setTypeAndFormat(prop.AdditionalProperties, *s.Elem)
		}
	case models.ShapeRef:
		prop.Ref = refPath(s.Name)
	case models.ShapeKindOnly:
		prop.Type = s.Name // #92: the kind alone, no format, no unsigned floor
	case models.ShapeMarshaler, models.ShapeRecursive:
		return // {}: any JSON value; never descend into a Marshaler's Elem
	default:
		setBasicTypeAndFormat(prop, s.Name)
	}
}
```

- `refProperty(field, name)`: `ref := &OpenAPIProperty{Ref: refPath(name)}`. If `isPointerField(field)`, return `{Type: object, AllOf: [ref], Nullable: true, Description}`; otherwise return `ref`. The array branch is gone: `[]User` now takes the generic path, which gives the same output — `Description` on the array, `minItems` applied, `dive` skipped by the `Items.Ref` guard. `refProperty`'s `applyExample` was a no-op on arrays.
- `isPointerField`: `field.ParamType == "" && field.ResolvedShape().Kind == models.ShapePointer` (`type PC *int64` gets `nullable`).
- `applyValidationConstraints`:

```go
	resolved := field.ResolvedShape()
	if len(field.Constraints) > 0 && !isMarshalerLeaf(resolved) {
		constraintsFor(resolved, field.Constraints).applyTo(prop)
	}
	if len(field.ElementConstraints) == 0 || prop.Items == nil || prop.Items.Ref != "" {
		return
	}
	elem := stripPointers(resolved)
	if isSequence(elem) && elem.Elem != nil {
		elem = *elem.Elem
	}
	if isMarshalerLeaf(elem) {
		return // no tag keyword on a Marshaler leaf: format/enum/pattern ignore kind (#88 hazard)
	}
	constraintsFor(elem, field.ElementConstraints).applyTo(prop.Items)
```

- `constraintsFor`: `base := stripPointers(shape)` replaces the one-pointer unwrap, and `effKind := effectiveKind(base)`. The `byteSlice` test is unchanged, so `[]Flag` resolved to `[]byte` drops length bounds (R9).
- `addFieldSchemaRefs`: per field, call `addRefNames(out, ti.Fields[j].ResolvedShape())`. `addRefNames` recurses through pointer/slice/array/map `Elem`, records `ShapeRef` names, and has a `default:` that skips Marshaler and recursive leaves. Keep the nil-entry guard and its comment verbatim.
- Delete `namedScalarItems` and `setNamedScalarType`, and the `UnderlyingKind` branch with the map-ref early return in `buildFieldProperty`. Struct-valued maps now go through `setTypeAndFormat`, and **gain constraint keywords** (R17, open decision 2).
- `wellKnownFormats` doc: the Month/Weekday paragraph's "this entry types the positions that classification skips (map values, payloads)" becomes "(payloads, until #110; fields resolve through knownUnderlyingBuiltins at every depth)". The RawMessage paragraph's "Its underlying []byte is invisible to the AST-only analyzer" becomes "A defined type over it (`type Raw json.RawMessage`) is resolved by the analyzer to `[]byte`; the alias and direct uses keep this entry."

- [ ] **Step 1: Builders** (`shape_builders_test.go`): `refTo(name)`, `kindOnlyLeaf(kind)`, `marshalerOf(name string, under models.TypeShape)`, `recursiveLeaf(name)`, and `withResolution(f *models.FieldInfo, r models.TypeShape) *models.FieldInfo`, which sets `f.Resolution = &r` on a copy.
- [ ] **Step 2: Failing tests** (`openapi_test.go`, new):
  - `TestSetTypeAndFormatResolutionLeaves`:
    - `ShapeRef` at depth: `[][]$User`, `map[string][][]$User`, `[]map[string]$User`.
    - `ShapeKindOnly` at depth: `map[string]kind:integer`, `[][]kind:number`.
    - `ShapeMarshaler` whose `Elem` holds a `$User` emits `{}`, and no `$ref` anywhere.
    - `ShapeRecursive` emits `{}`.
    - `**int64` emits `{integer, int64}`.
  - `TestBuildFieldPropertyReadsResolution`: Shape `named(PC)` with resolution `*int64` gives `nullable`. `named(PAddr)` with `*$Address` gives `allOf`+`nullable`. `named(Tags)` with `*[]string` gives no `nullable`. A Shape-only field with no Resolution behaves as today.
  - `TestMarshalerLeafTakesNoKeywords`:
    - A Marshaler leaf with `min/max`, `oneof`, `email`, or a `regexp` tag gives exactly `&OpenAPIProperty{}`.
    - The same leaf with `example:"active"` gives `{Example: "active"}`.
    - `*Status` gives `{}` (no `nullable`).
    - `[]Status` with `min=1` / `dive,oneof=1 2` gives `{array, minItems 1, items {}}`.
    - `json.RawMessage` with `oneof` still gets `enum` (the brief: other leaves keep today's handling).
  - `TestAddFieldSchemaRefsWalksResolution`: refs at depth are collected, a Marshaler's `Elem` refs are not, and a json:"-" field with only a Shape adds nothing.
  - `TestMapOfStructTakesCardinality`: `map[string]$User` with `min=1` gives `minProperties: 1` (R17).
  - `TestFallbackLeavesInEveryPosition`: the emitted side of the analyzer's `TestWarnedRowsInEveryPosition`. Leaves: `marshalerOf("Status", prim(int))`, `recursiveLeaf("Tree")`, `named("decimal.Decimal")`, `prim("complex128")`, `prim(goTypeUintptr)`. For each, the seven positions are built with `ptrOf`/`sliceOf`/`mapOf` around the leaf as `Resolution` (Shape `named("X")` in the same structure). Expected, with `L` the leaf's schema (`{}` for Marshaler/recursive, `{type: object}` for the other three):
    - `%s` → `L`;
    - `*%s` → `{}` for a `{}` leaf (no type, so no `nullable`); `{type: object, nullable: true}` for an `object` leaf (`buildFieldProperty`'s `prop.Type != ""` rule; confirm against `origin/main`'s output for a direct `*decimal.Decimal` before pinning, and if they differ, stop and report rather than pick one);
    - `[]%s` → `{type: array, items: L}`; `map[string]%s` → `{type: object, additionalProperties: L}`;
    - `[][]%s`, `map[string][]%s`, `[]map[string]%s` → the same nesting around `L`. A pointer behind a container adds nothing.
  - `TestMultiPointerMatchesSinglePointer` (#120's field rows; backs the "fixes #120's field rows" claim). Each row builds two hand-built fields whose `Resolution` differs only in pointer depth and asserts `fieldInfoToProperty` gives equal properties:
    - `**T` and `***T` vs `*T` for `T` ∈ `prim(int64)`, `prim(string)`, `named("time.Time")`, `[]byte`, `[]int64`, `map[string]int64`, `map[string]$Address`, `[]$Address`, `[]int64` (the `**[]Cents` resolution);
    - `**int64` with `validate:"min=1,max=9" example:"3"` vs the same on `*int64` (bounds and example kept);
    - `[]**int64` with `dive,min=1` vs `[]*int64` with the same; `map[string]**int64` vs `map[string]*int64`;
    - a query param `**int64` vs a query param `*int64`: `{integer, int64}`, no `nullable`;
    - #120's "correct today" rows must not move: `**$Address`, `**Cents` (resolution `**int64`), `[]**$Address`, `map[string]**$Address` equal their `*T` forms **and** equal `origin/main`'s output for the same fields (run the `origin/main` binary over a scratch layout under `scratchpad/pr1/` and pin the cells).
    - `Result[**T]` is not a field and is not asserted (payload half, out of scope).
  - `TestUntypedBuiltinsFallToObject` (parity with `models.UntypedBuiltinNames`): `setBasicTypeAndFormat` gives exactly `{type: object}` for every name in `models.UntypedBuiltinNames` and for `uintptr`, and a non-object type for each of `bool, string, byte, rune, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, any, interface{}`. A builtin added to the generator's typed cases, or a new untyped one, then fails here, not silently in the analyzer.
- [ ] **Step 3: Migrate existing tests** (expected output unchanged unless stated). Each moves its construction to `Resolution`:
  - `openapi_test.go`:
    - 938, 1205–1230, 1466–1507: `RefName:` becomes a `$Name` resolution.
    - 1525–1533: the `time.Month` helper becomes resolution `prim(int)`.
    - 1655–1670: `MapValueRefName` becomes `map[string]$Address` and `map[string][]$Address`.
    - 3146–3230: `UnderlyingKind`/`Builtin` become resolutions `prim(int64)` / `[]prim(int64)` / `[][]prim(int64)`; the `[][]Cents` case at 3208 keeps its nested-array and `dive` expectations.
    - 3252 `TestFieldInfoToPropertyNamedScalarMatchesBuiltin`: the named field is Shape `nest(named("N"))` with Resolution `nest(prim(builtin))`. **Delete the byte/uint8 skip (≈3277–3282)**: `[]byte`/`[]uint8` now equal the bare `[]B`'s `{string, byte}` and nested forms, as the brief asks. After migration this comparison is **tautological by construction**: the resolution *is* the hand-expanded shape. Keep it anyway, since the brief requires it skip-free and it still pins that the generator reads `Resolution` over `Shape`. Do **not** route it back through `Shape` to make it "meaningful". The real named-vs-expanded guard is the analyzer-plus-generator `TestNamedResolutionOracle` (Task 5 Step 4); say so in the test's doc comment.
    - 3308/3357: the resolution is `prim(uint32)` / `prim(builtin)`.
    - 3390 `TestFieldInfoToPropertyNamedScalarKindOnly`: the resolution is `kindOnlyLeaf(kind)` (and `ptrOf`/`sliceOf` around it); expected values unchanged.
  - `fixed_array_test.go`:
    - 66–100 `TestFixedArrayFieldMatchesSliceForm`: the Address cases use `seq($Address)`, `*seq(*$Address)` and `map[string]seq($Address)`. **The two `Flag` rows change on purpose; they are the exception to "expected output unchanged".** The test builds each row as a slice and as an array and asserts they are equal. That does not hold for a byte-kind element: only a byte *slice* is base64 (#98, R5 vs R8). With the element resolved to `prim(goTypeUint8)`, the slice form of `[4]Flag` is `{string, byte}`, so the `require` sanity line fails. The slice form of `[2][3]Flag` has base64 items, but the array form has integer-array items. On `origin/main` these rows passed only because `namedScalarItems` typed `[]Flag` as an integer array, which is #97's bug. **Do not change code to make them pass.** Replace the `flag` helper with a `cents` helper: Shape `named("Cents")` and Resolution `prim(goTypeInt64)`, both in the given structure, set with `withResolution`. Rename the rows `[4]Cents` (it keeps its `dive max=9`) and `[2][3]Cents`. They keep the "named scalar element at any depth" coverage. Reword the doc comment's "a named scalar element at any depth" as "a named non-byte scalar element at any depth (a byte-kind element is #98's exception, pinned in TestFixedByteArrayFieldIsIntegerArray)".
    - `TestFixedByteArrayFieldIsIntegerArray`: move the named byte-kind coverage here, against the integer-items expectation. The case struct gains an optional `res *models.TypeShape`; when it is set, the loop wraps the field with `withResolution`. Add these rows:
      - `[4]Flag`: Shape `arrayOf(named("Flag"))`, res `arrayOf(prim(goTypeUint8))` → `byteArr`;
      - `[2][3]Flag`: Shape `arrayOf(arrayOf(named("Flag")))`, res `arrayOf(arrayOf(prim(goTypeUint8)))` → `{array, items: byteArr}`;
      - `[]Flag` as the contrast row: Shape `sliceOf(named("Flag"))`, res `sliceOf(prim(goTypeUint8))` → `{string, byte}` (R8).
    - `TestFixedByteArrayTagsMatchUint16Array`: add one assertion. The same tagged field with Shape `arrayOf(named("Flag"))` and Resolution `arrayOf(prim(goTypeUint8))` equals `got`. This replaces the old `[4]Flag` row's `dive max=9` coverage: a named byte-kind array keeps cardinality and dive bounds.
  - `constraints_test.go`:
    - Drop the `underlyingKind` column.
    - The four rows at 302–326 change shape `named("Cents")`/`named("time.Duration")`/`named("Status")` to `kindOnlyLeaf(typeInteger)`. They keep their names minus "via UnderlyingKind" and their expected values.
    - Call sites 638, 654 and 716 lose the `""` argument.
- [ ] **Step 4:** `go test ./internal/generator` and `go test ./internal/spectest` (**zero golden diff**; if one moves, the parity rule broke, so fix the code), then `go test ./...`.
- [ ] **Step 5:** gocognit `-over 15 internal/generator/openapi.go internal/generator/constraints.go` prints nothing.

## Task 5: Spectest and commands coverage (fixtures, oracle, strict)

**Files:**
- create `internal/spectest/testdata/named_resolution/{go.mod,module.go,types.go,api.go,domain/domain.go}`
- create `internal/spectest/testdata/named_fallback/{go.mod,module.go,api.go,types/cents.go}`
- modify `internal/spectest/spectest_test.go`, `internal/commands/generate_test.go`, `internal/commands/doctor_test.go`
- modify `internal/spectest/testdata/named_scalar_builtins/handler.go` (comment only)

**Fixture rules (CLAUDE.md):** each fixture has a `go.mod` with the module under `github.com/example/`, `go 1.25`, and `require github.com/gaborage/go-bricks v0.53.0`; no `go.sum`, no `replace`. The package name drops the underscore. Fixtures are AST-only, so imports of `github.com/google/uuid`, `github.com/shopspring/decimal` and `encoding/json/jsontext` need no `require`, and method bodies are trivial (`return nil, nil`).

- [ ] **Step 1: `named_resolution`** (`package namedresolution`, module `github.com/example/namedresolution`). It holds **only resolvable rows** and must be strict-clean.
  - `module.go`: the `Module` boilerplate (copy `named_scalar_builtins/module.go`'s shape) and one route, `server.GET(hr, r, "/resolved", m.get, server.WithTags("resolved"))`, where `get(req ResolvedQuery, ctx server.HandlerContext) (server.Result[Resolved], server.IAPIError)`.
  - `types.go` declares:
    - `User`, `Address` (structs); `Cents int64`, `Flag byte`, `U8 = uint8`
    - `Tags []string`, `TagsA = []string`, `Chain Tags`, `Attrs map[string]string`, `Stamps []time.Time`, `Ids []Cents`
    - `UserList []User`, `PAddr *Address`, `PC *int64`, `FlagB bool`, `AnyD any`, `Shaper interface{ Area() float64 }`
    - `Pair [2]int`, `Key [4]byte`, `Arr [3]byte`
    - `Blob []byte`, `BlobA = []byte`, `B2 Blob`, `Flags []Flag`, `Raw json.RawMessage`, `RawOfRaw Raw`
    - `Stamp = time.Time`, `IDA = uuid.UUID`, `RawA = json.RawMessage`, `Amount json.Number`
    - `Status int` + `func (s Status) MarshalText() ([]byte, error)`, `StatusD Status`, `Odd int` + `func (Odd) MarshalText() string`
    - `FlagU byte` + `func (f *FlagU) UnmarshalJSON(b []byte) error`, `TagsM []string` + `func (TagsM) MarshalJSON() ([]byte, error)`
    - **R18, sibling-file layout:** `types.go` is the **only** file that imports `github.com/example/namedresolution/domain`. It declares `Members []domain.Member`, `PMember *domain.Member`, `MemberMap map[string]domain.Member`, and `struct Holder { Item domain.Item \`json:"item"\`; Items []domain.Item \`json:"items"\` }`. **`api.go` and `module.go` must not import `domain`**: if either does, the fixture passes without exercising the bug.
  - `domain/domain.go` (`package domain`): `type Member struct { Name string \`json:"name"\` }`, `type Item struct { SKU string \`json:"sku"\` }`.
  - `api.go`: struct `Resolved`. Each field carries a `// -> …` comment with its R-row cell. It has one field per R1–R10 type, plus these extra positions:
    - `TagsPtr *Tags`, `TagsV Tags validate:"min=1,dive,max=5"`, `Attrs validate:"min=1"`
    - `PCs []PC`, `Flags3 [3]Flag`, `FlagPtrs []*Flag`
    - `Grid [][]Cents`, `CentsMap map[string]Cents`, `CentsSM []map[string]Cents`
    - `UGrid [][]User`, `UGridPtr *[][]User`, `USM []map[string]User`, `UserMap map[string]User validate:"min=1"`
    - `BlobPtr *Blob`, `FlagSlice []Flag`, `FlagGrid [][]Flag`, `FlagMap map[string]Flag`, `U8s []U8`, `FlagUs []FlagU`
    - `FlagV []Flag validate:"min=1,dive,max=3" example:"AQI="`, `StatusD StatusD`, `Odd Odd`
    - R18: `Members Members`, `PMember PMember`, `MemberMap MemberMap`, `Holder Holder`. The golden then locks `$ref`s to `Member` and `Item` and both components.
  - Struct `ResolvedQuery` has query params `Tags`, `*Tags`, `Ids`, `FlagB`, `Amount`, `TagsM` and `Status`. The Marshaler params are exempt, so the fixture stays strict-clean.
- [ ] **Step 2: `named_fallback`** (`package namedfallback`, module `github.com/example/namedfallback`, subpackage `types/cents.go`: `package types\n\ntype Cents int64\n`). Route `GET /fallback` returns `server.Result[Fallback]`. It declares:
  - Marshaler types:
    - `Status int` with `MarshalText`; `StatusA = Status`; `StatusP int` with `func (*StatusP) UnmarshalJSON([]byte) error`
    - `TagsM []string` with `MarshalJSON`; `FlagV byte` with `func (*FlagV) MarshalJSON() ([]byte, error)`
    - `Wire int` with `func (Wire) MarshalJSONTo(enc *jsontext.Encoder) error`; `Appended int` with `func (Appended) AppendText(b []byte) ([]byte, error)`
    - `Users []User` with `MarshalJSON`
  - Recursive types: `Tree map[string]Tree`, `List []List`, `Left []Right`, `Right map[string]Left`
  - Unresolvable, untyped-builtin and uintptr types: `StampD time.Time`, `IDD uuid.UUID`, `C complex128`, `Addr uintptr`
  - Struct `Fallback` has one field per R11–R14 row, with the R12 tag variants (`StatusBound`, `StatusEnum`, `StatusEx`, `StatusPtr`, `Statuses`), `FlagVs []FlagV`, `Decimal decimal.Decimal`, `AliasedTime t.Time`, `AliasedRaw j.RawMessage`, `Cents b.Cents` (`b "github.com/example/namedfallback/types"`), `Err error`, `CW C` and `AddrMap map[string]uintptr`. The golden locks that **no `User` component** is emitted.
- [ ] **Step 3:** `named_scalar_builtins/handler.go` line 27 comment: `// -> {object} (uintptr: never typed, see README)` becomes `// -> {object}, and one analyzer diagnostic (uintptr: never typed, see README)`. This is a comment-only edit, and the golden must not move.
- [ ] **Step 4: Oracle** (`spectest_test.go`, `TestNamedResolutionOracle`). It builds a temp project programmatically:
  - The cases are `{decls, named, expanded, tag, positions}` rows, using the same declarations as `named_resolution/types.go`. **Only Marshaler-free named types are cases in all positions.** The explicit list (named → expanded):
    - `Tags` → `[]string`, `TagsA` → `[]string`, `Chain` → `[]string`, `Attrs` → `map[string]string`, `Stamps` → `[]time.Time`, `Ids` → `[]int64`, `Cents` → `int64`;
    - `UserList` → `[]User`, `PAddr` → `*Address`, `PC` → `*int64`, `FlagB` → `bool`, `AnyD` → `any`, `Shaper` → `interface{ Area() float64 }`;
    - `Pair` → `[2]int`, `Key` → `[4]byte`, `Arr` → `[3]byte`, `Flag` → `byte`, `U8` → `uint8`;
    - `Blob`/`BlobA`/`B2`/`Raw`/`RawOfRaw` → `[]byte`, `Flags` → `[]byte`;
    - `Stamp` → `time.Time`, `IDA` → `uuid.UUID`, `RawA` → `json.RawMessage`, `Amount` → `string`;
    - `StatusD` → `int`, `Odd` → `int` (both Marshaler-free: a defined type drops `Status`'s method, and `Odd`'s `MarshalText() string` does not count);
    - the R18 sibling-file types `Members` → `[]domain.Member`, `PMember` → `*domain.Member`, `MemberMap` → `map[string]domain.Member` (the oracle's struct file must import `domain` for the expanded fields, so the oracle checks equality only; the sibling-file context bug is exercised by `named_resolution` and `TestResolveRegistersRefLeavesAtDepth`, not here);
    - tags from R2/R9: `Tags` with `validate:"min=1,dive,max=5"`, `Attrs` with `validate:"min=1"`, `Flags` with `validate:"min=1,dive,max=3" example:"AQI="`.
  - **Excluded:** `Status`, `TagsM` (Marshaler: `{}` plus a warning, never equal to their expansion) and `User`/`Address` (structs, not named non-struct types). **`FlagU` is a case only in the slice positions** `[]%s`, `[][]%s` and `map[string][]%s`, expanded as `byte` (rule 6 clears it only as a slice element; elsewhere it is a Marshaler leaf that warns), and as its query param (params skip the guard).
  - The positions for every other case are `%s`, `*%s`, `[]%s`, `map[string]%s`, `[][]%s`, `map[string][]%s` and `[]map[string]%s`. Do not weaken the oracle's no-warnings assertion to admit a case: drop or restrict the case instead, and say why in its row.
  - Emit struct `Oracle` with fields `N<i>P<j>`/`E<i>P<j>` (json `n<i>p<j>`/`e<i>p<j>`), plus a request struct with `query:"n<i>"`/`query:"e<i>"` for each case.
  - Run `Generate` and `Validate`, yaml-unmarshal, and assert `props["n…"] == props["e…"]` for every pair and the same for each param schema. A failure message names the case and position.
  - Also run `analyzer.New(dir).AnalyzeProject()` and assert `Warnings` is empty.
- [ ] **Step 5:** Extend `TestNamedScalarBuildTaggedWidths` (spectest_test.go:135). Add `ByKey map[string]Word \`json:"byKey"\`` and `Grid [][]Word \`json:"grid"\`` to `widthsModule`, plus expectations: disagree gives `{type: object, additionalProperties: {type: integer}}` and an array of arrays of `{type: integer}`; agree gives the same with `format: int64`.
- [ ] **Step 6: Commands** (`generate_test.go`), modelled on `TestRunGenerateUnresolvablePayloadFallsBack` (≈1253):
  - `TestRunGenerateFieldFallbacksFailStrict`: one subtest per warned row kind (Marshaler `Status`, `TagsM`, `FlagV`; recursive `Tree`; `decimal.Decimal`; `t.Time`; `StampD`; `IDD`; `error`; `type C complex128`; `Addr uintptr`; `map[string]uintptr`). Each asserts:
    - strict fails with `Warnings: 1\n` and leaves no artifact;
    - a non-strict `--validate` run succeeds and the property is `{}` or `object` as the R-table says.
  - `TestRunGenerateNamedResolutionFixtureStrictClean`: runs `runGenerate` with `Strict: true, Validate: true` on `filepath.Join("..", "spectest", "testdata", "named_resolution")` into a temp output. It returns no error and stdout contains `Warnings: 0\n`.
    - `runGenerate` also runs the go-bricks floor check, which `spectest.Generate` skips. First run `grep -n 'minGoBricksVer\|verifiedGoBricksVer' internal/commands/doctor.go` and confirm the fixture's `v0.53.0` is ≥ the floor and ≤ the verified version. On `514e417` the floor is `v0.45.0` and the verified version is `v0.53.0`, so the run is clean.
    - If the constants have moved and the fixture warns for that reason, assert only that no `field ` warning appears instead. The analyzer-level "no warnings" assertion in the oracle (Step 4) stays the primary strict-clean guard.
- [ ] **Step 7: Doctor** (`doctor_test.go`): one test showing that a project whose only issue is a Marshaler body field prints "Ready with caveats" and its warning. Locate the existing caveats assertion pattern with `grep -n "Ready with caveats" internal/commands/doctor_test.go`.
- [ ] **Step 8:** `go test ./internal/spectest -update`, then `go test ./internal/spectest` (green), then `git status --porcelain internal/spectest/testdata`. Only `named_resolution/`, `named_fallback/` and `named_scalar_builtins/handler.go` may appear.
  - Review **both new goldens line by line** against the R-table. Every property must match its row; a mismatch is a code bug, never a golden to accept.
  - `nested_schema`-style redocly validation is not required.

## Task 6: Delete the old carrier and resolver

**Files:**
- `internal/models/models.go`: delete `RefName`, `MapValueRefName`, `UnderlyingKind` and `UnderlyingBuiltin`.
- `internal/analyzer/analyzer.go`:
  - In `registerFieldRefAt`, delete the transitional `RefName`/`MapValueRefName` stamping and the `UnderlyingKind` clearing. In `extractStructFields`, delete the transitional `resolveNamedScalars` call and its comment.
  - Delete `resolveNamedScalars`, `scalarBuiltin`, `resolveUnderlyingBuiltin`, `namedScalarBuiltin`, `underlyingScalarBuiltin`, `localTypeUnderlyings`, `namedTypeUnderlyingInFile`, `underlyingIdentString` and `shapeMapValueBase`. `unused` would flag them.
  - Keep `shapeBaseName` (`embeddedFields` uses it), `sameBuiltin`, `primitiveKind` and `knownUnderlyingBuiltins`. Update `knownUnderlyingBuiltins`' doc: it names the json.Number defined-chain rule in `kindBackedUnderlying`.
- `internal/analyzer/builtins.go:9`: the comment "(also a value of FieldInfo.UnderlyingKind)" becomes "(also the Name of a ShapeKindOnly Resolution leaf)".
- Analyzer tests to migrate. Each keeps its cases against the new resolver, asserting with `renderShape(f.ResolvedShape())`:

| Test (analyzer_test.go line) | Migration |
|---|---|
| `TestUnderlyingIdentString` (3477) | Becomes `TestResolveDeclUnderlying`: `resolveUnderlying` on the same five exprs. `int64` → `int64`; `pkg.Name` → `ShapeNamed{decl}` plus an unresolvable note; selector-base-not-ident → `unknown`; struct → `unknown`; **`composite_slice` `[]byte` → `[]byte`**, the deliberate update the brief names. |
| `TestRegisterNestedTypes` (3717–3720) | `Addr` → `$Address`; `Parent` → `*$Node`; `Children` → `[]$Node`; `Tags` → `[]string`. |
| `TestRegisterTypeHandlesMutualRecursion` (3758–3759) | `*$B` / `*$A`. |
| map refs (5449–5451) | `addrs` → `map[string]$Address`; `tags` → `map[string]string`. |
| embedded nesting (5512) | `$Base`. |
| `TestPrimitiveKind` (5631) | Already updated in Task 3 Step 5 (`bool` → `kindBoolean`). |
| `TestResolveUnderlyingBuiltin` (5648) | Rows become rendered resolutions: `amount`/`chained`/`wait`/`ttl` → `int64`; `month`/`fiscal` → `int`; `workdays` → `[]int`; `total` → `json.Number`; **`qty` → `string`** (deliberate: `type Quantity json.Number` is a string both ways); `count` → `uint32`; `counts` → `[]uint32`; `ratio` → `float32`; `label`/`plain` → `string`/`int`; `nested` → `$Inner`. |
| `TestLocalTypeUnderlyingSiblingFile` (≈5810) | `localTypeDecls` finds `Cents` in `b.go`; resolving `Cents` gives `int64` and `Wrapped` gives `$Wrapped`; `Missing` gives no decls; an unreadable dir still yields the own file's declaration. |
| `TestLocalTypeUnderlyingBuildTaggedVariantsDeterministic` (≈5848) | 200 iterations: `localTypeDecls("Word")` has two decls in sorted order, and resolving `Word` gives `kind:integer`. |
| `TestLocalTypeUnderlyingsSkipsOtherPackages` (5872) | One decl; `int64`. |
| `TestResolveUnderlyingBuiltinPredeclaredAliases` (6654) | `byte`, `rune`, `uintptr`. Also assert that `ptr` now raises the uintptr warning (README: named wrappers are covered). |
| `TestResolveUnderlyingBuiltinUnclassified` (6697) | `Ext` → `Ext` (named, unresolvable); `Via` → `Ext` (the chain bottoms out in Ext's unresolvable leaf); `A` → `cycle:A`. |
| `TestResolveUnderlyingBuiltinBuildTaggedVariants` (6743) | Per case, `size`/`sizes`/`span` render as: `kind:integer`/`[]kind:integer`/`kind:integer`; `int64`×3; `int64`×3; `kind:integer`×3; `kind:integer`×3; `kind:integer`×3; `kind:string`×3; `kind:string`×3; `uint8` forms (`[]uint8` for sizes, which is now base64: intended, per the brief); `rune` forms; struct variant `$Word` / `[]$Word` / `$Span`. |
| `TestPromotedNamedScalarResolvesInDeclaringPackage` (≈6905) | `amount` → `uint64`, `splits` → `[]uint64`. |
| `TestStructExtractedInDeclaringFile` (Task 3 Step 0) | Drop the `RefName == "Item"` assertions; the `[]$Item` / `$Item` resolution assertions added in Task 3 Step 2 stay. |

- `shape_test.go`: delete `TestShapeMapValueBase` (154–182). Update the comment at 118, which names `namedScalarBuiltin`: it says "registerTypeAt and the resolver".
- [ ] **Step 0: #101's test, failing first** (`TestPromotedFieldIgnoresEmbeddingPackageNames`, `analyzeDirectiveProject`): `other/base.go` declares `type Code string`, `type Inner struct{ X int }` and `type Base struct{ C Code; In Inner }`; `shop/req.go` declares `type Code struct{ V int }`, `type Inner struct{ Z bool }` and `type Req struct{ other.Base }`, returned by a route. Assert `c` → `string`; the `in` leaf's component (looked up by the leaf's final name) has exactly the field `x`; no component has a field `v` or `z` (neither of `shop`'s structs is registered); warnings are empty. Before Step 1 it **fails** on the `v`/`z` assertion, because the transitional `shapeBaseName` stamping still registers `shop`'s `Code` and `Inner` in the embedding file. This is why it lives here and not in Task 3. After Step 1 it passes, which is what `Closes #101` rests on.
- [ ] **Step 1:** Make the deletions and run `go build ./...`. Fix compile errors only by migrating to `ResolvedShape()`; no behavior edits.
- [ ] **Step 2:** `go test ./...` and `go test ./internal/spectest`, green with no golden diff.
- [ ] **Step 3:** `grep -rn 'UnderlyingKind\|UnderlyingBuiltin\|RefName\|MapValueRefName\|resolveNamedScalars\|namedScalarItems\|setNamedScalarType\|underlyingIdentString\|localTypeUnderlyings' internal cmd` finds nothing outside comments that explain history. Reword those too.

## Task 7: Docs

- [ ] **README** (`## Known limitations`, 207+):
  - In the uintptr entry (225–232), replace the closing sentence "The diagnostic only covers fields whose type bottoms out in the builtin `uintptr` (including `[]uintptr`); a named wrapper (`type Addr uintptr`) or a map value (`map[string]uintptr`) still emits `object` silently." with: "The diagnostic covers every field whose type holds a `uintptr` at any depth — `[]uintptr`, a map value (`map[string]uintptr`) and a named wrapper (`type Addr uintptr`) alike."
  - Add three entries after it:
    1. "A field whose type has its own `MarshalJSON`, `MarshalJSONTo`, `MarshalText`, `AppendText`, `UnmarshalJSON`, `UnmarshalJSONFrom` or `UnmarshalText` method (with the exact `encoding/json` signature, on the type or its pointer, or on a type it aliases) is documented as `{}` (any JSON value), with a warning, and its `validate` keywords are dropped: `encoding/json` writes it through that method, not through its underlying type. A defined type over it drops the methods and is documented from the underlying type. A path, query or header parameter of such a type is bound by kind, so it keeps the underlying type's schema and does not warn. A slice of a byte-sized type that only decodes through such a method stays a base64 string. There is no per-field override yet."
    2. "A named type that contains itself through a slice or map (`type Tree map[string]Tree`) is documented down to the point where it recurs, which becomes `{}`, with a warning."
    3. "A field type that resolves to no schema — a third-party type (`decimal.Decimal`), a well-known type under an aliased import (`t.Time`, `j.RawMessage`), a named non-struct type from another package of the project (`b.Cents`), or a defined type over a well-known struct (`type Stamp time.Time`, `type ID uuid.UUID`) — is documented as an untyped object with a warning (so `--strict` fails on it)."
  - Do **not** add #88's overturned "documented from their underlying type" line.
  - The build-tag entry (307–317): append "The same applies wherever such a type appears: slice items, map values and nested containers."
- [ ] **ADR** `docs/adr/0002-named-type-resolution.md`: change front matter `status: proposed` to `status: accepted`, and delete line 6, "Status flips to accepted when the resolver PR (#109) lands." plus the blank line after it.
- [ ] **CONTEXT.md:** no edit. **Resolution** and **Marshaler type** already describe this PR, and Shape stays syntactic. State this in the PR's review notes only if asked.
- [ ] **models.go** comments as in **Data model**. The `wellKnownFormats` doc is done in Task 4.

## Task 8: Lint and complexity

- [ ] `make check` with the pinned golangci-lint. Expect `exhaustive` on any new `ShapeKind` switch missing a `default`, `gocritic` (`unnamedResult`, `paramTypeCombine`, `hugeParam`), `revive` comments on the two exported accessors, and `goconst` on repeated literals such as method names. Fix them; don't `//nolint` them.
- [ ] `go run github.com/uudashr/gocognit/cmd/gocognit@latest -over 15 internal/analyzer internal/generator internal/models` prints nothing. Check the budget table by eye with `gocognit internal/analyzer/resolution.go internal/analyzer/marshaler.go | sort -rn | head`.

## Task 9: Coverage and mutation checks

- [ ] **Coverage:** with `S=<scratchpad>/pr1`, run `go test -coverprofile=$S/cover.out ./internal/analyzer ./internal/generator && go tool cover -func=$S/cover.out | grep -E 'resolution.go|marshaler.go|analyzer.go:.*(structDeclSite|registerRefLeaf|registerFieldRefAt)|openapi.go:.*(setTypeAndFormat|buildFieldProperty|refProperty|isPointerField|addFieldSchemaRefs|addRefNames|stripPointers)|constraints.go:.*(constraintsFor|applyValidationConstraints|effectiveKind|isMarshalerLeaf)'`. Every listed function must be ≥90%; aim for 100%. Uncovered lines must be defensive nil guards that a unit test can still hit with a hand-built `TypeShape{Kind: ShapeSlice}` (nil `Elem`), so add those tests. Spectest and commands lines do not count for these packages.
- [ ] **Mutation 1 (substitution):** commit the work locally first. Then make `resolveShape` return `s` unchanged for `ShapeNamed` local leaves (keep qualified handling) and run `go test ./internal/analyzer ./internal/spectest`. It **must fail**, at minimum in `TestResolveFieldRows`, `TestNamedResolutionOracle` and `TestGoldenFixtures/named_resolution`. Restore with `git checkout -- internal/analyzer/resolution.go` and confirm `git diff` is empty.
- [ ] **Mutation 2 (guard):** make `marshalerSet` return `marshalerMethods{}` and run the same tests. They **must fail**, at minimum `TestMarshalerGuardPositions`, `TestByteSliceElementRule` (`[]FlagV`), `TestFieldFallbackWarnsOncePerDeclaration` and `TestGoldenFixtures/named_fallback`. Restore and confirm the diff is empty.
- [ ] **Mutation 3 (ref sites):** make `registerRefLeaf` ignore `site` and register with `pkgFile{astFile, filePath}` (the struct's file). Run the same tests. They **must fail**, at minimum the sibling-file case of `TestResolveRegistersRefLeavesAtDepth`, `TestSiblingFileRefSites` and `TestGoldenFixtures/named_resolution` (`Member`/`Item` components vanish); `TestRunGenerateNamedResolutionFixtureStrictClean` must also fail (the fixture now warns). Restore and confirm the diff is empty.
- [ ] **Mutation 4 (`structDeclSite` call sites, one at a time):** for each of the three Step 0 call sites, separately, pass the caller's `astFile, filePath` instead of the declaring file and path, run `go test ./internal/analyzer ./internal/spectest`, then restore and confirm `git diff` is empty before the next one. Each **must fail**:
  - 4a, `registerTypeAt`: `TestStructExtractedInDeclaringFile` (the `Resp` leg) and `TestGoldenFixtures/named_resolution` (`Holder`'s `item`/`items`);
  - 4b, `resolveTypeSpecChain`'s base case: `TestStructExtractedInDeclaringFile` (the `RespA` leg, alias in `module.go`);
  - 4c, `embeddedFields`' local branch: `TestSiblingFileRefSites` (the `Outer{Resp}` case: `Resp`'s fields are extracted in `module.go`, `m.Item` goes unresolvable and warns). `TestStructExtractedInDeclaringFile` deliberately has no embedding variant, so 4c is pinned only from Task 3 on; at the Step 0 checkpoint the zero-golden-diff gate is its only guard.
  If one does not fail, the missing test is the bug: add it, don't drop the mutation.
- [ ] Record all four in the PR's `## Verification` (the CLAUDE.md rule: a mutation check that bit belongs there).

## Task 10: Commit

- [ ] One commit (squash-ready), signed. Write the message to a file and run `git commit -F <file>` (the commit hook blocks heredoc `-m`). Subject (Conventional Commit; `fix` because most of the change is the analyzer's resolver; changelog section "Fixed", PATCH bump pre-1.0; a one-commit PR squashes under this subject, not the PR title):

```
fix(analyzer): resolve local named types in every field position
```

Body (standalone; squash history is what a maintainer reads):

```
A local named non-struct type now documents exactly what its underlying
type documents, in every field position: direct, behind a pointer, as
slice or array items, as map values, and nested to any depth. The
analyzer resolves each field once at extraction, in the declaring
file's context, into FieldInfo.Resolution, which reuses the Shape
vocabulary plus four Resolution-only leaf kinds:
- ref: a registered struct;
- kind-only: build-tagged widths disagree (#92);
- marshaler;
- recursive.
Every field consumer reads FieldInfo.ResolvedShape(). The generator
types the resolved form with one recursive walk, retiring the one-level
named-scalar branch and the one-level $ref array. RefName,
MapValueRefName, UnderlyingKind and UnderlyingBuiltin are gone.
TypeInfo.Resolution is declared for payload resolution (#110) and left
nil. A distinct Resolution type was rejected: it would duplicate the
container walk, and the accessor must return Shape's type.

Two context fixes make "resolved in the declaring file" hold:
- A struct's fields are extracted in the file that declares the struct
  (structDeclSite), not the file that referenced it. Before, a struct in
  types.go behind a handler in module.go resolved its qualified field
  types against module.go's imports and emitted object silently.
- Each $ref leaf is registered in the file it was resolved in (a
  per-field site record keyed by the Resolution root), so a sibling
  file's `type Users []domain.User` and a promoted cross-package
  struct's refs register correctly. Residual: one field whose chain
  binds the same alias to two packages in two files registers both
  under the first. A leaf that still fails to register warns, except
  past the depth cap, which has its own warning.
Together these are #123's two seams, so #123 closes here. Its two
residuals stay open: components are keyed by package name, so two
packages both named domain that each register Price share one
component (a separate bug), and a dropped embed still does not warn
(#116). They also close #101: a field promoted from another package
registers its struct refs in that package, and a local struct sharing
a scalar's name no longer captures it.

Resolution rules, following encoding/json:
- An alias inherits its target whole, methods included.
- A defined type drops its target's methods. Over a well-known name it
  inherits only a kind-backed underlying: time.Duration, Month and
  Weekday as before, and json.RawMessage ([]byte) and json.Number
  (string) only at the end of a defined chain.
- A slice of a uint8-kind element is base64 unless the element has an
  encode-side method.
- A named type met again inside itself is cut to {}.
- Build-tagged declarations merge leaf by leaf, falling back to the
  first scalar's kind.
- A struct leaf at any depth becomes a $ref, and its component is
  registered and referenced.

A Marshaler type has any of seven methods with the exact encoding/json
signature (MarshalJSON, MarshalJSONTo, MarshalText, AppendText,
UnmarshalJSON, UnmarshalJSONFrom, UnmarshalText), on the type or its
pointer or through an alias. It is documented as {} with no validate
keywords in a body. Parameters are bound by kind and keep the
underlying schema.

Each field declaration that still falls back raises one warning (deduped
by position), so --strict fails and doctor reports caveats:
- a Marshaler type;
- recursion;
- an unresolvable type: third-party, aliased import, another package's
  named type until #100, or a defined type over time.Time or uuid.UUID;
- an error, complex64 or complex128 leaf (models.UntypedBuiltinNames,
  pinned against the generator's object fallback by a parity test);
- a struct leaf that could not be registered.
The uintptr warning now also covers named wrappers and map values, and
says "holds a uintptr"; TestRunGenerateFixedArrayFields' assertion of
the old "is a uintptr" text is updated to match.

Also: every pointer level is shed when typing, so a **T field emits
what *T emits (**Cents keeps its schema; **int64, **[]T and
**map[string]T stop being object). That fixes #120's field rows,
params included; its Result[**T] payload half stays open. Struct-valued
maps now take minProperties/maxProperties like any other map.
TestFixedArrayFieldMatchesSliceForm's two Flag rows become Cents rows
on purpose: a byte-kind element is the one place a slice and an array
differ (only a byte slice is base64), and those rows passed only
because []Flag was typed as an integer array (#97). Named byte-kind
arrays are now pinned as integer arrays, tags included, in the two
TestFixedByteArray* tests. No existing golden moved. New fixtures
named_resolution (resolvable rows, including the sibling-file layout;
strict-clean) and named_fallback (warned rows), an oracle test comparing
each Marshaler-free named type with its hand-expanded form in seven
positions and as a query parameter, and a seven-position test per
warned row. ADR 0002 is accepted.

Closes #109. Closes #88. Closes #97. Closes #123. Closes #101.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

PR body (three headings, under 150 words; ends with the attribution line):

```markdown
## What
Local named non-struct types fell back to `object` or lost nesting; each now emits its hand-expanded schema in every field position, and sibling-file structs resolve against their own imports. Marshaler types become `{}` in bodies, remaining field fallbacks warn, and `**T` fields emit what `*T` emits (#120's field rows). Closes #109, closes #88, closes #97, closes #123, closes #101.

## Impact
Schemas change for named slices, maps, arrays, pointers, bools and aliases, byte-kind slices (now base64), and nested `[][]T`/`[]map[string]T` (new `$ref`s). `--strict` newly fails, and `doctor` reports caveats, on Marshaler, recursive, unresolvable (including other-package named types until #100), `error`/complex and uintptr-wrapper fields. Marshaler types previously typed by kind are now `{}`.

## Verification
Two new golden fixtures; no existing golden moved. Local gocognit ≤15 on touched files. Mutation checks bit: reverting the substitution, Marshaler guard, ref-site registration or any `structDeclSite` call site fails the new tests.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

---

## Impact (user-visible behavior changes; SemVer surfaces: generated-output shape, doctor/validation)

1. Named slices, maps, arrays, pointers and `bool`/`any`/interface wrappers, and aliases of well-known types (`Stamp = time.Time`, `IDA = uuid.UUID`, `RawA = json.RawMessage`), move from `object` to real schemas in every field position, with their `validate` keywords applied (`minItems`, `minProperties`, `dive`).
2. A pointer to a named slice or map (`*Tags`) emits no `nullable`, exactly as `*[]string` does. A named pointer to a scalar (`type PC *int64`) gains `nullable`.
3. Struct leaves reachable through named composites or nested containers (`UserList`, `PAddr`, `[][]User`, `*[][]User`, `[]map[string]User`, `map[string][][]User`) now emit `$ref`s and their components. `[][]User` and `map[string][][]User` no longer lose a level.
4. Byte-kind slices emit `{type: string, format: byte}`: `type Blob []byte`, `[]Flag`, `type Flags []Flag`, `type Raw json.RawMessage`, `[]U8`, and `[]FlagU` (decode-only methods). They were `object` or integer arrays. Fixed arrays (`[3]Flag`, `type Arr [3]byte`) stay integer arrays.
5. `type Amount json.Number` (and `type Quantity json.Number`) moves from `object` to `{type: string}`.
6. Marshaler types become `{}` in bodies, including ones #78/#92 typed before. `type Status int` with `MarshalText` was `{integer, int64}`. `validate` keywords on them are dropped; `example` is kept verbatim.
7. **`--strict` newly fails, and `doctor` reports "Ready with caveats"**, on fields of:
   - Marshaler types;
   - recursive named types;
   - unresolvable types: third-party (`decimal.Decimal`), aliased imports (`t.Time`, `j.RawMessage`), other-package named types until #100, and `type StampD time.Time` / `type IDD uuid.UUID`;
   - `error`, `complex64` and `complex128` fields at any depth, and named wrappers over them (`type C complex128`);
   - a struct leaf that cannot be registered as a component (it was demoted to `object` silently);
   - named `uintptr` wrappers and `uintptr` map values (the uintptr warning; its text now says "holds a uintptr", which a consumer matching the old text must update).
8. Struct-valued maps (`map[string]User`) gain `minProperties`/`maxProperties` from `validate` (open decision 2).
9. Multi-level pointers are typed as their target: a `**T` field (any depth, params included) emits what `*T` emits (`**int64` was `object`; `**[]Address` was one `Address`; open decision 5). This fixes #120's field rows; its `Result[**T]` payload half stays open.
10. Query, path and header params of named types emit the hand-expanded param schema; Marshaler params are typed by kind and unwarned.
11. **Pre-existing context bug fixed:** a struct declared in a sibling file of the file that references it now has its fields resolved against **its own** file's imports, and promoted cross-package fields register their struct refs in their own package. Fields like `Items []m.Item` (where only the declaring file imports `m`) and a promoted `other.Base`'s `In Inner` move from a silent `object` to a `$ref` plus a component, and a qualified embed in such a struct is no longer dropped. This closes #123 (except its `schemaKey` same-package-name residual and dropped-embed warnings, #116) and #101: a promoted field whose type name also names a local struct (`other.Base{C Code}` with a local `type Code struct`) no longer `$ref`s the local struct.

---

## Open decisions (recommended default first)

1. **Delete `RefName`/`MapValueRefName`/`UnderlyingKind`/`UnderlyingBuiltin` from `FieldInfo`. Default: delete.**
   - The brief says field consumers read only the accessor, and keeping stamped-but-unread fields is the dual-source hazard it warns about.
   - Cost: ~45 hand-built test sites migrate to `Resolution` (Tasks 4 and 6).
   - Alternative: keep `RefName` and have `ResolvedShape()` fold it into the Shape when `Resolution` is nil. That saves ~25 generator test edits but leaves a second source of truth.
2. **Struct-valued maps take `validate` cardinality (R17). Default: accept.** It falls out of the single typing walk, matches every other map and the hand-expanded rule, and moves no golden. Keeping today's drop needs a special case in `buildFieldProperty`.
3. **Unresolvable param fields warn. Default: yes.** Only Marshaler params are exempt, because only they are typed correctly by kind. A `decimal.Decimal` query param is still documented as `object`, which is a real fallback.
4. **A named type whose underlying decodes to `ShapeUnknown`** (`type Fn func()`, `type P Page[int]`, `type X (int)`). **Default: substitute silently** (`object`, no warning), consistent with the brief's "ShapeUnknown leaves are unchanged" and with direct `func()`/generic fields; #114 owns generics. Alternative: warn as unresolvable, since every other remaining fallback warns.
5. **Shed every pointer level in the generator's typing** (`stripPointers` in `setTypeAndFormat`, `buildFieldProperty`, `constraintsFor` and `applyValidationConstraints`). **Default: yes.** Without it, `**Cents` regresses from `{integer, int64}` (the retired named-scalar path ignored pointer depth) to `object`, and `[]**Address` loses its `$ref`. Side effect: hand-expanded `**int64` moves from `object` to `{integer, int64}`, along with the rest of #120's field rows (named in the commit body and Impact 9; #120 stays open for `Result[**T]`). No golden or existing test pins `**`; `TestMultiPointerMatchesSinglePointer` (Task 4) now does.
6. **Depth cap = 8 named types open at once** (parity with `resolveTypeSpecChain`). **Default: 8.** It also cuts legitimate distinct nesting deeper than 8 (`type A0 []A1 … A9`), with the recursion warning. Raise it to e.g. 32 if that is judged realistic.
7. **Promoted cross-package struct-field refs** still register in the embedding struct's file context — **resolved, no longer open** (this is #101 and #123's promoted-field seam; both close). Rule 9's per-field ref sites register every `ShapeRef` leaf in the file it was resolved in, which covers promoted cross-package refs and sibling-file qualified refs alike; any remaining demotion warns (`unregisteredRefFieldWarning`). The one residual left open is decision 8.
8. **Same written name, two meanings, one field** (rule 9's first-wins `refSites[name]`). **Default: accept as a residual** and record it in the commit body. It needs one field whose named-type chain spans two files that bind the same alias to different packages. Alternative: key sites by `(name, import path)` and carry the import path on the leaf, which needs a `TypeShape` field in `models`.
9. **Untyped builtins warn even outside named types** (rule 8: a plain `Err error` field). **Default: yes**, as the brief's "any leaf that would reach `setBasicTypeAndFormat`'s unrecognised-name `object`" says. No existing fixture or test has such a field. Alternative: warn only when the builtin is reached through a named type, which leaves `error` fields silent.
10. **`Closes #123` despite its `schemaKey` residual. Default: close, and file the residual as its own issue before merge** (the planner never touches GitHub, so filing is the user's step). #123's Direction (both seams) is implemented in full and every reproduced row is fixed when only one package named `domain` registers `Price`. The residual (two same-named packages share one component) is the separate bug #123's Notes already call out, and dropped-embed warnings belong to #116. Alternative: write `Refs #123` and leave it open until the `schemaKey` bug is fixed.
11. **`Closes #101` from this PR. Default: yes.** Rule 9 plus Task 6's deletion of the `shapeBaseName` stamping fix both halves, and Task 6 Step 0's test pins #101's exact layout. Alternative: `Refs #101` if the user wants #101 closed by its own PR.

## Self-review notes (planner)

- Every brief acceptance criterion maps to a step:
  - rows and positions: resolvable rows in all seven positions by the oracle (Task 5 Step 4, Marshaler-free cases; `FlagU` in slice positions only); warned rows in all seven positions by `TestWarnedRowsInEveryPosition` (Task 3 Step 2) and `TestFallbackLeavesInEveryPosition` (Task 4 Step 2); directs and tag variants by `TestResolveFieldRows` and the two fixtures;
  - "any leaf that would reach the unrecognised-name `object`": rule 8 (untyped builtins) and the `registerRefLeaf` demotion warning;
  - analyzer specifics: Task 3 Step 2;
  - warn once and `--strict`: Task 3 Step 2 and Task 5 Step 6;
  - deliberate test updates: Task 4 Step 3 and Task 6;
  - unchanged tests: `TestNamedScalarBuildTaggedWidths` (Task 5 Step 5), `TestUintptrFieldWarns`, and the two payload tests;
  - goldens: Task 5 Step 8;
  - README and ADR: Task 7;
  - complexity and coverage: Tasks 8 and 9;
  - mutation: Task 9;
  - commit type and Impact: Task 10.
- The #125 amendment is honoured: `namedScalarItems` is removed and its `[][]Cents` test is kept and migrated. `json.Number` stays out of `knownUnderlyingBuiltins`, and `time.Month`/`time.Weekday` stay in it.
- Golden-parity traps checked against the code:
  - `well_known`'s `Month validate:"min=1,max=12"` and `Workdays dive` keep their bounds only because rule 2.2 substitutes `knownUnderlyingBuiltins`. `setBasicTypeAndFormat("int")` and `wellKnownFormats[time.Month]` both emit `{integer, int64}`.
  - A project struct named like a well-known type stays a `$ref` because rule 2.1 runs first.
  - `refProperty`'s array branch output equals the generic path's.
  - The old map-ref branch differs only in constraint keywords, and no golden has a tagged struct map.

## Review dispositions (2026-10-08 review round)

Each finding was checked against `origin/main` at `514e417` before it was applied.

| # | Severity | Finding | Disposition |
|---|---|---|---|
| 1 | blocker | Qualified `ShapeRef` leaf from a sibling-file declaration is registered with the field's file, so its alias resolves against the wrong imports (silent `object`, or a wrong `$ref` under a shadowing alias). | **Applied.** Confirmed: `registerTypeAt` sends dotted names to `registerQualifiedTypeAt(name, astFile, …)` → `resolveQualifiedStruct` → `fileImports(astFile)`. Fixed by rule 9 (per-field ref sites keyed by the `Resolution` root, registered through `registerTypeAt` so the depth cap holds) and the `unregisteredRefFieldWarning` on any non-depth-cap demotion. Tests: sibling-file and alias-shadow cases in `TestResolveRegistersRefLeavesAtDepth`, `TestRegisterRefLeafDemotion`, R18 in `named_resolution`. The finding's per-leaf-pointer option was not taken: `resolveShape` returns values, so leaves have no stable address. |
| 2 | major | Structs declared in a sibling file are extracted with the referencing file's context, so the plan's rule 2.4 would warn on valid code; the "declaring file" premise is false. | **Applied, wider than stated.** Confirmed in code (`findStructDefinition` drops the file) and by the scratch `xf` project (`Warnings: 0`, `object` fields). New Task 3 Step 0 (`structDeclSite`) at three callers: the two the finding named plus `resolveTypeSpecChain`'s base case (3290), which has the same defect for `type UA = Holder`. Own checkpoint with a zero-golden-diff gate; the scratch prototype of the first two changes was green with no golden diff. Its second half (qualified leaves) is finding 1's fix. Open decision 7 is closed by the same mechanism. |
| 3 | blocker | Changing "is a uintptr" to "holds a uintptr" breaks `internal/commands/fixed_array_test.go:155`, which the plan never lists. | **Applied.** Confirmed by `git grep`. Kept the new wording (accurate for map values and wrappers) and added the test update to Task 3 Step 5, the Task 3 file list and the commit body. |
| 4 | major | The oracle reuses `FlagU`, which only clears in slice positions, yet asserts no warnings in all seven positions. | **Applied.** The oracle's case list is now explicit: Marshaler-free types only in all positions; `Status`/`TagsM` excluded; `FlagU` only in `[]`, `[][]` and `map[string][]` positions (and as a param); no weakening of the no-warnings assertion. |
| 5 | major | Leaves that reach `setBasicTypeAndFormat`'s `object` default without a note (`error`, `complex64`, `complex128`, wrappers over them, demoted refs) emit silently, against the brief. | **Applied.** Confirmed against #109's Warnings bullet and `setBasicTypeAndFormat`'s `default:`. Rule 8 + `untypedBuiltinFieldWarning`, with `models.UntypedBuiltinNames` as the single source and a generator parity test; rule 4 routes builtins through `resolveShape` so `type C complex128` warns; demoted refs warn (finding 1). Grep found no existing field of these types, so no collateral warnings. Open decision 9 records warning on plain `error` fields. |
| 6 | major | Warned rows are covered almost only in the direct position, though the brief requires every row in each position. | **Applied.** `TestWarnedRowsInEveryPosition` (analyzer: render + exactly one warning, 8 representatives × 7 positions) and `TestFallbackLeavesInEveryPosition` (generator: emitted schema, with the pointer rule spelled out per leaf kind). The self-review mapping is corrected. |

No finding was rejected.

## Review dispositions (2026-10-08, round 2)

Each finding was checked against `origin/main` at `514e417` and, for finding 3, against the live issue text.

| # | Severity | Finding | Disposition |
|---|---|---|---|
| 1 | major | `markOnce(seen *map[string]struct{}, …)` fails gocritic `ptrToRefParam`. | **Applied.** Reproduced: the pinned golangci-lint v2.12.2 with this repo's `.golangci.yml` reports `ptrToRefParam: consider 'seen' to be of non-pointer type` on a scratch module. `New()` (analyzer.go:142) is the only constructor and no test builds the struct another way, so the maps are now eager: `uintptrWarned`, `fieldWarned`, `fieldSites` in `New()` and `make` in the `AnalyzeProject` reset; `markOnce` takes a plain map; `warnUintptrField`'s lazy block goes. The plan's old claim that eager init would panic in tests was wrong and is removed. `tagWarned` is explicitly left lazy. |
| 2 | major | `TestStructExtractedInDeclaringFile` cannot catch the `resolveTypeSpecChain` base-case fix (alias beside `Resp`), and a `typeRegistry` check passes on main via the other route. | **Applied.** Trace confirmed (base case returns the caller's file; with `RespA` in `types.go` the recursion already enters `types.go`). `RespA` moves to `module.go`; assertions are per component (`RefName` at Step 0, `ResolvedShape()` from Task 3, `RefName` dropped in Task 6); a qualified embed row (#123's `domain.Base`) is added. New Mutation 4 reverts each call site separately and names the catching test; the `embeddedFields` site is caught by `TestSiblingFileRefSites` from Task 3 on, and only by the zero-diff gate at the Step 0 checkpoint (stated). |
| 3 | major | The plan settles #123, #101 and #120's field rows without naming them; PR body's `Closes #109, #88, #97` links only #109. | **Applied, with tests behind each claim.** Confirmed against the issues: Step 0 + rule 9 are #123's two seams (every row, including the request-body row via the shared `registerType` entry at analyzer.go:3115 and the `Invoice`/`Line` row since `resolveQualifiedStruct` returns the declaring file); #101 follows from rule 9 plus Task 6's deletions; `stripPointers` covers #120's field rows. Repo setting confirmed `squash_merge_commit_message: COMMIT_MESSAGES`. Added `Closes #123`/`Closes #101` (commit body: one `Closes` per issue; PR body: `Closes #109, closes #88, …`, a keyword before every number, kept under the 150-word cap), #120 field-row wording in the commit body, PR What and Impact 9, residuals in Out of scope, `TestPromotedFieldIgnoresEmbeddingPackageNames` (Task 6 Step 0: it cannot pass while the transitional stamping exists) and `TestMultiPointerMatchesSinglePointer` (Task 4). Open decisions 10 and 11 record the close-vs-refs choice. |

No round-2 finding was rejected.

## Review dispositions (2026-10-08, round 3)

The finding was checked against `origin/main` at `514e417`. `internal/generator/fixed_array_test.go:66–110` builds each row through `sliceOf` and `arrayOf` and requires the slice form to be an array equal to the array form. `openapi.go:1720` types a slice of `byte`/`uint8` as base64.

| # | Severity | Finding | Disposition |
|---|---|---|---|
| 1 | major | Migrating `TestFixedArrayFieldMatchesSliceForm`'s `flag` helper to `prim(goTypeUint8)` with "expected output unchanged" fails both `Flag` rows. The slice form becomes base64, and the "fix the code" framing pushes toward reverting #97/#98. | **Applied.** Task 4 Step 3 now states that the two rows change on purpose and must not be fixed in code. They become `[4]Cents`/`[2][3]Cents` (resolution `prim(goTypeInt64)`) and keep the any-depth named-scalar coverage. Named byte-kind arrays are pinned as integer arrays in `TestFixedByteArrayFieldIsIntegerArray` (`[4]Flag`, `[2][3]Flag`, plus `[]Flag` → base64 as the contrast row) and with tags in `TestFixedByteArrayTagsMatchUint16Array`. The commit body records the test update. A `git grep` of `UnderlyingBuiltin` in `origin/main` tests found no byte-kind slice row the plan does not already handle: `TestFieldInfoToPropertyNamedScalarMatchesBuiltin` (3252) has byte/uint8 nested cases, and Step 3 already deletes their skip and expects base64. `TestFieldInfoToPropertyNamedScalarExample`'s named byte is a direct field and is unaffected. |

No round-3 finding was rejected.

## Execution notes

- Branch cut in place with `git checkout -B fix/named-type-resolution origin/main` (as the executor's task text directs), not in the `../go-bricks-openapi-ntr` worktree Task 0 Step 1 describes. Baseline `make check` and gocognit were green on `514e417`.
- Every zero-golden-diff gate held: after Task 3 Step 0 (`structDeclSite` alone), Task 3 Step 6, Task 4 Step 4 and Task 6 Step 2. The single `-update` run (Task 5 Step 8) touched only `named_resolution/`, `named_fallback/` and the `named_scalar_builtins/handler.go` comment; both new goldens were checked cell by cell against R1–R18. `*decimal.Decimal`, `**Address`, `**Cents`, `[]**Address` and `map[string]**Address` were confirmed against a `514e417` binary before being pinned (they match the plan's cells).
- **Unconstructible test row (Task 3 Step 2, `TestFieldFallbackWarnsOncePerDeclaration`):** "a field holding both a uintptr leaf and a Marshaler leaf" cannot be written: a Resolution has one leaf chain, and a Marshaler leaf's `Elem` is never walked. The precedence is pinned instead by `TestUintptrOutranksFallback`: resolveField's warning tail is extracted as `warnResolvedField(f, field, fb)`, and a hand-built uintptr Resolution with a noted Marshaler fallback gets only the uintptr warning (and dedupes); `type P uintptr` with `MarshalJSON` renders `marshal:P(uintptr)`.
- **Helpers beyond the interface list:** `registerResolutionRefs` (the walk `registerFieldRefAt` delegates to; it let the Task 3 transitional stamping coexist), `warnResolvedField` (above), `resolveElem` and `noteUntypedBuiltin` (split out of `resolveShape`, 10 → 3), `mergeSameKind` (out of `mergeShapes`, 11 → 5), `marshalerCandidate` and `marshalerMethods.with` (out of `methodsInFile`, 14 → 7), so every function meets the budget table, not just the 15 cap. `registerFieldRefAt` lost its unused `pkg` parameter in Task 6.
- **Dead guards dropped for coverage:** `warnUintptrField`'s `field == nil || isJSONExcluded` guard (resolveField already returns for json:"-", and the field is never nil) and `matchMarshalerSignature`'s `fd.Type == nil` (go/parser always sets it).
- **goconst:** `"complex64"`/`"complex128"` hit the threshold once the tests used them; they are now `goTypeComplex64`/`goTypeComplex128` in `builtins.go`.
- **Test placement:** new generator tests live in `internal/generator/resolution_test.go` (not `openapi_test.go`), the oracle in `internal/spectest/oracle_test.go` (not `spectest_test.go`), and the strict/fixture/doctor command tests in `internal/commands/named_resolution_test.go` (not `generate_test.go`/`doctor_test.go`). Two test builders were added for the mechanical migration: `substNamed` and `resolvedTo` (`shape_builders_test.go`).
- **Extra coverage tests:** `TestResolveDegenerateShapes` (containers with nil `Elem`, nil Resolution), `TestResolveDeclsWithNoCommonShape` (the merge's last, unresolvable fallback and a same-kind element mismatch), `TestMatchMarshalerSignatureUnknownName`, and three more wrong-signature rows (an unrelated method, a local `*Encoder`, `*jsontext.Decoder` for `MarshalJSONTo`). Every function in `resolution.go`, `marshaler.go` and the touched generator/constraint functions is at 100% statement coverage in its own package's profile.
- **Mutation checks (Task 9), run against the commit, each restored with `git checkout` to an empty diff:** (1) `resolveShape` returning local named leaves unchanged fails `TestResolveFieldRows`, `TestNamedResolutionOracle` and `TestGoldenFixtures/named_resolution` (plus most resolver tests and nine existing goldens, since local struct refs also come from `resolveLocal`); (2) `marshalerSet` returning nothing fails `TestMarshalerGuardPositions`, `TestByteSliceElementRule`, `TestFieldFallbackWarnsOncePerDeclaration` and `TestGoldenFixtures/named_fallback`; (3) registering every ref leaf in the struct's file fails both sibling-file cases of `TestResolveRegistersRefLeavesAtDepth`, `TestSiblingFileRefSites`, `TestPromotedFieldIgnoresEmbeddingPackageNames`, `TestGoldenFixtures/named_resolution` and `TestRunGenerateNamedResolutionFixtureStrictClean`; (4a) `registerTypeAt` fails `TestStructExtractedInDeclaringFile`, `TestNamedResolutionOracle` and `TestGoldenFixtures/named_resolution`; (4b) `resolveTypeSpecChain`'s base case fails `TestStructExtractedInDeclaringFile`; (4c) `embeddedFields`' local branch fails `TestSiblingFileRefSites/embedded_sibling-file_struct`.
