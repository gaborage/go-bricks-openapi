package analyzer

import (
	"go/ast"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

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

// methodBits is a set of the seven methods, one bit each.
type methodBits uint8

const (
	bitMarshalJSON methodBits = 1 << iota
	bitMarshalText
	bitMarshalJSONTo
	bitAppendText
	bitUnmarshalJSON
	bitUnmarshalText
	bitUnmarshalJSONFrom
)

// marshalerMethods is what a type's own method set says about its JSON form.
type marshalerMethods struct {
	encode  bool       // MarshalJSON, MarshalJSONTo, MarshalText or AppendText
	decode  bool       // UnmarshalJSON, UnmarshalJSONFrom or UnmarshalText
	first   string     // the first matching method name met, for the warning
	has     methodBits // every matching method
	ptrText bool       // MarshalText is not in the value method set (pointer receiver only)
}

func (m marshalerMethods) found() bool { return m.encode || m.decode }

// textBothWays reports whether encoding/json writes and reads the type as a
// JSON string on every toolchain and in every position (ADR 0002):
// MarshalText in the value method set with no MarshalJSON or MarshalJSONTo,
// and UnmarshalText with no UnmarshalJSON or UnmarshalJSONFrom. AppendText
// alone does not count: the classic encoding/json ignores it and writes the
// underlying type. Nor does a MarshalText only on the pointer: encoding/json
// skips it on a non-addressable value (a map value, a Result[T] payload) and
// writes the underlying type there. UnmarshalText may be on the pointer: a
// decode target is always addressable.
func (m marshalerMethods) textBothWays() bool {
	const jsonSpecific = bitMarshalJSON | bitMarshalJSONTo | bitUnmarshalJSON | bitUnmarshalJSONFrom
	return m.has&bitMarshalText != 0 && m.has&bitUnmarshalText != 0 && m.has&jsonSpecific == 0 && !m.ptrText
}

// typePred tests one parameter or result type; imports maps the method's own
// file's import names to paths.
type typePred func(e ast.Expr, imports map[string]string) bool

// methodSignature is the exact parameter and result list one of the seven
// methods must have to count, and which side of the wire it is on.
type methodSignature struct {
	params  []typePred
	results []typePred
	encode  bool
	bit     methodBits
}

var (
	jsontextEncoderPtr = jsontextPtr("Encoder")
	jsontextDecoderPtr = jsontextPtr("Decoder")

	// marshalerSignatures is the single table of the seven methods.
	marshalerSignatures = map[string]methodSignature{
		methodMarshalJSON:       {results: []typePred{isByteSliceExpr, isErrorIdent}, encode: true, bit: bitMarshalJSON},
		methodMarshalText:       {results: []typePred{isByteSliceExpr, isErrorIdent}, encode: true, bit: bitMarshalText},
		methodMarshalJSONTo:     {params: []typePred{jsontextEncoderPtr}, results: []typePred{isErrorIdent}, encode: true, bit: bitMarshalJSONTo},
		methodAppendText:        {params: []typePred{isByteSliceExpr}, results: []typePred{isByteSliceExpr, isErrorIdent}, encode: true, bit: bitAppendText},
		methodUnmarshalJSON:     {params: []typePred{isByteSliceExpr}, results: []typePred{isErrorIdent}, bit: bitUnmarshalJSON},
		methodUnmarshalText:     {params: []typePred{isByteSliceExpr}, results: []typePred{isErrorIdent}, bit: bitUnmarshalText},
		methodUnmarshalJSONFrom: {params: []typePred{jsontextDecoderPtr}, results: []typePred{isErrorIdent}, bit: bitUnmarshalJSONFrom},
	}

	// marshalerMethodOrder fixes the order a method set is read in, so the
	// method a warning names is deterministic.
	marshalerMethodOrder = []string{
		methodMarshalJSON, methodMarshalJSONTo, methodMarshalText, methodAppendText,
		methodUnmarshalJSON, methodUnmarshalJSONFrom, methodUnmarshalText,
	}

	// wellKnownMethodSets are the out-of-module types whose methods are known
	// without their source, read only when one is embedded (promotion): used
	// directly, each keeps its well-known schema. Every encode-side method
	// here has a value receiver, so none is pointer-only. time.Time has had
	// AppendText since Go 1.24, below the go 1.25 floor.
	wellKnownMethodSets = map[string][]string{
		models.WellKnownTimeTime:   {methodMarshalJSON, methodMarshalText, methodAppendText, methodUnmarshalJSON, methodUnmarshalText},
		models.WellKnownUUID:       {methodMarshalText, methodUnmarshalText},
		models.WellKnownRawMessage: {methodMarshalJSON, methodUnmarshalJSON},
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
	a.visitMarshalerMethods(file, name, func(method string, exact, ptr bool) {
		if exact {
			m = m.with(method, ptr)
		}
	})
	return m
}

// visitMarshalerMethods calls visit for every method of name (receiver name
// or *name) declared in file under one of the seven names, with whether its
// signature is exact and whether its receiver is the pointer. The file's
// imports are resolved once, and only when a candidate appears.
func (a *ProjectAnalyzer) visitMarshalerMethods(file *ast.File, name string, visit func(method string, exact, ptr bool)) {
	var imports map[string]string
	for _, decl := range file.Decls {
		fd, sig := a.marshalerCandidate(decl, name)
		if fd == nil {
			continue
		}
		if imports == nil {
			imports = a.fileImports(file)
		}
		_, ptr := fd.Recv.List[0].Type.(*ast.StarExpr)
		visit(fd.Name.Name, matchMarshalerSignature(fd, sig, imports), ptr)
	}
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

// with records one matching method: its side, its name if it is the first,
// and whether a MarshalText is missing from the value method set (ptrOnly).
func (m marshalerMethods) with(method string, ptrOnly bool) marshalerMethods {
	if m.first == "" {
		m.first = method
	}
	sig := marshalerSignatures[method]
	m.encode = m.encode || sig.encode
	m.decode = m.decode || !sig.encode
	m.has |= sig.bit
	m.ptrText = m.ptrText || (ptrOnly && method == methodMarshalText)
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

// methodEntry is one of the seven method names in a type's method set, as
// Go's selector rules see it (#111).
type methodEntry struct {
	depth     int    // 0: declared on the type or a type it aliases; n: promoted through n embeddings
	exact     bool   // the selected method has the exact interface signature
	ambiguous bool   // two embeds tie at the shallowest depth: in no method set, and it hides deeper ones
	ptrOnly   bool   // in the pointer's method set only: declared on *T, or promoted only through value embeds from such
	owner     string // depth 0: the declaring type, as a warning names it
	via       string // depth > 0: the embedded field it is promoted through, as written
}

// methodSet maps each of the seven names a type's method set holds to its
// entry. A name absent from the map is not in the set.
type methodSet map[string]methodEntry

// over adds to ms every name of deeper that ms lacks: a shallower name
// shadows a deeper one.
func (ms methodSet) over(deeper methodSet) methodSet {
	for name, e := range deeper {
		if _, ok := ms[name]; !ok {
			ms[name] = e
		}
	}
	return ms
}

// marshaler reads ms in marshalerMethodOrder: what its exact, unambiguous
// entries say about the type's JSON form, and the fallback the first one
// names (typeName is the struct a promoted method belongs to).
func (ms methodSet) marshaler(typeName string) (m marshalerMethods, fb fieldFallback) {
	for _, method := range marshalerMethodOrder {
		e, ok := ms[method]
		if !ok || !e.exact || e.ambiguous {
			continue
		}
		if !m.found() {
			fb = e.fallback(typeName, method)
		}
		m = m.with(method, e.ptrOnly)
	}
	return m, fb
}

// fallback is the warning an entry raises when its method makes the type {}.
func (e methodEntry) fallback(typeName, method string) fieldFallback {
	if e.depth == 0 {
		return fieldFallback{kind: fallbackMarshaler, typeName: e.owner, detail: method}
	}
	return fieldFallback{kind: fallbackMarshalerPromoted, typeName: typeName, detail: method, via: e.via}
}

// embedMethods is the method set of one embedded field, the field's type as
// written (its pointer shed), which its promoted entries name, and whether it
// is embedded by pointer.
type embedMethods struct {
	via string
	set methodSet
	ptr bool
}

// promote folds the method sets of a struct's embedded fields into the set
// they promote into the struct (Go's selector rules): each name at its
// shallowest depth plus one, from the one embed providing it there; two
// embeds tying at that depth make it ambiguous. A pointer embed puts its
// target's pointer-only methods in the struct's value method set; a value
// embed keeps them pointer-only.
func promote(embeds []embedMethods) methodSet {
	out := methodSet{}
	for _, em := range embeds {
		for name, e := range em.set {
			cand := methodEntry{depth: e.depth + 1, exact: e.exact, ambiguous: e.ambiguous, ptrOnly: e.ptrOnly && !em.ptr, via: em.via}
			prev, seen := out[name]
			switch {
			case !seen || cand.depth < prev.depth:
				out[name] = cand
			case cand.depth == prev.depth:
				out[name] = methodEntry{depth: cand.depth, ambiguous: true}
			}
		}
	}
	return out
}

// declaredMethods returns the methods declared on name (receiver name or
// *name) in every file of c's package under one of the seven names, exact or
// not: a non-exact one is in the method set, so it shadows a promoted one.
func (a *ProjectAnalyzer) declaredMethods(name string, c resolveCtx) methodSet {
	ms := methodSet{}
	owner := c.display(name)
	for _, pf := range a.samePackageFiles(c.file, c.path) {
		a.visitMarshalerMethods(pf.file, name, func(method string, exact, ptr bool) {
			e := ms[method]
			ms[method] = methodEntry{exact: e.exact || exact, ptrOnly: e.ptrOnly || ptr, owner: owner}
		})
	}
	return ms
}

// methodSetOf returns the method set of the named type name declared in c's
// package: the methods declared on it (when keep) and on every type it
// aliases, over what the struct at the end of its chain promotes. A defined
// type drops its target's declared methods but keeps what the target's struct
// promotes. open holds the typeKeys being walked: a type met again (S{*S})
// adds nothing, which is exact, since everything it would add again sits
// deeper than its first occurrence and is shadowed there. There is no depth
// cap: Go promotes through any depth of embedding, and open bounds the walk.
//
// A set computed without a cycle cut does not depend on the path to it, so
// it is memoized (per type, keep and the home package its owners are
// displayed from): a diamond of embeds is walked once per type, not once per
// path. A set a cycle cut shortened is path-dependent and recomputed.
func (a *ProjectAnalyzer) methodSetOf(name string, c resolveCtx, keep bool, open []string) methodSet {
	key := c.typeKey(name)
	if slices.Contains(open, key) {
		a.methodSetCuts++
		return methodSet{}
	}
	memoKey := c.st.home + "\x00" + key + "\x00" + strconv.FormatBool(keep)
	if ms, ok := a.methodSetMemo[memoKey]; ok {
		return maps.Clone(ms)
	}
	cuts := a.methodSetCuts
	ms := a.computeMethodSet(name, c, keep, append(slices.Clip(open), key))
	if a.methodSetCuts == cuts {
		if a.methodSetMemo == nil {
			a.methodSetMemo = map[string]methodSet{}
		}
		a.methodSetMemo[memoKey] = maps.Clone(ms)
	}
	return ms
}

// computeMethodSet is methodSetOf's walk, with name's key already on open.
func (a *ProjectAnalyzer) computeMethodSet(name string, c resolveCtx, keep bool, open []string) methodSet {
	a.methodSetWalks++
	own := methodSet{}
	if keep {
		own = a.declaredMethods(name, c)
	}
	ts, file, path, ok := a.resolveLocalTypeSpec(c.file, c.path, name)
	if !ok {
		return own
	}
	return own.over(a.rhsMethodSet(ts.Type, c.in(file, path), keep && ts.Assign.IsValid(), open))
}

// rhsMethodSet returns the method set a declaration's right-hand side
// contributes: what a struct literal promotes, or the set of the named type
// it names (keep false for a defined type's target).
func (a *ProjectAnalyzer) rhsMethodSet(rhs ast.Expr, c resolveCtx, keep bool, open []string) methodSet {
	switch t := rhs.(type) {
	case *ast.StructType:
		return a.promotedMethods(t, c, open)
	case *ast.Ident:
		return a.methodSetOf(t.Name, c, keep, open)
	case *ast.SelectorExpr:
		return a.qualifiedMethodSet(a.typeShape(t).Name, c, keep, open)
	default:
		return methodSet{}
	}
}

// qualifiedMethodSet returns the method set of q.T: an in-module type's, in
// its declaring package, else a well-known type's (wellKnownMethodSets), else
// none (a third-party type is not read).
func (a *ProjectAnalyzer) qualifiedMethodSet(qualified string, c resolveCtx, keep bool, open []string) methodSet {
	if site, name, ok := a.inModuleTypeSite(qualified, c.file); ok {
		return a.methodSetOf(name, c.in(site.file, site.path), keep, open)
	}
	ms := methodSet{}
	if !keep {
		return ms
	}
	for _, method := range wellKnownMethodSets[qualified] {
		ms[method] = methodEntry{exact: true, owner: qualified}
	}
	return ms
}

// promotedMethods returns the method set a struct literal's embedded fields
// promote, whatever their json tag: by value, by pointer, local, from another
// package of the module, or well-known. A field of the struct named like one
// of the seven methods (an embedded one by its type's name) sits at the
// struct's own depth as a non-exact entry: like a method there, it shadows a
// deeper promoted method and, once embedded, ties with one at its depth.
func (a *ProjectAnalyzer) promotedMethods(st *ast.StructType, c resolveCtx, open []string) methodSet {
	var embeds []embedMethods
	var fieldNames []string
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			embeds = append(embeds, a.embedMethodSet(f, c, open))
			fieldNames = append(fieldNames, embeddedFieldName(f.Type))
			continue
		}
		for _, n := range f.Names {
			fieldNames = append(fieldNames, n.Name)
		}
	}
	ms := promote(embeds)
	for _, n := range fieldNames {
		if _, isMethodName := marshalerSignatures[n]; isMethodName {
			ms[n] = methodEntry{}
		}
	}
	return ms
}

// embeddedFieldName is the field name Go gives an embedded field of type t:
// its type name, without pointer, package qualifier or type arguments.
func embeddedFieldName(t ast.Expr) string {
	switch e := t.(type) {
	case *ast.StarExpr:
		return embeddedFieldName(e.X)
	case *ast.IndexExpr:
		return embeddedFieldName(e.X)
	case *ast.IndexListExpr:
		return embeddedFieldName(e.X)
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.Ident:
		return e.Name
	default:
		return ""
	}
}

// embedMethodSet returns one embedded field's method set, its target's, and
// whether it is a pointer embed (which promotes the target's pointer-only
// methods into the value method set). A generic or builtin embed contributes
// nothing.
func (a *ProjectAnalyzer) embedMethodSet(f *ast.Field, c resolveCtx, open []string) embedMethods {
	s := a.typeShape(f.Type)
	ptr := false
	if s.Kind == models.ShapePointer && s.Elem != nil {
		s, ptr = *s.Elem, true
	}
	em := embedMethods{via: s.Name, ptr: ptr}
	switch {
	case s.Kind != models.ShapeNamed:
		em.set = methodSet{}
	case strings.Contains(s.Name, "."):
		em.set = a.qualifiedMethodSet(s.Name, c, true, open)
	default:
		em.set = a.methodSetOf(s.Name, c, true, open)
	}
	return em
}
