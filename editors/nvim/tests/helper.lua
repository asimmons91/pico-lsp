-- Headless smoke test for the optional Neovim helper.
--
-- Run with editors/nvim/tests/run.sh after building pico-lsp:
--   PICO_LSP_BIN=/path/to/pico-lsp nvim --headless --clean -u NONE \
--     --cmd "set runtimepath+=$PWD/editors/nvim" -l editors/nvim/tests/helper.lua

local root = vim.env.PICO_LSP_ROOT or vim.fn.getcwd()
vim.opt.runtimepath:append(root .. "/editors/nvim")

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

vim.cmd.edit(vim.fn.fnameescape(cart))
if not vim.wait(5000, function()
  return #vim.lsp.get_clients({ name = "pico_lsp" }) > 0
end, 50) then
  fail("pico-lsp did not attach to the cart")
end

vim.cmd.edit(vim.fn.fnameescape(included))
if not vim.wait(5000, function()
  return #vim.lsp.get_clients({ name = "pico_lsp", bufnr = 0 }) > 0
end, 50) then
  fail("helper did not attach the included .lua buffer")
end

vim.cmd.edit(vim.fn.fnameescape(unrelated))
vim.wait(100, function()
  return true
end)
if #vim.lsp.get_clients({ name = "pico_lsp", bufnr = 0 }) > 0 then
  fail("helper attached an unrelated .lua buffer")
end

print("nvim helper smoke: ok")
vim.cmd("qa!")
