# Releasing go-bricks-openapi

`go-bricks-openapi` is released with an **automated-calculation, scripted-signed-publish**
flow modeled on [go-bricks](https://github.com/gaborage/go-bricks). `release-please`
computes the version and writes the changelog; **you** merge its Release PR and cut a
**signed** tag locally; CI re-verifies, then GoReleaser publishes cross-platform binaries
with a SLSA build-provenance attestation.

- **Version source of truth:** `.release-please-manifest.json` (bot-maintained). The
  **git tag** is the release boundary you sign. There is no hand-maintained VERSION.
- **Single module:** the root module is the only release-please-managed package, tagged
  `vX.Y.Z`.
- **Pre-1.0 SemVer:** `feat` → MINOR, `fix` → PATCH, breaking is **capped to MINOR** while
  `0.x` (project convention; SemVer §4 permits anything pre-1.0). Mark breaking changes with
  `feat!:` / `BREAKING CHANGE:` so the bump + the ⚠ banner are correct.

## Versioning policy

While in `0.x`, the public contract may change between minor versions. The contract SemVer
governs is three surfaces:

1. **CLI surface** — command names, flag names/semantics, defaults, exit codes.
2. **Generated-output shape** — the structure of the emitted OpenAPI document.
3. **Doctor / validation behavior** — the GoBricks version floor and what passes vs. fails.

| Change in this release            | New version |
|-----------------------------------|-------------|
| New feature, or a breaking change | `0.MINOR.0` |
| Bug fixes only                    | `0.x.PATCH` |

Graduating to `v1.0.0` is a deliberate decision — made when we are willing to promise
backward compatibility on the three surfaces above — not an automatic milestone.

## 0. One-time setup

- **`RELEASE_PLEASE_TOKEN`** — a fine-grained PAT (repo `gaborage/go-bricks-openapi`,
  permissions **Contents: Read and write** + **Pull requests: Read and write**), stored as a
  repo secret. release-please uses it so its Release PR triggers CI and its label edits are
  reliable — the default `GITHUB_TOKEN` does neither. Create it once:
  `gh secret set RELEASE_PLEASE_TOKEN --repo gaborage/go-bricks-openapi`.
- **Squash setting** — live value is `squash_merge_commit_title: COMMIT_OR_PR_TITLE` (verified:
  `gh api repos/gaborage/go-bricks-openapi --jq .squash_merge_commit_title`), not `PR_TITLE`.
  Under `COMMIT_OR_PR_TITLE`, a **one-commit PR uses that commit's own subject** as the squash
  message, and only a **multi-commit PR** falls back to the PR title. Consequence: on a
  single-commit PR — the common case here — the **commit** subject, not the PR title, is what
  release-please parses, so that subject must be Conventional Commit form. This repo is already
  squash-only.
- **Tag protection (recommended — not currently configured)** — GitHub runs `release.yml` from
  the **pushed tag's own tree**, so the in-job checks live inside a file the tag author controls;
  that threat is real. Today, those in-job checks — annotated-tag, ancestor-of-`main`, and
  SSH-signature verification against `main`'s `allowed_signers` — are the **only** control on the
  publish path; no `v*` tag-protection ruleset exists (verified: `gh api
  'repos/gaborage/go-bricks-openapi/rulesets?includes_parents=true'` returns one ruleset, target
  `branch`, not `tag`). A `v*` tag-protection ruleset restricting tag creation is **recommended
  but not currently configured**. It would **not** stop the maintainer or a credential acting as
  them — GitHub exposes no separate tag-push permission, so anyone with repo write access can
  already create `v*` tags. It **would** stop a non-human `contents: write` principal — e.g. a
  subverted third-party action inside `release-please.yml`, which runs on every push to `main` —
  from creating one directly. Independent of all of the above: a `v*` tag alone, with no workflow
  run at all, already publishes runnable source to `proxy.golang.org` and every `go
  install`/`go get` consumer, immutably. This repo already demonstrated the mechanism: `v0.1.0` —
  an annotated but **unsigned** tag — published six assets, because `release.yml` in that tag's
  own tree (48 lines, a single job, workflow-level `contents`/`id-token`/`attestations: write`)
  carried no annotated/ancestor/signature check at all (`git tag -v v0.1.0` →
  `error: no signature found`).
- **Signature trust root** — `release.yml` verifies a tag's signature against the copy of
  `.github/allowed_signers` **on `main`** and requires the tagged commit to be an ancestor of
  `main`. These in-job annotated/ancestor/signature checks bind a **cooperative** tagger — they
  catch maintainer error (a lightweight tag, an unsigned tag, a tag pointing off `main`) but not a
  malicious tag pusher, because they live in the file the tag author controls. `allowed_signers`
  on `main` is not independently locked down: the repo's one ruleset has a `RepositoryRole` bypass
  actor at `bypass_mode: "always"`, there is **no CODEOWNERS** file, and PR #20 merged with zero
  reviews against a rule that otherwise requires one approval. Whoever holds admin can edit
  `allowed_signers` on `main` without review.
- **Residual risk even with all of the above** — tag-controlled **code**, not just tag-controlled
  workflow YAML, still executes inside the privileged `publish` job: `.goreleaser.yaml` runs
  `before: hooks: [go mod tidy]` and builds `./cmd/go-bricks-openapi` from the tag's own source,
  inside the job holding `contents`/`pull-requests`/`id-token`/`attestations: write`. No GitHub
  Environment gates those secrets (`gh api repos/gaborage/go-bricks-openapi/environments` →
  `total_count: 0`). A `v*` ruleset and a pinned workflow file close the "modified `release.yml`"
  door; they do not close this one.
- **Local SSH signing** — `make release` signs the tag (`git tag -s`). Configure
  `git config gpg.format ssh` + `user.signingkey` with the key listed in
  `.github/allowed_signers` (1Password or `ssh-agent`).

## 1. release-please keeps a standing Release PR

On every push to `main` (and on demand via *Actions → release-please → Run workflow*),
`release-please` opens/updates a `chore(main): release X.Y.Z` PR **as a draft**
(`draft-pull-request: true` in `release-please-config.json`) that computes the next version from
the Conventional-Commit **subjects of the squash commits landed on `main`** — not PR titles — and
writes the `CHANGELOG.md` section + bumps `.release-please-manifest.json`. It does **not** tag or
publish (`skip-github-release: true`).

Release notes are generated from commit subjects, so they must follow
[Conventional Commits](https://www.conventionalcommits.org/). `docs:` / `chore:` / `ci:` /
`test:` / `build:` are hidden from the changelog.

## 2. Cut the release (local)

```bash
# 1. Mark the standing "chore(main): release X.Y.Z" PR ready for review (it opens as a
#    draft) and merge it. This lands CHANGELOG + manifest on main.
git checkout main && git pull
# 2. Immediately cut the signed tag (read the version from the merged PR / manifest):
make release VERSION=v0.2.0
```

`make release` is **not** read-only: its gate (`make check`) runs `fmt` (`go fmt ./...`) first,
which rewrites tracked `.go` files if any weren't already formatted. It asserts `VERSION ==
.release-please-manifest.json == CHANGELOG top section`, runs the full gate (`make check` +
`make vuln` + `make sec`), probes signing, creates a **signed annotated** tag (`git tag -s`),
verifies the signature locally, and pushes the tag. It does **not** edit `CHANGELOG.md`
(release-please owns it) — but if `fmt` rewrites anything else, the gate tripwires on it:
`scripts/release.sh` aborts with "the release gate (make check → fmt) modified tracked files"
rather than silently tagging an unformatted tree.

> **Rule (release-please #1561):** never merge a Release PR you are not ready to `make release`
> immediately. A merged-but-untagged Release PR keeps its `autorelease: pending` label and
> **deadlocks all future Release PRs**. `release.yml` clears that label after publishing.

### Dry run (optional)

```bash
goreleaser release --snapshot --clean   # inspect artifacts locally; publishes nothing
```

### Recovering a missed tag (the #1561 deadlock)

If a Release PR was merged but `make release` never ran, every later `release-please`
run fails red with "merged but not tagged" (and opens no new Release PR) until the
missed version is tagged. `make release` refuses to help once `main` has moved past the
release commit — tagging HEAD would ship undocumented commits — so sign the release
commit itself:

```bash
git checkout main && git pull
REL_COMMIT="$(git log -1 --format=%H -- .release-please-manifest.json)"   # the merged Release PR squash
git log -1 --oneline "$REL_COMMIT"                                        # sanity: 'chore(main): release X.Y.Z'
git tag -s vX.Y.Z -m "Release vX.Y.Z" "$REL_COMMIT"
git -c gpg.ssh.allowedSignersFile=.github/allowed_signers tag -v vX.Y.Z
git push origin vX.Y.Z
```

`release.yml` re-verifies the tagged commit, publishes, and clears the
`autorelease: pending` label; the next push to `main` (or a manual
`release-please` run via *Actions → release-please → Run workflow*) then opens the
Release PR for everything merged since.

## 3. What `release.yml` does (on tag push)

1. **Re-verifies the tagged commit** independently — `make validate-cli` + `make test` +
   `go mod tidy` diff + `make vuln` / `make sec`.
2. Asserts the tag is **annotated**, an **ancestor of `main`**, and **SSH-signature-valid**
   against `.github/allowed_signers` on `main`.
3. Extracts the `CHANGELOG.md` section for the tag as the release notes.
4. Runs **GoReleaser** → cross-platform binaries (linux/darwin/windows × amd64/arm64, minus
   windows/arm64) + `checksums.txt` + the GitHub Release (body = the CHANGELOG section).
5. Attaches a keyless **SLSA build-provenance attestation** over `dist/checksums.txt`.
6. Clears the merged Release PR's `autorelease: pending` label.

CI never signs.

## 4. If the release fails

- **Before the tag is pushed:** fix locally, re-run `make release`.
- **`release.yml` red, tag already pushed:** do NOT reuse the version. Yank the tag (§5), fix
  forward, bump PATCH, re-release.

## 5. Yanking / retracting a bad version

```bash
git tag -d v0.2.0
git push origin :refs/tags/v0.2.0
gh release delete v0.2.0 --yes   # if a release was created
```

A tag delete does NOT recall a version already pulled via `go get`. Ship a follow-up release
adding a `retract` directive to `go.mod`:

```go
retract v0.2.0 // <reason>; use v0.2.1+
```

## 6. Verifying a release

```bash
# Honest version from a go install build (no ldflags needed)
go install github.com/gaborage/go-bricks-openapi/cmd/go-bricks-openapi@v0.2.0
go-bricks-openapi version            # -> go-bricks-openapi version v0.2.0

# Provenance of a downloaded archive
gh attestation verify go-bricks-openapi_0.2.0_darwin_arm64.tar.gz \
  --repo gaborage/go-bricks-openapi
```

## 7. Supply-chain posture

Unlike upstream go-bricks (which ships no binaries — tag-signature provenance only),
`go-bricks-openapi` ships **signed tags + prebuilt binaries + SLSA build provenance**. Still
deferred (revisit on demand): cosign artifact signing, SBOM (CycloneDX), Homebrew tap, Docker
image, GitHub Action wrapper.
