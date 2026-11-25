# M24 — End-User Documentation and Onboarding

**Status:** In progress — the documentation, support files, demo cart, and link
check shipped in `v1.9.0` on 2026-11-25. M24 was reworked around end-user
documentation and onboarding after registry publication stalled on publisher
account setup. The real screenshot, manual VS Code verification, and tagged
`pico8.nvim` release remain open. Release:
https://github.com/asimmons91/pico-lsp/releases/tag/v1.9.0

**Depends on:** M15 (VS Code client), M21 (settings schema), M22 (server health),
M33 (standard Neovim hooks). The Neovim client release depends on
[`pico8.nvim`](https://github.com/asimmons91/pico8.nvim) M32–M34. A manual tag is
acceptable for M24; `pico8.nvim` M35 adds release automation.

**Release target:** v1.9.0 shipped the documentation and onboarding set. A later
patch or docs-only update carries the real screenshot and the manual VS Code
verification once they are available. The version is chosen at release time.
Extension `X.Y.Z` downloads server tag
`vX.Y.Z`. Registry publication is not a release target.

**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

A new user can discover, install, use, and get help with `pico-lsp` from the
repository and the GitHub release. The public docs cover a five-minute
quickstart, the PICO-8 editing workflow, the supported LuaCATS type features,
the CLI, migration from `pico8-ls`, and the tested editor matrix. The VS Code
extension has a real screenshot or short recording, a recorded manual VSIX
verification, and a product page that says plainly that it is distributed
through GitHub rather than a registry. The Neovim client is tagged and has a
copy-paste install path in the root README. Bug reports have a template and a
private security-reporting path.

The VS Code Marketplace and Open VSX publication are **parked**: the publisher
accounts are not ready, so M24 must not depend on them. The implemented registry
jobs stay dormant behind `PUBLISH_REGISTRIES`; the backlog owns the launch.

No LSP protocol, server behavior, type analysis, formatting, or extension
feature changes are in scope.

**Demo (VS Code):** On a clean VS Code 1.95+ install, download
`pico-lsp-vscode-<version>.vsix` from the GitHub release, verify its checksum,
install it with **Install from VSIX**, and open `testdata/smoke.p8`. The
extension downloads the matching `pico-lsp` server, attaches to the cart, and
provides diagnostics, completion, the cart-budget status bar, and the glyph
picker without requiring Go or a manual server install. Offline use requires a
local server binary and `pico8.serverPath`. An upgrade replaces the VSIX and
downloads the matching server; uninstall and cache cleanup follow the documented
path.

**Demo (Neovim):** Install the tagged `pico8.nvim` release, open a `.p8` cart,
and see diagnostics, completion, hover, symbols, the cart budget, and `gq` range
formatting. The root README shows the same install snippet.

## Decisions

| ID | Decision | Choice |
|---|---|---|
| D1 | Milestone identity | M24 is **End-User Documentation and Onboarding**. The earlier Marketplace launch scope is replaced; registry publication is a backlog item, not a current milestone deliverable. |
| D2 | VS Code distribution | The GitHub release VSIX is the only supported VS Code channel for now. Do not claim Marketplace or Open VSX availability. |
| D3 | Registries | Out of scope until the `asimmons91` publisher and registry tokens exist. Keep the implemented, opt-in release jobs; do not remove or re-enable them as part of M24. |
| D4 | Release timing | v1.9.0 shipped the documentation set with the M33 release. Ship a later patch when the real screenshot and manual VS Code verification are ready; choose the patch version at release time. |
| D5 | Manual verification | Run the VSIX-only checklist on a real VS Code installation and record it in `docs/release-checklist.md`. Do not mark registry checks. |
| D6 | Demo assets | Add a real screenshot or short recording from the manual verification. The illustrative banner remains supplementary. |
| D7 | Icon and trademark | Keep the original artwork at `editors/vscode/icons/pico-lsp.png`. The official PICO-8 logo is not published without written permission. |
| D8 | Publisher identity | Keep the extension ID `asimmons91.pico-lsp`. Claiming and verifying the publisher is part of the parked registry work. |
| D9 | Support scope | Local desktop VS Code and Open VSX-compatible editors via the VSIX. Remote SSH, WSL, Dev Containers, Codespaces, and web extension hosts remain unsupported. |
| D10 | Verification environment | CI keeps build, lint, unit, integration, and packaging checks. Desktop behavior and the VSIX install path are verified on the maintainer's machine. |
| D11 | No behavior changes | Do not change LSP responses, server defaults, extension feature behavior, settings, or the download/checksum contract. CLI help text may mention `decode`; that is documentation, not behavior. |
| D12 | Version-pinned docs | Update the version pins in the root README, install docs, and VS Code product page as part of the release. |
| D13 | Listing disclosure | Keep the AI-assisted-development note and the link to the root README in the VS Code product page. |
| D14 | VSIX is primary | The GitHub release VSIX and `.vsix.sha256` are the primary and pinned install path. Offline use also requires a local server binary and `pico8.serverPath`. |
| D15 | Display name | Keep **PICO-8 LSP** as the extension display name. |
| D16 | Documentation set | M24 owns the quickstart, demo cart, PICO-8 workflow, type guide, CLI reference, migration guide, editor matrix, support files, and repository metadata. |
| D17 | Neovim client | Tag `pico8.nvim` (manually if M35 release automation is not ready) and link the tagged install path from the root README. The plugin work stays in that repository; M24 owns the cross-repo install documentation. |
| D18 | Registry backlog | Record the publisher verification, PATs, `PUBLISH_REGISTRIES=true`, publication, and listing verification in the post-v1 backlog; M24 supplies the screenshots. |

### Resolved decisions

1. **v1.8.1 assets are not republished.** v1.9.0 is the current release; the
   earlier GitHub-only assets remain unchanged.
2. **Icon:** original artwork; no official Lexaloffle logo is published.
3. **VS Code channel:** GitHub release VSIX only for now.
4. **Neovim channel:** tagged `pico8.nvim` release plus the documented binary
   install.
5. **Documentation release:** v1.9.0 carried the documentation and VSIX
   metadata; the real screenshot and manual verification may ride a later
   patch.

### Parked: registry publication

The following work is intentionally **not** part of M24. It is recorded in
[docs/post-v1-backlog.md](post-v1-backlog.md) and blocked on account setup:

- claim and verify the `asimmons91` publisher in both registries;
- configure `VSCE_PAT` and `OVSX_PAT` and set `PUBLISH_REGISTRIES=true`;
- publish the same verified VSIX to both registries;
- verify both listings render the README, screenshot, icon, license, changelog,
  links, categories, and keywords;
- restore registry-first install wording only after both listings are live.

The release workflow already contains independent, tag-only
`publish-marketplace` and `publish-openvsx` jobs gated by
`PUBLISH_REGISTRIES`. They stay implemented and skipped.

## Scope

### In scope

- A `docs/getting-started.md` quickstart with a copyable cart and a link to the
  repository demo cart.
- A `docs/pico8-workflow.md` guide covering carts on disk, external editing,
  reloading with `load`, save conflicts, includes, and read-only `.p8.png` carts.
- A user-facing LuaCATS and type guide with examples for annotations, classes,
  inheritance, `(exact)`, generics, overloads, tagged unions, narrowing, and
  `off`/`basic`/`strict`.
- A `docs/cli.md` reference for `--version`, `--log-file`, `--log-level`,
  `decode`, exit codes, and the stdout rule.
- A migration and coexistence guide for `pico8-ls` and `lua_ls`/LuaLS.
- A `docs/editors.md` index with the tested editor and version matrix, plus a
  concrete Helix settings example.
- A root README support/privacy summary and links to the new guides.
- `.github/ISSUE_TEMPLATE/` bug and configuration files, `SECURITY.md`, and
  `CONTRIBUTING.md`.
- GitHub repository metadata: description, topics, homepage, and social preview.
- Real screenshots or a short recording for the root README and the VS Code
  product page.
- VSIX-only manual verification on a real VS Code installation.
- A tagged `pico8.nvim` release and a copy-paste install snippet in the root
  README.
- Documentation consistency for the GitHub-only channel, version pins, and the
  parked registry status.
- A documentation link check in CI.

### Out of scope

- VS Code Marketplace and Open VSX publication, publisher verification, tokens,
  listing verification, and registry-first install wording.
- Removing or changing the opt-in registry jobs.
- Server, parser, analysis, formatter, LSP, or extension feature changes.
- New settings, commands, requests, diagnostics, code actions, or cart formats.
- Remote extension hosts, web extension hosts, or Codespaces.
- Bundling platform server binaries inside the VSIX.
- Auto-updating the server independently of the extension.
- Homebrew and Mason packaging.
- Zed or other editor clients.
- Repository-wide visual redesign or localization.

## Deliverables

1. `docs/M24-end-user-onboarding.md` — this specification.
2. `docs/getting-started.md` — install-to-first-feature quickstart.
3. `examples/hello.p8` — a small, public demo cart linked from the quickstart;
   `testdata/smoke.p8` remains the feature smoke-test cart.
4. `docs/pico8-workflow.md` — carts on disk, external editing, reload, save
   conflicts, includes, and PNG limits.
5. `docs/types.md` — LuaCATS and gradual type guide.
6. `docs/cli.md` — command-line reference.
7. `docs/migrating.md` — `pico8-ls` migration and LuaLS coexistence.
8. `docs/editors.md` — tested editor matrix and per-editor entry points.
9. `README.md` — quickstart link, screenshots, support/privacy, and the
   GitHub-VSIX install path.
10. `docs/install.md` — channel table without a registry promise, plus VSIX
    install, upgrade, uninstall, cache, and offline paths.
11. `docs/troubleshooting.md` — "not in a registry" guidance and the existing
    proxy, cache, and version-mismatch sections.
12. `editors/vscode/README.md` — GitHub-VSIX product page, screenshot, quick
    start, requirements, privacy, commands, and limitations.
13. `editors/vscode/images/` — screenshot or recording captured from the manual
    verification.
14. `.github/ISSUE_TEMPLATE/bug.yml`, `.github/ISSUE_TEMPLATE/config.yml`,
    `SECURITY.md`, and `CONTRIBUTING.md`.
15. `internal/docscheck/linkcheck_test.go` and `.github/workflows/ci.yml` —
    local Markdown link check.
16. `docs/release-checklist.md` — M24 checklist, VSIX-only manual verification,
    and the parked registry section.
17. `docs/ROADMAP.md`, `docs/post-v1-backlog.md`, and `AGENTS.md` — status,
    decisions, and backlog links.
18. `CHANGELOG.md` and `editors/vscode/CHANGELOG.md` — entries for the patch, if
    one is cut.

## Work breakdown

1. [x] Reconcile public wording: GitHub VSIX is the current VS Code channel;
   registry publication is parked, not "coming in a later patch."
2. [x] Write `docs/getting-started.md` and add `examples/hello.p8`; link both
   from the root README.
3. [x] Write `docs/pico8-workflow.md`, including the default carts locations,
   external editing, `load`/reload, save conflicts, includes, and read-only PNG
   carts.
4. [x] Write `docs/types.md` with runnable annotation examples and the
   `off`/`basic`/`strict` behavior.
5. [x] Write `docs/cli.md`; the reference explains the `decode` subcommand and
   the top-level help limitation.
6. [x] Write `docs/migrating.md` for `pico8-ls` users and LuaLS coexistence.
7. [x] Write `docs/editors.md` with the tested matrix and a concrete Helix
   settings example; link it from the root README.
8. [x] Add the issue template, `SECURITY.md`, and `CONTRIBUTING.md`; add a root
   README support/privacy section, a CHANGELOG link, and CI/release badges.
9. [x] Set the GitHub description, topics, and homepage. A social preview still
   needs a manual upload in the repository settings.
10. [ ] Capture a real screenshot or recording and add it to the root README and
    `editors/vscode/README.md`.
11. [ ] Run the VSIX-only manual verification on a real VS Code installation and
    record it in `docs/release-checklist.md`.
12. [ ] Tag `pico8.nvim` and replace the root README's unpinned plugin install
    with the tagged install, keeping the raw `vim.lsp.config` fallback.
13. [x] Add the documentation link check to CI and fix broken links.
14. [x] Cut the documentation/onboarding release as v1.9.0 because the VSIX
    product page and extension metadata changed.
15. [x] Update the roadmap, backlog, changelogs, and `AGENTS.md`.

## Detailed requirements

### 1. Quickstart and demo cart

`docs/getting-started.md` must take a new user from nothing to a working
language server in one page:

- prerequisites and non-prerequisites: no Go for the VS Code VSIX, no PICO-8
  install required to run the language server, network access for the first
  server download;
- install paths for VS Code (VSIX), Neovim (`pico8.nvim` tag), and Helix
  (`editors/helix/languages.toml`);
- a copyable minimal `.p8` cart and the `examples/hello.p8` link;
- what should appear: diagnostics, completion, hover, symbols, cart budget;
- one or two things to try next, and links to configuration, types, and
  troubleshooting;
- a "what this is not" note: it does not run carts, replace the PICO-8 editor,
  or provide the Lua standard library.

The quickstart must not require cloning the repository or building Go. The demo
cart must be small enough to paste into PICO-8 and must not depend on internal
test comments.

### 2. PICO-8 workflow

`docs/pico8-workflow.md` must explain the real editing loop:

- where PICO-8 stores carts by default on Linux, macOS, and Windows, and how to
  open a cart folder in the editor;
- editing a `.p8` outside PICO-8, then reloading the cart in PICO-8 with `load`;
- the risk of PICO-8 saving from memory over external edits, and how to avoid
  losing work;
- external-editor workflow support where PICO-8 provides it, without promising
  editor integration PICO-8 does not have;
- `#include` ownership and the nearest-ancestor rule;
- read-only `.p8.png` carts and the `pico-lsp decode` fallback;
- when to use `testdata/smoke.p8` versus the user's own cart.

Do not invent PICO-8 keyboard shortcuts or configuration keys. Verify the
documented paths and commands against PICO-8 0.2.7 before publishing.

### 3. LuaCATS and type guide

`docs/types.md` must be written for users, not implementers. It must include:

- a minimal annotated function and the hover/diagnostic result;
- `---@class`, fields, inheritance, and `(exact)` with examples;
- generics, overloads, and operator annotations at a practical level;
- tagged unions and flow narrowing;
- aliases, enums, `---@param`, `---@return`, and `---@type`;
- `off`, `basic`, and `strict` with a short "which should I use?" recommendation;
- the low-false-positive rule and when `any` is the right answer;
- links to the diagnostic reference and configuration.

Every example must be valid PICO-8 Lua and must not use the Lua standard
library.

### 4. CLI reference

`docs/cli.md` must document:

- `pico-lsp` with no arguments (stdio server);
- `--version`, `--log-file`, and `--log-level` with accepted levels;
- `pico-lsp decode --output PATH CART.p8.png`;
- exit codes and the stdout rule;
- examples for editor configuration and bug-report logs.

`pico-lsp --help` currently lists only the server flags. Either add a one-line
`decode` hint to the help output or make the limitation explicit in the
reference. Do not change server behavior.

### 5. Migration and coexistence

`docs/migrating.md` must cover:

- moving from `pico8-ls`: the `p8_` and `p8-` completion aliases, feature
  differences, and how to disable the old server;
- running alongside LuaLS or `lua_ls`: avoid attaching both to cart-owned `.lua`
  files, and document the workspace-scoping behavior;
- which server owns a `.lua` include;
- what to do when both servers produce duplicate diagnostics or completion;
- a short checklist for a clean switch.

### 6. Editor setup and compatibility

`docs/editors.md` must be the single index for editor support:

| Editor | Tested version | Status | Install path |
|---|---|---|---|
| VS Code | 1.95+ | Supported, local desktop only | GitHub release VSIX |
| Neovim | 0.11+ (0.12 for `gx`) | Supported | Tagged `pico8.nvim` or manual `vim.lsp.config` |
| Helix | Stable at release time (record exact version) | Supported | `editors/helix/languages.toml` |
| Open VSX-compatible editors | Current stable | Best-effort via VSIX; not separately tested | VSIX |
| Zed | — | Post-v1 backlog | — |

The table must record what was actually tested. Do not claim a Helix version
that was not run. Include one concrete Helix settings example that passes the
`pico8` section through the server definition or the supported language
configuration, rather than telling users to consult their Helix release notes.

### 7. Support, security, and repository hygiene

Add:

- `.github/ISSUE_TEMPLATE/bug.yml` with required fields for `pico-lsp
  --version`, editor and version, OS/architecture, the VS Code diagnostics block
  when applicable, a minimal cart or include, and the relevant log lines;
- `.github/ISSUE_TEMPLATE/config.yml` linking the docs, troubleshooting, and
  security policy;
- `SECURITY.md` with a private reporting path and a scope statement for the
  server, extension, and public packages;
- `CONTRIBUTING.md` with the Go version, test commands, branch and commit rules,
  docs expectations, and the AI-assisted-development disclosure;
- a root README **Support** section linking the docs, issue tracker, security
  policy, and troubleshooting guide, with a short no-telemetry/no-cart-execution
  privacy summary and a CHANGELOG link;
- a CI or release badge row in the root README, if the badge targets are stable.

Set the GitHub repository description, topics (`pico-8`, `pico8`,
`language-server`, `lsp`, `lua`, `neovim`, `helix`, `vscode`), homepage, and
social preview. These are release-checklist items, not code changes.

### 8. VS Code VSIX product page and demo assets

`editors/vscode/README.md` remains the product page packaged in the VSIX. It
must lead with the GitHub VSIX install, not a registry search. Keep the existing
requirements, first-run download, offline, commands, privacy, AI disclosure, and
limitations sections. Add:

- a real screenshot or short recording near the top, captured from the manual
  verification, with alt text and an absolute `https://` URL;
- a link to the quickstart and demo cart;
- a plain statement that the extension is not currently listed in the VS Code
  Marketplace or Open VSX and that the VSIX is the supported channel;
- the local-desktop-only limitation in the requirements and limitations
  sections.

Do not package internal images if the README uses absolute URLs. Keep
`.vscodeignore` excluding `src/`, tests, scripts, and internal notes.

### 9. Neovim client release

`pico8.nvim` must have a tagged release before the root README presents it as
the primary Neovim install. The root README must show:

- the lazy.nvim snippet with `main = "pico_lsp"` and `lazy = false`;
- the tagged release or branch pin, not an unreleased branch;
- the manual `vim.lsp.config` fallback for users who do not want the plugin;
- the `editors/nvim` helper only as a compatibility note or remove it in favor
  of the plugin, without breaking existing links.

The actual tag and release notes live in `pico8.nvim`; record the tag in this
milestone and the roadmap. Do not duplicate the plugin's implementation in this
repository.

### 10. Documentation consistency and link checks

Replace current "planned for a later patch" registry wording with "not currently
published; install the GitHub release VSIX" in:

- `README.md`;
- `docs/install.md`;
- `docs/troubleshooting.md`;
- `editors/vscode/README.md`;
- `docs/M6-packaging-and-release.md`;
- `docs/M15-vscode-client.md`;
- `docs/M21-vscode-settings.md`;
- `AGENTS.md`.

Historical release records and changelog entries may keep the v1.8.1 GitHub-only
wording; do not rewrite shipped history.

Add a documentation link check test under `internal/docscheck` and run it in
CI. It must validate relative links and anchors in `README.md`, `docs/`, and
`editors/`, and fail on broken local links. External link failures may warn
rather than fail if the check is flaky.

## Manual verification on the VS Code machine

**Status:** not run. Record results in
[release-checklist.md](release-checklist.md#manual-vs-code-verification).

Run these checks on the maintainer's machine with a real VS Code installation.
Do not mark them complete from CI, a headless shell, or a different editor.

| Check | How | Expected |
|---|---|---|
| VSIX install | Install the GitHub release VSIX | Install succeeds; checksum matches; extension activates on a `.p8` |
| First-run server download | Open a `.p8` with a clean cache | Matching server downloads, verifies, and starts |
| Offline/custom server | Set `pico8.serverPath` and disable downloads | No network download; features work |
| Cart features | Open `examples/hello.p8` and `testdata/smoke.p8` | Diagnostics, completion, hover, symbols, folding, navigation, rename, semantic tokens, formatting, inlay hints, code lens, and code actions work |
| Glyph picker | Run **PICO-8: Insert P8SCII Glyph** and type `p8_left` | Picker inserts glyphs; completion works in code and strings |
| Status bar | Edit the cart | Token, character, and compressed budgets update |
| Settings schema | Search `pico8`; set `typeChecking` to `strict` and an invalid value | Descriptions/defaults appear; invalid enum is flagged |
| Version mismatch | Point `pico8.serverPath` at a different release server | One advisory warning with the documented actions |
| Copy diagnostics | Run **PICO-8: Copy Server Diagnostics** | Versions, source, path, platform, cache root, and last error appear; no cart source |
| Upgrade | Install the previous VSIX, then the new VSIX | Extension updates and downloads the matching server |
| Uninstall | Uninstall and clear the cache using the documented path | Extension and cached server are removed as documented |
| Product page | Open the installed extension's details page | README, screenshot, links, license, and changelog render correctly |
| VSIX contents | `vsce ls` and inspect the package | No source, tests, scripts, images, or internal notes beyond the intended assets |
| Screenshot | Capture the running extension on the demo cart | A real screenshot or recording is added to `editors/vscode/images/` |
| Remote/web limitation | Read the product page and docs | Unsupported hosts are clearly stated; no support claim is made |

Run the checks on each platform the test machine supports. Record any unverified
platform as open rather than implying it passed.

## Testing and CI

- `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt -l .`, and
  `golangci-lint run ./...` remain green; this milestone should not change Go
  behavior.
- Extension lint, typecheck, unit tests, integration tests, and VSIX packaging
  remain green.
- `goreleaser check` and a snapshot build remain green.
- `vsce ls` confirms the packaged file set.
- `examples/hello.p8` parses in the `pkg/cart` tests.
- The documentation link check passes.
- The release workflow validates that the extension version matches the tag
  before publishing.
- CI does not publish to either registry from a branch or pull request.
- Manual VSIX verification is recorded in `docs/release-checklist.md`, not
  simulated as a CI pass.

## Exit criteria

- [x] M24 is retitled and scoped as end-user documentation and onboarding;
      registry publication is recorded in the post-v1 backlog and is not an
      M24, roadmap, or release-checklist gate.
- [x] `docs/getting-started.md`, `examples/hello.p8`, and the root README
      quickstart link exist.
- [x] `docs/pico8-workflow.md` covers carts on disk, external editing, reload,
      save conflicts, includes, and read-only PNG carts.
- [x] `docs/types.md` covers the supported LuaCATS features with valid examples.
- [x] `docs/cli.md` documents every flag and `decode`; the top-level help
      limitation is explicit.
- [x] `docs/migrating.md` covers `pico8-ls` and LuaLS coexistence.
- [x] `docs/editors.md` has a tested version matrix and a concrete Helix
      settings example.
- [x] `.github/ISSUE_TEMPLATE/`, `SECURITY.md`, `CONTRIBUTING.md`, and the root
      README support/privacy section exist.
- [x] GitHub description, topics, and homepage are set.
- [ ] A social preview is uploaded in the repository settings.
- [ ] A real screenshot or recording from the manual verification is in the
      root README and `editors/vscode/README.md`.
- [ ] The VSIX-only manual verification is recorded for the available platforms:
      install, first-run download, offline server, cart features, settings, glyph
      picker, status bar, diagnostics, upgrade, and uninstall.
- [ ] `pico8.nvim` has a tagged release and the root README uses the tagged
      install path.
- [x] Public docs no longer promise a Marketplace or Open VSX launch; they state
      that the GitHub VSIX is the current channel.
- [x] The documentation link check passes in CI.
- [x] Changelogs, roadmap, backlog, and `AGENTS.md` are updated.
- [x] No LSP, server, parser, analysis, formatter, or extension feature behavior
      changed.
- [x] Registry publication remains parked in the backlog with the publisher,
      token, publication, and listing-verification steps intact.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Users expect a Marketplace search result | Lead with the GitHub VSIX in the README and product page; say plainly that registries are not live; link checksums and offline setup. |
| The docs-only patch is confused with a feature release | Keep the changelog and release notes explicit; no server or extension behavior changes. |
| Screenshots go stale | Capture from the recorded manual verification and refresh when the UI changes. |
| `pico8.nvim` is untagged or drifts from the server docs | Tag the client release and pin the README snippet; record the tag in the roadmap. |
| PICO-8 save/reload behavior is documented incorrectly | Verify against PICO-8 0.2.7 on Linux, macOS, and Windows before publishing; link the official manual. |
| Type-guide examples drift from the implementation | Use examples covered by the type tests or add doc examples to the test fixtures. |
| Documentation link rot | Add the CI link check and keep external links warning-only if flaky. |
| Registry work is forgotten | Keep the blocked backlog item and the dormant release jobs; the release checklist points at both. |
| Scope creep into feature work | Keep server and extension behavior unchanged; new features need their own milestone. |
