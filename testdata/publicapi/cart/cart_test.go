package publicapi

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/asimmons91/pico-lsp/pkg/cart"
)

func TestCartPublicAPI(t *testing.T) {
	dir := t.TempDir()
	cartPath := filepath.Join(dir, "game.p8")
	includePath := filepath.Join(dir, "lib.lua")
	text := "pico-8 cartridge // pico-8.com\r\nversion 42\r\n__lua__\r\na=1\r\n-->8 second\r\n#include \"lib.lua\"\r\nb=2\r\n__gfx__\r\n0000\r\n"

	doc, err := cart.Parse(cartPath, []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Path() != cartPath {
		t.Fatalf("Path() = %q, want %q", doc.Path(), cartPath)
	}
	if !doc.HasCode() || doc.CodeStartLine() != 3 {
		t.Fatalf("HasCode/CodeStartLine = %t/%d", doc.HasCode(), doc.CodeStartLine())
	}
	if !bytes.Contains([]byte(doc.Code()), []byte("-->8 second\r\n")) {
		t.Fatalf("Code() = %q, want CRLF tab marker", doc.Code())
	}
	tabs := doc.Tabs()
	if len(tabs) != 2 || tabs[0].Index != 0 || tabs[1].Name != "second" || tabs[1].MarkerLine != 4 {
		t.Fatalf("Tabs() = %+v", tabs)
	}
	includes := doc.Includes()
	if len(includes) != 1 || includes[0].Path != "lib.lua" || includes[0].Line != 5 {
		t.Fatalf("Includes() = %+v", includes)
	}

	if err := os.WriteFile(cartPath, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(includePath, []byte("x=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	owner, err := cart.NewResolver(nil).Owner(includePath)
	if err != nil {
		t.Fatal(err)
	}
	if owner != cartPath {
		t.Fatalf("Owner = %q, want %q", owner, cartPath)
	}

	out, err := doc.ReplaceCode("x=1\n")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("__lua__\r\nx=1\r\n__gfx__")) {
		t.Fatalf("ReplaceCode output = %q, want CRLF body", out)
	}
	if !bytes.Contains(out, []byte("pico-8 cartridge // pico-8.com\r\nversion 42\r\n")) {
		t.Fatal("ReplaceCode changed the header")
	}
}
