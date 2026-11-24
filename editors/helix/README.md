# Helix setup

Helix attaches `pico-lsp` to `.p8` carts through its language-server
configuration. The server supports diagnostics, symbols, completion, hover,
signature help, semantic tokens, inlay hints, and code actions. The cart budget
is available through the `pico8/cartInfo` request; Helix has no built-in status
UI for it.

## Install

Install a GitHub release binary with [../../docs/install.md](../../docs/install.md),
or build from a checkout:

```sh
CGO_ENABLED=0 go build -trimpath -o pico-lsp ./cmd/pico-lsp
```

Confirm `pico-lsp --version` works in the same environment that launches Helix.

## languages.toml

Copy [languages.toml](languages.toml) into `~/.config/helix/languages.toml`, or
merge the definitions into an existing file:

```toml
[language-server.pico-lsp]
command = "pico-lsp"

[[language]]
name = "pico8"
scope = "source.pico8"
file-types = ["p8"]
comment-token = "--"
language-servers = ["pico-lsp"]
```

For a locally built binary, use an absolute path:

```toml
[language-server.pico-lsp]
command = "/absolute/path/to/pico-lsp"
```

To collect server logs:

```toml
[language-server.pico-lsp]
command = "pico-lsp"
args = ["--log-level", "debug", "--log-file", "/tmp/pico-lsp.log"]
```

Avoid pointing `--log-file` at a file inside the workspace if your editor
watches workspace files; use a temporary directory.

## Configuration

Server settings use the `pico8` section. Helix passes `config` on the language
server as initialization options, which the server accepts as
`initializationOptions.pico8`:

```toml
[language-server.pico-lsp]
command = "pico-lsp"
config = { pico8 = { typeChecking = "basic", inlayHints = false } }
```

See [../../docs/configuration.md](../../docs/configuration.md) for every setting
and [../../docs/editors.md](../../docs/editors.md) for the tested matrix.

## Smoke test

1. Confirm Helix recognizes the language and server configuration:

   ```sh
   hx --health pico8
   ```

   The configured language server should be listed without an error. If you
   have not installed the optional tree-sitter grammar, plain highlighting is
   expected; the LSP still attaches.

2. Open the test cart:

   ```sh
   hx testdata/smoke.p8
   ```

3. Check diagnostics, completion, hover, signature help, and symbols. Open a
   cart with a deliberate error and confirm the diagnostic lands on the right
   line.

4. Save, edit, and close the buffer. The server should stay attached and Helix
   should not report protocol errors.

5. Quit Helix while the server is running. It should exit cleanly rather than
   hang.

## P8SCII glyph input

Type `p8_left`, `p8_heart`, or another glyph trigger in code or a string and
accept the completion to insert the glyph. The `p8-` form is accepted for
pico8-ls compatibility. Control entries such as `p8_font_custom` (`\14`) are
offered only inside strings. Helix receives the standard completion items;
there is no native Helix picker in this milestone. Custom font slots use the
`pico8.glyphs` setting documented in
[../../docs/configuration.md](../../docs/configuration.md#p8scii-glyph-snippets).

## Read-only `.p8.png` carts

Helix does not have a portable command for opening virtual documents. Decode a
PNG cart to a text file and open it:

```sh
pico-lsp decode --output /tmp/cart.p8 path/to/cart.p8.png
hx /tmp/cart.p8
```

This is a static fallback; re-run the command after replacing the PNG. Editing
the PNG requires exporting a text cart from PICO-8.

## Included `.lua` files

Helix attaches by file type. The server analyzes included `.lua` files,
including closed includes, whenever the owning `.p8` cart is attached. v1 does
not ship a Helix cart-ownership helper, so open the owning cart for analysis.
If you attach Helix to `.lua` buffers in a PICO-8 project, avoid running
`lua-language-server` on the same buffers.

## Syntax highlighting

The repository ships an editor-oriented tree-sitter grammar at
[../tree-sitter-pico8](../tree-sitter-pico8). It highlights cart headers,
section names, tab markers, Lua code, PICO-8 dialect forms, and non-code
payloads without changing the `pico8` language ID or the server contract.

Add the grammar to `~/.config/helix/languages.toml` (use an absolute path):

```toml
[[grammar]]
name = "pico8"
source = { path = "/absolute/path/to/pico-lsp/editors/tree-sitter-pico8" }
```

Then add `grammar = "pico8"` to the `[[language]]` entry and run:

```sh
hx --grammar build
hx --health pico8
```

The committed parser targets ABI 14 for current Helix releases. If your Helix
build does not load it, see the grammar's
[README](../tree-sitter-pico8/README.md) for regeneration notes. Without the
grammar, the LSP still attaches and semantic tokens continue to highlight Lua
code; non-code sections fall back to plain text.

## Troubleshooting

- `hx --health pico8` reports whether the language server command is found.
- `hx --log /tmp/helix.log` writes Helix's own debug log to a separate file.
- `pico-lsp --version` verifies which binary Helix would run.
- Use `--log-file` to capture server logs.
- See [../../docs/troubleshooting.md](../../docs/troubleshooting.md) for more.
