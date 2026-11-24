# Contributing

Thanks for helping improve `pico-lsp`. This is an AI-assisted project: most code
and documentation are produced by AI agents under human direction and review.
Inspect changes for your use case before relying on them in critical workflows.

## Development setup

Requirements:

- Go 1.27.4, pinned in `mise.toml`;
- Node.js 26.13.1 for the VS Code extension, also pinned in `mise.toml`;
- `golangci-lint` and `govulncheck` for the full gates.

```sh
mise install
go version          # should report 1.27.4
go build ./...
```

## Before opening a pull request

Run the checks that apply to your change:

```sh
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
golangci-lint run ./...
govulncheck ./...
```

For the VS Code extension:

```sh
cd editors/vscode
npm ci
npm run lint
npm run typecheck
npm run test:unit
npm run test:integration
npm run package
```

For the Neovim helper and Tree-sitter grammar, see the commands in
`editors/nvim/tests/run.sh` and `editors/tree-sitter-pico8/test.sh`.

Documentation changes must keep relative links valid. The `go test ./...` run
includes a local Markdown link check.

## Branches and commits

- Branch from an up-to-date `main`.
- Use `feat/<short-slug>`, `fix/<short-slug>`, or `chore/<short-slug>`.
- Use Conventional Commits: `<type>(<optional scope>): <description>`.
- Use an imperative, lowercase description with no trailing period.
- Mark breaking changes with `!` and/or a `BREAKING CHANGE:` footer.
- Open a pull request; do not push directly to `main`.

## Code review

Open a pull request and wait for the required review and green CI. Address
review findings on the same branch; do not open a separate pull request for
review fixes.

## Documentation rules

- `docs/ROADMAP.md` is the roadmap index.
- Milestone specs under `docs/M*.md` own scope, interfaces, and acceptance
  criteria.
- Keep `docs/ROADMAP.md` links valid when adding or renaming milestone files.
- Update the roadmap and any affected spec when a locked decision changes.
- Do not invent build or test results; report the commands you ran.

## Reporting bugs

Use the bug report form in the issue tracker. Include:

- `pico-lsp --version`;
- editor and version;
- operating system and architecture;
- the VS Code **PICO-8: Copy Server Diagnostics** block when applicable;
- a minimal cart or include;
- relevant log lines with private source removed.

Security issues go through [SECURITY.md](SECURITY.md), not the public tracker.

## License

Contributions are accepted under the MIT license. See [LICENSE](LICENSE).
