# M3 — Language Intelligence & Built-in API

**Status:** In progress — implementation and automated checks complete; manual editor verification pending
**Depends on:** M2  
**Blocks:** M4  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

The server feels like a real language server: completion, hover, signature help, semantic tokens, and snippets for both user code and the complete PICO-8 0.2.7 API. This is the milestone where the server becomes useful for everyday editing even before type inference lands.

**Demo:** Type `circ` and see `circ`, `circfill`; hover `circfill` for the signature and docs; type `circfill(` and get signature help with the active parameter; type `player.` and get fields/methods from known table shapes; semantic tokens distinguish built-ins, locals, fields, strings, comments, and numbers in Neovim.

## Scope

### In scope

- Completion with member access, scope awareness, snippets, and resolve.
- Hover with signatures, docs, and declaration context.
- Signature help with active-parameter tracking.
- Semantic tokens (full).
- Completion inside LuaCATS annotations (shallow name scanning; full type semantics arrive in M4) and `#include` paths.
- Complete built-in API documentation and metadata.
- Editor snippets for common PICO-8 idioms.
- Performance and payload-size discipline.

### Out of scope

- Type inference and type-aware member completion (M4).
- Type diagnostics (M4).
- Inlay hints (M4).
- Token/char code lens and limit diagnostics (M5).
- Formatter (M7).
- Pull diagnostics (M7).

## Deliverables

1. Completion provider with `resolveProvider: true`.
2. Hover provider.
3. Signature help provider.
4. Semantic tokens provider with a stable legend.
5. `#include` path completion.
6. `---@` annotation completion scaffolding.
7. Built-in API stub with full docs/signatures/optional parameters.
8. Snippet set for PICO-8 idioms and annotations.
9. Golden tests for all text responses.

## Completion

### Trigger characters

Advertise:

```json
{
  "triggerCharacters": [".", ":", "@", "#"],
  "resolveProvider": true
}
```

Completion must also work on manual invocation (`<C-Space>` / editor equivalent) at any identifier boundary.

### Context detection

Before listing items, determine the completion context from the document text and AST:

1. **Member access** — cursor after `.` or `:`. Find the receiver expression and its symbol/table shape.
   - `.` offers fields and methods.
   - `:` offers methods only (methods that declare an implicit `self`).
   - If the receiver is unknown/`any`, return an empty list rather than every symbol in the program.
2. **Identifier prefix** — cursor in the middle of an identifier. Offer locals in scope, then globals, then built-ins, then keywords/snippets.
3. **`---@` annotation** — cursor after `---@` or inside an annotation type position. Offer annotation names (`param`, `return`, `class`, `field`, `type`, `alias`, `enum`, `meta`, `deprecated`, `cast`, `diagnostic`) and, after `@class`/`@type`/`@param`/`@return`, known class/alias names. Types beyond basic names arrive in M4; M3 can offer names already declared.
4. **`#include` path** — cursor after `#include ` or inside its path. Offer relative `.lua` files from the cart directory. Do not offer directories unless entering one.
5. **String/keyword context** — no member completion; keywords may still be offered at statement boundaries.
6. **Comment context** — only annotation completion if the comment starts with `---@`; otherwise empty.

### Item construction

Each `CompletionItem` should include:

- `label`: display name.
- `kind`: `Function`, `Method`, `Variable`, `Field`, `Class`, `Enum`, `Keyword`, `Snippet`, `Module`, `Constant`.
- `detail`: signature or short type info.
- `documentation`: filled on resolve for user/built-in symbols.
- `insertText` / `textEdit`: correct replacement range, including the prefix.
- `insertTextFormat`: plain or snippet.
- `sortText`: stable grouping so locals sort before globals before built-ins before keywords.
- `filterText`: original name.
- `tags`: `Deprecated` when applicable.
- `data`: enough to resolve the item later (symbol ID/URI, or built-in name).

Do not send full Markdown docs for hundreds of items. Use `completionItem/resolve` to attach docs and full signatures.

### Member completion without types (M3)

M3 has no full type inference, so member completion uses:

- table literal fields from direct construction;
- fields assigned with `t.x = ...` in the cart;
- methods declared with `function t:m()`;
- `self` fields inside methods when the receiver is statically known;
- class fields from `---@class` annotations only for completion names (M4 adds type checking).

If the receiver cannot be resolved, return empty. This is intentionally conservative.

### Snippets

Provide snippets for:

- `if ... then ... end`
- `if ... then ... else ... end`
- `for i=1,n do ... end`
- `for x in all(t) do ... end`
- `while ... do ... end`
- `function name(...) ... end`
- anonymous function
- `_init`, `_update`, `_update60`, `_draw` callback skeletons
- `---@class` block
- `---@param`/`---@return` function doc block
- `#include` line
- PICO-8 button glyphs and common constants if they are useful as text snippets

Use `InsertTextFormat.Snippet` and keep placeholders simple. Snippets must not be offered inside strings/comments except annotation snippets in `---@` context.

### Completion resolve

On resolve:

- Attach Markdown documentation from the built-in stub or user doc comments.
- Attach the full signature.
- Attach the declaration location as `detail` or a `see also` link where appropriate.
- Never mutate the original item in a way that changes its insertion text.

## Hover

Hover returns Markdown with:

1. A fenced code block containing the symbol signature or declaration:
   - `function move(p, dx, dy)`
   - `local player`
   - `field x` / `Player.x`
   - `_update()`
2. Type information when available (M4 adds inferred types; M3 shows annotations if present).
3. Doc comment Markdown from `---` comments or the built-in stub.
4. A source line such as `PICO-8 0.2.7 API` for built-ins, or the relative file/tab for user symbols.
5. For `#include`, the resolved relative path and a note that includes are flattened and non-recursive.
6. For `?`, an explanation of the print shorthand.
7. For `@`/`%`/`$` when hovered as operators, a short peek/peek2/peek4 explanation.

Rules:

- Hover on whitespace returns empty.
- Hover on a built-in returns docs even though go-to-definition is empty.
- Hover on a field with unknown receiver returns empty.
- Keep payloads under a reasonable size; truncate long docs with a link to the manual if needed.
- Use Markdown; do not return plain text with broken formatting.

## Signature help

- Trigger on `(`; retrigger on `,`.
- Find the innermost call at the cursor, respecting nesting and strings/comments.
- Determine the active parameter by counting top-level commas within the call, ignoring commas inside nested calls, tables, parentheses, brackets, and strings.
- Return:
  - `signatures`: one for built-ins (optionals marked), one for user functions unless overloads exist later.
  - `activeSignature`: 0 in M3.
  - `activeParameter`: computed index, clamped to the last parameter.
  - Parameter `label`s should be substrings of the signature label for client compatibility.
  - Parameter documentation from annotations/built-in docs.
- Missing/extra arguments are not errors; signature help merely highlights the current parameter.
- For methods, label includes `self` only if the user-facing signature should; prefer showing the declaration signature without implicit self in M3, and mark it clearly in docs.
- Signature help inside a malformed call should use the best partial parse available.

## Semantic tokens

Advertise `textDocument/semanticTokens/full`. Full tokens only in v1; deltas are post-v1.

### Legend

Token types (stable order):

```
namespace, type, class, enum, interface, struct, typeParameter,
parameter, variable, property, enumMember, event, function, method,
macro, keyword, modifier, comment, string, number, regexp, operator
```

Token modifiers:

```
declaration, definition, readonly, static, deprecated, defaultLibrary,
documentation
```

### Mapping

- Keywords → `keyword`.
- Identifiers that resolve to locals → `variable`.
- Parameters and `self` → `parameter`.
- Globals/functions → `function` or `variable`.
- Fields/methods → `property`/`method`.
- Class/alias names from annotations → `class`/`type`.
- Labels → `namespace` or `variable` (choose one and test).
- Built-ins → their kind + `defaultLibrary`.
- Deprecated symbols → `deprecated`.
- Comments → `comment` (including `---@` annotations; do not split annotations in v1).
- Strings, numbers → `string`, `number`.
- Operators/punctuation → `operator` where useful.
- Include paths → `string` with `documentation` modifier.

Rules:

- Tokens must be sorted by line/character and must not overlap.
- Do not emit zero-length tokens.
- Use UTF-16 lengths to match the negotiated position encoding.
- If UTF-8 encoding is negotiated, lengths are UTF-8 bytes; centralize.
- Whole-document requests only; return all tokens, not a range subset, in M3.
- Cap token count if a pathological file appears; log and truncate deterministically.

## Built-in API metadata

The M2 stub is the source of truth. M3 makes it complete and user-facing.

For every documented function:

```lua
---Draw a filled circle.
---
---@param x number
---@param y number
---@param r number
---@param col? integer
function circfill(x, y, r, col) end
```

Requirements:

- Signatures use PICO-8's `[optional]` semantics via `?`/`---@param name?`.
- Every function has a one-line summary and parameter docs where useful.
- Constants have docs and types (numbers; aliases may come in M4).
- Lifecycle callbacks are documented as optional globals with no parameters.
- Deprecated API names, if any, use `---@deprecated`.
- No Lua standard library names.
- Keep documentation concise; link to the official manual rather than duplicating it verbatim.
- Include a `---@meta` marker.
- API coverage test from M2 remains green.

## Include path completion

- Detect `#include ` at the start of a line, outside comments/strings.
- List `.lua` files relative to the cart root, excluding hidden directories and the cart's own file.
- Insert paths with forward slashes for portability; preserve a typed `./` prefix if present.
- Completion replacement range covers the path text after `#include `.
- Do not offer files outside the workspace root unless the user typed `../` and the server's file access policy allows it. v1 allows navigating up within the workspace; document the behavior.
- Missing-target completion is not an error; M5 adds the “create file” code action.

## Annotation completion

M3 offers names only, using a shallow scan of `---@class` and `---@alias` declarations. M4 replaces this with the full annotation parser and adds type-aware completion.

- `---@` → annotation keywords.
- `---@class ` → known class/alias names plus free text.
- `---@type `, `---@param name `, `---@return ` → known class/alias/primitive names.
- `---@field ` → field names are free text; types offer known names.
- Do not offer annotation completion outside a `---` comment.

## Architecture and files

```
internal/lsp/completion.go        context detection, items, resolve
internal/lsp/hover.go             symbol/builtin/include hover
internal/lsp/signature.go         call detection, active parameter
internal/lsp/semantic_tokens.go   token legend and emission
internal/lsp/include_completion.go
internal/analysis/docs.go         doc comment extraction and Markdown
internal/analysis/members.go      table/method member discovery
internal/builtin/pico8-0.2.7.lua  complete API stub
internal/builtin/docs.go          lookup helpers
testdata/lsp/completion/          golden completion fixtures
testdata/lsp/hover/               golden hover fixtures
testdata/lsp/signature/           golden signature fixtures
testdata/lsp/semantic/            golden semantic token fixtures
```

## Data flow

```
request position
      │
      ▼
document + LineIndex → byte offset
      │
      ▼
CartSnapshot (AST + symbols + builtins)
      │
      ├─ completion: context → candidate symbols → items → resolve docs
      ├─ hover: node/symbol at offset → Markdown
      ├─ signature: call node + offset → active parameter
      └─ semantic tokens: full AST/symbol walk → encoded token list
```

## Error handling

- Invalid position → empty result, not an error.
- Unknown document → empty result for completion/hover/signature; semantic tokens return empty.
- Malformed/partial AST → best-effort results.
- Missing built-in stub → log fatal once; completion/hover for user symbols still works.
- Completion item resolve with stale data → return the item unchanged.
- Cap completion candidates (for example, 1000) and log truncation.
- Never block on file I/O during completion; include path completion may read the directory synchronously but must be cheap and bounded.

## Testing

### Completion

- Empty-prefix completion at statement start.
- Locals before globals before built-ins.
- Member completion after `.` and `:` with table literals/methods.
- No member completion for unknown receivers.
- Built-in completion (`circ` → `circ`, `circfill`).
- Snippet insertion and replacement ranges.
- `---@` annotation completion.
- `#include` path completion, including subdirectories.
- Resolve attaches docs without changing insert text.

### Hover

- Local, global, function, method, field, built-in, include, `?`, peek operators.
- Markdown escaping for docs containing backticks/brackets.
- Long docs truncation.
- Hover on comments/strings returns empty.

### Signature help

- Simple built-in call, nested call, commas in strings/tables/parens.
- Optional parameter labels.
- Active parameter clamped when cursor is past the last argument.
- Method call.
- Incomplete call and parse-error call.

### Semantic tokens

- Snapshot encode/decode for a representative cart.
- No overlapping tokens; sorted order.
- UTF-16 vs UTF-8 lengths.
- Built-ins marked `defaultLibrary`.
- Comments/strings/numbers/keywords/operators.
- Multibyte glyphs.

### Performance

- Completion < 5 ms on a 64 KB cart after cache.
- Hover < 2 ms.
- Semantic tokens for a 64 KB cart < 10 ms.
- No full workspace scan on completion.

### Manual

- Neovim with `vim.lsp.completion` or `nvim-cmp`, hover, and signature help.
- Helix completion/hover/signature.
- Compare built-in docs against the official manual for a sample of 20 functions.

## Implementation notes

- `internal/builtin/pico8-0.2.7.lua` remains the single source of truth. Its
  `---` summaries, `---@param name? type` annotations, and `---@type number`
  constants are parsed into completion/signature/hover metadata.
- Parameter types are intentionally shallow until M4. Polymorphic values use
  `any`; table, function, string, boolean, thread, and numeric positions use
  their PICO-8-facing type names.
- Member completion uses direct table construction, field assignments,
  methods, `self`, and shallow `---@class`/`---@type` fields. There is no
  inference beyond those names.
- Include path completion stays inside the cart directory when no workspace
  folder is configured. With a workspace folder, typed `../` paths are allowed
  as long as the resolved directory remains inside that folder.
- Completion is capped at 1000 items; semantic tokens at 20000 tokens. Both
  truncations are logged and deterministic.
- Semantic token requests are full-document only. The stable legend is defined
  in `internal/lsp/semantic_tokens.go` and checked by a test.

## Exit criteria

- [x] Completion works for locals, globals, fields/methods, built-ins, snippets, annotations, and include paths.
- [x] Completion resolve attaches docs without bloating initial responses.
- [x] Hover covers user symbols, built-ins, includes, shorthand, and peek operators.
- [x] Signature help computes the active parameter correctly in nested code.
- [x] Semantic tokens are stable, non-overlapping, and correct for UTF-16.
- [x] The 0.2.7 API stub is complete and coverage-tested.
- [x] No standard Lua library names appear in completion or built-ins.
- [x] Performance targets are met by `TestPerformanceTargets` after cache warm-up.
- [ ] Neovim and Helix manual tests pass. (Not runnable in this environment; run before marking M3 done.)

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Completion is noisy or shows everything for unknown receivers | Conservative member completion; scope-aware sorting; empty result when resolution is unknown. |
| Built-in docs drift from the manual | Manifest/coverage tests; review samples each release; keep a single stub source of truth. |
| Signature active-parameter bugs | Reuse one tokenizer/parser-aware scan; test nested strings/tables/parens. |
| Semantic tokens overlap comments/strings | Emit from AST/symbol spans with a single pass and assert no overlaps in tests. |
| Response payloads are huge | Resolve docs lazily; cap candidates; send semantic tokens full but compact. |
| Position encoding bugs | Use the M0 conversion layer; test UTF-16 and UTF-8 explicitly. |
