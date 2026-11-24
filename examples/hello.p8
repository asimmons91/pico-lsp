pico-8 cartridge // http://www.pico-8.com
version 42
__lua__
-- hello.p8 -- a small cart for trying pico-lsp
-- Move the circle with the arrow keys.

function _init()
 x=64
 y=64
end

function _update()
 if (btn(0)) x-=1
 if (btn(1)) x+=1
 if (btn(2)) y-=1
 if (btn(3)) y+=1
end

function _draw()
 cls(1)
 circfill(x,y,4,10)
 print("hello pico-lsp",2,2,7)
end
__gfx__
00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000
