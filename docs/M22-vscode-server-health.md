# M22 — VS Code Server Health and Version Reporting

**Status:** Proposed — not started

**Depends on:** M15  
**Release target:** v1.9.0 (may ship with M21)  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

The VS Code extension reports which `pico-lsp` binary is running, where it came
from, and whether its version matches the extension. A custom
`pico8.serverPath` that points at an older or newer release server produces one
clear, non-blocking warning with actionable commands; development builds are
labeled without a mismatch warning. Users and maintainers can copy a
diagnostics block containing the extension version, server version, source,
platform, cache path, and last startup error without opening a debugger. The
LSP server and protocol do not change.

**Demo:** Set `pico8.serverPath` to a `pico-lsp` v1.7.0 binary while running the
1.9.0 extension. The server still starts, but VS Code shows one warning:
“pico-lsp v1.7.0 does not match the PICO-8 extension 1.9.0.” The actions are
**Show Server Info**, **Open Settings**, and **Show Log**. Running **PICO-8:
Copy Server Diagnostics** places a Markdown block with both versions and the
binary path on the clipboard for an issue report. A development build reports
“development build” without a mismatch warning.

## Decisions proposed

| ID | Decision | Choice |
|---|---|---|
| D1 | Version source | Read `ServerInfo.Version` from the language client's `initializeResult`; do not spawn an extra process or add a server request. |
| D2 | Normalization | Trim whitespace and one leading `v`; compare plain `X.Y.Z` release versions by exact string. Do not add semver-range compatibility logic. |
| D3 | Mismatch policy | Warn once per server start, never block or auto-restart. The server remains usable. |
| D4 | Development versions | `dev`, empty, pre-release, and Go pseudo-versions are labeled “development build” and do not produce a version-mismatch warning. |
| D5 | Source tracking | Track `serverPath`, `cache`, or `download` through server resolution so diagnostics explain where the binary came from. |
| D6 | Commands | Add `pico8.client.showServerInfo` and `pico8.client.copyServerDiagnostics`. |
| D7 | Show Server Info | Write a structured block to the existing **PICO-8 Language Server** output channel and reveal it. It works even when the server is not running. |
| D8 | Copy Server Diagnostics | Copy the same information as Markdown to the clipboard and show a short confirmation. |
| D9 | Privacy | Include versions, paths, platform, cache location, and last error; never include source text or cart contents. |
| D10 | Release | M22 may ship together with M21 as v1.9.0; the milestones remain independently reviewable. |

## Scope

### In scope

- Reading and normalizing the server version after `initialize`.
- Tracking the resolved server source and path.
- A one-time mismatch warning with actions.
- `Show Server Info` and `Copy Server Diagnostics` commands.
- Server information for running, failed, and stopped clients.
- Unit tests for version classification, formatting, and source tracking.
- Integration tests for the matching server, commands, and clipboard output.
- README, troubleshooting, configuration, changelog, roadmap, and backlog
  updates.

### Out of scope

- Enforcing a compatible version range or refusing to start a mismatched
  server.
- Auto-updating or auto-replacing a custom server.
- Downloading a different server version on demand.
- New LSP requests, server commands, or server changes.
- Telemetry, crash reporting, or network calls.
- Remote host support and the full settings schema; those are M20 and M21.
- `.p8.png` virtual documents.

## Deliverables

1. `editors/vscode/src/server/version.ts` with version normalization and
   classification helpers.
2. `ResolvedServer` metadata from `resolveServer` and `resolveWithProgress`.
3. Server details tracked by `PicoClient` for running, failed, and stopped
   states.
4. `pico8.client.showServerInfo` and `pico8.client.copyServerDiagnostics`
   commands and manifest entries.
5. A one-time mismatch warning after startup.
6. Unit and integration tests.
7. Documentation, changelog, and release notes.

## Detailed requirements

### 1. Version classification

Implement pure helpers, for example:

```ts
type ServerVersionStatus = 'match' | 'mismatch' | 'development' | 'unknown';

function normalizeVersion(value: string): string;
function isReleaseVersion(value: string): boolean;
function classifyServerVersion(
  extensionVersion: string,
  serverVersion: string | undefined
): ServerVersionStatus;
```

Rules:

- Trim whitespace and strip one leading `v`/`V` before comparing.
- A release version is exactly `X.Y.Z` with non-negative decimal components.
  Pre-release suffixes, build metadata, `dev`, empty strings, and Go
  pseudo-versions are not release versions.
- `match` when both normalized values are release versions and equal.
- `mismatch` when both are release versions and differ.
- `development` when the server version is present but not a release version.
- `unknown` when the server does not report a version.
- Do not use a semver library and do not implement range compatibility in this
  milestone.
- The server currently reports versions with a `v` prefix (`v1.7.0`) while the
  extension manifest uses `1.7.0`; normalization must bridge that difference.

### 2. Server details

Track at least:

```ts
interface ServerDetails {
  extensionVersion: string;
  serverVersion?: string;
  versionStatus: ServerVersionStatus;
  source: 'serverPath' | 'cache' | 'download' | 'unknown';
  serverPath?: string;
  platform: string;
  arch: string;
  cacheRoot: string;
  workspaceFolders: number;
  attachLua: boolean;
  running: boolean;
  lastError?: string;
}
```

Requirements:

- `resolveServer` returns `{ path, source }` instead of only a path. Keep the
  resolution order and behavior unchanged.
- `source` is `serverPath` when `pico8.serverPath` is used, `cache` when a
  verified cached binary is reused, and `download` when the extension downloads
  a server during this start.
- Capture `client.initializeResult?.serverInfo?.version` after
  `client.start()` resolves.
- Record `lastError` when server resolution or startup fails.
- Clear stale details when a restart begins, but keep the last error available
  to the info command if the next start fails.
- The details must be available through a method or extension export for the
  integration test, for example `getServerInfo()`.
- Do not expose source text, cart contents, or environment variables.

### 3. Mismatch warning

After a successful start:

- `match`: no warning.
- `development`: no warning; show “development build” in Server Info.
- `unknown`: show one warning that the server did not report a version, with
  **Show Server Info** and **Show Log**.
- `mismatch` with source `serverPath`: show one warning containing both
  versions, with **Show Server Info**, **Open Settings**, and **Show Log**.
- `mismatch` with source `cache` or `download`: show one warning containing
  both versions, with **Clear Server Cache**, **Show Server Info**, and
  **Show Log**. Clearing the cache remains a user action and does not stop the
  running server; the user restarts through the existing command and the next
  start downloads again. Never clear the cache automatically.

The warning appears at most once per client start. A manual restart may warn
again. It must not block activation, change the document selector, stop the
server, or modify settings. The server's features continue to work while the
warning is visible.

### 4. Commands

`pico8.client.showServerInfo`:

- Always available, including when the server failed to start.
- Writes a block to the **PICO-8 Language Server** output channel and reveals
  it.
- Includes extension version, server version or “not running”, version status,
  source, server path, platform/architecture, cache root, workspace-folder
  count, Lua attachment state, and last error.
- Does not require an active cart or `.lua` editor.

`pico8.client.copyServerDiagnostics`:

- Copies the same information as Markdown to the clipboard.
- Shows a short information message confirming the copy.
- Works when the server is running, stopped, or failed.
- Never includes source text, cart contents, or telemetry identifiers.

Add both commands to `package.json` under the existing `pico8.client.*`
namespace and to the activation events. Add them to the README command list.

### 5. Error handling

- If startup failed before a server version was available, Server Info shows
  `Server: not running`, the resolution error, and the attempted source/path.
- If `initialize` succeeds without `serverInfo.version`, classify as `unknown`
  and warn once; do not fail startup.
- If the version string is malformed, classify as `development` and show the
  raw value in Server Info.
- If the output channel cannot be opened or the clipboard write fails, show an
  actionable error with **Show Log**; do not crash the extension.
- Match the existing command behavior for a missing client: commands that need
  a running server still explain that the server is not running, but Show/Copy
  Server Info must remain useful.

### 6. Testing

Unit tests:

- normalization of `v1.7.0`, `1.7.0`, whitespace, `V1.7.0`, empty strings;
- release classification for `1.7.0`, `1.7.0-rc.1`, `1.7.0+build`, `dev`,
  and Go pseudo-versions;
- `match`, `mismatch`, `development`, and `unknown` statuses;
- Server Info formatting for custom, cache, download, running, and failed
  states;
- source tracking through `resolveServer` for custom path, cached binary, and
  downloaded binary;
- diagnostics contain no source text or cart contents.

Integration tests:

- Build the test server with
  `-ldflags "-X github.com/asimmons91/pico-lsp/internal/version.Version=<extension package version>"`
  so `initializeResult.serverInfo.version` reports a release version. Update
  the CI and release integration build commands accordingly. If a stamped
  server is unavailable, assert `development` instead and rely on the unit
  tests for the release-status branches.
- Start the stamped test server through `pico8.serverPath`; assert
  `getServerInfo()` reports `source: serverPath`, a release server version, and
  `match` status.
- Run `pico8.client.showServerInfo` and assert it does not throw.
- Run `pico8.client.copyServerDiagnostics` and read the clipboard; assert it
  contains the extension version, server version, source, and platform.
- Restart the server and confirm the info remains accurate.
- A full end-to-end mismatch test may use a small LSP stub that reports a
  different version; the unit tests are the required coverage for the warning
  branches.

### 7. Documentation and release

- `editors/vscode/README.md`: add the two commands and a short version-check
  note.
- `docs/troubleshooting.md`: add “Version mismatch” and “Copy server
  diagnostics” sections with the warning actions.
- `docs/configuration.md`: state that `pico8.serverPath` is not version-checked
  before startup but produces a non-blocking warning afterward.
- `docs/release-checklist.md`: verify the copied diagnostics on a release
  build.
- `CHANGELOG.md` under `Unreleased`, then the v1.9.0 release entry.
- Update `docs/ROADMAP.md` and the post-v1 backlog when the milestone status
  changes.

## Interfaces and state

- `ResolvedServer` is extension-internal; no public extension API is promised.
- `ServerDetails` is assembled by `PicoClient` and may be exposed through the
  existing test-only extension exports.
- The server continues to advertise `ServerInfo` exactly as it does today.
- No new server request, response, or command is introduced.
- The output channel and clipboard are the only new user-visible surfaces.

## Testing and CI

- `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt -l .`, and
  `golangci-lint run ./...` remain green; this milestone does not change Go
  code.
- Extension lint, typecheck, unit tests, integration tests, and VSIX packaging
  remain green.
- The integration suite continues to run without network access by using
  `pico8.serverPath`.
- The release checklist verifies the diagnostics command against a real
  release server.

## Exit criteria

- [ ] The extension reads the server version from `initializeResult` after
      startup.
- [ ] `v1.7.0` and `1.7.0` normalize to the same release version; pre-release,
      `dev`, and pseudo-versions are treated as development.
- [ ] A release-version mismatch warns once with the correct actions and never
      blocks startup.
- [ ] The extension tracks `serverPath`, `cache`, and `download` sources.
- [ ] **Show Server Info** works when the server is running, stopped, or
      failed.
- [ ] **Copy Server Diagnostics** copies versions, source, path, platform,
      cache root, and last error, with no source text.
- [ ] Unit tests cover matching, development, unknown, and mismatch statuses;
      integration tests cover the matching server and both commands.
- [ ] README, troubleshooting, configuration, changelog, roadmap, and backlog
      are updated.
- [ ] No LSP protocol, server behavior, telemetry, or download-security change.
- [ ] `v1.9.0` is tagged after the release PR merges (possibly with M21).

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Development builds produce constant mismatch warnings | Treat `dev`, pre-release, and pseudo-versions as development and suppress the warning. |
| Users expect a mismatch to block startup | Document that the check is advisory and keeps the server usable. |
| Version normalization hides a real mismatch | Unit-test the exact normalization table, including the server's `v` prefix. |
| A server omits `ServerInfo` | Classify as `unknown`, warn once, and keep startup successful. |
| Diagnostics leak source or cart contents | Build the block from metadata only; unit-test that no document text is included. |
| Cache/download mismatch is confused with a custom path | Track the source and offer **Clear Server Cache** only for downloaded or cached servers. |
| Commands fail when startup failed | Keep Server Info independent of a running client and include the last error. |
| Changing `resolveServer` breaks existing tests | Return a small `{ path, source }` object and update the focused resolution tests. |
