# Troubleshooting

## The server does not attach

1. Confirm the binary is on `PATH` and is the version you expect:

   ```sh
   command -v pico-lsp
   pico-lsp --version
   ```

2. Confirm the editor recognizes the file type:
   - Neovim: `:set filetype?` should print `filetype=pico8` for a `.p8` file.
   - Helix: `hx --health pico8` should list `pico-lsp` without an error.

3. Check the editor's LSP log:
   - Neovim: `:lua =vim.lsp.log.get_filename()`
   - Helix: usually `~/.cache/helix/helix.log`, or pass `--log /tmp/helix.log`.

4. Start the server manually with debug logging and a known log path:

   ```sh
   pico-lsp --log-level debug --log-file /tmp/pico-lsp.log
   ```

   The server waits for LSP messages on stdin. A normal editor connection should
   log `pico-lsp started`, followed by `initialized` and document sync events.

## The server starts but no language features appear

- Confirm the editor sent `initialize` and `initialized`. Features are disabled
  until initialization completes.
- Confirm the file is a text `.p8` cart or a `.lua` file. For `.p8.png`, use
  `:Pico8OpenPng` in Neovim or `pico-lsp decode` to open the decoded virtual
  text; opening the binary directly is not supported.
- If the file is outside the workspace, some include features use the cart
  directory as the workspace boundary. Open the project root or configure
  workspace folders.
- For included `.lua` files, open the owning `.p8` cart or enable the Neovim
  helper. Helix attaches by file type; it does not ship a cart-ownership helper
  in v1.

## Included `.lua` files are not attached

The server analyzes includes whenever the owning `.p8` cart is attached, even
when the included file is closed. Separate editor buffers need their own LSP
attachment.

- Neovim: use the optional helper in `editors/nvim/lua/pico_lsp/`, or attach
  manually with `vim.lsp.start` from the cart root.
- Helix: open the owning cart. Automatic attachment to cart-owned `.lua` buffers
  is not configured by the v1 Helix snippet.
- Avoid running `lua_ls` and `pico-lsp` on the same cart-owned `.lua` buffer.
  Two Lua language servers will publish competing diagnostics.

## Configuration is not applied

The server prefers `workspace/configuration` when the client advertises support,
then falls back to `initializationOptions`, then
`workspace/didChangeConfiguration`. Fields returned by a configuration pull
override initialization values field by field.

- Confirm the section name is `pico8`.
- Confirm your editor actually implements `workspace/configuration`; otherwise
  pass the same table as `init_options`/`initializationOptions`.
- Check the server log after the `initialized` notification for configuration
  pull failures.
- Severity values are `off`, `hint`, `info`, `warning`, and `error`. Invalid
  values fall back to the documented default.

## Unicode, P8SCII, and UTF-16 positions

The server defaults to UTF-16 positions and negotiates UTF-8 when the client
advertises it. In Neovim, check the attached client's offset encoding:

```vim
:lua =vim.lsp.get_clients({ bufnr = 0 })[1].offset_encoding
```

PICO-8 glyphs can use base codepoints plus U+FE0F variation selectors. The
server's token and character counters follow PICO-8/shrinko8 semantics, not Go
byte length. If a diagnostic lands on the wrong column, include the server
version, editor version, offset encoding, and a minimal cart in the bug report.

## P8SCII glyph input

Type `p8_left`, `p8_heart`, or another glyph trigger in code or a string and
accept the completion to insert the canonical P8SCII glyph. The `p8-` form
(`p8-left`, `p8-x-key`, and pico8-ls-compatible aliases) is also accepted.
`p8-` aliases parse as subtraction if completion is dismissed, so prefer `p8_`
unless compatibility matters.

Control-code entries such as `p8_font_custom` (`\14`) are offered only inside
strings. Use the VS Code command **PICO-8: Insert P8SCII Glyph** or Neovim
`:Pico8InsertGlyph` to browse the catalog, or pass a glyph name to either
command to insert it directly.

Custom font slots are declared in `pico8.glyphs` by P8SCII byte. The server
inserts the canonical Unicode spelling for that byte; it cannot inspect runtime
font data at `0x5600` or know the custom pixels. See
[configuration.md](configuration.md#p8scii-glyph-snippets).

## Multiple carts and include ownership

A `.lua` file belongs to the nearest ancestor `.p8` cart that includes it. If
several carts in the same directory include the file, the server chooses
deterministically and logs the others at debug level. Rename or move carts to
make ownership unambiguous.

## Cart budgets and compressed size

Token, character, and compressed-size budgets describe the assembled cart after
include expansion and tab joining. Compressed size is cart-wide, uses PICO-8's
pxa stream format, and counts each P8SCII glyph as one byte. It is not the byte
size of the `.p8` file on disk.

- Standalone `.lua` files report `compressedStatus: "unavailable"`; PICO-8 only
  applies the compressed limit when saving a cart.
- Oversized or pathological code can also report unavailable instead of a
  misleading limit error. Token and character diagnostics continue to work.
- A `.p8` text cart may exceed the compressed limit; PICO-8 only refuses to save
  that code as `.p8.png` or `.p8.rom`.
- Use `pico8.limits.diagnostics` and `pico8.limits.codeLens` to turn the budget
  UI off independently.

## `.p8.png` carts

Read-only PNG carts open as decoded virtual documents. In Neovim, use
`:Pico8OpenPng path/to/cart.p8.png`; the buffer is intentionally not modifiable.
Use `:Pico8ReloadPng` after replacing the PNG. Helix and other editors can use
`pico-lsp decode --output /tmp/cart.p8 path/to/cart.p8.png` and open the decoded
text file.

Editing a PNG cart is not supported; export a text `.p8` from PICO-8 when you
need to modify it. Corrupt, truncated, or non-standard PNG images are reported
as decode errors.

## Formatter and code actions

`textDocument/formatting` is supported for `.p8` and `.lua` documents. It
refuses documents with syntax errors, missing/unreadable includes, and
read-only `.p8.png` carts. The formatter preserves PICO-8 shorthand and all
non-code cart sections; it does not wrap long lines or reorder statements.

Code actions cover generated type annotations, diagnostic suppression, and
creating missing `.lua` includes. A “Create missing include file” action is only
offered for `.lua` paths inside the workspace boundary.

M14 refactors add extract-to-local-function, safe global-to-local conversion,
generated `---@class` blocks, `if`/`while` shorthand conversion,
exact-class missing-field inserts, and unused-local removal. Refactors are
deliberately conservative: they refuse selections that cross functions,
control-flow boundaries, comments, multi-value calls, or include ordering. No
action is offered when the editor buffer differs from the analyzed snapshot.
Remove-unused is applied through the `pico8.removeUnused` command so the editor
can ask for confirmation; an impure initializer (such as a function call) blocks
removal. Disable all M14 actions with `pico8.refactors.enabled: false`; see
[api.md](api.md) for the action kinds.

## VS Code extension

The VS Code client downloads the server version matching its own version from
the GitHub release and verifies SHA-256. If first-run download fails, check the
proxy settings (`http.proxy` and `http.proxyStrictSSL`), configure
`pico8.serverPath`, or use **PICO-8: Clear Downloaded Server Cache** before
retrying. Use **PICO-8: Show Language Server Output** for download and process
errors.

The extension attaches to `.p8` files and to all `.lua` files in workspace
folders that contain a `.p8` cart. Disable another Lua language server for those
folders to avoid duplicate diagnostics. `.lua` files outside PICO-8 workspaces
are intentionally not attached.

The VS Code client supports local desktop windows only. Remote extension hosts,
web extension hosts, and read-only `.p8.png` virtual documents are not supported
in the current release. Marketplace and Open VSX publishing are follow-ups;
install the VSIX from the GitHub release.

### VS Code settings validation

The extension contributes the full `pico8` schema, so the Settings UI shows the
defaults the server uses and `settings.json` gets completion and validation. A
value outside a contributed enum, such as `"pico8.typeChecking": "stric"`, is
flagged by VS Code before the server sees it. The server still falls back to its
default if an invalid value arrives from another client.

To compare effective defaults, search for `pico8` in Settings and hover a
setting to see its default, or open **Preferences: Open Default Settings (JSON)**
and search for `pico8`. If a setting is accepted by VS Code but appears to have
no effect, confirm it is a documented server setting and check the **PICO-8
Language Server** output channel for a configuration warning. Server settings
are window-scoped; `pico8.serverPath` is the only machine-scoped setting.

## Logs and stdout

Routine logs go to stderr or the `--log-file` path. The server never writes to
stdout except LSP protocol frames. If an editor reports malformed LSP messages,
check whether another tool is writing to the same stdout stream or wrapping the
server command.

Logs may contain source excerpts only when trace logging is explicitly enabled.
Review a log before attaching it to a public issue.

## Release download warnings

Release binaries are unsigned. Verify `checksums.txt` before overriding
Gatekeeper or SmartScreen warnings. Instructions are in
[docs/install.md](install.md).

## Reporting a bug

Include:

- `pico-lsp --version`;
- editor and version;
- operating system and architecture;
- a minimal `.p8` cart or `.lua` include that reproduces the problem;
- the relevant log lines from `--log-file`;
- whether the issue occurs with the default configuration.

The server has no telemetry and does not collect data automatically.
