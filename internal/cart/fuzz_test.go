package cart

import (
	"testing"

	"go.lsp.dev/uri"
)

func FuzzIncludeFlatten(f *testing.F) {
	f.Add([]byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\na=1\n"))
	f.Add([]byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"a.lua\"\n"))
	f.Add([]byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n--[[\n#include \"a.lua\"\n]]\n"))
	f.Add([]byte("a=1\nfunction f() return 1 end\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		root := uri.File("/fuzz/cart.p8")
		p := newMapProvider()
		p.files[root] = string(data)
		p.files[uri.File("/fuzz/a.lua")] = "x=1\n#include \"nested.lua\"\n"
		loader := NewLoader(p, "", nil)
		snap, err := loader.Load(root)
		if err != nil {
			return
		}
		if snap.Cart == nil || snap.Chunk == nil {
			t.Fatal("incomplete snapshot")
		}
	})
}
