package analysis

import (
	"testing"

	"github.com/asimmons91/pico-lsp/internal/analysis/types"
)

func typeResolver(names map[string]types.Type) TypeResolver {
	return func(name string) (types.Type, bool) {
		typ, ok := names[CanonicalName(name)]
		return typ, ok
	}
}

func TestParseTypeGrammar(t *testing.T) {
	player := types.NewRecord("Player")
	player.AddField(&types.Field{Name: "x", Type: types.NumberType})
	names := map[string]types.Type{"player": player}
	resolve := typeResolver(names)
	cases := []struct {
		input string
		want  string
	}{
		{"number", "number"},
		{"number?", "number?"},
		{"number | string", "number | string"},
		{"number[]", "number[]"},
		{"table<string, number>", "table<string, number>"},
		{"{x: number, y?: string}", "{x: number, y?: string}"},
		{"fun(x: number, y?: string): boolean", "fun(x: number, y?: string): boolean"},
		{"[number, string]", "[number, string]"},
		{`"left" | "right"`, `"left" | "right"`},
		{"Player", "Player"},
		{"Player[]", "Player[]"},
	}
	for _, tc := range cases {
		got, err := parseType(tc.input, resolve)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.input, err)
		}
		if display := types.Display(got); display != tc.want {
			t.Fatalf("parse %q = %q, want %q", tc.input, display, tc.want)
		}
	}
}

func TestParseTypePrefixWithDescription(t *testing.T) {
	typeText, doc := splitTrailingType("fun(x: number): boolean whether it worked")
	if typeText != "fun(x: number): boolean" || doc != "whether it worked" {
		t.Fatalf("split = (%q, %q)", typeText, doc)
	}
	typeText, doc = splitTrailingType("Player the player")
	if typeText != "Player" || doc != "the player" {
		t.Fatalf("split player = (%q, %q)", typeText, doc)
	}
}

func TestParseTypeErrors(t *testing.T) {
	if _, err := parseType("Player", nil); err == nil {
		t.Fatal("unknown type was accepted")
	}
	if _, err := parseType("table<string", nil); err == nil {
		t.Fatal("malformed table type was accepted")
	}
	if _, err := parseType("fun(x: number", nil); err == nil {
		t.Fatal("malformed function type was accepted")
	}
	if _, err := parseType("{x number}", nil); err == nil {
		t.Fatal("malformed record type was accepted")
	}
}
