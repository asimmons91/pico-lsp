package cart

import (
	"testing"

	"github.com/asimmons91/pico-lsp/internal/syntax"
)

func countText(text string) (int, int) {
	res := syntax.Lex([]byte(text), nil)
	return CountTokens(res.Tokens), CountChars([]byte(text))
}

func TestCountTokens(t *testing.T) {
	tests := []struct {
		code string
		want int
	}{
		{"", 0},
		{"a=1", 3},
		{"local a=1", 3},
		{"a+=1", 3},
		{"a=a+1", 5},
		{"print(\"hi\")", 3},
		{"x.y()", 3},
		{"t={1,2,3}", 6},
		{"a,b=1,2", 5},
		{"if (x) y=1 end", 6},
		{"if x then y=1 end", 6},
		{"?x", 2},
		{"?x,y", 3},
		{"a=-1", 3},
		{"a=~1", 3},
		{"a=-b", 4},
		{"a=~b", 4},
		{"a=@b", 4},
		{"a=b%2", 5},
		{"a=#t", 4},
		{"a..b", 3},
		{"-- comment", 0},
		{"--[[ block ]]", 0},
		{"---@param x number", 0},
		{"a=1 // comment", 3},
		{"(a)", 2},
		{"f()", 2},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got, _ := countText(tt.code)
			if got != tt.want {
				t.Fatalf("CountTokens(%q) = %d, want %d", tt.code, got, tt.want)
			}
		})
	}
}

func TestCountTokensCompoundAssignmentMatchesRoundNumber(t *testing.T) {
	compound, _ := countText("a+=1")
	plain, _ := countText("a=1")
	if compound != plain {
		t.Fatalf("compound = %d, plain = %d; want one operator token", compound, plain)
	}
}

func TestCountCharsUnicode(t *testing.T) {
	_, chars := countText("a⬅️b")
	if chars != 3 {
		t.Fatalf("chars = %d, want 3 (variation selector counts once with its glyph)", chars)
	}
	_, chars = countText("🐱🐱")
	if chars != 2 {
		t.Fatalf("chars = %d, want 2", chars)
	}
	_, chars = countText("a\r\nb\rc")
	if chars != 5 {
		t.Fatalf("chars = %d, want 5 (CRLF/CR normalize to one newline each)", chars)
	}
}
