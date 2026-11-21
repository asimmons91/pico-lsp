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

func codeActionsAt(t *testing.T, h *harness, ctx context.Context, u uri.URI, text, needle string) []protocol.CommandOrCodeAction {
	t.Helper()
	pos := positionAt(t, text, needle, 0)
	actions, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
		Range:        protocol.Range{Start: pos, End: pos},
	})
	if err != nil {
		t.Fatalf("code action: %v", err)
	}
	return actions
}

func applyProtocolEdit(t *testing.T, text string, edit protocol.TextEdit) string {
	t.Helper()
	index := source.New([]byte(text), protocol.PositionEncodingKindUTF16)
	start, end, err := index.RangeOffsets(edit.Range)
	if err != nil {
		t.Fatalf("range offsets: %v", err)
	}
	if start < 0 || end > len(text) || start > end {
		t.Fatalf("invalid edit range %+v", edit.Range)
	}
	return text[:start] + edit.NewText + text[end:]
}

func TestM23TokenSavingCodeActions(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
score = 0
print(score)
score = score + 1
local a = peek(0x5f00)
`
	expected := []struct {
		kind     string
		needle   string
		title    string
		newText  string
		wantLine string
	}{
		{
			kind:     "refactor.rewrite.token-saving.print",
			needle:   "print(score)",
			title:    "Use ? print shorthand (saves 1 token)",
			newText:  "?score",
			wantLine: "?score",
		},
		{
			kind:     "refactor.rewrite.token-saving.compound",
			needle:   "score = score + 1",
			title:    "Use compound assignment += (saves 2 tokens)",
			newText:  "score += 1",
			wantLine: "score += 1",
		},
		{
			kind:     "refactor.rewrite.token-saving.peek",
			needle:   "peek(0x5f00)",
			title:    "Use @ peek shorthand (saves 1 token)",
			newText:  "@0x5f00",
			wantLine: "local a = @0x5f00",
		},
	}
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	for _, tt := range expected {
		actions := codeActionsAt(t, h, ctx, root, text, tt.needle)
		action := findActionByKind(t, actions, tt.kind)
		if action == nil {
			t.Fatalf("missing %s in %+v", tt.kind, actions)
		}
		if action.Title != tt.title {
			t.Fatalf("%s title = %q, want %q", tt.kind, action.Title, tt.title)
		}
		if action.IsPreferred != nil {
			t.Fatalf("%s isPreferred = %v, want nil", tt.kind, *action.IsPreferred)
		}
		if action.Edit == nil {
			t.Fatalf("%s has no edit", tt.kind)
		}
		edits := action.Edit.Changes[root]
		if len(edits) != 1 || edits[0].NewText != tt.newText {
			t.Fatalf("%s edits = %+v", tt.kind, edits)
		}
		wantStart := positionAt(t, text, tt.needle, 0)
		wantEnd := positionAt(t, text, tt.needle, len(tt.needle))
		if edits[0].Range.Start != wantStart || edits[0].Range.End != wantEnd {
			t.Fatalf("%s range = %+v, want %+v-%+v", tt.kind, edits[0].Range, wantStart, wantEnd)
		}
		edited := applyProtocolEdit(t, text, edits[0])
		if !strings.Contains(edited, tt.wantLine) {
			t.Fatalf("%s result missing %q:\n%s", tt.kind, tt.wantLine, edited)
		}
	}
}

func TestM23SelectionReturnsOneActionPerCandidate(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
x = x + 1
print(x)
local a = peek(x)
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	start := positionAt(t, text, "x = x + 1", 0)
	end := positionAt(t, text, "peek(x)", len("peek(x)"))
	actions, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        protocol.Range{Start: start, End: end},
	})
	if err != nil {
		t.Fatalf("code action: %v", err)
	}
	var kinds []string
	seen := make(map[string]bool)
	for _, action := range actions {
		candidate, ok := action.(*protocol.CodeAction)
		if !ok || candidate.Kind == nil || !strings.HasPrefix(string(*candidate.Kind), "refactor.rewrite.token-saving.") {
			continue
		}
		kinds = append(kinds, string(*candidate.Kind))
		if seen[string(*candidate.Kind)] {
			t.Fatalf("duplicate kind %s", *candidate.Kind)
		}
		seen[string(*candidate.Kind)] = true
		if candidate.Edit != nil && len(candidate.Edit.Changes[root]) != 1 {
			t.Fatalf("%s merged edits: %+v", *candidate.Kind, candidate.Edit)
		}
	}
	want := []string{
		"refactor.rewrite.token-saving.compound",
		"refactor.rewrite.token-saving.print",
		"refactor.rewrite.token-saving.peek",
	}
	if strings.Join(kinds, ",") != strings.Join(want, ",") {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
}

func TestM23IncludedFileEditsStayInOriginalFile(t *testing.T) {
	cartText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib.lua\"\n"
	libText := "print(score)\n"
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", cartText)
	lib := writeTemp(t, dir, "lib.lua", libText)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, cartText)
	openDoc(t, h, ctx, lib, libText)

	actions := codeActionsAt(t, h, ctx, lib, libText, "print(score)")
	action := findActionByKind(t, actions, "refactor.rewrite.token-saving.print")
	if action == nil {
		t.Fatalf("missing print action in %+v", actions)
	}
	if len(action.Edit.Changes) != 1 {
		t.Fatalf("edit touches %d files", len(action.Edit.Changes))
	}
	edits := action.Edit.Changes[lib]
	if len(edits) != 1 || edits[0].NewText != "?score" {
		t.Fatalf("lib edits = %+v", edits)
	}
	if _, ok := action.Edit.Changes[root]; ok {
		t.Fatalf("edit touched root cart: %+v", action.Edit.Changes)
	}
}

func TestM23DisabledAndMetaAndVirtual(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nprint(x)\n"
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	actions := codeActionsAt(t, h, ctx, root, text, "print(x)")
	if findActionByKind(t, actions, "refactor.rewrite.token-saving.print") == nil {
		t.Fatalf("missing print action before disabling refactors")
	}
	h.srv.cfg.ApplyInitializationOptions(protocol.LSPAny([]byte(`{"pico8":{"refactors":{"enabled":false}}}`)))
	actions = codeActionsAt(t, h, ctx, root, text, "print(x)")
	if findActionByKind(t, actions, "refactor.rewrite.token-saving.print") != nil {
		t.Fatalf("token-saving action offered while refactors disabled: %+v", actions)
	}
	h.srv.cfg.ApplyInitializationOptions(protocol.LSPAny([]byte(`{"pico8":{"refactors":{"enabled":true}}}`)))

	metaText := "---@meta\nprint(x)\n"
	metaPath := writeTemp(t, dir, "stub.lua", metaText)
	openDoc(t, h, ctx, metaPath, metaText)
	actions = codeActionsAt(t, h, ctx, metaPath, metaText, "print(x)")
	if findActionByKind(t, actions, "refactor.rewrite.token-saving.print") != nil {
		t.Fatalf("token-saving action offered in meta file: %+v", actions)
	}

	virtual := uri.URI("pico8-cart:/tmp/cart.p8.png")
	actions, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: virtual},
		Range:        protocol.Range{},
	})
	if err != nil {
		t.Fatalf("virtual code action: %v", err)
	}
	if len(actions) != 0 {
		t.Fatalf("virtual cart offered actions: %+v", actions)
	}
}

func TestM23SnapshotFreshnessGate(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nprint(x)\n"
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.srv.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: root, LanguageID: protocol.LanguageKind("pico8"), Version: 1, Text: text,
		},
	}); err != nil {
		t.Fatalf("didOpen: %v", err)
	}
	nav, ok := h.srv.navigationFor(ctx, root)
	if !ok {
		t.Fatal("no navigation context")
	}
	if !h.srv.snapshotFresh(nav) {
		t.Fatal("fresh analysis reported stale")
	}
	if err := h.srv.docs.Change(root, 2, []protocol.TextDocumentContentChangeEvent{
		&protocol.TextDocumentContentChangeWholeDocument{Text: text + "-- changed\n"},
	}); err != nil {
		t.Fatalf("change: %v", err)
	}
	if h.srv.snapshotFresh(nav) {
		t.Fatal("stale analysis reported fresh")
	}
}
