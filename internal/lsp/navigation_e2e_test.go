package lsp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/source"
)

func positionAt(t *testing.T, text, needle string, offset int) protocol.Position {
	t.Helper()
	index := strings.Index(text, needle)
	if index < 0 {
		t.Fatalf("needle %q not found", needle)
	}
	pos, err := source.New([]byte(text), protocol.PositionEncodingKindUTF16).Position(index + offset)
	if err != nil {
		t.Fatalf("position: %v", err)
	}
	return pos
}

func definitionLocations(t *testing.T, result protocol.DefinitionResult) []protocol.Location {
	t.Helper()
	switch r := result.(type) {
	case protocol.LocationSlice:
		return r
	case protocol.DefinitionLinkSlice:
		// protocol's union decoder selects the link arm for an empty array
		// because both arms have the same JSON shape.
		out := make([]protocol.Location, 0, len(r))
		for _, link := range r {
			out = append(out, protocol.Location{URI: link.TargetURI, Range: link.TargetRange})
		}
		return out
	case *protocol.Location:
		if r == nil {
			return nil
		}
		return []protocol.Location{*r}
	case nil:
		return nil
	default:
		t.Fatalf("definition result = %T", result)
		return nil
	}
}

func readNavigationFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "navigation", filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("read navigation fixture %s: %v", name, err)
	}
	return string(data)
}

func newNavigationFixture(t *testing.T) (dir string, root uri.URI, rootText string, include uri.URI, includeText string) {
	t.Helper()
	dir = t.TempDir()
	rootText = readNavigationFixture(t, "cart.p8")
	includeText = readNavigationFixture(t, "lib/player.lua")
	writeTemp(t, dir, "lib/util.lua", readNavigationFixture(t, "lib/util.lua"))
	include = writeTemp(t, dir, "lib/player.lua", includeText)
	root = writeTemp(t, dir, "cart.p8", rootText)
	return dir, root, rootText, include, includeText
}

func TestEndToEndDefinitionReferencesAndHighlights(t *testing.T) {
	dir, root, rootText, include, includeText := newNavigationFixture(t)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{
		WorkspaceFoldersInitializeParams: protocol.WorkspaceFoldersInitializeParams{
			WorkspaceFolders: protocol.NewNullable([]protocol.WorkspaceFolder{{URI: uri.File(dir), Name: "fixture"}}),
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	// Definition: root call -> include declaration.
	callPos := positionAt(t, rootText, "make_player()", 0)
	result, err := h.server.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     callPos,
		},
	})
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	locs := definitionLocations(t, result)
	if len(locs) != 1 {
		t.Fatalf("definition locations = %+v, want one", locs)
	}
	if locs[0].URI != include {
		t.Fatalf("definition URI = %s, want %s", locs[0].URI, include)
	}
	wantName := positionAt(t, includeText, "make_player", 0)
	if locs[0].Range.Start != wantName {
		t.Fatalf("definition start = %+v, want %+v", locs[0].Range.Start, wantName)
	}

	// References include the declaration and every cross-tab use.
	declPos := positionAt(t, rootText, "player = make_player()", 0)
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
		t.Fatalf("references = %+v, want 3 (decl, update call, print)", refs)
	}
	withoutDecl, err := h.server.References(ctx, &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     declPos,
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	})
	if err != nil {
		t.Fatalf("references without decl: %v", err)
	}
	if len(withoutDecl) != 2 {
		t.Fatalf("references without decl = %+v, want 2", withoutDecl)
	}

	// References from an include-declared symbol reach the root cart.
	includeRefs, err := h.server.References(ctx, &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: include},
			Position:     positionAt(t, includeText, "make_player", 0),
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: true},
	})
	if err != nil {
		t.Fatalf("include references: %v", err)
	}
	if len(includeRefs) != 2 {
		t.Fatalf("include references = %+v, want declaration and root call", includeRefs)
	}

	// Document highlights stay within the requested document (both tabs).
	highlights, err := h.server.DocumentHighlight(ctx, &protocol.DocumentHighlightParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     positionAt(t, rootText, "print(player)", len("print(")),
		},
	})
	if err != nil {
		t.Fatalf("documentHighlight: %v", err)
	}
	if len(highlights) != 3 {
		t.Fatalf("highlights = %+v, want 3", highlights)
	}
	writes := 0
	for _, highlight := range highlights {
		if highlight.Kind == protocol.DocumentHighlightKindWrite {
			writes++
		}
	}
	if writes != 1 {
		t.Fatalf("writes = %d, want 1", writes)
	}

	// A built-in has no definition, references, or rename.
	builtinPos := positionAt(t, rootText, "print(player)", 0)
	result, err = h.server.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     builtinPos,
		},
	})
	if err != nil {
		t.Fatalf("builtin definition: %v", err)
	}
	if locs := definitionLocations(t, result); len(locs) != 0 {
		t.Fatalf("builtin definition = %+v, want none", locs)
	}
	prepare, err := h.server.PrepareRename(ctx, &protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     builtinPos,
		},
	})
	if err != nil {
		t.Fatalf("prepareRename builtin: %v", err)
	}
	if prepare != nil {
		t.Fatalf("prepareRename builtin = %+v, want nil", prepare)
	}
	_, err = h.server.Rename(ctx, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     builtinPos,
		},
		NewName: "println",
	})
	if err == nil {
		t.Fatal("rename builtin succeeded, want error")
	}

	h.exit(t, true)
}

func TestEndToEndRenameAcrossFiles(t *testing.T) {
	_, root, rootText, include, includeText := newNavigationFixture(t)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	callPos := positionAt(t, rootText, "make_player()", 0)
	prepare, err := h.server.PrepareRename(ctx, &protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     callPos,
		},
	})
	if err != nil {
		t.Fatalf("prepareRename: %v", err)
	}
	placeholder, ok := prepare.(*protocol.PrepareRenamePlaceholder)
	if !ok || placeholder.Placeholder != "make_player" {
		t.Fatalf("prepareRename = %T %+v", prepare, prepare)
	}

	result, err := h.server.Rename(ctx, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     callPos,
		},
		NewName: "spawn_player",
	})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if result == nil || len(result.Changes) != 2 {
		t.Fatalf("rename changes = %+v, want two files", result)
	}
	rootEdits := result.Changes[root]
	if len(rootEdits) != 1 || rootEdits[0].NewText != "spawn_player" {
		t.Fatalf("root edits = %+v", rootEdits)
	}
	includeEdits := result.Changes[include]
	if len(includeEdits) != 1 || includeEdits[0].NewText != "spawn_player" {
		t.Fatalf("include edits = %+v", includeEdits)
	}
	want := positionAt(t, includeText, "make_player", 0)
	if includeEdits[0].Range.Start != want {
		t.Fatalf("include edit start = %+v, want %+v", includeEdits[0].Range.Start, want)
	}

	// Casing: all-uppercase declarations rename to uppercase.
	upper := writeTemp(t, t.TempDir(), "upper.p8", "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nPLAYER=1\nprint(PLAYER)\n")
	openDoc(t, h, ctx, upper, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nPLAYER=1\nprint(PLAYER)\n")
	upperResult, err := h.server.Rename(ctx, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: upper},
			Position:     positionAt(t, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nPLAYER=1\nprint(PLAYER)\n", "PLAYER=1", 0),
		},
		NewName: "hero",
	})
	if err != nil {
		t.Fatalf("uppercase rename: %v", err)
	}
	for _, edit := range upperResult.Changes[upper] {
		if edit.NewText != "HERO" {
			t.Fatalf("uppercase rename edit = %+v, want HERO", edit)
		}
	}

	h.exit(t, true)
}

func TestTypedRenameText(t *testing.T) {
	tests := []struct {
		name        string
		occurrences []analysis.Occurrence
		want        string
	}{
		{name: "uppercase", occurrences: []analysis.Occurrence{{Text: "PLAYER"}, {Text: "PLAYER"}}, want: "HERO"},
		{name: "lowercase", occurrences: []analysis.Occurrence{{Text: "player"}}, want: "hero"},
		{name: "mixed", occurrences: []analysis.Occurrence{{Text: "Player"}, {Text: "PLAYER"}}, want: "hero"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := typedRenameText("hero", tt.occurrences); got != tt.want {
				t.Fatalf("typedRenameText = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEndToEndMethodFieldDefinitionAndRename(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nobj = {}\nfunction obj:update()\n  self.x = 1\nend\nfunction _update()\n  obj:update()\n  print(obj.x)\nend\n"
	root := writeTemp(t, dir, "method.p8", text)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	callPos := positionAt(t, text, "obj:update()", len("obj:"))
	result, err := h.server.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     callPos,
		},
	})
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	locs := definitionLocations(t, result)
	if len(locs) != 1 || locs[0].URI != root {
		t.Fatalf("method definition = %+v, want the declaration in the cart", locs)
	}
	if locs[0].Range.Start != positionAt(t, text, "function obj:update", len("function obj:")) {
		t.Fatalf("method definition start = %+v", locs[0].Range.Start)
	}

	selfDefinition, err := h.server.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     positionAt(t, text, "self.x", 0),
		},
	})
	if err != nil {
		t.Fatalf("self definition: %v", err)
	}
	selfLocs := definitionLocations(t, selfDefinition)
	if len(selfLocs) != 1 || selfLocs[0].Range.Start != positionAt(t, text, "function obj:update", len("function obj:")) {
		t.Fatalf("self definition = %+v, want the method name", selfLocs)
	}

	rename, err := h.server.Rename(ctx, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     callPos,
		},
		NewName: "tick",
	})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	edits := rename.Changes[root]
	if len(edits) != 2 || edits[0].NewText != "tick" || edits[1].NewText != "tick" {
		t.Fatalf("method rename edits = %+v, want declaration and call", edits)
	}

	fieldRename, err := h.server.Rename(ctx, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     positionAt(t, text, "self.x", len("self.")),
		},
		NewName: "y",
	})
	if err != nil {
		t.Fatalf("field rename: %v", err)
	}
	fieldEdits := fieldRename.Changes[root]
	if len(fieldEdits) != 2 || fieldEdits[0].NewText != "y" || fieldEdits[1].NewText != "y" {
		t.Fatalf("field rename edits = %+v, want self.x and obj.x", fieldEdits)
	}

	h.exit(t, true)
}

func TestEndToEndIncludePathDefinition(t *testing.T) {
	_, root, rootText, include, _ := newNavigationFixture(t)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	for _, needle := range []string{"#include \"lib/player.lua\"", "lib/player.lua"} {
		pos := positionAt(t, rootText, needle, 0)
		result, err := h.server.Definition(ctx, &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: root},
				Position:     pos,
			},
		})
		if err != nil {
			t.Fatalf("include definition: %v", err)
		}
		locs := definitionLocations(t, result)
		if len(locs) != 1 || locs[0].URI != include {
			t.Fatalf("include %q definition = %+v, want %s", needle, locs, include)
		}
		if locs[0].Range.Start.Line != 0 || locs[0].Range.Start.Character != 0 {
			t.Fatalf("include definition start = %+v, want 0:0", locs[0].Range.Start)
		}
	}

	h.exit(t, true)
}

func TestEndToEndTopLevelLocalAcrossInclude(t *testing.T) {
	dir := t.TempDir()
	rootText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib/shared.lua\"\nprint(shared)\n"
	includeText := "local shared = 1\n"
	root := writeTemp(t, dir, "cart.p8", rootText)
	include := writeTemp(t, dir, "lib/shared.lua", includeText)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	_, semantics, _, ok := h.srv.carts.SemanticsFor(ctx, root)
	if !ok {
		t.Fatal("no semantics")
	}
	for _, diag := range semantics.Diagnostics {
		if diag.Code == analysis.CodeUndefinedGlobal {
			t.Fatalf("unexpected undefined-global for a top-level local from an include: %+v", diag)
		}
	}
	result, err := h.server.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     positionAt(t, rootText, "shared)", 0),
		},
	})
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	locs := definitionLocations(t, result)
	if len(locs) != 1 || locs[0].URI != include {
		t.Fatalf("definition = %+v, want include %s", locs, include)
	}

	h.exit(t, true)
}

func TestEndToEndPrepareRenameUsesCursorSpelling(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nPlayer = 1\nprint(player)\n"
	root := writeTemp(t, dir, "case.p8", text)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	prepare, err := h.server.PrepareRename(ctx, &protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     positionAt(t, text, "print(player)", len("print(")),
		},
	})
	if err != nil {
		t.Fatalf("prepareRename: %v", err)
	}
	placeholder, ok := prepare.(*protocol.PrepareRenamePlaceholder)
	if !ok || placeholder.Placeholder != "player" {
		t.Fatalf("prepareRename = %T %+v, want placeholder player", prepare, prepare)
	}

	h.exit(t, true)
}

func TestEndToEndGlyphRename(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n⬅ = 1\nprint(⬅)\n"
	root := writeTemp(t, dir, "glyph.p8", text)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	rename, err := h.server.Rename(ctx, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     positionAt(t, text, "⬅ = 1", 0),
		},
		NewName: "⬆",
	})
	if err != nil {
		t.Fatalf("glyph rename: %v", err)
	}
	edits := rename.Changes[root]
	if len(edits) != 2 {
		t.Fatalf("glyph rename edits = %+v, want declaration and use", edits)
	}
	for _, edit := range edits {
		if edit.NewText != "⬆" {
			t.Fatalf("glyph rename edit = %+v, want ⬆", edit)
		}
	}

	h.exit(t, true)
}

func TestEndToEndWorkspaceFoldersChange(t *testing.T) {
	first := uri.File(t.TempDir())
	second := uri.File(t.TempDir())
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{
		WorkspaceFoldersInitializeParams: protocol.WorkspaceFoldersInitializeParams{
			WorkspaceFolders: protocol.NewNullable([]protocol.WorkspaceFolder{{URI: first, Name: "first"}}),
		},
	})
	if err := h.srv.DidChangeWorkspaceFolders(context.Background(), &protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{
			Removed: []protocol.WorkspaceFolder{{URI: first, Name: "first"}},
			Added:   []protocol.WorkspaceFolder{{URI: second, Name: "second"}},
		},
	}); err != nil {
		t.Fatalf("didChangeWorkspaceFolders: %v", err)
	}
	folders := h.srv.WorkspaceFolders()
	if len(folders) != 1 || folders[0] != second {
		t.Fatalf("workspace folders = %+v, want %s", folders, second)
	}

	h.exit(t, true)
}

func TestEndToEndMissingIncludeDefinitionIsEmpty(t *testing.T) {
	dir := t.TempDir()
	rootText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"missing.lua\"\n"
	root := writeTemp(t, dir, "cart.p8", rootText)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	result, err := h.server.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     positionAt(t, rootText, "missing.lua", 0),
		},
	})
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if locs := definitionLocations(t, result); len(locs) != 0 {
		t.Fatalf("missing include definition = %+v, want empty", locs)
	}

	h.exit(t, true)
}

func TestEndToEndWorkspaceSymbols(t *testing.T) {
	dir, root, rootText, _, _ := newNavigationFixture(t)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{
		WorkspaceFoldersInitializeParams: protocol.WorkspaceFoldersInitializeParams{
			WorkspaceFolders: protocol.NewNullable([]protocol.WorkspaceFolder{{URI: uri.File(dir), Name: "fixture"}}),
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	result, err := h.server.Symbols(ctx, &protocol.WorkspaceSymbolParams{Query: "mkpl"})
	if err != nil {
		t.Fatalf("workspace symbols: %v", err)
	}
	symbols, ok := result.(protocol.SymbolInformationSlice)
	if !ok {
		t.Fatalf("workspace symbols result = %T", result)
	}
	found := false
	for _, symbol := range symbols {
		if symbol.Name == "make_player" {
			found = true
		}
	}
	if !found {
		t.Fatalf("workspace symbols = %+v, missing make_player", symbols)
	}

	cancelled, cancelRequest := context.WithCancel(context.Background())
	cancelRequest()
	_, err = h.srv.Symbols(cancelled, &protocol.WorkspaceSymbolParams{Query: ""})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled workspace symbols error = %v, want context.Canceled", err)
	}

	h.exit(t, true)
}

func TestEndToEndWorkspaceSymbolsFallBackToOpenDocuments(t *testing.T) {
	_, root, rootText, _, _ := newNavigationFixture(t)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	result, err := h.server.Symbols(ctx, &protocol.WorkspaceSymbolParams{Query: "make"})
	if err != nil {
		t.Fatalf("workspace symbols: %v", err)
	}
	symbols := result.(protocol.SymbolInformationSlice)
	if len(symbols) == 0 {
		t.Fatal("expected workspace symbols from the open cart")
	}

	h.exit(t, true)
}

func TestEndToEndWorkspaceSymbolLimit(t *testing.T) {
	dir := t.TempDir()
	code := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n"
	for i := 0; i < maxWorkspaceSymbolResults+10; i++ {
		code += fmt.Sprintf("function f%d() end\n", i)
	}
	root := writeTemp(t, dir, "many.p8", code)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{
		WorkspaceFoldersInitializeParams: protocol.WorkspaceFoldersInitializeParams{
			WorkspaceFolders: protocol.NewNullable([]protocol.WorkspaceFolder{{URI: uri.File(dir), Name: "fixture"}}),
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, code)

	result, err := h.server.Symbols(ctx, &protocol.WorkspaceSymbolParams{Query: ""})
	if err != nil {
		t.Fatalf("workspace symbols: %v", err)
	}
	symbols := result.(protocol.SymbolInformationSlice)
	if len(symbols) != maxWorkspaceSymbolResults {
		t.Fatalf("workspace symbols = %d, want %d", len(symbols), maxWorkspaceSymbolResults)
	}

	h.exit(t, true)
}

func TestEndToEndUndefinedGlobalDiagnostics(t *testing.T) {
	dir := t.TempDir()
	rootText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nfunction _update()\n  print(typo)\nend\n"
	root := writeTemp(t, dir, "diag.p8", rootText)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	params := waitDiagnostic(t, h.client, root)
	var found *protocol.Diagnostic
	for i := range params.Diagnostics {
		if params.Diagnostics[i].Code == protocol.String("undefined-global") {
			found = &params.Diagnostics[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("diagnostics = %+v, want undefined-global", params.Diagnostics)
	}
	if found.Severity != protocol.DiagnosticSeverityWarning {
		t.Fatalf("severity = %v, want warning", found.Severity)
	}
	if source, ok := found.Source.Get(); !ok || source != DiagnosticSource {
		t.Fatalf("source = %v, want %s", found.Source, DiagnosticSource)
	}

	h.exit(t, true)
}

func TestEndToEndBuiltinsAndForwardReferencesHaveNoDiagnostics(t *testing.T) {
	dir := t.TempDir()
	rootText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nfunction _update()\n  later()\n  btn(0)\nend\nfunction later() end\n"
	root := writeTemp(t, dir, "clean.p8", rootText)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	_, semantics, _, ok := h.srv.carts.SemanticsFor(ctx, root)
	if !ok {
		t.Fatal("no semantics for clean cart")
	}
	for _, diag := range semantics.Diagnostics {
		if diag.Code == analysis.CodeUndefinedGlobal {
			t.Fatalf("unexpected undefined-global: %+v", diag)
		}
	}

	h.exit(t, true)
}

func TestEndToEndUndefinedGlobalInInclude(t *testing.T) {
	dir := t.TempDir()
	rootText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib/bad.lua\"\n"
	includeText := "function bad()\n  return typo\nend\n"
	root := writeTemp(t, dir, "cart.p8", rootText)
	include := writeTemp(t, dir, "lib/bad.lua", includeText)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	params := waitDiagnostic(t, h.client, include)
	found := false
	for _, diag := range params.Diagnostics {
		if diag.Code == protocol.String("undefined-global") {
			found = true
		}
	}
	if !found {
		t.Fatalf("include diagnostics = %+v, want undefined-global", params.Diagnostics)
	}

	h.exit(t, true)
}

func TestEndToEndUndefinedGlobalSeverityConfiguration(t *testing.T) {
	dir := t.TempDir()
	rootText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nprint(typo)\n"
	root := writeTemp(t, dir, "diag.p8", rootText)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{
		InitializationOptions: protocol.LSPAny([]byte(`{"pico8":{"diagnostics":{"undefinedGlobal":"error"}}}`)),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	params := waitDiagnostic(t, h.client, root)
	found := false
	for _, diag := range params.Diagnostics {
		if diag.Code == protocol.String("undefined-global") {
			if diag.Severity != protocol.DiagnosticSeverityError {
				t.Fatalf("severity = %v, want error", diag.Severity)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v, want undefined-global", params.Diagnostics)
	}

	h.exit(t, true)
}

func TestEndToEndRenameCollisionReturnsError(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nlocal x = 1\ndo\n  local y = 2\n  print(x, y)\nend\n"
	root := writeTemp(t, dir, "collision.p8", text)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	_, err := h.server.Rename(ctx, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     positionAt(t, text, "x = 1", 0),
		},
		NewName: "y",
	})
	if err == nil {
		t.Fatal("rename collision succeeded, want error")
	}
	var rpcErr *jsonrpc2.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error = %T, want JSON-RPC error", err)
	}

	h.exit(t, true)
}
