# M17 — P8SCII Public Package

**Status:** Merged — v1.6.0 release pending

**Depends on:** M16  
**Release target:** v1.6.0  
**Blocks:** pico-editor M1  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

External Go programs can map P8SCII without importing `internal/` packages.
The authoritative 256-entry glyph table, canonical Unicode spellings, and
display widths live upstream in pico-lsp, so pico-editor and other consumers
share one source of truth for encoding, decoding, and cursor geometry.

**Demo:** A small consumer module imports
`github.com/asimmons91/pico-lsp/pkg/p8scii`, iterates all 256 glyphs, and prints
each byte, canonical text, and width. `Encode("♥")` returns `0x87`;
`Decode([]byte{0x87})` returns `"♥"`; `Lookup(0x8b)` reports an 8-pixel-wide
left-arrow glyph. The consumer has no `internal/` import and builds from the
tagged release. pico-editor M1 does the same to size glyphs at 2×–8×.

## Decisions

Confirmed before implementation:

| ID | Decision | Choice |
|---|---|---|
| D1 | Relationship to internal code | Thin public facade. The canonical mapping and width table remain in `internal/p8scii`. |
| D2 | Public surface | Glyph metadata, `GlyphFor`, `Encode`, `Decode`, `Lookup`, and `Glyphs`. |
| D3 | Width unit | Pixels at 1×: `4` for `0x10`–`0x7f`, `8` for `0x80`–`0xff`, `0` for control bytes. |
| D4 | Printable classification | `0x10`–`0xff` are printable glyphs; `0x00`–`0x0f` are control bytes. |
| D5 | Compatibility | Stable semver from v1.6.0; additive changes only. |
| D6 | Rendering | Out of scope. No font atlas, pixel data, or drawing code. |

## Scope

### In scope

- Glyph metadata for all 256 P8SCII bytes in `internal/p8scii`.
- The P8SCII display-width table.
- A public `pkg/p8scii` facade with stable exported types and functions.
- Godoc comments and runnable examples.
- An external consumer test that imports only `pkg/p8scii`.
- The v1.6.0 changelog entry, tag, and release notes.

### Out of scope

- Font atlas data, glyph pixels, anti-aliasing, or rendering.
- P8SCII input modes (`Ctrl+J`, `Ctrl+K`, `Ctrl+P`, Shift+letter tables).
- Changing the existing byte-to-Unicode mapping or PICO-8 version support.
- Glyph picker completeness (`pkg/glyphs` is M19), custom glyph definitions,
  or editor UI.
- `.p8.png`, cart parsing, or LSP behavior.
- pico-editor code or dependencies.

## Deliverables

1. `internal/p8scii/glyph.go` with the 256-entry metadata and width table.
2. `pkg/p8scii/p8scii.go` with the public facade and godoc.
3. Table tests for every byte, width class, and encode/decode round-trip.
4. `pkg/p8scii/example_test.go` with runnable examples.
5. An external consumer module at `testdata/publicapi/p8scii` with module path
   `example.com/pico-lsp-publicapi-p8scii` and a local `replace` to the repo
   root.
6. CI updates: `gofmt -l .`, the consumer module test, and the `pkg/p8scii`
   fuzz target in the nightly list.
7. `CHANGELOG.md` entry and `v1.6.0` release.

## Detailed requirements

### 1. Glyph metadata

- Add an internal type:

  ```go
  type Glyph struct {
      Byte      byte
      Text      string
      Width     int  // pixels at 1x
      Printable bool
  }
  ```

- `Glyphs()` returns a fresh copy of all 256 entries in byte order. Callers may
  not mutate shared state.
- `Lookup(b byte) Glyph` returns the entry for one byte. It is total; no error
  and no panic for any byte.
- `Text` is always `Decode([]byte{b})`. Never hand-write a multi-rune glyph
  spelling; this keeps `U+FE0F` sequences canonical.
- `Width` is `0` for `0x00`–`0x0f`, `4` for `0x10`–`0x7f`, and `8` for
  `0x80`–`0xff`. This is the locked pico-editor width rule.
- `Printable` is false only for `0x00`–`0x0f`. Control bytes still have a
  canonical `Text` for decode/encode round-tripping, but consumers must not
  draw them as cells.
- The width table is a single source of truth. Do not derive width from Unicode
  properties, rune count, `runewidth`, or grapheme clusters.
- pico-editor M1 consumes pixel widths and converts them to 4-pixel cells. The
  editor owns cluster iteration and cursor snapping; `pkg/p8scii` supplies the
  byte table and widths those routines use.

### 2. Public facade

```go
package p8scii

type Glyph = internalp8scii.Glyph

func Encode(text string) []byte
func Decode(code []byte) string
func Lookup(b byte) Glyph
func GlyphFor(text string) (Glyph, bool)
func Glyphs() []Glyph
```

- `GlyphFor` resolves a canonical text cluster to its glyph. It first tries the
  exact text, then retries once with one trailing `U+FE0F` removed. This maps
  both `😐` and `😐️` to byte `0x8c` without duplicating VS16 logic in the editor.
  Unknown text returns `(Glyph{}, false)`.

- The public package may import `internal/p8scii`; external consumers cannot.
- Exported functions and fields must not require callers to import internal
  packages. The `Glyph` alias is the only bridge and exposes no internal fields
  or methods.
- `Encode` and `Decode` keep their current behavior exactly, including the
  fallback that reinterprets unknown characters as UTF-8 bytes.
- Do not add a second copy of the charset or width table in `pkg/`.

### 3. Tests

- Every byte: `Lookup(b).Text == Decode([]byte{b})`,
  `Lookup(b).Byte == b`, `Lookup(b).Printable` matches the control/printable
  class, and width matches the class table.
- Every printable byte: `Encode(Lookup(b).Text)` returns exactly `[]byte{b}`.
- `GlyphFor` resolves every `Glyph.Text`, accepts a base glyph plus one trailing
  `U+FE0F`, and rejects unknown text. In particular, both `😐` and `😐️` resolve
  to `0x8c`.
- Control bytes: decode/encode round-trip where the current mapping supports
  it; document any exception rather than changing behavior.
- `Glyphs()` returns 256 entries, is sorted by byte, and is a copy: mutating the
  result does not affect a later call.
- External consumer module: imports only `pkg/p8scii`, calls every exported
  function, and asserts a representative wide/narrow glyph. The module path is
  `example.com/pico-lsp-publicapi-p8scii`, so it cannot import
  `github.com/asimmons91/pico-lsp/internal/...`; CI runs `go test ./...` in that
  directory.
- Fuzz `Encode` followed by `Decode` for valid UTF-8 input: no panic, and the
  result is valid UTF-8. (Round-trip equality is not required for unknown
  characters because Encode preserves UTF-8 bytes.)

### 4. Documentation and release

- Godoc explains the P8SCII byte, canonical Unicode text, control bytes, and
  width units.
- Add examples for encoding, decoding, iterating glyphs, and looking up a width.
- Add a short "Go public packages" section to `docs/api.md` or `README.md`
  linking `pkg/p8scii`.
- Add `CHANGELOG.md` under `Unreleased`, then cut `v1.6.0` after the PR merges.
- Release notes state that this is additive and does not change the LSP server.

## Interfaces and state

The public package has no mutable global state. The internal table is
initialized once and never changed. `Glyphs` returns a copy; `Lookup` returns a
value.

## Error handling

- Every public function is total. There are no new error paths.
- Unknown Unicode input continues to encode as UTF-8 bytes, matching PICO-8 and
  the existing behavior.
- A wrong width classification is a release-blocking correctness bug, not a
  runtime error.

## Testing and CI

- `go test ./...` and `go test -race ./...` pass.
- `golangci-lint run`, `gofmt`, and `go vet` pass.
- `gofmt` covers the whole repository (`gofmt -l .`).
- The external consumer module builds with `GOFLAGS=-mod=readonly` and runs in
  CI.
- The `pkg/p8scii` fuzz target is added to the short CI fuzz list and the
  nightly fuzz workflow.
- No screenshots are required. Visual verification belongs to pico-editor M1.

## Exit criteria

- [x] `internal/p8scii` exposes all 256 glyph entries and the width table.
- [x] `pkg/p8scii` exposes only the agreed public API and no internal types in
      exported signatures.
- [x] All 256 bytes pass metadata, `Printable`, width, and round-trip tests.
- [x] `GlyphFor` resolves every canonical `Glyph.Text`, accepts one trailing
      `U+FE0F`, and rejects unknown text.
- [x] The external consumer module imports only `pkg/p8scii`, uses an external
      module path, and runs in CI.
- [x] `go test ./...`, `-race`, lint, vet, gofmt, and the consumer readonly
      build pass.
- [ ] `v1.6.0` is tagged and pico-editor M1 can pin it.
- [x] No existing LSP behavior or P8SCII mapping changed.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Width table disagrees with PICO-8 | One class table, exhaustive tests, and pico-editor M1's screenshot comparison. |
| Public API leaks internal types | Consumer module compile test and an API review against exported signatures. |
| Facade drifts from internal mapping | `pkg/p8scii` only aliases and forwards; no duplicate tables. |
| Stable API freezes a mistake | Keep the surface minimal now; additional helpers are additive later. |
