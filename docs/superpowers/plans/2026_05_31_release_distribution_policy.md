# Release & Distribution Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the existing-but-unproven release pipeline honor the approved policy so the first real release (`v0.1.0`) is correct and repeatable — honest version reporting, keyless build provenance, and a documented manual-tagging runbook.

**Architecture:** Four independent changes. (1) A unit-testable `ResolveVersion` helper in the `commands` package adds a `runtime/debug` build-metadata fallback so `go install`ed binaries report their real version. (2) The tag-triggered release workflow gains a keyless SLSA provenance attestation over GoReleaser's checksums file. (3) The README documents the working `@latest` install and how to verify provenance. (4) A new `RELEASING.md` captures the versioning policy and the manual cut-a-release runbook. No `.goreleaser.yaml` change is needed.

**Tech Stack:** Go 1.25 · cobra · GoReleaser v2 · GitHub Actions (`actions/attest-build-provenance@v4`) · testify.

**Spec:** `docs/superpowers/specs/2026_05_31_release_distribution_policy_design.md`

---

## File Structure

| File | Responsibility | Action |
|------|----------------|--------|
| `internal/commands/version.go` | Owns version resolution + printing. Add `ResolveVersion` + a `readBuildVersion` seam + `devVersion` const. | Modify |
| `internal/commands/version_test.go` | Unit tests for version resolution across the precedence cases. | Modify |
| `cmd/go-bricks-openapi/main.go` | Wire `ResolveVersion(version)` into the root command + version subcommand. | Modify |
| `.github/workflows/release.yml` | Add provenance permissions + the attestation step. | Modify |
| `README.md` | Document `@latest` install + a "Verifying a release" snippet + a Releasing pointer. | Modify |
| `RELEASING.md` | The manual-tagging runbook, `0.x` bump rules, commit hygiene, verification. | Create |

---

## Task 1: Honest version reporting (`ResolveVersion` + BuildInfo seam)

**Why:** `main.version` is only set by ldflags (Makefile / GoReleaser). A user who runs `go install …@v0.1.0` never triggers those ldflags, so `version` prints `dev`. `go install` records the resolved module version in the binary's build metadata, so a `runtime/debug.ReadBuildInfo()` fallback recovers it. The lookup goes behind a package-level `var` seam so tests can drive every precedence branch deterministically (the same seam idiom the repo already uses for `run()` and the `version` global).

**Files:**
- Modify: `internal/commands/version.go`
- Test: `internal/commands/version_test.go`
- Modify: `cmd/go-bricks-openapi/main.go:18-41` (`buildRootCmd`)

- [ ] **Step 1: Write the failing test**

Append to `internal/commands/version_test.go` (the package already imports `testing` and `testify/assert`):

```go
// TestResolveVersion verifies the precedence: an ldflags-injected version wins;
// otherwise the module build version recorded by `go install`; otherwise "dev".
// readBuildVersion is swapped out so the build-metadata branch is deterministic.
func TestResolveVersion(t *testing.T) {
	tests := []struct {
		name     string
		injected string
		buildVer string
		buildOK  bool
		want     string
	}{
		{"ldflag wins over build metadata", "v1.2.3", "v9.9.9", true, "v1.2.3"},
		{"go install tag used when injected is the dev sentinel", "dev", "v0.1.0", true, "v0.1.0"},
		{"go install tag used when injected is empty", "", "v0.1.0", true, "v0.1.0"},
		{"falls back to dev when no build metadata (dev sentinel)", "dev", "", false, "dev"},
		{"falls back to dev when no build metadata (empty)", "", "", false, "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := readBuildVersion
			t.Cleanup(func() { readBuildVersion = orig })
			readBuildVersion = func() (string, bool) { return tt.buildVer, tt.buildOK }

			assert.Equal(t, tt.want, ResolveVersion(tt.injected))
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails (does not compile)**

Run: `go test ./internal/commands/ -run TestResolveVersion`
Expected: build failure — `undefined: readBuildVersion` and `undefined: ResolveVersion`.

- [ ] **Step 3: Write the minimal implementation**

In `internal/commands/version.go`, add `"runtime/debug"` to the import block and add the following above `NewVersionCommand`:

```go
// devVersion is the sentinel used when no real version is available — it matches
// the default value of main.version before ldflags or module metadata override it.
const devVersion = "dev"

// readBuildVersion returns the main module's version from the metadata the Go
// toolchain embeds in the binary, and whether a usable value was found. It is a
// package-level var so tests can substitute the lookup.
//
// `go install module@vX.Y.Z` records the tag here even though it never runs the
// Makefile/GoReleaser ldflags — this is what lets the go-install channel report
// an honest version. A local `go build` reports "(devel)" or empty, which is
// treated as "no usable version".
var readBuildVersion = func() (string, bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		return "", false
	}
	return v, true
}

// ResolveVersion picks the most authoritative version string available.
// Precedence: an ldflags-injected build version (GoReleaser / `make build`) wins;
// otherwise the main module version recorded by `go install`; otherwise the
// "dev" sentinel. injected is main.version, which defaults to "dev".
func ResolveVersion(injected string) string {
	if injected != "" && injected != devVersion {
		return injected
	}
	if v, ok := readBuildVersion(); ok {
		return v
	}
	return devVersion
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/commands/ -run TestResolveVersion -v`
Expected: PASS (all five subtests).

- [ ] **Step 5: Wire `ResolveVersion` into the CLI**

Replace `buildRootCmd` in `cmd/go-bricks-openapi/main.go` (lines 18-41) with the version below. The only changes are computing `resolved` once and using it for both `Version:` and `NewVersionCommand(...)`:

```go
// buildRootCmd constructs the go-bricks-openapi root command with all subcommands.
// The version is resolved once (ldflags > go-install module metadata > "dev") and
// surfaced by both the --version flag and the version subcommand.
func buildRootCmd() *cobra.Command {
	resolved := commands.ResolveVersion(version)

	rootCmd := &cobra.Command{
		Use:   "go-bricks-openapi",
		Short: "Generate OpenAPI specs for go-bricks services",
		Long: `Static analysis-based OpenAPI 3.0.1 specification generator for go-bricks applications.

This tool analyzes go-bricks services and generates OpenAPI specifications automatically
from route registrations, type definitions, and validation tags.`,
		Version: resolved,
		// Errors and usage are reported once by run() (a single "Error:" line, no
		// usage dump) rather than by cobra's default handler on a RunE error.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(
		commands.NewGenerateCommand(),
		commands.NewValidateCommand(),
		commands.NewDoctorCommand(),
		commands.NewVersionCommand(resolved),
	)

	return rootCmd
}
```

- [ ] **Step 6: Run the full command + main suite to verify no regression**

Run: `go test -race ./cmd/... ./internal/commands/...`
Expected: PASS. (The existing `main_test.go` version tests inject non-`dev` versions like `test-9.9.9`, so `ResolveVersion` returns them unchanged; the default-`dev` paths resolve to `dev` because a `go test` binary reports no usable module version.)

- [ ] **Step 7: Smoke-test the built binary**

Run: `make validate-cli`
Expected: ends with `✓ CLI validation passed`. (`make build` injects `git describe --tags --always --dirty` via ldflags — a short commit SHA while no tags exist — so `ResolveVersion` returns that injected value and the `version` line prints the SHA. `validate-cli` only asserts the command runs, not its value.)

- [ ] **Step 8: Commit**

```bash
git add internal/commands/version.go internal/commands/version_test.go cmd/go-bricks-openapi/main.go
git commit -m "fix: report honest version for go install builds

Fall back to runtime/debug build metadata when main.version was not set
by ldflags, so 'go install ...@vX.Y.Z' reports the tag instead of 'dev'.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Build provenance in the release workflow

**Why:** Attach a keyless (OIDC + Sigstore) SLSA build provenance attestation to every release, verifiable with `gh attestation verify`. `subject-checksums` points at GoReleaser's `dist/checksums.txt`, producing one attestation covering every archive and binary listed there. `create-storage-record: false` avoids the org-only storage-record path (this is a user-owned repo), so only the two long-established permission scopes are needed.

**Files:**
- Modify: `.github/workflows/release.yml:8-9` (permissions) and add a step after the GoReleaser step (lines 28-34).

- [ ] **Step 1: Widen the workflow permissions**

Replace the `permissions:` block (lines 8-9) with:

```yaml
permissions:
  contents: write       # create the GitHub Release and upload assets
  id-token: write       # mint the OIDC token for keyless Sigstore signing
  attestations: write   # persist the build provenance attestation
```

- [ ] **Step 2: Add the attestation step after GoReleaser**

Immediately after the existing `Run GoReleaser` step (currently ending at line 34), add:

```yaml
      - name: Attest build provenance
        uses: actions/attest-build-provenance@v4
        with:
          # GoReleaser writes dist/checksums.txt (sha256 digests + names); one
          # attestation covers every archive and binary listed in it.
          subject-checksums: dist/checksums.txt
          # Storage records are org-only; this is a user-owned repo, so skip it
          # and keep the permission set to id-token + attestations.
          create-storage-record: false
```

- [ ] **Step 3: Verify the expected keys exist and are ordered correctly**

Dependency-free structural check (no YAML library needed — the authoritative
validation is GitHub running the workflow on the first real tag):

```bash
f=.github/workflows/release.yml
grep -q "id-token: write" "$f" \
  && grep -q "attestations: write" "$f" \
  && grep -q "contents: write" "$f" \
  && grep -q "actions/attest-build-provenance@v4" "$f" \
  && grep -q "subject-checksums: dist/checksums.txt" "$f" \
  && [ "$(grep -n 'goreleaser-action' "$f" | head -1 | cut -d: -f1)" \
       -lt "$(grep -n 'attest-build-provenance' "$f" | head -1 | cut -d: -f1)" ] \
  && echo "release.yml OK: provenance after GoReleaser, permissions + subject set"
```

Expected: `release.yml OK: provenance after GoReleaser, permissions + subject set`
(If `actionlint` is installed, `actionlint .github/workflows/release.yml` is a stronger optional check.)

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: attest SLSA build provenance for release artifacts

Add a keyless actions/attest-build-provenance step over GoReleaser's
checksums.txt, verifiable with 'gh attestation verify'.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Document install + release verification in the README

**Why:** `go install …@latest` starts resolving the moment `v0.1.0` is tagged, and users need to know how to verify the new provenance. This task adds a "Verifying a release" subsection and a pointer to the runbook from Task 4.

**Files:**
- Modify: `README.md` (Installation section, lines 14-30; add a Releasing pointer before `## License`).

- [ ] **Step 1: Add the verification subsection**

In `README.md`, find this block (lines 21-23):

```
Prebuilt binaries for Linux, macOS, and Windows are attached to each
[GitHub Release](https://github.com/gaborage/go-bricks-openapi/releases).
```

Insert the following immediately after it (before the `Or build from source:` line). The outer fence below is shown with four backticks so the inner code block is literal — write the inner content (from `### Verifying a release` through the closing triple-backtick) into the file:

````
### Verifying a release

Every release artifact carries a signed [SLSA build provenance](https://slsa.dev/)
attestation generated by GitHub. Verify a downloaded archive against this
repository before trusting it:

```bash
gh attestation verify go-bricks-openapi_0.1.0_darwin_arm64.tar.gz \
  --repo gaborage/go-bricks-openapi
```
````

- [ ] **Step 2: Add a Releasing pointer**

In `README.md`, immediately before the final `## License` section, insert:

```
## Releasing

Versioning and the manual release process are documented in
[RELEASING.md](RELEASING.md).

```

- [ ] **Step 3: Verify the additions landed and links are intact**

Run:

```bash
grep -q "### Verifying a release" README.md \
  && grep -q "gh attestation verify" README.md \
  && grep -q "(RELEASING.md)" README.md \
  && echo "README updated"
```

Expected: `README updated`

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: document go install + release provenance verification

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Add the `RELEASING.md` runbook

**Why:** With manual tagging there's no bot to encode the policy, so the versioning rules, commit-hygiene expectations, and the cut-a-release steps live in a runbook. (Filename stays `RELEASING.md` — an all-caps doc convention with no separators, so the snake_case-for-authored-files preference doesn't change it.)

**Files:**
- Create: `RELEASING.md`

- [ ] **Step 1: Create the runbook**

Create `RELEASING.md` with exactly this content (shown inside a four-backtick fence so the inner code blocks are literal):

````
# Releasing

`go-bricks-openapi` follows [Semantic Versioning](https://semver.org/) and is
currently in its `0.x` series. Releases are cut **manually** by pushing an
annotated tag; the `Release` workflow does the rest.

## Versioning policy

While in `0.x`, the public contract may change between minor versions. The
contract SemVer governs is three surfaces:

1. **CLI surface** — command names, flag names/semantics, defaults, exit codes.
2. **Generated-output shape** — the structure of the emitted OpenAPI document.
3. **Doctor / validation behavior** — the GoBricks version floor and what
   passes vs. fails.

Bump rules while in `0.x`:

| Change in this release            | New version |
|-----------------------------------|-------------|
| New feature, or a breaking change | `0.MINOR.0` |
| Bug fixes only                    | `0.x.PATCH` |

Graduating to `v1.0.0` is a deliberate decision — made when we are willing to
promise backward compatibility on the three surfaces above — not an automatic
milestone.

## Commit hygiene (feeds the changelog)

Release notes are generated from commit subjects, so they must follow
[Conventional Commits](https://www.conventionalcommits.org/). `feat:` and
`fix:` (and similar) appear in the notes; `docs:`, `test:`, `chore:`, `ci:`,
and merge commits are filtered out. There is no committed `CHANGELOG.md` — the
GitHub Releases page is the changelog.

## Cutting a release

1. Make sure `main` is green on CI, including the **GoReleaser config check** job.
2. Choose the next version with the bump rules above. Review the commits since
   the previous tag:

   ```bash
   git log "$(git describe --tags --abbrev=0)"..HEAD --oneline
   ```

3. *(Optional)* Dry-run locally to inspect artifacts and generated notes:

   ```bash
   goreleaser release --snapshot --clean
   ```

4. Tag and push an annotated, `v`-prefixed tag:

   ```bash
   git checkout main && git pull
   git tag -a v0.1.0 -m "v0.1.0"
   git push origin v0.1.0
   ```

5. The `Release` workflow runs GoReleaser, publishes the GitHub Release with
   generated notes and checksummed binaries, and attaches a signed SLSA build
   provenance attestation.

### Pre-releases

To validate a release before finalizing, tag a release candidate such as
`v0.1.0-rc.1`. GoReleaser marks any tag containing `-` as a GitHub pre-release
(`prerelease: auto`); promote it by tagging the final `v0.1.0` once you're
satisfied.

## Verifying a release

```bash
# Honest version from a go install build (no ldflags needed)
go install github.com/gaborage/go-bricks-openapi/cmd/go-bricks-openapi@v0.1.0
go-bricks-openapi version            # -> go-bricks-openapi version v0.1.0

# Provenance of a downloaded archive
gh attestation verify go-bricks-openapi_0.1.0_darwin_arm64.tar.gz \
  --repo gaborage/go-bricks-openapi
```
````

- [ ] **Step 2: Verify the file is present and well-formed**

Run:

```bash
test -f RELEASING.md \
  && grep -q "## Cutting a release" RELEASING.md \
  && grep -q "0.MINOR.0" RELEASING.md \
  && grep -q "prerelease: auto" RELEASING.md \
  && echo "RELEASING.md OK"
```

Expected: `RELEASING.md OK`

- [ ] **Step 3: Commit**

```bash
git add RELEASING.md
git commit -m "docs: add RELEASING runbook (versioning policy + cut steps)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Final verification (whole branch)

- [ ] **Run the pre-commit gate**

Run: `make check`
Expected: `fmt` + `lint` + `test` (race) + `validate-cli` all pass.

- [ ] **Confirm the GoReleaser config is still valid**

Run: `goreleaser check`
Expected: `1 configuration file(s) validated` with no errors. (Mirrors the CI `release-check` job; `.goreleaser.yaml` was not modified, so this should pass unchanged.)

---

## Release (maintainer, post-merge — NOT a coding task)

After this branch merges to `main` and CI is green, the maintainer cuts the
first release by hand (this is the runbook in `RELEASING.md`, not work for the
implementing agent):

- [ ] Tag `v0.1.0`: `git checkout main && git pull && git tag -a v0.1.0 -m "v0.1.0" && git push origin v0.1.0`
- [ ] Confirm the `Release` workflow succeeds (GoReleaser + provenance step).
- [ ] `go install …/cmd/go-bricks-openapi@v0.1.0 && go-bricks-openapi version` prints `v0.1.0`.
- [ ] `gh attestation verify <downloaded-archive> --repo gaborage/go-bricks-openapi` succeeds.
- [ ] The GitHub Releases page shows categorized notes and checksummed assets.
