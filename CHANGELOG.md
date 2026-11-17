# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Document the project's AI-assisted, AI vibe-coded development process in the
  README.

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

[Unreleased]: https://github.com/asimmons91/pico-lsp/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/asimmons91/pico-lsp/releases/tag/v1.0.0
