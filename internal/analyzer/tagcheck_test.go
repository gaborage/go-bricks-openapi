package analyzer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tagWarnings filters a.Warnings() down to the malformed-struct-tag
// diagnostics, so a fixture's other diagnostics (if any) don't interfere with
// an exact count assertion.
func tagWarnings(warnings []string) []string {
	var out []string
	for _, w := range warnings {
		if strings.Contains(w, "is not readable by reflect.StructTag") {
			out = append(out, w)
		}
	}
	return out
}

// TestHiddenTagKeysMustNotWarn is Table 1: every row is vet-clean (reflect
// consumes it fully), so hiddenTagKeys must report nothing. This is the
// regression guard for the rewrite: the eight rows explicitly called out below
// are the two false-positive families ("value ends in <key>:" and a
// punctuation byte in a key name) that sank the previous, substring-based
// design.
func TestHiddenTagKeysMustNotWarn(t *testing.T) {
	cases := []struct {
		name string
		tag  string
	}{
		{"decoy doc key (godoc ends in doc)", `json:"a" godoc:"note"`},
		{"decoy param key (queryparam ends in param)", `queryparam:"x" json:"a"`},
		{"decoy json key (myjson ends in json)", `myjson:"a" validate:"required"`},
		{"decoy header key (xheader ends in header)", `xheader:"H" json:"a"`},
		{"decoy validate key (govalidate ends in validate)", `govalidate:"r" json:"a"`},
		{"decoy query key (xquery ends in query)", `xquery:"p" json:"a"`},
		{"decoy example key (doexample ends in example)", `doexample:"e" json:"a"`},
		{"decoy query key (notquery ends in query)", `notquery:"decoy" query:"page"`},
		{"decoy example key, real example follows, well-formed", `myexample:"decoy" example:"real"`},
		{"present-but-empty param", `param:"" json:"x"`},
		{"present-but-empty header", `header:"" json:"y"`},
		{"duplicate json key, well-formed", `json:"a" json:"b"`},
		{"value ends in 'example:' (family 1)", `json:"a" doc:"see example:"`},
		{"value ends in 'json:' (family 1)", `doc:"prefix json:"`},
		{"value IS 'json:' (family 1)", `example:"json:"`},
		{"value ends in 'example:' via gorm decoy (family 1)", `gorm:"column:example:" json:"a"`},
		{"punctuation key byte '/' (family 2)", `x/json:"a"`},
		{"punctuation key byte '$' (family 2)", `$header:"H" json:"b"`},
		{"punctuation key byte '@' (family 2)", `@doc:"d" json:"b"`},
		{"punctuation key byte '*' (family 2)", `*example:"e" json:"b"`},
		{"empty tag", ``},
		{"whitespace-only tag", `   `},
		{"real fixture tag from struct_tags/api.go", `json:"note" doc:"say \"hi\" now" myexample:"decoy" example:"a note"`},
		{
			"malformed, but no scanned key is hidden — 'query:' is text inside myfoo's value, not a key",
			"json:\"a\"\nmyfoo:\"see query:\"",
		},
	}
	require.Len(t, cases, 24, "Table 1 must carry all 24 MUST-NOT-WARN rows")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hiddenTagKeys(tc.tag)
			assert.Empty(t, got, "hiddenTagKeys(%q) = %v, want none", tc.tag, got)
		})
	}
}

// TestHiddenTagKeysMustWarn is Table 2: reflect's scan stops before the end of
// the tag, so the listed keys are genuinely unreadable. The last row exists
// specifically to make the keyStartsIn loop load-bearing: its remainder
// contains "example:\"" twice — once inside the myexample decoy (correctly
// skipped, preceded by a key byte) and once for real. A single strings.Index
// would stop at the decoy and report nothing.
func TestHiddenTagKeysMustWarn(t *testing.T) {
	cases := []struct {
		name string
		tag  string
		want []string
	}{
		{"multi-line tag breaks after json", "json:\"name\"\n\tvalidate:\"required\"", []string{"validate"}},
		{"tab-separated pairs breaks after json", "json:\"a\"\tquery:\"q\"", []string{"query"}},
		{"multi-line tag hides three keys", "json:\"n\"\ndoc:\"d\" example:\"e\" validate:\"v\"", []string{"doc", "example", "validate"}},
		{"comma directly after value breaks scan", `json:"id", param:"id"`, []string{"param"}},
		{"unterminated value", `json:"e" validate:"f`, []string{"validate"}},
		{"decoy then real example — loop must not stop at decoy", "json:\"a\"\nmyexample:\"d\" example:\"e\"", []string{"example"}},
		{
			"a scanned key appearing inside a hidden VALUE must not be reported",
			"json:\"a\"\ndoc:\"see query:\"", []string{"doc"},
		},
	}
	require.Len(t, cases, 7, "Table 2 must carry all 7 MUST-WARN rows")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hiddenTagKeys(tc.tag)
			assert.Equal(t, tc.want, got, "hiddenTagKeys(%q)", tc.tag)
		})
	}
}

// TestHiddenTagKeysKnownNotCaught is Table 3: a deliberate, documented limit.
// In each case reflect's scan reads a mangled key literally (",example",
// ";validate", ",json") and the tag is otherwise well-formed by reflect's own
// grammar, so tagScanEnd reports the tag fully consumed and hiddenTagKeys never
// even compares individual keys. Catching these needs a "which punctuation
// would a human plausibly put in a key name" heuristic — exactly what produced
// the previous design's false positives.
func TestHiddenTagKeysKnownNotCaught(t *testing.T) {
	cases := []struct {
		name string
		tag  string
	}{
		{"reflect reads a key named ',example'", `myexample:"decoy",example:"real"`},
		{"reflect reads a key named ';validate'", `json:"b";validate:"c"`},
		{"reflect reads a key named ',json'", `,json:"a" validate:"b"`},
	}
	require.Len(t, cases, 3, "Table 3 must carry all 3 known-not-caught rows")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hiddenTagKeys(tc.tag)
			assert.Empty(t, got, "hiddenTagKeys(%q) = %v, want none (known limit)", tc.tag, got)
		})
	}
}

// TestTagValueEnd directly tests tagValueEnd, because hiddenTagKeys/keyStartsIn
// no longer let every tagValueEnd bug reach an observable difference: an
// off-by-one in its return offset leaves a single stray, non-key byte at the
// front of the next remainder, and tagTokenAt's leading skip-loop silently
// swallows it. The i+1-vs-i row and the escaped-quote row are exactly the two
// behaviours that were proven end-to-end-unmutatable; testing them here closes
// that gap directly.
func TestTagValueEnd(t *testing.T) {
	cases := []struct {
		name string
		tag  string
		q    int
		want int
	}{
		{"simple value, pins the i+1 return", `doc:"ab"`, 4, 8},
		{"escaped quote in value, pins the \\ escape step", `doc:"a\"b"`, 4, 10},
		{"unterminated value", `doc:"unterminated`, 4, -1},
		{"empty value", `doc:""`, 4, 6},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tagValueEnd(tc.tag, tc.q)
			assert.Equal(t, tc.want, got, "tagValueEnd(%q, %d)", tc.tag, tc.q)
		})
	}
}

// TestTagScanEnd asserts properties of tagScanEnd rather than specific offsets,
// since only the boundary conditions (fully consumed vs. not, and the empty
// string) are part of the contract other functions rely on.
func TestTagScanEnd(t *testing.T) {
	t.Run("well-formed tag is fully consumed", func(t *testing.T) {
		tag := `json:"a" doc:"b"`
		got := tagScanEnd(tag)
		assert.Equal(t, len(tag), got, "tagScanEnd(%q)", tag)
	})
	t.Run("multi-line tag stops before the end", func(t *testing.T) {
		tag := "json:\"a\"\ndoc:\"b\""
		got := tagScanEnd(tag)
		assert.Less(t, got, len(tag), "tagScanEnd(%q)", tag)
	})
	t.Run("unterminated value stops before the end", func(t *testing.T) {
		tag := `json:"a`
		got := tagScanEnd(tag)
		assert.Less(t, got, len(tag), "tagScanEnd(%q)", tag)
	})
	t.Run("empty tag", func(t *testing.T) {
		got := tagScanEnd("")
		assert.Equal(t, 0, got, `tagScanEnd("")`)
	})
}

// TestUnreadableTagKeysWarningText is Table 4, fixture shape 1: a plain (non-
// embedded) request field with a malformed multi-line tag. Asserts the
// warning names the field, the hidden key, and go vet, and that exactly one
// warning is emitted.
func TestUnreadableTagKeysWarningText(t *testing.T) {
	dir := writeAnalyzerProject(t, "module.go", `package widgets

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string { return "widgets" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error { return nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/widgets", m.createWidget, server.WithTags("widgets"))
}

type CreateWidgetReq struct {
	Note string "json:\"note\"\n\tvalidate:\"required\""
}

type WidgetResp struct {
	ID string "json:\"id\""
}

func (m *Module) createWidget(req CreateWidgetReq, ctx server.HandlerContext) (WidgetResp, server.IAPIError) {
	return WidgetResp{}, nil
}
`)

	a := New(dir)
	if _, err := a.AnalyzeProject(); err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}

	warnings := tagWarnings(a.Warnings(t.Context()))
	require.Len(t, warnings, 1, "expected exactly one malformed-tag warning, got: %v", a.Warnings(t.Context()))
	assert.Contains(t, warnings[0], "Note")
	assert.Contains(t, warnings[0], "validate")
	assert.Contains(t, warnings[0], "go vet")
}

// TestUnreadableTagKeysDedupeAcrossCallSites is Table 4, fixture shape 2: an
// embedded field with an explicit (readable) json name and a malformed
// continuation. embeddedFields (call site 3b) sees the tag first, then falls
// through to buildFieldInfo -> parseFieldTags (call site 3a) on the SAME
// *ast.BasicLit. Without the file:line:col dedupe this would warn twice.
func TestUnreadableTagKeysDedupeAcrossCallSites(t *testing.T) {
	dir := writeAnalyzerProject(t, "module.go", `package gadgets

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string { return "gadgets" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error { return nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/gadgets", m.createGadget, server.WithTags("gadgets"))
}

type CreateGadgetReq struct {
	Base "json:\"base\"\n\tvalidate:\"required\""
}

type GadgetResp struct {
	ID string "json:\"id\""
}

func (m *Module) createGadget(req CreateGadgetReq, ctx server.HandlerContext) (GadgetResp, server.IAPIError) {
	return GadgetResp{}, nil
}
`)

	a := New(dir)
	if _, err := a.AnalyzeProject(); err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}

	warnings := tagWarnings(a.Warnings(t.Context()))
	require.Len(t, warnings, 1,
		"explicit json name + malformed continuation must warn once, not twice (embeddedFields and parseFieldTags share the same *ast.BasicLit); got: %v",
		a.Warnings(t.Context()))
}

// TestUnreadableTagKeysEmbeddedPromotionPath is Table 4, fixture shape 3: an
// embedded field whose malformed tag hides "json" itself, so embeddedFields
// falls through to promotion. Only call site 3b (inside embeddedFields) ever
// sees this *ast.BasicLit — buildFieldInfo/parseFieldTags is invoked on the
// EMBEDDED struct's own fields, not on the anonymous field's tag. Without the
// 3b call, this warns zero times.
func TestUnreadableTagKeysEmbeddedPromotionPath(t *testing.T) {
	dir := writeAnalyzerProject(t, "module.go", `package thingamajigs

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string { return "thingamajigs" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error { return nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/thingamajigs", m.createThing, server.WithTags("thingamajigs"))
}

type Base struct {
	ID string "json:\"id\""
}

type CreateThingReq struct {
	Base "\n\tjson:\"real\""
}

type ThingResp struct {
	ID string "json:\"id\""
}

func (m *Module) createThing(req CreateThingReq, ctx server.HandlerContext) (ThingResp, server.IAPIError) {
	return ThingResp{}, nil
}
`)

	a := New(dir)
	if _, err := a.AnalyzeProject(); err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}

	warnings := tagWarnings(a.Warnings(t.Context()))
	require.Len(t, warnings, 1,
		"malformed tag hiding json itself on an embedded field must warn via embeddedFields (call site 3b); got: %v",
		a.Warnings(t.Context()))
	assert.Contains(t, warnings[0], "Base")
	assert.Contains(t, warnings[0], "json")
}

// TestUnreadableTagKeysEmbeddedGenericName pins the warning text for an
// embedded GENERIC type (Base[T], an *ast.IndexExpr — valid Go since 1.18).
// The decoder does not model that node, so its shape is ShapeUnknown and
// shapeBaseName yields ""; embeddedFields substitutes "unknown", which is the
// name the retired typeToString/baseStructTypeName pipeline produced. The
// embed itself is unresolvable either way and contributes no fields; only the
// warning text depends on the name, and it must not move.
func TestUnreadableTagKeysEmbeddedGenericName(t *testing.T) {
	dir := writeAnalyzerProject(t, "module.go", `package gizmos

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string { return "gizmos" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error { return nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/gizmos", m.createGizmo, server.WithTags("gizmos"))
}

type Base[T any] struct {
	Value T
}

type CreateGizmoReq struct {
	Base[string] "json:\"base\"\n\tvalidate:\"required\""
}

type GizmoResp struct {
	ID string "json:\"id\""
}

func (m *Module) createGizmo(req CreateGizmoReq, ctx server.HandlerContext) (GizmoResp, server.IAPIError) {
	return GizmoResp{}, nil
}
`)

	a := New(dir)
	if _, err := a.AnalyzeProject(); err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}

	warnings := tagWarnings(a.Warnings(t.Context()))
	require.Len(t, warnings, 1, "expected exactly one malformed-tag warning, got: %v", a.Warnings(t.Context()))
	assert.Contains(t, warnings[0], "struct tag on field unknown at",
		"an unmodeled embed must still be named \"unknown\" in the warning")
	assert.Contains(t, warnings[0], "validate")
}
