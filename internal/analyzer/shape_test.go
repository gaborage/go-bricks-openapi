package analyzer

import (
	"go/ast"
	"go/parser"
	"testing"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// renderShape re-renders a TypeShape the way typeToString renders the AST —
// test-only helper pinning decoder fidelity. Production code never renders shapes.
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

func mustParse(t *testing.T, src string) ast.Expr {
	t.Helper()
	expr, err := parser.ParseExpr(src)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	return expr
}

var shapeParityCorpus = []string{
	"string", "int", "int64", "uint8", "byte", "bool", "float64", "any",
	"Address", "time.Time", "json.RawMessage", "uuid.UUID",
	"*string", "*Address", "**Address", "*time.Time",
	"[]string", "[]byte", "[]uint8", "[]Address", "[][]int", "*[]Address",
	"map[string]int", "map[string][]Address", "map[string]map[string]int",
	"*map[string]int", "map[int]string",
	"interface{}", "*interface{}", "[]interface{}",
	"chan int", "func()", "struct{}", "[3]int",
}

func TestTypeShapeParityWithTypeToString(t *testing.T) {
	a := New("")
	for _, src := range shapeParityCorpus {
		expr := mustParse(t, src)
		want := a.typeToString(expr)
		got := renderShape(a.typeShape(expr))
		if got != want {
			t.Errorf("shape parity for %q: got %q, want %q", src, got, want)
		}
	}
}

func TestTypeShapeStructure(t *testing.T) {
	a := New("")
	s := a.typeShape(mustParse(t, "map[string][]Address"))
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
	if k := a.typeShape(mustParse(t, "chan int")).Kind; k != models.ShapeUnknown {
		t.Errorf("chan int kind = %v, want unknown", k)
	}
	// A selector whose package part is not a bare identifier is unmodeled.
	if k := a.typeShape(mustParse(t, "pkg.sub.Type")).Kind; k != models.ShapeUnknown {
		t.Errorf("pkg.sub.Type kind = %v, want unknown", k)
	}
}

// TestShapeBaseName ports TestBaseStructTypeName to shapes. The one deliberate
// divergence is documented in the map row: the old string helper returned
// "map[string]Address" verbatim, which failed every registry lookup; "" fails
// them identically (verified: registerTypeAt and namedScalarKind both bottom out
// at a name lookup no declaration can match, with no warning and no side effect).
func TestShapeBaseName(t *testing.T) {
	a := New("")
	cases := map[string]string{
		"Address":            "Address",
		"*Address":           "Address",
		"[]Address":          "Address",
		"[]*Address":         "Address",
		"**[]*Address":       "Address",
		"*[]Address":         "Address",
		"[][]Address":        "Address",
		"**Address":          "Address",
		"[]string":           "string",
		"time.Time":          "time.Time",
		"int64":              "int64",
		"interface{}":        "interface{}",
		"map[string]Address": "", // a map has no base struct name
		"chan int":           "", // unmodeled shape
	}
	for src, want := range cases {
		if got := shapeBaseName(a.typeShape(mustParse(t, src))); got != want {
			t.Errorf("shapeBaseName(%s) = %q, want %q", src, got, want)
		}
	}
	if got := shapeBaseName(models.TypeShape{Kind: models.ShapePointer}); got != "" {
		t.Errorf("shapeBaseName(dangling pointer) = %q, want \"\"", got)
	}
}

// TestShapeMapValueBase ports TestMapValueStructName. The pointer unwrap on the
// map itself is ONE level (old: strings.TrimPrefix(t, "*")); the unwrap on the
// VALUE is unbounded (old: baseStructTypeName's loop).
func TestShapeMapValueBase(t *testing.T) {
	a := New("")
	type result struct {
		name  string
		isMap bool
	}
	cases := map[string]result{
		"map[string]Address":   {"Address", true},
		"map[string]string":    {"string", true},
		"*map[string]Address":  {"Address", true},
		"**map[string]Address": {"", false}, // one-level unwrap only
		"map[string][]Address": {"Address", true},
		"map[string]*Address":  {"Address", true},
		"[]Address":            {"", false},
		"Address":              {"", false},
		"*Address":             {"", false},
		"string":               {"", false},
	}
	for src, want := range cases {
		name, isMap := shapeMapValueBase(a.typeShape(mustParse(t, src)))
		if name != want.name || isMap != want.isMap {
			t.Errorf("shapeMapValueBase(%s) = (%q,%v), want (%q,%v)", src, name, isMap, want.name, want.isMap)
		}
	}
	if _, isMap := shapeMapValueBase(models.TypeShape{Kind: models.ShapeMap}); isMap {
		t.Error("shapeMapValueBase(map with nil Elem) reported isMap")
	}
}
