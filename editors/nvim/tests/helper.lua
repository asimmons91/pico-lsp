-- Headless smoke test for the optional Neovim helper.
--
-- Run with editors/nvim/tests/run.sh after building pico-lsp:
--   PICO_LSP_BIN=/path/to/pico-lsp nvim --headless --clean -u NONE \
--     --cmd "set runtimepath+=$PWD/editors/nvim" -l editors/nvim/tests/helper.lua

local root = vim.env.PICO_LSP_ROOT or vim.fn.getcwd()
vim.opt.runtimepath:append(root .. "/editors/nvim")
vim.cmd("filetype plugin on")

local binary = vim.env.PICO_LSP_BIN or "pico-lsp"
local tmp = vim.fn.tempname()
vim.fn.mkdir(tmp, "p")

local cart = tmp .. "/cart.p8"
local included = tmp .. "/lib.lua"
local unrelated = tmp .. "/other.lua"
vim.fn.writefile({
  "pico-8 cartridge // pico-8.com",
  "version 42",
  "__lua__",
  '#include "lib.lua"',
  "x = 1",
}, cart)
vim.fn.writefile({ "function lib() end" }, included)
vim.fn.writefile({ "print('unrelated')" }, unrelated)

vim.filetype.add({ extension = { p8 = "pico8" } })
vim.lsp.config("pico_lsp", {
  cmd = { binary },
  filetypes = { "pico8" },
  root_dir = tmp,
})
vim.lsp.enable("pico_lsp")
require("pico_lsp").setup()

local function fail(message)
  vim.api.nvim_err_writeln("nvim helper smoke: " .. message)
  vim.cmd("cquit 1")
end

local default_probe = vim.api.nvim_create_buf(false, true)
vim.bo[default_probe].filetype = "pico8"
local default_mapping = vim.api.nvim_buf_call(default_probe, function()
  return vim.fn.maparg("U", "i", false, true)
end)
if next(default_mapping) ~= nil then
  fail("pico8 glyph maps were installed without opt-in")
end
vim.api.nvim_buf_delete(default_probe, { force = true })

vim.g.pico8_glyph_maps = 1

vim.cmd.edit(vim.fn.fnameescape(cart))
if not vim.wait(5000, function()
  return #vim.lsp.get_clients({ name = "pico_lsp" }) > 0
end, 50) then
  fail("pico-lsp did not attach to the cart")
end

if vim.bo.commentstring ~= "-- %s" then
  fail("ftplugin did not set commentstring")
end
if vim.bo.comments ~= ":---,:--,://" then
  fail("ftplugin did not set comments")
end

local expected_glyphs = {
  U = "⬆️",
  D = "⬇️",
  L = "⬅️",
  R = "➡️",
  O = "🅾️",
  X = "❎",
}
for key, glyph in pairs(expected_glyphs) do
  local mapping = vim.fn.maparg(key, "i", false, true)
  if next(mapping) == nil or mapping.buffer ~= 1 or mapping.rhs ~= glyph then
    fail("ftplugin did not install the " .. key .. " glyph mapping")
  end
end

vim.api.nvim_buf_set_lines(0, -1, -1, false, { "" })
vim.api.nvim_win_set_cursor(0, { vim.api.nvim_buf_line_count(0), 0 })
local esc = vim.api.nvim_replace_termcodes("<Esc>", true, false, true)
vim.api.nvim_feedkeys("iUDLROX" .. esc, "xt", false)
local inserted = vim.api.nvim_buf_get_lines(0, -2, -1, false)[1]
if inserted ~= "⬆️⬇️⬅️➡️🅾️❎" then
  fail("ftplugin glyph mappings inserted " .. vim.inspect(inserted))
end

local function check_wrap(leader, message)
  local probe = vim.api.nvim_create_buf(false, true)
  vim.bo[probe].filetype = "pico8"
  vim.bo[probe].formatoptions = "tcq"
  vim.bo[probe].textwidth = 30
  vim.api.nvim_buf_set_lines(probe, 0, -1, false, {
    leader .. " this is a long comment line that should wrap onto another comment line",
  })
  vim.api.nvim_buf_call(probe, function()
    vim.cmd("normal! gqG")
  end)
  local wrapped = vim.api.nvim_buf_get_lines(probe, 0, -1, false)
  if #wrapped < 2 then
    fail("ftplugin comment formatting did not wrap the " .. leader .. " comment")
  end
  for _, line in ipairs(wrapped) do
    if line:sub(1, #leader + 1) ~= leader .. " " then
      fail("ftplugin comment formatting did not continue the " .. leader .. " prefix: " .. line)
    end
  end
  vim.api.nvim_buf_delete(probe, { force = true })
end

check_wrap("--", "dash")
check_wrap("//", "slash")

vim.cmd.edit(vim.fn.fnameescape(included))
if not vim.wait(5000, function()
  return #vim.lsp.get_clients({ name = "pico_lsp", bufnr = 0 }) > 0
end, 50) then
  fail("helper did not attach the included .lua buffer")
end

vim.api.nvim_win_set_cursor(0, { 1, #vim.api.nvim_buf_get_lines(0, 0, 1, false)[1] })
vim.cmd("Pico8InsertGlyph left")
if not vim.wait(5000, function()
  local text = table.concat(vim.api.nvim_buf_get_lines(0, 0, -1, false), "\n")
  return text:find("⬅️", 1, true) ~= nil
end, 50) then
  fail("Pico8InsertGlyph did not insert the left glyph")
end

vim.cmd.edit(vim.fn.fnameescape(unrelated))
vim.wait(100, function()
  return true
end)
if #vim.lsp.get_clients({ name = "pico_lsp", bufnr = 0 }) > 0 then
  fail("helper attached an unrelated .lua buffer")
end

local png_fixture = root .. "/testdata/p8png/smoke.p8.png"
if vim.fn.filereadable(png_fixture) == 1 then
  vim.cmd("Pico8OpenPng " .. vim.fn.fnameescape(png_fixture))
  if not vim.wait(5000, function()
    return vim.api.nvim_buf_get_name(0):match("^pico8%-cart:") ~= nil
      and #vim.lsp.get_clients({ name = "pico_lsp", bufnr = 0 }) > 0
  end, 50) then
    fail("Pico8OpenPng did not open an attached virtual buffer")
  end
  if vim.bo.modifiable then
    fail("Pico8OpenPng opened a modifiable buffer")
  end
  vim.cmd("Pico8ReloadPng")
  if vim.api.nvim_buf_get_name(0):match("^pico8%-cart:") == nil then
    fail("Pico8ReloadPng replaced the virtual buffer")
  end
end

print("nvim helper smoke: ok")
vim.cmd("qa!")
