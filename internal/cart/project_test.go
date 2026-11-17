package cart

import (
	"testing"

	"go.lsp.dev/uri"
)

func projectProvider(files map[string]string) *mapProvider {
	p := newMapProvider()
	for path, text := range files {
		p.files[uri.File(path)] = text
	}
	return p
}

func cartText(code string) string {
	return "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n" + code
}

func TestResolveNearestCart(t *testing.T) {
	p := projectProvider(map[string]string{
		"/proj/game.p8":        cartText("#include \"player.lua\"\n"),
		"/proj/sub/game2.p8":   cartText("#include \"player.lua\"\n"),
		"/proj/sub/player.lua": "",
	})
	r := NewResolver(NewLoader(p, "", nil))
	target := uri.File("/proj/sub/player.lua")
	if got := r.Resolve(target); got != uri.File("/proj/sub/game2.p8") {
		t.Fatalf("owner = %s, want sub/game2.p8", got)
	}
}

func TestResolveStandaloneLua(t *testing.T) {
	p := projectProvider(map[string]string{
		"/proj/sub/loose.lua": "",
	})
	r := NewResolver(NewLoader(p, "", nil))
	target := uri.File("/proj/sub/loose.lua")
	if got := r.Resolve(target); got != target {
		t.Fatalf("owner = %s, want standalone", got)
	}
}

func TestResolveP8OwnsItself(t *testing.T) {
	p := projectProvider(map[string]string{
		"/proj/game.p8": cartText("a=1\n"),
	})
	r := NewResolver(NewLoader(p, "", nil))
	target := uri.File("/proj/game.p8")
	if got := r.Resolve(target); got != target {
		t.Fatalf("owner = %s, want itself", got)
	}
}

func TestResolveIgnoresCommentedInclude(t *testing.T) {
	p := projectProvider(map[string]string{
		"/proj/game.p8":    cartText("--[[\n#include \"player.lua\"\n]]\n"),
		"/proj/player.lua": "",
	})
	r := NewResolver(NewLoader(p, "", nil))
	target := uri.File("/proj/player.lua")
	if got := r.Resolve(target); got != target {
		t.Fatalf("owner = %s, want standalone", got)
	}
}

func TestResolveMultipleCartsSorted(t *testing.T) {
	p := projectProvider(map[string]string{
		"/proj/b.p8":       cartText("#include \"player.lua\"\n"),
		"/proj/a.p8":       cartText("#include \"player.lua\"\n"),
		"/proj/player.lua": "",
	})
	r := NewResolver(NewLoader(p, "", nil))
	target := uri.File("/proj/player.lua")
	if got := r.Resolve(target); got != uri.File("/proj/a.p8") {
		t.Fatalf("owner = %s, want a.p8", got)
	}
}

func TestResolveCacheInvalidation(t *testing.T) {
	p := projectProvider(map[string]string{
		"/proj/game.p8":    cartText("a=1\n"),
		"/proj/player.lua": "",
	})
	loader := NewLoader(p, "", nil)
	r := NewResolver(loader)
	target := uri.File("/proj/player.lua")
	if got := r.Resolve(target); got != target {
		t.Fatalf("owner = %s, want standalone", got)
	}

	p.files[uri.File("/proj/game.p8")] = cartText("#include \"player.lua\"\n")
	r.Invalidate()
	if got := r.Resolve(target); got != uri.File("/proj/game.p8") {
		t.Fatalf("owner after invalidation = %s, want game.p8", got)
	}
}
