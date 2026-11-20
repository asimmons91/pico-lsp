# M16 — P8SCII Glyph Snippets and Input

**Status:** Done — shipped in v1.4.0; the catalog was expanded to every
printable P8SCII byte in v1.7.0.  
**Depends on:** M15  
**Release target:** v1.4.0; v1.7.0 catalog expansion.  
**Roadmap:** [ROADMAP.md](ROADMAP.md)

## Outcome

PICO-8's P8SCII glyphs and user-declared custom font slots become first-class,
discoverable editor input. The server owns a glyph catalog and offers
ASCII-triggered completion that inserts the canonical Unicode spelling for the
selected P8SCII byte. A searchable picker command lets users browse and insert
glyphs without knowing their Unicode codepoints. The feature works in `.p8`
code, cart-owned `.lua` files, and string literals, and insertion respects
UTF-16 positions, P8SCII character counting, and compressed-size encoding.

**Demo:** A developer types `p8_le` in code or in a string and accepts the
`⬅️  p8_left` completion item; the alias is replaced with `⬅️`. Typing `p8_x`
offers `❎`, and `p8_heart` offers `♥`. A developer working on a cart with a
custom font at `0x5600` configures a `logo` entry for byte `0x80`; selecting it
inserts `█`, the canonical Unicode spelling for that P8SCII slot, while PICO-8
draws the cart's custom shape at runtime. The command **PICO-8: Insert P8SCII
Glyph** opens a searchable picker showing glyph, name, byte, category, and
description. Inside a string, `p8_font_custom` inserts the `\14` escape that
switches to the custom font.

## Decisions

Confirmed before implementation:

| ID | Decision | Choice |
|---|---|---|
| D1 | Meaning of “custom” | Built-in glyph aliases, byte-based custom font slots, and string-only control-code snippets. |
| D2 | Completion trigger | `p8_` primary, `p8-` compatibility, and a searchable picker. |
| D3 | Where custom definitions live | `pico8.glyphs` in user/workspace settings first. A per-cart definitions file is deferred. |
| D4 | Catalog breadth | Every printable P8SCII byte (`0x10`–`0xff`) is cataloged, including ASCII, puny-font letters, Hiragana, and Katakana, plus custom entries. |
| D5 | Editor scope | Server surface, VS Code Quick Pick, and the optional Neovim helper command. Helix gets completion. |
| D6 | Name conflicts | Custom names and aliases override built-ins with a logged warning. |
| D7 | Static editor snippet files | None; the server catalog is the source of truth. |
| D8 | Control-code coverage | Common escapes only; the full `0`–`15` set and arbitrary `\0xxx` forms are deferred. |

## Scope

### In scope

- A server-owned P8SCII glyph catalog derived from `internal/p8scii`.
- Stable names, aliases, categories, and descriptions for every printable
  P8SCII byte, plus the curated custom-font slots.
- pico8-ls-compatible aliases such as `p8-left`, `p8-x-key`, and `p8-jelpi`
  where they map cleanly.
- User-defined glyph entries in `pico8.glyphs`, with a name and either a P8SCII
  byte, a P8SCII character, or a control-code escape.
- ASCII-triggered completion in code and string contexts.
- A `pico8/glyphs` request and a `pico8.insertGlyph` command.
- A VS Code Quick Pick command and a Neovim `:Pico8InsertGlyph` command.
- Helix support through the standard completion path.
- Validation, configuration, tests, and documentation.

### Out of scope

- Rendering, previewing, or inferring custom font pixels.
- Parsing `poke(0x5600, ...)` or other runtime font initialization.
- Changing P8SCII byte-to-glyph mapping or PICO-8 version support.
- General-purpose Unicode snippet expansion unrelated to P8SCII.
- Per-cart glyph definition files; M16 reads definitions from settings only.
- Per-glyph aliases for every ASCII and kana entry. The primary `p8_<name>`
  trigger is always available.
- New source syntax for glyph aliases; aliases are completion-only and must not
  change the language.
- Automatic replacement without accepting a completion item or invoking the
  command.
- Editing read-only `.p8.png` virtual documents.
- Cart execution, emulation, sprite/audio tooling, or `.p8.png` writing.
- Marketplace/Open VSX publishing or a native Helix picker.

## Requirements

### Catalog

- Add exported glyph metadata in `internal/p8scii` (or a small
  `internal/glyphs` package that imports it). Do not duplicate the 256-byte
  table.
- Each entry includes:
  - stable ASCII `name`;
  - canonical Unicode `text`;
  - optional `stringText` for source spellings that differ inside a string
    literal, such as ASCII `"` (`\"`) and `\` (`\\`);
  - P8SCII `byte` (`0`–`255`);
  - optional aliases;
  - category (`button`, `symbol`, `ascii`, `puny`, `kana`, `control`, `custom`);
  - human-readable description.
- Canonical text is produced with `p8scii.Decode([]byte{byte})`; never by
  hand-writing multi-rune glyph spellings.
- For glyph entries, `p8scii.Encode(text)` must return exactly one byte. This
  check is on bytes, not runes, so base-codepoint-plus-`U+FE0F` glyphs such as
  `⬅️` pass. Accepted text is canonicalized back through
  `p8scii.Decode(byte)` before insertion.
- The built-in catalog covers every printable byte `0x10`–`0xff`:
  - `0x10`–`0x1f`, `0x7f`–`0x99`, and `0xfe`–`0xff` keep their curated symbol
    names and aliases;
  - `0x20`–`0x7e` receive ASCII names (`space`, `punct_*`, `digit_*`,
    `letter_*`, and `puny_*` for the puny-font uppercase slots);
  - `0x9a`–`0xfd` receive `hiragana_*` and `katakana_*` names.
- Control codes `0x00`–`0x0f` are marked as string-only and insert their source
  escape spelling (for example `\14`), not a raw control byte.
- The initial control-code set is `\*`, `\#`, `\-`, `\|`, `\+`, `\^`, `\a`,
  `\f`, `\14`, and `\15`. The remaining control codes and arbitrary `\0xxx`
  forms are deferred.
- Custom entries are merged after the built-in catalog. A duplicate custom
  name or alias overrides the built-in entry. Invalid entries are skipped with
  a single logged warning per entry; they never fail completion.
- Catalog construction is cheap and may be rebuilt per request. Do not add
  incremental infrastructure for 256 entries.

### Configuration

The schema for this milestone:

```json
{
  "pico8": {
    "glyphs": [
      { "name": "logo", "byte": 128, "description": "custom font slot 0x80" },
      { "name": "heart", "glyph": "♥" },
      { "name": "font_custom", "text": "\\14", "context": "string" }
    ]
  }
}
```

- Each entry has exactly one of `byte`, `glyph`, or `text`. Field presence, not
  a zero value, distinguishes `byte: 0` from an omitted field.
- `byte` is authoritative for custom font slots; the server inserts the
  canonical Unicode spelling for that byte.
- Byte and glyph entries must resolve to printable P8SCII values (`0x10`–`0xff`).
- Control codes `0x00`–`0x0f` must use a `text` entry with
  `"context": "string"`, not a `byte` or `glyph` entry.
- `glyph` is accepted only when it encodes to exactly one P8SCII byte.
- `text` is intended for control-code escapes and must set
  `"context": "string"` so it is offered only in strings. Glyph and byte entries
  default to both identifier and string contexts.
- `name` is ASCII and matches `[a-z0-9_]+` case-insensitively. Every entry is
  available as `p8_<name>`; aliases add additional triggers.
- Aliases are full completion triggers, for example `p8_logo` or `p8-logo`.
  Aliases must begin with `p8_` or `p8-` so they stay inside the glyph
  namespace and do not pollute ordinary identifier completion.
- Duplicate custom names and aliases override built-ins with a logged warning;
  when two custom entries collide, the later list entry wins. Invalid entries
  are skipped rather than shadowing a valid definition.
- Invalid entries do not produce cart diagnostics; configuration warnings go to
  the server log.
- Configuration changes take effect without restarting the server.
- Omitting `pico8.glyphs` preserves the current/default custom entries; an
  explicit empty list clears them.
- Definitions are read from the `pico8.glyphs` settings list, which is scoped to
  the user/workspace rather than discovered from cart contents. Included `.lua`
  files use the same workspace settings as their owning cart.

### Completion

- Support `p8_<name>` as the primary trigger and `p8-<name>` for pico8-ls
  compatibility. A dotted `p8.` namespace is not planned because `.` is already
  member access.
- Glyph triggers use an ASCII scanner over letters, digits, `_`, and `-`.
  `p8-` needs this scanner because `-` terminates normal identifier completion;
  the edit must replace the entire `p8-...` trigger.
- Check for glyph completion before the normal string/comment early returns in
  `internal/lsp/completion.go` so glyphs can be inserted inside strings.
- Offer glyph items only when the trigger prefix is present. Do not dump the
  catalog on ordinary identifier completion.
- Do not offer normal identifier, keyword, or code-snippet completions inside
  strings.
- Do not offer glyph completion in comments, annotation contexts, or `#include`
  paths.
- Completion item behavior:
  - label shows the glyph and alias, for example `⬅️  p8_left`;
  - `filterText` includes the alias, name, glyph, and byte in hex;
  - detail shows `P8SCII 0x8b · left button` or the custom description;
  - documentation shows the glyph, byte, Unicode codepoint sequence, category,
    and the custom-font caveat when applicable;
  - `textEdit` replaces the full trigger using `LineIndex.Range`;
  - `insertTextFormat` is `PlainText` for single-character insertion;
  - kind is `Constant` for glyphs and `Snippet` only for template-style entries.
- If no catalog entry matches, return no glyph items rather than falling through
  to unrelated completions.
- Alias matching is ASCII case-insensitive, matching PICO-8 identifier folding.
- A completed glyph is inserted as canonical Unicode. For a font-slot entry the
  inserted text may look like a different built-in glyph; the label and
  description identify the custom slot.
- `p8_` is deliberately safer than `p8-`: if completion is dismissed,
  `p8_left` remains a valid identifier, while `p8-left` parses as subtraction.

### Picker and command

- Add a custom request `pico8/glyphs`:
  - accepts optional `textDocument.uri` and `position` together so clients can
    request context-appropriate entries and future per-cart definitions;
  - when a position is supplied, filters out string-only entries outside a
    string and returns each entry's `context`;
  - returns a stable JSON list of entries with `name`, `aliases`, `text`,
    `byte`, `category`, `description`, and `context`.
- Add `pico8.insertGlyph` to `workspace/executeCommand`:
  - accepts `textDocument`, `position`, and exactly one of `name` or `byte`;
  - validates the document is open and editable;
  - rejects a string-only entry when the position is not inside a string;
  - replaces a `p8_` or `p8-` trigger immediately before the cursor when
    present, otherwise inserts at the cursor;
  - refuses read-only `.p8.png` virtual documents;
  - applies a standard `WorkspaceEdit` through `workspace/applyEdit` when the
    client advertises it, otherwise returns the edit for the client to apply;
  - never writes files directly.
- Register the request alongside `pico8/cartInfo` and `pico8/decodeCart`, and
  advertise the command in `executeCommandProvider`.

### Editor integration

- **VS Code:** contribute `pico8.client.insertGlyph` in `contributes.commands`
  and `activationEvents`, and add `pico8.glyphs` to `contributes.configuration`.
  The command requests `pico8/glyphs` with the active cursor position, shows a
  Quick Pick with `matchOnDescription` and `matchOnDetail`, and invokes
  `pico8.insertGlyph`. Provide an optional keybinding placeholder but no default
  conflict-prone shortcut.
- **Neovim:** add `:Pico8InsertGlyph` to the optional helper. Use
  `vim.ui.select`, pass the cursor position to `pico8/glyphs`, and use the
  standard LSP client; do not require Telescope.
- **Helix:** completion works through `textDocument/completion`. A native picker
  is out of scope until Helix exposes a suitable command UI.
- **Static snippets:** do not ship hand-maintained per-editor glyph lists. If a
  fallback is later required, generate it from the server catalog (D7).

### Encoding and budgets

- The inserted glyph itself must be one P8SCII character and encode to exactly
  one compressed byte. Completion replaces a longer `p8_...` alias, so the net
  change in cart size depends on the trigger length.
- Control-code entries insert source escape text such as `\14`; the character
  and compressed budgets count the source characters, not the runtime byte the
  escape produces.
- Multi-rune glyphs such as `⬅️` occupy two UTF-16 code units in LSP positions
  but one P8SCII character and one compressed byte. Tests must cover this.
- Completion must not change token counts until an edit is accepted.

## Testing

- Catalog unit tests:
  - every built-in entry round-trips through `Decode`/`Encode` as one byte;
  - multi-rune button glyphs are canonical;
  - control entries are string-only;
  - duplicate custom names and aliases override built-ins with a warning;
  - invalid bytes, unknown glyphs, and multi-byte encodings are skipped.
- Configuration tests:
  - partial settings preserve defaults;
  - `byte`, `glyph`, and `text` forms work;
  - control bytes and control glyphs are rejected in favor of `text` entries;
  - string-only entries without a string context are rejected;
  - invalid entries warn once and are skipped;
  - changes refresh completion and the picker without restart.
- Completion e2e tests:
  - `p8_le` → `⬅️` and `p8_left` → `⬅️`;
  - `p8_x` → `❎`, `p8_heart` → `♥`;
  - glyph completion inside strings and not in comments/annotations/includes;
  - no glyph dump without a trigger;
  - no regression to symbol, built-in, keyword, or snippet completion;
  - UTF-16 edit ranges after `⬅️` and other multi-rune glyphs;
  - custom font slot insertion inserts the canonical stand-in glyph.
- Command and picker tests:
  - `pico8/glyphs` response is stable and includes custom settings entries;
  - a position-limited request filters string-only entries in code;
  - `pico8.insertGlyph` inserts at cursor and replaces a typed trigger;
  - string-only entries are rejected outside strings;
  - invalid or missing arguments are rejected;
  - read-only `.p8.png` virtual documents are rejected;
  - `workspace/applyEdit` and fallback-result paths both work.
- Budget tests:
  - the inserted glyph encodes to one compressed byte;
  - net character/compressed changes account for the replaced trigger;
  - `\14` counts as source characters;
  - token counts are unchanged until acceptance.
- Editor tests:
  - VS Code Quick Pick command and insertion;
  - Neovim `:Pico8InsertGlyph`;
  - Helix completion smoke test.
- Performance: completion and picker requests stay within the existing
  editor-request budget; no new background work is needed for a 256-entry
  catalog.

## Exit criteria

- [x] D1–D8 are resolved and recorded in this spec.
- [x] The server-owned catalog covers the agreed glyph set and validates
      one-byte encoding.
- [x] `p8_` completion works in code and strings; `p8-` compatibility works.
- [x] Custom glyph entries work from `pico8.glyphs` and override built-ins with
      a warning.
- [x] `pico8/glyphs` and `pico8.insertGlyph` are implemented and tested.
- [x] VS Code and Neovim integration works; Helix behavior is documented.
- [x] UTF-16, character-budget, and compressed-size tests pass.
- [x] Configuration, API, README, troubleshooting, and editor docs are updated.
- [x] Existing completion, position, and budget behavior does not regress.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| The server cannot know custom font pixels | Require a user-declared byte mapping; insert the canonical Unicode stand-in; document the limitation. |
| `p8-` aliases parse as subtraction if not completed | Make `p8_` primary; document `p8-` as compatibility. |
| Glyph insertion breaks compressed-size assumptions | Validate `Encode(text)` is exactly one byte and always use `Decode(byte)` for canonical text. |
| Completion inside strings interferes with other features | Only activate with a glyph trigger; return empty otherwise; add string-context tests. |
| Server-initiated `workspace/applyEdit` deadlocks the request loop | Release the read loop with `jsonrpc2.Async(ctx)` before calling the client, matching the configuration-pull pattern. |
| Bad configuration crashes completion | Validate and skip invalid entries with one warning; bound the catalog size. |
| Custom names drift from the cart's actual font | Settings are workspace-wide; document cart-specific names/descriptions and defer per-cart definition files. |
| Editor behavior differs | Keep the catalog and insertion semantics in the server; use standard LSP completion, request, and command surfaces. |

## Implementation notes

- Implemented files:
  - `internal/glyphs/` for the built-in catalog and custom-definition
    validation, using `internal/p8scii` for encoding;
  - `internal/lsp/completion.go`, `completion_context.go`, and `glyphs.go`;
  - `internal/lsp/server.go` for capabilities and custom-request registration;
  - `internal/config/config.go` for settings and warning validation;
  - `editors/vscode/src/extension.ts`, `editors/vscode/src/client.ts`,
    `editors/vscode/src/config.ts`, `editors/vscode/package.json`, and client
    tests;
  - `editors/nvim/lua/pico_lsp/init.lua` and its tests;
  - `docs/configuration.md`, `docs/api.md`, `README.md`,
    `docs/troubleshooting.md`, and editor READMEs.
- Reuse `analysis.CanonicalName` for ASCII alias folding.
- Use `nav.file.Index.Range` for every completion or command edit; never compute
  columns from byte lengths.
- `glyphPrefix` scans backward over ASCII letters, digits, `_`, and `-` and
  stops at a string boundary or a non-trigger byte.
- Detect glyph completion before the early `inString`/`inComment` returns.
- `makeTextItem` currently sets `FilterText` to the label; glyph items need an
  explicit filter that includes the alias and byte.
- The `pico8/glyphs` request mirrors `pico8/cartInfo`: register the method in
  `protocolMethods` and dispatch it from `Server.Request`.
- The insertion command mirrors `CommandShowCartInfo`/`CommandRemoveUnused` in
  `Server.ExecuteCommand`; use `Server.Client().ApplyEdit` when available.
  Record the client's `workspace.applyEdit` capability during initialize and
  fall back to returning the edit when it is absent. Call
  `jsonrpc2.Async(ctx)` before that server-initiated request so the read loop
  can accept the client's response, matching `pullConfiguration`.
- Do not write to stdout outside the LSP protocol. Configuration warnings go to
  the server log.

## Deferred follow-ups

No scope decisions remain for M16. These items are intentionally deferred:

- A per-cart glyph definition file that travels with the cart.
- Additional per-glyph aliases and other catalog expansions.
- The full P8SCII control-code set and arbitrary `\0xxx` escape entries.
- A native Helix picker beyond standard completion.
- Generated static editor-snippet fallbacks if a client cannot use completion.
