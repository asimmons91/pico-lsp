# M8 — Read-Only `.p8.png` Carts

**Status:** Planned  
**Depends on:** M7  
**Release target:** v1.1.0 (batched with M7, M9, M10)  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

Users can open a `.p8.png` cartridge directly in Neovim and use navigation,
diagnostics, completion, hover, symbols, and cart-info features without
exporting it to text first. The decoded cart is read-only; editing still requires
saving a text `.p8`.

**Demo:** In Neovim, run `:Pico8OpenPng` on a `.p8.png` cart. A read-only
`pico8-cart:///...` buffer opens with the same code intelligence as the
equivalent `.p8`. Diagnostics land on the decoded source, `pico8/cartInfo`
reports token/character counts, and a missing include resolves relative to the
PNG cart's directory.

## Scope

### In scope

- A PNG cart decoder for the standard 160×205 cartridge image format.
- Uncompressed code, new `\0pxa` compression, and old `:c:\0` compression.
- P8SCII-to-Unicode decoding, BOM/newline handling, tabs, and `#include`
  resolution relative to the PNG file.
- A read-only virtual document interface served as
  `pico8-cart:///<absolute-path-to-png>`.
- A custom request to decode a PNG cart and return its virtual document text and
  metadata.
- A Neovim helper command that opens the virtual buffer, sets `filetype=pico8`,
  marks it `nomodifiable`, and starts the existing client.
- A CLI decode command for Helix and other editors:
  `pico-lsp decode [--output PATH] CART.p8.png`.
- Decode-on-demand caching keyed by file identity/mtime/hash, invalidated by
  watched-file changes.
- Diagnostics for unsupported, corrupt, truncated, or non-PNG files.
- Token/character counts and cart info identical to the equivalent text cart.
- Clear errors when a read-only virtual document is asked to format or edit.

### Out of scope

- Writing or modifying `.p8.png` files.
- Compressed-size budgeting/compressor parity.
- Sprite/map/audio previews or editing.
- Live two-way synchronization between a PNG and an exported text cart.
- Automatic conversion/export UI.
- Solving editor limitations for clients that cannot open custom URI schemes.

## Architecture

### Virtual document

- The server exposes a read-only document URI:
  `pico8-cart:///<absolute-png-path>` (Windows drive paths remain absolute).
- The URI is a namespace for the decoded text, not a real filesystem path. The
  server must not call `os.Open` on it.
- A decoded virtual document has the same `Cart`/`Snapshot` shape as a text
  cart, including file IDs, tabs, includes, source maps, tokens, and characters.
- The original `.p8.png` URI remains the authoritative file for cache
  invalidation and `pico8/cartInfo`.

### Decode request

Add a custom request:

```
pico8/decodeCart
params: { textDocument: { uri } }
result: {
  sourceUri, virtualUri, text, version,
  tabs: [...], includes: [...]
}
```

The request accepts a `.p8.png` URI and returns the decoded document text plus
the virtual URI. It is cancellable and never writes to disk.

### Editor integration

- `editors/nvim` adds `:Pico8OpenPng [path]`:
  - requests `pico8/decodeCart`;
  - opens a scratch buffer with the virtual URI;
  - sets `filetype=pico8`, `nomodifiable`, and a read-only status hint;
  - attaches/reuses the `pico_lsp` client.
- `:Pico8ReloadPng` re-requests the decoded text after the PNG changes. The
  server also invalidates caches and republishes diagnostics from watcher events.
- Helix and other editors can use `pico-lsp decode --output /tmp/cart.p8
  cart.p8.png` and open the resulting text file. Document this as a static
  fallback, not live PNG support.
- Formatting and text-editing code actions on a virtual document are refused
  with a message explaining that `.p8.png` is read-only. Navigation and
  annotation-only actions that do not edit the virtual text may remain.

### Cache and invalidation

- Cache decoded snapshots by canonical source URI plus mtime/size and a content
  hash when mtime is unavailable.
- Invalidate on `workspace/didChangeWatchedFiles` create/change/delete events for
  `.p8.png`.
- Republish diagnostics and clear stale diagnostics when a PNG is replaced or
  deleted.
- Keep cache entries bounded; evict least-recently-used decoded carts when
  necessary.

## Decoder requirements

- Validate PNG structure and dimensions; reject non-cart images with a clear
  diagnostic.
- Read the cartridge payload from the documented low-bit channels.
- Support:
  - uncompressed code;
  - new `\0pxa` compressed code;
  - old `:c:\0` compressed code.
- Decode P8SCII to Unicode for editor display while preserving source offsets for
  diagnostics and navigation.
- Match PICO-8 newline and tab behavior: tabs are `-->8` boundaries; non-Lua
  sections are retained as metadata or opaque regions.
- Do not execute cart code or trust decoded lengths without bounds checks.
- Use a proven MIT-compatible reference implementation for compression details
  and cross-check against real carts.

## Testing

- Fixtures for uncompressed, new-compressed, and old-compressed carts, including
  multi-tab carts and carts with includes.
- Parity test: decoded `.p8.png` token/character counts, symbols, includes, and
  diagnostics match the equivalent text `.p8`.
- Corrupt/truncated/non-PNG/oversized inputs produce stable diagnostics and no
  panics.
- Source-map tests for diagnostics, definitions, references, and completion on
  decoded text.
- Cache tests for mtime/hash changes, deletion, and watcher invalidation.
- LSP e2e: `pico8/decodeCart`, virtual `didOpen`, hover/definition/completion,
  diagnostics, `pico8/cartInfo`, read-only rejection of formatting/edit actions.
- CLI e2e: stdout decode and `--output` file decode.
- Neovim headless test: `:Pico8OpenPng` opens a read-only buffer with the client
  attached; unrelated PNGs fail cleanly.
- Performance: decode and analyze a typical PNG cart under 50 ms; cached
  navigation remains within v1 request budgets.
- No writes outside the requested `--output` path or normal LSP logs.

## Exit criteria

- [ ] The decoder handles uncompressed, new-compressed, and old-compressed carts.
- [ ] `pico8/decodeCart` returns a stable read-only virtual document.
- [ ] Neovim `:Pico8OpenPng` opens the cart with LSP features and read-only
      behavior.
- [ ] `pico-lsp decode` works for Helix/scripted users.
- [ ] Decoded token/character counts and cart info match the text cart.
- [ ] Watcher changes invalidate the cache and republish diagnostics.
- [ ] Corrupt/unsupported carts fail cleanly without panics.
- [ ] `.p8.png` remains unsupported for writing/formatting.
- [ ] README, install, troubleshooting, API, and editor docs are updated.
- [ ] Performance and corpus tests pass.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| PNG payload decoding is subtly wrong | Use a proven reference, fixtures for all formats, and parity tests against text carts. |
| Virtual URI support differs by editor | Ship the Neovim helper plus a CLI fallback; document the boundary. |
| Source positions drift after P8SCII decoding | Preserve original byte offsets and test diagnostics/navigation on real carts. |
| Stale decoded text after PNG replacement | Cache hashing plus watcher invalidation and an explicit reload command. |
| Read-only actions confuse users | Mark buffers `nomodifiable` and return clear errors for edit requests. |
| Large/complex PNGs are slow | Decode on demand, cache snapshots, and enforce the existing cart size cap. |
