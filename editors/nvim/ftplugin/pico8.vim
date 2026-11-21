" PICO-8 carts use Lua-style comments.
if exists("b:did_ftplugin")
  finish
endif
let b:did_ftplugin = 1

setlocal commentstring=--\ %s
setlocal comments=:---,:--,://

let b:undo_ftplugin = "setlocal commentstring< comments<"

" Optional PICO-8 editor-style glyph shortcuts. Set g:pico8_glyph_maps = 1
" before opening a cart. Shift-A..Z insert the 26 typeable P8SCII symbols
" (0x80-0x99) in alphabetical order, matching the PICO-8 editor. The mappings
" fetch canonical text from pico8/glyphs, so they need an attached pico_lsp
" client. Use CTRL-V u<hex> to insert a literal uppercase letter, for example
" CTRL-V u0055 for U.
if get(g:, "pico8_glyph_maps", 0)
  let b:pico8_glyph_bufnr = str2nr(expand("<abuf>"))
  if b:pico8_glyph_bufnr == 0
    let b:pico8_glyph_bufnr = bufnr("%")
  endif
  execute "silent! lua require('pico_lsp').install_glyph_maps(" . b:pico8_glyph_bufnr . ")"
  for s:code in range(char2nr("A"), char2nr("Z"))
    let b:undo_ftplugin .= "|silent! iunmap <buffer> " . nr2char(s:code)
  endfor
  let b:undo_ftplugin .= "|unlet! b:pico8_glyph_bufnr"
  unlet s:code
endif
