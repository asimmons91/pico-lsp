package p8compress

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asimmons91/pico-lsp/internal/p8scii"
)

type fixture struct {
	File        string `json:"file"`
	Compressed  int    `json:"compressed"`
	P8SCIIBytes int    `json:"p8sciiBytes"`
}

func loadFixtures(t *testing.T) []fixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "p8compress", "sizes.json"))
	if err != nil {
		t.Fatalf("read fixture manifest: %v", err)
	}
	var fixtures []fixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("decode fixture manifest: %v", err)
	}
	if len(fixtures) < 8 {
		t.Fatalf("fixture count = %d, want at least 8", len(fixtures))
	}
	return fixtures
}

// TestCompressReferenceParity checks sizes produced by Lexaloffle's reference
// pxa compressor. See testdata/p8compress/README.md for generation details.
func TestCompressReferenceParity(t *testing.T) {
	for _, tc := range loadFixtures(t) {
		text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "p8compress", tc.File))
		if err != nil {
			t.Fatalf("%s: %v", tc.File, err)
		}
		code := p8scii.Encode(string(text))
		if len(code) != tc.P8SCIIBytes {
			t.Errorf("%s: P8SCII bytes = %d, want %d", tc.File, len(code), tc.P8SCIIBytes)
		}
		out, ok := Compress(code)
		if !ok {
			t.Errorf("%s: compression unavailable", tc.File)
			continue
		}
		if len(out) != tc.Compressed {
			t.Errorf("%s: compressed = %d, want %d", tc.File, len(out), tc.Compressed)
		}
	}
}

// TestCompressGoldenBytes pins the exact stream for small fixtures, not just
// its length, so stream-format regressions are caught.
func TestCompressGoldenBytes(t *testing.T) {
	for _, name := range []string{"simple.lua", "utf8.lua"} {
		text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "p8compress", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		wantHex, err := os.ReadFile(filepath.Join("..", "..", "testdata", "p8compress", name+".pxa.hex"))
		if err != nil {
			t.Fatalf("%s golden: %v", name, err)
		}
		want, err := hex.DecodeString(strings.TrimSpace(string(wantHex)))
		if err != nil {
			t.Fatalf("%s golden hex: %v", name, err)
		}
		got, ok := Compress(p8scii.Encode(string(text)))
		if !ok {
			t.Fatalf("%s: compression unavailable", name)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s: compressed stream differs from reference", name)
		}
	}
}

func TestCompressRawFallback(t *testing.T) {
	code := []byte("print(1)")
	out, ok := Compress(code)
	if !ok {
		t.Fatal("compression unavailable")
	}
	if !bytes.Equal(out, code) {
		t.Fatalf("out = %q, want raw input", out)
	}
}

func TestCompressEmpty(t *testing.T) {
	out, ok := Compress(nil)
	if !ok || len(out) != 0 {
		t.Fatalf("empty: out=%v ok=%v", out, ok)
	}
}

func TestCompressOversized(t *testing.T) {
	if _, ok := Compress(make([]byte, MaxInput+1)); ok {
		t.Fatal("oversized input reported a size")
	}
}

func TestCompressWorkLimit(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "p8compress", "near_limit.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := CompressLimit(text, 1000); ok {
		t.Fatal("tiny work limit reported a size")
	}
}

func TestCompressRoundTrip(t *testing.T) {
	for _, tc := range loadFixtures(t) {
		text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "p8compress", tc.File))
		if err != nil {
			t.Fatalf("%s: %v", tc.File, err)
		}
		code := p8scii.Encode(string(text))
		out, ok := Compress(code)
		if !ok {
			t.Fatalf("%s: compression unavailable", tc.File)
		}
		got, ok := decompress(t, out, len(code))
		if !ok {
			t.Fatalf("%s: output did not decompress", tc.File)
		}
		if !bytes.Equal(got, code) {
			t.Fatalf("%s: round trip differs (%d vs %d bytes)", tc.File, len(got), len(code))
		}
	}
}

// decompress is a test-only port of PICO-8's pxa decoder. Raw (uncompressed)
// code is returned unchanged.
func decompress(t *testing.T, data []byte, wantLen int) ([]byte, bool) {
	t.Helper()
	if !bytes.HasPrefix(data, []byte{0, 'p', 'x', 'a'}) {
		return data, true
	}
	if len(data) < HeaderSize {
		t.Fatalf("short pxa header")
	}
	uncSize := int(data[4])<<8 | int(data[5])
	comSize := int(data[6])<<8 | int(data[7])
	if uncSize != wantLen || comSize < HeaderSize || comSize > len(data) {
		t.Fatalf("bad pxa header: unc=%d com=%d len=%d", uncSize, comSize, len(data))
	}
	br := testBitReader{data: data[HeaderSize:comSize]}
	out := make([]byte, 0, uncSize)
	mtf := make([]byte, 256)
	for i := range mtf {
		mtf[i] = byte(i)
	}
	for len(out) < uncSize {
		if br.bit() == 1 {
			extra := 0
			for br.bit() == 1 {
				extra++
				if extra > 12 {
					t.Fatalf("literal category overflow")
				}
			}
			index := br.bits(4+extra) + ((1<<extra)-1)<<4
			if index < 0 || index >= 256 {
				t.Fatalf("literal index %d", index)
			}
			ch := mtf[index]
			out = append(out, ch)
			copy(mtf[1:index+1], mtf[:index])
			mtf[0] = ch
			continue
		}
		offlen := 15
		if br.bit() == 1 {
			if br.bit() == 1 {
				offlen = 5
			} else {
				offlen = 10
			}
		}
		offset := br.bits(offlen) + 1
		if offset == 1 && offlen == 10 {
			for {
				ch := br.bits(8)
				if ch == 0 {
					break
				}
				out = append(out, byte(ch))
				if len(out) > uncSize {
					t.Fatalf("raw block overflow")
				}
			}
			continue
		}
		count := 3
		for {
			part := br.bits(3)
			count += part
			if part != 7 {
				break
			}
		}
		if offset <= 0 || offset > len(out) || count > uncSize-len(out) {
			t.Fatalf("bad copy offset=%d count=%d out=%d", offset, count, len(out))
		}
		for i := 0; i < count; i++ {
			out = append(out, out[len(out)-offset])
		}
	}
	return out, true
}

type testBitReader struct {
	data      []byte
	pos       int
	bitOffset uint
}

func (r *testBitReader) bit() int {
	if r.pos >= len(r.data) {
		panic("testBitReader: out of data")
	}
	v := int((r.data[r.pos] >> r.bitOffset) & 1)
	r.bitOffset++
	if r.bitOffset == 8 {
		r.bitOffset = 0
		r.pos++
	}
	return v
}

func (r *testBitReader) bits(n int) int {
	value := 0
	for i := 0; i < n; i++ {
		value |= r.bit() << i
	}
	return value
}
