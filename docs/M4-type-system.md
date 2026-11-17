# M4 — Type System & Annotations

**Status:** Done — automated checks pass; manual editor parity waived for v1.  
**Depends on:** M3  
**Blocks:** M5  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

Users can annotate PICO-8 code with LuaCATS comments and get useful type information without leaving the editor. The server infers types for unannotated code where it can, checks explicit annotations, narrows types through control flow, and powers type-aware completion, hover, signature help, diagnostics, and inlay hints.

Type checking is **gradual and low-noise**. Annotations are optional. Unannotated code should not suddenly light up with errors.

**Demo:**

```lua
---@class Player
---@field x number
---@field y number
---@field hp integer
---@field sprite integer

---@param p Player
---@param dx number
---@param dy number
---@return boolean moved
local function move(p, dx, dy)
    p.x += dx
    p.y += dy
    return dx != 0 or dy != 0
end

---@type Player
local player = { x = 64, y = 64, hp = 3, sprite = 1 }

if (btn(0)) move(player, -1, 0)
```

- Hover `move` shows `function move(p: Player, dx: number, dy: number): boolean`.
- Completion after `player.` shows `x`, `y`, `hp`, `sprite`.
- Hover `player` shows `Player`.
- Passing `"left"` as `dx` produces a warning.
- Optional inlay hints show inferred types for unannotated locals.

## Scope

### In scope

- LuaCATS comment annotation parser and association.
- Core type model and canonicalization.
- Gradual inference for locals, globals, functions, tables, and expressions.
- Call-site inference for unannotated function parameters.
- Flow narrowing for common Lua idioms.
- Type checking of explicit annotations.
- Type-aware completion, hover, signature help, and inlay hints.
- Type diagnostics with `off`/`basic`/`strict` modes.
- `---@diagnostic` suppression for type diagnostics.
- PICO-8 iterator/collection special cases.

### Out of scope

- Luau-style inline type syntax (`local x: number`).
- Generic classes/functions (`@generic`) in v1.
- Overload resolution (`@overload`).
- Operator metatypes (`@operator`).
- Metatable-aware inference (`setmetatable`, `__index`).
- Exact class checking by default.
- Cross-cart type analysis.
- Full Lua standard library types.

## Annotation support

### Required annotations

| Annotation | Behavior |
|---|---|
| `---@class Name` | Declares a named structural table class. Open by default. `(exact)` classes reject unknown fields in `basic`; `strict` additionally checks table literals for missing fields. |
| `---@field [scope] name? Type description` | Adds a field to the preceding class. Optional field means `Type|nil`. |
| `---@param name? Type description` | Binds a parameter type. `name?` makes it optional. |
| `---@return Type [name] [description]` | Adds a return type; multiple lines form a tuple. |
| `---@type Type` | Attaches a type to the next local/assignment/expression. |
| `---@alias Name Type` | Declares a named alias, including union aliases. |
| `---@enum Name` | Declares an enum-like table/alias. Literal values become members. |
| `---@meta` | Marks the built-in stub; suppress user-code diagnostics and completion context. |
| `---@deprecated` | Marks a symbol deprecated; semantic tokens and completion tags. |
| `---@diagnostic state: code, ...` | Disables/enables diagnostics on the next line, current line, or file. |
| `---@cast name [+|-]Type, ...` | Narrows or widens a variable from this point. |
| `--[[@as Type]]` | Forces an expression type. Must be parsed from a block comment. |

### Deferred annotations

Parse and ignore without errors, but do not implement semantics in v1:

`@generic`, `@overload`, `@operator`, `@module`, `@package`, `@private`,
`@protected`, `@public`, `@version`, `@async`, `@nodiscard`, `@see`,
`@source`, `@vararg`.

`@generic`, `@overload`, and `@operator` were deferred in M4 and are implemented
by [M10](M10-type-system-2.md).

Unknown annotations produce an `annotation-invalid` hint, not a hard error.

### Type grammar

```
Type        := Union
Union       := Postfix ('|' Postfix)*
Postfix     := Primary ('[]' | '?')*
Primary     := 'nil' | 'any' | 'boolean' | 'number' | 'integer'
             | 'string' | 'table' | 'function' | 'thread' | 'userdata'
             | Name ('<' Type (',' Type)* '>')?
             | '{' Field (',' Field)* '}'
             | '[' Type (',' Type)* ']'
             | 'fun' '(' ParamList ')' (':' TypeList)?
             | StringLiteral | NumberLiteral | 'true' | 'false'
```

Notes:

- `T?` is sugar for `T | nil`.
- `integer` is an alias for `number` with documentation metadata. PICO-8 numbers are 16.16 fixed point; do not model ranges in v1.
- `table<K,V>` is a map; `T[]` is an array; `{ name: T }` is a record.
- `fun(x: number, y?: string): boolean` is a function type.
- `[T1, T2]` is a tuple type, primarily for multiple returns.
- Literal types are supported for enums and constants.
- Type names are case-insensitive under PICO-8 folding; preserve display spelling.

### Annotation association

- Attach contiguous `---@` comment blocks to the next declaration.
- A non-comment blank line breaks association.
- Regular `---` documentation lines may appear between annotations and the declaration.
- `@field` attaches to the nearest preceding `@class` block.
- `@param` matches parameters by name; unmatched names produce a warning.
- `@return` lines accumulate in order.
- `@type` attaches to the immediately following local declaration, assignment target, or expression statement.
- `--[[@as T]]` attaches to the expression it follows on the same line.
- `@cast` applies from its line onward within the current block.
- `@diagnostic` applies to the next line, current line, or whole file as specified.

### Annotation diagnostics

| Code | Default | Example |
|---|---|---|
| `annotation-invalid` | Warning | Unknown annotation, malformed type, missing name. |
| `annotation-unknown-type` | Warning | `---@param p Playr` |
| `annotation-duplicate` | Warning | Two `@param` entries for the same name. |
| `annotation-unknown-param` | Warning | `@param` name not in the function. |
| `annotation-missing-name` | Hint | `@field` without a name. |

Annotation errors must not prevent analysis; treat the annotation as absent after reporting.

## Type model

```go
type Type interface{ isType() }

type Primitive struct{ Kind PrimitiveKind } // nil, boolean, number, string, table, function, any
type Literal struct{ Value any }
type Union struct{ Members []Type }
type Array struct{ Elem Type }
type Map struct{ Key, Elem Type }
type Record struct {
    Name    string
    Fields  map[string]*Field
    Open    bool
    Exact   bool
    Parents []*Record
}
type Field struct {
    Name     string
    Type     Type
    Optional bool
    Decl     source.Span
}
type Function struct {
    Params  []Param
    Returns Tuple
    Vararg  Type
}
type Tuple struct{ Elems []Type; Vararg Type }
type TypeVar struct{ ID int; Bound Type }
```

Requirements:

- Canonicalize unions: flatten nested unions, remove duplicates, absorb `any`, sort deterministically for display.
- `nil` is a first-class type, not “no type.”
- `any` is the top type; assignability to/from `any` is always allowed and produces no diagnostics.
- `never` is optional; if added, use it for unreachable code only.
- Records are open by default. Missing-field diagnostics only apply to exact classes or `strict` mode.
- Function parameter arity is lenient: missing arguments are `nil`; extra arguments are allowed (PICO-8 ignores extras). Do not report arity errors by default.
- Optional parameters and `T?` are equivalent for call checking.
- Display types using LuaCATS syntax: `Player`, `Player?`, `number[]`, `table<string, number>`, `fun(x: number): boolean`.

## Inference pipeline

Run after the M2 bind pass, on the same cart snapshot.

### 1. Collect annotations and declarations

- Parse all `---@` blocks.
- Attach `@class`/`@alias`/`@enum` declarations to the cart’s type namespace.
- Attach `@param`/`@return`/`@type` to symbols.
- Report annotation diagnostics.

### 2. Create type variables

- Every unannotated local gets a fresh type variable.
- Every unannotated function parameter gets a fresh type variable, constrained by call sites.
- Every unannotated function gets a return tuple type variable.
- Globals get a type variable merged across assignments.
- Table literals start as records/arrays based on their fields.

### 3. Infer expressions and statements

Bottom-up abstract interpretation:

- Literals → primitive/literal types.
- Names → symbol type.
- Unary/binary operators → operator rules.
- Table constructors → records/arrays/maps; later field assignments add fields if the record is open.
- Function literals → function type with params/returns.
- Calls → callee return tuple; constrain arguments against params.
- Index/field → element/field type; unknown receiver → `any`.
- Varargs/multiple assignment → tuple destructuring.
- Assignments constrain the target type with the value type.
- `return` statements constrain the function’s return tuple.

### 4. Iterate for unannotated functions

- For unannotated parameters, union the argument types observed at all call sites in the cart.
- Iterate to a fixed point with a small cap (for example, 5 passes).
- If a parameter has no call sites or the union exceeds a threshold (for example, 8 members), fall back to `any`.
- If a function is recursive and unannotated, use its current return type; if unresolved, `any`.
- Never emit a diagnostic solely because a type stayed `any`.

### 5. Narrow types

Apply flow-sensitive narrowing within functions and blocks:

- `if x then` → in the then branch, remove `nil` and `false` from `x`.
- `if not x then` → else branch removes `nil`/`false`.
- `if x == nil then return end` → after the if, `x` is non-nil.
- `if x ~= nil then` → then branch non-nil.
- `type(x) == "number"` / `"string"` / `"boolean"` / `"table"` / `"function"` → narrow in the matching branch.
- `x and x.field` → `x` is non-nil in the right operand.
- `x or default` → result is non-nil if `default` is non-nil.
- Assignments reset narrowing for that variable from that point.
- Loops: do not attempt complex fixed-point narrowing; keep the declared/union type and narrow only within the current iteration body.
- `---@cast` and `--[[@as T]]` override the current type.
- Narrowing applies to locals and parameters; globals are flow-insensitive unless the function clearly owns them.

### 6. Check annotations

Emit diagnostics when:

- An annotated assignment receives an incompatible type.
- A call argument is incompatible with a parameter.
- A `return` value is incompatible with an annotated return.
- A field is accessed on an exact class and does not exist.
- A `nil` value is used where non-nil is required, only in `strict`.
- A `@param`/`@return` name is unknown.

Compatibility is structural for records and nominal for named classes, with open records allowing extra fields. `any` is compatible with everything. Unions match if any member is compatible.

## Operator rules

| Operator | Result |
|---|---|
| `+ - * / \ % ^` | `number` |
| Unary `-`, `~`, `@`, `%`, `$` | `number` |
| `..` | `string` (PICO-8 allows a number on the right; do not warn in `basic`) |
| `& | ^^ << >> >>> <<> >><` | `number` |
| Comparisons | `boolean` |
| `not` | `boolean` |
| `and` | Union of the falsy part of the left and the right type; simplify to `left | right` initially |
| `or` | Union of the truthy part of the left and the right type; simplify to `left | right` initially |
| `#` | `number` |

## PICO-8 collection special cases

Provide internal type signatures for:

- `all(t)` → iterator yielding the element type of `t`.
- `pairs(t)` → iterator yielding key/value types.
- `ipairs(t)` → iterator yielding index/element types.
- `foreach(t, f)` → `f` receives the element type of `t`.
- `add(t, v)` → `v` must be compatible with `t`’s element type.
- `del(t, v)` → same as `add`.
- `deli(t, i)` → index type.
- `count(t, [v])` → `number`.
- `pack(...)` → tuple table.
- `unpack(t, [i], [j])` → tuple of element types.
- `select(n, ...)` → vararg/tuple slice.

Implementation can special-case these names in the inference engine rather than requiring full generics. Keep the behavior documented and tested.

## Type-aware LSP features

### Completion

- After `.`/`:`, use the receiver type:
  - record/class → fields and methods, optional fields marked;
  - array → numeric index, not named fields;
  - map → no named fields;
  - `any`/unknown → empty member completion;
  - union → fields present on every member? Offer the union of fields; mark ambiguous fields in `detail`.
- After `self.`, use the method receiver type.
- Completion inside `---@param`/`---@return`/`---@type` offers known classes, aliases, and primitives.
- Deprecated fields/functions get the `Deprecated` tag.

### Hover

- Show the symbol’s declared/annotated type; if inferred, show the inferred type.
- Function signature shows optional parameters and return types.
- Class hover shows the class name and a compact field list.
- Field hover shows the receiver type and field type.
- If a type is a union, display a readable union, capped in length.
- If a type is `any`, say `any` and do not fabricate details.

### Signature help

- Use the function type’s parameter list.
- Mark optional parameters.
- Show parameter names, types, and docs.
- Active parameter is clamped; variadic tail allows any index.
- Overloads are not implemented in v1; if multiple function types are encountered, use the first and log.

### Inlay hints

- Config: `pico8.inlayHints` (default `false`).
- Show inferred type hints for:
  - `local x = ...` when `x` has no explicit annotation and the type is not `any`;
  - function parameters when inferred and not annotated (optional; start with locals only);
  - function return types if clear and not annotated (optional).
- Hint label: `: Type` for variables, `: (params) -> returns` for functions.
- `InlayHintKind.Type`; `paddingLeft: false`, `paddingRight: true` as appropriate.
- Do not show hints for `any`, for explicitly annotated symbols, or inside comments/strings.
- Respect the requested range; do not compute the whole cart for a small range if avoidable.

## Diagnostics and modes

### Modes

| Mode | Behavior |
|---|---|
| `off` | Parse annotations for docs/completion; no type diagnostics; no inference-based inlay hints. Explicit annotation types may still power hover/completion. |
| `basic` (default) | Infer and check explicit annotations; report assign/param/return mismatches and annotation errors. No strict nil checks; undefined fields only on exact classes. |
| `strict` | Adds `need-check-nil` and missing-field checks for open records. Explicit `(exact)` classes already warn in `basic` (M9). |

### Type diagnostics

| Code | `basic` | `strict` | Notes |
|---|---|---|---|
| `assign-type-mismatch` | Warning | Warning | Explicit annotation target vs value. |
| `param-type-mismatch` | Warning | Warning | Call argument vs annotated/ inferred parameter. |
| `return-type-mismatch` | Warning | Warning | Return vs annotated return. |
| `undefined-field` | Off (open records) / warning (exact) | Warning | Exact classes or strict. |
| `need-check-nil` | Off | Warning | Using `T?` where `T` is required. |
| `missing-fields` | Warning (explicit exact classes) | Warning | Table literal missing required fields. M9 extends this to explicit `(exact)` classes in `basic`. |
| `deprecated` | Hint | Hint | Use of `---@deprecated`. |
| `annotation-*` | Warning | Warning | See annotation table. |
| `type-unknown` | Hint/off | Hint/off | Type could not be inferred; avoid noise. |

Rules:

- Never emit a type diagnostic when either side is `any`.
- Default type diagnostics to warning/hint. Only emit error severity if the user explicitly raises a code in configuration.
- Suppress diagnostics covered by `---@diagnostic disable-next-line`, `disable-line`, `disable`, and `enable`.
- Suppress type diagnostics inside `---@meta` files and the built-in stub.
- Deduplicate diagnostics that arise from the same expression via multiple paths.
- Respect `pico8.maxProblems`.

## Architecture and files

```
internal/analysis/types/type.go        type model, canonicalization, display
internal/analysis/types/unify.go       compatibility, assignability, member lookup
internal/analysis/annotation_types.go  LuaCATS type grammar
internal/analysis/type_annotations.go  annotation collection, association, directives
internal/analysis/infer.go             inference, narrowing, and PICO-8 iterators
internal/analysis/type_diagnostics.go  diagnostic suppression
internal/lsp/inlay_hints.go            inferred local-type hints
```

## Error handling

- A malformed annotation is a local diagnostic; analysis continues with the annotation treated as absent.
- Type inference cycles terminate via a pass cap and unification occurs in a deterministic order.
- If the type graph becomes too large or a pass exceeds its budget, fall back to `any` for unresolved symbols and log once.
- Never panic on cyclic tables, recursive functions, or self-referential annotations.
- If `apiVersion` is unsupported, type analysis still runs against 0.2.7 with a warning.
- Type features must degrade gracefully if the M3 built-in stub failed to load.

## Testing

### Annotation tests

- Every supported annotation, including malformed forms.
- Type grammar: primitives, unions, optional, arrays, maps, records, functions, tuples, literals.
- Association: blank lines, docs, multiple annotations, `@field` with `@class`, `@param` matching, `@return` order, `@cast`, `--[[@as]]`.
- Case folding of type names.
- Deferred annotations parse without errors.

### Inference tests

- Literals, locals, globals, multiple assignment, functions, methods, tables, varargs, multiple returns.
- Call-site inference for unannotated params; recursion; no call sites → `any`.
- Table shape growth; array/map distinction; open vs exact.
- Iterators: `all`, `pairs`, `ipairs`, `foreach`, `add`, `del`, `pack`, `unpack`, `select`.
- Operators and coercion rules.
- Narrowing cases listed above.
- No false positives on the unannotated corpus in `basic` mode.

### LSP tests

- Type-aware completion for fields/methods, unions, `self`.
- Hover displays annotations and inferred types.
- Signature help with optional/variadic params.
- Inlay hints on/off, range-limited, no hints for `any`/annotated symbols.
- Diagnostics for assign/param/return/undefined-field/need-check-nil in each mode.
- `---@diagnostic` suppression.
- UTF-16 ranges for type diagnostics.

### Performance

- Inference on a 64 KB cart < 10 ms for `basic`, < 20 ms for `strict`.
- Completion/hover with types < 5 ms.
- No memory blow-up from unions; cap union members and canonicalize.

## Implementation notes

- Annotation and inference tests live with the implementation in
  `internal/analysis/*_test.go` and `internal/lsp/*_test.go` rather than
  separate `testdata/types` fixture trees.
- Inlay hints currently cover inferred local declarations only. Parameter and
  return hints remain optional follow-ups as described above.
- `@type` is attached to local declarations and assignment targets. Standalone
  expression statements are not annotated in v1.
- `@cast` is applied from its source line onward in the file. It does not yet
  reset at nested block boundaries.
- `--[[@as]]` is associated with the nearest same-line expression ending before
  the comment; nested subexpression selection is best-effort.
- Built-in functions receive type signatures from the 0.2.7 stub docs, with a
  small fallback return-type table for common PICO-8 functions. Calls to
  built-ins are not type-checked to keep unannotated carts quiet.
- Type checking defaults to `basic`; `strict` adds nil checks and
  missing-field warnings for open records. Explicit `(exact)` classes already
  warn about unknown and missing fields in `basic` (M9). All type diagnostics
  are warnings or hints.
- Performance on the 64 KB test cart is measured in
  `TestM4InferencePerformance` (basic and strict) and the existing M3
  completion/hover/semantic-token targets.

## Exit criteria

- [x] All required LuaCATS annotations parse and attach correctly.
- [x] Type model canonicalizes unions and displays LuaCATS syntax.
- [x] Basic inference covers locals, globals, functions, tables, operators, and PICO-8 iterators.
- [x] Explicit annotation mismatches produce warnings in `basic`.
- [x] `strict` adds nil/exact-field checks behind a setting.
- [x] Type-aware completion, hover, signature help, and inlay hints work.
- [x] `---@diagnostic` suppression works.
- [x] Unannotated corpus produces no type-error noise in `basic`.
- [x] Performance targets are met.
- [x] `go test -race ./...` passes.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Type inference noise erodes trust | Gradual defaults, `any` fallback, warnings only, corpus tests, easy disable. |
| Call-site inference overfits or explodes | Union cap, fixed-point cap, fallback to `any`, no diagnostics on unresolved. |
| Complex unions become unreadable | Canonicalize and cap display; use named aliases. |
| Annotation parser ambiguity | Dedicated annotation lexer/parser; golden tests; treat errors as absent. |
| Cyclic types/functions hang | Pass caps; visited sets; deterministic worklist. |
| Strict mode is too noisy | Keep it opt-in; ship only after corpus validation. |
| Type-aware member completion regresses M3 | Keep a fallback path to M3 member discovery when types are unknown. |
