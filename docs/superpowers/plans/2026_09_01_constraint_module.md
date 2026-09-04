# Constraint Module Implementation Plan (PR #2, shipped as 2a + 2b)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the validate-tag → OpenAPI-keyword pipeline into one deep, generator-internal module: a typed constraint set replaces the `(string, any)` pair list, the 13 pass-through applicators and the `\x00bound` placeholder machinery disappear, and callers get one entry point that fills a property (collection and element scope) without knowing `dive` exists.

**Architecture:** Two PRs. **2a** is a pure relocation: `internal/analyzer/constraints.go` (+ test) moves to `internal/generator`, both exported names become unexported, the inverted analyzer←generator seam closes. Behavior and tests byte-identical apart from package/constant reconciliation. **2b** is the deepening on top: `constraintSet` (typed fields, most-restrictive merge at set time, last-sorted-key-wins for `format`/`pattern`/`enum`), `applyTo(prop)` replacing the applicator map, and `applyValidationConstraints(prop, field)` as the single entry point that owns the element (`dive`) path including the "$ref items take nothing" rule.

**Tech Stack:** Go 1.25 floor; `go/ast` static analysis; goldens in `internal/spectest`.

**Spec:** Grilling decisions Q5/Q10/Q11/Q12/Q13 (this file is the operative record); vocabulary in `/CONTEXT.md` ("Constraint set"). Baseline commit for line anchors: `faaa718` (main after PR #55).

## Global Constraints

- **Bit-for-bit behavior preservation.** `go test ./internal/spectest` passes untouched; NEVER `-update` in these PRs. A golden diff is a bug.
- Silent drops stay silent (unmapped validate keys, unparsable numerics, `ne`, bare `ip`, `[]byte` length bounds). No new warnings, `--strict` unchanged.
- **Example coercion stays in `fieldInfoToProperty`** — the constraint module must not touch `Example`.
- **Ordering invariant (Q13):** constraints apply AFTER `setTypeAndFormat`; the uint `minimum: 0` pre-stamp (`openapi.go` ≈1573/1580 in `setBasicTypeAndFormat`) relies on being overwritten by an explicit `min`/`gte`. Pin it with a test in 2b; folding the floor into the module is issue #54, not this PR.
- `internal/models` untouched. No `_test.go` under `internal/models`.
- Settled invariants untouched: `lookupStructTag`, `unquoteLiteral`, `hiddenTagKeys`, `referencedSchemaNames`, `schema == nil`.
- Gates before each PR: `make fmt lint test` (pinned golangci-lint via `make dev-deps` — a newer local version can pass where the pinned one fails, seen on PR #55), `go test ./internal/spectest`, `go run github.com/uudashr/gocognit/cmd/gocognit@latest -over 15 <touched files>` (SonarCloud S3776 is server-side and blocking; `make lint` does not check it). Baseline: `MapConstraintToOpenAPI` is at 12, `buildFieldProperty` at 12 — headroom is thin, split before pushing.
- Sonar new-code ≥80% coverage: moved lines count as new code — the moved tests carry coverage only if they still execute; the new `constraintSet` code needs direct unit tests, not golden-only coverage.
- Squash-only; PR body = `## What` / `## Impact` / `## Verification`, ≤150 words, no gate transcripts. Personal repo: no `Refs:`.
- Every gh command: `GH_TOKEN=$(gh auth token -u gaborage) gh ...`, one gh command per shell invocation. Do not `@coderabbitai` nudge; it auto-reviews new heads. If rate-limited, wait for its stated window, then nudge once.
- Work in a fresh worktree off `origin/main` (the `go-bricks-openapi-shape` worktree is stale — PR #55 is merged, its branch deleted).

---

## PR 2a — relocate the constraint mapper into the generator

Branch: `refactor/constraint-module-move`. Title: `refactor(generator): move constraint mapping out of the analyzer` (≤72 chars).

### Task 1: Split the Go-builtin classifiers the analyzer keeps

**Files:**
- Create: `internal/analyzer/builtins.go`
- Create: `internal/analyzer/builtins_test.go`
- Modify: `internal/analyzer/constraints.go` (remove what moved), `internal/analyzer/constraints_test.go` (remove the two moved tests)

**Interfaces:**
- Produces (analyzer-private, unchanged names): `isStringType`, `isIntegerType`, `isFloatType`, `isNumericType` — consumed by `isBuiltinShapeName` (`analyzer.go` ≈3380) and, until Task 2, by `constraints.go`.
- Also keeps analyzer-side the constants `constraints.go` currently hosts but `analyzer.go` reads: `goTypeByte`, `goTypeUint8`, `goTypeBool`, `goTypeAny`, `goTypeInterface`, `unknownTypeName`, `kindInteger` (used by `knownUnderlyingKinds` ≈2913), `goTypeInt64`, `goTypeFloat32`, `goTypeFloat64` (used by the classifiers). Verify each with `grep -n '<name>' internal/analyzer/analyzer.go internal/analyzer/tagcheck.go` before deciding where it lives; anything referenced outside `constraints.go` stays in the analyzer.

- [ ] **Step 1: Create `builtins.go`** with the four classifier functions moved verbatim from `constraints.go` ≈633–660, plus the constants they and `analyzer.go` need (as identified by the grep above). Header comment:

```go
// Go builtin type-name classification. Used by the Shape decoder to tell a
// primitive from a named type. The generator's constraint module keeps its own
// classification for OpenAPI-kind purposes; the two lists are the same lexical
// fact (Go's builtin numeric/string types) and change only when Go does.
```

- [ ] **Step 2: Move `TestIsStringType` and `TestIsNumericType`** (`constraints_test.go` ≈672–725) into `builtins_test.go` verbatim.

- [ ] **Step 3: Verify** `go build ./... && go test ./internal/analyzer` — PASS. `constraints.go` still compiles because it reads the classifiers from the same package.

- [ ] **Step 4: Commit**

```bash
git add internal/analyzer/
git commit -m "refactor(analyzer): keep Go builtin classifiers in their own file"
```

### Task 2: Move `constraints.go` + test into `internal/generator`, unexport, reconcile

**Files:**
- Move: `internal/analyzer/constraints.go` → `internal/generator/constraints.go`
- Move: `internal/analyzer/constraints_test.go` → `internal/generator/constraints_test.go`
- Modify: `internal/generator/openapi.go` (call sites ≈1463, ≈1625, ≈1653; drop the `analyzer` import if nothing else uses it — ≈575 is a comment only)

**Interfaces:**
- Produces (generator-private): `func mapConstraintToOpenAPI(shape models.TypeShape, underlyingKind string, constraints map[string]string) []openAPIConstraint`, `type openAPIConstraint struct{ Name string; Value any }`. Everything else in the file unchanged.

- [ ] **Step 1: `git mv` both files.** Change `package analyzer` → `package generator` in both.

- [ ] **Step 2: Unexport.** `MapConstraintToOpenAPI` → `mapConstraintToOpenAPI`, `OpenAPIConstraint` → `openAPIConstraint` (all occurrences in both moved files and in `openapi.go`). Drop `analyzer.` prefixes at the three `openapi.go` sites; remove the `internal/analyzer` import when `go build` reports it unused.

- [ ] **Step 3: Reconcile constants and helpers** — run `go build ./internal/generator` and resolve every "redeclared" / "undefined" error with these rules:
  - Redeclared with the **same value** in `openapi.go` (expect `goTypeFloat32`, `goTypeFloat64`, `goTypeBool`, `goTypeAny`, `goTypeInterface`, `formatDateTime`, `formatUUID`, possibly `goTypeInt64`/`kindInteger`-equivalents like `typeInteger`): delete the copy in the moved file; if the generator's name differs but the value is identical (e.g. generator `typeInteger = "integer"` vs moved `kindInteger = "integer"`), switch the moved file to the generator's name.
  - Undefined because it lived in `analyzer.go` (expect `constraintRequired = "required"`, `boolTrueString = "true"`, `goTypeString = "string"` — the last probably already exists in the generator): add to the moved file's const block with the same value.
  - The classifiers `isStringType`/`isIntegerType`/`isFloatType`/`isNumericType` are now undefined in the generator (they stayed in analyzer, Task 1): add generator-private copies to `constraints.go` **verbatim** (same bodies). This is the accepted one-list-per-package duplication; do not export or share across packages.
  - Test-only builder collisions: `prim`/`named`/`ptrOf`/`sliceOf`/`mapOf` already exist in `internal/generator/shape_builders_test.go` — delete them from the moved test; keep `unknownShape()` only if the generator lacks it.
  - `models` import: keep in `constraints.go` (for `models.TypeShape`).

- [ ] **Step 4: Verify identity of the move.** `git diff -M --stat HEAD~1` must show both files as renames (`R0xx`), and the non-rename hunks must be limited to: package clause, capitalization of the two names, constant/builder reconciliation. If the diff shows a logic hunk, revert it — 2a changes no behavior.

- [ ] **Step 5: Gates.** `make fmt lint test && go test ./internal/spectest`. gocognit on `internal/generator/constraints.go` — unchanged from baseline (12 on the mapper). Also confirm the coverage gate has a chance: `go test ./internal/generator -cover` should report the constraints file's functions executed (the moved table test does this).

- [ ] **Step 6: Commit + PR**

```bash
git add -A
git commit -m "refactor(generator): move constraint mapping out of the analyzer

The mapper emits OpenAPI vocabulary (minLength, pattern, enum) and had
exactly two callers, both in the generator; living in the analyzer put
the seam on the wrong side and forced an exported surface nobody outside
the generator used. Pure relocation: behavior and tests unchanged beyond
package/constant reconciliation. The Go-builtin classifiers the Shape
decoder relies on stay analyzer-side (builtins.go)."
```

PR body:

```markdown
## What
Validate-tag → OpenAPI-constraint mapping lived in `internal/analyzer` but emits generator vocabulary and was called only from the generator, so the seam pointed the wrong way. The mapper and its tests move into `internal/generator`, unexported; the Go-builtin classifiers the Shape decoder uses stay analyzer-side.

## Impact
None. Pure relocation — the follow-up PR replaces the pair-list internals with a typed constraint set.

## Verification
Golden fixtures byte-identical (no `-update`). `git diff -M` shows both files as renames; non-rename hunks are package/constant reconciliation only.
```

Wait for merge before starting 2b (line anchors below assume 2a's tree).

---

## PR 2b — typed constraint set, single entry point

Branch: `refactor/constraint-set` off `origin/main` after 2a merges. Title: `refactor(generator): typed constraint set replaces pair-list applicators` (≤72 chars — count it; trim to `refactor(generator): typed constraint set replaces applicators` if over).

### Task 3: `numericBound` + `constraintSet` with merge semantics, tested first

**Files:**
- Modify: `internal/generator/constraints.go` (new types replace `openAPIConstraint`, `boundState`, `boundPlaceholder`, `resolveMostRestrictive`, `readBound`, `exclusivePartner`, `mergeBound`, `compareBoundValues`, `boundInt64`, `boundFloat64`, `materializeBounds`, `lowerBoundKeywords`, `upperBoundKeywords`, `isLowerBound`, `isUpperBound`, `isBoundKeyword`)
- Test: `internal/generator/constraints_test.go` (new tests in this task; table conversion is Task 5)

**Interfaces:**
- Produces:

```go
// numericBound is one candidate or resolved numeric bound (minimum/maximum).
// Integer-valued bounds keep int64 precision so two distinct values above 2^53
// never collapse when compared; only a fractional bound compares as float64.
type numericBound struct {
	intVal    int64
	floatVal  float64
	isFloat   bool
	exclusive bool // the bound came from gt/lt (exclusiveMinimum/Maximum: true)
}

// constraintSet is the typed image of one validate tag in OpenAPI vocabulary
// (CONTEXT.md: "Constraint set"). Zero value = no constraints. Bounds hold the
// most-restrictive candidate seen so far; format/pattern/enum hold the last
// value set (callers set them in sorted validator-key order, so precedence is
// "last sorted key wins", matching the retired last-writer-wins applicators).
type constraintSet struct {
	format  string
	pattern string
	enum    []any
	minLength, maxLength         *int
	minItems, maxItems           *int
	minProperties, maxProperties *int
	minimum, maximum             *numericBound
}

func (s *constraintSet) setFormat(v string)
func (s *constraintSet) setPattern(v string)
func (s *constraintSet) setEnum(v []any)
func (s *constraintSet) setMinLength(n int)     // keeps the LARGER of existing/new
func (s *constraintSet) setMaxLength(n int)     // keeps the SMALLER
func (s *constraintSet) setMinItems(n int)      // larger
func (s *constraintSet) setMaxItems(n int)      // smaller
func (s *constraintSet) setMinProperties(n int) // larger
func (s *constraintSet) setMaxProperties(n int) // smaller
func (s *constraintSet) setMinimum(b numericBound) // larger; on tie exclusive ||= cand.exclusive
func (s *constraintSet) setMaximum(b numericBound) // smaller; on tie exclusive ||= cand.exclusive
func (s constraintSet) applyTo(prop *OpenAPIProperty) // writes only set fields
```

- [ ] **Step 1: Write the failing tests** — these pin the exact semantics `resolveMostRestrictive`/`mergeBound` have today (read `constraints.go` ≈123–267 in the 2a tree side by side while writing expectations):

```go
func TestConstraintSetBoundsKeepMostRestrictive(t *testing.T) {
	var s constraintSet
	s.setMinLength(1)
	s.setMinLength(10)
	s.setMinLength(5)
	if *s.minLength != 10 {
		t.Errorf("minLength = %d, want 10 (largest floor)", *s.minLength)
	}
	s.setMaxLength(100)
	s.setMaxLength(20)
	s.setMaxLength(50)
	if *s.maxLength != 20 {
		t.Errorf("maxLength = %d, want 20 (smallest ceiling)", *s.maxLength)
	}
}

func TestConstraintSetNumericTieExclusiveWins(t *testing.T) {
	var s constraintSet
	s.setMinimum(numericBound{intVal: 5})                  // gte=5
	s.setMinimum(numericBound{intVal: 5, exclusive: true}) // gt=5
	if !s.minimum.exclusive || s.minimum.intVal != 5 {
		t.Errorf("tie must keep exclusive: %+v", *s.minimum)
	}
	// A strictly larger inclusive floor beats a smaller exclusive one and drops
	// its exclusivity (exclusivity travels with the winning value).
	s.setMinimum(numericBound{intVal: 7})
	if s.minimum.exclusive || s.minimum.intVal != 7 {
		t.Errorf("larger inclusive floor must win outright: %+v", *s.minimum)
	}
}

func TestConstraintSetNumericInt64Precision(t *testing.T) {
	var s constraintSet
	s.setMinimum(numericBound{intVal: 9007199254740993}) // 2^53 + 1
	s.setMinimum(numericBound{intVal: 9007199254740992}) // 2^53
	if s.minimum.intVal != 9007199254740993 {
		t.Errorf("int64 bounds must compare as int64, got %d", s.minimum.intVal)
	}
	var f constraintSet
	f.setMaximum(numericBound{floatVal: 2.5, isFloat: true})
	f.setMaximum(numericBound{intVal: 3})
	if !f.maximum.isFloat || f.maximum.floatVal != 2.5 {
		t.Errorf("mixed compare must fall back to float64 and keep the winner's type: %+v", *f.maximum)
	}
}

func TestConstraintSetLastSetWinsForFormatPatternEnum(t *testing.T) {
	var s constraintSet
	s.setFormat("email")
	s.setFormat("uuid")
	s.setPattern("^a")
	s.setPattern("^b")
	s.setEnum([]any{"x"})
	s.setEnum([]any{"y"})
	if s.format != "uuid" || s.pattern != "^b" || len(s.enum) != 1 || s.enum[0] != "y" {
		t.Errorf("last set must win: %+v", s)
	}
}

func TestConstraintSetApplyToWritesOnlySetFields(t *testing.T) {
	prop := &OpenAPIProperty{Minimum: floatPtr(0)} // the uint pre-stamp
	(constraintSet{}).applyTo(prop)
	if prop.Minimum == nil || *prop.Minimum != 0 {
		t.Error("empty set must not clear the uint minimum:0 pre-stamp")
	}
	s := constraintSet{minimum: &numericBound{intVal: 5, exclusive: true}, minLength: intPtr(3), enum: []any{"a"}}
	s.applyTo(prop)
	if *prop.Minimum != 5 || prop.ExclusiveMinimum == nil || !*prop.ExclusiveMinimum || *prop.MinLength != 3 || len(prop.Enum) != 1 {
		t.Errorf("applyTo did not write set fields: %+v", prop)
	}
	if prop.ExclusiveMaximum != nil || prop.Maximum != nil {
		t.Error("applyTo must not write unset fields")
	}
}

func intPtr(n int) *int { return &n }
```

(`floatPtr` exists in `openapi.go`. If `intPtr` already exists in a generator test file, reuse it.)

- [ ] **Step 2: Run, verify failure** — `go test ./internal/generator -run 'TestConstraintSet' -v` → undefined types.

- [ ] **Step 3: Implement.** Replace the whole block ≈123–267 (from `lowerBoundKeywords` through `materializeBounds`) with:

```go
type numericBound struct {
	intVal    int64
	floatVal  float64
	isFloat   bool
	exclusive bool
}

// boundFromParsed converts a parseNumeric result (int64 or float64) into a bound.
func boundFromParsed(v any, exclusive bool) numericBound {
	if f, ok := v.(float64); ok {
		return numericBound{floatVal: f, isFloat: true, exclusive: exclusive}
	}
	return numericBound{intVal: v.(int64), exclusive: exclusive} // parseNumeric only yields int64 or float64
}

func (b numericBound) float64Value() float64 {
	if b.isFloat {
		return b.floatVal
	}
	return float64(b.intVal)
}

// compareNumericBounds orders two bounds: int64 against int64 exactly, anything
// involving a float as float64. Returns -1/0/1.
func compareNumericBounds(a, b numericBound) int {
	if !a.isFloat && !b.isFloat {
		return cmp.Compare(a.intVal, b.intVal)
	}
	return cmp.Compare(a.float64Value(), b.float64Value())
}

type constraintSet struct {
	format                       string
	pattern                      string
	enum                         []any
	minLength, maxLength         *int
	minItems, maxItems           *int
	minProperties, maxProperties *int
	minimum, maximum             *numericBound
}

func (s *constraintSet) setFormat(v string)  { s.format = v }
func (s *constraintSet) setPattern(v string) { s.pattern = v }
func (s *constraintSet) setEnum(v []any)     { s.enum = v }

// keepLarger/keepSmaller implement most-restrictive for integer keywords:
// validator/v10 enforces ALL rules, so the binding floor is the largest and
// the binding ceiling the smallest when distinct validator keys collapse onto
// one OpenAPI keyword (min & gte -> minimum, min/len/gt -> minLength, ...).
func keepLarger(dst **int, n int) {
	if *dst == nil || n > **dst {
		v := n
		*dst = &v
	}
}
func keepSmaller(dst **int, n int) {
	if *dst == nil || n < **dst {
		v := n
		*dst = &v
	}
}

func (s *constraintSet) setMinLength(n int)     { keepLarger(&s.minLength, n) }
func (s *constraintSet) setMaxLength(n int)     { keepSmaller(&s.maxLength, n) }
func (s *constraintSet) setMinItems(n int)      { keepLarger(&s.minItems, n) }
func (s *constraintSet) setMaxItems(n int)      { keepSmaller(&s.maxItems, n) }
func (s *constraintSet) setMinProperties(n int) { keepLarger(&s.minProperties, n) }
func (s *constraintSet) setMaxProperties(n int) { keepSmaller(&s.maxProperties, n) }

// mergeNumeric folds a candidate into the running winner. lower selects the
// larger value; !lower the smaller. On EQUAL value, exclusive beats inclusive
// (strictly more restrictive) — exclusivity is OR-ed, value kept.
func mergeNumeric(dst **numericBound, cand numericBound, lower bool) {
	if *dst == nil {
		c := cand
		*dst = &c
		return
	}
	ord := compareNumericBounds(cand, **dst)
	switch {
	case ord == 0:
		(*dst).exclusive = (*dst).exclusive || cand.exclusive
	case (lower && ord > 0) || (!lower && ord < 0):
		c := cand
		*dst = &c
	}
}

func (s *constraintSet) setMinimum(b numericBound) { mergeNumeric(&s.minimum, b, true) }
func (s *constraintSet) setMaximum(b numericBound) { mergeNumeric(&s.maximum, b, false) }

// applyTo writes every set keyword onto prop and leaves unset ones alone — so
// a pre-stamped value (the uint minimum: 0) survives an empty set and is
// overwritten only by an explicit bound, exactly as the applicator loop did.
func (s constraintSet) applyTo(prop *OpenAPIProperty) {
	if s.format != "" {
		prop.Format = s.format
	}
	if s.pattern != "" {
		prop.Pattern = s.pattern
	}
	if s.enum != nil {
		prop.Enum = s.enum
	}
	copyInt(&prop.MinLength, s.minLength)
	copyInt(&prop.MaxLength, s.maxLength)
	copyInt(&prop.MinItems, s.minItems)
	copyInt(&prop.MaxItems, s.maxItems)
	copyInt(&prop.MinProperties, s.minProperties)
	copyInt(&prop.MaxProperties, s.maxProperties)
	if s.minimum != nil {
		prop.Minimum = floatPtr(s.minimum.float64Value())
		if s.minimum.exclusive {
			prop.ExclusiveMinimum = boolPtr(true)
		}
	}
	if s.maximum != nil {
		prop.Maximum = floatPtr(s.maximum.float64Value())
		if s.maximum.exclusive {
			prop.ExclusiveMaximum = boolPtr(true)
		}
	}
}

func copyInt(dst **int, src *int) {
	if src != nil {
		v := *src
		*dst = &v
	}
}

func boolPtr(b bool) *bool { return &b }
```

Equivalence notes to check against the old code while implementing: (1) old `applyMinimumConstraint` used `toFloat64Ptr(value)` on the winner's typed value — `float64(int64)` for ints, identity for floats; `float64Value()` is the same conversion. (2) Old `materializeBounds` emitted `exclusiveMinimum: true` only when the winner was exclusive — same as the guarded `boolPtr(true)` here, and the applicator set `ExclusiveMinimum = &val` (true); an inclusive winner never wrote `false`, so neither does `applyTo`. (3) `format == ""` is never produced by a handler (formatTagMap/datetimeFormat always non-empty), so the empty-string guard cannot suppress a real value — verify with grep on `setFormat` callers after Task 4. (4) `enum` from `handleEnumConstraint` is non-nil (guarded `len(tokens) == 0 → nil` means no set call); `handleEqConstraint` always produces a one-element slice. (5) Old `compareBoundValues` treated `int` (length keywords) and `int64` alike via `boundInt64` — length keywords never reached `mergeBound` with mixed types because each keyword has one value type; the typed set separates them structurally, so no case is lost.

- [ ] **Step 4: Run** `go test ./internal/generator -run 'TestConstraintSet' -v` → PASS. The rest of the package won't compile yet (handlers still return pair lists) — that's Task 4; do not commit between 3 and 4 unless the build is green. If you want a commit boundary, temporarily keep the old block alongside the new (both compile) and delete it in Task 4.

### Task 4: Handlers write into the set; `constraintsFor` replaces `mapConstraintToOpenAPI`

**Files:**
- Modify: `internal/generator/constraints.go` (handlers ≈322–600 in the 2a tree; `dispatchScalarConstraint` ≈302; `mapConstraintToOpenAPI` ≈67)

**Interfaces:**
- Produces: `func constraintsFor(shape models.TypeShape, underlyingKind string, constraints map[string]string) constraintSet`. Deletes `mapConstraintToOpenAPI` and `openAPIConstraint`.
- Handler shape: `func(s *constraintSet, key, value, effKind string) bool` — returns true iff the key was recognized AND a value was written (an unparsable numeric returns false and, because no other handler recognizes that key, the rule drops silently — identical to today's `nil` return). Dispatch order unchanged.

- [ ] **Step 1: Convert `mapConstraintToOpenAPI` → `constraintsFor`.** Keep the classification head (pointer unwrap, `byteSlice`, `isSlice`, `isMap`, `effKind`) verbatim; replace the accumulation:

```go
func constraintsFor(shape models.TypeShape, underlyingKind string, constraints map[string]string) constraintSet {
	var set constraintSet
	// ... classification head unchanged ...
	for _, key := range sortedKeys(constraints) {
		if key == constraintRequired {
			continue // handled at schema level
		}
		value := constraints[key]
		switch {
		case isSlice:
			applySliceCardinality(&set, key, value)
		case isMap:
			applyMapCardinality(&set, key, value)
		default:
			applyScalarConstraint(&set, key, value, effKind)
		}
	}
	return set
}
```

The sorted-key comment block stays (it now explains why "last sorted key wins" is deterministic).

- [ ] **Step 2: Convert each handler mechanically.** Pattern (shown for `min`; do the same for every handler in the dispatch list, preserving each one's key checks, parse calls, and drop conditions exactly):

```go
// applyMinConstraint maps 'min' to minLength (strings) or minimum (numbers).
func applyMinConstraint(s *constraintSet, key, value, effKind string) bool {
	if key != "min" {
		return false
	}
	if isEffectiveString(effKind) {
		if length, err := strconv.Atoi(value); err == nil {
			s.setMinLength(length)
			return true
		}
	} else if isEffectiveNumeric(effKind) {
		//nolint:S8148 // NOSONAR: invalid validation tag values are silently skipped
		if minVal, err := parseNumeric(value); err == nil {
			s.setMinimum(boundFromParsed(minVal, false))
			return true
		}
	}
	return false
}
```

Mapping of the old emissions:

| old handler emits | new call |
|---|---|
| `{format, X}` | `s.setFormat(X)` |
| `{pattern, X}` | `s.setPattern(X)` |
| `{minLength, n}` / `{maxLength, n}` | `s.setMinLength(n)` / `s.setMaxLength(n)` (`len` calls both) |
| `{minItems, n}` / `{maxItems, n}` | `s.setMinItems(n)` / `s.setMaxItems(n)` |
| `{minProperties, n}` / `{maxProperties, n}` | `s.setMinProperties(n)` / `s.setMaxProperties(n)` |
| `{minimum, v}` alone (min, gte) | `s.setMinimum(boundFromParsed(v, false))` |
| `{minimum, v}, {exclusiveMinimum, true}` (gt) | `s.setMinimum(boundFromParsed(v, true))` |
| `{maximum, v}` alone (max, lte) | `s.setMaximum(boundFromParsed(v, false))` |
| `{maximum, v}, {exclusiveMaximum, true}` (lt) | `s.setMaximum(boundFromParsed(v, true))` |
| `{enum, []any}` | `s.setEnum(vals)` |

`applyScalarConstraint` replaces `dispatchScalarConstraint`: same ordered handler list, `for _, h := range handlers { if h(s, key, value, effKind) { return } }`. Keep `handleStringLengthComparison`'s clamp logic; it now calls `s.setMinLength`/`s.setMaxLength`. Keep `parseNumeric` (still used by `coerceEnum` and via `boundFromParsed`), `tokenizeOneOf`, `coerceEnum`, `datetimeFormat`, `formatTagMap`, `stringContentPatterns`, `effectiveKind`, `isEffectiveString`, `isEffectiveNumeric`, `sortedKeys`.

- [ ] **Step 3: Delete the applicator layer in `openapi.go`:** `constraintApplicators` map, `applyConstraint`, the 13 `apply*Constraint` functions, and `toFloat64Ptr` (grep for other callers first — `toFloat64Ptr` may be used by `coerceExample`; if so, keep it). Temporarily make `applyConstraints` and `applyElementConstraints` compile by calling `constraintsFor(...).applyTo(prop)` / `.applyTo(prop.Items)` — Task 6 replaces both with the single entry point.

- [ ] **Step 4: Build + goldens** — `go build ./... && go test ./internal/spectest` PASS. The package's own tests still fail to compile (table expects `[]openAPIConstraint`) — Task 5.

### Task 5: Convert the mapper's test corpus to property-out assertions

**Files:**
- Modify: `internal/generator/constraints_test.go`

**Interfaces:**
- The corpus tests the module's real surface: `constraintsFor(...).applyTo(&got)`, compared to a `want OpenAPIProperty`. This is the "interface is the test surface" conversion; do it now because the pair-list return type no longer exists.

- [ ] **Step 1: Convert the table.** Field `expected []openAPIConstraint` → `want OpenAPIProperty`. Row conversion rules (mechanical; ~70 rows):
  - `{Name: "format", Value: "email"}` → `Format: "email"`
  - `{Name: "pattern", Value: p}` → `Pattern: p`
  - `{Name: "minLength", Value: 5}` → `MinLength: intPtr(5)` (same for max/Items/Properties)
  - `{Name: "minimum", Value: int64(18)}` → `Minimum: floatPtr(18)`; `Value: 0.5` → `Minimum: floatPtr(0.5)`
  - `{Name: "exclusiveMinimum", Value: true}` → `ExclusiveMinimum: boolPtr(true)` (and `Maximum` twins)
  - `{Name: "enum", Value: []any{...}}` → `Enum: []any{...}` (keep the `int64(1)` typed elements — YAML emits `1` for int64 and `1.0`-style for float64, so element types are part of behavior)
  - An empty `expected` (dropped constraint) → `want: OpenAPIProperty{}`.
  - Rows whose old expectation encoded duplicate non-bound keywords (e.g. two `format` entries) — the old assertion helpers used a name→value map that kept the LAST entry; the new `want` carries only that last value. Check each such row against the old helper (`assertConstraintsMatch` ≈632) before converting.
- Loop body: `var got OpenAPIProperty; constraintsFor(tt.shape, tt.underlyingKind, tt.constraints).applyTo(&got); if !reflect.DeepEqual(got, tt.want) { t.Errorf("%s: got %+v, want %+v", tt.description, got, tt.want) }`. Delete `assertConstraintsMatch`, `constraintByName`, `assertEnumConstraint`.
- `TestMapConstraintToOpenAPIDeterministicOverlap` (≈555) + `assertDeterministicConstraint` (≈599): keep the scenarios; assert on the property field the keyword maps to.
- Add the precedence pin (Q10):

```go
func TestConstraintsForRepeatedKeywordLastSortedKeyWins(t *testing.T) {
	var got OpenAPIProperty
	// sorted: email < uuid4 -> uuid wins; alpha < contains -> contains' pattern wins;
	// eq < oneof -> oneof's enum wins. Matches the retired applicators' last-writer-wins.
	constraintsFor(prim("string"), "", map[string]string{
		"email": "true", "uuid4": "true",
		"alpha": "true", "contains": "x",
		"eq": "a", "oneof": "b c",
	}).applyTo(&got)
	if got.Format != "uuid" || got.Pattern != "x" || !reflect.DeepEqual(got.Enum, []any{"b", "c"}) {
		t.Errorf("precedence drift: %+v", got)
	}
}
```

Derive the expected values by running the OLD code path once (check out `origin/main`'s generator in a scratch worktree, or reason from `formatTagMap`/`stringContentPatterns`: `uuid4 → "uuid"`, `contains=x → regexp.QuoteMeta("x") = "x"`, `oneof="b c" → ["b","c"]`). If the old behavior differs from this guess, the test must encode the OLD behavior — never "fix" precedence in this PR.

- [ ] **Step 2: Run** `go test ./internal/generator` → PASS; `go test ./internal/spectest` → PASS.

- [ ] **Step 3: Commit Tasks 3–5 together** (they only compile together):

```bash
git add internal/generator/
git commit -m "refactor(generator): typed constraint set replaces the pair-list applicators

constraintSet holds one field per OpenAPI keyword and resolves
most-restrictive bounds at set time, so the \x00bound placeholder,
positional exclusive-partner consumption, and the 13 type-asserting
applicators are gone. Precedence for repeated format/pattern/enum keys
(last sorted validator key wins) is unchanged and now pinned by a test.
The mapper's corpus asserts on the emitted property instead of an
intermediate pair list."
```

### Task 6: Single entry point owning the element path

**Files:**
- Modify: `internal/generator/openapi.go` — `applyConstraints` (≈1616), `applyElementConstraints` (≈1452), call sites in `buildFieldProperty` (≈1376–1377, ≈1384, ≈1394–1395) and `refProperty` (≈1422)
- Test: `internal/generator/openapi_test.go` (`TestApplyConstraints` ≈1630 calls `gen.applyConstraints`)

**Interfaces:**
- Produces: `func applyValidationConstraints(prop *OpenAPIProperty, field *models.FieldInfo)` — plain function (no receiver; the module holds no generator state). Deletes `applyConstraints` and `applyElementConstraints`.

- [ ] **Step 1: Write the failing tests** (in `openapi_test.go`, or a new `constraints_entry_test.go`):

```go
func TestApplyValidationConstraintsRefItemsTakeNothing(t *testing.T) {
	// Slice-of-$ref: collection cardinality lands on the array, element (dive)
	// rules have nowhere valid to go on a $ref and must be dropped — the rule
	// refProperty used to enforce by simply not calling the element path.
	arr := &OpenAPIProperty{Type: typeArray, Items: &OpenAPIProperty{Ref: "#/components/schemas/Address"}}
	field := &models.FieldInfo{
		Shape:              sliceOf(named("Address")),
		Constraints:        map[string]string{"min": "1"},
		ElementConstraints: map[string]string{"required": "true", "min": "2"},
	}
	applyValidationConstraints(arr, field)
	if arr.MinItems == nil || *arr.MinItems != 1 {
		t.Errorf("minItems not applied: %+v", arr)
	}
	if arr.Items.MinLength != nil || arr.Items.Minimum != nil || arr.Items.MinItems != nil {
		t.Errorf("element rules must not be stamped beside a $ref: %+v", arr.Items)
	}
}

func TestApplyValidationConstraintsElementPath(t *testing.T) {
	prop := &OpenAPIProperty{Type: typeArray, Items: &OpenAPIProperty{Type: typeString}}
	field := &models.FieldInfo{
		Shape:              sliceOf(prim("string")),
		Constraints:        map[string]string{"max": "3"},
		ElementConstraints: map[string]string{"email": "true"},
	}
	applyValidationConstraints(prop, field)
	if prop.MaxItems == nil || *prop.MaxItems != 3 || prop.Items.Format != "email" {
		t.Errorf("collection and element scopes both apply: %+v / %+v", prop, prop.Items)
	}
}

func TestApplyValidationConstraintsUintFloorOverwritten(t *testing.T) {
	// Ordering invariant: setTypeAndFormat pre-stamps minimum: 0 for uints and
	// relies on constraint application running afterwards to overwrite it.
	var gen OpenAPIGenerator
	prop := &OpenAPIProperty{}
	field := &models.FieldInfo{Shape: prim("uint"), Constraints: map[string]string{"min": "5"}}
	gen.setTypeAndFormat(prop, field.Shape)
	if prop.Minimum == nil || *prop.Minimum != 0 {
		t.Fatalf("precondition: uint pre-stamp missing: %+v", prop)
	}
	applyValidationConstraints(prop, field)
	if *prop.Minimum != 5 {
		t.Errorf("explicit min must overwrite the uint floor, got %v", *prop.Minimum)
	}
	// And with no constraint, the floor survives.
	bare := &OpenAPIProperty{}
	gen.setTypeAndFormat(bare, prim("uint"))
	applyValidationConstraints(bare, &models.FieldInfo{Shape: prim("uint")})
	if bare.Minimum == nil || *bare.Minimum != 0 {
		t.Errorf("empty constraints must leave the uint floor: %+v", bare)
	}
}
```

(Adapt `var gen OpenAPIGenerator` to however tests construct a generator today — check `TestApplyConstraints` ≈1620.)

- [ ] **Step 2: Run, verify failure** (undefined `applyValidationConstraints`).

- [ ] **Step 3: Implement**, replacing `applyConstraints` + `applyElementConstraints`:

```go
// applyValidationConstraints fills prop's constraint keywords from the field's
// validate tag: collection-scope rules onto prop, element-scope (post-dive)
// rules onto prop.Items. Must run after setTypeAndFormat — the uint minimum: 0
// pre-stamp relies on an explicit min/gte overwriting it here.
//
// Element rules apply only when prop is an array whose items are an inline
// schema. A $ref must stand alone (OpenAPI 3.0 ignores its siblings), so
// element rules on a slice-of-struct have nowhere valid to go and drop — the
// rule refProperty used to enforce by not calling the element path at all.
func applyValidationConstraints(prop *OpenAPIProperty, field *models.FieldInfo) {
	if len(field.Constraints) > 0 {
		constraintsFor(field.Shape, field.UnderlyingKind, field.Constraints).applyTo(prop)
	}
	if len(field.ElementConstraints) == 0 || prop.Items == nil || prop.Items.Ref != "" {
		return
	}
	// Element shape: unwrap ONE pointer then ONE slice layer ("*[]Address" -> "Address").
	elem := field.Shape
	if elem.Kind == models.ShapePointer && elem.Elem != nil {
		elem = *elem.Elem
	}
	if elem.Kind == models.ShapeSlice && elem.Elem != nil {
		elem = *elem.Elem
	}
	constraintsFor(elem, field.UnderlyingKind, field.ElementConstraints).applyTo(prop.Items)
}
```

Equivalence check per old call site (read each in the 2a tree before replacing):

| site | old calls | new | note |
|---|---|---|---|
| `buildFieldProperty` UnderlyingKind-slice branch ≈1376–1377 | `applyConstraints` + `applyElementConstraints` | one call | `Items` is an inline `{Type: kind}` — element path runs, same as before |
| `buildFieldProperty` path 5 ≈1384 | `applyConstraints` only | one call | `Items == nil` → element path no-ops (old code never called it; same result) |
| `buildFieldProperty` general ≈1394–1395 | both | one call | identical |
| `refProperty` slice branch ≈1422 | `applyConstraints` only | one call | `Items.Ref != ""` → element path skipped — this is the guard that preserves the deliberate drop; keep the explanatory comment at the call site pointing at the guard |

`len(field.Constraints) > 0` guard preserved so an empty map writes nothing (the old early return).

- [ ] **Step 4: Update `TestApplyConstraints`** (≈1620–1640) to call `applyValidationConstraints(prop, tt.field)`; expectations unchanged.

- [ ] **Step 5: Gates**

```bash
make fmt lint test
go test ./internal/spectest
go run github.com/uudashr/gocognit/cmd/gocognit@latest -over 15 internal/generator/constraints.go internal/generator/openapi.go
grep -n 'openAPIConstraint\|constraintApplicators\|boundPlaceholder\|resolveMostRestrictive\|applyElementConstraints\|func (g \*OpenAPIGenerator) applyConstraints' internal/ -r
```

Expected: all green, goldens untouched, gocognit silent, the grep finds nothing (all retired names gone).

- [ ] **Step 6: Commit**

```bash
git add internal/generator/
git commit -m "refactor(generator): one entry point applies validate constraints

applyValidationConstraints owns both scopes: collection rules onto the
property, element (dive) rules onto items — and encodes the rule that a
\$ref item takes no keywords, which refProperty previously enforced by
not calling the element path. Callers stop orchestrating dive. The
uint minimum:0 pre-stamp ordering is preserved and pinned by a test
(folding the floor into the module is #54)."
```

### Task 7: PR 2b

- [ ] Push `refactor/constraint-set`, open the PR (`GH_TOKEN` prefix, one gh command per invocation). Body:

```markdown
## What
The validate-tag pipeline carried its result as an ordered `(name, any)` pair list, which needed a placeholder-and-partner pass to resolve most-restrictive bounds and 13 type-asserting applicators to write it. A typed `constraintSet` now resolves bounds at set time and writes the property directly; one entry point owns both collection and element (`dive`) scope, including the rule that a `$ref` item takes no keywords.

## Impact
Generator-internal only. Precedence for repeated `format`/`pattern`/`enum` validator keys (last sorted key wins) and the uint `minimum: 0` overwrite ordering are unchanged and now test-pinned.

## Verification
Golden fixtures byte-identical (no `-update`). Mapper corpus re-asserted on the emitted property. gocognit ≤15 on touched files.
```

- [ ] Wait for CodeRabbit's auto-review; expect one round of nits. All 8 required checks + CodeQL + SonarCloud must be green; author cannot self-approve — CodeRabbit's APPROVED (or the maintainer's `--admin`) is the merge path.

---

## Self-review notes (planner)

- Q5 → 2a (move, unexport, seam corrected). Q10 → Task 3–4 (typed set, precedence pinned first). Q11 → 2a moves tests mechanically; the corpus conversion to property-out happens in 2b only because the return type ceases to exist — kept mechanical via the row-conversion table. Q12 → Task 6 (single entry). Q13 → Task 6 test `UintFloorOverwritten`; fold deferred to #54.
- Discovered constraint not in the grilling: analyzer's `isBuiltinShapeName` depends on the classifiers — resolved by keeping them analyzer-side (`builtins.go`) and giving the module verbatim private copies; documented as accepted one-list-per-package.
- Discovered trap not in the grilling: `refProperty` never applied element constraints — encoded as the `Items.Ref != ""` guard with a dedicated test, so the deepening doesn't silently stamp keywords beside a `$ref`.
- Type consistency: `constraintSet`, `numericBound`, `boundFromParsed`, `constraintsFor`, `applyValidationConstraints`, `keepLarger/keepSmaller`, `mergeNumeric`, `copyInt`, `boolPtr`, `intPtr`, `floatPtr` used consistently across Tasks 3–6.
- Judgment calls delegated with read-first duty: constant reconciliation (Task 2 Step 3), rows with duplicate non-bound keywords (Task 5 Step 1), `toFloat64Ptr` other callers (Task 4 Step 3), precedence expected values derived from old behavior (Task 5).
