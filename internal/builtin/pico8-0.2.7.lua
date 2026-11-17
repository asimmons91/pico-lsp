-- PICO-8 0.2.7 built-in API stub.
--
-- API names and signatures are transcribed from the PICO-8 0.2.7 manual
-- (https://www.lexaloffle.com/dl/docs/pico-8_manual.html). The one-line
-- descriptions are original summaries written for pico-lsp; they are not
-- copied from the manual or from another language server. PICO-8 is a product
-- of Lexaloffle Games.
--
-- This file is parsed by pico-lsp's own PICO-8 lexer/parser. It must stay in
-- the PICO-8 dialect: no Lua standard library modules, no `require`, and no
-- `---@meta` section other than the marker below.
---@meta

-- Constants ----------------------------------------------------------------

--- Left direction button index for btn()/btnp().
btn_left = 0
--- Right direction button index for btn()/btnp().
btn_right = 1
--- Up direction button index for btn()/btnp().
btn_up = 2
--- Down direction button index for btn()/btnp().
btn_down = 3
--- O button index for btn()/btnp().
btn_o = 4
--- X button index for btn()/btnp().
btn_x = 5

-- PICO-8 also accepts the button glyphs directly in source. Both the bare
-- character and the variation-selector form are accepted by the editor.
--- Left button glyph.
⬅ = 0
⬅️ = 0
--- Right button glyph.
➡ = 1
➡️ = 1
--- Up button glyph.
⬆ = 2
⬆️ = 2
--- Down button glyph.
⬇ = 3
⬇️ = 3
--- O button glyph.
🅾 = 4
🅾️ = 4
--- X button glyph.
❎ = 5

-- Lifecycle callbacks ------------------------------------------------------

--- Called once when the cart starts, before the first update.
function _init() end

--- Called once per frame at 30 fps.
function _update() end

--- Called once per frame at 60 fps instead of _update().
function _update60() end

--- Called once per frame to draw the screen.
function _draw() end

-- System -------------------------------------------------------------------

--- Load a cart by filename or BBS id.
function load(filename, breadcrumb, param_str) end

--- Save the current cart to a filename.
function save(filename) end

--- Open the host's cart folder.
function folder() end

--- List cart files in a directory, or return the current directory listing.
function ls(directory) end

--- Run the cart from the start, optionally with a parameter string.
function run(param_str) end

--- Stop the cart and optionally print a message.
function stop(message) end

--- Resume a stopped cart.
function resume() end

--- Assert that a condition is true, stopping the cart otherwise.
function assert(condition, message) end

--- Reboot PICO-8.
function reboot() end

--- Reset draw state and related runtime values.
function reset() end

--- Print cart size and token information.
function info() end

--- Flip the back buffer to the screen and wait for the next frame.
function flip() end

--- Print a string to the host console or a file.
function printh(str, filename, overwrite, save_to_desktop) end

--- Return seconds elapsed since the cart started.
function time() end

--- Alias for time().
function t() end

--- Read a system status value.
function stat(x) end

--- Run a special system command.
function extcmd(cmd_str, p1, p2) end

-- Graphics -----------------------------------------------------------------

--- Set the clipping rectangle.
function clip(x, y, w, h, clip_previous) end

--- Set a pixel.
function pset(x, y, col) end

--- Read a pixel colour.
function pget(x, y) end

--- Read a sprite-sheet pixel colour.
function sget(x, y) end

--- Write a sprite-sheet pixel colour.
function sset(x, y, col) end

--- Read sprite flags.
function fget(n, f) end

--- Write sprite flags.
function fset(n, f, val) end

--- Print a string at an optional position and colour.
function print(str, x, y, col) end

--- Set the print cursor position.
function cursor(x, y, col) end

--- Set or read the draw colour.
function color(col) end

--- Clear the screen.
function cls(col) end

--- Set the screen camera offset.
function camera(x, y) end

--- Draw a circle outline.
function circ(x, y, r, col) end

--- Draw a filled circle.
function circfill(x, y, r, col) end

--- Draw an oval outline.
function oval(x0, y0, x1, y1, col) end

--- Draw a filled oval.
function ovalfill(x0, y0, x1, y1, col) end

--- Draw a line.
function line(x0, y0, x1, y1, col) end

--- Draw a rectangle outline.
function rect(x0, y0, x1, y1, col) end

--- Draw a filled rectangle.
function rectfill(x0, y0, x1, y1, col) end

--- Draw a rounded rectangle outline.
function rrect(x, y, w, h, r, col) end

--- Draw a filled rounded rectangle.
function rrectfill(x, y, w, h, r, col) end

--- Remap colours in a draw, display, or secondary palette.
function pal(c0, c1, p) end

--- Set colour transparency for sprite drawing.
function palt(c, t) end

--- Draw a sprite.
function spr(n, x, y, w, h, flip_x, flip_y) end

--- Draw a scaled or cropped sprite.
function sspr(sx, sy, sw, sh, dx, dy, dw, dh, flip_x, flip_y) end

--- Set the fill pattern.
function fillp(p) end

-- Tables -------------------------------------------------------------------

--- Append a value to a sequence.
function add(tbl, val, index) end

--- Remove the first matching value from a sequence.
function del(tbl, val) end

--- Remove a value at an index from a sequence.
function deli(tbl, i) end

--- Count matching values in a table.
function count(tbl, val) end

--- Iterate over sequence values.
function all(tbl) end

--- Call a function for every table value.
function foreach(tbl, func) end

--- Iterate over every key and value in a table.
function pairs(tbl) end

--- Iterate over sequence indices and values.
function ipairs(tbl) end

--- Return the next key and value in a table.
function next(tbl, key) end

--- Pack arguments into a table.
function pack(...) end

--- Unpack sequence values into multiple return values.
function unpack(tbl, i, j) end

-- Input --------------------------------------------------------------------

--- Read a button state.
function btn(b, pl) end

--- Read whether a button was newly pressed.
function btnp(b, pl) end

-- Audio --------------------------------------------------------------------

--- Play a sound effect.
function sfx(n, channel, offset, length) end

--- Play or stop music.
function music(n, fade_len, channel_mask) end

-- Map ----------------------------------------------------------------------

--- Read a map tile.
function mget(x, y) end

--- Write a map tile.
function mset(x, y, val) end

--- Draw map tiles.
function map(tile_x, tile_y, sx, sy, tile_w, tile_h, layers) end

--- Draw a textured line from map tiles.
function tline(x0, y0, x1, y1, mx, my, mdx, mdy, layers) end

-- Memory -------------------------------------------------------------------

--- Read bytes from memory.
function peek(addr, n) end

--- Write bytes to memory.
function poke(addr, val1, val2) end

--- Read a 16-bit value from memory.
function peek2(addr) end

--- Write a 16-bit value to memory.
function poke2(addr, val) end

--- Read a 32-bit value from memory.
function peek4(addr) end

--- Write a 32-bit value to memory.
function poke4(addr, val) end

--- Copy a memory range.
function memcpy(dest_addr, source_addr, len) end

--- Load data from a cart file into memory.
function reload(dest_addr, source_addr, len, filename) end

--- Store memory into a cart file.
function cstore(dest_addr, source_addr, len, filename) end

--- Fill a memory range with a byte value.
function memset(dest_addr, val, len) end

--- Send or receive serial data.
function serial(channel, address, length) end

-- Math ---------------------------------------------------------------------

--- Return the larger of two values.
function max(x, y) end

--- Return the smaller of two values.
function min(x, y) end

--- Return the middle of three values.
function mid(x, y, z) end

--- Round down to an integer.
function flr(x) end

--- Round up to an integer.
function ceil(x) end

--- Return the cosine of an angle.
function cos(x) end

--- Return the sine of an angle.
function sin(x) end

--- Return the angle for a direction vector.
function atan2(dx, dy) end

--- Return the square root of a value.
function sqrt(x) end

--- Return the absolute value.
function abs(x) end

--- Return a random number.
function rnd(x) end

--- Set the random seed.
function srand(x) end

--- Return the sign of a value.
function sgn(x) end

-- Bitwise ------------------------------------------------------------------

--- Bitwise AND.
function band(x, y) end

--- Bitwise OR.
function bor(x, y) end

--- Bitwise XOR.
function bxor(x, y) end

--- Bitwise NOT.
function bnot(x) end

--- Shift bits left.
function shl(x, n) end

--- Arithmetic shift bits right.
function shr(x, n) end

--- Logical shift bits right.
function lshr(x, n) end

--- Rotate bits left.
function rotl(x, n) end

--- Rotate bits right.
function rotr(x, n) end

-- Menu ---------------------------------------------------------------------

--- Configure a custom pause-menu item.
function menuitem(index, label, callback) end

-- Strings and type conversion ---------------------------------------------

--- Convert a value to a string.
function tostr(val, format_flags) end

--- Convert a value to a number.
function tonum(val, format_flags) end

--- Return the string for one or more character codes.
function chr(val0, val1) end

--- Return character codes from a string.
function ord(str, index, num_results) end

--- Return a substring.
function sub(str, pos0, pos1) end

--- Split a string into a table.
function split(str, separator, convert_numbers) end

--- Return the type of a value.
function type(val) end

--- Convert a value to a string using Lua's tostring semantics.
function tostring(val) end

--- Convert a value to a number using Lua's tonumber semantics.
function tonumber(val, base) end

-- Cart data ----------------------------------------------------------------

--- Open the persistent cart-data area.
function cartdata(id) end

--- Read a persistent cart-data value.
function dget(index) end

--- Write a persistent cart-data value.
function dset(index, value) end

-- Metatables ---------------------------------------------------------------

--- Set a table's metatable.
function setmetatable(tbl, m) end

--- Return a table's metatable.
function getmetatable(tbl) end

--- Set a table field without invoking metamethods.
function rawset(tbl, key, value) end

--- Read a table field without invoking metamethods.
function rawget(tbl, key) end

--- Compare two values without invoking metamethods.
function rawequal(tbl1, tbl2) end

--- Return the raw length of a table.
function rawlen(tbl) end

-- Coroutines ---------------------------------------------------------------

--- Create a coroutine from a function.
function cocreate(func) end

--- Resume a coroutine.
function coresume(c, p0, p1) end

--- Return a coroutine's status.
function costatus(c) end

--- Yield from a coroutine.
function yield(...) end

-- Lua base ----------------------------------------------------------------

--- Return selected arguments or the number of arguments.
function select(index, ...) end
