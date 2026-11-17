# PICO-8 LSP API

pico-lsp exposes two PICO-8-specific entry points in addition to the standard
LSP methods.

## `pico8/cartInfo`

Custom request for statusline and tooling integrations.

```json
{
  "textDocument": { "uri": "file:///path/to/cart.p8" }
}
```

The request accepts the root cart or any included file. Included files resolve
to their owning cart.

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
