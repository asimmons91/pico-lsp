# Helix setup

This config attaches `pico-lsp` to `.p8` carts. The server syncs documents
incrementally, reports syntax and include diagnostics, and provides document
symbols and folding ranges. Syntax highlighting requires a tree-sitter grammar
and arrives in a later milestone; token-budget UI arrives in M5.

## Prerequisites

Build or install `pico-lsp` so it is on `PATH`:

```sh
go install github.com/asimmons91/pico-lsp/cmd/pico-lsp@latest
# or, from a checkout:
go build -o ./pico-lsp ./cmd/pico-lsp
```

## languages.toml

Add this to `~/.config/helix/languages.toml` (or a project-local
`.helix/languages.toml`):

```toml
[language-server.pico-lsp]
command = "pico-lsp"
args = ["--log-level", "debug", "--log-file", "/tmp/pico-lsp.log"]

[[language]]
name = "pico8"
scope = "source.pico8"
file-types = ["p8"]
comment-token = "--"
language-servers = ["pico-lsp"]
```

`language-servers` must reference the name defined in the
`[language-server.<name>]` table. Remove the `args` entry if you do not want a
debug log file.

For a locally built binary, use an absolute path in `command`:

```toml
[language-server.pico-lsp]
command = "/absolute/path/to/pico-lsp"
```

## Smoke test

1. Confirm Helix recognizes the language and server configuration:

   ```sh
   hx --health pico8
   ```

   The configured language server should be listed without an error. Missing
   highlight or textobject queries are expected in v1 because no PICO-8
   tree-sitter grammar is shipped yet; the server still attaches.

2. Open the test cart:

   ```sh
   hx testdata/smoke.p8
   ```

3. Open the symbol picker with `<space>s` to inspect tabs, functions, and
   includes. Open a cart with a deliberate syntax error and confirm the
   diagnostic lands on the right line.

   Helix 25.07 has no code-folding feature or LSP folding consumer, so the
   server's folding ranges are not displayed there. Folding works in Neovim and
   clients that support `textDocument/foldingRange`.

4. Edit, save, and close the file. The server must stay attached and Helix must
   not report protocol errors.

5. Quit Helix while the server is running (for example with `:q`). The server
   should exit cleanly rather than hang.

## Verifying the server logs

Helix writes language-server stderr to its own log, usually
`~/.cache/helix/helix.log`. Because `pico-lsp` sends routine logs to stderr
(never stdout), you can also point it at a dedicated file with `--log-file`.
Look for `pico-lsp started`, `initialized`, and `document changed` entries.

## Included `.lua` files

Helix attaches by file type. The server analyzes included `.lua` files,
including diagnostics for closed includes, whenever it is attached to the
owning `.p8` cart. Open the cart itself; automatic `.lua` attachment for
cart-owned files is editor-specific and not part of the Helix config in v1.

## Troubleshooting

- `hx --health pico8` reports whether the language server command is found.
- `hx --log /tmp/helix.log` writes Helix's own debug log to a separate file.
- `pico-lsp --version` verifies which binary Helix would run.
- If the server does not attach, check that `pico-lsp` is executable and on
  `PATH`.
