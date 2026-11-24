package cart_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asimmons91/pico-lsp/pkg/cart"
)

func TestExampleHelloCart(t *testing.T) {
	path := filepath.Join("..", "..", "examples", "hello.p8")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := cart.Parse(path, data)
	if err != nil {
		t.Fatalf("parse example cart: %v", err)
	}
	if !doc.HasCode() {
		t.Fatal("example cart has no code")
	}
	if !strings.Contains(doc.Code(), "function _draw()") {
		t.Fatalf("example cart code = %q", doc.Code())
	}
}
