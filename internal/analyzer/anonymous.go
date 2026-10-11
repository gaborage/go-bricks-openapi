package analyzer

import (
	"go/ast"
	"go/types"
	"strings"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Anonymous structs (issue 118; CONTEXT.md, "Resolution"). A struct literal
// written as a field's type, at any depth, decodes as a ShapeStruct carrying
// its literal. A field's resolution extracts the literal's fields exactly as
// a named struct's are extracted (tags, validate constraints, embedded
// promotion), in the file the literal was written in, and the generator
// documents it inline: it has no name, so it gets no component. Its fields
// warn and register their refs as a named struct's fields do. A field is
// resolved only once it survives its own struct's dominance rule and
// parameter deduplication (dropDominatedFields, dropDuplicateParams), so a
// literal is extracted, and raises its warnings, only for a field that
// reaches the document.
//
// Accepted residual: every build-tagged declaration of a named type over a
// literal is extracted, so a malformed tag or a dropped embed in it warns
// once per declaration, though only the first is documented.

// maxLiteralExtractions bounds how many anonymous structs one outermost
// extraction expands, itself included. closesLiteralCycle bounds a cycle; this
// bounds what no cycle guard can see: named types over anonymous structs that
// reach one another many ways (type Q1 []struct{ A, B Q2 }, Q2 over Q3, and
// so on) unfold 2^n inline objects, where a named struct would be one $ref.
// Set far above any realistic field; past it the rest are {} with one
// latched warning.
const maxLiteralExtractions = 1000

// literalLimitWarning is maxLiteralExtractions' latched warning.
const literalLimitWarning = "a field's anonymous structs expand to more than %d inline objects — the rest are truncated to an untyped schema ({}); " +
	"check for named slice or map types over anonymous structs that reach one another many ways"

// anonStructDisplay is how a warning names an anonymous struct where its
// resolution stopped. The warning's head already prints the declared type,
// literal included.
const anonStructDisplay = "struct{…}"

// resolveStruct resolves an anonymous struct. Outside a resolution that
// documents it inline (a parameter, which is bound by kind; a quiet run; a
// resolution that only classifies a type) it is returned as is, sharing the
// Shape's AnonymousStruct, which nothing may write into. Otherwise one that
// embeds a Marshaler type is itself one (ADR 0002), a text or noted Marshaler
// leaf; one that closes a cycle (closesLiteralCycle) is cut like a recursion,
// noted; one past maxTypeRegistrationDepth is cut with the name-less depth
// warning; and one past maxLiteralExtractions in its outermost extraction is
// cut with that limit's latched warning. Any other is extracted
// (extractLiteral).
func (a *ProjectAnalyzer) resolveStruct(s models.TypeShape, c resolveCtx) models.TypeShape {
	if !c.st.inline || s.Struct == nil || s.Struct.Literal == nil {
		return s
	}
	lit := s.Struct.Literal
	if leaf, ok := a.literalMarshalerLeaf(lit, c); ok {
		return leaf
	}
	// structIdentity is the literal's position, shared by every parse of its
	// file, so a literal re-entered through the package cache's parse is
	// still caught.
	key := a.structIdentity(lit)
	if a.closesLiteralCycle(key) {
		c.st.note(fieldFallback{kind: fallbackRecursive, typeName: anonStructDisplay})
		return models.TypeShape{Kind: models.ShapeRecursive, Name: anonStructDisplay}
	}
	depth := c.st.depth + 1
	if depth > maxTypeRegistrationDepth {
		a.warnDepthExceeded()
		return models.TypeShape{Kind: models.ShapeRecursive, Name: anonStructDisplay}
	}
	if len(a.openLiterals) > 0 && a.literalExtracts >= maxLiteralExtractions {
		a.warnLiteralLimit()
		return models.TypeShape{Kind: models.ShapeRecursive, Name: anonStructDisplay}
	}
	fields := a.extractLiteral(lit, key, c, depth)
	return models.TypeShape{Kind: models.ShapeStruct, Struct: &models.AnonymousStruct{Literal: lit, Fields: fields}}
}

// extractLiteral extracts an anonymous struct's fields at depth, one
// registration level below the struct declaring the field, in the file it
// was written in, holding it open. Its embed ancestor set is fresh: a literal
// is a distinct type, so encoding/json promotes even its enclosing struct
// into it, and closesLiteralCycle is what stops a cycle back to it. The
// result is never nil, so an extracted literal with no fields is told apart
// from an unextracted one.
func (a *ProjectAnalyzer) extractLiteral(lit *ast.StructType, key string, c resolveCtx, depth int) []models.FieldInfo {
	if len(a.openLiterals) == 0 {
		a.literalExtracts = 0
	}
	a.openLiterals = append(a.openLiterals, key)
	a.literalExtracts++
	defer a.closeLiteral()
	fields := a.extractStructFields(lit, c.file, c.path, map[string]struct{}{}, depth)
	if fields == nil {
		return []models.FieldInfo{}
	}
	a.foldInlineRefSites(fields, c)
	return fields
}

// foldInlineRefSites adds the ref sites and declaring directories an
// anonymous struct's fields recorded to c's, first wins, so resolveEachDecl
// sees the structs its literal reaches: build-tagged declarations written
// alike whose one qualifier names two packages clash. Nothing registers
// from these entries: the field holding the literal has no ref leaf of its
// own.
func (a *ProjectAnalyzer) foldInlineRefSites(fields []models.FieldInfo, c resolveCtx) {
	for i := range fields {
		if site, ok := a.fieldSites[fields[i].Resolution]; ok {
			foldRefSites(c.st.refSites, c.st.refDirs, site.refSites, site.refDirs)
		}
	}
}

// closeLiteral closes the innermost open anonymous struct. Closing the
// outermost forgets the cycles its extraction found, so what one field
// documents never depends on which fields were resolved before it.
func (a *ProjectAnalyzer) closeLiteral() {
	a.openLiterals = a.openLiterals[:len(a.openLiterals)-1]
	if len(a.openLiterals) == 0 {
		clear(a.literalCycles)
	}
}

// warnLiteralLimit raises maxLiteralExtractions' warning, once per run: like
// warnDepthExceeded, it can fire many times on one input.
func (a *ProjectAnalyzer) warnLiteralLimit() {
	if a.literalWarned {
		return
	}
	a.literalWarned = true
	a.addWarningf(literalLimitWarning, maxLiteralExtractions)
}

// closesLiteralCycle reports whether the anonymous struct key closes a cycle
// of anonymous structs, so it is cut rather than extracted: it is open, or
// the outermost open one's extraction already found it on a cycle with one
// that is open. That open one and every one opened after it are then on a
// cycle with key, and join its class.
//
// Cutting only a literal that is open would bound each path, not the
// document: k sibling literals that each embed their enclosing struct
// (struct{ *Parent; V int }) reach one another in every order, so the
// expansion would grow with k!. Cutting a literal already known to share a
// cycle with an open one expands each literal of a cycle at most once per
// outermost extraction, so those k fields document k² anonymous structs.
func (a *ProjectAnalyzer) closesLiteralCycle(key string) bool {
	root := a.literalCycleRoot(key)
	for i, open := range a.openLiterals {
		if a.literalCycleRoot(open) == root {
			a.joinLiteralCycle(root, a.openLiterals[i+1:])
			return true
		}
	}
	return false
}

// joinLiteralCycle puts every anonymous struct of keys into root's cycle
// class.
func (a *ProjectAnalyzer) joinLiteralCycle(root string, keys []string) {
	for _, k := range keys {
		if r := a.literalCycleRoot(k); r != root {
			a.literalCycles[r] = root
		}
	}
}

// literalCycleRoot is the representative of key's cycle class: key itself
// while no cycle holds it.
func (a *ProjectAnalyzer) literalCycleRoot(key string) string {
	for {
		next, ok := a.literalCycles[key]
		if !ok {
			return key
		}
		key = next
	}
}

// literalMarshalerLeaf reports whether an anonymous struct is a Marshaler
// type through the methods its embedded fields promote (a literal declares
// none), with its leaf: what structMarshalerLeaf returns for a named struct.
func (a *ProjectAnalyzer) literalMarshalerLeaf(lit *ast.StructType, c resolveCtx) (models.TypeShape, bool) {
	return methodSetLeaf(a.promotedMethods(lit, c, nil), anonStructDisplay, anonStructDisplay, c)
}

// sameLiteral reports whether two build-tagged declarations' anonymous
// structs are written alike (literalText), so they merge. Two files whose
// imports bind one qualifier to different packages compare equal here, but
// the refs their fields record clash in resolveEachDecl (foldInlineRefSites).
func sameLiteral(x, y *models.AnonymousStruct) bool {
	if x == nil || y == nil || x.Literal == nil || y.Literal == nil {
		return false
	}
	return literalText(x.Literal) == literalText(y.Literal)
}

// literalText is an anonymous struct as written, tags included: its type
// (types.ExprString elides tags) followed by every field's raw tag, nested
// literals' too, in source order, each after a separator so an untagged field
// still counts.
func literalText(lit *ast.StructType) string {
	var b strings.Builder
	b.WriteString(types.ExprString(lit))
	ast.Inspect(lit, func(n ast.Node) bool {
		if f, ok := n.(*ast.Field); ok {
			b.WriteByte(0)
			if f.Tag != nil {
				b.WriteString(f.Tag.Value)
			}
		}
		return true
	})
	return b.String()
}

// warnInlineFields raises the warnings of the fields of every anonymous
// struct r holds on its element path, as registerStructKeyed raises a named
// struct's: warnResolvedField on each field the literal's own extraction
// kept, which recurses into the anonymous structs those hold.
func (a *ProjectAnalyzer) warnInlineFields(r *models.TypeShape) {
	walkResolvedLeaves(r, func(leaf *models.TypeShape) {
		if leaf.Kind != models.ShapeStruct || leaf.Struct == nil {
			return
		}
		for i := range leaf.Struct.Fields {
			a.warnResolvedField(&leaf.Struct.Fields[i])
		}
	})
}

// registerInlineRefs registers the refs of an anonymous struct's fields at
// depth, the registration depth of the struct's own fields, each in the file
// its name was written in (registerFieldRefAt), stamping their final names in
// place: a named struct reachable only through an anonymous one registers,
// so its $ref cannot dangle.
func (a *ProjectAnalyzer) registerInlineRefs(fields []models.FieldInfo, astFile *ast.File, filePath string, depth int) {
	for i := range fields {
		a.registerFieldRefAt(&fields[i], astFile, filePath, depth)
	}
}
