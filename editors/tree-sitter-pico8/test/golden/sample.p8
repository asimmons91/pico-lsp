pico-8 cartridge // pico-8.com
version 42
__lua__
-- dash comment
// slash comment
x = 0x1f + 0b101 + .5
if (x > 1) x += 1 else x = 2
while (x < 3) x += 1
? "hello", x
y = @0x5f00 + $0x5f02 + %0x5f04
z = 7 \ 2
-->8 second
function tick(dt)
  return dt
end
__gfx__
0000000000000000
__label__
hello
