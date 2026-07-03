# Design: go-bricks v0.45.0 compatibility

**Date:** 2026-07-01
**Status:** Approved
**Scope decision:** Full v0.45 accuracy (parse correctness + spec accuracy), delivered as 3 stacked PRs.

## Background

go-bricks-openapi is a pure `go/ast` static analyzer: it parses go-bricks consumer
source and never imports or builds the framework. go-bricks v0.45.0 shipped the
ADR-034 "echo-free boundary" breaking release. Research (4-agent workflow +
follow-up analysis, verified against the v0.45 source and empirically against
go-bricks-demo-project) established:

- The core module contract (`RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar)`)
  and typed-handler shapes are **unchanged since v0.13.0**; none of the v0.40–v0.45
  breaking changes alter the code shapes the analyzer parses.
- The happy path is accurate: success envelope `{data, meta{timestamp, traceId}}`,
  constructor status codes (Created→201, Accepted→202, NoContent→204, NewResult→n),
  JOSE wire shape (`application/jose`), and raw-response mode all match v0.45.
- Two parse-correctness defects and a cluster of spec-accuracy defects make the
  tool unreliable for v0.45 projects today (details below).

"v0.45-compatible" means: the tool **parses all idiomatic v0.45 consumer shapes**
and its emitted specs **accurately describe a v0.45 runtime**.

## Verified defects driving this work

1. **Handler-field delegation drops every route** (empirical: demo project emits
   `paths: {}`, 0/15 routes). `maybeRecurseHelper` (analyzer.go:714-725) only
   recurses when the call receiver is a bare ident equal to the module receiver;
   `m.handler.RegisterRoutes(hr, r)` — the shape used by all 5 demo modules — is
   silently dropped.
2. **`server.WithPublic()` is a phantom API.** It has never existed in any
   go-bricks version (verified via `git log -S` across full history); consumer
   code using it cannot compile. The analyzer's recognition (analyzer.go:996-998),
   `models.Route.Public`, the generator's `security: []` path, and the testdata
   fixtures that use it are all dead or invalid.
3. **Spurious 422.** The generator emits `422` for validation-tagged requests
   (openapi.go:663-666), but go-bricks returns **400** for validation failures
   (`NewBadRequestError` with `details.validationErrors`); 422 only arises from
   explicit `BusinessLogicError`, invisible to static analysis.
4. **Tenant security over-claims.** `X-Tenant-ID` apiKey is emitted document-wide
   by default, but the v0.45 runtime enforces it only when `multitenant.enabled`
   is true with a header resolver (subdomain/path resolvers don't read it; the
   header name is configurable; failure is 400, not 401).
5. **JOSE errors under-modeled.** Spec emits only 400/500 for JOSE routes; the
   runtime's primary failure codes are **401** (decrypt/signature/kid) and **415**
   (plaintext rejected), and post-trust errors are sealed `application/jose`, not JSON.
6. **Shallow error envelope.** Runtime emits `{error: {code, message, details?},
   meta: {timestamp, traceId}}`; the spec types `error`/`meta` as bare objects.
7. **Gaps:** generic `server.RegisterHandler[T,R](hr, r, method, path, handler, opts...)`
   form unrecognized; `WithModule` (overrides module/tag/operationId namespace)
   silently ignored; no version reference anywhere newer than testdata's v0.37.0 pins.

## Design

### 1. Compatibility policy

- Doctor floor stays **v0.13.0** (honest — core contract stable since then).
- README + doctor output gain a **"verified through v0.45.0"** statement
  (new constant alongside `minGoBricksVer`).
- All testdata `go.mod` pins move v0.37.0 → v0.45.0.
- Fixtures referencing `WithPublic` are rewritten so every fixture is valid,
  compilable v0.45 consumer code.

### 2. Analyzer — parse correctness

- **Field delegation:** extend `maybeRecurseHelper` so a receiver of form
  `<recvVar>.<field>.<method>(...)` resolves the field's declared struct type
  (reusing `resolveFieldType` from the handler-args path), finds the method on
  that struct (cross-file within the package, same as today's recursion), and
  recurses with the registrar/prefix threaded through. Cycle guard key becomes
  `(typeName, methodName)` instead of bare method name. An unresolvable receiver
  or method produces a warning (fails under `--strict`) — never a silent drop.
- **`RegisterHandler` generic form:** recognized; HTTP method read from the
  string-literal method argument; non-literal method → warning.
- **`WithModule(name)`:** overrides the route's module/tag/operationId namespace.
- **`WithMiddleware`:** intentionally ignored; documented.
- **`//openapi:public` directive:** a comment line immediately above a route
  registration call (`server.<METHOD>` or `server.RegisterHandler`) marks that
  operation public. Replaces `WithPublic` recognition entirely;
  `models.Route.Public` stays as the carrier.

### 3. Generator — spec accuracy for v0.45

- **Remove 422.** Validation-tagged requests document the real **400** with a
  typed body; `details.validationErrors[]{field, message, value}` modeled as
  optional (runtime includes details only in development).
- **Typed error envelope:** `{error: {code: string, message: string, details?: object},
  meta: {timestamp: date-time, traceId: string}}`.
- **JOSE error catalog:** add **401** and **415**; pre-trust errors stay
  `application/json {code, message}`; post-trust errors documented as sealed
  `application/jose`.
- **Tenant security:** default stays ON (`--no-tenant-security` to omit,
  unchanged); scheme description corrected to state enforcement is
  deployment-dependent (multitenancy + header resolver) and failure is 400;
  new `--tenant-header <name>` flag renames the header in the scheme.
- **Public routes:** `Route.Public` → operation-level `security: []`.

### 4. Testing & verification

- TDD (superpowers:test-driven-development) throughout.
- New spectest fixtures: `delegation` (mirrors demo shape), `register_handler`,
  `with_module`, `public_directive`; `WithPublic` fixtures rewritten.
- Acceptance gate: tool vs go-bricks-demo-project emits **15/15 routes**
  (today: 0/15); doctor reports compatible.
- Sonar new-code coverage ≥80%: extract pure logic instead of stubbed var-seam
  closures (established seam pattern).

### 5. Delivery — 3 stacked PRs (≤400 LOC each)

1. `fix(analyzer):` handler-field delegation + `delegation` fixture.
2. `feat(analyzer):` `//openapi:public` directive (WithPublic removal),
   `RegisterHandler`, `WithModule`, testdata pin refresh.
3. `fix(generator):` 422 removal, typed error envelope, JOSE catalog,
   tenant description + `--tenant-header`, README/doctor version statements.

Per PR: build + full tests + demo-project empirical run, then `/simplify` +
`/security-review` before push; CodeRabbit required-reviewer gate on merge.

## Non-goals (documented as known limitations)

- Raw/untyped routes (`r.Add`, `RegisterReadyHandler`).
- Routes registered outside module `RegisterRoutes` (e.g. `RootGroup()` in main).
- Module methods promoted from embedded structs (no upstream pattern exists yet).
- `traceparent` / `X-Request-ID` response-header modeling.
- JOSE sentinel relaxation (`jose:` tag on named fields; documented convention is `_`).
- Reading target-project config YAML to detect multitenancy/resolver.
