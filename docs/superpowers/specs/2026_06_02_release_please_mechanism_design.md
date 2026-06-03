# Release-please + Signed-tag Mechanism — Design

**Status:** Approved (design)
**Date:** 2026-06-02
**Supersedes:** the release-trigger and changelog decisions in
[`2026_05_31_release_distribution_policy_design.md`](2026_05_31_release_distribution_policy_design.md)
(§3 manual tagging, §4 no `CHANGELOG.md`, and the §8 deferral of release-please). That doc
deferred release-please + a committed changelog as "revisit at `1.0` or **on demand**"; this is
the on-demand trigger. The **versioning policy** (three contract surfaces, `0.x` bump rules),
**honest version reporting** (`ResolveVersion`), and **SLSA provenance** from that doc are
retained unchanged.

## 1. Goal

Adopt go-bricks' release mechanism in `go-bricks-openapi` — an automated-calculation,
scripted-signed-publish flow — **while keeping** this repo's GoReleaser binaries + SLSA
build provenance (which go-bricks itself lists as deferred). Net result: go-bricks' Phase-1
automation + signed-tag governance layered on top of the Phase-2 artifacts go-bricks lacks.

## 2. The mechanism (target)

1. **`release-please.yml`** (push to `main`) keeps a standing `chore(main): release vX.Y.Z`
   PR that computes the next version from Conventional-Commit subjects, writes the
   `CHANGELOG.md` section, and bumps `.release-please-manifest.json`. `skip-github-release:
   true` — it never tags or publishes.
2. **`make release VERSION=vX.Y.Z`** (`scripts/release.sh`) is read-and-verify-only: asserts
   `VERSION == manifest == CHANGELOG top section`, runs the full local gate
   (`make check` + `make vuln` + `make sec`), checks CI is green for the commit, probes
   signing, creates a **signed annotated** tag, verifies it locally against
   `.github/allowed_signers`, and pushes.
3. **`release.yml`** (tag push `v*`) — a `verify` job re-checks the tagged commit
   (build/test/tidy/vuln/sec); a `publish` job asserts the tag is annotated, an ancestor of
   `main`, and SSH-signature-valid against `allowed_signers` **on `main`**, extracts the
   CHANGELOG section, runs **GoReleaser** (binaries + checksums + Release, body = CHANGELOG
   section via `--release-notes`), attaches the **SLSA attestation**, and clears the
   `autorelease: pending` label (#1561 deadlock breaker).

## 3. Implementation surface

| File | Change |
|---|---|
| `release-please-config.json` | New. `release-type: go`, `skip-github-release: true`, `draft-pull-request: true`, `bump-minor-pre-major: true`, `bump-patch-for-minor-pre-major: false`, Keep-a-Changelog sections. |
| `.release-please-manifest.json` | New. Seeded `{ ".": "0.1.0" }` (current shipped release). |
| `CHANGELOG.md` | New. Seeded `## [0.1.0]` section; release-please appends above it. |
| `.github/allowed_signers` | New. Maintainer SSH key — CI tag-signature trust root. |
| `.github/workflows/release-please.yml` | New. Pinned `release-please-action` SHA + `RELEASE_PLEASE_TOKEN`. |
| `scripts/release.sh` | New. Single-module adaptation of go-bricks' release script (CI check → `ci.yml`). |
| `.github/workflows/release.yml` | Rewritten. `verify` + `publish` jobs; signed-tag gate; GoReleaser w/ `--release-notes`; SLSA kept; clear label. |
| `.goreleaser.yaml` | `changelog: { disable: true }` — notes now come from CHANGELOG.md. |
| `Makefile` | Add `vuln`, `sec`, `release` targets (pinned scanner versions). |
| `RELEASING.md` | Rewritten to the go-bricks runbook (keeps binaries + SLSA). |

## 4. Adaptations from go-bricks

- **Single module** — no `tools/migration` second module / second tag stream.
- **Local gate** — `make check` + `make vuln` + `make sec` (one module).
- **CI-green check** targets `ci.yml` (this repo's workflow), not `ci-v2.yml`.
- **Publish via GoReleaser** (not bare `gh release create`) so binaries + checksums + SLSA
  are produced; the CHANGELOG section is passed with `goreleaser release --release-notes=…`.
- **`make release` guard** uses `$(origin VERSION) == command line` because this Makefile has
  a git-describe `VERSION` default (a `test -n` guard would always pass).

## 5. One-time setup (maintainer)

`RELEASE_PLEASE_TOKEN` secret; "Default to PR title for squash merge commits" ON; optional
`v*` tag-protection ruleset. Local SSH signing already configured.

## 6. Out of scope

- No wholesale CI rewrite to go-bricks' path-filtered `ci-v2.yml` (tailored to its two-module
  layout). The existing `ci.yml` + `release-check` (GoReleaser config check) stay. The 8
  required status checks are unchanged, so the merge ruleset is unaffected.
- No pre-release/RC path — `release.sh` enforces strict `v0.MINOR.PATCH`, matching go-bricks.
- cosign / SBOM / Homebrew / Docker / GitHub Action wrapper remain deferred.

## 7. Verification

- `goreleaser check` + `goreleaser release --snapshot --clean` (config + matrix compile).
- `make vuln`, `make sec`, `make check` green.
- `bash -n scripts/release.sh`; workflow lint.
- Valid JSON for config + manifest.
- The first real release-please PR (after merge) + the next `make release` are the live
  end-to-end proof; CI's `release-check` snapshot de-risks the GoReleaser config on every PR.
