# Command-line reference

`pico-lsp` normally runs as a language server over stdio and is launched by your
editor. The same binary also has a small set of command-line flags and a
`decode` subcommand.

## Server mode

```sh
pico-lsp
```

With no subcommand, `pico-lsp` starts the LSP server on stdin/stdout. Editors
start this process for you. Do not run it in a terminal and type JSON into it;
use your editor's LSP configuration instead.

| Flag | Default | Meaning |
|---|---|---|
| `--version` | — | Print the version and exit. |
| `--log-file PATH` | stderr | Append logs to `PATH` instead of stderr. |
| `--log-level LEVEL` | `info` | `debug`, `info`, `warn`/`warning`, or `error`. |

Examples:

```sh
pico-lsp --version
pico-lsp --log-level debug --log-file /tmp/pico-lsp.log
```

`--log-file` is useful when an editor launches the server with an unknown
working directory. Use a temporary path outside the workspace so file watchers
do not react to log writes.

## Decode a `.p8.png` cart

```sh
pico-lsp decode [--output PATH] CART.p8.png
```

`decode` reads a PNG cart and writes the text `.p8` source to stdout, or to
`PATH` with `--output`. It never writes a PNG file.

```sh
pico-lsp decode --output cart.p8 cart.p8.png
```

This is the fallback for editors without virtual-document support. Neovim's
`pico8.nvim` client can open PNG carts directly.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success, normal shutdown, `--version`, or `decode` completed. |
| `1` | Runtime or I/O failure: decode error, log-file open error, output write error, or an unexpected connection close. |
| `2` | Usage error: unknown flag, missing flag value, unexpected argument, or missing decode input. |

## stdout rule

In server mode, stdout carries only LSP protocol frames. Routine logs go to
stderr or `--log-file`. In `decode` mode, the decoded text is the stdout output;
that is the command's result, not server output.

If an editor reports malformed LSP messages, check that no wrapper script or
other tool writes to the same stdout stream. See
[Logs and stdout](troubleshooting.md#logs-and-stdout).

## Help

```sh
pico-lsp --help
pico-lsp decode --help
```

The top-level help lists the server flags. The `decode` subcommand has its own
help and usage line.

## See also

- [Getting started](getting-started.md)
- [Install](install.md)
- [Troubleshooting](troubleshooting.md)
