package lsp

import (
	"context"
	"strings"
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

func TestSemanticTokensIncludePath(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib/player.lua\"\nprint(1)\n"
	root := writeTemp(t, dir, "cart.p8", text)
	writeTemp(t, dir, "lib/player.lua", "function make_player() end\n")
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	tokens := semanticTokens(t, h, ctx, root)
	path := findSemanticToken(t, tokens, text, protocol.PositionEncodingKindUTF16, "lib/player.lua", 0)
	if path.Kind != stString || path.Mods != smDocumentation {
		t.Fatalf("include path token = %+v, want string + documentation", path)
	}
	// The directive keyword itself must not be highlighted as a string.
	keywordPos := positionAt(t, text, "#include", 0)
	for _, token := range tokens {
		if token.Line == keywordPos.Line && token.Start == keywordPos.Character {
			t.Fatalf("unexpected token on #include keyword: %+v", token)
		}
	}
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

func TestSemanticTokensHighlightLuaCATSAnnotations(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@class Ship
---@field spd number
---@field [public] hp number
---@param dx? number horizontal delta
---@return boolean ok
---@return number, string
---@alias Mode "left" | "right"
---@cast speed +number
---@operator call(x: number): string
ship = {}

function ship:update(dx)
  self.spd = 1
end
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	tokens := semanticTokens(t, h, ctx, root)
	encoding := protocol.PositionEncodingKindUTF16

	tag := findSemanticTokenPart(t, tokens, text, encoding, "---@class Ship", 0, 3, len("@class"))
	if tag.Kind != stKeyword {
		t.Fatalf("@class kind = %d, want keyword", tag.Kind)
	}
	className := findSemanticTokenPart(t, tokens, text, encoding, "---@class Ship", 0, len("---@class "), len("Ship"))
	if className.Kind != stClass || className.Mods&smDeclaration == 0 {
		t.Fatalf("class name = %+v, want class + declaration", className)
	}
	field := findSemanticTokenPart(t, tokens, text, encoding, "---@field spd number", 0, len("---@field "), len("spd"))
	if field.Kind != stProperty {
		t.Fatalf("field name = %+v, want property", field)
	}
	fieldType := findSemanticTokenPart(t, tokens, text, encoding, "---@field spd number", 0, len("---@field spd "), len("number"))
	if fieldType.Kind != stType {
		t.Fatalf("field type = %+v, want type", fieldType)
	}
	scope := findSemanticTokenPart(t, tokens, text, encoding, "---@field [public] hp number", 0, len("---@field ["), len("public"))
	if scope.Kind != stModifier {
		t.Fatalf("field scope = %+v, want modifier", scope)
	}
	scopedField := findSemanticTokenPart(t, tokens, text, encoding, "---@field [public] hp number", 0, len("---@field [public] "), len("hp"))
	if scopedField.Kind != stProperty {
		t.Fatalf("scoped field = %+v, want property", scopedField)
	}
	secondReturn := findSemanticTokenPart(t, tokens, text, encoding, "---@return number, string", 0, len("---@return number, "), len("string"))
	if secondReturn.Kind != stType {
		t.Fatalf("second return type = %+v, want type", secondReturn)
	}
	param := findSemanticTokenPart(t, tokens, text, encoding, "---@param dx? number", 0, len("---@param "), len("dx?"))
	if param.Kind != stParameter {
		t.Fatalf("param name = %+v, want parameter", param)
	}
	paramType := findSemanticTokenPart(t, tokens, text, encoding, "---@param dx? number", 0, len("---@param dx? "), len("number"))
	if paramType.Kind != stType {
		t.Fatalf("param type = %+v, want type", paramType)
	}
	aliasName := findSemanticTokenPart(t, tokens, text, encoding, "---@alias Mode", 0, len("---@alias "), len("Mode"))
	if aliasName.Kind != stType {
		t.Fatalf("alias name = %+v, want type", aliasName)
	}
	aliasValue := findSemanticTokenPart(t, tokens, text, encoding, `---@alias Mode "left"`, 0, len(`---@alias Mode `), len(`"left"`))
	if aliasValue.Kind != stString {
		t.Fatalf("alias literal = %+v, want string", aliasValue)
	}
	castName := findSemanticTokenPart(t, tokens, text, encoding, "---@cast speed +number", 0, len("---@cast "), len("speed"))
	if castName.Kind != stVariable {
		t.Fatalf("cast target = %+v, want variable", castName)
	}
	callParam := findSemanticTokenPart(t, tokens, text, encoding, "---@operator call(x: number)", 0, len("---@operator call("), len("x"))
	if callParam.Kind != stParameter {
		t.Fatalf("call operator parameter = %+v, want parameter", callParam)
	}
	callParamType := findSemanticTokenPart(t, tokens, text, encoding, "---@operator call(x: number)", 0, len("---@operator call(x: "), len("number"))
	if callParamType.Kind != stType {
		t.Fatalf("call operator parameter type = %+v, want type", callParamType)
	}
	dashes := findSemanticTokenPart(t, tokens, text, encoding, "---@class Ship", 0, 0, len("---"))
	if dashes.Kind != stComment {
		t.Fatalf("annotation prefix = %+v, want comment", dashes)
	}

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

func TestIncludePathSpanSkipsIndentation(t *testing.T) {
	text := "  #include \"lib/x.lua\""
	got, ok := includePathSpan(text, source.Span{FileID: 0, Start: 0, End: len(text)})
	if !ok {
		t.Fatal("includePathSpan returned false")
	}
	wantStart := strings.Index(text, "lib/x.lua")
	wantEnd := wantStart + len("lib/x.lua")
	if got.Start != wantStart || got.End != wantEnd {
		t.Fatalf("include path span = %+v, want [%d,%d)", got, wantStart, wantEnd)
	}
}
