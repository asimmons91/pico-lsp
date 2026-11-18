# PICO-8 tree-sitter grammar

Editor-oriented syntax highlighting for PICO-8 text carts. The grammar parses
`.p8` headers, named sections, code-tab markers, embedded Lua, and PICO-8
dialect syntax. It is intentionally independent of `pico-lsp`'s analysis
lexer/parser: highlighting must never replace or influence diagnostics,
completion, or semantic tokens.

The grammar is derived from
[tree-sitter-lua](https://github.com/tree-sitter-grammars/tree-sitter-lua) by
Munif Tanjim (MIT). See [LICENSE.tree-sitter-lua](LICENSE.tree-sitter-lua).

## Layout

- `grammar.js` — grammar source.
- `src/` — committed generated parser at ABI 14, plus the external scanner.
- `queries/highlights.scm`, `queries/indents.scm`, `queries/textobjects.scm` —
  editor queries.
- `test/corpus/` — grammar corpus tests.
- `test/golden/` — a representative cart and golden highlight captures.
- `test.sh` — generate, test, query-validate, compare the highlight golden,
  parse the full M1 cart corpus, and check truncated-cart safety.

## Requirements

The committed parser targets **ABI 14** for current Neovim and Helix releases.
Use tree-sitter CLI **0.24.7** to regenerate it:

```sh
npm install -g tree-sitter-cli@0.24.7
# or download the release binary:
curl -fsSL https://github.com/tree-sitter/tree-sitter/releases/download/v0.24.7/tree-sitter-linux-x64.gz \
  | gunzip > ~/.local/bin/tree-sitter
chmod +x ~/.local/bin/tree-sitter
```

Run the grammar checks:

```sh
editors/tree-sitter-pico8/test.sh
```

The script runs `tree-sitter generate`, rejects stale generated sources,
executes the corpus tests, validates the queries, parses every `.p8` cart under
`testdata/`, and checks truncated carts do not hang or crash the parser.

## Neovim

Compile the parser (Neovim expects a shared library named after the language):

```sh
cd /path/to/pico-lsp/editors/tree-sitter-pico8
mkdir -p ~/.local/share/nvim/site/parser
cc -shared -fPIC -I src src/parser.c src/scanner.c \
  -o ~/.local/share/nvim/site/parser/pico8.so
```

Copy the queries and start highlighting for the `pico8` filetype:

```sh
mkdir -p ~/.local/share/nvim/site/queries/pico8
cp queries/*.scm ~/.local/share/nvim/site/queries/pico8/
```

```lua
vim.api.nvim_create_autocmd("FileType", {
  pattern = "pico8",
  callback = function()
    pcall(vim.treesitter.start, 0, "pico8")
  end,
})
```

`pico8` is also the server language ID, so the same buffer can use the LSP and
tree-sitter highlighting. The existing `editors/nvim/syntax/pico8.vim` remains
available as a no-compiler fallback.

## Helix

Add the grammar and language to `~/.config/helix/languages.toml`. The grammar
source path must be absolute:

```toml
[[grammar]]
name = "pico8"
source = { path = "/absolute/path/to/pico-lsp/editors/tree-sitter-pico8" }

[[language]]
name = "pico8"
scope = "source.pico8"
file-types = ["p8"]
comment-token = "--"
grammar = "pico8"
language-servers = ["pico-lsp"]
```

Build and verify it:

```sh
hx --grammar build
hx --health pico8
```

Helix uses its own tree-sitter runtime; the ABI 14 parser is compatible with
current releases. If your Helix build only supports an older ABI, regenerate
the parser with a matching tree-sitter CLI and update the grammar dependency
version deliberately.

## Zed

Zed support is not shipped as an extension in this release; see
[../zed/README.md](../zed/README.md). The grammar and queries are suitable for a
thin Zed extension that declares:

- language name `pico8`, scope `source.pico8`, file extension `p8`;
- this repository's `editors/tree-sitter-pico8` tree-sitter source;
- the highlight query from `queries/highlights.scm`.

Until that extension exists, use `pico-lsp`'s semantic tokens for Lua code and
the editor's plain-text handling for non-code sections.

## PICO-8 coverage

The grammar includes explicit nodes for:

- cart version/header lines;
- `__lua__`, `__gfx__`, `__gff__`, `__label__`, `__map__`, `__sfx__`,
  `__music__`, and unknown sections;
- code-tab markers (`-->8`), kept out of Lua block parsing;
- `//` and `--` comments, long comments, short and long strings, PICO-8
  escape sequences, decimal/hex/binary numerals;
- shorthand `?` printing and single-line `if`/`while` forms, including
  same-line `else`;
- compound assignments, integer division (`\`), bit rotations/shifts, xor, and
  unary peek operators (`@`, `%`, `$`).

Non-code section payloads are opaque and are never parsed as Lua. The grammar
is highlighting-only; `pico-lsp` remains authoritative for diagnostics and
semantic behavior.
