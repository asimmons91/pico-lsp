// Package p8scii maps between PICO-8's 256-byte P8SCII encoding and Unicode
// text and describes each glyph's display width.
//
// Bytes 0x00-0x0f are control bytes. They have canonical Unicode text for
// encoding and decoding, but they are not printable cells and consumers must
// not draw them. Bytes 0x10-0x7f are 4-pixel-wide glyphs at 1x, and bytes
// 0x80-0xff are 8-pixel-wide glyphs. Widths are pixels, not terminal cells or
// rune counts; callers that draw an editor grid convert them to cells.
//
// The package is a thin, stable facade over the server's canonical P8SCII
// table. It is additive from v1.6.0 and does not change the LSP server.
package p8scii

import internalp8scii "github.com/asimmons91/pico-lsp/internal/p8scii"

// Glyph describes one P8SCII byte.
type Glyph = internalp8scii.Glyph

// Encode converts Unicode text to the P8SCII bytes PICO-8 would store.
// Unknown characters are reinterpreted as their UTF-8 bytes, matching PICO-8
// and the server's behavior.
func Encode(text string) []byte {
	return internalp8scii.Encode(text)
}

// Decode converts P8SCII bytes to Unicode text.
func Decode(code []byte) string {
	return internalp8scii.Decode(code)
}

// Lookup returns the glyph for b. It is total; every byte has an entry.
func Lookup(b byte) Glyph {
	return internalp8scii.Lookup(b)
}

// GlyphFor resolves canonical glyph text to its glyph. It first tries the
// exact text, then retries once with one trailing U+FE0F variation selector
// removed. Unknown text returns false.
func GlyphFor(text string) (Glyph, bool) {
	return internalp8scii.GlyphFor(text)
}

// Glyphs returns a copy of all 256 glyphs in byte order. Callers may mutate the
// result without affecting later calls.
func Glyphs() []Glyph {
	return internalp8scii.Glyphs()
}
