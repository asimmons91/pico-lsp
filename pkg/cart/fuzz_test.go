package cart_test

import (
	"errors"
	"testing"

	"github.com/asimmons91/pico-lsp/pkg/cart"
)

func FuzzParse(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\na=1\n__gfx__\n"))
	f.Add([]byte("pico-8 cartridge // pico-8.com\r\nversion 42\r\n__lua__\r\na=1\r\n__gfx__\r\n"))
	f.Add([]byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\ns=\"a\rb\"\n__gfx__\n"))
	f.Add([]byte("\ufeffpico-8 cartridge // pico-8.com\nversion 42\n__lua__\n-->8\n"))
	f.Add([]byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"missing.lua\"\n"))
	f.Add([]byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n__\n"))
	f.Add([]byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n___\n____\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := cart.Parse("fuzz.p8", data)
		if err != nil {
			if !typedParseError(err) {
				t.Fatalf("Parse returned an untyped error: %v", err)
			}
			return
		}
		_ = doc.Path()
		_ = doc.Code()
		_ = doc.HasCode()
		_ = doc.CodeStartLine()
		_ = doc.Tabs()
		_ = doc.Sections()
		_ = doc.Includes()
		if doc.HasCode() {
			if _, err := doc.ReplaceCode(doc.Code()); err != nil {
				t.Fatalf("ReplaceCode(Code()) failed: %v", err)
			}
		}
	})
}

func typedParseError(err error) bool {
	if errors.Is(err, cart.ErrUnsupportedFormat) ||
		errors.Is(err, cart.ErrInvalidEncoding) ||
		errors.Is(err, cart.ErrTooLarge) ||
		errors.Is(err, cart.ErrDuplicateLua) ||
		errors.Is(err, cart.ErrMalformedSection) {
		return true
	}
	var parseErr *cart.ParseError
	return errors.As(err, &parseErr)
}
