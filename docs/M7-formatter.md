# M7 — AST-Aware Formatter

**Status:** Done — implemented for the v1.1.0 batch.  
**Depends on:** M6  
**Release target:** v1.1.0 (batched with M8–M10)  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

Users can format PICO-8 code without changing behavior. The formatter understands
the dialect AST, preserves line-bounded shorthand, keeps comments and LuaCATS
annotations attached to their declarations, and is idempotent on the full
corpus.

**Demo:** Open a messy cart, run `textDocument/formatting`, and get consistently
indented code with comments and annotations preserved. A shorthand
`if (btn(0)) x += 1` stays on one line. Formatting twice produces the same text.

## Scope

### In scope

- `textDocument/formatting` with full-document edits.
- Formatting Lua code in `.p8` `__lua__` sections and tabs while preserving all
  non-code sections byte-for-byte.
- Formatting standalone and included `.lua` documents.
- Standard `FormattingOptions`: `tabSize`, `insertSpaces`,
  `trimTrailingWhitespace`, `insertFinalNewline`.
- `pico8.format.maxBlankLines` (default `2`) to cap consecutive blank lines.
- Indentation for functions, blocks, tables, continuations, and shorthand
  constructs.
- Comments, `---@` annotations, long strings, `#include` directives, and tab
  markers preserved in place.
- Normalized spacing around operators, commas, assignments, and declarations.
- Refuse to format when syntax or include errors make a safe rewrite impossible.
- Preserve the document's existing line-ending style.

### Out of scope

- Range formatting and format-on-type.
- Byte-for-byte parity with `pico8-ls`.
- Line wrapping or aggressive table layout/alignment.
- Reordering statements, comments, includes, or tabs.
- Formatting read-only `.p8.png` virtual documents.
- A formatter LSP capability for clients that only support range formatting.

## Behavior

### Errors

- If the requested document has syntax errors, return an LSP `RequestFailed`
  error with a message that formatting was skipped and the errors must be fixed.
- If the cart has unreadable or missing includes, do not produce edits for the
  root cart. Format a standalone `.lua` include normally when requested
  directly.
- Never return a partial edit set that could leave a cart half-formatted.

### `.p8` carts

- Preserve section headers, `__gfx__`/`__map__`/`__sfx__`/`__music__` payloads,
  and all other non-Lua sections exactly.
- Format each code tab independently; keep `-->8` markers and tab names.
- Preserve the final line terminator behavior of each section.

### Formatting rules

- Use spaces or tabs according to `FormattingOptions`.
- Do not split line-bounded shorthand:
  - `if (cond) stmt` stays one line.
  - `while (cond) stmt` stays one line.
  - `?expr` stays one line.
  - A following `else` must not attach to a shorthand `if`.
- Keep compound assignments, peek operators, and `\` integer division intact.
- Preserve blank lines up to `maxBlankLines`; collapse longer runs.
- Keep comment-only lines and annotation blocks immediately above their target
  declarations.
- Avoid changing string contents or long-string bodies.
- Produce one statement per line unless the source uses shorthand.

## Implementation notes

- Prefer an AST-driven printer over token re-spacing. The parser must expose
  enough source trivia or attach comments/annotations to nodes so they can be
  emitted deterministically.
- Keep formatting logic inside `internal/format` (new package) so the LSP layer
  only translates documents, options, and edits.
- For `.p8` files, a formatter should format the selected code lines and splice
  them into the original text; do not re-emit non-code sections from a lossy
  model.
- If the AST does not retain a construct losslessly, leave that line unchanged
  and log a debug message rather than guessing.
- Formatting must not mutate the cached cart snapshot. Verify that the token and
  comment streams are unchanged; refuse the edit if verification fails.
- If a `FormattingOptions` field is absent, use stable defaults (`tabSize=2`,
  `insertSpaces=true`, trim trailing whitespace, insert final newline).

## Testing

- Golden formatting tests for every dialect construct and shorthand form.
- Idempotence: format twice and compare.
- Semantic regression: parse before and after, compare AST shape and symbol
  spans modulo whitespace; comments must remain present.
- Section-preservation tests for every `.p8` section and tab marker.
- Comment/annotation attachment tests, including blank lines between an
  annotation and its declaration.
- Line-ending tests: LF, CRLF, lone CR, and files without a final newline.
- Corpus test: every `testdata/carts/**/*.p8` formats without syntax changes and
  is idempotent.
- Fuzz formatting: arbitrary parsed programs must either refuse or produce
  reparsable, semantically equivalent output.
- LSP e2e: formatting request, error on syntax errors, options round-trip, and
  no edits for read-only documents.
- Performance: formatting a 64 KB cart under 25 ms on a typical developer
  machine, plus a lexical verification pass.

## Exit criteria

- [x] `textDocument/formatting` is advertised and implemented for `.p8` and
      `.lua` documents.
- [x] Shorthand constructs never change line boundaries.
- [x] Comments, annotations, includes, tab markers, and non-code sections are
      preserved.
- [x] Formatting is idempotent across the corpus.
- [x] Formatting refuses unsafe documents with a clear error.
- [x] Tests cover LF/CRLF/CR and missing final newlines.
- [x] Performance and fuzz targets pass.
- [x] Configuration and README documentation are updated.
- [x] No v1 behavior or performance budget regressions.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Formatter changes shorthand semantics | AST-only printing, corpus semantic comparison, explicit line-boundary tests. |
| Comments/annotations detach from declarations | Preserve comment trivia and add attachment goldens. |
| `.p8` section data is corrupted | Splice formatted code into original text; never regenerate non-code sections. |
| Formatter output does not parse | Reparse before returning edits; refuse on mismatch. |
| Style churn floods reviews | Format is user-invoked only; no format-on-save default. |
