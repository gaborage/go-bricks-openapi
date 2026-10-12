package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Untagged embeds (issue 116). encoding/json promotes the exported fields of
// an embedded struct, writes an exported embedded non-struct type as one
// field keyed by its type name, and skips an unexported non-struct embed,
// even one with a json name (issue 185). classifyEmbed decides which applies
// from source; an untagged embed it cannot classify is dropped with a
// warning.

// Dropped-embed warnings. Each is raised at most once per embed position.
const (
	unresolvableEmbedWarning = "embedded field %s at %s has type %s: %s cannot be read (a type from outside the module, " +
		"an in-module package whose buildable files all carry a build constraint " +
		"and whose first one by name has another package clause, or a name with no declaration in its package), so neither " +
		"its fields nor its methods are known — the embedded field is dropped from the spec"
	genericEmbedWarning = "embedded field %s at %s has type %s: %s is a generic instantiation, which is not expanded " +
		"— the embedded field is dropped from the spec"
)

// embedClass is what an untagged embed's type is, as classifyEmbed finds it.
type embedClass int

const (
	embedSkip         embedClass = iota // an unexported non-struct: dropped silently, json name or not, as encoding/json skips it
	embedMarshaler                      // a well-known Marshaler type keeping its methods: dropped silently unless a parameter
	embedStruct                         // a struct: its fields are promoted
	embedOpaqueStruct                   // a defined type over time.Time or a netip struct, with no exported field: nothing is promoted
	embedNonStruct                      // a non-struct type: one property
	embedGeneric                        // a generic instantiation: dropped with a warning
	embedUnresolvable                   // a type no declaration is read for: dropped with a warning
)

// embedTarget is an embed's classification: the struct to promote
// (embedStruct) or the type where classification stopped (the warned
// classes, and embedMarshaler's well-known type).
type embedTarget struct {
	class embedClass
	site  structSite
	stop  string
}

// embedClassifier follows an embed's chain of named types from the package
// whose directory is home.
type embedClassifier struct {
	a    *ProjectAnalyzer
	home string
}

// classifyEmbed classifies an embed of astFile. A pointer is shed (an embed
// is T or *T). A struct, reached through any chain of local and in-module
// aliases and defined types, is promoted, exported or not; an unexported
// non-struct embed, a predeclared type (error, int, any), a local generic
// non-struct type (list[int]) or an alias of a well-known Marshaler type
// that is not a struct (uuid.UUID) among them, is skipped, json name or
// not, as encoding/json skips it before it reads the tag. An unexported
// embed classification stops short of (unreadable, or generic) is warned
// like an exported one: encoding/json promotes it if it is a struct.
func (a *ProjectAnalyzer) classifyEmbed(field *ast.Field, astFile *ast.File, filePath string) embedTarget {
	t := field.Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	ec := embedClassifier{a: a, home: filepath.Dir(filePath)}
	var target embedTarget
	switch e := t.(type) {
	case *ast.Ident:
		target = ec.local(astFile, filePath, e.Name, 0, false)
	case *ast.SelectorExpr:
		target = ec.qualified(astFile, a.typeShape(e).Name, 0, false)
	default:
		target = ec.generic(astFile, filePath, t)
	}
	if target.nonStruct() && !token.IsExported(embeddedFieldName(field.Type)) {
		return embedTarget{class: embedSkip}
	}
	return target
}

// nonStruct reports whether t's type is known not to be a struct: a
// non-struct type, or a well-known Marshaler type other than the structs
// (wellKnownStructs).
func (t embedTarget) nonStruct() bool {
	return t.class == embedNonStruct || t.class == embedMarshaler && !wellKnownStructs[t.stop]
}

// generic classifies an embedded generic instantiation, which is not
// expanded: an unexported one declared in file's package as a non-struct
// type, a type literal (type list[T any] []T) or a named non-struct type
// followed as local follows it (type id[T any] int64, type qc[T any]
// b.Cents) or an alias of a non-struct well-known Marshaler (type raw[T
// any] = json.RawMessage), is skipped, as encoding/json skips an unexported
// non-struct embed, and one over time.Time or a netip struct is opaque; any
// other is warned, since encoding/json promotes the fields of one that is a
// struct.
func (ec embedClassifier) generic(file *ast.File, path string, t ast.Expr) embedTarget {
	warned := embedTarget{class: embedGeneric, stop: types.ExprString(t)}
	var base ast.Expr
	switch g := t.(type) {
	case *ast.IndexExpr:
		base = g.X
	case *ast.IndexListExpr:
		base = g.X
	}
	id, ok := base.(*ast.Ident)
	if !ok || token.IsExported(id.Name) {
		return warned
	}
	ts, tsFile, tsPath, found := ec.a.resolveLocalTypeSpec(file, path, id.Name)
	if !found {
		return warned
	}
	var named embedTarget
	defined := !ts.Assign.IsValid()
	switch rhs := ast.Unparen(ts.Type).(type) {
	case *ast.Ident:
		named = ec.local(tsFile, tsPath, rhs.Name, 1, defined)
	case *ast.SelectorExpr:
		named = ec.qualified(tsFile, ec.a.typeShape(rhs).Name, 1, defined)
	case *ast.StructType, *ast.IndexExpr, *ast.IndexListExpr:
		return warned // a struct, or a generic type this does not expand
	default:
		return embedTarget{class: embedSkip} // slice, array, map, interface, func, chan
	}
	switch {
	case named.nonStruct():
		return embedTarget{class: embedSkip} // a non-struct, or a non-struct Marshaler (type raw[T any] = json.RawMessage)
	case named.class == embedOpaqueStruct:
		return named
	default:
		return warned // a struct, a well-known Marshaler struct, or a type not read
	}
}

// local classifies name declared in file's package, or predeclared when no
// declaration of it is found. defined reports whether the chain so far
// crossed a defined type, which drops its target's methods. A chain deeper
// than maxNamedResolutionDepth (only invalid code, a cycle, gets there) is
// left to the property's Resolution, which cuts it with a warning.
func (ec embedClassifier) local(file *ast.File, path, name string, depth int, defined bool) embedTarget {
	if depth >= maxNamedResolutionDepth {
		return embedTarget{class: embedNonStruct}
	}
	if st, pkg, f, p, ok := ec.a.resolveTypeSpecChain(file, path, name, 0); ok {
		return embedTarget{class: embedStruct, site: structSite{st: st, pkg: pkg, file: f, path: p}}
	}
	ts, tsFile, tsPath, ok := ec.a.resolveLocalTypeSpec(file, path, name)
	if !ok {
		if isBuiltinShapeName(name) {
			return embedTarget{class: embedNonStruct} // error, int, any: a declaration of the name would have won
		}
		return embedTarget{class: embedUnresolvable, stop: ec.display(file, path, name)}
	}
	defined = defined || !ts.Assign.IsValid()
	switch rhs := ast.Unparen(ts.Type).(type) {
	case *ast.Ident:
		return ec.local(tsFile, tsPath, rhs.Name, depth+1, defined)
	case *ast.SelectorExpr:
		return ec.qualified(tsFile, ec.a.typeShape(rhs).Name, depth+1, defined)
	case *ast.StructType:
		// Only a parenthesized literal, type PS (struct{ ... }), gets here:
		// resolveTypeSpecChain found any other.
		return embedTarget{class: embedStruct, site: structSite{st: rhs, pkg: tsFile.Name.Name, file: tsFile, path: tsPath}}
	case *ast.IndexExpr, *ast.IndexListExpr:
		return embedTarget{class: embedGeneric, stop: types.ExprString(rhs)}
	default:
		return embedTarget{class: embedNonStruct} // slice, array, map, interface, func, chan
	}
}

// qualified classifies q.T as written in file: an in-module struct, a
// well-known Marshaler type (wellKnownMarshalerEmbed), a well-known or
// kind-backed non-struct type (both through file's imports, wellKnownName),
// or an in-module named type, followed in its declaring package; anything
// else is unresolvable. defined is local's.
func (ec embedClassifier) qualified(file *ast.File, qualified string, depth int, defined bool) embedTarget {
	if q, ok := ec.a.resolveQualifiedStruct(qualified, file); ok {
		return embedTarget{class: embedStruct, site: structSite{st: q.st, pkg: q.pkg, file: q.file, path: q.filePath}}
	}
	if name, ok := ec.a.wellKnownName(qualified, file); ok {
		if _, marshaler := wellKnownMethodSets[name]; marshaler {
			return wellKnownMarshalerEmbed(name, defined)
		}
		return embedTarget{class: embedNonStruct}
	}
	if site, name, ok := ec.a.inModuleTypeSite(qualified, file); ok {
		return ec.local(site.file, site.path, name, depth+1, defined)
	}
	return embedTarget{class: embedUnresolvable, stop: qualified}
}

// wellKnownStructs are the well-known Marshaler types that are structs, none
// with an exported field: time.Time and the netip structs.
var wellKnownStructs = map[string]bool{
	models.WellKnownTimeTime:      true,
	models.WellKnownNetipAddr:     true,
	models.WellKnownNetipPrefix:   true,
	models.WellKnownNetipAddrPort: true,
}

// wellKnownMarshalerEmbed classifies an embed whose chain ends at a
// well-known Marshaler type. Reached directly or through aliases only, the
// type keeps its methods, which the embedding struct promotes: the
// promoted-method path owns its body, so it is a property only as a path,
// query or header parameter (embeddedFields). A defined type on the way
// drops them, and encoding/json reads the underlying type: time.Time and
// the netip structs (netip.Addr, netip.Prefix, netip.AddrPort), structs with
// no exported field, add nothing, while uuid.UUID ([16]byte),
// json.RawMessage and net.IP ([]byte) and json.Number (string) are one
// property, typed as a field of the written type is (a byte string, a
// string, or an untyped object with that field's warning).
func wellKnownMarshalerEmbed(qualified string, defined bool) embedTarget {
	switch {
	case !defined:
		return embedTarget{class: embedMarshaler, stop: qualified}
	case wellKnownStructs[qualified]:
		return embedTarget{class: embedOpaqueStruct}
	default:
		return embedTarget{class: embedNonStruct}
	}
}

// display is name as a warning shows it: bare in the embedding package,
// qualified by its package clause in any other.
func (ec embedClassifier) display(file *ast.File, path, name string) string {
	if filepath.Dir(path) == ec.home {
		return name
	}
	return file.Name.Name + "." + name
}

// embedProperty builds an untagged non-struct embed's one field: named and
// keyed by the type's unqualified name, as written (Cents for *b.Cents, CA
// for an alias of Cents), and typed as a field of the written type. A path,
// query or header parameter gets no JSON name, but dominanceKey gives it its
// Name, the same key.
func (a *ProjectAnalyzer) embedProperty(field *ast.Field, astFile *ast.File, filePath string, depth int) rankedField {
	name := embeddedFieldName(field.Type)
	f := a.buildFieldInfo(name, field, astFile, filePath, depth)
	if f.JSONName == "" && f.ParamType == "" {
		f.JSONName = name // encoding/json's key: the type name
	}
	f.embedName = true
	return f
}

// warnDroppedEmbed raises a dropped embed's one warning, once per position.
// While quietEmbeds is set it neither warns nor marks the position, so the
// same embed still warns when a struct holding it is registered.
func (a *ProjectAnalyzer) warnDroppedEmbed(field *ast.Field, t embedTarget) {
	if a.quietEmbeds {
		return
	}
	loc := a.fieldLoc(field)
	if !markOnce(a.fieldWarned, loc) {
		return
	}
	head := []any{embeddedFieldName(field.Type), loc, types.ExprString(field.Type), t.stop}
	if t.class == embedGeneric {
		a.addWarningf(genericEmbedWarning, head...)
		return
	}
	a.addWarningf(unresolvableEmbedWarning, head...)
}

// fieldRank is what encoding/json's dominance rule reads of a field beside
// its key: its depth below the struct being extracted (0 for the struct's
// own fields), and whether its JSON name is an untagged embed's type name
// rather than a tag.
type fieldRank struct {
	depth     int
	embedName bool
}

// rankedField is an extracted field with its rank and its declaration. Ranks
// live only while a struct's fields are merged; models.FieldInfo never
// carries them.
type rankedField struct {
	models.FieldInfo
	fieldRank
	decl fieldDecl
}

// fieldDecl is where an extracted field is resolved once it survives
// (extractStructFields): its AST field, the file declaring its struct, and
// that struct's registration depth. For a promoted field that is the
// embedded struct's depth, one past the outer struct's, so an anonymous
// struct in it is extracted two levels below the outer struct, while a named
// struct in it registers one level below, as registerStructKeyed registers
// every final field, promoted or not. Change both together or neither.
type fieldDecl struct {
	field *ast.Field
	file  *ast.File
	path  string
	depth int
}

// dominanceKey is the key encoding/json writes f under: its JSON name, or
// its Go name when it has none (the generator's key too, propertyName), and
// none for json:"-". A path, query or header parameter has one too: go-bricks
// decodes the body into the whole request struct, so a body value of that
// key lands in the parameter field.
func (f *rankedField) dominanceKey() string {
	switch f.JSONName {
	case jsonSkipValue:
		return ""
	case "":
		return f.Name
	default:
		return f.JSONName
	}
}

// paramKey is a parameter's location and name, the pair OpenAPI requires to
// be unique, or "" for a body field.
func (f *rankedField) paramKey() string {
	if f.ParamType == "" {
		return ""
	}
	return f.ParamType + ":" + f.ParamName
}

// tagged reports whether f's key comes from a json tag.
func (f *rankedField) tagged() bool {
	return f.JSONName != "" && !f.embedName
}

// outranks reports whether f dominates o under encoding/json's rule: it is
// shallower, or, at one depth, tagged where o is not.
func (f *rankedField) outranks(o *rankedField) bool {
	if f.depth != o.depth {
		return f.depth < o.depth
	}
	return f.tagged() && !o.tagged()
}

// dropDominatedFields applies encoding/json's dominance rule to every body
// key (dominanceKey) across fields, a struct's shallow fields followed by its
// promoted ones: a field another of its key outranks is dropped, and of the
// fields of the best rank only the first stays, where encoding/json drops
// them all (two untagged fields at one depth, or two tagged ones). A
// parameter takes part under its key, so it hides the body fields it
// outranks, but is never dropped itself: it stays a parameter whatever the
// body holds.
func dropDominatedFields(fields []rankedField) []rankedField {
	best := bestByKey(fields)
	kept := map[string]bool{}
	out := make([]rankedField, 0, len(fields))
	for i := range fields {
		f := &fields[i]
		if k := f.dominanceKey(); k != "" {
			won := !kept[k] && !best[k].outranks(f)
			kept[k] = kept[k] || won
			if !won && f.ParamType == "" {
				continue
			}
		}
		out = append(out, *f)
	}
	return out
}

// bestByKey returns, for each body key in fields, the first field of the
// best rank of that key.
func bestByKey(fields []rankedField) map[string]*rankedField {
	best := map[string]*rankedField{}
	for i := range fields {
		k := fields[i].dominanceKey()
		if b, ok := best[k]; k != "" && (!ok || fields[i].outranks(b)) {
			best[k] = &fields[i]
		}
	}
	return best
}

// dropDuplicateParams keeps the first parameter of each location and name
// (paramKey) in fields, a struct's shallow fields followed by its promoted
// ones, so the struct's own parameter beats a promoted one, and among
// promoted ones the first wins: OpenAPI rejects two.
func dropDuplicateParams(fields []rankedField) []rankedField {
	seen := map[string]bool{}
	out := fields[:0]
	for i := range fields {
		if k := fields[i].paramKey(); k != "" {
			if seen[k] {
				continue
			}
			seen[k] = true
		}
		out = append(out, fields[i])
	}
	return out
}
