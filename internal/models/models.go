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
	// wraps that plaintext after decrypt-and-verify). It is also set for a struct
	// Marshaler payload whose struct carries a jose: tag, although that struct is
	// not registered (#111): the route stays application/jose, and its plaintext
	// goes undocumented.
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
	// For any other payload Shape is the argument as written (a pointer kept).
	// A payload that does not register as a project struct is resolved by the
	// analyzer (Resolution) and typed from it exactly as a struct field of its
	// type, never $ref'd itself; a builtin carries no Name, a well-known type
	// (time.Time, uuid.UUID, json.Number, ...: WellKnownTypeNames) keeps its
	// Name, and any other name is cleared. Only a root that resolves to nothing
	// (an unresolvable or undeclared name, a defined type over a well-known
	// struct, disagreeing build-tagged declarations, or an unmodelled leaf such
	// as func()) keeps the name-cleared, warned, untyped-object fallback. A
	// nameless composite over a named func or chan type (map[string]Fn) gets no
	// Resolution and is typed from its Shape, a container of untyped objects,
	// with no per-type warning, exactly as a field of its type.
	Shape *TypeShape
	// Resolution is the payload's Shape with every named non-struct type of the
	// module substituted by what it stands for and every struct leaf a ShapeRef,
	// except a struct Marshaler type, which is a ShapeText or ShapeMarshaler leaf
	// (CONTEXT.md, "Resolution"). Stamped by the analyzer on a payload that does
	// not register as a struct; nil for a request, except one whose type is a
	// struct Marshaler type (#111) with a body (not params-only, or JOSE): its
	// Resolution is its text or Marshaler leaf, its Name is cleared and its
	// Fields are only its parameters. A params-only (at least one field, every
	// one a parameter), non-JOSE one keeps its Name and parameter Fields and has
	// no Resolution; a zero-field one is not params-only. Also nil for a
	// registered struct payload,
	// the NoContentResult marker, a payload whose root resolves to nothing, and
	// a nameless composite whose resolution has an unmodelled leaf
	// (map[string]Fn), which is typed from its Shape like the field of its type.
	// Read it through ResolvedShape.
	Resolution *TypeShape
}

// ResolvedShape returns the payload's Resolution, else its Shape (nil when it
// has neither).
func (t *TypeInfo) ResolvedShape() *TypeShape {
	if t.Resolution != nil {
		return t.Resolution
	}
	return t.Shape
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
	// is the type as named at that position (an alias, or a struct promoting
	// the method); its warning names the declaring type or the embed. Elem is
	// the quietly resolved underlying form of a non-struct Marshaler type, kept
	// for diagnostics and tests only (the byte-slice decision reads it before
	// the leaf is built); a struct Marshaler leaf has no Elem. Nothing emits,
	// registers or otherwise reads what is under Elem.
	ShapeMarshaler ShapeKind = "marshaler"
	// ShapeRecursive is a named type met again inside its own resolution, or
	// past maxNamedResolutionDepth. Name is that type. The cut point emits {}.
	ShapeRecursive ShapeKind = "recursive"
	// ShapeText is a Marshaler type encoding/json writes and reads as a JSON
	// string on every toolchain and in every position (#111): MarshalText in
	// the value method set with no MarshalJSON or MarshalJSONTo, and
	// UnmarshalText with no UnmarshalJSON or UnmarshalJSONFrom, declared or
	// promoted. Name is the type. It emits
	// {type: string} and takes no validate keyword.
	ShapeText ShapeKind = "text"
)

// TypeShape is the syntactic container structure of a field's declared type,
// decoded once from the AST at extraction. Purely syntactic — it carries no
// registry knowledge. The registry outcome lives in FieldInfo.Resolution and
// TypeInfo.Resolution, which reuse this vocabulary plus the five
// Resolution-only kinds (ShapeRef, ShapeKindOnly, ShapeMarshaler,
// ShapeRecursive, ShapeText).
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
