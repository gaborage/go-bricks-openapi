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
// header write with the node it sits at and the declaration it targets, and
// every return that serves a result; names keeps a write only when a return
// serving its target is reachable after it.
type headerCollector struct {
	analyzer      *ProjectAnalyzer
	serverAliases map[string]struct{}
	body          *ast.BlockStmt
	// scopes is the binding chain in effect at the node being visited, and
	// cur that node: every write recorded is attributed to it.
	scopes constScopes
	cur    ast.Node
	// closures are the spans of the body's function literals, whose nodes
	// are skipped.
	closures []ast.Node
	// writes are the recorded writes, one per node, target and kind.
	writes map[writeID]*headerWrite
	// served maps each return that serves a result to what it returns.
	served map[*ast.ReturnStmt]servedValue
	// returned are the names of wrapper literals returned directly with a nil
	// error.
	returned nameSet
	// reassigned are the declarations a plain assignment targets after they
	// are declared; a linked map among them contributes nothing, since which
	// map the result ends up holding depends on the order of the writes.
	reassigned map[varDeclKey]struct{}
	// errResult is the declaration of the second named result (an empty or
	// blank name, which no identifier resolves to, when there is none), and
	// errAssigned whether the body ever assigns it anything but the literal
	// nil: a bare return then serves nothing. namedResult is the first named
	// result's declaration, which a serving bare return returns.
	errResult   varDeclKey
	errAssigned bool
	namedResult varDeclKey
}

// writeID identifies the writes one node makes to one declaration: to its
// Headers field (toMap false) or to it as a header map variable (toMap true).
type writeID struct {
	node  ast.Node
	key   varDeclKey
	toMap bool
}

// headerWrite is what one node writes to one declaration: header names, and
// for a Headers field the local maps it is given.
type headerWrite struct {
	writeID
	names nameSet
	links map[varDeclKey]struct{}
}

// servedValue is what a serving return returns: a declaration (hasKey), or a
// wrapper literal whose Headers element holds the local maps in links.
type servedValue struct {
	key    varDeclKey
	hasKey bool
	links  map[varDeclKey]struct{}
}

// extractResponseHeaders returns the canonical, sorted, deduplicated header
// names a handler writes to its result, or nil when it writes none or its
// declared first result is not a Result wrapper (by value or behind one
// pointer, which go-bricks serves alike: the wrappers' provider methods have
// value receivers).
//
// A serving return is one in the handler's own body that gives a variable
// (as is or with its address taken) or a wrapper literal as its first result
// with the bare nil identifier as its error (go-bricks drops the result
// whenever the error is non-nil), or a bare return, which returns the first
// named result, when the named error result is never assigned anything but
// the literal nil. A write to a variable counts only when a serving return of
// that same declaration is reachable after it (reaches):
//
//	res.Headers = <composite literal>      // each string-literal key
//	res.Headers.Set("K", v) / .Add("K", v)
//	res.Headers["K"] = v
//	res := server.Result[T]{Headers: <composite literal>, ...}
//
// and the Headers element of a wrapper literal returned by a serving return
// counts too. Where Headers is given a local map variable instead
// (res.Headers = h, server.Result[T]{Headers: h}), a write to that map's
// declaration — h := http.Header{"K": ...}, h.Set/Add("K", ...), h["K"] = ...
// — counts when a serving return of a result holding it is reachable after
// the write.
func (a *ProjectAnalyzer) extractResponseHeaders(funcDecl *ast.FuncDecl, serverAliases map[string]struct{}) []string {
	if funcDecl.Body == nil || a.resultWrapperKind(funcDecl.Type.Results, serverAliases) == models.ResultWrapperNone {
		return nil
	}
	c := &headerCollector{
		analyzer:      a,
		serverAliases: serverAliases,
		body:          funcDecl.Body,
		closures:      functionLiterals(funcDecl.Body),
		writes:        map[writeID]*headerWrite{},
		served:        map[*ast.ReturnStmt]servedValue{},
		returned:      nameSet{},
		reassigned:    map[varDeclKey]struct{}{},
		errResult:     varDeclKey{name: secondNamedResult(funcDecl), from: funcDecl.Body.Pos()},
		namedResult:   varDeclKey{name: firstNamedResult(funcDecl), from: funcDecl.Body.Pos()},
	}
	outer := funcTypeBindings(funcDecl.Type, receiverVarName(funcDecl.Recv), funcDecl.Body.Pos())
	// The assignments go first: whether a bare return serves the named result
	// (errAssigned) must be known when the returns are recorded.
	c.walk(funcDecl.Body, outer, func(n ast.Node) {
		if assign, ok := n.(*ast.AssignStmt); ok {
			c.noteAssignments(assign)
		}
	})
	c.walk(funcDecl.Body, outer, c.visit)
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

// followingStmts returns the statements that can run after n without a
// jump: in every statement list enclosing n, the statements after the one
// holding n. A switch or select clause is never one of them — its siblings
// are other arms — and neither is the other arm of an if/else, which sits in
// no list with the arm holding n. A write in the initializer of an if,
// switch, type switch or for statement runs before every arm, so that
// statement's bodies follow it (initBodies). Statements before n in an
// enclosing loop body could run on a later iteration but are not counted, so
// a write is dropped rather than credited to them; nor are statements a goto
// reaches. Levels are taken innermost first, and a list that ends in a
// return stops the climb: control never leaves it to reach the enclosing
// statements.
func followingStmts(body *ast.BlockStmt, n ast.Node) []ast.Stmt {
	var levels []followLevel // outermost first
	ast.Inspect(body, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		if bodies := initBodies(node, n); bodies != nil {
			levels = append(levels, followLevel{tail: bodies})
		}
		list := stmtList(node)
		for i, stmt := range list {
			if within(stmt, n) {
				levels = append(levels, listLevel(list, i))
			}
		}
		return true
	})
	var out []ast.Stmt
	for i := len(levels) - 1; i >= 0; i-- {
		out = appendFollowing(out, levels[i].tail)
		if levels[i].terminal {
			break
		}
	}
	return out
}

// listLevel is the level of followingStmts' climb at a statement list whose
// i-th statement holds n: the statements after it, terminal when the list
// ends in a return. When that statement is a switch clause whose body ends
// in fallthrough, the level is instead the body of the next clause, and of
// each further one the chain falls through to, terminal when the last of
// them ends in a return; no other clause ever follows.
func listLevel(list []ast.Stmt, i int) followLevel {
	var tail []ast.Stmt
	last := list[len(list)-1]
	for j := i; j+1 < len(list) && endsInFallthrough(list[j]); j++ {
		next := list[j+1].(*ast.CaseClause)
		tail = append(tail, next.Body...)
		last = lastStmt(next.Body)
	}
	if tail == nil {
		tail = list[i+1:]
	}
	_, ends := last.(*ast.ReturnStmt)
	return followLevel{tail: tail, terminal: ends}
}

// lastStmt returns the last statement of list, or nil when it is empty.
func lastStmt(list []ast.Stmt) ast.Stmt {
	if len(list) == 0 {
		return nil
	}
	return list[len(list)-1]
}

// within reports whether node lies inside outer.
func within(outer, node ast.Node) bool {
	return outer.Pos() <= node.Pos() && node.End() <= outer.End()
}

// followLevel is one level of followingStmts' climb: the statements that
// follow n there, and whether that level's list ends in a return.
type followLevel struct {
	tail     []ast.Stmt
	terminal bool
}

// initBodies returns the bodies of an if, switch, type switch or for
// statement whose initializer holds n — every arm, since the initializer
// runs before all of them — or nil.
func initBodies(node, n ast.Node) []ast.Stmt {
	var init ast.Stmt
	var bodies []ast.Stmt
	switch s := node.(type) {
	case *ast.IfStmt:
		init, bodies = s.Init, []ast.Stmt{s.Body, s.Else}
	case *ast.SwitchStmt:
		init, bodies = s.Init, []ast.Stmt{s.Body}
	case *ast.TypeSwitchStmt:
		init, bodies = s.Init, []ast.Stmt{s.Body}
	case *ast.ForStmt:
		init, bodies = s.Init, []ast.Stmt{s.Body}
	}
	if init == nil || n.Pos() < init.Pos() || init.End() < n.End() {
		return nil
	}
	return slices.DeleteFunc(bodies, func(b ast.Stmt) bool { return b == nil })
}

// appendFollowing appends the statements of list that are not switch or
// select clauses.
func appendFollowing(out, list []ast.Stmt) []ast.Stmt {
	for _, stmt := range list {
		switch stmt.(type) {
		case *ast.CaseClause, *ast.CommClause:
			continue
		}
		out = append(out, stmt)
	}
	return out
}

// servingAfter returns the serving returns in the statements that can run
// after n (followingStmts), at any depth. A return inside a function literal
// is never in served (the walk skips literals), so it is never returned.
func (c *headerCollector) servingAfter(n ast.Node) []*ast.ReturnStmt {
	var out []*ast.ReturnStmt
	for _, stmt := range followingStmts(c.body, n) {
		ast.Inspect(stmt, func(node ast.Node) bool {
			if ret, ok := node.(*ast.ReturnStmt); ok {
				if _, serving := c.served[ret]; serving {
					out = append(out, ret)
				}
			}
			return true
		})
	}
	return out
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

// names keeps the writes a serving return of their target can follow: first
// the writes to a Headers field, which also decide which maps each serving
// return's result holds, then the writes to a local map.
func (c *headerCollector) names() []string {
	out := nameSet{}
	maps.Copy(out, c.returned)
	holders := map[*ast.ReturnStmt]map[varDeclKey][]ast.Node{}
	for _, w := range c.writes {
		if !w.toMap {
			c.keepFieldWrite(w, out, holders)
		}
	}
	for _, w := range c.writes {
		if w.toMap && c.mapReaches(w, holders) {
			maps.Copy(out, w.names)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return slices.Sorted(maps.Keys(out))
}

// keepFieldWrite keeps a write to <decl>.Headers when a serving return of
// decl can follow it, and records the maps the write gives the field as held
// by each such return's result, with the node that gives it.
func (c *headerCollector) keepFieldWrite(w *headerWrite, out nameSet, holders map[*ast.ReturnStmt]map[varDeclKey][]ast.Node) {
	for _, ret := range c.servingAfter(w.node) {
		if s := c.served[ret]; !s.hasKey || s.key != w.key {
			continue
		}
		maps.Copy(out, w.names)
		if holders[ret] == nil {
			holders[ret] = map[varDeclKey][]ast.Node{}
		}
		for m := range w.links {
			holders[ret][m] = append(holders[ret][m], w.node)
		}
	}
}

// mapReaches reports whether a write to a local map can be followed by a
// serving return whose result holds that map: a returned literal holding it,
// or a variable a link the same return follows gave it to (holders), where
// the link and the write lie on one path — one can run after the other — in
// either order. A map that is ever reassigned holds nothing.
func (c *headerCollector) mapReaches(w *headerWrite, holders map[*ast.ReturnStmt]map[varDeclKey][]ast.Node) bool {
	if _, reassigned := c.reassigned[w.key]; reassigned {
		return false
	}
	for _, ret := range c.servingAfter(w.node) {
		if _, literal := c.served[ret].links[w.key]; literal {
			return true
		}
		for _, link := range holders[ret][w.key] {
			if c.onePath(link, w.node) {
				return true
			}
		}
	}
	return false
}

// onePath reports whether a and b are one statement, or b can run after a,
// or a after b (followingStmts). Statements that connect only across loop
// iterations are not followed, so such a pair is not on one path.
func (c *headerCollector) onePath(a, b ast.Node) bool {
	// One statement doing both (h["K"], res.Headers = v, h) is one path.
	return a == b || follows(c.body, a, b) || follows(c.body, b, a)
}

// follows reports whether later lies in a statement that can run after n.
func follows(body *ast.BlockStmt, n, later ast.Node) bool {
	for _, stmt := range followingStmts(body, n) {
		if within(stmt, later) {
			return true
		}
	}
	return false
}

// visit records one node of the handler's own body.
func (c *headerCollector) visit(n ast.Node) {
	c.cur = n
	switch s := n.(type) {
	case *ast.ReturnStmt:
		c.recordReturn(s)
	case *ast.AssignStmt:
		c.recordAssign(s)
	case *ast.DeclStmt:
		c.recordVarDecl(s)
	case *ast.CallExpr:
		c.recordCall(s)
	}
}

// noteAssignments records, in a first walk and wherever the statement sits,
// which existing variables it assigns (reassigned) and whether it gives the
// named error result anything but the literal nil (errAssigned). A name the
// statement itself declares enters scope at its end; any other resolves to
// an earlier declaration, so it is assigned, not declared.
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

// recordReturn records a serving return: what declaration it returns, or the
// Headers element of a wrapper literal it returns directly.
func (c *headerCollector) recordReturn(ret *ast.ReturnStmt) {
	if len(ret.Results) == 0 {
		if !c.errAssigned && c.namedResult.name != "" && c.namedResult.name != "_" {
			c.served[ret] = servedValue{key: c.namedResult, hasKey: true}
		}
		return
	}
	if !returnsNilError(ret) {
		return
	}
	if key, ok := c.identDecl(stripAddressOf(ret.Results[0]), ret.Pos()); ok {
		c.served[ret] = servedValue{key: key, hasKey: true}
		return
	}
	links := map[varDeclKey]struct{}{}
	if c.addWrapperLiteral(ret.Results[0], c.returned, links) {
		c.served[ret] = servedValue{links: links}
	}
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
			w := c.write(key, false)
			c.linkOrAdd(rhs, w.names, w.links)
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
	if lit, ok := stripAddressOf(rhs).(*ast.CompositeLit); ok && c.isWrapperLiteral(lit) {
		w := c.write(key, false)
		c.addWrapperLiteral(lit, w.names, w.links)
		return
	}
	if lit, ok := ast.Unparen(rhs).(*ast.CompositeLit); ok {
		c.addLiteralKeys(lit, c.write(key, true).names)
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
// writes to <decl>.Headers, or to a local map variable. nil when expr is
// neither.
func (c *headerCollector) headerMap(expr ast.Expr) nameSet {
	switch x := ast.Unparen(expr).(type) {
	case *ast.SelectorExpr:
		if key, ok := c.headersDecl(x); ok {
			return c.write(key, false).names
		}
	case *ast.Ident:
		if key, ok := c.identDecl(x, x.Pos()); ok {
			return c.write(key, true).names
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

// write returns the write the visited node makes to key, creating it.
func (c *headerCollector) write(key varDeclKey, toMap bool) *headerWrite {
	id := writeID{node: c.cur, key: key, toMap: toMap}
	if c.writes[id] == nil {
		c.writes[id] = &headerWrite{writeID: id, names: nameSet{}, links: map[varDeclKey]struct{}{}}
	}
	return c.writes[id]
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
