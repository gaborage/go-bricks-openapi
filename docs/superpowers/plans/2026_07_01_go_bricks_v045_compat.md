# go-bricks v0.45.0 Compatibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make go-bricks-openapi parse all idiomatic go-bricks v0.45.0 consumer shapes and emit specs that accurately describe a v0.45 runtime.

**Architecture:** Three stacked PRs on top of `feature/go-bricks-v045-compat`. PR1 fixes the route-dropping delegation bug in the analyzer's walker. PR2 replaces the phantom `WithPublic` option with an `//openapi:public` comment directive and adds `RegisterHandler`/`WithModule` recognition. PR3 corrects the generator's error/security modeling. The tool stays a pure `go/ast` analyzer — it never imports go-bricks.

**Tech Stack:** Go 1.25, cobra, kin-openapi (validation), testify, golden-file spectest harness.

**Spec:** `docs/superpowers/specs/2026_07_01_go_bricks_v045_compat_design.md` (approved).

## Global Constraints

- Doctor go-bricks floor stays `v0.13.0`; new "verified through v0.45.0" statement (PR3).
- Tool toolchain floor stays `go 1.25` (go.mod unchanged).
- Emitted spec version stays OpenAPI **3.0.1**.
- Sonar quality gate: **≥80% coverage on new code**; extract pure logic instead of stubbed var-seam closures.
- Each PR ≤400 LOC (excluding golden files); Conventional Commit messages.
- Do NOT edit `CHANGELOG.md` (release-please owns it).
- New doc/plan files use snake_case names.
- Golden regen: `go test ./internal/spectest -update` — always eyeball the diff before committing; never regen to paper over a behavior regression.
- Empirical acceptance target: `go run ./cmd/go-bricks-openapi generate --project ../go-bricks-demo-project` emits **15/15** routes (today 0/15).
- Before each PR: run `/simplify`, then `/security-review`, fix findings, only then push and open the PR (user instruction).
- Branch stack: PR1 `feature/gb045-pr1-delegation` (base: `main`), PR2 `feature/gb045-pr2-directive` (base: PR1 branch), PR3 `feature/gb045-pr3-generator-accuracy` (base: PR2 branch). The current `feature/go-bricks-v045-compat` branch (spec + this plan) merges first or its docs commit is cherry-picked into PR1.

---

# PR 1 — fix(analyzer): handler-field delegation

### Task 1: Walker recurses into `m.<field>.Method(hr, r)` delegates

**Files:**
- Modify: `internal/analyzer/analyzer.go:574-590` (`extractRoutesFromFuncBodyWithAliases` — new param + walker fields)
- Modify: `internal/analyzer/analyzer.go:594-603` (`routeWalker` struct — new fields)
- Modify: `internal/analyzer/analyzer.go:714-750` (`maybeRecurseHelper` — split into same-receiver + field-delegate paths)
- Modify: `internal/analyzer/analyzer.go:540-562` (`collectRoutesFromFile` — pass root registrar name)
- Modify: `internal/analyzer/analyzer.go:566-568` (`extractRoutesFromFuncBody` — pass empty registrar)
- Test: `internal/analyzer/analyzer_test.go`

**Interfaces:**
- Consumes (existing, unchanged): `resolveQualifiedStruct(qualified string, astFile *ast.File) (qualifiedStruct, bool)` (analyzer.go:1921), `findMethodDecl`, `routeRegistrarParam`, `findStructDefinition`, `extractImportAliases(file, serverImportPath)`, `receiverVarName`, `addWarningf`.
- Produces: `resolveDelegateContext(astFile *ast.File, filePath, moduleStruct, fieldName string) (delegateContext, bool)` and `fieldTypeExpr(structName, fieldName string, astFile *ast.File, filePath string) (name string, qualified bool)` on `*ProjectAnalyzer`; walker fields `registrars map[string]bool`. Cycle-guard stack keys change from `method` to `structName + "." + method`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/analyzer/analyzer_test.go` (imports `context`, `require`, `assert` already present):

```go
// TestFieldDelegatedRegisterRoutes verifies routes registered through the
// m.<field>.Method(hr, r) delegation shape are discovered: same-package
// delegate, group-prefix threading into the delegate, and typed handlers
// resolved against the DELEGATE struct (not the module).
func TestFieldDelegatedRegisterRoutes(t *testing.T) {
	src := `package mod
import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)
type Module struct{ h *Handler }
func (m *Module) Name() string { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error { return nil }
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	m.h.RegisterRoutes(hr, r)
	v1 := r.Group("/v1")
	m.h.RegisterWrites(hr, v1)
}
type Handler struct{}
type Thing struct{ ID int64 ` + "`json:\"id\"`" + ` }
func (h *Handler) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/things", h.list)
}
func (h *Handler) RegisterWrites(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/things", h.create)
}
func (h *Handler) list(ctx server.HandlerContext) (server.Result[Thing], server.IAPIError) {
	return server.NewResult(200, Thing{}), nil
}
func (h *Handler) create(req Thing, ctx server.HandlerContext) (server.Result[Thing], server.IAPIError) {
	return server.Created(Thing{}), nil
}
`
	_, routes := analyzeSingleModule(t, src)
	require.Len(t, routes, 2, "both delegated route sets must be discovered")

	list := routeForPath(t, routes, "GET /things")
	assert.Equal(t, "list", list.HandlerName)
	require.NotNil(t, list.Response, "handler signature must resolve against the delegate struct")

	create := routeForPath(t, routes, "POST /v1/things")
	assert.Equal(t, "create", create.HandlerName)
	require.NotNil(t, create.Request, "request type must resolve against the delegate struct")
}

// TestFieldDelegationCycleGuard verifies mutually-delegating registration
// methods terminate (the stack key is struct-qualified, so a delegate method
// named RegisterRoutes does not collide with the module's own RegisterRoutes).
func TestFieldDelegationCycleGuard(t *testing.T) {
	src := `package mod
import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)
type Module struct{ h *Handler }
func (m *Module) Name() string { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error { return nil }
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	m.h.RegisterRoutes(hr, r)
}
type Handler struct{ again *Handler }
type Thing struct{ ID int64 ` + "`json:\"id\"`" + ` }
func (h *Handler) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/once", h.get)
	h.again.RegisterRoutes(hr, r) // self-delegation: must not loop or duplicate
}
func (h *Handler) get(ctx server.HandlerContext) (server.Result[Thing], server.IAPIError) {
	return server.NewResult(200, Thing{}), nil
}
`
	_, routes := analyzeSingleModule(t, src)
	require.Len(t, routes, 1, "cycle guard must stop self-delegation without duplicating routes")
}

// TestFieldDelegationWarnsOnUnresolvable verifies the fail-loud contract: a
// delegated call that PASSES a registrar but whose target cannot be resolved
// warns (routes are being dropped), while ordinary field method calls that do
// not receive a registrar stay silent.
func TestFieldDelegationWarnsOnUnresolvable(t *testing.T) {
	src := `package mod
import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/rs/zerolog"
)
type Module struct {
	mystery zerolog.Logger
}
func (m *Module) Name() string { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error { return nil }
type Thing struct{ ID int64 ` + "`json:\"id\"`" + ` }
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	m.mystery.RegisterRoutes(hr, r) // external type: unresolvable, receives registrar -> warn
	m.mystery.Print("starting")     // ordinary call, no registrar -> silent
	server.GET(hr, r, "/ok", m.ok)
}
func (m *Module) ok(ctx server.HandlerContext) (server.Result[Thing], server.IAPIError) {
	return server.NewResult(200, Thing{}), nil
}
`
	a, routes := analyzeSingleModule(t, src)
	require.Len(t, routes, 1)

	warnings := a.Warnings(context.Background())
	require.Len(t, warnings, 1, "exactly one warning: the dropped delegation, not the Print call")
	assert.Contains(t, warnings[0], "mystery")
	assert.Contains(t, warnings[0], "RegisterRoutes")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/analyzer/ -run 'TestFieldDelegat' -v`
Expected: FAIL — `TestFieldDelegatedRegisterRoutes` gets 0 routes (`require.Len` fails), the other two fail similarly.

- [ ] **Step 3: Implement delegation recursion**

In `internal/analyzer/analyzer.go`:

3a. Extend the walker struct (replace lines 594-603) — add `registrars`:

```go
// routeWalker collects routes from a RegisterRoutes method body and the
// same-receiver helper methods / handler-field delegates it transitively calls.
type routeWalker struct {
	a             *ProjectAnalyzer
	astFile       *ast.File
	filePath      string
	structName    string // struct owning the walked method, for handler resolution
	recvVar       string // receiver var name of the walked method; "" if none
	serverAliases map[string]struct{}
	stack         map[string]bool // "<struct>.<method>" keys currently on the walk stack (cycle guard)
	// registrars holds identifier names known to be route registrars in the
	// walked body (the method's own RouteRegistrar param); group vars are
	// tracked separately in prefixes. Used to warn when a delegation that
	// receives a registrar cannot be resolved (routes would silently vanish).
	registrars map[string]bool
	routes     []models.Route
}
```

3b. Change `extractRoutesFromFuncBodyWithAliases` (lines 574-590) to accept and seed the root registrar and struct-qualified stack key:

```go
func (a *ProjectAnalyzer) extractRoutesFromFuncBodyWithAliases(
	body *ast.BlockStmt, astFile *ast.File, filePath, structName, recvVar, rootRegistrar string, serverAliases map[string]struct{},
) []models.Route {
	w := &routeWalker{
		a:             a,
		astFile:       astFile,
		filePath:      filePath,
		structName:    structName,
		recvVar:       recvVar,
		serverAliases: serverAliases,
		// Seed the recursion stack with the entry method so a helper that calls
		// back into RegisterRoutes cannot re-walk it (cycle guard for the root).
		stack:      map[string]bool{structName + "." + moduleMethodRegisterRoutes: true},
		registrars: registrarSet(rootRegistrar),
	}
	w.walkBody(body, nil)
	return w.routes
}

// registrarSet builds the walker's known-registrar set from a (possibly empty)
// registrar parameter name.
func registrarSet(name string) map[string]bool {
	s := map[string]bool{}
	if name != "" {
		s[name] = true
	}
	return s
}
```

3c. Update the two callers:

`extractRoutesFromFuncBody` (line 566-568):

```go
func (a *ProjectAnalyzer) extractRoutesFromFuncBody(body *ast.BlockStmt) []models.Route {
	return a.extractRoutesFromFuncBodyWithAliases(body, nil, "", "", "", "", map[string]struct{}{frameworkPkgServer: {}})
}
```

`collectRoutesFromFile` (line 557-558) — derive the root registrar name from the decl:

```go
		recvVar := receiverVarName(funcDecl.Recv)
		_, rootRegistrar := a.routeRegistrarParam(funcDecl, serverAliases)
		routes = append(routes, a.extractRoutesFromFuncBodyWithAliases(funcDecl.Body, astFile, filePath, structName, recvVar, rootRegistrar, serverAliases)...)
```

3d. Replace `maybeRecurseHelper` (lines 714-750) with the split implementation:

```go
// maybeRecurseHelper recurses into route-registration targets reachable from
// the walked body: same-receiver helpers (m.helper(hr, r)) and handler-field
// delegates (m.handler.RegisterRoutes(hr, r) — the idiomatic pattern where a
// module forwards the registry to a struct field, in-package or in an in-module
// sibling package). The RouteRegistrar-parameter requirement excludes
// non-registration methods so their internal server.* calls are not mistaken
// for routes. The struct-qualified stack guards against infinite recursion.
func (w *routeWalker) maybeRecurseHelper(call *ast.CallExpr, prefixes map[string]string) {
	if w.recvVar == "" || w.astFile == nil {
		return
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	switch recv := sel.X.(type) {
	case *ast.Ident:
		if recv.Name != w.recvVar {
			return
		}
		w.recurseSameReceiver(call, sel.Sel.Name, prefixes)
	case *ast.SelectorExpr:
		base, ok := recv.X.(*ast.Ident)
		if !ok || base.Name != w.recvVar {
			return
		}
		w.recurseFieldDelegate(call, recv.Sel.Name, sel.Sel.Name, prefixes)
	}
}

// recurseSameReceiver walks a same-receiver helper (m.helper(...)) that has a
// server.RouteRegistrar parameter, threading the caller's group prefix.
func (w *routeWalker) recurseSameReceiver(call *ast.CallExpr, method string, prefixes map[string]string) {
	key := w.structName + "." + method
	if w.stack[key] {
		return
	}
	decl := w.a.findMethodDecl(w.astFile, w.filePath, w.structName, method)
	idx, paramName := w.a.routeRegistrarParam(decl, w.serverAliases)
	if idx < 0 {
		return // not a route-registration helper
	}
	w.stack[key] = true
	w.walkBody(decl.Body, w.seedFromCall(call, idx, paramName, prefixes))
	delete(w.stack, key)
}

// recurseFieldDelegate walks a handler-field delegate (m.<field>.<method>(...)).
// Non-registration field calls (m.logger.Info(...)) fail to resolve or lack a
// registrar param and are silently skipped; a call that PASSES a registrar but
// cannot be resolved is warned about, because its routes are being dropped.
func (w *routeWalker) recurseFieldDelegate(call *ast.CallExpr, fieldName, method string, prefixes map[string]string) {
	delegate, ok := w.a.resolveDelegateContext(w.astFile, w.filePath, w.structName, fieldName)
	if !ok {
		w.warnIfRegistrarPassed(call, fieldName, method, prefixes)
		return
	}
	key := delegate.structName + "." + method
	if w.stack[key] {
		return
	}
	decl := w.a.findMethodDecl(delegate.astFile, delegate.filePath, delegate.structName, method)
	idx, paramName := w.a.routeRegistrarParam(decl, delegate.serverAliases)
	if idx < 0 {
		w.warnIfRegistrarPassed(call, fieldName, method, prefixes)
		return
	}
	dw := &routeWalker{
		a:             w.a,
		astFile:       delegate.astFile,
		filePath:      delegate.filePath,
		structName:    delegate.structName,
		recvVar:       receiverVarName(decl.Recv),
		serverAliases: delegate.serverAliases,
		stack:         w.stack, // shared: cycles across delegation chains must terminate
		registrars:    registrarSet(paramName),
	}
	w.stack[key] = true
	dw.walkBody(decl.Body, w.seedFromCall(call, idx, paramName, prefixes))
	delete(w.stack, key)
	w.routes = append(w.routes, dw.routes...)
}

// seedFromCall threads the group prefix bound to the call's registrar argument
// into the callee's registrar parameter name.
func (w *routeWalker) seedFromCall(call *ast.CallExpr, idx int, paramName string, prefixes map[string]string) map[string]string {
	seed := map[string]string{}
	if paramName != "" && idx < len(call.Args) {
		if argIdent, ok := call.Args[idx].(*ast.Ident); ok {
			if prefix := prefixes[argIdent.Name]; prefix != "" {
				seed[paramName] = prefix
			}
		}
	}
	return seed
}

// warnIfRegistrarPassed emits a diagnostic when an unresolvable delegated call
// receives a known registrar (the walked method's own registrar param or a
// group derived from it) — the strongest static signal that route
// registrations are being dropped. Calls without a registrar stay silent.
func (w *routeWalker) warnIfRegistrarPassed(call *ast.CallExpr, fieldName, method string, prefixes map[string]string) {
	for _, arg := range call.Args {
		ident, ok := arg.(*ast.Ident)
		if !ok {
			continue
		}
		if w.registrars[ident.Name] || prefixes[ident.Name] != "" {
			w.a.addWarningf("skipping %s.%s.%s(...): it receives a route registrar but its routes could not be resolved (delegate type or method not found)",
				w.recvVar, fieldName, method)
			return
		}
	}
}
```

3e. Add the delegate-context resolver near `resolveFieldType` (after line 940):

```go
// delegateContext is the file/package context of a module field's struct type,
// letting the walker recurse into the delegate's methods with the aliases of
// the file that declares them.
type delegateContext struct {
	structName    string
	astFile       *ast.File
	filePath      string
	serverAliases map[string]struct{}
}

// resolveDelegateContext resolves module field fieldName to an in-module struct.
// ok=false for external or unresolvable field types — the normal case for
// non-handler fields (loggers, services), so callers must not warn on it alone.
func (a *ProjectAnalyzer) resolveDelegateContext(astFile *ast.File, filePath, moduleStruct, fieldName string) (delegateContext, bool) {
	fieldType, qualified := a.fieldTypeExpr(moduleStruct, fieldName, astFile, filePath)
	if fieldType == "" {
		return delegateContext{}, false
	}
	if qualified {
		q, ok := a.resolveQualifiedStruct(fieldType, astFile)
		if !ok {
			return delegateContext{}, false
		}
		return delegateContext{
			structName:    q.typeName,
			astFile:       q.file,
			filePath:      q.filePath,
			serverAliases: a.extractImportAliases(q.file, serverImportPath),
		}, true
	}
	// Same package: findMethodDecl searches the whole package from this file,
	// so keep the current file context (matching same-receiver helper behavior,
	// which also uses the entry file's aliases for cross-file helpers).
	return delegateContext{
		structName:    fieldType,
		astFile:       astFile,
		filePath:      filePath,
		serverAliases: a.extractImportAliases(astFile, serverImportPath),
	}, true
}

// fieldTypeExpr returns the declared type of field fieldName on structName:
// ("Handler", false) for an in-package type, ("handlers.Handler", true) for a
// package-qualified one, ("", false) when unresolvable. Pointers are stripped.
func (a *ProjectAnalyzer) fieldTypeExpr(structName, fieldName string, astFile *ast.File, filePath string) (name string, qualified bool) {
	if structName == "" {
		return "", false
	}
	structType, err := a.findStructDefinition(astFile, filePath, structName)
	if err != nil || structType.Fields == nil {
		return "", false
	}
	for _, field := range structType.Fields.List {
		for _, n := range field.Names {
			if n.Name == fieldName {
				return qualifiedTypeName(field.Type)
			}
		}
	}
	return "", false
}

// qualifiedTypeName renders a field type expression as an (optionally
// package-qualified) type name, stripping a leading pointer.
func qualifiedTypeName(expr ast.Expr) (string, bool) {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return qualifiedTypeName(t.X)
	case *ast.Ident:
		return t.Name, false
	case *ast.SelectorExpr:
		if pkg, ok := t.X.(*ast.Ident); ok {
			return pkg.Name + "." + t.Sel.Name, true
		}
	}
	return "", false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/analyzer/ -run 'TestFieldDelegat' -v`
Expected: PASS (3 tests). Then the full suite: `go test ./...`
Expected: PASS — if any existing helper-recursion test fails on the stack-key change, the fix is in the test's expectation only if it asserted the OLD collision behavior; otherwise investigate before touching goldens.

- [ ] **Step 5: Commit**

```bash
git add internal/analyzer/analyzer.go internal/analyzer/analyzer_test.go
git commit -m "fix(analyzer): follow handler-field delegation in RegisterRoutes

m.handler.RegisterRoutes(hr, r) previously dropped every route silently."
```

### Task 2: Cross-package delegation spectest fixture

**Files:**
- Create: `internal/spectest/testdata/delegation/go.mod`
- Create: `internal/spectest/testdata/delegation/module.go`
- Create: `internal/spectest/testdata/delegation/internal/handlers/payments.go`
- Create: `internal/spectest/testdata/delegation/expected.yaml` (golden, generated)

**Interfaces:**
- Consumes: the spectest harness auto-discovers any `testdata/<dir>` with a `go.mod` (spectest_test.go:30) and compares against `expected.yaml`.
- Produces: the `delegation` fixture — the regression gate for Task 1 across packages.

- [ ] **Step 1: Write the fixture (failing until goldens exist)**

`internal/spectest/testdata/delegation/go.mod`:

```
module github.com/example/delegation

go 1.25

require github.com/gaborage/go-bricks v0.45.0
```

`internal/spectest/testdata/delegation/module.go`:

```go
// Package payments demonstrates the handler-field delegation pattern the
// go-bricks-demo-project uses: RegisterRoutes forwards the registry to a
// handler struct in an in-module sibling package, including through a group.
package payments

import (
	"github.com/example/delegation/internal/handlers"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module owns no routes itself — everything is delegated.
type Module struct {
	handler *handlers.PaymentHandler
}

func (m *Module) Name() string                    { return "payments" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	m.handler.RegisterRoutes(hr, r)
	v1 := r.Group("/v1")
	m.handler.RegisterAdmin(hr, v1)
}
```

`internal/spectest/testdata/delegation/internal/handlers/payments.go`:

```go
// Package handlers hosts the delegated route registrations.
package handlers

import (
	"net/http"

	"github.com/gaborage/go-bricks/server"
)

// PaymentHandler registers and serves the payment routes.
type PaymentHandler struct{}

// Payment is the payment resource.
type Payment struct {
	ID     string `json:"id"`
	Amount int64  `json:"amount" validate:"required,min=1"`
}

func (h *PaymentHandler) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/payments/:id", h.getPayment, server.WithTags("payments"))
	server.POST(hr, r, "/payments", h.createPayment, server.WithTags("payments"))
}

func (h *PaymentHandler) RegisterAdmin(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/payments/pending", h.listPending, server.WithTags("admin"))
}

type getPaymentRequest struct {
	ID string `param:"id" validate:"required"`
}

func (h *PaymentHandler) getPayment(req getPaymentRequest, ctx server.HandlerContext) (server.Result[Payment], server.IAPIError) {
	return server.NewResult(http.StatusOK, Payment{ID: req.ID}), nil
}

func (h *PaymentHandler) createPayment(req Payment, ctx server.HandlerContext) (server.Result[Payment], server.IAPIError) {
	return server.Created(req), nil
}

func (h *PaymentHandler) listPending(ctx server.HandlerContext) (server.Result[[]Payment], server.IAPIError) {
	return server.NewResult(http.StatusOK, []Payment{}), nil
}
```

- [ ] **Step 2: Run spectest to verify the fixture fails (no golden yet)**

Run: `go test ./internal/spectest -run 'TestFixtures/delegation' -v`
Expected: FAIL with "missing golden file; run `go test ./internal/spectest -update`"

- [ ] **Step 3: Generate and review the golden**

Run: `go test ./internal/spectest -run 'TestFixtures/delegation' -update && cat internal/spectest/testdata/delegation/expected.yaml`
Verify by eye: paths `/payments/{id}` (GET), `/payments` (POST), `/v1/payments/pending` (GET) — 3 paths; `Payment` component present; the `/v1` prefix threaded through delegation. If routes are missing, Task 1 is incomplete — do NOT commit the golden.

- [ ] **Step 4: Run the full suite**

Run: `go test ./...`
Expected: PASS, including `TestFixtures/delegation` now comparing clean.

- [ ] **Step 5: Empirical acceptance check against the demo project**

```bash
go run ./cmd/go-bricks-openapi generate --project ../go-bricks-demo-project \
  --output /tmp/demo-openapi.yaml --verbose 2>&1 | tail -20
grep -c "^  /" /tmp/demo-openapi.yaml || python3 -c "
import yaml; print(sum(len(v) for v in yaml.safe_load(open('/tmp/demo-openapi.yaml'))['paths'].values()))"
```

Expected: **15 operations** across the demo's 5 modules (was 0). If fewer, list the missing routes against `grep -rn 'server\.\(GET\|POST\|PUT\|PATCH\|DELETE\)' ../go-bricks-demo-project/internal/` and fix before proceeding.

- [ ] **Step 6: Commit**

```bash
git add internal/spectest/testdata/delegation/
git commit -m "test(spectest): cross-package handler-delegation fixture"
```

### Task 3: PR1 gates and pull request

- [ ] **Step 1: Quality gates**

Run: `make lint 2>/dev/null || (go vet ./... && command -v staticcheck >/dev/null && staticcheck ./...)` then `go test ./... -cover`
Expected: clean; analyzer package coverage not lower than before (new funcs are all exercised by the 3 unit tests + fixture).

- [ ] **Step 2: Run /simplify, then /security-review** (user-mandated gates)

Invoke the `/simplify` skill on the working tree; apply its fixes; re-run `go test ./...`. Then invoke `/security-review`; resolve findings (note: `#nosec G304` patterns already established for validated file reads — new file reads in this PR go through existing parse helpers only).

- [ ] **Step 3: Push and open PR**

```bash
git push -u origin feature/gb045-pr1-delegation
gh pr create --base main --title "fix(analyzer): follow handler-field delegation in RegisterRoutes" --body "$(cat <<'EOF'
## Summary
- Modules that delegate registration via a handler field (`m.handler.RegisterRoutes(hr, r)`) previously produced an EMPTY spec — all routes silently dropped (go-bricks-demo-project: 0/15 routes). The walker now resolves the field's struct (in-package or in-module cross-package via the existing resolveQualifiedStruct machinery) and recurses with the delegate's own context.
- Cycle guard keys are struct-qualified (`Type.Method`) so delegate methods named RegisterRoutes cannot collide with the module's.
- Fail-loud: a delegated call that receives a registrar but cannot be resolved emits a warning (fails under --strict).

## Testing
- 3 new analyzer unit tests (same-package delegation, cycle guard, warning contract).
- New `delegation` spectest fixture with cross-package handlers + group threading.
- Empirical: demo project now emits 15/15 operations.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

# PR 2 — feat(analyzer): //openapi:public directive, RegisterHandler, WithModule

Branch: `git checkout -b feature/gb045-pr2-directive` (from PR1 branch).

### Task 4: `//openapi:public` directive replaces phantom `WithPublic`

**Files:**
- Modify: `internal/analyzer/analyzer.go` (New() init; 3 parse sites at ~327, ~1193, ~2045; `routeFromCall` at 676-704; `extractRouteMetadata` — delete `case "WithPublic"` at 996-998)
- Modify: `internal/models/models.go:51-55` (Route.Public doc comment)
- Modify: `internal/analyzer/analyzer_test.go:3760-3793` (rewrite `TestWithPublicMetadata` → `TestPublicDirective`)
- Modify: `internal/spectest/testdata/registration/storefront.go` (2 `WithPublic` uses → directives)
- Modify: `internal/spectest/testdata/package_func/health.go:25` (1 use → directive)

**Interfaces:**
- Produces: `ProjectAnalyzer.publicDirectives map[string]map[int]struct{}`; `indexPublicDirectives(astFile *ast.File)`; `isPublicRoute(pos token.Pos) bool`; constant `publicDirective = "//openapi:public"`.
- Key invariant: goldens for `registration` and `package_func` must remain **byte-identical** after the rewrite (the directive reproduces exactly what `WithPublic()` produced) — run the spectest WITHOUT `-update` to prove it.

- [ ] **Step 1: Rewrite the unit test (failing)**

Replace `TestWithPublicMetadata` (analyzer_test.go:3760-3793) with:

```go
// TestPublicDirective verifies the //openapi:public comment directive flips
// route.Public: alone, inside a doc-comment block, and NOT via unrelated
// comments or the removed server.WithPublic() option (phantom API — it never
// existed in go-bricks, so real consumer code cannot contain it).
func TestPublicDirective(t *testing.T) {
	src := `package mod
import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)
type Module struct{}
func (m *Module) Name() string { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error { return nil }
type Thing struct{ ID int64 ` + "`json:\"id\"`" + ` }
func (m *Module) health(ctx server.HandlerContext) (server.Result[Thing], server.IAPIError) { return server.Created(Thing{}), nil }
func (m *Module) login(ctx server.HandlerContext) (server.Result[Thing], server.IAPIError) { return server.Created(Thing{}), nil }
func (m *Module) private(ctx server.HandlerContext) (server.Result[Thing], server.IAPIError) { return server.Created(Thing{}), nil }
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	//openapi:public
	server.GET(hr, r, "/health", m.health)

	// Login issues the session token.
	//openapi:public
	server.POST(hr, r, "/login", m.login, server.WithTags("auth"))

	// An ordinary comment must not mark the route public.
	server.GET(hr, r, "/private", m.private)
}
`
	_, routes := analyzeSingleModule(t, src)

	health := routeForPath(t, routes, "GET /health")
	assert.True(t, health.Public, "//openapi:public directly above the call must set Public")

	login := routeForPath(t, routes, "POST /login")
	assert.True(t, login.Public, "directive inside a doc-comment block must set Public")
	assert.Equal(t, []string{"auth"}, login.Tags, "WithTags still applies alongside the directive")

	private := routeForPath(t, routes, "GET /private")
	assert.False(t, private.Public, "an unrelated comment must not mark the route public")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/analyzer/ -run TestPublicDirective -v`
Expected: FAIL — `health.Public` is false (directive not recognized yet).

- [ ] **Step 3: Implement the directive**

3a. Constant (near the other `moduleMethod*` constants, ~line 43):

```go
// publicDirective marks the route registered on the following line as public
// (no tenant security) in the generated spec. go-bricks v0.45 has no per-route
// tenant opt-out API, so the tool provides one as a comment directive, in the
// spirit of go:generate.
const publicDirective = "//openapi:public"
```

3b. Field on `ProjectAnalyzer` + init in `New` (find `func New(` and the struct literal; add):

```go
	publicDirectives map[string]map[int]struct{} // filename -> lines where a directive comment group ends
```

and in the constructor: `publicDirectives: map[string]map[int]struct{}{},`

3c. Indexer + query (near the parse helpers):

```go
// indexPublicDirectives records, for every comment group containing an
// `//openapi:public` line, the file line the group ends on. routeFromCall marks
// a route public when a directive group ends directly above the registration
// call. Position-keyed (file+line) so it works regardless of which file a
// walked body lives in (helpers and delegates may be cross-file). Idempotent
// across re-parses of the same file.
func (a *ProjectAnalyzer) indexPublicDirectives(astFile *ast.File) {
	for _, cg := range astFile.Comments {
		found := false
		for _, c := range cg.List {
			if strings.TrimSpace(c.Text) == publicDirective {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		pos := a.fileSet.Position(cg.End())
		lines := a.publicDirectives[pos.Filename]
		if lines == nil {
			lines = map[int]struct{}{}
			a.publicDirectives[pos.Filename] = lines
		}
		lines[pos.Line] = struct{}{}
	}
}

// isPublicRoute reports whether a public directive comment group ends on the
// line directly above pos (a route registration call's first token).
func (a *ProjectAnalyzer) isPublicRoute(pos token.Pos) bool {
	p := a.fileSet.Position(pos)
	_, ok := a.publicDirectives[p.Filename][p.Line-1]
	return ok
}
```

3d. Call `a.indexPublicDirectives(astFile)` immediately after each of the three successful `parser.ParseFile` calls (analyzer.go ~327 in `analyzeGoFile`, ~1193 in `parsePackage`, ~2045 in `parsePackageDir`).

3e. In `routeFromCall` (after the metadata loop, before `return route`):

```go
	if w.a.isPublicRoute(call.Pos()) {
		route.Public = true
	}
```

3f. Delete `case "WithPublic": route.Public = true` from `extractRouteMetadata` (lines 996-998).

3g. Update `models.Route.Public` comment (models.go:51-55):

```go
	// Public is true when the registration is annotated with an
	// `//openapi:public` comment directive on the line(s) directly above it:
	// the generator emits operation-level `security: []` so liveness probes and
	// other tenant-agnostic endpoints (/health, /login, webhooks) are
	// documented as requiring no auth. (go-bricks itself has no per-route
	// tenant opt-out API as of v0.45.)
	Public bool
```

- [ ] **Step 4: Rewrite the fixtures to use the directive**

`internal/spectest/testdata/registration/storefront.go` — replace the two option usages:

```go
	// Route registered inside a conditional block.
	if true {
		//openapi:public
		server.GET(hr, r, "/health", m.health, server.WithTags("ops"))
	}

	// Concatenated path from a constant: /api/version
	//openapi:public
	server.GET(hr, r, apiBase+"/version", m.version, server.WithTags("ops"))
```

`internal/spectest/testdata/package_func/health.go:25`:

```go
	//openapi:public
	server.GET(hr, r, "/ping", ping, server.WithTags("health"))
```

- [ ] **Step 5: Run the full suite WITHOUT -update — goldens must not change**

Run: `go test ./...`
Expected: PASS. `TestFixtures/registration` and `TestFixtures/package_func` comparing clean against the EXISTING goldens proves the directive is a drop-in replacement for the phantom option. If a golden diff appears, the directive association is buggy — fix the code, do not regen.

- [ ] **Step 6: Commit**

```bash
git add internal/analyzer/ internal/models/models.go internal/spectest/testdata/registration/ internal/spectest/testdata/package_func/
git commit -m "feat(analyzer)!: replace phantom server.WithPublic with //openapi:public directive

server.WithPublic() never existed in any go-bricks release; code using it
cannot compile. The comment directive provides the public-route opt-out."
```

### Task 5: Recognize `server.RegisterHandler[T,R](hr, r, method, path, handler, opts...)`

**Files:**
- Modify: `internal/analyzer/analyzer.go:791-814` (`validateServerCall` → returns a shape), `676-704` (`routeFromCall` uses the shape), `856-884` (`extractHandlerInfo` takes the handler expr)
- Test: `internal/analyzer/analyzer_test.go`

**Interfaces:**
- Produces: `routeCallShape{method string; pathIdx, handlerIdx, optsIdx int}`; `validateServerCall(call, aliases) (routeCallShape, bool)`; `extractHandlerInfo(handlerArg ast.Expr, astFile, filePath, structName)` (first param changes from `*ast.CallExpr` to the handler expression).
- Consumes: `isHTTPMethod`, `extractPathFromArg`, `extractRouteMetadata`.

- [ ] **Step 1: Write the failing test**

```go
// TestRegisterHandlerGenericForm verifies the exported generic registration
// form server.RegisterHandler(hr, r, method, path, handler, opts...) is
// recognized with the method taken from a string literal or an http.MethodX
// constant, and that options shift right by one position.
func TestRegisterHandlerGenericForm(t *testing.T) {
	src := `package mod
import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)
type Module struct{}
func (m *Module) Name() string { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error { return nil }
type Thing struct{ ID int64 ` + "`json:\"id\"`" + ` }
func (m *Module) get(ctx server.HandlerContext) (server.Result[Thing], server.IAPIError) { return server.NewResult(200, Thing{}), nil }
func (m *Module) create(req Thing, ctx server.HandlerContext) (server.Result[Thing], server.IAPIError) { return server.Created(Thing{}), nil }
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.RegisterHandler(hr, r, "GET", "/things/:id", m.get, server.WithTags("things"))
	server.RegisterHandler(hr, r, http.MethodPost, "/things", m.create)
	server.RegisterHandler(hr, r, someVar, "/dropped", m.get) // non-literal method: warn + skip
}
var someVar = "PUT"
`
	a, routes := analyzeSingleModule(t, src)
	require.Len(t, routes, 2)

	get := routeForPath(t, routes, "GET /things/{id}")
	assert.Equal(t, "get", get.HandlerName)
	assert.Equal(t, []string{"things"}, get.Tags, "options shifted by one must still parse")

	create := routeForPath(t, routes, "POST /things")
	require.NotNil(t, create.Request)

	warnings := a.Warnings(context.Background())
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "RegisterHandler")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/analyzer/ -run TestRegisterHandlerGenericForm -v`
Expected: FAIL — 0 routes found.

- [ ] **Step 3: Implement**

3a. Shape type + rewritten `validateServerCall` (replace lines 791-814):

```go
// routeCallShape locates a registration call's arguments: server.GET keeps the
// path at Args[2] and handler at Args[3]; server.RegisterHandler carries an
// explicit method at Args[2], shifting path/handler/options right by one.
type routeCallShape struct {
	method     string
	pathIdx    int
	handlerIdx int
	optsIdx    int
}

// validateServerCall reports whether call is a route registration —
// server.METHOD(hr, r, path, handler, opts...) or
// server.RegisterHandler(hr, r, method, path, handler, opts...) — and returns
// its argument shape. A RegisterHandler whose method argument is not a string
// literal or http.MethodX constant is warned about and skipped (the route
// cannot be documented without a static method).
func (a *ProjectAnalyzer) validateServerCall(callExpr *ast.CallExpr, serverAliases map[string]struct{}) (routeCallShape, bool) {
	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return routeCallShape{}, false
	}
	pkgIdent, ok := selExpr.X.(*ast.Ident)
	if !ok || !a.aliasContains(serverAliases, pkgIdent.Name, frameworkPkgServer) {
		return routeCallShape{}, false
	}

	name := selExpr.Sel.Name
	if name == registerHandlerFuncName {
		if len(callExpr.Args) < 5 {
			return routeCallShape{}, false
		}
		method, ok := staticHTTPMethod(callExpr.Args[2])
		if !ok || !a.isHTTPMethod(method) {
			a.addWarningf("skipping a server.RegisterHandler route: its method argument is not a static HTTP method (string literal or http.MethodX constant)")
			return routeCallShape{}, false
		}
		return routeCallShape{method: method, pathIdx: 3, handlerIdx: 4, optsIdx: 5}, true
	}

	if !a.isHTTPMethod(name) || len(callExpr.Args) < 3 {
		return routeCallShape{}, false
	}
	return routeCallShape{method: name, pathIdx: 2, handlerIdx: 3, optsIdx: 4}, true
}

// staticHTTPMethod resolves a method argument that is a string literal ("GET")
// or an http.MethodX selector (http.MethodGet) to its method name.
func staticHTTPMethod(arg ast.Expr) (string, bool) {
	switch m := arg.(type) {
	case *ast.BasicLit:
		if m.Kind == token.STRING {
			return strings.Trim(m.Value, `"`), true
		}
	case *ast.SelectorExpr:
		if pkg, ok := m.X.(*ast.Ident); ok && pkg.Name == "http" && strings.HasPrefix(m.Sel.Name, "Method") {
			return strings.ToUpper(strings.TrimPrefix(m.Sel.Name, "Method")), true
		}
	}
	return "", false
}
```

Add near the other method-name constants: `const registerHandlerFuncName = "RegisterHandler"`.

3b. Update `routeFromCall` (676-704) to consume the shape:

```go
func (w *routeWalker) routeFromCall(call *ast.CallExpr, prefixes map[string]string) *models.Route {
	shape, ok := w.a.validateServerCall(call, w.serverAliases)
	if !ok {
		return nil
	}

	rawPath, resolved := w.a.extractPathFromArg(call.Args[shape.pathIdx])
	if !resolved {
		w.a.addWarningf("skipping a server.%s route: its path argument could not be resolved to a literal string", shape.method)
		return nil
	}

	prefix := ""
	if reg, ok := call.Args[1].(*ast.Ident); ok {
		prefix = prefixes[reg.Name]
	}

	route := &models.Route{
		Method: strings.ToUpper(shape.method),
		Tags:   []string{},
		Path:   normalizePath(prefix + rawPath),
	}
	if shape.handlerIdx < len(call.Args) {
		route.HandlerName, route.Request, route.Response, route.SuccessStatus =
			w.a.extractHandlerInfo(call.Args[shape.handlerIdx], w.astFile, w.filePath, w.structName)
	}
	for i := shape.optsIdx; i < len(call.Args); i++ {
		w.a.extractRouteMetadata(call.Args[i], route, w.serverAliases)
	}
	if w.a.isPublicRoute(call.Pos()) {
		route.Public = true
	}
	return route
}
```

3c. Change `extractHandlerInfo` (856-884) to take the handler expression directly — delete its internal `len(callExpr.Args) <= 3` guard and `callExpr.Args[3]` access; first parameter becomes `handlerArg ast.Expr` passed straight to `a.resolveHandler(handlerArg, ...)`. Update any other caller the compiler flags.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/analyzer/ -run TestRegisterHandlerGenericForm -v && go test ./...`
Expected: PASS everywhere (goldens untouched — no fixture uses RegisterHandler yet).

- [ ] **Step 5: Commit**

```bash
git add internal/analyzer/
git commit -m "feat(analyzer): recognize server.RegisterHandler generic registration form"
```

### Task 6: Honor `WithModule` override; stamp only unset modules

**Files:**
- Modify: `internal/analyzer/analyzer.go:980-998` (`extractRouteMetadata` — new case), `~345-350` (module stamping loop in `analyzeGoFile`)
- Test: `internal/analyzer/analyzer_test.go`

**Interfaces:**
- Produces: `route.Module` may now be pre-set by `WithModule(...)`; the stamping loop in `analyzeGoFile` fills it only when empty (`Package` is always stamped).

- [ ] **Step 1: Write the failing test**

```go
// TestWithModuleOverride verifies server.WithModule(name) overrides the
// route's owning-module namespace (tags/operationId grouping) while routes
// without it keep the discovering module's name.
func TestWithModuleOverride(t *testing.T) {
	src := `package mod
import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)
type Module struct{}
func (m *Module) Name() string { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error { return nil }
type Thing struct{ ID int64 ` + "`json:\"id\"`" + ` }
func (m *Module) a(ctx server.HandlerContext) (server.Result[Thing], server.IAPIError) { return server.NewResult(200, Thing{}), nil }
func (m *Module) b(ctx server.HandlerContext) (server.Result[Thing], server.IAPIError) { return server.NewResult(200, Thing{}), nil }
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/renamed", m.a, server.WithModule("billing"))
	server.GET(hr, r, "/normal", m.b)
}
`
	_, routes := analyzeSingleModule(t, src)

	renamed := routeForPath(t, routes, "GET /renamed")
	assert.Equal(t, "billing", renamed.Module, "WithModule must override the module namespace")

	normal := routeForPath(t, routes, "GET /normal")
	assert.Equal(t, "mod", normal.Module, "routes without WithModule keep the discovering module")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/analyzer/ -run TestWithModuleOverride -v`
Expected: FAIL — `renamed.Module` is "mod" (stamping overwrites).

- [ ] **Step 3: Implement**

3a. `extractRouteMetadata` — add before the closing brace of the switch:

```go
	case "WithModule":
		// Overrides RouteDescriptor.ModuleName at runtime; mirror it so
		// tags/operationId grouping matches the live registry.
		if name := a.extractStringFromFirstArg(callExpr); name != "" {
			route.Module = name
		}
```

3b. In `analyzeGoFile`'s stamping loop (~line 345), make Module conditional:

```go
	for i := range module.Routes {
		if module.Routes[i].Module == "" {
			module.Routes[i].Module = module.Name
		}
		module.Routes[i].Package = module.Package
	}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/analyzer/ -run TestWithModuleOverride -v && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/analyzer/
git commit -m "feat(analyzer): honor server.WithModule route-namespace override"
```

### Task 7: Refresh testdata go-bricks pins to v0.45.0

**Files:**
- Modify: every `internal/spectest/testdata/*/go.mod` (15 existing + `delegation` already at v0.45.0)

- [ ] **Step 1: Bump the pins**

```bash
grep -rl "go-bricks v0.37.0" internal/spectest/testdata --include=go.mod | xargs sed -i '' 's/go-bricks v0\.37\.0/go-bricks v0.45.0/'
grep -rn "go-bricks v0" internal/spectest/testdata --include=go.mod
```

Expected: every fixture now requires `v0.45.0`.

- [ ] **Step 2: Verify goldens unaffected and suite green**

Run: `go test ./...`
Expected: PASS with NO golden changes (the pin feeds nothing the generator emits; if a golden changes, stop and investigate).

- [ ] **Step 3: Commit, run PR2 gates, open PR**

```bash
git add internal/spectest/testdata/
git commit -m "test: pin fixture go-bricks requirement to v0.45.0"
```

Run `/simplify`, apply, re-test; run `/security-review`, resolve; then:

```bash
git push -u origin feature/gb045-pr2-directive
gh pr create --base feature/gb045-pr1-delegation --title "feat(analyzer): //openapi:public directive, RegisterHandler + WithModule recognition" --body "$(cat <<'EOF'
## Summary
- Replaces phantom `server.WithPublic()` (verified: never existed in ANY go-bricks release — code using it cannot compile) with an `//openapi:public` comment directive. Goldens for the rewritten fixtures are byte-identical, proving drop-in equivalence.
- Recognizes the exported generic `server.RegisterHandler(hr, r, method, path, handler, opts...)` form (string literal or http.MethodX constant); non-static methods warn and skip.
- Honors `server.WithModule(name)` so tags/operationId grouping matches the runtime registry.
- All 16 fixture go.mod pins now require go-bricks v0.45.0.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

# PR 3 — fix(generator): v0.45 error/security accuracy

Branch: `git checkout -b feature/gb045-pr3-generator-accuracy` (from PR2 branch).

### Task 8: Remove spurious 422; typed error envelope

**Files:**
- Modify: `internal/generator/openapi.go:662-711` (`buildResponses` — drop 422 block, update 400 description), `790-803` (delete `routeHasValidation` if unreferenced — `grep -n routeHasValidation` first), `1007-1016` (`errorResponseSchema` — typed)
- Test: `internal/generator/openapi_test.go` + regenerate all goldens

**Interfaces:**
- Consumes: `metaEnvelopeSchema()` (the existing meta property builder used by `successResponseSchema`), `propNameError/propNameMeta/propNameCode/propNameMessage`, `typeObject/typeString` constants.
- Produces: `ErrorResponse` component with nested `error{code,message,details}` + `meta{timestamp,traceId}`; no route emits 422.

- [ ] **Step 1: Write the failing test**

Add to `internal/generator/openapi_test.go` (mirror the file's existing spec-string assertions style):

```go
// TestNo422AndTypedErrorEnvelope locks the v0.45 error contract: validation
// failures surface as the framework's 400 (422 only arises from explicit
// BusinessLogicError, invisible to static analysis), and ErrorResponse models
// the real envelope {error:{code,message,details}, meta:{timestamp,traceId}}.
func TestNo422AndTypedErrorEnvelope(t *testing.T) {
	project := &models.Project{
		Name: "svc", Version: "1.0.0",
		Modules: []models.Module{{
			Name: "users", Package: "users",
			Routes: []models.Route{{
				Method: "POST", Path: "/users", HandlerName: "create", Module: "users", Package: "users",
				Request: &models.TypeInfo{Name: "CreateUser", Package: "users", Fields: []models.FieldInfo{
					{Name: "Email", Type: "string", JSONName: "email", Required: true, RawValidation: "required,email"},
				}},
			}},
		}},
		Types: map[string]*models.TypeInfo{},
	}
	spec, err := New("", "", "").Generate(project)
	require.NoError(t, err)

	assert.NotContains(t, spec, `"422"`, "no 422: the framework returns 400 on validation failure")
	assert.Contains(t, spec, "Bad Request", "400 remains the validation-failure response")
	assert.Contains(t, spec, "validationErrors", "the 400 details contract is documented")
	assert.Contains(t, spec, "traceId", "meta is typed, not a bare object")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generator/ -run TestNo422AndTypedErrorEnvelope -v`
Expected: FAIL — spec contains `"422"` and no `validationErrors`.

- [ ] **Step 3: Implement**

3a. In `buildResponses`: change the 400 line and delete the 422 block:

```go
	responses := map[string]*OpenAPIResponse{
		"400": {Description: "Bad Request — malformed request or failed validation", Content: jsonMediaRef(errorSchema)},
		"500": {Description: "Internal Server Error", Content: jsonMediaRef(errorSchema)},
	}
```

(delete the `if routeHasValidation(route.Request) { responses["422"] = ... }` block). Update the function's doc comment (lines 662-666) to state: 400 covers binding AND validation failures (v0.45 returns 400 with `details.validationErrors`; 422 only arises from handler-level BusinessLogicError, invisible statically). Run `grep -n routeHasValidation internal/generator/` — if `buildResponses` was the only caller, delete the function (and its test if one exists).

3b. Replace `errorResponseSchema` (1007-1016):

```go
// errorResponseSchema mirrors go-bricks' error envelope:
// {error:{code,message,details?}, meta:{timestamp,traceId}}. details carries
// contextual payloads — a 400 validation failure holds
// validationErrors: [{field,message,value}] — and is emitted only in
// development environments, so it is documented but not required.
func errorResponseSchema() *OpenAPISchema {
	return &OpenAPISchema{
		Type: typeObject,
		Properties: map[string]*OpenAPIProperty{
			propNameError: {
				Type:        typeObject,
				Description: "Error details",
				Properties: map[string]*OpenAPIProperty{
					propNameCode:    {Type: typeString, Description: "Machine-readable error code (e.g. BAD_REQUEST, NOT_FOUND, INTERNAL_ERROR, or a custom business code)"},
					propNameMessage: {Type: typeString, Description: "Human-readable error message"},
					"details":       {Type: typeObject, Description: "Contextual error payload, emitted in development environments only. Validation failures carry validationErrors: [{field, message, value}]."},
				},
			},
			propNameMeta: metaEnvelopeSchema(),
		},
		Required: []string{propNameError},
	}
}
```

- [ ] **Step 4: Regenerate goldens, review, run suite**

Run: `go test ./internal/spectest -update && git diff --stat internal/spectest/testdata/`
Review the diff: expected changes are ONLY (a) removed 422 responses, (b) the richer ErrorResponse schema, (c) the new 400 description. Then: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generator/ internal/spectest/testdata/
git commit -m "fix(generator): drop spurious 422, type the error envelope for v0.45

go-bricks returns 400 on validation failure; 422 only arises from explicit
BusinessLogicError, which static analysis cannot see."
```

### Task 9: JOSE error catalog (401, 415, sealed post-trust 500)

**Files:**
- Modify: `internal/generator/openapi.go:693-705` (`buildResponses` JOSE branch), `1018-1030` (`joseErrorEnvelopeSchema` doc/description only if needed)
- Test: `internal/generator/openapi_test.go` + `internal/spectest/testdata/jose/expected.yaml` (regen)

**Interfaces:**
- Consumes: `errorSchemaName(route)`, `schemaJOSEErrorEnvelope`, `jsonMediaRef`, `mediaJOSE`, `joseTokenSchema()`, `OpenAPIMediaType`.

- [ ] **Step 1: Write the failing test**

```go
// TestJOSEErrorCatalog locks the v0.45 JOSE failure contract: pre-trust
// failures are application/json minimal envelopes on 400 (malformed), 401
// (decrypt/signature/kid — the primary auth-failure class), and 415 (plaintext
// rejected); post-trust failures are sealed application/jose, modeled as an
// alternate 500 content type.
func TestJOSEErrorCatalog(t *testing.T) {
	project := &models.Project{
		Name: "svc", Version: "1.0.0",
		Modules: []models.Module{{
			Name: "vault", Package: "vault",
			Routes: []models.Route{{
				Method: "POST", Path: "/seal", HandlerName: "seal", Module: "vault", Package: "vault",
				Request: &models.TypeInfo{Name: "SealReq", Package: "vault", JOSE: true, Fields: []models.FieldInfo{
					{Name: "Payload", Type: "string", JSONName: "payload"},
				}},
			}},
		}},
		Types: map[string]*models.TypeInfo{},
	}
	spec, err := New("", "", "").Generate(project)
	require.NoError(t, err)

	assert.Contains(t, spec, `"401"`, "JOSE decrypt/verify failures are 401")
	assert.Contains(t, spec, `"415"`, "plaintext on a JOSE route is 415")
	assert.Contains(t, spec, "JOSE_PLAINTEXT_REJECTED")
	assert.Contains(t, spec, "application/jose", "post-trust errors are sealed")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generator/ -run TestJOSEErrorCatalog -v`
Expected: FAIL — no 401/415 in the spec.

- [ ] **Step 3: Implement — in `buildResponses`, after the `responses` map literal**

```go
	// JOSE routes carry the full pre-trust failure catalog: 401 for
	// decrypt/verify/kid failures (the primary class), 415 for a plaintext
	// request on a sealed route. Post-trust failures (after inbound verify)
	// are sealed application/jose envelopes; pre-trust ones cannot be (no
	// established keys), hence the dual 500 content.
	if errorSchema == schemaJOSEErrorEnvelope {
		responses["401"] = &OpenAPIResponse{
			Description: "Unauthorized — JOSE decrypt/verify failure (JOSE_DECRYPT_FAILED, JOSE_SIGNATURE_INVALID, JOSE_KID_UNKNOWN, JOSE_KID_MISSING)",
			Content:     jsonMediaRef(errorSchema),
		}
		responses["415"] = &OpenAPIResponse{
			Description: "Unsupported Media Type — plaintext request on a JOSE route (JOSE_PLAINTEXT_REJECTED)",
			Content:     jsonMediaRef(errorSchema),
		}
		responses["500"].Content[mediaJOSE] = &OpenAPIMediaType{Schema: joseTokenSchema()}
	}
```

- [ ] **Step 4: Regenerate the jose golden, review, run suite**

Run: `go test ./internal/spectest -update && git diff internal/spectest/testdata/jose/expected.yaml | head -60` then `go test ./...`
Expected: jose golden gains 401/415 + dual-content 500; all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generator/ internal/spectest/testdata/
git commit -m "fix(generator): document the real JOSE failure catalog (401, 415, sealed 500)"
```

### Task 10: Honest tenant scheme + `--tenant-header` flag

**Files:**
- Modify: `internal/generator/openapi.go:101-110` (Config), `112-120` + `234-253` (generator field + NewWithConfig), `373-408` (`securityScheme` struct gains Description; `securitySchemes()` parameterized)
- Modify: `internal/commands/generate.go` (GenerateOptions field, flag, Config wiring)
- Test: `internal/generator/openapi_test.go`, `internal/commands/generate_test.go`

**Interfaces:**
- Produces: `Config.TenantHeader string` (empty → `X-Tenant-ID`); flag `--tenant-header`; scheme description constant `tenantSchemeDescription`.

- [ ] **Step 1: Write the failing tests**

Generator test:

```go
// TestTenantSchemeHonestyAndHeaderOverride verifies the tenant scheme
// documents its deployment-dependent enforcement (400 on failure, not 401)
// and that --tenant-header renames the header.
func TestTenantSchemeHonestyAndHeaderOverride(t *testing.T) {
	project := &models.Project{Name: "svc", Version: "1.0.0", Modules: []models.Module{}, Types: map[string]*models.TypeInfo{}}

	spec, err := New("", "", "").Generate(project)
	require.NoError(t, err)
	assert.Contains(t, spec, "X-Tenant-ID", "default header name")
	assert.Contains(t, spec, "multitenancy", "description states enforcement is deployment-dependent")
	assert.Contains(t, spec, "400", "description states failure mode is 400")

	custom, err := NewWithConfig(&Config{TenantHeader: "X-Org-ID"}).Generate(project)
	require.NoError(t, err)
	assert.Contains(t, custom, "X-Org-ID")
	assert.NotContains(t, custom, "X-Tenant-ID")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generator/ -run TestTenantSchemeHonesty -v`
Expected: FAIL — no description, no override support.

- [ ] **Step 3: Implement**

3a. `Config` gains:

```go
	// TenantHeader overrides the header name in the tenant security scheme
	// (default X-Tenant-ID; go-bricks header resolvers are configurable).
	TenantHeader string
```

3b. Generator field `tenantHeader string`; in `New(...)` set `tenantHeader: defaultTenantHeader`; in `NewWithConfig`:

```go
	tenantHeader := cfg.TenantHeader
	if tenantHeader == "" {
		tenantHeader = defaultTenantHeader
	}
```

with `const defaultTenantHeader = "X-Tenant-ID"` next to `tenantSecurityScheme` (line 374), replacing the hardcoded literal at line 400.

3c. `securityScheme` struct (line 383) gains `Description string \`yaml:"description,omitempty"\``; `securitySchemes()` becomes a method using the field:

```go
// tenantSchemeDescription is deliberately honest about v0.45 semantics: the
// header is enforced only for multi-tenant deployments using a header
// resolver, the failure mode is 400 (not the 401 an apiKey scheme usually
// implies), and subdomain/path resolvers never read it.
const tenantSchemeDescription = "Tenant identifier. Enforced only when the service runs with " +
	"multitenancy enabled and a header-based tenant resolver; a missing or invalid tenant yields " +
	"HTTP 400. Deployments resolving the tenant from the subdomain or URL path do not read this header."

func (g *OpenAPIGenerator) securitySchemes() map[string]securityScheme {
	return map[string]securityScheme{
		tenantSecurityScheme: {Type: "apiKey", In: paramTypeHeader, Name: g.tenantHeader, Description: tenantSchemeDescription},
	}
}
```

Update the call site (line 332) to `g.securitySchemes()`.

3d. `internal/commands/generate.go`: add `TenantHeader string` to `GenerateOptions`, register after line 90:

```go
	cmd.Flags().StringVar(&opts.TenantHeader, "tenant-header", "", "Header name for the tenant security scheme (default X-Tenant-ID)")
```

and wire `TenantHeader: opts.TenantHeader` where the `generator.Config` is built (find the `Config{` literal in generate.go's run path). Add a generate_test.go case asserting the flag reaches the spec (mirror the existing `--no-tenant-security` test).

- [ ] **Step 4: Regenerate goldens (description lands in every fixture), review, run suite**

Run: `go test ./internal/spectest -update && git diff --stat internal/spectest/testdata/ && go test ./...`
Expected: goldens gain only the scheme description; suite PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generator/ internal/commands/ internal/spectest/testdata/
git commit -m "fix(generator): honest tenant-scheme description + --tenant-header override"
```

### Task 11: Doctor "verified through v0.45.0" + README/docs refresh

**Files:**
- Modify: `internal/commands/doctor.go:33-39` (new constant), `~287+303` (output lines)
- Modify: `internal/commands/doctor_test.go` (output assertions)
- Modify: `README.md:85` (targets line), flags table (add `--tenant-header`), new "Known limitations" bullets

- [ ] **Step 1: Update doctor**

In the constants block (after line 39):

```go
	// verifiedGoBricksVer is the newest go-bricks release the analyzer's
	// recognized patterns and the generator's emitted runtime contract have
	// been verified against (fixtures + the demo-project acceptance run).
	// Bump it as part of each framework-compatibility pass.
	verifiedGoBricksVer = "v0.45.0"
```

Change the success line (line 303):

```go
	fmt.Printf("✅ %s version compatible (floor %s, verified through %s)\n", goBricksDep, minGoBricksVer, verifiedGoBricksVer)
```

Update the corresponding doctor_test.go expected-output assertions (grep for `version compatible`).

- [ ] **Step 2: Update README**

- Line 85 → `- Targets **GoBricks v0.13.0+** projects (verified through **v0.45.0**; the `doctor` command enforces the floor).`
- Flags table: add `| --tenant-header | Header name for the tenant security scheme (default X-Tenant-ID) |`.
- Document the `//openapi:public` directive in the usage section (short example above a `server.GET` call).
- Add a "Known limitations" list: raw/untyped routes (`r.Add`, `RegisterReadyHandler`), routes registered outside module `RegisterRoutes` (e.g. `RootGroup()` in main), embedded-module method promotion, response trace headers (`traceparent`, `X-Request-ID`) not modeled, JOSE `jose:` tags recognized on the `_` sentinel field only, `server.WithMiddleware(...)` intentionally ignored (middleware names carry no spec semantics), tenant enforcement not derived from runtime config.

- [ ] **Step 3: Run everything, commit**

Run: `go test ./... && go build ./...`
Expected: PASS.

```bash
git add internal/commands/ README.md
git commit -m "docs: verified-through v0.45.0 statement, --tenant-header, known limitations"
```

### Task 12: PR3 gates, final acceptance, pull request

- [ ] **Step 1: Full acceptance run against the demo project**

```bash
go run ./cmd/go-bricks-openapi doctor --project ../go-bricks-demo-project
go run ./cmd/go-bricks-openapi generate --project ../go-bricks-demo-project --output /tmp/demo-openapi.yaml --strict --validate --verbose
python3 -c "
import yaml; d = yaml.safe_load(open('/tmp/demo-openapi.yaml'))
ops = sum(len(v) for v in d['paths'].values()); print('operations:', ops)
assert ops == 15, 'expected 15 operations'
assert '422' not in open('/tmp/demo-openapi.yaml').read()"
```

Expected: doctor green with "verified through v0.45.0"; strict+validate generation succeeds; 15 operations; no 422.

- [ ] **Step 2: Run /simplify, then /security-review; fix findings; re-run tests**

- [ ] **Step 3: Push and open PR**

```bash
git push -u origin feature/gb045-pr3-generator-accuracy
gh pr create --base feature/gb045-pr2-directive --title "fix(generator): accurate v0.45 error and security modeling" --body "$(cat <<'EOF'
## Summary
- Removes the spurious 422 (go-bricks returns 400 on validation failure; verified against v0.45 source) and types the error envelope {error:{code,message,details}, meta:{timestamp,traceId}}.
- JOSE routes now document the real failure catalog: 401 (decrypt/signature/kid), 415 (plaintext rejected), sealed application/jose post-trust 500.
- Tenant scheme description states the honest v0.45 semantics (deployment-dependent enforcement, 400 failure mode); new --tenant-header flag for custom header names.
- doctor reports "verified through v0.45.0"; README documents //openapi:public and known limitations.

## Acceptance
- go-bricks-demo-project (v0.45.0): doctor green, strict+validated generation, 15/15 operations, no 422.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Post-plan notes for the executor

- Merge order: PR1 → retarget PR2 to main → merge → retarget PR3 → merge (CodeRabbit is a required reviewer on main's ruleset; expect gosec G304 and Sonar S3776 chatter — keep new functions small and use the established `#nosec G304` justification pattern only for validated in-project paths).
- If any task's premise is contradicted by the code (line numbers drift, a helper is missing), stop and re-read the surrounding code before adapting — do not force the plan's code verbatim onto a moved target.
- The spec (`docs/superpowers/specs/2026_07_01_go_bricks_v045_compat_design.md`) is the source of truth for scope disputes.
