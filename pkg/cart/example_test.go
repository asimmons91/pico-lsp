package cart_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/asimmons91/pico-lsp/pkg/cart"
)

func ExampleParse() {
	doc, err := cart.Parse("game.p8", []byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\na = 1\n__gfx__\n"))
	if err != nil {
		panic(err)
	}
	fmt.Println(doc.HasCode())
	fmt.Println(doc.CodeStartLine())
	fmt.Println(doc.Code())
	// Output:
	// true
	// 3
	// a = 1
}

func ExampleDocument_Tabs() {
	doc, err := cart.Parse("game.p8", []byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\na = 1\n-->8 second\nb = 2\n"))
	if err != nil {
		panic(err)
	}
	for _, tab := range doc.Tabs() {
		fmt.Printf("%d %s %d %d\n", tab.Index, tab.Name, tab.StartLine, tab.EndLine)
	}
	// Output:
	// 0 Tab 1 3 4
	// 1 second 5 6
}

func ExampleDocument_Includes() {
	doc, err := cart.Parse("game.p8", []byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib/a.lua\"\n#include \"other.p8:1\"\n"))
	if err != nil {
		panic(err)
	}
	for _, include := range doc.Includes() {
		fmt.Printf("%s tab=%d line=%d\n", include.Path, include.Tab, include.Line)
	}
	// Output:
	// lib/a.lua tab=-1 line=3
	// other.p8 tab=1 line=4
}

func ExampleDocument_ReplaceCode() {
	doc, err := cart.Parse("game.p8", []byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nold = 1\n__gfx__\n"))
	if err != nil {
		panic(err)
	}
	out, err := doc.ReplaceCode("new = 2\n")
	if err != nil {
		panic(err)
	}
	fmt.Print(string(out))
	// Output:
	// pico-8 cartridge // pico-8.com
	// version 42
	// __lua__
	// new = 2
	// __gfx__
}

func ExampleResolver_Owner() {
	dir, err := os.MkdirTemp("", "pico-cart-example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	game := filepath.Join(dir, "game.p8")
	target := filepath.Join(dir, "lib.lua")
	if err := os.WriteFile(game, []byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib.lua\"\n"), 0o644); err != nil {
		panic(err)
	}
	if err := os.WriteFile(target, []byte("x = 1\n"), 0o644); err != nil {
		panic(err)
	}

	owner, err := cart.NewResolver(nil).Owner(target)
	if err != nil {
		panic(err)
	}
	fmt.Println(filepath.Base(owner))
	// Output:
	// game.p8
}
