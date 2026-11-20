# Change Log

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
