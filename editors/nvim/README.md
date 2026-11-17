# Neovim 0.11+ setup

This config attaches `pico-lsp` to `.p8` carts. The server speaks LSP, syncs
documents incrementally, reports syntax and include diagnostics, and provides
document symbols and folding ranges. Token-budget UI arrives in M5.

## Prerequisites

Build or install `pico-lsp` so it is on `PATH`:

```sh
go install github.com/asimmons91/pico-lsp/cmd/pico-lsp@latest
# or, from a checkout:
go build -o ./pico-lsp ./cmd/pico-lsp
```

## Neovim config

Add this to `init.lua` (or a file under `lua/`):

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
server pulls the `pico8` section after the `initialized` notification and
prefers pulled values per field; fields the client does not return fall back to
`init_options`.

To run a locally built binary without installing it, use an absolute path:

```lua
vim.lsp.config("pico_lsp", {
  cmd = { "/absolute/path/to/pico-lsp" },
  filetypes = { "pico8" },
})
```

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

3. Inspect the negotiated position encoding (default `utf-16`, or `utf-8`
   when the client offers it):

   ```vim
   :lua =vim.lsp.get_clients({ bufnr = 0 })[1].offset_encoding
   ```

4. Try the M1 language features:

   ```vim
   :lua vim.lsp.buf.document_symbol()
   ```

   For folding, enable LSP folding in the buffer:

   ```vim
   :setlocal foldmethod=expr foldexpr=v:lua.vim.lsp.foldexpr()
   ```

   Open a cart with a deliberate syntax error and confirm the diagnostic lands
   on the right file and line. Included `.lua` files produce diagnostics even
   when they are not open in a buffer.

5. Edit, save, and close the buffer. The language server must stay attached and
   Neovim must not report protocol errors.

6. Stop Neovim while the server is running (for example with `:qa`). The server
   should exit cleanly rather than hang.

## Incremental sync check

The server advertises incremental sync. To confirm a change was applied, use a
temporary log file and enable debug logging:

```lua
vim.lsp.config("pico_lsp", {
  cmd = { "pico-lsp", "--log-level", "debug", "--log-file", "/tmp/pico-lsp.log" },
  filetypes = { "pico8" },
})
```

Then edit the cart and look for `document changed` lines in
`/tmp/pico-lsp.log`. Routine server logs never go to stdout, so they cannot
corrupt the protocol stream.

## Included `.lua` files

Included `.lua` files are not attached automatically in M0. The optional
Neovim helper plugin that detects cart-owned `.lua` buffers is part of M6. For
now, attach manually in a known PICO-8 project if you need to:

```vim
:lua vim.lsp.start({ name = "pico_lsp", cmd = { "pico-lsp" }, root_dir = vim.fn.getcwd() })
```

Avoid running another Lua language server on cart-owned `.lua` buffers once the
helper is available.

## Troubleshooting

- `:checkhealth vim.lsp` shows Neovim's LSP health and attached clients.
- `:lua =vim.lsp.log.get_filename()` prints Neovim's LSP client log path.
- `pico-lsp --version` verifies which binary Neovim would run.
- `:lua =vim.lsp.get_clients()` lists active clients and their capabilities.
- If the server does not attach, check that `pico-lsp` is executable and on
  `PATH`, then run `:checkhealth vim.lsp`.
