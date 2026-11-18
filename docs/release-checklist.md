# Release checklist

This checklist covers the v1.x GitHub-only release process. Homebrew and Mason
publishing are deferred to post-v1. The current release is **v1.2.0**.

## Pre-release

- [x] `main` is green on Linux, macOS, and Windows CI.
- [x] `go test -race ./...` passes locally or on Linux CI.
- [x] `gofmt -l cmd internal` is empty.
- [x] `go vet ./...` passes.
- [x] `golangci-lint run ./...` passes.
- [x] `govulncheck ./...` passes.
- [x] The full corpus tests pass without panics.
- [x] Performance targets in the roadmap are met.
- [x] `goreleaser check` validates `.goreleaser.yaml`.
- [x] `goreleaser release --snapshot --clean` builds archives and checksums.
- [x] `pico-lsp --version` reports the intended release tag.
- [x] README, install, configuration, diagnostics, and troubleshooting docs are
      accurate.
- [x] Known limitations are documented: formatter, `.p8.png`, VS Code/Zed,
      Homebrew/Mason deferral, unsigned binaries.
- [x] `CHANGELOG.md` has a dated `1.2.0` section.
- [x] `LICENSE` and the PICO-8 trademark disclaimer are present.
- [x] The test cart `testdata/smoke.p8` opens and attaches in Neovim.
- [x] The optional Neovim helper attaches included `.lua` buffers and ignores
      unrelated Lua buffers.
- [x] Helix opens `testdata/smoke.p8` and attaches; run the manual feature matrix
      before publishing if Helix behavior changed. The v1.1.0 release re-ran the
      automated and Neovim checks; v1.2.0 did not change Helix integration, so
      the manual Helix matrix from v1.0.0 remains the last manual run.
- [x] No stdout writes occur outside LSP framing.
- [x] No telemetry or network calls are added.

## Tag and publish

- [x] Merge the release pull request into `main`.
- [x] Verify the merge commit is green in CI.
- [x] Create the annotated tag:

  ```sh
  git tag -a v1.2.0 -m "pico-lsp v1.2.0"
  git push origin v1.2.0
  ```

- [x] Watch the `Release` workflow complete.
- [x] Confirm the GitHub release contains Linux, macOS, and Windows archives for
      amd64 and arm64 plus `checksums.txt`.
- [x] Download and run `pico-lsp --version` from at least one release archive.
- [x] Verify the checksum for the downloaded archive.
- [x] Confirm the release notes contain install commands and the Go version and
      commit.

## Post-release

- [x] Verify `go install github.com/asimmons91/pico-lsp/cmd/pico-lsp@v1.2.0`
      builds for a developer.
- [x] Update any documentation that needs the released version.
- [ ] Open post-v1 tracking issues for Homebrew, Mason, formatter, `.p8.png`,
      VS Code, and Zed work as needed.

## VS Code extension (v1.3.0+)

- [ ] `editors/vscode/package.json` version matches the release tag.
- [ ] `npm ci`, lint, typecheck, unit tests, and integration tests pass.
- [ ] `pico-lsp-vscode-<version>.vsix` builds and contains no extension source.
- [ ] The VSIX and `.vsix.sha256` are attached to the GitHub release.
- [ ] Install the VSIX on Linux, macOS, and Windows local desktop VS Code.
- [ ] First-run download, checksum failure, offline `serverPath`, and restart
      paths work.

## Rollback

If a release is broken:

1. Mark the GitHub release as a pre-release or delete it.
2. Do not force-move the tag; publish a patch release instead.
3. Document the issue in the changelog and release notes.
