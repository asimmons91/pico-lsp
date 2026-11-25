# PICO-8 LSP for VS Code

PICO-8 language support for local desktop VS Code, powered by the open-source
[`pico-lsp`](https://github.com/asimmons91/pico-lsp) server. Get diagnostics,
completion, navigation, formatting, refactors, P8SCII glyph input, and live cart
budgets without installing Go or managing a server binary.

![Illustrative pixel-art banner showing a game cartridge, a pixel heart, code brackets, a cursor, and a small rocket on a dark navy background](https://raw.githubusercontent.com/asimmons91/pico-lsp/main/editors/vscode/images/pico-lsp-banner.png)

## Install

The extension is distributed as a GitHub release VSIX. Install it:

1. Download `pico-lsp-vscode-1.9.0.vsix` and
   `pico-lsp-vscode-1.9.0.vsix.sha256` from the matching
   [GitHub release](https://github.com/asimmons91/pico-lsp/releases).
2. Verify the checksum:

   ```sh
   sha256sum -c pico-lsp-vscode-1.9.0.vsix.sha256
   ```

3. In VS Code, open the Extensions view, select **...**, and choose
   **Install from VSIX...**.

The extension is not currently listed in the VS Code Marketplace or Open VSX.
The VSIX remains available for pinned, registry-lag, and offline use. Registry
publication is parked in the
[post-v1 backlog](https://github.com/asimmons91/pico-lsp/blob/main/docs/post-v1-backlog.md).

Offline use also requires a local `pico-lsp` binary and `pico8.serverPath`; the
VSIX does not bundle the server. See the
[install guide](https://github.com/asimmons91/pico-lsp/blob/main/docs/install.md#vs-code)
for platform-specific checksum commands and offline setup.

## Requirements

- VS Code 1.95.0 or newer.
- Local desktop VS Code and Open VSX-compatible desktop editors only. Remote
  SSH, WSL, Dev Containers, Codespaces, and web extension hosts are not
  supported.
- No Go toolchain is required. The extension downloads the matching checksummed
  `pico-lsp` release on first use.
- Network access to GitHub on first use, unless you configure a local server
  with `pico8.serverPath`.

## Quick start

1. Open a folder containing a `.p8` cart.
2. The extension downloads the server version that matches the extension,
   verifies its SHA-256 checksum, and starts it.
3. Open the cart. Diagnostics, completion, hover, symbols, and the cart-budget
   status bar appear as the server starts.
4. `.lua` files in the same workspace folder attach automatically when the
   folder contains a `.p8` cart.

The extension version and server version are identical by default. Extension
`1.9.0`, for example, downloads `pico-lsp` v1.9.0.

New to `pico-lsp`? See the
[getting-started guide](https://github.com/asimmons91/pico-lsp/blob/main/docs/getting-started.md)
and open the
[demo cart](https://github.com/asimmons91/pico-lsp/blob/main/examples/hello.p8).

## Features

- PICO-8-aware diagnostics for syntax, includes, undefined globals, types, and
  cart limits, with configurable severities.
- Completion, hover, signature help, semantic tokens, and inlay hints for your
  symbols and the PICO-8 0.2.7 API.
- Document symbols, folding, go-to-definition, references, highlights, rename,
  and workspace symbols across tabs and non-recursive `#include` files.
- LuaCATS annotations with classes, inheritance, generics, overloads,
  tagged-union narrowing, and gradual `off`/`basic`/`strict` type checking.
- Formatting that preserves PICO-8 shorthand, comments, annotations, includes,
  and non-code cart sections.
- Token, character, and PICO-8-compatible compressed-size budgets in the status
  bar, with warnings as a cart approaches a limit.
- Conservative code actions and refactors, plus token-saving rewrites for `?`,
  compound assignments, and peek aliases.
- P8SCII glyph completion (`p8_left`) and a searchable glyph picker, including
  custom font-slot entries through `pico8.glyphs`.
- PICO-8 syntax highlighting and a `.p8` file icon that follows the active file
  icon theme when it supports language default icons.

## Privacy and security

- No telemetry, analytics, crash reporting, or accounts.
- Cart code is never executed. The server only parses and analyzes source.
- The only network request made by the extension is the checksummed download of
  the matching `pico-lsp` release from this project's GitHub releases. VS Code
  itself may check its configured extension source for updates; this extension
  is not currently listed in either registry.
- Untrusted workspaces are disabled. The extension does not start a server or
  download a binary there.
- Logs may contain source excerpts only when LSP trace logging is explicitly
  enabled with `pico8.trace.server`. Review logs before sharing them.

## AI-assisted development

This project is developed with AI coding agents under human direction and
review, as described in the
[root README](https://github.com/asimmons91/pico-lsp#ai-assisted-development).
Inspect the implementation for your use case before relying on it in critical
workflows, and report bugs with a minimal cart.

## Settings

Search for `pico8` in the VS Code Settings UI. Every documented server setting
appears with its type, allowed values, default, and description. Server settings
are window-scoped. The client-owned settings are:

| Setting | Default | Description |
|---|---|---|
| `pico8.serverPath` | `""` | Absolute path to a `pico-lsp` executable. Disables downloads. Machine-scoped. |
| `pico8.autoDownload` | `true` | Download the matching verified server when it is not cached. |
| `pico8.attachLua` | `true` | Attach to `.lua` files in PICO-8 workspace folders. |
| `pico8.trace.server` | `off` | LSP trace level: `off`, `messages`, or `verbose`. |

Server settings such as `pico8.typeChecking`, `pico8.diagnostics`,
`pico8.format`, `pico8.limits`, `pico8.refactors`, and `pico8.glyphs` are
documented in the
[configuration reference](https://github.com/asimmons91/pico-lsp/blob/main/docs/configuration.md).
The manifest and server defaults are kept in sync by a repository test.

### Offline or custom server

Set an absolute `pico8.serverPath` to use an existing binary and skip downloads:

```json
{
  "pico8.serverPath": "/usr/local/bin/pico-lsp",
  "pico8.autoDownload": false
}
```

## Commands

Open the Command Palette and search for `PICO-8`:

- **PICO-8: Show Cart Info**
- **PICO-8: Insert P8SCII Glyph**
- **PICO-8: Restart Language Server**
- **PICO-8: Show Language Server Output**
- **PICO-8: Show Server Info**
- **PICO-8: Copy Server Diagnostics**
- **PICO-8: Clear Downloaded Server Cache**

**PICO-8: Copy Server Diagnostics** copies the extension and server versions,
source, path, platform, cache root, workspace-folder count, and the last startup
error when present, without cart source text. Paste it into a bug report.

## Logs and version mismatch

Use **PICO-8: Show Language Server Output** for server logs and download errors,
and **PICO-8: Show Server Info** for the running versions and source. If
`pico8.trace.server` is enabled, the trace channel records LSP traffic.

A release server that does not match the extension version still starts and
keeps all features; the extension shows one advisory warning with actions to
inspect the server, open settings, clear the cache, or view the log. Development
builds report "development build" and do not warn.

If a download fails behind a proxy, configure VS Code's `http.proxy` and
`http.proxyStrictSSL` settings, or set `pico8.serverPath`.

## Limitations

- Local desktop VS Code and Open VSX-compatible desktop editors only. Remote
  SSH, WSL, Dev Containers, Codespaces, and web extension hosts are not
  supported.
- `.p8.png` virtual documents are not supported in this extension. Decode a PNG
  cart with `pico-lsp decode --output cart.p8 cart.p8.png` and open the text
  cart.
- The extension is distributed as a GitHub release VSIX. Marketplace and Open
  VSX publication is not currently available; registry work is parked in the
  post-v1 backlog. Use the matching VSIX when you need a specific version.
- Offline use requires `pico8.serverPath` (and usually
  `pico8.autoDownload: false`).
- The extension downloads the server version that matches its own version. It
  does not update the server independently.
- PICO-8 workspaces should disable other Lua language servers, such as LuaLS,
  for their `.lua` files to avoid duplicate diagnostics and competing
  completion. This extension does not manage other language servers.

## Support

- [Documentation and source](https://github.com/asimmons91/pico-lsp)
- [Install and upgrade guide](https://github.com/asimmons91/pico-lsp/blob/main/docs/install.md)
- [Configuration reference](https://github.com/asimmons91/pico-lsp/blob/main/docs/configuration.md)
- [Troubleshooting](https://github.com/asimmons91/pico-lsp/blob/main/docs/troubleshooting.md)
- [Bug reports](https://github.com/asimmons91/pico-lsp/issues)

PICO-8 is a trademark of Lexaloffle Games. This extension is an independent
project and is not affiliated with or endorsed by Lexaloffle Games.
