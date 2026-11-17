# M0 — Foundation & Walking Skeleton

**Status:** Done  
**Depends on:** none  
**Blocks:** M1  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

A real LSP process that starts, speaks the protocol over stdio, tracks open documents, converts positions correctly, logs safely, and survives malformed requests. It does not understand PICO-8 yet; it is the skeleton every later milestone builds on.

**Demo:** Open a `.p8` file in Neovim and Helix. `:checkhealth vim.lsp` / `hx --health pico8` shows `pico-lsp` attached with document sync and position encoding. Editing, saving, and closing the file produces no errors and no protocol corruption.

## Scope

### In scope

- Go module wiring and dependency selection.
- `cmd/pico-lsp` entry point and flags.
- LSP lifecycle over stdio.
- Document store with full and incremental sync.
- Byte-offset ↔ LSP position conversion, including UTF-16.
- Configuration scaffold.
- Logging, tracing, panic recovery, cancellation.
- Diagnostic publication abstraction.
- Minimal in-memory LSP integration test harness.
- Neovim and Helix smoke-test instructions.

### Out of scope

- Parsing PICO-8 code.
- Syntax diagnostics.
- Cart/include handling.
- Completion, hover, definition, or any semantic feature.
- Workspace scanning.
- Packaging/release artifacts.

## Deliverables

1. `pico-lsp` binary that starts and serves LSP on stdin/stdout.
2. `initialize`/`initialized`/`shutdown`/`exit` implemented correctly.
3. `textDocument/didOpen`, `didChange`, `didSave`, `didClose` maintaining versions and text.
4. `source.LineIndex` with round-trip position conversion.
5. `internal/config` with defaults and `workspace/configuration` support.
6. Structured logging to stderr or a file; never stdout.
7. Panic recovery around every request; server stays alive.
8. End-to-end tests using an in-memory pipe.
9. Neovim and Helix setup docs under `editors/`.

## Detailed requirements

### 1. Process and transport

- Read/write LSP over stdin/stdout using `jsonrpc2.NewStream` (LSP `Content-Length` framing).
- Default transport is stdio. Do not add TCP in v1 unless needed for debugging; if added, it must be behind an explicit flag.
- All non-protocol output goes to stderr or a log file. A single `fmt.Println` is a release-blocking bug.
- `--log-file PATH` writes structured logs to a file.
- `--log-level debug|info|warn|error` controls verbosity; default `info`.
- `--version` prints the binary version and exits.
- Handle `SIGINT`/`SIGTERM` by closing the connection and exiting cleanly.

### 2. LSP lifecycle

`initialize` returns:

```go
InitializeResult{
    Capabilities: ServerCapabilities{
        TextDocumentSync: &TextDocumentSyncOptions{
            OpenClose: true,
            Change:    protocol.TextDocumentSyncKindIncremental,
            Save:      &SaveOptions{IncludeText: false},
        },
        PositionEncoding: protocol.PositionEncodingKindUTF16,
        Workspace: &WorkspaceServerCapabilities{
            WorkspaceFolders: &WorkspaceFoldersServerCapabilities{
                Supported: true,
            },
        },
    },
    ServerInfo: &ServerInfo{Name: "pico-lsp", Version: version},
}
```

> Implementation note: with `go.lsp.dev/protocol` v1.0.1, fields such as
> `TextDocumentSync`, `Save`, and `Workspace` are union or optional wrappers and
> `ServerInfo` is a value type. The concrete construction lives in
> `internal/lsp/server.go`.

- Advertise only capabilities that are actually implemented. Later milestones add providers.
- Read `general.positionEncodings` from client capabilities. Prefer `utf-8` if offered, otherwise `utf-16`. The chosen encoding is returned in `positionEncoding` and used for all conversions.
- `initialized` performs no heavy workspace scan. Background indexing belongs to later milestones.
- `shutdown` returns success and marks the server as shutting down.
- `exit` exits with code `0` if shutdown was received, otherwise `1`.
- Unknown requests return `MethodNotFound`; unknown notifications are ignored.
- `$/cancelRequest` is accepted and ignored in M0. Most handlers are short-lived and run in order, but `initialized` releases the read loop with `jsonrpc2.Async` while pulling configuration, so later notifications can overlap with that pull. Revisit cancellation when long-running or concurrent handlers exist.

### 3. Document store

- Key documents by URI. Accept `file://` URIs; reject or ignore non-file schemes with a clear log message.
- Store `URI`, `LanguageID`, `Version`, `Text`, and a `LineIndex`.
- `didOpen` replaces any cached text and marks the document open.
- `didChange` supports incremental ranges as advertised. Handle multiple changes in one notification in order.
- If a client sends a full-text change despite incremental advertisement, accept it defensively.
- `didSave` is a no-op unless `text` is included; if included, update the overlay.
- `didClose` removes the open overlay. If the file still exists on disk, later analysis may fall back to disk content; if it does not, mark it missing.
- Guard all document state with a mutex or a single-writer event loop. Requests must see a consistent snapshot.
- Maintain a monotonically increasing document version. Reject `didChange` notifications whose version is not greater than the current version, log a warning, and keep the prior snapshot; diagnostics carry the version when the protocol allows it.

### 4. Position mapping

Implement `internal/source.LineIndex`:

```go
type LineIndex struct {
    text       []byte
    lineStarts []int
}

func (l *LineIndex) Offset(pos protocol.Position) (int, error)
func (l *LineIndex) Position(offset int) (protocol.Position, error)
func (l *LineIndex) Range(start, end int) protocol.Range
```

Requirements:

- Internally use byte offsets. Convert only at the LSP boundary.
- LSP columns are UTF-16 code units by default. Use `unicode/utf16` or a rune scan; never use `len(string)` as a column.
- Correctly handle:
  - LF, CRLF, and lone CR line endings.
  - Multi-byte UTF-8.
  - Surrogate pairs (emoji, PICO-8 glyphs).
  - Columns past end-of-line: clamp to line length, matching common LSP client behavior.
  - Empty files and files without a trailing newline.
- Rebuild the line index after each accepted change. Carts are small; O(n) rebuild is fine.
- Add property-style tests that round-trip every byte boundary and every line/column pair for representative texts.
- If UTF-8 position encoding is negotiated, columns are bytes; keep the conversion strategy centralized so features never branch on encoding.

### 5. Configuration scaffold

Create `internal/config`:

```go
type Settings struct {
    APIVersion    string                    `json:"apiVersion"`
    TypeChecking  string                    `json:"typeChecking"`
    InlayHints    bool                      `json:"inlayHints"`
    MaxProblems   int                       `json:"maxProblems"`
    Diagnostics   DiagnosticSettings        `json:"diagnostics"`
}

type DiagnosticSettings struct {
    UndefinedGlobal string `json:"undefinedGlobal"`
    UnusedLocal     string `json:"unusedLocal"`
}
```

Defaults:

- `apiVersion: "0.2.7"`
- `typeChecking: "basic"`
- `inlayHints: false`
- `maxProblems: 100`
- `undefinedGlobal: "warning"`
- `unusedLocal: "off"`

`maxProblems: 0` disables truncation; negative values fall back to the default.

Behavior:

- If the client supports `workspace/configuration`, request section `pico8` after `initialized` and on `workspace/didChangeConfiguration`.
- Otherwise use `initializationOptions.pico8` if present.
- Otherwise use defaults.
- Pulled values override initialization options per field. Fields absent from a pulled section fall back to initialization options, then defaults. A null or missing section value leaves the initialization-options settings in place.
- Unknown keys are ignored; invalid enum values fall back to defaults and log a warning.
- Do not block `initialize` on configuration requests.

### 6. Logging and tracing

- Use `log/slog` with a text or JSON handler.
- Log at `info` for lifecycle events, `debug` for document changes, `warn` for recoverable protocol/config issues.
- Include request method and duration at debug level.
- On panic in a handler:
  - Recover.
  - Log the stack trace and request method.
  - Return `InternalError` for requests; ignore the notification after logging.
  - Never terminate the server.
- Support `$/setTrace` by storing the trace level; detailed trace logging can be minimal in M0.
- Use `window/logMessage` only for messages the user should see (e.g., unsupported API version later). Routine logs stay out of the UI.

### 7. Diagnostics publisher

Create a small abstraction now so later milestones do not all call the client directly:

```go
type Diagnostics struct {
    Client protocol.Client
}

func (d *Diagnostics) Publish(ctx context.Context, uri uri.URI, version *int32, items []protocol.Diagnostic)
func (d *Diagnostics) Clear(ctx context.Context, uri uri.URI)
```

- Publish empty diagnostics to clear a URI.
- Deduplicate identical consecutive publishes.
- Never publish before `initialized`.
- Respect `pico8.maxProblems`; truncate deterministically and log when truncating.
- Diagnostic conversion helpers belong in `internal/lsp` so ranges always use `LineIndex`.

### 8. Placeholder handlers

Add stubs for the capabilities later milestones will implement. They should return empty results and be individually replaceable:

- `textDocument/documentSymbol`
- `textDocument/definition`
- `textDocument/references`
- `textDocument/hover`
- `textDocument/completion`
- `textDocument/signatureHelp`
- `textDocument/foldingRange`
- `textDocument/semanticTokens/full`
- `textDocument/codeAction`
- `textDocument/codeLens`
- `workspace/symbol`
- `workspace/executeCommand`

Do not advertise them until implemented, except where a placeholder is useful for internal testing.

### 9. Editor smoke tests

Provide:

- `editors/nvim/README.md` with Neovim 0.11+ config using `vim.lsp.config` and `vim.lsp.enable`.
- `editors/helix/README.md` with a `languages.toml` snippet.
- A tiny `testdata/smoke.p8` cart.
- Instructions to run `pico-lsp` directly and inspect `:checkhealth vim.lsp` / `hx --health pico8`.

## Architecture and files

```
cmd/pico-lsp/main.go              flags, logging setup, stdio stream, server run
internal/lsp/server.go            Server struct, lifecycle, capabilities
internal/lsp/handlers.go          request/notification handlers
internal/lsp/documents.go         document store and incremental edits
internal/lsp/diagnostics.go       diagnostics publisher
internal/lsp/positions.go         span/range helpers
internal/source/lineindex.go      byte ↔ position conversion
internal/config/config.go         settings, defaults, pull/fallback
internal/version/version.go       version injection
```

## Interfaces and state

```go
type Server struct {
    protocol.UnimplementedServer

    client atomic.Value // protocol.Client
    docs   *DocumentStore
    cfg    *config.Manager
    diags  *Diagnostics
    log    *slog.Logger
    trace  protocol.TraceValue
}

type Document struct {
    URI        uri.URI
    LanguageID string
    Version    int32
    Text       string
    Index      *source.LineIndex
}
```

Requests take a consistent snapshot from the document store and never mutate it. The client dispatcher is installed in an `atomic.Value` before the connection starts reading, so handlers never race with connection setup.

## Error handling

- Invalid params: return `InvalidParams`, log at debug.
- Unknown document: for notifications, log and ignore; for requests, return an empty/nil result where the protocol allows, otherwise `InvalidParams`.
- Out-of-range positions: clamp for diagnostics/hover lookups; never panic.
- I/O errors on stdio: log and exit non-zero.
- Panic recovery is mandatory for every handler.

## Testing

### Unit tests

- `LineIndex` round-trip for ASCII, UTF-8, emoji, P8 glyphs, CRLF, lone CR, empty lines, trailing newline.
- Incremental edit application for insert, delete, replace, multi-edit, and edits at EOF.
- Configuration precedence: pull config > initializationOptions > defaults.
- Diagnostic truncation and deduplication.

### End-to-end tests

Use `net.Pipe` with `protocol.NewServer`/`protocol.NewClient` or the equivalent JSON-RPC pipe API:

1. Send `initialize`; assert capabilities and position encoding.
2. Send `initialized`.
3. `didOpen` a document; assert no error.
4. `didChange` with an incremental edit; assert the stored text.
5. Send an unknown request; assert `MethodNotFound`.
6. Send `shutdown`, then `exit`.

Add a test that captures stdout with an `io.Pipe` and asserts every write is a valid LSP frame. Run with `-race`.

### Manual smoke test

- Neovim 0.11+: attach, edit, save, close, reopen.
- Helix: attach and inspect logs.
- Kill the server while the editor is running; the editor should report a clean exit, not a hang.

## Exit criteria

- [x] `pico-lsp` builds with `CGO_ENABLED=0`.
- [x] `initialize`/`shutdown`/`exit` comply with the LSP lifecycle.
- [x] Incremental document sync is correct for insert/delete/replace.
- [x] UTF-16 position conversion passes round-trip tests.
- [x] No stdout writes occur outside the JSON-RPC stream.
- [x] Panics are recovered and logged; the server stays alive.
- [x] Configuration pull and fallback work.
- [x] Neovim and Helix attach successfully using the documented configs.
- [x] `go test -race ./...` passes.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| LSP library API churn | Pin the module version; keep protocol wiring isolated in `internal/lsp`. |
| stdout corruption | Single writer abstraction; test capturing stdout frames; never use `fmt.Print*` in server code. |
| UTF-16 bugs | Centralize conversion in `LineIndex`; property tests; test P8 glyphs explicitly. |
| Request races with document changes | Snapshot documents under lock; parse outside the lock; version all snapshots. |
| Panic in a handler kills the editor session | Recover in a middleware wrapper around every handler. |
