package p8scii

import (
	"bytes"
	"testing"
)

func TestDecode(t *testing.T) {
	cases := []struct {
		code []byte
		want string
	}{
		{[]byte{0x80, 0x81, '\n'}, "█▒\n"},
		{[]byte("print(1)"), "print(1)"},
		{[]byte{0x8d, 0x8e}, "♪🅾️"},
	}
	for _, tc := range cases {
		if got := Decode(tc.code); got != tc.want {
			t.Errorf("Decode(%v) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestEncodeDecodeAllBytes(t *testing.T) {
	for i := 0; i < 256; i++ {
		code := []byte{byte(i)}
		text := Decode(code)
		got := Encode(text)
		if !bytes.Equal(got, code) {
			t.Errorf("byte %#02x: Decode=%q Encode=%v, want %v", i, text, got, code)
		}
	}
}

func TestEncodeUnicode(t *testing.T) {
	cases := []struct {
		text string
		want []byte
	}{
		{"⬅️➡️⬆️⬇️", []byte{0x8b, 0x91, 0x94, 0x83}},
		{"🅾️❎", []byte{0x8e, 0x97}},
		{"あいうえお", []byte{0x9a, 0x9b, 0x9c, 0x9d, 0x9e}},
		{"\U0001d622", []byte{'A'}},
		{"é", []byte{0xc3, 0xa9}}, // unknown glyphs keep their UTF-8 bytes
	}
	for _, tc := range cases {
		if got := Encode(tc.text); !bytes.Equal(got, tc.want) {
			t.Errorf("Encode(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestEncodeASCII(t *testing.T) {
	text := "function f(a,b) return a+b end\n"
	if got := Encode(text); string(got) != text {
		t.Fatalf("Encode(%q) = %q", text, got)
	}
}
