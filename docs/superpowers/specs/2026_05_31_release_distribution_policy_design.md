# Release & Distribution Policy — Design

**Status:** Approved (design)
**Date:** 2026-05-31
**Scope:** Versioning, release, and distribution policy for `go-bricks-openapi`, plus the minimal mechanism changes needed to make the existing pipeline honor it.

## 1. Goal & scope

Define and document the versioning, release, and distribution policy for
`go-bricks-openapi`, and make the existing (scaffolded-but-unproven) release
pipeline actually honor it.

The pipeline is already ~80% built — `.goreleaser.yaml`, a tag-triggered
`.github/workflows/release.yml`, a CI `release-check` snapshot job, and an
ldflags version seam — but **no tag has ever been cut**, so nothing has been
exercised end-to-end and a few correctness gaps are latent.

This work is **policy-first**: the written policy is the primary deliverable;
the implementation is the minimal set of changes required to make that policy
true, and nothing more. The first concrete outcome is a correct, repeatable
`v0.1.0` release.

## 2. Versioning policy

- **SemVer, starting at `v0.1.0`, with a `0.x` runway.** While in `0.x`,
  breaking changes are *allowed* without a major bump.
- **Bump rules inside `0.x`:**
  - `0.MINOR.0` — new features *or* breaking changes.
  - `0.x.PATCH` — bug fixes only.
  - The maintainer decides the bump per tag (see the runbook in §3).
- **The public contract SemVer governs** — three surfaces:
  1. **CLI surface** — command names, flag names/semantics, defaults, exit codes.
  2. **Generated-output shape** — the structure of the emitted OpenAPI document
     (downstream tooling diffs against it).
  3. **Doctor / validation behavior** — the GoBricks version floor and what
     passes vs. fails.
- **Graduation to `v1.0.0`** is an explicit, deliberate decision — made the day
  the maintainer is willing to *promise* backward compatibility on the three
  surfaces above. It is not an automatic milestone.
- **Pre-releases:** to validate a release before finalizing, tag
  `vX.Y.Z-rc.N`. `.goreleaser.yaml`'s `prerelease: auto` already flags any tag
  containing `-` as a GitHub pre-release; promote by tagging the final version.

## 3. Release trigger & process (manual tagging)

No bot, no new dependencies. Releases are cut by hand via an annotated tag; the
existing tag-triggered `release.yml` does the rest. This runbook is documented
in a new `RELEASING.md`:

1. Confirm `main` is green — CI passing, including the `release-check` snapshot
   job.
2. Pick the version per the `0.x` bump rules (§2) by scanning the
   conventional-commit types merged since the last tag.
3. *(Optional)* Local dry-run: `goreleaser release --snapshot --clean` to
   eyeball the artifacts and generated notes.
4. Tag and push:
   `git tag -a vX.Y.Z -m "vX.Y.Z" && git push origin vX.Y.Z`.
5. `release.yml` fires GoReleaser → builds the matrix, generates notes,
   publishes the GitHub Release + provenance attestation.
6. Verify (see §6 / §9).

## 4. Changelog

**Release-notes only.** GoReleaser already generates categorized notes from
conventional commits and posts them to the GitHub Releases page — that page
*is* the changelog. There is **no committed `CHANGELOG.md`**.

Implication: **commit-message hygiene is load-bearing.** The
`feat:` / `fix:` / etc. prefixes feed the release notes, and the existing
GoReleaser filters drop `docs:` / `test:` / `chore:` / `ci:` and merge commits.
`RELEASING.md` documents this expectation.

## 5. Distribution channels (two, baseline)

1. **`go install`** —
   `go install github.com/gaborage/go-bricks-openapi/cmd/go-bricks-openapi@latest`.
   Works the instant `v0.1.0` is tagged. (Today it fails: there is no semver
   tag to resolve.)
2. **Prebuilt + checksummed binaries** on GitHub Releases — linux/darwin/windows
   × amd64/arm64 (minus windows/arm64), already configured in `.goreleaser.yaml`.

**Required mechanism fix — honest version reporting.** Today `main.version` is
only set by ldflags (Makefile / GoReleaser builds), so anyone who `go install`s
a tagged build sees `version dev`. Fix: a small, unit-testable
`resolveVersion(injected)` helper that falls back to
`runtime/debug.ReadBuildInfo().Main.Version` when the ldflag was not applied.

Precedence:
1. ldflag-injected value (GoReleaser / `make build`) — wins when present.
2. `BuildInfo` main module version (`go install …@vX.Y.Z` → the tag, or a
   pseudo-version for `@<commit>`).
3. `dev` fallback (local `go build`, or `BuildInfo` reporting `(devel)`).

Without this fix the `go install` channel — the primary one — misreports its
version.

## 6. Build provenance (GitHub-native)

Add keyless SLSA build provenance to `release.yml`:

- **Permissions:** add `id-token: write` and `attestations: write` (keep the
  existing `contents: write`).
- **Step:** after GoReleaser runs, a single `actions/attest-build-provenance`
  step with `subject-checksums: dist/checksums.txt` — one attestation covering
  every archive and binary listed in the checksums file.
- **No long-lived secrets** — OIDC + Sigstore keyless.
- **Verification:**
  `gh attestation verify <file> --repo gaborage/go-bricks-openapi`. Documented
  in README and `RELEASING.md`.

## 7. Concrete mechanism changes (full implementation surface)

1. **`internal/commands/version.go`** (and/or `cmd/go-bricks-openapi/main.go`) —
   add a `resolveVersion()` helper with the `BuildInfo` fallback described in
   §5, with unit tests in `version_test.go`.
2. **`.github/workflows/release.yml`** — add the two permissions and the
   `actions/attest-build-provenance` step (§6).
3. **`README.md`** — note that `@latest` resolves once `v0.1.0` is tagged; add a
   "Verifying a release" snippet using `gh attestation verify`.
4. **New `RELEASING.md`** — the runbook (§3), the `0.x` bump rules (§2), and the
   commit-hygiene note (§4).
5. **`.goreleaser.yaml`** — no changes required; it already matches this policy.

## 8. Explicitly NOT doing (YAGNI)

Each of the following is a documented "revisit at `1.0` or on demand," not part
of this work:

- release-please / semantic-release (automated version bumping)
- committed `CHANGELOG.md`
- Homebrew tap
- Docker image
- GitHub Action wrapper
- cosign signing
- SBOM (CycloneDX)

## 9. Verification

- **Pre-merge:** the existing `release-check` job already proves the GoReleaser
  config and the full build matrix compile on every PR — no change needed.
- **Unit:** the `resolveVersion` helper is covered by tests across the three
  precedence cases.
- **First-release validation (manual, one-time)** after tagging `v0.1.0`:
  1. `go install …@v0.1.0 && go-bricks-openapi version` prints `v0.1.0`.
  2. A downloaded release archive passes `gh attestation verify`.
  3. The GitHub Releases notes render correctly from the commit history.

## 10. Risks

- **The first tag is the real test.** Provenance and release notes cannot be
  fully exercised until a tag exists. Mitigation: the optional local
  `--snapshot` dry-run plus the always-on `release-check` job de-risk most of
  it; the attestation step is the one piece only provable on a real tag.
- **Commit hygiene becomes load-bearing** for changelog quality — mitigated by
  the guidance in `RELEASING.md`.
