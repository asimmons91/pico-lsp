pico-8 cartridge // pico-8.com
version 42
__lua__
#include "lib/player.lua"
function _init()
  player = make_player()
  player:update()
end
-->8 second
#include "lib/util.lua"
function helper()
  print(player)
  return util()
end
