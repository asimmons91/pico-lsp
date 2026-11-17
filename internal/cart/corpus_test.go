package cart

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/source"
)

type tokenFixture struct {
	Path   string `json:"path"`
	Tokens int    `json:"tokens"`
	Chars  int    `json:"chars"`
}

func corpusFiles(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", "carts")
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".p8" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}
	return files
}

// TestCorpusParses requires every valid corpus cart to parse without a panic
// and without syntax or include diagnostics.
func TestCorpusParses(t *testing.T) {
	files := corpusFiles(t)
	if len(files) < 50 {
		t.Fatalf("corpus has %d carts, want at least 50", len(files))
	}

	loader := NewLoader(DiskProvider{}, "", nil)
	for _, path := range files {
		abs, err := filepath.Abs(path)
		if err != nil {
			t.Fatalf("abs: %v", err)
		}
		start := time.Now()
		snap, err := loader.Load(uri.File(abs))
		if err != nil {
			t.Errorf("%s: load: %v", path, err)
			continue
		}
		if len(snap.Cart.Diagnostics) != 0 {
			t.Errorf("%s: diagnostics: %+v", path, snap.Cart.Diagnostics)
		}
		if len(snap.Errors) != 0 {
			for i, e := range snap.Errors {
				if i >= 3 {
					break
				}
				file := snap.Cart.File(e.Span.FileID)
				name := ""
				if file != nil {
					name = file.URI.String()
				}
				t.Errorf("%s: syntax error %s:%d-%d %s", path, name, e.Span.Start, e.Span.End, e.Message)
			}
		}
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Errorf("%s: parse took %v", path, elapsed)
		}
	}
}

func TestTokenFixtures(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "tokens", "counts.json"))
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	var fixtures []tokenFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("decode fixtures: %v", err)
	}
	if len(fixtures) < 20 {
		t.Fatalf("token fixtures = %d, want at least 20", len(fixtures))
	}

	loader := NewLoader(DiskProvider{}, "", nil)
	for _, fixture := range fixtures {
		path := filepath.Join("..", "..", filepath.FromSlash(fixture.Path))
		abs, err := filepath.Abs(path)
		if err != nil {
			t.Fatalf("%s: abs: %v", fixture.Path, err)
		}
		snap, err := loader.Load(uri.File(abs))
		if err != nil {
			t.Fatalf("%s: load: %v", fixture.Path, err)
		}
		if snap.Cart.Tokens != fixture.Tokens {
			t.Errorf("%s: tokens = %d, want %d", fixture.Path, snap.Cart.Tokens, fixture.Tokens)
		}
		if snap.Cart.Chars != fixture.Chars {
			t.Errorf("%s: chars = %d, want %d", fixture.Path, snap.Cart.Chars, fixture.Chars)
		}
		tokenSum, charSum := 0, 0
		for _, count := range snap.Cart.FileTokens {
			tokenSum += count
		}
		for _, count := range snap.Cart.FileChars {
			charSum += count
		}
		if tokenSum != snap.Cart.Tokens || charSum != snap.Cart.Chars {
			t.Errorf("%s: per-file contributions = %d tokens/%d chars, want %d/%d",
				fixture.Path, tokenSum, charSum, snap.Cart.Tokens, snap.Cart.Chars)
		}
	}
}

func BenchmarkParseCorpusCart(b *testing.B) {
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "carts", "apa64__try-pico8", "topdown1.p8"))
	if err != nil {
		b.Fatal(err)
	}
	loader := NewLoader(DiskProvider{}, "", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := loader.Load(uri.File(path)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse64K(b *testing.B) {
	unit := "function f()\n  local t={1,2,3}\n  return t[1]\nend\n"
	code := strings.Repeat(unit, (64*1024)/len(unit))
	root := uri.File("/bench/cart.p8")
	p := newMapProvider()
	p.files[root] = "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n" + code
	loader := NewLoader(p, "", nil)
	b.SetBytes(int64(len(code)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := loader.Load(root); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFlattenTenIncludes(b *testing.B) {
	root := uri.File("/bench/cart.p8")
	p := newMapProvider()
	code := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n"
	for i := 0; i < 10; i++ {
		name := string(rune('a' + i))
		inc := uri.File("/bench/" + name + ".lua")
		p.files[inc] = "function f" + name + "() return 1 end\n"
		code += "#include \"" + name + ".lua\"\n"
	}
	p.files[root] = code
	loader := NewLoader(p, "", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := loader.Load(root); err != nil {
			b.Fatal(err)
		}
	}
}

func TestCorpusTokenSpansAreWithinFiles(t *testing.T) {
	loader := NewLoader(DiskProvider{}, "", nil)
	for _, path := range corpusFiles(t) {
		abs, err := filepath.Abs(path)
		if err != nil {
			t.Fatalf("abs: %v", err)
		}
		snap, err := loader.Load(uri.File(abs))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		check := func(kind, text string, span source.Span) {
			file := snap.Cart.File(span.FileID)
			if file == nil {
				t.Fatalf("%s: %s %q has no file: %+v", path, kind, text, span)
			}
			if span.Start < 0 || span.End < span.Start || span.End > len(file.Text) {
				t.Fatalf("%s: %s %q span out of bounds: %+v (len %d)", path, kind, text, span, len(file.Text))
			}
		}
		for _, tok := range snap.Tokens {
			check("token", tok.Text, tok.Span)
		}
		for _, comment := range snap.Comments {
			check("comment", comment.Text, comment.Span)
		}
	}
}
