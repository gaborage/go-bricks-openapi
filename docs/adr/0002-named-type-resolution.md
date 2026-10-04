---
status: proposed
date: 2026-09-28
---

Status flips to accepted when the resolver PR (#109) lands.

# Named types resolve into a separate Resolution; Marshaler types never resolve through their underlying type

A named non-struct type (`type Tags []string`, `type Cents int64`, `b.Cents`) will be documented as exactly what the type it stands for would be, in every position — field, slice item, map value, nested container, payload — so `Tags` emits what `[]string` emits and the Go name does not survive into the spec. That outcome will be carried as a Resolution alongside the field's or payload's Shape, never written into the Shape itself, and every consumer will read the resolved form when one exists. A Marshaler type (see `CONTEXT.md`) is the exception: `encoding/json` consults its methods before its underlying type, so it will never be resolved through that type. It will be documented as a string only when `encoding/json` selects text encoding in both directions on every toolchain — `MarshalText` to encode (alone or with `AppendText`), `UnmarshalText` to decode — because the JSON-specific methods (`MarshalJSON`, `MarshalJSONTo`, `UnmarshalJSON`, `UnmarshalJSONFrom`) take precedence over the text ones; every other Marshaler type will be documented as `{}` with a warning. Decided while triaging #88, #97, #100, #109, #110 and #111.

## Considered options

- Rewrite Shape in place at extraction. It needs no consumer edits, but Shape is defined as purely syntactic, and a rewritten Shape cannot express #92's kind-only case, where build-tagged declarations disagree on a named scalar's width and only its kind survives.
- Give each named slice or map type its own component and `$ref` it. It keeps the Go name and handles recursive types naturally, but adds components that the no-unused-components rule and the payload `$ref` rules must track. Inlining was chosen; a type that recurs into itself will be cut off at the recursion point as `{}` with a warning.
- Resolve a Marshaler type through its underlying type until its methods are modelled. That turns today's vague or warned fallback into a confidently wrong schema — an enum with `MarshalText` documented as `integer` while the wire carries `"active"`. `{}` accepts the real payload in both directions; `type: object` would reject it.

## Consequences

- Every remaining fallback on a field will warn, as payload fallbacks already do, so `--strict` will newly fail for fields of unresolvable out-of-module types (`decimal.Decimal`) and of Marshaler types. There is no per-field override yet.
- A pointer to a named slice or map (`*Tags`) will be documented exactly as the pointer to its underlying type (`*[]string`): no `nullable`.
- A type whose marshal methods are all on its pointer is a Marshaler type in every position. `encoding/json` ignores such a method on a non-addressable value — a map value, or a payload returned by value — and writes the underlying type there. But one component serves every position and both `Result[T]` and `Result[*T]` routes, so modelling each position would make the shared schema wrong for one of them; `{}` accepts both forms.
- `MarshalJSONTo`, `AppendText` and `UnmarshalJSONFrom` are consulted only by the jsonv2-backed `encoding/json`: opt-in through `GOEXPERIMENT=jsonv2` since Go 1.25, and the default build in Go 1.27.1, where this was verified. The classic implementation writes a type that has only those methods through its underlying type. The tool cannot see the target project's toolchain, so such a type is a Marshaler type either way; `{}` accepts both forms. For the same reason, a type that encodes as text only through `AppendText` is `{}`, not a string.
