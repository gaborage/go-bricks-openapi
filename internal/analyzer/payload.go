package analyzer

import (
	"go/ast"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Payload resolution (#110; CONTEXT.md, "Resolution"; docs/adr/0002). A
// response payload that does not register as a project struct is resolved by
// the field resolver, in the handler's file, and typed from its Resolution
// exactly as a struct field of its type is. Its root is never nullable.

// Payload fallback warnings. Each payload raises at most one. The first
// argument is the payload type as written; the unresolvable one keeps its
// pre-#110 text, naming the type that resolved to nothing.
const (
	unresolvablePayloadWarning = "response type %s resolves to no schema component — emitting an untyped schema " +
		"(declare it as a struct in the project for a typed spec)"
	marshalerPayloadWarning = "response type %s: %s has its own %s method, which encoding/json uses instead of its " +
		"underlying type — emitting an untyped schema ({})"
	recursivePayloadWarning = "response type %s: %s contains itself — the recursion is cut to an untyped schema ({})"
	depthCapPayloadWarning  = "response type %s: its named types nest more than %d deep, so %s is cut to an untyped " +
		"schema ({}) as if it were recursive"
	definedOverQualifiedPayloadWarning = "response type %s: %s is a defined type over %s, which resolves to no schema " +
		"(a defined type drops its target's methods, and a type from outside the module is not resolved) — emitting an untyped object"
	declsDisagreePayloadWarning = "response type %s: %s's build-tagged declarations disagree on shape (no common " +
		"container or scalar kind; build constraints are not evaluated) — emitting an untyped object"
	untypedBuiltinPayloadWarning = "response type %s holds the builtin %s, a Go type with no JSON schema " +
		"— emitting an untyped object"
	uintptrPayloadWarning = "response type %s holds a uintptr, a machine address with no meaningful API contract " +
		"— emitting an untyped schema (use a sized integer type)"
	unregisteredRefPayloadWarning = "response type %s: struct %s could not be registered as a component " +
		"— emitting an untyped object"
	marshalerPromotedPayloadWarning = "response type %s: %s has the %s method promoted from its embedded %s, which " +
		"encoding/json uses instead of its fields — emitting an untyped schema ({})"
	marshalerRequestWarning = "request type %s: %s has its own %s method, which encoding/json uses instead of its " +
		"fields — emitting an untyped requestBody schema ({}); its path, query and header fields stay parameters"
	marshalerPromotedRequestWarning = "request type %s: %s has the %s method promoted from its embedded %s, which " +
		"encoding/json uses instead of its fields — emitting an untyped requestBody schema ({}); its path, query and " +
		"header fields stay parameters"
)

// resolvePayload resolves a payload that did not register as a project struct
// in the file it was written in, and reports whether it stamped a Resolution.
// A payload whose root resolves to nothing, or whose resolution has an
// unmodelled leaf, is left untouched for screenUnresolvedPayload. A resolved
// payload names no component (its Name and Fields are cleared) unless its base
// is a well-known type, which keeps its name; the structs it reaches register
// in the file their names were written in, and its one fallback, if any, is
// warned about.
func (a *ProjectAnalyzer) resolvePayload(ti *models.TypeInfo, written string, astFile *ast.File, filePath string) bool {
	if ti.Shape == nil {
		return false
	}
	c := newResolveCtx(astFile, filePath, false)
	r := a.resolveShape(*ti.Shape, c)
	if !payloadResolves(&r) {
		return false
	}
	ti.Resolution = &r
	if !isWellKnownLeaf(payloadBaseShape(*ti.Shape)) {
		ti.Name = ""
		ti.Fields = nil
	}
	warned := a.warnPayload(written, &r, c.st.first)
	a.registerPayloadRefs(&r, c.st.refSites, pkgFile{file: astFile, path: filePath}, written, warned)
	return true
}

// payloadResolves reports whether a payload's resolution r can be typed: its
// root, every pointer shed, is not a named type outside WellKnownTypeNames,
// and no leaf is unmodelled. A non-well-known named leaf BELOW a container
// root still resolves ([]decimal.Decimal): warnPayload warns from the fallback
// the resolver noted, exactly as the field of that type warns.
func payloadResolves(r *models.TypeShape) bool {
	root := r
	for root.Kind == models.ShapePointer && root.Elem != nil {
		root = root.Elem
	}
	if root.Kind == models.ShapeNamed && !models.WellKnownTypeNames[root.Name] {
		return false
	}
	return !hasUnknownLeaf(r)
}

// isWellKnownLeaf reports whether s is a well-known named type (time.Time,
// uuid.UUID, ...: models.WellKnownTypeNames).
func isWellKnownLeaf(s models.TypeShape) bool {
	return s.Kind == models.ShapeNamed && models.WellKnownTypeNames[s.Name]
}

// warnPayload raises a resolved payload's one warning and reports whether it
// warned: the uintptr one when any leaf is a uintptr (it outranks every
// fallback, as for fields), else the first fallback's. written is the payload
// type as written.
func (a *ProjectAnalyzer) warnPayload(written string, r *models.TypeShape, fb fieldFallback) bool {
	if holdsUintptr(r) {
		a.addWarningf(uintptrPayloadWarning, written)
		return true
	}
	switch fb.kind {
	case fallbackNone:
		return false
	case fallbackMarshaler:
		a.addWarningf(marshalerPayloadWarning, written, fb.typeName, fb.detail)
	case fallbackRecursive:
		a.addWarningf(recursivePayloadWarning, written, fb.typeName)
	case fallbackDepthCap:
		a.addWarningf(depthCapPayloadWarning, written, maxNamedResolutionDepth, fb.typeName)
	case fallbackUnresolvable:
		a.addWarningf(unresolvablePayloadWarning, fb.typeName)
	case fallbackDefinedOverQualified:
		a.addWarningf(definedOverQualifiedPayloadWarning, written, fb.typeName, fb.detail)
	case fallbackDeclsDisagree:
		a.addWarningf(declsDisagreePayloadWarning, written, fb.typeName)
	case fallbackMarshalerPromoted:
		a.addWarningf(marshalerPromotedPayloadWarning, written, fb.typeName, fb.detail, fb.via)
	default:
		a.addWarningf(untypedBuiltinPayloadWarning, written, fb.detail)
	}
	return true
}

// registerPayloadRefs registers every ShapeRef leaf of a payload's resolution
// r in the file its name was written in (sites), else in the payload's own
// file (def). A leaf that cannot register is demoted to a named leaf (it emits
// object) and warns once, unless the payload already warned.
func (a *ProjectAnalyzer) registerPayloadRefs(r *models.TypeShape, sites map[string]pkgFile, def pkgFile, written string, warned bool) {
	walkResolvedLeaves(r, func(leaf *models.TypeShape) {
		if leaf.Kind != models.ShapeRef {
			return
		}
		name := leaf.Name // registerLeafAt renames or demotes the leaf in place
		site, ok := sites[name]
		if !ok {
			site = def
		}
		if !a.registerLeafAt(leaf, site, 0) && !warned {
			a.addWarningf(unregisteredRefPayloadWarning, written, name)
			warned = true
		}
	})
}

// IsTypedPayload reports whether the generator documents a request or response
// payload with a resolved schema: a registered struct (or a well-known type
// that keeps its name), or a payload whose schema has no fallback leaf. So
// Result[AnyD] ({}) is typed and Result[Tier] ({}) is not. doctor and
// generate's route count classify routes by it.
func IsTypedPayload(ti *models.TypeInfo) bool {
	if ti == nil {
		return false
	}
	if ti.Resolution == nil && ti.Name != "" {
		return true
	}
	s := ti.ResolvedShape()
	if s == nil {
		return false
	}
	typed := true
	walkResolvedLeaves(s, func(leaf *models.TypeShape) {
		typed = typed && !isFallbackLeaf(leaf)
	})
	return typed
}

// isFallbackLeaf reports whether a schema leaf is a fallback: a named type
// outside WellKnownTypeNames (an unresolved name, or a demoted ref), a uintptr
// or untyped builtin, a Marshaler type, a recursion cut, or an unmodelled leaf.
func isFallbackLeaf(leaf *models.TypeShape) bool {
	switch leaf.Kind {
	case models.ShapeNamed:
		return !models.WellKnownTypeNames[leaf.Name]
	case models.ShapePrimitive:
		return leaf.Name == goTypeUintptr || models.UntypedBuiltinNames[leaf.Name]
	case models.ShapeRef, models.ShapeKindOnly, models.ShapeText:
		return false
	default:
		return true
	}
}

// namedShape is the type a request or response TypeInfo names, spelled as the
// Shape decoder would: bare when declared in astFile's package, qualified by
// its import name otherwise (registerPayloadType's rule).
func namedShape(ti *models.TypeInfo, astFile *ast.File) models.TypeShape {
	name := ti.Name
	if ti.Package != "" && ti.Package != astFile.Name.Name {
		name = ti.Package + "." + name
	}
	return models.TypeShape{Kind: models.ShapeNamed, Name: name}
}

// marshalerNamed resolves the type ti names in a body position and reports
// whether it is a Marshaler type (a text or Marshaler leaf), with the leaf and
// the fallback it noted. A struct Marshaler type never registers (#111).
func (a *ProjectAnalyzer) marshalerNamed(ti *models.TypeInfo, astFile *ast.File, filePath string) (models.TypeShape, fieldFallback, bool) {
	if ti.Name == "" {
		return models.TypeShape{}, fieldFallback{}, false
	}
	c := newResolveCtx(astFile, filePath, false)
	r := a.resolveShape(namedShape(ti, astFile), c)
	return r, c.st.first, r.Kind == models.ShapeMarshaler || r.Kind == models.ShapeText
}

// structSite is a struct declaration and the package and file declaring it.
type structSite struct {
	st   *ast.StructType
	pkg  string
	file *ast.File
	path string
}

// payloadStructSite finds the struct a request TypeInfo names, as
// registerPayloadType would: a local chain, or a struct of another package
// of the module.
func (a *ProjectAnalyzer) payloadStructSite(ti *models.TypeInfo, astFile *ast.File, filePath string) (structSite, bool) {
	if ti.Package != "" && ti.Package != astFile.Name.Name {
		q, ok := a.resolveQualifiedStruct(ti.Package+"."+ti.Name, astFile)
		return structSite{st: q.st, pkg: q.pkg, file: q.file, path: q.filePath}, ok
	}
	st, pkg, file, path, ok := a.resolveTypeSpecChain(astFile, filePath, ti.Name, 0)
	return structSite{st: st, pkg: pkg, file: file, path: path}, ok
}

// populateMarshalerRequest documents a request whose type is a struct
// Marshaler type (#111): its body is the Marshaler leaf's schema ({} with one
// warning, or a string), it names no component, and only its path, query and
// header fields are kept, as parameters. It reports false for any other
// request.
func (a *ProjectAnalyzer) populateMarshalerRequest(h *handlerAnalysis, astFile *ast.File, filePath string) bool {
	ti := h.request
	leaf, fb, isMarshaler := a.marshalerNamed(ti, astFile, filePath)
	if !isMarshaler {
		return false
	}
	site, ok := a.payloadStructSite(ti, astFile, filePath)
	if !ok {
		return false
	}
	params, paramsOnly := a.requestParamFields(site, ti.Name)
	ti.Fields = params
	if paramsOnly && !ti.JOSE {
		// Bound from parameters only (isParamsOnlyType's sense): no body is
		// documented, as before #111; Name is kept so doctor counts it typed.
		return true
	}
	ti.Name = ""
	ti.Resolution = &leaf
	a.warnMarshalerRequest(h.requestType, fb)
	return true
}

// requestParamFields extracts a struct Marshaler request's fields without
// registering it, and keeps its parameters: each warns and registers its refs
// as registerStructKeyed would.
func (a *ProjectAnalyzer) requestParamFields(s structSite, name string) ([]models.FieldInfo, bool) {
	fields := a.extractStructFields(s.st, s.pkg, s.file, s.path, map[string]struct{}{name: {}}, 0)
	total := len(fields)
	params := fields[:0]
	for i := range fields {
		if fields[i].ParamType != "" {
			params = append(params, fields[i])
		}
	}
	for i := range params {
		a.warnResolvedField(&params[i])
	}
	for i := range params {
		a.registerFieldRefAt(&params[i], s.file, s.path, 1)
	}
	// Zero fields is not params-only: such a request (type Ping struct{}
	// with UnmarshalJSON) reads its body through its own method, so it gets
	// a body.
	return params, total > 0 && len(params) == total
}

// structMarshalerJOSE reports whether the struct a Marshaler payload names
// carries a jose: tag, as registerType would have recorded: the route stays
// JOSE on the wire whatever the plaintext's methods.
func (a *ProjectAnalyzer) structMarshalerJOSE(ti *models.TypeInfo, astFile *ast.File, filePath string) bool {
	site, ok := a.payloadStructSite(ti, astFile, filePath)
	return ok && hasJOSETag(site.st)
}

// warnMarshalerRequest raises a struct Marshaler request's one warning, or
// none for a string-both-ways type. written is the request type as written.
func (a *ProjectAnalyzer) warnMarshalerRequest(written string, fb fieldFallback) {
	switch fb.kind {
	case fallbackMarshaler:
		a.addWarningf(marshalerRequestWarning, written, fb.typeName, fb.detail)
	case fallbackMarshalerPromoted:
		a.addWarningf(marshalerPromotedRequestWarning, written, fb.typeName, fb.detail, fb.via)
	default:
		// A string both ways: documented, no warning.
	}
}
