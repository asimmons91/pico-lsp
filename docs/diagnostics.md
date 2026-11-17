# Diagnostics

All diagnostics use `source: "pico-lsp"` and a stable `code`. Severity defaults
come from [configuration.md](configuration.md); most rules can be changed or
disabled there. If diagnostics are missing or land in the wrong place, see
[troubleshooting.md](troubleshooting.md).

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
