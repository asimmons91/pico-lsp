package p8scii

import "testing"

func TestGlyphTable(t *testing.T) {
	glyphs := Glyphs()
	if len(glyphs) != 256 {
		t.Fatalf("Glyphs() returned %d entries, want 256", len(glyphs))
	}
	for i, glyph := range glyphs {
		b := byte(i)
		if glyph.Byte != b {
			t.Errorf("Glyphs()[%d].Byte = %#02x, want %#02x", i, glyph.Byte, b)
		}
		if want := Decode([]byte{b}); glyph.Text != want {
			t.Errorf("byte %#02x: Text = %q, want %q", b, glyph.Text, want)
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

func TestGlyphsReturnsCopy(t *testing.T) {
	first := Glyphs()
	if len(first) == 0 {
		t.Fatal("Glyphs() returned no entries")
	}
	original := first[0].Text
	first[0] = Glyph{}
	if got := Glyphs()[0].Text; got != original {
		t.Fatalf("Glyphs() shares state: got %q after mutation, want %q", got, original)
	}
	if got := Lookup(0).Text; got != original {
		t.Fatalf("Lookup(0).Text = %q after Glyphs mutation, want %q", got, original)
	}
}

func TestGlyphFor(t *testing.T) {
	for i := 0; i < 256; i++ {
		glyph := Lookup(byte(i))
		got, ok := GlyphFor(glyph.Text)
		if !ok {
			t.Errorf("GlyphFor(%q) for byte %#02x returned false", glyph.Text, i)
			continue
		}
		if got.Byte != glyph.Byte {
			t.Errorf("GlyphFor(%q).Byte = %#02x, want %#02x", glyph.Text, got.Byte, glyph.Byte)
		}

		got, ok = GlyphFor(glyph.Text + "\ufe0f")
		if !ok {
			t.Errorf("GlyphFor(%q) with trailing U+FE0F returned false", glyph.Text)
			continue
		}
		if got.Byte != glyph.Byte {
			t.Errorf("GlyphFor(%q) with trailing U+FE0F = %#02x, want %#02x", glyph.Text, got.Byte, glyph.Byte)
		}
	}

	cases := []struct {
		text string
		byte byte
	}{
		{"😐", 0x8c},
		{"😐\ufe0f", 0x8c},
	}
	for _, tc := range cases {
		got, ok := GlyphFor(tc.text)
		if !ok || got.Byte != tc.byte {
			t.Errorf("GlyphFor(%q) = (%#02x, %t), want (%#02x, true)", tc.text, got.Byte, ok, tc.byte)
		}
	}

	for _, text := range []string{"", "not a glyph", "⬅", "😐\ufe0f\ufe0f", "\U0001d622"} {
		if got, ok := GlyphFor(text); ok {
			t.Errorf("GlyphFor(%q) = (%#02x, true), want false", text, got.Byte)
		}
	}
}
