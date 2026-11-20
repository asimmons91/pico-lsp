package glyphs_test

import (
	"reflect"
	"testing"

	internalglyphs "github.com/asimmons91/pico-lsp/internal/glyphs"
	"github.com/asimmons91/pico-lsp/pkg/glyphs"
)

func TestAliasesMatchInternalCatalog(t *testing.T) {
	if got, want := glyphs.Builtins(), internalglyphs.Builtins(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Builtins() differs from internal catalog")
	}
	if glyphs.ContextBoth != internalglyphs.ContextBoth || glyphs.ContextString != internalglyphs.ContextString {
		t.Fatalf("context constants differ from internal catalog")
	}
}

func TestBuiltinsReturnsCopy(t *testing.T) {
	first := glyphs.Builtins()
	if len(first) == 0 {
		t.Fatal("Builtins() returned no entries")
	}
	first[0].Name = "changed"
	if len(first[0].Aliases) > 0 {
		first[0].Aliases[0] = "changed"
	}

	second := glyphs.Builtins()
	if second[0].Name == "changed" {
		t.Fatal("mutating Builtins() changed a later call")
	}
	for _, alias := range second[0].Aliases {
		if alias == "changed" {
			t.Fatal("mutating a Builtins() alias changed a later call")
		}
	}
}

func TestLookup(t *testing.T) {
	entries := glyphs.Builtins()
	for _, name := range []string{"left", "p8_left", "p8-left", "LEFT"} {
		entry, ok := glyphs.FindByName(entries, name)
		if !ok || entry.Name != "left" {
			t.Fatalf("FindByName(%q) = %+v, %v", name, entry, ok)
		}
	}
	entry, ok := glyphs.FindByName(entries, "missing")
	if ok || entry.Name != "" {
		t.Fatalf("FindByName(missing) = %+v, %v", entry, ok)
	}

	entry, ok = glyphs.FindByByte(entries, 0x97)
	if !ok || entry.Name != "x" {
		t.Fatalf("FindByByte(0x97) = %+v, %v", entry, ok)
	}
	if _, ok := glyphs.FindByByte(entries, 0x00); ok {
		t.Fatal("FindByByte(0x00) found an entry")
	}
}

func TestBuildCustomEntries(t *testing.T) {
	byteValue := 0x80
	entries, warnings := glyphs.Build([]glyphs.Definition{
		{Name: "my_heart", Glyph: "♥", Description: "custom heart"},
		{Name: "left", Byte: &byteValue, Description: "override"},
	})
	if len(warnings) != 1 {
		t.Fatalf("Build warnings = %v, want one built-in override", warnings)
	}

	heart, ok := glyphs.FindByName(entries, "my_heart")
	if !ok || heart.Byte != 0x87 || heart.Text != "♥" || !heart.Custom {
		t.Fatalf("custom heart = %+v, %v", heart, ok)
	}
	left, ok := glyphs.FindByName(entries, "p8_left")
	if !ok || left.Name != "left" || left.Byte != 0x80 || !left.Custom {
		t.Fatalf("custom left = %+v, %v", left, ok)
	}
	if entry, ok := glyphs.FindByByte(entries, 0x80); !ok || entry.Name != "left" || entry.Description != "override" {
		t.Fatalf("FindByByte(0x80) = %+v, %v", entry, ok)
	}
}

func TestNormalizeDefinition(t *testing.T) {
	entry, err := glyphs.NormalizeDefinition(glyphs.Definition{Name: "Logo", Glyph: "♥"})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Name != "logo" || entry.Byte != 0x87 || entry.Context != glyphs.ContextBoth || !entry.Custom {
		t.Fatalf("NormalizeDefinition = %+v", entry)
	}

	if _, err := glyphs.NormalizeDefinition(glyphs.Definition{Name: "bad", Context: glyphs.ContextString}); err == nil {
		t.Fatal("NormalizeDefinition accepted a definition without a value")
	}
}

func FuzzNormalizeDefinition(f *testing.F) {
	f.Add("logo", "", "", "both", 128, "p8-logo")
	f.Add("my_heart", "♥", "", "both", -1, "")
	f.Add("my_font", "", `\14`, "string", -1, "p8-font")
	f.Add("", "", "", "", -1, "")
	f.Add("left", "♥", `\14`, "other", 256, "not-an-alias")
	f.Fuzz(func(t *testing.T, name, glyph, text, context string, byteValue int, alias string) {
		var bytePtr *int
		if byteValue >= 0 {
			value := byteValue
			bytePtr = &value
		}
		var aliases []string
		if alias != "" {
			aliases = []string{alias}
		}
		definition := glyphs.Definition{
			Name:    name,
			Glyph:   glyph,
			Text:    text,
			Context: glyphs.Context(context),
			Byte:    bytePtr,
			Aliases: aliases,
		}
		entry, err := glyphs.NormalizeDefinition(definition)
		if err != nil {
			return
		}

		entries, _ := glyphs.Build([]glyphs.Definition{definition})
		if len(entries) == 0 {
			t.Fatal("Build returned an empty catalog")
		}
		found, ok := glyphs.FindByName(entries, entry.Name)
		if !ok || found.Name != entry.Name {
			t.Fatalf("FindByName(%q) = %+v, %v", entry.Name, found, ok)
		}
		if found, ok := glyphs.FindByByte(entries, entry.Byte); !ok || found.Byte != entry.Byte {
			t.Fatalf("FindByByte(%#02x) = %+v, %v", entry.Byte, found, ok)
		}
	})
}
