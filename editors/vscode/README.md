# PICO-8 for VS Code

This extension adds PICO-8 language support to local desktop VS Code using
[`pico-lsp`](https://github.com/asimmons91/pico-lsp). It manages a matching
server download, contributes PICO-8 syntax highlighting, and shows cart token,
character, and compressed-size budgets in the status bar.

## Install

Marketplace publishing is not available yet. Install the
`pico-lsp-vscode-<version>.vsix` asset from the matching
[GitHub release](https://github.com/asimmons91/pico-lsp/releases):

1. Download the VSIX and `.vsix.sha256` file.
2. In VS Code, open **Extensions**, select **...**, then
   **Install from VSIX...**.
3. Open a folder containing a `.p8` cart.

The extension version and server version are identical. For example, extension
`1.7.0` downloads `pico-lsp` v1.7.0 on first use.

## Server installation

On first use in a workspace with a `.p8` file, the extension downloads the
matching release archive for the current platform, verifies SHA-256 from
`checksums.txt`, and caches it in the extension's global storage. No download
occurs in untrusted workspaces; the extension disables itself there.

To use an existing binary instead, set an absolute `pico8.serverPath`. This
disables automatic downloads:

```json
{
  "pico8.serverPath": "/usr/local/bin/pico-lsp",
  "pico8.autoDownload": false
}
```

If the download fails behind a proxy, configure VS Code's `http.proxy` and
`http.proxyStrictSSL` settings, or use `pico8.serverPath`.

The extension reads the running server's version after startup. A release
server that does not match the extension version still starts and keeps all
features; the extension shows one advisory warning with **Show Server Info**,
**Open Settings** (for a custom `pico8.serverPath`), and **Show Log**. Cached or
downloaded mismatches offer **Clear Server Cache** instead. Development builds
report “development build” and do not produce a mismatch warning. Use
**PICO-8: Copy Server Diagnostics** to copy the extension and server versions,
source, path, platform, cache root, and last startup error for an issue report.

## Language support

- `.p8` files use the contributed `pico8` language and TextMate grammar.
- `.p8` files show the PICO-8 logo when the active file icon theme supports
  language default icons. Icon themes that set `showLanguageModeIcons: false`
  keep their own generic icon.
- All `.lua` files in workspace folders that contain at least one `.p8` cart
  attach to the server. Other `.lua` files do not.
- PICO-8 workspaces should disable LuaLS for their `.lua` files to avoid
  duplicate diagnostics and competing completion. This extension does not
  manage other language servers.

The server provides diagnostics, completion, hover, signature help, document
symbols, folding, navigation, rename, semantic tokens, formatting, inlay hints,
code lenses, code actions, refactors, and P8SCII glyph input. Type `p8_left` in
code or a string to insert `⬅️`, or run **PICO-8: Insert P8SCII Glyph** to open
a searchable picker. Custom font slots are configured with `pico8.glyphs`.

## Status bar

The status bar shows token, character, and compressed budgets for the active
cart or attached Lua file. Click it to run **PICO-8: Show Cart Info**. Warning
and error colors appear at 90% of a limit and above a limit.

## Settings

Every documented `pico8.*` server setting appears in the VS Code Settings UI
and in `settings.json` with its type, allowed values, default, and description.
Search for `pico8` in Settings. Server settings are window-scoped; VS Code
validates enum values and rejects unknown keys in `limits`, `format`,
`diagnostics`, `lint`, and `refactors`. The same settings work in other editors
because the server reads them through the standard LSP configuration section.

The client-owned settings are:

| Setting | Default | Description |
|---|---|---|
| `pico8.serverPath` | `""` | Absolute path to a `pico-lsp` executable. Disables downloads. Machine-scoped. |
| `pico8.autoDownload` | `true` | Download the matching verified server when not cached. |
| `pico8.attachLua` | `true` | Attach to `.lua` files in PICO-8 workspace folders. |
| `pico8.trace.server` | `off` | LSP trace level: `off`, `messages`, or `verbose`. |

Server settings such as `pico8.typeChecking`, `pico8.diagnostics`,
`pico8.format`, and `pico8.glyphs` are documented in the
[configuration reference](https://github.com/asimmons91/pico-lsp/blob/main/docs/configuration.md).
The extension manifest and the server defaults are kept in sync by a repository
test.

## Commands

- **PICO-8: Show Cart Info** (`pico8.client.showCartInfo`)
- **PICO-8: Insert P8SCII Glyph** (`pico8.client.insertGlyph`)
- **PICO-8: Restart Language Server** (`pico8.client.restartServer`)
- **PICO-8: Show Language Server Output** (`pico8.client.showOutput`)
- **PICO-8: Show Server Info** (`pico8.client.showServerInfo`)
- **PICO-8: Copy Server Diagnostics** (`pico8.client.copyServerDiagnostics`)
- **PICO-8: Clear Downloaded Server Cache** (`pico8.client.clearServerCache`)

**PICO-8: Show Server Info** writes the current extension version, server
version and source, path, platform, cache root, workspace-folder count, Lua
attachment state, and last startup error to the **PICO-8 Language Server**
output channel. Both info commands work when the server is stopped or failed.
**PICO-8: Copy Server Diagnostics** copies the same block as Markdown to the
clipboard.

## Logs and troubleshooting

Use **PICO-8: Show Language Server Output** for server logs and download errors.
The trace channel records LSP traffic when `pico8.trace.server` is enabled.
Release binaries are unsigned; checksum verification covers the download, but
the server itself is not code-signed.

## Limitations

- Local desktop VS Code only. Remote SSH, WSL, Dev Containers, Codespaces, and
  web extension hosts are not supported in this release.
- `.p8.png` virtual documents are not supported; export a text `.p8` cart.
- Marketplace and Open VSX publishing are follow-ups; install the VSIX from the
  GitHub release.
- The extension downloads the server version matching its own version. It does
  not auto-update independently.

## Development

The repository pins Node.js 26.13.1 in `mise.toml`:

```sh
mise install
cd editors/vscode
npm ci
npm run build
npm run typecheck
npm run lint
npm run test:unit
```

Integration tests require a built `pico-lsp` and a Linux display server. CI
stamps the binary with the extension manifest version so the suite can assert a
release match; an unstamped local binary is accepted as a development build:

```sh
version="$(node -p "require('./package.json').version")"
(cd ../.. && go build -trimpath \
  -ldflags "-X github.com/asimmons91/pico-lsp/internal/version.Version=${version}" \
  -o /tmp/pico-lsp ./cmd/pico-lsp)
PICO_LSP_TEST_SERVER=/tmp/pico-lsp npm run test:integration
# On headless Linux: xvfb-run -a env PICO_LSP_TEST_SERVER=/tmp/pico-lsp npm run test:integration
```

See the [repository](https://github.com/asimmons91/pico-lsp) for source and
project documentation.

PICO-8 is a trademark of Lexaloffle Games. This extension is an independent
project and is not affiliated with Lexaloffle.
