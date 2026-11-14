package analyzer

import (
	"go/ast"
	"go/token"
	"maps"
	"net/textproto"
	"slices"
	"strings"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Response headers (issue 71): the names a handler writes to its result's
// Headers field. go-bricks copies every Result.Headers / ResultWithMeta.Headers
// entry onto the success response with Header().Add, on the envelope, raw and
// JOSE paths alike and before the 204/304 no-body short-circuit, so each name
// is documented on the success response whatever its flavor or code.
//
// Inference is deliberately narrow. A missed optional header costs one hint;
// a misattributed one documents a header the route never sends. So only
// string-literal keys that are valid header field names count (a constant,
// variable or call key is ignored silently — a warning would count toward
// --strict), and only writes in the handler's own body: a write inside any
// function literal is skipped, because whether it reaches the wire depends on
// when, or whether, the literal runs. Variables are matched by declaration,
// not by name, so a same-named local in an inner block (or a range variable)
// is not the returned result.

// Header-related names on the framework's result wrappers.
const (
	resultHeadersFieldName = "Headers"
	// contentTypeHeader is dropped: OpenAPI 3.0's Response Object says a
	// Content-Type entry in headers SHALL be ignored.
	contentTypeHeader = "Content-Type"
	// headerNameSymbols are the non-alphanumeric tchars of RFC 9110's token,
	// the grammar of a header field name.
	headerNameSymbols = "!#$%&'*+-.^_`|~"
)

// headerWriteMethods are the http.Header methods whose first argument is a
// header name that ends up on the response. Del removes one and is ignored.
var headerWriteMethods = map[string]struct{}{"Set": {}, "Add": {}}

// varDeclKey identifies one local variable declaration: its name and the
// position it enters scope at (binding.from). Two variables with one name in
// different blocks have different keys.
type varDeclKey struct {
	name string
	from token.Pos
}

// nameSet is a set of header names.
type nameSet map[string]struct{}

// headerCollector accumulates, in one scoped walk of a handler body, every
// header write keyed by the declaration it targets; extractResponseHeaders
// keeps only those that reach a returned result.
type headerCollector struct {
	analyzer      *ProjectAnalyzer
	serverAliases map[string]struct{}
	// scopes is the binding chain in effect at the node being visited.
	scopes constScopes
	// closures are the spans of the body's function literals, whose nodes
	// are skipped.
	closures []ast.Node
	// results are the declarations returned as the first result with a nil
	// error, or by a bare return of the first named result.
	results map[varDeclKey]struct{}
	// fieldWrites are the names written to <decl>.Headers (or by a wrapper
	// literal bound to <decl>); fieldLinks the local header maps assigned to
	// <decl>.Headers.
	fieldWrites map[varDeclKey]nameSet
	fieldLinks  map[varDeclKey]map[varDeclKey]struct{}
	// mapWrites are the names written to a local header map variable.
	mapWrites map[varDeclKey]nameSet
	// returned are the names of wrapper literals returned directly with a nil
	// error, and returnedLinks the local maps such a literal's Headers holds.
	returned      nameSet
	returnedLinks map[varDeclKey]struct{}
	// reassigned are the declarations a plain assignment targets after they
	// are declared; a linked map among them contributes nothing, since which
	// map the result ends up holding depends on the order of the writes.
	reassigned map[varDeclKey]struct{}
	// terminals are the body's statement lists that end in a return.
	terminals []terminalBlock
	// errResult is the declaration of the second named result (an empty or
	// blank name, which no identifier resolves to, when there is none), and errAssigned whether the body ever assigns it
	// anything but the literal nil: a bare return then counts nothing.
	errResult   varDeclKey
	errAssigned bool
	// successReturns are the positions of the body's returns, outside
	// function literals, that serve a result: a nil-error return, or a bare
	// return when it counts the named result.
	successReturns []token.Pos
}

// terminalBlock is one statement list (a block or a case clause body) whose
// last statement is a return, and whether that return carries a non-nil
// error, which makes go-bricks drop the result.
type terminalBlock struct {
	from, to    token.Pos
	errorReturn bool
}

// extractResponseHeaders returns the canonical, sorted, deduplicated header
// names a handler writes to its result, or nil when it writes none or its
// declared first result is not a Result wrapper (by value or behind one
// pointer, which go-bricks serves alike: the wrappers' provider methods have
// value receivers).
//
// The result is every variable a return statement in the handler's own body
// gives as its first result, as is or with its address taken, with the bare
// nil identifier as its error (go-bricks drops the result whenever the error
// is non-nil), plus the first named result when the body has a bare return.
// A write counts when it targets one of those declarations, in a branch or
// not:
//
//	res.Headers = <composite literal>      // each string-literal key
//	res.Headers.Set("K", v) / .Add("K", v)
//	res.Headers["K"] = v
//	res := server.Result[T]{Headers: <composite literal>, ...}
//
// and so does the Headers element of a wrapper literal returned directly with
// a nil error. Where Headers is given a local map variable instead
// (res.Headers = h, server.Result[T]{Headers: h}), the names written to that
// map's declaration count — h := http.Header{"K": ...}, h.Set/Add("K", ...),
// h["K"] = ... — wherever in the body they are written.
func (a *ProjectAnalyzer) extractResponseHeaders(funcDecl *ast.FuncDecl, serverAliases map[string]struct{}) []string {
	if funcDecl.Body == nil || a.resultWrapperKind(funcDecl.Type.Results, serverAliases) == models.ResultWrapperNone {
		return nil
	}
	c := &headerCollector{
		analyzer:      a,
		serverAliases: serverAliases,
		closures:      functionLiterals(funcDecl.Body),
		results:       map[varDeclKey]struct{}{},
		fieldWrites:   map[varDeclKey]nameSet{},
		fieldLinks:    map[varDeclKey]map[varDeclKey]struct{}{},
		mapWrites:     map[varDeclKey]nameSet{},
		returned:      nameSet{},
		returnedLinks: map[varDeclKey]struct{}{},
		reassigned:    map[varDeclKey]struct{}{},
		terminals:     terminalBlocks(funcDecl.Body),
		errResult:     varDeclKey{name: secondNamedResult(funcDecl), from: funcDecl.Body.Pos()},
	}
	outer := funcTypeBindings(funcDecl.Type, receiverVarName(funcDecl.Recv), funcDecl.Body.Pos())
	// The assignments go first: whether a bare return serves the named result
	// (errAssigned) decides which returns rescue a write from an error path.
	c.walk(funcDecl.Body, outer, func(n ast.Node) {
		if assign, ok := n.(*ast.AssignStmt); ok {
			c.noteAssignments(assign)
		}
	})
	c.successReturns = successReturns(funcDecl.Body, !c.errAssigned)
	bare := false
	c.walk(funcDecl.Body, outer, func(n ast.Node) {
		if ret, ok := n.(*ast.ReturnStmt); ok && len(ret.Results) == 0 {
			bare = true
		}
		c.visit(n)
	})
	if named := firstNamedResult(funcDecl); bare && !c.errAssigned && named != "" && named != "_" {
		c.results[varDeclKey{name: named, from: funcDecl.Body.Pos()}] = struct{}{}
	}
	return c.names()
}

// functionLiterals returns every function literal in body, outermost ones
// included once each (nested literals lie inside them anyway).
func functionLiterals(body *ast.BlockStmt) []ast.Node {
	var lits []ast.Node
	ast.Inspect(body, func(n ast.Node) bool {
		if lit, ok := n.(*ast.FuncLit); ok {
			lits = append(lits, lit)
			return false
		}
		return true
	})
	return lits
}

// secondNamedResult returns the name of a function's second result when its
// results are named, or "".
func secondNamedResult(funcDecl *ast.FuncDecl) string {
	var names []string
	for _, field := range funcDecl.Type.Results.List {
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}
	if len(names) < 2 {
		return ""
	}
	return names[1]
}

// walk visits every node of body outside function literals with the binding
// chain in effect there set on the collector.
func (c *headerCollector) walk(body *ast.BlockStmt, outer map[string]binding, visit func(ast.Node)) {
	walkScoped(body, outer, func(n ast.Node, scopes constScopes) {
		if c.inClosure(n) {
			return
		}
		c.scopes = scopes
		visit(n)
	})
}

// successReturns returns the positions of body's returns, outside function
// literals, that serve a result: those with the bare nil identifier as their
// error, and bare returns when bareServes.
func successReturns(body *ast.BlockStmt, bareServes bool) []token.Pos {
	var out []token.Pos
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if returnsNilError(s) || (bareServes && len(s.Results) == 0) {
				out = append(out, s.Pos())
			}
		}
		return true
	})
	return out
}

// terminalBlocks returns every statement list in body whose last
// statement is a return. A function literal's lists are
// included but harmless: no recorded write lies inside one.
func terminalBlocks(body *ast.BlockStmt) []terminalBlock {
	var out []terminalBlock
	ast.Inspect(body, func(n ast.Node) bool {
		list := stmtList(n)
		if len(list) == 0 {
			return true
		}
		if ret, ok := list[len(list)-1].(*ast.ReturnStmt); ok {
			errorReturn := len(ret.Results) == 2 && !returnsNilError(ret)
			out = append(out, terminalBlock{from: n.Pos(), to: n.End(), errorReturn: errorReturn})
		}
		return true
	})
	return out
}

// stmtList returns the statement list a block or a switch/select clause
// holds, or nil for any other node.
func stmtList(n ast.Node) []ast.Stmt {
	switch b := n.(type) {
	case *ast.BlockStmt:
		return b.List
	case *ast.CaseClause:
		return b.Body
	case *ast.CommClause:
		return b.Body
	}
	return nil
}

// onErrorPath reports whether the innermost statement list enclosing n that
// ends in a return ends in one with a non-nil error, and no return serving a
// result (successReturns) follows n inside that list, nested statements
// included: every path from such a write ends in a response go-bricks
// answers with the error, so it never reaches the success response. A write
// that reaches an error return by other control flow (a goto, a loop exit)
// is not detected. Any serving return rescues the write, whichever variable
// it returns.
func (c *headerCollector) onErrorPath(n ast.Node) bool {
	var innermost *terminalBlock
	for i := range c.terminals {
		t := &c.terminals[i]
		if t.from <= n.Pos() && n.End() <= t.to && (innermost == nil || t.from > innermost.from) {
			innermost = t
		}
	}
	if innermost == nil || !innermost.errorReturn {
		return false
	}
	for _, pos := range c.successReturns {
		if n.End() <= pos && pos < innermost.to {
			return false
		}
	}
	return true
}

// inClosure reports whether n lies inside one of the body's function literals.
func (c *headerCollector) inClosure(n ast.Node) bool {
	for _, lit := range c.closures {
		if lit.Pos() <= n.Pos() && n.End() <= lit.End() {
			return true
		}
	}
	return false
}

// names merges the writes that reach a returned result.
func (c *headerCollector) names() []string {
	out := nameSet{}
	maps.Copy(out, c.returned)
	c.addLinked(out, c.returnedLinks)
	for key := range c.results {
		maps.Copy(out, c.fieldWrites[key])
		c.addLinked(out, c.fieldLinks[key])
	}
	if len(out) == 0 {
		return nil
	}
	return slices.Sorted(maps.Keys(out))
}

// addLinked adds the names written to each linked local map.
func (c *headerCollector) addLinked(out nameSet, links map[varDeclKey]struct{}) {
	for key := range links {
		if _, reassigned := c.reassigned[key]; reassigned {
			continue
		}
		maps.Copy(out, c.mapWrites[key])
	}
}

// visit records one node of the handler's own body.
func (c *headerCollector) visit(n ast.Node) {
	if ret, ok := n.(*ast.ReturnStmt); ok {
		c.recordReturn(ret)
		return
	}
	if c.onErrorPath(n) {
		return
	}
	switch s := n.(type) {
	case *ast.AssignStmt:
		c.recordAssign(s)
	case *ast.DeclStmt:
		c.recordVarDecl(s)
	case *ast.CallExpr:
		c.recordCall(s)
	}
}

// noteAssignments records, in a first walk and wherever the statement sits,
// which existing
// variables it assigns (reassigned) and whether it gives the named error
// result anything but the literal nil (errAssigned). A name the statement
// itself declares enters scope at its end; any other resolves to an earlier
// declaration, so it is assigned, not declared.
func (c *headerCollector) noteAssignments(stmt *ast.AssignStmt) {
	for i, lhs := range stmt.Lhs {
		key, ok := c.identDecl(ast.Unparen(lhs), stmt.End())
		if !ok || key.from == stmt.End() {
			continue
		}
		c.reassigned[key] = struct{}{}
		if key == c.errResult && (len(stmt.Rhs) != len(stmt.Lhs) || !isNilIdent(stmt.Rhs[i])) {
			c.errAssigned = true
		}
	}
}

// isNilIdent reports whether expr is the bare nil identifier.
func isNilIdent(expr ast.Expr) bool {
	id, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && id.Name == "nil"
}

// recordReturn records a nil-error return: its first result's declaration
// joins the results, or a wrapper literal returned directly contributes its
// Headers element.
func (c *headerCollector) recordReturn(ret *ast.ReturnStmt) {
	if !returnsNilError(ret) {
		return
	}
	if key, ok := c.identDecl(stripAddressOf(ret.Results[0]), ret.Pos()); ok {
		c.results[key] = struct{}{}
		return
	}
	c.addWrapperLiteral(ret.Results[0], c.returned, c.returnedLinks)
}

// recordAssign records each target of an assignment.
func (c *headerCollector) recordAssign(stmt *ast.AssignStmt) {
	for i, lhs := range stmt.Lhs {
		var rhs ast.Expr
		if len(stmt.Rhs) == len(stmt.Lhs) {
			rhs = stmt.Rhs[i]
		}
		c.recordTarget(lhs, rhs, stmt.End())
	}
}

// recordTarget records the header names one assignment target receives; rhs
// is nil when the assignment's values do not pair with its targets
// (`a, b := f()`). end is where the statement's own declarations are in
// scope, so a defined name resolves to the variable it declares.
func (c *headerCollector) recordTarget(lhs, rhs ast.Expr, end token.Pos) {
	switch target := ast.Unparen(lhs).(type) {
	case *ast.Ident:
		if key, ok := c.identDecl(target, end); ok && rhs != nil {
			c.bindValue(key, rhs)
		}
	case *ast.SelectorExpr:
		if key, ok := c.headersDecl(target); ok && rhs != nil {
			c.linkOrAdd(rhs, c.set(c.fieldWrites, key), c.links(key))
		}
	case *ast.IndexExpr:
		if into := c.headerMap(target.X); into != nil {
			c.addKey(target.Index, into)
		}
	}
}

// bindValue records what binding a variable to rhs writes: a wrapper
// literal's Headers element for a would-be result, or a header map literal's
// keys for a would-be local map.
func (c *headerCollector) bindValue(key varDeclKey, rhs ast.Expr) {
	if c.addWrapperLiteral(rhs, c.set(c.fieldWrites, key), c.links(key)) {
		return
	}
	if lit, ok := ast.Unparen(rhs).(*ast.CompositeLit); ok {
		c.addLiteralKeys(lit, c.set(c.mapWrites, key))
	}
}

// recordVarDecl records `var x = <expr>` bindings like `x := <expr>`.
func (c *headerCollector) recordVarDecl(stmt *ast.DeclStmt) {
	decl, ok := stmt.Decl.(*ast.GenDecl)
	if !ok || decl.Tok != token.VAR {
		return
	}
	for _, spec := range decl.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok || len(vs.Names) != len(vs.Values) {
			continue
		}
		for i, name := range vs.Names {
			if key, ok := c.identDecl(name, stmt.End()); ok {
				c.bindValue(key, vs.Values[i])
			}
		}
	}
}

// recordCall records the name a res.Headers.Set / .Add call, or one on a
// local header map, writes.
func (c *headerCollector) recordCall(call *ast.CallExpr) {
	method, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return
	}
	if _, writes := headerWriteMethods[method.Sel.Name]; !writes {
		return
	}
	if into := c.headerMap(method.X); into != nil {
		c.addKey(call.Args[0], into)
	}
}

// headerMap returns the set the names written into expr belong to: the
// field writes of <decl>.Headers, or the map writes of a local variable.
// nil when expr is neither.
func (c *headerCollector) headerMap(expr ast.Expr) nameSet {
	switch x := ast.Unparen(expr).(type) {
	case *ast.SelectorExpr:
		if key, ok := c.headersDecl(x); ok {
			return c.set(c.fieldWrites, key)
		}
	case *ast.Ident:
		if key, ok := c.identDecl(x, x.Pos()); ok {
			return c.set(c.mapWrites, key)
		}
	}
	return nil
}

// headersDecl resolves <ident>.Headers to the declaration of ident.
func (c *headerCollector) headersDecl(sel *ast.SelectorExpr) (varDeclKey, bool) {
	if sel.Sel.Name != resultHeadersFieldName {
		return varDeclKey{}, false
	}
	id, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok {
		return varDeclKey{}, false
	}
	return c.identDecl(id, id.Pos())
}

// identDecl resolves expr, when it is an identifier, to the local declaration
// in scope at position at. A package-level or undeclared name resolves to
// nothing.
func (c *headerCollector) identDecl(expr ast.Expr, at token.Pos) (varDeclKey, bool) {
	id, ok := expr.(*ast.Ident)
	if !ok || id.Name == "_" {
		return varDeclKey{}, false
	}
	b, found := c.scopes.resolve(id.Name, at)
	if !found {
		return varDeclKey{}, false
	}
	return varDeclKey{name: id.Name, from: b.from}, true
}

// set returns the name set of key in sets, creating it.
func (c *headerCollector) set(sets map[varDeclKey]nameSet, key varDeclKey) nameSet {
	if sets[key] == nil {
		sets[key] = nameSet{}
	}
	return sets[key]
}

// links returns the local maps linked to key's Headers, creating the set.
func (c *headerCollector) links(key varDeclKey) map[varDeclKey]struct{} {
	if c.fieldLinks[key] == nil {
		c.fieldLinks[key] = map[varDeclKey]struct{}{}
	}
	return c.fieldLinks[key]
}

// linkOrAdd records a value given to a Headers field: a composite literal's
// keys into into, or a local map variable into links.
func (c *headerCollector) linkOrAdd(value ast.Expr, into nameSet, links map[varDeclKey]struct{}) {
	switch v := ast.Unparen(value).(type) {
	case *ast.CompositeLit:
		c.addLiteralKeys(v, into)
	case *ast.Ident:
		if key, ok := c.identDecl(v, v.Pos()); ok {
			links[key] = struct{}{}
		}
	}
}

// addWrapperLiteral records the Headers element of a server.Result[T]{...} /
// server.ResultWithMeta[T]{...} literal, its address taken or not, and
// reports whether expr was such a literal. Anything else, including a literal
// whose qualifier is a local shadowing the server package, adds nothing.
func (c *headerCollector) addWrapperLiteral(expr ast.Expr, into nameSet, links map[varDeclKey]struct{}) bool {
	lit, ok := stripAddressOf(expr).(*ast.CompositeLit)
	if !ok || !c.isWrapperLiteral(lit) {
		return false
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := kv.Key.(*ast.Ident); ok && key.Name == resultHeadersFieldName {
			c.linkOrAdd(kv.Value, into, links)
		}
	}
	return true
}

// isWrapperLiteral reports whether lit's type is a Result wrapper whose
// server qualifier still names the imported package at that position.
func (c *headerCollector) isWrapperLiteral(lit *ast.CompositeLit) bool {
	typ := unwrapGenericType(lit.Type)
	if !c.analyzer.isResultWrapper(typ, c.serverAliases) {
		return false
	}
	pkg := typ.(*ast.SelectorExpr).X.(*ast.Ident)
	_, shadowed := c.scopes.resolve(pkg.Name, pkg.Pos())
	return !shadowed
}

// addLiteralKeys records each key of a header map composite literal. Its type
// expression is not inspected: http.Header{...}, an aliased net/http's
// Header{...} and map[string][]string{...} all count.
func (c *headerCollector) addLiteralKeys(lit *ast.CompositeLit, into nameSet) {
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			c.addKey(kv.Key, into)
		}
	}
}

// addKey records a header name written as a string literal, in the canonical
// form the framework's Header().Add sends it under. Any other key expression,
// a name that is not a valid header field name and Content-Type add nothing.
func (c *headerCollector) addKey(expr ast.Expr, into nameSet) {
	value, ok := stringLiteralValue(expr)
	if !ok || !validHeaderName(value) {
		return
	}
	name := textproto.CanonicalMIMEHeaderKey(value)
	if name == contentTypeHeader {
		return
	}
	into[name] = struct{}{}
}

// validHeaderName reports whether name is an RFC 9110 token, the grammar of
// a header field name: one or more ASCII letters, digits or tchar symbols.
func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !isTokenChar(r) {
			return false
		}
	}
	return true
}

// isTokenChar reports whether r is an RFC 9110 tchar.
func isTokenChar(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	}
	return strings.ContainsRune(headerNameSymbols, r)
}

// stripAddressOf sheds parentheses and one address-of operator.
func stripAddressOf(expr ast.Expr) ast.Expr {
	expr = ast.Unparen(expr)
	if u, ok := expr.(*ast.UnaryExpr); ok && u.Op == token.AND {
		return ast.Unparen(u.X)
	}
	return expr
}
