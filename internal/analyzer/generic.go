package analyzer

import (
	"go/ast"
	"go/types"
	"strings"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Unresolved instantiations (issue 138). A generic type written with its type
// arguments (CONTEXT.md, "Instantiation") decodes as a ShapeInstantiation,
// which is not resolved until #114 substitutes its arguments. Wherever it is
// written it is documented as before — an untyped object, no requestBody, an
// untyped payload — and raises one warning, so --strict fails on it.

// Instantiation warnings. The field one is raised once per field declaration
// (warnFieldFallback), the request and payload ones once per route. Each
// names the type as written, then the Instantiation, as written, where
// resolution stopped: the outermost one on the type's element path, or in the
// declaration it resolves through.
const (
	genericFieldWarning   = "field %s at %s has type %s: %s is a generic type instantiation, which is not resolved — emitting an untyped object"
	genericRequestWarning = "request type %s: %s is a generic type instantiation, which is not resolved — no requestBody emitted"
	genericPayloadWarning = "response type %s: %s is a generic type instantiation, which is not resolved — emitting an untyped schema"
)

// warnNonStructRequest raises the one warning of a request that does not
// register as a struct. A request whose type, a pointer written in it shed,
// resolves to an Instantiation gets the generic request warning, unless its
// generic type is declared as a non-struct type, which go-bricks cannot bind
// (F26). A generic type the tool cannot locate (outside the module,
// undeclared) gets the generic warning too: go-bricks binds it if it is a
// struct, so a panic cannot be claimed. Every other request gets F26.
func (a *ProjectAnalyzer) warnNonStructRequest(h *handlerAnalysis, astFile *ast.File, filePath string) {
	if inst, ok := a.requestInstantiation(h.requestExpr, astFile, filePath); ok {
		a.addWarningf(genericRequestWarning, h.requestType, inst)
		return
	}
	a.addWarningf(nonStructRequestWarning, h.requestType)
}

// requestInstantiation resolves a request's type, expr as written in astFile,
// and returns the Instantiation, as written, its root resolves to: directly
// (Box[A], *Box[A]) or through aliases and defined types (type AddrPage
// Page[Address]). The root resolves as its underlying type, its methods
// ignored: F26 is a claim about the type's kind, which methods do not change,
// so a Marshaler or text type declared over an Instantiation (type MPage
// Page[Address] with MarshalJSON) is seen through to it. Only a pointer
// written in expr is shed: a named pointer type over one (type PP *Page[A],
// type PtrBox[T any] *Box[T]) is a non-struct request like type PA *Address
// (F26). ok is false when the root is anything else (LP, a named slice of
// one, included) or the Instantiation's generic type is declared as a
// non-struct type. A nil expr decodes as ShapeUnknown, so it is not one.
func (a *ProjectAnalyzer) requestInstantiation(expr ast.Expr, astFile *ast.File, filePath string) (inst string, ok bool) {
	if star, isStar := expr.(*ast.StarExpr); isStar {
		expr = star.X
	}
	c := newResolveCtx(astFile, filePath, false)
	r := a.resolveUnderlying(types.ExprString(expr), expr, c)
	fb := c.st.first
	if r.Kind != models.ShapeInstantiation || fb.kind != fallbackGeneric {
		return "", false
	}
	return fb.typeName, !a.genericBaseNonStruct(fb.detail, *fb.site)
}

// genericBaseNonStruct reports whether base, a generic type as written in
// site, is declared as a type that is not a struct. Its declaration is
// followed, type arguments shed, through generic aliases and defined types
// (type GA[T any] = Box[T]), local or in another package of the module, to a
// struct (false), or to any other type literal or a builtin (true). A chain
// the tool cannot follow to its end (out of the module, undeclared, a cycle)
// is not reported.
func (a *ProjectAnalyzer) genericBaseNonStruct(base string, site pkgFile) bool {
	name := base
	for range maxNamedResolutionDepth {
		ts, at, ok := a.typeSpecOf(name, site)
		if !ok {
			return isBuiltinShapeName(name)
		}
		next, isStruct := typeSpecTarget(ts.Type)
		if next == "" {
			return !isStruct
		}
		name, site = next, at
	}
	return false
}

// typeSpecOf finds the declaration of name as written in site: in site's
// package, or, for q.T, in another package of the module.
func (a *ProjectAnalyzer) typeSpecOf(name string, site pkgFile) (*ast.TypeSpec, pkgFile, bool) {
	if strings.Contains(name, ".") {
		declSite, local, ok := a.inModuleTypeSite(name, site.file)
		if !ok {
			return nil, pkgFile{}, false
		}
		site, name = declSite, local
	}
	ts, file, path, ok := a.resolveLocalTypeSpec(site.file, site.path, name)
	return ts, pkgFile{file: file, path: path}, ok
}

// typeSpecTarget returns the type name a declaration's right-hand side
// names, parentheses and type arguments shed (Box for Box[T], q.T for
// q.T[A]), or "" for a type literal, with whether that literal is a struct.
func typeSpecTarget(rhs ast.Expr) (name string, isStruct bool) {
	switch t := unwrapGenericType(ast.Unparen(rhs)).(type) {
	case *ast.StructType:
		return "", true
	case *ast.Ident:
		return t.Name, false
	case *ast.SelectorExpr:
		if pkg, ok := t.X.(*ast.Ident); ok {
			return pkg.Name + "." + t.Sel.Name, false
		}
	}
	return "", false
}

// payloadInstantiation returns the Instantiation, as written, that a
// handler's payload type (payloadExpr) holds on its element path, or "". A
// pointer to a result wrapper (*server.Result[T]), which payloadExpr does not
// unwrap, is go-bricks' own generic type, not one of the project's: it is
// not reported, so its handler stays as it was before issue 138.
func (a *ProjectAnalyzer) payloadInstantiation(results *ast.FieldList, serverAliases map[string]struct{}) string {
	if results == nil || len(results.List) == 0 {
		return ""
	}
	payload := a.payloadExpr(results.List[0].Type, serverAliases)
	if star, ok := payload.(*ast.StarExpr); ok && a.isResultWrapper(unwrapGenericType(singlePointer(star).X), serverAliases) {
		return ""
	}
	s := a.typeShape(payload)
	return firstInstantiation(&s)
}

// warnGenericPayload raises the generic payload warning of a handler whose
// payload holds an Instantiation and so resolved to no TypeInfo at all
// (Result[Page[A]], Result[[]Page[A]], a bare Pair[K, V]). The nil response
// check is defensive: today no payload holding an Instantiation resolves to
// a TypeInfo, but one that does is warned about by screenGenericPayload.
func (a *ProjectAnalyzer) warnGenericPayload(h *handlerAnalysis) {
	if h.response == nil && h.responseInst != "" {
		a.addWarningf(genericPayloadWarning, h.responseType, h.responseInst)
	}
}

// screenGenericPayload screens a payload whose resolution did not resolve:
// when the first fallback it noted (fb) is an Instantiation (Result[AddrPage],
// Result[LP]), it raises the generic payload warning and clears the name, so
// screenUnresolvedPayload stays silent, and reports true.
func (a *ProjectAnalyzer) screenGenericPayload(ti *models.TypeInfo, written string, fb fieldFallback) bool {
	if fb.kind != fallbackGeneric {
		return false
	}
	a.addWarningf(genericPayloadWarning, written, fb.typeName)
	ti.Name = ""
	ti.Fields = nil
	return true
}

// nestedEmbedName is the Go name of a json-tagged embed's field: typeName,
// its type's base name, except for a generic instantiation, whose base name
// is empty: its generic type as written, pointer and type arguments shed
// (Page for Page[Address], pg.Page for *pg.Page[Address]).
func nestedEmbedName(t ast.Expr, typeName string) string {
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if base := unwrapGenericType(t); base != t {
		return types.ExprString(base)
	}
	return typeName
}
