-- Optional Neovim helper: attach pico-lsp to cart-owned .lua buffers.
--
-- Enable with:
--   vim.opt.runtimepath:append("/path/to/pico-lsp/editors/nvim")
--   require("pico_lsp").setup()
--
-- The helper never runs for .p8 carts (use vim.lsp.config/vim.lsp.enable for
-- those) and ignores .lua files that are not included by a nearby .p8 cart.

local M = {}

local default_settings = {
  pico8 = {
    apiVersion = "0.2.7",
    typeChecking = "basic",
    inlayHints = false,
  },
}

local warned_missing_binary = false

local function notify(message, level)
  vim.notify("pico-lsp: " .. message, level or vim.log.levels.WARN)
end

local function normalize(path)
  local absolute = vim.fn.fnamemodify(path, ":p")
  local ok, normalized = pcall(vim.fs.normalize, absolute)
  if ok then
    return normalized
  end
  return absolute
end

local function same_path(a, b)
  if vim.fn.has("win32") == 1 or vim.fn.has("mac") == 1 then
    return a:lower() == b:lower()
  end
  return a == b
end

local function read_lines(path)
  local ok, lines = pcall(vim.fn.readfile, path)
  if not ok then
    return nil
  end
  return lines
end

local function include_target(line)
  local rest = line:match("^%s*#include%s+(.+)$")
  if not rest then
    return nil
  end
  rest = vim.trim(rest)
  local quoted = rest:match('^"([^"]*)"') or rest:match("^'([^']*)'")
  if quoted then
    rest = quoted
  end
  rest = rest:gsub("\\", "/")
  -- .p8 includes can select a tab with a single hex-digit suffix.
  rest = rest:gsub(":%x$", "")
  return rest
end

local function cart_includes(cart, target)
  local lines = read_lines(cart)
  if not lines then
    return false
  end
  local directory = vim.fs.dirname(cart)
  for _, line in ipairs(lines) do
    local include = include_target(line)
    if include then
      local candidate = normalize(directory .. "/" .. include)
      if same_path(candidate, target) then
        return true
      end
    end
  end
  return false
end

local function find_cart(path)
  local directory = vim.fs.dirname(path)
  while directory do
    local carts = vim.fn.globpath(directory, "*.p8", false, true)
    table.sort(carts)
    for _, cart in ipairs(carts) do
      local normalized = normalize(cart)
      if not same_path(normalized, path) and cart_includes(normalized, path) then
        return normalized
      end
    end
    local parent = vim.fs.dirname(directory)
    if not parent or parent == directory then
      break
    end
    directory = parent
  end
  return nil
end

local function resolved_config(opts)
  local config
  local ok, value = pcall(function()
    return vim.lsp.config["pico_lsp"]
  end)
  if ok and type(value) == "table" then
    config = vim.deepcopy(value)
  else
    config = {}
  end
  config.name = "pico_lsp"
  config.cmd = opts.cmd or config.cmd or { "pico-lsp" }
  config.settings = vim.tbl_deep_extend("force", default_settings, config.settings or {}, opts.settings or {})
  return config
end

local function attach(bufnr)
  if M.disabled or not vim.api.nvim_buf_is_valid(bufnr) then
    return
  end
  if vim.bo[bufnr].filetype ~= "lua" then
    return
  end

  local name = vim.api.nvim_buf_get_name(bufnr)
  if name == "" or name:sub(-4):lower() ~= ".lua" then
    return
  end
  if #vim.lsp.get_clients({ name = "pico_lsp", bufnr = bufnr }) > 0 then
    return
  end

  local path = normalize(name)
  local cart = find_cart(path)
  if not cart then
    return
  end

  local config = resolved_config(M.opts)
  local cmd = config.cmd
  if type(cmd) == "table" and cmd[1] and vim.fn.executable(cmd[1]) == 0 then
    if not warned_missing_binary then
      warned_missing_binary = true
      notify("command not found: " .. cmd[1] .. " (set cmd in pico_lsp.setup)")
    end
    return
  end

  config.root_dir = vim.fs.dirname(cart)
  if not config.root_dir then
    return
  end

  local ok, err = pcall(vim.lsp.start, config, { bufnr = bufnr })
  if not ok then
    notify(tostring(err), vim.log.levels.ERROR)
  end
end

local function attach_loaded_lua_buffers()
  for _, bufnr in ipairs(vim.api.nvim_list_bufs()) do
    if vim.api.nvim_buf_is_loaded(bufnr) then
      attach(bufnr)
    end
  end
end

function M.setup(opts)
  opts = opts or {}
  M.opts = opts
  M.disabled = opts.disable == true or vim.g.pico_lsp_helper_disabled == 1

  if M.group then
    pcall(vim.api.nvim_del_augroup_by_id, M.group)
  end
  if M.disabled then
    return
  end

  M.group = vim.api.nvim_create_augroup("pico_lsp_helper", { clear = true })
  vim.api.nvim_create_autocmd("FileType", {
    group = M.group,
    pattern = "lua",
    callback = function(args)
      attach(args.buf)
    end,
  })
  vim.api.nvim_create_autocmd("BufEnter", {
    group = M.group,
    pattern = "*.lua",
    callback = function(args)
      attach(args.buf)
    end,
  })
  vim.api.nvim_create_autocmd("BufWritePost", {
    group = M.group,
    pattern = "*.p8",
    callback = attach_loaded_lua_buffers,
  })

  attach_loaded_lua_buffers()
end

return M
