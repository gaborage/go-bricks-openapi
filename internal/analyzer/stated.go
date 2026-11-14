package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// The Stated schema (#117, ADR 0004; CONTEXT.md, "Stated schema"): an
// `//openapi:schema <type> [<format>]` type Directive in the doc comment of a
// package-level type declaration states the wire schema of a Marshaler type
// the project declares. Placement and arguments are decided syntactically
// when a file is indexed (placeTypeDirective, addSchemaLine); whether the
// type accepts it needs its method set, so that verdict is computed lazily,
// once per declaration site (schemaVerdictFor), by the resolver when a body
// position reaches the type and by warnTypeDirective after the walk.

// Schema directive diagnostics. Each is raised once per comment group.
const (
	schemaNoArgsMsg     = "directive schema requires a type: string, integer, number or boolean (e.g. //openapi:schema string decimal)"
	schemaTooManyMsg    = "directive schema: ignoring %q — expected a type and at most one format"
	schemaBadTypeMsg    = "directive schema: ignoring %q — expected string, integer, number or boolean"
	schemaAboveGroupMsg = "directive schema has no effect — place it on a type inside the group"
	schemaConflictMsg   = "directive schema has no effect — its lines state different schemas, so none applies"
	schemaBuildTagMsg   = "directive schema has no effect — the build-tagged declarations of %s state different schemas, so none applies"
	schemaGenericMsg    = "directive schema has no effect — %s is a generic type, whose wire form usually depends on its type arguments"
	schemaNotMarshalMsg = "directive schema has no effect — %s is not a Marshaler type, so it is documented from its declaration"
	schemaAliasMsg      = "directive schema has no effect — %s is an alias of %s; state it on %s"
	schemaPtrOnlyMsg    = "directive schema has no effect — %s's %s method is not in its value method set, and encoding/json skips it " +
		"on a map value or a payload returned by value; declare it on the value receiver"
	schemaTextMsg = "directive schema %s has no effect — %s encodes as text both ways, so only string [<format>] applies"
)

// statedTypes are the type words `schema` accepts: the OpenAPI scalars.
var statedTypes = map[string]bool{"string": true, "integer": true, "number": true, "boolean": true}

// typeDoc is what a comment group is to a file's package-level type
// declarations: the doc comment of spec, or of a whole parenthesized group
// (aboveGroup). The zero value is neither.
type typeDoc struct {
	spec       *ast.TypeSpec
	aboveGroup bool
}

// typeDocs maps the doc comments of astFile's package-level type
// declarations, as go/parser attaches them: an ungrouped `type X …`'s is its
// GenDecl's, a grouped type's its TypeSpec's, and a group's own GenDecl doc
// applies to no type. A type declared inside a function body is not
// package-level, so its doc comment is not a placement.
func typeDocs(astFile *ast.File) map[*ast.CommentGroup]typeDoc {
	docs := map[*ast.CommentGroup]typeDoc{}
	for _, decl := range astFile.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
			addTypeDocs(docs, gd)
		}
	}
	return docs
}

// addTypeDocs adds one type declaration's doc comments to docs.
func addTypeDocs(docs map[*ast.CommentGroup]typeDoc, gd *ast.GenDecl) {
	if !gd.Lparen.IsValid() {
		if ts, ok := gd.Specs[0].(*ast.TypeSpec); ok && gd.Doc != nil {
			docs[gd.Doc] = typeDoc{spec: ts}
		}
		return
	}
	if gd.Doc != nil {
		docs[gd.Doc] = typeDoc{aboveGroup: true}
	}
	for _, spec := range gd.Specs {
		if ts, ok := spec.(*ast.TypeSpec); ok && ts.Doc != nil {
			docs[ts.Doc] = typeDoc{spec: ts}
		}
	}
}

// parseSchemaArgs parses `schema`'s argument string: a type word and an
// optional free-form format. problem is the diagnostic for arguments it
// rejects, in which case the line is ignored.
func parseSchemaArgs(args string) (s models.StatedSchema, problem string) {
	fields := strings.Fields(args)
	switch {
	case len(fields) == 0:
		return s, schemaNoArgsMsg
	case len(fields) > 2:
		return s, fmt.Sprintf(schemaTooManyMsg, args)
	case !statedTypes[fields[0]]:
		return s, fmt.Sprintf(schemaBadTypeMsg, fields[0])
	}
	s.Type = fields[0]
	if len(fields) == 2 {
		s.Format = fields[1]
	}
	return s, ""
}

// addSchemaLine folds one `schema` line into g: a valid one joins g's
// distinct schemas (an identical repeat collapses silently), an invalid one
// warns for itself and takes no part in the comparison.
func (a *ProjectAnalyzer) addSchemaLine(g *directiveGroup, args string, pos token.Pos) {
	if !g.schemaAt.IsValid() {
		g.schemaAt = pos
	}
	s, problem := parseSchemaArgs(args)
	if problem != "" {
		a.warnDirectivef(pos, "%s", problem)
		return
	}
	if !slices.Contains(g.schemas, s) {
		g.schemas = append(g.schemas, s)
	}
}

// statedDecl is one type declaration whose doc comment carries a `schema`
// line, and the file declaring it.
type statedDecl struct {
	g    *directiveGroup
	file *ast.File
	path string
}

// placeTypeDirective records where g sits among astFile's type declarations
// (d, from typeDocs) and, when it is a type's doc comment carrying a valid
// `schema` line, indexes it under the type's declaration site. Purely
// syntactic: indexing runs while a package directory is being parsed, so it
// must not resolve anything.
func (a *ProjectAnalyzer) placeTypeDirective(g *directiveGroup, d typeDoc, path string, astFile *ast.File) {
	g.aboveGroup = d.aboveGroup
	if d.spec == nil {
		return
	}
	g.typeSpec, g.typeKey = d.spec, typeKeyOf(path, d.spec.Name.Name)
	if len(g.schemas) == 0 {
		return
	}
	if a.statedDecls == nil {
		a.statedDecls = map[string][]statedDecl{}
	}
	a.statedDecls[g.typeKey] = append(a.statedDecls[g.typeKey], statedDecl{g: g, file: astFile, path: path})
}

// schemaVerdict is what the directives on one declaration site's type
// declarations amount to: the Stated schema that applies, or the reason none
// does (empty when no declaration states one).
type schemaVerdict struct {
	stated *models.StatedSchema
	reason string
}

// statedSchema returns the Stated schema that applies to the type declared at
// key (a typeKey), or nil.
func (a *ProjectAnalyzer) statedSchema(key string) *models.StatedSchema {
	return a.schemaVerdictFor(key).stated
}

// schemaVerdictFor returns the memoized verdict for the type declared at key.
// Its package directory is parsed first, so every build-tagged declaration's
// directive is indexed.
func (a *ProjectAnalyzer) schemaVerdictFor(key string) schemaVerdict {
	if v, ok := a.schemaVerdicts[key]; ok {
		return v
	}
	dir, _, _ := strings.Cut(key, "\x00")
	_, _ = a.parsePackageDir(dir) // indexes the directory's files; an unreadable one adds none
	v := a.computeSchemaVerdict(key)
	if a.schemaVerdicts == nil {
		a.schemaVerdicts = map[string]schemaVerdict{}
	}
	a.schemaVerdicts[key] = v
	return v
}

// computeSchemaVerdict decides the verdict for key: a declaration without
// the directive states nothing, exactly one distinct Stated schema across
// the build-tagged declarations applies unless the type refuses it, and two
// different ones apply neither.
func (a *ProjectAnalyzer) computeSchemaVerdict(key string) schemaVerdict {
	decls := a.statedDecls[key]
	s, n := soleStated(decls)
	switch {
	case n == 0:
		return schemaVerdict{}
	case n > 1:
		return schemaVerdict{reason: fmt.Sprintf(schemaBuildTagMsg, decls[0].g.typeSpec.Name.Name)}
	}
	if reason := a.schemaRefusal(decls, s); reason != "" {
		return schemaVerdict{reason: reason}
	}
	return schemaVerdict{stated: &s}
}

// soleStated returns the first distinct Stated schema among decls, counting
// only declarations whose group states exactly one, and how many distinct
// ones there are.
func soleStated(decls []statedDecl) (first models.StatedSchema, distinctCount int) {
	var distinct []models.StatedSchema
	for _, d := range decls {
		if len(d.g.schemas) == 1 && !slices.Contains(distinct, d.g.schemas[0]) {
			distinct = append(distinct, d.g.schemas[0])
		}
	}
	if len(distinct) == 0 {
		return models.StatedSchema{}, 0
	}
	return distinct[0], len(distinct)
}

// schemaRefusal returns why the type decls declare does not accept s, or ""
// when it does: a generic type (decided from its type parameters, not from
// Marshaler detection), a type that is not a Marshaler type, a plain alias
// (which uses its target's Stated schema), a type with an encode-side method
// outside its value method set, or a text-both-ways type stated as anything
// but a string.
func (a *ProjectAnalyzer) schemaRefusal(decls []statedDecl, s models.StatedSchema) string {
	spec := decls[0].g.typeSpec
	name := spec.Name.Name
	if slices.ContainsFunc(decls, func(d statedDecl) bool { return d.g.typeSpec.TypeParams != nil }) {
		return fmt.Sprintf(schemaGenericMsg, name)
	}
	c := newResolveCtx(decls[0].file, decls[0].path, false)
	m := a.typeMarshalerMethods(name, c)
	if !m.found() {
		return fmt.Sprintf(schemaNotMarshalMsg, name)
	}
	if spec.Assign.IsValid() && !a.marshalerSet(name, c).found() {
		target := types.ExprString(spec.Type)
		return fmt.Sprintf(schemaAliasMsg, name, target, target)
	}
	return methodsRefusal(name, m, s)
}

// methodsRefusal returns why a Marshaler type name with methods m does not
// accept s, or "" when it does.
func methodsRefusal(name string, m marshalerMethods, s models.StatedSchema) string {
	if method := m.ptrEncode(); method != "" {
		return fmt.Sprintf(schemaPtrOnlyMsg, name, method)
	}
	if m.textBothWays() && s.Type != goTypeString {
		return fmt.Sprintf(schemaTextMsg, s.Type, name)
	}
	return ""
}

// typeMarshalerMethods returns what name's method set, in c's package, says
// about its JSON form, as resolveLocal reads it in identity mode: a struct
// (or a chain ending at one) through its declared and promoted methods, an
// interface (or a chain ending at one) through its declared and embedded
// methods (both methodSetOf), any other type through the methods declared on
// it (marshalerSet).
func (a *ProjectAnalyzer) typeMarshalerMethods(name string, c resolveCtx) marshalerMethods {
	if _, _, _, _, ok := a.resolveTypeSpecChain(c.file, c.path, name); ok || a.endsAtInterface(c.file, c.path, name, 0) {
		m, _ := a.methodSetOf(name, c, true, nil).marshaler(c.display(name))
		return m
	}
	return a.marshalerSet(name, c)
}

// warnTypeDirective raises a group's own `schema` diagnostic, at its first
// `schema` line: above a whole type group, lines that disagree, or the
// verdict's reason the type does not accept its Stated schema.
func (a *ProjectAnalyzer) warnTypeDirective(g *directiveGroup) {
	if !slices.Contains(g.names, directiveNameSchema) {
		return
	}
	if problem := a.typeDirectiveProblem(g); problem != "" {
		a.warnDirectivef(g.schemaAt, "%s", problem)
	}
}

// typeDirectiveProblem returns g's `schema` diagnostic, or "" when it has
// none (it applies, it is Detached, or every line was invalid on its own).
func (a *ProjectAnalyzer) typeDirectiveProblem(g *directiveGroup) string {
	switch {
	case g.aboveGroup:
		return schemaAboveGroupMsg
	case g.typeSpec == nil || len(g.schemas) == 0:
		return ""
	case len(g.schemas) > 1:
		return schemaConflictMsg
	}
	return a.schemaVerdictFor(g.typeKey).reason
}
