package cart

import (
	"unicode/utf8"

	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// CountTokens applies PICO-8's token counting rules to a lexed program.
//
// Compound assignments count as one token because PICO-8/shrinko8 count the
// operator once. This is narrower than the misleading "operator + assignment"
// phrasing in some community references; the golden fixtures use shrinko8.
func CountTokens(tokens []syntax.Token) int {
	count := 0
	for i := range tokens {
		if countsToken(tokens, i) {
			count++
		}
	}
	return count
}

// CountTokensByFile attributes counted tokens to their original source file.
func CountTokensByFile(tokens []syntax.Token) map[source.FileID]int {
	out := make(map[source.FileID]int)
	for i := range tokens {
		if countsToken(tokens, i) {
			out[tokens[i].Span.FileID]++
		}
	}
	return out
}

func countsToken(tokens []syntax.Token, i int) bool {
	tok := tokens[i]
	switch tok.Kind {
	case syntax.TokenEOF, syntax.TokenComment, syntax.TokenInvalid:
		return false
	}

	if tok.Kind == syntax.TokenPunct || tok.Kind == syntax.TokenKeyword {
		switch tok.Text {
		case ",", ".", ":", ";", "::", ")", "]", "}":
			return false
		}
		if tok.Kind == syntax.TokenKeyword {
			switch tok.Value {
			case "local", "end":
				return false
			}
		}
	}

	if (tok.Text == "-" || tok.Text == "~") && i+1 < len(tokens) && tokens[i+1].Kind == syntax.TokenNumber {
		if i == 0 || isUnaryFoldedContext(tokens[i-1]) {
			return false
		}
	}
	return true
}

func isUnaryFoldedContext(prev syntax.Token) bool {
	switch prev.Kind {
	case syntax.TokenNumber, syntax.TokenString, syntax.TokenName:
		return false
	}
	switch prev.Text {
	case ")", "]", "}", ";":
		return false
	}
	if prev.Kind == syntax.TokenKeyword && prev.Value == "end" {
		return false
	}
	return true
}

// CountCharsBySegment attributes flattened character contributions to the
// original file that supplied each segment. Duplicate includes are counted
// once per occurrence.
func CountCharsBySegment(segments []Segment, flat []byte) map[source.FileID]int {
	out := make(map[source.FileID]int)
	previousEnd := 0
	for _, segment := range segments {
		start, end := segment.FlatStart, segment.FlatEnd
		if start < 0 {
			start = 0
		}
		if end > len(flat) {
			end = len(flat)
		}
		if start > previousEnd {
			// Separators inserted by the flattener are part of the root cart's
			// assembled source.
			out[0] += CountChars(flat[previousEnd:start])
		}
		if end <= start {
			continue
		}
		out[segment.FileID] += CountChars(flat[start:end])
		previousEnd = end
	}
	if previousEnd < len(flat) {
		out[0] += CountChars(flat[previousEnd:])
	}
	return out
}

// CountChars returns the number of PICO-8/P8SCII characters in assembled
// code. Line endings are normalized to LF, and glyphs stored as a base
// codepoint plus U+FE0F (the emoji variation selector) count once. This
// matches shrinko8 and PICO-8's editor semantics.
func CountChars(code []byte) int {
	count := 0
	for i := 0; i < len(code); {
		switch code[i] {
		case '\r':
			if i+1 < len(code) && code[i+1] == '\n' {
				i++
			}
			count++
			i++
			continue
		case '\n':
			count++
			i++
			continue
		}
		r, size := utf8.DecodeRune(code[i:])
		if r != '\uFE0F' {
			count++
		}
		i += size
	}
	return count
}
