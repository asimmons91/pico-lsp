# M18 — Cart Public Package

**Status:** Spec'd — implementation not started  
**Depends on:** M17  
**Release target:** v1.7.0  
**Blocks:** pico-editor M4  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

External Go programs can open a `.p8` or `.lua` document, read its code tabs and
sections, replace only the code body while preserving every other byte, and
resolve which cart owns an included `.lua` file. pico-editor M4 builds its cart
view, save path, and include association on this package without importing
`internal/cart`.

**Demo:** Parse a real cart, print its tab names, replace the `__lua__` body
with a new tab set, and verify that the header, `__gfx__`, `__sfx__`, and every
other non-code byte are unchanged. Resolve `/work/lib.lua` to
`/work/game.p8` when `game.p8` includes it, or to itself when it is standalone.
The consumer imports only `pkg/cart`.

## Decisions

Confirmed before implementation:

| ID | Decision | Choice |
|---|---|---|
| D1 | Public model | A new `Document` API. Internal `cart.Cart`, `Snapshot`, and source-map types are not exposed. |
| D2 | Code replacement | `ReplaceCode` takes the complete code body, including `-->8` markers and LF line endings. |
| D3 | Byte preservation | Only the `__lua__` body changes. The BOM, header, section order, unknown sections, and every non-code byte are preserved. |
| D4 | File writing | `ReplaceCode` returns bytes. Atomic writes and fsync stay in pico-editor's `fsutil`. |
| D5 | Includes | Include ownership resolution is in v1.7.0. |
| D6 | Compatibility | Stable semver from v1.7.0; additive changes only. |
| D7 | Read-only `.p8.png` | Out of scope; `Parse` returns a typed unsupported-format error. |

## Scope

### In scope

- `pkg/cart.Document` for `.p8` and standalone `.lua` files.
- Section and code-tab metadata.
- Byte-preserving `ReplaceCode`, including carts without a `__lua__` section.
- Include ownership resolution with a public filesystem `Source`.
- Typed parse and validation errors.
- Golden tests for round-trips, BOM, CRLF, unknown sections, and included
  files.
- An external consumer module that imports only `pkg/cart`.
- The v1.7.0 changelog entry, tag, and release notes.

### Out of scope

- `.p8.png` decoding, writing, or virtual documents.
- Atomic file writes, backups, permissions, or external-change detection.
- Token/character/compressed-size budgets; those stay in the LSP layer.
- Syntax analysis, diagnostics, semantic tokens, or include flattening.
- Sprite, map, sound, music, or label editing.
- A workspace file tree or editor UI.
- pico-editor code or dependencies.

## Deliverables

1. `pkg/cart/document.go` with the public `Document`, `Kind`, `Tab`, and
   `Section` types.
2. `pkg/cart/splice.go` with byte-preserving `ReplaceCode`.
3. `pkg/cart/resolver.go` with `Source`, `DiskSource`, and `Resolver`.
4. `pkg/cart/errors.go` with typed errors.
5. Unit and golden tests under `pkg/cart/testdata/`.
6. An external consumer module under `testdata/publicapi`.
7. `CHANGELOG.md` entry and `v1.7.0` release.

## Detailed requirements

### 1. Document API

```go
package cart

type Kind int

const (
    KindP8 Kind = iota
    KindLua
)

type Tab struct {
    Index     int
    Name      string
    StartLine int // 0-based, inclusive, within Code
    EndLine   int // 0-based, inclusive, within Code
}

type Section struct {
    Name      string
    Known     bool
    StartLine int // 0-based, inclusive, in the original file
    EndLine   int // 0-based, inclusive, in the original file
}

type Document struct { /* opaque */ }

func Parse(path string, data []byte) (*Document, error)

func (d *Document) Path() string
func (d *Document) Kind() Kind
func (d *Document) Code() string
func (d *Document) Tabs() []Tab
func (d *Document) Sections() []Section
func (d *Document) ReplaceCode(code string) ([]byte, error)
```

- `Parse` never reads the filesystem and never writes. `path` is used for the
  extension, error messages, and ownership metadata.
- `Parse` supports `.p8` and `.lua` paths. Extension matching is
  case-insensitive. Other extensions, including `.p8.png`, return the typed
  unsupported-format error.
- `Parse` accepts UTF-8 with an optional BOM. Binary data, files over the
  supported size, and `.p8.png` paths return typed errors.
- `Path()` returns the cleaned path passed to `Parse`.
- `Code()` returns the `__lua__` body excluding the `__lua__` header line, with
  CRLF normalized to LF. It preserves a final newline when the body has one.
  For a `.lua` document it returns the whole file with LF normalization.
- `Tabs()` returns the code tabs for a `.p8` document and nil for `.lua`.
  Tab 1 starts at line 0. A line whose first non-space characters are `-->8`
  starts a new tab and belongs to that new tab. The tab `Name` is the trimmed
  text after `-->8`, or `Tab N` when empty.
- `Sections()` returns sections in file order, including unknown sections.
  `Known` is true for `lua`, `gfx`, `gff`, `label`, `map`, `sfx`, and `music`.
  Duplicate `__lua__` sections are a parse error. For `.lua` documents,
  `Sections()` returns nil.
- The returned slices are copies. Mutating them does not affect the document.
- A `.p8` with no `__lua__` section parses successfully with empty `Code()`,
  nil `Tabs()`, and a `Sections()` list that omits `lua`.

### 2. Byte-preserving replacement

- `ReplaceCode` accepts the complete code body, including any `-->8` markers.
  It does not accept per-tab slices and does not synthesize markers.
- Input is normalized to LF. PICO-8 accepts LF carts; non-code sections keep
  their original bytes, including CRLF.
- The output is the original byte slice with exactly one region replaced:
  the bytes after the `__lua__\n` header through the start of the next section
  header (or EOF).
- Ensure the new code ends with at least one `\n` before the next section
  header when the code is non-empty. Do not trim blank lines the caller
  supplied. If the next section starts immediately after the header and the
  code is empty, do not invent a blank line.
- If the input code is empty, the next section header follows the `__lua__`
  line directly.
- Preserve the BOM and the header (`pico-8 cartridge`, `version N`, and their
  line endings) exactly.
- If no `__lua__` section exists, insert one before the first non-`lua` section,
  or at the end when there are no sections. The inserted header uses `\n`.
  This is the only case where the output adds bytes outside the existing code
  region.
- `ReplaceCode` on a `.lua` document returns the normalized code bytes.
- Replacement never reorders sections, drops unknown sections, or rewrites
  their contents.
- The method is safe to call repeatedly; it does not mutate the document.

### 3. Include ownership

```go
type Source interface {
    ReadFile(path string) ([]byte, error)
    ReadDir(path string) ([]string, error)
}

type DiskSource struct{}

type Resolver struct { /* opaque */ }

func NewResolver(source Source) *Resolver
func (r *Resolver) Owner(path string) (string, error)
func (r *Resolver) Invalidate()
```

- `Source.ReadDir` returns full paths to entries in a directory. `DiskSource`
  implements both methods with the local filesystem.
- `NewResolver(nil)` uses `DiskSource`.
- `Owner` returns:
  - the cleaned path for a `.p8` file;
  - the nearest ancestor `.p8` that includes the `.lua` file;
  - the cleaned path itself when no cart owns the `.lua` file.
- Include detection parses `__lua__` bodies and standalone `.lua` files using
  the same comment/string-aware scan as `internal/cart`. Commented-out includes
  do not count. `#include` is non-recursive.
- When multiple ancestor carts include the file, the nearest directory wins;
  ties are broken by lexical path order and logged by the caller if needed.
- Unreadable candidate carts are skipped. `Owner` returns an error only for an
  invalid path (empty, relative, or not a `.p8`/`.lua` path).
- Results are cached. `Invalidate` clears the cache; the caller invokes it after
  file tree changes or buffer edits.
- `Resolver` is safe for concurrent use.

### 4. Errors

- Define typed sentinel errors or an exported `ParseError` with path, line, and
  message fields. At minimum:
  - unsupported format (`.p8.png`),
  - binary or invalid UTF-8 input,
  - oversized input,
  - duplicate `__lua__` section,
  - invalid path.
- Errors must be usable with `errors.Is`/`errors.As`.
- `ReplaceCode` must not return partial output with an error. On failure, the
  caller keeps its original bytes.

### 5. Testing

- Golden `.p8` fixtures covering:
  - header plus Lua plus every known section,
  - unknown sections,
  - BOM,
  - CRLF in non-code sections,
  - empty code,
  - no `__lua__` section,
  - duplicate `__lua__`,
  - `-->8` tabs with and without names.
- Splice tests assert that every byte outside the Lua body is identical and
  that only the expected region changes.
- `.lua` tests cover whole-file replacement, BOM handling, and LF
  normalization.
- Resolver tests cover nearest-cart ownership, multiple carts, standalone
  `.lua`, commented includes, short-string continuations, cache invalidation,
  and unreadable candidates.
- External consumer module imports only `pkg/cart` and exercises `Parse`,
  `ReplaceCode`, `Tabs`, `Sections`, and `Resolver`.
- Fuzz `Parse` with arbitrary bytes: no panic, valid UTF-8 in/out, and errors
  are typed.

## Interfaces and state

`Document` is immutable after `Parse`. `ReplaceCode` is a pure function of the
document and the input text. `Resolver` owns a mutex-protected cache and a
`Source`; it has no global state.

## Error handling

- Parse failures are returned, never panicked.
- Splice failures preserve the original bytes.
- Resolver IO failures are best-effort: unreadable carts are skipped; only
  invalid input paths return errors.
- No `.p8.png` support in v1.7.0; callers must handle the typed error.

## Testing and CI

- `go test ./...`, `go test -race ./...`, `go vet`, `gofmt`, and
  `golangci-lint run` pass.
- Golden and fuzz tests run in CI.
- The external consumer module builds with `GOFLAGS=-mod=readonly`.

## Exit criteria

- [ ] `pkg/cart.Document` parses `.p8` and `.lua` documents without exposing
      internal types.
- [ ] `ReplaceCode` changes only the `__lua__` body on golden carts.
- [ ] BOM, CRLF, unknown sections, and section order are preserved.
- [ ] Missing and duplicate `__lua__` cases match the spec.
- [ ] `Resolver.Owner` passes nearest-cart, include, cache, and standalone
      tests.
- [ ] The external consumer module imports only `pkg/cart`.
- [ ] `v1.7.0` is tagged and pico-editor M4 can pin it.
- [ ] No LSP behavior changed and existing internal behavior is preserved.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Save corrupts non-code sections | Golden byte-diff tests on real carts; only the Lua region is replaced. |
| Line-ending normalization surprises users | Normalize only the code body; preserve all other bytes and document the rule. |
| Public API leaks internal types | New `Document` model, consumer module, and an exported-signature review. |
| Include ownership drifts from the LSP resolver | Both paths use the same `internal/cart` include scanner; table tests compare them. |
| Resolver hides IO failures | Document best-effort behavior; invalid paths return errors; callers can log unreadable candidates. |
