package p8png

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func encodePNG(t *testing.T, codeRegion []byte) []byte {
	t.Helper()
	if len(codeRegion) > codeSize {
		t.Fatalf("code region too large: %d", len(codeRegion))
	}
	payload := make([]byte, CartWidth*CartHeight)
	copy(payload[romSize:], codeRegion)
	img := image.NewNRGBA(image.Rect(0, 0, CartWidth, CartHeight))
	for y := 0; y < CartHeight; y++ {
		for x := 0; x < CartWidth; x++ {
			b := payload[y*CartWidth+x]
			img.SetNRGBA(x, y, color.NRGBA{
				R: 0xfc | (b>>4)&3,
				G: 0xfc | (b>>2)&3,
				B: 0xfc | b&3,
				A: 0xfc | (b>>6)&3,
			})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func uncompressedRegion(code []byte) []byte {
	region := make([]byte, codeSize)
	copy(region, code)
	return region
}

type bitWriter struct {
	data []byte
	bit  uint
}

func (w *bitWriter) writeBit(bit int) {
	if w.bit == 0 {
		w.data = append(w.data, 0)
	}
	if bit != 0 {
		w.data[len(w.data)-1] |= 1 << w.bit
	}
	w.bit = (w.bit + 1) & 7
}

func (w *bitWriter) writeBits(value, n int) {
	for i := 0; i < n; i++ {
		w.writeBit((value >> i) & 1)
	}
}

func newCompressed(code []byte) []byte {
	var bw bitWriter
	// Offset 1 with a 10-bit offset selects the literal-block form.
	bw.writeBit(0)
	bw.writeBit(1)
	bw.writeBit(0)
	bw.writeBits(0, 10)
	for _, ch := range code {
		bw.writeBits(int(ch), 8)
	}
	bw.writeBits(0, 8)
	return append([]byte{0, 'p', 'x', 'a', byte(len(code) >> 8), byte(len(code)), byte((len(bw.data) + 8) >> 8), byte(len(bw.data) + 8)}, bw.data...)
}

var oldInverse = func() map[byte]byte {
	out := make(map[byte]byte)
	for i, ch := range oldCodeTable {
		if i != 0 {
			out[ch] = byte(i)
		}
	}
	return out
}()

func oldCompressed(code []byte) []byte {
	out := []byte{':', 'c', ':', 0, byte(len(code) >> 8), byte(len(code)), 0, 0}
	for _, ch := range code {
		if idx, ok := oldInverse[ch]; ok {
			out = append(out, idx)
		} else {
			out = append(out, 0, ch)
		}
	}
	return append(out, 0, 0)
}

func TestDecodeFormats(t *testing.T) {
	code := []byte("x=1\nfunction f()\nend\n")
	cases := []struct {
		name string
		data []byte
	}{
		{name: "uncompressed", data: uncompressedRegion(code)},
		{name: "new", data: newCompressed(code)},
		{name: "old", data: oldCompressed(code)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Decode(encodePNG(t, tc.data))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if result.Code != string(code) {
				t.Fatalf("code = %q, want %q", result.Code, code)
			}
			if !bytes.Contains([]byte(result.Text), code) {
				t.Fatalf("text = %q", result.Text)
			}
		})
	}
}

func TestDecodeP8SCII(t *testing.T) {
	code := []byte{0x80, 0x81, '\n'}
	result, err := Decode(encodePNG(t, uncompressedRegion(code)))
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != "█▒\n" {
		t.Fatalf("code = %q", result.Code)
	}
}

func TestDecodeErrors(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	var small bytes.Buffer
	if err := png.Encode(&small, img); err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(small.Bytes()); err == nil {
		t.Fatal("small PNG decoded")
	}
	if _, err := Decode([]byte("not a png")); err == nil {
		t.Fatal("non-PNG decoded")
	}
	badNew := newCompressed([]byte("x=1\n"))
	badNew[4], badNew[5] = 0xff, 0xff
	if _, err := Decode(encodePNG(t, badNew)); err == nil {
		t.Fatal("oversized compressed header decoded")
	}
}

func TestDecodeFixture(t *testing.T) {
	result, err := DecodeFile("../../testdata/p8png/smoke.p8.png")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Code, "function _init()") {
		t.Fatalf("fixture code = %q", result.Code)
	}
	if !strings.Contains(result.Text, "__lua__") {
		t.Fatalf("fixture text = %q", result.Text)
	}
}
