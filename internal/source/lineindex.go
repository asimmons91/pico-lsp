// Package source contains source-text infrastructure shared by the language
// server. Positions at the LSP boundary are always converted through a
// LineIndex; byte offsets are used internally.
package source

import (
	"fmt"
	"sort"
	"unicode/utf8"

	"go.lsp.dev/protocol"
)

// LineIndex maps byte offsets in a UTF-8 text buffer to LSP positions and back.
//
// The zero value is not usable; construct one with New.
type LineIndex struct {
	text       []byte
	lineStarts []int
	encoding   protocol.PositionEncodingKind
}

// New builds a LineIndex for text. encoding selects how LSP character columns
// are interpreted. Unknown encodings fall back to UTF-16, which every LSP
// client must support.
func New(text []byte, encoding protocol.PositionEncodingKind) *LineIndex {
	switch encoding {
	case protocol.PositionEncodingKindUTF8,
		protocol.PositionEncodingKindUTF16,
		protocol.PositionEncodingKindUTF32:
	default:
		encoding = protocol.PositionEncodingKindUTF16
	}

	owned := make([]byte, len(text))
	copy(owned, text)

	return &LineIndex{
		text:       owned,
		lineStarts: buildLineStarts(owned),
		encoding:   encoding,
	}
}

// Len returns the length of the indexed text in bytes.
func (l *LineIndex) Len() int { return len(l.text) }

// LineCount returns the number of indexed lines. Empty input has one (empty)
// line.
func (l *LineIndex) LineCount() int { return len(l.lineStarts) }

// Encoding returns the negotiated position encoding.
func (l *LineIndex) Encoding() protocol.PositionEncodingKind { return l.encoding }

// Offset converts an LSP position to a byte offset.
//
// Positions past the end of the document, or columns past the end of a line,
// are clamped to the nearest valid position. A column that lands inside a
// multi-byte UTF-8 sequence or a UTF-16 surrogate pair is clamped down to the
// start of that character.
func (l *LineIndex) Offset(pos protocol.Position) (int, error) {
	line := int(pos.Line)
	if line >= len(l.lineStarts) {
		return len(l.text), nil
	}

	start := l.lineStarts[line]
	end := l.lineEnd(line)
	return advanceColumn(l.text[start:end], start, pos.Character, l.encoding), nil
}

// Position converts a byte offset to an LSP position.
//
// Negative offsets are an error. Offsets past the end of the document are
// clamped. Offsets inside a line terminator are reported at the end of that
// line's content.
func (l *LineIndex) Position(offset int) (protocol.Position, error) {
	if offset < 0 {
		return protocol.Position{}, fmt.Errorf("source: negative offset %d", offset)
	}
	if offset > len(l.text) {
		offset = len(l.text)
	}

	line := l.lineForOffset(offset)
	start := l.lineStarts[line]
	end := l.lineEnd(line)
	if offset > end {
		offset = end
	}
	// Never report a position inside a multi-byte UTF-8 sequence. Offsets that
	// land there are implementation details, not valid LSP positions.
	for offset > start && offset < len(l.text) && !utf8.RuneStart(l.text[offset]) {
		offset--
	}

	return protocol.Position{
		Line:      uint32(line),
		Character: l.column(start, offset),
	}, nil
}

// Range converts a byte range to an LSP range. Reversed ranges are normalized
// and out-of-bounds offsets are clamped.
func (l *LineIndex) Range(start, end int) protocol.Range {
	if start > end {
		start, end = end, start
	}
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
	if end > len(l.text) {
		end = len(l.text)
	}

	startPos, _ := l.Position(start)
	endPos, _ := l.Position(end)
	return protocol.Range{Start: startPos, End: endPos}
}

// RangeOffsets converts an LSP range to a pair of byte offsets.
func (l *LineIndex) RangeOffsets(r protocol.Range) (start, end int, err error) {
	start, err = l.Offset(r.Start)
	if err != nil {
		return 0, 0, err
	}
	end, err = l.Offset(r.End)
	if err != nil {
		return 0, 0, err
	}
	if start > end {
		start, end = end, start
	}
	return start, end, nil
}

// buildLineStarts records the byte offset at which each line begins. It treats
// LF, CRLF, and lone CR as line breaks.
func buildLineStarts(text []byte) []int {
	starts := make([]int, 1, 1+len(text)/32)
	starts[0] = 0

	for i := 0; i < len(text); {
		switch text[i] {
		case '\r':
			if i+1 < len(text) && text[i+1] == '\n' {
				i += 2
			} else {
				i++
			}
			starts = append(starts, i)
		case '\n':
			i++
			starts = append(starts, i)
		default:
			i++
		}
	}

	return starts
}

// lineForOffset returns the zero-based line containing offset. An offset at a
// line start belongs to that line.
func (l *LineIndex) lineForOffset(offset int) int {
	line := sort.Search(len(l.lineStarts), func(i int) bool {
		return l.lineStarts[i] > offset
	}) - 1
	if line < 0 {
		return 0
	}
	return line
}

// lineEnd returns the byte offset just past the last content byte of line,
// excluding its line terminator.
func (l *LineIndex) lineEnd(line int) int {
	start := l.lineStarts[line]
	next := len(l.text)
	if line+1 < len(l.lineStarts) {
		next = l.lineStarts[line+1]
	}

	end := next
	if end <= start {
		return end
	}

	switch l.text[end-1] {
	case '\n':
		end--
		if end > start && l.text[end-1] == '\r' {
			end--
		}
	case '\r':
		end--
	}

	return end
}

// column returns the LSP character column of offset within the line starting at
// start. offset must not be inside a line terminator.
func (l *LineIndex) column(start, offset int) uint32 {
	if offset <= start {
		return 0
	}

	segment := l.text[start:offset]
	switch l.encoding {
	case protocol.PositionEncodingKindUTF8:
		return uint32(len(segment))
	case protocol.PositionEncodingKindUTF32:
		return uint32(utf8.RuneCount(segment))
	default:
		var units uint32
		for len(segment) > 0 {
			r, size := utf8.DecodeRune(segment)
			if r >= 0x10000 {
				units += 2
			} else {
				units++
			}
			segment = segment[size:]
		}
		return units
	}
}

// advanceColumn returns the byte offset of the given LSP column within segment,
// relative to base. Columns past the end of the segment are clamped. A column
// inside a character is clamped down to the character's first byte.
func advanceColumn(segment []byte, base int, column uint32, encoding protocol.PositionEncodingKind) int {
	switch encoding {
	case protocol.PositionEncodingKindUTF8:
		if int64(column) >= int64(len(segment)) {
			return base + len(segment)
		}
		offset := int(column)
		for offset > 0 && !utf8.RuneStart(segment[offset]) {
			offset--
		}
		return base + offset

	case protocol.PositionEncodingKindUTF32:
		var units uint32
		for i := 0; i < len(segment); {
			if units >= column {
				return base + i
			}
			_, size := utf8.DecodeRune(segment[i:])
			units++
			i += size
		}
		return base + len(segment)

	default:
		var units uint32
		for i := 0; i < len(segment); {
			if units >= column {
				return base + i
			}

			r, size := utf8.DecodeRune(segment[i:])
			width := uint32(1)
			if r >= 0x10000 {
				width = 2
			}
			if units+width > column {
				return base + i
			}

			units += width
			i += size
		}
		return base + len(segment)
	}
}
