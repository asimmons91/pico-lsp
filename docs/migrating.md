# Migrating from pico8-ls and LuaLS

[`pico8-ls`](https://github.com/japhib/pico8-ls) is another PICO-8 language
server. `pico-lsp` covers the editor features people expect from it, adds
LuaCATS types, and understands carts and non-recursive includes. It is not a
drop-in replacement for every `pico8-ls` or LuaLS behavior.

## What carries over

- **P8SCII completion aliases.** The primary trigger is `p8_<name>`.
  `p8-<name>` is also accepted for `pico8-ls` compatibility, for example
  `p8_left` or `p8-left`.
- **Standard LSP features.** Completion, hover, signature help, definitions,
  references, rename, document symbols, folding, diagnostics, semantic tokens,
  and code actions work through the editor's normal LSP UI.
- **Formatting.** `pico-lsp` formats carts with an AST-aware formatter that keeps
  shorthand, comments, annotations, includes, and non-code sections. It is not
  guaranteed to match `pico8-ls` byte for byte.

## What is different

- Types are LuaCATS comments only. There is no Luau-style inline syntax.
- Type checking is gradual and defaults to `basic`.
- The built-in API is PICO-8 0.2.7. The Lua standard library is not available.
- `.p8.png` carts are read-only. Use PICO-8 to export a text `.p8` when you need
  to edit one.
- Includes are non-recursive, matching PICO-8.

## Disable the old server first

Running two PICO-8 language servers on the same buffer produces duplicate
completion, hover, diagnostics, and code actions.

### Neovim

Remove or disable the old `pico8-ls` entry from your LSP configuration, then
enable `pico_lsp`:

```lua
-- Remove the old server's vim.lsp.config/enable calls and any
-- lspconfig setup for pico8-ls.
vim.lsp.enable("pico_lsp")
```

If the old server is registered through `nvim-lspconfig`, its server name is
`pico8_ls`. Remove it from the list of servers you enable for `pico8` buffers.
Check `:checkhealth vim.lsp` and
`:lua =vim.lsp.get_clients({ bufnr = 0 })` to confirm only one client remains.

### VS Code

Disable or uninstall `PollywogGames.pico8-ls` for the workspace, then install the
`pico-lsp` VSIX. Use **PICO-8: Copy Server Diagnostics** to confirm which server
is attached.

### Helix

Remove the old server from the `language-servers` array for the `pico8`
language, and keep only `pico-lsp`:

```toml
[[language]]
name = "pico8"
language-servers = ["pico-lsp"]
```

## Coexisting with LuaLS

LuaLS (`lua_ls`/`lua-language-server`) is useful for ordinary Lua projects, but
it does not understand PICO-8 shorthand, token limits, carts, or non-recursive
includes. Do not attach both servers to cart-owned `.lua` files.

- **VS Code:** keep `pico8.attachLua` enabled for PICO-8 workspace folders, and
  scope or disable LuaLS for those folders.
- **Neovim:** use the `pico8.nvim` helper (or `editors/nvim` helper) for
  cart-owned `.lua` buffers, and disable `lua_ls` for those buffers.
- **Helix:** open the owning `.p8` cart for full analysis. Helix has no
  cart-owned include helper in v1.

If duplicate diagnostics appear on a `.lua` file, check which clients are
attached before changing settings.

## Which server owns a `.lua` file?

A `.lua` file belongs to the nearest ancestor `.p8` cart that includes it with
`#include`. That cart is the authoritative program. The server uses the cart to
resolve globals, types, and includes, even when the `.lua` file is open on its
own.

Open the owning cart when you want full-cart analysis, workspace symbols, or
include diagnostics. Unrelated `.lua` files are intentionally left alone.

## Clean-switch checklist

- [ ] Install the `pico-lsp` binary or VS Code VSIX.
- [ ] Remove or disable the old PICO-8 server for this workspace.
- [ ] Enable `pico_lsp` (Neovim) or install the VSIX (VS Code) or `pico-lsp`
      (Helix).
- [ ] Open the owning `.p8` cart.
- [ ] Confirm exactly one language client is attached.
- [ ] Check that diagnostics and completion are not duplicated.
- [ ] If you use LuaLS, scope it away from cart-owned `.lua` files.
- [ ] Adjust `pico8.typeChecking` and diagnostics if the old server was noisier
      or quieter than you want.

## See also

- [Getting started](getting-started.md)
- [Editors](editors.md)
- [Configuration](configuration.md)
- [Troubleshooting](troubleshooting.md#included-lua-files-are-not-attached)
