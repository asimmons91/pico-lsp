package p8scii_test

import (
	"fmt"

	"github.com/asimmons91/pico-lsp/pkg/p8scii"
)

func ExampleEncode() {
	code := p8scii.Encode("♥")
	fmt.Printf("% x\n", code)
	// Output:
	// 87
}

func ExampleDecode() {
	fmt.Println(p8scii.Decode([]byte{0x87}))
	// Output:
	// ♥
}

func ExampleGlyphs() {
	glyphs := p8scii.Glyphs()
	wide := 0
	for _, glyph := range glyphs {
		if glyph.Width == 8 {
			wide++
		}
	}
	fmt.Printf("%d glyphs, %d wide\n", len(glyphs), wide)
	// Output:
	// 256 glyphs, 128 wide
}

func ExampleLookup() {
	glyph := p8scii.Lookup(0x8b)
	fmt.Printf("%#02x %s width=%d printable=%t\n", glyph.Byte, glyph.Text, glyph.Width, glyph.Printable)
	// Output:
	// 0x8b ⬅️ width=8 printable=true
}

func ExampleGlyphFor() {
	for _, text := range []string{"😐", "😐\ufe0f"} {
		glyph, ok := p8scii.GlyphFor(text)
		fmt.Printf("%#02x %t\n", glyph.Byte, ok)
	}
	// Output:
	// 0x8c true
	// 0x8c true
}
