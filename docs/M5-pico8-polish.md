# M5 — PICO-8-Specific Polish

**Status:** Done — automated checks pass; manual editor parity waived for v1.  
**Depends on:** M4  
**Blocks:** M6  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

The server stops feeling like a generic Lua language server and starts feeling like a PICO-8 tool. It surfaces the constraints and idioms that matter: token/character budgets, include problems, lifecycle callbacks, PICO-8-only API usage, and useful quick fixes.

**Demo:** Open a large cart and see `4210 / 8192 tokens · 27433 / 65535 chars` above the code. Add includes until the token count crosses 90% and then 100%, with a warning and then an error. A nested `#include` is flagged, `_update` plus `_update60` produces a hint, and a quick fix generates missing `---@param` annotations for a function.

## Scope

### In scope

- Token and character budget code lens and diagnostics.
- `pico8/cartInfo` request and `pico8.showCartInfo` command.
- Include diagnostics and include-related quick fixes.
- PICO-8-specific lint rules with low false positives.
- Code actions for annotations, diagnostics suppression, and include creation.
- Diagnostic configuration, severity mapping, and problem limits.
- Workspace/file-watching behavior for carts and includes.
- Robustness and performance validation on the full corpus.

### Out of scope

- Formatter (M7).
- Compressed-size calculation for `.p8.png` (M7).
- Sprite/map/audio previews.
- Runtime/emulator integration.
- API version selection beyond 0.2.7.
- Advanced range-checking for every API argument (optional stretch; see below).

## Token and character budget

### Counts

Use the M1 token/char counter over the flattened cart. Never recompute per file independently for the limit; PICO-8 limits apply to the assembled program.

Expose:

```go
type CartInfo struct {
    RootURI      uri.URI
    Tokens       int
    TokenLimit   int // 8192
    Chars        int
    CharLimit    int // 65535
    Files        []FileInfo
    Includes     []IncludeInfo
    Tabs         []TabInfo
}

type FileInfo struct {
    URI    uri.URI
    Tokens int
    Chars  int
    Bytes  int
}
```

Per-file counts are informational; the limit applies to the sum after flattening. Duplicate includes count twice, matching PICO-8.

### Code lens

- Place a single code lens at the top of the root cart document (line 0).
- Label format:

  `4210 / 8192 tokens · 27433 / 65535 chars`

- Optionally include a compact status icon: `✓`, `!`, or `×`.
- Clicking runs `pico8.showCartInfo` with the cart root URI.
- Do not place a lens in included files.
- Make the lens configurable: `pico8.limits.codeLens` (default `true`).
- If counts are unavailable (cart not analyzed), omit the lens rather than showing zero.

### Diagnostics

| Condition | Severity | Code | Range |
|---|---|---|---|
| Tokens > 8192 | Error | `token-limit` | Cart root, line 0 |
| Tokens >= 90% of limit | Warning | `token-limit` | Cart root, line 0 |
| Chars > 65535 | Error | `char-limit` | Cart root, line 0 |
| Chars >= 90% of limit | Warning | `char-limit` | Cart root, line 0 |

- Emit at most one token-limit and one char-limit diagnostic per cart.
- Include the current count and limit in the message.
- Add a related information entry pointing at the largest included file or tab when useful.
- Do not emit warnings at exactly 90%? Define `>= floor(limit*0.9)`; test boundaries.
- Thresholds are fixed in v1; reserve `pico8.limits.warnAtPercent` for later.
- Allow disabling limit diagnostics while keeping the code lens: `pico8.limits.diagnostics` (default `true`).

### `pico8.showCartInfo`

`workspace/executeCommand` command:

```json
{
  "command": "pico8.showCartInfo",
  "arguments": ["file:///path/to/cart.p8"]
}
```

Behavior:

- Compute/read the latest `CartInfo`.
- Show a `window/showMessage` (info or warning) with a compact summary and the largest contributors.
- Return the `CartInfo` as the command result for clients that want it.

### `pico8/cartInfo`

Custom request:

```
pico8/cartInfo
params: { textDocument: { uri } }
result: CartInfo
```

- Useful for Neovim statusline plugins.
- Cancellable.
- If the URI is an included file, return its owning cart's info.
- If no cart is found, return an error with code `cart-not-found`.
- Document the request in `docs/configuration.md` or `docs/api.md`.

## Include diagnostics and features

### Diagnostics

| Condition | Severity | Code |
|---|---|---|
| Include target missing | Error | `include-missing` |
| Include target unreadable/binary/oversized | Error | `include-unreadable` |
| Include inside an included file (nested) | Warning | `include-nested` |
| Include path escapes the workspace | Error/Warning | `include-outside-workspace` |
| Include target is not a `.lua` file | Hint/Warning | `include-unusual-type` |
| Duplicate include | Hint, off by default | `include-duplicate` |
| Include directive inside a string/comment | No diagnostic; not a directive | — |

Requirements:

- Report at the path range, not the whole line, where possible.
- Include the resolved path and the cart root in the message when helpful.
- Keep analyzing the rest of the cart after a missing include.
- Re-evaluate include diagnostics when any cart member changes or a watched file is created/deleted.
- PICO-8 includes are non-recursive; the nested warning must explain that the nested line will be pasted literally.

### Include navigation and completion

M2/M3 provide go-to-definition and path completion. M5 adds:

- Hover on an include path: resolved path, file size, token contribution, nested status.
- Code action “Create missing include file” when the target does not exist.
  - Creates an empty `.lua` file with a `WorkspaceEdit` `CreateFile` resource operation.
  - Ask for confirmation? LSP code actions are explicit user actions; no extra prompt.
  - Refuse paths outside the workspace.
- Code action “Open included file” is unnecessary; go-to-definition covers it.
- Include path completion excludes the root cart itself and hidden directories.

## PICO-8 lint rules

All rules must be individually configurable and default to low noise. Use diagnostic codes prefixed with `pico8-`.

### Lifecycle callbacks

- `_init`, `_update`, `_update60`, `_draw` are optional global functions.
- If both `_update` and `_update60` are defined:
  - Hint: “`_update60` is defined, so `_update` will not be called.”
  - Range: the `_update` declaration name.
  - Code: `pico8-callback-precedence`.
  - Default: warning? Use **hint** to avoid anxiety; users can raise it.
- If a lifecycle callback is declared with parameters:
  - Warning: “PICO-8 calls `_update` with no arguments.”
  - Code: `pico8-callback-params`.
- If `_update60` and `_update` are both annotated differently, do not add type errors.

### Lua standard library usage

PICO-8 has no Lua standard library. For known names:

`require`, `dofile`, `loadfile`, `load`, `loadstring`, `string`, `table`, `math`, `io`, `os`, `package`, `coroutine`, `debug`, `utf8`

- `undefined-global` already covers most reads. Add a targeted hint:
  “PICO-8 does not include the Lua standard library; use the PICO-8 API.”
- Code: `pico8-no-stdlib`.
- Do not emit for user-defined globals with the same name; user definitions win.
- Do not emit in `---@meta` files.

### API literal range checks (stretch)

Optional for M5; only if time and corpus validation allow. Keep behind `pico8.lint.apiRanges` (default `false`).

Examples:

- Color arguments: `0..15` for drawing functions, `0..15` for `pal`.
- Sprite index: `0..255`.
- SFX/music index: `0..63`; allow `-1` for stop/disable where documented.
- `flip_x`/`flip_y`: booleans or 0/1.
- `stat(n)`: known stat numbers only; warn on unknown literals.

Only check literal arguments at statically known API call sites. Never check dynamic expressions. Never emit if the API signature is uncertain. This is a nice-to-have, not an exit criterion.

### Style and correctness hints

Default off unless noted:

- `unused-local` — off; `pico8.diagnostics.unusedLocal`.
- `shadowed-local` — off.
- `duplicate-table-field` — hint.
- `unreachable-code` — hint for statements after `return`/`break` in the same block.
- `empty-block` — off.
- `deprecated` — hint from M4.
- `global-in-function` — off; assigning a global inside a function may be intentional and PICO-8 code often uses it.

Do not add rules that require whole-program execution or dynamic analysis.

## Code actions

### Generate type annotations

For a function declaration with no `---@param`/`---@return`:

- Action: “Generate type annotations from inferred types.”
- Produces an edit inserting a documentation block above the function.
- Only include types that are not `any`; use `any` for unknown parameters if needed? Better: omit unknown params or use `any` with a note.
- Include `@param` for each parameter and `@return` for each return.
- Preserve existing annotations; merge missing ones.
- Requires M4 type info.
- If no type info is available, do not offer the action.

### Suppress diagnostic

For diagnostics with a code:

- Action: “Suppress `<code>` on this line.”
- Inserts `---@diagnostic disable-next-line: <code>` above the line.
- Only offer for diagnostics that support suppression.
- Do not offer for syntax errors or include errors where suppression would hide real breakage.

### Create missing include file

- Offered on `include-missing`.
- Creates an empty `.lua` file at the resolved path.
- Refuses paths outside the workspace.
- After creation, the client watcher or `didChangeWatchedFiles` triggers re-analysis.

### Add local

Optional: for an assignment to an undeclared local-looking name inside a function, offer “Declare as local.” This is risky to infer; do not implement unless the intent is unambiguous.

### Deprecated API

If a symbol is deprecated, no code action; hover/semantic tokens show it.

## Diagnostics configuration and limits

Extend `internal/config`:

```json
{
  "pico8": {
    "maxProblems": 100,
    "limits": {
      "codeLens": true,
      "diagnostics": true
    },
    "diagnostics": {
      "undefinedGlobal": "warning",
      "unusedLocal": "off",
      "unreachableCode": "hint",
      "duplicateTableField": "hint"
    },
    "lint": {
      "apiRanges": false
    }
  }
}
```

Rules:

- Severity values: `off`, `hint`, `warning`, `error`.
- Type diagnostics remain warnings in `basic`; users may raise them.
- Syntax and missing includes remain errors and are not configurable to `off` (they can be ignored per line only if the protocol allows; syntax errors cannot).
- `maxProblems` applies per document per publish; truncate deterministically and log.
- Document all codes in `docs/diagnostics.md` by M6.
- Changing settings re-publishes diagnostics for open carts without a restart.

## Workspace and file watching

- Register for `workspace/didChangeWatchedFiles` when the client supports it, for `**/*.p8` and `**/*.lua` (or the narrowest workspace-relative globs the client accepts).
- When an include file changes on disk:
  - If open, use the overlay.
  - If closed, reload and re-analyze the owning cart.
  - Publish updated diagnostics for all affected files.
- When a file is created/deleted/renamed:
  - Recompute include ownership for affected carts.
  - Clear diagnostics for removed files.
  - Update `pico8/cartInfo`.
- Do not scan the whole workspace at startup. Ownership is resolved on demand.
- Multiple carts: nearest-ancestor owner rule from M2. If a `.lua` is included by multiple carts, only the owner publishes diagnostics; log the others at debug.
- If an include file is open but its owning cart is not, analyze it as a standalone cart with built-ins until ownership is resolved.

## Robustness and performance validation

- Run the full corpus with all diagnostics enabled.
- Track false-positive rates for lint rules; any rule that fires on more than a small percentage of valid carts should be re-evaluated or defaulted off.
- Benchmark:
  - full analysis + diagnostics on 64 KB < 15 ms;
  - code lens/cart info < 1 ms after cache;
  - no workspace scan on startup.
- Stress:
  - cart with 50 includes;
  - deep nesting;
  - huge comments/strings;
  - cyclic ownership attempt (A includes B includes A is non-recursive; ensure no hang).
- Ensure all diagnostics clear when a cart is fixed, closed, or deleted.

## Architecture and files

```
internal/cart/tokens.go             finalize counts and per-file contributions
internal/lsp/code_lens.go           cart budget lens
internal/lsp/cart_info.go           pico8/cartInfo + command
internal/lsp/code_actions.go        annotations, suppression, include creation
internal/lsp/diagnostics.go         aggregation, config, truncation
internal/lsp/watched_files.go       file watching and invalidation
internal/analysis/lint.go           PICO-8 lint rules
internal/analysis/lint_api.go       optional API range checks
docs/diagnostics.md                 diagnostic code reference
testdata/lint/                      lint fixtures
testdata/limits/                    token/char limit fixtures
```

## Data flow

```
cart snapshot (M1-M4)
      │
      ├─ CartInfo (tokens, chars, files, includes)
      │       ├─ code lens
      │       ├─ limit diagnostics
      │       └─ pico8/cartInfo / showCartInfo
      │
      ├─ include records → include diagnostics/navigation/actions
      ├─ symbols/types → lint rules
      ├─ diagnostics → config filter → maxProblems → publish
      └─ watched files → invalidate cart → re-analyze → republish
```

## Error handling

- If counts are stale relative to the document version, recompute before publishing the lens/diagnostic.
- If a file watcher event references an unknown file, ignore it.
- If include creation fails, return a JSON-RPC error with the reason.
- If diagnostic reprocessing exceeds a time budget, publish syntax/include diagnostics first and type/lint diagnostics on the next pass.
- Never remove diagnostics for files not owned by the cart.
- Settings changes must be idempotent; avoid duplicate diagnostic publishes.

## Testing

### Token/char limits

- 0, 1, 90%, just under 90%, 90%, 100%, over 100%.
- Includes counted; comments/annotations excluded from tokens; long strings one token.
- Duplicate includes count twice.
- Code lens label format.
- `pico8/cartInfo` for root and included file; unknown cart error.
- `pico8.showCartInfo` result and message.

### Includes

- Missing, nested, unreadable, outside workspace, unusual extension.
- Create-missing-file action; workspace edit resource operation.
- Watched file create/change/delete updates diagnostics and cart info.
- Ownership with two carts sharing a file.

### Lint

- `_update` + `_update60` hint.
- Lifecycle callback params.
- Standard-library names vs user globals.
- Unused local on/off.
- Duplicate table fields.
- Unreachable code.
- Optional API range checks behind the setting.

### Code actions

- Annotation generation with and without inferred types.
- Suppression insertion for a type diagnostic.
- Refusal for syntax/include errors.
- Create include file refusal outside workspace.

### Workspace

- Multi-cart workspace; only nearest owner publishes.
- Close/reopen a cart; diagnostics clear and reappear.
- Rename/delete an include file.
- Cancellation of slow workspace operations.

### Corpus and fuzzing

- Run all diagnostics over the corpus; review every new diagnostic for false positives.
- Fuzz include paths and watched-file events.

## Implementation notes

- Limit and cart-info tests live with the implementation rather than in
  separate `testdata/limits` and `testdata/lint` trees.
- Per-file token/character contributions are recorded by the cart loader;
  flattener-inserted separators are attributed to the root cart.
- `pico8.lint.apiRanges` is reserved; literal API range checking remains the
  optional stretch item and is disabled by default.
- File watching is dynamically registered when the client supports it.
  Deletion uses the same cleanup path as closing a document so diagnostics for
  removed files are cleared.
- Include outside-workspace checks use configured workspace folders plus the
  cart directory, so carts opened outside a workspace remain usable.
- Generated annotations merge missing `@param`/`@return` entries and omit
  inferred `any` types.

## Exit criteria

- [x] Token and char counts are accurate for the assembled cart.
- [x] Code lens shows the budget and updates after edits.
- [x] Warning at 90% and error at 100% for tokens and chars.
- [x] `pico8/cartInfo` and `pico8.showCartInfo` work.
- [x] Include diagnostics cover missing/nested/unreadable/outside-workspace cases.
- [x] “Create missing include file” works.
- [x] Lifecycle callback hint and no-stdlib hint work with low noise.
- [x] Code actions generate annotations and suppress diagnostics safely.
- [x] File watching updates diagnostics and cart info.
- [x] Lint rules are configurable and corpus-validated.
- [x] Performance and stress tests pass.
- [x] `go test -race ./...` passes.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Token counter mismatch shakes trust | Oracle fixtures, boundary tests, cross-check with PICO-8/shrinko8. |
| Lint noise | Default new rules off or hint; corpus validation; easy config. |
| Code lens clutter | Single lens at root; configurable; compact label. |
| Watcher storms | Debounce invalidation, deduplicate, ignore unknown files. |
| Stale diagnostics across carts | Ownership index and clear-on-close; only owner publishes. |
| API range checks are wrong | Keep behind a default-off flag; only check literals and known signatures. |
| Include creation affects disk | Explicit code action, workspace-scoped path, clear error reporting. |
