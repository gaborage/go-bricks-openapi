package analyzer

import (
	"go/ast"
	"go/token"
	"slices"
	"strconv"
)

// Constructor Resolution: one table maps every go-bricks `server` constructor
// this analyzer recognizes to the HTTP status it produces — success
// constructors (which document the operation's success response) and error
// constructors (whose statuses are inferred as extra error responses) alike.
//
// The two readers filter the same table: statusForConstructor answers the
// success path and ignores error entries; errorStatusFromCall answers the
// inference path and ignores success entries.

// constructorSpec is one row of the constructor table.
type constructorSpec struct {
	// status is the fixed status the constructor produces, used when statusArg
	// is negative.
	status int
	// statusArg is the index of the call argument carrying the status, or -1
	// when the status is fixed. An argument this analyzer cannot read (a
	// variable, a computed expression) resolves to 0 — "not resolvable".
	statusArg int
	// isError marks a constructor that produces an error response rather than
	// a success Result.
	isError bool
}

// successSpec / errorSpec build the fixed-status rows of the table.
func successSpec(status int) constructorSpec {
	return constructorSpec{status: status, statusArg: -1}
}

func errorSpec(status int) constructorSpec {
	return constructorSpec{status: status, statusArg: -1, isError: true}
}

// constructorStatus is the single constructor -> status table. Error
// constructors are the go-bricks v0.53.0 `server` package set.
var constructorStatus = map[string]constructorSpec{
	// Success constructors.
	"Created":           successSpec(201),
	"Accepted":          successSpec(202),
	"NoContent":         successSpec(204),
	"NewResult":         {statusArg: 0},
	"NewResultWithMeta": {statusArg: 0},

	// Error constructors.
	"NewBadRequestError":         errorSpec(400),
	"NewValidationError":         errorSpec(400),
	"NewUnauthorizedError":       errorSpec(401),
	"NewForbiddenError":          errorSpec(403),
	"NewNotFoundError":           errorSpec(404),
	"NewConflictError":           errorSpec(409),
	"NewBusinessLogicError":      errorSpec(422),
	"NewTooManyRequestsError":    errorSpec(429),
	"NewInternalServerError":     errorSpec(500),
	"NewServiceUnavailableError": errorSpec(503),
	// NewBaseAPIError(code, message, status) carries its status third.
	"NewBaseAPIError": {statusArg: 2, isError: true},
}

// resolve returns the status a call with these arguments produces, or 0 when it
// is not resolvable. shadows carries the enclosing body's local declarations,
// so an `http` qualifier that body hides resolves to nothing.
func (s constructorSpec) resolve(args []ast.Expr, shadows shadowIndex) int {
	if s.statusArg < 0 {
		return s.status
	}
	if s.statusArg >= len(args) {
		return 0
	}
	return statusFromArg(args[s.statusArg], shadows)
}

// statusForConstructor maps a server result-constructor name to its HTTP success
// status. For NewResult/NewResultWithMeta the status is the first argument
// resolved via successStatusFromArg (an integer literal or an http.StatusXxx
// constant, in the success range). Returns 0 for anything unrecognized, and for
// an error constructor: an error return never documents the operation's success
// response.
func statusForConstructor(name string, args []ast.Expr, shadows shadowIndex) int {
	spec, ok := constructorStatus[name]
	if !ok || spec.isError {
		return 0
	}
	return successStatus(spec.resolve(args, shadows))
}

// successStatusFromArg resolves a status argument on the SUCCESS path: the
// Status field of a Result literal, and the `<ident>.Status = …` write that
// overrides it.
func successStatusFromArg(arg ast.Expr, shadows shadowIndex) int {
	return successStatus(statusFromArg(arg, shadows))
}

// successStatus keeps the success path to the 2xx range. A handler returning a
// 4xx/5xx as a non-error Result is a misuse the generator does not try to model
// as a success response: stamping it would replace the operation's baseline 400
// (or an inferred error response of that code) with a success entry. Outside
// the range the caller falls back to its 200 default.
func successStatus(code int) int {
	if code < successStatusMin || code > successStatusMax {
		return 0
	}
	return code
}

// successStatusMin/successStatusMax bound the status codes the success path
// accepts — the HTTP success range.
const (
	successStatusMin = 200
	successStatusMax = 299
)

// inferErrorStatuses collects the error statuses a handler body demonstrably
// produces, by matching calls to the go-bricks error constructors against the
// constructor table. The result is sorted and deduplicated.
//
// The walk covers the handler's own body ONLY — every statement in it,
// including nested blocks and closures declared inside it, but no call it
// makes. A constructor reached through a helper (even one in the same package)
// is invisible here by design; `//openapi:errors` declares what the walk cannot
// see. README's "Known limitations" documents the boundary.
func (a *ProjectAnalyzer) inferErrorStatuses(funcDecl *ast.FuncDecl, serverAliases map[string]struct{}) []int {
	if funcDecl.Body == nil {
		return nil
	}
	shadows := newShadowIndex(funcDecl)
	var codes []int
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if code := a.errorStatusFromCall(call, serverAliases, shadows); code != 0 {
			codes = append(codes, code)
		}
		return true
	})
	return mergeErrorStatuses(nil, codes)
}

// errorStatusFromCall resolves one call expression to the error status it
// produces, or 0. Only a call qualified by the local alias(es) the go-bricks
// server package is imported under counts, so a same-named constructor on
// another package contributes nothing. A status outside the error range — an
// unresolvable NewBaseAPIError argument, or a resolvable but non-error one —
// is ignored silently, with no diagnostic: a missing error response costs one
// documented branch, a wrong one misleads every consumer.
//
// A qualifier the body itself declares — `server := newFake()`, or a parameter
// named `server` — names that local, not the import, and contributes nothing.
func (a *ProjectAnalyzer) errorStatusFromCall(call *ast.CallExpr, serverAliases map[string]struct{}, shadows shadowIndex) int {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return 0
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || !a.aliasContains(serverAliases, pkg.Name, frameworkPkgServer) {
		return 0
	}
	if shadows.shadows(pkg.Name, pkg.Pos()) {
		return 0
	}
	spec, ok := constructorStatus[sel.Sel.Name]
	if !ok || !spec.isError {
		return 0
	}
	code := spec.resolve(call.Args, shadows)
	if code < errorStatusMin || code > errorStatusMax {
		return 0
	}
	return code
}

// mergeErrorStatuses unions added into existing, returning a sorted,
// deduplicated set. With nothing to add the existing set is returned unchanged,
// so a route that carries no new codes keeps exactly what it had.
func mergeErrorStatuses(existing, added []int) []int {
	if len(added) == 0 {
		return existing
	}
	codes := append(slices.Clone(existing), added...)
	slices.Sort(codes)
	return slices.Compact(codes)
}

// statusFromArg resolves a status argument: NewResult's first, a Result
// literal's Status field, or NewBaseAPIError's third. It accepts a bare integer
// literal (NewResult(201, ...)) and the idiomatic net/http status constant
// (NewResult(http.StatusCreated, ...)), which is the dominant real-world form.
// Returns 0 when the argument is anything else (a variable, a non-http
// constant), letting the caller fall back to its default.
//
// It resolves the WHOLE constant set; each path then keeps its own range —
// successStatus for the success side, errorStatusFromCall for the error side.
// An `http` qualifier the enclosing body declares itself names that local, not
// net/http, and resolves to nothing — the caller's own fallback then applies.
func statusFromArg(arg ast.Expr, shadows shadowIndex) int {
	switch a := arg.(type) {
	case *ast.BasicLit:
		if a.Kind == token.INT {
			if v, err := strconv.Atoi(a.Value); err == nil {
				return v
			}
		}
	case *ast.SelectorExpr:
		if pkg, ok := a.X.(*ast.Ident); ok && pkg.Name == stdlibPkgHTTP && !shadows.shadows(pkg.Name, pkg.Pos()) {
			return httpStatusConstants[a.Sel.Name]
		}
	}
	return 0
}

// httpStatusConstants maps the net/http 2xx, 4xx and 5xx status-constant names
// to their codes. The success range serves the result constructors; the error
// ranges serve NewBaseAPIError's status argument. Resolving a name here does
// NOT make it usable on either path — the success path keeps itself to 2xx
// (successStatus) and the inference path to 4xx/5xx (errorStatusFromCall). 1xx
// and 3xx are absent: no go-bricks constructor produces one, and a handler
// naming one is not a shape this analyzer models.
var httpStatusConstants = map[string]int{
	"StatusOK":                   200,
	"StatusCreated":              201,
	"StatusAccepted":             202,
	"StatusNonAuthoritativeInfo": 203,
	"StatusNoContent":            204,
	"StatusResetContent":         205,
	"StatusPartialContent":       206,
	"StatusMultiStatus":          207,
	"StatusAlreadyReported":      208,
	"StatusIMUsed":               226,

	"StatusBadRequest":                   400,
	"StatusUnauthorized":                 401,
	"StatusPaymentRequired":              402,
	"StatusForbidden":                    403,
	"StatusNotFound":                     404,
	"StatusMethodNotAllowed":             405,
	"StatusNotAcceptable":                406,
	"StatusProxyAuthRequired":            407,
	"StatusRequestTimeout":               408,
	"StatusConflict":                     409,
	"StatusGone":                         410,
	"StatusLengthRequired":               411,
	"StatusPreconditionFailed":           412,
	"StatusRequestEntityTooLarge":        413,
	"StatusRequestURITooLong":            414,
	"StatusUnsupportedMediaType":         415,
	"StatusRequestedRangeNotSatisfiable": 416,
	"StatusExpectationFailed":            417,
	"StatusTeapot":                       418,
	"StatusMisdirectedRequest":           421,
	"StatusUnprocessableEntity":          422,
	"StatusLocked":                       423,
	"StatusFailedDependency":             424,
	"StatusTooEarly":                     425,
	"StatusUpgradeRequired":              426,
	"StatusPreconditionRequired":         428,
	"StatusTooManyRequests":              429,
	"StatusRequestHeaderFieldsTooLarge":  431,
	"StatusUnavailableForLegalReasons":   451,

	"StatusInternalServerError":           500,
	"StatusNotImplemented":                501,
	"StatusBadGateway":                    502,
	"StatusServiceUnavailable":            503,
	"StatusGatewayTimeout":                504,
	"StatusHTTPVersionNotSupported":       505,
	"StatusVariantAlsoNegotiates":         506,
	"StatusInsufficientStorage":           507,
	"StatusLoopDetected":                  508,
	"StatusNotExtended":                   510,
	"StatusNetworkAuthenticationRequired": 511,
}
