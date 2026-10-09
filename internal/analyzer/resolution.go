package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Named type resolution (CONTEXT.md, "Resolution"; docs/adr/0002). Each field
// is resolved once, at extraction, in the file that declares its struct: every
// local named non-struct type is substituted by what it stands for, at every
// depth, and every struct leaf becomes a ShapeRef that registration stamps
// with its final component name.

// maxNamedResolutionDepth bounds how many named types may be open at once in
// one field's resolution: nine, the hops resolveTypeSpecChain's `depth > 8`
// cap allows. A deeper chain is cut like a recursion.
const maxNamedResolutionDepth = 9

// Field fallback warnings. Each is raised at most once per field declaration.
// Every one opens with the field's declared type, as written, then names the
// named type where its resolution stopped and why.
const (
	marshalerFieldWarning = "field %s at %s has type %s: %s has its own %s method, which encoding/json uses instead of its " +
		"underlying type — emitting an untyped schema ({}); parameters are typed by kind and unaffected"
	recursiveFieldWarning = "field %s at %s has type %s: %s contains itself — the recursion is cut to an untyped schema ({})"
	depthCapFieldWarning  = "field %s at %s has type %s: its named types nest more than %d deep, so %s is cut to an untyped " +
		"schema ({}) as if it were recursive"
	unresolvableFieldWarning = "field %s at %s has type %s: %s resolves to no schema (a third-party type, a well-known type " +
		"under an aliased import, a named type from another package, or a name with no declaration in its package) " +
		"— emitting an untyped object"
	definedOverQualifiedFieldWarning = "field %s at %s has type %s: %s is a defined type over %s, which resolves to no schema " +
		"(a defined type drops its target's methods, and a type from another package is not resolved) — emitting an untyped object"
	declsDisagreeFieldWarning = "field %s at %s has type %s: %s's build-tagged declarations disagree on shape (no common " +
		"container or scalar kind; build constraints are not evaluated) — emitting an untyped object"
	untypedBuiltinFieldWarning = "field %s at %s has type %s, which holds the builtin %s, a Go type with no JSON schema " +
		"— emitting an untyped object"
	unregisteredRefFieldWarning = "field %s at %s has type %s: struct %s could not be registered as a component " +
		"— emitting an untyped object"
	unknownFieldPosition = "unknown position"
)

type resolveMode int

const (
	identityMode   resolveMode = iota // methods count: a direct use, an alias, a nested element
	underlyingMode                    // a defined type's underlying: methods are dropped
)

type fallbackKind int

const (
	fallbackNone fallbackKind = iota
	fallbackMarshaler
	fallbackRecursive
	fallbackDepthCap
	fallbackUnresolvable
	fallbackDefinedOverQualified
	fallbackDeclsDisagree
	fallbackUntypedBuiltin
)

// fieldFallback is the first fallback a field's resolution met.
type fieldFallback struct {
	kind     fallbackKind
	typeName string // the named type where resolution stopped
	// detail is the Marshaler method (fallbackMarshaler), the qualified
	// underlying (fallbackDefinedOverQualified), or the builtin
	// (fallbackUntypedBuiltin).
	detail string
}

// resolveState is the state shared by one field's resolution.
type resolveState struct {
	param    bool               // path/query/header param: exempt from the Marshaler guard
	stack    []string           // local names being resolved
	first    fieldFallback      // the first fallback noted, which picks the warning
	refSites map[string]pkgFile // ShapeRef leaf name -> file it was written in (registered there), first wins
}

// note records fb unless an earlier fallback was already noted.
func (s *resolveState) note(fb fieldFallback) {
	if s.first.kind == fallbackNone {
		s.first = fb
	}
}

// cut reports whether name must not be opened, and why: fallbackRecursive
// when it is already being resolved, fallbackDepthCap when
// maxNamedResolutionDepth names are open, else fallbackNone. It is checked
// before pushing.
func (s *resolveState) cut(name string) fallbackKind {
	if slices.Contains(s.stack, name) {
		return fallbackRecursive
	}
	if len(s.stack) >= maxNamedResolutionDepth {
		return fallbackDepthCap
	}
	return fallbackNone
}

// fieldSite is what registration needs from extraction, keyed by the field's
// Resolution root pointer in ProjectAnalyzer.fieldSites.
type fieldSite struct {
	loc      string             // fieldLoc(field), for the field's warnings
	declared string             // the field's type as written, for the field's warnings
	refSites map[string]pkgFile // from resolveState.refSites
	first    fieldFallback      // from resolveState.first, warned once the field survives the merge
}

// resolveCtx is the file a type expression was written in plus the shared
// per-field resolution state.
type resolveCtx struct {
	file *ast.File
	path string
	// definer is the defined type whose underlying chain is being resolved,
	// so a chain that ends in an unresolvable qualified type names it rather
	// than an alias met on the way. resolveDecl sets it on entering a defined
	// declaration in identity mode, the only way into a noted underlying chain
	// (the quiet Marshaler run's notes are discarded); a stale value under a
	// composite is overwritten there before any underlying chain can note it.
	definer string
	st      *resolveState
}

// pkgFile is one parsed file of a package and its path.
type pkgFile struct {
	file *ast.File
	path string
}

// typeDecl is one declaration of a local named type and the file declaring it.
type typeDecl struct {
	spec *ast.TypeSpec
	file *ast.File
	path string
}

func newResolveCtx(file *ast.File, path string, param bool) resolveCtx {
	return resolveCtx{file: file, path: path, st: &resolveState{param: param, refSites: map[string]pkgFile{}}}
}

// in is c with the same state, resolving in another file.
func (c resolveCtx) in(file *ast.File, path string) resolveCtx {
	c.file, c.path = file, path
	return c
}

// quiet is c with a private copy of the state: what it notes and the ref
// sites it records are discarded. The stack is cloned so the quiet run's
// pushes cannot overwrite the caller's backing array. Today the isolation is
// unobservable: the Marshaler fallback is noted before the quiet run (and
// note keeps the first), and no consumer descends into a Marshaler's Elem. It
// is forward-looking for #111, which will read that Elem.
func (c resolveCtx) quiet() resolveCtx {
	c.st = &resolveState{param: c.st.param, stack: slices.Clone(c.st.stack), refSites: map[string]pkgFile{}}
	return c
}

// recordRef notes the file a ShapeRef leaf's name was written in, so the leaf
// registers against that file's imports and declarations.
func (c resolveCtx) recordRef(name string) {
	if _, seen := c.st.refSites[name]; !seen {
		c.st.refSites[name] = pkgFile{file: c.file, path: c.path}
	}
}

// resolveField stamps f's Resolution and records its site: where its ref
// leaves register and the fallback it noted. It raises no warning: a promoted
// field may yet be shadowed by mergeFieldsByPrecedence, so registerStructAt
// warns only for the fields that survive (warnResolvedField). A json:"-" body
// field is skipped.
func (a *ProjectAnalyzer) resolveField(f *models.FieldInfo, field *ast.Field, astFile *ast.File, filePath string) {
	if isJSONExcluded(f) {
		return
	}
	c := newResolveCtx(astFile, filePath, f.ParamType != "")
	r := a.resolveShape(f.Shape, c)
	f.Resolution = &r
	a.fieldSites[f.Resolution] = fieldSite{
		loc: a.fieldLoc(field), declared: types.ExprString(field.Type), refSites: c.st.refSites, first: c.st.first,
	}
}

// warnResolvedField raises a resolved field's one warning, from the site
// resolveField recorded: the uintptr one when any leaf is a uintptr (it
// outranks every fallback), else the first fallback's. A field with no site
// (json:"-", or hand-built) never warns.
func (a *ProjectAnalyzer) warnResolvedField(f *models.FieldInfo) {
	if f.Resolution == nil {
		return
	}
	site, ok := a.fieldSites[f.Resolution]
	if !ok {
		return
	}
	if holdsUintptr(f.Resolution) {
		a.warnUintptrField(f, site.loc)
		return
	}
	a.warnFieldFallback(f, &site)
}

// resolveShape resolves every named leaf of s. Container nodes are copied
// fresh (registration mutates leaves in place); a map's Key is kept as is.
func (a *ProjectAnalyzer) resolveShape(s models.TypeShape, c resolveCtx) models.TypeShape {
	switch s.Kind {
	case models.ShapePointer, models.ShapeArray, models.ShapeMap:
		s.Elem = a.resolveElem(s.Elem, c)
	case models.ShapeSlice:
		s.Elem = a.resolveSliceElem(s.Elem, c)
	case models.ShapeNamed:
		if strings.Contains(s.Name, ".") {
			return a.resolveQualified(s, c)
		}
		return a.resolveLocal(s, c, identityMode)
	case models.ShapePrimitive:
		noteUntypedBuiltin(s.Name, c)
	default:
		// ShapeUnknown: generics, anonymous structs, chan, func.
	}
	return s
}

// resolveElem resolves a container element into a freshly allocated node.
func (a *ProjectAnalyzer) resolveElem(e *models.TypeShape, c resolveCtx) *models.TypeShape {
	if e == nil {
		return nil
	}
	r := a.resolveShape(*e, c)
	return &r
}

// noteUntypedBuiltin notes a builtin with no JSON schema (error, complex64,
// complex128: models.UntypedBuiltinNames).
func noteUntypedBuiltin(name string, c resolveCtx) {
	if models.UntypedBuiltinNames[name] {
		c.st.note(fieldFallback{kind: fallbackUntypedBuiltin, typeName: name, detail: name})
	}
}

// resolveSliceElem resolves a slice element, applying the byte-slice rule
// (#97): a byte-kind element that is a Marshaler type only on the decode side
// keeps the slice a base64 []byte, unwarned.
func (a *ProjectAnalyzer) resolveSliceElem(e *models.TypeShape, c resolveCtx) *models.TypeShape {
	if e == nil {
		return nil
	}
	if e.Kind != models.ShapeNamed || strings.Contains(e.Name, ".") {
		return a.resolveElem(e, c)
	}
	snapshot := c.st.first
	r := a.resolveLocal(*e, c, identityMode)
	if r.Kind == models.ShapeMarshaler && isBytePrimitive(r.Elem) && !a.marshalerSet(r.Name, c).encode {
		c.st.first = snapshot
		b := *r.Elem
		return &b
	}
	return &r
}

// isBytePrimitive reports whether s is the builtin byte or uint8.
func isBytePrimitive(s *models.TypeShape) bool {
	return s != nil && s.Kind == models.ShapePrimitive && (s.Name == goTypeByte || s.Name == goTypeUint8)
}

// resolveQualified resolves a qualified leaf (q.T): an in-module struct is a
// ref, a kind-backed stdlib type its builtin, a well-known type stays as is,
// and anything else is unresolvable (it emits object).
func (a *ProjectAnalyzer) resolveQualified(leaf models.TypeShape, c resolveCtx) models.TypeShape {
	if _, ok := a.resolveQualifiedStruct(leaf.Name, c.file); ok {
		c.recordRef(leaf.Name)
		return models.TypeShape{Kind: models.ShapeRef, Name: leaf.Name}
	}
	if b, ok := knownUnderlyingBuiltins[leaf.Name]; ok {
		return models.TypeShape{Kind: models.ShapePrimitive, Name: b}
	}
	if !models.WellKnownTypeNames[leaf.Name] {
		c.st.note(fieldFallback{kind: fallbackUnresolvable, typeName: leaf.Name})
	}
	return leaf
}

// resolveLocal resolves a local named type. A struct (or a chain ending at
// one) is a ref; a name already open, or one past the depth cap, is cut; in
// identity mode a non-param
// Marshaler type is a Marshaler leaf over its (quietly resolved) underlying;
// anything else resolves through its declarations.
func (a *ProjectAnalyzer) resolveLocal(leaf models.TypeShape, c resolveCtx, mode resolveMode) models.TypeShape {
	name := leaf.Name
	if _, _, _, _, ok := a.resolveTypeSpecChain(c.file, c.path, name, 0); ok {
		c.recordRef(name)
		return models.TypeShape{Kind: models.ShapeRef, Name: name}
	}
	if why := c.st.cut(name); why != fallbackNone {
		c.st.note(fieldFallback{kind: why, typeName: name})
		return models.TypeShape{Kind: models.ShapeRecursive, Name: name}
	}
	decls := a.localTypeDecls(name, c.file, c.path)
	if len(decls) == 0 {
		c.st.note(fieldFallback{kind: fallbackUnresolvable, typeName: name})
		return leaf
	}
	if mode == identityMode && !c.st.param {
		if m := a.marshalerSet(name, c); m.found() {
			c.st.note(fieldFallback{kind: fallbackMarshaler, typeName: name, detail: m.first})
			under := a.resolveDecls(name, decls, c.quiet(), underlyingMode)
			return models.TypeShape{Kind: models.ShapeMarshaler, Name: name, Elem: &under}
		}
	}
	return a.resolveDecls(name, decls, c, mode)
}

// resolveDecls resolves every declaration of name with name open, then merges
// the build-tagged variants: leaf by leaf, else the first scalar's kind, else
// name is unresolvable (a lone declaration that resolves always merges).
func (a *ProjectAnalyzer) resolveDecls(name string, decls []typeDecl, c resolveCtx, mode resolveMode) models.TypeShape {
	c.st.stack = append(c.st.stack, name)
	defer func() { c.st.stack = c.st.stack[:len(c.st.stack)-1] }()
	results := make([]models.TypeShape, 0, len(decls))
	for _, d := range decls {
		results = append(results, a.resolveDecl(name, d, c, mode))
	}
	if merged, ok := mergeDecls(results); ok {
		return merged
	}
	if kind := firstScalarKind(results); kind != "" {
		return models.TypeShape{Kind: models.ShapeKindOnly, Name: kind}
	}
	c.st.note(fieldFallback{kind: fallbackDeclsDisagree, typeName: name})
	return models.TypeShape{Kind: models.ShapeNamed, Name: name}
}

// resolveDecl resolves one declaration in its own file. In identity mode an
// alias is its target, methods included; a defined type (or any declaration
// in underlying mode) is its target's underlying type, and a defined type
// met in identity mode opens a new underlying chain it is the definer of.
func (a *ProjectAnalyzer) resolveDecl(name string, d typeDecl, c resolveCtx, mode resolveMode) models.TypeShape {
	dc := c.in(d.file, d.path)
	if mode == identityMode {
		if d.spec.Assign.IsValid() {
			return a.resolveShape(a.typeShape(d.spec.Type), dc)
		}
		dc.definer = name
	}
	return a.resolveUnderlying(name, d.spec.Type, dc)
}

// resolveUnderlying resolves the underlying type of name's declaration rhs:
// a builtin or composite as written (nested leaves keep identity mode), a
// local name by its own underlying (dropping its methods), and a qualified
// name only when kind-backed — any other qualified underlying (time.Time,
// uuid.UUID, decimal.Decimal) leaves name unresolvable, noted against the
// chain's definer (name itself when resolution started in underlying mode).
func (a *ProjectAnalyzer) resolveUnderlying(name string, rhs ast.Expr, c resolveCtx) models.TypeShape {
	s := a.typeShape(rhs)
	if s.Kind != models.ShapeNamed {
		return a.resolveShape(s, c)
	}
	if !strings.Contains(s.Name, ".") {
		return a.resolveLocal(s, c, underlyingMode)
	}
	if u, ok := kindBackedUnderlying(s.Name); ok {
		return u
	}
	definer := c.definer
	if definer == "" {
		definer = name
	}
	c.st.note(fieldFallback{kind: fallbackDefinedOverQualified, typeName: definer, detail: s.Name})
	return models.TypeShape{Kind: models.ShapeNamed, Name: name}
}

// kindBackedUnderlying returns the underlying type of a qualified well-known
// type a defined chain may end in: the knownUnderlyingBuiltins, plus
// json.RawMessage ([]byte) and json.Number (string). The last two apply only
// at the end of a defined chain; used directly they stay well-known leaves.
func kindBackedUnderlying(qualified string) (models.TypeShape, bool) {
	if b, ok := knownUnderlyingBuiltins[qualified]; ok {
		return models.TypeShape{Kind: models.ShapePrimitive, Name: b}, true
	}
	switch qualified {
	case models.WellKnownRawMessage:
		elem := models.TypeShape{Kind: models.ShapePrimitive, Name: goTypeByte}
		return models.TypeShape{Kind: models.ShapeSlice, Elem: &elem}, true
	case models.WellKnownJSONNumber:
		return models.TypeShape{Kind: models.ShapePrimitive, Name: goTypeString}, true
	}
	return models.TypeShape{}, false
}

// mergeDecls folds the resolutions of a type's declarations pairwise.
func mergeDecls(results []models.TypeShape) (models.TypeShape, bool) {
	merged := results[0]
	for _, r := range results[1:] {
		var ok bool
		if merged, ok = mergeShapes(merged, r); !ok {
			return models.TypeShape{}, false
		}
	}
	return merged, true
}

// mergeShapes merges two declarations' resolutions: containers of one kind
// merge their elements, equal leaves are kept, and scalar leaves of one
// OpenAPI kind merge to that kind alone (#92).
func mergeShapes(x, y models.TypeShape) (models.TypeShape, bool) {
	if x.Kind == y.Kind {
		if merged, ok := mergeSameKind(x, y); ok {
			return merged, true
		}
	}
	if kind := leafKind(x); kind != "" && kind == leafKind(y) {
		return models.TypeShape{Kind: models.ShapeKindOnly, Name: kind}, true
	}
	return models.TypeShape{}, false
}

// mergeSameKind merges two shapes of one kind: containers by their elements,
// primitives when they are one builtin, other leaves when they are one name.
func mergeSameKind(x, y models.TypeShape) (models.TypeShape, bool) {
	switch x.Kind {
	case models.ShapePointer, models.ShapeSlice, models.ShapeArray, models.ShapeMap:
		return mergeElems(x, y)
	case models.ShapePrimitive:
		return x, sameBuiltin(x.Name, y.Name)
	default:
		return x, x.Name == y.Name
	}
}

// mergeElems merges the elements of two containers of one kind.
func mergeElems(x, y models.TypeShape) (models.TypeShape, bool) {
	if x.Elem == nil || y.Elem == nil {
		return x, x.Elem == nil && y.Elem == nil
	}
	e, ok := mergeShapes(*x.Elem, *y.Elem)
	if !ok {
		return models.TypeShape{}, false
	}
	x.Elem = &e
	return x, true
}

// leafKind is the OpenAPI kind of a scalar leaf: primitiveKind of a
// primitive, the Name of a kind-only leaf, else "".
func leafKind(s models.TypeShape) string {
	switch s.Kind {
	case models.ShapePrimitive:
		return primitiveKind(s.Name)
	case models.ShapeKindOnly:
		return s.Name
	default:
		return ""
	}
}

// firstScalarKind is the kind of the first declaration that is a scalar leaf.
func firstScalarKind(results []models.TypeShape) string {
	for _, r := range results {
		if kind := leafKind(r); kind != "" {
			return kind
		}
	}
	return ""
}

// samePackageFiles returns file first, then every other file of its package
// in its directory in sorted path order. Files of another package in the
// directory (a //go:build ignore generator) are skipped, and an unreadable
// directory yields just file.
func (a *ProjectAnalyzer) samePackageFiles(file *ast.File, path string) []pkgFile {
	out := []pkgFile{{file: file, path: path}}
	files, err := a.parsePackageDir(filepath.Dir(path))
	if err != nil {
		return out
	}
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if p == path || files[p].Name.Name != file.Name.Name {
			continue
		}
		out = append(out, pkgFile{file: files[p], path: p})
	}
	return out
}

// localTypeDecls returns every declaration of name in file's package, in
// samePackageFiles order. Build constraints are not evaluated, so a type
// declared in several build-tagged files has one entry per file.
func (a *ProjectAnalyzer) localTypeDecls(name string, file *ast.File, path string) []typeDecl {
	var decls []typeDecl
	for _, pf := range a.samePackageFiles(file, path) {
		if ts := typeSpecInFile(pf.file, name); ts != nil {
			decls = append(decls, typeDecl{spec: ts, file: pf.file, path: pf.path})
		}
	}
	return decls
}

// walkResolvedLeaves calls fn on every leaf of s, descending through pointer,
// slice, array and map elements only: never into a map's Key, and never into
// a Marshaler leaf's Elem (nothing under it is emitted or registered).
func walkResolvedLeaves(s *models.TypeShape, fn func(leaf *models.TypeShape)) {
	if s == nil {
		return
	}
	switch s.Kind {
	case models.ShapePointer, models.ShapeSlice, models.ShapeArray, models.ShapeMap:
		walkResolvedLeaves(s.Elem, fn)
	default:
		fn(s)
	}
}

// holdsUintptr reports whether any emitted leaf of s is the builtin uintptr.
func holdsUintptr(s *models.TypeShape) bool {
	found := false
	walkResolvedLeaves(s, func(leaf *models.TypeShape) {
		found = found || (leaf.Kind == models.ShapePrimitive && leaf.Name == goTypeUintptr)
	})
	return found
}

// fieldLoc renders a field's position as a project-relative file:line:col.
func (a *ProjectAnalyzer) fieldLoc(field *ast.Field) string {
	pos := a.fileSet.Position(field.Pos())
	return fmt.Sprintf("%s:%d:%d", relToRoot(a.projectRoot, pos.Filename), pos.Line, pos.Column)
}

// markOnce reports whether loc is new to seen, recording it.
func markOnce(seen map[string]struct{}, loc string) bool {
	if _, dup := seen[loc]; dup {
		return false
	}
	seen[loc] = struct{}{}
	return true
}

// warnFieldFallback raises the warning for the first fallback noted in a
// field's site, once per field declaration. The warning names the field's
// declared type, then the named type where resolution stopped.
func (a *ProjectAnalyzer) warnFieldFallback(f *models.FieldInfo, site *fieldSite) {
	fb := site.first
	if fb.kind == fallbackNone {
		return
	}
	if !markOnce(a.fieldWarned, site.loc) {
		return
	}
	head := []any{f.Name, site.loc, site.declared}
	switch fb.kind {
	case fallbackMarshaler:
		a.addWarningf(marshalerFieldWarning, append(head, fb.typeName, fb.detail)...)
	case fallbackRecursive:
		a.addWarningf(recursiveFieldWarning, append(head, fb.typeName)...)
	case fallbackDepthCap:
		a.addWarningf(depthCapFieldWarning, append(head, maxNamedResolutionDepth, fb.typeName)...)
	case fallbackUnresolvable:
		a.addWarningf(unresolvableFieldWarning, append(head, fb.typeName)...)
	case fallbackDefinedOverQualified:
		a.addWarningf(definedOverQualifiedFieldWarning, append(head, fb.typeName, fb.detail)...)
	case fallbackDeclsDisagree:
		a.addWarningf(declsDisagreeFieldWarning, append(head, fb.typeName)...)
	default:
		a.addWarningf(untypedBuiltinFieldWarning, append(head, fb.detail)...)
	}
}

// registerRefLeaf registers one ShapeRef leaf in the file its name was
// written in, stamping the final component name. A leaf that cannot register
// is demoted to a named leaf (it emits object) and warns, unless the depth cap
// truncated it, which raised its own warning.
func (a *ProjectAnalyzer) registerRefLeaf(leaf *models.TypeShape, f *models.FieldInfo, site pkgFile, depth int) {
	if reg := a.registerTypeAt(leaf.Name, site.file.Name.Name, site.file, site.path, depth); reg != nil {
		leaf.Name = reg.Name
		return
	}
	leaf.Kind = models.ShapeNamed
	if depth > maxTypeRegistrationDepth {
		return
	}
	fs := a.fieldSites[f.Resolution]
	if fs.loc == "" { // defensive: every extracted field has a site; only a hand-built FieldInfo lacks one
		a.addWarningf(unregisteredRefFieldWarning, f.Name, unknownFieldPosition, leaf.Name, leaf.Name)
		return
	}
	if markOnce(a.fieldWarned, fs.loc) {
		a.addWarningf(unregisteredRefFieldWarning, f.Name, fs.loc, fs.declared, leaf.Name)
	}
}
