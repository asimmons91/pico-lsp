# M33 — Standard LSP Hooks for Neovim

**Status:** Done (2026-11-24); shipped in v1.9.0 (2026-11-25). Implemented in
`internal/cart` and `internal/lsp`; the canonical Neovim client is
[`pico8.nvim`](https://github.com/asimmons91/pico8.nvim).

**Depends on:** M7 (formatter), M8 (`.p8.png` virtual documents), M11 (pull
diagnostics), M14 (code actions), M25 (Neovim test job and config), M30
(in-editor include navigation fallback)

**Release target:** Shipped in v1.9.0.

**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

The server exposes three standard LSP capabilities that Neovim can consume
without custom integration:

1. `textDocument/rangeFormatting` lets Neovim set `formatexpr` and makes `gq`
   invoke the PICO-8 formatter on the selected lines.
2. `textDocument/documentLink` makes `#include` paths clickable/openable with
   Neovim 0.12's `gx` and other clients' document-link UI.
3. `workspace/codeLens/refresh` lets the server refresh the cart-budget lens
   after cart changes on clients that advertise refresh support.

All behavior is standard LSP and remains available to every client; no
Neovim-specific server branch is added.

**Demo:** In Neovim 0.11+, select a few lines in a cart and press `gq`; the
server returns edits for those lines only. A range that would change the line
count is refused; Neovim's `formatexpr` may ignore the error, so use a
whole-document format command when that happens. In Neovim 0.12+, put the cursor
on an `#include` path and press `gx`; the target is passed to `vim.ui.open` and
the system opener. M30's `gf` remains the in-editor path. On a client that
supports code-lens refresh, the budget lens updates after a cart edit without a
manual refresh.

## Decisions proposed

| ID | Decision | Choice |
|---|---|---|
| D1 | Range formatting scope | Format complete lines covered by the request. Return edits only for those lines. |
| D2 | Line preservation | Format the full document for context, but return a `RequestFailed` error when the formatted document changes the line count. Neovim's `formatexpr` may swallow that error; the server contract is still an error rather than a partial edit. |
| D3 | Range boundaries | Expand a partial-line range to whole lines and clamp it to the document. An empty expanded range returns no edits. Invalid params and missing/unopened documents are errors; the refusals in D4 remain separate errors. |
| D4 | Syntax and includes | Reuse document-formatting safety: syntax errors, missing/unreadable includes, and `.p8.png` virtual documents are refused with the existing error style. |
| D5 | Document links | Return links for resolved `#include` directives in the requested file. Skip missing, unreadable, and nested includes. Do not add a resolve provider. |
| D6 | Link range | Use the directive's path token range, including quotes if that is the parser's existing span; the link target is the resolved file URI. |
| D7 | Tab selectors | The link target is the file URI without a fragment. A tooltip mentions the selected tab when the directive has a `:<hex>` suffix. |
| D8 | Code-lens refresh | Advertise no new server capability; inspect `workspace.codeLens.refreshSupport` from `initialize` and call `workspace/codeLens/refresh` only when supported. |
| D9 | Refresh throttling | Coalesce refreshes per cart and do not send one for every analysis pass. A failed refresh is logged and does not affect analysis or diagnostics. |
| D10 | No behavior changes | Document formatting, diagnostics, analysis, and code-action behavior are unchanged. |

Implementation decisions resolved during delivery:

- Document-link ranges use a new parser-owned `PathSpan` on `cart.Include`, so
  quoted and unquoted targets are linked precisely without making the whole
  directive clickable.
- Range formatting mirrors document formatting: only the cart root is refused
  when the cart has a missing or unreadable include, so an included `.lua` file
  can still be formatted when a sibling include is broken.
- No code-lens refresh is scheduled while `pico8.limits.codeLens` is false; the
  budget lens does not exist, so a refresh cannot change it.

## Scope

### In scope

- `DocumentRangeFormattingProvider` capability and handler.
- Conservative line-range formatting using the existing formatter.
- `DocumentLinkProvider` capability and handler for `#include` directives.
- Optional `workspace/codeLens/refresh` after cart changes.
- LSP e2e tests for all three capabilities.
- README, API, and capability documentation.

### Out of scope

- Changing formatter output, options, or token/comment safety.
- Formatting non-Lua cart sections.
- Range formatting across a line-count-changing edit.
- Editing include paths, following links, or opening a tab directly.
- New diagnostic behavior.
- New code-lens resolution logic.
- Neovim-only protocol extensions or custom requests.
- VS Code client changes; it may consume the capabilities automatically.

## Deliverables

1. `internal/lsp/formatting.go` or a new `range_formatting.go` — capability and
   handler.
2. `internal/lsp/document_links.go` — include link handler and capability.
3. Server capability wiring in `internal/lsp/server.go`.
4. Code-lens refresh support in the cart analysis/refresh path.
5. LSP e2e tests and golden fixtures where useful.
6. `docs/api.md` — standard LSP capability section.
7. `README.md` and `editors/nvim/README.md` — `gq`/`gx` notes.
8. `docs/ROADMAP.md` and `docs/post-v1-backlog.md` — milestone status.

## Detailed requirements

### 1. Range formatting

Add `DocumentRangeFormattingProvider: protocol.Boolean(true)` and implement:

```go
func (s *Server) RangeFormatting(
    ctx context.Context,
    params *protocol.DocumentRangeFormattingParams,
) ([]protocol.TextEdit, error)
```

Requirements:

- Reject nil params, an empty document URI, and unknown/unopened documents
  with `InvalidParams` or the existing navigation error. Do not copy document
  formatting's `nil, nil` success path for these cases.
- Refuse virtual `.p8.png` documents and `.p8.png` source URIs with the same
  messages as document formatting.
- Refuse a cart with missing or unreadable includes.
- Use the same settings and formatting options as document formatting:
  `TabSize`, `InsertSpaces`, trailing-whitespace/final-newline options, and
  `pico8.format.maxBlankLines`.
- Format the full document to preserve block context.
- If formatting returns `ErrSyntax`, return the existing "fix the errors first"
  error.
- Compare the original and formatted line counts. If they differ, return a
  `RequestFailed` error explaining that range formatting would change the line
  count and that the user should format the whole document. Do not return a
  partial edit. Neovim's `formatexpr` may not display the server error; the
  response must still be correct for clients that do.
- Expand the requested LSP range to complete lines:
  - start line = requested start line;
  - end line = requested end line, inclusive when the end character is greater
    than zero;
  - clamp to the document.
- If the expanded range is empty, return an empty edit list.
- If the formatted lines for the range equal the original lines, return an
  empty edit list.
- Return one `TextEdit` covering the expanded range with the corresponding
  formatted lines. Do not return edits outside the requested range.
- Preserve trailing newline/EOL behavior: the edit must not add or remove the
  final newline unless the selected range includes the final line and the
  formatter option explicitly requests it. Prefer returning no edit when the
  only difference is outside the selected lines.
- Non-Lua cart sections produce no edits because the formatter preserves them;
  do not error merely because the range is in `__gfx__` or another section.
- Range formatting never writes to disk.

Neovim behavior to verify:

- Neovim sets `formatexpr` when the server advertises range formatting.
- `gq` on selected lines sends `textDocument/rangeFormatting`.
- `gq` on a single line formats only that line.
- A line-count-changing document returns a `RequestFailed` response and no
  partial edit. Neovim may silently do nothing; document the whole-document
  workaround.

### 2. Document links

Add `DocumentLinkProvider: &protocol.DocumentLinkOptions{}` and implement
`textDocument/documentLink`.

Requirements:

- Load the resolved cart snapshot for the requested document.
- Return links only for directives whose `DirectiveSpan` belongs to the
  requested file. Do not return links from included files in the same response.
- For each resolved include with a file URI:
  - set `Range` to the directive/path span already tracked by the cart model;
  - set `Target` to the resolved URI;
  - set a short tooltip such as `Open lib/player.lua` or
    `Open lib/player.lua (tab 2)`.
- Skip:
  - missing includes;
  - unreadable includes;
  - nested includes that PICO-8 does not follow;
  - duplicate links for the same range and target.
- Do not add a resolve provider; targets are known immediately.
- Return an empty slice for documents with no includes and for `.p8.png`
  virtual documents.
- Keep URI encoding consistent with the existing location/definition code.
- Bound the result to a reasonable number of directives; carts are small, but
  avoid unbounded allocation from malformed input.

Neovim 0.12 maps `gx` to document links when available. The default handler
passes the target to `vim.ui.open`, which uses the system opener rather than
opening a Neovim buffer; M30's `gf` remains the in-editor path. Verify `gx` by
stubbing `vim.ui.open` or by asserting the `textDocument/documentLink` result.
Neovim 0.11 has no document-link support in `gx`.

### 3. Code-lens refresh

- Read `params.Capabilities.Workspace.CodeLens.RefreshSupport` during
  `initialize` and store it with the other client capabilities.
- After a cart analysis that can change the budget lens (open, change, save,
  include change, settings change), schedule at most one
  `workspace/codeLens/refresh` request per cart within a short throttle window
  (for example 250 ms).
- Use the client's `CodeLensRefresh` method with a detached, bounded context so
  the call cannot block the analysis loop.
- If the client did not advertise refresh support, do nothing.
- Log refresh failures at debug or warn level; never fail the analysis request
  or retry aggressively.
- Do not send refresh while shutting down.
- Do not add a server capability for refresh; it is a client capability.

### 4. Documentation

- Add a "Standard LSP capabilities" subsection to `docs/api.md` listing range
  formatting, document links, and code-lens refresh, with client expectations.
- Document in `editors/nvim/README.md`:
  - `gq` works when the server advertises range formatting and Neovim sets
    `formatexpr`;
  - `gx` on an include path uses Neovim 0.12's document-link handling and
    `vim.ui.open` (system opener); it does not open an editor buffer;
  - `gf` remains the in-editor fallback and is M30.
- Update the root README feature list only if it claims formatting or include
  navigation.
- Note the line-count restriction for range formatting so users know when to
  format the whole document.

## Interfaces and state

- New standard capabilities:
  - `documentRangeFormattingProvider: true`
  - `documentLinkProvider: {}`
- New server-side client capability state: code-lens refresh support.
- New handlers: `RangeFormatting`, `DocumentLink`.
- No new custom request, command, setting, or diagnostic code.
- No change to existing document formatting or include resolution.

## Error handling

- Invalid params: return `InvalidParams` consistently with existing handlers.
- Unknown/unopened document: return `InvalidParams` or `RequestFailed` as
  appropriate, not an empty success that hides a client bug.
- Syntax errors: return `RequestFailed` with the existing wording.
- Missing includes: refuse cart formatting as today.
- Line-count-changing range: return `RequestFailed` with a message that names
  the full-document workaround.
- Document links never fail the whole response for one bad include; skip it.
- Code-lens refresh failures are logged and ignored.

## Testing and CI

- LSP e2e tests:
  - initialize advertises range formatting and document links;
  - range formatting a whole-line selection returns edits inside the range;
  - range formatting a single line works;
  - a partial-line range expands to whole lines;
  - an empty range returns no edits;
  - a document whose formatted line count differs returns the documented error
    and no edits;
  - nil params, an empty URI, and an unknown/unopened document return errors,
    not empty success;
  - syntax errors and missing includes are refused;
  - `.p8.png` virtual documents are refused;
  - document links for resolved includes have the expected range and target;
  - missing/unreadable/nested includes produce no link;
  - links are filtered to the requested file;
  - a Neovim 0.12 `gx` smoke check stubs `vim.ui.open` and asserts the expected
    target, or the more portable check asserts the `documentLink` response
    directly;
  - code-lens refresh is requested only when the client advertises support;
  - refresh is throttled and does not run after shutdown.
- Golden tests for link ranges in quoted, unquoted, and tab-suffixed includes.
- Existing formatter, navigation, and diagnostic tests remain green.
- `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt -l .`,
  `golangci-lint run ./...`, and `govulncheck ./...` remain green.
- Add a headless Neovim smoke check for `gq` (0.11+) and, on 0.12+ only, `gx`
  with a stubbed `vim.ui.open`, in the Neovim CI job. The canonical check is
  `pico8.nvim/tests/standard_hooks.lua`, run by its `tests/run.sh` on both
  supported Neovim versions.
- The `gq` smoke check includes a line-count-changing fixture and asserts the
  buffer is unchanged after the refusal; it must not apply a partial edit.

## Exit criteria

- [x] The server advertises and implements range formatting.
- [x] `gq` formats the selected lines and refuses unsafe line-count changes
      without applying a partial edit.
- [x] The server advertises and implements document links for resolved
      `#include` directives.
- [x] A Neovim 0.12 `gx` smoke check verifies the document link through a
      stubbed `vim.ui.open`; 0.11 uses M30's `gf`.
- [x] Code-lens refresh is sent only to capable clients and is throttled.
- [x] Existing formatting, navigation, and diagnostics behavior is unchanged.
- [x] API and Neovim docs describe the new capabilities.
- [x] LSP e2e tests and the Neovim smoke check pass.

## Risks

| Risk | Mitigation |
|---|---|
| Formatting the full document to format a range could apply unintended context | Return edits only for the requested complete lines and refuse line-count changes. |
| Neovim's `gq` expects range formatting semantics that differ from full-document formatting | Test `gq`, single-line, and whole-line selections headlessly. |
| Document-link ranges may include quotes or the whole directive | Use the parser-owned path span and add golden tests for quoted, unquoted, and tab-suffixed includes. |
| Code-lens refresh could create request loops | Throttle per cart and never trigger analysis from the refresh request. |
| Older clients do not support document links | Keep `gf` and other navigation as the fallback; document version requirements. |
