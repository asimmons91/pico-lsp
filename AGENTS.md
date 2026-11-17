# AGENTS.md

## Current state

- M0 (Foundation & Walking Skeleton), M1 (Cart Model & Syntax Front-End), and M2 (Semantic Core & Navigation) are implemented: `cmd/pico-lsp` plus `internal/lsp`, `internal/cart`, `internal/syntax`, `internal/source`, `internal/analysis`, `internal/builtin`, `internal/config`, and `internal/version`.
- The server speaks LSP over stdio, syncs documents, analyzes `.p8`/`.lua` carts and includes, publishes syntax/include/undefined-global diagnostics, and serves document symbols, folding ranges, definitions, references, document highlights, rename, and workspace symbols.
- M3 and later are not implemented. There is no completion, hover, signature help, semantic tokens, type checking, token-budget UI, or formatter yet.
- CI: `.github/workflows/ci.yml` runs gofmt, vet, unit/race tests, a short fuzz pass, and a static build on pull requests. `.github/workflows/fuzz-nightly.yml` runs longer fuzz passes. M6 expands this with lint, build matrices, and release workflows.
- Module: `github.com/asimmons91/pico-lsp`. Go 1.27.4 is pinned in `mise.toml`.
- No README, lint config, or release tooling exists yet. The 51-cart corpus and token fixtures live under `testdata/`.
- Do not invent or claim build/test results. Run the commands below and report the actual output.

## Source of truth

- `docs/ROADMAP.md` is the index and records locked product decisions.
- `docs/M0-*.md` through `docs/M7-*.md` are authoritative for each milestone's scope, interfaces, and exit criteria.
- Read the relevant milestone spec before implementing anything. Do not expand v1 scope without updating the roadmap.
- If a locked decision changes, update `docs/ROADMAP.md` and every affected spec in the same change.

## Planning and open decisions

- While planning or thinking through an implementation, surface any open decision that affects architecture, scope, behavior, or technical tradeoffs. Prompt the user instead of silently choosing.
- Present the options with a recommendation and keep the user informed as the plan or tradeoffs change.
- Do not re-ask decisions already locked in `docs/ROADMAP.md` or a milestone spec.
- Record newly resolved decisions in `docs/ROADMAP.md` or the affected milestone spec.

## Locked v1 scope

- v1 = core LSP + type features. Formatter and `.p8.png` support are post-v1 (M7).
- Editors: Neovim + Helix. VS Code and Zed clients are post-v1.
- Types: LuaCATS comments only; default mode `basic`; no Luau-style inline syntax.
- Includes: faithful PICO-8 (non-recursive). A `.lua` file is owned by the nearest ancestor `.p8` cart that includes it.
- Built-ins: PICO-8 0.2.7 only. `pico8.apiVersion` is reserved for future versions.
- Distribution: static Go binary; release binaries + Homebrew + Mason.
- Naming: binary/repo `pico-lsp`; language ID and config section `pico8`; accept `pico-8` aliases.
- License: MIT.
- Release names, versioning, and signing are documented assumptions in `docs/ROADMAP.md`; confirm before M6.

## Implementation order

- Follow M0 → M1 → M2 → M3 → M4 → M5 → M6. M7 is a post-v1 backlog, not v1 work.
- Follow the file paths in each spec (`cmd/pico-lsp`, `internal/...`). Do not start a milestone before its dependency exits.
- Use the hand-written PICO-8 lexer/parser (M1); do not substitute tree-sitter, `gopher-lua`, or `golua`.
- Carts are capped at roughly 64 KB and 8192 tokens. Full reparse and full analysis are intentional; do not build incremental parsing or a query system.
- Never write to stdout except LSP protocol messages. Use stderr or `--log-file`.
- LSP positions default to UTF-16. Convert through `source.LineIndex`; never use `len(string)` as a column count.

## PICO-8 gotchas

- `//` is a comment, not floor division. `\` is integer division.
- `@`, `%`, `$` are read-only peek/peek2/peek4 operators; they are not assignment targets.
- `?` and single-line `if`/`while` are line-bounded. A next-line `else` must not attach to a shorthand `if`.
- `#include` is non-recursive. Flatten includes with a source map back to original files.
- Identifiers case-fold (PICO-8 has no uppercase). Preserve original text, but normalize for lookup.
- Limits are 8192 tokens and 65535 characters; comments and `---@` annotations cost no tokens.
- There is no Lua standard library. Use only the PICO-8 API.
- `.p8.png` is unsupported in v1; only text `.p8` and standalone `.lua` files.

## Git workflow

- Before starting any task, create a branch from an up-to-date `main`:
  `feat/<short-slug>` for features, `fix/<short-slug>` for bug fixes, or
  `chore/<short-slug>` for docs, tooling, and maintenance.
- Never work directly on `main`; it is protected and direct pushes are rejected.
- Use Conventional Commits for every commit: `<type>(<optional scope>): <description>`.
  Allowed types: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `build`, `ci`, `perf`.
- Use imperative, lowercase descriptions with no trailing period. Mark breaking changes
  with `!` and/or a `BREAKING CHANGE:` footer.
- Push the branch and open a PR; do not bypass the `Protect main` ruleset.
- After opening the PR, run the `/review` OpenCode command against the PR
  (pass its URL or number). Review locally unless the user asks for posted
  comments. Address the findings, commit the fixes, and push them to the same
  branch/PR; do not open a separate PR for review fixes.
- If `/review` is incomplete or fails, re-run it or report the gap; never treat
  missing coverage as a clean review.

## Commands

- `mise install` installs the pinned Go toolchain; `go version` should report 1.27.4.
- `go build ./...` builds the server and library packages.
- `CGO_ENABLED=0 go build -o pico-lsp ./cmd/pico-lsp` verifies the static release build path.
- `go test ./...` runs unit and in-memory LSP integration tests.
- `go test -race ./...` is the required race-free gate.
- `go vet ./...` and `gofmt -l cmd internal` catch correctness and formatting issues.
- No lint configuration or release tooling exists yet; add and document them with the milestone that introduces them.

## Documentation rules

- Keep `docs/ROADMAP.md` links valid when adding or renaming milestone files.
- Milestone specs own the details; the roadmap owns scope and cross-milestone decisions.
