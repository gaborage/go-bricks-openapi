package analyzer

import (
	"cmp"
	"go/ast"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Instantiation substitution (issue 114; CONTEXT.md, "Instantiation"; ADR
// 0003). An Instantiation's type arguments are bound to its generic
// declaration's type parameters (a typeEnv), and the declaration is resolved
// with every parameter replaced at every depth. Each argument resolves in the
// file it was written in, with the bindings in effect there; the
// declaration resolves in its own package. A generic struct becomes one
// component per distinct Instantiation, named at registration; any other
// generic type is inlined, as every named composite is (ADR 0002).

// typeEnv binds the type parameters of one generic declaration, by name, to
// the arguments of one Instantiation of it.
type typeEnv map[string]boundArg

// boundArg is one type argument: its Shape as written, the file it was
// written in, and the bindings in effect there (non-nil when it was written
// inside another generic declaration, Page[T]'s Items []Box[T]).
type boundArg struct {
	shape models.TypeShape
	site  pkgFile
	env   typeEnv
}

// genericDecl is the declaration of an Instantiation's generic type and the
// file declaring it.
type genericDecl struct {
	spec *ast.TypeSpec
	site pkgFile
}

// instance is one distinct struct Instantiation, recorded at resolution and
// registered as a component the first time a ref to it registers.
type instance struct {
	decl     genericDecl
	st       *ast.StructType
	env      typeEnv
	written  string // the Instantiation as first written, for warnings
	identity string // instIdentity: what two Instantiations share when they are one type
	ti       *models.TypeInfo
}

// instRefPrefix opens the provisional Name of a ShapeRef leaf to an
// instance, which registerLeafAt replaces with the final component name. It
// holds a NUL byte, so it is never a Go type name.
const instRefPrefix = "inst\x00"

// instKeyDir stands in for a declaring directory in an instance's schemaKey,
// so its name assignment is keyed by its identity.
const instKeyDir = "\x00inst\x00"

// anyArgName spells the empty interface as a type argument: any and
// interface{} are one type.
const anyArgName = "any"

// instCutDetail is the fieldFallback.detail of a recursion or depth-cap cut
// whose cut is an Instantiation.
const instCutDetail = "instantiation"

// identityCap stands for the part of an argument's identity past
// maxNamedResolutionDepth named steps; an Instantiation whose identity holds
// it is cut (instCut), never shared.
const identityCap = "\x00cap"

// resolveNamed resolves a ShapeNamed leaf: a type parameter bound in c's
// declaration as its argument, a qualified name as resolveQualified does,
// and any other name as a local type.
func (a *ProjectAnalyzer) resolveNamed(s models.TypeShape, c resolveCtx) models.TypeShape {
	if strings.Contains(s.Name, ".") {
		return a.resolveQualified(s, c)
	}
	if arg, bound := c.env[s.Name]; bound {
		return a.resolveShape(arg.shape, arg.ctx(c))
	}
	return a.resolveLocal(s, c, identityMode)
}

// ctx is c resolving in the file arg was written in, with the bindings in
// effect there.
func (arg *boundArg) ctx(c resolveCtx) resolveCtx {
	ac := c.in(arg.site.file, arg.site.path)
	ac.env = arg.env
	return ac
}

// resolveInstantiation resolves an Instantiation written in c's file. A
// generic type the tool cannot read (out of the module, undeclared, or with
// another number of type parameters) is noted and left as written, so it
// keeps issue 138's warning. A generic type whose method set holds one of the
// seven marshal methods, on a generic receiver or promoted, is a text or
// Marshaler leaf, outside a parameter. A generic struct is a ref to its
// instance, registered later, unless an argument is an anonymous struct, in
// which case the whole Instantiation is inlined (ADR 0003). Any other generic
// type resolves through its declaration with its parameters bound: an alias
// as its target, a defined type as its underlying type. An Instantiation met
// again inside its own resolution is cut like a recursion. One whose
// arguments hold a func or chan is refused (funcChanArg).
func (a *ProjectAnalyzer) resolveInstantiation(s models.TypeShape, c resolveCtx) models.TypeShape {
	return a.resolveInstantiationAs(s, c, true)
}

// underlyingInstantiation resolves an Instantiation that is a defined type's
// underlying type (type DM MJ[int], type GM[T any] MJ[T]). A defined type
// drops its underlying type's methods, so the generic type's marshal methods
// are not read at this top level; a field or element inside it keeps them.
func (a *ProjectAnalyzer) underlyingInstantiation(s models.TypeShape, c resolveCtx) models.TypeShape {
	return a.resolveInstantiationAs(s, c, false)
}

// resolveInstantiationAs is resolveInstantiation, reading the generic type's
// marshal methods only when methods is set.
func (a *ProjectAnalyzer) resolveInstantiationAs(s models.TypeShape, c resolveCtx, methods bool) models.TypeShape {
	g, ok := a.genericDeclOf(s.Inst, c)
	if !ok {
		noteInstantiation(s.Inst, c)
		return s
	}
	env := g.bind(s.Inst.Args, c)
	key := a.instIdentity(g, env, 0)
	if cut, isCut := instCut(key, s.Inst.Written, c); isCut {
		return cut
	}
	if methods {
		if r, isMarshaler := a.genericMarshalerLeaf(g, env, key, s.Inst.Written, c); isMarshaler {
			return r
		}
	}
	if kind := funcChanArg(g, env); kind != "" {
		c.st.note(fieldFallback{kind: fallbackFuncChan, typeName: s.Inst.Written, detail: kind})
		return models.TypeShape{Kind: models.ShapeUnknown, Name: kind}
	}
	return a.expandInstantiation(g, env, key, s.Inst.Written, c, methods)
}

// instCut cuts the Instantiation identified by key, written as written,
// like a recursion when it is already open in c, and like the depth cap when
// its identity was capped (identityCap): two capped identities may be two
// different types, so neither is documented as the other.
func instCut(key, written string, c resolveCtx) (models.TypeShape, bool) {
	why := c.st.cut(key)
	if why == fallbackNone && strings.Contains(key, identityCap) {
		why = fallbackDepthCap
	}
	if why == fallbackNone {
		return models.TypeShape{}, false
	}
	shown := cmp.Or(c.st.openInsts[key], written)
	c.st.note(fieldFallback{kind: why, typeName: shown, detail: instCutDetail})
	return models.TypeShape{Kind: models.ShapeRecursive, Name: shown}, true
}

// expandInstantiation resolves g's declaration under env: a generic struct
// as structInstantiation does, an alias as its target and a defined type as
// its underlying type. Without methods (underlyingInstantiation) an alias's
// target is an underlying type too: type X[T any] = MJ[T]; type DX X[int]
// drops MJ's methods.
func (a *ProjectAnalyzer) expandInstantiation(g genericDecl, env typeEnv, key, written string, c resolveCtx, methods bool) models.TypeShape {
	dc := c.in(g.site.file, g.site.path)
	dc.env = env
	if st, isStruct := ast.Unparen(g.spec.Type).(*ast.StructType); isStruct {
		return a.structInstantiation(st, g, env, key, written, dc)
	}
	c.st.open(key, written)
	defer c.st.close(key)
	if g.spec.Assign.IsValid() && methods {
		return a.resolveShape(a.typeShape(g.spec.Type), dc)
	}
	dc.definer = written
	return a.resolveUnderlying(written, g.spec.Type, dc)
}

// funcChanArg returns func or chan when an argument env binds holds one at
// any depth (Box[chan int], Box[[]func()], Page[Box[chan int]]), in g's
// parameter order, else "". encoding/json cannot marshal a func or chan
// (UnsupportedTypeError), so such an Instantiation is refused: no component,
// and the func or chan warning, as a field of that type gets.
func funcChanArg(g genericDecl, env typeEnv) string {
	for _, name := range typeParamNames(g.spec) {
		arg := env[name]
		if kind := holdsFuncChan(boundShape(&arg)); kind != "" {
			return kind
		}
	}
	return ""
}

// holdsFuncChan returns func or chan when s holds one at any depth: through
// pointers, slices, arrays, maps and Instantiation arguments, as written.
func holdsFuncChan(s models.TypeShape) string {
	if s.Kind == models.ShapeUnknown && (s.Name == models.UnknownFunc || s.Name == models.UnknownChan) {
		return s.Name
	}
	var inner []models.TypeShape
	for _, e := range []*models.TypeShape{s.Elem, s.Key} {
		if e != nil {
			inner = append(inner, *e)
		}
	}
	if s.Inst != nil {
		inner = append(inner, s.Inst.Args...)
	}
	for _, e := range inner {
		if kind := holdsFuncChan(e); kind != "" {
			return kind
		}
	}
	return ""
}

// open pushes key, an Instantiation's identity, on the resolution stack,
// recording written, the Instantiation as written there, which a recursion
// cut at key names: Tree[int], not the Tree[T] its declaration writes.
func (s *resolveState) open(key, written string) {
	s.stack = append(s.stack, key)
	if s.openInsts == nil {
		s.openInsts = map[string]string{}
	}
	s.openInsts[key] = written
}

// close pops key, which open pushed last.
func (s *resolveState) close(key string) {
	s.stack = s.stack[:len(s.stack)-1]
	delete(s.openInsts, key)
}

// genericDeclOf finds the declaration of inst's generic type as written in
// c's file, local or in another package of the module, with as many type
// parameters as inst has arguments.
func (a *ProjectAnalyzer) genericDeclOf(inst *models.Instantiation, c resolveCtx) (genericDecl, bool) {
	if inst == nil {
		return genericDecl{}, false
	}
	ts, at, ok := a.typeSpecOf(inst.Base, pkgFile{file: c.file, path: c.path})
	if !ok || len(typeParamNames(ts)) != len(inst.Args) || len(inst.Args) == 0 {
		return genericDecl{}, false
	}
	return genericDecl{spec: ts, site: at}, true
}

// typeParamNames returns ts's type parameter names, in order.
func typeParamNames(ts *ast.TypeSpec) []string {
	if ts.TypeParams == nil {
		return nil
	}
	var names []string
	for _, f := range ts.TypeParams.List {
		for _, n := range f.Names {
			names = append(names, n.Name)
		}
	}
	return names
}

// bind binds g's type parameters to args, written in c's file under c's
// bindings.
func (g genericDecl) bind(args []models.TypeShape, c resolveCtx) typeEnv {
	env := typeEnv{}
	for i, name := range typeParamNames(g.spec) {
		env[name] = boundArg{shape: args[i], site: pkgFile{file: c.file, path: c.path}, env: c.env}
	}
	return env
}

// genericMarshalerLeaf resolves an Instantiation of g, in a body position,
// whose method set (methods declared on its generic receiver, and those its
// struct promotes) holds one of the seven marshal methods: a text leaf when
// it is a string both ways, else a noted Marshaler leaf, named as written. A
// generic type takes no Stated schema (#117 refuses the directive on one),
// so the leaf carries none.
func (a *ProjectAnalyzer) genericMarshalerLeaf(g genericDecl, env typeEnv, key, written string, c resolveCtx) (models.TypeShape, bool) {
	if c.st.param {
		return models.TypeShape{}, false
	}
	dc := c.in(g.site.file, g.site.path)
	m, fb := a.methodSetOf(g.spec.Name.Name, dc, true, nil).marshaler(dc.display(g.spec.Name.Name))
	if !m.found() {
		return models.TypeShape{}, false
	}
	// As marshalerLeaf: a byte-kind type with only decode-side methods is a
	// plain byte as a slice element, so the slice stays base64 (#97).
	if c.byteElem && !m.encode {
		if under := a.expandInstantiation(g, env, key, written, c.quiet(), false); isBytePrimitive(&under) {
			return under, true
		}
	}
	return methodSetLeaf(m, fb, written, nil, c), true
}

// structInstantiation resolves an Instantiation of a generic struct: inline,
// its fields extracted under env, when an argument is an anonymous struct,
// which has no name to spell; else a ref to its instance, recorded once per
// identity.
func (a *ProjectAnalyzer) structInstantiation(st *ast.StructType, g genericDecl, env typeEnv, key, written string, dc resolveCtx) models.TypeShape {
	if holdsAnonymousArg(env) {
		return a.resolveStruct(models.TypeShape{Kind: models.ShapeStruct, Struct: &models.AnonymousStruct{Literal: st}}, dc)
	}
	ref := instRefPrefix + key
	if _, seen := a.instances[ref]; !seen {
		a.instances[ref] = &instance{decl: g, st: st, env: env, written: written, identity: key}
	}
	return models.TypeShape{Kind: models.ShapeRef, Name: ref}
}

// holdsAnonymousArg reports whether any argument env binds is an anonymous
// struct, through the bindings it was written under.
func holdsAnonymousArg(env typeEnv) bool {
	for _, arg := range env {
		if boundShape(&arg).Kind == models.ShapeStruct {
			return true
		}
	}
	return false
}

// boundShape follows arg through the type parameters it names to the Shape
// written for it.
func boundShape(arg *boundArg) models.TypeShape {
	for range maxNamedResolutionDepth {
		next, bound := arg.env[arg.shape.Name]
		if arg.shape.Kind != models.ShapeNamed || !bound {
			break
		}
		arg = &next
	}
	return arg.shape
}

// instIdentity identifies an Instantiation of g under env: its generic
// type's declaration and each argument's identity (argIdentity), so two
// spellings of one Go type are one Instantiation.
func (a *ProjectAnalyzer) instIdentity(g genericDecl, env typeEnv, depth int) string {
	parts := make([]string, 0, len(env))
	for _, name := range typeParamNames(g.spec) {
		arg := env[name]
		parts = append(parts, a.argIdentity(arg.shape, arg.site, arg.env, depth+1))
	}
	return typeKeyOf(g.site.path, g.spec.Name.Name) + "[" + strings.Join(parts, ",") + "]"
}

// argIdentity identifies a type argument s, written in at under env, by
// what it is rather than how it is spelled: a builtin by its name (any for
// interface{}), a composite by its kind and element, a type parameter as its
// argument, an alias as its target, an in-module named type by its
// declaring directory and name, an out-of-module one by its import path and
// name, and an Instantiation by its own identity.
func (a *ProjectAnalyzer) argIdentity(s models.TypeShape, at pkgFile, env typeEnv, depth int) string {
	if depth > maxNamedResolutionDepth {
		return identityCap
	}
	// A composite level is finite syntax, so it spends no depth: only named
	// steps (a parameter, an alias, an Instantiation) can loop.
	elem := func(e *models.TypeShape) string {
		if e == nil {
			return ""
		}
		return a.argIdentity(*e, at, env, depth)
	}
	switch s.Kind {
	case models.ShapePrimitive:
		return primitiveArgName(s.Name)
	case models.ShapePointer:
		return "*" + elem(s.Elem)
	case models.ShapeSlice:
		return "[]" + elem(s.Elem)
	case models.ShapeArray:
		return "[N]" + elem(s.Elem)
	case models.ShapeMap:
		return "map[" + elem(s.Key) + "]" + elem(s.Elem)
	case models.ShapeNamed:
		return a.namedIdentity(s.Name, at, env, depth)
	case models.ShapeInstantiation:
		c := newResolveCtx(at.file, at.path, false)
		c.env = env
		if g, ok := a.genericDeclOf(s.Inst, c); ok {
			return a.boundInstIdentity(g, g.bind(s.Inst.Args, c), depth+1)
		}
		return filepath.Dir(at.path) + "\x00" + s.Inst.Written
	case models.ShapeStruct:
		if s.Struct != nil && s.Struct.Literal != nil {
			return "struct@" + a.structIdentity(s.Struct.Literal)
		}
	default:
	}
	return string(s.Kind) + ":" + s.Name
}

// boundInstIdentity is instIdentity for an argument's Instantiation of g
// under env, except that a generic alias (type Alias[T any] = Box[T]) is
// identified by its target under env, since Go treats Box[Alias[int]] and
// Box[Box[int]] as one type.
func (a *ProjectAnalyzer) boundInstIdentity(g genericDecl, env typeEnv, depth int) string {
	if g.spec.Assign.IsValid() {
		return a.argIdentity(a.typeShape(g.spec.Type), g.site, env, depth)
	}
	return a.instIdentity(g, env, depth)
}

// envIdentity identifies the bindings env holds, by each parameter's name
// and its argument's identity, in name order; "" for no bindings.
func (a *ProjectAnalyzer) envIdentity(env typeEnv) string {
	names := slices.Sorted(maps.Keys(env))
	parts := make([]string, 0, len(names))
	for _, name := range names {
		arg := env[name]
		parts = append(parts, name+"="+a.argIdentity(arg.shape, arg.site, arg.env, 0))
	}
	return strings.Join(parts, ",")
}

// primitiveArgName spells a builtin type argument: interface{} as any, the
// one type both name.
func primitiveArgName(name string) string {
	if name == goTypeInterface {
		return anyArgName
	}
	return name
}

// namedIdentity is argIdentity for a named type written in at under env.
func (a *ProjectAnalyzer) namedIdentity(name string, at pkgFile, env typeEnv, depth int) string {
	if arg, bound := env[name]; bound && !strings.Contains(name, ".") {
		return a.argIdentity(arg.shape, arg.site, arg.env, depth+1)
	}
	if target, site, ok := a.aliasTarget(name, at); ok {
		return a.argIdentity(target, site, nil, depth+1)
	}
	if qual, local, qualified := strings.Cut(name, "."); qualified {
		if site, n, ok := a.inModuleTypeSite(name, at.file); ok {
			return typeKeyOf(site.path, n)
		}
		if path, ok := a.fileImports(at.file)[qual]; ok {
			return path + "." + local
		}
		return name
	}
	return typeKeyOf(at.path, name)
}

// aliasTarget returns the type a non-generic alias name, as written in at,
// stands for and the file declaring it, local or in another package of the
// module. ok is false for any other name.
func (a *ProjectAnalyzer) aliasTarget(name string, at pkgFile) (models.TypeShape, pkgFile, bool) {
	ts, site, ok := a.typeSpecOf(name, at)
	if !ok || !ts.Assign.IsValid() || ts.TypeParams != nil {
		return models.TypeShape{}, pkgFile{}, false
	}
	return a.typeShape(ts.Type), site, true
}

// registerInstantiation registers inst as a component, once, and returns
// its TypeInfo: its name is spelled first (instanceName), which registers
// every argument that has a component of its own, then its fields are
// extracted under its bindings. nil past maxTypeRegistrationDepth.
func (a *ProjectAnalyzer) registerInstantiation(inst *instance, depth int) *models.TypeInfo {
	if inst.ti != nil {
		return inst.ti
	}
	if depth > maxTypeRegistrationDepth {
		a.warnDepthExceeded()
		return nil
	}
	name := a.instanceName(inst, depth)
	if inst.ti != nil { // spelling an argument registered it (Address{ Owner *Box[Address] })
		return inst.ti
	}
	pkg := inst.decl.site.file.Name.Name
	key := a.schemaKey(name, pkg, instKeyDir+inst.identity)
	ti := &models.TypeInfo{Name: key, Package: pkg}
	inst.ti = ti
	a.typeRegistry[key] = ti
	a.fillStruct(ti, inst.st, inst.decl.site, inst.env, depth)
	return ti
}

// instanceName spells inst's component name (ADR 0003): its generic type's
// name, package-qualified when another package's generic type of that name
// came first (genericBaseName), then each argument (spellArg), joined by -.
func (a *ProjectAnalyzer) instanceName(inst *instance, depth int) string {
	params := typeParamNames(inst.decl.spec)
	parts := make([]string, 0, len(params)+1)
	parts = append(parts, a.genericBaseName(inst.decl))
	for _, name := range params {
		arg := inst.env[name]
		parts = append(parts, a.spellArg(arg.shape, arg.site, arg.env, depth))
	}
	return strings.Join(parts, "-")
}

// genericBaseName is the name a generic type takes in its Instantiations'
// component names: its own name, or, when a generic type of that name in
// another directory took it first, the name qualified by its package
// (exportedPkgName), then numbered, as schemaKey qualifies a struct's.
func (a *ProjectAnalyzer) genericBaseName(g genericDecl) string {
	name, pkg := g.spec.Name.Name, g.site.file.Name.Name
	k := filepath.Dir(g.site.path) + "\x00" + name
	if final, ok := a.baseAssign[k]; ok {
		return final
	}
	final := name
	if a.baseUsed[final] {
		final = exportedPkgName(pkg) + name
		for i := 2; a.baseUsed[final]; i++ {
			final = exportedPkgName(pkg) + name + strconv.Itoa(i)
		}
	}
	a.baseUsed[final] = true
	a.baseAssign[k] = final
	return final
}

// spellArg spells a type argument s, written in at under env, in a
// component name (ADR 0003): a builtin by its name (any for interface{}),
// a pointer, slice, array or map as ptr-, slice-, array- or map-K- before
// its element, a type parameter as its argument, an alias as its target, a
// named type with a component of its own by that component's final name,
// any other named type as package.Name, the package named from its import
// path, and an Instantiation by its own name.
func (a *ProjectAnalyzer) spellArg(s models.TypeShape, at pkgFile, env typeEnv, depth int) string {
	elem := func(e *models.TypeShape) string {
		if e == nil {
			return ""
		}
		return a.spellArg(*e, at, env, depth)
	}
	switch s.Kind {
	case models.ShapePrimitive:
		return primitiveArgName(s.Name)
	case models.ShapePointer:
		return "ptr-" + elem(s.Elem)
	case models.ShapeSlice:
		return "slice-" + elem(s.Elem)
	case models.ShapeArray:
		return "array-" + elem(s.Elem)
	case models.ShapeMap:
		return "map-" + elem(s.Key) + "-" + elem(s.Elem)
	case models.ShapeNamed:
		return a.spellNamed(s.Name, at, env, depth)
	case models.ShapeInstantiation:
		return a.spellInstantiation(s, at, env, depth)
	default:
	}
	return string(s.Kind)
}

// spellNamed is spellArg for a named type written in at under env.
func (a *ProjectAnalyzer) spellNamed(name string, at pkgFile, env typeEnv, depth int) string {
	if arg, bound := env[name]; bound && !strings.Contains(name, ".") {
		return a.spellArg(arg.shape, arg.site, arg.env, depth)
	}
	if target, site, ok := a.aliasTarget(name, at); ok {
		return a.spellArg(target, site, nil, depth)
	}
	if final, ok := a.componentName(models.TypeShape{Kind: models.ShapeNamed, Name: name}, at, nil, depth); ok {
		return final
	}
	qual, local, qualified := strings.Cut(name, ".")
	if !qualified {
		return at.file.Name.Name + "." + name
	}
	if site, n, ok := a.inModuleTypeSite(name, at.file); ok {
		return site.file.Name.Name + "." + n
	}
	if path, ok := a.fileImports(at.file)[qual]; ok {
		return a.unaliasedImportName(path) + "." + local
	}
	return name
}

// spellInstantiation is spellArg for an Instantiation written in at under
// env: its component's final name when it has one, else its generic type's
// name and its arguments, joined by -.
func (a *ProjectAnalyzer) spellInstantiation(s models.TypeShape, at pkgFile, env typeEnv, depth int) string {
	if final, ok := a.componentName(s, at, env, depth); ok {
		return final
	}
	_, base, qualified := strings.Cut(s.Inst.Base, ".")
	if !qualified {
		base = s.Inst.Base
	}
	parts := []string{base}
	for i := range s.Inst.Args {
		parts = append(parts, a.spellArg(s.Inst.Args[i], at, env, depth))
	}
	return strings.Join(parts, "-")
}

// componentName resolves s, written in at under env, as a body field's type
// is, and, when it is a ref to a component (a struct, or a struct
// Instantiation), registers that component and returns its final name.
func (a *ProjectAnalyzer) componentName(s models.TypeShape, at pkgFile, env typeEnv, depth int) (string, bool) {
	c := newResolveCtx(at.file, at.path, false)
	c.env = env
	r := a.resolveShape(s, c)
	if r.Kind != models.ShapeRef {
		return "", false
	}
	site, ok := c.st.refSites[r.Name]
	if !ok {
		site = at
	}
	if !a.registerLeafAt(&r, site, depth+1) {
		return "", false
	}
	return r.Name, true
}

// fillStruct extracts a struct component's fields, in the file declaring
// it, under env, the bindings of the Instantiation it documents (nil for a
// non-generic struct), then warns about them and registers what they
// reference, one level deeper. ti is already in the registry (the cycle
// guard).
func (a *ProjectAnalyzer) fillStruct(ti *models.TypeInfo, st *ast.StructType, site pkgFile, env typeEnv, depth int) {
	// Seed the embedded-promotion ancestor set with this struct so a
	// self-embed (type Node struct{ *Node }, or *NA for type NA = Node) is
	// not promoted into itself.
	ti.Fields = a.extractUnder(env, func() []models.FieldInfo {
		return a.extractStructFields(st, site.file, site.path, map[string]struct{}{a.structIdentity(st): {}}, depth)
	})
	ti.JOSE = hasJOSETag(st)
	// Warn only now that the field list is final: a promoted field shadowed
	// by an outer one never reaches the document. Warnings precede ref
	// registration, which shares the fieldWarned dedupe.
	for i := range ti.Fields {
		a.warnResolvedField(&ti.Fields[i])
	}
	for i := range ti.Fields {
		a.registerFieldRefAt(&ti.Fields[i], site.file, site.path, depth+1)
	}
}

// extractUnder runs extract with env as the bindings every field it builds
// resolves under (buildFieldInfo records them), restoring the outer ones.
func (a *ProjectAnalyzer) extractUnder(env typeEnv, extract func() []models.FieldInfo) []models.FieldInfo {
	outer := a.curEnv
	a.curEnv = env
	defer func() { a.curEnv = outer }()
	return extract()
}

// registerInstanceLeaf registers the instance a ShapeRef leaf's provisional
// name names, stamping its final component name, and reports whether leaf
// named one (handled) and whether it registered. A leaf that cannot
// register is demoted to a named leaf spelled as written (it emits object).
func (a *ProjectAnalyzer) registerInstanceLeaf(leaf *models.TypeShape, depth int) (handled, ok bool) {
	inst, isInstance := a.instances[leaf.Name]
	if !isInstance {
		return false, false
	}
	if ti := a.registerInstantiation(inst, depth); ti != nil {
		leaf.Name = ti.Name
		return true, true
	}
	leaf.Kind, leaf.Name = models.ShapeNamed, inst.written
	return true, false
}

// instanceOf returns the instance r, a resolution, is a ref to, or nil.
func (a *ProjectAnalyzer) instanceOf(r models.TypeShape) *instance {
	if r.Kind != models.ShapeRef {
		return nil
	}
	return a.instances[r.Name]
}

// followInstantiation follows a defined type's right-hand side rhs, an
// Instantiation written in file, to the struct it reaches
// (instantiationStruct).
func (a *ProjectAnalyzer) followInstantiation(rhs ast.Expr, file *ast.File, path string) (chainEnd, bool) {
	return a.instantiationStruct(a.typeShape(rhs), newResolveCtx(file, path, false), 0)
}

// instantiationStruct follows s, an Instantiation written in c's file under
// c's bindings, to the struct it reaches, with the bindings its fields
// resolve under: a generic struct's literal under the Instantiation's own
// bindings; through a generic type declared over another Instantiation
// (type PageA[T any] Page[T]) that one, its arguments under the outer
// bindings; through a generic type declared over a named type (type
// BaseOf[T any] Base) the struct that name reaches, whose fields hold no
// type parameter. A generic type the tool cannot read, or one reaching no
// struct, reaches nothing.
func (a *ProjectAnalyzer) instantiationStruct(s models.TypeShape, c resolveCtx, depth int) (chainEnd, bool) {
	if depth > maxNamedResolutionDepth || s.Kind != models.ShapeInstantiation {
		return chainEnd{}, false
	}
	g, ok := a.genericDeclOf(s.Inst, c)
	if !ok {
		return chainEnd{}, false
	}
	env := g.bind(s.Inst.Args, c)
	switch rhs := ast.Unparen(g.spec.Type).(type) {
	case *ast.StructType:
		return chainEnd{st: rhs, name: g.spec.Name.Name, pkg: g.site.file.Name.Name, file: g.site.file, path: g.site.path, env: env}, true
	case *ast.IndexExpr, *ast.IndexListExpr:
		dc := c.in(g.site.file, g.site.path)
		dc.env = env
		return a.instantiationStruct(a.typeShape(rhs), dc, depth+1)
	default:
		return a.followSpec(g.spec, g.site.file, g.site.path, depth, false)
	}
}
