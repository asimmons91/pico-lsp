# M9 — Type System I: Inheritance, Exact Classes, Metatables, Narrowing

**Status:** Planned  
**Depends on:** M8  
**Release target:** v1.1.0 (batched with M7, M8, M10)  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

Annotated PICO-8 projects gain practical object-oriented typing and flow-aware nil
handling without increasing default noise. Class hierarchies, exact classes,
common metatable-based factories, and everyday guard patterns are understood by
hover, completion, signature help, and diagnostics.

**Demo:** A `---@class Enemy: Entity` value resolves inherited fields and methods;
a `---@class (exact) Point` reports missing fields in a literal; a
`setmetatable({}, { __index = Point })` factory is treated as a `Point`; and
`assert(p)` / `if not p then return end` narrow `p` to non-nil for the rest of
the scope.

## Scope

### In scope

- Class inheritance:
  - `---@class Child: Parent`;
  - field and method lookup through the parent chain;
  - structural compatibility with parent classes;
  - assignment/parameter/return checks against inherited types;
  - cycle detection reported as an annotation error.
- Exact classes:
  - `---@class (exact) Name`;
  - missing/extra-field diagnostics for explicit exact class literals;
  - inherited fields counted as present;
  - diagnostics default to warning only when the class is explicitly exact.
- Metatable inference:
  - `setmetatable({}, { __index = Class })`;
  - `setmetatable(obj, Class)` where `Class` is an annotated class;
  - common `__index` function/factory patterns when the receiver is an explicit
    class;
  - keep `any` fallback when the pattern is ambiguous.
- Better narrowing:
  - `assert(x)`;
  - `x = x or default`;
  - `if not x then return end` and equivalent early-return guards;
  - `x and x.field` / `x or default` chains;
  - composition with existing `---@cast` and `type(x)` narrowing.
- Hover, completion, signature help, inlay hints, and diagnostics consume the
  new type information.
- Configuration and diagnostics documentation updates.

### Out of scope

- Generics, overloads, operators, and discriminated unions (M10).
- Runtime metatable emulation or executing cart code.
- Dynamic metatable changes that cannot be inferred statically.
- Type guards with new custom syntax.
- Exact-class checking for unannotated tables by default.
- API version selection.

## Requirements

- Extend the LuaCATS annotation parser and type model without changing v1
  annotations.
- Preserve `any` fallback and low-noise defaults. New diagnostics default to
  `warning` or `hint` only where explicit annotations make the intent clear.
- Field lookup must respect the existing PICO-8 case-folding rules for
  identifiers while keeping string keys case-sensitive.
- Inherited fields must appear in completion and hover with their declaring
  class noted.
- Class cycles and unknown parent classes produce a clear annotation diagnostic
  and do not crash or recurse indefinitely.
- Exact-class field checks must not fire on tables that are not explicitly
  annotated with `(exact)`.
- Metatable inference must be conservative: if the class or `__index` target is
  ambiguous, keep `any` rather than guessing.
- Narrowing must be scope-aware and must not leak assumptions across branches or
  function boundaries.
- No changes to `off`/`basic`/`strict` contract beyond the new explicit
  constructs. Existing v1 type diagnostics keep their defaults.

## Diagnostics

Reuse existing codes where possible:

| Code | New behavior |
|---|---|
| `annotation-invalid` | Unknown parent class, invalid `(exact)` placement, class cycles. |
| `assign-type-mismatch` | Parent/child compatibility and exact class assignments. |
| `param-type-mismatch` | Inherited parameter expectations. |
| `return-type-mismatch` | Inherited return expectations. |
| `undefined-field` | Field missing from a class and its parents. |
| `missing-fields` | Exact class literal missing inherited/declared fields. |
| `need-check-nil` | Existing nil checks extended to narrowed values. |

If a new code is unavoidable, it must be documented and default to hint/warning.

## Testing

- Annotation parser tests for inheritance, exact classes, malformed parents, and
  cycles.
- Type resolution tests for multi-level inheritance, overridden fields/methods,
  optional fields, and aliases.
- Metatable inference tests for common factories, ambiguous cases, and
  `__index` functions.
- Narrowing tests for `assert`, `or`, early returns, nested scopes, loops, and
  reassignment.
- Hover/completion/signature/inlay tests for inherited members and narrowed
  values.
- Corpus validation: no new diagnostics on unannotated carts; annotated fixture
  carts exercise every new construct.
- LSP e2e tests for diagnostics and member completion.
- Golden tests for type display.
- Performance: full analysis of a 64 KB annotated cart remains under the v1
  budget.

## Exit criteria

- [ ] Inheritance lookup and compatibility work across multiple levels.
- [ ] Exact classes produce the intended missing/extra-field diagnostics.
- [ ] Common metatable factories infer the annotated class.
- [ ] The listed narrowing patterns work without leaking across scopes.
- [ ] Hover, completion, signature help, and inlay hints use inherited/narrowed
      types.
- [ ] New diagnostics are documented, configurable where appropriate, and
      corpus-validated.
- [ ] Existing v1 defaults and performance budgets do not regress.
- [ ] Tests and docs are updated.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Inheritance cycles or recursive lookup | Cycle detection with clear annotation diagnostics and depth limits. |
| Exact classes create false positives | Only explicit `(exact)` classes; inherited fields count; warnings default. |
| Metatable inference guesses wrong | Conservative pattern matching with `any` fallback. |
| Narrowing leaks across branches | Scope-aware environments and branch merge tests. |
| Corpus noise | Run annotated and unannotated corpus fixtures; default new checks off/hint until proven. |
| Type complexity slows analysis | Benchmark; keep lookup maps and avoid repeated graph walks. |
