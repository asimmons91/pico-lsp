# Helix setup

This config attaches `pico-lsp` to `.p8` carts and exercises the M0 server:
lifecycle, document sync, incremental edits, and position-encoding negotiation.
PICO-8 language features and syntax highlighting arrive in later milestones.

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

3. Edit, save, and close the file. The server must stay attached and Helix must
   not report protocol errors.

4. Quit Helix while the server is running (for example with `:q`). The server
   should exit cleanly rather than hang.

## Verifying the server logs

Helix writes language-server stderr to its own log, usually
`~/.cache/helix/helix.log`. Because `pico-lsp` sends routine logs to stderr
(never stdout), you can also point it at a dedicated file with `--log-file`.
Look for `pico-lsp started`, `initialized`, and `document changed` entries.

## Included `.lua` files

Helix attaches by file type. Included `.lua` files are not distinguished from
unrelated Lua files in M0; cart-owned attachment and include-aware detection
are later milestones. For now, prefer opening the `.p8` cart itself.

## Troubleshooting

- `hx --health pico8` reports whether the language server command is found.
- `hx --log /tmp/helix.log` writes Helix's own debug log to a separate file.
- `pico-lsp --version` verifies which binary Helix would run.
- If the server does not attach, check that `pico-lsp` is executable and on
  `PATH`.
