# M11 — Pull Diagnostics

**Status:** Planned  
**Depends on:** M10  
**Release target:** v1.2.0 (batched with M12–M14)  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

Clients that support LSP pull diagnostics can request diagnostics on demand
instead of relying on pushed `textDocument/publishDiagnostics` notifications.
The server produces one canonical diagnostic set per cart snapshot, returns
related-file reports for includes, preserves `pico8.maxProblems`, and falls back
to push diagnostics for clients without pull support.

**Demo:** A client advertising `textDocument.diagnostic` opens a cart and calls
`textDocument/diagnostic`. The response is a full report for the cart with
related reports for changed include files. No duplicate push notification is
published for the same snapshot. A client without pull support still receives
normal pushed diagnostics.

## Scope

### In scope

- Capability negotiation for `textDocument.diagnostic` and optional
  `workspace/diagnostic`.
- Full document reports for the requested document, with stable `resultId`
  values tied to the analyzed cart snapshot.
- `relatedDocuments` for included files, tabs, virtual `.p8.png` documents, and
  any other file whose diagnostics changed for the same cart snapshot.
- Optional bounded `workspace/diagnostic`, limited to open carts or active
  workspace folders.
- Configuration selecting `auto`, `push`, or `pull` behavior.
- Deduplication so a pull-capable client does not receive duplicate push
  notifications for the same diagnostic snapshot.
- Cancellation and error responses that do not crash or hang the server.
- Documentation, configuration, and LSP tests.

### Out of scope

- Changes to the diagnostic rules themselves.
- Pull-diagnostic streaming/partial result progress unless required by a client
  capability.
- Workspace-wide scans of unopened carts.
- API version selection and editor-specific clients.

## Requirements

- Detect pull support from client capabilities. A client without
  `textDocument.diagnostic` must continue to receive pushed diagnostics exactly
  as it does in v1.1.
- `pico8.diagnostics.mode`:
  - `auto` (default): pull when the client supports it, otherwise push.
  - `push`: always publish; pull requests still answer correctly.
  - `pull`: only answer pull requests; do not publish identical snapshots.
- `textDocument/diagnostic` returns a full report for the requested document.
  Unknown or unopened documents return a JSON-RPC error rather than an empty
  report that hides a client bug.
- `relatedDocuments` contains full reports for other files whose diagnostics
  changed in the same cart snapshot. It must not recurse indefinitely through
  includes or duplicate the requested document.
- `workspace/diagnostic` is optional. It is bounded to open carts and active
  workspace folders, cancellable, and returns a partial/full result shape that
  clients can handle.
- Result IDs are deterministic for a cart snapshot and configuration. A
  `previousResultId` matching the current snapshot may return an unchanged
  report; a stale or unknown ID returns a full report.
- `pico8.maxProblems` truncation is applied per original document, matching
  push behavior. Related reports use the same cap semantics.
- Include and virtual-document URI mapping preserves original file identities;
  diagnostics never move to the flattened cart URI when they belong to an
  include.
- Existing `---@diagnostic` suppression and diagnostic severity configuration
  apply unchanged to pull reports.
- A push/pull transition (configuration change or client capability change)
  produces exactly one authoritative diagnostic set for the next snapshot.
- Logging goes to stderr/`--log-file`; pull handling never writes to stdout
  outside LSP framing.

## Configuration

Add `pico8.diagnostics.mode` with `auto`, `push`, and `pull`, defaulting to
`auto`. The existing `pico8.maxProblems` and diagnostic severity settings remain
authoritative. Update the configuration contract and examples in
[ROADMAP.md](ROADMAP.md) and [configuration.md](configuration.md).

## Testing

- Unit tests for full/unchanged report construction, result-ID derivation,
  related-document collection, truncation, and mode/capability decisions.
- In-memory LSP e2e tests for:
  - pull-capable clients in `auto` and `pull` modes;
  - push-only clients;
  - `relatedDocuments` for includes and tabs;
  - no duplicate push notifications in auto/pull modes;
  - `previousResultId` unchanged and stale paths;
  - `workspace/diagnostic` bounds, cancellation, and partial results;
  - unknown/unopened document errors.
- Golden tests for report JSON structure.
- Corpus tests confirming pull and push reports contain identical diagnostics.
- Performance test for a 64 KB cart: diagnostic pull after cache stays within
  the existing editor-request budget.

## Exit criteria

- [ ] Pull-capable clients receive full document reports and related documents.
- [ ] Push-only clients keep the v1.1 behavior.
- [ ] `auto`, `push`, and `pull` modes work without duplicate publication.
- [ ] Result IDs, truncation, and suppression behave deterministically.
- [ ] Optional workspace diagnostics are bounded and cancellable.
- [ ] Tests, configuration docs, and the diagnostics reference are updated.
- [ ] Existing v1/v1.1 diagnostics and performance budgets do not regress.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Pull and push both publish the same diagnostics | One snapshot-owned report path, mode tests, and e2e assertions that count notifications. |
| Related documents duplicate or leak diagnostics | Deterministic per-file report map and original URI ownership tests. |
| Workspace diagnostics scan too much | Bound to open carts/active folders, cancellation, and partial reports. |
| Result IDs become stale after configuration changes | Include snapshot/config generation in the ID and test stale-ID fallback. |
| Capability handling diverges between clients | Capability-driven unit tests plus a push-only fallback path. |
