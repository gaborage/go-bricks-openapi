---
status: accepted
date: 2026-09-28
---

# Named types resolve into a separate Resolution; Marshaler types never resolve through their underlying type

A named non-struct type (`type Tags []string`, `type Cents int64`, `b.Cents`) is documented as exactly what the type it stands for would be, in every position — field, slice item, map value, nested container, payload — so `Tags` emits what `[]string` emits and the Go name does not survive into the spec. That outcome is carried as a Resolution alongside the field's or payload's Shape, never written into the Shape itself, and every consumer reads the resolved form when one exists. A Marshaler type (see `CONTEXT.md`) is the exception: `encoding/json` consults its methods before its underlying type, so it is never resolved through that type and is documented as `{}` with a warning unless a narrower rule (a text marshaler in both directions is a string) applies. Decided while triaging #88, #97, #100, #109, #110 and #111.

## Considered options

- Rewrite Shape in place at extraction. It needs no consumer edits, but Shape is defined as purely syntactic, and a rewritten Shape cannot express #92's kind-only case, where build-tagged declarations disagree on a named scalar's width and only its kind survives.
- Give each named slice or map type its own component and `$ref` it. It keeps the Go name and handles recursive types naturally, but adds components that the no-unused-components rule and the payload `$ref` rules must track. Inlining was chosen; a type that recurs into itself is cut off at the recursion point as `{}` with a warning.
- Resolve a Marshaler type through its underlying type until its methods are modelled. That turns today's vague or warned fallback into a confidently wrong schema — an enum with `MarshalText` documented as `integer` while the wire carries `"active"`. `{}` accepts the real payload in both directions; `type: object` would reject it.

## Consequences

- Every remaining fallback on a field now warns, as payload fallbacks already did, so `--strict` newly fails for fields of unresolvable out-of-module types (`decimal.Decimal`) and of Marshaler types. There is no per-field override yet.
- A pointer to a named slice or map (`*Tags`) is documented exactly as the pointer to its underlying type (`*[]string`): no `nullable`.
