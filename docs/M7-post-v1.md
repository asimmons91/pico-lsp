# M7 — Post-v1

**Status:** Planned  
**Depends on:** M6  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

Post-v1 work expands PICO-8 parity, editor reach, and type-system depth without compromising the v1 contract. This milestone is intentionally a prioritized backlog, not a commitment to ship every item together.

**Guiding rule:** v1 features must remain stable. M7 additions are opt-in or isolated enough that existing users see no regressions.

## Prioritization

| Priority | Item | Value | Risk |
|---|---|---|---|
| P1 | AST-aware formatter | Parity with `pico8-ls`; everyday utility | High: line-bounded shorthand can change semantics |
| P1 | `.p8.png` read-only support | Open shared carts directly | Medium: binary decoding and compressed code formats |
| P1 | Advanced type features | Closer to `lua_ls`; better OOP support | High: inference complexity and false positives |
| P2 | VS Code client | Reach mainstream editor users | Low: thin client + binary packaging |
| P2 | Zed extension | Modern editor support | Medium: extension API and binary distribution |
| P2 | Pull diagnostics | Better LSP 3.17+ integration | Low: protocol work, duplicate-publish avoidance |
| P3 | Refactors and code actions | Productivity | Medium: safety and scope |
| P3 | Compressed-size budget | Complete `.p8.png` limits | Medium: exact compressor parity |

## 1. AST-aware formatter

### Goal

Format PICO-8 code without changing semantics, including line-bounded shorthand.

### Requirements

- `textDocument/formatting` with full-document edits.
- Refuse to format when syntax errors exist; return a clear error or no edits.
- Preserve comments and annotations, including `---@` blocks.
- Format indentation for functions, blocks, tables, and continuations.
- Respect PICO-8 shorthand:
  - Never split `if (cond) stmt` across lines.
  - Never attach a next-line `else` to a shorthand `if`.
  - Keep `?expr` on one line.
  - Keep compound assignments intact and valid.
- Handle tabs vs spaces via `FormattingOptions` (`insertSpaces`, `tabSize`).
- Preserve intentional blank lines up to a configurable maximum.
- Do not reorder code or comments.
- Optionally offer a “one statement per line” style, matching `pico8-ls`’s formatter options.
- Add `pico8.format.*` settings only if needed; prefer standard LSP formatting options.

### Testing

- Golden format tests for every dialect construct.
- Idempotence: formatting twice produces the same output.
- Semantic regression: reparse formatted output and compare AST shape (ignoring whitespace/comments).
- Line-boundary tests from the M1 corpus.
- Comment/annotation preservation tests.

### Risks

- A formatter that breaks shorthand is worse than no formatter. Ship only after exhaustive corpus tests and a feature flag if necessary.

## 2. `.p8.png` read-only support

### Goal

Open `.p8.png` carts directly for navigation, diagnostics, and types without requiring an export step.

### Requirements

- Decode the standard 160×205 PNG cart payload.
- Support uncompressed code and both compressed code formats:
  - new `\0pxa` format;
  - old `:c:\0` format.
- Map P8SCII bytes to Unicode where needed.
- Expose the code as a read-only virtual document or as a source-mapped overlay.
- Do not write `.p8.png` in v1 of this feature.
- Report a clear diagnostic for unsupported/corrupt carts.
- Cache decoded code by file mtime/hash; invalidate on watched-file changes.
- Document that editing requires saving/exporting as `.p8` first.

### Testing

- Fixtures for uncompressed, new-compressed, and old-compressed carts.
- Round-trip decode against known carts.
- Corrupt/truncated PNG handling.
- Token counts match the equivalent `.p8`.
- Source mapping positions for diagnostics/navigation.
- Performance on large carts.

### Risks

- Binary decoding is easy to get subtly wrong; use an existing MIT implementation (for example, `shrinko8` or a PICO-8 PNG parser) as a reference and cross-check with real carts.
- Do not couple this to the core cart model; keep it an input adapter that produces the same `Cart` interface.

## 3. Advanced type features

### Goal

Move from pragmatic gradual typing toward `lua_ls`-class expressiveness while keeping noise low.

### Candidates

1. **Generics**
   - `---@generic T`
   - Generic functions and classes (`Box<T>`).
   - Instantiation at call sites.
   - Preserve `any` fallback for uninferable generics.
2. **Overloads**
   - `---@overload fun(...): ...`
   - Signature help cycles/selects overloads.
   - Call checking chooses the best match; ambiguity falls back to no error.
3. **Class inheritance**
   - `---@class Child: Parent`
   - Field/method lookup through parents.
   - Structural compatibility with parents.
4. **Exact classes**
   - `---@class (exact)`.
   - Missing/extra field diagnostics in `basic` if the class is exact; document.
5. **Metatable inference**
   - `setmetatable({}, {__index = Class})` treats the result as an instance of `Class`.
   - `__index` functions and common OOP factories.
   - `__call`, `__add`, `__tostring` could inform operator/typing later.
6. **Operators**
   - `---@operator add(Other): Result`, etc.
   - Use for operator type checking and hover.
7. **Better narrowing**
   - `assert(x)`, `x = x or default`, early return patterns, `and`/`or` chains.
   - Discriminated unions based on a tag field (`kind == "player"`) if practical.
   - `type(x)` and `---@cast` composition.
8. **Type guards / user-defined narrowing**
   - Optional custom annotations if the community wants them; avoid inventing syntax prematurely.
9. **API versioning**
   - `pico8.apiVersion` accepts supported versions with different stub sets.
10. **Pull diagnostics**
    - `textDocument/diagnostic` and `workspace/diagnostic`.
    - Related documents for cart-wide changes.
    - Avoid double-publishing with push diagnostics; make push/pull configurable or choose pull when the client supports it.

### Requirements

- Every new type feature has:
  - parser/annotation support;
  - inference rules;
  - hover/completion/signature integration;
  - diagnostics with a mode/config;
  - corpus validation for false positives;
  - golden tests.
- New diagnostics default to off or hint until proven low-noise.
- Keep type checking warnings, not errors, unless users explicitly opt into strict errors.

### Risks

- Complexity grows quickly. Add features one at a time behind flags or config, and validate against the corpus and real annotated carts.
- Avoid type-system research projects that do not improve PICO-8 ergonomics.

## 4. VS Code client

### Goal

Make installation easy for VS Code users.

### Requirements

- Thin TypeScript client extension using `vscode-languageclient`.
- Bundle or download the `pico-lsp` binary for the platform.
- Contribute:
  - `.p8` language ID and syntax highlighting;
  - commands for cart info;
  - configuration schema mirroring `pico8.*`;
  - status bar item for tokens/chars (using `pico8/cartInfo`).
- Publish to the VS Code Marketplace and Open VSX.
- Keep the server editor-agnostic; no server code should depend on VS Code.
- Provide a fallback for users who manage the binary themselves (`pico8.serverPath`).

### Testing

- Extension activation, server start, config round-trip.
- Binary download/bundling on all target platforms.
- Smoke test: open cart, hover, definition, completion, cart info.

## 5. Zed extension

### Goal

Provide PICO-8 support in Zed.

### Requirements

- Zed extension that downloads or bundles the `pico-lsp` binary.
- Language configuration for `.p8` (and included `.lua` if Zed supports it).
- Optional tree-sitter grammar for syntax highlighting; semantic tokens may cover code.
- Document installation and config.
- Keep the extension thin; all logic stays in the server.

### Testing

- Extension install and server launch on macOS/Linux.
- Basic LSP features in a real cart.
- Binary architecture detection.

## 6. Pull diagnostics

### Goal

Use LSP 3.17 pull diagnostics where supported, reducing stale push diagnostics and improving cart-wide updates.

### Requirements

- `textDocument/diagnostic` returns full reports for the requested document.
- `relatedDocuments` includes other files whose diagnostics changed due to cart analysis.
- `workspace/diagnostic` optionally reports all open carts (cancellable, bounded).
- Configuration to choose push vs pull; default to pull when the client supports it.
- Avoid duplicate publishes.
- Preserve `pico8.maxProblems` behavior.

## 7. Refactors and code actions

Candidates:

- Extract selection to a local function.
- Convert a global to a local when safe.
- Generate `---@class` from a table literal.
- Add missing fields to an exact class annotation.
- Convert a standard `if ... then ... end` to shorthand and vice versa (with safety checks).
- Organize includes? PICO-8 includes are order-sensitive; do not reorder automatically.
- Remove unused locals/functions with confirmation.

All refactors must be conservative and previewable via `WorkspaceEdit`.

## 8. Compressed-size budget

### Goal

Report the `.p8.png` compressed code size and warn near the limit.

### Requirements

- Implement or integrate a PICO-8-compatible compressor.
- Add compressed size to `pico8/cartInfo`.
- Warn/error at the documented compressed-size limit.
- Validate against PICO-8’s own output for fixtures.
- Keep it informational until exact parity is proven.

## Non-goals for M7

- Executing or emulating PICO-8 carts.
- Sprite/map/audio editing previews.
- A full Lua 5.4 standard library.
- Nested include support (PICO-8 remains non-recursive).
- Luau-style inline type syntax unless the community explicitly requests a fork mode.
- Replacing `pico8-ls`’s formatter byte-for-byte; behavioral parity is enough.

## Cross-cutting requirements

- No regressions in v1 behavior or performance budgets.
- Every M7 feature is opt-in or additive.
- Update the roadmap, README, configuration docs, and diagnostic reference.
- Add a migration note for any config or behavior change.
- Keep the editor-agnostic server contract.
- Maintain the same testing standards: unit, golden, LSP e2e, corpus, fuzz where applicable.

## Exit criteria

There is no single M7 exit; each item ships independently when its own acceptance criteria are met. The milestone is “done” when the prioritized items selected for the next release are complete and documented.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Formatter changes semantics | AST-aware formatting, reparse comparison, corpus tests, feature flag. |
| `.p8.png` decoding bugs | Use proven reference, fixtures for all formats, read-only scope. |
| Advanced types create noise | One feature at a time, default off/hint, corpus validation. |
| VS Code/Zed packaging drift | Thin clients, shared binary release assets, smoke tests. |
| Pull diagnostics duplicate pushes | Config switch, clear ownership, e2e tests. |
| Scope creep | Prioritized backlog; no M7 item blocks a v1 patch release. |
