# M20 — Remote VS Code Extension Hosts

**Status:** Proposed — not started

**Depends on:** M15  
**Release target:** v1.8.0  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

VS Code users can open a remote workspace over Remote - SSH, WSL, Dev
Containers, or a Codespaces remote extension host and get the same PICO-8
support as local desktop. The extension runs in the workspace extension host,
downloads or reuses the matching `pico-lsp` binary for the remote operating
system and architecture, and caches it in the remote extension storage.
`pico8.serverPath` is interpreted on the remote machine. Local desktop behavior
is unchanged, and web extension hosts remain unsupported.

**Demo:** From a laptop, connect VS Code to a Linux x64 SSH host or dev
container and open a folder containing `cart.p8`. The extension installs in the
remote extension host, downloads `pico-lsp_1.8.0_linux_amd64.tar.gz`, verifies
`checksums.txt`, starts the server, and provides diagnostics, completion,
formatting, code actions, cart budgets, and glyph insertion. Setting
`pico8.serverPath` in Remote settings to `/usr/local/bin/pico-lsp` uses the
remote binary without a download. Opening the same folder in a web extension
host still shows a clear unsupported-host message.

## Decisions proposed

| ID | Decision | Choice |
|---|---|---|
| D1 | Where the extension runs | Declare `"extensionKind": ["workspace"]`; in a remote window the extension runs in the remote extension host. |
| D2 | Server target | Use the remote host's `process.platform` and `process.arch`; support the existing six local desktop targets. |
| D3 | Server cache | Use the remote `context.globalStorageUri`; the download, checksum, extraction, and atomic-install rules are unchanged. |
| D4 | `pico8.serverPath` | Interpret it as a path on the remote machine. It remains machine-scoped and still disables auto-download. |
| D5 | Supported hosts | Remote SSH, WSL, Dev Containers, and Codespaces remote extension hosts on desktop VS Code. Web extension hosts (`vscode.dev`, browser-only Codespaces) remain unsupported. |
| D6 | Workspace trust | Keep `untrustedWorkspaces.supported: false`; remote workspaces must be trusted before the extension downloads or executes a server. |
| D7 | Proxy | Read VS Code's `http.proxy` and `http.proxyStrictSSL` in the remote extension host; document that these are remote/user settings. |
| D8 | Server contract | No LSP protocol or server changes. The server remains stdio-only and editor-agnostic. |
| D9 | UI | Status bar, commands, cart info, output channels, and the glyph picker must keep working through the remote extension host. |
| D10 | Release verification | Require a Remote SSH Linux smoke test and one additional remote smoke test on WSL or a container-based host. macOS and Windows remote hosts use the existing target mapping and are checked in the release checklist when available. |

## Scope

### In scope

- `extensionKind` and remote-host detection in the extension manifest and entry
  point.
- Remote server target resolution, download, checksum verification, caching,
  and cleanup.
- Remote `pico8.serverPath` validation and documentation.
- Workspace detection and `.lua` attachment over remote file systems.
- Remote-window behavior for the status bar, commands, output channels, cart
  info, and glyph picker.
- Unit tests for host detection and manifest declarations.
- A documented manual remote smoke matrix.
- README, install, configuration, troubleshooting, release-checklist, roadmap,
  and changelog updates.

### Out of scope

- Web extension hosts, browser-only Codespaces, and a WASM server build.
- Remote debugging, port forwarding, SSH configuration, or Dev Container
  definitions.
- Auto-updating the server independently of the extension.
- Bundling platform binaries inside the VSIX.
- Marketplace and Open VSX publishing.
- Telemetry, crash reporting, or new network calls.
- LSP server changes or editor-specific server behavior.
- `.p8.png` virtual documents, settings-schema expansion, and server-health
  reporting; those are separate milestones.

## Deliverables

1. `editors/vscode/package.json` with `"extensionKind": ["workspace"]`.
2. A testable host-support helper that rejects web extension hosts and allows
   local desktop and remote desktop hosts.
3. Remote host wiring in `editors/vscode/src/extension.ts` and
   `editors/vscode/src/client.ts`.
4. Unit tests for host detection, target mapping, and the manifest declaration.
5. Documentation for installing the extension and VSIX in a remote window,
   remote `pico8.serverPath`, proxy use, and cache cleanup.
6. A remote smoke-test checklist in `docs/release-checklist.md`.
7. `CHANGELOG.md` and `v1.8.0` release notes.

## Detailed requirements

### 1. Extension host and manifest

- Add `"extensionKind": ["workspace"]` to `editors/vscode/package.json`.
- Replace the unconditional `vscode.env.remoteName` refusal in
  `editors/vscode/src/extension.ts` with a guard that rejects only web extension
  hosts. Desktop remote hosts must activate normally.
- Implement the guard as a pure helper, for example
  `unsupportedHostReason(uiKind, remoteName)`, so it can be unit tested without
  starting VS Code.
- Allow local desktop hosts (`remoteName` unset, desktop UI), Remote - SSH,
  WSL, Dev Containers, attached containers, and Codespaces remote hosts.
- Reject `vscode.UIKind.Web` with an actionable message explaining that the
  server is a native Go binary and that the folder must be opened in desktop
  VS Code.
- Do not add `onStartupFinished` or `*` activation. The existing activation
  events still apply.
- Keep `capabilities.untrustedWorkspaces.supported` and
  `capabilities.virtualWorkspaces.supported` unchanged. Remote file systems are
  not virtual workspaces in the VS Code sense.

### 2. Server resolution, download, and cache

- Resolve the target from the remote extension host's `process.platform` and
  `process.arch`. The existing Linux, macOS, and Windows amd64/arm64 mapping is
  authoritative.
- Cache under the remote `context.globalStorageUri`, keyed by extension/server
  version, OS, and architecture, exactly as the local client does.
- Keep the current resolution order: explicit `pico8.serverPath`, verified
  cache, auto-download when enabled, then an actionable error.
- Keep SHA-256 verification, safe archive extraction, atomic rename, and Unix
  executable permissions unchanged. Never execute an unverified or partially
  extracted binary.
- If the remote platform or architecture is unsupported, fail with the same
  clear target error used locally.
- If the remote host blocks GitHub or the configured proxy is unusable, show
  the existing Retry / Open Settings / Show Log actions and point at
  `pico8.serverPath`.
- `pico8.client.clearServerCache` clears the remote cache and its confirmation
  text says that the cache is on the remote machine when `remoteName` is set.
- Development builds still require `pico8.serverPath`; do not fall back to a
  release download in a development extension host.

### 3. Settings and paths

- Document that `pico8.serverPath` refers to the remote machine in a remote
  window. It remains machine-scoped and disables auto-download.
- Read `http.proxy` and `http.proxyStrictSSL` in the remote extension host.
  Document that Remote settings and workspace settings may override user
  settings there.
- Keep `pico8.autoDownload` and `pico8.attachLua` window-scoped.
- Workspace detection and `.lua` ownership continue to use `file` URIs in the
  remote extension host; no new URI schemes are introduced in this milestone.
- Server `cwd` remains the single remote workspace folder when exactly one
  folder is PICO-8-enabled.

### 4. UI and lifecycle parity

- The status bar must update for the active cart or attached `.lua` file in a
  remote window.
- `pico8.client.showCartInfo`, `pico8.client.insertGlyph`,
  `pico8.client.restartServer`, `pico8.client.showOutput`, and
  `pico8.client.clearServerCache` must work in remote windows.
- Output and trace channels remain in the remote extension host and are visible
  in the remote window.
- Server crashes continue to show the existing warning with Show Log and
  Restart actions; do not add automatic restart loops.
- If the extension is installed locally but not in the remote host, the
  extension must not run a local server against remote files. Document how VS
  Code prompts to install a workspace extension remotely and how to install a
  VSIX in the remote window.

### 5. Security and privacy

- Keep the existing HTTPS GitHub-release source, SHA-256 verification, and
  safe extraction rules.
- Do not add telemetry, analytics, crash reporting, or network calls beyond the
  server download.
- Do not open inbound ports or start a TCP listener.
- Logs may contain remote file paths; do not log source text unless trace
  logging is explicitly enabled.
- The extension remains disabled in untrusted workspaces.

### 6. Testing

Unit tests:

- host-support helper for local desktop, SSH, WSL, Dev Container, attached
  container, Codespaces remote, and web;
- manifest test asserting `extensionKind` is `["workspace"]`;
- target mapping remains correct for all six release assets;
- `pico8.serverPath` validation with Unix and Windows remote path shapes where
  practical;
- cache and cleanup behavior with a remote-style global-storage root.

Integration tests:

- Existing local desktop tests must pass without behavior changes.
- Keep the integration suite on `pico8.serverPath` so it does not require
  network access.
- Add an assertion that commands and the status bar still work after a client
  restart in the local host; remote transport is covered by the manual matrix.

Manual remote matrix (documented in `docs/release-checklist.md`):

| Host | Architecture | Checks |
|---|---|---|
| Remote - SSH, Linux | amd64 | install, download, checksum, cart features, status bar, cache cleanup |
| WSL | amd64 or arm64 | install, attach to `.lua`, glyph insertion |
| Dev Container or Codespaces remote | amd64 or arm64 | install, download, proxy/offline `serverPath` |
| Remote - SSH, macOS | arm64 | target selection and `serverPath` when available |
| Remote - SSH, Windows | amd64 | zip extraction and `pico-lsp.exe` when available |

The Remote SSH Linux row and at least one of the WSL or Dev
Container/Codespaces rows are release-blocking. The macOS/Windows rows are
required when the maintainer has access to those hosts; otherwise the
limitation is recorded in the release checklist.

### 7. Documentation and release

- `editors/vscode/README.md`: remote support, install-in-remote, remote
  `pico8.serverPath`, proxy, and cache-cleanup notes; remove the blanket
  “local desktop only” limitation and keep the web limitation.
- `docs/install.md`: installing the extension or VSIX in a remote window.
- `docs/configuration.md`: remote meaning of `pico8.serverPath` and proxy
  settings.
- `docs/troubleshooting.md`: remote download failures, proxy/offline use,
  remote cache location, and the install-locally-vs-remotely mistake.
- `docs/release-checklist.md`: the remote smoke matrix.
- `CHANGELOG.md` and `v1.8.0` release notes.
- Link this milestone from `docs/ROADMAP.md` and the post-v1 backlog.

## Interfaces and state

- No new public server API, LSP request, or command is required.
- The extension's host support is determined from `vscode.env.uiKind` and
  `vscode.env.remoteName` at activation.
- Server paths, cache keys, and settings remain owned by the extension host in
  which the extension runs.
- The remote extension host is the only new deployment context; no server state
  moves to the UI host.

## Error handling

- Web extension host: one clear message, then stop activation.
- Unsupported remote platform/architecture: retain the existing actionable
  target error.
- Remote download failure: retain Retry, Open Settings, and Show Log.
- Remote `pico8.serverPath` missing, non-file, or non-executable: fail before
  starting the server with the path named in the error.
- Remote cache is not writable: fail with the cache path and suggest
  `pico8.serverPath` or fixing remote permissions.
- Missing remote extension installation: document the VS Code prompt and VSIX
  path; do not attempt to copy a local binary to the remote host.

## Testing and CI

- `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt -l .`, and
  `golangci-lint run ./...` remain green.
- Extension `npm ci`, lint, typecheck, unit tests, integration tests, and VSIX
  packaging remain green.
- Add the host-support unit tests and manifest assertion to the normal
  extension unit suite.
- Add the remote matrix to the release checklist; a remote CI runner is not
  required for this milestone.
- The release workflow still validates that the extension version matches the
  tag.

## Exit criteria

- [ ] The extension declares `"extensionKind": ["workspace"]` and activates in
      Remote - SSH, WSL, and a container-based remote host.
- [ ] Remote windows download, verify, cache, and start the matching server for
      the remote platform and architecture.
- [ ] `pico8.serverPath` works with a remote path and suppresses downloads.
- [ ] Diagnostics, completion, formatting, code actions, cart budgets, glyph
      insertion, and cache cleanup work in a remote window.
- [ ] Web extension hosts are rejected with a clear message.
- [ ] Local desktop behavior and all existing tests are unchanged.
- [ ] The remote smoke matrix is recorded; Remote SSH Linux and at least one
      of WSL/Dev Container/Codespaces pass.
- [ ] `pico8.serverPath`, proxy, cache, and install documentation is updated.
- [ ] No LSP protocol, server behavior, telemetry, or download-security change.
- [ ] `v1.8.0` is tagged after the release PR merges.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Extension runs on the UI side and starts a local server for remote files | `extensionKind: ["workspace"]`; host-support unit test and remote smoke test. |
| Remote users cannot install the extension or VSIX | Document both VS Code's remote install prompt and remote VSIX installation. |
| Remote hosts block GitHub or have a different proxy | Use remote `http.proxy`/`http.proxyStrictSSL`; keep actionable errors and `pico8.serverPath`. |
| Wrong binary for the remote platform | Reuse the six-target mapping and add architecture checks to the smoke matrix. |
| Remote cache grows across extension updates | Keep the existing cache-clear command and document the remote cache path. |
| Remote path semantics confuse machine-scoped settings | Document that machine scope means the remote machine in a remote window. |
| A future VS Code remote type behaves like web | Reject only `UIKind.Web`; add new remote names to the allow path rather than blocking unknown desktop hosts. |
