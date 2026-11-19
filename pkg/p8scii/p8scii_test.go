package p8scii_test

import (
	"bytes"
	"testing"

	"github.com/asimmons91/pico-lsp/pkg/p8scii"
)

func TestGlyphTable(t *testing.T) {
	glyphs := p8scii.Glyphs()
	if len(glyphs) != 256 {
		t.Fatalf("Glyphs() returned %d entries, want 256", len(glyphs))
	}
	for i, glyph := range glyphs {
		b := byte(i)
		if glyph.Byte != b {
			t.Errorf("Glyphs()[%d].Byte = %#02x, want %#02x", i, glyph.Byte, b)
		}
		if want := p8scii.Decode([]byte{b}); glyph.Text != want {
			t.Errorf("byte %#02x: Text = %q, want %q", b, glyph.Text, want)
		}
		if want := p8scii.Lookup(b); glyph != want {
			t.Errorf("Glyphs()[%d] = %+v, want Lookup(%#02x) = %+v", i, glyph, b, want)
		}
		wantWidth := 8
		switch {
		case b < 0x10:
			wantWidth = 0
		case b < 0x80:
			wantWidth = 4
		}
		if glyph.Width != wantWidth {
			t.Errorf("byte %#02x: Width = %d, want %d", b, glyph.Width, wantWidth)
		}
		if wantPrintable := b >= 0x10; glyph.Printable != wantPrintable {
			t.Errorf("byte %#02x: Printable = %t, want %t", b, glyph.Printable, wantPrintable)
		}
	}
}

func TestEncodedGlyphsRoundTrip(t *testing.T) {
	for i := 0; i < 256; i++ {
		b := byte(i)
		glyph := p8scii.Lookup(b)
		if got := p8scii.Encode(glyph.Text); !bytes.Equal(got, []byte{b}) {
			t.Errorf("byte %#02x: Encode(%q) = %v, want [%#02x]", b, glyph.Text, got, b)
		}
	}
}

func TestGlyphsReturnsCopy(t *testing.T) {
	first := p8scii.Glyphs()
	original := first[0].Text
	first[0] = p8scii.Glyph{}
	if got := p8scii.Glyphs()[0].Text; got != original {
		t.Fatalf("Glyphs() shares state: got %q after mutation, want %q", got, original)
	}
	if got := p8scii.Lookup(0).Text; got != original {
		t.Fatalf("Lookup(0).Text = %q after Glyphs mutation, want %q", got, original)
	}
}

func TestGlyphFor(t *testing.T) {
	for i := 0; i < 256; i++ {
		glyph := p8scii.Lookup(byte(i))
		got, ok := p8scii.GlyphFor(glyph.Text)
		if !ok || got.Byte != glyph.Byte {
			t.Errorf("GlyphFor(%q) = (%#02x, %t), want (%#02x, true)", glyph.Text, got.Byte, ok, glyph.Byte)
		}

		got, ok = p8scii.GlyphFor(glyph.Text + "\ufe0f")
		if !ok || got.Byte != glyph.Byte {
			t.Errorf("GlyphFor(%q + U+FE0F) = (%#02x, %t), want (%#02x, true)", glyph.Text, got.Byte, ok, glyph.Byte)
		}
	}

	cases := []struct {
		text string
		byte byte
	}{
		{"😐", 0x8c},
		{"😐\ufe0f", 0x8c},
		{"♥", 0x87},
		{"⬅️", 0x8b},
	}
	for _, tc := range cases {
		got, ok := p8scii.GlyphFor(tc.text)
		if !ok || got.Byte != tc.byte {
			t.Errorf("GlyphFor(%q) = (%#02x, %t), want (%#02x, true)", tc.text, got.Byte, ok, tc.byte)
		}
	}

	for _, text := range []string{"", "not a glyph", "⬅", "😐\ufe0f\ufe0f", "\U0001d622"} {
		if got, ok := p8scii.GlyphFor(text); ok {
			t.Errorf("GlyphFor(%q) = (%#02x, true), want false", text, got.Byte)
		}
	}
}

func TestEncodeKeepsExistingBehavior(t *testing.T) {
	cases := []struct {
		text string
		want []byte
	}{
		{"♥", []byte{0x87}},
		{"⬅️➡️⬆️⬇️", []byte{0x8b, 0x91, 0x94, 0x83}},
		{"\U0001d622", []byte{'A'}},
		{"é", []byte{0xc3, 0xa9}},
		{"print(1)", []byte("print(1)")},
	}
	for _, tc := range cases {
		if got := p8scii.Encode(tc.text); !bytes.Equal(got, tc.want) {
			t.Errorf("Encode(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}
