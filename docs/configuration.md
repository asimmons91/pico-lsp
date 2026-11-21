# Configuration

pico-lsp reads configuration from the `pico8` section using
`workspace/configuration` when the client supports pulls, falling back to
`initializationOptions.pico8` and `workspace/didChangeConfiguration`.

For installation and editor setup, see [install.md](install.md),
[../editors/nvim/README.md](../editors/nvim/README.md), and
[../editors/helix/README.md](../editors/helix/README.md).

```json
{
  "pico8": {
    "apiVersion": "0.2.7",
    "typeChecking": "basic",
    "inlayHints": false,
    "maxProblems": 100,
    "limits": {
      "codeLens": true,
      "diagnostics": true
    },
    "format": {
      "maxBlankLines": 2
    },
    "diagnostics": {
      "mode": "auto",
      "undefinedGlobal": "warning",
      "unusedLocal": "off",
      "unreachableCode": "hint",
      "duplicateTableField": "hint",
      "callbackPrecedence": "hint",
      "callbackParams": "warning",
      "noStdlib": "hint",
      "shadowedLocal": "off",
      "emptyBlock": "off",
      "globalInFunction": "off",
      "includeOutsideWorkspace": "warning",
      "includeUnusualType": "hint",
      "includeDuplicate": "off"
    },
    "lint": {
      "apiRanges": false
    },
    "refactors": {
      "enabled": true
    },
    "glyphs": [
      { "name": "logo", "byte": 128, "description": "custom font slot 0x80" },
      { "name": "my_font", "text": "\\14", "context": "string" }
    ]
  }
}
```

## Settings

| Setting | Values | Default | Meaning |
|---|---|---|---|
| `apiVersion` | `0.2.7` | `0.2.7` | PICO-8 API version. |
| `typeChecking` | `off`, `basic`, `strict` | `basic` | Type-diagnostic mode. |
| `inlayHints` | boolean | `false` | Show inferred local type hints. |
| `maxProblems` | integer | `100` | Per-document diagnostic cap; `0` disables truncation. |
| `diagnostics.mode` | `auto`, `push`, `pull` | `auto` | Diagnostic transport. `auto` pulls when the client supports it and pushes otherwise. |
| `limits.codeLens` | boolean | `true` | Show the cart budget code lens. |
| `limits.diagnostics` | boolean | `true` | Publish token, character, and compressed-size limit diagnostics. |
| `format.maxBlankLines` | integer | `2` | Maximum consecutive blank lines; `0` removes blank lines. |
| `lint.apiRanges` | boolean | `false` | Optional literal API range checks. |
| `refactors.enabled` | boolean | `true` | Offer the extract, rewrite, add-missing-fields, and remove-unused code actions. |
| `glyphs` | array | `[]` | Custom P8SCII glyph snippets for `p8_` completion and the glyph picker. |

## P8SCII glyph snippets

`pico8.glyphs` adds custom names for P8SCII characters. Each entry has a
`name` and exactly one of `byte`, `glyph`, or `text`:

```json
{
  "pico8": {
    "glyphs": [
      { "name": "logo", "byte": 128, "description": "custom font slot 0x80" },
      { "name": "my_heart", "glyph": "♥", "aliases": ["p8-love"] },
      { "name": "my_font", "text": "\\14", "context": "string" }
    ]
  }
}
```

- `byte` is authoritative for custom font slots and must be a printable P8SCII
  value from `0x10` through `0xff`. The server inserts the canonical Unicode
  spelling for that byte. A custom font changes how PICO-8 draws the byte, not
  which byte is stored in the cart.
- `glyph` is accepted only when it encodes to exactly one P8SCII byte. Use the
  variation-selector spelling for button glyphs, for example `⬅️`, not a bare
  `⬅`.
- `text` is a P8SCII control escape such as `\14`, `\15`, `\*`, or `\#`.
  Text entries must set `"context": "string"` and are offered only inside
  strings.
- `name` is ASCII and becomes the default trigger `p8_<name>`. `aliases` add
  full `p8_`/`p8-` triggers such as `p8-love`.
- Custom names and aliases override built-in entries with a logged warning.
  Invalid entries are skipped. Omitting `glyphs` preserves the current list; an
  explicit empty array clears custom entries.

Type `p8_left` (or `p8-left`) in code or a string and accept the completion to
insert the glyph. Inside strings, `p8_font` offers the string-only control
entries. The VS Code command **PICO-8: Insert P8SCII Glyph** and the Neovim
`:Pico8InsertGlyph` command open a searchable picker; both commands can also be
called with a glyph name to insert it directly. Control entries are not offered
outside strings.

The server cannot infer custom font pixels from cart code. Declare custom font
slots with `byte` entries and a description so collaborators know what the
placeholder represents.

Compressed-size budgeting applies to `.p8` carts, including decoded `.p8.png`
virtual carts. The server reports the size PICO-8's pxa compressor would store
for the assembled code, warns from 14054 bytes (90% of the 15616-byte limit),
and reports an error above 15616 bytes. Standalone `.lua` files have no
compressed budget. If compression cannot produce a bounded result, the value is
reported as unavailable and no compressed-size diagnostic is published; token
and character budgets are unaffected.

`basic` checks explicit annotations, class inheritance, inherited members,
`(exact)` class literals, metatable factories, generics, overloads, operator
annotations, tagged unions, and flow narrowing. `strict` adds `need-check-nil`
and missing-field checks for open records. `off` keeps explicit annotation
types for hover and completion but publishes no type diagnostics.

Diagnostic severity values are `off`, `hint`, `info`, `warning`, and `error`.
Set a rule to `off` to disable it. Syntax errors and missing includes remain
errors and cannot be disabled globally.

`diagnostics.mode` selects the transport. In `auto` the server answers
`textDocument/diagnostic` when the client advertises pull support and otherwise
publishes diagnostics. `push` always publishes while still answering pull
requests. `pull` suppresses duplicate push publication; if the client does not
support pull diagnostics the server logs one warning and falls back to push.
Switching transports clears or republishes diagnostics so the editor never
keeps a stale set from the other mode. `pico8.maxProblems` applies per original
document in both transports.

Settings changes are applied without restarting the server. Open carts are
re-analyzed and diagnostics are republished.

## VS Code settings

The VS Code extension contributes the full `pico8` schema. Every server setting
documented above appears in **Settings** and in `settings.json` with its type,
allowed values, default, and description. VS Code validates edits against the
schema; for example, `pico8.typeChecking` accepts only `off`, `basic`, and
`strict`, and the nested `limits`, `format`, `diagnostics`, `lint`, and
`refactors` objects reject unknown keys.

Server settings use `window` scope because the server pulls one workspace-level
`pico8` section rather than per-resource settings. `pico8.serverPath` remains
`machine`-scoped because it names an executable on the machine that runs the
extension host. The extension forwards the same section through
`initializationOptions.pico8` and the server's `workspace/configuration` pull,
and other LSP clients can set the same keys.

`pico8.serverPath` is not version-checked before startup. After the server
starts, the extension reads the version from the LSP `initializeResult` and
shows one non-blocking warning when a release server version differs from the
extension version. The warning never stops the server or replaces a custom
binary: a custom-path mismatch offers **Show Server Info**, **Open Settings**,
and **Show Log**, while a cached or downloaded mismatch offers **Clear Server
Cache**, **Show Server Info**, and **Show Log**. Development versions (`dev`,
pre-release versions, and Go pseudo-versions) are labeled “development build”
and do not warn. The comparison is an exact normalized `X.Y.Z` check, not a
semver compatibility range. Use **PICO-8: Copy Server Diagnostics** to copy the
versions, source, path, platform, cache root, workspace-folder count, Lua
attachment state, and last startup error when present, without copying source
text or cart contents.

The VS Code client also reads four extension-owned settings. They are not sent
to the language server as behavior settings.

| Setting | Type | Default | Scope | Meaning |
|---|---|---|---|---|
| `pico8.serverPath` | string | `""` | machine | Absolute path to a `pico-lsp` executable. Disables automatic download. |
| `pico8.autoDownload` | boolean | `true` | window | Download the checksummed server matching the extension version when it is not cached. |
| `pico8.attachLua` | boolean | `true` | window | Attach to all `.lua` files in workspace folders that contain a `.p8` cart. |
| `pico8.trace.server` | `off`, `messages`, `verbose` | `off` | window | LSP trace level in the **PICO-8 Language Server Trace** channel. |

All other `pico8.*` settings are server settings documented above, including
`pico8.glyphs`. Defaults shown in the Settings UI match the server's
`internal/config` defaults; a Go drift test fails if the extension manifest and
server configuration diverge.

See [diagnostics.md](diagnostics.md) for the complete code list and
[api.md](api.md) for `pico8/cartInfo` and `pico8.showCartInfo`.
