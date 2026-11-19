# Neovim 0.11+ setup

This directory contains:

- the recommended `vim.lsp.config` setup for `.p8` carts;
- an optional helper for attaching `pico-lsp` to cart-owned `.lua` buffers;
- `ftdetect`, `syntax`, and `ftplugin` files for PICO-8 carts;
- a headless smoke test for the helper.

The syntax, ftplugin, and helper are optional. The server works with the base
LSP config alone.

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

## P8SCII glyph input

Type `p8_left` in code or a string and accept the completion to insert `⬅️`.
The `p8-` form is accepted for pico8-ls compatibility. Control entries such as
`p8_font_custom` (`\14`) are offered only inside strings.

The helper adds a searchable picker:

```vim
:Pico8InsertGlyph
```

Pass a name to insert directly:

```vim
:Pico8InsertGlyph left
```

Custom font slots are declared in `pico8.glyphs`; see the
[configuration reference](../../docs/configuration.md#p8scii-glyph-snippets).
The server inserts the canonical Unicode spelling for the chosen byte because
it cannot inspect custom font pixels.

## Syntax highlighting

The repository ships an editor-oriented tree-sitter grammar at
[../tree-sitter-pico8](../tree-sitter-pico8). It keeps the `pico8` language ID
used by the server and adds highlighting for cart headers, sections, tab
markers, dialect syntax, and non-code payloads.

Compile the committed parser and install the queries:

```sh
cd /path/to/pico-lsp/editors/tree-sitter-pico8
mkdir -p ~/.local/share/nvim/site/parser
cc -shared -fPIC -I src src/parser.c src/scanner.c \
  -o ~/.local/share/nvim/site/parser/pico8.so

mkdir -p ~/.local/share/nvim/site/queries/pico8
cp queries/*.scm ~/.local/share/nvim/site/queries/pico8/
```

Start it for `.p8` buffers:

```lua
vim.api.nvim_create_autocmd("FileType", {
  pattern = "pico8",
  callback = function()
    pcall(vim.treesitter.start, 0, "pico8")
  end,
})
```

The committed parser targets ABI 14 for current Neovim releases. See the
grammar [README](../tree-sitter-pico8/README.md) for regeneration details.

If you prefer not to compile a parser, the included `syntax/pico8.vim` remains
a fallback. Add the runtime path as above; `ftdetect` sets `filetype=pico8` for
`.p8` files. LSP semantic tokens cover Lua code, while the syntax file also
highlights non-code `.p8` sections.

## Comment settings

`ftplugin/pico8.vim` sets buffer-local comment options for carts:

```vim
setlocal commentstring=--\ %s
setlocal comments=:---,:--
```

This makes `gcc`/`gc` and comment-aware formatting use PICO-8's `--` comment
token. Neovim configs normally enable ftplugins with `filetype plugin on`;
LazyVim and most distributions do this by default. Add
`vim.cmd("filetype plugin on")` if your config turns it off. Like the syntax
file, the ftplugin requires adding `editors/nvim` to `runtimepath`.

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
    if err or not result then return end
    local parts = {
      ("%d / %d tokens"):format(result.tokens, result.tokenLimit),
      ("%d / %d chars"):format(result.chars, result.charLimit),
    }
    if result.compressedStatus ~= "unavailable" then
      table.insert(parts, ("%d / %d compressed"):format(
        result.compressed, result.compressedLimit))
    end
    print(table.concat(parts, " · "))
  end)
end)
```

The cart root also gets a budget code lens. Run it to show the
`pico8.showCartInfo` summary. The lens marks compressed budgets with
`(warning)` from 90% of the limit and `(over limit)` above it.

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
