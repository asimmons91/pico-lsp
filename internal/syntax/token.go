// Package syntax contains the PICO-8 lexer, AST, and parser.
package syntax

import (
	"strings"

	"github.com/asimmons91/pico-lsp/internal/source"
)

// TokenKind classifies a lexical token.
type TokenKind int

const (
	TokenEOF TokenKind = iota
	TokenInvalid
	TokenName
	TokenKeyword
	TokenNumber
	TokenString
	TokenPunct
	TokenComment
	TokenInclude
)

// String returns a stable name for diagnostics and tests.
func (k TokenKind) String() string {
	switch k {
	case TokenEOF:
		return "eof"
	case TokenInvalid:
		return "invalid"
	case TokenName:
		return "name"
	case TokenKeyword:
		return "keyword"
	case TokenNumber:
		return "number"
	case TokenString:
		return "string"
	case TokenPunct:
		return "punct"
	case TokenComment:
		return "comment"
	case TokenInclude:
		return "include"
	default:
		return "unknown"
	}
}

// Token is one lexed token. Text retains the original source spelling.
type Token struct {
	Kind TokenKind
	Text string
	// Value is the unescaped inner text for strings and the literal spelling for
	// other tokens. Long-string delimiters are excluded from Value.
	Value  string
	Span   source.Span
	Line   int
	Column int
}

// Is reports whether the token is punctuation or a keyword with the given
// spelling.
func (t Token) Is(text string) bool {
	if t.Kind == TokenKeyword {
		return strings.EqualFold(t.Text, text)
	}
	return t.Kind == TokenPunct && t.Text == text
}

// CommentKind classifies comments for folding and doc-comment indexing.
type CommentKind int

const (
	CommentLine CommentKind = iota
	CommentBlock
	CommentDoc
	CommentTab
)

// Comment is preserved trivia, ordered by source position.
type Comment struct {
	Kind CommentKind
	Text string
	Span source.Span
	Line int
}

// Error is a structured lexical or parse error. The LSP layer turns it into a
// diagnostic with the syntax code.
type Error struct {
	Span    source.Span
	Message string
}

// DiagnosticCode is the stable LSP code for syntax errors.
const DiagnosticCode = "syntax"

// Result is the output of a lexer run.
type Result struct {
	Tokens   []Token
	Comments []Comment
	Errors   []Error
}
