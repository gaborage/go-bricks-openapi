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
// register as a struct. The only anonymous struct that gets here is a
// field-less one (struct{}, *struct{}, go-bricks' request type for a route
// that takes no input; one with a field is documented by anonymousRequest):
// it gets none, as it has nothing to document, and a named empty struct
// documents nothing either. A request whose type, a
// pointer written in it shed, resolves to an Instantiation gets the generic
// request warning, unless its generic type is declared as a non-struct type,
// which go-bricks cannot bind (F26). A generic type the tool cannot locate
// (outside the module, undeclared) gets the generic warning too: go-bricks
// binds it if it is a struct, so a panic cannot be claimed. A request whose
// type's build-tagged declarations each reach a different struct gets the
// build-tagged request warning (requestStructCopies). Every other request
// gets F26.
func (a *ProjectAnalyzer) warnNonStructRequest(h *handlerAnalysis, astFile *ast.File, filePath string) {
	if anonymousStruct(h.requestExpr) != nil {
		return
	}
	if inst, ok := a.requestInstantiation(h.requestExpr, astFile, filePath); ok {
		a.addWarningf(genericRequestWarning, h.requestType, inst)
		return
	}
	if fb, ok := a.requestStructCopies(h.requestExpr, astFile, filePath); ok {
		a.addWarningf(declsDisagreeRequestWarning, h.requestType, fb.typeName, declsCause(fb))
		return
	}
	a.addWarningf(nonStructRequestWarning, h.requestType)
}

// requestStructCopies resolves a request's type, expr as written in astFile,
// a pointer written in it shed, as requestInstantiation does, and returns the
// disagreement its root stops at when that is a type whose build-tagged
// declarations each reach a struct, but not the same one
// (structCopiesDisagree). Every build binds such a type as a struct, so F26
// would be false. Copies that disagree any other way (type Rows []A beside
// type Rows []B) are non-structs in every build, and keep F26, as does a
// container of such a type ([]W, type WS []W, type PW *W), whose root is no
// struct in any build.
func (a *ProjectAnalyzer) requestStructCopies(expr ast.Expr, astFile *ast.File, filePath string) (fieldFallback, bool) {
	expr = pointee(expr)
	c := newResolveCtx(astFile, filePath, false)
	r := a.resolveUnderlying(types.ExprString(expr), expr, c)
	fb := c.st.first
	return fb, r.Kind == models.ShapeNamed && fb.kind == fallbackDeclsDisagree && fb.structCopies
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
// (F26). Parentheses around and under that pointer are shed too (pointee).
// ok is false when the root is anything else (LP, a named slice of one,
// included) or the Instantiation's generic type is declared as a non-struct
// type. A nil expr decodes as ShapeUnknown, so it is not one.
func (a *ProjectAnalyzer) requestInstantiation(expr ast.Expr, astFile *ast.File, filePath string) (inst string, ok bool) {
	expr = pointee(expr)
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
// handler's payload type (payloadExpr) holds on its element path, or "".
// payloadExpr sheds a single pointer to a result wrapper, so
// *server.Result[Page[A]] reports Page[A]. Behind two or more pointers
// (**server.Result[T]) the wrapper is not unwrapped and is reported itself:
// go-bricks then encodes the Result struct under data, which stays untyped,
// so the generic payload warning lets --strict catch it.
func (a *ProjectAnalyzer) payloadInstantiation(results *ast.FieldList, serverAliases map[string]struct{}) string {
	if results == nil || len(results.List) == 0 {
		return ""
	}
	payload := a.payloadExpr(results.List[0].Type, serverAliases)
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
