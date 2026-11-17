package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func writeTemp(t *testing.T, dir, name, text string) uri.URI {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return uri.File(path)
}

func openDoc(t *testing.T, h *harness, ctx context.Context, u uri.URI, text string) {
	t.Helper()
	if err := h.server.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        u,
			LanguageID: protocol.LanguageKind("pico8"),
			Version:    1,
			Text:       text,
		},
	}); err != nil {
		t.Fatalf("didOpen: %v", err)
	}
}

func waitDiagnostic(t *testing.T, client *testClient, u uri.URI) *protocol.PublishDiagnosticsParams {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case params := <-client.diagnostics:
			if params.URI == u {
				return params
			}
		case <-deadline:
			t.Fatalf("timed out waiting for diagnostics on %s", u.String())
		}
	}
}

func symbolNames(symbols []protocol.DocumentSymbol) []string {
	var names []string
	for _, symbol := range symbols {
		names = append(names, symbol.Name)
		names = append(names, symbolNames(symbol.Children)...)
	}
	return names
}

func containsString(list []string, want string) bool {
	for _, value := range list {
		if value == want {
			return true
		}
	}
	return false
}

func TestEndToEndCartDiagnostics(t *testing.T) {
	dir := t.TempDir()
	rootText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"missing.lua\"\nprint(1)\n"
	root := writeTemp(t, dir, "cart.p8", rootText)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	params := waitDiagnostic(t, h.client, root)
	if len(params.Diagnostics) == 0 {
		t.Fatal("no diagnostics")
	}
	found := false
	for _, diag := range params.Diagnostics {
		if diag.Code == protocol.String("include-missing") {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v, want include-missing", params.Diagnostics)
	}

	h.exit(t, true)
}

func TestEndToEndSyntaxDiagnosticsInInclude(t *testing.T) {
	dir := t.TempDir()
	rootText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib/player.lua\"\n"
	root := writeTemp(t, dir, "cart.p8", rootText)
	includeText := "function player_update()\n  y = 1 +\nend\n"
	include := writeTemp(t, dir, "lib/player.lua", includeText)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	params := waitDiagnostic(t, h.client, include)
	if len(params.Diagnostics) == 0 {
		t.Fatal("no include diagnostics")
	}
	found := false
	for _, diag := range params.Diagnostics {
		if diag.Code == protocol.String("syntax") {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v, want syntax", params.Diagnostics)
	}

	h.exit(t, true)
}

func TestEndToEndDocumentSymbolsAndFolding(t *testing.T) {
	dir := t.TempDir()
	rootText := `pico-8 cartridge // pico-8.com
version 42
__lua__
function _init()
  x=1
end
-->8 player
#include "lib/player.lua"
`
	root := writeTemp(t, dir, "cart.p8", rootText)
	includeText := `function player_update()
  return 1
end
`
	include := writeTemp(t, dir, "lib/player.lua", includeText)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	result, err := h.server.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
	})
	if err != nil {
		t.Fatalf("documentSymbol: %v", err)
	}
	symbols, ok := result.(protocol.DocumentSymbolSlice)
	if !ok {
		t.Fatalf("result = %T, want DocumentSymbolSlice", result)
	}
	names := symbolNames(symbols)
	for _, want := range []string{"Tab 1", "player", "_init", "lib/player.lua"} {
		if !containsString(names, want) {
			t.Errorf("symbols = %v, missing %q", names, want)
		}
	}

	result, err = h.server.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: include},
	})
	if err != nil {
		t.Fatalf("include documentSymbol: %v", err)
	}
	symbols, ok = result.(protocol.DocumentSymbolSlice)
	if !ok {
		t.Fatalf("include result = %T", result)
	}
	if !containsString(symbolNames(symbols), "player_update") {
		t.Fatalf("include symbols = %v, missing player_update", symbolNames(symbols))
	}

	folds, err := h.server.FoldingRanges(ctx, &protocol.FoldingRangeParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
	})
	if err != nil {
		t.Fatalf("foldingRanges: %v", err)
	}
	if len(folds) == 0 {
		t.Fatal("expected folding ranges")
	}
	foundTab := false
	for _, fold := range folds {
		if fold.StartLine == 3 && fold.EndLine >= 5 {
			foundTab = true
		}
	}
	if !foundTab {
		t.Fatalf("folds = %+v, want tab fold", folds)
	}

	h.exit(t, true)
}

func TestEndToEndP8PNGDiagnostic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cart.p8.png")
	root := uri.File(path)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, "PNG")

	params := waitDiagnostic(t, h.client, root)
	if len(params.Diagnostics) != 1 || !strings.Contains(string(params.Diagnostics[0].Message.(protocol.String)), "not supported") {
		t.Fatalf("diagnostics = %+v", params.Diagnostics)
	}

	h.exit(t, true)
}

func TestEndToEndStandaloneLuaDiagnostics(t *testing.T) {
	dir := t.TempDir()
	text := "x = 1 +\n"
	u := writeTemp(t, dir, "main.lua", text)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, u, text)

	params := waitDiagnostic(t, h.client, u)
	if len(params.Diagnostics) == 0 || params.Diagnostics[0].Code != protocol.String("syntax") {
		t.Fatalf("diagnostics = %+v, want syntax", params.Diagnostics)
	}

	h.exit(t, true)
}

func TestEndToEndChangeClearsIncludeDiagnostics(t *testing.T) {
	dir := t.TempDir()
	rootText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"a.lua\"\n"
	root := writeTemp(t, dir, "cart.p8", rootText)
	badText := "x = 1 +\n"
	include := writeTemp(t, dir, "a.lua", badText)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)
	openDoc(t, h, ctx, include, badText)

	params := waitDiagnostic(t, h.client, include)
	if len(params.Diagnostics) == 0 {
		t.Fatal("expected include diagnostics")
	}

	goodText := "x = 1\n"
	if err := h.server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: include},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangeWholeDocument{Text: goodText},
		},
	}); err != nil {
		t.Fatalf("didChange: %v", err)
	}

	deadline := time.After(3 * time.Second)
	for {
		select {
		case params := <-h.client.diagnostics:
			if params.URI == include && len(params.Diagnostics) == 0 {
				h.exit(t, true)
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for include diagnostics to clear")
		}
	}
}

func TestEndToEndLuaOwnedByCart(t *testing.T) {
	dir := t.TempDir()
	rootText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"player.lua\"\n"
	writeTemp(t, dir, "cart.p8", rootText)
	luaText := "function player()\n  return 1 +\nend\n"
	luaURI := writeTemp(t, dir, "player.lua", luaText)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Only the .lua is opened; the owning .p8 is discovered on disk.
	openDoc(t, h, ctx, luaURI, luaText)
	params := waitDiagnostic(t, h.client, luaURI)
	if len(params.Diagnostics) == 0 {
		t.Fatal("expected diagnostics for cart-owned lua")
	}
	h.exit(t, true)
}

func TestEndToEndRemovingIncludeClearsDiagnostics(t *testing.T) {
	dir := t.TempDir()
	rootText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"a.lua\"\n"
	root := writeTemp(t, dir, "cart.p8", rootText)
	writeTemp(t, dir, "a.lua", "x = 1 +\n")

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	openDoc(t, h, ctx, root, rootText)
	include := uri.File(filepath.Join(dir, "a.lua"))
	params := waitDiagnostic(t, h.client, include)
	if len(params.Diagnostics) == 0 {
		t.Fatal("expected include diagnostics")
	}

	withoutInclude := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nx=1\n"
	if err := h.server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: root},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangeWholeDocument{Text: withoutInclude},
		},
	}); err != nil {
		t.Fatalf("didChange: %v", err)
	}

	deadline := time.After(3 * time.Second)
	for {
		select {
		case params := <-h.client.diagnostics:
			if params.URI == include && len(params.Diagnostics) == 0 {
				h.exit(t, true)
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for removed include diagnostics to clear")
		}
	}
}

func TestEndToEndFoldingMultilineConstructs(t *testing.T) {
	dir := t.TempDir()
	text := `--[[
multi
line
]]
local t = {
  1,
  2,
}
local s = [[
long
string
]]
if (x) y=1
`
	u := writeTemp(t, dir, "main.lua", text)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, u, text)

	folds, err := h.server.FoldingRanges(ctx, &protocol.FoldingRangeParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
	})
	if err != nil {
		t.Fatalf("foldingRanges: %v", err)
	}
	var hasComment, hasTable, hasString, hasShorthand bool
	for _, fold := range folds {
		switch {
		case fold.StartLine == 0 && fold.EndLine >= 3:
			hasComment = true
		case fold.StartLine == 4 && fold.EndLine >= 7:
			hasTable = true
		case fold.StartLine == 8 && fold.EndLine >= 11:
			hasString = true
		case fold.StartLine == 12:
			hasShorthand = true
		}
	}
	if !hasComment || !hasTable || !hasString {
		t.Fatalf("folds = %+v, want comment/table/string folds", folds)
	}
	if hasShorthand {
		t.Fatalf("folds = %+v, single-line shorthand must not fold", folds)
	}

	h.exit(t, true)
}

func TestEndToEndSymbolKinds(t *testing.T) {
	dir := t.TempDir()
	text := `local top=1
global=2
function obj:method()
  ::again::
  goto again
end
`
	u := writeTemp(t, dir, "symbols.lua", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, u, text)

	result, err := h.server.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
	})
	if err != nil {
		t.Fatalf("documentSymbol: %v", err)
	}
	symbols := result.(protocol.DocumentSymbolSlice)
	kinds := map[string]protocol.SymbolKind{}
	var walk func([]protocol.DocumentSymbol)
	walk = func(list []protocol.DocumentSymbol) {
		for _, symbol := range list {
			kinds[symbol.Name] = symbol.Kind
			walk(symbol.Children)
		}
	}
	walk(symbols)

	want := map[string]protocol.SymbolKind{
		"top":    protocol.SymbolKindVariable,
		"global": protocol.SymbolKindVariable,
		"method": protocol.SymbolKindMethod,
		"again":  protocol.SymbolKindKey,
	}
	for name, kind := range want {
		if kinds[name] != kind {
			t.Errorf("symbol %q kind = %v, want %v (all=%v)", name, kinds[name], kind, kinds)
		}
	}
	h.exit(t, true)
}

func TestEndToEndP8SectionFolding(t *testing.T) {
	dir := t.TempDir()
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
a=1
__gfx__
0000
1111
2222
__sfx__
0000
`
	u := writeTemp(t, dir, "sections.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, u, text)

	folds, err := h.server.FoldingRanges(ctx, &protocol.FoldingRangeParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
	})
	if err != nil {
		t.Fatalf("foldingRanges: %v", err)
	}
	foundGfx := false
	for _, fold := range folds {
		if fold.StartLine == 4 && fold.EndLine >= 7 {
			foundGfx = true
		}
	}
	if !foundGfx {
		t.Fatalf("folds = %+v, want __gfx__ section fold", folds)
	}
	h.exit(t, true)
}
