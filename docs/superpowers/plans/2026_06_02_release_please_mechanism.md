# Release-please + Signed-tag Mechanism — Implementation Plan

**Spec:** [`../specs/2026_06_02_release_please_mechanism_design.md`](../specs/2026_06_02_release_please_mechanism_design.md)
**Date:** 2026-06-02

## Tasks

1. **release-please config** — `release-please-config.json` (go release-type, skip-github-release,
   draft PR, pre-major bump rules, Keep-a-Changelog sections) + `.release-please-manifest.json`
   seeded to `0.1.0`. ✅
2. **CHANGELOG.md** — seed a `## [0.1.0]` section so release-please appends above it and
   `release.yml` can extract notes. ✅
3. **`.github/allowed_signers`** — maintainer SSH key (CI trust root). ✅
4. **`scripts/release.sh`** — single-module port: version reconciliation, CI-green check vs
   `ci.yml`, local gate (`make check`/`vuln`/`sec`), signing probe, signed tag, local verify,
   push. `chmod +x`. ✅
5. **`.github/workflows/release-please.yml`** — pinned action SHA, `RELEASE_PLEASE_TOKEN`. ✅
6. **`.github/workflows/release.yml`** — `verify` + `publish`; signed-tag gate vs
   `allowed_signers@main`; GoReleaser `--release-notes`; SLSA kept; clear `autorelease:
   pending`. ✅
7. **`.goreleaser.yaml`** — `changelog: { disable: true }`. ✅
8. **`Makefile`** — `vuln`, `sec`, `release` targets; pinned `GOVULNCHECK_VERSION` /
   `GOSEC_VERSION`; `release` guard via `$(origin VERSION)`. ✅
9. **`RELEASING.md`** — rewrite to the go-bricks runbook (keep binaries + SLSA). ✅
10. **Docs** — this spec + plan; supersession note on the 2026-05-31 doc. ✅

## Verification gate (before PR)

- [ ] `goreleaser check` and `goreleaser release --snapshot --clean`
- [ ] `make vuln`, `make sec`, `make check`
- [ ] `bash -n scripts/release.sh`; workflow lint (actionlint if available)
- [ ] `jq . release-please-config.json .release-please-manifest.json`
- [ ] Adversarial review of the signed-tag gate + release-please config + shell robustness

## Post-merge (maintainer, out of PR scope)

- [ ] Create `RELEASE_PLEASE_TOKEN` secret.
- [ ] Confirm "Default to PR title for squash merge commits" is ON.
- [ ] (Optional) `v*` tag-protection ruleset.
- [ ] Merge the first standing release-please PR, then `make release VERSION=…` to exercise
      the flow end-to-end.
