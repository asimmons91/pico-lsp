package cart

import (
	"fmt"
	"strings"
)

// ReplaceCode returns the cart bytes with the __lua__ body replaced by code.
// The input is the complete code body, including -->8 markers.
//
// Only the existing body region changes. The BOM, header, section order,
// unknown sections, line endings outside the code body, and every other byte
// are preserved. Code is normalized to LF and then restored to CRLF when the
// original body used CRLF. A lone CR is P8SCII glyph 13 data and is never
// treated as a line terminator.
//
// ReplaceCode never mutates the document and never returns partial output.
func (d *Document) ReplaceCode(code string) ([]byte, error) {
	if !d.hasCode {
		return nil, fmt.Errorf("%w: %s", ErrNoCodeSection, d.path)
	}

	normalized := strings.ReplaceAll(code, "\r\n", "\n")
	if d.headerUnterminated && normalized != "" {
		return nil, fmt.Errorf("%w: %s", ErrUnterminatedHeader, d.path)
	}

	newline := "\n"
	if d.codeUsesCRLF {
		newline = "\r\n"
		normalized = strings.ReplaceAll(normalized, "\n", newline)
	}
	if d.codeEnd < len(d.data) && normalized != "" && !strings.HasSuffix(normalized, "\n") {
		normalized += newline
	}

	out := make([]byte, 0, len(d.data)-len(d.code)+len(normalized))
	out = append(out, d.data[:d.codeStart]...)
	out = append(out, normalized...)
	out = append(out, d.data[d.codeEnd:]...)
	return out, nil
}
