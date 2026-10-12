# Changelog

## [0.6.0](https://github.com/gaborage/go-bricks-openapi/compare/v0.5.0...v0.6.0) (2026-10-12)


### ⚠ BREAKING CHANGES

* **analyzer:** a struct embedding an unexported non-struct type under an explicit json name loses that property from the generated spec, and so does any path, query or header parameter the embed's tag declared. Regenerate clients from the new spec.
* **analyzer:** existing operations can change. A delegate route that borrowed a same-named module constant moves to its own path. A newly emitted route can register a struct that takes a component's bare name, so a same-named struct of another package becomes <Pkg><Name> (Widget becomes WidgetsWidget) and existing $refs move, and its handler can take an operationId an existing operation held, which then gains a numeric suffix (widgetsGet becomes widgetsGet2). Regenerate clients.

### Added

* **commands:** add --fail-on-warnings to write the spec, then fail ([#186](https://github.com/gaborage/go-bricks-openapi/issues/186)) ([53c2f1a](https://github.com/gaborage/go-bricks-openapi/commit/53c2f1a96495d292bdbe9a0ad58edc79160f876d)), closes [#65](https://github.com/gaborage/go-bricks-openapi/issues/65)


### Fixed

* **analyzer:** check the import path before using well-known types ([#190](https://github.com/gaborage/go-bricks-openapi/issues/190)) ([818a240](https://github.com/gaborage/go-bricks-openapi/commit/818a2404bb2e22f5cf7d52ee549db082a8de4404)), closes [#164](https://github.com/gaborage/go-bricks-openapi/issues/164)
* **analyzer:** resolve a delegate's own package constants in route paths ([#193](https://github.com/gaborage/go-bricks-openapi/issues/193)) ([64557eb](https://github.com/gaborage/go-bricks-openapi/commit/64557eb2b4bc63049856aad493faf9e9b6d20ac6)), closes [#184](https://github.com/gaborage/go-bricks-openapi/issues/184)
* **analyzer:** skip unexported non-struct embeds that carry a json name ([#194](https://github.com/gaborage/go-bricks-openapi/issues/194)) ([4d175bb](https://github.com/gaborage/go-bricks-openapi/commit/4d175bb4722e06f926d206411f5fd577ef76c7e3)), closes [#185](https://github.com/gaborage/go-bricks-openapi/issues/185)
* **analyzer:** warn on params go-bricks cannot bind ([#195](https://github.com/gaborage/go-bricks-openapi/issues/195)) ([843b19a](https://github.com/gaborage/go-bricks-openapi/commit/843b19aafc1dffd252010fa6d08614cde14d24bf)), closes [#121](https://github.com/gaborage/go-bricks-openapi/issues/121)

## [0.5.0](https://github.com/gaborage/go-bricks-openapi/compare/v0.4.2...v0.5.0) (2026-10-12)


### ⚠ BREAKING CHANGES

* **generator:** every exported field with no JSON name (no json tag, or one like json:",omitempty") changes its property key from its lower-first form to its Go name (uRL -> URL, iD -> ID, createdBy -> CreatedBy), so required lists and generated client field names move. Regenerate clients from the new spec.
* **analyzer:** a response payload of an anonymous struct type (server.Result[struct{…}] or ResultWithMeta, behind any pointer, as a slice or array element, as a map value, the same as a bare return, or a named non-struct type over one) is now documented inline, as a field of its type is, every pointer shed, so never nullable, instead of data: {type: object, description: Response data}. One that embeds a Marshaler type follows the Marshaler rules: a text one is a string, any other is {} with the promoted-Marshaler payload warning. doctor counts such a route as typed unless it embeds a non-text Marshaler. A payload whose root, every written pointer shed, is a jose-tagged anonymous struct is not documented: it is left as before (application/json, the untyped fallback), tracked in issue 171. A request-less handler returning an anonymous struct payload, other than one jose-tagged at its root, is now found, so its constructor-derived success status (201, 202, …) and inferred error responses are documented where 200 and none were. A named struct reachable only through such a payload is registered, so components appear and component names can move as for fields: a same-named struct of another package can become <Pkg><Name>, moving $refs and generated client type names; regenerate clients. That struct's fields raise their field warnings. The anonymous struct expansion limit warning now names the outermost anonymous struct's position. A payload's anonymous structs now share the 1000-inline-object limit and its warning, and a payload holding a named type over an anonymous struct whose build-tagged declarations disagree now raises the disagreeing-declarations warning instead of the named non-struct one. A request parameter of anonymous struct type (struct{…}, *struct{…}) now raises a warning, still emitting no requestBody, unless it has no field (struct{}, go-bricks' no-input request type, stays silent). Either way its handler is now found, so where its payload resolves to nothing its constructor-derived success status and inferred error responses are documented where 200 and none were. So generate --strict fails where it passed. It passes where it failed only on the untyped-route warning of an anonymous struct payload, other than one jose-tagged at its root, on a route with no request or with a field-less anonymous struct request (struct{} or *struct{}), or on the named non-struct or no-schema-component warning of a named type over an anonymous struct (type Rows []struct{…}, local or from another package) whose build-tagged declarations, if any, agree.
* **analyzer:** a field whose type is or holds an anonymous struct (struct{…} directly, behind a pointer, as a slice or array element, as a map value, nested in another, or reached through a named slice, map or pointer type) is now an inline object with its properties, required list and its fields' validate keywords instead of a bare object. *struct{…} keeps nullable. Such a field sheds any validate keyword of its own that the bare object carried, such as enum or format. A named struct reachable only through an anonymous struct is now registered, so new components appear and its own fields raise their warnings. A struct newly reached this way can take the bare name, so a same-named struct of another package becomes <Pkg><Name>, moving $refs and generated client type names; regenerate clients. An anonymous struct that embeds a Marshaler type is now {} with the promoted-Marshaler warning, or a string, silently, when the embed promotes MarshalText and UnmarshalText alone. One that contains itself, alone or with other anonymous structs, is cut to {} where it recurs, with a recursion warning. One past the type registration depth limit is cut to {} with the depth warning. A field whose anonymous structs expand past 1000 inline objects is truncated with a warning. In a body field, build-tagged declarations of a named type over an anonymous struct (type Rows []struct{…}) that are not written alike (names, types or raw tags, so even a differing ,omitempty) or that reach different structs are now an untyped object with the disagreeing-declarations warning, so a field of such a type loses its container shape (a slice its array type, a map its additionalProperties, a pointer its nullable). The anonymous struct's own fields raise the warnings a named struct's fields raise (unresolvable type, Marshaler type, uintptr, generic instantiation, malformed tag, dropped embed). So generate --strict fails where it passed, and doctor reports caveats where it reported all checks passed.
* **analyzer:** generate --strict now fails on projects with generic fields, parameters, requests or payloads, which raise new unresolved-instantiation warnings; a request of a named type over a struct instantiation gets the generic warning instead of F26. A handler with a request parameter written as a generic instantiation, whatever its payload (an anonymous struct included), or a request-less handler whose payload holds a generic instantiation (other than a pointer to a result wrapper), is now recognised as the route's handler, so a route whose handler was not recognised before now documents its real success status and inferred error responses instead of the default 200.
* **analyzer:** embedded fields now follow encoding/json. Structs that embed an exported non-struct type, or an alias or defined type over a struct, gain properties; body fields are deduplicated by the key and dominance rule encoding/json applies, so a field it hides is no longer documented; a struct embedding json.Number becomes {} with a warning; component names can move between <Name> and <Pkg><Name>; tagged non-struct and well-known Marshaler embeds become parameters, and a repeated parameter is listed once. Embeds of out-of-module, unreadable or generic types, and non-struct embeds whose type falls back, now warn, so generate --strict can newly fail. "Who sees a change" above lists every case.
* **analyzer:** Result[**T], ResultWithMeta[**T] and bare **T payload data schemas move from object to what *T emits, and specs gain components for structs reached only through a **T payload. Such a struct can now take a short name first, so a same-named struct of another package registered later is renamed (User becomes <Pkg>User), moving $refs and generated client type names; regenerate clients. Request-less routes with a **T payload gain their 201/202/NewResult success code and inferred error responses.
* **analyzer:** build-tagged copies of a type that bind one qualifier to two packages, where the qualified struct is a slice item, map value, pointer target or nested container element (written directly or through a local alias or defined type of it), are documented as an untyped object with the "build-tagged declarations disagree on shape" warning, instead of a $ref to one variant's struct picked by file order, so generate --strict can newly fail on such projects.
* **analyzer:** in an imported in-module directory whose every file the go command can build carries a build constraint, where a file of the package's own clause that it never builds (a leading _ or . in its name, or //go:build ignore) sorts before a file of another clause under another constraint (//go:build tools, or a GOOS/GOARCH-suffixed name such as a_windows.go), an unaliased import of the package now falls back: fields and named types are untyped objects with a warning, embedded structs lose their promoted fields silently, requests get no requestBody, and generate --strict fails. An unaliased import of a real package of that other clause, placed earlier in the same file, falls back the same way, because the misnamed import replaces its entry. Behind a file whose name starts with _ or ., named non-struct types under an aliased import fall back too, or, when the other clause's file declares the same name, silently take its declaration, so generate --strict passes on a wrong schema. Generated component names can change: a struct that newly resolves through an import this fix repairs (such as one behind a sorted-first generator or tool) can take the bare name, so a same-named struct of another package becomes `<Pkg><Name>` (Addr becomes ModAddr); $refs and generated client type names move; regenerate clients.

### Fixed

* **analyzer:** document anonymous struct fields inline ([#181](https://github.com/gaborage/go-bricks-openapi/issues/181)) ([859c4d2](https://github.com/gaborage/go-bricks-openapi/commit/859c4d22971080919f431b0578df8dce15fe07f4))
* **analyzer:** document anonymous struct payloads inline ([#182](https://github.com/gaborage/go-bricks-openapi/issues/182)) ([5f62fec](https://github.com/gaborage/go-bricks-openapi/commit/5f62fec6aafb584f633d3520f27aeb6e7e3c4a8a)), closes [#118](https://github.com/gaborage/go-bricks-openapi/issues/118)
* **analyzer:** document json ,string scalars as strings ([#153](https://github.com/gaborage/go-bricks-openapi/issues/153)) ([4a8bbc1](https://github.com/gaborage/go-bricks-openapi/commit/4a8bbc1f2dea1632ffc355025cc77c978ed44d30)), closes [#115](https://github.com/gaborage/go-bricks-openapi/issues/115)
* **analyzer:** resolve imported packages from their importable files ([#140](https://github.com/gaborage/go-bricks-openapi/issues/140)) ([09293e7](https://github.com/gaborage/go-bricks-openapi/commit/09293e766527e5532ef999e823e7fbddc1601d61)), closes [#122](https://github.com/gaborage/go-bricks-openapi/issues/122)
* **analyzer:** resolve qualifiers in the file that declares them ([#162](https://github.com/gaborage/go-bricks-openapi/issues/162)) ([6f472a5](https://github.com/gaborage/go-bricks-openapi/commit/6f472a58983d71246930e4ce8809f61d29d96be4)), closes [#123](https://github.com/gaborage/go-bricks-openapi/issues/123)
* **analyzer:** resolve Result[**T] payloads exactly like Result[*T] ([#168](https://github.com/gaborage/go-bricks-openapi/issues/168)) ([c73a0e1](https://github.com/gaborage/go-bricks-openapi/commit/c73a0e15cd8ce8fbff6400a0bbda4c196d8f570a)), closes [#120](https://github.com/gaborage/go-bricks-openapi/issues/120)
* **analyzer:** stop dropping untagged embeds silently ([#169](https://github.com/gaborage/go-bricks-openapi/issues/169)) ([1de3384](https://github.com/gaborage/go-bricks-openapi/commit/1de3384fa1e9c495d2b062887113f91e55f4da2b)), closes [#116](https://github.com/gaborage/go-bricks-openapi/issues/116)
* **analyzer:** warn on request types behind two or more pointers ([#170](https://github.com/gaborage/go-bricks-openapi/issues/170)) ([32a4177](https://github.com/gaborage/go-bricks-openapi/commit/32a41774b5bc2f76bc1403d799c6ecaf7fb172dd)), closes [#132](https://github.com/gaborage/go-bricks-openapi/issues/132)
* **analyzer:** warn on unresolved generic type instantiations ([#174](https://github.com/gaborage/go-bricks-openapi/issues/174)) ([caf4475](https://github.com/gaborage/go-bricks-openapi/commit/caf4475271fd6ccf21171135fa32dd0242426e10)), closes [#138](https://github.com/gaborage/go-bricks-openapi/issues/138)
* **commands:** raise doctor's Go floor to 1.26 to match go.mod ([#172](https://github.com/gaborage/go-bricks-openapi/issues/172)) ([070edb8](https://github.com/gaborage/go-bricks-openapi/commit/070edb858fe3ff3798b715b2ccf2cd63bb2f0a44)), closes [#151](https://github.com/gaborage/go-bricks-openapi/issues/151)
* **generator:** drop format for non-RFC 3339 datetime layouts ([#161](https://github.com/gaborage/go-bricks-openapi/issues/161)) ([4516a15](https://github.com/gaborage/go-bricks-openapi/commit/4516a1550b12b52d3ca4c7c750c23be4212d1935)), closes [#103](https://github.com/gaborage/go-bricks-openapi/issues/103)
* **generator:** key untagged fields by their Go name ([#183](https://github.com/gaborage/go-bricks-openapi/issues/183)) ([8eb7460](https://github.com/gaborage/go-bricks-openapi/commit/8eb7460f8c12ba034e46c5905483a9a1c6046770)), closes [#166](https://github.com/gaborage/go-bricks-openapi/issues/166)
* **generator:** type os.FileMode, fs.FileMode, net.IP and netip types ([#173](https://github.com/gaborage/go-bricks-openapi/issues/173)) ([16e366b](https://github.com/gaborage/go-bricks-openapi/commit/16e366b52e88ff2180c2652f34c135b59368a129)), closes [#139](https://github.com/gaborage/go-bricks-openapi/issues/139)

## [0.4.2](https://github.com/gaborage/go-bricks-openapi/compare/v0.4.1...v0.4.2) (2026-10-09)


### ⚠ BREAKING CHANGES

* **analyzer:** generated component names can change. A route-package struct can lose its bare name to a same-named struct now registered through another package's named type (User becomes QualifiednamedtypesUser), and a second package sharing a clause, or a named type over a struct sharing a name with one in the struct's package, gets `<Pkg><Name>` (ModelItem, BUser). $refs and generated client type names move; regenerate clients.
* **analyzer:** payload data schemas move from object to typed schemas, and specs gain components for structs reached only through a payload. A struct registered through a payload can now take a short name first, so a same-named struct of another package registered later is renamed (User becomes `<Pkg>User`), moving $refs and generated client type names; regenerate clients. Request-less routes with a map, nested-slice or pointer-to-container payload gain their 201/202/NewResult success code and inferred error responses.
* **analyzer:** struct Marshaler types and structs that promote a Marshaler method lose their components; their $refs become {} or {type: string}, which moves generated client types, and routes whose payload or request is a non-text Marshaler type newly count as untyped and fail generate --strict, a zero-field request included. Such a type no longer holds its short component name, so a same-named struct of another package that was qualified is renamed to it (BMoney becomes Money, and Money now holds that struct), moving $refs and generated client type names; regenerate clients.

### Added

* **analyzer:** add openapi:errors directive for declared errors ([#85](https://github.com/gaborage/go-bricks-openapi/issues/85)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **analyzer:** infer error responses from handler constructor calls ([#87](https://github.com/gaborage/go-bricks-openapi/issues/87)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **analyzer:** warn on directives detached from a registration ([#91](https://github.com/gaborage/go-bricks-openapi/issues/91)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **commands:** report unresolved routes in generate and doctor output ([#86](https://github.com/gaborage/go-bricks-openapi/issues/86)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **generate:** print warning count in the run summary ([#73](https://github.com/gaborage/go-bricks-openapi/issues/73)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))


### Fixed

* **analyzer:** document [N]byte as an integer array, not base64 ([#126](https://github.com/gaborage/go-bricks-openapi/issues/126)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **analyzer:** document bare handler returns like Result[T] ([#124](https://github.com/gaborage/go-bricks-openapi/issues/124)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **analyzer:** resolve group registrar prefixes positionally ([#94](https://github.com/gaborage/go-bricks-openapi/issues/94)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **analyzer:** resolve local named types in every field position ([#128](https://github.com/gaborage/go-bricks-openapi/issues/128)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **analyzer:** resolve named types from other in-module packages ([#129](https://github.com/gaborage/go-bricks-openapi/issues/129)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **analyzer:** resolve route paths from scoped consts and concatenation ([#82](https://github.com/gaborage/go-bricks-openapi/issues/82)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **analyzer:** resolve success status through local result bindings ([#74](https://github.com/gaborage/go-bricks-openapi/issues/74)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **analyzer:** type named and composite response payloads like fields ([#130](https://github.com/gaborage/go-bricks-openapi/issues/130)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **analyzer:** type text Marshaler types as strings, never $ref one ([#131](https://github.com/gaborage/go-bricks-openapi/issues/131)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **analyzer:** warn on non-struct handler request types ([#127](https://github.com/gaborage/go-bricks-openapi/issues/127)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **generator:** drop date and date-time examples kin rejects ([#106](https://github.com/gaborage/go-bricks-openapi/issues/106)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **generator:** emit builtin format and floor for named scalars ([#92](https://github.com/gaborage/go-bricks-openapi/issues/92)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **generator:** emit format byte for []byte and drop non-base64 examples ([#90](https://github.com/gaborage/go-bricks-openapi/issues/90)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **generator:** emit untyped schema for json.RawMessage ([#93](https://github.com/gaborage/go-bricks-openapi/issues/93)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **generator:** map byte and rune named scalars to integer ([#80](https://github.com/gaborage/go-bricks-openapi/issues/80)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **generator:** render slice result payloads as typed arrays ([#76](https://github.com/gaborage/go-bricks-openapi/issues/76)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **generator:** stop emitting dangling $ref for non-struct payloads ([#108](https://github.com/gaborage/go-bricks-openapi/issues/108)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **generator:** type json.Number, time.Month and time.Weekday ([#125](https://github.com/gaborage/go-bricks-openapi/issues/125)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **generator:** widen int, uint and uint32 to format int64 ([#105](https://github.com/gaborage/go-bricks-openapi/issues/105)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **release:** build with Go 1.27.2 and lint with golangci-lint v2.14.0 ([#133](https://github.com/gaborage/go-bricks-openapi/issues/133)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))
* **release:** run govulncheck v1.8.0 so the gate works on Go 1.27 ([#135](https://github.com/gaborage/go-bricks-openapi/issues/135)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))


### Changed

* **analyzer:** drop unread TypeInfo.IsPointer ([#72](https://github.com/gaborage/go-bricks-openapi/issues/72)) ([bc9e442](https://github.com/gaborage/go-bricks-openapi/commit/bc9e4425f41ee39705e54114e69b35002420019b))

## [0.4.1](https://github.com/gaborage/go-bricks-openapi/compare/v0.4.0...v0.4.1) (2026-10-09)


### ⚠ BREAKING CHANGES

* **analyzer:** generated component names can change. A route-package struct can lose its bare name to a same-named struct now registered through another package's named type (User becomes QualifiednamedtypesUser), and a second package sharing a clause, or a named type over a struct sharing a name with one in the struct's package, gets `<Pkg><Name>` (ModelItem, BUser). $refs and generated client type names move; regenerate clients.
* **analyzer:** payload data schemas move from object to typed schemas, and specs gain components for structs reached only through a payload. A struct registered through a payload can now take a short name first, so a same-named struct of another package registered later is renamed (User becomes `<Pkg>User`), moving $refs and generated client type names; regenerate clients. Request-less routes with a map, nested-slice or pointer-to-container payload gain their 201/202/NewResult success code and inferred error responses.
* **analyzer:** struct Marshaler types and structs that promote a Marshaler method lose their components; their $refs become {} or {type: string}, which moves generated client types, and routes whose payload or request is a non-text Marshaler type newly count as untyped and fail generate --strict, a zero-field request included. Such a type no longer holds its short component name, so a same-named struct of another package that was qualified is renamed to it (BMoney becomes Money, and Money now holds that struct), moving $refs and generated client type names; regenerate clients.

### Added

* **analyzer:** add openapi:errors directive for declared errors ([#85](https://github.com/gaborage/go-bricks-openapi/issues/85)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **analyzer:** infer error responses from handler constructor calls ([#87](https://github.com/gaborage/go-bricks-openapi/issues/87)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **analyzer:** warn on directives detached from a registration ([#91](https://github.com/gaborage/go-bricks-openapi/issues/91)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **commands:** report unresolved routes in generate and doctor output ([#86](https://github.com/gaborage/go-bricks-openapi/issues/86)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **generate:** print warning count in the run summary ([#73](https://github.com/gaborage/go-bricks-openapi/issues/73)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))


### Fixed

* **analyzer:** document [N]byte as an integer array, not base64 ([#126](https://github.com/gaborage/go-bricks-openapi/issues/126)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **analyzer:** document bare handler returns like Result[T] ([#124](https://github.com/gaborage/go-bricks-openapi/issues/124)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **analyzer:** resolve group registrar prefixes positionally ([#94](https://github.com/gaborage/go-bricks-openapi/issues/94)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **analyzer:** resolve local named types in every field position ([#128](https://github.com/gaborage/go-bricks-openapi/issues/128)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **analyzer:** resolve named types from other in-module packages ([#129](https://github.com/gaborage/go-bricks-openapi/issues/129)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **analyzer:** resolve route paths from scoped consts and concatenation ([#82](https://github.com/gaborage/go-bricks-openapi/issues/82)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **analyzer:** resolve success status through local result bindings ([#74](https://github.com/gaborage/go-bricks-openapi/issues/74)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **analyzer:** type named and composite response payloads like fields ([#130](https://github.com/gaborage/go-bricks-openapi/issues/130)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **analyzer:** type text Marshaler types as strings, never $ref one ([#131](https://github.com/gaborage/go-bricks-openapi/issues/131)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **analyzer:** warn on non-struct handler request types ([#127](https://github.com/gaborage/go-bricks-openapi/issues/127)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **generator:** drop date and date-time examples kin rejects ([#106](https://github.com/gaborage/go-bricks-openapi/issues/106)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **generator:** emit builtin format and floor for named scalars ([#92](https://github.com/gaborage/go-bricks-openapi/issues/92)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **generator:** emit format byte for []byte and drop non-base64 examples ([#90](https://github.com/gaborage/go-bricks-openapi/issues/90)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **generator:** emit untyped schema for json.RawMessage ([#93](https://github.com/gaborage/go-bricks-openapi/issues/93)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **generator:** map byte and rune named scalars to integer ([#80](https://github.com/gaborage/go-bricks-openapi/issues/80)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **generator:** render slice result payloads as typed arrays ([#76](https://github.com/gaborage/go-bricks-openapi/issues/76)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **generator:** stop emitting dangling $ref for non-struct payloads ([#108](https://github.com/gaborage/go-bricks-openapi/issues/108)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **generator:** type json.Number, time.Month and time.Weekday ([#125](https://github.com/gaborage/go-bricks-openapi/issues/125)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **generator:** widen int, uint and uint32 to format int64 ([#105](https://github.com/gaborage/go-bricks-openapi/issues/105)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))
* **release:** build with Go 1.27.2 and lint with golangci-lint v2.14.0 ([#133](https://github.com/gaborage/go-bricks-openapi/issues/133)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))


### Changed

* **analyzer:** drop unread TypeInfo.IsPointer ([#72](https://github.com/gaborage/go-bricks-openapi/issues/72)) ([e4dad56](https://github.com/gaborage/go-bricks-openapi/commit/e4dad5623158fd63e81f58dcd030ca93b8020c12))

## [0.4.0](https://github.com/gaborage/go-bricks-openapi/compare/v0.3.1...v0.4.0) (2026-10-09)


### ⚠ BREAKING CHANGES

* **analyzer:** struct Marshaler types and structs that promote a Marshaler method lose their components; their $refs become {} or {type: string}, which moves generated client types, and routes whose payload or request is a non-text Marshaler type newly count as untyped and fail generate --strict, a zero-field request included. Such a type no longer holds its short component name, so a same-named struct of another package that was qualified is renamed to it (BMoney becomes Money, and Money now holds that struct), moving $refs and generated client type names; regenerate clients.
* **analyzer:** payload data schemas move from object to typed schemas, and specs gain components for structs reached only through a payload. A struct registered through a payload can now take a short name first, so a same-named struct of another package registered later is renamed (User becomes <Pkg>User), moving $refs and generated client type names; regenerate clients. Request-less routes with a map, nested-slice or pointer-to-container payload gain their 201/202/NewResult success code and inferred error responses.
* **analyzer:** generated component names can change. A route-package struct can lose its bare name to a same-named struct now registered through another package's named type (User becomes QualifiednamedtypesUser), and a second package sharing a clause, or a named type over a struct sharing a name with one in the struct's package, gets <Pkg><Name> (ModelItem, BUser). $refs and generated client type names move; regenerate clients.

### Added

* **analyzer:** add openapi:errors directive for declared errors ([#85](https://github.com/gaborage/go-bricks-openapi/issues/85)) ([895baf6](https://github.com/gaborage/go-bricks-openapi/commit/895baf679522f8d18769ac36747bcf57530fa408)), closes [#68](https://github.com/gaborage/go-bricks-openapi/issues/68)
* **analyzer:** infer error responses from handler constructor calls ([#87](https://github.com/gaborage/go-bricks-openapi/issues/87)) ([6015661](https://github.com/gaborage/go-bricks-openapi/commit/6015661eec18e361f93c758fd7b5e04f67a10a3e)), closes [#68](https://github.com/gaborage/go-bricks-openapi/issues/68)
* **analyzer:** warn on directives detached from a registration ([#91](https://github.com/gaborage/go-bricks-openapi/issues/91)) ([bb6ab23](https://github.com/gaborage/go-bricks-openapi/commit/bb6ab23cb61515cc94a32036f5c6708344b799d8)), closes [#84](https://github.com/gaborage/go-bricks-openapi/issues/84)
* **commands:** report unresolved routes in generate and doctor output ([#86](https://github.com/gaborage/go-bricks-openapi/issues/86)) ([8db1833](https://github.com/gaborage/go-bricks-openapi/commit/8db1833ce0433e2ebf642c1ab0ef7e82b26c496d)), closes [#64](https://github.com/gaborage/go-bricks-openapi/issues/64)
* **generate:** print warning count in the run summary ([#73](https://github.com/gaborage/go-bricks-openapi/issues/73)) ([d788968](https://github.com/gaborage/go-bricks-openapi/commit/d78896847b9afb81caa0efcf0ff1c98374a3821b))


### Fixed

* **analyzer:** document [N]byte as an integer array, not base64 ([#126](https://github.com/gaborage/go-bricks-openapi/issues/126)) ([e8413c2](https://github.com/gaborage/go-bricks-openapi/commit/e8413c23592982bd605c62643fab6957651b833f))
* **analyzer:** document bare handler returns like Result[T] ([#124](https://github.com/gaborage/go-bricks-openapi/issues/124)) ([2fe498d](https://github.com/gaborage/go-bricks-openapi/commit/2fe498d0707c2e6a446f2485682580ab89a59b53)), closes [#119](https://github.com/gaborage/go-bricks-openapi/issues/119)
* **analyzer:** resolve group registrar prefixes positionally ([#94](https://github.com/gaborage/go-bricks-openapi/issues/94)) ([a00ac9c](https://github.com/gaborage/go-bricks-openapi/commit/a00ac9c404d9bb2d5257d8dd8048e4245238fbc0)), closes [#81](https://github.com/gaborage/go-bricks-openapi/issues/81)
* **analyzer:** resolve local named types in every field position ([#128](https://github.com/gaborage/go-bricks-openapi/issues/128)) ([feeafaa](https://github.com/gaborage/go-bricks-openapi/commit/feeafaa289499f8c1513c367760a052d20f40174)), closes [#109](https://github.com/gaborage/go-bricks-openapi/issues/109) [#88](https://github.com/gaborage/go-bricks-openapi/issues/88) [#97](https://github.com/gaborage/go-bricks-openapi/issues/97) [#123](https://github.com/gaborage/go-bricks-openapi/issues/123) [#101](https://github.com/gaborage/go-bricks-openapi/issues/101) [#120](https://github.com/gaborage/go-bricks-openapi/issues/120)
* **analyzer:** resolve named types from other in-module packages ([#129](https://github.com/gaborage/go-bricks-openapi/issues/129)) ([66f8dfb](https://github.com/gaborage/go-bricks-openapi/commit/66f8dfbce03cc7b1160fb3e77904fb3e9cd4eaee)), closes [#100](https://github.com/gaborage/go-bricks-openapi/issues/100) [#123](https://github.com/gaborage/go-bricks-openapi/issues/123)
* **analyzer:** resolve route paths from scoped consts and concatenation ([#82](https://github.com/gaborage/go-bricks-openapi/issues/82)) ([32676d1](https://github.com/gaborage/go-bricks-openapi/commit/32676d194fb1a4f543b55ae154949c30dc9228b9)), closes [#64](https://github.com/gaborage/go-bricks-openapi/issues/64)
* **analyzer:** resolve success status through local result bindings ([#74](https://github.com/gaborage/go-bricks-openapi/issues/74)) ([e1fedc1](https://github.com/gaborage/go-bricks-openapi/commit/e1fedc18c9cd9b4f58faee3672125a2e1a93fb1e))
* **analyzer:** type named and composite response payloads like fields ([#130](https://github.com/gaborage/go-bricks-openapi/issues/130)) ([6d6746a](https://github.com/gaborage/go-bricks-openapi/commit/6d6746a4fbd04e6951152ea9d44746741d803fc8)), closes [#110](https://github.com/gaborage/go-bricks-openapi/issues/110) [#120](https://github.com/gaborage/go-bricks-openapi/issues/120) [#111](https://github.com/gaborage/go-bricks-openapi/issues/111)
* **analyzer:** type text Marshaler types as strings, never $ref one ([#131](https://github.com/gaborage/go-bricks-openapi/issues/131)) ([7ff8391](https://github.com/gaborage/go-bricks-openapi/commit/7ff839128dca9e069cd54566c72a87211caddb09)), closes [#116](https://github.com/gaborage/go-bricks-openapi/issues/116) [#117](https://github.com/gaborage/go-bricks-openapi/issues/117) [#113](https://github.com/gaborage/go-bricks-openapi/issues/113) [#114](https://github.com/gaborage/go-bricks-openapi/issues/114) [#123](https://github.com/gaborage/go-bricks-openapi/issues/123) [#111](https://github.com/gaborage/go-bricks-openapi/issues/111)
* **analyzer:** warn on non-struct handler request types ([#127](https://github.com/gaborage/go-bricks-openapi/issues/127)) ([514e417](https://github.com/gaborage/go-bricks-openapi/commit/514e417f26da10afea470d4b6cf190c750abc294))
* **generator:** drop date and date-time examples kin rejects ([#106](https://github.com/gaborage/go-bricks-openapi/issues/106)) ([d960b30](https://github.com/gaborage/go-bricks-openapi/commit/d960b3041d3675552d6be90f58067f7db3f29309))
* **generator:** emit builtin format and floor for named scalars ([#92](https://github.com/gaborage/go-bricks-openapi/issues/92)) ([1a0d447](https://github.com/gaborage/go-bricks-openapi/commit/1a0d4477c66d69c06a5f8eb4bc7a4c2c24282eb8))
* **generator:** emit format byte for []byte and drop non-base64 examples ([#90](https://github.com/gaborage/go-bricks-openapi/issues/90)) ([37ee1da](https://github.com/gaborage/go-bricks-openapi/commit/37ee1da46cdbe6a81daaf25c99c6e614832014b4)), closes [#79](https://github.com/gaborage/go-bricks-openapi/issues/79)
* **generator:** emit untyped schema for json.RawMessage ([#93](https://github.com/gaborage/go-bricks-openapi/issues/93)) ([49eb47d](https://github.com/gaborage/go-bricks-openapi/commit/49eb47dc18b80105be32902dc8671a3a37e4b668)), closes [#83](https://github.com/gaborage/go-bricks-openapi/issues/83)
* **generator:** map byte and rune named scalars to integer ([#80](https://github.com/gaborage/go-bricks-openapi/issues/80)) ([162e244](https://github.com/gaborage/go-bricks-openapi/commit/162e244594f367092759d662590d914cd9f71b98)), closes [#60](https://github.com/gaborage/go-bricks-openapi/issues/60)
* **generator:** render slice result payloads as typed arrays ([#76](https://github.com/gaborage/go-bricks-openapi/issues/76)) ([eb25d58](https://github.com/gaborage/go-bricks-openapi/commit/eb25d582c2ff24856de8650631f989ed4cd88d93))
* **generator:** stop emitting dangling $ref for non-struct payloads ([#108](https://github.com/gaborage/go-bricks-openapi/issues/108)) ([72f68f1](https://github.com/gaborage/go-bricks-openapi/commit/72f68f1993bc0564fe70ebde35c7a19cef8544b2)), closes [#96](https://github.com/gaborage/go-bricks-openapi/issues/96)
* **generator:** type json.Number, time.Month and time.Weekday ([#125](https://github.com/gaborage/go-bricks-openapi/issues/125)) ([a841603](https://github.com/gaborage/go-bricks-openapi/commit/a8416032178d37eb8ae017234e99f86baecb87e2)), closes [#89](https://github.com/gaborage/go-bricks-openapi/issues/89)
* **generator:** widen int, uint and uint32 to format int64 ([#105](https://github.com/gaborage/go-bricks-openapi/issues/105)) ([09c4048](https://github.com/gaborage/go-bricks-openapi/commit/09c4048e71fe3654efe888723b217c69035338ed)), closes [#95](https://github.com/gaborage/go-bricks-openapi/issues/95)


### Changed

* **analyzer:** drop unread TypeInfo.IsPointer ([#72](https://github.com/gaborage/go-bricks-openapi/issues/72)) ([cfa67ed](https://github.com/gaborage/go-bricks-openapi/commit/cfa67ed36eb641cdf4bc56d52c07d978948d8deb))

## [0.3.1](https://github.com/gaborage/go-bricks-openapi/compare/v0.3.0...v0.3.1) (2026-09-05)


### Changed

* **analyzer:** decode type shape once, retire string parsing ([#55](https://github.com/gaborage/go-bricks-openapi/issues/55)) ([faaa718](https://github.com/gaborage/go-bricks-openapi/commit/faaa718208f059cc7c89b615cd0519a04b76558e))
* **generator:** move constraint mapping out of the analyzer ([#58](https://github.com/gaborage/go-bricks-openapi/issues/58)) ([1043d43](https://github.com/gaborage/go-bricks-openapi/commit/1043d43f4a5b38f7af8402e4da9937412a3713ab))
* **generator:** typed constraint set replaces pair-list applicators ([#61](https://github.com/gaborage/go-bricks-openapi/issues/61)) ([96c3ddb](https://github.com/gaborage/go-bricks-openapi/commit/96c3ddb1df9c05e0f2987db02189fdac9c1782ea))

## [0.3.0](https://github.com/gaborage/go-bricks-openapi/compare/v0.2.0...v0.3.0) (2026-07-26)


### ⚠ BREAKING CHANGES

* **commands:** doctor now fails projects on go-bricks < v0.45.0 — the release that hid echo.* types behind go-bricks boundary abstractions (upstream #627). The old v0.13.0 floor predated the API era the analyzer's recognized patterns, the fixtures, and the generator's emitted runtime contract are actually verified against.

### Added

* **analyzer:** warn when a malformed struct tag hides read keys ([#50](https://github.com/gaborage/go-bricks-openapi/issues/50)) ([41532ed](https://github.com/gaborage/go-bricks-openapi/commit/41532ed305c96882d858a0b92e9fcb4289fd9c21))
* **commands:** require go-bricks v0.45.0+ and verify through v0.53.0 ([#26](https://github.com/gaborage/go-bricks-openapi/issues/26)) ([54d71b5](https://github.com/gaborage/go-bricks-openapi/commit/54d71b56d2f1ef8e069d8f763b5b1dec92e00b6c))


### Fixed

* **analyzer:** bound struct-registration recursion depth (DoS hardening) ([#38](https://github.com/gaborage/go-bricks-openapi/issues/38)) ([b30a757](https://github.com/gaborage/go-bricks-openapi/commit/b30a757b2917f6d84a55e8f22bb68449076570be))
* **analyzer:** decode Go string literals instead of trimming delimiters ([#45](https://github.com/gaborage/go-bricks-openapi/issues/45)) ([97ff8e0](https://github.com/gaborage/go-bricks-openapi/commit/97ff8e02f57093485dc32b6c61d5ff4ecc1cf61a))
* **analyzer:** detect jose: tag on any field, matching jose.ScanType ([#44](https://github.com/gaborage/go-bricks-openapi/issues/44)) ([56ead6b](https://github.com/gaborage/go-bricks-openapi/commit/56ead6b162c172a72d1d382a9b2fc1a8fce4d2aa))
* **analyzer:** enforce project-root containment incl. symlinks on the parse path ([#37](https://github.com/gaborage/go-bricks-openapi/issues/37)) ([4b89135](https://github.com/gaborage/go-bricks-openapi/commit/4b89135a075115956f8a137733ce953bafa70887))
* **analyzer:** follow package-level helper functions in RegisterRoutes ([#35](https://github.com/gaborage/go-bricks-openapi/issues/35)) ([d255dfc](https://github.com/gaborage/go-bricks-openapi/commit/d255dfca3dd8f17227e6ca41ba24a63917dc98ea))
* **analyzer:** read struct tags with reflect.StructTag, retiring extractTag ([#46](https://github.com/gaborage/go-bricks-openapi/issues/46)) ([cfb3c20](https://github.com/gaborage/go-bricks-openapi/commit/cfb3c2013c10a5a4ca2a119a325e770e93746070))
* **analyzer:** recognize explicitly-instantiated server.RegisterHandler[T,R] routes ([#24](https://github.com/gaborage/go-bricks-openapi/issues/24)) ([3a66dbc](https://github.com/gaborage/go-bricks-openapi/commit/3a66dbc402c7fa8e3efbcb217b4c29ed4396a3c4))
* **analyzer:** resolve type aliases to structs; warn instead of dangling $refs ([#33](https://github.com/gaborage/go-bricks-openapi/issues/33)) ([7173cb2](https://github.com/gaborage/go-bricks-openapi/commit/7173cb23de77a9f589a5c685bb6a7b4c224e79f5))
* **analyzer:** warn on unparsable files, dir-keyed module dedup, skip nested modules ([#32](https://github.com/gaborage/go-bricks-openapi/issues/32)) ([ce050f5](https://github.com/gaborage/go-bricks-openapi/commit/ce050f5c0b0617d2ac7e466b3d5471caae8bd242))
* **commands:** fail --strict when the analyzer drops routes (emits warnings) ([#23](https://github.com/gaborage/go-bricks-openapi/issues/23)) ([8a38bdc](https://github.com/gaborage/go-bricks-openapi/commit/8a38bdc80d8384acd18bdf414e47cf385c2f9be0))
* **commands:** share the content verdict between doctor and generate; honor only applicable replaces ([#36](https://github.com/gaborage/go-bricks-openapi/issues/36)) ([1602975](https://github.com/gaborage/go-bricks-openapi/commit/1602975b554f24ab1e45f93e2273c4266e959a54))
* **generator:** coerce example tag values to the declared schema type ([#48](https://github.com/gaborage/go-bricks-openapi/issues/48)) ([d426212](https://github.com/gaborage/go-bricks-openapi/commit/d426212ca349d67fbbb1f7b43e9a60c3552a1179))
* **generator:** emit an unconstrained schema for any/interface{} values ([#39](https://github.com/gaborage/go-bricks-openapi/issues/39)) ([8ba3de4](https://github.com/gaborage/go-bricks-openapi/commit/8ba3de4b49e6de970754c5adf46ff960c0c67a35))
* **generator:** mark pointer fields nullable (accept serialized null) ([#40](https://github.com/gaborage/go-bricks-openapi/issues/40)) ([aad52ec](https://github.com/gaborage/go-bricks-openapi/commit/aad52ecf4d8154836e8cbf337a544935d99f82ff))
* **generator:** synthesize path parameters for uncovered template variables ([#34](https://github.com/gaborage/go-bricks-openapi/issues/34)) ([9f17943](https://github.com/gaborage/go-bricks-openapi/commit/9f179431a59afbcf6f81065f3872ef299fc2f38e))

## [0.2.0](https://github.com/gaborage/go-bricks-openapi/compare/v0.1.0...v0.2.0) (2026-07-14)


### Added

* **analyzer:** //openapi:public directive, RegisterHandler + WithModule recognition ([#15](https://github.com/gaborage/go-bricks-openapi/issues/15)) ([de637a9](https://github.com/gaborage/go-bricks-openapi/commit/de637a95e3ca86288ce964e1345c46771c29db7f))
* **analyzer:** recognize raw RouteRegistrar.Add routes ([#17](https://github.com/gaborage/go-bricks-openapi/issues/17)) ([030f739](https://github.com/gaborage/go-bricks-openapi/commit/030f739438b77c1ee1476f94b2c64d75403be5e4))
* **release:** adopt go-bricks release-please + signed-tag mechanism ([#12](https://github.com/gaborage/go-bricks-openapi/issues/12)) ([77dff6c](https://github.com/gaborage/go-bricks-openapi/commit/77dff6c8cf71070a136324ac668108415796fe0f))


### Fixed

* **analyzer:** follow handler-field delegation in RegisterRoutes ([#14](https://github.com/gaborage/go-bricks-openapi/issues/14)) ([d45dd33](https://github.com/gaborage/go-bricks-openapi/commit/d45dd333f02484dfdc67e507fb9d73914709ad68))
* **analyzer:** resolve project root to an absolute path before walking ([#19](https://github.com/gaborage/go-bricks-openapi/issues/19)) ([60ae316](https://github.com/gaborage/go-bricks-openapi/commit/60ae3166f6a951a835cce4dc764edfaa9889a261))
* **generator:** accurate v0.45 error and security modeling ([#16](https://github.com/gaborage/go-bricks-openapi/issues/16)) ([62aab60](https://github.com/gaborage/go-bricks-openapi/commit/62aab60ee093ce4a05a1a877fff223d692c1a793))
* **generator:** exclude path/query/header params from the request-body schema ([#21](https://github.com/gaborage/go-bricks-openapi/issues/21)) ([0c9d4b6](https://github.com/gaborage/go-bricks-openapi/commit/0c9d4b6052837f58c58d77ee516898d82b778346))
* **generator:** property-less types no longer produce unresolvable references ([#22](https://github.com/gaborage/go-bricks-openapi/issues/22)) ([d0f1c7c](https://github.com/gaborage/go-bricks-openapi/commit/d0f1c7c064b8e102a768a6e073ad406c41e80a63))

## [0.1.0](https://github.com/gaborage/go-bricks-openapi/releases/tag/v0.1.0) (2026-06-01)


### Added

* import go-bricks-openapi as a standalone repository ([#1](https://github.com/gaborage/go-bricks-openapi/issues/1))
* OpenAPI schema validation command and `--validate` flag ([#6](https://github.com/gaborage/go-bricks-openapi/issues/6))
* per-operation security opt-out via `server.WithPublic()` ([#7](https://github.com/gaborage/go-bricks-openapi/issues/7))
* emit `minProperties`/`maxProperties` for map field cardinality ([#8](https://github.com/gaborage/go-bricks-openapi/issues/8))
* emit most-restrictive bound for overlapping validate rules ([#9](https://github.com/gaborage/go-bricks-openapi/issues/9))
* honest versioning, SLSA build provenance, and release runbook ([#11](https://github.com/gaborage/go-bricks-openapi/issues/11))
