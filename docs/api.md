# PICO-8 LSP API

pico-lsp exposes PICO-8-specific entry points in addition to the standard LSP
methods.

## `pico8/cartInfo`

Custom request for statusline and tooling integrations.

```json
{
  "textDocument": { "uri": "file:///path/to/cart.p8" }
}
```

The request accepts the root cart, an included file, a `.p8.png` source URI, or
an open `pico8-cart:/...` virtual document. Included files resolve to their
owning cart.

The result:

```json
{
  "rootUri": "file:///path/to/cart.p8",
  "tokens": 4210,
  "tokenLimit": 8192,
  "chars": 27433,
  "charLimit": 65535,
  "compressed": 12000,
  "compressedLimit": 15616,
  "compressedWarning": 14054,
  "compressedStatus": "ok",
  "files": [
    { "uri": "file:///path/to/cart.p8", "tokens": 4000, "chars": 26000, "bytes": 30000 }
  ],
  "includes": [
    { "path": "lib/player.lua", "uri": "file:///path/to/lib/player.lua", "tokens": 210, "chars": 1433, "bytes": 1500 }
  ],
  "tabs": [
    { "index": 0, "name": "main", "startLine": 3, "endLine": 200 }
  ]
}
```

`files`, `includes`, and `tabs` are omitted when empty. Counts are for the
assembled cart, so duplicate includes count each occurrence. Per-file counts are
informational. Tab indexes are zero-based; `.p8` include selectors use the same
numbering.

`compressed` is the number of bytes PICO-8 would store for the assembled code:
a pxa stream including its 8-byte header when compression helps, or the raw
P8SCII code length when it does not. It is computed from the flattened program
after include expansion and tab joining, with line endings normalized and each
P8SCII glyph counted as one byte. The value is cart-wide; per-file compressed
contributions are not reported because compression is global.

`compressedStatus` is one of:

- `ok` — below the warning threshold.
- `warning` — at or above `compressedWarning` (14054 bytes) and at or below the
  limit.
- `error` — above `compressedLimit` (15616 bytes); PICO-8 refuses to save a
  `.p8.png` or `.p8.rom` above this limit.
- `unavailable` — standalone `.lua` files, oversized code, or an input that
  exceeds the compressor's bounded work budget. `compressed` is `0` in this
  case, and no compressed-size diagnostic is published.

If no cart can be found, the request fails with JSON-RPC error code `-32001`
and message `cart-not-found`.

## `pico8/decodeCart`

Decodes a read-only `.p8.png` cart for an editor helper. The request accepts the
PNG source URI and never writes to disk:

```json
{
  "textDocument": { "uri": "file:///path/to/cart.p8.png" }
}
```

The result:

```json
{
  "sourceUri": "file:///path/to/cart.p8.png",
  "virtualUri": "pico8-cart:/path/to/cart.p8",
  "text": "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n...",
  "version": 42,
  "tabs": [],
  "includes": []
}
```

The returned `text` can be opened in a `pico8-cart:` virtual buffer and sent to
the server with `textDocument/didOpen`. Virtual documents are read-only;
`textDocument/didChange`, formatting, and editing code actions do not modify the
PNG. The same transform is available from the CLI:

```sh
pico-lsp decode --output /tmp/cart.p8 path/to/cart.p8.png
```

## `pico8.showCartInfo`

`workspace/executeCommand` command. The code lens uses it, and clients may call
it directly:

```json
{
  "command": "pico8.showCartInfo",
  "arguments": ["file:///path/to/cart.p8"]
}
```

The server shows a compact `window/showMessage` summary and returns the
`CartInfo` object as the command result.

## `pico8/glyphs`

Custom request that returns the server's P8SCII glyph catalog for a picker or
tooling integration.

```json
{
  "textDocument": { "uri": "file:///path/to/cart.p8" },
  "position": { "line": 10, "character": 4 }
}
```

Both parameters are optional; when supplied, they must be supplied together.
A position filters out string-only control entries when the cursor is not
inside a string. Without parameters, the request returns every entry with its
`context` field.

```json
{
  "glyphs": [
    {
      "name": "left",
      "text": "⬅️",
      "byte": 139,
      "aliases": ["p8-left"],
      "category": "button",
      "description": "Left button",
      "context": "both"
    },
    {
      "name": "font_custom",
      "text": "\\14",
      "byte": 14,
      "category": "control",
      "description": "Switch to the custom font at 0x5600",
      "context": "string"
    }
  ]
}
```

`byte` is the P8SCII code, not a Unicode codepoint. `context` is `both` or
`string`. Custom entries from `pico8.glyphs` appear in the same list and may
include `"custom": true`.

## `pico8.insertGlyph`

`workspace/executeCommand` command used by the glyph picker. It inserts at the
cursor, or replaces a `p8_`/`p8-` trigger immediately before the cursor:

```json
{
  "command": "pico8.insertGlyph",
  "arguments": [
    {
      "textDocument": { "uri": "file:///path/to/cart.p8" },
      "position": { "line": 10, "character": 4 },
      "name": "left"
    }
  ]
}
```

Use exactly one of `name` or `byte`. The command refuses read-only `.p8.png`
virtual documents and string-only control entries outside a string.

When the client advertises `workspace.applyEdit`, the server applies the edit
and returns `{"applied": true}`. Otherwise it returns the edit for the client:

```json
{
  "applied": false,
  "edit": {
    "changes": {
      "file:///path/to/cart.p8": [
        {
          "range": {
            "start": { "line": 10, "character": 0 },
            "end": { "line": 10, "character": 5 }
          },
          "newText": "⬅️"
        }
      ]
    }
  }
}
```

## Code actions and refactors

`textDocument/codeAction` returns standard `CodeAction`s. The M14 actions use
these kinds:

| Kind | Action |
|---|---|
| `refactor.extract.function` | Extract a single expression or statement into a local function. |
| `refactor.rewrite.class` | Generate a `---@class` block from a table literal. |
| `refactor.rewrite.local` | Convert a safe global declaration to `local`. |
| `refactor.rewrite.shorthand` | Convert between standard and PICO-8 shorthand `if`/`while`. |
| `quickfix.pico-lsp.add-missing-fields` | Add required fields to a table literal flagged by `missing-fields`. |
| `quickfix.pico-lsp.remove-unused` | Remove an unused local or local function. |

All actions except remove-unused embed a previewable `WorkspaceEdit`. Edits are
always expressed in the coordinates of the original `.p8` file or included
`.lua` file; includes are never added, removed, or reordered. No action is
offered when an open document differs from the analyzed snapshot. Disable the
M14 actions with `pico8.refactors.enabled: false`.

Remove-unused returns a code action with a `pico8.removeUnused` command instead
of an embedded edit so the editor can treat it as an explicit confirmation. The
command re-validates the symbol and sends a standard `workspace/applyEdit`:

```json
{
  "command": "pico8.removeUnused",
  "arguments": [
    { "uri": "file:///path/to/cart.p8", "start": 123, "end": 129 }
  ]
}
```

The server refuses the command if the analysis is stale, the symbol is no longer
an unused local, or the edit cannot be constructed. Code actions require the
symbol to be a local or local function with no reads. Write-only locals,
declarations with impure initializers (such as calls), and declarations in
`---@meta` files are not removable. Attached doc blocks containing file-level
tags such as `@class`, `@alias`, `@enum`, `@meta`, `@diagnostic`, or `@cast`
also block the action.

## Budget code lens

The root cart document has one code lens on line 0:

```
4210 / 8192 tokens · 27433 / 65535 chars · 12000 / 15616 compressed
```

Clicking it runs `pico8.showCartInfo`. The lens appends `(warning)` at or above
90% of the compressed-size limit and `(over limit)` above the limit; carts
without a compressed budget show `compressed n/a`. Disable the lens with
`pico8.limits.codeLens: false`.

## Go public packages

Go programs can import stable public facades without importing `internal/`
packages. These packages are additive and do not change the LSP protocol.
`pico8/cartInfo`, `pico8/decodeCart`, and `pico8/glyphs` remain protocol
requests; they are not Go APIs.

### `pkg/p8scii`

`github.com/asimmons91/pico-lsp/pkg/p8scii` exposes PICO-8 0.2.7's 256-byte
P8SCII charset for editors and other tools:

```go
glyphs := p8scii.Glyphs()           // all 256 entries, in byte order
glyph := p8scii.Lookup(0x8b)        // Byte, Text, Width, Printable
glyph, ok := p8scii.GlyphFor("😐️")  // canonical text, optional trailing U+FE0F
code := p8scii.Encode("♥")          // []byte{0x87}
text := p8scii.Decode([]byte{0x87}) // "♥"
```

`Text` is the canonical Unicode spelling. `Width` is the display width in
pixels at 1×: `0` for control bytes `0x00`–`0x0f`, `4` for `0x10`–`0x7f`, and
`8` for `0x80`–`0xff`. `Printable` is false only for control bytes. `Glyphs`
returns a copy. Added in v1.6.0 and stable under semver.

### `pkg/cart`

`github.com/asimmons91/pico-lsp/pkg/cart` parses and splices text `.p8` carts:

```go
doc, err := cart.Parse("game.p8", data) // pure: no filesystem access
doc.Code()                              // exact __lua__ body
doc.Tabs()                              // zero-based index, exclusive EndLine
doc.Sections()                          // known and unknown sections
doc.Includes()                          // lexical #include directives
out, err := doc.ReplaceCode(code)       // only the __lua__ body changes
owner, err := cart.NewResolver(nil).Owner("/work/game/lib.lua")
```

`Parse` accepts `.p8` only and rejects `.p8.png`, binary input, and input over
1 MiB. `ReplaceCode` preserves the BOM, header, sections, and every non-code
byte; it uses the original code body's CRLF style and leaves a data CR (P8SCII
glyph 13) as data, forcing a CRLF terminator when the CR is adjacent to one.
`Owner` requires an absolute `.p8` or `.lua` path; it finds the nearest
ancestor cart that includes the `.lua` file and is safe for concurrent use.
Added in v1.6.0 and stable under semver.

### `pkg/lsp`

`github.com/asimmons91/pico-lsp/pkg/lsp` runs the server in-process over a
caller-owned stream:

```go
serverPipe, clientPipe := net.Pipe()

srv := lsp.New(lsp.Options{Logger: logger}) // nil Logger is silent
conn := srv.Serve(ctx, jsonrpc2.NewStream(serverPipe))
defer conn.Close()

_, clientConn, client := protocol.NewClient(
	clientCtx, editor, jsonrpc2.NewStream(clientPipe),
)
defer clientConn.Close()

result, err := client.Initialize(ctx, params)
```

`New` always returns a usable server, and the zero `Options` is valid. One
server serves one connection. Configuration arrives through
`initializationOptions.pico8` and `workspace/configuration`; there are no Go
settings types. The caller supplies the transport, workspace folders in the
`initialize` params, and the lifecycle: cancel the context or close the
returned connection. `Serve` never writes to stdout, and a nil logger discards
all records. The in-process server advertises the same capabilities and
`ServerInfo` version as the stdio binary. Added in v1.6.0 and stable under
semver.

### `pkg/glyphs`

`github.com/asimmons91/pico-lsp/pkg/glyphs` exposes the P8SCII glyph catalog
used by `pico8/glyphs` and glyph completion:

```go
entries := glyphs.Builtins()                    // 250 built-in entries, a copy
heart, heartOK := glyphs.FindByName(entries, "p8_heart")
left, leftOK := glyphs.FindByByte(entries, 0x8b)

slot := 0x80
entries, warnings := glyphs.Build([]glyphs.Definition{
	{Name: "logo", Byte: &slot},
})
```

`Entry` and `Definition` alias the server's catalog types, so their JSON tags
are part of the public contract. `Builtins` returns a copy and `Build` keeps
the server's merge, override, and warning behavior. `pico8/glyphs` remains the
canonical picker path for LSP clients; both read the same catalog. Added in
v1.6.0 and stable under semver.
