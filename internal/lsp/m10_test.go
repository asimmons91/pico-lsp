package lsp

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
)

func TestM10GenericHoverAndCompletion(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@class Box<T>
---@field value T

---@generic T
---@param value T
---@return Box<T>
local function box(value)
  return {value = value}
end

local numbers = box(1)
local nv = numbers.value
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := initializeWithOptions(t, `{"pico8":{"typeChecking":"basic","inlayHints":true}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	boxHover := hoverMarkup(t, h, ctx, root, positionAt(t, text, "local numbers = box(1)", len("local numbers = ")+1))
	for _, want := range []string{"box<T>(value: T)", "Box<T>"} {
		if !strings.Contains(boxHover, want) {
			t.Fatalf("generic function hover missing %q:\n%s", want, boxHover)
		}
	}
	numbersHover := hoverMarkup(t, h, ctx, root, positionAt(t, text, "local nv = numbers.value", len("local nv = ")+1))
	if !strings.Contains(numbersHover, "**Type:** `Box<number>`") {
		t.Fatalf("generic local hover:\n%s", numbersHover)
	}
	items := completionItems(t, h, ctx, root, positionAt(t, text, "local nv = numbers.value", len("local nv = numbers.")))
	item := findCompletion(items, "value")
	if item == nil {
		t.Fatalf("generic member completion = %v", completionLabels(items))
	}
	if detail, _ := item.Detail.Get(); !strings.Contains(detail, "number") {
		t.Fatalf("generic member detail = %q", detail)
	}

	hints := inlayHints(t, h, ctx, text, protocol.Range{Start: protocol.Position{}, End: protocol.Position{Line: 100}})
	labels := make([]string, 0, len(hints))
	for _, hint := range hints {
		label, _ := hint.Label.(protocol.String)
		labels = append(labels, string(label))
	}
	if !strings.Contains(strings.Join(labels, " | "), ": Box<number>") {
		t.Fatalf("generic inlay hints = %v", labels)
	}
}

func TestM10OverloadHoverAndSignatureHelp(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@overload fun(x: number): number
---@overload fun(x: string): string
local function identity(x)
  return x
end

local n = identity(1)
local s = identity("x")
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := initializeWithOptions(t, `{"pico8":{"typeChecking":"basic"}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	hover := hoverMarkup(t, h, ctx, root, positionAt(t, text, "local n = identity(1)", len("local n = ")+1))
	if !strings.Contains(hover, "**Overloads**") || !strings.Contains(hover, "fun(x: number): number") || !strings.Contains(hover, "fun(x: string): string") {
		t.Fatalf("overload hover:\n%s", hover)
	}
	help := signatureHelp(t, h, ctx, root, positionAt(t, text, `local s = identity("x")`, len(`local s = identity(`)))
	if help == nil || len(help.Signatures) != 2 {
		t.Fatalf("overload signature help = %+v", help)
	}
	if active := activeParameter(t, help); active != 0 {
		t.Fatalf("overload active parameter = %d", active)
	}
	if help.ActiveSignature == nil || *help.ActiveSignature != 1 {
		t.Fatalf("overload active signature = %v, want string overload", help.ActiveSignature)
	}
}

func TestM10OperatorAndTaggedUnionLSP(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@class Vec
---@field x number
---@operator add(Vec): Vec
---@operator unm(): Vec
local Vec = {}

local a = setmetatable({x = 1}, {__index = Vec})
local b = setmetatable({x = 2}, {__index = Vec})
local sum = a + b
local sx = sum.x

---@class Shape
---@field kind "circle" | "square"
---@class Circle: Shape
---@field kind "circle"
---@field radius number
---@class Square: Shape
---@field kind "square"
---@field side number

---@type Circle | Square
local shape = {}
if shape.kind == "circle" then
  local radius = shape.radius
  local tag = shape.kind
end
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := initializeWithOptions(t, `{"pico8":{"typeChecking":"basic"}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	classHover := hoverMarkup(t, h, ctx, root, positionAt(t, text, "{__index = Vec}", len("{__index = ")+1))
	if !strings.Contains(classHover, "**Operators:**") || !strings.Contains(classHover, "add(Vec): Vec") {
		t.Fatalf("operator class hover:\n%s", classHover)
	}
	sumHover := hoverMarkup(t, h, ctx, root, positionAt(t, text, "local sx = sum.x", len("local sx = ")+1))
	if !strings.Contains(sumHover, "**Type:** `Vec`") {
		t.Fatalf("operator result hover:\n%s", sumHover)
	}
	items := completionItems(t, h, ctx, root, positionAt(t, text, "local radius = shape.radius", len("local radius = shape.")))
	labels := completionLabels(items)
	if !containsString(labels, "radius") || containsString(labels, "side") {
		t.Fatalf("tagged-union completion = %v", labels)
	}
	kindHover := hoverMarkup(t, h, ctx, root, positionAt(t, text, "local radius = shape.radius", len("local radius = ")+1))
	if !strings.Contains(kindHover, "Circle") {
		t.Fatalf("tagged-union hover:\n%s", kindHover)
	}
	tagHover := hoverMarkup(t, h, ctx, root, positionAt(t, text, "local tag = shape.kind", len("local tag = shape.")+1))
	if !strings.Contains(tagHover, `"circle"`) {
		t.Fatalf("tag literal hover:\n%s", tagHover)
	}
}

func TestM10AnnotationCompletion(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@class Vec
---@operator a
local Vec = {}
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := initializeWithOptions(t, `{"pico8":{"typeChecking":"basic"}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	items := completionItems(t, h, ctx, root, positionAt(t, text, "---@operator a", len("---@operator a")))
	if findCompletion(items, "add") == nil {
		t.Fatalf("operator completion = %v", completionLabels(items))
	}
}
