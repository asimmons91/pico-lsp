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
// are preserved. Code is normalized to LF and restored to the body's CRLF
// style. A lone CR is P8SCII glyph 13 data and is never treated as a line
// terminator. A data CR immediately before a caller CRLF, or at the end of code
// that needs a following-section terminator, forces a CRLF terminator so the CR
// stays data; the resulting body then counts as CRLF-styled for later
// replacements.
//
// ReplaceCode never mutates the document and never returns partial output.
func (d *Document) ReplaceCode(code string) ([]byte, error) {
	if !d.hasCode {
		return nil, fmt.Errorf("%w: %s", ErrNoCodeSection, d.path)
	}

	normalized := normalizeCallerNewlines(code)
	if d.headerUnterminated && normalized != "" {
		return nil, fmt.Errorf("%w: %s", ErrUnterminatedHeader, d.path)
	}

	newline := "\n"
	if d.codeUsesCRLF {
		newline = "\r\n"
		normalized = convertLFToCRLF(normalized)
	}
	if d.codeEnd < len(d.data) && normalized != "" && !strings.HasSuffix(normalized, "\n") {
		if strings.HasSuffix(normalized, "\r") {
			// Keep the caller's trailing CR as glyph-13 data. A lone LF would
			// pair with it and turn it into a CRLF line terminator.
			normalized += "\r\n"
		} else {
			normalized += newline
		}
	}

	out := make([]byte, 0, len(d.data)-len(d.code)+len(normalized))
	out = append(out, d.data[:d.codeStart]...)
	out = append(out, normalized...)
	out = append(out, d.data[d.codeEnd:]...)
	return out, nil
}

// normalizeCallerNewlines converts CRLF to LF while keeping a data CR that
// immediately precedes a CRLF distinct from the terminator. The LF after such a
// CR is restored to CRLF so reparsing still sees the CR as glyph-13 data.
func normalizeCallerNewlines(code string) string {
	var b strings.Builder
	b.Grow(len(code))
	for i := 0; i < len(code); i++ {
		if code[i] == '\r' && i+1 < len(code) && code[i+1] == '\n' {
			b.WriteByte('\n')
			i++
			continue
		}
		b.WriteByte(code[i])
	}
	normalized := b.String()
	if strings.Contains(normalized, "\r\n") {
		normalized = strings.ReplaceAll(normalized, "\r\n", "\r\r\n")
	}
	return normalized
}

// convertLFToCRLF converts bare LFs without doubling an existing data-CR CRLF.
func convertLFToCRLF(code string) string {
	var b strings.Builder
	b.Grow(len(code))
	for i := 0; i < len(code); i++ {
		if code[i] == '\n' && (i == 0 || code[i-1] != '\r') {
			b.WriteString("\r\n")
			continue
		}
		b.WriteByte(code[i])
	}
	return b.String()
}
