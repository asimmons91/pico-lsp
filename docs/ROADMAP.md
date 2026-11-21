# PICO-8 Language Server — Roadmap

**Status:** v1.8.0 released with the VS Code settings schema and server health
reporting, annotation-aware navigation, and token-saving code actions.

**Last updated:** 2026-11-21
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
| Editors | Neovim + Helix | VS Code client is M15; M21 adds the full VS Code settings schema; M22 adds server health; remote and web extension hosts remain out of scope; Zed remains post-v1; the server remains editor-agnostic. |
| Neovim helper | Helper plugin, optional at runtime | Auto-attaches to `.lua` files owned by a cart; avoids attaching to unrelated Lua buffers. |
| Syntax highlighting | Vim syntax file + semantic tokens | Ship a small `.p8` syntax file for sections/dialect; semantic tokens cover Lua code. |
| Project naming | `pico-lsp` + `pico8` | Binary/repo `pico-lsp`; primary language ID `pico8`; config section `pico8`; accept `pico-8` aliases. |
| License | MIT | Permissive, ecosystem-compatible, attribution for reused fixtures/resources. |
| Type annotations | LuaCATS comments only | `---@class`, `---@param`, etc. No Luau-style inline syntax. |
| Type checking default | `basic` | Explicit annotation mismatches warn; strict nil/field checks are opt-in. |
| Includes | Faithful PICO-8 | Non-recursive includes. Nested includes produce a warning. |
| Include ownership | Nearest ancestor cart | A `.lua` file belongs to the nearest ancestor `.p8` that includes it. |
| Built-in API | PICO-8 0.2.7 only | Reserve `pico8.apiVersion` for future versions. |
| Install channels | GitHub release binaries for v1 | Checksummed archives and `go install`; M15 adds a VS Code VSIX asset. Homebrew tap, Mason package, and Marketplace/Open VSX publishing are post-v1. |
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
| M7 | AST-Aware Formatter | Shorthand-safe formatting with comments and annotations preserved | M6 | Done |
| M8 | Read-Only `.p8.png` Carts | Decode PNG carts into read-only virtual documents with full language features | M7 | Done |
| M9 | Type System I | Inheritance, exact classes, metatable inference, stronger narrowing | M8 | Done |
| M10 | Type System II | Generics, overloads, operators, and discriminated unions | M9 | Done |
| M11 | Pull Diagnostics | LSP 3.17 pull diagnostics with related files and push fallback | M10 | Done |
| M12 | PICO-8 Tree-sitter Grammar | Editor highlighting, indents, and text objects for `.p8` carts | M11 | Done |
| M13 | Compressed-size Budget | PICO-8-compatible compressed code-size reporting and limits | M12 | Done |
| M14 | Refactors and Code Actions | Conservative, previewable refactors for everyday PICO-8 edits | M13 | Done |
| M15 | VS Code Client | Installable VS Code extension with managed server downloads and cart budgets | M14 | Done |
| M16 | P8SCII Glyph Snippets and Input | ASCII-triggered glyph completion, custom font slot entries, and a searchable picker | M15 | Done |
| M17 | P8SCII Public Package | Stable `pkg/p8scii` with the full glyph table, widths, and encode/decode | M16 | Done (v1.6.0) |
| M18 | Cart Public Package | Public cart document, byte-preserving `ReplaceCode`, and include ownership | M17 | Done (v1.6.0) |
| M19 | LSP and Glyph Public Packages | In-process `pkg/lsp` server and public glyph catalog | M18 | Done (v1.6.0) |
| M21 | VS Code Settings Schema | Every server setting is discoverable and validated in VS Code Settings | M15 | Done (v1.8.0) |
| M22 | VS Code Server Health | Version mismatch warnings and copyable server diagnostics | M15 | Done (v1.8.0) |
| M23 | Token-Saving Quick Fixes | Conservative code actions for `?`, compound assignments, peek aliases, and shorthand | M14 | Done (v1.8.0) |

M7–M10 shipped together as **v1.1.0**. M11–M14 shipped together as **v1.2.0**.
M15 shipped as **v1.3.0**. M16 shipped as **v1.4.0**. Post-v1.4 additions
shipped as **v1.5.0**: LuaCATS annotation semantic highlighting, VS Code
PICO-8 file icons, a Neovim comment ftplugin, and incomplete-statement
completion hardening. **v1.5.1** followed with formatter EOF normalization and
workspace-relative cart paths in the VS Code status tooltip.

M17–M19 are the public-package releases for pico-editor. They ship together
in **v1.6.0**: p8scii, cart, and the in-process LSP and glyph catalog. The
originally planned separate v1.7.0 and v1.8.0 milestones were folded into
v1.6.0 so all three packages share one supported snapshot. The v1.7.0 tag is
now the later, additive catalog-coverage release.

M21, M22, and M23 shipped together in **v1.8.0**; separate milestone specs do
not require separate tags.

M23 adds cursor-local token-saving code actions on top of the M14 shorthand
conversion; the pattern catalog, safety rules, deferred patterns, and resolved
decisions are in [M23](M23-token-saving-quick-fixes.md). Zero-argument
`print()`, `flr` division, redundant parentheses, and cart-wide fix-all remain
deferred.

Each milestone has a detailed spec:

- [M0 — Foundation & Walking Skeleton](M0-foundation.md)
- [M1 — Cart Model & Syntax Front-End](M1-cart-and-syntax.md)
- [M2 — Semantic Core & Navigation](M2-semantic-core.md)
- [M3 — Language Intelligence & Built-in API](M3-language-intelligence.md)
- [M4 — Type System & Annotations](M4-type-system.md)
- [M5 — PICO-8-Specific Polish](M5-pico8-polish.md)
- [M6 — Packaging, Editor Integration & v1](M6-packaging-and-release.md)
- [M7 — AST-Aware Formatter](M7-formatter.md)
- [M8 — Read-Only `.p8.png` Carts](M8-p8png.md)
- [M9 — Type System I](M9-type-system-1.md)
- [M10 — Type System II](M10-type-system-2.md)
- [M11 — Pull Diagnostics](M11-pull-diagnostics.md)
- [M12 — PICO-8 Tree-sitter Grammar](M12-tree-sitter-grammar.md)
- [M13 — Compressed-size Budget](M13-compressed-size-budget.md)
- [M14 — Refactors and Code Actions](M14-refactors-and-code-actions.md)
- [M15 — VS Code Client](M15-vscode-client.md)
- [M16 — P8SCII Glyph Snippets and Input](M16-p8scii-glyphs.md)
- [M17 — P8SCII Public Package](M17-p8scii.md)
- [M18 — Cart Public Package](M18-cart.md)
- [M19 — LSP and Glyph Public Packages](M19-lsp-and-glyphs.md)
- [M21 — Full Server Settings Schema in the VS Code Client](M21-vscode-settings.md)
- [M22 — VS Code Server Health and Version Reporting](M22-vscode-server-health.md)
- [M23 — Token-Saving Quick Fixes](M23-token-saving-quick-fixes.md) — shipped in v1.8.0

Uncommitted post-v1 ideas live in the [post-v1 backlog](post-v1-backlog.md).

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
- Zed or other editor-specific clients. The VS Code client is M15.
- A full Lua standard library or `require` implementation.
- Luau-style inline type syntax.
- Advanced type features in v1.0: generic classes, overload resolution,
  operator metatypes, metatable-aware inference, exact class checking by
  default. These were delivered by the post-v1 M9/M10 milestones.
- Incremental parsing or a Salsa-style query system.
- Telemetry, cloud features, or network services.
- Cart execution, emulation, sprite/map previews, or audio tooling.

## Editor integration packages (post-v1)

pico-editor (`github.com/asimmons91/pico-editor`) consumes pico-lsp in-process.
That integration requires stable public facades over the existing analysis
code. These are additive post-v1 releases; they do not change the LSP server's
wire contract or defaults. A marker-recognition fix that aligns the internal
tab scanner with PICO-8's column-0 rule, and a lone-CR line-scanning correction
that matches P8SCII, are allowed and must be recorded in the changelog.

| Release | Milestone | Public package | Purpose | Blocks |
| --- | --- | --- | --- | --- |
| `v1.6.0` | M17–M19 | `pkg/p8scii`, `pkg/cart`, `pkg/lsp`, `pkg/glyphs` | Canonical P8SCII data, cart parsing and ownership, in-process server, and glyph catalog | pico-editor M1, M4, M5 |
| `v1.7.0` | post-M19 | `pkg/glyphs` | Catalog every printable P8SCII byte, including ASCII, puny letters, and kana, for the glyph picker | pico-editor M6 |

Rules:

- Each package is a thin public facade, or an editor-shaped public model that
  wraps existing internal behavior. The cart `Document` wraps the internal
  parser; it does not duplicate it. Do not fork behavior or duplicate the
  parser, P8SCII mapping, or LSP server.
- Facades expose only what the editor needs. Keep the public API small, and
  cover it with tests and examples.
- Tag `v1.6.0` only after the p8scii, cart, LSP, and glyph package tests pass.
  A milestone pin must reference a tag, never a branch or pseudo-version. The
  originally planned separate v1.7.0 and v1.8.0 milestones were folded into
  v1.6.0; the v1.7.0 tag is the later additive catalog-coverage release
  required by pico-editor M6.
- pico-editor must not import `internal/` packages; Go enforces this.
- The editor pins each release before the corresponding milestone starts. Its
  schedule is recorded in
  [pico-editor's architecture document](https://github.com/asimmons91/pico-editor/blob/main/docs/ARCHITECTURE.md).

## Post-v1 editor additions

- **Neovim Shift-glyph input:** the optional `editors/nvim` ftplugin can
  install PICO-8's `Shift-A` through `Shift-Z` insert-mode mappings for the 26
  typeable P8SCII symbols at `0x80`-`0x99` when `g:pico8_glyph_maps` is
  enabled. The mappings fetch each glyph by byte from the server's
  `pico8/glyphs` catalog, so canonical Unicode spellings stay server-owned;
  outside editable Lua code, in include paths, or without a client, they fall
  back to the literal uppercase letter. They are opt-in because they replace
  uppercase letters in code. PICO-8's `CTRL-J`/`CTRL-K`/`CTRL-P` input modes
  remain out of scope.

## VS Code client follow-ups

M21 and M22 are separate, additive client milestones. They do not change the
LSP wire contract, server defaults, or the editor-agnostic server contract.

- **M21 — Settings schema:** the server's existing `pico8.*` settings are
  discoverable and validated in the VS Code Settings UI, with drift tests
  against `internal/config`.
- **M22 — Server health:** the extension reports the running server version
  and source, warns once on a release-version mismatch, and copies diagnostics
  for issue reports.

Each milestone has its own spec. M21 and M22 shipped in v1.8.0.

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
| `annotation-invalid` | Warning | M4/M9 |
| `annotation-unknown-type` | Warning | M4 |
| `annotation-duplicate` | Warning | M4 |
| `annotation-unknown-param` | Warning | M4 |
| `annotation-missing-name` | Hint | M4 |
| `assign-type-mismatch` | Warning | M4/M9 |
| `param-type-mismatch` | Warning | M4/M9 |
| `return-type-mismatch` | Warning | M4/M9 |
| `undefined-field` | Warning (exact classes only) | M4/M9 |
| `need-check-nil` | Off in `basic`; warning in `strict` | M4 |
| `missing-fields` | Warning for explicit `(exact)` classes in `basic`; otherwise off in `basic`, warning in `strict` | M4/M9 |
| `operator-type-mismatch` | Hint | M10 |
| `deprecated` | Hint | M4 |
| `duplicate-table-field` | Hint | M5 |
| `unreachable-code` | Hint | M5 |
| `shadowed-local` | Off | M5 |
| `empty-block` | Off | M5 |
| `global-in-function` | Off | M5 |
| `token-limit` | Error when exceeded; warning at 90% | M5 |
| `char-limit` | Error when exceeded; warning at 90% | M5 |
| `compressed-size-limit` | Warning at 90%; error when exceeded | M13 |
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
      "mode": "auto",
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
    },
    "refactors": {
      "enabled": true
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

Wall-clock performance gates are opt-in with `PICO_LSP_PERF=1` and run through
the manual **Performance** workflow or locally. Default CI skips hard timing
assertions because shared runners are too noisy for stable absolute thresholds.

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
- **Release versioning:** milestones produced `v0.x` prereleases through M6 (`v1.0.0`); post-v1 releases use minor versions (`v1.1.0`, `v1.2.0`, `v1.3.0`, `v1.4.0`, `v1.5.0`, `v1.5.1`, `v1.6.0`, `v1.7.0`, `v1.8.0`).
- **Install channels:** v1 publishes GitHub release archives and documents `go install`. Homebrew and Mason packaging are deferred to post-v1.
- **macOS/Windows signing:** not required for v1, but the unsigned-download warnings are documented in [install.md](install.md) and [troubleshooting.md](troubleshooting.md).
- **Post-v1 batching:** M7–M10 shipped as v1.1.0; M11–M14 shipped as v1.2.0; M15 shipped as v1.3.0; M16 shipped as v1.4.0; the post-v1.4 editor and annotation additions shipped as v1.5.0; v1.5.1 shipped formatter and VS Code fixes. M17–M19 shipped together as v1.6.0; v1.7.0 expands the glyph catalog to every printable byte. M21, M22, and M23 shipped together as v1.8.0. Later promoted features use minor releases unless they introduce a breaking change.

## How this roadmap is maintained

- Milestone specs are the source of truth for scope, interfaces, and acceptance criteria.
- Mark a milestone **In progress** when implementation starts and **Done** when its implementation exit criteria pass. A deferred release tag remains an open checklist item in the milestone spec.
- Any change to a locked decision updates this roadmap and every affected spec in the same change.
- New ideas go to the [post-v1 backlog](post-v1-backlog.md) or a promoted milestone spec; they do not expand v1 scope without an explicit decision.
