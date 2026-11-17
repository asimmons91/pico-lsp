pico-8 cartridge // pico-8.com
version 42
__lua__
-- M9 annotated fixture: inheritance, exact classes, metatables, narrowing.
---@class Entity
---@field x number
---@field y number
---@field hp number
local Entity = {}

---@param dx number
---@param dy number
function Entity:move(dx, dy)
  self.x += dx
  self.y += dy
end

---@class Enemy: Entity
---@field damage number
local Enemy = {}

---@class (exact) Point
---@field x number
---@field y number
---@field label? string

---@type Point
local origin = {x = 0, y = 0}

---@param p Point
---@return number
local function point_x(p)
  return p.x
end

local enemy = setmetatable({x = 1, y = 2, hp = 3, damage = 4}, {__index = Enemy})
enemy:move(1, 1)

---@param target Entity?
local function aim(target)
  assert(target)
  local x = target.x
  if not target then return x end
  local result = target or enemy
  return result.x
end

local info = {Health = 1}
local folded = info.health
local exact = info["health"]

function _draw()
  cls()
  print(point_x(origin), folded + exact, aim(enemy))
end
