package analyzer

import (
	"go/ast"
	"go/token"
)

// binding is one name declared in a lexical block. A binding shadows any
// outer binding of the same name from `from` onward, whether or not it carries
// a resolvable string value: a local `const n = 42`, a `var p = compute()` and
// a `p := f()` all hide a package-level constant of that name, so resolution
// must stop at them rather than fall through to the outer value.
type binding struct {
	value string
	// ok reports whether the binding has a resolvable string-constant value.
	ok bool
	// from is the position the name enters scope: the end of its declaration.
	// Go scopes a block-level declaration from there to the end of the block,
	// so a registration written above the declaration must not see it.
	from token.Pos
}

// constScopes is the lexical chain of block-level bindings in effect at a point
// in a walked function body: outermost first, innermost last. The package-level
// constant map (ProjectAnalyzer.constants) is the implicit outermost scope and
// is consulted only when no local binding of the name is visible.
//
// A chain is treated as a value: push copies rather than appending in place, so
// two sibling blocks pushed onto the same parent never see each other's
// bindings.
type constScopes []map[string]binding

// resolve reports the innermost binding of name that is in scope at position
// at. found is false when no local block binds the name at that point, which is
// the only case where the caller may fall back to the package-level map.
func (s constScopes) resolve(name string, at token.Pos) (b binding, found bool) {
	for i := len(s) - 1; i >= 0; i-- {
		if candidate, ok := s[i][name]; ok && candidate.from <= at {
			return candidate, true
		}
	}
	return binding{}, false
}

// push returns the chain extended by one scope. An empty scope is a no-op, and
// the copy is exact-capacity so a later push by a sibling cannot overwrite this
// chain's tail.
func (s constScopes) push(scope map[string]binding) constScopes {
	if len(scope) == 0 {
		return s
	}
	extended := make(constScopes, len(s), len(s)+1)
	copy(extended, s)
	return append(extended, scope)
}

// scopeVisitor walks a function body maintaining the lexical binding chain in
// effect at each node and hands every node to visit with that chain.
type scopeVisitor struct {
	scopes constScopes
	visit  func(ast.Node, constScopes)
}

// walkScoped walks body, invoking visit for every node with the binding chain
// in effect there. outer is the enclosing function signature's scope, in effect
// for the whole body.
func walkScoped(body *ast.BlockStmt, outer map[string]binding, visit func(ast.Node, constScopes)) {
	if body == nil {
		return
	}
	ast.Walk(&scopeVisitor{scopes: constScopes(nil).push(outer), visit: visit}, body)
}

// Visit implements ast.Visitor. A scope-introducing node is handed to visit
// with its own scope already pushed, and descending into it returns a child
// visitor holding the extended chain — which is what keeps a sibling block from
// seeing this block's bindings.
func (v *scopeVisitor) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		return nil
	}
	scope, introduces := scopeBindings(n)
	if !introduces || len(scope) == 0 {
		v.visit(n, v.scopes)
		return v
	}
	child := &scopeVisitor{scopes: v.scopes.push(scope), visit: v.visit}
	child.visit(n, child.scopes)
	return child
}

// scopeBindings returns the names a node binds in the lexical scope it
// introduces, and whether it introduces one at all. Blocks are the obvious
// case; the bodies of `switch`/`select` clauses are implicit blocks, and the
// init clause of an `if`/`for`/`switch` (and a `range` clause's key/value)
// binds into an implicit block wrapping the whole statement.
func scopeBindings(n ast.Node) (map[string]binding, bool) {
	switch node := n.(type) {
	case *ast.BlockStmt:
		return stmtBindings(node.List), true
	case *ast.CaseClause:
		return stmtBindings(node.Body), true
	case *ast.CommClause:
		return stmtBindings(node.Body), true
	case *ast.IfStmt:
		return stmtBindings([]ast.Stmt{node.Init}), true
	case *ast.ForStmt:
		return stmtBindings([]ast.Stmt{node.Init}), true
	case *ast.SwitchStmt:
		return stmtBindings([]ast.Stmt{node.Init}), true
	case *ast.TypeSwitchStmt:
		return stmtBindings([]ast.Stmt{node.Init, node.Assign}), true
	case *ast.RangeStmt:
		return rangeBindings(node), true
	case *ast.FuncLit:
		return funcTypeBindings(node.Type, "", node.Body.Pos()), true
	}
	return nil, false
}

// funcTypeBindings records the names a function signature binds — its
// parameters, its named results and its receiver — as shadow-only bindings in
// scope for the whole body. None can hold a constant, so they exist only to
// stop resolution falling through to a package-level constant of the same name.
func funcTypeBindings(sig *ast.FuncType, recvVar string, from token.Pos) map[string]binding {
	scope := map[string]binding{}
	if recvVar != "" && recvVar != "_" {
		scope[recvVar] = binding{from: from}
	}
	if sig != nil {
		bindFieldNames(scope, sig.Params, from)
		bindFieldNames(scope, sig.Results, from)
	}
	return scope
}

// bindFieldNames records every named field in a parameter or result list.
func bindFieldNames(scope map[string]binding, list *ast.FieldList, from token.Pos) {
	if list == nil {
		return
	}
	for _, field := range list.List {
		for _, name := range field.Names {
			bind(scope, name, binding{from: from})
		}
	}
}

// rangeBindings records the key/value names a `for ... := range` clause binds.
// They never hold a resolvable constant, so they exist only to shadow.
func rangeBindings(node *ast.RangeStmt) map[string]binding {
	scope := map[string]binding{}
	if node.Tok != token.DEFINE {
		return scope
	}
	for _, expr := range []ast.Expr{node.Key, node.Value} {
		bindIdent(scope, expr, node.Body.Pos())
	}
	return scope
}

// stmtBindings collects the names a statement list binds in its own scope.
// Nested blocks are excluded — each forms its own scope. A `const` bound to a
// string literal is the only resolvable shape; every other declaration is
// recorded as an unresolvable binding so it still shadows.
func stmtBindings(stmts []ast.Stmt) map[string]binding {
	scope := map[string]binding{}
	for _, stmt := range stmts {
		switch typed := stmt.(type) {
		case *ast.DeclStmt:
			bindDecl(scope, typed)
		case *ast.AssignStmt:
			bindShortDecl(scope, typed)
		}
	}
	return scope
}

// bindDecl records a `const` or `var` declaration statement's names.
func bindDecl(scope map[string]binding, stmt *ast.DeclStmt) {
	decl, ok := stmt.Decl.(*ast.GenDecl)
	if !ok || (decl.Tok != token.CONST && decl.Tok != token.VAR) {
		return
	}
	for _, spec := range decl.Specs {
		bindValueSpec(scope, spec, decl.Tok == token.CONST)
	}
}

// bindShortDecl records the names a `:=` statement binds.
func bindShortDecl(scope map[string]binding, stmt *ast.AssignStmt) {
	if stmt.Tok != token.DEFINE {
		return
	}
	for _, lhs := range stmt.Lhs {
		bindIdent(scope, lhs, stmt.End())
	}
}

// bindValueSpec records one const/var spec's names, carrying a string value
// only for a constant initialized by a string literal. The declared type is
// ignored on purpose, so a typed constant (`const p Path = "/x"`) resolves like
// an untyped one.
func bindValueSpec(scope map[string]binding, spec ast.Spec, isConst bool) {
	valueSpec, ok := spec.(*ast.ValueSpec)
	if !ok {
		return
	}
	for i, name := range valueSpec.Names {
		value := ""
		if isConst && i < len(valueSpec.Values) {
			value = stringLiteralValue(valueSpec.Values[i])
		}
		bind(scope, name, binding{value: value, ok: value != "", from: valueSpec.End()})
	}
}

// bindIdent records an identifier as an unresolvable (shadow-only) binding.
func bindIdent(scope map[string]binding, expr ast.Expr, from token.Pos) {
	if ident, ok := expr.(*ast.Ident); ok {
		bind(scope, ident, binding{from: from})
	}
}

// bind records one name, skipping the blank identifier.
func bind(scope map[string]binding, name *ast.Ident, b binding) {
	if name == nil || name.Name == "_" {
		return
	}
	scope[name.Name] = b
}

// collectConstStrings records every name in a const spec bound to a string
// literal. Used for the package-level constant map, whose entries are in scope
// everywhere in the package and so need no position.
func collectConstStrings(spec ast.Spec, into map[string]string) {
	valueSpec, ok := spec.(*ast.ValueSpec)
	if !ok {
		return
	}
	for i, name := range valueSpec.Names {
		if i >= len(valueSpec.Values) {
			continue
		}
		if value := stringLiteralValue(valueSpec.Values[i]); value != "" {
			into[name.Name] = value
		}
	}
}

// stringLiteralValue returns the value of a string-literal expression, or "" if
// the expression is anything else.
func stringLiteralValue(expr ast.Expr) string {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return unquoteLiteral(lit.Value)
	}
	return ""
}
