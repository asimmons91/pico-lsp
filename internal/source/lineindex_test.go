package source

import (
	"strings"
	"testing"
	"unicode/utf8"

	"go.lsp.dev/protocol"
)

var testTexts = []string{
	"",
	"a",
	"abc",
	"abc\ndef",
	"abc\ndef\n",
	"\n",
	"\n\n",
	"a\r\nb\r\n",
	"a\rb\r",
	"a\r\nb\nc\rd",
	"héllo wörld",
	"a😀b",
	"😀😀\n😀",
	"⬅️🅾️❎",
	"x⬅️\ny",
	"line with trailing spaces   \n",
	"tab\there\nend",
	"a\n\nb",
}

func TestLineIndexRoundTrip(t *testing.T) {
	encodings := []protocol.PositionEncodingKind{
		protocol.PositionEncodingKindUTF8,
		protocol.PositionEncodingKindUTF16,
		protocol.PositionEncodingKindUTF32,
	}

	for _, encoding := range encodings {
		for _, text := range testTexts {
			t.Run(string(encoding)+"/"+testName(text), func(t *testing.T) {
				index := New([]byte(text), encoding)
				ranges := contentRanges(text)

				if index.Len() != len(text) {
					t.Fatalf("Len() = %d, want %d", index.Len(), len(text))
				}
				if index.LineCount() != len(ranges) {
					t.Fatalf("LineCount() = %d, want %d", index.LineCount(), len(ranges))
				}

				for _, r := range ranges {
					for offset := r[0]; offset <= r[1]; {
						pos, err := index.Position(offset)
						if err != nil {
							t.Fatalf("Position(%d): %v", offset, err)
						}

						got, err := index.Offset(pos)
						if err != nil {
							t.Fatalf("Offset(%+v): %v", pos, err)
						}
						if got != offset {
							t.Fatalf("round trip offset %d -> %+v -> %d", offset, pos, got)
						}

						if offset == r[1] {
							break
						}
						_, size := utf8.DecodeRuneInString(text[offset:])
						offset += size
					}
				}
			})
		}
	}
}

func TestLineIndexByteOffsetsAreSafe(t *testing.T) {
	encodings := []protocol.PositionEncodingKind{
		protocol.PositionEncodingKindUTF8,
		protocol.PositionEncodingKindUTF16,
		protocol.PositionEncodingKindUTF32,
	}

	for _, encoding := range encodings {
		for _, text := range testTexts {
			index := New([]byte(text), encoding)

			for offset := 0; offset <= len(text); offset++ {
				pos, err := index.Position(offset)
				if err != nil {
					t.Fatalf("%q/%s: Position(%d): %v", text, encoding, offset, err)
				}

				back, err := index.Offset(pos)
				if err != nil {
					t.Fatalf("%q/%s: Offset(%+v): %v", text, encoding, pos, err)
				}
				if back > offset {
					t.Fatalf("%q/%s: offset %d -> %+v -> %d moved forward", text, encoding, offset, pos, back)
				}
				if back < len(text) && !utf8.RuneStart(text[back]) {
					t.Fatalf("%q/%s: offset %d -> %+v -> %d is not a rune boundary", text, encoding, offset, pos, back)
				}
			}
		}
	}
}

func TestLineIndexKnownPositions(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		encoding protocol.PositionEncodingKind
		offset   int
		want     protocol.Position
	}{
		{name: "empty", text: "", offset: 0, want: protocol.Position{}},
		{name: "ascii end", text: "abc", offset: 3, want: protocol.Position{Line: 0, Character: 3}},
		{name: "second line", text: "abc\ndef", offset: 4, want: protocol.Position{Line: 1, Character: 0}},
		{name: "trailing newline", text: "abc\n", offset: 4, want: protocol.Position{Line: 1, Character: 0}},
		{name: "crlf first line end", text: "a\r\nb", offset: 1, want: protocol.Position{Line: 0, Character: 1}},
		{name: "crlf second line", text: "a\r\nb", offset: 3, want: protocol.Position{Line: 1, Character: 0}},
		{name: "crlf inside terminator", text: "a\r\nb", offset: 2, want: protocol.Position{Line: 0, Character: 1}},
		{name: "lone cr", text: "a\rb", offset: 2, want: protocol.Position{Line: 1, Character: 0}},
		{name: "lone cr inside terminator", text: "a\rb", offset: 1, want: protocol.Position{Line: 0, Character: 1}},
		{name: "utf8", text: "héllo", encoding: protocol.PositionEncodingKindUTF8, offset: 3, want: protocol.Position{Line: 0, Character: 3}},
		{name: "utf16 multibyte", text: "héllo", offset: 3, want: protocol.Position{Line: 0, Character: 2}},
		{name: "utf16 emoji", text: "a😀b", offset: 5, want: protocol.Position{Line: 0, Character: 3}},
		{name: "utf16 emoji end", text: "a😀b", offset: 6, want: protocol.Position{Line: 0, Character: 4}},
		{name: "utf32 emoji", text: "a😀b", encoding: protocol.PositionEncodingKindUTF32, offset: 6, want: protocol.Position{Line: 0, Character: 3}},
		{name: "utf16 p8 glyphs", text: "⬅️x", offset: len("⬅️x"), want: protocol.Position{Line: 0, Character: 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoding := tt.encoding
			if encoding == "" {
				encoding = protocol.PositionEncodingKindUTF16
			}
			index := New([]byte(tt.text), encoding)

			got, err := index.Position(tt.offset)
			if err != nil {
				t.Fatalf("Position(%d): %v", tt.offset, err)
			}
			if got != tt.want {
				t.Fatalf("Position(%d) = %+v, want %+v", tt.offset, got, tt.want)
			}
		})
	}
}

func TestLineIndexOffsetClamping(t *testing.T) {
	index := New([]byte("a😀b\r\nsecond"), protocol.PositionEncodingKindUTF16)

	tests := []struct {
		name string
		pos  protocol.Position
		want int
	}{
		{name: "start", pos: protocol.Position{}, want: 0},
		{name: "after ascii", pos: protocol.Position{Line: 0, Character: 1}, want: 1},
		{name: "inside surrogate clamps down", pos: protocol.Position{Line: 0, Character: 2}, want: 1},
		{name: "after emoji", pos: protocol.Position{Line: 0, Character: 3}, want: 5},
		{name: "past line end", pos: protocol.Position{Line: 0, Character: 999}, want: 6},
		{name: "past document", pos: protocol.Position{Line: 99, Character: 99}, want: index.Len()},
		{name: "second line", pos: protocol.Position{Line: 1, Character: 2}, want: len("a😀b\r\nse")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := index.Offset(tt.pos)
			if err != nil {
				t.Fatalf("Offset(%+v): %v", tt.pos, err)
			}
			if got != tt.want {
				t.Fatalf("Offset(%+v) = %d, want %d", tt.pos, got, tt.want)
			}
		})
	}
}

func TestLineIndexRange(t *testing.T) {
	index := New([]byte("abc\ndef\n"), protocol.PositionEncodingKindUTF16)

	got := index.Range(4, 1)
	want := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 1},
		End:   protocol.Position{Line: 1, Character: 0},
	}
	if got != want {
		t.Fatalf("Range(4,1) = %+v, want %+v", got, want)
	}

	got = index.Range(-2, 100)
	want = protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 2, Character: 0},
	}
	if got != want {
		t.Fatalf("Range(-2,100) = %+v, want %+v", got, want)
	}

	start, end, err := index.RangeOffsets(protocol.Range{
		Start: protocol.Position{Line: 1, Character: 1},
		End:   protocol.Position{Line: 0, Character: 2},
	})
	if err != nil {
		t.Fatalf("RangeOffsets: %v", err)
	}
	if start != 2 || end != 5 {
		t.Fatalf("RangeOffsets = (%d,%d), want (2,5)", start, end)
	}

	if got := index.Range(-5, -1); got != (protocol.Range{}) {
		t.Fatalf("Range(-5,-1) = %+v, want zero range", got)
	}
}

func TestLineIndexNegativeOffset(t *testing.T) {
	index := New([]byte("abc"), protocol.PositionEncodingKindUTF16)
	if _, err := index.Position(-1); err == nil {
		t.Fatal("Position(-1) succeeded, want error")
	}
}

func TestLineIndexUnknownEncodingFallsBackToUTF16(t *testing.T) {
	index := New([]byte("a😀b"), protocol.PositionEncodingKind("made-up"))
	if index.Encoding() != protocol.PositionEncodingKindUTF16 {
		t.Fatalf("Encoding() = %q, want utf-16", index.Encoding())
	}
	got, err := index.Position(5)
	if err != nil {
		t.Fatal(err)
	}
	if got.Character != 3 {
		t.Fatalf("Character = %d, want 3", got.Character)
	}
}

func TestLineIndexInvalidUTF8DoesNotPanic(t *testing.T) {
	index := New([]byte{0xff, 'a', '\n', 0xfe}, protocol.PositionEncodingKindUTF16)

	for offset := 0; offset <= index.Len(); offset++ {
		pos, err := index.Position(offset)
		if err != nil {
			t.Fatalf("Position(%d): %v", offset, err)
		}
		if _, err := index.Offset(pos); err != nil {
			t.Fatalf("Offset(%+v): %v", pos, err)
		}
	}
}

func contentRanges(text string) [][2]int {
	var ranges [][2]int
	start := 0

	for i := 0; i < len(text); {
		switch text[i] {
		case '\r':
			ranges = append(ranges, [2]int{start, i})
			if i+1 < len(text) && text[i+1] == '\n' {
				i += 2
			} else {
				i++
			}
			start = i
		case '\n':
			ranges = append(ranges, [2]int{start, i})
			i++
			start = i
		default:
			i++
		}
	}

	ranges = append(ranges, [2]int{start, len(text)})
	return ranges
}

func testName(text string) string {
	if text == "" {
		return "empty"
	}
	name := strings.NewReplacer("\r", "CR", "\n", "LF").Replace(text)
	if len(name) > 30 {
		name = name[:30]
	}
	return name
}
