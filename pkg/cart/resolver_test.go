package cart_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"go.lsp.dev/uri"

	internalcart "github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/pkg/cart"
)

type mapSource struct {
	files map[string]string
	dirs  map[string][]string
}

func newMapSource() *mapSource {
	return &mapSource{files: make(map[string]string), dirs: make(map[string][]string)}
}

func (s *mapSource) add(path, text string) {
	s.files[path] = text
	dir := filepath.Dir(path)
	s.dirs[dir] = append(s.dirs[dir], path)
}

func (s *mapSource) ReadFile(path string) ([]byte, error) {
	text, ok := s.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return []byte(text), nil
}

func (s *mapSource) ReadDir(dir string) ([]string, error) {
	entries, ok := s.dirs[dir]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]string(nil), entries...), nil
}

func cartText(code string) string {
	return "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n" + code
}

func TestOwnerReturnsCartPath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "game.p8")
	resolver := cart.NewResolver(newMapSource())
	got, err := resolver.Owner(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("Owner(%q) = %q", path, got)
	}
}

func TestOwnerNearestCart(t *testing.T) {
	root := t.TempDir()
	game := filepath.Join(root, "game.p8")
	inner := filepath.Join(root, "lib", "inner.p8")
	target := filepath.Join(root, "lib", "util.lua")
	source := newMapSource()
	source.add(game, cartText("#include \"lib/util.lua\"\n"))
	source.add(inner, cartText("#include \"util.lua\"\n"))
	source.add(target, "x=1\n")

	resolver := cart.NewResolver(source)
	got, err := resolver.Owner(target)
	if err != nil {
		t.Fatal(err)
	}
	if got != inner {
		t.Fatalf("Owner = %q, want nearest %q", got, inner)
	}
}

func TestOwnerLexicalTieBreak(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.p8")
	b := filepath.Join(root, "b.p8")
	target := filepath.Join(root, "lib.lua")
	source := newMapSource()
	source.add(b, cartText("#include \"lib.lua\"\n"))
	source.add(a, cartText("#include \"lib.lua\"\n"))
	source.add(target, "x=1\n")

	resolver := cart.NewResolver(source)
	got, err := resolver.Owner(target)
	if err != nil {
		t.Fatal(err)
	}
	if got != a {
		t.Fatalf("Owner = %q, want lexical first %q", got, a)
	}
}

func TestOwnerStandaloneLua(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "lib.lua")
	source := newMapSource()
	source.add(filepath.Join(root, "game.p8"), cartText("x=1\n"))
	source.add(target, "x=1\n")

	resolver := cart.NewResolver(source)
	got, err := resolver.Owner(target)
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Fatalf("Owner = %q, want %q", got, target)
	}
}

func TestOwnerIgnoresCommentedAndStringIncludes(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "lib.lua")
	source := newMapSource()
	source.add(filepath.Join(root, "game.p8"), cartText(
		"-- #include \"lib.lua\"\n"+
			"s = \"#include \\\"lib.lua\\\"\"\n"+
			"long = [[\n#include \"lib.lua\"\n]]\n"+
			"-->8 tab #include \"lib.lua\"\n",
	))
	source.add(target, "x=1\n")

	resolver := cart.NewResolver(source)
	got, err := resolver.Owner(target)
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Fatalf("Owner = %q, want standalone %q", got, target)
	}
}

func TestOwnerIgnoresShortStringContinuation(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "lib.lua")
	source := newMapSource()
	source.add(filepath.Join(root, "game.p8"), cartText(
		"s='line1\\\n#include \"lib.lua\"\\\nline3'\n",
	))
	source.add(target, "x=1\n")

	resolver := cart.NewResolver(source)
	got, err := resolver.Owner(target)
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Fatalf("Owner = %q, want standalone %q", got, target)
	}
}

func TestOwnerIncludeSelectors(t *testing.T) {
	root := t.TempDir()
	game := filepath.Join(root, "game.p8")
	source := newMapSource()
	source.add(game, cartText(
		"#include \"other.p8:1\"\n"+
			"#include \"script.lua:2\"\n",
	))
	source.add(filepath.Join(root, "other.p8"), cartText("x=1\n"))
	source.add(filepath.Join(root, "script.lua:2"), "x=1\n")

	resolver := cart.NewResolver(source)
	plain, err := resolver.Owner(filepath.Join(root, "script.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if plain != filepath.Join(root, "script.lua") {
		t.Fatalf("script.lua owner = %q, want standalone", plain)
	}
	if _, err := resolver.Owner(filepath.Join(root, "script.lua:2")); !errors.Is(err, cart.ErrInvalidPath) {
		t.Fatalf("script.lua:2 err = %v, want ErrInvalidPath", err)
	}
	other, err := resolver.Owner(filepath.Join(root, "other.p8"))
	if err != nil {
		t.Fatal(err)
	}
	if other != filepath.Join(root, "other.p8") {
		t.Fatalf("other.p8 owner = %q, want itself", other)
	}
}

func TestOwnerQuotedPathWithSpaces(t *testing.T) {
	root := t.TempDir()
	game := filepath.Join(root, "game.p8")
	target := filepath.Join(root, "lib", "with space.lua")
	source := newMapSource()
	source.add(game, cartText("#include \"lib/with space.lua\"\n"))
	source.add(target, "x=1\n")

	resolver := cart.NewResolver(source)
	got, err := resolver.Owner(target)
	if err != nil {
		t.Fatal(err)
	}
	if got != game {
		t.Fatalf("Owner = %q, want %q", got, game)
	}
}

func TestOwnerCacheInvalidation(t *testing.T) {
	root := t.TempDir()
	game := filepath.Join(root, "game.p8")
	target := filepath.Join(root, "lib.lua")
	source := newMapSource()
	source.add(game, cartText("x=1\n"))
	source.add(target, "x=1\n")

	resolver := cart.NewResolver(source)
	got, err := resolver.Owner(target)
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Fatalf("Owner = %q, want standalone", got)
	}

	source.files[game] = cartText("#include \"lib.lua\"\n")
	if got, _ = resolver.Owner(target); got != target {
		t.Fatalf("cached Owner = %q, want %q", got, target)
	}
	resolver.Invalidate()
	if got, err = resolver.Owner(target); err != nil {
		t.Fatal(err)
	}
	if got != game {
		t.Fatalf("Owner after Invalidate = %q, want %q", got, game)
	}
}

func TestOwnerSkipsUnreadableCandidates(t *testing.T) {
	root := t.TempDir()
	game := filepath.Join(root, "game.p8")
	target := filepath.Join(root, "lib.lua")
	source := newMapSource()
	source.add(game, cartText("#include \"lib.lua\"\n"))
	source.add(target, "x=1\n")
	source.dirs[root] = append(source.dirs[root], filepath.Join(root, "broken.p8"))

	resolver := cart.NewResolver(source)
	got, err := resolver.Owner(target)
	if err != nil {
		t.Fatal(err)
	}
	if got != game {
		t.Fatalf("Owner = %q, want %q", got, game)
	}
}

func TestOwnerInvalidPath(t *testing.T) {
	root := t.TempDir()
	resolver := cart.NewResolver(newMapSource())
	for _, path := range []string{"", "relative.lua", filepath.Join(root, "cart.txt"), filepath.Join(root, "cart.p8.png")} {
		if _, err := resolver.Owner(path); !errors.Is(err, cart.ErrInvalidPath) {
			t.Errorf("Owner(%q) err = %v, want ErrInvalidPath", path, err)
		}
	}
}

func TestOwnerConcurrent(t *testing.T) {
	root := t.TempDir()
	game := filepath.Join(root, "game.p8")
	target := filepath.Join(root, "lib.lua")
	source := newMapSource()
	source.add(game, cartText("#include \"lib.lua\"\n"))
	source.add(target, "x=1\n")

	resolver := cart.NewResolver(source)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if _, err := resolver.Owner(target); err != nil {
					t.Error(err)
					return
				}
				if j%5 == 0 {
					resolver.Invalidate()
				}
			}
		}()
	}
	wg.Wait()
}

func TestOwnerParityWithInternalResolver(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	game := filepath.Join(root, "game.p8")
	inner := filepath.Join(root, "lib", "inner.p8")
	target := filepath.Join(root, "lib", "util.lua")
	standalone := filepath.Join(root, "standalone.lua")
	mustWrite(game, cartText("#include \"lib/util.lua\"\n"))
	mustWrite(inner, cartText("#include \"util.lua\"\n"))
	mustWrite(target, "x=1\n")
	mustWrite(standalone, "x=1\n")

	public := cart.NewResolver(nil)
	internal := internalcart.NewResolver(internalcart.NewLoader(nil, "", nil))
	for _, path := range []string{game, inner, target, standalone} {
		want := filepath.Clean(internal.Resolve(uri.File(path)).FsPath())
		got, err := public.Owner(path)
		if err != nil {
			t.Fatalf("Owner(%q): %v", path, err)
		}
		if !samePath(got, want) {
			t.Errorf("Owner(%q) = %q, internal resolver = %q", path, got, want)
		}
	}
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return a == b
}

func TestDiskSource(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "game.p8")
	if err := os.WriteFile(file, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := cart.DiskSource{}
	data, err := source.ReadFile(file)
	if err != nil || string(data) != "data" {
		t.Fatalf("ReadFile = %q, %v", data, err)
	}
	entries, err := source.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{file, filepath.Join(root, "sub")}
	if len(entries) != len(want) {
		t.Fatalf("ReadDir = %v, want %v", entries, want)
	}
	for i := range want {
		if entries[i] != want[i] {
			t.Fatalf("ReadDir = %v, want %v", entries, want)
		}
	}
}
