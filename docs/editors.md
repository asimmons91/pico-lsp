# Editors

`pico-lsp` speaks standard LSP over stdio. This page is the index for the
editors that have documented setup and tests. The server remains usable from any
LSP client, but only the listed paths are supported.

## Tested matrix

| Editor | Tested version | Status | Install path |
|---|---|---|---|
| VS Code | 1.95+ engine; desktop checks pending | Supported, local desktop only | GitHub release VSIX |
| Neovim | 0.12.5 headless tests; 0.11+ supported | Supported | `pico8.nvim` (repository install) or manual `vim.lsp.config` |
| Helix | 25.07 (last recorded; re-verify on current stable) | Supported | `editors/helix/languages.toml` |
| Open VSX-compatible editors | Current stable | Best-effort via VSIX; not separately tested | GitHub release VSIX |
| Zed | — | Post-v1 backlog | — |

The table records what has actually been run. Re-run the manual matrix when an
editor's LSP integration changes. Do not claim support for a version or feature
that was not verified.

## VS Code

Install the GitHub release VSIX; the extension is not currently in the VS Code
Marketplace or Open VSX.

1. Download `pico-lsp-vscode-<version>.vsix` and its `.sha256` file from the
   [latest release](https://github.com/asimmons91/pico-lsp/releases).
2. Verify the checksum with `sha256sum` (Linux/macOS) or `Get-FileHash`
   (PowerShell).
3. In VS Code, open **Extensions**, select **...**, and choose
   **Install from VSIX...**.
4. Open a folder containing a `.p8` cart.

The extension downloads the matching server on first use, attaches to `.p8`
files, and attaches to `.lua` files in workspace folders that contain a `.p8`
cart. It adds the cart-budget status bar, P8SCII glyph picker, commands, and
settings schema.

Limitations:

- Local desktop windows only. Remote SSH, WSL, Dev Containers, Codespaces, and
  web extension hosts are unsupported.
- `.p8.png` virtual documents are not supported; decode them with the CLI.
- The extension does not manage other language servers. Disable LuaLS for
  cart-owned `.lua` files.

See [editors/vscode/README.md](../editors/vscode/README.md) for the full product
page and [install.md](install.md#vs-code) for offline and cache details.

## Neovim

Neovim 0.11+ is supported. Neovim 0.12 adds `gx` document-link handling; on 0.11,
use `gf` or the include navigation helper.

Install a `pico-lsp` release binary, then choose one client:

### `pico8.nvim` (recommended)

```lua
return {
  {
    "asimmons91/pico8.nvim",
    main = "pico_lsp",
    lazy = false,
    opts = {},
  },
}
```

The plugin manages filetype detection, LSP activation, cart-owned `.lua`
attachment, `pico8/cartInfo` status helpers, the P8SCII glyph picker, PNG cart
opening, `gq` range formatting, and the Tree-sitter parser installer.

### Manual `vim.lsp.config`

```lua
vim.filetype.add({ extension = { p8 = "pico8" } })

vim.lsp.config("pico_lsp", {
  cmd = { "pico-lsp" },
  filetypes = { "pico8" },
  root_markers = { ".git" },
  settings = {
    pico8 = {
      typeChecking = "basic",
      inlayHints = false,
    },
  },
})

vim.lsp.enable("pico_lsp")
```

Use the manual path if you do not want a plugin. The plugin is required for
automatic cart-owned `.lua` attachment and the PNG virtual-document commands.

Limitations:

- Read-only `.p8.png` virtual documents require `pico8.nvim` or the `editors/nvim`
  helper.
- PICO-8's `CTRL-J`, `CTRL-K`, and `CTRL-P` input modes are not implemented;
  they conflict with Neovim's insert-mode keys.
- `gq` refuses a range edit that would change the line count. Format the whole
  document in that case.

See [editors/nvim/README.md](../editors/nvim/README.md) and the
[`pico8.nvim` README](https://github.com/asimmons91/pico8.nvim).

## Helix

Helix uses `config` on the language server as initialization options. The server
accepts the `pico8` section there:

```toml
[language-server.pico-lsp]
command = "pico-lsp"
config = { pico8 = { typeChecking = "basic", inlayHints = false } }

[[language]]
name = "pico8"
scope = "source.pico8"
file-types = ["p8"]
comment-token = "--"
language-servers = ["pico-lsp"]
```

Copy the full definitions from
[`editors/helix/languages.toml`](../editors/helix/languages.toml). The optional
Tree-sitter grammar is documented in the
[Helix setup guide](../editors/helix/README.md#syntax-highlighting).

Limitations:

- Helix 25.07 has no folding UI or LSP folding consumer.
- There is no cart-owned `.lua` attachment helper; open the owning `.p8` cart for
  full analysis.
- `.p8.png` carts must be decoded with `pico-lsp decode` before opening.
- Helix does not have a portable virtual-document command, so PNG support is the
  static decode fallback.

## Open VSX-compatible editors

The same VSIX can be installed in VSCodium and other Open VSX-compatible desktop
editors. This path is best-effort and not part of the tested matrix. The
extension is not currently published to Open VSX; install the VSIX directly.

Remote, web, and container extension hosts are unsupported.

## Zed

A Zed extension is on the post-v1 backlog. The tree-sitter grammar is ready for
a thin extension, but there is no packaged install path in v1. See
[editors/zed/README.md](../editors/zed/README.md). Do not treat that note as a
supported setup guide.

## See also

- [Getting started](getting-started.md)
- [Neovim setup](../editors/nvim/README.md)
- [Helix setup](../editors/helix/README.md)
- [VS Code product page](../editors/vscode/README.md)
- [Troubleshooting](troubleshooting.md)
