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
