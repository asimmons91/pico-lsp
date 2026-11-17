package lsp

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

const m3SignatureCart = `pico-8 cartridge // pico-8.com
version 42
__lua__
--- Move the player.
---@param dx number
---@param dy? number
function move(dx, dy) end

local player = {x = 1}
function player:step(dx) end

circfill(10, 20, 30, 7)
circfill(10, max(1, 2), "a,b", {x = 1, y = 2}, 9)
move(1, 2)
move(1, -- ignored, comma
  2)
player:step(1)
move(
`

func signatureHelp(t *testing.T, h *harness, ctx context.Context, u uri.URI, position protocol.Position) *protocol.SignatureHelp {
	t.Helper()
	help, err := h.server.SignatureHelp(ctx, &protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     position,
		},
	})
	if err != nil {
		t.Fatalf("signature help: %v", err)
	}
	return help
}

func activeParameter(t *testing.T, help *protocol.SignatureHelp) uint32 {
	t.Helper()
	if help == nil {
		t.Fatal("signature help is nil")
	}
	value, ok := help.ActiveParameter.Get()
	if !ok {
		t.Fatal("activeParameter is absent")
	}
	return value
}

func parameterLabel(t *testing.T, param protocol.ParameterInformation) string {
	t.Helper()
	switch label := param.Label.(type) {
	case protocol.String:
		return string(label)
	default:
		t.Fatalf("parameter label type = %T", param.Label)
		return ""
	}
}

func goldenSignature(help *protocol.SignatureHelp) any {
	if help == nil || len(help.Signatures) == 0 {
		return nil
	}
	signature := help.Signatures[0]
	type goldenParameter struct {
		Label         string `json:"label"`
		Documentation string `json:"documentation,omitempty"`
	}
	out := struct {
		Label           string            `json:"label"`
		Parameters      []goldenParameter `json:"parameters"`
		Documentation   string            `json:"documentation,omitempty"`
		ActiveSignature uint32            `json:"activeSignature"`
		ActiveParameter *uint32           `json:"activeParameter,omitempty"`
	}{
		Label:         signature.Label,
		Documentation: markupText(signature.Documentation),
	}
	if help.ActiveSignature != nil {
		out.ActiveSignature = *help.ActiveSignature
	}
	if value, ok := help.ActiveParameter.Get(); ok {
		out.ActiveParameter = &value
	}
	for _, param := range signature.Parameters {
		label := ""
		if param.Label != nil {
			if s, ok := param.Label.(protocol.String); ok {
				label = string(s)
			}
		}
		out.Parameters = append(out.Parameters, goldenParameter{
			Label:         label,
			Documentation: markupText(param.Documentation),
		})
	}
	return out
}

func markupText(value any) string {
	switch v := value.(type) {
	case *protocol.MarkupContent:
		return v.Value
	case protocol.String:
		return string(v)
	default:
		return ""
	}
}

func TestSignatureHelpCallbackBodyComma(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\ncircfill(function() return 1, 2 end, 2)\n"
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	// The cursor is on the comma inside the callback body. It is not an
	// argument separator of circfill, so the active parameter stays at 0.
	pos := positionAt(t, text, "return 1, 2", len("return 1,"))
	help := signatureHelp(t, h, ctx, root, pos)
	if help == nil {
		t.Fatal("signature help is nil")
	}
	if active := activeParameter(t, help); active != 0 {
		t.Fatalf("active parameter = %d, want 0 (callback comma ignored)", active)
	}
}

func TestEndToEndSignatureHelp(t *testing.T) {
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", m3SignatureCart)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, m3SignatureCart)

	// Built-in signature and optional label.
	pos := positionAt(t, m3SignatureCart, "circfill(10, 20, 30, 7)", len("circfill(10, 20, "))
	help := signatureHelp(t, h, ctx, root, pos)
	if help == nil || len(help.Signatures) != 1 {
		t.Fatalf("signature help = %+v", help)
	}
	label := help.Signatures[0].Label
	if label != "function circfill(x, y, r, [col])" {
		t.Fatalf("builtin signature = %q", label)
	}
	if active := activeParameter(t, help); active != 2 {
		t.Fatalf("builtin active parameter = %d, want 2", active)
	}
	if len(help.Signatures[0].Parameters) != 4 {
		t.Fatalf("builtin parameters = %+v", help.Signatures[0].Parameters)
	}
	for _, param := range help.Signatures[0].Parameters {
		if !strings.Contains(label, parameterLabel(t, param)) {
			t.Fatalf("parameter %q is not a substring of %q", parameterLabel(t, param), label)
		}
	}
	goldenJSON(t, "signature/builtin.json", goldenSignature(help))

	// Active parameter clamps to the last declared parameter.
	pos = positionAt(t, m3SignatureCart, "circfill(10, max(1, 2), \"a,b\", {x = 1, y = 2}, 9)", len(`circfill(10, max(1, 2), "a,b", {x = 1, y = 2}, 9`))
	help = signatureHelp(t, h, ctx, root, pos)
	if active := activeParameter(t, help); active != 3 {
		t.Fatalf("nested active parameter = %d, want clamped 3", active)
	}
	goldenJSON(t, "signature/nested.json", goldenSignature(help))

	// The innermost call wins.
	pos = positionAt(t, m3SignatureCart, "max(1, 2)", len("max(1, "))
	help = signatureHelp(t, h, ctx, root, pos)
	if help == nil || help.Signatures[0].Label != "function max(x, y)" {
		t.Fatalf("nested call signature = %+v", help)
	}
	if active := activeParameter(t, help); active != 1 {
		t.Fatalf("nested max active parameter = %d, want 1", active)
	}

	// User function with documented parameters.
	pos = positionAt(t, m3SignatureCart, "move(1, 2)", len("move(1, "))
	help = signatureHelp(t, h, ctx, root, pos)
	if help == nil {
		t.Fatal("user signature help is nil")
	}
	if got := help.Signatures[0].Label; got != "function move(dx, dy)" {
		t.Fatalf("user signature = %q", got)
	}
	if active := activeParameter(t, help); active != 1 {
		t.Fatalf("user active parameter = %d, want 1", active)
	}
	paramDoc := markupText(help.Signatures[0].Parameters[1].Documentation)
	if !strings.Contains(paramDoc, "number") {
		t.Fatalf("dy documentation = %q", paramDoc)
	}
	goldenJSON(t, "signature/user.json", goldenSignature(help))

	// Commas inside comments do not advance the active parameter.
	pos = positionAt(t, m3SignatureCart, "move(1, -- ignored, comma\n  2)", len("move(1, -- ignored, comma\n  2"))
	help = signatureHelp(t, h, ctx, root, pos)
	if help == nil {
		t.Fatal("comment signature help is nil")
	}
	if active := activeParameter(t, help); active != 1 {
		t.Fatalf("comment active parameter = %d, want 1", active)
	}

	// Method calls show the declaration signature without implicit self.
	pos = positionAt(t, m3SignatureCart, "player:step(1)", len("player:step("))
	help = signatureHelp(t, h, ctx, root, pos)
	if help == nil {
		t.Fatal("method signature help is nil")
	}
	if got := help.Signatures[0].Label; got != "function player:step(dx)" {
		t.Fatalf("method signature = %q", got)
	}
	if active := activeParameter(t, help); active != 0 {
		t.Fatalf("method active parameter = %d, want 0", active)
	}
	goldenJSON(t, "signature/method.json", goldenSignature(help))

	// Incomplete call still uses the partial parse.
	pos = positionAt(t, m3SignatureCart, "\nmove(\n", len("\nmove("))
	help = signatureHelp(t, h, ctx, root, pos)
	if help == nil {
		t.Fatal("incomplete signature help is nil")
	}
	if got := help.Signatures[0].Label; got != "function move(dx, dy)" {
		t.Fatalf("incomplete signature = %q", got)
	}
	if active := activeParameter(t, help); active != 0 {
		t.Fatalf("incomplete active parameter = %d, want 0", active)
	}
	goldenJSON(t, "signature/incomplete.json", goldenSignature(help))

	// Outside a call there is no signature help.
	if help := signatureHelp(t, h, ctx, root, positionAt(t, m3SignatureCart, "local player", len("local "))); help != nil {
		t.Fatalf("signature help outside call = %+v", help)
	}
}
