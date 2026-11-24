# Security policy

## Supported versions

Security fixes are applied to the current release line. The latest release is
listed on the [releases page](https://github.com/asimmons91/pico-lsp/releases).
Older releases may not receive fixes.

## Reporting a vulnerability

Use GitHub's private vulnerability reporting:

https://github.com/asimmons91/pico-lsp/security/advisories/new

Do not open a public issue for a suspected vulnerability. Include:

- the affected `pico-lsp` version (`pico-lsp --version`);
- the editor and version;
- the operating system and architecture;
- a minimal cart or configuration that reproduces the issue;
- the impact you believe is possible;
- any relevant logs with private source removed.

If the private advisory form is unavailable, open a public issue that asks for a
private contact path without including exploit details.

## Scope

This policy covers:

- the `pico-lsp` server binary;
- the VS Code extension in `editors/vscode`;
- the public packages under `pkg/`;
- the repository's release and publication workflows.

It does not cover PICO-8 itself, third-party editors, or third-party language
servers.

## Security posture

- Cart code is parsed and analyzed, never executed.
- The server has no telemetry, analytics, or network access.
- The VS Code extension downloads the matching server only from this project's
  GitHub releases and verifies its SHA-256 checksum before execution.
- Untrusted VS Code workspaces are disabled.
- Logs may contain source excerpts only when trace logging is explicitly
  enabled. Review a log before sharing it.

## Disclosure

We will acknowledge a report, investigate it, and coordinate a fix and
disclosure through the GitHub advisory. There is no bug bounty.
