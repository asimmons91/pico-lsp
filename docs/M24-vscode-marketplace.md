# M24 — VS Code Marketplace Launch and Documentation Cleanup

**Status:** In progress — `v1.8.1` was released on 2026-11-22 as a GitHub-only
release (server archives and VSIX). Registry publication and manual VS Code
verification remain pending for a later patch. Release:
https://github.com/asimmons91/pico-lsp/releases/tag/v1.8.1

**Depends on:** M15 (VS Code client), M21 (settings schema), M22 (server health)

**Release target:** `v1.8.1` is a GitHub-only patch. Registry publication is
deferred to a later patch once `asimmons91` is verified in both registries and
`VSCE_PAT`/`OVSX_PAT` are configured. Extension `X.Y.Z` downloads server tag
`vX.Y.Z`.

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

The `v1.8.1` GitHub-only release is an intermediate step. The discoverability,
listing, and registry outcomes complete with the later registry patch.

**Demo:** On a clean VS Code 1.95+ install, search for “PICO-8 LSP”, install
`asimmons91.pico-lsp`, and open `testdata/smoke.p8`. The extension downloads the
matching `pico-lsp` server, attaches to the cart, and provides diagnostics,
completion, the cart-budget status bar, and the glyph picker without requiring
Go or a manual server install. The same VSIX installs from Open VSX in a
compatible editor, and from the GitHub release for pinned or registry-lag use;
offline use also requires a local server binary and `pico8.serverPath`. An
upgrade replaces the extension and downloads the matching server; the
uninstall path removes the extension and explains how to clear the cached
server.

The `v1.8.1` release is GitHub-only: install the VSIX from the GitHub release.
The Marketplace/Open VSX install steps in this demo apply once the later
registry patch is live.

## Decisions

| ID | Decision | Choice |
|---|---|---|
| D1 | Milestone identity | M24, **In progress** (v1.8.1 GitHub-only; registry launch later). |
| D2 | Listing source | `editors/vscode/README.md` is the Marketplace/Open VSX listing. The root `README.md` remains the repository landing page and is updated separately. |
| D3 | Registries | Publish to both the VS Code Marketplace and Open VSX. The publishers are independent and independently retryable. If registry processing is delayed, the VSIX remains the documented fallback. |
| D4 | Release version | **`v1.8.1`** is the GitHub-only release containing the server tag and rebuilt extension. The registry patch uses a later tag. The 1.8.0 artifact is not republished. |
| D5 | Release order | Publish and verify the server release first, then the extension. Extension version `X.Y.Z` downloads server tag `vX.Y.Z`. |
| D6 | Publication automation | Add Marketplace and Open VSX publishers to the tag-only release workflow, backed by repository secrets and a documented manual fallback. |
| D7 | Icon and trademark | **Original artwork.** `editors/vscode/icons/pico-lsp.png` replaces the official PICO-8 logo for the listing and `.p8` file icon and is covered by the project's MIT license. The FAQ places a separate software product in the "write first" category, so the official logo is not published without written permission. |
| D8 | Publisher identity | Keep `asimmons91.pico-lsp` as the extension ID. The `asimmons91` publisher must be claimed and verified in both registries before the first publish; this remains a release-checklist item. |
| D9 | Support scope | Local desktop VS Code and Open VSX-compatible editors only. Remote SSH, WSL, Dev Containers, Codespaces, and web extension hosts remain unsupported and must be prominent in the listing. |
| D10 | Verification environment | Build, lint, unit, integration, and packaging checks run in CI. Registry installation and desktop behavior are verified on the maintainer's machine with a real VS Code installation and recorded in the release checklist. |
| D11 | No behavior changes | Do not change LSP responses, server defaults, extension feature behavior, settings, or the download/checksum contract. |
| D12 | Version-pinned docs | Update the version pins in the root README and install docs as part of the release. Do not leave examples pointing at the previous tag. |
| D13 | Listing disclosure | Include a short AI-assisted-development note and a link to the project README, matching the root README's existing disclosure. |
| D14 | VSIX fallback | Keep the GitHub release VSIX and `.vsix.sha256` as the pinned and registry-lag fallback. Offline use also requires a local server binary and `pico8.serverPath`. |
| D15 | Listing display name | **PICO-8 LSP.** The extension ID remains `asimmons91.pico-lsp`. |
| D16 | v1.8.1 release scope | **GitHub server archives and VSIX only.** Registry publication is deferred to a later patch because the publisher and PAT secrets are not configured. Set the repository variable `PUBLISH_REGISTRIES` to `true` and add `VSCE_PAT`/`OVSX_PAT` for that later release. |

### Resolved decisions

1. **Release tag:** `v1.8.1`.
2. **Icon:** original artwork; no official Lexaloffle logo is published.
3. **Registry sequence:** both registries, with independent publication jobs,
   gated by the `PUBLISH_REGISTRIES` repository variable, and the VSIX fallback
   documented.
4. **Listing name:** `PICO-8 LSP`.
5. **v1.8.1 scope:** GitHub release archives and VSIX only; registry publication
   follows in a later patch.

### Pending publication items

- After the GitHub-only release, claim and verify the `asimmons91` publisher in
  both registries.
- Configure the `VSCE_PAT` and `OVSX_PAT` repository secrets and set
  `PUBLISH_REGISTRIES=true`.
- Add a real screenshot or short recording from the manual verification to
  `editors/vscode/images/` and the listing before the registry patch.
- Publish the later registry patch, verify both listings, and record the manual
  VS Code verification in `docs/release-checklist.md`.

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

1. [x] Resolve the open decisions: release tag, icon/trademark path, registry
       sequence, and listing name.
2. [x] Rewrite `editors/vscode/README.md` as the product listing and commit an
       illustrative banner. Real screenshots remain a publication item.
3. [x] Add listing metadata, the original icon, and image assets; clean up
       `.vscodeignore`.
4. [x] Update root README, install, troubleshooting, and configuration links.
5. [x] Add registry publication and manual fallback to the release workflow.
6. [x] Update release checklist, changelogs, roadmap, backlog, and version pins.
7. [x] Build and verify the server and extension; inspect the VSIX contents.
8. [x] Publish `v1.8.1` as a GitHub-only release.
9. [ ] Run the manual verification checklist on the VS Code machine and record
       the results.
10. [ ] Prepare the later registry patch: add real screenshots, set the secrets
       and `PUBLISH_REGISTRIES=true`, publish both registries, verify both
       listings, update `AGENTS.md`, and mark M24 Done.

## Detailed requirements

### 1. Marketplace and Open VSX listing

`editors/vscode/README.md` is packaged into the VSIX and rendered by both
registries. Editing the root README does not update the listing.

The listing must contain, in this order:

> **v1.8.1 exception:** the registries are not live for this release, so the
> packaged README leads with manual VSIX installation and marks the registry
> sections as planned for a later patch. Restore the Marketplace-first order
> when the later registry patch ships.

1. A short value statement and an image near the top. The committed banner is
   original illustrative artwork; a real screenshot or recording from the manual
   verification must replace or accompany it before publication.
2. **Install** with the Marketplace path first:
   - Extensions view, search `PICO-8 LSP` or `asimmons91.pico-lsp`;
   - `code --install-extension asimmons91.pico-lsp`;
   - the Marketplace item link.
3. **Open VSX** installation for VSCodium and compatible editors, including the
   Open VSX item link.
4. **Manual VSIX** installation for pinned or registry-lag use, with
   the GitHub release link and checksum verification. Offline use also requires
   a local server binary and `pico8.serverPath`.
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
Helix sections. For v1.8.1 it must distinguish:

- **server** channels: GitHub release archives and `go install`;
- **extension** channels: the GitHub release VSIX now, with Marketplace and Open
  VSX listed as planned for a later patch.

Include the exact extension ID. The `code --install-extension` command returns
with the registry patch. Keep the VSIX path for pinned, offline, and registry-lag
versions; offline use requires a local server and `pico8.serverPath`.

`docs/install.md` must add a channel table and document:

- VSIX installation and checksum verification as the v1.8.1 path;
- Marketplace and Open VSX UI/CLI installation as planned for a later patch;
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
- VSIX fallback for registry outages or pinned versions; offline machines also
  need a local server and `pico8.serverPath`.

Keep the existing version-mismatch and copy-diagnostics sections.

### 4. Package metadata and VSIX contents

`editors/vscode/package.json` keeps the existing publisher, name, repository,
license, categories, keywords, activation events, and capabilities, updates the
version to `1.8.1`, and adds or confirms:

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

**Resolved: original artwork.** `editors/vscode/icons/pico-lsp.png` is original
pixel art (a generic cartridge with a rainbow label), covered by the project's
MIT license, and does not reproduce the official PICO-8 logo or wordmark. It is
used for both the listing icon and the `.p8` language icon. `icons/README.md`
records the license and trademark position.

The PICO-8 FAQ says separate software products must write to Lexaloffle before
using the logo, and a non-affiliation disclaimer does not replace permission.
The official logo is therefore not published unless written permission is
obtained and recorded here.

### 6. Versioning and release order

The extension version pins the server release: extension `X.Y.Z` downloads
`https://github.com/asimmons91/pico-lsp/releases/download/vX.Y.Z/...`.

The release tag is resolved as `v1.8.1` for the GitHub-only release. The registry
patch uses a later tag. The order is:

1. Publish `v1.8.1` server archives and the VSIX with `PUBLISH_REGISTRIES`
   unset, so the registry jobs are skipped.
2. Verify the GitHub release assets, VSIX checksum, and first-run download.
3. For the later registry patch: claim and verify the publisher, add
   `VSCE_PAT`/`OVSX_PAT`, set `PUBLISH_REGISTRIES=true`, tag the patch, and
   verify both listings.

Do not publish the extension to a registry before the matching server release
exists. Do not rebuild and replace the already-published 1.8.0 GitHub asset; cut
the new `v1.8.1` tag instead. Update any remaining version pins in `README.md`,
`docs/install.md`, the changelogs, and release notes.

### 7. Registry publication

**Implemented** in `.github/workflows/release.yml` as two independent tag-only
jobs after the server release job:

- `publish-marketplace` uses the `VSCE_PAT` secret.
- `publish-openvsx` uses the `OVSX_PAT` secret.

Registry publishing is gated by the repository variable `PUBLISH_REGISTRIES`.
When it is unset or not `true`, both jobs are skipped. `v1.8.1` is published
with the variable unset; the later registry patch sets it to `true`.

Both jobs download the `vscode-vsix` CI artifact, validate the manifest version
against the tag, verify the VSIX checksum, fail clearly when the token is
missing, and publish that exact VSIX. The release job also re-runs the packaged
file assertions before uploading the artifact, and retains it for 90 days. The
jobs never run for pull requests or branches because the workflow only triggers
on `v*` tags.

The manual fallback commands and token scopes are in
[release-checklist.md](release-checklist.md#registry-automation). Both commands
use `--skip-duplicate` so a retry can complete when the version was already
published; always verify the published version afterwards.

Requirements remain:

- Verify the published version and listing after each registry processes the
  upload; Open VSX may lag.
- Roll back by unpublishing where supported or publishing a patch release. Never
  reuse a version number.
- If only one registry is live, the README, install docs, and this milestone
  must say so explicitly. Both registries are the target; the listing claims
  must be adjusted if publication fails for one.

### 8. Documentation consistency

The repository has been updated for:

- “Marketplace” / “Open VSX” followed by “follow-up”, “deferred”, or “not
  available yet” in current install, troubleshooting, extension, and release
  documents;
- “GitHub-only” release wording;
- “VSIX only” extension install wording;
- internal milestone names such as “M15 extension” in user-facing copy;
- stale version pins and old release links.

The install-channel decision and release assumptions in `docs/ROADMAP.md` record
the committed milestone. M6 and M15 carry short “superseded by M24” notes rather
than rewritten shipped history.

`docs/configuration.md` did not need an install-statement change; the settings
reference remains editor-agnostic.

### 9. Changelogs and repository state

- `CHANGELOG.md`: the GitHub-only v1.8.1 release, listing preparation, and
  opt-in registry automation are in the `1.8.1` section.
- `editors/vscode/CHANGELOG.md`: the extension `1.8.1` section describes the
  VSIX release and the prepared Marketplace/Open VSX listing.
- `docs/release-checklist.md`: registry steps and manual verification are part
  of the standard release.
- `docs/ROADMAP.md`: M24 stays **In progress** until the release and
  verification complete; mark it Done only afterwards.
- `docs/post-v1-backlog.md`: the promoted link is kept and Marketplace/Open VSX
  is removed from the deferred priority list.
- `AGENTS.md`: records the implementation and the pending publication. Update
  the current release and extension status again after publication.

## Manual verification on the VS Code machine

**Status:** not yet run. Registry publication and every desktop check below
remain open; record results in
[release-checklist.md](release-checklist.md#manual-vs-code-verification).

Implementation changes are made in the repository, but registry and desktop
checks must be performed on the maintainer's machine with a real VS Code
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

Run these checks on each platform the test machine supports. Record any
unverified platform as an open release item rather than implying it passed.

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

- [x] The release tag, icon/trademark path, and registry scope are resolved and
      recorded in this spec and `docs/ROADMAP.md`.
- [x] `editors/vscode/README.md` is a complete Marketplace/Open VSX listing with
      install paths, requirements, privacy, support, limitations, AI disclosure,
      and trademark notice, using the v1.8.1 VSIX-first install path. An
      illustrative banner is committed.
- [ ] A real screenshot or recording from the manual verification is added to the
      listing before publication.
- [x] Root README, install, troubleshooting, and release-checklist docs describe
      Marketplace, Open VSX, VSIX, upgrade, uninstall, cache, and offline paths.
- [ ] No current README, install, troubleshooting, extension listing, or release
      document describes Marketplace/Open VSX publishing as pending; historical
      milestone statements are marked superseded. This is met when the later
      registry patch ships; `v1.8.1` accurately says registry publication is
      planned for a later patch.
- [x] `package.json` listing metadata and icon are complete; the VSIX contains
      README, CHANGELOG, LICENSE, and icon but no source, tests, scripts, or
      internal notes. Verified locally with `vsce ls`.
- [x] The icon/trademark question is resolved without publishing an unlicensed
      PICO-8 logo.
- [x] The `v1.8.1` GitHub-only release is cut with server archives and the VSIX;
      the registry jobs are skipped because `PUBLISH_REGISTRIES` is unset.
- [ ] The later registry patch is tagged after the release PR merges, with the
      server release published and verified before the extension.
- [ ] Marketplace and Open VSX publish the same verified VSIX, independently
      retryable, with a documented manual fallback and rollback path.
- [ ] Marketplace and Open VSX listings show the correct version, README,
      screenshots or recording, icon, license, changelog, repository links,
      categories, and keywords.
- [ ] The manual VS Code machine checklist is recorded for the available
      platforms: clean install, upgrade, uninstall, first-run download, offline
      server, cart features, settings, glyph picker, status bar, diagnostics,
      and listing rendering.
- [x] Changelogs, roadmap, backlog, release notes, and `AGENTS.md` are updated.
- [x] No LSP, server, parser, analysis, formatter, or extension feature behavior
      changed.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| The official PICO-8 logo is a trademark and the FAQ requires permission for a separate software product | Resolved with original artwork; the official logo is not published and the trademark disclaimer remains. |
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
