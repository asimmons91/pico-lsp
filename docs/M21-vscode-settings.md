# M21 — Full Server Settings Schema in the VS Code Client

**Status:** Proposed — not started

**Depends on:** M15  
**Release target:** v1.9.0 (may ship with M22)  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

Every documented `pico8.*` server setting appears in VS Code's Settings UI and
`settings.json` with the correct type, enum, default, scope, and description.
Users can discover settings by searching for “PICO-8”, get completion and
validation while editing `settings.json`, and see the same defaults the server
uses. The server and extension behavior do not change; this milestone adds
configuration metadata and drift tests.

**Demo:** Open **Settings** and search for `pico8`. The results include
`PICO-8 › Type Checking`, `PICO-8 › Diagnostics › Undefined Global`,
`PICO-8 › Limits › Code Lens`, `PICO-8 › Format › Max Blank Lines`, and the
existing client settings. Editing `"pico8.typeChecking": "stric"` shows a
validation error, while `"strict"` is accepted. A repository test fails if a
server setting, default, or enum is added without updating the VS Code schema.

## Decisions proposed

| ID | Decision | Choice |
|---|---|---|
| D1 | Schema location | Contribute the settings inline in `editors/vscode/package.json`. Do not add a build-time generator in this milestone. |
| D2 | Behavior source of truth | `internal/config` remains authoritative for defaults and allowed values; the VS Code schema is metadata that must match it. |
| D3 | Drift protection | Add a Go test that reads the extension manifest and compares every server setting, default, and enum with `internal/config`. |
| D4 | Scope | All top-level server settings use `"scope": "window"` because the server pulls the `pico8` section without a per-resource scope URI. `pico8.serverPath` remains machine-scoped. |
| D5 | Nested objects | Give `limits`, `format`, `diagnostics`, `lint`, and `refactors` `"additionalProperties": false` so typos are visible. |
| D6 | Descriptions | Every property has a description; complex settings use `markdownDescription` with links to `docs/configuration.md` and `docs/diagnostics.md`. |
| D7 | Glyph schema | Preserve the existing `pico8.glyphs` schema and behavior. Do not tighten it beyond today's fields. |
| D8 | Runtime behavior | No server defaults, configuration parsing, extension startup, or LSP behavior changes. |
| D9 | Release | M21 may ship together with M22 as v1.9.0; the milestones remain independently reviewable. |

## Scope

### In scope

- `package.json` contributions for all documented server settings.
- Correct types, enums, integer minimums, defaults, descriptions, and scopes.
- Nested objects for `limits`, `format`, `diagnostics`, `lint`, and `refactors`.
- A Go drift test against `internal/config`.
- Extension unit and integration tests for the contributed defaults.
- README, configuration, troubleshooting, changelog, roadmap, and backlog
  updates.

### Out of scope

- Changing server configuration behavior, defaults, or validation.
- API-version selection or additional PICO-8 built-in sets.
- Resource-scoped or language-scoped server settings.
- A generated `package.json` or a new settings-generation tool.
- Custom Settings UI views, webviews, or configuration wizards.
- Marketplace/Open VSX publishing.
- Remote host support and server-health reporting; those are M20 and M22.

## Settings inventory

All settings below are contributed under the `PICO-8` configuration title.
Server settings are `window`-scoped; extension-only settings keep their current
scopes.

| Setting | Type | Default | Values / notes |
|---|---|---:|---|
| `pico8.apiVersion` | string enum | `"0.2.7"` | Only `0.2.7` is supported; unsupported values warn and fall back. |
| `pico8.typeChecking` | string enum | `"basic"` | `off`, `basic`, `strict`. |
| `pico8.inlayHints` | boolean | `false` | Show inferred local type hints. |
| `pico8.maxProblems` | integer | `100` | Minimum `0`; `0` disables truncation. |
| `pico8.limits.codeLens` | boolean | `true` | Show the cart-budget code lens. |
| `pico8.limits.diagnostics` | boolean | `true` | Publish token, character, and compressed-size limit diagnostics. |
| `pico8.format.maxBlankLines` | integer | `2` | Minimum `0`; `0` removes consecutive blank lines. |
| `pico8.diagnostics.mode` | string enum | `"auto"` | `auto`, `push`, `pull`; `auto` pulls when supported and pushes otherwise. |
| `pico8.diagnostics.undefinedGlobal` | severity enum | `"warning"` | `off`, `hint`, `info`, `warning`, `error`. |
| `pico8.diagnostics.unusedLocal` | severity enum | `"off"` | |
| `pico8.diagnostics.unreachableCode` | severity enum | `"hint"` | |
| `pico8.diagnostics.duplicateTableField` | severity enum | `"hint"` | |
| `pico8.diagnostics.callbackPrecedence` | severity enum | `"hint"` | |
| `pico8.diagnostics.callbackParams` | severity enum | `"warning"` | |
| `pico8.diagnostics.noStdlib` | severity enum | `"hint"` | |
| `pico8.diagnostics.shadowedLocal` | severity enum | `"off"` | |
| `pico8.diagnostics.emptyBlock` | severity enum | `"off"` | |
| `pico8.diagnostics.globalInFunction` | severity enum | `"off"` | |
| `pico8.diagnostics.includeOutsideWorkspace` | severity enum | `"warning"` | |
| `pico8.diagnostics.includeUnusualType` | severity enum | `"hint"` | |
| `pico8.diagnostics.includeDuplicate` | severity enum | `"off"` | |
| `pico8.lint.apiRanges` | boolean | `false` | Optional literal API range checks. |
| `pico8.refactors.enabled` | boolean | `true` | Offer M14 code actions and refactors. |
| `pico8.glyphs` | array | `[]` | Existing custom P8SCII glyph schema. |
| `pico8.serverPath` | string | `""` | Machine-scoped; explicit server executable. |
| `pico8.autoDownload` | boolean | `true` | Window-scoped; download the matching verified server. |
| `pico8.attachLua` | boolean | `true` | Window-scoped; attach to `.lua` files in PICO-8 workspace folders. |
| `pico8.trace.server` | string enum | `"off"` | `off`, `messages`, `verbose`. |

## Detailed requirements

### 1. Manifest structure

- Keep one `contributes.configuration` object with title `PICO-8`, or split
  server and client settings into separate titled objects only if the Settings
  UI remains clear. Do not duplicate keys.
- Every property has a `description` or `markdownDescription`.
- Object settings (`limits`, `format`, `diagnostics`, `lint`, `refactors`)
  declare their nested `properties` and `"additionalProperties": false`.
- Leaf defaults match `internal/config.Default()` exactly.
- Enum arrays match the server constants exactly:
  - `typeChecking`: `off`, `basic`, `strict`.
  - `diagnostics.mode`: `auto`, `push`, `pull`.
  - every severity rule: `off`, `hint`, `info`, `warning`, `error`.
  - `apiVersion`: `0.2.7`.
- `maxProblems` and `format.maxBlankLines` declare `"minimum": 0` and
  `"type": "integer"`.
- `pico8.serverPath` keeps `"scope": "machine"`.
- All other top-level server settings declare `"scope": "window"`; nested leaf
  settings inherit the object's scope.
- The existing `pico8.glyphs` item schema is preserved, including the `oneOf`
  requirement for exactly one of `byte`, `glyph`, or `text`.
- Use absolute documentation links in `markdownDescription` so the package
  renders correctly outside the repository, for example
  `https://github.com/asimmons91/pico-lsp/blob/main/docs/configuration.md#settings`.

### 2. Descriptions

Descriptions must be concise and behaviorally accurate. At minimum:

- `apiVersion` states that only `0.2.7` is currently supported and that
  unsupported values fall back with a warning.
- `typeChecking` explains `off` (annotations for hover/completion, no type
  diagnostics), `basic` (explicit annotation checks), and `strict` (nil and
  missing-field checks).
- `maxProblems` states that `0` disables truncation.
- `diagnostics.mode` explains the pull/push fallback.
- Severity settings link to the diagnostic reference.
- `limits` explains that the two switches control the cart-budget code lens
  and limit diagnostics.
- `format.maxBlankLines` states that `0` removes blank lines.
- `refactors.enabled` states that it controls the M14 actions.
- `glyphs` keeps the current description and item requirements.
- `serverPath`, `autoDownload`, `attachLua`, and `trace.server` keep their
  current descriptions.

### 3. Drift tests

Add `internal/config/vscode_settings_test.go` (or an equivalent test package)
that:

- locates `editors/vscode/package.json` from the repository root;
- parses `contributes.configuration` whether it is an object or an array;
- collects the `pico8.*` properties;
- compares the server property set against `config.Default()` and the known
  extension-only keys;
- compares every leaf default with `config.Default()`, including nested
  `limits`, `format`, `diagnostics`, `lint`, and `refactors`;
- compares enum values with the exported constants in `internal/config`;
- checks `minimum: 0` for `maxProblems` and `format.maxBlankLines`;
- checks `additionalProperties: false` on the nested objects;
- fails with a message that names the missing or mismatched setting and tells
  the contributor to update both `internal/config` and the VS Code manifest.

The test must run as part of `go test ./...` and must not require Node.js or a
built extension.

Add a Vitest test that:

- loads the manifest;
- asserts every contributed property has a description;
- asserts the nested object properties are present and described;
- asserts the `pico8.glyphs` item schema still requires `name` and exactly one
  of `byte`, `glyph`, or `text`.

### 4. Integration and packaging checks

- In the VS Code integration suite, assert
  `workspace.getConfiguration('pico8').inspect('typeChecking')?.defaultValue`
  is `basic`, and that nested defaults for `diagnostics.undefinedGlobal`,
  `limits.codeLens`, and `format.maxBlankLines` are visible.
- Assert one extension-only setting still has its expected default; the
  manifest test checks its declared scope.
- `npm run package`/`vsce` must accept the manifest and produce the VSIX.
- Manual check: search Settings for `pico8`, verify all settings render with
  descriptions, and verify an invalid enum produces a validation warning in
  `settings.json`.
- Do not add a settings UI webview or custom editor for this milestone.

### 5. Documentation

- `editors/vscode/README.md`: state that all server settings appear in the
  Settings UI and link to `docs/configuration.md`; keep the client-only table
  or replace it with a short grouped list.
- `docs/configuration.md`: add a VS Code section explaining the Settings UI,
  `window` scope, `serverPath` machine scope, and the fact that the server also
  accepts the same settings from other LSP clients.
- `docs/troubleshooting.md`: mention schema validation errors and how to
  compare effective defaults.
- `CHANGELOG.md` under `Unreleased`, then the v1.9.0 release entry.
- Update `docs/ROADMAP.md` and the post-v1 backlog when the milestone status
  changes.

## Interfaces and state

- No new server request, response, command, or setting key.
- The extension forwards the same `pico8` section through
  `initializationOptions` and `workspace/configuration`; the schema only makes
  it discoverable.
- VS Code owns validation and presentation. The server remains forgiving of
  invalid values and falls back to defaults as it does today.
- The Go drift test is a repository test, not a runtime dependency.

## Error handling

- Invalid `settings.json` values are reported by VS Code's schema validation;
  the server still normalizes or falls back at runtime.
- A missing or renamed server setting is a test failure, not a runtime warning.
- A future server setting added without schema metadata must fail the drift
  test rather than silently disappearing from the Settings UI.
- Descriptions must not promise behavior the server does not implement.

## Testing and CI

- `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt -l .`, and
  `golangci-lint run ./...` remain green, including the new drift test.
- Extension lint, typecheck, unit tests, integration tests, and VSIX packaging
  remain green.
- The drift test runs on Linux, macOS, and Windows CI like the rest of the Go
  suite.
- Manual Settings UI check is recorded in the release checklist.
- No network access is added to tests.

## Exit criteria

- [ ] Every documented server setting is present in `package.json` with the
      correct type, default, enum, minimum, scope, and description.
- [ ] Nested `limits`, `format`, `diagnostics`, `lint`, and `refactors` objects
      reject unknown keys.
- [ ] `pico8.glyphs` keeps its existing schema and behavior.
- [ ] The Go drift test fails on a missing setting or changed default/enum and
      passes on the current tree.
- [ ] Extension unit and integration tests assert the contributed defaults.
- [ ] A manual Settings search shows the full PICO-8 section, and invalid enum
      values are flagged.
- [ ] README, configuration, troubleshooting, changelog, roadmap, and backlog
      are updated.
- [ ] No server behavior, extension startup behavior, or LSP capability
      changes.
- [ ] `v1.9.0` is tagged after the release PR merges (possibly with M22).

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Schema drifts from server defaults | Go drift test compares defaults, enums, keys, and minimums; CI fails before release. |
| Settings appear editable but do not apply per folder | Use `window` scope and document that the server pulls one workspace-level `pico8` section. |
| A future server setting is forgotten | The drift test requires the new key to appear in the manifest in the same change. |
| `additionalProperties: false` blocks a future key | Update the schema and test in the same change that adds the server setting; the failure is intentional. |
| Duplicated descriptions drift from the docs | Link to the docs for detail and keep schema descriptions short; review both in the same PR. |
| Settings UI becomes noisy | Group nested settings, use accurate defaults, and keep client-only settings distinct from server settings. |
| The Go test couples config to the editor package | Keep the test narrow and read-only; it enforces the product contract rather than importing editor code. |
