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

## Conventions

**Context-first**:
Exported APIs accept a `context.Context` as the first parameter even when unused. Comment-only convention, applied inconsistently today — recorded as observed, not reconciled.
