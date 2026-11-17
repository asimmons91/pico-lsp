package syntax

import (
	"strings"
	"unicode/utf8"

	"github.com/asimmons91/pico-lsp/internal/source"
)

// Mapper converts a range in the flattened program to an original-file span.
type Mapper func(start, end int) source.Span

const maxLexErrors = 100

var keywords = map[string]struct{}{
	"and": {}, "break": {}, "do": {}, "else": {}, "elseif": {},
	"end": {}, "false": {}, "for": {}, "function": {}, "goto": {},
	"if": {}, "in": {}, "local": {}, "nil": {}, "not": {}, "or": {},
	"repeat": {}, "return": {}, "then": {}, "true": {}, "until": {},
	"while": {},
}

// Lex tokenizes a flattened PICO-8 program. mapper maps flat byte offsets back
// to original files; when nil, an identity mapper with FileID 0 is used.
func Lex(src []byte, mapper Mapper) Result {
	if mapper == nil {
		mapper = func(start, end int) source.Span {
			return source.Span{Start: start, End: end}
		}
	}

	l := &lexer{
		text:               string(src),
		mapper:             mapper,
		lineOnlyWhitespace: true,
	}
	l.run()

	l.tokens = append(l.tokens, Token{
		Kind: TokenEOF,
		Span: l.mapper(len(l.text), len(l.text)),
		Line: l.line,
	})

	return Result{
		Tokens:   l.tokens,
		Comments: l.comments,
		Errors:   l.errors,
	}
}

type lexer struct {
	text               string
	mapper             Mapper
	offset             int
	line               int
	lineStart          int
	lineOnlyWhitespace bool
	tokens             []Token
	comments           []Comment
	errors             []Error
	tooManyErrors      bool
	tokenLine          int
	tokenLineStart     int
}

func (l *lexer) run() {
	for l.offset < len(l.text) {
		l.tokenLine = l.line
		l.tokenLineStart = l.lineStart
		c := l.text[l.offset]

		switch {
		case c == ' ' || c == '\t' || c == '\v' || c == '\f':
			l.offset++
		case c == '\r' || c == '\n':
			l.consumeNewline()
		case c == '-' && l.peek(1) == '-':
			l.lexDashComment()
			l.lineOnlyWhitespace = false
		case c == '/' && l.peek(1) == '/':
			l.lexLineComment()
			l.lineOnlyWhitespace = false
		case c == '\'' || c == '"':
			l.lexShortString()
			l.lineOnlyWhitespace = false
		case c == '[' && l.longBracketLevel(l.offset) >= 0:
			l.lexLongString()
			l.lineOnlyWhitespace = false
		case isDigit(c) || (c == '.' && isDigit(l.peek(1))):
			l.lexNumber()
			l.lineOnlyWhitespace = false
		case l.lineOnlyWhitespace && l.hasPrefix("#include") && l.includeBoundary():
			l.lexIncludeDirective()
			l.lineOnlyWhitespace = false
		case isIdentStart(c) || (c >= utf8.RuneSelf && l.validNonASCII()):
			l.lexIdentifier()
			l.lineOnlyWhitespace = false
		default:
			l.lexPunctOrInvalid()
			l.lineOnlyWhitespace = false
		}
	}
}

func (l *lexer) peek(off int) byte {
	i := l.offset + off
	if i < 0 || i >= len(l.text) {
		return 0
	}
	return l.text[i]
}

func (l *lexer) hasPrefix(prefix string) bool {
	return strings.HasPrefix(l.text[l.offset:], prefix)
}

func (l *lexer) includeBoundary() bool {
	i := l.offset + len("#include")
	if i >= len(l.text) {
		return true
	}
	c := l.text[i]
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

func (l *lexer) consumeNewline() {
	if l.text[l.offset] == '\r' && l.peek(1) == '\n' {
		l.offset += 2
	} else {
		l.offset++
	}
	l.line++
	l.lineStart = l.offset
	l.lineOnlyWhitespace = true
}

// advanceTo updates the line bookkeeping for text[from:to] without changing
// offset. It is used after scanning multi-line strings and comments.
func (l *lexer) advanceTo(from, to int) {
	for i := from; i < to; i++ {
		switch l.text[i] {
		case '\r':
			if i+1 < to && l.text[i+1] == '\n' {
				i++
			}
			l.line++
			l.lineStart = i + 1
		case '\n':
			l.line++
			l.lineStart = i + 1
		}
	}
}

func (l *lexer) addToken(kind TokenKind, start, end int, value string) {
	text := l.text[start:end]
	if value == "" {
		value = text
	}
	l.tokens = append(l.tokens, Token{
		Kind:   kind,
		Text:   text,
		Value:  value,
		Span:   l.mapper(start, end),
		Line:   l.tokenLine,
		Column: start - l.tokenLineStart,
	})
}

func (l *lexer) addError(start, end int, message string) {
	if l.tooManyErrors {
		return
	}
	if len(l.errors) >= maxLexErrors-1 {
		l.tooManyErrors = true
		l.errors = append(l.errors, Error{
			Span:    l.mapper(start, end),
			Message: "too many lexical errors; stopped reporting",
		})
		return
	}
	l.errors = append(l.errors, Error{
		Span:    l.mapper(start, end),
		Message: message,
	})
}

func (l *lexer) lexIdentifier() {
	start := l.offset
	for l.offset < len(l.text) {
		l.tokenLine = l.line
		l.tokenLineStart = l.lineStart
		c := l.text[l.offset]
		if isIdentContinue(c) {
			l.offset++
			continue
		}
		if c >= utf8.RuneSelf {
			r, size := utf8.DecodeRuneInString(l.text[l.offset:])
			if r == utf8.RuneError && size == 1 {
				break
			}
			l.offset += size
			continue
		}
		break
	}

	text := l.text[start:l.offset]
	if _, ok := keywords[strings.ToLower(text)]; ok {
		l.addToken(TokenKeyword, start, l.offset, strings.ToLower(text))
		return
	}
	l.addToken(TokenName, start, l.offset, text)
}

func (l *lexer) lexNumber() {
	start := l.offset

	if l.text[l.offset] == '0' && l.offset+1 < len(l.text) {
		switch l.text[l.offset+1] {
		case 'x', 'X':
			if isHex(l.peek(2)) {
				l.offset += 2
				l.scanDigits(isHex)
				if l.peek(0) == '.' && l.peek(1) != '.' {
					l.offset++
					l.scanDigits(isHex)
				}
				l.addToken(TokenNumber, start, l.offset, "")
				return
			}
		case 'b', 'B':
			if isBinary(l.peek(2)) {
				l.offset += 2
				l.scanDigits(isBinary)
				if l.peek(0) == '.' && l.peek(1) != '.' {
					l.offset++
					l.scanDigits(isBinary)
				}
				l.addToken(TokenNumber, start, l.offset, "")
				return
			}
		}
	}

	if l.text[l.offset] == '.' {
		l.offset++
		l.scanDigits(isDigit)
	} else {
		l.scanDigits(isDigit)
		if l.peek(0) == '.' && l.peek(1) != '.' {
			l.offset++
			l.scanDigits(isDigit)
		}
	}

	if l.peek(0) == 'e' || l.peek(0) == 'E' {
		next := l.peek(1)
		if isDigit(next) || ((next == '+' || next == '-') && isDigit(l.peek(2))) {
			l.offset++
			if l.peek(0) == '+' || l.peek(0) == '-' {
				l.offset++
			}
			l.scanDigits(isDigit)
		}
	}

	l.addToken(TokenNumber, start, l.offset, "")
}

func (l *lexer) scanDigits(accept func(byte) bool) {
	for l.offset < len(l.text) && accept(l.text[l.offset]) {
		l.offset++
	}
}

func (l *lexer) lexShortString() {
	start := l.offset
	quote := l.text[l.offset]
	l.offset++
	contentStart := l.offset

	for l.offset < len(l.text) {
		l.tokenLine = l.line
		l.tokenLineStart = l.lineStart
		c := l.text[l.offset]
		switch {
		case c == quote:
			value := l.text[contentStart:l.offset]
			l.offset++
			l.addToken(TokenString, start, l.offset, value)
			return
		case c == '\\':
			l.offset++
			if l.offset >= len(l.text) {
				l.addError(start, l.offset, "unterminated string")
				l.addToken(TokenString, start, l.offset, l.text[contentStart:l.offset])
				return
			}
			if l.text[l.offset] == '\r' || l.text[l.offset] == '\n' {
				l.consumeNewline()
			} else if l.text[l.offset] >= utf8.RuneSelf {
				_, size := utf8.DecodeRuneInString(l.text[l.offset:])
				l.offset += size
			} else {
				l.offset++
			}
		case c == '\r' || c == '\n':
			l.addError(start, l.offset, "unterminated string")
			l.addToken(TokenString, start, l.offset, l.text[contentStart:l.offset])
			return
		case c >= utf8.RuneSelf:
			r, size := utf8.DecodeRuneInString(l.text[l.offset:])
			if r == utf8.RuneError && size == 1 {
				l.addError(l.offset, l.offset+1, "invalid UTF-8 byte in string")
			}
			l.offset += size
		default:
			l.offset++
		}
	}

	l.addError(start, l.offset, "unterminated string")
	l.addToken(TokenString, start, l.offset, l.text[contentStart:l.offset])
}

func (l *lexer) lexLongString() {
	start := l.offset
	level := l.longBracketLevel(l.offset)
	contentStart := l.offset + level + 2
	close := "]" + strings.Repeat("=", level) + "]"
	idx := strings.Index(l.text[contentStart:], close)
	if idx < 0 {
		l.offset = len(l.text)
		l.advanceTo(start, l.offset)
		l.addError(start, l.offset, "unterminated long string")
		l.addToken(TokenString, start, l.offset, l.text[contentStart:l.offset])
		return
	}
	closeStart := contentStart + idx
	l.offset = closeStart + len(close)
	l.advanceTo(start, l.offset)
	l.addToken(TokenString, start, l.offset, l.text[contentStart:closeStart])
}

// longBracketLevel returns the number of '=' in [[, [=[, [==[, etc., or -1.
func (l *lexer) longBracketLevel(pos int) int {
	if pos >= len(l.text) || l.text[pos] != '[' {
		return -1
	}
	i := pos + 1
	for i < len(l.text) && l.text[i] == '=' {
		i++
	}
	if i < len(l.text) && l.text[i] == '[' {
		return i - pos - 1
	}
	return -1
}

func (l *lexer) lexDashComment() {
	start := l.offset
	if l.longBracketLevel(l.offset+2) >= 0 {
		levelStart := l.offset + 2
		level := l.longBracketLevel(levelStart)
		contentStart := levelStart + level + 2
		close := "]" + strings.Repeat("=", level) + "]"
		idx := strings.Index(l.text[contentStart:], close)
		if idx < 0 {
			l.offset = len(l.text)
			l.advanceTo(start, l.offset)
			l.addError(start, l.offset, "unterminated long comment")
		} else {
			l.offset = contentStart + idx + len(close)
			l.advanceTo(start, l.offset)
		}
		l.addComment(CommentBlock, start, l.offset)
		return
	}
	l.lexLineCommentText(start, CommentLine)
}

func (l *lexer) lexLineComment() {
	l.lexLineCommentText(l.offset, CommentLine)
}

func (l *lexer) lexLineCommentText(start int, kind CommentKind) {
	l.offset += 2
	for l.offset < len(l.text) {
		l.tokenLine = l.line
		l.tokenLineStart = l.lineStart
		c := l.text[l.offset]
		if c == '\r' || c == '\n' {
			break
		}
		l.offset++
	}
	text := l.text[start:l.offset]
	if strings.HasPrefix(text, "-->8") {
		kind = CommentTab
	} else if strings.HasPrefix(text, "---") && kind == CommentLine {
		kind = CommentDoc
	}
	l.addComment(kind, start, l.offset)
}

func (l *lexer) addComment(kind CommentKind, start, end int) {
	l.comments = append(l.comments, Comment{
		Kind: kind,
		Text: l.text[start:end],
		Span: l.mapper(start, end),
		Line: l.tokenLine,
	})
	if kind == CommentTab {
		l.tokens = append(l.tokens, Token{
			Kind:   TokenTabMarker,
			Text:   l.text[start:end],
			Value:  "",
			Span:   l.mapper(start, end),
			Line:   l.tokenLine,
			Column: start - l.tokenLineStart,
		})
	}
}

func (l *lexer) lexIncludeDirective() {
	start := l.offset
	lineEnd := start
	for lineEnd < len(l.text) && l.text[lineEnd] != '\r' && l.text[lineEnd] != '\n' {
		lineEnd++
	}
	path := strings.TrimSpace(l.text[start+len("#include") : lineEnd])
	if len(path) >= 2 && ((path[0] == '"' && path[len(path)-1] == '"') || (path[0] == '\'' && path[len(path)-1] == '\'')) {
		path = path[1 : len(path)-1]
	}
	l.offset = lineEnd
	l.addToken(TokenInclude, start, lineEnd, path)
}

func (l *lexer) lexPunctOrInvalid() {
	start := l.offset
	for _, op := range []string{
		">>>=", "<<>=", ">><=",
		"...", ">>>", "<<>", ">><", "..=", "^^=", "<<=", ">>=",
		"::", "..", "==", "~=", "!=", "<=", ">=", "+=", "-=", "*=", "/=",
		"\\=", "%=", "^=", "&=", "|=", "^^", "<<", ">>",
	} {
		if strings.HasPrefix(l.text[l.offset:], op) {
			l.offset += len(op)
			l.addToken(TokenPunct, start, l.offset, "")
			return
		}
	}

	switch l.text[l.offset] {
	case '+', '-', '*', '/', '\\', '%', '^', '#', '&', '|', '~',
		'<', '>', '=', '(', ')', '[', ']', '{', '}', ';', ',', '.', ':',
		'?', '@', '$':
		l.offset++
		l.addToken(TokenPunct, start, l.offset, "")
	default:
		c := l.text[l.offset]
		if c >= utf8.RuneSelf {
			r, size := utf8.DecodeRuneInString(l.text[l.offset:])
			if r == utf8.RuneError && size == 1 {
				l.addError(start, start+1, "invalid UTF-8 byte")
				l.offset++
				l.addToken(TokenInvalid, start, l.offset, "")
				return
			}
			// Valid non-ASCII characters are accepted as identifiers by the
			// main loop; reaching here means something unusual.
			l.offset += size
			l.addToken(TokenInvalid, start, l.offset, "")
			return
		}
		l.addError(start, start+1, "invalid character "+strconvQuoteByte(c))
		l.offset++
		l.addToken(TokenInvalid, start, l.offset, "")
	}
}

func (l *lexer) validNonASCII() bool {
	r, size := utf8.DecodeRuneInString(l.text[l.offset:])
	return !(r == utf8.RuneError && size == 1)
}

func strconvQuoteByte(c byte) string {
	if c >= 0x20 && c < 0x7f {
		return "'" + string(c) + "'"
	}
	return "0x" + hexByte(c)
}

func hexByte(c byte) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[c>>4], digits[c&0xf]})
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHex(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isBinary(c byte) bool { return c == '0' || c == '1' }

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '\x1e' || c == '\x1f'
}

func isIdentContinue(c byte) bool {
	return isIdentStart(c) || isDigit(c)
}
