# M10 — Type System II: Generics, Overloads, Operators, Unions

**Status:** Done  
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

- [x] `---@generic` functions and classes instantiate and infer correctly.
- [x] `---@overload` drives signature help and best-match checking with safe
      fallback.
- [x] Operator annotations check explicit operator use and appear in hover.
- [x] Tagged-union narrowing works with no scope leaks.
- [x] Type display is readable and truncated deterministically.
- [x] New diagnostics are documented, configurable where appropriate, and
      corpus-validated.
- [x] Existing M9/default behavior and performance budgets do not regress.
- [x] Tests, docs, and migration notes are updated.

## Implementation notes

- Generic parameters are scoped `*types.GenericParam` nodes. Class parameters
  live on the class record; function parameters live on the function type. The
  annotation parser resolves names through a declaration-local resolver, so
  names from a class or function cannot leak into global type completion.
- Generic classes are instantiated as specialized record copies with
  substituted fields, parents, and operators. Nested instantiations
  (`Box<Box<number>>`), generic parent classes, and trailing defaults are
  supported. The template record remains the named type for `---@type Box`.
- Generic functions bind parameters from argument types at each call and
  substitute parameter and return types. Unbound parameters use their declared
  default and then `any`. Recursive or ambiguous bindings are bounded and do not
  produce diagnostics.
- Overloads are stored beside the primary signature. Candidate selection scores
  arity and assignability, prefers exact matches, and returns `any` without a
  diagnostic when candidates tie or none fit. Signature help exposes every
  candidate and chooses the active signature from the recorded argument types;
  clients can cycle the list.
- Operator annotations accept the standard LuaCATS names plus `rotl`/`rotr` for
  PICO-8 rotations. Binary, unary, comparison, and `call` operators substitute
  generic parameters, inherit through class parents, and appear in class hover.
  Operand mismatches use the `operator-type-mismatch` hint and are suppressible.
- Tagged-union narrowing matches a dot field against string or number literals
  in `==`/`~=`/`!=` conditions and composes with `and`, `or`, early returns, and
  loop conditions. Narrowing is dropped after a field assignment to the same
  receiver. Unrelated or non-literal tags are not narrowed.
- User-defined type guards are deferred: no established LuaCATS/PICO-8 syntax
  can be adopted without introducing a project-specific dialect.
- `TestM9AnnotatedInferencePerformance` now includes generic, overload, operator,
  and tagged-union constructs in the 64 KB annotated timing cart.

## Migration notes

- No configuration changes are required. Existing carts keep their current
  behavior; generics, overloads, and operators only affect explicitly
  annotated code.
- `---@generic`, `---@overload`, and `---@operator` are no longer ignored.
  Malformed declarations now report `annotation-invalid`.
- `operator-type-mismatch` is a hint and can be suppressed with
  `---@diagnostic` like other type diagnostics.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Generic inference explodes in complexity | Start with functions/containers; bound recursion depth; fallback to `any`. |
| Overload ambiguity produces noise | Best-match with ambiguity fallback; diagnostics opt-in. |
| Operator annotations conflict with built-ins | Apply only to explicit user annotations; preserve PICO-8 API types. |
| Discriminated unions mis-narrow mutable values | Scope/branch-aware narrowing and reassignment invalidation tests. |
| Type display becomes unreadable | Deterministic truncation and display goldens. |
| Scope creep into type-system research | Keep to practical LuaCATS patterns; defer custom syntax unless clearly needed. |
