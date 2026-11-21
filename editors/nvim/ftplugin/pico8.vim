" PICO-8 carts use Lua-style comments.
if exists("b:did_ftplugin")
  finish
endif
let b:did_ftplugin = 1

setlocal commentstring=--\ %s
setlocal comments=:---,:--,://

let b:undo_ftplugin = "setlocal commentstring< comments<"

" Optional PICO-8 editor-style glyph shortcuts. Set g:pico8_glyph_maps = 1
" before opening a cart. These replace uppercase U/D/L/R/O/X in insert mode,
" matching the PICO-8 editor's Shift-L/R/U/D/O/X shortcuts. Use CTRL-V before
" a letter to insert it literally.
if get(g:, "pico8_glyph_maps", 0)
  inoremap <buffer> U ⬆️
  inoremap <buffer> D ⬇️
  inoremap <buffer> L ⬅️
  inoremap <buffer> R ➡️
  inoremap <buffer> O 🅾️
  inoremap <buffer> X ❎
  let b:undo_ftplugin .= "|silent! iunmap <buffer> U|silent! iunmap <buffer> D|silent! iunmap <buffer> L|silent! iunmap <buffer> R|silent! iunmap <buffer> O|silent! iunmap <buffer> X"
endif
