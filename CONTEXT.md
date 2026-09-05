# go-bricks-openapi

Static analysis of a go-bricks project's source (AST only, never compiled or executed) to emit an OpenAPI specification.

## Language

**Shape**:
The syntactic container structure of a field's declared type — pointer, slice, map, named, primitive, or unknown — decoded once from the AST at extraction. Purely syntactic; carries no registry knowledge.
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

**Pre-flight check**:
A `doctor`-only check on the environment around the project — Go version, directory layout, `go.mod` presence — made before any Survey and able to stop it. Not part of the Survey.
_Avoid_: diagnostic (reserved for what a Survey raises), health check

## Conventions

**Context-first**:
Exported APIs accept a `context.Context` as the first parameter even when unused. Comment-only convention, applied inconsistently today — recorded as observed, not reconciled.
