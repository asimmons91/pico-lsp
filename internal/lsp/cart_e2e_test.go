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
	for _, symbol := range symbols {
		if symbol.Name == "top" {
			if symbol.Range.Start.Character != 0 || symbol.SelectionRange.Start.Character != 6 {
				t.Errorf("top ranges = %+v / %+v, want full local declaration / name", symbol.Range, symbol.SelectionRange)
			}
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

func TestEndToEndDidOpenBeforeInitialized(t *testing.T) {
	dir := t.TempDir()
	text := "x = 1 +\n"
	u := writeTemp(t, dir, "early.lua", text)

	h := newHarness(t, newTestClient())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := h.server.Initialize(ctx, &protocol.InitializeParams{}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	openDoc(t, h, ctx, u, text)
	select {
	case params := <-h.client.diagnostics:
		t.Fatalf("diagnostics published before initialized: %+v", params)
	case <-time.After(20 * time.Millisecond):
	}

	if err := h.server.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		t.Fatalf("initialized: %v", err)
	}
	params := waitDiagnostic(t, h.client, u)
	if len(params.Diagnostics) == 0 {
		t.Fatal("diagnostics were not published after initialized")
	}

	h.exit(t, true)
}

func TestEndToEndSymbolsDoNotLeakAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	rootText := `pico-8 cartridge // pico-8.com
version 42
__lua__
function outer()
#include "lib.lua"
end
`
	root := writeTemp(t, dir, "cart.p8", rootText)
	libText := "function inner()\n  return 1\nend\n"
	lib := writeTemp(t, dir, "lib.lua", libText)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	rootResult, err := h.server.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
	})
	if err != nil {
		t.Fatalf("root documentSymbol: %v", err)
	}
	rootNames := symbolNames(rootResult.(protocol.DocumentSymbolSlice))
	if !containsString(rootNames, "outer") {
		t.Fatalf("root symbols = %v, missing outer", rootNames)
	}
	if containsString(rootNames, "inner") {
		t.Fatalf("root symbols = %v, leaked included inner", rootNames)
	}

	libResult, err := h.server.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: lib},
	})
	if err != nil {
		t.Fatalf("include documentSymbol: %v", err)
	}
	libSymbols := libResult.(protocol.DocumentSymbolSlice)
	libNames := symbolNames(libSymbols)
	if !containsString(libNames, "inner") {
		t.Fatalf("include symbols = %v, missing inner", libNames)
	}
	for _, symbol := range libSymbols {
		if symbol.Name == "inner" && symbol.Range.Start.Line != 0 {
			t.Fatalf("inner range starts at line %d, want 0", symbol.Range.Start.Line)
		}
	}

	h.exit(t, true)
}

func TestEndToEndOwnershipChangeRefreshesSymbols(t *testing.T) {
	dir := t.TempDir()
	luaText := "function one()\nend\n"
	lua := writeTemp(t, dir, "a.lua", luaText)
	writeTemp(t, dir, "cart.p8", "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nx=1\n")

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, lua, luaText)

	names := func() []string {
		result, err := h.server.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: lua},
		})
		if err != nil {
			t.Fatalf("documentSymbol: %v", err)
		}
		return symbolNames(result.(protocol.DocumentSymbolSlice))
	}
	if !containsString(names(), "one") {
		t.Fatalf("initial symbols = %v, want one", names())
	}

	// The cart starts including the lua, then the lua changes.
	writeTemp(t, dir, "cart.p8", "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"a.lua\"\n")
	newText := "function two()\nend\n"
	if err := h.server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: lua},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangeWholeDocument{Text: newText},
		},
	}); err != nil {
		t.Fatalf("didChange: %v", err)
	}

	// Remove the include again and touch the cart so ownership is re-resolved.
	withoutInclude := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nx=1\n"
	cartURI := writeTemp(t, dir, "cart.p8", withoutInclude)
	openDoc(t, h, ctx, cartURI, withoutInclude)
	namesAfter := names()
	if !containsString(namesAfter, "two") || containsString(namesAfter, "one") {
		t.Fatalf("symbols after ownership change = %v, want two", namesAfter)
	}

	h.exit(t, true)
}

func TestEndToEndClosingUnsavedCartClearsIncludeDiagnostics(t *testing.T) {
	dir := t.TempDir()
	include := writeTemp(t, dir, "a.lua", "x = 1 +\n")
	root := uri.File(filepath.Join(dir, "unsaved.p8"))
	rootText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"a.lua\"\n"

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	params := waitDiagnostic(t, h.client, include)
	if len(params.Diagnostics) == 0 {
		t.Fatal("expected include diagnostics")
	}

	if err := h.server.DidClose(ctx, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
	}); err != nil {
		t.Fatalf("didClose: %v", err)
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

func TestEndToEndFoldingOneStatementBlocks(t *testing.T) {
	dir := t.TempDir()
	text := `function f()
 return 1
end
if x then
 y=1
end
while x do
 y=2
end
for i=1,2 do
 y=i
end
do
 y=3
end
repeat
 y=4
until x
`
	u := writeTemp(t, dir, "blocks.lua", text)
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
	starts := map[uint32]bool{}
	for _, fold := range folds {
		starts[fold.StartLine] = true
	}
	for _, start := range []uint32{0, 3, 6, 9, 12, 15} {
		if !starts[start] {
			t.Errorf("folds = %+v, missing block starting at line %d", folds, start)
		}
	}
	h.exit(t, true)
}

func TestEndToEndIncludedP8ChangeRefreshesParent(t *testing.T) {
	dir := t.TempDir()
	rootText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"other.p8\"\n"
	root := writeTemp(t, dir, "root.p8", rootText)
	otherText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nfunction one()\nend\n"
	other := writeTemp(t, dir, "other.p8", otherText)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	openDoc(t, h, ctx, root, rootText)
	openDoc(t, h, ctx, other, otherText)

	rootFlat := func() string {
		snap, _, ok := h.srv.carts.SnapshotFor(ctx, root)
		if !ok {
			t.Fatal("missing root snapshot")
		}
		return string(snap.Cart.Flat)
	}
	if !strings.Contains(rootFlat(), "function one") {
		t.Fatalf("root flat = %q, want function one", rootFlat())
	}

	changedText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nfunction two()\nend\n"
	if err := h.server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: other},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangeWholeDocument{Text: changedText},
		},
	}); err != nil {
		t.Fatalf("didChange: %v", err)
	}
	if _, err := h.server.Hover(ctx, &protocol.HoverParams{}); err != nil {
		t.Fatalf("barrier: %v", err)
	}
	flat := rootFlat()
	if !strings.Contains(flat, "function two") || strings.Contains(flat, "function one") {
		t.Fatalf("root flat after include edit = %q, want two", flat)
	}

	if err := h.server.DidClose(ctx, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: other},
	}); err != nil {
		t.Fatalf("didClose: %v", err)
	}
	if _, err := h.server.Hover(ctx, &protocol.HoverParams{}); err != nil {
		t.Fatalf("barrier: %v", err)
	}
	flat = rootFlat()
	if !strings.Contains(flat, "function one") || strings.Contains(flat, "function two") {
		t.Fatalf("root flat after include close = %q, want disk version one", flat)
	}

	h.exit(t, true)
}
