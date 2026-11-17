package cart

import (
	"unicode/utf8"

	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// CountTokens applies PICO-8's token counting rules to a lexed program.
//
// Compound assignments count as one token because PICO-8/shrinko8 count the
// operator once. This is narrower than the misleading "operator + assignment"
// phrasing in some community references; the golden fixtures use shrinko8.
func CountTokens(tokens []syntax.Token) int {
	count := 0
	for i, tok := range tokens {
		switch tok.Kind {
		case syntax.TokenEOF, syntax.TokenComment, syntax.TokenInvalid, syntax.TokenTabMarker:
			continue
		}

		if tok.Kind == syntax.TokenPunct || tok.Kind == syntax.TokenKeyword {
			switch tok.Text {
			case ",", ".", ":", ";", "::", ")", "]", "}":
				continue
			}
			if tok.Kind == syntax.TokenKeyword {
				switch tok.Value {
				case "local", "end":
					continue
				}
			}
		}

		if (tok.Text == "-" || tok.Text == "~") && i+1 < len(tokens) && tokens[i+1].Kind == syntax.TokenNumber {
			if i == 0 || isUnaryFoldedContext(tokens[i-1]) {
				continue
			}
		}

		count++
	}
	return count
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
