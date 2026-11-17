package lsp

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

const m3HoverCart = `pico-8 cartridge // pico-8.com
version 42
__lua__
--- Move the player.
---@param dx? number
---@param dy? number
---@return number
function move(dx, dy)
  return (dx or 0) + (dy or 0)
end

` + "--- See [manual] and `circ`.\n" + `function documented() end

local player = {x = 10}
player.hp = 3

function player:step() end

? "hello"
a = @0x5f00
b = %0x5f00
c = $0x5f00
circfill(1, 2, 3)
#include "lib/player.lua"
nosuch.field = 1
`

func hoverMarkup(t *testing.T, h *harness, ctx context.Context, u uri.URI, position protocol.Position) string {
	t.Helper()
	result, err := h.server.Hover(ctx, &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     position,
		},
	})
	if err != nil {
		t.Fatalf("hover: %v", err)
	}
	if result == nil {
		return ""
	}
	switch contents := result.Contents.(type) {
	case *protocol.MarkupContent:
		return contents.Value
	case protocol.String:
		return string(contents)
	case *protocol.MarkedStringWithLanguage: //nolint:staticcheck // Legacy hover encoding remains supported.
		return contents.Value
	default:
		t.Fatalf("hover contents type = %T", result.Contents)
		return ""
	}
}

func TestEndToEndHover(t *testing.T) {
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", m3HoverCart)
	writeTemp(t, dir, "lib/player.lua", "function make_player() end\n")
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, m3HoverCart)

	cases := []struct {
		name    string
		pos     protocol.Position
		contain []string
		golden  string
	}{
		{
			name:    "user function",
			pos:     positionAt(t, m3HoverCart, "move(dx, dy)", 1),
			contain: []string{"```lua", "function move(dx?: number, dy?: number): number", "Move the player.", "dx", "dy", "optional", "Returns", "cart.p8"},
			golden:  "hover/user-function.md",
		},
		{
			name:    "user local",
			pos:     positionAt(t, m3HoverCart, "local player", len("local ")),
			contain: []string{"```lua", "local player", "cart.p8"},
			golden:  "hover/user-local.md",
		},
		{
			name:    "markdown escaping",
			pos:     positionAt(t, m3HoverCart, "documented()", 1),
			contain: []string{`See \[manual\] and ` + "`circ`", "function documented()"},
			golden:  "hover/markdown-escape.md",
		},
		{
			name:    "field",
			pos:     positionAt(t, m3HoverCart, "player.hp", len("player.")+1),
			contain: []string{"field hp"},
			golden:  "hover/field.md",
		},
		{
			name:    "method",
			pos:     positionAt(t, m3HoverCart, "player:step", len("player:")+1),
			contain: []string{"function player:step()"},
			golden:  "hover/method.md",
		},
		{
			name:    "builtin",
			pos:     positionAt(t, m3HoverCart, "circfill(", 2),
			contain: []string{"function circfill(x: number, y: number, r: number, col?: number)", "Draw a filled circle", "PICO-8 0.2.7 API"},
			golden:  "hover/builtin.md",
		},
		{
			name:    "include",
			pos:     positionAt(t, m3HoverCart, `#include "lib/player.lua"`, 2),
			contain: []string{`#include "lib/player.lua"`, "Resolved path: `lib/player.lua`", "non-recursive"},
			golden:  "hover/include.md",
		},
		{
			name:    "shorthand",
			pos:     positionAt(t, m3HoverCart, `? "hello"`, 0),
			contain: []string{"?expr", "print(expr)"},
			golden:  "hover/shorthand.md",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			markdown := hoverMarkup(t, h, ctx, root, tc.pos)
			if markdown == "" {
				t.Fatal("hover returned no result")
			}
			for _, want := range tc.contain {
				if !strings.Contains(markdown, want) {
					t.Fatalf("hover missing %q:\n%s", want, markdown)
				}
			}
			goldenText(t, tc.golden, markdown)
		})
	}

	for _, peek := range []struct {
		needle string
		want   string
		golden string
	}{
		{needle: "@0x5f00", want: "peek()", golden: "hover/peek.md"},
		{needle: "%0x5f00", want: "peek2()", golden: "hover/peek2.md"},
		{needle: "$0x5f00", want: "peek4()", golden: "hover/peek4.md"},
	} {
		t.Run("peek "+peek.needle[:1], func(t *testing.T) {
			markdown := hoverMarkup(t, h, ctx, root, positionAt(t, m3HoverCart, peek.needle, 0))
			if !strings.Contains(markdown, peek.want) {
				t.Fatalf("hover missing %q:\n%s", peek.want, markdown)
			}
			goldenText(t, peek.golden, markdown)
		})
	}

	empty := []struct {
		name string
		pos  protocol.Position
	}{
		{name: "comment", pos: positionAt(t, m3HoverCart, "--- Move the player.", 5)},
		{name: "string", pos: positionAt(t, m3HoverCart, `? "hello"`, len(`? "he`))},
		{name: "whitespace", pos: positionAt(t, m3HoverCart, "  return", 1)},
		{name: "unknown field", pos: positionAt(t, m3HoverCart, "nosuch.field", len("nosuch.")+1)},
	}
	for _, tc := range empty {
		t.Run("empty "+tc.name, func(t *testing.T) {
			if markdown := hoverMarkup(t, h, ctx, root, tc.pos); markdown != "" {
				t.Fatalf("hover on %s = %q, want empty", tc.name, markdown)
			}
		})
	}
}

func TestHoverOperatorFollowedByIdentifier(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nlocal addr = 0x5f00\n?circfill(1, 2, 3)\nx = @addr\n"
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	cases := []struct {
		name string
		find string
		want string
	}{
		{"print shorthand", "?circfill", "Draw a filled circle"},
		{"peek operator", "@addr", "local addr"},
	}
	for _, tc := range cases {
		// Offset 1 lands on the first byte of the identifier after the operator.
		pos := positionAt(t, text, tc.find, 1)
		got := hoverMarkup(t, h, ctx, root, pos)
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s hover at first identifier byte = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestTruncateMarkdownIsUTF8Safe(t *testing.T) {
	value := strings.Repeat("é", maxHoverMarkdown)
	got := truncateMarkdown(value)
	if !strings.HasSuffix(got, "*(truncated)*") {
		t.Fatalf("truncated markdown suffix = %q", got[len(got)-20:])
	}
	if !strings.Contains(got, "[PICO-8 manual]") {
		t.Fatal("truncated markdown is missing the manual link")
	}
	if len(got) > maxHoverMarkdown+len("\n\nSee the [PICO-8 manual](https://www.lexaloffle.com/pico-8.php?page=manual) for the full documentation.\n\n*(truncated)*") {
		t.Fatalf("truncated length = %d", len(got))
	}
	for _, r := range got {
		if r == '\uFFFD' {
			t.Fatal("truncation split a UTF-8 rune")
		}
	}
}

func TestHoverShowsParamAndClassFieldAnnotations(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@class Player
---@field score? number points
local p = {}
p.score = 1

---@param dx? number horizontal
function move(dx)
  print(dx)
end
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	for _, tc := range []struct {
		name    string
		needle  string
		offset  int
		contain []string
	}{
		{name: "class field", needle: "p.score", offset: len("p.") + 1, contain: []string{"Player.score", "number", "optional", "points"}},
		{name: "parameter", needle: "print(dx)", offset: len("print(") + 1, contain: []string{"parameter dx", "number", "optional", "horizontal"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			markdown := hoverMarkup(t, h, ctx, root, positionAt(t, text, tc.needle, tc.offset))
			for _, want := range tc.contain {
				if !strings.Contains(markdown, want) {
					t.Fatalf("hover missing %q:\n%s", want, markdown)
				}
			}
		})
	}
}
