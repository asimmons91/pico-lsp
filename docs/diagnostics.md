# Diagnostics

All diagnostics use `source: "pico-lsp"` and a stable `code`. Severity defaults
come from [configuration.md](configuration.md); most rules can be changed or
disabled there. If diagnostics are missing or land in the wrong place, see
[troubleshooting.md](troubleshooting.md).

## Push and pull

Clients that advertise `textDocument.diagnostic` can use pull diagnostics.
`pico8.diagnostics.mode` controls the transport:

- `auto` (default) pulls when the client supports it, otherwise pushes.
- `push` always publishes; pull requests still return the same diagnostics.
- `pull` answers pull requests and suppresses duplicate pushes. A client
  without pull support receives pushed diagnostics plus one warning in the
  server log.

A text-document pull returns a full report with a stable `resultId`, or an
unchanged report when the client sends the current `previousResultId`. Related
documents from the same cart (includes and other tabs) are returned when the
client advertises related-document support. `workspace/diagnostic` is bounded
to open carts and can stream partial results. `pico8.maxProblems` truncates each
original document independently, and pull reports use the same severity and
suppression rules as pushed diagnostics.

## Cart and syntax

| Code | Default | Meaning |
|---|---|---|
| `syntax` | Error | Lexer or parser error. |
| `include-missing` | Error | Included file does not exist. |
| `include-unreadable` | Error | Included file is unreadable, binary, or too large. |
| `include-nested` | Warning | Nested `#include` is pasted literally. |
| `include-outside-workspace` | Warning | Include resolves outside the workspace boundary. |
| `include-unusual-type` | Hint | Include target is not `.lua` or `.p8`. |
| `include-duplicate` | Off | Same include target appears more than once. |
| `token-limit` | Error/Warning | Assembled cart is over or near the 8192-token limit. |
| `char-limit` | Error/Warning | Assembled cart is over or near the 65535-character limit. |
| `compressed-size-limit` | Error/Warning | Assembled cart is over or near PICO-8's 15616-byte compressed code limit. Warning from 14054 bytes; error above 15616 bytes. |

Cart budget diagnostics are attached to the cart root and controlled by
`pico8.limits.diagnostics`. They are not suppressible with
`---@diagnostic` directives. Compressed size is reported only for `.p8` carts
(including decoded `.p8.png` virtual carts), not for standalone `.lua` files.

## Semantic

| Code | Default | Meaning |
|---|---|---|
| `undefined-global` | Warning | Read of an undeclared global. |
| `unknown-label` | Warning | Goto target does not exist. |
| `duplicate-label` | Warning | Label is declared more than once. |

## Annotations

| Code | Default | Meaning |
|---|---|---|
| `annotation-invalid` | Warning | Unknown or malformed annotation, unknown or non-class parent, inheritance cycle, or invalid `(exact)` placement. |
| `annotation-unknown-type` | Warning | Annotation references an unknown type. |
| `annotation-duplicate` | Warning | Duplicate `@param` name. |
| `annotation-unknown-param` | Warning | `@param` name is not in the function. |
| `annotation-missing-name` | Hint | `@field` has no name. |

## Types

| Code | Default | Meaning |
|---|---|---|
| `assign-type-mismatch` | Warning | Value is not assignable to an annotated target, including extra fields on an explicit `(exact)` class. |
| `param-type-mismatch` | Warning | Call argument is not assignable to a parameter. |
| `return-type-mismatch` | Warning | Return value is not assignable to an annotated return. |
| `undefined-field` | Warning | Field is missing on an exact class and its parents (or any record in `strict`). |
| `need-check-nil` | Off in `basic` | Optional value is used where non-nil is required. |
| `missing-fields` | Warning for `(exact)` | Table literal is missing required fields of an explicit `(exact)` class; in `strict`, applies to any annotated record. |
| `operator-type-mismatch` | Hint | Explicit `---@operator` operand does not match the value used with the operator. |
| `deprecated` | Hint | Deprecated symbol is used. |
| `type-unknown` | Reserved | Type could not be inferred; unused in v1. |

## PICO-8 lint

| Code | Default | Meaning |
|---|---|---|
| `pico8-callback-precedence` | Hint | Both `_update` and `_update60` exist; `_update` is not called. |
| `pico8-callback-params` | Warning | Lifecycle callback declares parameters. |
| `pico8-no-stdlib` | Hint | Lua standard-library name used; PICO-8 has no standard library. |
| `duplicate-table-field` | Hint | Table constructor repeats a field name. |
| `unreachable-code` | Hint | Statement follows `return` or `break` in the same block. |
| `unused-local` | Off | Local is declared and never read. |
| `shadowed-local` | Off | Local shadows an outer declaration. |
| `empty-block` | Off | Block contains no statements. |
| `global-in-function` | Off | Global is assigned inside a function. |

## Code actions

`missing-fields` diagnostics carry a `quickfix.pico-lsp.add-missing-fields`
action for explicit `(exact)` classes. It inserts the absent fields into the
flagged table literal with type-appropriate placeholder values; open classes and
`strict`-only missing-field checks do not get the fix.

`unused-local` diagnostics carry a `quickfix.pico-lsp.remove-unused` action.
The action is also offered from the declaration itself when the diagnostic is
disabled. It is executed through the `pico8.removeUnused` confirmation command
and is refused for write-only locals, impure initializers, or unresolved
same-name uses. See [api.md](api.md) for all refactor kinds.

## Suppression

Type and PICO-8 lint diagnostics can be suppressed with LuaCATS directives:

```lua
---@diagnostic disable-next-line: assign-type-mismatch
local value = "not a number"

---@diagnostic disable-line: unused-local
local unused = 1
```

`disable` and `enable` apply from their line onward; `disable-line` applies to
the same line and `disable-next-line` to the following line. Omit the code list
to affect all suppressible diagnostics.

Syntax and include errors are not suppressible.
