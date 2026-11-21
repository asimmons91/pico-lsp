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

func openAnnotationCart(t *testing.T, text string) (*harness, uri.URI, context.Context, func()) {
	t.Helper()
	root := writeTemp(t, t.TempDir(), "annotations.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	openDoc(t, h, ctx, root, text)
	return h, root, ctx, cancel
}

func textEditRangeText(t *testing.T, text string, edit protocol.TextEdit) string {
	t.Helper()
	index := source.New([]byte(text), protocol.PositionEncodingKindUTF16)
	start, err := index.Offset(edit.Range.Start)
	if err != nil {
		t.Fatalf("edit start: %v", err)
	}
	end, err := index.Offset(edit.Range.End)
	if err != nil {
		t.Fatalf("edit end: %v", err)
	}
	return text[start:end]
}

func editLineIsAnnotation(t *testing.T, text string, edit protocol.TextEdit) bool {
	t.Helper()
	lines := strings.Split(text, "\n")
	line := int(edit.Range.Start.Line)
	if line < 0 || line >= len(lines) {
		t.Fatalf("edit line = %d, want within document", line)
	}
	return strings.HasPrefix(strings.TrimSpace(lines[line]), "---@")
}

func TestEndToEndAnnotationParamRenameAndNavigation(t *testing.T) {
	text := strings.Join([]string{
		"pico-8 cartridge // pico-8.com",
		"version 42",
		"__lua__",
		"---@param dx? number",
		"function move(dx)",
		"  return dx",
		"end",
		"",
	}, "\n")
	h, root, ctx, cancel := openAnnotationCart(t, text)
	defer cancel()
	defer h.exit(t, true)

	annotationPos := positionAt(t, text, "---@param dx?", len("---@param "))
	prepare, err := h.server.PrepareRename(ctx, &protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     annotationPos,
		},
	})
	if err != nil {
		t.Fatalf("prepareRename: %v", err)
	}
	placeholder, ok := prepare.(*protocol.PrepareRenamePlaceholder)
	if !ok || placeholder.Placeholder != "dx" {
		t.Fatalf("prepareRename = %T %+v, want dx", prepare, prepare)
	}
	if got := placeholder.Range.End.Character - placeholder.Range.Start.Character; got != 2 {
		t.Fatalf("prepareRename range width = %d, want 2 (optional marker excluded)", got)
	}

	rename, err := h.server.Rename(ctx, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     annotationPos,
		},
		NewName: "delta",
	})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	edits := rename.Changes[root]
	if len(edits) != 3 {
		t.Fatalf("rename edits = %+v, want annotation, declaration, and use", edits)
	}
	if got := textEditRangeText(t, text, edits[0]); got != "dx" {
		t.Fatalf("annotation edit text = %q, want dx", got)
	}
	for _, edit := range edits {
		if edit.NewText != "delta" {
			t.Fatalf("edit = %+v, want delta", edit)
		}
	}

	declPos := positionAt(t, text, "function move(dx)", len("function move("))
	def, err := h.server.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     annotationPos,
		},
	})
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	locs := definitionLocations(t, def)
	if len(locs) != 1 || locs[0].Range.Start != declPos {
		t.Fatalf("definition = %+v, want parameter declaration", locs)
	}

	refs, err := h.server.References(ctx, &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     declPos,
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: true},
	})
	if err != nil {
		t.Fatalf("references: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("references = %+v, want 3", refs)
	}
	refs, err = h.server.References(ctx, &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     declPos,
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	})
	if err != nil {
		t.Fatalf("references without declaration: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("references without declaration = %+v, want 2", refs)
	}

	highlights, err := h.server.DocumentHighlight(ctx, &protocol.DocumentHighlightParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     annotationPos,
		},
	})
	if err != nil {
		t.Fatalf("documentHighlight: %v", err)
	}
	if len(highlights) != 3 {
		t.Fatalf("highlights = %+v, want 3", highlights)
	}
}

func TestEndToEndAnnotationTypeRenameAndNavigation(t *testing.T) {
	text := strings.Join([]string{
		"pico-8 cartridge // pico-8.com",
		"version 42",
		"__lua__",
		"---@class Player",
		"local Player = {}",
		"---@param p Player",
		"---@return Player",
		"local function identity(p)",
		"  return p",
		"end",
		"---@type Player",
		"local player = Player",
		"",
	}, "\n")
	h, root, ctx, cancel := openAnnotationCart(t, text)
	defer cancel()
	defer h.exit(t, true)

	refPos := positionAt(t, text, "---@param p Player", len("---@param p "))
	def, err := h.server.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     refPos,
		},
	})
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	locs := definitionLocations(t, def)
	classPos := positionAt(t, text, "---@class Player", len("---@class "))
	if len(locs) != 1 || locs[0].Range.Start != classPos {
		t.Fatalf("definition = %+v, want class declaration", locs)
	}

	refs, err := h.server.References(ctx, &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     refPos,
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: true},
	})
	if err != nil {
		t.Fatalf("references: %v", err)
	}
	if len(refs) != 4 {
		t.Fatalf("references = %+v, want class declaration and three type references", refs)
	}

	prepare, err := h.server.PrepareRename(ctx, &protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     refPos,
		},
	})
	if err != nil {
		t.Fatalf("prepareRename: %v", err)
	}
	placeholder, ok := prepare.(*protocol.PrepareRenamePlaceholder)
	if !ok || placeholder.Placeholder != "Player" {
		t.Fatalf("prepareRename = %T %+v, want Player", prepare, prepare)
	}

	rename, err := h.server.Rename(ctx, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     refPos,
		},
		NewName: "Hero",
	})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	edits := rename.Changes[root]
	if len(edits) != 4 {
		t.Fatalf("rename edits = %+v, want four annotation edits", edits)
	}
	for _, edit := range edits {
		if got := textEditRangeText(t, text, edit); got != "Player" {
			t.Fatalf("edit text = %q, want Player", got)
		}
		if !editLineIsAnnotation(t, text, edit) {
			t.Fatalf("rename edited code instead of an annotation: %+v", edit)
		}
	}
}

func TestEndToEndAnnotationFieldRenameAndNavigation(t *testing.T) {
	text := strings.Join([]string{
		"pico-8 cartridge // pico-8.com",
		"version 42",
		"__lua__",
		"---@class Player",
		"---@field hp? number",
		"---@field mp number",
		"local Player = {}",
		"Player.hp = 1",
		"---@type Player",
		"local p = Player",
		"p.hp = 2",
		"---@type Player",
		"local q = Player",
		"print(q.hp)",
		"",
	}, "\n")
	h, root, ctx, cancel := openAnnotationCart(t, text)
	defer cancel()
	defer h.exit(t, true)

	annotationPos := positionAt(t, text, "---@field hp?", len("---@field "))
	qhpPos := positionAt(t, text, "print(q.hp)", len("print(q."))

	prepare, err := h.server.PrepareRename(ctx, &protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     annotationPos,
		},
	})
	if err != nil {
		t.Fatalf("prepareRename: %v", err)
	}
	placeholder, ok := prepare.(*protocol.PrepareRenamePlaceholder)
	if !ok || placeholder.Placeholder != "hp" {
		t.Fatalf("prepareRename = %T %+v, want hp", prepare, prepare)
	}
	if got := placeholder.Range.End.Character - placeholder.Range.Start.Character; got != 2 {
		t.Fatalf("prepareRename range width = %d, want 2 (optional marker excluded)", got)
	}

	def, err := h.server.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     qhpPos,
		},
	})
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	locs := definitionLocations(t, def)
	if len(locs) != 1 || locs[0].Range.Start != annotationPos {
		t.Fatalf("definition = %+v, want ---@field hp", locs)
	}

	refs, err := h.server.References(ctx, &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     qhpPos,
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: true},
	})
	if err != nil {
		t.Fatalf("references: %v", err)
	}
	if len(refs) != 4 {
		t.Fatalf("references = %+v, want annotation, Player.hp, p.hp, q.hp", refs)
	}

	highlights, err := h.server.DocumentHighlight(ctx, &protocol.DocumentHighlightParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     qhpPos,
		},
	})
	if err != nil {
		t.Fatalf("documentHighlight: %v", err)
	}
	if len(highlights) != 4 {
		t.Fatalf("highlights = %+v, want 4", highlights)
	}

	rename, err := h.server.Rename(ctx, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     qhpPos,
		},
		NewName: "vigor",
	})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	edits := rename.Changes[root]
	if len(edits) != 4 {
		t.Fatalf("rename edits = %+v, want four", edits)
	}
	if got := textEditRangeText(t, text, edits[0]); got != "hp" {
		t.Fatalf("annotation edit text = %q, want hp (optional marker excluded)", got)
	}
	for _, edit := range edits {
		if edit.NewText != "vigor" {
			t.Fatalf("edit = %+v, want vigor", edit)
		}
	}

	_, err = h.server.Rename(ctx, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     qhpPos,
		},
		NewName: "mp",
	})
	if err == nil {
		t.Fatal("rename to an existing class field succeeded, want error")
	}
}

func TestEndToEndAnnotationCastRename(t *testing.T) {
	text := strings.Join([]string{
		"pico-8 cartridge // pico-8.com",
		"version 42",
		"__lua__",
		"---@class Player",
		"local Player = {}",
		"---@type Player|nil",
		"local p = nil",
		"---@cast p Player",
		"print(p)",
		"",
	}, "\n")
	h, root, ctx, cancel := openAnnotationCart(t, text)
	defer cancel()
	defer h.exit(t, true)

	declPos := positionAt(t, text, "local p = nil", len("local "))
	castPos := positionAt(t, text, "---@cast p Player", len("---@cast "))
	def, err := h.server.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     castPos,
		},
	})
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	locs := definitionLocations(t, def)
	if len(locs) != 1 || locs[0].Range.Start != declPos {
		t.Fatalf("definition = %+v, want local declaration", locs)
	}

	rename, err := h.server.Rename(ctx, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     declPos,
		},
		NewName: "px",
	})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	edits := rename.Changes[root]
	if len(edits) != 3 {
		t.Fatalf("rename edits = %+v, want declaration, cast, and print", edits)
	}
	foundCast := false
	for _, edit := range edits {
		if edit.Range.Start == castPos {
			foundCast = true
		}
	}
	if !foundCast {
		t.Fatalf("cast target was not renamed: %+v", edits)
	}
}
