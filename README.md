# pico-lsp

`pico-lsp` is a fast, editor-agnostic language server for PICO-8 carts. It
understands the PICO-8 dialect, `.p8` tabs, non-recursive `#include` files, the
PICO-8 0.2.7 API, and LuaCATS type annotations.

It is a static Go binary with no runtime dependencies. It is not a generic Lua
language server: PICO-8 semantics win, including shorthand syntax, token and
character budgets, and the absence of the Lua standard library.

## AI-assisted development

**This is an AI vibe-coded project.** Most of the code, tests, and documentation
were produced by AI coding agents working from human direction and written
milestone specifications. A human maintainer reviews changes, runs the automated
test suite, and cuts releases, but this is not a traditionally hand-written
codebase and does not have a large human review team.

Treat it accordingly: inspect the implementation for your use case before
relying on it in critical workflows, expect occasional rough edges, and report
bugs with a minimal reproduction so they can be verified and fixed.

## Features

- Parse and analyze `.p8` carts, tabs, and included `.lua` or `.p8` files as one
  program, with source maps back to the original files.
- Syntax, include, undefined-global, type, and PICO-8 lint diagnostics.
- Document symbols, folding ranges, go-to-definition, references, highlights,
  rename, and workspace symbols.
- Completion, hover, signature help, semantic tokens, and inlay hints for user
  symbols and the PICO-8 0.2.7 API.
- LuaCATS classes, aliases, enums, params, returns, and gradual flow-aware type
  inference with `off`, `basic`, and `strict` checking.
- Cart token/character budget code lens, `pico8/cartInfo`, and limit warnings.
- Include diagnostics and quick fixes, generated annotations, and diagnostic
  suppression.
- Optional auto-attachment to cart-owned `.lua` buffers in Neovim.

## Install

v1 is distributed as checksummed GitHub release archives. Homebrew and Mason
packaging are deferred to post-v1; see the [roadmap](docs/ROADMAP.md).

Download the archive for your OS and architecture from the
[v1.0.0 release](https://github.com/asimmons91/pico-lsp/releases/tag/v1.0.0), or
install with Go:

```sh
go install github.com/asimmons91/pico-lsp/cmd/pico-lsp@v1.0.0
```

Verify the binary:

```sh
pico-lsp --version
```

See [docs/install.md](docs/install.md) for per-OS instructions, checksums,
upgrades, and uninstalling.

## Neovim 0.11+

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

To attach cart-owned `.lua` buffers automatically, add `editors/nvim` to your
runtime path and enable the helper plugin:

```lua
require("pico_lsp").setup()
```

Full setup, keymaps, the `pico8/cartInfo` statusline recipe, and the optional
Vim syntax file are in [editors/nvim/README.md](editors/nvim/README.md).

## Helix

Add the language and server definitions from
[editors/helix/languages.toml](editors/helix/languages.toml) to your Helix
`languages.toml`, then open a `.p8` cart. See
[editors/helix/README.md](editors/helix/README.md) for installation and smoke
tests.

## Configuration

The server reads the `pico8` configuration section from
`workspace/configuration`, falling back to `initializationOptions` or
`workspace/didChangeConfiguration`.

```json
{
  "pico8": {
    "apiVersion": "0.2.7",
    "typeChecking": "basic",
    "inlayHints": false,
    "maxProblems": 100,
    "limits": {
      "codeLens": true,
      "diagnostics": true
    },
    "diagnostics": {
      "undefinedGlobal": "warning",
      "unusedLocal": "off"
    }
  }
}
```

See [docs/configuration.md](docs/configuration.md) for every setting and
[docs/diagnostics.md](docs/diagnostics.md) for diagnostic codes, severities, and
suppression.

`pico8/cartInfo` returns the current cart budget and per-file contributions.
`pico8.showCartInfo` is the matching `workspace/executeCommand` command used by
the code lens. Both are documented in [docs/api.md](docs/api.md).

## Known limitations

- No formatter yet. An AST-aware formatter is planned as
  [M7](docs/M7-formatter.md) for v1.1.0.
- `.p8.png` carts are not supported yet. Read-only PNG carts are planned as
  [M8](docs/M8-p8png.md) for v1.1.0; until then, export a text `.p8`.
- VS Code and Zed clients remain in the
  [post-v1 backlog](docs/post-v1-backlog.md). The server remains
  editor-agnostic.
- Included `.lua` auto-attachment is a Neovim helper; Helix users open the
  owning `.p8` cart or configure their own attachment strategy.
- macOS and Windows binaries are unsigned. See
  [docs/troubleshooting.md](docs/troubleshooting.md) for Gatekeeper and
  SmartScreen notes.

## Development

The project requires Go 1.27.4 (pinned in `mise.toml`).

```sh
go test ./...
go test -race ./...
go vet ./...
golangci-lint run ./...
govulncheck ./...
CGO_ENABLED=0 go build -o pico-lsp ./cmd/pico-lsp
```

Release packaging is validated with:

```sh
goreleaser check
goreleaser release --snapshot --clean
```

Milestone specifications live in [docs/](docs/), starting with
[docs/ROADMAP.md](docs/ROADMAP.md).

## License

MIT. See [LICENSE](LICENSE).

PICO-8 is a trademark of Lexaloffle Games. This project is not affiliated with
or endorsed by Lexaloffle Games.
