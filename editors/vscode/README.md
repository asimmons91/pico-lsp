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
`1.5.1` downloads `pico-lsp` v1.5.1 on first use.

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

| Setting | Default | Description |
|---|---|---|
| `pico8.serverPath` | `""` | Absolute path to a `pico-lsp` executable. Disables downloads. |
| `pico8.autoDownload` | `true` | Download the matching verified server when not cached. |
| `pico8.attachLua` | `true` | Attach to `.lua` files in PICO-8 workspace folders. |
| `pico8.trace.server` | `off` | LSP trace level: `off`, `messages`, or `verbose`. |
| `pico8.glyphs` | `[]` | Custom P8SCII glyph snippets for completion and the picker. |

Server settings such as `pico8.typeChecking`, `pico8.diagnostics`,
`pico8.format`, and `pico8.glyphs` are documented in the
[configuration reference](https://github.com/asimmons91/pico-lsp/blob/main/docs/configuration.md).

## Commands

- **PICO-8: Show Cart Info** (`pico8.client.showCartInfo`)
- **PICO-8: Insert P8SCII Glyph** (`pico8.client.insertGlyph`)
- **PICO-8: Restart Language Server** (`pico8.client.restartServer`)
- **PICO-8: Show Language Server Output** (`pico8.client.showOutput`)
- **PICO-8: Clear Downloaded Server Cache** (`pico8.client.clearServerCache`)

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

Integration tests require a built `pico-lsp` and a Linux display server:

```sh
PICO_LSP_TEST_SERVER=/path/to/pico-lsp npm run test:integration
# On headless Linux: xvfb-run -a env PICO_LSP_TEST_SERVER=... npm run test:integration
```

See the [repository](https://github.com/asimmons91/pico-lsp) for source and
project documentation.

PICO-8 is a trademark of Lexaloffle Games. This extension is an independent
project and is not affiliated with Lexaloffle.
