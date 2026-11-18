# M14 — Refactors and Code Actions

**Status:** Done  
**Depends on:** M13  
**Release target:** v1.2.0 (batched with M11–M13)  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

Everyday PICO-8 edits gain conservative, previewable refactors and code actions.
The server produces standard `WorkspaceEdit`s that editors can preview, refuses
unsafe transformations, and never silently reorders order-sensitive includes.

**Demo:** A developer selects an expression, invokes "Extract to local function",
and previews a `WorkspaceEdit` that introduces a local function with inferred
parameters/returns. On a table literal, "Generate `---@class`" inserts a class
annotation. An exact-class missing-fields warning offers a quick fix that adds
the missing fields.

## Scope

### In scope

- Extract selection to a local function.
- Convert a global to a local when safe.
- Generate `---@class` from a table literal.
- Add missing fields to an exact class annotation.
- Convert standard `if ... then ... end` to PICO-8 shorthand and vice versa
  with line-safety checks.
- Remove unused locals/functions with explicit confirmation.
- Code-action metadata, `WorkspaceEdit` construction, and client-facing
  commands where LSP code actions cannot express a choice.
- Tests, documentation, and configuration.

### Out of scope

- Automatically reordering `#include` directives.
- Changing program behavior or formatting unrelated code.
- Rename/move-file ecosystems beyond the existing rename support.
- Abstract syntax/type-system research beyond M10 behavior.
- Editor-specific UI beyond standard `CodeAction`/`WorkspaceEdit` metadata.

## Requirements

- Every refactor returns a standard `WorkspaceEdit`; no direct file writes.
- Add source actions with stable kinds:
  - `refactor.extract.function`
  - `refactor.rewrite.class`
  - `refactor.rewrite.local`
  - `refactor.rewrite.shorthand`
  - `quickfix.pico-lsp.add-missing-fields`
  - `quickfix.pico-lsp.remove-unused`
- Actions that require confirmation use a command or `CodeAction` metadata that
  editors can preview; the server must never mutate buffers directly.
- Extract-function:
  - Works only on a single expression or statement selection.
  - Captures locals conservatively and inserts the function in the enclosing
    block.
  - Produces a call site with inferred arguments and return values.
  - Refuses selections that cross function boundaries, labels, `break`, or
    shorthand line boundaries.
- Global-to-local:
  - Offered only when all reads/writes resolve to the same global and no
    cross-function forward reference requires global scope.
  - Preserves initializers and include-order visibility.
- Generate `---@class`:
  - Uses a table literal's field names and inferred types.
  - Handles nested/array/string-key fields conservatively; ambiguous fields use
    `any`.
  - Inserts a contiguous class block above the declaration without disturbing
    docs.
- Add missing fields:
  - Derives fields from exact-class diagnostics and the class hierarchy.
  - Inserts only missing fields with `any` or inferred types.
  - Does not modify open classes.
- If/shorthand conversion:
  - Preserves line-bounded semantics and never attaches a next-line `else` to a
    shorthand `if`.
  - Refuses multi-line bodies that cannot be expressed safely.
- Remove unused:
  - Uses existing `unused-local` analysis and requires explicit confirmation.
  - Refuses removal when a use is unresolved or captured dynamically.
- Include order is immutable: no action may add, remove, or reorder
  `#include` lines unless it is the existing explicit include quick fix.
- All actions respect cart/source maps and produce edits in original files, not
  flattened cart coordinates.
- No action is offered when the analysis is from a stale snapshot.
- Documentation lists supported actions, safety limits, and editor behavior.

## Testing

- Unit tests for each refactor's eligibility, generated text, source mapping,
  and refusal cases.
- LSP e2e tests for `textDocument/codeAction`, `WorkspaceEdit` ranges, multiple
  files, included `.lua` files, and confirmation commands.
- Golden tests for generated `---@class` blocks and missing-field inserts.
- Shorthand conversion tests covering next-line `else`, comments, annotations,
  tab boundaries, and line-length limits.
- Include-order tests asserting that no refactor reorders or duplicates
  `#include` directives.
- Corpus regression tests ensuring no action changes unrelated code.
- Performance check: code-action requests on a 64 KB cart stay within the
  editor-request budget.

## Implementation notes

- The transforms live in `internal/refactor`; the LSP layer only converts their
  edits into `WorkspaceEdit`s and exposes the confirmation command.
- `pico8.refactors.enabled` (default `true`) disables every M14 action without
  affecting rename, formatting, or the existing M5 actions.
- Remove-unused is the only action returned without an embedded edit. Its code
  action carries `pico8.removeUnused`; executing that command re-validates the
  symbol against fresh analysis and sends a standard `workspace/applyEdit`.
  Removal is refused for write-only locals, impure initializers, and unresolved
  same-name uses.
- Extract-function targets a single expression or statement, refuses writes to
  captured locals, nested functions, varargs, labels/goto/break/return, and
  multi-value call contexts. Captured locals become parameters.
- Global-to-local only prefixes a top-level first declaration and refuses
  nested-block declarations, cross-file uses, earlier function references,
  lifecycle callbacks, and unresolved same-name occurrences.
- Generate-class uses the assignment target as the class name. Identifier
  fields use inferred/annotated types; string-key identifier fields are emitted
  as `any`; array and dynamic keys are omitted. An existing class with the same
  name suppresses the action.
- Add-missing-fields only records fixes for `(exact)` classes. It inserts
  missing fields into the flagged table literal with a type-appropriate
  placeholder and never edits open classes.
- Shorthand conversion only accepts single-statement bodies, refuses comments
  and multi-line bodies, and caps generated lines at 120 columns. A next-line
  `else` remains attached to its outer standard `if`.
- Every action is refused when any open file in the analyzed snapshot differs
  from the snapshot text.

## Exit criteria

- [x] The listed refactors and quick fixes are available and previewable.
- [x] Unsafe transformations are refused with no partial edits.
- [x] Include order is never mutated by a refactor.
- [x] Workspace edits use original source coordinates across includes/tabs.
- [x] Tests cover eligibility, refusals, and editor e2e behavior.
- [x] Documentation, configuration, and diagnostic references are updated.
- [x] Existing v1/v1.1 behavior and performance budgets do not regress.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Refactor changes semantics | Conservative eligibility checks, previewable edits, and refusal tests. |
| Shorthand conversion breaks line-sensitive code | AST-aware conversion with explicit next-line `else` tests. |
| Cross-file edits land in flattened coordinates | Source-map-aware edit construction and included-file e2e tests. |
| Unused removal deletes live code | Use existing unused analysis, require confirmation, and refuse unresolved captures. |
| Include order changes | Hard invariant: refactors never add, remove, or reorder includes. |
| Editor behavior differs | Standard LSP code-action kinds and documented preview expectations. |
