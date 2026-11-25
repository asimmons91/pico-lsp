# Getting started

This guide takes you from nothing to a working PICO-8 language server in about
five minutes. The server is editor-agnostic; pick the section for your editor.

## What you need

- VS Code 1.95+ for the extension; no Go toolchain is required.
- Neovim 0.11+ or Helix for the editor-agnostic server. Install the `pico-lsp`
  binary and configure the editor.
- PICO-8 0.2.7 if you want to run carts. It is not required to start the
  language server, but `pico-lsp` does not run carts itself.
- Network access on first use for the VS Code extension, unless you point it at
  a local server with `pico8.serverPath`.

## Try the demo cart

The repository includes [examples/hello.p8](../examples/hello.p8). It draws a
circle and moves it with the arrow keys.

```lua
function _init()
 x=64
 y=64
end

function _update()
 if (btn(0)) x-=1
 if (btn(1)) x+=1
 if (btn(2)) y-=1
 if (btn(3)) y+=1
end

function _draw()
 cls(1)
 circfill(x,y,4,10)
 print("hello pico-lsp",2,2,7)
end
```

Save this as `hello.p8` if you do not want to clone the repository. The full
example includes the standard header and a `__gfx__` section, so it is the safest
starting point.

## VS Code

The extension is distributed as a GitHub release VSIX. It is not currently
listed in the VS Code Marketplace or Open VSX.

1. Download `pico-lsp-vscode-<version>.vsix` and its `.sha256` file from the
   [latest GitHub release](https://github.com/asimmons91/pico-lsp/releases).
2. Verify the checksum:

   ```sh
   sha256sum -c pico-lsp-vscode-<version>.vsix.sha256
   ```

   On macOS without `sha256sum`, use
   `shasum -a 256 -c pico-lsp-vscode-<version>.vsix.sha256`.
3. In VS Code, open **Extensions**, select **...**, and choose
   **Install from VSIX...**.
4. Open a folder containing `hello.p8`, not just the single file. The extension
   downloads the matching server on first use.

The extension version and server version match. Extension `1.9.0` downloads
`pico-lsp` v1.9.0.

If the extension cannot download the server, set `pico8.serverPath` and
`pico8.autoDownload: false`. See the
[install guide](install.md#offline-or-air-gapped-use).

## Neovim

Install a `pico-lsp` release binary first, then verify it:

```sh
pico-lsp --version
```

The recommended client is [`pico8.nvim`](https://github.com/asimmons91/pico8.nvim).
With lazy.nvim or LazyVim:

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

`lazy = false` makes the plugin's `ftdetect`, `ftplugin`, and `syntax` files
available before the first `.p8` buffer opens. `opts = {}` enables the client.

If you prefer not to use the plugin, the minimal `vim.lsp.config` setup is:

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

Open the cart with `nvim hello.p8`. The optional `editors/nvim` helper in this
repository attaches `pico-lsp` to cart-owned `.lua` files; the `pico8.nvim`
plugin includes the maintained version. See the
[Neovim setup guide](../editors/nvim/README.md) for the helper, keymaps, PNG
carts, and P8SCII input.

## Helix

Install a `pico-lsp` release binary and copy the definitions from
[`editors/helix/languages.toml`](../editors/helix/languages.toml) into
`~/.config/helix/languages.toml`:

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

Helix passes `config` as initialization options, which the server accepts as
`initializationOptions.pico8`. Open the cart with `hx hello.p8` and check
`hx --health pico8`.

See the [Helix setup guide](../editors/helix/README.md) for grammar
highlighting and troubleshooting.

## What to try

Open the demo cart and try these in the editor:

- Hover over `circfill` or `x` to see the type and documentation.
- Type `circ` and accept completion for `circfill`.
- Add `foo = 1` and reference `fo` to see completion for the new global.
- Introduce `print("hello"` without the closing parenthesis and watch the syntax
  diagnostic move as you fix it.
- Check the cart budget: token, character, and compressed-size information is
  available in the VS Code status bar, the Neovim code lens and statusline, and
  the `pico8/cartInfo` request in any client.

## What this is not

- It does not run, compile, or emulate carts.
- It does not replace the PICO-8 editor. It edits the same `.p8` and `.lua` files
  that PICO-8 loads.
- It does not provide the Lua standard library. Only the PICO-8 0.2.7 API is
  available.
- It does not write to `.p8.png` carts. PNG carts are read-only.

## Next steps

- [PICO-8 workflow](pico8-workflow.md): edit carts outside PICO-8 without losing
  changes.
- [Types](types.md): annotate carts with LuaCATS and choose a checking mode.
- [CLI](cli.md): flags, logging, and PNG decoding.
- [Editors](editors.md): tested versions and per-editor feature differences.
- [Configuration](configuration.md): every `pico8.*` setting.
- [Troubleshooting](troubleshooting.md): the server does not attach or
  features are missing.
