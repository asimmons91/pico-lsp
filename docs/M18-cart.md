# M18 — Cart Public Package

**Status:** Planned — spec ready, implementation not started

**Depends on:** M17  
**Release target:** v1.7.0  
**Blocks:** pico-editor M4  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

External Go programs can open a `.p8` cart, read its code body, tabs, sections,
and include directives, replace only the code body while preserving every other
byte, and resolve which cart owns an included `.lua` file. pico-editor M4 builds
its cart view, save path, and include association on this package without
importing `internal/cart`.

**Demo:** Parse a real cart, print its tab names and include selectors, replace
the `__lua__` body with a new tab set, and verify with a byte diff that the
header, `__gfx__`, `__sfx__`, line endings, and every other non-code byte are
unchanged. Resolve `/work/lib.lua` to `/work/game.p8` when `game.p8` includes
it, or to itself when it is standalone. The consumer imports only `pkg/cart`.

## Decisions

Confirmed before implementation:

| ID | Decision | Choice |
|---|---|---|
| D1 | Public model | A new cart-only `Document` API. Internal `cart.Cart`, `Snapshot`, and source-map types are not exposed. |
| D2 | Parse boundary | `Parse` is pure: it never reads the filesystem, resolves `#include`, or flattens includes. `Code()` retains directive text. |
| D3 | Code replacement | `ReplaceCode` takes the complete code body, including `-->8` markers. It preserves blank lines and the code section's newline style. |
| D4 | Missing code section | A cart without `__lua__` parses for inspection (`HasCode` false) but cannot be spliced; `ReplaceCode` returns a typed error. No section is created implicitly. |
| D5 | Byte preservation | Only the `__lua__` body changes. The BOM, header, section order, unknown sections, and every non-code byte are preserved. |
| D6 | File writing | `ReplaceCode` returns bytes. Atomic writes and fsync stay in pico-editor's `fsutil`. |
| D7 | Includes | `Includes()` exposes lexical include paths and tab selectors. `Resolver` owns filesystem-aware ownership resolution. |
| D8 | Compatibility | Stable semver from v1.7.0; additive changes only. |
| D9 | Read-only `.p8.png` | Out of scope; `Parse` returns a typed unsupported-format error. |

## Scope

### In scope

- `pkg/cart.Document` for `.p8` carts.
- Code body, tab, section, and include-directive metadata.
- Byte-preserving `ReplaceCode`, including CRLF and missing-trailing-newline
  cases.
- Include ownership resolution with a public filesystem `Source`.
- Typed parse and validation errors.
- Godoc, runnable examples, and a `docs/api.md`/README entry.
- Golden tests for byte round-trips, BOM, CRLF, unknown sections, includes, and
  empty tabs.
- An external consumer module that imports only `pkg/cart`.
- CI wiring for the consumer module, formatting, and new fuzz target.
- The v1.7.0 changelog entry, tag, and release notes.

### Out of scope

- `.p8.png` decoding, writing, or virtual documents.
- Parsing standalone `.lua` documents; the editor opens those as plain files.
- Include flattening, source maps, diagnostics, token/character/compressed
  budgets, or syntax analysis.
- Filesystem reads during `Parse`.
- Atomic file writes, backups, permissions, or external-change detection.
- Sprite, map, sound, music, or label editing.
- A workspace file tree or editor UI.
- pico-editor code or dependencies.

## Deliverables

1. `pkg/cart/document.go` with `Document`, `Tab`, `Section`, and `Include`.
2. `pkg/cart/splice.go` with byte-preserving `ReplaceCode`.
3. `pkg/cart/resolver.go` with `Source`, `DiskSource`, and `Resolver`.
4. `pkg/cart/errors.go` with typed errors.
5. Unit, golden, fuzz, and race tests under `pkg/cart`, using
   `pkg/cart/testdata/`.
6. `pkg/cart/example_test.go` with runnable examples.
7. An external consumer module at `testdata/publicapi/cart` with module path
   `example.com/pico-lsp-publicapi-cart` and a local `replace` to the repo root.
8. CI updates: `gofmt -l cmd internal pkg testdata/publicapi`, the consumer
   module test, and the `pkg/cart` fuzz target in the nightly list.
9. `CHANGELOG.md` entry and `v1.7.0` release.

## Detailed requirements

### 1. Document API

```go
package cart

type Tab struct {
    Index      int    // zero-based, matching pico-editor and pico8/cartInfo
    Name       string // text after -->8, or "Tab N"
    StartLine  int    // first content line, full-cart, zero-based
    EndLine    int    // exclusive, full-cart
    MarkerLine int    // hidden -->8 line, or -1 for the first tab
}

type Section struct {
    Name      string
    Known     bool
    StartLine int // zero-based, full-cart
    EndLine   int // exclusive, full-cart
}

type Include struct {
    Path         string // as written, without quotes
    ResolvedPath string // cleaned lexical path relative to the cart
    Tab          int    // zero-based selected tab, or -1 for all/none
    Line         int    // zero-based full-cart line of the directive
}

type Document struct { /* opaque */ }

func Parse(path string, data []byte) (*Document, error)

func (d *Document) Path() string
func (d *Document) Code() string
func (d *Document) HasCode() bool
func (d *Document) CodeStartLine() int
func (d *Document) Tabs() []Tab
func (d *Document) Sections() []Section
func (d *Document) Includes() []Include
func (d *Document) ReplaceCode(code string) ([]byte, error)
```

- `Parse` never reads the filesystem and never writes. `path` is used for the
  extension, error messages, and lexical include resolution.
- `Parse` supports `.p8` paths only. Extension matching is case-insensitive.
  Other extensions, including `.p8.png`, return the typed unsupported-format
  error.
- `Parse` accepts UTF-8 with an optional BOM. Binary data, invalid UTF-8, and
  files over the supported size return typed errors.
- Duplicate `__lua__` sections are a parse error with a line number.
- `Path()` returns the cleaned path passed to `Parse`.
- `Code()` returns exactly the bytes between the `__lua__` header line and the
  next section header (or EOF), excluding the header line. CRLF and LF are
  preserved exactly as stored. The BOM and header are not part of `Code()`.
- `HasCode()` reports whether a `__lua__` section exists. A cart without one
  still parses; `Code()` is empty and `Tabs()` is nil.
- `CodeStartLine()` returns the full-cart zero-based line of the first code
  body line. It returns `0` when `HasCode()` is false.
- `Sections()` returns sections in file order, including unknown sections.
  `Known` is true for `lua`, `gfx`, `gff`, `label`, `map`, `sfx`, and `music`.
- Returned slices are copies. Mutating them does not affect the document.

### 2. Tabs

- A tab marker is a line whose **column 0** starts with `-->8`. This matches
  pico-editor's locked cart-tab rule. Do not use first-non-space detection for
  the public model.
- Tab 0 is the text before the first marker. Its `MarkerLine` is `-1` and its
  `StartLine` is `CodeStartLine()`.
- A marker line belongs to the tab it introduces as `MarkerLine`; it is not
  content. The next tab's `StartLine` is `MarkerLine + 1`.
- `EndLine` is exclusive and is the next marker line, or one past the last
  code line.
- Empty tabs are valid: `StartLine == EndLine`, with `MarkerLine` set. Tab 0 is
  still returned when the body starts with a marker; it has zero content lines.
- `Name` is the trimmed text after `-->8`, or `Tab N` when empty.
- Tab indices are zero-based.
- The public and internal LSP tab models must agree. If the internal
  first-non-space scanner differs, align the shared helper rather than
  duplicating tab parsing in `pkg/cart`.

### 3. Include directives

- `Includes()` scans the raw code body without reading included files. It
  recognizes `#include` directives outside strings and comments, using the same
  lexical scanner as `internal/cart`.
- `Path` is unquoted. `ResolvedPath` is
  `filepath.Clean(filepath.Join(filepath.Dir(cartPath), path))` and is lexical
  only; it may not exist and is absolute when the cart path is absolute.
- `Tab` parses a trailing `:N` selector as a zero-based tab, or `-1` when
  absent. Quoted paths and paths with spaces are supported.
- `Line` is the zero-based full-cart directive line.
- Commented-out directives and directives inside strings/comments are ignored.
- A missing include file does not make `Parse` fail. It is not an IO operation;
  LSP diagnostics remain the source of missing/unreadable include warnings.
- `Includes()` returns a copy in source order.

### 4. Byte-preserving replacement

- `ReplaceCode` requires `HasCode()`. Otherwise it returns
  `ErrNoCodeSection`; it never inserts a section.
- The input is the complete code body, including `-->8` markers.
- The output replaces only the existing `__lua__` body region. The header,
  BOM, section order, unknown sections, and all other bytes are preserved.
- Newline style:
  - Normalize the input's CRLF/CR/LF line endings to LF first.
  - If the original code body contains CRLF, convert those LF endings to CRLF
    before splicing.
  - Otherwise write LF.
  - Preserve blank lines and do not trim caller-supplied text.
- Ensure the resulting code ends with at least one newline before the next
  section header when the code is non-empty. If the next section follows an
  empty body immediately after the header, do not invent a blank line.
- If the `__lua__` header line itself has no trailing line terminator and the
  new code is non-empty, insert `\n` after the header. This is the only
  header-adjacent byte this method may add; all other header bytes stay
  identical. Add a fixture for this case.
- Replacement handles:
  - an empty code body,
  - a body with no trailing newline before a following section,
  - code ending with or without a newline,
  - a cart with no following section,
  - repeated calls,
  - caller text containing markers and blank lines.
- On any error, return no bytes; the caller keeps its original data.
- The method does not mutate the document.

### 5. Include ownership

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
- Ownership uses `Document.Includes()` and the same lexical scanner as the LSP
  resolver. Tab selectors are honored when matching includes; they are not
  accepted as `Owner` inputs.
- Multiple ancestor carts: nearest directory wins; lexical path order breaks
  ties. Unreadable candidate carts are skipped.
- `Owner` returns an error only for an invalid path (empty, relative, or not a
  `.p8`/`.lua` path).
- Results are cached. `Invalidate` clears the cache; the caller invokes it after
  file tree changes or buffer edits.
- `Resolver` is safe for concurrent use, including `Owner` concurrent with
  `Invalidate`.

### 6. Errors

- Define exported typed errors usable with `errors.Is`/`errors.As`:
  - unsupported format (`.p8.png` and non-`.p8` extensions),
  - binary or invalid UTF-8 input,
  - oversized input,
  - duplicate `__lua__` section,
  - missing `__lua__` section for `ReplaceCode`,
  - invalid path.
- `Parse` rejects malformed section markers with a typed error containing the
  line number.
- `ReplaceCode` never returns partial output.

### 7. Tests

- Golden `.p8` fixtures covering:
  - header plus Lua plus every known section,
  - unknown sections,
  - BOM,
  - CRLF code and CRLF non-code sections,
  - empty body,
  - header without a trailing newline and non-empty code,
  - no `__lua__` section,
  - duplicate `__lua__`,
  - leading `-->8` marker,
  - consecutive markers (empty tabs),
  - include directives with quotes, spaces, and `:N` selectors,
  - a missing include file.
- Splice tests assert every byte outside the code body is identical and that
  only the expected region changes.
- Round-trip tests: `Parse`→`ReplaceCode(Code())` is byte-identical for the
  corpus, including CRLF carts and carts with no trailing newline.
- Boundary tests: input without a trailing newline, extra blank lines, empty
  code, repeated replacement, and an error path with no partial output.
- Copies: mutating `Tabs()`, `Sections()`, or `Includes()` results does not
  affect later calls.
- Parser independence: a cart containing `#include "missing.lua"` parses
  successfully without touching the filesystem; `Code()` retains the directive.
- Resolver tests: nearest owner, multiple carts, standalone `.lua`, commented
  includes, short-string continuations, tab selectors, quoted paths, cache
  invalidation, invalid paths, unreadable candidates, concurrent
  `Owner`/`Invalidate`, and parity with the internal LSP resolver.
- Typed error tests for every error category, including `errors.As` line
  numbers.
- External consumer module: imports only `pkg/cart`, tests `Parse`, `Code`,
  `Tabs`, `Includes`, `ReplaceCode`, and `Resolver`.
- Fuzz `Parse` with arbitrary bytes: no panic and typed errors.

### 8. Documentation and release

- Godoc documents the pure `Parse` boundary, CRLF behavior, tab line
  conventions, and include lexical rules.
- Add runnable examples for parsing, splicing, tabs, includes, and ownership.
- Add a short `pkg/cart` entry to `docs/api.md` or `README.md`.
- Add `CHANGELOG.md` under `Unreleased`, then cut `v1.7.0` after the PR merges.
- Release notes state that this is additive and does not change the LSP server.

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

- `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt -l .`, and
  `golangci-lint run` pass.
- `gofmt` covers `cmd`, `internal`, `pkg`, and `testdata/publicapi`.
- The external consumer module runs in CI with `GOFLAGS=-mod=readonly`.
- The `pkg/cart` fuzz target is added to the short CI fuzz list and the nightly
  fuzz workflow.
- Golden and fuzz tests run in CI.

## Exit criteria

- [ ] `pkg/cart.Document` parses `.p8` carts without exposing internal types.
- [ ] `Parse` never reads the filesystem or flattens includes.
- [ ] `Code()` preserves CRLF; `ReplaceCode` re-applies the original code
      newline style and changes only the `__lua__` body.
- [ ] BOM, unknown sections, section order, and blank lines are preserved.
- [ ] Missing, empty, and duplicate `__lua__` cases match the spec.
- [ ] `Tabs` follows column-0 markers, zero-based indices, full-cart lines,
      exclusive ends, `MarkerLine`, and empty-tab behavior.
- [ ] `Includes` handles quotes, spaces, and tab selectors without IO.
- [ ] `Resolver.Owner` passes nearest-cart, include, cache, concurrency, and
      standalone tests.
- [ ] Typed errors are covered by `errors.Is`/`errors.As` tests.
- [ ] The external consumer module imports only `pkg/cart` and runs in CI.
- [ ] `go test -race`, lint, vet, gofmt, and the consumer readonly build pass.
- [ ] `v1.7.0` is tagged and pico-editor M4 can pin it.
- [ ] No LSP behavior changed and existing internal behavior is preserved.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Save corrupts non-code sections | Golden byte-diff tests on real carts; only the Lua body is replaced. |
| CRLF normalization surprises users | `Code()` preserves the original style; `ReplaceCode` restores it; round-trip tests include CRLF carts. |
| Public API leaks internal types | New `Document` model, consumer module, and an exported-signature review. |
| Parse accidentally resolves includes | Pure parse contract, missing-include fixture, and no loader/Provider dependency in `pkg/cart`. |
| Include ownership drifts from the LSP resolver | Shared include scanner and public-vs-internal parity tests. |
| Tab model diverges from LSP | Column-0 shared helper, full-cart line tests, and parity fixtures. |
| Resolver races | Mutex-protected cache plus concurrent `Owner`/`Invalidate` race tests. |
