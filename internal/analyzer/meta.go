package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"path/filepath"
	"strings"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// The `//openapi:meta <Type>` route Directive (issue 69) documents the meta
// keys a server.ResultWithMeta[T] handler supplies: the named struct joins
// the framework's timestamp/traceId in the success envelope's meta, as
// allOf: [<the framework block>, {$ref: <Type>}].
//
// Grammar is checked when the file is indexed (addMetaLine). Everything else
// needs the registration file, so it runs once per comment group when a
// registration applies the group (applyMetaDirective), in this order, each
// refusal raising exactly one warning and applying nothing: every line's
// resolution and Marshaler check, the repeated-line comparison, the route's
// applicability, then the reserved keys. The type is registered only once
// every check passed, so a refused directive adds no component.

// Meta directive diagnostics.
const (
	metaNoArgsMsg     = "directive meta requires one type name (e.g. //openapi:meta PageMeta)"
	metaGenericMsg    = "directive meta: %s is a generic type instantiation, which is not resolved — meta not documented"
	metaBadArgMsg     = "directive meta: ignoring %q — expected one type name (Name or pkg.Name)"
	metaUnresolvedMsg = "directive meta: %s does not resolve to a struct in this module — meta not documented"
	// metaGenericDeclMsg refuses a generic type named without type arguments,
	// whose fields depend on them (issue 114).
	metaGenericDeclMsg = "directive meta: %s is a generic type, which is not resolved — meta not documented"
	metaMarshalerMsg   = "directive meta: %s is a Marshaler type, which gets no component — meta not documented"
	metaConflictMsg    = "directive meta: conflicting types %s and %s in one comment group — meta not documented"
	metaNoWrapperMsg   = "directive meta has no effect — the handler does not return server.ResultWithMeta[T] (or a pointer to one)"
	metaRawMsg         = "directive meta has no effect — a WithRawResponse() route sends no envelope"
	metaJOSEMsg        = "directive meta is not applied — meta inside a sealed JOSE envelope is not documented"
	metaNoBodyMsg      = "directive meta has no effect — a 204 response has no body"
	metaReservedMsg    = "directive meta: %s declares %q, a framework-managed meta key — meta not documented"
)

// metaReservedKeys are the meta keys go-bricks always writes itself; a
// handler value for either is dropped.
var metaReservedKeys = map[string]bool{"timestamp": true, "traceId": true}

// metaLine is one `meta` line that passed the grammar check: its argument as
// written and where it sits.
type metaLine struct {
	arg string
	pos token.Pos
}

// metaTarget is a `meta` line resolved to a struct of this module.
type metaTarget struct {
	line metaLine
	// ti names the type as registerPayloadType takes it: Name, and Package
	// set to the registration file's package or the import alias.
	ti   *models.TypeInfo
	site structSite
	file *ast.File
	path string
	// key identifies the type the name denotes, aliases followed, so two
	// lines naming one type (an alias and its target) compare equal without
	// registering either.
	key string
	// registered is the type's registered TypeInfo, set by registeredMeta.
	registered *models.TypeInfo
}

// parseMetaArg checks a `meta` line's argument: exactly one Name or
// pkg.Name token. problem is the diagnostic for one it rejects.
func parseMetaArg(args string) (problem string) {
	switch {
	case args == "":
		return metaNoArgsMsg
	case strings.Contains(args, "["):
		return fmt.Sprintf(metaGenericMsg, args)
	case !isTypeNameToken(args):
		return fmt.Sprintf(metaBadArgMsg, args)
	}
	return ""
}

// isTypeNameToken reports whether s is one identifier or two joined by a dot.
func isTypeNameToken(s string) bool {
	pkg, name, qualified := strings.Cut(s, ".")
	if !qualified {
		return token.IsIdentifier(s)
	}
	return token.IsIdentifier(pkg) && token.IsIdentifier(name)
}

// addMetaLine folds one `meta` line into g: a line failing the grammar warns
// for itself and takes no part in the comparison.
func (a *ProjectAnalyzer) addMetaLine(g *directiveGroup, args string, pos token.Pos) {
	if problem := parseMetaArg(args); problem != "" {
		a.warnDirectivef(pos, "%s", problem)
		return
	}
	g.metaLines = append(g.metaLines, metaLine{arg: args, pos: pos})
}

// applyMetaDirective stamps the meta type g's `meta` lines document onto
// route, computing the verdict (and raising its one warning) the first time
// a registration applies g. An accepted type is stamped as its unregistered
// TypeInfo, which registerRouteMeta registers once the duplicate-route pass
// has dropped every loser, so a dropped route registers no component.
func (a *ProjectAnalyzer) applyMetaDirective(route *models.Route, g *directiveGroup) {
	if len(g.metaLines) == 0 {
		return
	}
	if !g.metaDone {
		g.metaDone = true
		if t := a.metaVerdict(route, g.metaLines); t != nil {
			if a.metaPending == nil {
				a.metaPending = map[*models.TypeInfo]*metaTarget{}
			}
			a.metaPending[t.ti] = t
			g.metaApplied = t.ti
		}
	}
	route.Meta = g.metaApplied
}

// registerRouteMeta registers the meta type of every route the duplicate-route
// pass kept and stamps the registered TypeInfo on it.
func (a *ProjectAnalyzer) registerRouteMeta(modules []models.Module) {
	for i := range modules {
		for j := range modules[i].Routes {
			if r := &modules[i].Routes[j]; r.Meta != nil {
				r.Meta = a.registeredMeta(r.Meta)
			}
		}
	}
}

// registeredMeta registers the accepted meta type stamped as pending, once.
func (a *ProjectAnalyzer) registeredMeta(pending *models.TypeInfo) *models.TypeInfo {
	t := a.metaPending[pending]
	if t.registered == nil {
		t.registered = a.registerPayloadType(t.ti, t.file, t.path)
	}
	return t.registered
}

// metaVerdict runs every check after the grammar on lines for route and
// returns the accepted type, or nil when the directive is refused.
func (a *ProjectAnalyzer) metaVerdict(route *models.Route, lines []metaLine) *metaTarget {
	t, ok := a.soleMetaTarget(lines)
	if !ok {
		return nil
	}
	if problem := metaApplicability(route); problem != "" {
		a.warnDirectivef(t.line.pos, "%s", problem)
		return nil
	}
	if key := a.reservedMetaKey(t.site); key != "" {
		a.warnDirectivef(t.line.pos, metaReservedMsg, t.line.arg, key)
		return nil
	}
	return &t
}

// soleMetaTarget resolves every line, each failing one warning for itself,
// and returns the one type the rest name. Lines naming one type collapse;
// two types warn at the later line, and neither applies.
func (a *ProjectAnalyzer) soleMetaTarget(lines []metaLine) (metaTarget, bool) {
	var resolved []metaTarget
	for _, l := range lines {
		if t, ok := a.resolveMetaLine(l); ok {
			resolved = append(resolved, t)
		}
	}
	if len(resolved) == 0 {
		return metaTarget{}, false
	}
	first := resolved[0]
	for i := range resolved[1:] {
		if t := &resolved[i+1]; t.key != first.key {
			a.warnDirectivef(t.line.pos, metaConflictMsg, first.line.arg, t.line.arg)
			return metaTarget{}, false
		}
	}
	return first, true
}

// resolveMetaLine resolves l's type name in the file holding it, the
// registration file: its package for Name, its imports for pkg.Name. It
// warns and reports false when the name is no struct of this module or is a
// Marshaler type, which gets no component.
func (a *ProjectAnalyzer) resolveMetaLine(l metaLine) (metaTarget, bool) {
	file, path := a.fileAt(l.pos)
	if file == nil {
		a.warnDirectivef(l.pos, metaUnresolvedMsg, l.arg)
		return metaTarget{}, false
	}
	ti := &models.TypeInfo{Name: l.arg, Package: file.Name.Name}
	if pkg, name, qualified := strings.Cut(l.arg, "."); qualified {
		ti = &models.TypeInfo{Name: name, Package: pkg}
	}
	site, ok := a.payloadStructSite(ti, file, path)
	if !ok {
		a.warnDirectivef(l.pos, metaUnresolvedMsg, l.arg)
		return metaTarget{}, false
	}
	if spec := typeSpecInFile(site.file, ti.Name); spec != nil && spec.TypeParams != nil {
		a.warnDirectivef(l.pos, metaGenericDeclMsg, l.arg)
		return metaTarget{}, false
	}
	_, fb, isMarshaler := a.marshalerNamed(ti, file, path)
	if isMarshaler {
		a.warnDirectivef(l.pos, metaMarshalerMsg, l.arg)
		return metaTarget{}, false
	}
	if fb.kind == fallbackDeclsDisagree && fb.detail == declsInterfaceCause {
		// Its build-tagged copies (or a name on its chain) disagree on an
		// interface's marshal methods (issue 179): no one declaration is the
		// type, and registerPayloadType registers none.
		a.warnDirectivef(l.pos, metaUnresolvedMsg, l.arg)
		return metaTarget{}, false
	}
	return metaTarget{line: l, ti: ti, site: site, file: file, path: path, key: a.metaKey(ti, site, file, path)}, true
}

// fileAt returns the parsed file holding pos and its path, or nil when it
// cannot be read.
func (a *ProjectAnalyzer) fileAt(pos token.Pos) (file *ast.File, path string) {
	path = a.fileSet.Position(pos).Filename
	files, err := a.parsePackageDir(filepath.Dir(path))
	if err != nil {
		return nil, ""
	}
	return files[path], path
}

// metaKey identifies the type ti names: the declaring site of the type it
// denotes. A Go alias is the type it names, so aliases are followed (type PM
// = pagination.Meta is pagination.Meta); a defined type is a type of its own
// and is not. A qualified struct, or a local name whose alias chain cannot be
// followed, is the struct site resolution reached.
func (a *ProjectAnalyzer) metaKey(ti *models.TypeInfo, site structSite, file *ast.File, path string) string {
	if ti.Package == file.Name.Name {
		if key, ok := a.localTypeKey(ti.Name, file, path, 0); ok {
			return key
		}
	}
	return filepath.Dir(site.path) + "\x00" + ti.Name
}

// maxMetaAliasHops bounds localTypeKey's alias chain, as followChain's.
const maxMetaAliasHops = 8

// localTypeKey is metaKey for a name declared in file's package: its own
// declaring site, or, for an alias, its target's.
func (a *ProjectAnalyzer) localTypeKey(name string, file *ast.File, path string, depth int) (string, bool) {
	spec, specFile, specPath, ok := a.resolveLocalTypeSpec(file, path, name)
	if !ok || depth > maxMetaAliasHops {
		return "", false
	}
	if !spec.Assign.IsValid() {
		return filepath.Dir(specPath) + "\x00" + name, true
	}
	switch target := ast.Unparen(spec.Type).(type) {
	case *ast.Ident:
		return a.localTypeKey(target.Name, specFile, specPath, depth+1)
	case *ast.SelectorExpr:
		if q, ok := a.resolveQualifiedStruct(types.ExprString(target), specFile); ok {
			return filepath.Dir(q.filePath) + "\x00" + q.typeName, true
		}
	}
	return "", false
}

// metaApplicability returns why the directive cannot apply to route, or ""
// when its success response is the inline JSON envelope: the handler must
// return server.ResultWithMeta[T] (or one pointer to it), and the route must
// be neither raw, nor JOSE (the generator's application/jose condition), nor
// bodiless.
func metaApplicability(route *models.Route) string {
	switch {
	case route.ResultWrapper != models.ResultWrapperResultWithMeta:
		return metaNoWrapperMsg
	case route.RawResponse:
		return metaRawMsg
	case route.Response != nil && route.Response.JOSE:
		return metaJOSEMsg
	case route.Response != nil && route.Response.NoContent:
		return metaNoBodyMsg
	}
	return ""
}

// reservedMetaKey returns the first framework-managed key among the
// properties the struct at s would emit, promoted ones included, or "". It
// extracts the fields without registering anything and without a diagnostic
// (withoutDiagnostics): a refused type is never emitted, so its fields must
// not warn, and an accepted one warns when it is registered.
func (a *ProjectAnalyzer) reservedMetaKey(s structSite) string {
	var fields []models.FieldInfo
	a.withoutDiagnostics(func() {
		fields = a.extractStructFields(s.st, s.file, s.path, map[string]struct{}{a.structIdentity(s.st): {}}, 0)
	})
	for i := range fields {
		if key := bodyPropertyKey(&fields[i]); metaReservedKeys[key] {
			return key
		}
	}
	return ""
}

// withoutDiagnostics runs probe with every diagnostic it would raise
// discarded and every once-only latch it would set restored, so a later
// extraction of the same fields still warns exactly once. Embeds are read
// quietly (quietEmbeds), as requestParamFields reads them.
func (a *ProjectAnalyzer) withoutDiagnostics(probe func()) {
	n := len(a.warnings)
	tagWarned, fieldWarned, uintptrWarned := maps.Clone(a.tagWarned), maps.Clone(a.fieldWarned), maps.Clone(a.uintptrWarned)
	fieldWarnings, depthWarned, literalWarned := maps.Clone(a.fieldWarnings), a.depthWarned, a.literalWarned
	a.quietEmbeds = true
	probe()
	a.quietEmbeds = false
	a.warnings = a.warnings[:n]
	a.tagWarned, a.fieldWarned, a.uintptrWarned = tagWarned, fieldWarned, uintptrWarned
	a.fieldWarnings, a.depthWarned, a.literalWarned = fieldWarnings, depthWarned, literalWarned
}

// bodyPropertyKey is the JSON key a field is emitted under, as the
// generator's propertyName keys it, or "" for a path, query or header
// parameter and a json:"-" field.
func bodyPropertyKey(f *models.FieldInfo) string {
	if f.ParamType != "" || f.JSONSkip {
		return ""
	}
	if f.JSONName != "" {
		return f.JSONName
	}
	return f.Name
}
