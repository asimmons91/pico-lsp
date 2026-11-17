package lsp

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
)

func inlayHints(t *testing.T, h *harness, ctx context.Context, text string, r protocol.Range) []protocol.InlayHint {
	t.Helper()
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	openDoc(t, h, ctx, root, text)
	hints, err := h.srv.InlayHint(ctx, &protocol.InlayHintParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        r,
	})
	if err != nil {
		t.Fatalf("inlay hints: %v", err)
	}
	return hints
}

func initInlayHarness(t *testing.T, options string) *harness {
	t.Helper()
	h := newHarness(t, newTestClient())
	params := &protocol.InitializeParams{}
	if options != "" {
		params.InitializationOptions = protocol.LSPAny([]byte(options))
	}
	h.initialize(t, params)
	return h
}

func TestInlayHintsConfigurationAndRange(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
local x = 1
local s = "hi"
local t = {a = 1}
---@type number
local annotated = 1
`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t.Run("disabled by default", func(t *testing.T) {
		h := initInlayHarness(t, "")
		hints := inlayHints(t, h, ctx, text, protocol.Range{Start: protocol.Position{}, End: protocol.Position{Line: 100}})
		if len(hints) != 0 {
			t.Fatalf("hints = %+v, want none", hints)
		}
	})

	t.Run("inferred locals only", func(t *testing.T) {
		h := initInlayHarness(t, `{"pico8":{"inlayHints":true}}`)
		hints := inlayHints(t, h, ctx, text, protocol.Range{Start: protocol.Position{}, End: protocol.Position{Line: 100}})
		labels := make([]string, 0, len(hints))
		for _, hint := range hints {
			label, _ := hint.Label.(protocol.String)
			labels = append(labels, string(label))
		}
		joined := strings.Join(labels, " | ")
		for _, want := range []string{": number", ": string", ": {a: number}"} {
			if !strings.Contains(joined, want) {
				t.Fatalf("hints = %v, missing %q", labels, want)
			}
		}
		for _, hint := range hints {
			if hint.Position.Line == 7 {
				t.Fatalf("annotated local received a hint: %+v", hint)
			}
		}
	})

	t.Run("range limited", func(t *testing.T) {
		h := initInlayHarness(t, `{"pico8":{"inlayHints":true}}`)
		hints := inlayHints(t, h, ctx, text, protocol.Range{Start: protocol.Position{Line: 3}, End: protocol.Position{Line: 3, Character: 100}})
		if len(hints) != 1 || hints[0].Position.Line != 3 {
			t.Fatalf("range hints = %+v, want one on line 3", hints)
		}
	})

	t.Run("type checking off", func(t *testing.T) {
		h := initInlayHarness(t, `{"pico8":{"inlayHints":true,"typeChecking":"off"}}`)
		hints := inlayHints(t, h, ctx, text, protocol.Range{Start: protocol.Position{}, End: protocol.Position{Line: 100}})
		if len(hints) != 0 {
			t.Fatalf("off hints = %+v, want none", hints)
		}
	})
}

func TestTypeAwareMemberCompletion(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
local inferred = {x = 1, name = "pico"}
local check = inferred.x

---@class Player
---@field hp? integer
local p = {}
local ph = p.hp
function p:hit()
  local sh = self.hp
end

---@type {a: number} | {b: string}
local union = {}
local ua = union.a

---@type {a: number}?
local optionalUnion = {}
local oa = optionalUnion.a

local function unknown() end
local anyValue = unknown()
local anyUse = anyValue.field

local xs = {1, 2}
local xsUse = xs.foo
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	itemsAt := func(lineNeedle string, offset int) map[string]protocol.CompletionItem {
		pos := positionAt(t, text, lineNeedle, offset)
		items := completionItems(t, h, ctx, root, pos)
		out := make(map[string]protocol.CompletionItem)
		for _, item := range items {
			out[item.Label] = item
		}
		return out
	}

	fields := itemsAt("local check = inferred.x", len("local check = inferred."))
	if _, ok := fields["x"]; !ok {
		t.Fatalf("inferred fields = %v", fields)
	}
	if detail, _ := fields["x"].Detail.Get(); !strings.Contains(detail, "number") {
		t.Fatalf("x detail = %q", detail)
	}
	if _, ok := fields["name"]; !ok {
		t.Fatalf("inferred fields = %v", fields)
	}

	player := itemsAt("local ph = p.hp", len("local ph = p."))
	if _, ok := player["hp"]; !ok {
		t.Fatalf("class fields = %v", player)
	}
	if detail, _ := player["hp"].Detail.Get(); !strings.Contains(detail, "integer") || !strings.Contains(detail, "?") {
		t.Fatalf("hp detail = %q", detail)
	}

	self := itemsAt("local sh = self.hp", len("local sh = self."))
	if _, ok := self["hp"]; !ok {
		t.Fatalf("self fields = %v", self)
	}

	optionalFields := itemsAt("local oa = optionalUnion.a", len("local oa = optionalUnion."))
	if item, ok := optionalFields["a"]; !ok {
		t.Fatalf("optional union fields = %v", optionalFields)
	} else if detail, _ := item.Detail.Get(); strings.Contains(detail, "ambiguous") {
		t.Fatalf("optional nil member marked field ambiguous: %q", detail)
	}

	anyFields := itemsAt("local anyUse = anyValue.field", len("local anyUse = anyValue."))
	if len(anyFields) != 0 {
		t.Fatalf("any member completion = %v, want empty", anyFields)
	}
	arrayFields := itemsAt("local xsUse = xs.foo", len("local xsUse = xs."))
	if len(arrayFields) != 0 {
		t.Fatalf("array member completion = %v, want empty", arrayFields)
	}

	union := itemsAt("local ua = union.a", len("local ua = union."))
	for _, name := range []string{"a", "b"} {
		item, ok := union[name]
		if !ok {
			t.Fatalf("union fields = %v", union)
		}
		if detail, _ := item.Detail.Get(); !strings.Contains(detail, "ambiguous") {
			t.Fatalf("%s detail = %q", name, detail)
		}
	}
}

func TestTypeAnnotationCompletionIncludesAliasesAndEnums(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@alias Mode "left" | "right"
---@enum Direction
local Direction = {up = 1}
---@type M
local m = "left"
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	pos := positionAt(t, text, "---@type M", len("---@type M"))
	items := completionItems(t, h, ctx, root, pos)
	if findCompletion(items, "Mode") == nil {
		t.Fatalf("annotation completion = %v, missing Mode", completionLabels(items))
	}
	pos = positionAt(t, text, "---@type M", len("---@type "))
	items = completionItems(t, h, ctx, root, pos)
	if findCompletion(items, "Direction") == nil {
		t.Fatalf("annotation completion = %v, missing Direction", completionLabels(items))
	}
}

func TestTypeAwareHover(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@class Player
---@field x number
---@field hp integer

---@param p Player
---@param dx number
---@return boolean moved
local function move(p, dx)
  p.x += dx
  return dx != 0
end

---@type Player
local player = {x = 64, hp = 3}
move(player, 1)
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	moveHover := hoverMarkup(t, h, ctx, root, positionAt(t, text, "move(player", 1))
	for _, want := range []string{"function move(p: Player, dx: number): boolean", "Returns:"} {
		if !strings.Contains(moveHover, want) {
			t.Fatalf("move hover missing %q:\n%s", want, moveHover)
		}
	}
	playerHover := hoverMarkup(t, h, ctx, root, positionAt(t, text, "move(player", len("move(")+1))
	if !strings.Contains(playerHover, "**Type:** `Player`") || !strings.Contains(playerHover, "**Fields:**") {
		t.Fatalf("player hover:\n%s", playerHover)
	}
	fieldHover := hoverMarkup(t, h, ctx, root, positionAt(t, text, "p.x += dx", len("p.")+1))
	if !strings.Contains(fieldHover, "**Type:** `number`") {
		t.Fatalf("field hover:\n%s", fieldHover)
	}
}

func TestTypedSignatureHelpOptionalAndVariadic(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@param x number
---@param y? string
function f(x, y) end
f(1, "a", true)
poke(0x5f00, 1, 2, 3, 4)
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	pos := positionAt(t, text, "f(1, \"a\", true)", len("f(1, \"a\", "))
	help := signatureHelp(t, h, ctx, root, pos)
	if help == nil {
		t.Fatal("missing user signature help")
	}
	if got := help.Signatures[0].Label; got != "function f(x: number, y?: string)" {
		t.Fatalf("user label = %q", got)
	}
	if active := activeParameter(t, help); active != 1 {
		t.Fatalf("user active = %d, want 1", active)
	}

	pos = positionAt(t, text, "poke(0x5f00, 1, 2, 3, 4)", len("poke(0x5f00, 1, 2, 3, "))
	help = signatureHelp(t, h, ctx, root, pos)
	if help == nil {
		t.Fatal("missing builtin signature help")
	}
	if got := help.Signatures[0].Label; !strings.Contains(got, "...: number") {
		t.Fatalf("poke label = %q", got)
	}
	if len(help.Signatures[0].Parameters) < 2 {
		t.Fatalf("poke parameters = %+v", help.Signatures[0].Parameters)
	}
	if active := activeParameter(t, help); active != 3 {
		t.Fatalf("poke active = %d, want 3", active)
	}
}

func TestTypeCheckingOffKeepsExplicitTypes(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@class Player
---@field hp integer
---@type Player
local p = {}
local value = p.hp
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := initializeWithOptions(t, `{"pico8":{"typeChecking":"off"}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	pos := positionAt(t, text, "local value = p.hp", len("local value = p."))
	items := completionItems(t, h, ctx, root, pos)
	if findCompletion(items, "hp") == nil {
		t.Fatalf("off completion = %v", completionLabels(items))
	}
	hover := hoverMarkup(t, h, ctx, root, positionAt(t, text, "local value = p.hp", len("local value = ")+1))
	if !strings.Contains(hover, "**Type:** `Player`") {
		t.Fatalf("off hover:\n%s", hover)
	}
	hints, err := h.srv.InlayHint(ctx, &protocol.InlayHintParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        protocol.Range{Start: protocol.Position{}, End: protocol.Position{Line: 100}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hints) != 0 {
		t.Fatalf("off hints = %+v", hints)
	}
}
