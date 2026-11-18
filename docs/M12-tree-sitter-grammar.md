# M12 — PICO-8 Tree-sitter Grammar

**Status:** Done
**Depends on:** M11  
**Release target:** v1.2.0 (batched with M11, M13, M14)  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

The project ships an editor-oriented tree-sitter grammar for `.p8` carts. It
highlights cart sections, tab markers, PICO-8 dialect syntax, and embedded Lua
code without trying to replace the server's hand-written analyzer. The grammar
can be consumed by Helix, Neovim, Zed, and other tree-sitter editors.

**Demo:** Opening a `.p8` cart in a tree-sitter editor highlights `__lua__`,
`__gfx__`, `__map__`, `__label__`, tab boundaries, shorthand `?`, `if (cond)`
statements, peeks, strings, comments, numbers, and dialect operators. Non-code
section payloads remain visible but are not parsed as Lua.

## Scope

### In scope

- Grammar sources under `editors/tree-sitter-pico8/`.
- `.p8` cartridge sections and version/header lines.
- Tab boundaries and code-tab markers.
- Embedded Lua/PICO-8 code, including keywords, identifiers, numbers, strings,
  comments, and operators.
- PICO-8 dialect forms: `//` comments, `?` shorthand, single-line `if`/`while`,
  `+=`-style assignments, integer division, peeks, bits, and shorthand syntax.
- Non-code sections kept visible and highlighted without Lua payload parsing.
- Queries for highlights, indents, and text objects where tree-sitter supports
  them.
- Corpus and edge-case grammar tests.
- Documentation for Neovim, Helix, and Zed consumption.

### Out of scope

- Replacing the server lexer/parser or sharing analysis semantics with it.
- Semantic tokens, diagnostics, completion, or other LSP behavior.
- Full semantic validation of non-code section payloads.
- Shipping editor plugins or binary packaging.
- A new PICO-8 dialect; the grammar follows the server corpus.

## Requirements

- The grammar parses the full M1 corpus without errors caused by valid carts.
- `.p8` header, section names, version line, and tab markers are explicit
  grammar nodes so queries can target them.
- `__lua__` code uses a Lua-style grammar with PICO-8 extensions. Non-code
  sections are tokenized as section payloads, not parsed as Lua.
- Shorthand constructs are line-bounded, matching the server lexer/parser.
- Comments and strings cannot swallow subsequent code or sections.
- Query files provide sensible highlights for:
  - cart headers and section names;
  - tab markers;
  - Lua keywords, functions, variables, strings, numbers, comments, and
    operators;
  - PICO-8 shorthand and peek operators;
  - non-code section payloads.
- Indent and text-object queries are included where the target editor supports
  them.
- Generated parser sources are committed so editor consumers do not need the
  tree-sitter CLI to use a release.
- The grammar is tested with the tree-sitter CLI in CI and a repository script
  (`editors/tree-sitter-pico8/test.sh` or equivalent).
- Neovim and Helix integration documentation explains how to install/use the
  grammar without changing the server's language ID.
- The server remains the source of truth for diagnostics and semantic behavior;
  the grammar must not be used by analysis.

## Testing

- `tree-sitter generate`, `tree-sitter test`, and corpus fixtures covering every
  `.p8` section and tab boundary.
- Golden highlight-query tests for representative carts and dialect snippets.
- Edge cases: nested brackets/strings/comments, empty tabs, shorthand at line
  end, `//` comments, multi-line strings, non-code payloads, and cart headers.
- Fuzz or corpus-driven parse checks for malformed/truncated carts; grammar
  errors must remain local and not hang.
- CI job for grammar generation/tests in addition to the existing Go matrix.
- Documentation smoke test with the repository's Neovim/Helix integration.

## Exit criteria

- [x] The grammar parses the full corpus and every `.p8` section/tab form.
- [x] Highlight, indent, and text-object queries work in supported editors.
- [x] Non-code sections remain visible and are not parsed as Lua.
- [x] Generated parser sources and usage docs are committed.
- [x] Grammar CI and corpus/edge-case tests pass.
- [x] The server parser remains independent and unchanged.
- [x] Existing server tests and performance budgets do not regress.

## Implementation notes

- The grammar is derived from MIT-licensed `tree-sitter-lua` by Munif Tanjim
  and recorded in `editors/tree-sitter-pico8/LICENSE.tree-sitter-lua`.
- The committed parser is generated with tree-sitter CLI 0.24.7 and targets
  ABI 14 for current Neovim and Helix releases. `test.sh` rejects a different
  CLI series.
- Cart sections and tab markers are explicit nodes. Section headers are
  line-start external tokens; non-code payloads use an opaque line token and
  are never parsed as Lua.
- PICO-8 additions include `//` comments, `?` printing, line-bounded shorthand
  `if`/`while`, compound assignment operators, integer division, bit
  operations, unary peeks, hex/binary numbers, and P8SCII string escapes.
  Shorthand and print statements consume their line terminator so a following
  line cannot attach to the expression or to an `else` branch.
- `editors/tree-sitter-pico8/test.sh` runs generation, checks generated sources
  are current, runs corpus tests, validates queries, parses every `.p8` under
  `testdata/`, and checks truncated carts for hangs or crashes.
- CI adds a dedicated `tree-sitter grammar` job. Edits under `internal/` are
  deliberately not required: analysis still uses the hand-written lexer/parser.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Grammar and server lexer diverge | Shared corpus fixtures and dialect edge-case tests; grammar is highlighting-only. |
| Non-code sections parsed as Lua | Explicit section grammar with opaque payload nodes. |
| Generated sources drift from grammar | Regenerate in CI and fail on uncommitted generated diffs. |
| Shorthand grammar is line-sensitive | Dedicated line-bounded rules and corpus cases matching the server parser. |
| Editor install friction | Document Neovim, Helix, and Zed registration with the existing language IDs. |
