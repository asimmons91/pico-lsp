package p8scii_test

import (
	"testing"
	"unicode/utf8"

	"github.com/asimmons91/pico-lsp/pkg/p8scii"
)

func FuzzEncodeDecode(f *testing.F) {
	f.Add("")
	f.Add("print(1)")
	f.Add("♥⬅️😐️")
	f.Add("あいうえお")
	f.Add("é")
	f.Fuzz(func(t *testing.T, text string) {
		if !utf8.ValidString(text) {
			t.Skip()
		}
		decoded := p8scii.Decode(p8scii.Encode(text))
		if !utf8.ValidString(decoded) {
			t.Fatalf("Decode(Encode(%q)) = %q, which is not valid UTF-8", text, decoded)
		}
	})
}
