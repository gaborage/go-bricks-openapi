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
[Conventional Commits](https://www.conventionalcommits.org/). Commits appear in
the notes grouped by type, except `docs:`, `test:`, `chore:`, `ci:`, and merge
commits, which are filtered out. There is no committed `CHANGELOG.md` — the
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

4. Tag and push an annotated, `v`-prefixed tag (replace `vX.Y.Z` with the
   chosen version, e.g. `v0.1.0`):

   ```bash
   git checkout main && git pull
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
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
