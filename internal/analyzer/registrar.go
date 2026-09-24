package analyzer

import (
	"go/ast"
	"go/token"
)

// prefixCause says why an unresolved registrar prefix is unresolved. The zero
// value is the original cause: a Group argument — the registrar's own, or an
// ancestor group's — is not a resolvable string.
type prefixCause int

const (
	// causePathDependent: the registrar, or a registrar it derives from, is
	// reassigned inside a nested block, loop or closure, or after a closure
	// that reads it, so which value is in effect depends on the path taken at
	// run time.
	causePathDependent prefixCause = iota + 1
	// causeUntraced: the registrar, or a registrar it derives from, holds a
	// value the walk cannot follow to a Group(...) call — a call result, or a
	// parameter or local that shadows a group registrar.
	causeUntraced
)

// registrarPrefix is a route registrar's accumulated group path prefix.
// resolved is false when the prefix cannot be pinned to one string: routes
// registered on the registrar are Unresolved routes, because emitting them
// under a wrong or missing prefix would put a silently wrong path in the
// document.
type registrarPrefix struct {
	path     string
	resolved bool
	// cause records why an unresolved prefix is unresolved. It is the zero
	// value on every resolved prefix, so a group derived from a registrar
	// inherits the cause of whichever ancestor failed.
	cause prefixCause
}

// pathDependentPrefix is the binding of a registrar whose prefix depends on
// control flow.
func pathDependentPrefix() registrarPrefix {
	return registrarPrefix{cause: causePathDependent}
}

// untracedPrefix is the binding of a registrar whose value the walk cannot
// follow to a Group(...) call.
func untracedPrefix() registrarPrefix {
	return registrarPrefix{cause: causeUntraced}
}

// prefixMap binds registrar parameter names to the group prefix a caller
// handed them: the seed a helper walk starts from.
type prefixMap map[string]registrarPrefix

// declKey identifies one declaration of a name: the name plus the position the
// declaration enters scope (binding.from). Two variables that share a name —
// an outer registrar and an inner `:=` that shadows it — get distinct keys, so
// a registration binds to the variable it lexically refers to. A name no
// enclosing block or the function signature declares keys at token.NoPos.
type declKey struct {
	name string
	from token.Pos
}

// keyAt returns the declaration a name used at position at refers to.
func keyAt(name string, at token.Pos, scopes constScopes) declKey {
	if b, found := scopes.resolve(name, at); found {
		return declKey{name: name, from: b.from}
	}
	return declKey{name: name}
}

// groupCallArg narrows an expression to `<parent>.Group(arg)`, returning the
// parent registrar identifier and the unresolved prefix argument.
func groupCallArg(expr ast.Expr) (parent *ast.Ident, arg ast.Expr, ok bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil, nil, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != groupMethodName || len(call.Args) < 1 {
		return nil, nil, false
	}
	parent, ok = sel.X.(*ast.Ident)
	if !ok {
		return nil, nil, false
	}
	return parent, call.Args[0], true
}

// bindingWrite is one variable a statement writes, with the expression it
// receives. rhs is nil when the statement hands it no expression of its own:
// one value of a multi-value call (`a, b := f()`), or a range clause's
// key or value.
type bindingWrite struct {
	lhs *ast.Ident
	rhs ast.Expr
}

// bindingWrites returns the variables node n writes — by `=` or `:=`, by one
// spec of a `var` declaration with values, or by a `for k, v = range` clause —
// and the position the writes take effect at: the end of the statement or
// spec, or the start of a range body. Any other node writes nothing.
//
// A `var` declaration writes spec by spec, as Go declares it: a grouped
// `var ( a = r.Group("/a"); b = a.Group("/b") )` enters a into scope at the
// end of its own spec, so b's value already reads it. A const spec takes the
// same path: its values are constants, never registrars, so its writes bind
// nothing.
func bindingWrites(n ast.Node) ([]bindingWrite, token.Pos) {
	switch node := n.(type) {
	case *ast.AssignStmt:
		if node.Tok != token.ASSIGN && node.Tok != token.DEFINE {
			return nil, token.NoPos
		}
		return pairWrites(node.Lhs, node.Rhs), node.End()
	case *ast.ValueSpec:
		return specWrites(node), node.End()
	case *ast.RangeStmt:
		if node.Tok != token.ASSIGN {
			return nil, token.NoPos
		}
		return pairWrites([]ast.Expr{node.Key, node.Value}, nil), node.Body.Pos()
	}
	return nil, token.NoPos
}

// specWrites returns the variables one declaration spec gives a value. A spec
// without values leaves its variables at the zero value, which holds no
// registrar binding.
func specWrites(spec *ast.ValueSpec) []bindingWrite {
	if len(spec.Values) == 0 {
		return nil
	}
	names := make([]ast.Expr, len(spec.Names))
	for i, name := range spec.Names {
		names[i] = name
	}
	return pairWrites(names, spec.Values)
}

// pairWrites pairs each assigned identifier with its own right-hand side when
// the two lists line up one to one. Blank and non-identifier targets are
// skipped.
func pairWrites(lhs, rhs []ast.Expr) []bindingWrite {
	writes := make([]bindingWrite, 0, len(lhs))
	for i, expr := range lhs {
		ident, ok := expr.(*ast.Ident)
		if !ok || ident.Name == "_" {
			continue
		}
		write := bindingWrite{lhs: ident}
		if len(rhs) == len(lhs) {
			write.rhs = rhs[i]
		}
		writes = append(writes, write)
	}
	return writes
}

// writePositions records, for every variable the body writes, the position
// each write takes effect at. A loop or closure consults it: a registrar it
// captures that is written while it may still run cannot be pinned anywhere
// inside it, not even above the write, which a second iteration (or a later
// call) reaches holding the new value. The effect position, not the start of
// the statement, is what places a write after a closure created in its own
// right-hand side (`api = m.wrap(api, func() {...})`), which may run once the
// new value is in place.
func writePositions(body *ast.BlockStmt, outer map[string]binding) map[declKey][]token.Pos {
	positions := map[declKey][]token.Pos{}
	walkScoped(body, outer, func(n ast.Node, scopes constScopes) {
		writes, at := bindingWrites(n)
		for _, write := range writes {
			key := keyAt(write.lhs.Name, at, scopes)
			positions[key] = append(positions[key], at)
		}
	})
	return positions
}

// registrarFrame is the registrar state of one lexical scope of the walked
// body. Frames mirror the scopes the scoped walk pushes (functionBlocks.scopeOf),
// so a write can tell whether it is straight-line code in the block that
// declares the registrar or a reassignment from a nested one. A select statement gets a frame too,
// declaring nothing, so its body — the statement its clauses branch from — is
// never mistaken for a bare block.
type registrarFrame struct {
	node   ast.Node // the scope-introducing node; nil for the function-level frame
	parent *registrarFrame
	// scope is what the node declares, as functionBlocks.scopeOf reports it.
	scope map[string]binding
	// values holds the registrar bindings written in this scope: those of the
	// registrars it declares, and those of outer registrars it reassigns.
	values map[declKey]registrarPrefix
	// branchWrites holds the outer registrars one branch of this statement
	// reassigned. A sibling branch still sees the value from before the
	// statement; they turn path-dependent once the statement is left.
	branchWrites map[declKey]bool
	// fallthroughWrites holds the outer registrars a case clause that ends in
	// fallthrough reassigned, until the next clause is entered: that clause
	// runs both after it and on its own match, so they are path-dependent in
	// it.
	fallthroughWrites map[declKey]bool
	// repeatable marks a loop or a function literal: code that may run any
	// number of times, so an outer registrar reassigned inside it has no
	// single prefix anywhere inside it.
	repeatable bool
}

// declares reports whether this frame is the scope that declares key. The
// function-level frame also owns every name no block declares (package-level
// variables), so it declares everything that reaches it.
func (f *registrarFrame) declares(key declKey) bool {
	if f.parent == nil {
		return true
	}
	b, ok := f.scope[key.name]
	return ok && b.from == key.from
}

// set writes a binding into this frame.
func (f *registrarFrame) set(key declKey, value registrarPrefix) {
	if f.values == nil {
		f.values = map[declKey]registrarPrefix{}
	}
	f.values[key] = value
}

// reassigned records that a scope nested in f reassigned key. From here on
// key is path-dependent in f, since the nested scope may not have run —
// unless the nested scope was one branch of f, whose sibling branches still
// see the value from before f, so key turns path-dependent only when f is
// left.
func (f *registrarFrame) reassigned(key declKey, inBranch bool) {
	if !inBranch {
		f.set(key, pathDependentPrefix())
		return
	}
	if f.branchWrites == nil {
		f.branchWrites = map[declKey]bool{}
	}
	f.branchWrites[key] = true
}

// carry takes over the writes a bare block nested in f made to registrars
// declared outside it. A bare block has no branches of its own — an if keeps
// its own frame, and a switch or select body sits under its statement's — so
// its writes are all in values.
func (f *registrarFrame) carry(block *registrarFrame) {
	for key, value := range block.values {
		if !block.declares(key) {
			f.set(key, value)
		}
	}
}

// fellThrough records that a case clause of this switch body reassigned key
// and then fell through to the next clause.
func (f *registrarFrame) fellThrough(key declKey) {
	if f.fallthroughWrites == nil {
		f.fallthroughWrites = map[declKey]bool{}
	}
	f.fallthroughWrites[key] = true
}

// enter starts child, the frame of a scope nested in f. After a case clause
// that fell through, child is the next clause, so what that clause reassigned
// is path-dependent in it; the chain continues if child falls through too.
func (f *registrarFrame) enter(child *registrarFrame) {
	for key := range f.fallthroughWrites {
		child.set(key, pathDependentPrefix())
	}
	f.fallthroughWrites = nil
}

// written returns every key this frame holds a write of, directly or through
// one of its branches.
func (f *registrarFrame) written() map[declKey]bool {
	keys := make(map[declKey]bool, len(f.values)+len(f.branchWrites))
	for key := range f.values {
		keys[key] = true
	}
	for key := range f.branchWrites {
		keys[key] = true
	}
	return keys
}

// pendingWrite is a registrar write waiting for its statement to finish: Go
// evaluates a right-hand side before assigning, so a call inside it still
// sees the binding from before the statement.
type pendingWrite struct {
	at    token.Pos // where the write takes effect
	frame *registrarFrame
	key   declKey
	value registrarPrefix
}

// registrarTracker holds the registrar bindings in effect at the current point
// of a walk over one function body. The walk visits nodes in source order and
// hands each one to advance, so a registration reads the binding written by
// the last write above it in its own scope chain — not the last one in the
// body.
//
// A write is resolved exactly only as straight-line code in the block that
// declares the registrar (an if or switch header counts as the enclosing
// block, since it always runs, and so does a bare `{ }` block nested in it).
// A reassignment from a nested if, switch or select block stands
// inside that block, is invisible to its sibling branches (an else, a later
// case not reached by fallthrough — one that is sees it as path-dependent),
// and leaves the registrar path-dependent once the statement is left, since
// the block may not have run; a reassignment from a loop body or a closure
// leaves it path-dependent across the whole loop or closure as well, and so
// does any write carried from one iteration of a three-clause for loop into
// the next. A closure is never known to have run, so a registrar it
// reassigns stays path-dependent for the rest of the body. Branch outcomes are
// never merged: an `if` and an `else` assigning the same prefix still leave it
// unresolved. A goto is not modelled: a jump in either direction can skip or
// repeat a reassignment the walk reads as straight-line code.
type registrarTracker struct {
	top *registrarFrame
	// writes is the writePositions pre-pass over the body.
	writes map[declKey][]token.Pos
	// captured holds registrars a closure reassigns: from the end of that
	// closure on they are path-dependent whatever is assigned to them later.
	captured map[declKey]bool
	pending  []pendingWrite
	// blocks gives each frame the scope the walk pushes for its node, so a
	// function body's frame leaves the names its signature declares to the
	// signature, as the walk's own chain does.
	blocks functionBlocks
}

// newRegistrarTracker starts a walk of body. outer is the signature's scope,
// and seed binds the registrar parameters a caller handed prefixes to.
func newRegistrarTracker(body *ast.BlockStmt, outer map[string]binding, seed prefixMap) *registrarTracker {
	root := &registrarFrame{scope: outer}
	signature := constScopes(nil).push(outer)
	for name, prefix := range seed {
		root.set(keyAt(name, body.Pos(), signature), prefix)
	}
	return &registrarTracker{
		top:      root,
		writes:   writePositions(body, outer),
		captured: map[declKey]bool{},
		blocks:   newFunctionBlocks(body, outer),
	}
}

// advance moves the tracker to node n: it applies every write whose statement
// finished before n, leaves every scope that ended before n, then enters the
// scope n introduces, if any.
func (t *registrarTracker) advance(n ast.Node) {
	t.applyDue(n.Pos())
	for t.top.node != nil && t.top.node.End() <= n.Pos() {
		t.leave()
	}
	scope, introduces := t.frameScope(n)
	if !introduces {
		return
	}
	frame := &registrarFrame{node: n, parent: t.top, scope: scope, repeatable: isRepeatable(n)}
	t.top.enter(frame)
	t.top = frame
}

// frameScope reports whether n gets a frame, and what that frame declares:
// the scope the walk pushes for n, if any, or nothing for a select statement.
func (t *registrarTracker) frameScope(n ast.Node) (map[string]binding, bool) {
	if _, isSelect := n.(*ast.SelectStmt); isSelect {
		return nil, true
	}
	return t.blocks.scopeOf(n)
}

// applyDue applies the pending writes that take effect at or before pos. A
// write's frame is still open then: it ends no earlier than the write.
func (t *registrarTracker) applyDue(pos token.Pos) {
	kept := t.pending[:0]
	for _, write := range t.pending {
		if write.at <= pos {
			write.frame.set(write.key, write.value)
			continue
		}
		kept = append(kept, write)
	}
	t.pending = kept
}

// isRepeatable reports whether a scope-introducing node may run its body any
// number of times.
func isRepeatable(n ast.Node) bool {
	switch n.(type) {
	case *ast.ForStmt, *ast.RangeStmt, *ast.FuncLit:
		return true
	}
	return false
}

// isHeader reports whether a frame's own node is a statement whose header —
// an init statement, or a type switch's assignment — always runs when the
// statement is reached.
func isHeader(n ast.Node) bool {
	switch n.(type) {
	case *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
		return true
	}
	return false
}

// isBranch reports whether f is one alternative of a conditional statement:
// the body or else of an if, or a case or select clause.
func isBranch(f *registrarFrame) bool {
	switch f.node.(type) {
	case *ast.CaseClause, *ast.CommClause:
		return true
	}
	_, inIf := f.parent.node.(*ast.IfStmt)
	return inIf
}

// endsInFallthrough reports whether n is a case clause whose last statement
// hands control to the next clause.
func endsInFallthrough(n ast.Node) bool {
	clause, ok := n.(*ast.CaseClause)
	if !ok {
		return false
	}
	branch, ok := finalStmt(clause.Body).(*ast.BranchStmt)
	return ok && branch.Tok == token.FALLTHROUGH
}

// finalStmt is the statement a list ends in, as go/types reads it for a
// fallthrough: trailing empty statements (`fallthrough;;`) do not count, and a
// labeled statement (`L: fallthrough`) is the statement it labels. It is nil
// for a list with no other statement.
func finalStmt(list []ast.Stmt) ast.Stmt {
	for i := len(list) - 1; i >= 0; i-- {
		if _, empty := list[i].(*ast.EmptyStmt); !empty {
			return unlabeled(list[i])
		}
	}
	return nil
}

// unlabeled strips every label from a statement.
func unlabeled(stmt ast.Stmt) ast.Stmt {
	for {
		labeled, ok := stmt.(*ast.LabeledStmt)
		if !ok {
			return stmt
		}
		stmt = labeled.Stmt
	}
}

// isBareBlock reports whether f is a `{ }` block standing as a statement of
// its own, which always runs when reached: its parent frame is a block or a
// clause body rather than the statement it belongs to.
func isBareBlock(f *registrarFrame) bool {
	if _, ok := f.node.(*ast.BlockStmt); !ok {
		return false
	}
	switch f.parent.node.(type) {
	case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause:
		return true
	}
	return false
}

// leave pops the innermost frame. A bare block always ran, so what it wrote
// to outer registrars carries into the enclosing scope as it stands. Every
// outer registrar any other frame reassigned is path-dependent in the
// enclosing scope — the frame may not have run — and one a closure reassigned
// stays so for good. A case clause that falls through also hands what it
// reassigned to the next clause.
func (t *registrarTracker) leave() {
	left := t.top
	t.top = left.parent
	if isBareBlock(left) {
		t.top.carry(left)
		return
	}
	_, isClosure := left.node.(*ast.FuncLit)
	inBranch := isBranch(left)
	fallsThrough := endsInFallthrough(left.node)
	for key := range left.written() {
		if left.declares(key) {
			continue
		}
		t.top.reassigned(key, inBranch)
		if fallsThrough {
			t.top.fellThrough(key)
		}
		if isClosure {
			t.captured[key] = true
		}
	}
}

// assign records a write of value to key by the statement spanning pos to
// at: straight-line in the frame that declares key, or an override confined to
// the current frame for an outer registrar. The write takes effect at at. A
// write a later run of a loop or closure may observe is path-dependent from
// the start.
func (t *registrarTracker) assign(key declKey, pos, at token.Pos, value registrarPrefix) {
	frame := t.top
	if isHeader(frame.node) && !frame.declares(key) {
		// An if or switch header runs whenever the statement is reached, so
		// its write is straight-line code of the enclosing scope.
		frame = frame.parent
	}
	if carriedAcrossRuns(frame, key, pos) {
		value = pathDependentPrefix()
	}
	t.pending = append(t.pending, pendingWrite{at: at, frame: frame, key: key, value: value})
}

// carriedAcrossRuns reports whether a write at pos from frame to key may be
// seen by another run of a loop or closure: the write sits inside a loop or
// closure that key outlives, or key is declared by a three-clause for loop's
// init — Go copies it from one iteration into the next — and this is not that
// declaration.
func carriedAcrossRuns(frame *registrarFrame, key declKey, pos token.Pos) bool {
	for f := frame; ; f = f.parent {
		if f.declares(key) {
			_, isFor := f.node.(*ast.ForStmt)
			return isFor && pos >= key.from
		}
		if f.repeatable {
			return true
		}
	}
}

// holds reports whether key is a known registrar at the current position.
func (t *registrarTracker) holds(key declKey) bool {
	_, ok := t.lookup(key)
	return ok
}

// lookup returns the binding of key in effect at the current position; ok is
// false when key is not a known registrar there.
func (t *registrarTracker) lookup(key declKey) (registrarPrefix, bool) {
	dependent := t.captured[key]
	for f := t.top; f != nil; f = f.parent {
		dependent = dependent || t.reassignedWhileRunning(f, key)
		if value, ok := f.values[key]; ok {
			if dependent {
				return pathDependentPrefix(), true
			}
			return value, true
		}
	}
	return registrarPrefix{}, false
}

// reassignedWhileRunning reports whether f is a loop or closure that key
// outlives while key can be written during a run of it: from inside a loop,
// or from inside or after a closure, which may be called at any later point.
// The write that declares key takes effect exactly where key enters scope, so
// it does not count.
func (t *registrarTracker) reassignedWhileRunning(f *registrarFrame, key declKey) bool {
	if !f.repeatable || key.from >= runStart(f.node) {
		return false
	}
	_, isClosure := f.node.(*ast.FuncLit)
	for _, at := range t.writes[key] {
		if at >= f.node.Pos() && at > key.from && (isClosure || at < f.node.End()) {
			return true
		}
	}
	return false
}

// runStart is where the repeated part of a loop or closure begins. A variable
// declared before it outlives a single run; a three-clause for loop's init
// runs once, so what it declares is carried into every iteration.
func runStart(n ast.Node) token.Pos {
	if loop, ok := n.(*ast.ForStmt); ok {
		return loop.Body.Pos()
	}
	return n.Pos()
}

// bindingView is what one node of the walk sees: constants through the lexical
// chain in effect there, and registrars through the tracker's current state
// read through that same chain.
type bindingView struct {
	scopes     constScopes
	registrars *registrarTracker
}

// registrar resolves an identifier used as a route registrar, at its own
// position, to its group prefix. ok is false when it names no known registrar.
func (v bindingView) registrar(ident *ast.Ident) (registrarPrefix, bool) {
	if prefix, ok := v.registrars.lookup(keyAt(ident.Name, ident.Pos(), v.scopes)); ok {
		return prefix, true
	}
	return v.shadowedRegistrar(ident)
}

// shadowedRegistrar covers an identifier whose own declaration holds no
// registrar binding — a closure or range parameter, or a local given an
// untraceable value — but that hides a known registrar declared further out.
// Its prefix is unknown: keeping the hidden registrar's prefix would be a
// guess, and so would dropping it, so hiding a group registrar makes it
// untraced. Hiding the prefix-less root registrar leaves it a prefix-less
// registrar, the treatment every registrar the walk cannot trace gets.
func (v bindingView) shadowedRegistrar(ident *ast.Ident) (registrarPrefix, bool) {
	for _, from := range v.scopes.hidden(ident.Name, ident.Pos()) {
		outer, ok := v.registrars.lookup(declKey{name: ident.Name, from: from})
		if !ok {
			continue
		}
		if outer.resolved && outer.path == "" {
			return outer, true
		}
		return untracedPrefix(), true
	}
	return registrarPrefix{}, false
}
