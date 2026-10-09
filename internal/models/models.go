package models

// Project represents a simplified project structure for OpenAPI generation
type Project struct {
	Name        string
	Description string
	Version     string
	Modules     []Module
	// Types is the registry of every named struct type reachable from a route's
	// request or response (including nested and recursively-referenced types),
	// keyed by schema name. The generator emits one component schema per entry.
	Types map[string]*TypeInfo
}

// Module represents a go-bricks module
type Module struct {
	Name        string
	Package     string
	Description string
	Routes      []Route
}

// Route represents a discovered HTTP route
type Route struct {
	Method      string
	Path        string
	HandlerName string
	// OperationID is an explicit operationId set via server.WithHandlerName(...).
	// When set it is used verbatim (not module-qualified); otherwise the generator
	// derives a module-qualified, de-duplicated id from Module + HandlerName.
	OperationID string
	Summary     string
	Description string
	Tags        []string
	Request     *TypeInfo
	Response    *TypeInfo
	// Module and Package identify the owning go-bricks module, stamped at
	// discovery time. They survive the module->route flattening in the generator
	// (getAllRoutes) so later passes can namespace operationIds and disambiguate
	// component names across modules.
	Module  string
	Package string
	// SuccessStatus is the HTTP status of the success response, derived from the
	// handler's result constructor (server.Created -> 201, Accepted -> 202,
	// NewResult(n) -> n). Zero means "use the default" (200).
	SuccessStatus int
	// RawResponse is true when the route is registered WithRawResponse(): the
	// handler bypasses the standard data/meta envelope and returns its payload
	// directly (Strangler-Fig migration).
	RawResponse bool
	// Public is true when the registration is annotated with an
	// `//openapi:public` comment directive on the line(s) directly above it:
	// the generator emits operation-level `security: []` so liveness probes and
	// other tenant-agnostic endpoints (/health, /login, webhooks) are
	// documented as requiring no auth. (go-bricks itself has no per-route
	// tenant opt-out API as of v0.45.)
	Public bool
	// ErrorStatuses are extra error status codes the operation can return,
	// declared on the registration with an `//openapi:errors 404,409` comment
	// Directive, deduplicated and sorted ascending by the analyzer so the
	// stamped list is deterministic. The generator unions them with its unconditional 400/500
	// baseline (plus the JOSE 401/415 additions), deduplicating silently; each
	// added response reuses the route's error-envelope $ref and is described
	// with the canonical HTTP status text.
	ErrorStatuses []int
}

// UnresolvedRoute is one route registration the analysis found but could not
// reduce to a path — see CONTEXT.md, "Unresolved route". It is a diagnostic
// record, counted on its own and never folded into the typed/untyped ratio;
// the route itself never reaches the emitted document.
type UnresolvedRoute struct {
	// Form is the registration form as written, e.g. "server.GET" for a
	// server verb or "api.Add" for a raw registration on a group.
	Form string
	// File is the source file the registration lives in, relative to the
	// analyzed project root when that can be computed (absolute otherwise).
	File string
	// Line and Col are the 1-based source position of the registration call,
	// rendered as file:line:col like every other located analyzer diagnostic.
	Line int
	Col  int
	// Reason says what could not be resolved — the path argument itself, or
	// the group prefix of the registrar it was registered on.
	Reason string
}

// TypeInfo represents type metadata for requests and responses
type TypeInfo struct {
	Name    string
	Package string
	Fields  []FieldInfo
	// JOSE is true when the struct carries a `jose:"..."` tag on any field — typically
	// a sentinel `_ struct{}` field. Routes whose request or response type is JOSE-tagged
	// emit Content-Type: application/jose in the OpenAPI spec while keeping the documented
	// plaintext schema as the source of truth (the on-the-wire compact JOSE serialization
	// wraps that plaintext after decrypt-and-verify).
	JOSE bool
	// NoContent is true when the response type is server.NoContentResult — the
	// route returns 204 with no body. Such a TypeInfo carries no Name/Fields, so
	// no component schema is generated; later passes use this flag to emit a 204
	// response instead of a 200 with a body.
	NoContent bool
	// Shape is the decoded type of a response payload — the T of
	// server.Result[T] / server.ResultWithMeta[T], or of a bare handler return
	// T, which is documented exactly alike — using the same vocabulary
	// FieldInfo.Shape uses for struct fields. Nil for request types, for the
	// NoContentResult marker, and for a non-slice payload that registered as a
	// project struct (the analyzer sheds it, so the payload is $ref'd even when
	// its short name collides with a well-known type).
	//
	// For a slice payload ([]Item, []string) Kind is ShapeSlice — ShapeArray for
	// a fixed-size array payload ([4]byte, [2]Item) — and Elem is the
	// element's shape; Name keeps describing the ELEMENT (empty for a primitive
	// element), so type registration, schemaName and referencedSchemaNames need
	// no slice awareness: the component emitted for []Item is Item, and the
	// payload schema wraps a $ref to it in an array.
	//
	// For any other non-slice payload Shape is the argument as written (a
	// pointer kept; consumers shed one level). The generator documents a
	// well-known type (time.Time, uuid.UUID, json.Number, ...: WellKnownTypeNames) or
	// a builtin (string, int64, any, interface{}) inline from it and never $refs
	// it. A builtin carries no Name; a well-known type keeps its Name. Any other
	// name that resolves to no component is cleared by the analyzer, with a
	// warning, so the payload falls back to an untyped object.
	Shape *TypeShape
}

// Qualified names of the well-known stdlib/library types, spelled as the Shape
// decoder writes them (import alias + "." + name), so an aliased import
// (t "time" -> "t.Time") is deliberately not one.
const (
	WellKnownTimeTime     = "time.Time"
	WellKnownTimeDuration = "time.Duration"
	WellKnownUUID         = "uuid.UUID"
	WellKnownRawMessage   = "json.RawMessage"
	WellKnownJSONNumber   = "json.Number"
	WellKnownTimeMonth    = "time.Month"
	WellKnownTimeWeekday  = "time.Weekday"
)

// WellKnownTypeNames is the set of named types the generator documents inline
// from their Shape rather than as a component. It is the single list the
// analyzer screens response payload names against; a generator test pins the
// keys of the generator's wellKnownFormats equal to it, so the two cannot
// drift. A name the generator lacked would $ref a component that is never
// emitted; a name this set lacked would be warned about and left untyped.
var WellKnownTypeNames = map[string]bool{
	WellKnownTimeTime:     true,
	WellKnownTimeDuration: true,
	WellKnownUUID:         true,
	WellKnownRawMessage:   true,
	WellKnownJSONNumber:   true,
	WellKnownTimeMonth:    true,
	WellKnownTimeWeekday:  true,
}

// UntypedBuiltinNames are the Go builtins with no JSON Schema form: the
// generator types them as a bare object (setBasicTypeAndFormat's default),
// and the analyzer warns on any field leaf that is one. uintptr is not here:
// it has its own warning.
var UntypedBuiltinNames = map[string]bool{"error": true, "complex64": true, "complex128": true}

// ShapeKind classifies one level of a TypeShape. See CONTEXT.md: "Shape".
type ShapeKind string

const (
	ShapePointer ShapeKind = "pointer"
	ShapeSlice   ShapeKind = "slice"
	// ShapeArray is a fixed-size array ([N]T, any N — 0 and a constant name
	// included; the length is not recorded). It is kept apart from ShapeSlice
	// only because encoding/json base64-encodes a byte SLICE but writes a byte
	// array element by element; every other consumer treats it as a slice.
	ShapeArray     ShapeKind = "array"
	ShapeMap       ShapeKind = "map"
	ShapeNamed     ShapeKind = "named"     // a declared or qualified type name (Address, time.Time)
	ShapePrimitive ShapeKind = "primitive" // a builtin (string, int64, byte, any, interface{})
	ShapeUnknown   ShapeKind = "unknown"   // an AST shape the decoder does not model (chan, func, generics)

	// Resolution-only kinds. The analyzer's resolver writes them into a
	// Resolution; the Shape decoder never produces them, so a Shape stays
	// purely syntactic.

	// ShapeRef is a named struct that registers as a component. Name is its
	// component name: the declared name until registerFieldRefAt stamps the
	// final, collision-qualified one.
	ShapeRef ShapeKind = "ref"
	// ShapeKindOnly is a named scalar whose build-tagged declarations disagree
	// on width (#92). Name is the OpenAPI kind: integer, number, string or boolean.
	ShapeKindOnly ShapeKind = "kind-only"
	// ShapeMarshaler is a Marshaler type (CONTEXT.md) in a body position. Name
	// is the type that declares the method. Elem is its underlying resolution,
	// kept for #111. Nothing emits or registers what is under Elem.
	ShapeMarshaler ShapeKind = "marshaler"
	// ShapeRecursive is a named type met again inside its own resolution, or
	// past maxNamedResolutionDepth. Name is that type. The cut point emits {}.
	ShapeRecursive ShapeKind = "recursive"
)

// TypeShape is the syntactic container structure of a field's declared type,
// decoded once from the AST at extraction. Purely syntactic — it carries no
// registry knowledge. The registry outcome lives in FieldInfo.Resolution,
// which reuses this vocabulary plus the four Resolution-only kinds (ShapeRef,
// ShapeKindOnly, ShapeMarshaler, ShapeRecursive).
// The zero value (Kind "") is treated everywhere as ShapeUnknown.
type TypeShape struct {
	Kind ShapeKind
	// Name is set for ShapeNamed and ShapePrimitive: the identifier as written,
	// qualified for selector types ("time.Time"), including "interface{}" and "any".
	Name string
	// Key is the map key shape (ShapeMap only).
	Key *TypeShape
	// Elem is the pointed-to / element / map-value shape (ShapePointer, ShapeSlice,
	// ShapeArray, ShapeMap).
	Elem *TypeShape
}

// FieldInfo represents a struct field with validation metadata
type FieldInfo struct {
	Name string
	// Shape is the field's decoded type structure. Stamped by the analyzer at
	// extraction; hand-built test Projects must stamp it too — the generator
	// has NO string-parsing fallback.
	Shape         TypeShape
	JSONName      string            // Parsed from `json:"name"` tag
	ParamType     string            // "path", "query", "header", or "" for body fields
	ParamName     string            // Parsed from `param:"name"`, `query:"name"`, or `header:"name"` tags
	Required      bool              // Parsed from `validate:"required"` tag
	Description   string            // Parsed from `doc:"..."` tag
	Example       string            // Parsed from `example:"..."` tag
	RawValidation string            // Raw validation tag string (e.g., "required,email,min=5")
	Constraints   map[string]string // Parsed validation constraints for OpenAPI mapping
	// ElementConstraints holds validate rules that appear AFTER a `dive` token, so
	// they apply to each ELEMENT of a slice/array (e.g. `min=1,dive,email` puts min=1
	// on the array and email on each element). Nil when the field has no `dive`.
	ElementConstraints map[string]string
	// Resolution is the field's type with every local named non-struct type
	// substituted by what it stands for, at every depth, and every struct leaf
	// marked ShapeRef (CONTEXT.md, "Resolution"). Stamped by the analyzer for
	// every field that reaches the document (never for a json:"-" body field).
	// Read it only through ResolvedShape.
	Resolution *TypeShape
}

// ResolvedShape returns the field's Resolution, or its syntactic Shape when
// the field has none (a hand-built FieldInfo, or a json:"-" field).
func (f *FieldInfo) ResolvedShape() TypeShape {
	if f.Resolution != nil {
		return *f.Resolution
	}
	return f.Shape
}
