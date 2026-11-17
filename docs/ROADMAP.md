# PICO-8 Language Server — Roadmap

**Status:** v1.0.0 released  
**Last updated:** 2026-11-17  
**Module:** `github.com/asimmons91/pico-lsp`  
**Binary:** `pico-lsp`  
**Target PICO-8 API:** 0.2.7

## Vision

Build a fast, editor-agnostic PICO-8 language server in Go that:

- understands the real PICO-8 dialect, not a generic Lua approximation;
- understands carts, code tabs, and `#include` files as one program;
- provides the navigation and editor intelligence people expect from `pico8-ls`;
- adds practical LuaLS-style type annotations and gradual type inference;
- ships as a single static binary that is trivial to install in Neovim, Helix, and future editors.

The project is a **PICO-8 compiler front-end with an LSP shell**, not a generic Lua language server.

## Product principles

1. **PICO-8 first.** PICO-8 semantics win over generic Lua behavior. Includes are non-recursive, shorthand is line-bounded, and the standard Lua library does not exist.
2. **Low false positives.** Dynamic Lua defeats perfect static analysis. Prefer `any`, warnings, and silence over confident wrong errors.
3. **Gradual typing.** Types are optional. Annotated code gets stronger checking; unannotated code stays pleasant to edit.
4. **Editor-agnostic.** The server speaks standard LSP over stdio and has no editor-specific runtime dependencies.
5. **Simplicity over cleverness.** Carts are limited to roughly 64 KB of code. Full reparse and full re-analysis are fast enough. Do not build incremental parsing infrastructure until measurements demand it.
6. **Faithful but forgiving.** Parse a practical superset of the dialect when PICO-8's preprocessor is quirky, but diagnose definite incompatibilities.
7. **Zero-runtime distribution.** `CGO_ENABLED=0`, static binaries, no Node/Python/JVM requirement.

## Locked decisions

These decisions were made before milestone specs were written. Changing one requires updating this document and every affected milestone spec.

| Area | Decision | Notes |
|---|---|---|
| v1 scope | Core LSP + type features only | Formatter and `.p8.png` support are post-v1. |
| Editors | Neovim + Helix | VS Code and Zed clients are post-v1; the server remains editor-agnostic. |
| Neovim helper | Helper plugin, optional at runtime | Auto-attaches to `.lua` files owned by a cart; avoids attaching to unrelated Lua buffers. |
| Syntax highlighting | Vim syntax file + semantic tokens | Ship a small `.p8` syntax file for sections/dialect; semantic tokens cover Lua code. |
| Project naming | `pico-lsp` + `pico8` | Binary/repo `pico-lsp`; primary language ID `pico8`; config section `pico8`; accept `pico-8` aliases. |
| License | MIT | Permissive, ecosystem-compatible, attribution for reused fixtures/resources. |
| Type annotations | LuaCATS comments only | `---@class`, `---@param`, etc. No Luau-style inline syntax. |
| Type checking default | `basic` | Explicit annotation mismatches warn; strict nil/field checks are opt-in. |
| Includes | Faithful PICO-8 | Non-recursive includes. Nested includes produce a warning. |
| Include ownership | Nearest ancestor cart | A `.lua` file belongs to the nearest ancestor `.p8` that includes it. |
| Built-in API | PICO-8 0.2.7 only | Reserve `pico8.apiVersion` for future versions. |
| Install channels | GitHub release binaries for v1 | Checksummed archives and `go install`; Homebrew tap and Mason package are post-v1. |
| Distribution | Static Go binary | `go install` is documented for developers; release binaries are the primary user path. |
| LSP library | `go.lsp.dev/protocol` + `go.lsp.dev/jsonrpc2` | Typed LSP 3.18 types with `UnimplementedServer`; pinned to v1.0.1. |
| Position encoding | UTF-16 by default | Negotiate UTF-8 if the client supports it; UTF-16 must be correct first. |
| Diagnostics | Push-based | Pull diagnostics are post-v1. |
| Formatter | Post-v1 | PICO-8 shorthand is line-sensitive; this needs a dedicated AST-aware formatter. |
| `.p8.png` | Post-v1 | Text `.p8` and standalone `.lua` files are the v1 inputs. |

## Milestones

| ID | Milestone | Outcome | Depends on | Status |
|---|---|---|---|---|
| M0 | Foundation & Walking Skeleton | Server starts, speaks LSP, syncs documents, maps positions correctly | — | Done |
| M1 | Cart Model & Syntax Front-End | Understands `.p8`, includes, tabs, and the PICO-8 grammar; reports syntax errors | M0 | Done |
| M2 | Semantic Core & Navigation | Scopes, symbols, PICO-8 built-ins, definitions, references, rename | M1 | Done |
| M3 | Language Intelligence & Built-in API | Completion, hover, signature help, semantic tokens, API docs | M2 | Done |
| M4 | Type System & Annotations | LuaCATS, gradual inference, type-aware features, type diagnostics | M3 | Done |
| M5 | PICO-8-Specific Polish | Token/char budget, include diagnostics, lint rules, code actions | M4 | Done |
| M6 | Packaging, Editor Integration & v1 | Installable and tested v1 for Neovim and Helix | M5 | Done |
| M7 | Post-v1 | Formatter, `.p8.png`, VS Code, Zed, advanced types | M6 | Planned |

Each milestone has a detailed spec:

- [M0 — Foundation & Walking Skeleton](M0-foundation.md)
- [M1 — Cart Model & Syntax Front-End](M1-cart-and-syntax.md)
- [M2 — Semantic Core & Navigation](M2-semantic-core.md)
- [M3 — Language Intelligence & Built-in API](M3-language-intelligence.md)
- [M4 — Type System & Annotations](M4-type-system.md)
- [M5 — PICO-8-Specific Polish](M5-pico8-polish.md)
- [M6 — Packaging, Editor Integration & v1](M6-packaging-and-release.md)
- [M7 — Post-v1](M7-post-v1.md)

API and configuration references:

- [API](api.md)
- [Configuration](configuration.md)
- [Diagnostics](diagnostics.md)
- [Install](install.md)
- [Troubleshooting](troubleshooting.md)
- [Release checklist](release-checklist.md)

## v1 definition of done

The v1 release is complete when a user can:

1. Install `pico-lsp` from GitHub release binaries or `go install` (Homebrew and Mason are post-v1).
2. Open a `.p8` cart in Neovim or Helix and have the server attach automatically.
3. Edit carts and included `.lua` files with incremental document sync.
4. See accurate syntax diagnostics, document symbols, and folding ranges.
5. Navigate across tabs and includes with go-to-definition, references, and rename.
6. Get completion, hover, and signature help for user symbols and the PICO-8 0.2.7 API.
7. Add LuaCATS annotations and receive useful hover types, member completion, signature help, and basic type diagnostics.
8. See token and character budgets for the assembled cart, with warnings before limits and errors when exceeded.
9. Use a documented configuration and troubleshooting path.
10. Trust that the server never writes to stdout outside the LSP protocol, never hangs on malformed input, and never crashes the editor process.

## Non-goals for v1

- A formatter or formatter compatibility with `pico8-ls`.
- Reading or writing `.p8.png` carts.
- VS Code, Zed, or other editor-specific clients.
- A full Lua standard library or `require` implementation.
- Luau-style inline type syntax.
- Advanced type features: generic classes, overload resolution, operator metatypes, metatable-aware inference, exact class checking by default.
- Incremental parsing or a Salsa-style query system.
- Telemetry, cloud features, or network services.
- Cart execution, emulation, sprite/map previews, or audio tooling.

## Cross-cutting standards

### Diagnostics

All diagnostics use `source: "pico-lsp"` and a stable `code`.

| Code | Default severity | Milestone |
|---|---|---|
| `syntax` | Error | M1 |
| `include-missing` | Error | M1 |
| `include-nested` | Warning | M1 |
| `include-unreadable` | Error | M1/M5 |
| `include-outside-workspace` | Warning | M5 |
| `include-unusual-type` | Hint | M5 |
| `include-duplicate` | Off | M5 |
| `undefined-global` | Warning | M2 |
| `annotation-invalid` | Warning | M4 |
| `annotation-unknown-type` | Warning | M4 |
| `annotation-duplicate` | Warning | M4 |
| `annotation-unknown-param` | Warning | M4 |
| `annotation-missing-name` | Hint | M4 |
| `assign-type-mismatch` | Warning | M4 |
| `param-type-mismatch` | Warning | M4 |
| `return-type-mismatch` | Warning | M4 |
| `undefined-field` | Warning (exact classes only) | M4 |
| `need-check-nil` | Off in `basic`; warning in `strict` | M4 |
| `missing-fields` | Off in `basic`; warning in `strict` | M4 |
| `deprecated` | Hint | M4 |
| `duplicate-table-field` | Hint | M5 |
| `unreachable-code` | Hint | M5 |
| `shadowed-local` | Off | M5 |
| `empty-block` | Off | M5 |
| `global-in-function` | Off | M5 |
| `token-limit` | Error when exceeded; warning at 90% | M5 |
| `char-limit` | Error when exceeded; warning at 90% | M5 |
| `pico8-callback-precedence` | Hint | M5 |
| `pico8-callback-params` | Warning | M5 |
| `pico8-no-stdlib` | Hint | M5 |
| `unused-local` | Off | M5 |

Type-related diagnostics are warnings in `basic` mode and never block editing.

This table is a summary; each milestone spec is authoritative for its complete diagnostic list.

### Configuration contract

The server reads configuration from the `pico8` section via `workspace/configuration`, falling back to `initializationOptions.pico8`.

```json
{
  "pico8": {
    "apiVersion": "0.2.7",
    "typeChecking": "basic",
    "inlayHints": false,
    "maxProblems": 100,
    "limits": {
      "codeLens": true,
      "diagnostics": true
    },
    "diagnostics": {
      "undefinedGlobal": "warning",
      "unusedLocal": "off",
      "unreachableCode": "hint",
      "duplicateTableField": "hint",
      "callbackPrecedence": "hint",
      "callbackParams": "warning",
      "noStdlib": "hint"
    },
    "lint": {
      "apiRanges": false
    }
  }
}
```

`apiVersion` accepts only `"0.2.7"` in v1. Unsupported values produce a one-time warning and fall back to `0.2.7`.

### Testing policy

Every milestone must add tests appropriate to its surface:

- Golden/table tests for parser, analysis, and formatting of results.
- Fuzz/truncation tests for anything that parses untrusted text.
- In-memory LSP end-to-end tests for protocol behavior.
- A corpus of real, permissively licensed carts for parser and analysis regression tests.
- Token-count fixtures cross-checked against PICO-8 `info()` or `shrinko8`.
- `go test -race ./...` in CI.
- Benchmarks for full-cart parse/analyze and common editor requests.

### Performance budgets

These are targets for a 64 KB cart on a typical developer machine:

| Operation | Target |
|---|---|
| Full cart parse + analyze | < 10 ms |
| Hover / definition / completion after cache | < 5 ms |
| Server startup to `initialized` | < 100 ms |
| Resident memory with one cart open | < 150 MB |
| Workspace symbol scan | Cancellable; no startup scan required |

If a target is missed, profile before adding architectural complexity.

### Naming and compatibility

- Binary: `pico-lsp`.
- Module: `github.com/asimmons91/pico-lsp`.
- Language ID: `pico8`; accept `pico-8` and `pico8-lua` as aliases for compatibility with existing editor configs.
- Configuration section: `pico8`.
- File extensions: `.p8`, `.lua`.
- Custom request/command namespace: `pico8/...`.

### Security and privacy

- Never execute cart code.
- Read files only in service of workspace analysis; cap include file size and reject unreadable/binary inputs with diagnostics.
- No telemetry or network access.
- Logs may contain source text only when trace logging is explicitly enabled; document this.
- Never write to stdout except LSP messages.

## Release assumptions

These defaults were confirmed for the v1 release.

- **Repository/release names:** GitHub repo `asimmons91/pico-lsp`; release archives are named `pico-lsp_<version>_<os>_<arch>`.
- **Release versioning:** milestones produce `v0.x` prereleases; M6 cuts `v1.0.0`.
- **Install channels:** v1 publishes GitHub release archives and documents `go install`. Homebrew and Mason packaging are deferred to post-v1.
- **macOS/Windows signing:** not required for v1, but the unsigned-download warnings are documented in [install.md](install.md) and [troubleshooting.md](troubleshooting.md).

## How this roadmap is maintained

- Milestone specs are the source of truth for scope, interfaces, and acceptance criteria.
- Mark a milestone **In progress** when implementation starts and **Done** only when its exit criteria pass.
- Any change to a locked decision updates this roadmap and every affected spec in the same change.
- New ideas go to M7 or a new post-v1 spec; they do not expand v1 scope without an explicit decision.
