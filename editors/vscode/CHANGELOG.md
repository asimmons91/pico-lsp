# Change Log

## 1.8.1

- Publish the extension to the VS Code Marketplace and Open VSX as
  `asimmons91.pico-lsp`.
- Rename the display name to **PICO-8 LSP** and replace the official PICO-8
  logo with original artwork for the listing icon and the `.p8` file icon.
- Rewrite the packaged README as the Marketplace/Open VSX listing, including
  install paths, requirements, privacy, limitations, support links, the AI
  disclosure, and the trademark notice.
- Add listing metadata: homepage, issue tracker, gallery banner, and search
  keywords.
- Exclude internal icon notes and image sources from the VSIX.
- Bundle `pico-lsp` 1.8.1. No language-server or extension feature behavior
  changed.

## 1.8.0

- Bundle `pico-lsp` 1.8.0 with annotation-aware navigation, token-saving code
  actions, and the full server settings schema.
- Contribute the full `pico8.*` server settings schema to the VS Code Settings
  UI, including types, enums, defaults, window scopes, and descriptions.
- Reject unknown keys in the `limits`, `format`, `diagnostics`, `lint`, and
  `refactors` settings objects.
- Read the running server version from `initializeResult`, warn once when a
  release server does not match the extension, and label development builds
  without a mismatch warning.
- Track whether the server came from `pico8.serverPath`, a cached download, or
  a new download.
- Add **PICO-8: Show Server Info** and **PICO-8: Copy Server Diagnostics** for
  versions, source, path, platform, cache root, and the last startup error.

## 1.7.0

- Bundle `pico-lsp` 1.7.0 with the complete printable P8SCII glyph catalog for
  the picker and completion.
- Escape ASCII quotes and backslashes when inserting glyphs inside string
  literals.

## 1.6.0

- Bundle `pico-lsp` 1.6.0 with the public `pkg/p8scii`, `pkg/cart`,
  `pkg/lsp`, and `pkg/glyphs` Go packages.
- Recognize PICO-8 tab markers only at column 0.
- Preserve lone CRs as P8SCII data instead of starting a new line.

## 1.5.1

- Show cart and source-file paths relative to the workspace in the cart budget tooltip.
- Bundle `pico-lsp` 1.5.1, which normalizes Lua files to one trailing newline when formatting.

## 1.5.0

- Show the PICO-8 logo as the file icon for `.p8` files when the active file
  icon theme supports language default icons.
- Bundle `pico-lsp` 1.5.0, which adds LuaCATS annotation semantic highlighting
  and completes members while a receiver statement is still being typed.

## 1.4.0

- Add P8SCII glyph completion and the **PICO-8: Insert P8SCII Glyph** picker.
- Add the `pico8.glyphs` setting for custom built-in and custom font-slot glyph entries.

## 1.3.0

- Initial VS Code extension release.
- Download and checksum-verify the matching `pico-lsp` release binary.
- Attach to `.p8` files and `.lua` files in PICO-8 workspace folders.
- Contribute PICO-8 syntax highlighting and language configuration.
- Show cart token, character, and compressed-size budgets in the status bar.
- Add commands for cart info, server restart, output, and cache cleanup.
