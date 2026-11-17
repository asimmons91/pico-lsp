# M10 — Type System II: Generics, Overloads, Operators, Unions

**Status:** Planned  
**Depends on:** M9  
**Release target:** v1.1.0 (batched with M7–M9)  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

The type system reaches `lua_ls`-class expressiveness for the common annotation
patterns used by larger PICO-8 projects, while keeping all new diagnostics
opt-in or low-noise. Generic containers, overloads, operator annotations, and
tagged-union narrowing work in hover, completion, signature help, and call
checking.

**Demo:** A `---@generic T` `Box<T>` preserves its element type through
construction and `get`; an overloaded `draw` resolves the matching signature;
`---@operator add(Vec): Vec` checks vector addition; and `kind == "enemy"`
narrows a tagged union to the enemy variant.

## Scope

### In scope

- Generics:
  - `---@generic T` on functions and classes;
  - generic parameters in fields, params, and returns;
  - call-site instantiation and inference;
  - `any` fallback when an argument cannot be inferred.
- Overloads:
  - `---@overload fun(...): ...`;
  - signature help cycling/selection;
  - best-match call checking with ambiguity falling back to no error.
- Operators:
  - `---@operator add(Other): Result` and the documented Lua/PICO-8 operator
    set;
  - operator hover/completion where applicable;
  - operator type checking for explicit annotations.
- Discriminated unions:
  - unions of records with a shared tag field of literal string/number type;
  - narrowing on `tag == "value"`, `tag ~= "value"`, `and`/`or`, and early
    returns;
  - tagged fields retain their literal type in hover.
- User-defined type guards only if a syntax can be adopted without inventing a
  PICO-8-specific annotation dialect; otherwise document as deferred.
- Hover, completion, signature help, inlay hints, and diagnostics for all new
  constructs.
- Configuration and diagnostic documentation updates.

### Out of scope

- Runtime generics, reflection, or executing generic code.
- Dependent types, higher-kinded types, or type-level computation.
- Full Lua 5.4 standard-library typing.
- API version selection and stub sets.
- Pull diagnostics.
- Custom type-guard syntax not already established by LuaCATS.

## Requirements

- Generic parameter names are scoped to their annotation and cannot leak into
  the surrounding type namespace.
- Generic inference must be deterministic and stable across file order.
- Overload selection must prefer exact/annotation matches and fall back to `any`
  instead of reporting ambiguity.
- Operator annotations must not affect unannotated code or PICO-8 built-ins.
- Discriminated-union narrowing must be scope-aware, branch-aware, and must not
  narrow mutable objects unsafely after mutation.
- New diagnostics default to `off` or `hint` until corpus and real-project
  validation; promotion to warning is a separate decision.
- Type display (`hover`, inlay hints, signature help) must remain readable;
  deeply nested generic types are truncated with a stable marker.
- Every feature composes with M9 inheritance and narrowing.

## Annotation syntax

Use established LuaCATS-style forms where possible:

```lua
---@generic T
---@param value T
---@return Box<T>
local function box(value) end

---@class Box<T>
---@field value T

---@overload fun(x: number): number
---@overload fun(x: string): string
local function identity(x) end

---@class Vec
---@operator add(Vec): Vec
---@operator sub(Vec): Vec

---@class Shape
---@field kind "circle"|"square"

---@class Circle: Shape
---@field kind "circle"
---@field radius number
```

- Do not add inline type syntax or Luau-style annotations.
- If a construct cannot be parsed unambiguously, report `annotation-invalid` and
  fall back to `any`.

## Testing

- Parser and resolver tests for generic parameters, defaults, nested
  instantiation, and shadowing.
- Generic inference tests across calls, assignments, table literals, and
  recursive functions.
- Overload tests for selection, ambiguity fallback, signature help, and
  diagnostics.
- Operator tests for every supported operator, including invalid operand types
  and built-in overrides.
- Tagged-union narrowing tests for equality, inequality, `and`/`or`, early
  returns, loops, and reassignment.
- Hover/completion/signature/inlay golden tests for readable type display.
- Corpus tests: unannotated carts remain clean; annotated fixtures cover every
  construct.
- LSP e2e tests for diagnostics, hover, completion, and signature help.
- Fuzz tests for annotation parsing and type resolution.
- Performance: full analysis of a 64 KB annotated cart stays under the v1
  budget; generic/overload resolution must be bounded.

## Exit criteria

- [ ] `---@generic` functions and classes instantiate and infer correctly.
- [ ] `---@overload` drives signature help and best-match checking with safe
      fallback.
- [ ] Operator annotations check explicit operator use and appear in hover.
- [ ] Tagged-union narrowing works with no scope leaks.
- [ ] Type display is readable and truncated deterministically.
- [ ] New diagnostics are documented, configurable where appropriate, and
      corpus-validated.
- [ ] Existing M9/default behavior and performance budgets do not regress.
- [ ] Tests, docs, and migration notes are updated.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Generic inference explodes in complexity | Start with functions/containers; bound recursion depth; fallback to `any`. |
| Overload ambiguity produces noise | Best-match with ambiguity fallback; diagnostics opt-in. |
| Operator annotations conflict with built-ins | Apply only to explicit user annotations; preserve PICO-8 API types. |
| Discriminated unions mis-narrow mutable values | Scope/branch-aware narrowing and reassignment invalidation tests. |
| Type display becomes unreadable | Deterministic truncation and display goldens. |
| Scope creep into type-system research | Keep to practical LuaCATS patterns; defer custom syntax unless clearly needed. |
