# M1 — Cart Model & Syntax Front-End

**Status:** Planned  
**Depends on:** M0  
**Blocks:** M2  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

The server understands the structure of a PICO-8 cart and the PICO-8 dialect of Lua. It can flatten tabs and `#include` files into one analyzed program, parse it with useful error recovery, count tokens and characters, and provide syntax diagnostics, document symbols, and folding ranges.

**Demo:** Open a multi-tab `.p8` cart that includes `lib/player.lua`. The server reports a deliberate syntax error at the correct file and line, lists functions/tabs/includes in the outline, folds blocks and comments, and shows token/character counts that match PICO-8's `info()` output.

## Scope

### In scope

- `.p8` text cartridge reader.
- Standalone `.lua` support.
- Code tab handling (`-->8`).
- `#include` resolution, flattening, source maps, and include records.
- Full lexer for the PICO-8 dialect.
- Error-recovering parser and AST.
- Comment/trivia preservation for later annotations and docs.
- Syntax diagnostics.
- Document symbols and folding ranges.
- Token and character counting for the flattened cart.
- A real-cart test corpus.

### Out of scope

- Semantic analysis, scopes, symbols, and navigation (M2).
- Built-in API metadata (M2/M3).
- Completion, hover, and signature help (M3).
- Type annotations and inference (M4).
- Token/char limit diagnostics and code lens (M5).
- Formatting (M7).
- `.p8.png` decoding (M7).

## Deliverables

1. `internal/cart` package that reads `.p8` sections and produces a `Cart` with flattened source and source map.
2. Include resolution with diagnostics and records for editor features.
3. `internal/syntax` lexer covering the complete dialect.
4. AST and parser with error recovery and precise spans.
5. Syntax diagnostics published for every cart file.
6. Document symbols and folding ranges.
7. Token/char count API with golden tests.
8. Corpus tests and parser fuzz targets.

## PICO-8 dialect requirements

The lexer/parser must handle the following. Each item needs a dedicated test.

| Construct | Required behavior |
|---|---|
| `?` print shorthand | Statement-level `?expr[, ...]` becomes a print call. Line-bounded. Can follow a shorthand `if`/`while` body on the same line. |
| Single-line `if (cond) stmt` | Parenthesized condition required. Body is the rest of the line. A subsequent `else` on the same line belongs to it; `else` on the next line belongs to an enclosing block. |
| Single-line `while (cond) stmt` | Parenthesized condition required. Body is the rest of the line. |
| Compound assignment | `+= -= *= /= \= %= ^= ..= &= |= ^^= <<= >>= >>>= <<>= >><=`. Count as two tokens for PICO-8 limits. |
| `!=` | Alias for `~=`. |
| Bitwise operators | `& | ^^ << >> >>> <<> >><` and unary `~`. |
| Integer division | `\` only. `//` is a comment, never division. |
| Peek operators | Prefix `@`, `%`, `$` read a byte/word/dword. They are read-only and must not parse as assignment targets. `%` remains valid as binary modulo. |
| Comments | `--`, `--[[ ... ]]`, long comments with levels (`--[=[ ... ]=]`), and `//` to end of line. |
| Strings | Single/double quoted, long strings (`[[...]]`, `[=[...]=]`), P8SCII escapes (`\*`, `\#`, `\-`, `\|`, `\+`, `\^`, `\a`, `\f`, `\n`, etc.). Unknown escapes should not crash or produce syntax errors. |
| Numbers | Decimal, hex (`0x11`), fractional hex (`0x11.4000`), binary (`0b1010`), fractional binary, scientific notation, leading-dot decimals. |
| `goto` / labels | `goto name`, `::name::`; labels scoped to a function. |
| `#include` | Directive line, resolved by the cart layer before lexing. Included file text is parsed as part of the cart. |
| `-->8` | Tab boundary comment in `.p8` files. Used by outline/folding; not a statement. |
| Case folding | PICO-8 has no upper/lowercase distinction. Preserve original text in the AST, but semantic lookup later normalizes ASCII to lowercase. |
| Button glyphs | UTF-8 glyphs used in source should be accepted as tokens/values where PICO-8 accepts them. At minimum, do not corrupt spans or crash. |

## Cart model

### `.p8` parsing

- Accept a UTF-8 text file beginning with the PICO-8 header:

  ```
  pico-8 cartridge // pico-8.com
  version N
  ```

- Recognize sections at line start: `__lua__`, `__gfx__`, `__gff__`, `__label__`, `__map__`, `__sfx__`, `__music__`.
- Code is the content of `__lua__` until the next section marker. Missing `__lua__` means an empty cart.
- Preserve unknown section names instead of failing.
- Do not validate graphics/audio section contents in v1.
- Reject `.p8.png` with a clear diagnostic: “PICO-8 PNG carts are not supported yet; save as .p8.”
- A standalone `.lua` file is treated as a one-file cart with no container parsing.

### Code tabs

- Lines beginning with `-->8` (after optional whitespace) start a new code tab.
- Tabs are concatenated in order for analysis and token counting.
- For outline purposes, expose each tab as a namespace symbol. If the marker includes a name, use it; otherwise synthesize `Tab 1`, `Tab 2`, etc. Verify PICO-8's exact tab-name convention against real carts and update if needed.
- `-->8` remains a comment for the parser; it must not produce a syntax error.

### Include resolution

PICO-8 pastes included file contents in place of the `#include` line. Includes are relative to the cart and **not recursive**.

Algorithm:

1. Read the root cart text from the open-document overlay if available, otherwise from disk.
2. Find `#include` directives at the start of a line (after optional whitespace), outside long strings/comments. Match the directive case-sensitively as `#include`; PICO-8 source is lowercase. Trim whitespace; treat the rest of the line as the path. Support optional surrounding quotes for editor ergonomics.
3. Resolve the path relative to the cart's directory. Accept `/` and `\` separators. Reject NUL bytes and paths that escape the workspace root only if a configured security boundary exists; v1 permits `..` within the workspace.
4. Read the included file from its overlay if open, otherwise from disk. Cap file size (for example, 1 MB) and report `include-unreadable` for binary/oversized files.
5. Append the file contents to the flattened buffer with a trailing newline if missing.
6. Record an `Include{Path, ResolvedURI, DirectiveSpan, FileSpan, Missing, Nested}` for editor features.
7. Do **not** recursively process `#include` lines inside the included file. If one is found, leave it in the flattened text and emit `include-nested` on the included file's directive line. The parser may then report a syntax error; that is faithful to PICO-8.
8. Missing files produce `include-missing` at the directive and contribute no text. Keep analyzing the rest of the cart.
9. Duplicate includes are pasted each time, matching PICO-8. Do not silently deduplicate.

### Source map

Build a segment map from flattened byte ranges to original files:

```go
type FileID int

type Segment struct {
    FlatStart  int
    FlatEnd    int
    FileID     FileID
    FileOffset int
}

type File struct {
    URI  protocol.DocumentURI
    Text string
    Index *source.LineIndex
}

type Cart struct {
    RootURI   protocol.DocumentURI
    Files     []*File
    Segments  []Segment
    Includes  []Include
    Flat      []byte
    Tokens    int
    Chars     int
}
```

Requirements:

- Every token and AST node gets a span `{FileID, Start, End}` in original-file byte offsets.
- Tokens never expose flattened offsets to features. Mapping happens once in the lexer.
- Files that are included twice may appear multiple times in `Segments`; spans are still valid per file. Duplicate definitions are a semantic concern for M2.
- Include directive spans point at the root/included file line that contained the directive.
- Reflatten and reparse the whole cart whenever any member file changes. This is cheap within PICO-8 limits.

### File ownership and cart discovery

- A `.p8` file owns itself.
- A `.lua` file belongs to the nearest ancestor directory `.p8` cart whose include list contains it.
- If no cart includes it, analyze it as a standalone one-file cart.
- If multiple carts include it, the nearest ancestor wins. Log at debug and expose the chosen owner in `pico8/cartInfo` later. Do not merge diagnostics from multiple carts in v1.
- Include discovery for an open `.lua` may scan parent directories for `.p8` files and inspect their include lines. Cache the result and invalidate on cart changes.
- Do not eagerly scan the entire workspace at startup. Resolve ownership on demand.

### Token and character counts

Implement a dedicated counter over the flattened lexer token stream. Do not count AST nodes.

PICO-8 counting rules:

- Count: identifiers, literals, operators, opening brackets, and keywords except `local` and `end`.
- Do not count: closing brackets, `,`, `;`, `.`, `:`, `::`, `local`, `end`, comments, whitespace.
- A string counts as one token regardless of length.
- Compound assignment counts as two tokens.
- A unary `-` or `~` applied to a numeric literal counts as part of the literal.
- `?` shorthand counts consistently with `print` plus its argument list; verify against PICO-8 and shrinko8 fixtures.
- Count the assembled cart including all included files.
- Character count is the number of Unicode code points in the assembled code (PICO-8 stores glyphs as Unicode in `.p8`). Document the P8SCII nuance and verify with fixtures.
- Expose `Tokens` and `Chars` on `Cart`; M5 surfaces them in the editor.

Golden tests must cover:

- comments and annotations (zero tokens);
- `local`/`end` (zero);
- closing brackets (zero);
- compound assignments (two);
- shorthand if/while and `?`;
- long strings (one);
- includes.

Cross-check at least 20 carts against PICO-8's `info()` or `shrinko8 --count`. Store expected counts in `testdata/tokens/`.

## Lexer requirements

- Produce tokens with kind, original text, value, and original-file span.
- Preserve all comments and doc comments separately, in source order. The parser uses them for doc comments; M4 uses them for LuaCATS.
- Token kinds include: identifiers, keywords, numbers, strings, operators, punctuation, comments, `#include` placeholder, `-->8`, and EOF.
- Distinguish `%` unary peek from `%` binary modulo by parser context, not lexer guessing.
- Handle `...`, `..`, `.`, `..=`, `::`, `!=`, `~=`, `^^`, `<<>`, `>><`, `>>>`, and all compound assignment forms with maximal munch.
- Reject invalid bytes with a diagnostic, not a panic. Preserve offsets.
- Never allocate a new string per identifier when avoidable; intern or slice the source.

## Parser requirements

### AST shape

Expressions:

- `Nil`, `Bool`, `Number`, `String`, `Vararg`
- `Name`, `Unary`, `Binary`
- `Field`, `Index`, `Call`, `MethodCall`
- `Function` (anonymous)
- `Table` with array items, keyed fields, and `[expr]` keys
- `Paren`

Statements:

- `Local`, `LocalFunction`, `Assign` (with optional compound operator)
- `CallStatement`
- `Do`, `While`, `Repeat`, `If`, `NumericFor`, `GenericFor`
- `FunctionDecl` (including `a.b:c`)
- `Return`, `Break`, `Goto`, `Label`
- `PrintShorthand` (`?`)
- `Empty` (`;`)
- Include/tab markers are represented outside the AST as cart metadata.

Every node exposes `Span()`. Declarations also expose a selection span for the identifier.

### Precedence

Use PICO-8's precedence, not Lua 5.3/5.4's:

1. Calls, indexing, field access (left-to-right)
2. `^` (right-to-left)
3. Unary `-`, `~`, `not`, `@`, `%`, `$` (right-to-left)
4. `*`, `/`, `%`, `\` (left-to-right)
5. `+`, `-`
6. `..` (right-to-left)
7. `<<`, `>>`, `>>>`, `<<>`, `>><`
8. `&`
9. `~`/`^^` binary xor
10. `|`
11. Comparisons
12. `and`
13. `or`

### Error recovery

- Accumulate errors in a slice; do not abort on the first error.
- Cap errors at 100 per parse; add a final “too many errors” diagnostic.
- Synchronize at statement starts and block terminators:
  `local`, `function`, `if`, `for`, `while`, `repeat`, `return`, `break`, `goto`, `?`,
  `end`, `else`, `elseif`, `until`, `#include`, `-->8`, EOF.
- Guarantee forward progress: every recovery loop consumes at least one token.
- On missing `end`/`until`/`)`, report at the last token and close the block implicitly so later code is still analyzed.
- Truncated input at any byte offset must produce a bounded number of diagnostics without panic or hang.

### Shorthand parsing rules

These need dedicated regression tests:

- `if (x) y` → `If` with body on the same line.
- `if (x) y else z` → both branches on the same line.
- `if (x) y` followed by `else` on the next line → shorthand ends before the next line; the `else` belongs to an enclosing standard `if`, or is a syntax error if none exists.
- `while (x) y` → body on the same line.
- `while (x)` with no body and next-line code → syntax error at end of line, not consume the next line.
- `?x` and `?x,y` → print shorthand; arguments are the rest of the line.
- `if (x) ?y` and `while (x) ?y` are valid.
- Compound assignment on a field/index, e.g. `t.x += 1`, is valid.
- `@x = 1` is a syntax error (“peek operators are read-only”).

## Editor features in this milestone

### Syntax diagnostics

- Emit `syntax` diagnostics with exact ranges and a concise message.
- Include an expected/found hint where useful: “expected `end` to close `function` at 12:3”.
- Publish diagnostics for every file in the cart, including closed includes.
- Clear diagnostics for files removed from the include graph.

### Document symbols

- Hierarchical symbols for tabs, functions, global assignments, local declarations, methods, labels, and include files.
- Use `SymbolKind.Function`, `Variable`, `Method`, `Namespace`, `Module`, `Constant`, `Struct` where appropriate.
- `range` covers the full declaration; `selectionRange` covers the name.
- Do not include every local inside functions by default if it makes the outline unusable; include top-level locals and function declarations. Revisit in M3.
- Classes from LuaCATS annotations are added in M4.

### Folding ranges

- Function bodies, `if`/`elseif`/`else` branches, loops, `do` blocks, table constructors spanning lines.
- Multi-line comments and long strings.
- `-->8` tabs.
- `.p8` sections above `__lua__` and after it.
- Do not fold single-line shorthand statements.

### Selection ranges

Optional in M1; if implemented, return the AST node chain at the cursor, from identifier to expression/statement/block.

## Architecture and files

```
internal/cart/cart.go          Cart, sections, file ownership
internal/cart/sections.go      __lua__ and section scanner
internal/cart/include.go       directive discovery, resolution, flattening
internal/cart/sourcemap.go     Segment map and span mapping
internal/cart/tokens.go        PICO-8 token/char counter
internal/cart/project.go       cart discovery and nearest-owner logic
internal/syntax/token.go       token kinds and spans
internal/syntax/lexer.go       lexer
internal/syntax/ast.go         AST nodes
internal/syntax/parser.go      parser and recovery
internal/syntax/comments.go    comment/trivia index
internal/syntax/errors.go      structured parse errors
internal/syntax/walk.go        visitor helpers
internal/lsp/document_symbols.go
internal/lsp/folding.go
internal/lsp/syntax_diagnostics.go
testdata/carts/                permissively licensed corpus
testdata/tokens/               token count fixtures
```

## Data flow

```
didOpen / didChange / didSave
        │
        ▼
document store (text + version + LineIndex)
        │
        ▼
cart resolver
  ├─ find owning .p8
  ├─ read includes from overlay/disk
  ├─ flatten tabs + includes
  └─ build source map
        │
        ▼
lexer over flattened source
        │  tokens carry {file, offset}
        ▼
parser → AST + comments + parse errors
        │
        ├─ publish syntax diagnostics per file
        ├─ document symbols per file
        ├─ folding ranges per file
        └─ cache CartSnapshot for M2+
```

## Error handling

- Missing includes are diagnostics, not fatal errors.
- Unsupported `.p8.png` is a diagnostic and an empty cart snapshot.
- An included file that is open but deleted on disk uses the overlay until closed.
- If flattening fails entirely (root unreadable), publish a single error on the root and keep the previous snapshot for navigation.
- Parser errors never prevent document symbols/folding where a partial AST exists.
- Limit file reads and parsed bytes; log and diagnose oversized inputs.

## Testing

### Unit tests

- `.p8` section parsing: empty cart, header only, code only, all sections, unknown sections, no trailing newline, CRLF.
- Tab boundaries and synthesis of tab names.
- Include flattening: simple, nested (warning), missing, duplicate, quoted path, Windows separators, file with no trailing newline, include inside a long string/comment (must not be treated as a directive).
- Source map: every token span maps to the correct file/offset; include directive spans.
- Token counting: one test per rule above plus 20 real-cart fixtures.
- Lexer: one test per dialect construct, plus invalid bytes.
- Parser: expressions/precedence, statements, methods, table constructors, labels/goto, shorthand rules.
- Error recovery: truncated valid programs at every byte offset; missing `end`; stray `else`; `@x = 1`.
- Case folding: AST preserves text; semantic normalization is tested in M2.

### Corpus

- Add at least 50 permissively licensed `.p8` carts covering large code, tabs, includes, shorthand, comments, and odd syntax.
- Track: parse success, number of syntax diagnostics, panics, time.
- Goal: no panics, no hangs, and no syntax diagnostics on valid carts.

### Fuzzing

- `FuzzLexer`, `FuzzParser`, `FuzzIncludeFlatten`.
- Seeds from the corpus.
- CI runs a short fuzz pass; nightly runs longer.

### Benchmarks

- Parse and count a 64 KB cart; target < 10 ms.
- Flattening a cart with 10 includes; target < 2 ms.

## Exit criteria

- [ ] `.p8` carts and standalone `.lua` files load.
- [ ] Tabs and includes flatten correctly with source maps.
- [ ] Nested includes produce a warning and are not recursively processed.
- [ ] Missing includes produce an error diagnostic and do not break the rest of the cart.
- [ ] Lexer covers every construct in the dialect table with tests.
- [ ] Parser handles all expression/statement forms and recovers from truncated input.
- [ ] Syntax diagnostics point at the correct file and range.
- [ ] Document symbols and folding ranges work in Neovim and Helix.
- [ ] Token and character counts match PICO-8/shrinko8 for the fixture set.
- [ ] 50+ real carts parse without panic or hang.
- [ ] `go test -race ./...` passes.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| PICO-8 preprocessor quirks are weirder than documented | Build a corpus early; add targeted tests for line-boundary and `?` cases; prefer accepting a superset over false syntax errors. |
| Include flattening breaks spans | Centralize mapping in one lexer path; property-test every token span; never expose flat offsets. |
| Token counting is subtly wrong | Use an oracle (`info()`/shrinko8) and fixtures; test every counting rule. |
| Case folding surprises | Preserve original text in the AST; normalize only in semantic lookup; test uppercase carts. |
| Missing `end` recovery cascades | Implicitly close blocks at synchronization points; cap errors; test truncated inputs. |
| Corpus licensing | Only include carts with explicit permissive licenses; record attribution in `testdata/carts/README.md`. |
