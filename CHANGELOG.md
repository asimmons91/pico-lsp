# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- LSP 3.17 pull diagnostics: `textDocument/diagnostic` full/unchanged reports,
  related includes and tabs, bounded `workspace/diagnostic` with partial
  results, deterministic result IDs, and `pico8.diagnostics.mode` for
  `auto`/`push`/`pull` transport selection.

### Changed

- Plan M11–M14 — pull diagnostics, the PICO-8 tree-sitter grammar, compressed
  size budgets, and conservative refactors — for a batched v1.2.0 release.

## [1.1.0] - 2026-11-17

### Added

- Type system II: generic classes and functions with defaults, overload
  selection in hover/signature help/call checking, explicit operator
  annotations for binary, unary, comparison, and call operators, tagged-union
  narrowing for string and number tags, and the `operator-type-mismatch` hint.
- Type system I: class inheritance with multi-level field and method lookup,
  inherited-member completion/hover/signature help, `(exact)` class field
  checks, `setmetatable` factory inference, and stronger flow narrowing for
  `assert`, `or`, early returns, `and`/`or` chains, and `type()` guards.
  Narrowed types drive hover, member completion, and inlay hints.
- AST-guided `textDocument/formatting` for `.p8` and `.lua` documents, with
  shorthand/comment preservation, `.p8` section and tab safety, configurable
  blank-line capping, and formatter fuzz/corpus tests.
- Read-only `.p8.png` support: uncompressed/new/old compression decode, P8SCII
  mapping, `pico8/decodeCart`, `pico8-cart:` virtual documents, Neovim
  `:Pico8OpenPng`/`:Pico8ReloadPng`, and `pico-lsp decode`.

### Changed

- `missing-fields` now warns in `basic` for explicitly `(exact)` classes; open
  records still only require fields in `strict`.
- String table keys are case-sensitive in type lookup, while dot fields keep
  PICO-8's identifier case folding.
- Document the project's AI-assisted, AI vibe-coded development process in the
  README.
- Plan the post-v1 roadmap: M7 formatter, M8 read-only `.p8.png` carts, M9 type
  system I, and M10 type system II, batched for v1.1.0.

## [1.0.0] - 2026-11-17

### Added

- PICO-8 cart model for text `.p8` carts and standalone `.lua` files, including
  tabs, source maps, non-recursive `#include` flattening, token counting, and
  character counting.
- Hand-written PICO-8 lexer and parser with dialect shorthand, comments,
  line-bounded constructs, and syntax diagnostics.
- Semantic model with scopes, symbols, includes, workspace symbols, and
  PICO-8 0.2.7 built-in API documentation.
- Navigation and editing features: document symbols, folding ranges,
  definitions, references, document highlights, rename, completion (including
  member, annotation, and include-path completion), hover, signature help, full
  semantic tokens, and inlay hints.
- LuaCATS annotations, aliases, enums, classes, gradual type inference, flow
  narrowing, and `off`/`basic`/`strict` type checking.
- PICO-8-specific diagnostics and fixes: lifecycle callback checks, no-standard-
  library hints, duplicate table fields, unreachable code, unused/shadowed
  locals, empty blocks, global assignment hints, include diagnostics, and
  configurable severities.
- Cart budget code lens, token and character limit diagnostics, per-file
  contributions, the `pico8/cartInfo` request, and the `pico8.showCartInfo`
  command.
- Code actions for generating type annotations, suppressing diagnostics, and
  creating missing `.lua` includes.
- Dynamic watched-file registration and reanalysis for cart and include
  create/change/delete events.
- Neovim 0.11+ and Helix configuration, an optional Neovim helper for
  cart-owned `.lua` buffers, and a standalone Vim syntax file for `.p8` carts.
- Reproducible static release builds for Linux, macOS, and Windows on amd64 and
  arm64, with SHA-256 checksums and conventional-commit release notes.
- CI coverage for Linux, macOS, and Windows tests, race detection, linting,
  vulnerability scanning, build matrices, fuzz smoke tests, and goreleaser
  snapshot validation.
- README, install, configuration, diagnostics, API, troubleshooting, and
  release documentation.

### Notes

- v1 is distributed through GitHub release archives and `go install`.
  Homebrew and Mason packaging are deferred to post-v1.
- macOS and Windows release binaries are not signed.
- Formatter, `.p8.png`, VS Code, and Zed support are post-v1 work.

[Unreleased]: https://github.com/asimmons91/pico-lsp/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/asimmons91/pico-lsp/releases/tag/v1.1.0
[1.0.0]: https://github.com/asimmons91/pico-lsp/releases/tag/v1.0.0
