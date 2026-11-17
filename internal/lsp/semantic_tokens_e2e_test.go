package lsp

import (
	"context"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/source"
)

const m3SemanticCart = `pico-8 cartridge // pico-8.com
version 42
__lua__
-- comment
local player = {x = 10, name = "pico"}

function player:step(dx)
  self.x = self.x + dx
  circfill(self.x, 10, 2, 7)
  print(btn_left)
end

for i = 1, 10 do
  player:step(i)
end

---@deprecated
function old() end

::loop::
goto loop
`

type decodedSemanticToken struct {
	Line   uint32 `json:"line"`
	Start  uint32 `json:"start"`
	Length uint32 `json:"length"`
	Kind   uint32 `json:"kind"`
	Mods   uint32 `json:"mods"`
}

func decodeSemanticTokens(t *testing.T, data []uint32) []decodedSemanticToken {
	t.Helper()
	if len(data)%5 != 0 {
		t.Fatalf("semantic token data length = %d, want multiple of 5", len(data))
	}
	tokens := make([]decodedSemanticToken, 0, len(data)/5)
	line, start := uint32(0), uint32(0)
	for i := 0; i < len(data); i += 5 {
		line += data[i]
		if data[i] == 0 {
			start += data[i+1]
		} else {
			start = data[i+1]
		}
		tokens = append(tokens, decodedSemanticToken{
			Line:   line,
			Start:  start,
			Length: data[i+2],
			Kind:   data[i+3],
			Mods:   data[i+4],
		})
	}
	return tokens
}

func semanticTokens(t *testing.T, h *harness, ctx context.Context, u uri.URI) []decodedSemanticToken {
	t.Helper()
	result, err := h.server.SemanticTokensFull(ctx, &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
	})
	if err != nil {
		t.Fatalf("semantic tokens: %v", err)
	}
	return decodeSemanticTokens(t, result.Data)
}

func findSemanticToken(t *testing.T, tokens []decodedSemanticToken, text string, encoding protocol.PositionEncodingKind, needle string, occurrence int) decodedSemanticToken {
	t.Helper()
	return findSemanticTokenPart(t, tokens, text, encoding, needle, occurrence, 0, len(needle))
}

func findSemanticTokenPart(t *testing.T, tokens []decodedSemanticToken, text string, encoding protocol.PositionEncodingKind, needle string, occurrence, partOffset, partLength int) decodedSemanticToken {
	t.Helper()
	offset := -1
	search := 0
	for i := 0; i <= occurrence; i++ {
		next := indexFrom(text, needle, search)
		if next < 0 {
			t.Fatalf("needle %q occurrence %d not found", needle, occurrence)
		}
		offset = next
		search = next + len(needle)
	}
	offset += partOffset
	index := source.New([]byte(text), encoding)
	pos, err := index.Position(offset)
	if err != nil {
		t.Fatalf("position: %v", err)
	}
	endPos, err := index.Position(offset + partLength)
	if err != nil {
		t.Fatalf("end position: %v", err)
	}
	for _, token := range tokens {
		if token.Line != pos.Line || token.Start != pos.Character {
			continue
		}
		if token.Length != endPos.Character-pos.Character {
			t.Fatalf("token %q length = %d, want %d", needle, token.Length, endPos.Character-pos.Character)
		}
		return token
	}
	t.Fatalf("no semantic token for %q at %+v", needle, pos)
	return decodedSemanticToken{}
}

func indexFrom(text, needle string, start int) int {
	if start < 0 || start > len(text) {
		return -1
	}
	index := -1
	for i := start; i+len(needle) <= len(text); i++ {
		if text[i:i+len(needle)] == needle {
			index = i
			break
		}
	}
	return index
}

func TestSemanticTokenLegendIsStable(t *testing.T) {
	wantTypes := []string{
		"namespace", "type", "class", "enum", "interface", "struct", "typeParameter",
		"parameter", "variable", "property", "enumMember", "event", "function", "method",
		"macro", "keyword", "modifier", "comment", "string", "number", "regexp", "operator",
	}
	if len(semanticTokenTypes) != len(wantTypes) {
		t.Fatalf("token types = %v", semanticTokenTypes)
	}
	for i := range wantTypes {
		if semanticTokenTypes[i] != wantTypes[i] {
			t.Fatalf("token type %d = %q, want %q", i, semanticTokenTypes[i], wantTypes[i])
		}
	}
	wantModifiers := []string{
		"declaration", "definition", "readonly", "static", "deprecated", "defaultLibrary", "documentation",
	}
	if len(semanticTokenModifiers) != len(wantModifiers) {
		t.Fatalf("token modifiers = %v", semanticTokenModifiers)
	}
	for i := range wantModifiers {
		if semanticTokenModifiers[i] != wantModifiers[i] {
			t.Fatalf("token modifier %d = %q, want %q", i, semanticTokenModifiers[i], wantModifiers[i])
		}
	}
}

func TestEndToEndSemanticTokens(t *testing.T) {
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", m3SemanticCart)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, m3SemanticCart)

	tokens := semanticTokens(t, h, ctx, root)
	if len(tokens) == 0 {
		t.Fatal("no semantic tokens")
	}
	// Tokens must be sorted and must not overlap.
	for i := 1; i < len(tokens); i++ {
		prev := tokens[i-1]
		cur := tokens[i]
		if cur.Line < prev.Line {
			t.Fatalf("token line decreased: %+v then %+v", prev, cur)
		}
		if cur.Line == prev.Line && cur.Start < prev.Start+prev.Length {
			t.Fatalf("overlapping tokens: %+v then %+v", prev, cur)
		}
		if cur.Length == 0 {
			t.Fatalf("zero-length token: %+v", cur)
		}
	}

	encoding := protocol.PositionEncodingKindUTF16
	cases := []struct {
		needle     string
		occurrence int
		kind       uint32
		mods       uint32
	}{
		{needle: "-- comment", kind: stComment},
		{needle: `"pico"`, kind: stString},
		{needle: "function", kind: stKeyword},
		{needle: "local", kind: stKeyword},
		{needle: "player", occurrence: 0, kind: stVariable},
		{needle: "dx", occurrence: 0, kind: stParameter},
		{needle: "self", occurrence: 0, kind: stParameter},
		{needle: "step", occurrence: 1, kind: stMethod},
		{needle: "circfill", kind: stFunction, mods: smDefaultLibrary},
		{needle: "btn_left", kind: stVariable, mods: smDefaultLibrary},
		{needle: "old", kind: stFunction, mods: smDeprecated},
		{needle: "loop", occurrence: 0, kind: stNamespace, mods: smDeclaration},
	}
	for _, tc := range cases {
		token := findSemanticToken(t, tokens, m3SemanticCart, encoding, tc.needle, tc.occurrence)
		if token.Kind != tc.kind {
			t.Fatalf("%q kind = %d, want %d", tc.needle, token.Kind, tc.kind)
		}
		if token.Mods&tc.mods != tc.mods {
			t.Fatalf("%q modifiers = %b, want bits %b", tc.needle, token.Mods, tc.mods)
		}
	}
	// The table key is a property; find just the `x` in `x = 10`.
	property := findSemanticTokenPart(t, tokens, m3SemanticCart, encoding, "x = 10", 0, 0, len("x"))
	if property.Kind != stProperty {
		t.Fatalf("table key kind = %d, want property", property.Kind)
	}
	// Numbers and operators are represented.
	number := findSemanticToken(t, tokens, m3SemanticCart, encoding, "10", 0)
	if number.Kind != stNumber {
		t.Fatalf("number kind = %d", number.Kind)
	}
	operator := findSemanticToken(t, tokens, m3SemanticCart, encoding, "+", 0)
	if operator.Kind != stOperator {
		t.Fatalf("operator kind = %d", operator.Kind)
	}

	goldenJSON(t, "semantic/tokens.json", tokens)
}

func TestSemanticTokensUTF16AndUTF8Lengths(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nlocal s = \"é🙂\"\nprint(s)\n"
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)

	run := func(encoding protocol.PositionEncodingKind, wantLength uint32) {
		t.Helper()
		h := newHarness(t, newTestClient())
		params := &protocol.InitializeParams{}
		if encoding == protocol.PositionEncodingKindUTF8 {
			params.Capabilities.General = &protocol.GeneralClientCapabilities{
				PositionEncodings: []protocol.PositionEncodingKind{protocol.PositionEncodingKindUTF8},
			}
		}
		h.initialize(t, params)
		if got := h.srv.Encoding(); got != encoding {
			t.Fatalf("negotiated encoding = %q, want %q", got, encoding)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		openDoc(t, h, ctx, root, text)
		tokens := semanticTokens(t, h, ctx, root)
		for _, token := range tokens {
			if token.Kind != stString {
				continue
			}
			if token.Length != wantLength {
				t.Fatalf("%s string token length = %d, want %d", encoding, token.Length, wantLength)
			}
			return
		}
		t.Fatalf("%s: no string token", encoding)
	}
	run(protocol.PositionEncodingKindUTF16, 5)
	run(protocol.PositionEncodingKindUTF8, 8)
}
