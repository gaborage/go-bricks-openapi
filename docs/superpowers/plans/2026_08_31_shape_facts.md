> **SUPERSEDED (2026-09-01).** Executed and landed via PR #55 (`refactor/field-shape-facts`); the merged code, its commit bodies, and the golden suite are the authoritative record. Retained unchanged below as the executed plan.

# Field Shape Facts Implementation Plan (PR #1 of 2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the rendered type string `models.FieldInfo.Type` with a structured `Shape` value decoded once from the AST, deleting all 15 downstream string re-parse sites and both "keep in sync" twin helper pairs.

**Architecture:** The analyzer decodes each field's `ast.Expr` into a recursive `models.TypeShape` at extraction (the single existing stamp site, `buildFieldInfo`). Every downstream consumer — analyzer resolution, constraint mapping, generator property building — reads shape facts instead of prefix-stripping a string. `FieldInfo.Type` is deleted at the end; nothing displays it today (verified: zero display sites).

**Tech Stack:** Go 1.25 floor, `go/ast` only (static analysis — never compile/run the target project).

**Spec:** Grilling session record — summarized in the Global Constraints and task rationales below; domain terms in `/CONTEXT.md` (Shape vs Resolution). This plan file is the operative spec.

## Global Constraints

- **Bit-for-bit behavior preservation.** Golden fixtures must pass UNTOUCHED: `go test ./internal/spectest` with NO `-update` at any point in this PR. A changed golden means a bug in the refactor, never a golden refresh.
- Warning text, `--strict` semantics, silent constraint drops: all unchanged. No new warnings.
- `internal/models` stays struct-only, method-free, test-free. `TypeShape` is pure data. NEVER add a `_test.go` under `internal/models` — `TEST_PACKAGES` filters `/models` out, so such a test runs only on the Windows CI leg (repo trap, see CLAUDE.md).
- Settled invariants untouched: `lookupStructTag`, `unquoteLiteral`, `hiddenTagKeys`, `referencedSchemaNames` scan scope, the `schema == nil` check, example coercion only in `fieldInfoToProperty`.
- Per-site unwrap fidelity: the old code strips ONE leading `*` at some sites (`TrimPrefix`) and loops at others (`baseStructTypeName`). Each migrated site must preserve ITS depth discipline — noted per task.
- Behavior keys on `TypeShape.Name`, not on the primitive/named `Kind` distinction. If `primitive` vs `named` classification is ever ambiguous, it must not matter to output — every consumer switches on `Name` or on container kinds only.
- Gates before PR: `make fmt lint test` (needs `make dev-deps` once), `go test ./internal/spectest`, cognitive complexity ≤15 on every new/edited function (`go run github.com/uudashr/gocognit/cmd/gocognit@latest <file>` — SonarCloud go:S3776 is server-side and blocking; `make lint` alone does NOT check it).
- Sonar new-code gate: ≥80% coverage on new lines; the decoder and shape helpers must be exercised by package tests, not only through goldens (spectest fixtures don't count as coverage for `internal/*`).
- Line numbers below are anchors from commit `baedd32`; verify with a grep before each edit.
- Squash-only repo. Conventional Commit subject; suggested PR title: `refactor(analyzer): decode field type shape once, retire type-string parsing` (`refactor` → "Changed" changelog section; not breaking — all packages internal).
- PR body: `## What` / `## Impact` / `## Verification` only, ≤150 words (user's global standard). Verification states: goldens unmoved, gocognit run locally. Personal repo — no `Refs:` line.

---

### Task 1: `TypeShape` data type in models

**Files:**
- Modify: `internal/models/models.go`

**Interfaces:**
- Produces: `models.ShapeKind` (string type; constants `ShapePointer, ShapeSlice, ShapeMap, ShapeNamed, ShapePrimitive, ShapeUnknown`), `models.TypeShape{Kind ShapeKind; Name string; Key, Elem *TypeShape}`, and field `FieldInfo.Shape TypeShape` (value type at top level — zero value is safe and renders as unknown; `Key`/`Elem` are pointers because the type is recursive).

- [ ] **Step 1: Add the type and field** (no test — models is deliberately test-free; compilation is the check)

```go
// ShapeKind classifies one level of a TypeShape. See CONTEXT.md: "Shape".
type ShapeKind string

const (
	ShapePointer   ShapeKind = "pointer"
	ShapeSlice     ShapeKind = "slice"
	ShapeMap       ShapeKind = "map"
	ShapeNamed     ShapeKind = "named"     // a declared or qualified type name (Address, time.Time)
	ShapePrimitive ShapeKind = "primitive" // a builtin (string, int64, byte, any, interface{})
	ShapeUnknown   ShapeKind = "unknown"   // an AST shape the decoder does not model (chan, func, generics)
)

// TypeShape is the syntactic container structure of a field's declared type,
// decoded once from the AST at extraction. Purely syntactic — it carries no
// registry knowledge (that is Resolution: RefName/MapValueRefName/UnderlyingKind).
// The zero value (Kind "") is treated everywhere as ShapeUnknown.
type TypeShape struct {
	Kind ShapeKind
	// Name is set for ShapeNamed and ShapePrimitive: the identifier as written,
	// qualified for selector types ("time.Time"), including "interface{}" and "any".
	Name string
	// Key is the map key shape (ShapeMap only).
	Key *TypeShape
	// Elem is the pointed-to / element / map-value shape (ShapePointer, ShapeSlice, ShapeMap).
	Elem *TypeShape
}
```

In `FieldInfo`, add directly below the `Name` field (Type is deleted in Task 7, not here):

```go
	// Shape is the field's decoded type structure. Stamped by the analyzer at
	// extraction; hand-built test Projects must stamp it too — the generator
	// has NO string-parsing fallback.
	Shape TypeShape
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add internal/models/models.go
git commit -m "refactor(models): add TypeShape structured field shape"
```

---

### Task 2: Shape decoder in the analyzer, pinned against `typeToString`

**Files:**
- Modify: `internal/analyzer/analyzer.go` (new func beside `typeToString`, ≈line 3390)
- Test: `internal/analyzer/analyzer_test.go`

**Interfaces:**
- Consumes: `models.TypeShape` from Task 1.
- Produces: `func (a *ProjectAnalyzer) typeShape(expr ast.Expr) models.TypeShape` — total (never panics), returns `ShapeUnknown` for unmodeled AST nodes, mirroring `typeToString`'s `"unknown"` fallback.

- [ ] **Step 1: Write the failing parity test**

The strongest pin: for every expression the old renderer understood, decode+re-render must reproduce `typeToString` byte-for-byte. The renderer lives IN THE TEST ONLY (production code never renders shapes).

```go
// renderShape re-renders a TypeShape the way typeToString renders the AST —
// test-only helper pinning decoder fidelity.
func renderShape(s models.TypeShape) string {
	switch s.Kind {
	case models.ShapePointer:
		return "*" + renderShapePtr(s.Elem)
	case models.ShapeSlice:
		return "[]" + renderShapePtr(s.Elem)
	case models.ShapeMap:
		return "map[" + renderShapePtr(s.Key) + "]" + renderShapePtr(s.Elem)
	case models.ShapeNamed, models.ShapePrimitive:
		return s.Name
	default:
		return "unknown"
	}
}

func renderShapePtr(s *models.TypeShape) string {
	if s == nil {
		return "unknown"
	}
	return renderShape(*s)
}

func TestTypeShapeParityWithTypeToString(t *testing.T) {
	exprs := []string{
		"string", "int", "int64", "uint8", "byte", "bool", "float64", "any",
		"Address", "time.Time", "json.RawMessage", "uuid.UUID",
		"*string", "*Address", "**Address", "*time.Time",
		"[]string", "[]byte", "[]uint8", "[]Address", "[][]int", "*[]Address",
		"map[string]int", "map[string][]Address", "map[string]map[string]int",
		"*map[string]int", "map[int]string",
		"interface{}", "*interface{}", "[]interface{}",
		"chan int", "func()", "struct{}", "[3]int",
	}
	a := New()
	for _, src := range exprs {
		expr, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatalf("parse %q: %v", src, err)
		}
		want := a.typeToString(expr)
		got := renderShape(a.typeShape(expr))
		if got != want {
			t.Errorf("shape parity for %q: got %q, want %q", src, got, want)
		}
	}
}
```

Notes: import `go/parser` if not already imported in the test file. `New()` — check the actual constructor signature in analyzer.go (≈line 100–160) and adapt; if it requires args, use whatever existing analyzer tests use. `[3]int` (fixed-size array) parses as `*ast.ArrayType` with non-nil `Len` — `typeToString` renders it `"[]int"` today; the decoder must match that (decode as `ShapeSlice`, same lossiness). `chan int`/`func()`/`struct{}` must both render `"unknown"`.

Also add a direct structural test (parity alone can't see Kind classification):

```go
func TestTypeShapeStructure(t *testing.T) {
	a := New()
	expr, _ := parser.ParseExpr("map[string][]Address")
	s := a.typeShape(expr)
	if s.Kind != models.ShapeMap || s.Key == nil || s.Key.Name != "string" ||
		s.Elem == nil || s.Elem.Kind != models.ShapeSlice ||
		s.Elem.Elem == nil || s.Elem.Elem.Kind != models.ShapeNamed || s.Elem.Elem.Name != "Address" {
		t.Errorf("unexpected shape for map[string][]Address: %+v", s)
	}
	if k := a.typeShape(mustParse(t, "int64")).Kind; k != models.ShapePrimitive {
		t.Errorf("int64 kind = %v, want primitive", k)
	}
	if k := a.typeShape(mustParse(t, "Cents")).Kind; k != models.ShapeNamed {
		t.Errorf("Cents kind = %v, want named", k)
	}
}

func mustParse(t *testing.T, src string) ast.Expr {
	t.Helper()
	expr, err := parser.ParseExpr(src)
	if err != nil {
		t.Fatal(err)
	}
	return expr
}
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./internal/analyzer -run 'TestTypeShape' -v`
Expected: FAIL — `a.typeShape undefined`.

- [ ] **Step 3: Implement the decoder**

Place directly below `typeToString` (≈3414). Mirror its case set exactly:

```go
// builtinShapeNames are the identifiers decoded as ShapePrimitive. The
// primitive-vs-named distinction is informational — no consumer's OUTPUT may
// depend on it (behavior keys on Name and on container kinds).
var builtinShapeNames = map[string]bool{
	"string": true, "bool": true, "byte": true, "rune": true, "error": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"uintptr": true, "float32": true, "float64": true,
	"complex64": true, "complex128": true, "any": true,
}

// typeShape decodes an AST type expression into its structural Shape.
// Total: unmodeled nodes (chan, func, struct literals, generics) decode as
// ShapeUnknown, matching typeToString's "unknown" fallback.
func (a *ProjectAnalyzer) typeShape(expr ast.Expr) models.TypeShape {
	switch t := expr.(type) {
	case *ast.Ident:
		if builtinShapeNames[t.Name] {
			return models.TypeShape{Kind: models.ShapePrimitive, Name: t.Name}
		}
		return models.TypeShape{Kind: models.ShapeNamed, Name: t.Name}
	case *ast.StarExpr:
		elem := a.typeShape(t.X)
		return models.TypeShape{Kind: models.ShapePointer, Elem: &elem}
	case *ast.ArrayType:
		elem := a.typeShape(t.Elt)
		return models.TypeShape{Kind: models.ShapeSlice, Elem: &elem}
	case *ast.MapType:
		key := a.typeShape(t.Key)
		elem := a.typeShape(t.Value)
		return models.TypeShape{Kind: models.ShapeMap, Key: &key, Elem: &elem}
	case *ast.SelectorExpr:
		if pkg, ok := t.X.(*ast.Ident); ok {
			return models.TypeShape{Kind: models.ShapeNamed, Name: pkg.Name + "." + t.Sel.Name}
		}
	case *ast.InterfaceType:
		return models.TypeShape{Kind: models.ShapePrimitive, Name: "interface{}"}
	}
	return models.TypeShape{Kind: models.ShapeUnknown}
}
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `go test ./internal/analyzer -run 'TestTypeShape' -v`
Expected: PASS, including the `chan`/`func`/`[3]int` parity rows.

- [ ] **Step 5: Stamp `Shape` in `buildFieldInfo`** (≈3352 — `Type` keeps being stamped until Task 7):

```go
	fieldInfo := models.FieldInfo{
		Name:        name,
		Type:        a.typeToString(field.Type),
		Shape:       a.typeShape(field.Type),
		Constraints: make(map[string]string),
	}
```

- [ ] **Step 6: Full analyzer tests + goldens**

Run: `go test ./internal/analyzer ./internal/spectest`
Expected: PASS, goldens untouched (`git status` clean apart from source edits).

- [ ] **Step 7: Commit**

```bash
git add internal/analyzer/analyzer.go internal/analyzer/analyzer_test.go
git commit -m "refactor(analyzer): decode and stamp structured field Shape at extraction"
```

---

### Task 3: Migrate analyzer Resolution sites to shape walks

**Files:**
- Modify: `internal/analyzer/analyzer.go` — `registerFieldRefAt` (≈2892–2908), `resolveUnderlyingKind` (≈2922), and the string helpers `baseStructTypeName` (≈3036), `mapValueType` (≈3059), `mapValueStructName` (≈3077)

**Interfaces:**
- Consumes: `FieldInfo.Shape` (Task 2).
- Produces: `func shapeBaseName(s models.TypeShape) string` and `func shapeMapValueBase(s models.TypeShape) (string, bool)` — package-private analyzer helpers. DELETES: `baseStructTypeName`, `mapValueType` (analyzer twin), `mapValueStructName` (verify no other callers first — grep each name; `resolveUnderlyingKind` currently calls `baseStructTypeName`, migrate it in the same edit).

- [ ] **Step 1: Write failing unit tests for the two new helpers** (model them on the existing `TestBaseTypeName` table ≈analyzer_test.go:3459 — port its cases to shapes so coverage carries over):

```go
func TestShapeBaseName(t *testing.T) {
	a := New()
	cases := map[string]string{
		"Address": "Address", "*Address": "Address", "[]Address": "Address",
		"*[]Address": "Address", "[][]Address": "Address", "**Address": "Address",
		"time.Time": "time.Time", "int64": "int64",
		"map[string]Address": "", // a map has no base struct name — mirrors old behavior
		"chan int":           "", // unknown shape -> ""
	}
	for src, want := range cases {
		if got := shapeBaseName(a.typeShape(mustParse(t, src))); got != want {
			t.Errorf("shapeBaseName(%s) = %q, want %q", src, got, want)
		}
	}
}
```

**CRITICAL — verify old behavior first:** before writing the expected values above, read `baseStructTypeName` (≈3036–3047). It loops stripping `*` and `[]` prefixes. Confirm what it returns for `"map[string]Address"` (the string starts with `map[`, no strip applies → returns `"map[string]Address"` verbatim, which then fails struct lookup and flows into `resolveUnderlyingKind`, where `isStringType`/`primitiveKind` won't match and `namedScalarKind("map[string]Address"...)` fails). The shape version returning `""` for maps/unknown is behavior-equal ONLY IF `registerTypeAt("")` and `namedScalarKind("")` fail the same way `registerTypeAt("map[...]")` does — read both (`registerTypeAt` ≈2627, `namedScalarKind` ≈2939) and confirm empty-string inputs are harmless no-ops (they fail lookup and return nil/""). If not, make `shapeBaseName` return a non-matchable sentinel instead. Record the finding in the commit body.

Same table style for `shapeMapValueBase` (mirrors `mapValueStructName`: map value's base struct name after unwrapping pointer/slice on the VALUE — read ≈3077–3095 for exact unwrap depth):

```go
func TestShapeMapValueBase(t *testing.T) {
	a := New()
	type result struct {
		name  string
		isMap bool
	}
	cases := map[string]result{
		"map[string]Address":   {"Address", true},
		"map[string][]Address": {"Address", true},
		"*map[string]Address":  {"Address", true},
		"[]Address":            {"", false},
		"string":               {"", false},
	}
	for src, want := range cases {
		name, isMap := shapeMapValueBase(a.typeShape(mustParse(t, src)))
		if name != want.name || isMap != want.isMap {
			t.Errorf("shapeMapValueBase(%s) = (%q,%v), want (%q,%v)", src, name, isMap, want.name, want.isMap)
		}
	}
}
```

**Verify each expected value against the OLD functions before implementing** — e.g. does `mapValueStructName` unwrap a pointer-to-map (`*map[string]Address`)? It operates on the string via the old `mapValueType`, which does `TrimPrefix "*"` first — so yes. Does it unwrap `map[string]*Address`? Read the code; add the case with whatever the old answer is.

- [ ] **Step 2: Run, verify failure** (`go test ./internal/analyzer -run 'TestShape(BaseName|MapValueBase)' -v` → undefined)

- [ ] **Step 3: Implement**

```go
// shapeBaseName unwraps pointer/slice layers (any depth, mirroring the old
// baseStructTypeName loop) and returns the terminal Name. "" for maps and
// unknown shapes — non-matchable in the registry, like the raw strings the
// old code passed through.
func shapeBaseName(s models.TypeShape) string {
	for {
		switch s.Kind {
		case models.ShapePointer, models.ShapeSlice:
			if s.Elem == nil {
				return ""
			}
			s = *s.Elem
		case models.ShapeNamed, models.ShapePrimitive:
			return s.Name
		default:
			return ""
		}
	}
}

// shapeMapValueBase reports whether s is a map (after any leading pointers —
// mirror the old mapValueType TrimPrefix; verify single vs multi unwrap) and
// returns the base name of its value shape.
func shapeMapValueBase(s models.TypeShape) (string, bool) {
	if s.Kind == models.ShapePointer && s.Elem != nil {
		s = *s.Elem // old code: strings.TrimPrefix(t, "*") — ONE level
	}
	if s.Kind != models.ShapeMap || s.Elem == nil {
		return "", false
	}
	return shapeBaseName(*s.Elem), true
}
```

Then rewire in the same edit:
- `registerFieldRefAt` ≈2896: `mapValueStructName(f.Type)` → `shapeMapValueBase(f.Shape)`
- `registerFieldRefAt` ≈2902: `baseStructTypeName(f.Type)` → `shapeBaseName(f.Shape)`
- `registerFieldRefAt` ≈2907: `a.resolveUnderlyingKind(f.Type, ...)` → change `resolveUnderlyingKind` to take the base name directly: `a.resolveUnderlyingKind(shapeBaseName(f.Shape), astFile, filePath)` and inside it delete the `base := baseStructTypeName(typeStr)` line (rename param `typeStr` → `base`). Keep the `strings.Contains(base, ".")` qualified-type check — shape names carry the dot exactly as before.
- Delete `baseStructTypeName`, analyzer's `mapValueType`, `mapValueStructName` and their twin-comment blocks IF `grep -n 'baseStructTypeName\|mapValueType\|mapValueStructName' internal/analyzer/*.go` shows no remaining callers. Delete their direct tests (`TestBaseTypeName` etc.) — the shape tests above replace them.

- [ ] **Step 4: Full test + goldens**

Run: `go test ./internal/analyzer ./internal/spectest`
Expected: PASS. If ANY spectest golden diff appears, stop — a fidelity bug, not a golden refresh.

- [ ] **Step 5: Commit**

```bash
git add internal/analyzer/
git commit -m "refactor(analyzer): resolve field refs from Shape, retire string base-name helpers"
```

---

### Task 4: `MapConstraintToOpenAPI` takes a shape

**Files:**
- Modify: `internal/analyzer/constraints.go` (head of `MapConstraintToOpenAPI` ≈57–70; delete `isSliceType`/`isMapType`/`isByteSlice` string forms ≈326–338)
- Modify: `internal/generator/openapi.go` call sites ≈1451 and ≈1611
- Test: `internal/analyzer/constraints_test.go`

**Interfaces:**
- Consumes: `models.TypeShape`.
- Produces: `func MapConstraintToOpenAPI(shape models.TypeShape, underlyingKind string, constraints map[string]string) []OpenAPIConstraint` (stays exported and analyzer-resident — PR #2 moves and unexports it).

- [ ] **Step 1: Convert the test table.** `constraints_test.go` has a `fieldType string` table field feeding 2 call sites (≈501, 572) across ~71 entries. Add tiny in-package shape builders at the top of the test file and convert `fieldType: "..."` mechanically:

```go
func prim(name string) models.TypeShape { return models.TypeShape{Kind: models.ShapePrimitive, Name: name} }
func named(name string) models.TypeShape { return models.TypeShape{Kind: models.ShapeNamed, Name: name} }
func ptrOf(s models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: models.ShapePointer, Elem: &s}
}
func sliceOf(s models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeSlice, Elem: &s}
}
func mapOf(k, v models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeMap, Key: &k, Elem: &v}
}
```

Conversion table: `"string"` → `prim("string")`, `"*int64"` → `ptrOf(prim("int64"))`, `"[]string"` → `sliceOf(prim("string"))`, `"[]byte"` → `sliceOf(prim("byte"))`, `"map[string]int"` → `mapOf(prim("string"), prim("int"))`, `"Cents"` → `named("Cents")`, `"time.Duration"` → `named("time.Duration")`. Rename the table field `fieldType` → `shape`. Change the 2 call sites to pass `tt.shape`. If `models` isn't imported in the test file, add it. Keep the `parseNumeric` direct test (≈742) untouched.

- [ ] **Step 2: Run, verify compile failure** (`go test ./internal/analyzer -run TestMapConstraintToOpenAPI` → signature mismatch)

- [ ] **Step 3: Convert the function head.** Replace ≈lines 57–70:

```go
func MapConstraintToOpenAPI(shape models.TypeShape, underlyingKind string, constraints map[string]string) []OpenAPIConstraint {
	var result []OpenAPIConstraint

	// Old code stripped ONE leading "*" (TrimPrefix) — mirror exactly.
	base := shape
	if base.Kind == models.ShapePointer && base.Elem != nil {
		base = *base.Elem
	}
	// []byte/[]uint8 are well-known base64 string types, not arrays — treat them as
	// scalars (the well-known mapper already types them string/binary), not slices.
	// Their min/max are byte counts, which do NOT equal the base64-encoded character
	// length, so we deliberately drop length bounds on them rather than emit a wrong
	// minLength/maxLength. Map types are handled by a parallel branch below: their
	// effectiveKind is "" (neither string nor numeric), so min/max/len route to
	// minProperties/maxProperties (entry-count cardinality) rather than being dropped.
	byteSlice := base.Kind == models.ShapeSlice && base.Elem != nil &&
		(base.Elem.Name == "byte" || base.Elem.Name == "uint8")
	isSlice := base.Kind == models.ShapeSlice && !byteSlice
	isMap := base.Kind == models.ShapeMap
	effKind := effectiveKind(base.Name, underlyingKind)
```

`models` import needed in constraints.go. Behavior-equivalence notes to verify while editing (read the old lines side by side): (1) old `isByteSlice` compared the ONE-star-stripped string to `"[]byte"`/`"[]uint8"` — a `[]pkg.uint8` selector renders `"pkg.uint8"` as Elem.Name and correctly does NOT match, same as the old string compare; (2) old `effectiveKind(baseType, ...)` received `"[]string"`/`"map[..."` for containers and matched nothing — new code passes `base.Name` which is `""` for containers, also matching nothing; both routes end at `""`. (3) `**T`: old TrimPrefix left `"*T"` → not slice/map, effectiveKind no match → scalar dispatch; new one-level unwrap leaves a pointer shape, `Name` is `""` → identical routing. Delete the now-unused string helpers `isSliceType`, `isMapType`, `isByteSlice` from constraints.go (grep first — `isSliceType` may have other constraints.go-internal callers; if so, convert or keep until Task 5 and note it).

- [ ] **Step 4: Fix the two generator call sites** (temporary — Task 5 rewrites these functions anyway; make the minimal edit that compiles):
  - ≈1611 `applyConstraints`: `analyzer.MapConstraintToOpenAPI(field.Type, field.UnderlyingKind, field.Constraints)` → `analyzer.MapConstraintToOpenAPI(field.Shape, field.UnderlyingKind, field.Constraints)`
  - ≈1450–1451 `applyElementConstraints`: replace `elemType := sliceElementType(field.Type)` + the call with:

```go
	// Element shape: unwrap ONE pointer then ONE slice layer — the exact
	// discipline of the old sliceElementType ("*[]Address" -> "Address").
	elem := field.Shape
	if elem.Kind == models.ShapePointer && elem.Elem != nil {
		elem = *elem.Elem
	}
	if elem.Kind == models.ShapeSlice && elem.Elem != nil {
		elem = *elem.Elem
	}
	for _, c := range analyzer.MapConstraintToOpenAPI(elem, field.UnderlyingKind, field.ElementConstraints) {
		g.applyConstraint(prop.Items, c)
	}
```

(The `prop.Items == nil` guard at the top of the function already short-circuits every non-slice path — same as before.)

- [ ] **Step 5: Run everything**

Run: `go test ./internal/analyzer ./internal/generator ./internal/spectest`
Expected: analyzer + spectest PASS. Generator tests may fail where hand-built literals still lack `Shape` — those specific fixes belong to Task 5; if failures are ONLY missing-Shape (nil-ish zero → unknown routing), note them and proceed to Task 5 in the same session. Goldens must pass (real pipeline stamps Shape since Task 2).

- [ ] **Step 6: Commit**

```bash
git add internal/analyzer/ internal/generator/openapi.go
git commit -m "refactor(analyzer): MapConstraintToOpenAPI classifies from TypeShape"
```

---

### Task 5: Migrate generator property building; delete generator string helpers

**Files:**
- Modify: `internal/generator/openapi.go` — `buildFieldProperty` (≈1339–1408), `refProperty` (≈1414), `isPointerField` (≈1477), `setTypeAndFormat` (≈1530–1598), `wellKnownFormats` usage (≈1502), delete `sliceElementType` (≈1458), `isSliceType` (≈1464), `mapValueType` (≈1516)
- Test: `internal/generator/openapi_test.go` (~64 `Type:` literals), `internal/commands/doctor_test.go` (3 literals ≈1213, 1219, 1245)

**Interfaces:**
- Consumes: `FieldInfo.Shape`, builders pattern from Task 4.
- Produces: `func (g *OpenAPIGenerator) setTypeAndFormat(prop *OpenAPIProperty, shape models.TypeShape)` and package-private `func shapeAfterPointer(s models.TypeShape) models.TypeShape` (ONE-level unwrap, the generator's uniform discipline — every old generator site used single `TrimPrefix`).

- [ ] **Step 1: Add shape builders to `openapi_test.go`** — same five helpers as Task 4's Step 1 (separate package, so duplicate them; they are 5 lines each, test-only). Then convert every `Type: "..."` literal in `openapi_test.go` and the 3 in `doctor_test.go` to `Shape: <builder>` using the same conversion table. Do this file-by-file with compile checks; do NOT change expected outputs anywhere.

- [ ] **Step 2: Run generator tests, verify the failures are now signature/field errors only** (`go test ./internal/generator ./internal/commands` — expect compile errors pointing at the production sites you're about to convert; that's the red step for this task).

- [ ] **Step 3: Convert production sites.** Add the helper:

```go
// shapeAfterPointer unwraps ONE pointer level — the generator's uniform
// discipline, mirroring the single strings.TrimPrefix(goType, "*") the old
// string helpers all used.
func shapeAfterPointer(s models.TypeShape) models.TypeShape {
	if s.Kind == models.ShapePointer && s.Elem != nil {
		return *s.Elem
	}
	return s
}
```

Site-by-site (verify each against the old line before replacing):

| Old (string) | New (shape) |
|---|---|
| ≈1356 `mapValueType(field.Type)` + `MapValueRefName` guard | `u := shapeAfterPointer(field.Shape); if u.Kind == models.ShapeMap && field.MapValueRefName != ""` — the map-of-slices check ≈1359 `isSliceType(valueType)` becomes `u.Elem != nil && shapeAfterPointer(*u.Elem).Kind == models.ShapeSlice`. **Careful:** old code called `isSliceType` on the VALUE string, which itself TrimPrefixes one `*` — hence the inner `shapeAfterPointer`. `map[string]*[]Address` behaves identically. |
| ≈1373 `isSliceType(field.Type)` (UnderlyingKind branch) | `shapeAfterPointer(field.Shape).Kind == models.ShapeSlice` |
| ≈1390 `g.setTypeAndFormat(prop, field.Type)` | `g.setTypeAndFormat(prop, field.Shape)` |
| ≈1402 nullable gate `mapValueType`/`isPointerField`/`isSliceType` | `u := shapeAfterPointer(field.Shape)` then `if isPointerField(field) && u.Kind != models.ShapeSlice && u.Kind != models.ShapeMap && prop.Type != ""` |
| ≈1416 `isSliceType(field.Type)` in `refProperty` | `shapeAfterPointer(field.Shape).Kind == models.ShapeSlice` |
| ≈1478 `isPointerField` | `return field.ParamType == "" && field.Shape.Kind == models.ShapePointer` |

- [ ] **Step 4: Rewrite `setTypeAndFormat` on shapes.** The old function: strip one `*`; well-known lookup on the FULL remaining string (so `[]byte` wins before the array branch); array branch; map branch; basic-type switch; default object. Shape version preserving that exact order:

```go
// setTypeAndFormat maps a field's Shape to OpenAPI type and format.
func (g *OpenAPIGenerator) setTypeAndFormat(prop *OpenAPIProperty, shape models.TypeShape) {
	s := shapeAfterPointer(shape)

	// Well-known types first: []byte must win over the generic []T array branch,
	// and time.Time/uuid.UUID over the qualified-type object fallback.
	if wk, ok := wellKnownShape(s); ok {
		prop.Type = wk.typ
		if wk.format != "" {
			prop.Format = wk.format
		}
		return
	}

	if s.Kind == models.ShapeSlice {
		prop.Type = typeArray
		prop.Items = &OpenAPIProperty{}
		if s.Elem != nil {
			g.setTypeAndFormat(prop.Items, *s.Elem)
		}
		return
	}

	if s.Kind == models.ShapeMap {
		prop.Type = typeObject
		prop.AdditionalProperties = &OpenAPIProperty{}
		if s.Elem != nil {
			g.setTypeAndFormat(prop.AdditionalProperties, *s.Elem)
		}
		return
	}

	switch s.Name {
	// ... keep the ENTIRE existing basic-type switch body verbatim,
	// switching on s.Name instead of goType (string/int/uint/float/bool
	// cases, the uint minimum-0 stamps, the any/interface{} untyped return,
	// default -> typeObject).
	}
}

// wellKnownShape resolves the well-known stdlib/library schemas by shape:
// []byte / []uint8 (base64 binary string), and the qualified names in
// wellKnownFormats (time.Time, time.Duration, uuid.UUID, json.RawMessage).
func wellKnownShape(s models.TypeShape) (wellKnownType, bool) {
	if s.Kind == models.ShapeSlice && s.Elem != nil &&
		(s.Elem.Name == "byte" || s.Elem.Name == "uint8") {
		return wellKnownType{typeString, formatBinary}, true
	}
	if s.Kind == models.ShapeNamed {
		wk, ok := wellKnownFormats[s.Name]
		return wk, ok
	}
	return wellKnownType{}, false
}
```

Verification notes for this step: (1) old nested-map/slice recursion passed value STRINGS with no fresh pointer strip inside the recursive call — but the recursion re-enters setTypeAndFormat which strips a `*` at the top, so `map[string]*int` DID unwrap the value pointer. New code recurses with the raw elem shape and `shapeAfterPointer` runs at the top of the recursive call — identical. (2) `time.Duration` sits in `wellKnownFormats` keyed `"time.Duration"` — a ShapeNamed Name matches verbatim, including the alias-blindness documented at ≈1498 (an aliased `t.Time` renders Name `"t.Time"` and misses, same as before). (3) The zero-value/unknown shape falls to the switch default → `typeObject`, exactly like the old `"unknown"` string. Adjust `wellKnownFormats` keys/constants only if a grep shows `goTypeByteSlice`/`goTypeUint8Slice` become unused — then remove those two entries and constants (the byte-slice path now lives in `wellKnownShape`).

- [ ] **Step 5: Delete `sliceElementType`, generator `isSliceType`, generator `mapValueType`** — grep each for remaining callers first; all should be gone after Steps 3–4. Delete their twin comments.

- [ ] **Step 6: Full suite + goldens**

Run: `go test ./... 2>&1 | tail -20` — but note `make test` filters `/models`; run the full form: `make test && go test ./internal/spectest`
Expected: ALL PASS, zero golden diffs.

- [ ] **Step 7: Commit**

```bash
git add internal/generator/ internal/commands/doctor_test.go
git commit -m "refactor(generator): build properties from TypeShape, delete string type parsing"
```

---

### Task 6: Delete `FieldInfo.Type`

**Files:**
- Modify: `internal/models/models.go` (delete the field), `internal/analyzer/analyzer.go` (`buildFieldInfo` stops stamping; `typeToString` fate)

- [ ] **Step 1: Delete the `Type` field from `FieldInfo`**, delete `Type: a.typeToString(field.Type),` from `buildFieldInfo`.

- [ ] **Step 2: Chase the compiler.** `go build ./... && go vet ./...` — fix every remaining reference. Expected stragglers: none in production (all migrated in Tasks 3–5); possibly a few test assertions on `.Type` in `analyzer_test.go` — convert them to `Shape` assertions using `renderShape` from Task 2 or direct struct checks, preserving what each test verifies.

- [ ] **Step 3: `typeToString` fate.** `grep -n 'typeToString' internal/ -r` — if `buildFieldInfo` was its only production caller, delete it AND the parity test converts: change `TestTypeShapeParityWithTypeToString` to a golden table (`src string -> want string` using the same corpus with hard-coded expected strings via `renderShape`) so the decoder stays pinned after the reference implementation is gone. If `typeToString` has OTHER production callers (it may render TypeInfo names elsewhere), keep it and leave the parity test as is.

- [ ] **Step 4: Full gates**

```bash
make fmt lint test
go test ./internal/spectest
go run github.com/uudashr/gocognit/cmd/gocognit@latest -over 15 internal/analyzer/analyzer.go internal/analyzer/constraints.go internal/generator/openapi.go
grep -rn '\.Type\b' internal/generator/ internal/commands/ --include='*.go' | grep -v '_test.go' | grep -v 'prop\.Type\|wk\.typ\|TypeInfo\|OpenAPI'
```

Expected: fmt/lint/test clean; goldens untouched; gocognit prints NOTHING over 15; the final grep audits that no production generator/commands code reads a FieldInfo `.Type` (manually review any hits — `prop.Type` and `TypeInfo` noise is expected).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(models)!: delete FieldInfo.Type — Shape is the only type carrier

The rendered type string had one writer, zero display sites, and fifteen
prefix-stripping readers across two packages, two of them documented
keep-in-sync twins. Shape (decoded once from the AST at extraction)
replaces all of them. No fallback string parsing exists: hand-built
test Projects must stamp Shape."
```

(Use `!` only if you judge the internal model change worth flagging; all packages are `internal/`, so it is NOT a SemVer-surface break per RELEASING.md — drop the `!` and this note in that case. Recommended: drop it.)

---

### Task 7: PR

- [ ] **Step 1: Push branch `refactor/field-shape-facts`** (gh identity: `GH_TOKEN=$(gh auth token -u gaborage)` prefix on every gh command — auto mode may block combined export forms; run each gh command separately).
- [ ] **Step 2: PR body** (global format, nothing else):

```markdown
## What
`FieldInfo.Type` was a lossy rendered string re-parsed by 15 sites across analyzer and generator, two pairs held together by "keep in sync" comments. The analyzer now decodes a structured `Shape` once from the AST; every consumer reads facts, the string field and both twin pairs are deleted.

## Impact
Hand-built `models.Project` values (generator/commands tests) must stamp `Shape`; there is no string-parsing fallback. Constraint mapping (`MapConstraintToOpenAPI`) now takes a `TypeShape` — moves generator-side in the follow-up PR.

## Verification
Golden fixtures byte-identical (no `-update` used). Decoder pinned by parity corpus against the retired renderer. gocognit ≤15 on all touched files (SonarCloud S3776 is server-side only).
```

- [ ] **Step 3: Confirm CI: all 8 required checks + CodeQL green; SonarCloud new-code coverage ≥80% (decoder/helpers are unit-covered by design, not just golden-covered).**

---

## Self-review notes (planner)

- Spec coverage: Q7 delete → Task 6; Q8 recursive shape → Tasks 1–2; Q3/Q9 analyzer-stamps + resolution untouched → Tasks 2–3 (RefName/MapValueRefName/UnderlyingKind stamping logic unchanged, only their INPUT derivation migrates); Q4 no-fallback → Task 5 Step 1 + Task 6 grep audit; constraint-signature change is PR #1 scope because constraints.go:60/68/69/326 are four of the fifteen parse sites.
- Type consistency: `ShapeKind` constants, `TypeShape` field set, builder names (`prim/named/ptrOf/sliceOf/mapOf`), `shapeAfterPointer`, `shapeBaseName`, `shapeMapValueBase` used consistently across tasks.
- Known judgment calls delegated to executor WITH verification duty: `shapeBaseName("")`-for-maps equivalence (Task 3 Step 1), `mapValueStructName` pointer-unwrap depth (Task 3 Step 1), `typeToString` residual callers (Task 6 Step 3), `goTypeByteSlice` constant removal (Task 5 Step 4). Each carries an explicit read-the-old-code instruction; none may be skipped.
