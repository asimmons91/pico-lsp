# M24 — VS Code Marketplace Launch and Documentation Cleanup

**Status:** Proposed — specification only. Implementation, registry publishing,
and manual VS Code verification are pending.

**Depends on:** M15 (VS Code client), M21 (settings schema), M22 (server health)

**Release target:** Next patch or minor after v1.8.0. The exact tag is an open
decision; coordinated `v1.8.1` is recommended.

**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

The official VS Code extension is discoverable and installable from the VS Code
Marketplace and Open VSX, with a listing that accurately explains installation,
requirements, privacy, and limitations. Every public document stops describing
registry publishing as future work. The release process publishes the same
CI-built VSIX that passed the existing extension checks, after the matching
server release is available, with a documented manual fallback. A maintainer
records the desktop VS Code verification on a machine with a real VS Code
installation.

No LSP protocol, server behavior, type analysis, formatting, or extension
feature changes are in scope.

**Demo:** On a clean VS Code 1.95+ install, search for “PICO-8”, install
`asimmons91.pico-lsp`, and open `testdata/smoke.p8`. The extension downloads the
matching `pico-lsp` server, attaches to the cart, and provides diagnostics,
completion, the cart-budget status bar, and the glyph picker without requiring
Go or a manual server install. The same VSIX installs from Open VSX in a
compatible editor, and from the GitHub release for offline or pinned use. An
upgrade replaces the extension and downloads the matching server; the
uninstall path removes the extension and explains how to clear the cached
server.

## Decisions proposed

| ID | Decision | Choice |
|---|---|---|
| D1 | Milestone identity | M24, **Proposed**. |
| D2 | Listing source | `editors/vscode/README.md` is the Marketplace/Open VSX listing. The root `README.md` remains the repository landing page and is updated separately. |
| D3 | Registries | Publish to both the VS Code Marketplace and Open VSX. The publishers must be independently retryable. If Open VSX is delayed, its docs and listing claims must wait until it is actually live. |
| D4 | Release version | **Open.** Recommended: a coordinated patch release (`v1.8.1`) containing the server tag and rebuilt extension. A minor (`v1.9.0`) is acceptable if the launch is treated as a feature release. Do not republish the existing 1.8.0 artifact. |
| D5 | Release order | Publish and verify the server release first, then the extension. Extension version `X.Y.Z` downloads server tag `vX.Y.Z`. |
| D6 | Publication automation | Add Marketplace and Open VSX publishers to the tag-only release workflow, backed by repository secrets and a documented manual fallback. |
| D7 | Icon and trademark | **Open.** Recommended: replace the official PICO-8 logo with original artwork for the listing/icon, or obtain documented permission from Lexaloffle. Do not publish the official logo while `editors/vscode/icons/README.md` says permission is unconfirmed. |
| D8 | Publisher identity | Keep `asimmons91.pico-lsp` as the extension ID. Confirm that `asimmons91` is available and verified in both registries before writing listing links or badges. |
| D9 | Support scope | Local desktop VS Code only. Remote SSH, WSL, Dev Containers, Codespaces, and web extension hosts remain unsupported and must be prominent in the listing. |
| D10 | Verification environment | Build, lint, unit, integration, and packaging checks run in CI. Registry installation and desktop behavior are verified on the maintainer's machine with a real VS Code installation and recorded in the release checklist. |
| D11 | No behavior changes | Do not change LSP responses, server defaults, extension feature behavior, settings, or the download/checksum contract. |
| D12 | Version-pinned docs | Update the version pins in the root README and install docs as part of the release. Do not leave examples pointing at the previous tag. |
| D13 | Listing disclosure | Include a short AI-assisted-development note and a link to the project README, matching the root README's existing disclosure. |
| D14 | VSIX fallback | Keep the GitHub release VSIX and `.vsix.sha256` as the offline, pinned, and registry-lag fallback. |

### Open decisions to confirm before implementation

1. **Release tag:** `v1.8.1` (recommended) or `v1.9.0`.
2. **Icon:** original artwork (recommended) or documented permission to use the
   official PICO-8 logo.
3. **Registry sequence:** publish both together, or publish Marketplace first
   and keep Open VSX out of the docs until it is live.
4. **Listing name:** `PICO-8` or a more descriptive name such as
   `PICO-8 Language Support`.

Record the resolved choices here and in `docs/ROADMAP.md` before the release PR.

## Scope

### In scope

- Rewriting `editors/vscode/README.md` as the Marketplace/Open VSX product page.
- Updating the root README's VS Code install section and known limitations.
- Updating install, troubleshooting, configuration-link, and release-checklist
  documentation for registry installs, upgrades, uninstalls, and offline use.
- Listing metadata in `editors/vscode/package.json`: icon, repository links,
  homepage/bugs, categories, keywords, and any optional gallery banner.
- Listing screenshots or a short recording with absolute image URLs and alt
  text.
- Resolving the icon/trademark question and updating the trademark disclaimer.
- Excluding internal packaging notes from the VSIX.
- Version bump, changelog entries, release notes, and version-pinned docs.
- Marketplace and Open VSX publication from the release workflow, with
  independently retryable jobs and a manual fallback.
- Manual install, upgrade, uninstall, first-run download, offline, and feature
  verification on the VS Code machine.
- Roadmap, backlog, and `AGENTS.md` status updates.

### Out of scope

- Server, parser, analysis, formatter, LSP, or extension feature changes.
- New settings, commands, requests, diagnostics, or code actions.
- Remote extension hosts, web extension hosts, or Codespaces support.
- Bundling platform server binaries inside the VSIX.
- Auto-updating the server independently of the extension.
- Homebrew and Mason packaging.
- Zed or other editor clients.
- Marketplace analytics, paid listings, localization, or A/B testing.
- Republishing or silently replacing the existing 1.8.0 GitHub release asset.
- A general visual redesign of the extension.

## Deliverables

1. `docs/M24-vscode-marketplace.md` — this specification.
2. `editors/vscode/README.md` — complete Marketplace/Open VSX listing copy.
3. `README.md` — VS Code section, install-channel wording, limitations, and
   version pins.
4. `docs/install.md` — registry and VSIX install, upgrade, uninstall, cache,
   and offline instructions.
5. `docs/troubleshooting.md` — registry, proxy, Open VSX lag, upgrade, and
   version-mismatch guidance.
6. `docs/release-checklist.md` — listing checks, publish order, secrets,
   rollback, and the manual VS Code verification record.
7. `editors/vscode/package.json` — publisher/listing metadata and icon.
8. `editors/vscode/images/` — listing screenshots or recording assets, if
   screenshots are used.
9. `editors/vscode/.vscodeignore` — exclude internal notes and other
   non-runtime files.
10. `editors/vscode/CHANGELOG.md` and `CHANGELOG.md` — release entries.
11. `.github/workflows/release.yml` and, if useful,
    `editors/vscode/scripts/publish.mjs` — registry publication and manual
    fallback.
12. `.goreleaser.yaml` — release-footer install links.
13. `docs/ROADMAP.md` and `docs/post-v1-backlog.md` — promoted milestone status.
14. `docs/M6-packaging-and-release.md` and `docs/M15-vscode-client.md` —
    superseded notes where they defer Marketplace/Open VSX publishing.
15. `AGENTS.md` — current-state update after publication.

## Work breakdown

1. [ ] Resolve the open decisions: release tag, icon/trademark path, registry
       sequence, and listing name.
2. [ ] Rewrite `editors/vscode/README.md` and capture listing screenshots.
3. [ ] Add listing metadata, icon, and screenshot assets; clean up
       `.vscodeignore`.
4. [ ] Update root README, install, troubleshooting, and configuration links.
5. [ ] Add registry publication and manual fallback to the release workflow.
6. [ ] Update release checklist, changelogs, roadmap, backlog, and version pins.
7. [ ] Build and verify the server and extension; inspect the VSIX contents.
8. [ ] Publish the server release, then the extension to each registry.
9. [ ] Run the manual verification checklist on the VS Code machine and record
       the results.
10. [ ] Verify both listings, update `AGENTS.md`, and mark M24 Done.

## Detailed requirements

### 1. Marketplace and Open VSX listing

`editors/vscode/README.md` is packaged into the VSIX and rendered by both
registries. Editing the root README does not update the listing.

The listing must contain, in this order:

1. A short value statement and a screenshot or recording near the top.
2. **Install** with the Marketplace path first:
   - Extensions view, search `PICO-8` or `asimmons91.pico-lsp`;
   - `code --install-extension asimmons91.pico-lsp`;
   - the Marketplace item link.
3. **Open VSX** installation for VSCodium and compatible editors, including the
   Open VSX item link.
4. **Manual VSIX** installation for offline, pinned, or registry-lag use, with
   the GitHub release link and checksum verification.
5. **Requirements**: VS Code `^1.95.0`, local desktop only, no Go required.
6. **Quick start**: open a folder containing a `.p8` cart; the matching server
   downloads on first use.
7. Feature highlights written for users, not internal milestone names.
8. **Privacy and security**: no telemetry, cart code is never executed, the only
   network access is the checksummed GitHub server download, and untrusted
   workspaces are disabled.
9. A short **AI-assisted development** note linking to the root README.
10. Existing settings, commands, logs, and version-mismatch sections, trimmed or
    linked to the repository docs where appropriate.
11. **Limitations**: local desktop only; `.p8.png` virtual documents are not
    supported in VS Code; registry versions may lag; offline use requires
    `pico8.serverPath`.
12. **Support** links and the PICO-8 trademark disclaimer.

Links and images should use absolute `https://` URLs so both registries render
them consistently. Avoid GitHub-only Markdown extensions and verify the rendered
listing after publication.

### 2. Root README and install docs

`README.md` must gain a dedicated `## VS Code` section before the Neovim and
Helix sections. It must distinguish:

- **server** channels: GitHub release archives and `go install`;
- **extension** channels: Marketplace, Open VSX, and the GitHub release VSIX.

Include the exact extension ID and CLI install command. Keep the VSIX path for
offline and pinned versions. Remove the sentence that says Marketplace and Open
VSX publishing are follow-ups.

`docs/install.md` must add a channel table and document:

- Marketplace and Open VSX UI/CLI installation;
- VSIX installation and checksum verification;
- first-run server download and cache behavior;
- `pico8.serverPath` for offline or air-gapped installs;
- extension upgrade behavior for each channel;
- extension uninstall and cached-server cleanup;
- registry lag and the VSIX fallback.

Homebrew and Mason remain deferred; do not imply they are part of the extension
listing.

### 3. Troubleshooting

`docs/troubleshooting.md` must replace the “publishing are follow-ups” paragraph
and add:

- extension not found or registry version behind GitHub;
- Marketplace/Open VSX installed but the matching server download failed;
- proxy and corporate-registry restrictions;
- extension updated but server cache still old;
- how to restart, clear the server cache, or switch to `pico8.serverPath`;
- remote, web, and container hosts remain unsupported;
- VSIX fallback for registry outages or offline machines.

Keep the existing version-mismatch and copy-diagnostics sections.

### 4. Package metadata and VSIX contents

`editors/vscode/package.json` must keep the existing publisher, name, version,
repository, license, categories, keywords, activation events, and capabilities,
and add or confirm:

- a top-level `icon` pointing at a 128×128 PNG that is legally usable;
- `homepage` and `bugs` links;
- an optional `galleryBanner`;
- a display name and description that read well in search results;
- `README.md`, `CHANGELOG.md`, and `LICENSE` packaged in the VSIX.

`editors/vscode/.vscodeignore` must exclude `icons/README.md` and any other
internal notes. `vsce ls` must show no `src/`, `test/`, `scripts/`, source maps,
or development-only files. The existing version-drift test and all extension
checks must remain green.

### 5. Icon and trademark

The current `editors/vscode/icons/README.md` says the official PICO-8 logo is a
Lexaloffle trademark and that permission should be confirmed before publishing.
Resolve this before any registry upload:

- preferred: replace the listing icon and, if necessary, the language icon with
  original artwork that does not reproduce the official logo;
- alternative: record written permission to use the official logo;
- update the icon README and listing disclaimer to match the decision.

Do not claim the icon is MIT-licensed if it is not.

### 6. Versioning and release order

The extension version pins the server release: extension `X.Y.Z` downloads
`https://github.com/asimmons91/pico-lsp/releases/download/vX.Y.Z/...`.

Therefore:

1. Choose the release tag and update the extension manifest.
2. Publish the server release and verify archives and `checksums.txt`.
3. Build and verify the VSIX.
4. Publish the VSIX to the registries.
5. Verify first-run download from the public release.

Do not publish the extension before the matching server release exists. Do not
rebuild and replace the already-published 1.8.0 GitHub asset; cut a new tag
instead. Update the version pins in `README.md`, `docs/install.md`, the
changelogs, and release notes.

### 7. Registry publication

Add tag-only publication steps to `.github/workflows/release.yml` or a small
script invoked by it. Proposed secret names:

- `VSCE_PAT` for the VS Code Marketplace;
- `OVSX_PAT` for Open VSX.

Requirements:

- Publish the exact VSIX built and checked by CI.
- Keep Marketplace and Open VSX as independent jobs so one can be retried
  without republishing the other.
- Do not publish from pull requests, branches, or lightweight local builds.
- Fail the job clearly when a token, publisher, or version is wrong.
- Document the manual fallback commands and token scopes in
  `docs/release-checklist.md`.
- Verify the published version and listing after each registry processes the
  upload; Open VSX may lag.
- Roll back by unpublishing or publishing a patch release. Never reuse a
  version number.

If only one registry is live, the README, install docs, and this milestone must
say so explicitly.

### 8. Documentation consistency

Search the repository for all of these and resolve them:

- “Marketplace” / “Open VSX” followed by “follow-up”, “deferred”, or “not
  available yet”;
- “GitHub-only” release wording;
- “VSIX only” extension install wording;
- internal milestone names such as “M15 extension” in user-facing copy;
- stale version pins and old release links.

Update the install-channel decision and release assumptions in
`docs/ROADMAP.md` when the milestone is committed. Add a short “superseded by
M24” note to the historical deferral statements in M6 and M15 rather than
rewriting shipped milestone history.

`docs/configuration.md` should only change if a link or install statement needs
it; the settings reference itself is already editor-agnostic.

### 9. Changelogs and repository state

- `CHANGELOG.md`: add the marketplace launch, listing cleanup, and release
  automation to the next release section.
- `editors/vscode/CHANGELOG.md`: add the extension release section that
  Marketplace displays.
- `docs/release-checklist.md`: make the registry steps and manual verification
  part of the standard release.
- `docs/ROADMAP.md`: mark M24 Done only after the release and verification.
- `docs/post-v1-backlog.md`: keep the promoted link and remove the Marketplace
  item from the deferred priority list.
- `AGENTS.md`: update the current release, distribution path, and extension
  status after publication.

## Manual verification on the VS Code machine

The implementation branch is prepared in the repository, but registry and
desktop checks must be performed on the maintainer's machine with a real VS Code
installation. Do not mark these checks complete from CI, a headless shell, or a
different editor.

Record the following for each run:

- date and tester;
- OS and version;
- VS Code version;
- extension version and source (Marketplace, Open VSX, or VSIX);
- server version and source (`download`, `cache`, or `serverPath`);
- result and a short note or screenshot path.

| Check | How | Expected |
|---|---|---|
| Marketplace clean install | Extensions view and `code --install-extension asimmons91.pico-lsp` | Extension appears and activates on a `.p8` |
| Open VSX install | VSCodium or another Open VSX editor | Same extension ID installs and activates |
| VSIX fallback | Install from the GitHub release VSIX | Install succeeds; checksum matches |
| First-run server download | Open a `.p8` with a clean cache | Matching server downloads, verifies, and starts |
| Offline/custom server | Set `pico8.serverPath`, disable downloads if needed | No network download; features work |
| Cart features | Open `testdata/smoke.p8` | Diagnostics, completion, hover, symbols, folding, navigation, rename, semantic tokens, formatting, inlay hints, code lens, and code actions work |
| Glyph picker | Run **PICO-8: Insert P8SCII Glyph** and type `p8_left` | Picker inserts glyphs; completion works in code and strings |
| Status bar | Edit the cart | Token, character, and compressed budgets update |
| Settings schema | Search `pico8` in Settings; set `typeChecking` to `strict` and an invalid value | Descriptions/defaults appear; invalid enum is flagged |
| Version mismatch | Point `pico8.serverPath` at a different release server | One advisory warning with the documented actions |
| Copy diagnostics | Run **PICO-8: Copy Server Diagnostics** | Versions, source, path, platform, cache root, and last error appear; no cart source |
| Upgrade | Install the previous extension version, then the registry build | Extension updates and downloads the matching server |
| Uninstall | Uninstall and clear the cache using the documented path | Extension and cached server are removed as documented |
| Listing render | View the Marketplace and Open VSX pages | README, images, links, icon, license, and changelog render correctly |
| VSIX contents | `vsce ls` and inspect the package | No source, tests, scripts, or internal notes |
| Remote/web limitation | Read the listing and docs | Unsupported hosts are clearly stated; no support claim is made |

Run the platform rows the test machine supports. Record any unverified platform
as an open release item rather than implying it passed.

## Testing and CI

- `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt -l .`, and
  `golangci-lint run ./...` remain green; this milestone should not change Go
  code.
- Extension lint, typecheck, unit tests, integration tests, and VSIX packaging
  remain green.
- `goreleaser check` and a snapshot build remain green.
- `vsce ls` confirms the packaged file set.
- The release workflow validates that the extension version matches the tag
  before publishing.
- CI does not publish to either registry from a branch or pull request.
- Manual registry installation is not simulated as a CI pass; it is recorded in
  `docs/release-checklist.md`.

## Exit criteria

- [ ] The release tag, icon/trademark path, and registry scope are resolved and
      recorded in this spec and `docs/ROADMAP.md`.
- [ ] `editors/vscode/README.md` is a complete Marketplace/Open VSX listing with
      install paths, requirements, screenshots, privacy, support, limitations,
      AI disclosure, and trademark notice.
- [ ] Root README, install, troubleshooting, and release-checklist docs describe
      Marketplace, Open VSX, VSIX, upgrade, uninstall, cache, and offline paths.
- [ ] No public document says Marketplace/Open VSX publishing is a follow-up.
- [ ] `package.json` listing metadata and icon are complete; the VSIX contains
      README, CHANGELOG, LICENSE, and icon but no source, tests, scripts, or
      internal notes.
- [ ] The icon/trademark question is resolved without publishing an unlicensed
      PICO-8 logo.
- [ ] The chosen release tag is cut after the release PR merges, with the server
      release published and verified before the extension.
- [ ] Marketplace and Open VSX publish the same verified VSIX, independently
      retryable, with a documented manual fallback and rollback path.
- [ ] Marketplace and Open VSX listings show the correct version, README,
      screenshots, icon, license, changelog, repository links, categories, and
      keywords.
- [ ] The manual VS Code machine checklist is recorded for the available
      platforms: clean install, upgrade, uninstall, first-run download, offline
      server, cart features, settings, glyph picker, status bar, diagnostics,
      and listing rendering.
- [ ] Changelogs, roadmap, backlog, release notes, and `AGENTS.md` are updated.
- [ ] No LSP, server, parser, analysis, formatter, or extension feature behavior
      changed.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| The official PICO-8 logo is a trademark and the repository already flags it as needing permission | Resolve before packaging; prefer original artwork and keep the trademark disclaimer. |
| The extension downloads the server version matching its own version | Publish and verify the server release first; test first-run download from the public release. |
| Republishing 1.8.0 breaks the GitHub asset/checksum contract | Cut a new tag and rebuild the extension; never silently replace the old asset. |
| The listing README is packaged, so repository-only edits do not update Marketplace | Rewrite `editors/vscode/README.md` before building the VSIX and verify the rendered listing. |
| Marketplace or Open VSX rejects metadata, icon, or README | Validate with `vsce ls`/`vsce package`, review the listing preview, and keep the VSIX fallback. |
| Open VSX lags or its token fails | Publish jobs independently, document the manual fallback, and do not claim Open VSX availability until it is live. |
| Registry auto-update leaves an old cached server | Document **Clear Downloaded Server Cache**, restart, and `pico8.serverPath`; keep the version-mismatch warning. |
| Relative README links or images break on Marketplace | Use absolute `https://` URLs and alt text; verify the rendered listing. |
| Manual checks are claimed without access to a real VS Code window | Record the environment and results on the VS Code machine; leave unrun checks open. |
| Documentation work drifts into extension feature changes | Keep code changes out of scope; use existing commands and behavior. |
| Users expect remote/web support after seeing a Marketplace listing | Make local-desktop-only a top-level requirement in the listing and troubleshooting docs. |
