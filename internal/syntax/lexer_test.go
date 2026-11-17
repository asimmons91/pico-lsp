package syntax

import (
	"testing"

	"github.com/asimmons91/pico-lsp/internal/source"
)

func lex(t *testing.T, text string) Result {
	t.Helper()
	return Lex([]byte(text), nil)
}

func tokenSignature(tokens []Token) []string {
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		out = append(out, tok.Kind.String()+":"+tok.Text)
	}
	return out
}

func TestLexIdentifiersKeywordsAndGlyphs(t *testing.T) {
	res := lex(t, "local Player = _init + btn(➡️) IF then")
	want := []string{
		"keyword:local",
		"name:Player",
		"punct:=",
		"name:_init",
		"punct:+",
		"name:btn",
		"punct:(",
		"name:➡️",
		"punct:)",
		"keyword:IF",
		"keyword:then",
		"eof:",
	}
	got := tokenSignature(res.Tokens)
	if len(got) != len(want) {
		t.Fatalf("tokens = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d = %q, want %q", i, got[i], want[i])
		}
	}
	if len(res.Errors) != 0 {
		t.Fatalf("errors = %v", res.Errors)
	}
	if !res.Tokens[0].Is("LOCAL") {
		t.Fatal("keyword comparison should be case-insensitive")
	}
	if res.Tokens[1].Value != "Player" {
		t.Fatalf("identifier value = %q, want original spelling", res.Tokens[1].Value)
	}
}

func TestLexNumbers(t *testing.T) {
	tests := []string{
		"0", "123", "1.5", ".5", "1.", "1e3", "1E-3", "1e+3",
		"0xff", "0xFF", "0x11.4000", "0b1010", "0b1.01",
		"1..2", "1...2",
	}
	for _, text := range tests {
		t.Run(text, func(t *testing.T) {
			res := lex(t, text)
			if len(res.Errors) != 0 {
				t.Fatalf("errors = %v", res.Errors)
			}
			if res.Tokens[0].Kind != TokenNumber {
				t.Fatalf("kind = %v, want number", res.Tokens[0].Kind)
			}
		})
	}
}

func TestLexNumberSeparatedFromConcat(t *testing.T) {
	res := lex(t, "1..2")
	if len(res.Tokens) != 4 {
		t.Fatalf("tokens = %v", tokenSignature(res.Tokens))
	}
	if res.Tokens[0].Text != "1" || !res.Tokens[1].Is("..") || res.Tokens[2].Text != "2" {
		t.Fatalf("tokens = %v", tokenSignature(res.Tokens))
	}
}

func TestLexStrings(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		value string
	}{
		{name: "double", text: `"hello"`, value: "hello"},
		{name: "single", text: `'hello'`, value: "hello"},
		{name: "escapes", text: `"a\n\t\*"`, value: `a\n\t\*`},
		{name: "long", text: "[[hello\nworld]]", value: "hello\nworld"},
		{name: "long level", text: "[=[a]]b]=]", value: "a]]b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := lex(t, tt.text)
			if len(res.Errors) != 0 {
				t.Fatalf("errors = %v", res.Errors)
			}
			if res.Tokens[0].Kind != TokenString || res.Tokens[0].Value != tt.value {
				t.Fatalf("token = %+v, want string value %q", res.Tokens[0], tt.value)
			}
		})
	}
}

func TestLexUnterminatedStrings(t *testing.T) {
	for _, text := range []string{`"abc`, "[[abc", "[=[abc]]"} {
		t.Run(text, func(t *testing.T) {
			res := lex(t, text)
			if len(res.Errors) == 0 {
				t.Fatal("expected a lexical error")
			}
			if res.Tokens[0].Kind != TokenString {
				t.Fatalf("kind = %v, want string", res.Tokens[0].Kind)
			}
			if res.Tokens[0].Span.End != len(text) && text != `"abc` {
				t.Fatalf("span = %+v, want end %d", res.Tokens[0].Span, len(text))
			}
		})
	}
}

func TestLexComments(t *testing.T) {
	tests := []struct {
		text string
		kind CommentKind
	}{
		{"-- line", CommentLine},
		{"// line", CommentLine},
		{"---@class Foo", CommentDoc},
		{"-->8 tab", CommentTab},
		{"--[[block]]", CommentBlock},
		{"--[=[block]=]", CommentBlock},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			res := lex(t, tt.text)
			if len(res.Errors) != 0 {
				t.Fatalf("errors = %v", res.Errors)
			}
			if len(res.Comments) != 1 || res.Comments[0].Kind != tt.kind {
				t.Fatalf("comments = %+v, want kind %v", res.Comments, tt.kind)
			}
			wantTokens := 1
			if tt.kind == CommentTab {
				wantTokens = 2
				if res.Tokens[0].Kind != TokenTabMarker {
					t.Fatalf("first token = %v, want tab marker", res.Tokens[0].Kind)
				}
			}
			if len(res.Tokens) != wantTokens {
				t.Fatalf("tokens = %v, want %d", tokenSignature(res.Tokens), wantTokens)
			}
			if res.Tokens[len(res.Tokens)-1].Kind != TokenEOF {
				t.Fatal("token stream does not end with EOF")
			}
		})
	}
}

func TestLexSlashSlashIsCommentNotDivision(t *testing.T) {
	res := lex(t, "a // b\nc/d")
	if len(res.Comments) != 1 {
		t.Fatalf("comments = %+v, want one", res.Comments)
	}
	// a, c, /, d
	if len(res.Tokens) != 5 {
		t.Fatalf("tokens = %v", tokenSignature(res.Tokens))
	}
	if !res.Tokens[2].Is("/") {
		t.Fatalf("token = %q, want division", res.Tokens[2].Text)
	}
}

func TestLexOperatorsMaximalMunch(t *testing.T) {
	text := "... ..= .. . :: != ~= ^^ <<> >>< >>> <<>= >><= >>>= += -= *= /= \\= %= ^= &= |= <<= >>= == <= >="
	res := lex(t, text)
	want := []string{
		"punct:...", "punct:..=", "punct:..", "punct:.", "punct:::",
		"punct:!=", "punct:~=", "punct:^^", "punct:<<>", "punct:>><",
		"punct:>>>", "punct:<<>=", "punct:>><=", "punct:>>>=", "punct:+=",
		"punct:-=", "punct:*=", "punct:/=", "punct:\\=", "punct:%=",
		"punct:^=", "punct:&=", "punct:|=", "punct:<<=", "punct:>>=",
		"punct:==", "punct:<=", "punct:>=", "eof:",
	}
	got := tokenSignature(res.Tokens)
	if len(got) != len(want) {
		t.Fatalf("tokens = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLexIncludeDirective(t *testing.T) {
	res := lex(t, "  #include \"lib/player.lua\"\nprint(1)")
	if len(res.Tokens) != 6 { // include, print, (, 1, ), eof
		t.Fatalf("tokens = %v", tokenSignature(res.Tokens))
	}
	inc := res.Tokens[0]
	if inc.Kind != TokenInclude || inc.Value != "lib/player.lua" {
		t.Fatalf("include token = %+v", inc)
	}
	if inc.Line != 0 || inc.Column != 2 {
		t.Fatalf("line/column = %d/%d, want 0/2", inc.Line, inc.Column)
	}
}

func TestLexIncludeOnlyAtLineStart(t *testing.T) {
	res := lex(t, "x = #include")
	if len(res.Errors) != 0 {
		t.Fatalf("errors = %v", res.Errors)
	}
	if res.Tokens[0].Kind == TokenInclude {
		t.Fatal("include token recognized mid-line")
	}
}

func TestLexLineAndColumn(t *testing.T) {
	res := lex(t, "a\r\nb\rc\nd")
	lines := []int{}
	for _, tok := range res.Tokens {
		if tok.Kind != TokenEOF {
			lines = append(lines, tok.Line)
		}
	}
	want := []int{0, 1, 2, 3}
	if len(lines) != len(want) {
		t.Fatalf("lines = %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("lines = %v, want %v", lines, want)
		}
	}
}

func TestLexMappedSpans(t *testing.T) {
	mapper := func(start, end int) source.Span {
		return source.Span{FileID: 3, Start: start + 100, End: end + 100}
	}
	res := Lex([]byte("abc"), mapper)
	if res.Tokens[0].Span != (source.Span{FileID: 3, Start: 100, End: 103}) {
		t.Fatalf("span = %+v", res.Tokens[0].Span)
	}
	if res.Tokens[1].Span != (source.Span{FileID: 3, Start: 103, End: 103}) {
		t.Fatalf("eof span = %+v", res.Tokens[1].Span)
	}
}

func TestLexInvalidBytes(t *testing.T) {
	res := Lex([]byte{0xff, 'a'}, nil)
	if len(res.Errors) == 0 {
		t.Fatal("expected invalid-byte error")
	}
	if res.Tokens[0].Kind != TokenInvalid {
		t.Fatalf("token = %+v, want invalid", res.Tokens[0])
	}
}

func TestLexLineNumbersInMultilineTokens(t *testing.T) {
	res := lex(t, "[[a\nb]]\nc")
	if res.Tokens[0].Line != 0 {
		t.Fatalf("long string line = %d, want 0", res.Tokens[0].Line)
	}
	if res.Tokens[1].Line != 2 {
		t.Fatalf("next token line = %d, want 2", res.Tokens[1].Line)
	}
}
