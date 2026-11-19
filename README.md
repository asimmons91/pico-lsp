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
- Syntax, include, undefined-global, type, and PICO-8 lint diagnostics, with
  LSP 3.17 push/pull transport selection and related-file pull reports.
- Document symbols, folding ranges, go-to-definition, references, highlights,
  rename, and workspace symbols.
- Completion, hover, signature help, semantic tokens, and inlay hints for user
  symbols and the PICO-8 0.2.7 API.
- P8SCII glyph input: `p8_left`-style completion in code and strings, a
  searchable glyph picker, and custom font-slot entries through
  `pico8.glyphs`.
- LuaCATS classes with inheritance and `(exact)` field checks, generic classes
  and functions, overloads, operator annotations, tagged-union narrowing,
  metatable factory inference, aliases, enums, params, returns, and gradual
  flow-aware type inference with `off`, `basic`, and `strict` checking.
- AST-guided `textDocument/formatting` that preserves shorthand, comments,
  annotations, includes, and non-code cart sections.
- An editor-oriented tree-sitter grammar for `.p8` carts, with committed ABI 14
  parser sources, highlight/indent/text-object queries, and Neovim/Helix setup
  docs. It is highlighting-only and never used by analysis.
- Read-only `.p8.png` support through a decoded virtual document, `:Pico8OpenPng`
  in Neovim, and the `pico-lsp decode` CLI command.
- Cart token/character/compressed-size budget code lens, `pico8/cartInfo`, and
  limit warnings. Compressed sizes come from a PICO-8-compatible pxa
  compressor.
- Include diagnostics and quick fixes, generated annotations, and diagnostic
  suppression.
- A VS Code extension with a contributed `pico8` language, TextMate
  highlighting, managed checksummed server downloads, `.p8`/Lua attachment, and
  cart-budget status bar.
- Conservative refactors: extract to local function, safe global-to-local,
  generated `---@class`, `if`/`while` shorthand conversion, exact-class
  missing-field inserts, and confirmed unused-local removal. All edits are
  previewable workspace edits in original source coordinates.
- Optional auto-attachment to cart-owned `.lua` buffers in Neovim.

## Install

v1 is distributed as checksummed GitHub release archives. Homebrew and Mason
packaging are deferred to post-v1; see the [roadmap](docs/ROADMAP.md).

Download the archive for your OS and architecture from the
[v1.4.0 release](https://github.com/asimmons91/pico-lsp/releases/tag/v1.4.0), or
install with Go:

```sh
go install github.com/asimmons91/pico-lsp/cmd/pico-lsp@v1.4.0
```

Verify the binary:

```sh
pico-lsp --version
```

See [docs/install.md](docs/install.md) for per-OS instructions, checksums,
upgrades, and uninstalling.

VS Code support ships as `pico-lsp-vscode-<version>.vsix` with the matching
release. Install it with **Extensions: Install from VSIX...**; see
[editors/vscode/README.md](editors/vscode/README.md). Marketplace and Open VSX
publishing are follow-ups.

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
      "mode": "auto",
      "undefinedGlobal": "warning",
      "unusedLocal": "off"
    },
    "refactors": {
      "enabled": true
    },
    "glyphs": [
      { "name": "logo", "byte": 128, "description": "custom font slot 0x80" }
    ]
  }
}
```

See [docs/configuration.md](docs/configuration.md) for every setting and
[docs/diagnostics.md](docs/diagnostics.md) for diagnostic codes, severities, and
suppression.

`pico8/cartInfo` returns the current cart budget (tokens, characters, and
compressed size) and per-file contributions. `pico8.showCartInfo` is the
matching `workspace/executeCommand` command used by the code lens.
`pico8/glyphs` and `pico8.insertGlyph` power the P8SCII glyph picker. All are
documented in [docs/api.md](docs/api.md).

## Known limitations

- `.p8.png` carts are supported read-only. Edit and save an exported text `.p8`
  when you need to change the cart; the server never writes PNG carts.
- VS Code is supported by the M15 extension; Zed remains in the
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
editors/tree-sitter-pico8/test.sh
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
