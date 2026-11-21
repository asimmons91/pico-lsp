# M23 — Token-Saving Quick Fixes

**Status:** Proposed — specified, not committed to a release.  
**Depends on:** M14 (code actions and the M1/M5 token rules)  
**Release target:** Post-v1.8 (TBD)  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

PICO-8 developers can replace common verbose idioms with shorter PICO-8
equivalents from the editor lightbulb/code-action menu. Every new action is
conservative, previewable as a standard `WorkspaceEdit`, and offered only when
the rewrite is behavior-preserving and reduces the counted token total. The
existing M14 shorthand actions remain available even where their token count is
equal to the standard form.

**Demo:** With the cursor on `print(score)`, the editor offers
`Use ? print shorthand (saves 1 token)`. On `score = score + 1`, it offers
`Use compound assignment += (saves 2 tokens)`. On `peek(0x5f00)`, it offers
`Use @ peek shorthand (saves 1 token)`. Applying any action previews a normal
edit in the original `.p8`/`.lua` file. No action is offered for an ambiguous
or unsafe case.

The existing M14 shorthand `if`/`while` conversion remains the baseline for
line-bounded statement shorthand. This milestone adds the remaining common
token-saving patterns and treats shorthand conversion as part of the same
family without changing its semantics.

## Scope

### In scope

- `print(...)` statement to PICO-8 `?` print shorthand.
- Self-assignment to compound assignment for every compound operator the M1
  parser accepts.
- `peek`, `peek2`, and `peek4` calls to `@`, `%`, and `$` peek operators.
- Cursor/selection-local code actions under a stable token-saving kind family.
- Exact token-savings computation for action titles and eligibility.
- Safety analysis using the existing AST, symbol table, source maps, and
  token counter.
- Tests, documentation, and code-action reference updates.

### Out of scope

- Cart-wide “fix all token-saving patterns” edits.
- Automatically applying edits on save or formatting.
- Changing token/char/compressed counting semantics.
- Changing the existing `token-limit`, `char-limit`, or `compressed-size-limit`
  diagnostics or code lens.
- Whitespace/comment minification or character-count-only formatting.
- `~=` ↔ `!=` style swaps (no token savings).
- Removing `local`/`end` (they cost zero tokens in PICO-8).
- `flr(a / b)` → `a \ b` until integer-division semantics match PICO-8 for
  negative and non-integer operands. Track as a deferred pattern.
- Redundant-parenthesis removal until precedence and multi-value safety rules
  are specified separately. Track as a deferred pattern.
- `else if` → `elseif` restructuring. Track as a deferred pattern.
- Editor-specific UI beyond standard code actions and `WorkspaceEdit`.

## Decisions proposed

| ID | Decision | Choice |
|---|---|---|
| D1 | Milestone identity | M23, **Proposed**, post-v1.8. |
| D2 | Code-action kind | `refactor.rewrite.token-saving.*`, with subkinds for `print`, `compound`, and `peek`. Existing shorthand keeps `refactor.rewrite.shorthand`. |
| D3 | Trigger model | Only cursor/selection-local candidates. No cart-wide fix-all in the first version. |
| D4 | Configuration | Reuse `pico8.refactors.enabled`; do not add a new setting initially. |
| D5 | Diagnostic coupling | None. Actions are available regardless of budget diagnostics; exceeding a budget is not required. |
| D6 | Token savings | For the new patterns, compute from the PICO-8 token rules in `internal/cart`; show the exact positive delta in the title and omit the action when the delta is not positive. |
| D7 | Preferred flag | Leave `isPreferred` unset/false. Token-saving rewrites are stylistic, so editors must not surprise users by auto-preferring them. |
| D8 | Shorthand | Reuse M14's safe `if`/`while` conversion unchanged. Do not broaden eligibility in this milestone. |
| D9 | Bulk behavior | No `source.fixAll` or command in the first version. Defer to a separate decision after local actions ship. |
| D10 | Read-only/stale inputs | Offer no action for virtual `.p8.png` documents, `---@meta` files, or a snapshot that no longer matches open documents. |

## Pattern catalog

### P1 — Print shorthand

| Item | Requirement |
|---|---|
| Before | `print(expr)` or `print(a, b, ...)` as a statement |
| After | `?expr` or `?a, b, ...` |
| Savings | Computed exactly; typically one for a simple call (`print(x)` 3 → `?x` 2) |
| Availability | `print` resolves to the PICO-8 built-in, not a user-defined or shadowed value |

Eligibility:

- The call is a complete statement on one physical line, or the final statement
  of a same-line shorthand `if`/`while` body. In the latter case only the
  `print(...)` call is replaced; the preceding shorthand stays unchanged.
- The call has one or more arguments. Zero-argument conversion is allowed only
  if a PICO-8 parity fixture confirms identical output and token accounting.
- The call is not used as a value: not in `return`, an assignment expression, a
  table constructor, a nested call argument, or a method call.
- Nothing follows the call on the same line except a trailing `--`/`//`
  comment. In particular, another statement, another call, `else`, or `elseif`
  after the call makes the rewrite unsafe because `?` consumes the rest of the
  line. A trailing comment is preserved only if the parser and a PICO-8 parity
  fixture confirm `?` does not consume the comment as an argument; otherwise
  refuse.
- The call, its arguments, and any trailing comment contain no line break.
- The call contains no comments between `print` and its closing parenthesis.
- Multi-value argument behavior is preserved. In particular, a final function
  call argument (for example `print(f())`) must expand to the same arguments
  under `?`; add a parity fixture before offering that shape.
- `print` is not reassigned anywhere in the cart. Use the resolved symbol, not
  source text.

Titles:

- `Use ? print shorthand (saves N tokens)`

### P2 — Compound assignment

| Item | Requirement |
|---|---|
| Before | `target = target op value` |
| After | `target op= value` |
| Savings | Computed from the tokens in the target and operator; at least two for a simple name |
| Availability | The assignment and value are syntactically and semantically safe |

Operator map:

| Binary operator | Compound operator |
|---|---|
| `+` | `+=` |
| `-` | `-=` |
| `*` | `*=` |
| `/` | `/=` |
| `\` | `\=` |
| `%` | `%=` |
| `^` | `^=` |
| `..` | `..=` |
| `&` | `&=` |
| `\|` | `\|=` |
| `^^` | `^^=` |
| `<<` | `<<=` |
| `>>` | `>>=` |
| `>>>` | `>>>=` |
| `<<>` | `<<>=` |
| `>><` | `>><=` |

Eligibility:

- The assignment has exactly one target and one value.
- The assignment operator is plain `=`, not an existing compound assignment.
- The value is a binary expression whose operator appears in the map.
- The value's left operand is structurally the same lvalue as the target and
  resolves to the same symbol/field chain.
- The target is one of:
  - a simple name;
  - a chain of static field names such as `t.x` or `self.stats.hp`.
- Static field targets are refused when the receiver is dynamic, unresolved, or
  ambiguous, or when the field symbol cannot be resolved statically.
- Bracket indexing is refused in the first version, including literal indices,
  to avoid duplicated evaluation and side-effect questions.
- No comments exist between the assignment target, `=`, the repeated target, the
  operator, or the value.
- The replacement preserves the value's source text and parentheses exactly.
  Do not add parentheses or reformat the value.
- Multiple assignment, multiple return values, method-call targets, peek
  targets, and assignments with unresolved/dynamic receivers are refused.
- `target = value op target` is refused even for operators that happen to be
  commutative, because evaluation order is not worth guessing.

Titles:

- `Use compound assignment += (saves N tokens)`
- Use the actual compound operator in the title.

### P3 — Peek operators

| Item | Requirement |
|---|---|
| Before | `peek(addr)`, `peek2(addr)`, `peek4(addr)` |
| After | `@addr`, `%addr`, `$addr` |
| Savings | Computed exactly; typically one for a simple atom (`peek(a)` 3 → `@a` 2) |
| Availability | The function resolves to the PICO-8 built-in, takes exactly one argument, and the rewrite has a positive token delta |

Eligibility:

- The call has exactly one argument and no comments or line breaks.
- The called name resolves to the built-in `peek`, `peek2`, or `peek4`; a local,
  global, parameter, or field with the same spelling suppresses the action.
- The call is part of a value-producing expression. A bare `peek(x)` statement
  is refused because `@x` is not a valid Lua expression statement.
- Parentheses are required when the argument has lower precedence than the
  unary peek operator. If those parentheses consume the token savings, do not
  offer the action. For example, `peek(a + b)` would need `@(a + b)` and has no
  token savings, so it is refused.
- Calls used as values are allowed when the surrounding expression remains
  unchanged and the generated operator parses at the same precedence. Calls in
  assignment targets are impossible and are refused by syntax.
- The generated `%` is always in prefix position; binary modulo parsing is
  unchanged.
- No `poke`/`poke2`/`poke4` conversion exists; those have no shorthand.

Titles:

- `Use @ peek shorthand (saves N tokens)`
- `Use % peek shorthand (saves N tokens)`
- `Use $ peek shorthand (saves N tokens)`

### P4 — Existing shorthand if/while

M14 already provides:

- `Convert to PICO-8 shorthand if`
- `Convert to PICO-8 shorthand while`
- `Convert to standard if statement`
- `Convert to standard while statement`

This milestone verifies those actions remain available under
`pico8.refactors.enabled`, adds regression coverage alongside the new actions,
and may use the same token-delta helper for tests. It does not broaden the
single-statement/no-comment/line-length eligibility rules.

### Deferred patterns

| Pattern | Reason deferred |
|---|---|
| `flr(a / b)` → `a \ b` | PICO-8 `\` behavior for negative and non-integer operands must be verified against `flr`; conversion could change results. |
| Bracket/index compound targets (`t[1] = t[1] + 1`) | Index evaluation count, side effects, and table/metatable behavior need their own safety proof. |
| Redundant parentheses | Requires a complete precedence, associativity, multi-value, and table/call-context safety proof. |
| `else if` → `elseif` | Structural rewrite changes nesting and needs its own line-boundary tests. |
| Cart-wide “fix all” | Overlapping edits and unexpected bulk behavior need a separate preview/confirmation design. |
| Whitespace/comment shrinking | Character-count rather than token-count concern; formatter territory. |
| `~=` → `!=` | Same token count; style-only, not a token-saving action. |

## LSP interface

These are cursor/selection code actions, not diagnostic-bound quick fixes.
They are advertised under `refactor.rewrite` because they are reversible
rewrites, while still appearing in the editor's lightbulb/code-action menu.

Stable kinds:

- `refactor.rewrite.token-saving.print`
- `refactor.rewrite.token-saving.compound`
- `refactor.rewrite.token-saving.peek`

Requirements:

- `textDocument/codeAction` returns only candidates wholly inside the requested
  range or containing the cursor position after the existing refactor-range
  trimming.
- Each action carries a standard single-file `WorkspaceEdit` in original-file
  coordinates. Included-file edits are not combined with root-file edits.
- Titles for the new actions include the pattern and the exact positive token
  delta. Existing shorthand titles remain unchanged.
- New token-saving actions are not returned when the delta is zero or negative.
- `isPreferred` is not set.
- No command is used; applying the action is a normal previewable workspace
  edit.
- Actions are suppressed when `pico8.refactors.enabled` is false, when the
  document is a read-only virtual cart, when the file is a `---@meta` stub, or
  when the analyzed snapshot is stale relative to open documents.
- Candidate ordering is deterministic: outermost/source-order candidates first,
  then pattern ID, then title.
- If a selection contains multiple independent candidates, return one action
  per candidate and do not merge overlapping edits. The user applies one action
  at a time.

## Safety and semantics

- Rewrites must preserve runtime behavior. The AST and resolved symbols, not
  text matching alone, determine eligibility.
- `print`, `peek`, `peek2`, and `peek4` must resolve to built-in symbols.
  Reassigned or shadowed names are refused.
- PICO-8 line-bounded rules are mandatory. `?` consumes the rest of its line,
  and shorthand `if`/`while` bodies end at the line boundary.
- Never edit inside strings, comments, annotations, include paths, or tab
  markers.
- Preserve comments. If a comment falls inside the edited span and cannot be
  carried exactly, refuse the action rather than deleting it.
- Preserve source text for all operands; only the minimal operator/call syntax
  changes. Do not reformat the surrounding line.
- Never add, remove, or reorder `#include` directives.
- Never change multiple files.
- Token savings are computed with the same rules as
  `internal/cart.CountTokens`, including:
  - compound assignments count as one operator token;
  - comments, annotations, whitespace, `local`, `end`, closing brackets,
    commas, dots, and colons are not counted;
  - `?` and the peek operators follow the documented M1 counting rules.
- Generated titles must not claim a savings value that disagrees with the
  token counter. Actions optimize the token count; they make no promise about
  character or compressed size.
- No new diagnostics, no changes to existing diagnostic severities, and no
  automatic application.

## Configuration

No new setting is required for the first version. The token-saving actions are
controlled by the existing:

```json
{
  "pico8.refactors.enabled": true
}
```

If user feedback shows demand for finer control, a later additive decision can
add `pico8.tokenSaving.enabled` or per-pattern toggles without changing the LSP
kind family.

## Testing

### Unit tests

- `print` → `?`:
  - one argument, multiple arguments, nested expressions, comments;
  - zero-argument and final multi-value call-argument parity fixtures;
  - final statement of a same-line shorthand `if`/`while` body;
  - shadowed/reassigned `print`;
  - expression, return, nested-call, method-call, multi-line,
    trailing-statement, and same-line `else`/`elseif` refusals;
  - exact token delta.
- Compound assignment:
  - every operator in the map;
  - simple names, `self` fields, nested static fields;
  - bracket-index refusal, side-effectful target refusal, multiple assignment,
    mismatched left operand, comments, and complex precedence;
  - exact token delta and parenthesization.
- Peek operators:
  - `peek`, `peek2`, `peek4`;
  - low-precedence arguments requiring parentheses;
  - shadowed built-ins, wrong arity, comments, multi-line calls, and bare
    statement calls;
  - exact token delta.
- Existing shorthand if/while regression tests remain green.
- Token-delta helper tests cover comments, annotations, unary literal folding,
  and compound operators.

### LSP end-to-end tests

- `textDocument/codeAction` on each pattern returns the expected kind, title,
  token delta, and edit range.
- Applying each edit produces the expected source text.
- Included `.lua` and `.p8` tabs edit the original file, not flattened
  coordinates.
- No action is offered with `pico8.refactors.enabled: false`.
- No action is offered for virtual `.p8.png` documents or stale snapshots.
- Multiple candidates in a selection are returned without overlapping edits.
- Read-only/meta behavior is covered.
- Existing M14 code actions and rename behavior do not regress.

### Corpus and safety tests

- For a representative corpus subset, apply each offered action, reparse and
  reanalyze, and assert:
  - no syntax errors;
  - no new type or lint diagnostics;
  - token count does not increase and matches the advertised delta;
  - no edits touch include directives.
- Property tests for safe compound-assignment shapes and precedence.
- Truncated/corrupt input must return no action rather than panic.

### Performance

- A code-action request on a 64 KB cart remains within the M13/roadmap
  editor-request budget after analysis.
- Token-delta computation must not reanalyze the whole cart; tokenize only the
  affected source span or the edited file as needed.
- Returning multiple candidates from a large selection is bounded by a small
  documented limit.

## Documentation and release work

- Add the new code-action kinds and examples to [api.md](api.md).
- Document token-saving actions, eligibility, and refusals in this spec and the
  README feature list once implemented.
- Update [ROADMAP.md](ROADMAP.md) status and the post-v1 backlog when the
  milestone moves from Proposed to In progress/Done.
- Update CHANGELOG under Unreleased when implemented.
- No VS Code settings schema change unless a new setting is approved.
- Verify the VS Code, Neovim, and Helix code-action menus display the new
  actions without editor-specific work.

## Exit criteria

- [ ] `print(...)` → `?` is available for safe statement-level calls.
- [ ] Self-assignment → compound assignment is available for all supported
  operators and safe lvalues.
- [ ] `peek`/`peek2`/`peek4` → `@`/`%`/`$` is available for built-in calls.
- [ ] Existing shorthand if/while conversion remains available and unchanged.
- [ ] No action is offered when behavior, comments, line boundaries, source
  maps, read-only status, or token savings are unsafe.
- [ ] New action titles report exact token deltas computed with the PICO-8
  token rules.
- [ ] Unit, LSP e2e, corpus, safety, and performance tests pass.
- [ ] API/docs/roadmap/changelog updates are complete.
- [ ] Existing v1 through v1.8 behavior and performance budgets do not regress.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| `?` consumes the rest of the line and changes a following statement | Require the print call to be a complete one-statement line; preserve comments only after parity testing. |
| Built-in `print`/`peek` names are shadowed | Resolve symbols and require the PICO-8 built-in; refuse reassigned names. |
| Compound assignment changes evaluation order | Restrict to simple/dot lvalues and exact `target = target op value`; refuse indexing and side effects. |
| Peek operator precedence changes `a + b` | Parenthesize low-precedence arguments using AST precedence. |
| Token delta is wrong | Use the shared token counter; test every operator and refusal; omit non-positive actions. |
| Noisy editor menus | Cursor/selection-local actions only, no `isPreferred`, no bulk fix-all. |
| Source maps or includes drift | Use original coordinates, refuse cross-file edits, and test included files. |
| Read-only or stale buffers receive edits | Suppress actions for virtual/meta documents and stale snapshots. |
| Shorthand line-boundary regression | Keep M14's existing safety checks and add regression tests. |

## Open decisions for review

1. **Zero-argument `print()`:** include only if a PICO-8 fixture proves `?` and
   the token counter agree. Recommended: refuse until proven.
2. **`flr(a / b)` → `a \ b`:** include after negative/non-integer parity tests.
   Recommended: defer.
3. **Redundant parentheses removal:** include after a complete precedence and
   multi-value safety review. Recommended: defer.
4. **Bulk token-limit review:** add a cart-wide preview/confirmation flow only
   after local actions prove stable. Recommended: defer.
5. **Commitment:** M23 is specified but not committed to a release tag until
   these decisions are accepted and the milestone is promoted.
