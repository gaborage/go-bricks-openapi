# Changelog

## [0.3.0](https://github.com/gaborage/go-bricks-openapi/compare/v0.2.0...v0.3.0) (2026-07-24)


### ⚠ BREAKING CHANGES

* **commands:** doctor now fails projects on go-bricks < v0.45.0 — the release that hid echo.* types behind go-bricks boundary abstractions (upstream #627). The old v0.13.0 floor predated the API era the analyzer's recognized patterns, the fixtures, and the generator's emitted runtime contract are actually verified against.

### Added

* **commands:** require go-bricks v0.45.0+ and verify through v0.53.0 ([#26](https://github.com/gaborage/go-bricks-openapi/issues/26)) ([54d71b5](https://github.com/gaborage/go-bricks-openapi/commit/54d71b56d2f1ef8e069d8f763b5b1dec92e00b6c))


### Fixed

* **analyzer:** bound struct-registration recursion depth (DoS hardening) ([#38](https://github.com/gaborage/go-bricks-openapi/issues/38)) ([b30a757](https://github.com/gaborage/go-bricks-openapi/commit/b30a757b2917f6d84a55e8f22bb68449076570be))
* **analyzer:** enforce project-root containment incl. symlinks on the parse path ([#37](https://github.com/gaborage/go-bricks-openapi/issues/37)) ([4b89135](https://github.com/gaborage/go-bricks-openapi/commit/4b89135a075115956f8a137733ce953bafa70887))
* **analyzer:** follow package-level helper functions in RegisterRoutes ([#35](https://github.com/gaborage/go-bricks-openapi/issues/35)) ([d255dfc](https://github.com/gaborage/go-bricks-openapi/commit/d255dfca3dd8f17227e6ca41ba24a63917dc98ea))
* **analyzer:** recognize explicitly-instantiated server.RegisterHandler[T,R] routes ([#24](https://github.com/gaborage/go-bricks-openapi/issues/24)) ([3a66dbc](https://github.com/gaborage/go-bricks-openapi/commit/3a66dbc402c7fa8e3efbcb217b4c29ed4396a3c4))
* **analyzer:** resolve type aliases to structs; warn instead of dangling $refs ([#33](https://github.com/gaborage/go-bricks-openapi/issues/33)) ([7173cb2](https://github.com/gaborage/go-bricks-openapi/commit/7173cb23de77a9f589a5c685bb6a7b4c224e79f5))
* **analyzer:** warn on unparsable files, dir-keyed module dedup, skip nested modules ([#32](https://github.com/gaborage/go-bricks-openapi/issues/32)) ([ce050f5](https://github.com/gaborage/go-bricks-openapi/commit/ce050f5c0b0617d2ac7e466b3d5471caae8bd242))
* **commands:** fail --strict when the analyzer drops routes (emits warnings) ([#23](https://github.com/gaborage/go-bricks-openapi/issues/23)) ([8a38bdc](https://github.com/gaborage/go-bricks-openapi/commit/8a38bdc80d8384acd18bdf414e47cf385c2f9be0))
* **commands:** share the content verdict between doctor and generate; honor only applicable replaces ([#36](https://github.com/gaborage/go-bricks-openapi/issues/36)) ([1602975](https://github.com/gaborage/go-bricks-openapi/commit/1602975b554f24ab1e45f93e2273c4266e959a54))
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
