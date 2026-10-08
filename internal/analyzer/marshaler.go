package analyzer

import "go/ast"

// The Marshaler guard (CONTEXT.md, "Marshaler type"): a named type with any of
// encoding/json's seven own-format methods is written through that method, not
// through its underlying type, so its underlying type documents nothing about
// its wire form.

const jsontextImportPath = "encoding/json/jsontext"

// The seven methods encoding/json (v1 and v2) dispatches to instead of the
// underlying type.
const (
	methodMarshalJSON       = "MarshalJSON"
	methodMarshalText       = "MarshalText"
	methodMarshalJSONTo     = "MarshalJSONTo"
	methodAppendText        = "AppendText"
	methodUnmarshalJSON     = "UnmarshalJSON"
	methodUnmarshalText     = "UnmarshalText"
	methodUnmarshalJSONFrom = "UnmarshalJSONFrom"
)

// marshalerMethods is what a type's own method set says about its JSON form.
type marshalerMethods struct {
	encode bool   // MarshalJSON, MarshalJSONTo, MarshalText or AppendText
	decode bool   // UnmarshalJSON, UnmarshalJSONFrom or UnmarshalText
	first  string // the first matching method name met, for the warning
}

func (m marshalerMethods) found() bool { return m.encode || m.decode }

// typePred tests one parameter or result type; imports maps the method's own
// file's import names to paths.
type typePred func(e ast.Expr, imports map[string]string) bool

// methodSignature is the exact parameter and result list one of the seven
// methods must have to count, and which side of the wire it is on.
type methodSignature struct {
	params  []typePred
	results []typePred
	encode  bool
}

var (
	jsontextEncoderPtr = jsontextPtr("Encoder")
	jsontextDecoderPtr = jsontextPtr("Decoder")

	// marshalerSignatures is the single table of the seven methods.
	marshalerSignatures = map[string]methodSignature{
		methodMarshalJSON:       {results: []typePred{isByteSliceExpr, isErrorIdent}, encode: true},
		methodMarshalText:       {results: []typePred{isByteSliceExpr, isErrorIdent}, encode: true},
		methodMarshalJSONTo:     {params: []typePred{jsontextEncoderPtr}, results: []typePred{isErrorIdent}, encode: true},
		methodAppendText:        {params: []typePred{isByteSliceExpr}, results: []typePred{isByteSliceExpr, isErrorIdent}, encode: true},
		methodUnmarshalJSON:     {params: []typePred{isByteSliceExpr}, results: []typePred{isErrorIdent}},
		methodUnmarshalText:     {params: []typePred{isByteSliceExpr}, results: []typePred{isErrorIdent}},
		methodUnmarshalJSONFrom: {params: []typePred{jsontextDecoderPtr}, results: []typePred{isErrorIdent}},
	}
)

// marshalerSet scans every file of c's package for methods of name (receiver
// name or *name) that match one of the seven signatures exactly. A method
// declared on an alias receiver name counts only where that alias name is
// used (accepted residual).
func (a *ProjectAnalyzer) marshalerSet(name string, c resolveCtx) marshalerMethods {
	var m marshalerMethods
	for _, pf := range a.samePackageFiles(c.file, c.path) {
		m = a.methodsInFile(pf.file, name, m)
	}
	return m
}

// methodsInFile folds the matching methods of name declared in file into m.
// The file's imports are resolved once, and only when a candidate appears.
func (a *ProjectAnalyzer) methodsInFile(file *ast.File, name string, m marshalerMethods) marshalerMethods {
	var imports map[string]string
	for _, decl := range file.Decls {
		fd, sig := a.marshalerCandidate(decl, name)
		if fd == nil {
			continue
		}
		if imports == nil {
			imports = a.fileImports(file)
		}
		if matchMarshalerSignature(fd, sig, imports) {
			m = m.with(fd.Name.Name, sig.encode)
		}
	}
	return m
}

// marshalerCandidate returns decl and the signature it must have when decl is
// a method of name (receiver name or *name) named like one of the seven
// methods, else nil.
func (a *ProjectAnalyzer) marshalerCandidate(decl ast.Decl, name string) (*ast.FuncDecl, methodSignature) {
	fd, ok := decl.(*ast.FuncDecl)
	if !ok || !a.isMethodOnStruct(fd.Recv, name) {
		return nil, methodSignature{}
	}
	sig, known := marshalerSignatures[fd.Name.Name]
	if !known {
		return nil, methodSignature{}
	}
	return fd, sig
}

// with records one matching method: its side, and its name if it is the first.
func (m marshalerMethods) with(method string, encode bool) marshalerMethods {
	if m.first == "" {
		m.first = method
	}
	m.encode = m.encode || encode
	m.decode = m.decode || !encode
	return m
}

// matchMarshalerSignature reports whether fd's parameters and results are
// exactly sig's.
func matchMarshalerSignature(fd *ast.FuncDecl, sig methodSignature, imports map[string]string) bool {
	return matchTypes(fieldTypes(fd.Type.Params), sig.params, imports) &&
		matchTypes(fieldTypes(fd.Type.Results), sig.results, imports)
}

// matchTypes reports whether types has exactly one entry per predicate, each
// satisfying it.
func matchTypes(types []ast.Expr, preds []typePred, imports map[string]string) bool {
	if len(types) != len(preds) {
		return false
	}
	for i, pred := range preds {
		if !pred(types[i], imports) {
			return false
		}
	}
	return true
}

// fieldTypes flattens a field list into one type per declared name, or one
// type for an unnamed field. A nil list yields an empty slice.
func fieldTypes(fl *ast.FieldList) []ast.Expr {
	out := []ast.Expr{}
	if fl == nil {
		return out
	}
	for _, f := range fl.List {
		n := max(len(f.Names), 1)
		for range n {
			out = append(out, f.Type)
		}
	}
	return out
}

// isByteSliceExpr matches []byte or []uint8.
func isByteSliceExpr(e ast.Expr, _ map[string]string) bool {
	arr, ok := e.(*ast.ArrayType)
	if !ok || arr.Len != nil {
		return false
	}
	elt, ok := arr.Elt.(*ast.Ident)
	return ok && (elt.Name == goTypeByte || elt.Name == goTypeUint8)
}

// isErrorIdent matches the builtin error.
func isErrorIdent(e ast.Expr, _ map[string]string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == frameworkTypeError
}

// jsontextPtr matches *jsontext.<typeName>, where the package name maps
// through the method's own file's imports to encoding/json/jsontext exactly
// (an aliased import counts).
func jsontextPtr(typeName string) typePred {
	return func(e ast.Expr, imports map[string]string) bool {
		star, ok := e.(*ast.StarExpr)
		if !ok {
			return false
		}
		sel, ok := star.X.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != typeName {
			return false
		}
		pkg, ok := sel.X.(*ast.Ident)
		return ok && imports[pkg.Name] == jsontextImportPath
	}
}
