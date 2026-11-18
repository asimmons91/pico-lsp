package lsp

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/glyphs"
)

func findGlyphCompletion(items []protocol.CompletionItem, trigger string) *protocol.CompletionItem {
	for i := range items {
		item := &items[i]
		if filter, ok := item.FilterText.Get(); ok && containsWord(filter, trigger) {
			return item
		}
	}
	return nil
}

func containsWord(text, word string) bool {
	for _, field := range strings.Fields(text) {
		if field == word {
			return true
		}
	}
	return false
}

func TestEndToEndGlyphCompletion(t *testing.T) {
	dir := t.TempDir()
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
p8_le
local s = "p8_he"
-- p8_he
p8_font
`
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	pos := positionAt(t, text, "p8_le", len("p8_le"))
	items := completionItems(t, h, ctx, root, pos)
	left := findGlyphCompletion(items, "p8_left")
	if left == nil {
		t.Fatalf("p8_le completion = %v, missing p8_left", completionLabels(items))
	}
	if left.InsertTextFormat != protocol.InsertTextFormatPlainText {
		t.Fatalf("p8_left format = %v, want plain text", left.InsertTextFormat)
	}
	edit := itemTextEdit(t, *left)
	if edit.NewText != "⬅️" {
		t.Fatalf("p8_left edit = %+v, want ⬅️", edit)
	}
	if edit.Range.Start.Character != pos.Character-5 || edit.Range.End != pos {
		t.Fatalf("p8_left range = %+v, want the p8_le prefix ending at %+v", edit.Range, pos)
	}

	// pico8-ls-compatible hyphen aliases use a dedicated prefix scan.
	text2 := strings.ReplaceAll(text, "p8_le", "p8-le")
	root2 := writeTemp(t, dir, "hyphen.p8", text2)
	openDoc(t, h, ctx, root2, text2)
	pos = positionAt(t, text2, "p8-le", len("p8-le"))
	items = completionItems(t, h, ctx, root2, pos)
	left = findGlyphCompletion(items, "p8-left")
	if left == nil {
		t.Fatalf("p8-le completion = %v, missing p8-left", completionLabels(items))
	}
	edit = itemTextEdit(t, *left)
	if edit.NewText != "⬅️" || edit.Range.Start.Character != pos.Character-5 {
		t.Fatalf("p8-left edit = %+v", edit)
	}

	// Glyph completion works inside an ordinary string.
	pos = positionAt(t, text, `local s = "p8_he"`, len(`local s = "p8_he`))
	items = completionItems(t, h, ctx, root, pos)
	if !stringsHasGlyph(items, "♥") {
		t.Fatalf("string glyph completion = %v, missing heart", completionLabels(items))
	}

	// Control-code entries are string-only.
	pos = positionAt(t, text, "p8_font", len("p8_font"))
	items = completionItems(t, h, ctx, root, pos)
	if findGlyphCompletion(items, "p8_font_custom") != nil {
		t.Fatalf("control entry was offered in code: %v", completionLabels(items))
	}
	// Add a control prefix inside the string by editing a separate document.
	controlText := strings.ReplaceAll(text, `"p8_he"`, `"p8_font"`)
	controlRoot := writeTemp(t, dir, "control.p8", controlText)
	openDoc(t, h, ctx, controlRoot, controlText)
	pos = positionAt(t, controlText, `"p8_font"`, len(`"p8_font`))
	items = completionItems(t, h, ctx, controlRoot, pos)
	font := findGlyphCompletion(items, "p8_font_custom")
	if font == nil {
		t.Fatalf("string control completion = %v, missing p8_font_custom", completionLabels(items))
	}
	if edit := itemTextEdit(t, *font); edit.NewText != `\14` {
		t.Fatalf("font custom edit = %+v, want \\14", edit)
	}

	// Comments do not receive glyph completion.
	pos = positionAt(t, text, "-- p8_he", len("-- p8_he"))
	if items := completionItems(t, h, ctx, root, pos); len(items) != 0 {
		t.Fatalf("comment glyph completion = %v, want empty", completionLabels(items))
	}

	// An unmatched glyph prefix does not fall through to normal completion.
	unmatched := strings.ReplaceAll(text, "p8_le", "p8_zz")
	unmatchedRoot := writeTemp(t, dir, "unmatched.p8", unmatched)
	openDoc(t, h, ctx, unmatchedRoot, unmatched)
	pos = positionAt(t, unmatched, "p8_zz", len("p8_zz"))
	if items := completionItems(t, h, ctx, unmatchedRoot, pos); len(items) != 0 {
		t.Fatalf("unmatched glyph completion = %v, want empty", completionLabels(items))
	}
}

func TestGlyphCompletionUsesUTF16Range(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nlocal s = \"⬅️p8_le\"\n"
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	pos := positionAt(t, text, `local s = "⬅️p8_le"`, len(`local s = "⬅️p8_le`))
	items := completionItems(t, h, ctx, root, pos)
	left := findGlyphCompletion(items, "p8_left")
	if left == nil {
		t.Fatalf("glyph completion after a multi-rune glyph = %v", completionLabels(items))
	}
	edit := itemTextEdit(t, *left)
	if edit.Range.Start.Character != pos.Character-5 || edit.Range.End != pos {
		t.Fatalf("UTF-16 range = %+v, want prefix ending at %+v", edit.Range, pos)
	}
}

func TestGlyphCompletionInIncludedLua(t *testing.T) {
	dir := t.TempDir()
	cartText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib.lua\"\n"
	libText := "p8_le\n"
	root := writeTemp(t, dir, "cart.p8", cartText)
	lib := writeTemp(t, dir, "lib.lua", libText)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, cartText)
	openDoc(t, h, ctx, lib, libText)

	pos := positionAt(t, libText, "p8_le", len("p8_le"))
	items := completionItems(t, h, ctx, lib, pos)
	if findGlyphCompletion(items, "p8_left") == nil {
		t.Fatalf("included-file glyph completion = %v, missing p8_left", completionLabels(items))
	}
}

func TestEndToEndCustomGlyphCompletion(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\np8_lo\n"
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{
		InitializationOptions: protocol.LSPAny(`{"pico8":{"glyphs":[{"name":"logo","byte":128,"description":"custom font slot 0x80"}]}}`),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	pos := positionAt(t, text, "p8_lo", len("p8_lo"))
	items := completionItems(t, h, ctx, root, pos)
	logo := findGlyphCompletion(items, "p8_logo")
	if logo == nil {
		t.Fatalf("custom completion = %v, missing p8_logo", completionLabels(items))
	}
	if edit := itemTextEdit(t, *logo); edit.NewText != "█" {
		t.Fatalf("custom logo edit = %+v, want █", edit)
	}
}

func TestGlyphsRequestFiltersStringOnlyEntries(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nlocal s = \"\"\np8_le\n"
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	all := requestGlyphs(t, h, ctx, nil)
	if findGlyphEntry(all, "font_custom") == nil || findGlyphEntry(all, "left") == nil {
		t.Fatalf("all glyphs = %+v, missing built-ins", all)
	}

	codePos := positionAt(t, text, "p8_le", len("p8_le"))
	code := requestGlyphs(t, h, ctx, &glyphsParams{
		TextDocument: &protocol.TextDocumentIdentifier{URI: root},
		Position:     &codePos,
	})
	if findGlyphEntry(code, "font_custom") != nil {
		t.Fatalf("code glyphs included string-only control entry: %+v", code)
	}
	if findGlyphEntry(code, "left") == nil {
		t.Fatalf("code glyphs missing left: %+v", code)
	}

	stringPos := positionAt(t, text, `local s = ""`, len(`local s = "`))
	str := requestGlyphs(t, h, ctx, &glyphsParams{
		TextDocument: &protocol.TextDocumentIdentifier{URI: root},
		Position:     &stringPos,
	})
	if findGlyphEntry(str, "font_custom") == nil {
		t.Fatalf("string glyphs missing font_custom: %+v", str)
	}

	// The custom method must be registered in protocolMethods to work over the
	// wire, not only through the direct Server.Request dispatcher.
	var wire GlyphsResult
	if _, err := h.clientConn.Call(ctx, RequestGlyphs, map[string]any{}, &wire); err != nil {
		t.Fatalf("pico8/glyphs over the wire: %v", err)
	}
	if findGlyphEntry(wire.Glyphs, "left") == nil {
		t.Fatalf("wire glyphs missing left: %+v", wire.Glyphs)
	}
}

func TestInsertGlyphCommand(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nlocal s = \"\"\n"
	root := writeTemp(t, dir, "cart.p8", text)
	client := newTestClient()
	h := newHarness(t, client)
	h.initialize(t, &protocol.InitializeParams{
		Capabilities: protocol.ClientCapabilities{
			Workspace: &protocol.WorkspaceClientCapabilities{ApplyEdit: boolPtr(true)},
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	pos := positionAt(t, text, `local s = ""`, len(`local s = "`))
	if _, err := executeInsertGlyph(t, h, ctx, root, pos, "left", nil); err != nil {
		t.Fatalf("insert left: %v", err)
	}
	select {
	case applied := <-client.applied:
		edits := applied.Edit.Changes[root]
		if len(edits) != 1 || edits[0].NewText != "⬅️" {
			t.Fatalf("applied edits = %+v", edits)
		}
		if edits[0].Range.Start != pos || edits[0].Range.End != pos {
			t.Fatalf("insert range = %+v, want insertion at %+v", edits[0].Range, pos)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not apply the glyph edit")
	}

	// A typed trigger is replaced rather than duplicated.
	text2 := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nlocal s = \"p8_le\"\n"
	root2 := writeTemp(t, dir, "trigger.p8", text2)
	openDoc(t, h, ctx, root2, text2)
	pos = positionAt(t, text2, `"p8_le"`, len(`"p8_le`))
	if _, err := executeInsertGlyph(t, h, ctx, root2, pos, "left", nil); err != nil {
		t.Fatalf("insert left over trigger: %v", err)
	}
	select {
	case applied := <-client.applied:
		edits := applied.Edit.Changes[root2]
		if len(edits) != 1 || edits[0].NewText != "⬅️" {
			t.Fatalf("trigger edits = %+v", edits)
		}
		if edits[0].Range.Start.Character != pos.Character-5 {
			t.Fatalf("trigger range = %+v, want prefix replacement ending at %+v", edits[0].Range, pos)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not apply the trigger glyph edit")
	}

	// String-only entries are rejected in code.
	codeText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\np8_font\n"
	codeRoot := writeTemp(t, dir, "control.p8", codeText)
	openDoc(t, h, ctx, codeRoot, codeText)
	codePos := positionAt(t, codeText, "p8_font", len("p8_font"))
	if _, err := executeInsertGlyph(t, h, ctx, codeRoot, codePos, "font_custom", nil); err == nil {
		t.Fatal("inserted a string-only control entry in code")
	}

	// Read-only virtual carts are rejected.
	if _, err := executeInsertGlyph(t, h, ctx, uri.URI("pico8-cart:/tmp/cart.p8"), protocol.Position{}, "left", nil); err == nil {
		t.Fatal("inserted a glyph into a read-only virtual cart")
	}
}

func TestInsertGlyphCommandFallbackEdit(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\np8_x\n"
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	initialized := h.initialize(t, &protocol.InitializeParams{})
	if !containsString(initialized.Capabilities.ExecuteCommandProvider.Commands, CommandInsertGlyph) {
		t.Fatalf("executeCommandProvider = %+v, missing %s", initialized.Capabilities.ExecuteCommandProvider, CommandInsertGlyph)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	pos := positionAt(t, text, "p8_x", len("p8_x"))
	raw, err := executeInsertGlyphRaw(t, h, ctx, root, pos, "", intPtr(0x97))
	if err != nil {
		t.Fatalf("insert by byte: %v", err)
	}
	var result insertGlyphResult
	if err := protocol.Unmarshal(raw.(protocol.LSPAny), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Applied || result.Edit == nil {
		t.Fatalf("fallback result = %+v, want edit", result)
	}
	edits := result.Edit.Changes[root]
	if len(edits) != 1 || edits[0].NewText != "❎" {
		t.Fatalf("fallback edits = %+v, want ❎", edits)
	}
}

func requestGlyphs(t *testing.T, h *harness, ctx context.Context, params *glyphsParams) []glyphs.Entry {
	t.Helper()
	var raw protocol.LSPAny
	if params == nil {
		raw = protocol.LSPAny(`null`)
	} else {
		data, err := protocol.Marshal(params)
		if err != nil {
			t.Fatalf("marshal glyph params: %v", err)
		}
		raw = protocol.LSPAny(data)
	}
	result, err := h.srv.Request(ctx, RequestGlyphs, raw)
	if err != nil {
		t.Fatalf("pico8/glyphs: %v", err)
	}
	glyphResult, ok := result.(GlyphsResult)
	if !ok {
		t.Fatalf("pico8/glyphs result = %T", result)
	}
	return glyphResult.Glyphs
}

func findGlyphEntry(entries []glyphs.Entry, name string) *glyphs.Entry {
	for i := range entries {
		if entries[i].Name == name {
			return &entries[i]
		}
	}
	return nil
}

func executeInsertGlyph(t *testing.T, h *harness, ctx context.Context, u uri.URI, pos protocol.Position, name string, byteValue *int) (any, error) {
	t.Helper()
	return executeInsertGlyphRaw(t, h, ctx, u, pos, name, byteValue)
}

func executeInsertGlyphRaw(t *testing.T, h *harness, ctx context.Context, u uri.URI, pos protocol.Position, name string, byteValue *int) (any, error) {
	t.Helper()
	argument := map[string]any{
		"textDocument": map[string]string{"uri": u.String()},
		"position":     pos,
	}
	if name != "" {
		argument["name"] = name
	}
	if byteValue != nil {
		argument["byte"] = *byteValue
	}
	data, err := protocol.Marshal(argument)
	if err != nil {
		t.Fatalf("marshal insert glyph argument: %v", err)
	}
	return h.server.ExecuteCommand(ctx, &protocol.ExecuteCommandParams{
		Command:   CommandInsertGlyph,
		Arguments: []protocol.LSPAny{protocol.LSPAny(data)},
	})
}

func stringsHasGlyph(items []protocol.CompletionItem, text string) bool {
	for _, item := range items {
		if edit, ok := item.TextEdit.(*protocol.TextEdit); ok && edit.NewText == text {
			return true
		}
	}
	return false
}

func intPtr(value int) *int { return &value }
