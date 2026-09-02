# CLAUDE.md

Repo-specific gates and settled invariants for `go-bricks-openapi`. This is a static-analysis tool: it walks `go/ast` over a target project's source and never imports or runs it — anything requiring compiling or executing the target project is out of scope.

## What must go green

- `main` is governed by a repository **ruleset**, not classic branch protection.
- `gh api repos/:owner/:repo/branches/main/protection` returning 404 does **not** mean `main` is unprotected — check `gh api repos/:owner/:repo/rulesets` instead.
- Eight required status checks: `Test (ubuntu-latest)`, `Test (windows-latest)`, `Build & Validate (ubuntu-latest)`, `Build & Validate (windows-latest)`, `Lint`, `Security (gosec)`, `SonarCloud`, `GoReleaser config check`.
- The branch must be up to date with `main` before merge (strict policy).
- CodeQL also blocks and is **not** in that list — a separate `code_scanning` rule in the same ruleset, run via GitHub's CodeQL default setup; no workflow file backs it.
- CodeQL appears on PRs as `CodeQL`, `Analyze (go)`, `Analyze (actions)`.
- The ruleset also requires squash-only merges, linear history, one approving review, stale-review dismissal on push, and resolution of every review thread.
- The ruleset names no specific required reviewer — don't claim any bot is one.
- `SonarCloud` blocks on the server-computed gate only because the scan step passes `-Dsonar.qualitygate.wait=true` in `ci.yml`.

## The two complexity limits — only one is checkable locally

- `make lint` enforces **cyclomatic** complexity via `gocyclo` in `.golangci.yml`, at `min-complexity: 15`.
- `gocognit` is **not** enabled — the linter enforces no cognitive-complexity limit at all.
- SonarCloud separately enforces **cognitive** complexity via rule `go:S3776`, threshold 15, server-side only.
- Consequence: a function can pass `make lint` cleanly and still fail the required `SonarCloud` check.
- Split on cognitive grounds, not just cyclomatic, before pushing.
- Check locally with `go run github.com/uudashr/gocognit/cmd/gocognit@latest <file>`.
- `sonar-project.properties` configures no thresholds — every gate number is server-side in the default `Sonar way` gate, not in that file.
- The server-side gate: new-code coverage ≥ 80%, new duplicated lines ≤ 3%, new ratings A.
- Sonar analyzes `cmd` and `internal`, excluding `**/*_test.go` and `**/testdata/**` — a new fixture contributes nothing toward the coverage gate.

## `make check` mutates the working tree

- `check` runs `fmt lint test validate-cli`, in that order.
- `fmt` (`go fmt ./...`) runs first, so the gate itself rewrites tracked `.go` files.
- There is no formatting gate in CI — formatting is enforced only by running `make fmt` yourself.
- `make check` needs `golangci-lint` already installed via `make dev-deps` (version pinned by `GOLANGCI_VERSION`).
- On a fresh clone `make check` fails at the lint step with "command not found," not a lint finding.
- `gofmt -l ./cmd ./internal` has a permanent, unfixable hit at `internal/spectest/testdata/raw_add/api.go` (Go tooling skips `testdata/`).
- Don't use `gofmt -l` as the formatting check because of that permanent hit.
- `make validate-cli` depends on `build` and leaves a `go-bricks-openapi` binary in the repo root (gitignored).
- `make clean` runs `go clean -cache -testcache`, which wipes the machine-wide Go build/test cache, not just this repo's.
- `make sec`'s green exit isn't evidence it ran: gosec silently scans 0 files and exits 0 when given Go import paths.
- The `sec` target omits CI's scanned-file-count assertion — trust the CI `Security (gosec)` job, not a local `make sec`.
- `make validate-spec` (redocly) is deliberately excluded from `check` — it needs `npx` and network.
- `make validate-spec` runs only on Ubuntu, against one fixture: `internal/spectest/testdata/nested_schema`.
- It is belt-and-suspenders — the primary structural gate is the in-process kin-openapi validation in `internal/spectest`.

## Golden fixtures

- Regenerate with `go test ./internal/spectest -update` — there is no make target for this.
- "Update the goldens" must never mean `make update`, which is dependency updating (`go get -u ./... && go mod tidy`).
- `-update` is package-scoped: `go test ./... -update` fails every other package with "flag provided but not defined."
- A failing golden test never rewrites the golden — the write sits inside `if *update` and returns before the golden is read.
- A clean `git status` after a test run is not evidence a golden test passed — only the exit status is.
- Validation runs before comparison and before any `-update` write, so `-update` cannot persist a structurally invalid spec.
- Comparison is byte-exact apart from CRLF→LF on both sides.
- `.gitattributes` pins the fixture YAML to `eol=lf` as the primary guarantee; in-test normalization is the second line of defense.
- Fixtures are auto-enrolled by directory listing under `internal/spectest/testdata` — every subdirectory becomes a subtest, with no registration list.
- The golden test function is `TestGoldenFixtures`; a `-run 'TestFixtures/...'` pattern matches nothing and exits 0, looking like a pass.
- A new fixture needs a `go.mod`: module under `github.com/example/`, `go 1.25`, `require github.com/gaborage/go-bricks v0.53.0`, no `go.sum`, no `replace`.
- A new fixture also needs one `.go` file implementing a go-bricks module.
- Fixture Go code is only AST-parsed, never compiled — undeclared types are tolerated.
- Underscore directory names take an underscore-free package name (e.g. `jose_named_field/` → `package tokenize`).

## Test package set

- `TEST_PACKAGES` in the Makefile filters out `/models`: `go list ./... | grep -vE '/models$'`.
- CI's Unix leg uses the byte-identical filter, so `make test` and CI's Unix leg agree on which packages run.
- CI's Windows leg does **not** filter — it runs all 8 packages.
- A test added under `internal/models` is not run by `make test`, `make check`, or either Unix leg — but runs on Windows, where it can fail alone.
- `internal/models` is intentionally test-free (struct-only); it reports `[no test files]`.
- There is zero `t.Parallel()` in `internal/` or `cmd/`, deliberately.
- `internal/testutil`'s stdout-capture helper swaps the global `os.Stdout`; its doc says not to call it from parallel tests.
- The Windows leg forgives nothing by design — a pattern-based failure allowlist was deleted on purpose; don't reintroduce one.
- `go.mod` declares `go 1.25.0` with no `toolchain` directive while CI's setup-go steps pin a newer Go.
- The 1.25 floor is intentional — don't rely on language features newer than that.

## Settled invariants — do not "clean these up"

- `lookupStructTag` (`internal/analyzer/analyzer.go`) is the only sanctioned way to read a struct-tag key: `json`, `param`, `query`, `header`, `doc`, `example`, `validate`.
- It replaced a hand-rolled scanner that matched any key *ending in* the target (so `queryparam:"x"` hijacked `param:`) and truncated values at the first quote byte.
- It discards `reflect.StructTag.Lookup`'s presence bit on purpose — every call site must keep gating on `!= ""`.
- `json:""` is the sentinel that keeps an embedded struct promoted rather than nested; `param:""` would otherwise become a path parameter with an empty name.
- Refactoring to `if v, ok := Lookup(...); ok` is a behavior change, not a cleanup.
- `unquoteLiteral` (same file) is the only sanctioned way to turn a `go/parser` string literal's source text into its value.
- Its fallback strips only the delimiter actually present — a combined cutset would eat a raw struct tag's own closing quote, turning `json:"pan"` into `json:"pan`.
- `hiddenTagKeys` (`internal/analyzer/tagcheck.go`) warns when a struct tag is malformed enough that `reflect.StructTag`'s own scan stops early, hiding one of the seven keys `lookupStructTag` reads.
- Because analyzer warnings feed `--strict`, a project with such a tag newly fails `generate --strict` with no artifact — a behavior change, not a bug.
- The detector reports only when reflect's scan stops early; a mangled key that reflect still parses as a (wrong) key name — e.g. `json:"b";validate:"c"` reads a key literally named `;validate` — is not reported, and no tool in this repo's toolchain catches every such shape.
- In `internal/generator/openapi.go`, `referencedSchemaNames` deliberately does not scan non-JOSE request types.
- Adding them orphans a component for every params-only request type and trips redocly's `no-unused-components`.
- Don't "optimize" the `schema == nil` check in `generateSchemasFromTypes` into a zero-properties check — it would drop the component and dangle a `requestBody` `$ref`.
- The `json_excluded_request` golden locks this invariant.
- `example:` coercion runs in exactly one place: the thin `fieldInfoToProperty` wrapper, which calls `applyExample` after `buildFieldProperty` resolves the property's type.
- `buildFieldProperty` itself must not stamp examples.
- An `example:` with no valid representation in its declared type is dropped silently — no warning, no `--strict` failure.
- This is deliberate: a missing example costs one hint, an ill-typed one would make the whole document invalid.
- A kept example can still violate `enum`, `format`, `minLength`/`maxLength`, or `pattern` — a known, accepted residual.
- `NOSONAR` and `//nolint` are not interchangeable: `NOSONAR` suppresses a SonarCloud rule, `//nolint` names golangci-lint linters, and both forms exist here.
- `nolintlint` runs with `allow-unused: false`, so an unnecessary `//nolint` is itself a lint error — don't strip either blindly.

## Lint surprises

- `unused` is enabled even though `.golangci.yml` never names it: no `linters.default` key is set, so golangci-lint v2's `standard` set applies on top of the explicit `enable:` list.
- `gocritic` runs with all five tag groups on (`diagnostic`, `experimental`, `opinionated`, `performance`, `style`), far stricter than its default.
- `revive` runs at `confidence: 0`, so every rule at any confidence fires.
- `lll` is at 215, not the 120 default.
- Test files are linted, but `gocyclo`, `gosec`, `goconst`, `dupl`, `errcheck`, and `govet` are excluded on `_test.go`.
- The golangci-lint pin lives in two places that must move together: the `GOLANGCI_VERSION` variable in `Makefile`, and the `golangci-lint-action` `version:` key in `ci.yml`.
- A newer local golangci-lint can pass where the pinned one fails: PR #55 was clean under a local v2.13.2 while CI's pinned v2.12.2 flagged `goconst` (its occurrence counting differs and includes `_test.go` files). Lint with the pinned version — `make dev-deps` installs it — before trusting a local `make lint`.
- The mechanism: `make dev-deps` installs the pin into `$(go env GOPATH)/bin`, but `make lint` runs bare `golangci-lint`, so a Homebrew install earlier on `PATH` shadows it silently. Run `PATH="$HOME/go/bin:$PATH" make lint` (or check `golangci-lint --version` first).
- `goconst` counts occurrences across the whole package including `_test.go` files (it only suppresses *findings* there), so moving a file into a package can push existing literals over the threshold with no new code — PR #58 hit 18 such findings from a pure relocation.

## Commits & releases

- No git hook of any kind exists — `core.hooksPath` is unset, `.git/hooks/` holds only samples.
- Nothing in CI validates commit-message or PR-title format.
- Conventional Commit form is enforced by consequence only: release-please parses the squash subject into the changelog and the version bump.
- A malformed subject fails no gate; it silently misfiles or omits the release note.
- The squash subject is not always the PR title — the live setting is `squash_merge_commit_title: COMMIT_OR_PR_TITLE`.
- A one-commit PR uses the commit's subject; only a multi-commit PR uses the PR title.
- Changelog-visible types: `feat`→Added, `fix`→Fixed, `perf`/`refactor`→Changed, `deprecate`→Deprecated, `remove`→Removed, `revert`→Fixed.
- Hidden types that do not bump the version: `docs`, `chore`, `test`, `build`, `ci`.
- Pre-1.0 policy: `feat`→MINOR, `fix`→PATCH, and BREAKING is capped to MINOR while on 0.x.
- Still mark breaking changes `feat!:` / `BREAKING CHANGE:` even though the bump is capped.
- The three declared SemVer surfaces are the CLI surface, the generated-output shape, and doctor/validation behavior (see `RELEASING.md`).
- Never merge a Release PR you are not ready to tag immediately.
- A merged-but-untagged Release PR keeps its `autorelease: pending` label and release-please then aborts silently for all future releases (googleapis/release-please#1561).
- `release-please.yml` carries a guard step that turns that silent abort red.
- Don't diagnose a deadlock from `gh pr list --label 'autorelease: pending'` alone — it serves a stale search index and returns phantom entries.
- `make release` requires `VERSION` on the command line (`make release VERSION=v0.3.0`); it rejects it from the environment.
- `scripts/release.sh` hard-refuses anything but strict pre-1.0 `v0.MINOR.PATCH`.
- `RELEASING.md` describes a `v*` tag-protection ruleset as required and as the enforcing control — but no such ruleset currently exists.
- The only repository ruleset targets `branch`, not `tag`.
- Until that gap is closed, the in-job signature and ancestry checks in `release.yml` are the only control actually in place — don't assume otherwise.

## Doc rot

Stale design docs here are not deleted — they get a `> **SUPERSEDED (date).**` banner naming the authoritative source (see `docs/superpowers/specs/`). Apply the same convention to this file if it goes stale: supersede in place, don't delete.

See README's "Known limitations" section for the analyzer's known gaps — those are decided tradeoffs, not bugs to fix.

## Agent skills

### Issue tracker

Issues live in GitHub Issues for `gaborage/go-bricks-openapi`, via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context: one `CONTEXT.md` + `docs/adr/` at the repo root (created lazily). See `docs/agents/domain.md`.
