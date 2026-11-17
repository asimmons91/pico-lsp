# Neovim 0.11+ setup

This directory contains:

- the recommended `vim.lsp.config` setup for `.p8` carts;
- an optional helper for attaching `pico-lsp` to cart-owned `.lua` buffers;
- `ftdetect` and `syntax` files for PICO-8 carts;
- a headless smoke test for the helper.

The syntax and helper are optional. The server works with the base LSP config
alone.

## Install

Install a release binary with the instructions in
[../../docs/install.md](../../docs/install.md), or build from a checkout:

```sh
CGO_ENABLED=0 go build -trimpath -o pico-lsp ./cmd/pico-lsp
```

Make sure `pico-lsp` is on `PATH`, then verify it:

```sh
pico-lsp --version
```

## Base LSP config

Add this to `init.lua` or a file under `lua/`:

```lua
vim.filetype.add({ extension = { p8 = "pico8" } })

vim.lsp.config("pico_lsp", {
  cmd = { "pico-lsp" },
  filetypes = { "pico8" },
  root_markers = { ".git" },
  settings = {
    pico8 = {
      apiVersion = "0.2.7",
      typeChecking = "basic",
      inlayHints = false,
    },
  },
})

vim.lsp.enable("pico_lsp")
```

The same settings can be passed as `init_options = { pico8 = { ... } }`
instead of `settings`. If the client supports `workspace/configuration`, the
server pulls the `pico8` section after `initialized` and prefers pulled values
per field; fields the client does not return fall back to `init_options`.

For a locally built binary, use an absolute path:

```lua
vim.lsp.config("pico_lsp", {
  cmd = { "/absolute/path/to/pico-lsp" },
  filetypes = { "pico8" },
})
```

## Optional helper for included `.lua` files

The server analyzes included `.lua` files whenever the owning `.p8` cart is
attached, even when the include is closed. To edit an included `.lua` buffer
with the same client, add this repository's `editors/nvim` directory to your
runtime path and enable the helper:

```lua
vim.opt.runtimepath:append("/absolute/path/to/pico-lsp/editors/nvim")
require("pico_lsp").setup()
```

The helper:

- walks up from the `.lua` buffer to find a nearby `.p8` cart that includes it;
- starts or reuses the `pico_lsp` client with the cart directory as `root_dir`;
- ignores unrelated `.lua` buffers;
- does not create duplicate clients for the same cart;
- re-scans loaded `.lua` buffers after a `.p8` file is saved.

It reuses `vim.lsp.config["pico_lsp"]` when defined. You can pass fallbacks or
overrides directly:

```lua
require("pico_lsp").setup({
  cmd = { "/absolute/path/to/pico-lsp" },
  settings = {
    pico8 = { typeChecking = "strict" },
  },
})
```

Disable it with `require("pico_lsp").setup({ disable = true })` or
`vim.g.pico_lsp_helper_disabled = 1`.

If you prefer not to use the helper, attach manually from the cart root:

```vim
:lua vim.lsp.start({ name = "pico_lsp", cmd = { "pico-lsp" }, root_dir = vim.fn.getcwd() })
```

Avoid running `lua_ls` and `pico-lsp` on the same cart-owned `.lua` buffer.

## Read-only `.p8.png` carts

The helper defines two commands for PNG carts:

```vim
:Pico8OpenPng path/to/cart.p8.png
:Pico8ReloadPng
```

`Pico8OpenPng` requests `pico8/decodeCart`, opens a `pico8-cart:` virtual buffer,
marks it `nomodifiable`, and attaches the existing `pico_lsp` client. Use
`Pico8ReloadPng` after replacing the PNG file. Navigation, diagnostics,
completion, hover, and `pico8/cartInfo` work on the virtual buffer; formatting
and edits are refused because the PNG is read-only.

If you do not use the helper, decode to a text file for a static fallback:

```sh
pico-lsp decode --output /tmp/cart.p8 path/to/cart.p8.png
```

## Syntax highlighting

Add the runtime path as above. The included `ftdetect` sets `filetype=pico8`
for `.p8` files and `syntax/pico8.vim` highlights sections, tab markers,
comments, strings, numbers, dialect keywords, and operators.

The syntax file is independent of the LSP and can be copied to
`~/.config/nvim/syntax/` if you do not want the helper. LSP semantic tokens
cover Lua code; the syntax file also highlights non-code `.p8` sections.

## Keymaps and cart budget

The server does not install keymaps. Recommended mappings:

```lua
vim.keymap.set("n", "gd", vim.lsp.buf.definition)
vim.keymap.set("n", "K", vim.lsp.buf.hover)
vim.keymap.set("n", "<leader>rn", vim.lsp.buf.rename)
vim.keymap.set("n", "<leader>ca", vim.lsp.buf.code_action)
vim.keymap.set("n", "<leader>ci", function()
  vim.lsp.buf_request(0, "pico8/cartInfo", {
    textDocument = { uri = vim.uri_from_bufnr(0) },
  }, function(err, result)
    if not err and result then
      print(("%d / %d tokens · %d / %d chars"):format(
        result.tokens, result.tokenLimit, result.chars, result.charLimit))
    end
  end)
end)
```

The cart root also gets a budget code lens. Run it to show the
`pico8.showCartInfo` summary.

## Smoke test

1. Open the test cart:

   ```sh
   nvim testdata/smoke.p8
   ```

2. Confirm the filetype and attached client:

   ```vim
   :set filetype?
   :checkhealth vim.lsp
   :lua =vim.lsp.get_clients({ bufnr = 0 })
   ```

3. Try document symbols, hover, completion, and the cart info request:

   ```vim
   :lua vim.lsp.buf.document_symbol()
   :lua vim.lsp.buf.hover()
   ```

4. Open an included `.lua` file from a cart and confirm the helper attaches:

   ```vim
   :lua =vim.lsp.get_clients({ bufnr = 0 })
   ```

5. Run the headless helper test:

   ```sh
   CGO_ENABLED=0 go build -trimpath -o pico-lsp ./cmd/pico-lsp
   PICO_LSP_BIN="$PWD/pico-lsp" editors/nvim/tests/run.sh
   ```

6. Stop Neovim while the server is running. The server should exit cleanly.

## Troubleshooting

- `:checkhealth vim.lsp` shows Neovim's LSP health and attached clients.
- `:lua =vim.lsp.log.get_filename()` prints Neovim's LSP client log path.
- `pico-lsp --version` verifies which binary Neovim would run.
- Use `--log-level debug --log-file /tmp/pico-lsp.log` in `cmd` to collect
  server logs. Routine logs never go to stdout, so they cannot corrupt the
  protocol stream.
- If the helper does not attach an include, confirm the include is exactly a
  line beginning with `#include`, the path resolves relative to the cart, and
  the buffer is named `.lua`.
