package cart

import (
	"strings"
	"testing"

	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/p8compress"
)

func TestSnapshotCompressedSize(t *testing.T) {
	root := uri.File("/cart/compress.p8")
	p := newMapProvider()
	code := "function f()\n local t={1,2,3}\n return t[1]\nend\n"
	p.files[root] = "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n" + code
	snap, err := NewLoader(p, "", nil).Load(root)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := snap.CompressedSize()
	if !ok {
		t.Fatal("compressed size unavailable")
	}
	want, _ := p8compress.Size(compressedInput(snap.Cart.Flat))
	if got != want {
		t.Fatalf("compressed = %d, want %d", got, want)
	}
	if again, ok := snap.CompressedSize(); !ok || again != got {
		t.Fatalf("memoized compressed = %d, want %d", again, got)
	}
}

func TestSnapshotCompressedSizeStandaloneLua(t *testing.T) {
	root := uri.File("/tmp/standalone.lua")
	p := newMapProvider()
	p.files[root] = "print(1)\n"
	snap, err := NewLoader(p, "", nil).Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snap.CompressedSize(); ok {
		t.Fatal("standalone .lua reported a compressed size")
	}
}

func TestSnapshotCompressedSizeNormalizesNewlines(t *testing.T) {
	p := newMapProvider()
	lf := uri.File("/cart/lf.p8")
	crlf := uri.File("/cart/crlf.p8")
	body := "function f()\n return 1\nend\n"
	p.files[lf] = "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n" + body
	p.files[crlf] = strings.ReplaceAll(p.files[lf], "\n", "\r\n")
	loader := NewLoader(p, "", nil)
	lfSnap, err := loader.Load(lf)
	if err != nil {
		t.Fatal(err)
	}
	crlfSnap, err := loader.Load(crlf)
	if err != nil {
		t.Fatal(err)
	}
	lfSize, ok := lfSnap.CompressedSize()
	if !ok {
		t.Fatal("LF cart unavailable")
	}
	crlfSize, ok := crlfSnap.CompressedSize()
	if !ok {
		t.Fatal("CRLF cart unavailable")
	}
	if lfSize != crlfSize {
		t.Fatalf("LF size %d != CRLF size %d", lfSize, crlfSize)
	}
}

func TestCompressedInputP8SCII(t *testing.T) {
	// Each P8SCII glyph becomes one byte, so UTF-8 source compresses like its
	// narrower P8SCII representation.
	got := compressedInput([]byte("⬅️➡️⬆️⬇️"))
	want := []byte{0x8b, 0x91, 0x94, 0x83}
	if string(got) != string(want) {
		t.Fatalf("compressedInput = %v, want %v", got, want)
	}
}
