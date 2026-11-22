# Release checklist

This checklist covers the v1.x release process for GitHub server archives and the
VS Code extension published to the VS Code Marketplace and Open VSX. Homebrew
and Mason publishing are deferred to post-v1. This checklist prepares
**v1.8.1** (M24).

## Pre-release

- [ ] `main` is green on Linux, macOS, and Windows CI.
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
      accurate.
- [x] Known limitations are documented: local-desktop-only VS Code and Open VSX
      editors, Zed, Homebrew/Mason deferral, unsigned binaries, and the README
      limitations.
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

## VS Code listing and package

- [x] `editors/vscode/package.json` version matches the release tag.
- [x] The display name is **PICO-8 LSP**, the publisher is `asimmons91`, and the
      extension ID is `asimmons91.pico-lsp`.
- [ ] The `asimmons91` publisher name is claimed and verified in both the VS
      Code Marketplace and Open VSX before publishing.
- [x] The listing icon is original artwork, not the official Lexaloffle logo.
      `editors/vscode/icons/README.md` records the artwork's license.
- [x] `editors/vscode/README.md` uses absolute `https://` links and image URLs.
- [ ] The rendered listing has no broken links or images. Verify both registry
      pages after publication.
- [x] The listing states local desktop VS Code and Open VSX-compatible editors
      only, no Go requirement, no telemetry, no cart execution, and the offline
      `pico8.serverPath` path.
- [x] The listing includes the AI-assisted development disclosure and the
      trademark notice.
- [ ] A real screenshot or short recording from the manual verification below is
      added near the top of the listing before publication. The committed banner
      is illustrative artwork, not a screenshot.
- [x] `npm ci`, `npm run lint`, `npm run typecheck`, `npm run test:unit`, and
      `npm run test:integration` pass.
- [x] `npm run package` builds the VSIX at the repository root.
- [x] `vsce ls` shows `dist/extension.js`, the grammar, language configuration,
      README, CHANGELOG, LICENSE, and `icons/pico-lsp.png`.
- [x] `vsce ls` shows no `src/`, `test/`, `scripts/`, source maps, `images/`, or
      `icons/README.md`.
- [ ] The VSIX and `.vsix.sha256` are attached to the GitHub release.

## Tag and publish

- [ ] Merge the release pull request into `main`.
- [ ] Verify the merge commit is green in CI.
- [ ] Create the annotated tag:

  ```sh
  git tag -a v1.8.1 -m "pico-lsp v1.8.1"
  git push origin v1.8.1
  ```

- [ ] Watch the `Release` workflow complete.
- [ ] Confirm the GitHub release contains Linux, macOS, and Windows archives for
      amd64 and arm64 plus `checksums.txt`.
- [ ] Download and run `pico-lsp --version` from at least one release archive.
- [ ] Verify the checksum for the downloaded archive.
- [ ] Confirm the release notes contain install commands and the Go version and
      commit.
- [ ] Confirm the `publish-marketplace` and `publish-openvsx` jobs succeeded.
      They run only for tags, after the server release and VSIX verification.
- [ ] Confirm both registries report version `1.8.1` for
      `asimmons91.pico-lsp`.
- [ ] Confirm the Marketplace and Open VSX listings render the README, banner,
      icon, license, changelog, repository links, categories, and keywords.
- [ ] Install `asimmons91.pico-lsp` from each registry and run the manual
      verification below.
- [ ] Publish and verify the server release **before** the extension. Extension
      `1.8.1` downloads server tag `v1.8.1`.

## Registry automation

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
  secret and re-run the failed job.
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
  announcing the release. Until both are live, the release notes and README must
  name only the registry that is actually available and point to the VSIX for the
  rest.
- Roll back a bad extension by unpublishing the affected Marketplace version
  where possible, following Open VSX's registry policy, and publishing a patch
  release. Never reuse a version number.

## Post-release

- [ ] Verify `go install github.com/asimmons91/pico-lsp/cmd/pico-lsp@v1.8.1`
      builds for a developer.
- [ ] Update any documentation that needs the released version.
- [ ] Open post-v1 tracking issues for Homebrew, Mason, Zed, and API version
      selection as needed.

## Manual VS Code verification

Run these checks on the maintainer's machine with a real VS Code installation.
Do not mark them complete from CI, a headless shell, or a different editor.
Record one row per tested platform in the table below.

| Date | Tester | OS and version | VS Code version | Extension version and source | Server version and source | Result | Notes or screenshot |
|---|---|---|---|---|---|---|---|
| Not run | | | | | | Pending | |

- [ ] **Marketplace clean install:** Extensions view and
      `code --install-extension asimmons91.pico-lsp` both install the extension,
      and it activates on a `.p8`.
- [ ] **Open VSX install:** VSCodium or another Open VSX editor installs the
      same extension ID and activates it.
- [ ] **VSIX fallback:** the GitHub release VSIX installs and its checksum
      matches.
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
- [ ] **Upgrade:** installing the previous extension version and then the
      registry build updates the extension and downloads the matching server.
- [ ] **Uninstall:** uninstalling and clearing the documented cache path removes
      the extension and cached server as documented.
- [ ] **Listing render:** both registry pages show the correct README, images,
      links, icon, license, changelog, categories, and keywords.
- [ ] **VSIX contents:** `vsce ls` and a package inspection show no source,
      tests, scripts, images, or internal notes.
- [ ] **Remote/web limitation:** the listing and docs clearly state that remote,
      web, and container hosts are unsupported.

Record any unverified platform or check as open rather than implying it passed.

## Rollback

If a release is broken:

1. Mark the GitHub release as a pre-release or delete it.
2. Do not force-move the tag; publish a patch release instead.
3. Unpublish the affected Marketplace version where possible and follow Open
   VSX's registry policy; never reuse a version number.
4. Document the issue in the changelog and release notes.
