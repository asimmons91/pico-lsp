package glyphs

import (
	"strings"
	"testing"

	"github.com/asimmons91/pico-lsp/internal/p8scii"
)

func TestBuiltinsAreValid(t *testing.T) {
	entries := Builtins()
	if len(entries) != len(builtinSpecs)+len(controlSpecs) {
		t.Fatalf("Builtins() returned %d entries, want %d", len(entries), len(builtinSpecs)+len(controlSpecs))
	}
	names := make(map[string]struct{}, len(entries))
	triggers := make(map[string]struct{})
	for _, entry := range entries {
		if entry.Name == "" || entry.Category == "" || entry.Description == "" {
			t.Errorf("incomplete built-in entry: %+v", entry)
		}
		if _, exists := names[entry.Name]; exists {
			t.Errorf("duplicate built-in name %q", entry.Name)
		}
		names[entry.Name] = struct{}{}
		if entry.Context == ContextString {
			if !strings.HasPrefix(entry.Text, `\`) {
				t.Errorf("control entry %q text = %q, want an escape", entry.Name, entry.Text)
			}
		} else {
			encoded := p8scii.Encode(entry.Text)
			if len(encoded) != 1 || encoded[0] != entry.Byte {
				t.Errorf("entry %q text %q encodes to %v, want byte %#02x", entry.Name, entry.Text, encoded, entry.Byte)
			}
		}
		for _, trigger := range entry.AllTriggers() {
			if !strings.HasPrefix(trigger, "p8_") && !strings.HasPrefix(trigger, "p8-") {
				t.Errorf("entry %q trigger %q is outside the glyph namespace", entry.Name, trigger)
			}
			if _, exists := triggers[trigger]; exists {
				t.Errorf("duplicate built-in trigger %q", trigger)
			}
			triggers[trigger] = struct{}{}
		}
	}
}

func TestLookupBuiltins(t *testing.T) {
	entries := Builtins()
	for _, name := range []string{"left", "p8_left", "p8-left", "LEFT"} {
		entry, ok := FindByName(entries, name)
		if !ok || entry.Name != "left" {
			t.Fatalf("FindByName(%q) = %+v, %v", name, entry, ok)
		}
	}
	entry, ok := FindByByte(entries, 0x97)
	if !ok || entry.Name != "x" {
		t.Fatalf("FindByByte(0x97) = %+v, %v", entry, ok)
	}
}

func TestBuildCustomEntries(t *testing.T) {
	byte128 := 128
	entries, warnings := Build([]Definition{
		{Name: "logo", Byte: &byte128, Description: "custom font slot 0x80"},
		{Name: "my_heart", Glyph: "♥"},
		{Name: "my_font_custom", Text: `\14`, Context: ContextString},
	})
	if len(warnings) != 0 {
		t.Fatalf("Build warnings = %v", warnings)
	}
	for _, name := range []string{"logo", "my_heart", "my_font_custom"} {
		if _, ok := FindByName(entries, name); !ok {
			t.Fatalf("missing custom entry %q", name)
		}
	}
	logo, _ := FindByName(entries, "logo")
	if logo.Text != "█" || logo.Byte != 0x80 || logo.Context != ContextBoth || !logo.Custom {
		t.Fatalf("logo = %+v", logo)
	}
	heart, _ := FindByName(entries, "my_heart")
	if heart.Text != "♥" || heart.Byte != 0x87 || heart.Category != "custom" {
		t.Fatalf("my_heart = %+v", heart)
	}
	font, _ := FindByName(entries, "my_font_custom")
	if font.Text != `\14` || font.Byte != 14 || font.Context != ContextString || font.Category != "control" {
		t.Fatalf("my_font_custom = %+v", font)
	}
}

func TestBuildCustomOverrideAndInvalidEntries(t *testing.T) {
	byte128 := 128
	entries, warnings := Build([]Definition{
		{Name: "left", Byte: &byte128},
		{Name: "Bad Name", Byte: &byte128},
		{Name: "both", Byte: &byte128, Glyph: "♥"},
		{Name: "control", Byte: intPtr(1)},
		{Name: "unknown", Glyph: "é"},
		{Name: "text_without_context", Text: `\14`},
		{Name: "bad_alias", Glyph: "♥", Aliases: []string{"left"}},
	})
	if len(warnings) < 5 {
		t.Fatalf("Build warnings = %v, want invalid-entry warnings", warnings)
	}
	left, ok := FindByName(entries, "left")
	if !ok || !left.Custom || left.Byte != 0x80 {
		t.Fatalf("custom left = %+v, %v", left, ok)
	}
	for _, name := range []string{"Bad Name", "both", "control", "unknown", "text_without_context", "bad_alias"} {
		if _, ok := FindByName(entries, name); ok {
			t.Errorf("invalid entry %q was accepted", name)
		}
	}
}

func TestCustomAliasOverridesBuiltinRegardlessOfOrder(t *testing.T) {
	entries, warnings := Build([]Definition{
		{Name: "my_left", Glyph: "♥", Aliases: []string{"p8-right"}},
	})
	if len(warnings) == 0 {
		t.Fatal("Build did not warn about the built-in alias override")
	}
	entry, ok := FindByName(entries, "p8-right")
	if !ok || entry.Name != "my_left" {
		t.Fatalf("p8-right = %+v, %v; want custom my_left", entry, ok)
	}
}

func TestBuildLaterCustomEntryWins(t *testing.T) {
	first, second := 128, 129
	entries, _ := Build([]Definition{
		{Name: "slot", Byte: &first},
		{Name: "slot", Byte: &second},
	})
	entry, ok := FindByName(entries, "slot")
	if !ok || entry.Byte != 0x81 {
		t.Fatalf("slot = %+v, %v; want later byte 0x81", entry, ok)
	}
}

func TestNormalizeDefinitionRejectsAmbiguousFields(t *testing.T) {
	byte128 := 128
	tests := []Definition{
		{Name: "missing"},
		{Name: "both", Byte: &byte128, Glyph: "♥"},
		{Name: "", Glyph: "♥"},
		{Name: "bad name", Glyph: "♥"},
		{Name: "control_byte", Byte: intPtr(14)},
		{Name: "control_glyph", Glyph: "ᵉ"},
		{Name: "unknown_glyph", Glyph: "é"},
		{Name: "text_bad_context", Text: `\14`, Context: ContextBoth},
		{Name: "text_not_escape", Text: `14`, Context: ContextString},
		{Name: "bad_alias", Glyph: "♥", Aliases: []string{"heart"}},
	}
	for _, definition := range tests {
		if _, err := NormalizeDefinition(definition); err == nil {
			t.Errorf("NormalizeDefinition(%+v) succeeded, want error", definition)
		}
	}
}

func intPtr(value int) *int { return &value }
