package publicapi

import (
	"bytes"
	"testing"

	"github.com/asimmons91/pico-lsp/pkg/p8scii"
)

func TestP8SCIIPublicAPI(t *testing.T) {
	if got := p8scii.Encode("♥"); !bytes.Equal(got, []byte{0x87}) {
		t.Fatalf("Encode(♥) = %v, want [0x87]", got)
	}
	if got := p8scii.Decode([]byte{0x87}); got != "♥" {
		t.Fatalf("Decode([0x87]) = %q, want ♥", got)
	}

	narrow := p8scii.Lookup(0x10)
	if narrow.Width != 4 || !narrow.Printable {
		t.Errorf("Lookup(0x10) = %+v, want width 4 and printable", narrow)
	}
	wide := p8scii.Lookup(0x8b)
	if wide.Width != 8 || !wide.Printable || wide.Text != "⬅️" {
		t.Errorf("Lookup(0x8b) = %+v, want width 8, printable, and text ⬅️", wide)
	}
	control := p8scii.Lookup(0x00)
	if control.Width != 0 || control.Printable {
		t.Errorf("Lookup(0x00) = %+v, want width 0 and not printable", control)
	}

	glyph, ok := p8scii.GlyphFor("😐\ufe0f")
	if !ok || glyph.Byte != 0x8c {
		t.Errorf("GlyphFor(😐️) = (%+v, %t), want byte 0x8c", glyph, ok)
	}
	if _, ok := p8scii.GlyphFor("not a glyph"); ok {
		t.Error("GlyphFor(not a glyph) returned true")
	}

	glyphs := p8scii.Glyphs()
	if len(glyphs) != 256 {
		t.Fatalf("Glyphs() returned %d entries, want 256", len(glyphs))
	}
	for i, glyph := range glyphs {
		if glyph.Byte != byte(i) {
			t.Fatalf("Glyphs()[%d].Byte = %#02x, want %#02x", i, glyph.Byte, i)
		}
	}

	glyphs[0].Text = "mutated"
	if got := p8scii.Glyphs()[0].Text; got == "mutated" {
		t.Fatal("Glyphs() returns shared state")
	}
}
