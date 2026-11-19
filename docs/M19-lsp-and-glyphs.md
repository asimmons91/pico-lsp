# M19 — LSP and Glyph Public Packages

**Status:** Spec'd — implementation not started  
**Depends on:** M18  
**Release target:** v1.8.0  
**Blocks:** pico-editor M5  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

External Go programs can run pico-lsp in-process over any `jsonrpc2.Stream` and
use the public glyph catalog without importing `internal/lsp` or
`internal/glyphs`. pico-editor M5 creates a server with its own logger, connects
it over `net.Pipe`, and speaks LSP exactly as a stdio client would.

**Demo:** A consumer creates `lsp.New(lsp.Options{Logger: logger})`, builds a
`net.Pipe`, calls `Serve(ctx, jsonrpc2.NewStream(serverPipe))`, and drives
`initialize`, `didOpen`, diagnostics, semantic tokens, and `shutdown` from the
client side. The same program calls `glyphs.Builtins()` and
`glyphs.FindByName` to render a picker. It imports only `pkg/lsp` and
`pkg/glyphs`.

## Decisions

Confirmed before implementation:

| ID | Decision | Choice |
|---|---|---|
| D1 | Server surface | Minimal: `Options{Logger}`, `New`, and `Serve(ctx, stream)`. |
| D2 | Transport ownership | The caller supplies the stream. `pkg/lsp` does not create pipes, TCP listeners, or stdio transports. |
| D3 | Configuration | LSP-native only. Configuration arrives through `initializationOptions` and `workspace/configuration`; no public config types. |
| D4 | Lifecycle | The returned `jsonrpc2.Conn` plus the caller's context are the lifecycle API. No exit-signal helpers. |
| D5 | Glyph catalog | `pkg/glyphs` re-exports the catalog types and lookup/build functions. The LSP `pico8/glyphs` request remains the editor picker path. |
| D6 | Compatibility | Stable semver from v1.8.0; additive changes only. |
| D7 | Logging | The server logs only to the supplied logger. It never writes to stdout. A nil logger is silent. |

## Scope

### In scope

- `pkg/lsp` server facade over `internal/lsp`.
- `pkg/glyphs` catalog facade over `internal/glyphs`.
- Godoc and runnable examples for both packages.
- An in-memory end-to-end test of the public server.
- An external consumer module that imports only `pkg/lsp` and `pkg/glyphs`.
- The v1.8.0 changelog entry, tag, and release notes.

### Out of scope

- Public config manager, `config.Settings`, or other internal config types.
- `ExitSignal`, `ExitCode`, `Exited`, `ShuttingDown`, or stdio lifecycle
  helpers.
- A `net.Pipe` or in-process transport helper.
- Public custom request constants or response structs; `docs/api.md` remains
  the protocol contract.
- An LSP client implementation for consumers.
- Editor UI, glyph picker UI, or pico-editor code.
- `.p8.png`, cart API changes, or new language features.

## Deliverables

1. `pkg/lsp/server.go` with `Options`, `Server`, `New`, and `Serve`.
2. `pkg/glyphs/glyphs.go` with type aliases, constants, and forwarding
   functions.
3. Godoc examples for the server and catalog.
4. An in-memory LSP test using `net.Pipe`.
5. An external consumer module under `testdata/publicapi`.
6. `CHANGELOG.md` entry and `v1.8.0` release.

## Detailed requirements

### 1. Public server

```go
package lsp

type Options struct {
    Logger *slog.Logger
}

type Server struct { /* opaque */ }

func New(opts Options) *Server
func (s *Server) Serve(ctx context.Context, stream jsonrpc2.Stream) jsonrpc2.Conn
```

- `New` always returns a usable server. The zero `Options` is valid.
- A nil `Logger` uses `slog.New(slog.DiscardHandler)`. A library must not write
  to stderr or stdout unless the caller supplies a logger.
- `New` constructs the internal server with default configuration. The editor
  supplies configuration through the LSP `initialize` request and
  `workspace/configuration`; `pkg/lsp` exposes no internal config types.
- `Serve` starts one connection. It returns the `jsonrpc2.Conn` so the caller
  can wait for `Done`, inspect `Err`, and close it. One `Server` serves one
  connection; create a new `Server` for another connection.
- The server advertises the same capabilities as the stdio server. No feature is
  disabled in-process.
- The server's version in `ServerInfo` comes from `internal/version`.
- The server never writes to stdout. All logging goes through `Options.Logger`.
- Cancelling `ctx`, closing the connection, or `shutdown`/`exit` follows the
  same lifecycle as the stdio server.
- `pkg/lsp` must not expose `internal/lsp.Server`, `internal/config.Manager`, or
  internal request/response types in any signature.

### 2. Public glyph catalog

```go
package glyphs

type Context = internalglyphs.Context
type Definition = internalglyphs.Definition
type Entry = internalglyphs.Entry

const (
    ContextBoth   = internalglyphs.ContextBoth
    ContextString = internalglyphs.ContextString
)

func Builtins() []Entry
func Build(custom []Definition) ([]Entry, []string)
func NormalizeDefinition(definition Definition) (Entry, error)
func FindByName(entries []Entry, name string) (Entry, bool)
func FindByByte(entries []Entry, value byte) (Entry, bool)
```

- The aliases expose only stdlib fields and methods. `Entry` and `Definition`
  are safe for external use; their JSON tags are part of the public contract.
- `Builtins` returns a copy; callers may mutate the slice without affecting the
  catalog.
- `Build` preserves the current merge/override behavior and returns the warning
  strings unchanged.
- The LSP `pico8/glyphs` request remains the canonical picker path for
  pico-editor M5. `pkg/glyphs` is the direct Go catalog. Both read the same
  `internal/glyphs` data; there is no second catalog.
- Do not expose internal glyph storage or unexported normalization helpers.

### 3. Testing

- In-memory server test:
  - create `net.Pipe`, `lsp.New`, and `Serve`;
  - initialize with UTF-16 and a workspace folder;
  - open a `.p8` cart;
  - assert diagnostics and semantic tokens are delivered;
  - send `shutdown` and `exit`;
  - assert the connection closes without a panic or deadlock.
- Logging test: a nil logger produces no output; a supplied logger receives
  records; nothing is written to stdout.
- Concurrency test: run the server and client with `-race`, including
  overlapping requests and cancellation.
- Glyph tests: aliases expose the same values as the internal package; built-in
  lookup by name and byte; custom build/override behavior; returned slices are
  copies.
- External consumer module imports only `pkg/lsp` and `pkg/glyphs` and runs a
  minimal initialize/shutdown round-trip.
- Fuzz or stress the connection with malformed JSON-RPC frames; the server must
  not panic or hang.

### 4. Documentation and release

- Godoc documents the caller-owned stream, one-connection-per-server rule,
  logger behavior, and the fact that configuration is LSP-native.
- Add a short in-process example to `docs/api.md` or `README.md`.
- Document that `pico8/cartInfo` and `pico8/glyphs` remain protocol requests,
  not Go APIs.
- Add `CHANGELOG.md` under `Unreleased`, then cut `v1.8.0` after the PR merges.
- Release notes state that this is additive and does not change the stdio
  server or its capabilities.

## Interfaces and state

- `Server` owns the internal server and the logger. It has no package-level
  mutable state.
- `Serve` returns a connection; the caller owns its lifetime.
- `pkg/glyphs` functions are pure over their inputs; `Builtins` returns a copy.

## Error handling

- `New` and `Serve` do not return construction errors; protocol errors travel
  through the connection.
- A nil stream or context is a programmer error. Document that both must be
  non-nil and that violating this may panic; do not add a public error path for
  it.
- A panic inside the internal server is already recovered by its request
  handler; `pkg/lsp` must not remove that protection.
- Malformed frames close the connection; they never crash the process.

## Testing and CI

- `go test ./...`, `go test -race ./...`, `go vet`, `gofmt`, and
  `golangci-lint run` pass.
- The public server e2e test runs in CI.
- The external consumer module builds with `GOFLAGS=-mod=readonly`.

## Exit criteria

- [ ] `pkg/lsp` exposes only `Options`, `Server`, `New`, and `Serve`.
- [ ] `pkg/glyphs` exposes the catalog types, constants, and functions without
      internal types in signatures.
- [ ] An in-memory LSP client completes initialize, open, diagnostics,
      semantic tokens, and shutdown.
- [ ] The server writes nothing to stdout and respects the supplied logger.
- [ ] `go test -race` passes for the public server and catalog.
- [ ] The external consumer module imports only public packages.
- [ ] `v1.8.0` is tagged and pico-editor M5 can pin it.
- [ ] No existing stdio server behavior or LSP capability changed.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Public server leaks internal config types | New `Options` wrapper; no internal types in exported signatures. |
| In-process lifecycle differs from stdio | Reuse the internal server; e2e test with `net.Pipe`; no separate code path. |
| Logging corrupts a protocol stream | Logger-only API; no stdout writes; capture and assert in tests. |
| Glyph catalog drifts from the LSP request | Both read `internal/glyphs`; alias-focused tests and one shared fixture. |
| Stable API locks lifecycle details | Keep the surface minimal; lifecycle helpers can be added additively later. |
