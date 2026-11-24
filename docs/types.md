# Types and LuaCATS annotations

Types are optional. Unannotated PICO-8 code should stay quiet; annotations make
hover, completion, signature help, and diagnostics more precise.

`pico-lsp` uses LuaCATS comments (`---@...`). It does not use Luau-style inline
types. The default mode is `basic`; see [Checking modes](#checking-modes).

## A first annotation

```lua
---@class player
---@field x number
---@field y number
---@field hp integer

---@param p player
---@param dx number
---@param dy number
---@return boolean moved
function move(p,dx,dy)
 p.x+=dx
 p.y+=dy
 return dx!=0 or dy!=0
end

---@type player
local p={x=64,y=64,hp=3}

if (btn(0)) move(p,-1,0)
```

Hovering `move` shows `function move(p: player, dx: number, dy: number): boolean`.
Completion after `p.` offers `x`, `y`, and `hp`.

## Common annotations

| Annotation | Use |
|---|---|
| `---@param name Type` | Parameter type. `name?` marks an optional parameter. |
| `---@return Type` | Return type. Repeat for multiple returns. |
| `---@type Type` | Type for the next local, assignment, or expression. |
| `---@class Name` | Named table class. Add fields with `---@field`. |
| `---@field name Type` | Field on the preceding class. `name?` makes it optional. |
| `---@alias Name Type` | Named alias, including unions such as `circle\|square`. |
| `---@enum Name` | Enum-like table; literal values become members. |
| `---@deprecated` | Marks a symbol deprecated in completion and tokens. |
| `---@diagnostic state: code` | Disables or enables a diagnostic for a line or file. |

## Classes, inheritance, and exact fields

Classes are open by default: an object may have extra fields. Mark a class
`(exact)` when unknown fields should be rejected.

```lua
---@class entity
---@field x number
---@field y number

---@class player: entity
---@field hp integer
---@field sprite integer

---@class (exact) settings
---@field music boolean
---@field sfx boolean

---@type settings
local s={music=true,sfx=true}
```

In `basic` mode an exact-class table literal with a missing field warns, and an
unknown field warns. In `strict` mode open records also get missing-field checks.

## Optional fields and nil

Add `?` to a field or parameter name to make it optional. Optional fields are
typed as `Type|nil`.

```lua
---@class save_state
---@field slot? integer
---@field name? string

---@param state save_state
function load_save(state)
 if (state.slot!=nil) then
  printh("slot "..state.slot)
 end
end
```

## Generics

Use `---@generic` to preserve a type through a helper.

```lua
---@class box<T>
---@field value T

---@generic T
---@param value T
---@return box<T>
function box(value)
 return {value=value}
end

local health=box(3)
local title=box("player")
```

Hovering `health.value` shows `number`; `title.value` shows `string`.

## Overloads

Use `---@overload` when one function accepts different argument shapes. The
best match drives signature help and checking; ambiguous calls fall back safely
instead of reporting a confident wrong error.

```lua
---@overload fun(x: number): number
---@overload fun(x: string): string
function identity(x)
 return x
end
```

## Operator annotations

Use `---@operator` to describe arithmetic on a class.

```lua
---@class vec
---@field x number
---@field y number
---@operator add(vec): vec
---@operator sub(vec): vec

---@param a vec
---@param b vec
---@return vec
function add(a,b)
 return {x=a.x+b.x,y=a.y+b.y}
end
```

Operator annotations accept the standard LuaCATS names plus `rotl`/`rotr` for
PICO-8 rotations. See the
[M10 annotation syntax](M10-type-system-2.md#annotation-syntax) for the complete
list.

## Tagged unions and narrowing

Use string-literal fields to distinguish variants. Control-flow checks narrow
the type inside the branch.

```lua
---@class circle
---@field kind "circle"
---@field radius number

---@class square
---@field kind "square"
---@field size number

---@alias shape circle|square

---@param s shape
---@return number
function area(s)
 if (s.kind=="circle") then
  return 3.14159*s.radius*s.radius
 else
  return s.size*s.size
 end
end
```

## Aliases, enums, and casts

```lua
---@alias direction 0|1|2|3

---@enum buttons
buttons={left=0,right=1,up=2,down=3}

---@param d direction
---@return direction
function next_direction(d)
 if (d==0) return 1
 if (d==1) return 2
 if (d==2) return 3
 return 0
end

---@param value any
---@return direction
function as_direction(value)
 ---@cast value direction
 return value
end
```

`--[[@as Type]]` forces an expression type when inference cannot see the intent:

```lua
---@class level_data
---@field tiles table

local raw={tiles={}}
local data=raw --[[@as level_data]]
```

## Checking modes

Set the mode with `pico8.typeChecking`:

| Mode | Behavior |
|---|---|
| `off` | Parse annotations for hover and completion; do not report type diagnostics. |
| `basic` | Check explicit annotations, class inheritance, exact classes, generics, overloads, and tagged unions. This is the default. |
| `strict` | Add `need-check-nil` and missing-field checks for open records. |

Which mode should you use?

- Start with `basic`. It catches annotation mistakes without turning dynamic Lua
  into a wall of errors.
- Use `strict` for new code or a library you want to keep tight.
- Use `off` when you only want editor intelligence and no type diagnostics.

Every mode keeps the server forgiving: type diagnostics do not block editing,
and the server prefers `any` or silence over a confident wrong error.

## When to use `any`

Dynamic tables and PICO-8 metatable tricks can defeat static analysis. Use
`any` when the real shape is only known at runtime, or when an annotation would
be more misleading than helpful.

```lua
---@type any
local state={}
```

## Related documentation

- [Configuration](configuration.md): `pico8.typeChecking` and related settings.
- [Diagnostics](diagnostics.md): every type diagnostic code and severity.
- [API](api.md): the `pico8/cartInfo` request and editor commands.
- [M4 type-system spec](M4-type-system.md), [M9](M9-type-system-1.md), and
  [M10](M10-type-system-2.md): implementation details.
