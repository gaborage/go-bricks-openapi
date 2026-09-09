# go-bricks-openapi

Static analysis of a go-bricks project's source (AST only, never compiled or executed) to emit an OpenAPI specification.

## Language

**Shape**:
The syntactic container structure of a declared type — pointer, slice, map, named, primitive, or unknown — decoded once from the AST at extraction. Carried by every struct field, and by a response payload that is a container (`server.Result[[]Item]`). Purely syntactic; carries no registry knowledge.
_Avoid_: type string, kind (overloaded with OpenAPI's `type` keyword)

**Resolution**:
The registry outcome for a field's base type: which named schema it references and what scalar kind underlies it. A distinct, later phase than Shape — resolving requires the type registry; Shape does not.
_Avoid_: lookup, ref info

**Constraint set**:
The typed image of one `validate` tag expressed in OpenAPI schema vocabulary (bounds, format, pattern, enum). One tag produces one constraint set.
_Avoid_: constraint list, constraint pairs

**Survey**:
One reading of a project's source that every command consumes: the discovered modules and routes, their counts and typed/untyped classification, every diagnostic the reading raised, the go-bricks dependency status, and whether any of that warned. A Survey is data — it prints nothing and writes nothing.
_Avoid_: analysis run, pipeline, report, audit

**Unresolved route**:
A route whose registration the Survey found but whose path expression it could not reduce to a string — a function-local or concatenated constant, a qualified constant from another package, a call result. Distinct from an Untyped route, which has a path but no typed handler. An Unresolved route is a diagnostic and is counted on its own, never folded into the typed/untyped ratio.
_Avoid_: dropped route, skipped route, missing route

**Directive**:
An `//openapi:<name>` comment attached to a route registration that overrides or supplements what the Survey infers for that route — e.g. marking it public, naming its error statuses. One parser, one namespace; every directive shares the same placement rule.
_Avoid_: annotation, pragma, marker comment

**Pre-flight check**:
A `doctor`-only check on the environment around the project — Go version, directory layout, `go.mod` presence — made before any Survey and able to stop it. Not part of the Survey.
_Avoid_: diagnostic (reserved for what a Survey raises), health check

## Conventions

**Context-first**:
Exported APIs accept a `context.Context` as the first parameter even when unused. Comment-only convention, applied inconsistently today — recorded as observed, not reconciled.
