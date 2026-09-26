# go-bricks-openapi

Static analysis of a go-bricks project's source (AST only, never compiled or executed) to emit an OpenAPI specification.

## Language

**Shape**:
The syntactic container structure of a declared type — pointer, slice, map, named, primitive, or unknown — decoded once from the AST at extraction. Carried by every struct field, and by every response payload a result wrapper carries (`server.Result[[]Item]`, `server.Result[time.Time]`), so a well-known or builtin payload is typed from it rather than `$ref`'d — except a non-slice payload that registers as a project struct, which sheds its Shape so it is `$ref`'d even when its short name collides with a well-known type (`uuid.UUID`). Purely syntactic; carries no registry knowledge.
_Avoid_: type string, kind (overloaded with OpenAPI's `type` keyword)

**Resolution**:
The registry outcome for a field's base type: which named schema it references, or which builtin scalar underlies it (and so its kind). A distinct, later phase than Shape — resolving requires the type registry; Shape does not.
_Avoid_: lookup, ref info

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
An `//openapi:<name>` comment attached to a route registration that overrides or supplements what the Survey infers for that route — e.g. marking it public, naming its error statuses. One parser, one namespace; every directive shares the same placement rule.
_Avoid_: annotation, pragma, marker comment

**Detached directive**:
A comment group holding at least one recognised Directive that attaches to no registration the Survey recognised — separated from its call by a blank line, above an enclosing statement, inside a call, after code on the same line (`server.GET(...) //openapi:public`), or above a registration the Survey never walks. Only files the Survey's module discovery visits are checked; a file read just to resolve an import (e.g. in a nested Go module) is not. It is a diagnostic, raised once per group. A group attaches as soon as the call below it is recognised as a registration, so a Directive above an Unresolved route is never Detached; a group of only unknown names is reported as unknown, never Detached.
_Avoid_: orphan directive, misplaced directive, unattached comment

**Pre-flight check**:
A `doctor`-only check on the environment around the project — Go version, directory layout, `go.mod` presence — made before any Survey and able to stop it. Not part of the Survey.
_Avoid_: diagnostic (reserved for what a Survey raises), health check

## Conventions

**Context-first**:
Exported APIs accept a `context.Context` as the first parameter even when unused. Comment-only convention, applied inconsistently today — recorded as observed, not reconciled.
