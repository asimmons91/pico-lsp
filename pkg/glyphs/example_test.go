package glyphs_test

import (
	"fmt"

	"github.com/asimmons91/pico-lsp/pkg/glyphs"
)

func ExampleBuiltins() {
	entries := glyphs.Builtins()
	fmt.Println(len(entries))
	// Output:
	// 250
}

func ExampleFindByName() {
	entries := glyphs.Builtins()
	entry, ok := glyphs.FindByName(entries, "p8_heart")
	fmt.Printf("%#02x %s %v\n", entry.Byte, entry.Text, ok)
	// Output:
	// 0x87 ♥ true
}

func ExampleBuild() {
	byteValue := 0x80
	entries, warnings := glyphs.Build([]glyphs.Definition{{
		Name: "logo",
		Byte: &byteValue,
	}})
	entry, _ := glyphs.FindByName(entries, "p8_logo")
	fmt.Printf("%s %#02x custom=%t warnings=%d\n", entry.Name, entry.Byte, entry.Custom, len(warnings))
	// Output:
	// logo 0x80 custom=true warnings=0
}
