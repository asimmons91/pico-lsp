# M15 — VS Code Client

**Status:** Done — shipped in v1.3.0. macOS/Windows manual VSIX installation and
live first-run download verification remain follow-ups. The full server settings
schema is implemented in M21; server-health reporting is specified by M22.
Remote extension hosts are not planned.

**Depends on:** M14  
**Release target:** v1.3.0  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

VS Code users can install a checked VSIX from the GitHub release, open a PICO-8
workspace, and get the same language-server features as Neovim and Helix without
installing Go or managing a binary. The extension downloads the matching
checksummed `pico-lsp` release on first use, starts it over stdio, contributes
PICO-8 syntax highlighting, and exposes cart budget information in the status
bar.

**Demo:** Install `pico-lsp-vscode-1.3.0.vsix` from the v1.3.0 GitHub release.
Open a folder containing a `.p8` cart. The extension downloads
`pico-lsp_1.3.0_<os>_<arch>` from the same release, verifies `checksums.txt`,
starts the server, and shows diagnostics, completion, hover, code actions,
formatting, and code lenses. The status bar shows the cart's token, character,
and compressed-size budgets. Opening a `.lua` file in that workspace attaches
the server; opening a `.lua` file in a non-PICO-8 workspace does not.

## Decisions

These decisions were confirmed before the spec was written:

- The extension downloads a checksummed GitHub release archive instead of
  bundling binaries or requiring a user-managed server.
- Publisher/extension ID: `asimmons91.pico-lsp`.
- The extension version pins the matching server release version.
- All `.lua` files in workspace folders that contain a `.p8` cart attach to the
  server.
- M15 publishes a VSIX only as a GitHub release asset. VS Code Marketplace and
  Open VSX publishing are deferred to the M24 follow-up (proposed).
- M15 supports local desktop VS Code only. Remote extension hosts are not
  supported; web extension hosts remain out of scope.
- Read-only `.p8.png` virtual documents are out of scope.
- Node.js 26.13.1 is pinned in `mise.toml`; the extension declares VS Code
  `^1.95.0` and integration tests run against that minimum version.

## Scope

### In scope

- A TypeScript extension under `editors/vscode/`, bundled for distribution.
- Download, checksum verification, caching, and execution of the matching
  `pico-lsp` release binary.
- `pico8.serverPath` override for offline, development, and custom installs.
- Start/stop/restart lifecycle for one language client over stdio.
- `.p8` language contribution, TextMate syntax highlighting, and language
  configuration.
- Attachment to `.p8` files and to `.lua` files in PICO-8 workspace folders.
- Server configuration forwarding through `workspace/configuration` and
  `initializationOptions.pico8`.
- Status bar cart budgets using the `pico8/cartInfo` request.
- Commands for cart info, server restart, output, and cache cleanup.
- Output channel and `pico8.trace.server` logging.
- Unit tests, VS Code integration tests, VSIX packaging, and CI coverage.
- Release workflow changes that build `pico-lsp-vscode-<version>.vsix` and
  attach it to the GitHub release.
- README, install, configuration, troubleshooting, and release-checklist
  documentation.
- A post-v1 backlog entry for Marketplace/Open VSX publishing.

### Out of scope

- VS Code Marketplace and Open VSX publishing.
- Web extension hosts. Remote SSH, WSL, Dev Containers, and Codespaces are not
  supported.
- Read-only `.p8.png` virtual documents.
- Auto-updating the server independently of the extension.
- Bundling platform binaries inside the VSIX.
- Telemetry, analytics, crash reporting, or network calls other than the
  explicit server download.
- Code signing, notarization, or binary attestation beyond SHA-256 checksums.
- A debug adapter, cart runner, emulator, or sprite/audio previews.
- Managing or disabling other Lua language servers automatically.
- Zed integration.

## Extension layout

```text
editors/vscode/
  package.json
  tsconfig.json
  esbuild.mjs
  src/
    extension.ts
    client.ts
    server/
      resolve.ts
      download.ts
      checksum.ts
      platforms.ts
    workspace.ts
    statusBar.ts
    commands.ts
    config.ts
    log.ts
  syntaxes/pico8.tmLanguage.json
  language-configuration.json
  test/
  README.md
  CHANGELOG.md
  LICENSE
```

- Use TypeScript with strict type checking.
- Bundle the extension entry point with esbuild into `dist/extension.js`.
- Pin dependencies with a committed `package-lock.json`; CI uses `npm ci`.
- Declare the minimum supported VS Code engine in `package.json`; use a stable
  languageclient version compatible with that engine.
- Keep the extension source independent of the Go module; it communicates only
  through the documented LSP surface and GitHub release assets.

## Activation and language support

### Activation events

Activate on:

- `onLanguage:pico8`
- `workspaceContains:**/*.p8`
- the extension's contributed commands

Do not use `*` or `onStartupFinished`.

### PICO-8 workspace detection

A workspace folder is PICO-8-enabled when it contains at least one `.p8` file.
Detection must:

- run per workspace folder in multi-root workspaces;
- use `vscode.workspace.findFiles` with a bounded include and standard excludes
  (`.git`, `node_modules`, `dist`, `out`);
- refresh on workspace-folder changes and on `.p8` create/delete events;
- restrict `.lua` attachment to PICO-8-enabled workspace folders.

`.p8` files in any workspace folder attach. A standalone `.p8` file opened
without a workspace folder may attach when the language is `pico8`.

### `.lua` attachment

When `pico8.attachLua` is `true` (default):

- Attach the server to every `.lua` file in a PICO-8-enabled workspace folder.
- Do not attach to `.lua` files in other workspace folders.
- Do not attach to standalone `.lua` files outside a PICO-8-enabled folder.
- Restart the client when the set of PICO-8-enabled folders changes so document
  selectors stay accurate.
- Document that another Lua language server should be disabled for `.lua` files
  in PICO-8 workspace folders to avoid duplicate diagnostics and competing
  completions. The extension must not disable other extensions itself.

When `pico8.attachLua` is `false`, attach only to `.p8` files.

### Language contribution

- Register language ID `pico8`, display name `PICO-8`, alias `PICO-8 Cart`, and
  extension `.p8`.
- Add a TextMate grammar with scope `source.pico8` covering:
  - section headers (`__lua__`, `__gfx__`, `__gff__`, `__label__`, `__map__`,
    `__sfx__`, `__music__`, and unknown sections);
  - tab markers (`-->8`);
  - `--` and `//` comments;
  - strings and numbers;
  - PICO-8 keywords, operators, shorthand forms, and peek operators (`@`, `%`,
    `$`).
- Add `language-configuration.json` for comments, brackets, auto-closing pairs,
  and indentation.
- Post-v1: provide the PICO-8 logo as the language default icon for `.p8`
  files, with a third-party notice. Themes that set
  `showLanguageModeIcons: false` keep their own icon.
- Do not contribute the `lua` language. `.lua` files keep the editor's Lua
  language mode and receive LSP semantic tokens from `pico-lsp`.
- `.p8.png` remains unassociated binary data in M15.

## Server acquisition

### Version selection

- The extension version and the required server version are identical. For
  extension `1.3.0`, download server `v1.3.0`.
- The release workflow validates that `editors/vscode/package.json` matches the
  pushed tag and fails the release otherwise.
- Do not fall back to `latest` when the matching release or asset is missing.
- Development builds may use `pico8.serverPath`; they must not silently download
  a release server.

### Resolution order

At server start:

1. If `pico8.serverPath` is set, validate that the path exists, is a regular
   file, and is executable, then use it. Do not auto-download when an explicit
   path is configured.
2. Otherwise use a cached verified binary for the extension version, platform,
   and architecture.
3. Otherwise, when `pico8.autoDownload` is `true` (default), download and verify
   the matching release archive.
4. Otherwise fail with an actionable error that explains `pico8.serverPath` and
   `pico8.autoDownload`.

### Supported targets

Local desktop builds only:

| Platform | Architectures | Asset |
|---|---|---|
| Linux | amd64, arm64 | `pico-lsp_<version>_linux_<arch>.tar.gz` |
| macOS | amd64, arm64 | `pico-lsp_<version>_darwin_<arch>.tar.gz` |
| Windows | amd64, arm64 | `pico-lsp_<version>_windows_<arch>.zip` |

- Map `process.platform` and `process.arch` explicitly; reject unsupported
  combinations with a clear message.
- Download from
  `https://github.com/asimmons91/pico-lsp/releases/download/v<version>/`.
- Follow HTTPS redirects. Honor VS Code `http.proxy` and `http.proxyStrictSSL`
  settings for the download; if the configured proxy cannot be used, fail with a
  clear error that points at `pico8.serverPath` or `pico8.autoDownload`.

### Verification and cache

- Download the archive and `checksums.txt`.
- Require exactly one checksum entry for the selected archive.
- Verify SHA-256 before extraction; never execute an unverified binary.
- Extract only the expected `pico-lsp` or `pico-lsp.exe` member. Reject absolute
  paths, `..` traversal, symlinks, and unexpected members.
- Store the executable under the extension's `globalStorageUri`, keyed by
  version, OS, and architecture.
- Write to a temporary directory and atomically rename after verification.
- Serialize concurrent downloads with an in-process lock; a second request waits
  for the first.
- Make the binary executable on Unix.
- On failure, remove partial files and show an error with **Retry**, **Open
  Settings**, and **Show Log** actions.
- Do not cache a failed or unverified download.

### Server process

- Start `pico-lsp` with stdio transport.
- Start one language client per VS Code window. Use the workspace as the client
  root and advertise all `workspace/workspaceFolders`; do not start one client
  per folder.
- Send `initializationOptions.pico8` from the effective VS Code settings as a
  fallback, and advertise `workspace.configuration` so the server's
  `workspace/configuration` request works.
- Advertise `workspace.didChangeWatchedFiles.dynamicRegistration` and let the
  server's dynamic registrations drive `.p8`/`.lua` include reanalysis. Do not
  add duplicate static watchers.
- Capture stderr in the **PICO-8 Language Server** output channel.
- On unexpected exit, show a warning with the output channel and restart
  command. Do not restart-loop automatically.
- Stop and dispose the client on extension deactivation.
- Abort activation with a clear message when `vscode.env.remoteName` is set:
  M15 supports local desktop windows only.

## Commands

Contribute:

| Command | Behavior |
|---|---|
| `pico8.client.showCartInfo` | Call the server's `pico8.showCartInfo` command for the active cart; show the summary in the editor. |
| `pico8.client.restartServer` | Restart the language client without reloading VS Code. |
| `pico8.client.showOutput` | Reveal the **PICO-8 Language Server** output channel. |
| `pico8.client.clearServerCache` | Confirm and delete cached server binaries for this extension version. |

Extension command IDs use the `pico8.client.*` namespace because
`vscode-languageclient` automatically registers server-declared commands such as
`pico8.showCartInfo`. Keep the wrapper commands distinct so registering the
client never collides with the server command table.

Commands must report a clear error when no server is running or the active
editor is not part of a cart.

## Status bar

- One status bar item, visible only when the active editor is a `.p8` cart or an
  attached `.lua` file and the server is ready.
- Request `pico8/cartInfo` for the active document.
- Display token, character, and compressed budgets in a compact form, for
  example:
  `PICO-8 4210/8192 tok · 27433/65535 chr · 12.0/15.3 KB`
- Show a warning state at or above 90% of any budget and an error state above a
  limit, including `compressedStatus`.
- When `compressedStatus` is `unavailable`, show `compressed n/a` rather than a
  warning or error.
- Use a tooltip with per-file contributions and the cart root.
- Refresh on active-editor changes, debounced document changes (250–500 ms),
  saves, and server reconnection.
- Cancel superseded requests when the active editor or version changes.
- Clicking the item runs `pico8.client.showCartInfo`.
- If the server returns `cart-not-found` or the document is a `.p8.png`, hide
  the item.

## Settings

Extension-owned settings, contributed in `package.json`:

| Setting | Type | Default | Scope | Meaning |
|---|---|---|---|---|
| `pico8.serverPath` | string | `""` | machine | Explicit `pico-lsp` executable. Disables auto-download. |
| `pico8.autoDownload` | boolean | `true` | window | Download the matching verified server when it is not cached. |
| `pico8.attachLua` | boolean | `true` | window | Attach to `.lua` files in PICO-8 workspace folders. |
| `pico8.trace.server` | enum `off`/`messages`/`verbose` | `off` | window | Language client trace level. |

The server's existing `pico8.*` settings remain authoritative in
[configuration.md](configuration.md). The extension must forward the whole
`pico8` section through configuration requests; it must not invent parallel
server settings. Extension-only keys are ignored by the server.
M15 contributes only `pico8.glyphs` from the server section; M21 adds the full
settings schema so the remaining server settings appear in the Settings UI.

Changing `pico8.serverPath`, `pico8.autoDownload`, or `pico8.attachLua` prompts
or offers a restart through `pico8.client.restartServer`; the extension must not leave a
client running against a removed or replaced binary.

## Security and trust

- Download over HTTPS from the project's GitHub release only.
- Verify SHA-256 before execution.
- Never execute an archive member other than `pico-lsp`/`pico-lsp.exe`.
- Do not add telemetry or analytics.
- Do not send workspace contents anywhere except through the local language
  server.
- Set `capabilities.untrustedWorkspaces.supported` to `false`; the extension
  downloads and executes a binary and must not run in untrusted workspaces.
- Set `capabilities.virtualWorkspaces.supported` to `false`.
- Do not support web extension hosts.
- Logs may contain file paths; do not log source text unless trace logging is
  explicitly enabled.

## Testing

### Unit tests

- Platform/architecture to asset-name mapping for all six targets.
- Rejection of unsupported platforms.
- Checksum parsing, missing entries, malformed lines, and mismatches.
- Archive extraction safety, including traversal and unexpected members.
- Cache paths, atomic install, concurrent download serialization, and cleanup.
- `pico8.serverPath` validation and auto-download suppression.
- PICO-8 workspace detection, multi-root scoping, and `.lua` attachment.
- Status bar text and warning/error thresholds.
- Version pinning and release-tag validation helper.

### Integration tests

Use `@vscode/test-electron` against a test workspace and a prebuilt test
`pico-lsp` binary supplied through `pico8.serverPath`, so normal integration
tests do not require network access.

Assert:

- extension activation and server start;
- `.p8` diagnostics, completion, hover, semantic tokens, formatting, code
  lenses, and code actions;
- `pico8/cartInfo` and status bar updates;
- `.lua` attachment inside a PICO-8 folder;
- no attachment to `.lua` outside PICO-8 folders;
- restart and output commands;
- clear failure messages for a missing `serverPath`;
- remote-extension-host refusal based on `vscode.env.remoteName` (unit-level
  where a remote host is unavailable).

Run integration tests on Linux under `xvfb-run` in CI. Build the Go server
before the test job.

### Download tests

Use a local mock HTTP server that serves fixture archives and checksums. Cover:

- successful download, verification, extraction, and cache reuse;
- checksum mismatch;
- missing asset or checksum;
- interrupted download and cleanup;
- `autoDownload: false`;
- retry after failure.

### Packaging and release tests

- `npm run package` builds `pico-lsp-vscode-<version>.vsix`.
- Verify the VSIX contains `dist/extension.js`, the TextMate grammar,
  language configuration, README, CHANGELOG, and LICENSE; it must not contain
  tests, source maps, or downloaded binaries.
- PR CI runs lint, typecheck, unit tests, integration tests, and VSIX packaging.
- The release workflow checks `package.json` version against the tag, builds the
  VSIX, and attaches it to the GitHub release.

## Release and documentation

### CI

Add a `vscode` job to `.github/workflows/ci.yml`:

- set up Node with the version pinned in the extension package metadata;
- `npm ci`;
- lint and typecheck;
- unit tests;
- build the Go server and run VS Code integration tests under `xvfb-run`;
- package the VSIX and validate its contents.

Keep the Go build matrix and existing gates unchanged.

### Release workflow

Extend `.github/workflows/release.yml`:

- validate `editors/vscode/package.json` version equals the tag without `v`;
- build the Go server and extension;
- run extension unit/integration tests;
- produce `pico-lsp-vscode-<version>.vsix` and
  `pico-lsp-vscode-<version>.vsix.sha256`;
- upload both to the GitHub release alongside the platform archives and
  `checksums.txt`;
- ensure the release notes mention the VSIX asset and how to install it;
- do not publish to Marketplace or Open VSX.

### Documentation

- `editors/vscode/README.md`: install from VSIX, first-run download, proxy/offline
  behavior, `pico8.serverPath`, Lua/LuaLS guidance, commands, status bar,
  settings, logs, limitations, and manual server setup.
- Root `README.md`: VS Code install section pointing at the release VSIX,
  Marketplace deferred.
- `docs/install.md`: VSIX installation and upgrade/uninstall path.
- `docs/configuration.md`: extension-only settings and their scope.
- `docs/troubleshooting.md`: download/checksum failures, proxy/offline use,
  server crashes, LuaLS conflicts, output channel, remote/web/PNG limitations.
- `docs/release-checklist.md`: extension version validation, VSIX build,
  installation smoke test, and release-asset verification.
- `CHANGELOG.md`: M15 entry.
- `docs/ROADMAP.md` and [post-v1-backlog.md](post-v1-backlog.md): mark the VS
  Code client complete and leave Marketplace/Open VSX publishing as the
  remaining follow-up.

## Exit criteria

- [x] A checked `pico-lsp-vscode-1.3.0.vsix` and its SHA-256 file are attached
      to the GitHub release and install on Linux local desktop VS Code.
- [ ] The same VSIX is manually verified on macOS and Windows local desktop
      VS Code.
- [x] The downloader handles mock release assets, verifies checksums, and starts
      the cached server.
- [ ] Live first-run download from the public v1.3.0 release is verified in a
      real VS Code window.
- [x] `pico8.serverPath` runs a user-supplied server without any download.
- [x] `.p8` files attach automatically and receive diagnostics, completion,
      hover, semantic tokens, formatting, code lenses, and code actions.
- [x] All `.lua` files in PICO-8 workspace folders attach; unrelated Lua files
      do not.
- [x] The status bar shows cart budgets and updates after edits; clicking it
      shows the cart summary.
- [x] Restart is covered by the integration suite; output and cache-cleanup
      paths are implemented.
- [x] Missing assets, checksum failures, and offline `serverPath` use fail with
      actionable messages; web hosts and `.p8.png` remain documented
      unsupported paths; remote hosts are not supported.
- [x] No unverified binary is ever executed.
- [x] CI runs extension checks and integration tests; the release workflow
      attaches the VSIX.
- [x] Documentation, configuration, troubleshooting, release checklist, roadmap,
      and backlog are updated.
- [x] Existing v1/v1.1/v1.2 behavior and performance budgets do not regress.

## Follow-ups

- [M21 — Full Server Settings Schema in the VS Code Client](M21-vscode-settings.md)
  surfaces the server's `pico8.*` settings in the Settings UI.
- [M22 — VS Code Server Health and Version Reporting](M22-vscode-server-health.md)
  adds server version checks and copyable diagnostics.
- [M24 — VS Code Marketplace Launch and Documentation Cleanup](M24-vscode-marketplace.md)
  proposes Marketplace/Open VSX publication, public-documentation cleanup, and
  manual VS Code verification.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| First-run download fails offline or behind a proxy | Checksummed cache, retry UI, proxy support, and `pico8.serverPath` fallback. |
| Extension and server versions drift | Extension version pins the server; release workflow validates the tag/package version. |
| LuaLS and pico-lsp both attach to `.lua` | Per-workspace `.lua` scoping, `pico8.attachLua`, and documented LuaLS guidance. |
| A malicious or corrupted archive is executed | HTTPS, checksum verification, safe extraction, atomic install, no unverified execution. |
| VS Code API or languageclient drift | Pin the engine and dependencies, keep integration tests, and avoid editor-specific server changes. |
| Marketplace publishing blocks the release | Ship the VSIX as a GitHub release asset; M24 proposes registry publishing and keeps the VSIX as the fallback. |
| Web expectations | Reject web extension hosts with a clear message; remote hosts are not supported. |
| `.p8.png` opens as binary noise | Document the limitation and track virtual-document support separately. |
| Large status-bar request traffic | Debounce, cancel superseded requests, and refresh only for cart-owned editors. |
