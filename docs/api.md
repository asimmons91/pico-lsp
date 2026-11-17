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
4210 / 8192 tokens · 27433 / 65535 chars
```

Clicking it runs `pico8.showCartInfo`. Disable the lens with
`pico8.limits.codeLens: false`.
