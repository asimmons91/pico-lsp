# M6 — Packaging, Editor Integration & v1 Release

**Status:** Done — v1.0.0 published as a GitHub release; Homebrew and Mason are post-v1.  
**Depends on:** M5  
**Blocks:** M7  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

A user can install `pico-lsp` without a Go toolchain, attach it in Neovim or Helix with a small documented config, and get the full v1 feature set. The release is reproducible, checksummed, documented, and smoke-tested.

**Demo:** On a fresh machine, download the v1.0.0 GitHub release archive for
the current OS/architecture, verify `checksums.txt`, and put `pico-lsp` on
`PATH`. Developers can equivalently run:

```sh
go install github.com/asimmons91/pico-lsp/cmd/pico-lsp@v1.0.0
```

Open a `.p8` cart. The server attaches, shows the token budget, navigates
includes, completes the PICO-8 API, and reports type diagnostics.
`pico-lsp --version` prints `v1.0.0`.

## Scope

### In scope

- Version injection and reproducible release builds.
- goreleaser configuration, GitHub release archives, and checksums.
- Neovim and Helix setup documentation and configs.
- A small Neovim helper plugin that auto-attaches to included `.lua` buffers.
- A small Vim syntax file for `.p8` sections and dialect keywords.
- README, install, configuration, diagnostics, and troubleshooting docs.
- CI: tests, race, lint, fuzz smoke, build matrix, release snapshot.
- Release checklist and smoke tests.

### Out of scope

- VS Code and Zed clients (M7).
- Auto-update mechanisms.
- Telemetry, analytics, or crash reporting.
- Code signing/notarization unless chosen later.
- Formatter and `.p8.png` support (M7).
- Homebrew tap and Mason registry packaging (deferred to post-v1).
- A hosted package registry or Docker image.

> **Superseded by later milestones:** the VS Code client shipped in M15, and M24
> prepares VS Code Marketplace and Open VSX publishing for that extension.
> v1.8.1 shipped as a GitHub-only release; the registry launch follows in a later
> patch. Homebrew and Mason remain deferred.

## Release versioning

- Milestones M0–M5 produce `v0.x.y` prereleases as needed.
- M6 cuts `v1.0.0`.
- Follow SemVer:
  - breaking config/LSP behavior → major;
  - new features → minor;
  - fixes → patch.
- Tag format: `v1.0.0`.
- Changelog: Keep a Changelog format with an `Unreleased` section.
- Version is injected at build time and exposed via `--version` and `ServerInfo.Version`.

## Build and release artifacts

### Build settings

- `CGO_ENABLED=0`.
- `-trimpath`.
- `-ldflags "-s -w -X github.com/asimmons91/pico-lsp/internal/version.Version={{.Version}}"`.
- Go version pinned by `go.mod` and `mise.toml` (currently 1.27.4).
- Reproducible builds where practical; record the Go version and commit in release notes.

### Targets

At minimum:

| OS | Architectures | Notes |
|---|---|---|
| Linux | amd64, arm64 | Static; consider musl builds for Alpine. |
| macOS | amd64, arm64 | Unsigned by default; document Gatekeeper warning. |
| Windows | amd64, arm64 | `.exe`; PowerShell install docs. |

Artifacts:

- `pico-lsp_<version>_<os>_<arch>.tar.gz` for Unix.
- `pico-lsp_<version>_windows_<arch>.zip`.
- `checksums.txt` (SHA-256).
- Optional: SBOM/provenance if goreleaser supports it cleanly.
- Release notes with install commands and notable changes.

### goreleaser

Create `.goreleaser.yaml`:

- `builds` for the matrix above.
- `archives` with README/LICENSE included.
- `checksum`.
- `changelog` grouped by conventional commits if the repo uses them.
- `release` publishes the GitHub release and attachments.
- `snapshot` mode used in CI to validate the config on every PR.

## Homebrew and Mason

Homebrew tap publishing and Mason registry packaging are deferred to post-v1
by the v1 release decision. v1 installs use GitHub release archives and
`go install`; see [install.md](install.md). The goreleaser configuration and
release workflow deliberately do not publish formulae or registry packages.

## Editor integration

### Neovim 0.11+

Provide `editors/nvim/README.md` and a copy-paste config:

```lua
vim.filetype.add({ extension = { p8 = "pico8" } })

vim.lsp.config("pico_lsp", {
  cmd = { "pico-lsp" },
  filetypes = { "pico8" },
  root_markers = { ".git" },
  settings = {
    pico8 = {
      apiVersion = "0.2.7",
      typeChecking = "basic",
      inlayHints = false,
    },
  },
})
vim.lsp.enable("pico_lsp")
```

Additional guidance:

- `.p8` is the primary filetype. Included `.lua` files should also attach when they belong to a cart.
- Ship a small optional helper plugin under `editors/nvim/` that:
  - detects whether the current `.lua` buffer is included by a nearby `.p8` cart;
  - starts/attaches `pico_lsp` only for those buffers;
  - avoids duplicate clients and does not attach to unrelated Lua files;
  - can be disabled by users who prefer manual configuration or another client.
- Document the manual fallback: attach `pico_lsp` to `lua` in known PICO-8 projects and avoid running `lua_ls` on those buffers.
- Recommended keymaps:

  ```lua
  vim.keymap.set("n", "gd", vim.lsp.buf.definition)
  vim.keymap.set("n", "K", vim.lsp.buf.hover)
  vim.keymap.set("n", "<leader>rn", vim.lsp.buf.rename)
  vim.keymap.set("n", "<leader>ca", vim.lsp.buf.code_action)
  vim.keymap.set("n", "<leader>ci", function()
    vim.lsp.buf_request(0, "pico8/cartInfo", {
      textDocument = { uri = vim.uri_from_bufnr(0) },
    }, function(err, result)
      if not err and result then
        print(("%d / %d tokens · %d / %d chars"):format(
          result.tokens, result.tokenLimit, result.chars, result.charLimit))
      end
    end)
  end)
  ```

- Document `vim.lsp.log.get_filename()` and `--log-file` for troubleshooting.
- Document how to disable formatting providers if another formatter is configured (formatter is post-v1, so this is future-proofing).

### Helix

Provide `editors/helix/README.md` and a `languages.toml` snippet:

```toml
[[language]]
name = "pico8"
scope = "source.pico8"
file-types = ["p8"]
comment-token = "--"
language-servers = ["pico-lsp"]

[language-server.pico-lsp]
command = "pico-lsp"
```

Notes:

- Helix uses GitHub release binaries or `go install`; Homebrew and Mason are
  deferred to post-v1.
- Verify `.p8` syntax highlighting. If no grammar exists, document that semantic tokens cover Lua code but not gfx/audio sections; a tree-sitter grammar is post-v1.
- Verify inlay hints, completion, hover, signature help, and code actions in the current stable Helix.

### Vim syntax file and Neovim helper

Ship a small syntax file for `.p8` sections and dialect keywords:

- `editors/nvim/syntax/pico8.vim`
- `editors/nvim/ftdetect/pico8.vim`
- Highlight `__lua__`, `__gfx__`, `__gff__`, `__label__`, `__map__`, `__sfx__`, `__music__`, `-->8`, comments, strings, numbers, and dialect keywords/operators.
- Keep it independent of the LSP so users who only want syntax highlighting can use it.
- LSP semantic tokens are not a substitute for whole-file highlighting in the non-code sections.
- The helper plugin lives under `editors/nvim/lua/pico_lsp/` and is optional at runtime; it must never be required for the server to function.

## Documentation

Create/maintain:

- `README.md`:
  - What it is, feature list, screenshots/GIFs if available.
  - Install from GitHub release binaries and `go install`; document that
    Homebrew and Mason are deferred to post-v1.
  - Neovim and Helix quick starts.
  - Configuration example.
  - Link to roadmap and milestone specs.
  - “PICO-8 is a trademark of Lexaloffle Games” disclaimer.
- `docs/install.md`:
  - Per-OS instructions, checksums, upgrade/uninstall.
- `docs/configuration.md`:
  - Every setting, default, enum values, per-workspace behavior.
  - `initializationOptions` fallback.
  - `pico8/cartInfo` request and `pico8.showCartInfo` command.
- `docs/diagnostics.md`:
  - Every diagnostic code, default severity, how to configure/suppress.
- `docs/troubleshooting.md`:
  - Server not attaching, logs, stdout corruption symptoms, included `.lua` attachment, UTF-16/Unicode issues, multiple carts, `.p8.png` unsupported.
- `docs/architecture.md` (optional):
  - Link to the roadmap and summarize the cart/flatten/source-map design.
- `LICENSE`:
  - MIT.
- `CHANGELOG.md`.

## CI

A minimal test workflow (format, vet, unit/race tests, static build) may land before M6 to gate pull requests. The workflows below are the complete CI surface M6 owns.

GitHub Actions workflows:

### Test workflow

- Matrix: Linux, macOS, Windows.
- `go test ./...`
- `go test -race ./...` on Linux (and macOS if time allows).
- `go vet ./...`
- `golangci-lint`.
- `govulncheck ./...`.
- Build `CGO_ENABLED=0` on all target OS/arch (at least compile).
- Short fuzz pass (`go test -fuzz=FuzzParser -fuzztime=30s`) on Linux.
- Upload coverage.

### Release workflow

- Trigger on `v*` tags.
- Run tests first.
- `goreleaser release --clean`.
- Publish the GitHub release with archives and checksums.
- Announce in README/changelog.

### PR workflow

- `goreleaser check` and `goreleaser release --snapshot --clean` to validate
  packaging.
- Lint docs links? Optional.
- Ensure generated files (if any) are up to date.

## Release checklist

Before tagging `v1.0.0`:

- [x] All M0–M5 exit criteria met.
- [x] Full test suite, race, lint, govulncheck pass on all platforms.
- [x] Corpus tests pass with no panics.
- [x] Performance budgets met.
- [x] Token counts validated against PICO-8/shrinko8.
- [x] Neovim 0.11+ manual smoke test.
- [x] Helix stable manual smoke test.
- [x] Fresh-machine install via release binary.
- [ ] Optional: verify Homebrew or Mason once those post-v1 channels exist.
- [x] `pico-lsp --version` matches tag.
- [x] README and docs accurate; links valid.
- [x] Configuration defaults documented.
- [x] Diagnostic codes documented.
- [x] License and third-party attribution complete.
- [x] Changelog updated.
- [x] Checksums published.
- [x] No stdout writes outside LSP framing.
- [x] No telemetry/network calls.
- [x] Known limitations documented (formatter, `.p8.png`, VS Code/Zed, advanced types).

After tagging:

- [x] Verify release assets download and run.
- [x] Verify `go install ...@v1.0.0` builds for a developer.
- [x] Update docs with the released version.
- [ ] Open M7 tracking issues for post-v1 work.

## Architecture and files

```
.goreleaser.yaml
.github/workflows/ci.yml
.github/workflows/release.yml
.github/workflows/fuzz-nightly.yml
README.md
CHANGELOG.md
LICENSE
.golangci.yml
docs/install.md
docs/configuration.md
docs/diagnostics.md
docs/troubleshooting.md
docs/release-checklist.md
editors/nvim/README.md
editors/nvim/lua/pico_lsp/init.lua (auto-attach helper)
editors/nvim/ftdetect/pico8.vim
editors/nvim/syntax/pico8.vim
editors/helix/README.md
editors/helix/languages.toml
internal/version/version.go
```

## Error handling and support

- `pico-lsp --version` works without a config or workspace.
- If the server cannot start (bad flags, log path), print a clear error to stderr and exit non-zero.
- If the client disconnects, exit cleanly.
- Provide a `--log-file` path that works when the server is launched by an editor with an unknown working directory.
- Document how to collect logs for bug reports.
- Never require environment variables for normal operation.

## Testing

### Packaging tests

- `goreleaser release --snapshot --clean` succeeds on CI.
- Archives contain the binary, README, LICENSE, and CHANGELOG.
- Checksums match.
- `--version` output matches the tag.
- Binary runs on a clean container/VM without Go installed.

### Editor smoke tests

Automated where possible:

- Headless Neovim with the documented config, open `testdata/smoke.p8`, request hover/definition, assert response.
- Headless Neovim test for the helper plugin: opening an included `.lua` attaches `pico_lsp`; opening an unrelated `.lua` does not.
- Helix can be smoke-tested manually; optional scripted test if practical.

Manual matrix:

| Editor | OS | Checks |
|---|---|---|
| Neovim 0.11+ | Linux | attach, diagnostics, completion, hover, definition, rename, code lens, cartInfo |
| Neovim 0.11+ | macOS | same |
| Helix stable | Linux | attach, diagnostics, completion, hover, signature, inlay hints |
| Helix stable | macOS | same |

### Documentation tests

- Link checker in CI (optional).
- Validate JSON/TOML/Lua snippets in docs where feasible.
- Ensure the configuration reference matches `internal/config` defaults.

## Implementation decisions

- v1 publishes GitHub release archives only. Homebrew and Mason packaging are
  deferred to post-v1, so goreleaser does not configure a tap or registry.
- goreleaser v2.20.2 builds the six target OS/arch combinations with
  `CGO_ENABLED=0`, `-trimpath`, stripped symbols, and the version injected from
  the tag.
- The version package falls back to Go build information for
  `go install ...@v1.0.0` and normalizes a missing `v` prefix.
- `golangci-lint` v2.15.0 is the lint gate; `govulncheck` v1.9.0 is the
  vulnerability gate.
- macOS and Windows binaries remain unsigned, with Gatekeeper and SmartScreen
  warnings documented.
- CI covers Linux, macOS, and Windows tests; Linux race/lint/vulnerability
  checks; a six-target build matrix; short fuzzing; coverage artifacts; and a
  goreleaser snapshot.
- The optional Neovim helper is tested headlessly for cart-owned `.lua`
  attachment and unrelated-Lua refusal.

## Exit criteria

- [x] Release artifacts build on all target platforms.
- [x] GitHub release archives, checksums, and release workflow are in place.
- [x] Neovim and Helix configs are documented and smoke-tested.
- [x] The Neovim helper auto-attaches included `.lua` files and ignores unrelated Lua files.
- [x] The Vim syntax file ships and highlights `.p8` sections and dialect constructs.
- [x] README, install, configuration, diagnostics, and troubleshooting docs are complete.
- [x] CI covers tests, race, lint, build, and release snapshot.
- [x] Versioning and changelog are in place.
- [x] Release checklist is complete.
- [x] v1 known limitations are documented.
- [ ] Homebrew and Mason installs — deferred to post-v1.
- [x] `v1.0.0` is tagged and published.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Cross-platform build failures | CI matrix builds every target on every PR; goreleaser snapshot. |
| Package-manager lag | Homebrew and Mason are explicitly deferred to post-v1; v1 documents GitHub binaries and `go install`. |
| Unsigned macOS/Windows binaries | Document Gatekeeper/SmartScreen warnings; consider signing post-v1. |
| Neovim config drift across versions | Target 0.11+; include a fallback `vim.lsp.start` example for older versions if cheap. |
| Included `.lua` attachment is confusing | Ship the helper plugin, document manual fallback, test that unrelated Lua buffers are not attached, and make it clear `.p8` is primary. |
| Docs drift from behavior | Generate config/diagnostic tables from code where possible; add docs checks to release checklist. |
