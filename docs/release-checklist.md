# Release checklist

This checklist covers the v1.x release process for GitHub server archives and the
VS Code extension. Homebrew and Mason publishing are deferred to post-v1.
**v1.8.1 was released on 2026-11-22 as a GitHub-only release.** Marketplace and
Open VSX publication is parked in the [post-v1 backlog](post-v1-backlog.md); it
is not a release gate for M24.

## v1.8.1 release record

- Tag: `v1.8.1`
- Commit: `a3bd9929f9bc80cacd7615c205b6a64d058b590d`
- Release: https://github.com/asimmons91/pico-lsp/releases/tag/v1.8.1
- Release workflow: https://github.com/asimmons91/pico-lsp/actions/runs/45039342788
- Assets: Linux, macOS, and Windows archives for amd64 and arm64,
  `checksums.txt`, `pico-lsp-vscode-1.8.1.vsix`, and its `.sha256`
- Registry jobs: skipped because `PUBLISH_REGISTRIES` is unset
- `go install ...@v1.8.1` verified: reports `v1.8.1`

## M24 docs/onboarding release

M24 is the end-user documentation and onboarding milestone. It may ship as a
patch when the docs and VSIX are ready, or its docs may land without a tag if the
packaged extension does not change. Record which path was taken. See
[M24](M24-end-user-onboarding.md).

- [ ] `docs/getting-started.md` and `examples/hello.p8` exist and are linked from
      the root README.
- [ ] `docs/pico8-workflow.md`, `docs/types.md`, `docs/cli.md`,
      `docs/migrating.md`, and `docs/editors.md` exist and are linked from the
      relevant README sections.
- [ ] Root README, install, troubleshooting, VS Code product page, `docs/M6`,
      `docs/M15`, `docs/M21`, and `AGENTS.md` no longer promise a Marketplace or
      Open VSX launch.
- [ ] `.github/ISSUE_TEMPLATE/`, `SECURITY.md`, and `CONTRIBUTING.md` exist.
- [ ] GitHub description, topics, homepage, and social preview are set.
- [ ] `pico8.nvim` has a tagged release and the root README uses the tagged
      install path.
- [ ] The documentation link check passes in CI.
- [ ] A real screenshot or recording from the manual verification is in the root
      README and `editors/vscode/README.md`.
- [ ] The manual VS Code verification below is recorded for the available
      platforms, or unverified checks are recorded as open.
- [ ] If the VSIX changes, the version bump, changelogs, tag, and assets match;
      otherwise record why no tag was cut.

## Pre-release

- [x] `main` is green on Linux, macOS, and Windows CI.
- [x] `go test -race ./...` passes locally or on Linux CI.
- [x] `gofmt -l .` is empty.
- [x] `go vet ./...` passes.
- [x] `golangci-lint run ./...` passes with the pinned v2.15.0 release.
- [x] `govulncheck ./...` passes.
- [x] The full corpus tests pass without panics.
- [x] Performance targets in the roadmap are met
      (`PICO_LSP_PERF=1 go test -count=1 -p 1 -run Performance ./...`).
- [x] `goreleaser check` validates `.goreleaser.yaml`.
- [x] `goreleaser release --snapshot --clean` builds archives and checksums.
- [x] `actionlint` validates both GitHub Actions workflows.
- [x] `pico-lsp --version` reports the intended release tag in the stamped test
      build.
- [x] README, install, configuration, diagnostics, and troubleshooting docs are
      accurate for the GitHub-only release.
- [x] Known limitations are documented: local-desktop-only VS Code and Open VSX
      editors, deferred registry listing, Zed, Homebrew/Mason deferral, unsigned
      binaries, and the README limitations.
- [x] `CHANGELOG.md` has a dated `1.8.1` section.
- [x] `LICENSE` and the PICO-8 trademark disclaimer are present.
- [ ] The test cart `testdata/smoke.p8` opens and attaches in Neovim.
- [x] The optional Neovim helper attaches included `.lua` buffers and ignores
      unrelated Lua buffers (`editors/nvim/tests/run.sh` passes).
- [x] Helix behavior did not change; the v1.0.0 manual feature matrix remains the
      last manual run. Re-run it if Helix behavior changes.
- [x] No stdout writes occur outside LSP framing. No server code changed.
- [x] No telemetry or network calls are added. No server or extension feature
      code changed.

## VS Code package

- [x] `editors/vscode/package.json` version matches the release tag.
- [x] The display name is **PICO-8 LSP**, the publisher is `asimmons91`, and the
      extension ID is `asimmons91.pico-lsp`.
- [x] The listing icon is original artwork, not the official Lexaloffle logo.
      `editors/vscode/icons/README.md` records the artwork's license.
- [x] `editors/vscode/README.md` uses absolute `https://` links and image URLs,
      and its primary install path is the v1.8.1 VSIX.
- [x] The README states local desktop VS Code and Open VSX-compatible editors
      only, no Go requirement, no telemetry, no cart execution, and the offline
      `pico8.serverPath` path.
- [x] The README includes the AI-assisted development disclosure and the
      trademark notice.
- [x] `npm ci`, `npm run lint`, `npm run typecheck`, `npm run test:unit`, and
      `npm run test:integration` pass.
- [x] `npm run package` builds the VSIX at the repository root.
- [x] `vsce ls` shows `dist/extension.js`, the grammar, language configuration,
      README, CHANGELOG, LICENSE, and `icons/pico-lsp.png`.
- [x] `vsce ls` shows no `src/`, `test/`, `scripts/`, source maps, `images/`,
      `icons/README.md`, or `icons/pico8.png`.
- [x] CI and the release workflow run the same VSIX content assertions.
- [x] The VSIX and `.vsix.sha256` are attached to the GitHub release.

## Tag and publish v1.8.1 (GitHub-only)

- [x] Merge the release pull request into `main`.
- [x] Verify the merge commit is green in CI.
- [x] Confirm the `PUBLISH_REGISTRIES` repository variable is unset or not
      `true`, so the registry jobs are skipped.
- [x] Create the annotated tag:

  ```sh
  git tag -a v1.8.1 -m "pico-lsp v1.8.1"
  git push origin v1.8.1
  ```

- [x] Watch the `Release` workflow complete.
- [x] Confirm the GitHub release contains Linux, macOS, and Windows archives for
      amd64 and arm64 plus `checksums.txt`.
- [x] Confirm the release contains `pico-lsp-vscode-1.8.1.vsix` and
      `pico-lsp-vscode-1.8.1.vsix.sha256`.
- [x] Download and run `pico-lsp --version` from at least one release archive.
- [x] Verify the archive checksum and the VSIX checksum.
- [x] Confirm `publish-marketplace` and `publish-openvsx` are skipped.
- [x] Confirm the release notes describe v1.8.1 as GitHub-only and point to the
      VSIX.
- [ ] Install the release VSIX and run the manual verification below.

## Parked registry publication

Not part of M24. Do these steps only after the publisher accounts and tokens are
available; keep the GitHub VSIX as the supported channel until both listings are
live. Record the launch in the backlog, not as an M24 exit criterion.

- [ ] Claim and verify the `asimmons91` publisher in both the VS Code Marketplace
      and Open VSX.
- [ ] Add the `VSCE_PAT` and `OVSX_PAT` repository secrets.
- [ ] Restore the registry-first install order in `editors/vscode/README.md` only
      after both listings are live.
- [ ] Set `PUBLISH_REGISTRIES=true`.
- [ ] Tag the registry patch after the release PR merges; extension `X.Y.Z` must
      download server tag `vX.Y.Z`.
- [ ] Confirm `publish-marketplace` and `publish-openvsx` succeed.
- [ ] Verify both listings show the correct version, README, images, icon,
      license, changelog, repository links, categories, and keywords.
- [ ] Install from both registries and record the results.
- [ ] Update the backlog item and `AGENTS.md`.

## Registry automation

- Registry publishing is gated by the `PUBLISH_REGISTRIES` repository variable.
  When it is unset or not `true`, both publish jobs are skipped. This is the
  current GitHub-only path.
- `VSCE_PAT` and `OVSX_PAT` are repository secrets. Do not commit or print them.
- `VSCE_PAT` is an Azure DevOps personal access token with the Marketplace
  **Manage** scope and access to the `asimmons91` publisher.
- `OVSX_PAT` is an Open VSX access token for the `asimmons91` namespace.
- The `publish-marketplace` and `publish-openvsx` jobs are independent. Re-run
  only the failed job from the workflow run; do not republish the other registry.
- The jobs validate that the extension manifest version matches the tag, verify
  the VSIX checksum from the CI artifact, and publish that exact VSIX. The
  release job also re-runs the packaged-file assertions before uploading the
  artifact.
- The jobs fail clearly when the token is missing or rejected. Add or rotate the
  secret, set `PUBLISH_REGISTRIES=true`, and re-run the failed job.
- Manual fallback, using the verified VSIX downloaded from the GitHub release:

  ```bash
  set -euo pipefail
  version=1.8.1
  sha256sum -c "pico-lsp-vscode-${version}.vsix.sha256"
  # On macOS without sha256sum:
  # shasum -a 256 -c "pico-lsp-vscode-${version}.vsix.sha256"

  export VSCE_PAT='<marketplace-pat>'
  npx --yes @vscode/vsce@4.1.1 publish \
    --packagePath "pico-lsp-vscode-${version}.vsix" \
    --pat "$VSCE_PAT" \
    --skip-duplicate

  export OVSX_PAT='<open-vsx-pat>'
  npx --yes ovsx@1.4.0 publish \
    "pico-lsp-vscode-${version}.vsix" \
    --pat "$OVSX_PAT" \
    --skip-duplicate
  ```

- Exporting the tokens before the `npx` commands is required. A same-line
  assignment such as `VSCE_PAT=... npx ... --pat "$VSCE_PAT"` expands the
  variable before the assignment takes effect and passes an empty token.
- `--skip-duplicate` makes a retry safe when the same version was already
  published. It does not replace verifying that the published version is the
  intended VSIX; check the listing version and contents after a retry.
- The `vscode-vsix` artifact is retained for 90 days. If a delayed retry finds
  it expired, use the manual fallback above with the VSIX and checksum from the
  GitHub release.
- Registry processing can lag. Verify the version in both registries before
  announcing the registry patch. Until both are live, the release notes and
  README must name only the registry that is actually available and point to the
  VSIX for the rest.
- Roll back a bad extension by unpublishing the affected Marketplace version
  where possible, following Open VSX's registry policy, and publishing a patch
  release. Never reuse a version number.

## Post-release

- [x] Verify `go install github.com/asimmons91/pico-lsp/cmd/pico-lsp@v1.8.1`
      builds for a developer and reports `v1.8.1`.
- [ ] Update any documentation that needs the released version.
- [ ] Open post-v1 tracking issues for Homebrew, Mason, the parked registry
      launch, Zed, and API version selection as needed.

## Manual VS Code verification

Run these checks on the maintainer's machine with a real VS Code installation.
Do not mark them complete from CI, a headless shell, or a different editor.
Record one row per tested platform in the table below.

| Date | Tester | OS and version | VS Code version | Extension version and source | Server version and source | Result | Notes or screenshot |
|---|---|---|---|---|---|---|---|
| Not run | | | | | | Pending | |

- [ ] **VSIX install:** install `pico-lsp-vscode-1.8.1.vsix` from the GitHub
      release; the checksum matches and the extension activates on a `.p8`.
- [ ] **First-run download:** with a clean cache, opening a `.p8` downloads,
      verifies, and starts the matching server.
- [ ] **Offline/custom server:** `pico8.serverPath` with downloads disabled makes
      no network request and all features work.
- [ ] **Cart features:** `testdata/smoke.p8` shows diagnostics, completion,
      hover, symbols, folding, navigation, rename, semantic tokens, formatting,
      inlay hints, code lens, and code actions.
- [ ] **Glyph picker:** **PICO-8: Insert P8SCII Glyph**, typing `p8_left`, and
      completion in code and strings work.
- [ ] **Status bar:** token, character, and compressed budgets update while
      editing.
- [ ] **Settings schema:** searching `pico8` shows descriptions and defaults;
      `strict` is accepted and an invalid enum value is flagged.
- [ ] **Version mismatch:** pointing `pico8.serverPath` at a different release
      server shows one advisory warning with the documented actions.
- [ ] **Copy diagnostics:** **PICO-8: Copy Server Diagnostics** includes
      versions, source, path, platform, cache root, and the last error, with no
      cart source.
- [ ] **Upgrade:** installing the previous extension version and then the new
      VSIX updates the extension and downloads the matching server.
- [ ] **Uninstall:** uninstalling and clearing the documented cache path removes
      the extension and cached server as documented.
- [ ] **VSIX contents:** `vsce ls` and a package inspection show no source,
      tests, scripts, images, or internal notes.
- [ ] **Remote/web limitation:** the listing and docs clearly state that remote,
      web, and container hosts are unsupported.

The Marketplace clean-install, Open VSX install, and registry listing-render
checks are part of the parked registry publication item, not M24.

Record any unverified platform or check as open rather than implying it passed.

## Rollback

If a release is broken:

1. Mark the GitHub release as a pre-release or delete it.
2. Do not force-move the tag; publish a patch release instead.
3. For a bad registry patch, unpublish the affected Marketplace version where
   possible and follow Open VSX's registry policy; never reuse a version number.
4. Document the issue in the changelog and release notes.
