pico-8 cartridge // pico-8.com
version 42
__lua__
-- M10 annotated fixture: generics, overloads, operators, tagged unions.
---@class Box<T>
---@field value T

---@generic T
---@param value T
---@return Box<T>
local function box(value)
  return {value = value}
end

---@overload fun(x: number): number
---@overload fun(x: string): string
local function pick(x)
  return x
end

---@class Vec
---@field x number
---@operator add(Vec): Vec
---@operator unm(): Vec
local Vec = {}

---@class Shape
---@field kind "circle" | "square"
---@class Circle: Shape
---@field kind "circle"
---@field radius number
---@class Square: Shape
---@field kind "square"
---@field side number

function _init()
  local boxed = box(1)
  local picked = pick("x")
  local a = setmetatable({x = 1}, {__index = Vec})
  local b = setmetatable({x = 2}, {__index = Vec})
  local sum = a + b
  local negative = -sum
  ---@type Circle | Square
  local shape = {}
  if shape.kind == "circle" then
    print(boxed.value, picked, negative.x, shape.radius)
  end
end
