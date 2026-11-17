package cart

import (
	"github.com/asimmons91/pico-lsp/internal/source"
)

// appendFlat appends data to the flattened program and records its source
// segment. Adjacent chunks from the same file merge into one segment.
func appendFlat(flat []byte, segments []Segment, fileID source.FileID, fileOffset int, data []byte) ([]byte, []Segment) {
	if len(data) == 0 {
		return flat, segments
	}
	flatStart := len(flat)
	flat = append(flat, data...)
	if n := len(segments); n > 0 {
		last := &segments[n-1]
		lastFileEnd := last.FileOffset + (last.FlatEnd - last.FlatStart)
		if last.FileID == fileID && last.FlatEnd == flatStart && lastFileEnd == fileOffset {
			last.FlatEnd = len(flat)
			return flat, segments
		}
	}
	return flat, append(segments, Segment{
		FlatStart:  flatStart,
		FlatEnd:    len(flat),
		FileID:     fileID,
		FileOffset: fileOffset,
	})
}
