# Changelog

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
