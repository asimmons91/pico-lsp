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

## Budget code lens

The root cart document has one code lens on line 0:

```
4210 / 8192 tokens · 27433 / 65535 chars · 12000 / 15616 compressed
```

Clicking it runs `pico8.showCartInfo`. The lens appends `(warning)` at or above
90% of the compressed-size limit and `(over limit)` above the limit; carts
without a compressed budget show `compressed n/a`. Disable the lens with
`pico8.limits.codeLens: false`.
