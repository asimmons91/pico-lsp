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
---@type number
btn_left = 0
--- Right direction button index for btn()/btnp().
---@type number
btn_right = 1
--- Up direction button index for btn()/btnp().
---@type number
btn_up = 2
--- Down direction button index for btn()/btnp().
---@type number
btn_down = 3
--- O button index for btn()/btnp().
---@type number
btn_o = 4
--- X button index for btn()/btnp().
---@type number
btn_x = 5

-- PICO-8 also accepts the button glyphs directly in source. Both the bare
-- character and the variation-selector form are accepted by the editor.
--- Left button glyph.
---@type number
⬅ = 0
⬅️ = 0
--- Right button glyph.
---@type number
➡ = 1
➡️ = 1
--- Up button glyph.
---@type number
⬆ = 2
⬆️ = 2
--- Down button glyph.
---@type number
⬇ = 3
⬇️ = 3
--- O button glyph.
---@type number
🅾 = 4
🅾️ = 4
--- X button glyph.
---@type number
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
---@param filename string
---@param breadcrumb? string
---@param param_str? string
function load(filename, breadcrumb, param_str) end

--- Save the current cart to a filename.
---@param filename string
function save(filename) end

--- Open the host's cart folder.
function folder() end

--- List cart files in a directory, or return the current directory listing.
---@param directory? string
function ls(directory) end

--- Run the cart from the start, optionally with a parameter string.
---@param param_str? string
function run(param_str) end

--- Stop the cart and optionally print a message.
---@param message? string
function stop(message) end

--- Resume a stopped cart.
function resume() end

--- Assert that a condition is true, stopping the cart otherwise.
---@param condition boolean
---@param message? string
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
---@param str string
---@param filename? string
---@param overwrite? boolean
---@param save_to_desktop? boolean
function printh(str, filename, overwrite, save_to_desktop) end

--- Return seconds elapsed since the cart started.
function time() end

--- Alias for time().
function t() end

--- Read a system status value.
---@param x number
function stat(x) end

--- Run a special system command.
---@param cmd_str string
---@param p1? number
---@param p2? number
function extcmd(cmd_str, p1, p2) end

-- Graphics -----------------------------------------------------------------

--- Set the clipping rectangle.
---@param x number
---@param y number
---@param w number
---@param h number
---@param clip_previous? boolean
function clip(x, y, w, h, clip_previous) end

--- Set a pixel.
---@param x number
---@param y number
---@param col? number
function pset(x, y, col) end

--- Read a pixel colour.
---@param x number
---@param y number
function pget(x, y) end

--- Read a sprite-sheet pixel colour.
---@param x number
---@param y number
function sget(x, y) end

--- Write a sprite-sheet pixel colour.
---@param x number
---@param y number
---@param col? number
function sset(x, y, col) end

--- Read sprite flags.
---@param n number
---@param f? number
function fget(n, f) end

--- Write sprite flags.
---@param n number
---@param f? number
---@param val number
function fset(n, f, val) end

--- Print a string at an optional position and colour.
---@param str any
---@param x? number
---@param y? number
---@param col? number
function print(str, x, y, col) end

--- Set the print cursor position.
---@param x number
---@param y number
---@param col? number
function cursor(x, y, col) end

--- Set or read the draw colour.
---@param col? number
function color(col) end

--- Clear the screen.
---@param col? number
function cls(col) end

--- Set the screen camera offset.
---@param x? number
---@param y? number
function camera(x, y) end

--- Draw a circle outline.
---@param x number
---@param y number
---@param r number
---@param col? number
function circ(x, y, r, col) end

--- Draw a filled circle.
---@param x number
---@param y number
---@param r number
---@param col? number
function circfill(x, y, r, col) end

--- Draw an oval outline.
---@param x0 number
---@param y0 number
---@param x1 number
---@param y1 number
---@param col? number
function oval(x0, y0, x1, y1, col) end

--- Draw a filled oval.
---@param x0 number
---@param y0 number
---@param x1 number
---@param y1 number
---@param col? number
function ovalfill(x0, y0, x1, y1, col) end

--- Draw a line.
---@param x0 number
---@param y0 number
---@param x1? number
---@param y1? number
---@param col? number
function line(x0, y0, x1, y1, col) end

--- Draw a rectangle outline.
---@param x0 number
---@param y0 number
---@param x1 number
---@param y1 number
---@param col? number
function rect(x0, y0, x1, y1, col) end

--- Draw a filled rectangle.
---@param x0 number
---@param y0 number
---@param x1 number
---@param y1 number
---@param col? number
function rectfill(x0, y0, x1, y1, col) end

--- Draw a rounded rectangle outline.
---@param x number
---@param y number
---@param w number
---@param h number
---@param r number
---@param col? number
function rrect(x, y, w, h, r, col) end

--- Draw a filled rounded rectangle.
---@param x number
---@param y number
---@param w number
---@param h number
---@param r number
---@param col? number
function rrectfill(x, y, w, h, r, col) end

--- Remap colours in a draw, display, or secondary palette.
---@param c0 number
---@param c1 number
---@param p? number
function pal(c0, c1, p) end

--- Set colour transparency for sprite drawing.
---@param c number
---@param t? number
function palt(c, t) end

--- Draw a sprite.
---@param n number
---@param x number
---@param y number
---@param w? number
---@param h? number
---@param flip_x? boolean
---@param flip_y? boolean
function spr(n, x, y, w, h, flip_x, flip_y) end

--- Draw a scaled or cropped sprite.
---@param sx number
---@param sy number
---@param sw number
---@param sh number
---@param dx number
---@param dy number
---@param dw? number
---@param dh? number
---@param flip_x? boolean
---@param flip_y? boolean
function sspr(sx, sy, sw, sh, dx, dy, dw, dh, flip_x, flip_y) end

--- Set the fill pattern.
---@param p number
function fillp(p) end

-- Tables -------------------------------------------------------------------

--- Append a value to a sequence.
---@param tbl table
---@param val any
---@param index? number
function add(tbl, val, index) end

--- Remove the first matching value from a sequence.
---@param tbl table
---@param val any
function del(tbl, val) end

--- Remove a value at an index from a sequence.
---@param tbl table
---@param i? number
function deli(tbl, i) end

--- Count matching values in a table.
---@param tbl table
---@param val? any
function count(tbl, val) end

--- Iterate over sequence values.
---@param tbl table
function all(tbl) end

--- Call a function for every table value.
---@param tbl table
---@param func function
function foreach(tbl, func) end

--- Iterate over every key and value in a table.
---@param tbl table
function pairs(tbl) end

--- Iterate over sequence indices and values.
---@param tbl table
function ipairs(tbl) end

--- Return the next key and value in a table.
---@param tbl table
---@param key? any
function next(tbl, key) end

--- Pack arguments into a table.
---@param ... any
function pack(...) end

--- Unpack sequence values into multiple return values.
---@param tbl table
---@param i? number
---@param j? number
function unpack(tbl, i, j) end

-- Input --------------------------------------------------------------------

--- Read a button state.
---@param b? number
---@param pl? number
function btn(b, pl) end

--- Read whether a button was newly pressed.
---@param b number
---@param pl? number
function btnp(b, pl) end

-- Audio --------------------------------------------------------------------

--- Play a sound effect.
---@param n number
---@param channel? number
---@param offset? number
---@param length? number
function sfx(n, channel, offset, length) end

--- Play or stop music.
---@param n number
---@param fade_len? number
---@param channel_mask? number
function music(n, fade_len, channel_mask) end

-- Map ----------------------------------------------------------------------

--- Read a map tile.
---@param x number
---@param y number
function mget(x, y) end

--- Write a map tile.
---@param x number
---@param y number
---@param val number
function mset(x, y, val) end

--- Draw map tiles.
---@param tile_x number
---@param tile_y number
---@param sx? number
---@param sy? number
---@param tile_w? number
---@param tile_h? number
---@param layers? number
function map(tile_x, tile_y, sx, sy, tile_w, tile_h, layers) end

--- Draw a textured line from map tiles.
---@param x0 number
---@param y0 number
---@param x1 number
---@param y1 number
---@param mx number
---@param my number
---@param mdx? number
---@param mdy? number
---@param layers? number
function tline(x0, y0, x1, y1, mx, my, mdx, mdy, layers) end

-- Memory -------------------------------------------------------------------

--- Read bytes from memory.
---@param addr number
---@param n? number
function peek(addr, n) end

--- Write bytes to memory.
---@param addr number
---@param val1 number
---@param val2? number
function poke(addr, val1, val2) end

--- Read a 16-bit value from memory.
---@param addr number
function peek2(addr) end

--- Write a 16-bit value to memory.
---@param addr number
---@param val number
function poke2(addr, val) end

--- Read a 32-bit value from memory.
---@param addr number
function peek4(addr) end

--- Write a 32-bit value to memory.
---@param addr number
---@param val number
function poke4(addr, val) end

--- Copy a memory range.
---@param dest_addr number
---@param source_addr number
---@param len number
function memcpy(dest_addr, source_addr, len) end

--- Load data from a cart file into memory.
---@param dest_addr number
---@param source_addr number
---@param len number
---@param filename? string
function reload(dest_addr, source_addr, len, filename) end

--- Store memory into a cart file.
---@param dest_addr number
---@param source_addr number
---@param len number
---@param filename? string
function cstore(dest_addr, source_addr, len, filename) end

--- Fill a memory range with a byte value.
---@param dest_addr number
---@param val number
---@param len number
function memset(dest_addr, val, len) end

--- Send or receive serial data.
---@param channel number
---@param address number
---@param length number
function serial(channel, address, length) end

-- Math ---------------------------------------------------------------------

--- Return the larger of two values.
---@param x number
---@param y number
function max(x, y) end

--- Return the smaller of two values.
---@param x number
---@param y number
function min(x, y) end

--- Return the middle of three values.
---@param x number
---@param y number
---@param z number
function mid(x, y, z) end

--- Round down to an integer.
---@param x number
function flr(x) end

--- Round up to an integer.
---@param x number
function ceil(x) end

--- Return the cosine of an angle.
---@param x number
function cos(x) end

--- Return the sine of an angle.
---@param x number
function sin(x) end

--- Return the angle for a direction vector.
---@param dx number
---@param dy number
function atan2(dx, dy) end

--- Return the square root of a value.
---@param x number
function sqrt(x) end

--- Return the absolute value.
---@param x number
function abs(x) end

--- Return a random number.
---@param x number
function rnd(x) end

--- Set the random seed.
---@param x number
function srand(x) end

--- Return the sign of a value.
---@param x number
function sgn(x) end

-- Bitwise ------------------------------------------------------------------

--- Bitwise AND.
---@param x number
---@param y number
function band(x, y) end

--- Bitwise OR.
---@param x number
---@param y number
function bor(x, y) end

--- Bitwise XOR.
---@param x number
---@param y number
function bxor(x, y) end

--- Bitwise NOT.
---@param x number
function bnot(x) end

--- Shift bits left.
---@param x number
---@param n number
function shl(x, n) end

--- Arithmetic shift bits right.
---@param x number
---@param n number
function shr(x, n) end

--- Logical shift bits right.
---@param x number
---@param n number
function lshr(x, n) end

--- Rotate bits left.
---@param x number
---@param n number
function rotl(x, n) end

--- Rotate bits right.
---@param x number
---@param n number
function rotr(x, n) end

-- Menu ---------------------------------------------------------------------

--- Configure a custom pause-menu item.
---@param index number
---@param label? string
---@param callback? function
function menuitem(index, label, callback) end

-- Strings and type conversion ---------------------------------------------

--- Convert a value to a string.
---@param val any
---@param format_flags? string
function tostr(val, format_flags) end

--- Convert a value to a number.
---@param val any
---@param format_flags? string
function tonum(val, format_flags) end

--- Return the string for one or more character codes.
---@param val0 number
---@param val1? number
function chr(val0, val1) end

--- Return character codes from a string.
---@param str string
---@param index? number
---@param num_results? number
function ord(str, index, num_results) end

--- Return a substring.
---@param str string
---@param pos0 number
---@param pos1? number
function sub(str, pos0, pos1) end

--- Split a string into a table.
---@param str string
---@param separator? string
---@param convert_numbers? boolean
function split(str, separator, convert_numbers) end

--- Return the type of a value.
---@param val any
function type(val) end

--- Convert a value to a string using Lua's tostring semantics.
---@param val any
function tostring(val) end

--- Convert a value to a number using Lua's tonumber semantics.
---@param val any
---@param base? number
function tonumber(val, base) end

-- Cart data ----------------------------------------------------------------

--- Open the persistent cart-data area.
---@param id string
function cartdata(id) end

--- Read a persistent cart-data value.
---@param index number
function dget(index) end

--- Write a persistent cart-data value.
---@param index number
---@param value number
function dset(index, value) end

-- Metatables ---------------------------------------------------------------

--- Set a table's metatable.
---@param tbl table
---@param m table
function setmetatable(tbl, m) end

--- Return a table's metatable.
---@param tbl table
function getmetatable(tbl) end

--- Set a table field without invoking metamethods.
---@param tbl table
---@param key any
---@param value any
function rawset(tbl, key, value) end

--- Read a table field without invoking metamethods.
---@param tbl table
---@param key any
function rawget(tbl, key) end

--- Compare two values without invoking metamethods.
---@param tbl1 any
---@param tbl2 any
function rawequal(tbl1, tbl2) end

--- Return the raw length of a table.
---@param tbl table
function rawlen(tbl) end

-- Coroutines ---------------------------------------------------------------

--- Create a coroutine from a function.
---@param func function
function cocreate(func) end

--- Resume a coroutine.
---@param c thread
---@param p0? any
---@param p1? any
function coresume(c, p0, p1) end

--- Return a coroutine's status.
---@param c number
function costatus(c) end

--- Yield from a coroutine.
---@param ... number
function yield(...) end

-- Lua base ----------------------------------------------------------------

--- Return selected arguments or the number of arguments.
---@param index number
---@param ... number
function select(index, ...) end
