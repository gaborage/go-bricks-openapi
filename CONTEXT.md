# go-bricks-openapi

Static analysis of a go-bricks project's source (AST only, never compiled or executed) to emit an OpenAPI specification.

## Language

**Shape**:
The syntactic container structure of a declared type — pointer, slice, array, map, named, primitive, instantiation, anonymous struct, or unknown — decoded once from the AST at extraction. An anonymous struct (`struct{ X int }`) is decoded with its literal and nothing else; its fields belong to its Resolution. A fixed-size array (`[N]T`, its length not recorded) is a container kind distinct from a slice, because `encoding/json` base64-encodes a `[]byte` but writes a `[N]byte` as numbers; every rule other than that base64 one treats the two alike. Carried by every struct field, and by every response payload (a result wrapper's type argument, as in `server.Result[[]Item]` or `server.Result[time.Time]`, or a handler's bare first result of the same type), so a payload that does not register as a project struct is resolved (see **Resolution**) and typed like a field of its type rather than `$ref`'d — except a non-slice payload that registers as a project struct, which sheds its Shape so it is `$ref`'d even when its short name collides with a well-known type (`uuid.UUID`). Purely syntactic; carries no registry knowledge.
_Avoid_: type string, kind (overloaded with OpenAPI's `type` keyword)

**Resolution**:
The registry outcome for a declared type — of a field or of a payload — at any depth: which named schema a struct type references (a struct Marshaler type references none: it resolves to a string or an untyped leaf), or what a named non-struct type stands for (its underlying builtin scalar, and so its kind, or its underlying slice, map, array or pointer type). A distinct, later phase than Shape — resolving requires the type registry; Shape does not. A Marshaler type never resolves through its underlying type. An anonymous struct names no schema: in a field's type, at any depth, its Resolution carries its fields, each resolved as a named struct's field is, and it is documented inline (one that embeds a Marshaler type is itself one), except in the type of a path, query or header parameter, or of a struct Marshaler request's body field, where it stays unextracted (a bare object).
_Avoid_: lookup, ref info

**Instantiation**:
A generic type written with its type arguments (`Page[Address]`, `pg.List[string]`), whose Resolution substitutes those arguments into the generic declaration. Its Shape keeps the generic type and the arguments' Shapes as written, and never reads as the bare generic type. Until that substitution lands (#114), an Instantiation is not resolved: it is documented as an untyped object, wherever it is written, and raises a warning.
_Avoid_: generic type (that names the declaration), specialization

**Marshaler type**:
A type whose JSON form, in either direction, comes from its own `MarshalJSON`, `MarshalJSONTo`, `MarshalText`, `AppendText`, `UnmarshalJSON`, `UnmarshalJSONFrom` or `UnmarshalText` method — with the exact signature its `encoding/json` or `encoding` interface requires, on the type or its pointer, on a type it aliases, or promoted from a struct it embeds — rather than from its underlying type. A defined type over a Marshaler type drops that type's declared methods, but keeps whatever its underlying struct's embedded fields promote (`type MDW WrapLevel` is one). A struct field named like one of the methods hides a promoted method of that name, as Go's selector rules say. Only JSON bodies are affected: parameters are bound by kind.
_Avoid_: custom type, enum type, custom-marshaled type

**Stated schema**:
The wire schema a `schema` type Directive states for a Marshaler type the project declares, used in place of the `{}` that type would otherwise be documented as (or, for a type that encodes as text both ways, in place of its bare string).
_Avoid_: override, custom schema, manual schema

**Constraint set**:
The typed image of one `validate` tag expressed in OpenAPI schema vocabulary (bounds, format, pattern, enum). One tag produces one constraint set.
_Avoid_: constraint list, constraint pairs

**Survey**:
One reading of a project's source that every command consumes: the discovered modules and routes, their counts and typed/untyped classification, every diagnostic the reading raised, the go-bricks dependency status, and whether any of that warned. A Survey is data — it prints nothing and writes nothing.
_Avoid_: analysis run, pipeline, report, audit

**Unresolved route**:
A route whose registration the Survey found but whose path expression it could not reduce to a string — a `var` or `:=` local, a non-string local binding shadowing a constant of the same name, a qualified constant from another package, a call result, a group whose own prefix is unresolvable, a group registrar reassigned inside a branch of an `if`, `switch` or `select`, a loop or a closure, or after a closure that reads it, whose prefix therefore depends on control flow (a bare `{ }` block always runs, so a reassignment in it counts as one in the block enclosing it), or a group registrar holding a value the Survey cannot trace to a `Group(...)` call. Distinct from an Untyped route, which has a path but no typed handler. An Unresolved route is a diagnostic and is counted on its own, never folded into the typed/untyped ratio.
_Avoid_: dropped route, skipped route, missing route

**Directive**:
An `//openapi:<name>` comment that overrides or supplements what the Survey infers. One parser, one namespace; each directive name has exactly one placement. A route directive (`public`, `errors`, `meta`) is attached to a route registration and applies to that route — marking it public, naming its error statuses, naming the type of the meta keys its handler supplies. A type directive (`schema`) sits in a package-level type declaration's doc comment and applies to that type — stating its wire schema.
_Avoid_: annotation, pragma, marker comment

**Detached directive**:
A recognised Directive outside its name's placement: a route directive that attaches to no registration the Survey recognised (one that follows code on the same line never attaches), or a type directive anywhere but a package-level type declaration's doc comment (so one above a type declared in a function body). Placement is judged per directive name, so one comment group can hold a Detached directive beside one that applies. It is a diagnostic, raised once per group, and only in files the Survey's module discovery visits. A route directive above an Unresolved route is never Detached, an unknown name is reported as unknown rather than Detached, and a type directive in a package-level type declaration's doc comment that cannot apply there gets its own diagnostic instead.
_Avoid_: orphan directive, misplaced directive, unattached comment

**Pre-flight check**:
A `doctor`-only check on the environment around the project — Go version, directory layout, `go.mod` presence — made before any Survey and able to stop it. Not part of the Survey.
_Avoid_: diagnostic (reserved for what a Survey raises), health check

## Conventions

**Context-first**:
Exported APIs accept a `context.Context` as the first parameter even when unused. Comment-only convention, applied inconsistently today — recorded as observed, not reconciled.
