package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Parameter binding (issue 121). go-bricks binds a request's path, query and
// header parameters by reflect.Kind (setFieldValue, kindSetters and
// isStringSliceType, as of v0.58.1), plus exactly time.Time, and answers 400 to
// a request that sends a value of any other type. Such a parameter is still
// documented as before, but its field raises one warning, so --strict fails
// on it. Only the request type's own top-level fields are checked, as
// buildBindingPlan reads them, a tagged embedded non-struct type included:
// an embedded struct, tagged or not, is not (issue 147).

// unbindableParamWarning names the field, its position, the parameter's
// source and name, its type as written, and why go-bricks cannot bind it
// (one of the reasons below).
const unbindableParamWarning = "field %s at %s is the %s parameter %q of type %s, which go-bricks cannot bind (%s), " +
	"so a request that sends it gets a 400"

// Why a parameter cannot bind.
const (
	reasonArray        = "an array"
	reasonStruct       = "a struct other than time.Time"
	reasonMap          = "a map"
	reasonInterface    = "an interface"
	reasonComplex      = "a complex number"
	reasonUintptr      = "a uintptr"
	reasonFuncOrChan   = "a func or a chan"
	reasonPathSlice    = "a slice in a path parameter"
	reasonSliceElem    = "a slice whose element is not a string"
	reasonPointerSlice = "a pointer to a slice"
)

// binderKind is the class of reflect.Kind go-bricks' binder dispatches a
// parameter on. The zero value is a type the analyzer did not resolve far
// enough to tell, which gets no verdict.
type binderKind int

const (
	kindUnresolved binderKind = iota
	kindString                // a string kind: binds, and is the one slice element that binds
	kindScalar                // another kindSetters kind: bool, every int and uint but uintptr, float32, float64
	kindTime                  // exactly time.Time, setSpecialType's one type
	kindPointer
	kindSlice
	kindByteSlice // a well-known slice of byte, whose element the Resolution does not carry
	kindArray
	kindStruct
	kindMap
	kindInterface
	kindComplex
	kindUintptr
	kindFuncOrChan
)

// kindReasons is why each kind cannot bind; a kind absent here binds, or is
// a container judged by its element (paramBindReason).
var kindReasons = map[binderKind]string{
	kindArray:      reasonArray,
	kindStruct:     reasonStruct,
	kindMap:        reasonMap,
	kindInterface:  reasonInterface,
	kindComplex:    reasonComplex,
	kindUintptr:    reasonUintptr,
	kindFuncOrChan: reasonFuncOrChan,
}

// bigInt is math/big's Int as the Shape decoder writes it. It is not
// well-known until issue 113 types it, so its row is not consulted yet: a
// big.Int parameter is unresolvable and keeps that warning.
const bigInt = "big.Int"

// wellKnownBinderKinds is the binder kind of each well-known type name the
// binder cannot see through its schema: the analyzer keeps such a name as a
// leaf, so its declaration's kind is known only from here. It is consulted
// only for a name in models.WellKnownTypeNames, and a name with no row gets
// no verdict. The kind-backed names (time.Duration, time.Month,
// time.Weekday, os.FileMode, fs.FileMode) need no row: they resolve to their
// builtin first (knownUnderlyingBuiltins).
var wellKnownBinderKinds = map[string]binderKind{
	models.WellKnownTimeTime:      kindTime,
	models.WellKnownJSONNumber:    kindString,
	models.WellKnownUUID:          kindArray,
	models.WellKnownRawMessage:    kindByteSlice,
	models.WellKnownNetIP:         kindByteSlice,
	models.WellKnownNetipAddr:     kindStruct,
	models.WellKnownNetipPrefix:   kindStruct,
	models.WellKnownNetipAddrPort: kindStruct,
	bigInt:                        kindStruct, // *big.Int too, through the pointer rule
}

// warnUnbindableParams checks the parameters of the struct a request TypeInfo
// names, before the request registers, so that a parameter that cannot bind
// raises the binding warning as its one warning (setBindingWarning). A
// request that is no struct of the module is not checked.
func (a *ProjectAnalyzer) warnUnbindableParams(ti *models.TypeInfo, astFile *ast.File, filePath string) {
	site, ok := a.payloadStructSite(ti, astFile, filePath)
	if !ok || site.st.Fields == nil {
		return
	}
	for _, field := range site.st.Fields.List {
		a.warnUnbindableParam(field, site)
	}
}

// warnUnbindableParam raises the binding warning for one field of a request
// struct declared at site, as buildBindingPlan reads it: an exported field
// with a param, query or header tag (the analyzer's source, the last of the
// three present). Its type is resolved as a parameter is, in site's file,
// with its parentheses stripped. An embedded struct, or a pointer to one, is
// left to issue 147.
func (a *ProjectAnalyzer) warnUnbindableParam(field *ast.Field, site structSite) {
	name := paramFieldName(field)
	if name == "" || field.Tag == nil {
		return
	}
	source, param := a.parseParameterTags(unquoteLiteral(field.Tag.Value))
	if source == "" {
		return
	}
	c := newResolveCtx(site.file, site.path, true)
	r := a.resolveShape(a.typeShape(unparenType(field.Type)), c)
	if field.Names == nil && embedsStruct(&r, c.st.first) {
		return
	}
	reason, known := paramBindReason(&r, source, c.st.first)
	if !known || reason == "" {
		return
	}
	loc := a.fieldLoc(field)
	a.setBindingWarning(loc, fmt.Sprintf(unbindableParamWarning, name, loc, source, param, types.ExprString(field.Type), reason))
}

// setBindingWarning makes msg the one warning of the field declared at loc,
// whichever route reaches the field first. It takes the place of the warning
// the field already raised, as a field of a struct registered before a route
// took it as its request, or by an earlier route taking the same request.
// Otherwise it is added, and loc is marked so the uintptr or schema fallback
// warning the request's registration would raise is deduplicated away.
func (a *ProjectAnalyzer) setBindingWarning(loc, msg string) {
	a.uintptrWarned[loc] = struct{}{}
	a.fieldWarned[loc] = struct{}{}
	if i, warned := a.fieldWarnings[loc]; warned {
		a.warnings[i] = msg
		return
	}
	a.fieldWarnings[loc] = len(a.warnings)
	a.warnings = append(a.warnings, msg)
}

// paramFieldName is the name buildBindingPlan reads a field under: the first
// exported name it declares, or an embedded field's type name, and "" when
// that is unexported, as go-bricks skips the field.
func paramFieldName(field *ast.Field) string {
	if field.Names == nil {
		if name := embeddedFieldName(field.Type); ast.IsExported(name) {
			return name
		}
		return ""
	}
	for _, n := range field.Names {
		if n.IsExported() {
			return n.Name
		}
	}
	return ""
}

// unparenType strips the parentheses from a parameter's type, at its top
// and under its pointers, slices and arrays: reflect sees (int) as int and
// *(string) as *string, while the Shape decoder leaves a parenthesized type
// unknown. The field's own expression is not changed.
func unparenType(expr ast.Expr) ast.Expr {
	switch t := ast.Unparen(expr).(type) {
	case *ast.StarExpr:
		c := *t
		c.X = unparenType(t.X)
		return &c
	case *ast.ArrayType:
		c := *t
		c.Elt = unparenType(t.Elt)
		return &c
	default:
		return t
	}
}

// embedsStruct reports whether an embedded field's resolved type r is a
// struct, at any pointer depth: go-bricks binds such an embed as one field,
// and judging it is issue 147's.
func embedsStruct(r *models.TypeShape, fb fieldFallback) bool {
	for r != nil && r.Kind == models.ShapePointer {
		r = r.Elem
	}
	return binderKindOf(r, fb) == kindStruct
}

// paramBindReason reports why go-bricks cannot bind a parameter of resolved
// type r from source ("" when it can), and whether the analyzer resolved
// enough of r to tell. fb is the first fallback r's resolution noted: a
// parameter's type has one leaf path, so it is that leaf's.
func paramBindReason(r *models.TypeShape, source string, fb fieldFallback) (string, bool) {
	switch k := binderKindOf(r, fb); k {
	case kindUnresolved:
		return "", false
	case kindPointer:
		return pointerBindReason(r.Elem, source, fb)
	case kindSlice, kindByteSlice:
		return sliceBindReason(r, k, source, fb)
	default:
		return kindReasons[k], true
	}
}

// pointerBindReason judges a pointer parameter by what it points to:
// setFieldValue allocates and recurses, at any depth. A pointer to a slice
// never binds: the string-slice branch tests the field's own type, a pointer.
func pointerBindReason(elem *models.TypeShape, source string, fb fieldFallback) (string, bool) {
	if k := binderKindOf(elem, fb); k == kindSlice || k == kindByteSlice {
		return reasonPointerSlice, true
	}
	return paramBindReason(elem, source, fb)
}

// sliceBindReason judges a slice parameter of kind k: a path parameter never
// binds one, and a query or header parameter binds only a slice whose
// element is a string kind (isStringSliceType).
func sliceBindReason(r *models.TypeShape, k binderKind, source string, fb fieldFallback) (string, bool) {
	if source == paramTypePath {
		return reasonPathSlice, true
	}
	if k == kindByteSlice {
		return reasonSliceElem, true
	}
	switch binderKindOf(r.Elem, fb) {
	case kindString:
		return "", true
	case kindUnresolved:
		return "", false
	default:
		return reasonSliceElem, true
	}
}

// binderKindOf classifies one node of a parameter's Resolution. An
// Instantiation (issue 114), a recursion cut and an unresolvable name get no
// verdict; a Marshaler or text leaf never occurs, as a parameter is resolved
// by kind.
func binderKindOf(s *models.TypeShape, fb fieldFallback) binderKind {
	if s == nil {
		return kindUnresolved
	}
	switch s.Kind {
	case models.ShapePointer:
		if s.Elem == nil {
			return kindUnresolved
		}
		return kindPointer
	case models.ShapeSlice:
		return kindSlice
	case models.ShapeArray:
		return kindArray
	case models.ShapeMap:
		return kindMap
	case models.ShapeRef, models.ShapeStruct:
		return kindStruct
	case models.ShapePrimitive, models.ShapeKindOnly:
		return scalarBinderKind(*s)
	case models.ShapeNamed:
		return namedBinderKind(s.Name, fb)
	case models.ShapeUnknown, "":
		return kindFuncOrChan
	default:
		return kindUnresolved
	}
}

// scalarBinderKind classifies a builtin or a build-tagged scalar's kind.
func scalarBinderKind(s models.TypeShape) binderKind {
	switch kind := leafKind(s); kind {
	case "":
		// Not a kindSetters kind: one of the builtins below.
	case goTypeString:
		return kindString
	default:
		return kindScalar
	}
	switch s.Name {
	case goTypeUintptr:
		return kindUintptr
	case goTypeComplex64, goTypeComplex128:
		return kindComplex
	case goTypeAny, goTypeInterface, frameworkTypeError:
		return kindInterface
	default:
		return kindUnresolved
	}
}

// namedBinderKind classifies a named leaf: a well-known type kept as is (no
// fallback noted) by its row, and a defined type over one
// (fallbackDefinedOverQualified, its detail the well-known type) by that
// type's kind, except over time.Time, whose special case is by type
// identity: a defined type over it is a plain struct. Any other named leaf
// (an unresolvable name, disagreeing declarations) gets no verdict.
func namedBinderKind(name string, fb fieldFallback) binderKind {
	switch {
	case fb.kind == fallbackNone && models.WellKnownTypeNames[name]:
		return wellKnownBinderKinds[name]
	case fb.kind == fallbackDefinedOverQualified && models.WellKnownTypeNames[fb.detail]:
		if k := wellKnownBinderKinds[fb.detail]; k != kindTime {
			return k
		}
		return kindStruct
	default:
		return kindUnresolved
	}
}
