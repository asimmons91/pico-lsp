package p8scii

import "strings"

// Glyph describes one P8SCII byte.
type Glyph struct {
	// Byte is the P8SCII byte value, 0x00 through 0xff.
	Byte byte
	// Text is the canonical Unicode text for the glyph.
	Text string
	// Width is the display width in pixels at 1x: 0 for control bytes,
	// 4 for 0x10-0x7f, and 8 for 0x80-0xff.
	Width int
	// Printable reports whether the byte is a printable glyph. Control
	// bytes 0x00-0x0f are not printable.
	Printable bool
}

// glyphs is the canonical metadata table. It is initialized once from charset
// and never mutated.
var glyphs = func() [256]Glyph {
	var table [256]Glyph
	for i, text := range charset {
		b := byte(i)
		table[i] = Glyph{
			Byte:      b,
			Text:      text,
			Width:     widthFor(b),
			Printable: b >= 0x10,
		}
	}
	return table
}()

// widthFor returns the P8SCII display width in pixels at 1x for b.
func widthFor(b byte) int {
	switch {
	case b < 0x10:
		return 0
	case b < 0x80:
		return 4
	default:
		return 8
	}
}

// Glyphs returns a copy of all 256 glyphs in byte order. Callers may mutate
// the result without affecting later calls.
func Glyphs() []Glyph {
	out := make([]Glyph, len(glyphs))
	copy(out, glyphs[:])
	return out
}

// Lookup returns the glyph for b. It is total; every byte has an entry.
func Lookup(b byte) Glyph {
	return glyphs[b]
}

// GlyphFor resolves canonical glyph text to its glyph. It first tries the
// exact text, then retries once with one trailing U+FE0F variation selector
// removed. Unknown text returns false.
func GlyphFor(text string) (Glyph, bool) {
	if b, ok := glyphByte(text); ok {
		return glyphs[b], true
	}
	if base, ok := strings.CutSuffix(text, "\ufe0f"); ok {
		if b, ok := glyphByte(base); ok {
			return glyphs[b], true
		}
	}
	return Glyph{}, false
}

// glyphByte resolves text to a byte only when text is exactly the canonical
// spelling of that byte. Encode also accepts the Unicode mathematical italic
// letters; those aliases are deliberately not canonical glyph text.
func glyphByte(text string) (byte, bool) {
	encoded := Encode(text)
	if len(encoded) != 1 {
		return 0, false
	}
	b := encoded[0]
	if glyphs[b].Text != text {
		return 0, false
	}
	return b, true
}
