package cart

import (
	"strings"

	"github.com/asimmons91/pico-lsp/internal/p8compress"
	"github.com/asimmons91/pico-lsp/internal/p8scii"
)

// CompressedSize returns the number of bytes PICO-8 would use to store the
// assembled code: a pxa stream including its header, or the raw code length
// when compression does not help. It is computed once per snapshot. Standalone
// .lua files and carts whose code cannot be compressed within the work budget
// report ok=false.
func (s *Snapshot) CompressedSize() (int, bool) {
	if s == nil {
		return 0, false
	}
	s.compressedOnce.Do(func() {
		if s.Cart == nil || !s.Cart.CountsAvailable || !isCartRoot(s.Cart.RootURI.Path()) {
			return
		}
		s.compressedSize, s.compressedOK = p8compress.Size(compressedInput(s.Cart.Flat))
	})
	return s.compressedSize, s.compressedOK
}

func isCartRoot(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".p8")
}

// compressedInput mirrors the code PICO-8 stores: line endings are normalized
// to LF and each glyph is encoded as one P8SCII byte.
func compressedInput(flat []byte) []byte {
	normalized := make([]byte, 0, len(flat))
	for i := 0; i < len(flat); i++ {
		switch flat[i] {
		case '\r':
			normalized = append(normalized, '\n')
			if i+1 < len(flat) && flat[i+1] == '\n' {
				i++
			}
		default:
			normalized = append(normalized, flat[i])
		}
	}
	return p8scii.Encode(string(normalized))
}
