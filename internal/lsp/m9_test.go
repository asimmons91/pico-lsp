package lsp

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
)

func TestM9InheritedMembersInLSP(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@class Entity
---@field x number
---@field hp number
local Entity = {}
---@param amount number
function Entity:heal(amount)
  self.hp += amount
end

---@class Enemy: Entity
---@field damage number
local Enemy = {}
local e = setmetatable({}, {__index = Enemy})
local c = setmetatable({}, {__index = Enemy})
local ex = e.x
local cx = c.damage
e:heal(2)
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := initializeWithOptions(t, `{"pico8":{"typeChecking":"basic","inlayHints":true}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	items := completionItems(t, h, ctx, root, positionAt(t, text, "local cx = c.", len("local cx = c.")))
	byLabel := make(map[string]protocol.CompletionItem)
	for _, item := range items {
		byLabel[item.Label] = item
	}
	for _, want := range []string{"x", "hp", "damage", "heal"} {
		if _, ok := byLabel[want]; !ok {
			t.Fatalf("inherited completion missing %q in %v", want, completionLabels(items))
		}
	}
	if detail, _ := byLabel["x"].Detail.Get(); !strings.Contains(detail, "from Entity") {
		t.Fatalf("x detail = %q, want declaring class", detail)
	}
	if detail, _ := byLabel["heal"].Detail.Get(); !strings.Contains(detail, "from Entity") {
		t.Fatalf("heal detail = %q, want declaring class", detail)
	}
	if detail, _ := byLabel["damage"].Detail.Get(); strings.Contains(detail, "from ") {
		t.Fatalf("own field damage reported as inherited: %q", detail)
	}

	usedItems := completionItems(t, h, ctx, root, positionAt(t, text, "local ex = e.", len("local ex = e.")))
	usedByLabel := make(map[string]protocol.CompletionItem)
	for _, item := range usedItems {
		usedByLabel[item.Label] = item
	}
	if detail, _ := usedByLabel["x"].Detail.Get(); !strings.Contains(detail, "from Entity") {
		t.Fatalf("accessed inherited field x detail = %q", detail)
	}
	if detail, _ := usedByLabel["heal"].Detail.Get(); !strings.Contains(detail, "from Entity") {
		t.Fatalf("accessed inherited method heal detail = %q", detail)
	}

	hover := hoverMarkup(t, h, ctx, root, positionAt(t, text, "local ex = e.x", len("local ex = e.")+1))
	if !strings.Contains(hover, "**Type:** `number`") || !strings.Contains(hover, "Inherited from `Entity`") {
		t.Fatalf("inherited field hover:\n%s", hover)
	}

	help := signatureHelp(t, h, ctx, root, positionAt(t, text, "e:heal(2)", len("e:heal(")))
	if help == nil {
		t.Fatal("missing inherited method signature help")
	}
	if label := help.Signatures[0].Label; !strings.Contains(label, "function heal(") || !strings.Contains(label, "amount: number") {
		t.Fatalf("inherited method signature = %q", label)
	}

	hints := inlayHints(t, h, ctx, text, protocol.Range{Start: protocol.Position{}, End: protocol.Position{Line: 100}})
	labels := make([]string, 0, len(hints))
	for _, hint := range hints {
		label, _ := hint.Label.(protocol.String)
		labels = append(labels, string(label))
	}
	joined := strings.Join(labels, " | ")
	if !strings.Contains(joined, ": Enemy") {
		t.Fatalf("inlay hints = %v, missing metatable class", labels)
	}
}

func TestM9HoverAndHintsUseNarrowedTypes(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@class Player
---@field hp number

---@param p Player?
function f(p)
  local before = p
  assert(p)
  local after = p
  local hp = p.hp
  if not p then return end
  local guarded = p
end
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := initializeWithOptions(t, `{"pico8":{"typeChecking":"basic","inlayHints":true}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	beforeHover := hoverMarkup(t, h, ctx, root, positionAt(t, text, "local before = p", len("local before = ")+1))
	if !strings.Contains(beforeHover, "**Type:** `Player?`") {
		t.Fatalf("before hover:\n%s", beforeHover)
	}
	afterHover := hoverMarkup(t, h, ctx, root, positionAt(t, text, "local after = p", len("local after = ")+1))
	if !strings.Contains(afterHover, "**Type:** `Player`") || strings.Contains(afterHover, "Player?") {
		t.Fatalf("after hover:\n%s", afterHover)
	}
	guardedHover := hoverMarkup(t, h, ctx, root, positionAt(t, text, "local guarded = p", len("local guarded = ")+1))
	if !strings.Contains(guardedHover, "**Type:** `Player`") {
		t.Fatalf("guarded hover:\n%s", guardedHover)
	}

	hints := inlayHints(t, h, ctx, text, protocol.Range{Start: protocol.Position{}, End: protocol.Position{Line: 100}})
	byLine := make(map[uint32][]string)
	for _, hint := range hints {
		label, _ := hint.Label.(protocol.String)
		byLine[hint.Position.Line] = append(byLine[hint.Position.Line], string(label))
	}
	beforeLine := positionAt(t, text, "local before = p", 0).Line
	afterLine := positionAt(t, text, "local after = p", 0).Line
	if labels := byLine[beforeLine]; len(labels) != 1 || labels[0] != ": Player?" {
		t.Fatalf("before hint = %v", labels)
	}
	if labels := byLine[afterLine]; len(labels) != 1 || labels[0] != ": Player" {
		t.Fatalf("after hint = %v", labels)
	}
}

func TestM9ExactClassDiagnosticsE2E(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@class (exact) Point
---@field x number
---@field y number
local Point = {}

---@type Point
local missing = {x = 1}

---@type Point
local extra = {x = 1, y = 2, z = 3}
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := initializeWithOptions(t, `{"pico8":{"typeChecking":"basic"}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	_, semantics, _, ok := h.srv.carts.SemanticsFor(ctx, root)
	if !ok {
		t.Fatal("missing semantics")
	}
	var codes []string
	for _, diag := range semantics.Diagnostics {
		codes = append(codes, diag.Code)
	}
	if !containsString(codes, "missing-fields") {
		t.Fatalf("diagnostics = %v, missing missing-fields", codes)
	}
	if !containsString(codes, "assign-type-mismatch") {
		t.Fatalf("diagnostics = %v, missing assign-type-mismatch", codes)
	}
}
