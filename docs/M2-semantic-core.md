# M2 — Semantic Core & Navigation

**Status:** Done  
**Depends on:** M1  
**Blocks:** M3  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

The server understands names, scopes, declarations, and references across a whole cart. It can answer “where is this defined?”, “where is this used?”, “rename this safely”, and “what symbols exist in this workspace?” It also loads the PICO-8 0.2.7 API as a built-in global namespace so real carts are not flooded with undefined-global diagnostics.

**Demo:** In a cart with two code tabs and an included `lib/player.lua`, go-to-definition jumps from a call in the root to the function in the include, references find every use across tabs, rename updates all of them, and `btn`, `spr`, `circfill`, and `_update` resolve as built-ins.

## Scope

### In scope

- Scope and symbol binding for the flattened cart.
- Local, global, parameter, method, field, label, and built-in symbols.
- PICO-8 case-folding rules for lookup.
- Cart-wide global pre-pass and forward references.
- Embedded PICO-8 0.2.7 API stub and loader.
- Go-to-definition, references, prepare-rename, rename, document highlights.
- Workspace symbols with lazy scanning.
- Include-path go-to-definition.
- Basic undefined-global diagnostics using the loaded built-ins.

### Out of scope

- Completion, hover, and signature help presentation (M3).
- Type annotations and inference (M4).
- Unused/shadowing/pico8-callback lint rules (M5).
- Built-in docs UI and semantic tokens (M3).
- Renaming across multiple carts.
- Open-definition into the embedded built-in stub (hover in M3 is enough).

## Deliverables

1. `internal/analysis` scope/binding pass over the M1 AST.
2. Symbol table and symbol index per cart.
3. Built-in API stub `internal/builtin/pico8-0.2.7.lua` loaded through the same parser.
4. Navigation handlers with cross-file results.
5. Rename with `prepareRename` safety checks.
6. Document highlights.
7. Workspace symbols with cancellation and limits.
8. Scope and navigation tests over a multi-tab/include cart.

## Semantic model

### Symbol kinds

```go
type SymbolKind int

const (
    SymbolLocal SymbolKind = iota
    SymbolGlobal
    SymbolParam
    SymbolFunction
    SymbolMethod
    SymbolField
    SymbolLabel
    SymbolBuiltin
    SymbolClass // used from M4 onward
)
```

### Symbol record

```go
type Symbol struct {
    ID            int
    Name          string   // original spelling
    Canonical     string   // ASCII-lowercased lookup key
    Kind          SymbolKind
    DeclSpan      source.Span
    SelectionSpan source.Span
    Scope         *Scope
    Builtin       bool
    Doc           *DocComment // populated in M3
    Type          types.Type  // populated in M4
    Decls         []source.Span
    Uses          []source.Span
}
```

- `Name` preserves what the user typed.
- `Canonical` is used for all lookups because PICO-8 folds uppercase to lowercase.
- A symbol can have multiple declarations (globals assigned in several places); `Decls` records them all.
- `Uses` excludes comments and strings.

### Scopes

Implement lexical scopes:

- Chunk/cart scope.
- Function scope: parameters, vararg, `self` for methods, body locals.
- Block scope for `do ... end`, `if` branches, `while`, `for`, `repeat`.
- Numeric `for`: loop variable visible in the body only.
- Generic `for`: all loop variables visible in the body only.
- `repeat ... until cond`: body locals are visible in the condition.
- `local` declarations become visible **after** the initializer is evaluated. `local x = x` resolves the right-hand `x` to the outer/global symbol.
- `local function f` becomes visible inside its own body (for recursion).
- Function declaration `function a.b:c()` resolves `a`/`a.b`, adds field `c`, and binds an implicit `self` parameter whose value is the receiver.
- Labels are scoped to the enclosing function; `goto` resolves within that function.

The cart is analyzed as the flattened program from M1. Top-level locals can therefore cross include boundaries exactly as textual pasting would imply. This is intentional and a key difference from a per-file analyzer.

### Globals and forward references

- Any assignment without `local` creates or updates a global symbol.
- Function declarations at any scope without `local` are sugar for `f = function()`. They update a visible local if one exists; otherwise they create a global. Field-path declarations resolve the receiver and add a field instead.
- Build a cart-wide global pre-pass before resolving uses, so a function can be called before its definition.
- Multiple assignments to the same global merge into one symbol with multiple declarations.
- If a global is read but never declared and is not a built-in, report `undefined-global` (warning by default, configurable to hint/off/error).
- Do not report undefined globals inside a `local` initializer if the outer/global symbol exists.
- Assigning to a built-in is allowed; it shadows/overwrites the built-in for the cart. Keep both records and let the user symbol win for navigation. Hover may show both in M3.

### Case folding

- PICO-8 has no upper/lowercase distinction. Normalize ASCII letters to lowercase for `Canonical`.
- Preserve original spelling for display and edits.
- `Player` and `player` refer to the same symbol.
- Rename must update all case variants consistently; see rename rules.
- Non-ASCII glyphs are not case-folded.

### Fields

- Table literal fields (`{x = 1}`) create field symbols on the table.
- `t.x = value` adds/updates field `x` on `t`.
- Method declarations `function t:m()` add field `m` and bind `self`.
- Field resolution is best-effort in M2; type-aware field resolution and diagnostics arrive in M4.
- If the receiver is unknown/`any`, do not invent a field symbol or report an error.
- Indexing with a dynamic key (`t[k]`) does not create a named field symbol.

### Include path navigation

- `#include` directive records from M1 are indexed.
- Go-to-definition on the path returns the included file's start.
- References on an include path are not supported; rename of a path is not supported in v1.
- Missing includes produce no definition target.

## Built-in API stub

Create `internal/builtin/pico8-0.2.7.lua` with `---@meta`, embedded via `//go:embed`.

Requirements:

- Cover the full documented PICO-8 0.2.7 API: graphics, audio, input, math, memory, tables, strings, cartdata, system, and lifecycle callbacks.
- Include constants (`btn_left`, etc.) and pseudo-globals (`_init`, `_update`, `_update60`, `_draw`).
- No Lua standard library. Do not define `require`, `string`, `table`, `math`, `io`, `os`, `coroutine`, `package`, `utf8`, or `debug`.
- Use the server's own lexer/parser to load the stub. This validates the parser and ensures docs are written in the same dialect.
- Mark every symbol `Builtin: true`.
- Load once at startup and share the immutable symbol table across carts.
- `apiVersion` accepts only `"0.2.7"` in v1. Unsupported values log a one-time warning and fall back.

API coverage test:

- Maintain a manifest of expected API names (one per line or a small Go slice).
- Parse the stub and compare: every manifest name exists; every function in the stub is in the manifest. Fail CI on drift.
- Attribution: if API descriptions are derived from `pico8-ls` (MIT) or another source, record attribution in the stub header and `README.md`.

## LSP features

### LuaCATS annotation names

Navigation and rename treat declaration and reference names inside `---@`
comments as part of the semantic model. Annotation type names live in a
namespace separate from code identifiers.

- `@param` names resolve to the function's parameter symbols and count as uses.
- `@field` names declare canonical field symbols for the class. Runtime fields
  on receivers typed by that class link to the same canonical symbol,
  including locals, `@param` parameters, implicit method `self`, and receivers
  of subclasses.
- `@cast` names resolve to the visible local, parameter, or global.
- `@class`, `@alias`, and `@enum` names declare type symbols. Type identifiers
  in annotation type expressions resolve to them, including `@param`, `@return`,
  `@field`, `@type`, `@cast`, `@operator`, `@generic` constraints and defaults,
  `@overload`, class parents, alias targets, and `--[[@as Type]]`.
- Generic parameter names declared by `@generic`, a class header, or a method's
  class stay scoped to their annotation context and are not type references.
- Optional markers (`name?`), field scopes, and other annotation punctuation
  stay outside rename ranges.
- Go-to-definition on an annotation name returns the declaration it annotates.
  References include annotation occurrences. Rename updates annotation and code
  occurrences together.
- Renaming a class field updates its `@field` declaration and every linked
  runtime use across receivers. It refuses when any linked receiver has dynamic
  access.
- Renaming a type updates only annotation occurrences; code identifiers that
  happen to share a spelling are a separate namespace and are left alone.
  Likewise, code rename checks do not treat type names as collisions.
- Annotation occurrences never produce code lint diagnostics.

### Go-to-definition

- Resolve the symbol at the cursor:
  - locals, params, globals, functions, methods, fields, labels;
  - LuaCATS annotation names as described above;
  - built-ins: return no definition in v1 (hover shows docs in M3);
  - include paths: return the included file.
- Return all declarations when a global has multiple.
- Cross-file results use each file's URI and a `LineIndex`-derived range.
- If the cursor is on a non-annotation comment or string, return empty.
- If the cursor is on a field access whose receiver type is unknown, return empty rather than guessing.

### References

- Return all `Uses` plus declarations if `context.includeDeclaration` is true.
- Include uses in all cart files and tabs.
- Exclude comments and strings, except LuaCATS annotation names as described
  above.
- For fields, include all statically resolved accesses on the same receiver
  symbol and its linked class-field family.
- For built-ins, return empty (do not report the entire stub).

### Prepare rename / rename

`textDocument/prepareRename` returns the range and current placeholder only when the symbol is renameable.

Refuse rename for:

- built-in symbols;
- keywords, operators, labels if not safely resolvable;
- fields with dynamic access (`t[k]`) or unknown receiver types;
- include paths;
- symbols referenced from a different cart.

Rename rules:

- Rename all declarations and uses in the cart.
- New name must be a valid PICO-8 identifier and not a keyword. Type
  declarations also accept qualified names such as `Player.State`, but cannot
  use a reserved primitive type name such as `number` or `table`.
- Preserve the user's original casing style where reasonable: if the symbol was consistently uppercase, uppercase the new name; otherwise use the typed spelling. If usages mix cases, replace all with the typed spelling (PICO-8 folds them anyway).
- Do not rename inside comments or strings, except LuaCATS annotation names as
  described above.
- Return a `WorkspaceEdit` grouped by URI; do not apply edits directly.
- Check for collisions in the target scope; refuse or warn if a local collision
  would occur. Type declarations use their own namespace, so a type rename does
  not collide with a same-named local and a code rename does not collide with a
  same-named type.

### Document highlights

- Highlight all references to the symbol at the cursor within the current document.
- Return `Read` and `Write` kinds where distinguishable.
- If the symbol is a built-in, highlight only explicit uses in the document (or none).

### Workspace symbols

- Search functions, globals, methods, classes (from M4), and labels across carts in the workspace.
- Lazy: do not scan at startup.
- Cancellable; limit results (for example, 500) and report truncation in the log.
- Query matching is case-insensitive by canonical name and supports fuzzy subsequence matching if cheap.
- `workspace/symbol` responses contain `Location` or `WorkspaceSymbol` with container name (cart/tab).

### Include go-to-definition

- On the path portion of an `#include` line, return the included file.
- On the `#include` keyword itself, return the same target for convenience.
- If missing, return empty; M5 adds a code action to create the file.

## Diagnostics added in this milestone

| Code | Default | Behavior |
|---|---|---|
| `undefined-global` | Warning | A read of a name that is neither local, global, field, nor built-in. Configurable to `off`, `hint`, `warning`, `error`. |
| `unknown-label` | Warning | `goto` target not found in the function. |
| `duplicate-label` | Warning | Two labels with the same name in one function. |

Implementation notes:

- Undefined-global diagnostics are published per cart, including included files.
- Suppress them in `---@meta` files and inside the built-in stub.
- Do not report a global that is assigned later in the cart; the pre-pass handles this.
- Do not report fields on unknown receivers.
- Respect `pico8.maxProblems`.

## Architecture and files

```
internal/analysis/scope.go        Scope stack and lookup
internal/analysis/symbol.go       Symbol, SymbolTable
internal/analysis/bind.go         AST → symbols/scopes
internal/analysis/index.go        per-cart index, file→cart ownership
internal/analysis/builtin.go      load embedded stub, merge globals
internal/analysis/diagnostics.go  undefined-global, labels
internal/builtin/pico8-0.2.7.lua  API stub
internal/builtin/builtin.go       go:embed, manifest, loading
internal/lsp/definition.go
internal/lsp/references.go
internal/lsp/rename.go
internal/lsp/highlight.go
internal/lsp/workspace_symbols.go
internal/lsp/include_navigation.go
testdata/builtin/                 API manifest and coverage fixtures
testdata/navigation/              multi-tab/include fixture
```

## Data flow

```
CartSnapshot from M1
        │
        ▼
bind pass
  ├─ create cart scope
  ├─ pre-pass globals/functions/methods
  ├─ bind locals/params/blocks
  ├─ resolve uses to symbols
  └─ attach labels/goto
        │
        ├─ merge built-in stub symbols
        ├─ publish undefined-global diagnostics
        └─ build navigation index
                │
                ▼
definition / references / rename / highlights / workspace symbols
```

## Error handling

- If a file in the cart disappears, keep the last analysis for open documents and mark the missing file; do not crash.
- If the built-in stub fails to parse, log a fatal startup error and disable built-in symbols; all other features must still work.
- If rename produces a collision, return a JSON-RPC error with a clear message; do not partially apply.
- Ignore navigation requests for unknown documents; return empty.
- Bound workspace symbol scanning by time/result count and honor cancellation.

## Testing

### Unit tests

- Scope visibility:
  - `local x = x` resolves RHS to outer/global.
  - `local function f` resolves recursively.
  - for-loop variables are not visible outside the loop.
  - repeat-until condition sees body locals.
  - if/elseif/else branches do not leak locals.
  - method `self` binds to the receiver.
- Globals:
  - forward reference before definition;
  - multiple assignments;
  - global assigned in one include, used in another;
  - built-in shadowing.
- Case folding: `Player`/`player` resolve to one symbol; rename updates both spellings.
- Fields: table literal, assignment, method declaration, dynamic key not resolved.
- Labels/goto: forward/backward, nested functions isolate labels.
- Includes: nearest-owner resolution; two carts sharing a file choose the nearest.
- Built-in manifest coverage; no standard-library names present.
- Undefined-global: valid built-ins, user globals, forward refs; suppression in `meta`.

### End-to-end tests

- Definition/references/rename across two tabs and an include.
- Rename refuses built-ins.
- Rename returns a multi-file `WorkspaceEdit`.
- Workspace symbols are cancellable and limited.
- Include path definition.

### Corpus

- Run the binder over the M1 corpus.
- No panics; undefined-global diagnostics should be low. Investigate systematic false positives (dynamic globals, generated code) and tune.

## Implementation notes

Resolved during implementation:

- Built-in names and signatures are transcribed from the PICO-8 0.2.7 manual;
  the one-line descriptions in the stub are original summaries. No text is
  copied from `pico8-ls`.
- Workspace symbols scan the client's workspace folders lazily. When no folders
  are configured, they fall back to the carts behind open documents. The scan is
  capped at 500 results, 2000 files, and a 5 second budget. The scan releases
  the JSON-RPC read loop and honors `$/cancelRequest`, returning
  `RequestCancelled` when interrupted.
- A dot or method access that appears before its declaration creates a
  provisional field symbol. A later declaration merges with the earlier use, so
  rename does not miss forward references. Provisional fields have no
  declarations and cannot be renamed on their own.
- A field accessed through a string literal (`t["x"]`) is treated as a dynamic
  access: the cursor on the string does not navigate, and the receiver is marked
  unsafe for field rename because editing inside a string literal is out of
  scope for v1.
- `self` is bound as an implicit method parameter, but it has no textual
  declaration occurrence. Go-to-definition points at the method name; rename
  refuses it rather than editing a method name that is not a `self`
  declaration.
- Label rename refuses when an unresolved `goto` with the new name exists in
  the same function, since the renamed label would silently capture it.
- Unresolved dynamic globals such as `_ENV`-created names and test-harness
  globals may still produce `undefined-global` warnings in the corpus. They are
  intentionally not hard-coded as built-ins.

## Exit criteria

- [x] Every M1 AST declaration/use is bound to a symbol or intentionally ignored.
- [x] Cross-file definitions and references work across tabs and includes.
- [x] Rename works for locals/globals/functions/methods/fields and refuses unsafe cases.
- [x] Built-in 0.2.7 symbols resolve; no standard Lua library is present.
- [x] Undefined-global diagnostics are useful and configurable.
- [x] Workspace symbols work without startup scanning.
- [x] Case folding matches PICO-8 behavior.
- [x] `go test -race ./...` passes.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Case folding differs from PICO-8 in edge cases | Test uppercase carts from the corpus; document the canonicalization rule. |
| Flattened includes create surprising scope behavior | Add tests that mirror textual paste semantics; document in the spec and user docs. |
| Undefined-global diagnostics are noisy | Default to warning but make it easy to turn off; exclude meta files; refine with types in M4. |
| Rename corrupts code | Conservative prepare/rename checks; collision detection; never rename built-ins or dynamic fields. |
| Built-in stub drifts from the manual | Manifest coverage test; version pin; attribution and review checklist. |
| Workspace symbol scan is slow | Lazy, cancellable, result-limited; no startup scan. |
