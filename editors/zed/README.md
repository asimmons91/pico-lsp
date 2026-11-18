# Zed

A dedicated Zed extension is still on the post-v1 backlog. The PICO-8
tree-sitter grammar at [../tree-sitter-pico8](../tree-sitter-pico8) is ready for
a thin extension:

- language name `pico8`, scope `source.pico8`, file extension `p8`;
- grammar source `editors/tree-sitter-pico8` with committed ABI 14 sources;
- highlight query `queries/highlights.scm`, plus the indent and text-object
  queries when the target Zed version supports them;
- `pico-lsp` as the language server command, without changing the server's
  `pico8` language ID.

Until an extension is published, use the server through another editor or a
manual Zed development extension. Do not treat this note as a packaged Zed
install path.
