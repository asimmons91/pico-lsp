package lsp

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
)

func findActionByKind(t *testing.T, actions []protocol.CommandOrCodeAction, kind string) *protocol.CodeAction {
	t.Helper()
	for _, action := range actions {
		candidate, ok := action.(*protocol.CodeAction)
		if !ok || candidate.Kind == nil {
			continue
		}
		if string(*candidate.Kind) == kind {
			return candidate
		}
	}
	return nil
}

func TestM14CapabilitiesExposeRemoveUnused(t *testing.T) {
	h := newHarness(t, newTestClient())
	result := h.initialize(t, &protocol.InitializeParams{})
	if !containsString(result.Capabilities.ExecuteCommandProvider.Commands, CommandRemoveUnused) {
		t.Fatalf("executeCommandProvider = %+v", result.Capabilities.ExecuteCommandProvider)
	}
}

func TestM14ExtractFunctionAndGenerateClass(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
local a = 1
local b = 2
local c = a + b
local p = {x = 1, y = 2}
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	start := positionAt(t, text, "a + b", 0)
	end := positionAt(t, text, "a + b", len("a + b"))
	actions, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        protocol.Range{Start: start, End: end},
	})
	if err != nil {
		t.Fatalf("code action: %v", err)
	}
	extract := findActionByKind(t, actions, "refactor.extract.function")
	if extract == nil {
		t.Fatalf("missing extract action in %+v", actions)
	}
	if extract.Edit == nil || len(extract.Edit.Changes[root]) != 2 {
		t.Fatalf("extract edit = %+v", extract.Edit)
	}
	var inserted, replaced bool
	for _, edit := range extract.Edit.Changes[root] {
		if strings.Contains(edit.NewText, "local function extracted(a, b)") {
			inserted = true
		}
		if edit.NewText == "extracted(a, b)" {
			replaced = true
		}
	}
	if !inserted || !replaced {
		t.Fatalf("extract edits = %+v", extract.Edit.Changes[root])
	}

	classPos := positionAt(t, text, "{x = 1", 1)
	actions, err = h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        protocol.Range{Start: classPos, End: classPos},
	})
	if err != nil {
		t.Fatalf("class code action: %v", err)
	}
	class := findActionByKind(t, actions, "refactor.rewrite.class")
	if class == nil {
		t.Fatalf("missing class action in %+v", actions)
	}
	edits := class.Edit.Changes[root]
	if len(edits) != 1 || !strings.Contains(edits[0].NewText, "---@field x number") ||
		!strings.Contains(edits[0].NewText, "---@field y number") {
		t.Fatalf("class edits = %+v", edits)
	}
}

func TestM14IncludedFileEditsStayInOriginalFile(t *testing.T) {
	cartText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib.lua\"\nlocal total = 0\n"
	libText := "score = 0\nlocal p = {x = 1}\n"
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", cartText)
	lib := writeTemp(t, dir, "lib.lua", libText)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, cartText)
	openDoc(t, h, ctx, lib, libText)

	scorePos := positionAt(t, libText, "score", 0)
	actions, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: lib},
		Range:        protocol.Range{Start: scorePos, End: scorePos},
	})
	if err != nil {
		t.Fatalf("code action: %v", err)
	}
	local := findActionByKind(t, actions, "refactor.rewrite.local")
	if local == nil {
		t.Fatalf("missing local action in %+v", actions)
	}
	if len(local.Edit.Changes) != 1 || len(local.Edit.Changes[lib]) != 1 {
		t.Fatalf("local edit = %+v", local.Edit)
	}
	edit := local.Edit.Changes[lib][0]
	if edit.Range.Start != (protocol.Position{Line: 0, Character: 0}) ||
		edit.Range.End != (protocol.Position{Line: 0, Character: 0}) || edit.NewText != "local " {
		t.Fatalf("local edit = %+v", edit)
	}
	if _, ok := local.Edit.Changes[root]; ok {
		t.Fatalf("local edit touched the cart: %+v", local.Edit)
	}
	if !strings.HasPrefix(cartText, "pico-8 cartridge") || strings.Count(cartText, "#include") != 1 {
		t.Fatalf("include order changed: %q", cartText)
	}

	classPos := positionAt(t, libText, "{x = 1", 1)
	actions, err = h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: lib},
		Range:        protocol.Range{Start: classPos, End: classPos},
	})
	if err != nil {
		t.Fatalf("class code action: %v", err)
	}
	class := findActionByKind(t, actions, "refactor.rewrite.class")
	if class == nil {
		t.Fatalf("missing class action in %+v", actions)
	}
	if len(class.Edit.Changes) != 1 || len(class.Edit.Changes[lib]) != 1 {
		t.Fatalf("class edit = %+v", class.Edit)
	}
}

func TestM14GlobalToLocalRefusesCrossFileUse(t *testing.T) {
	cartText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib.lua\"\nprint(score)\n"
	libText := "score = 0\n"
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", cartText)
	lib := writeTemp(t, dir, "lib.lua", libText)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, cartText)
	openDoc(t, h, ctx, lib, libText)

	pos := positionAt(t, libText, "score", 0)
	actions, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: lib},
		Range:        protocol.Range{Start: pos, End: pos},
	})
	if err != nil {
		t.Fatalf("code action: %v", err)
	}
	if action := findActionByKind(t, actions, "refactor.rewrite.local"); action != nil {
		t.Fatalf("global-to-local crossed a file boundary: %+v", action)
	}
}

func TestM14ShorthandConversion(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nif x then\n  play()\nend\n"
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	pos := positionAt(t, text, "if x", 0)
	actions, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        protocol.Range{Start: pos, End: pos},
	})
	if err != nil {
		t.Fatalf("code action: %v", err)
	}
	shorthand := findActionByKind(t, actions, "refactor.rewrite.shorthand")
	if shorthand == nil {
		t.Fatalf("missing shorthand action in %+v", actions)
	}
	edits := shorthand.Edit.Changes[root]
	if len(edits) != 1 || edits[0].NewText != "if (x) play()" {
		t.Fatalf("shorthand edits = %+v", edits)
	}
}

func TestM14RemoveUnusedRequiresCommandConfirmation(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nlocal unused = 1\nlocal used = 2\nprint(used)\n"
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	client := newTestClient()
	h := newHarness(t, client)
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	pos := positionAt(t, text, "unused", 0)
	actions, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        protocol.Range{Start: pos, End: pos},
	})
	if err != nil {
		t.Fatalf("code action: %v", err)
	}
	remove := findActionByKind(t, actions, "quickfix.pico-lsp.remove-unused")
	if remove == nil {
		t.Fatalf("missing remove action in %+v", actions)
	}
	if remove.Edit != nil {
		t.Fatalf("remove action embedded an edit instead of a command: %+v", remove)
	}
	if remove.Command.Command != CommandRemoveUnused {
		t.Fatalf("command = %q", remove.Command.Command)
	}
	if _, err := h.srv.ExecuteCommand(ctx, &protocol.ExecuteCommandParams{
		Command:   remove.Command.Command,
		Arguments: remove.Command.Arguments,
	}); err != nil {
		t.Fatalf("execute command: %v", err)
	}
	select {
	case applied := <-client.applied:
		edits := applied.Edit.Changes[root]
		if len(edits) != 1 || edits[0].NewText != "" {
			t.Fatalf("applied edit = %+v", applied.Edit)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("workspace/applyEdit was not called")
	}
}

func TestM14MissingFieldsQuickfix(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@class (exact) Point
---@field x number
---@field y number
---@type Point
local p = {}
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	params := waitDiagnostic(t, h.client, root)
	var missing *protocol.Diagnostic
	for i := range params.Diagnostics {
		if code, _ := params.Diagnostics[i].Code.(protocol.String); string(code) == "missing-fields" {
			missing = &params.Diagnostics[i]
			break
		}
	}
	if missing == nil {
		t.Fatalf("missing-fields diagnostic not published: %+v", params.Diagnostics)
	}
	actions, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        missing.Range,
		Context:      protocol.CodeActionContext{Diagnostics: []protocol.Diagnostic{*missing}},
	})
	if err != nil {
		t.Fatalf("code action: %v", err)
	}
	fix := findActionByKind(t, actions, "quickfix.pico-lsp.add-missing-fields")
	if fix == nil {
		t.Fatalf("missing add-fields action in %+v", actions)
	}
	edits := fix.Edit.Changes[root]
	if len(edits) != 1 {
		t.Fatalf("edits = %+v", edits)
	}
	for _, want := range []string{"x = 0", "y = 0"} {
		if !strings.Contains(edits[0].NewText, want) {
			t.Fatalf("edit = %q, missing %q", edits[0].NewText, want)
		}
	}
}

func TestM14StaleAnalysisOffersNoActions(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nlocal a = 1\nlocal b = a + 1\n"
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

	// Change the overlay without notifying the cart manager so the cached
	// analysis is deliberately stale.
	if err := h.srv.docs.Change(root, 2, []protocol.TextDocumentContentChangeEvent{
		&protocol.TextDocumentContentChangeWholeDocument{Text: text + "-- changed\n"},
	}); err != nil {
		t.Fatalf("change: %v", err)
	}
	pos := positionAt(t, text, "a + 1", 0)
	actions, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        protocol.Range{Start: pos, End: pos},
	})
	if err != nil {
		t.Fatalf("code action: %v", err)
	}
	if len(actions) != 0 {
		t.Fatalf("stale analysis offered actions: %+v", actions)
	}
}

func TestM14RefactorsCanBeDisabled(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nlocal a = 1\nlocal b = a + 1\n"
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	h.srv.cfg.ApplyInitializationOptions(protocol.LSPAny([]byte(`{"pico8":{"refactors":{"enabled":false}}}`)))

	pos := positionAt(t, text, "a + 1", 0)
	actions, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        protocol.Range{Start: pos, End: pos},
	})
	if err != nil {
		t.Fatalf("code action: %v", err)
	}
	if findActionByKind(t, actions, "refactor.rewrite.shorthand") != nil {
		t.Fatalf("refactors were offered while disabled: %+v", actions)
	}
	for _, action := range actions {
		if candidate, ok := action.(*protocol.CodeAction); ok && candidate.Kind != nil &&
			strings.HasPrefix(string(*candidate.Kind), "refactor.") {
			t.Fatalf("refactor action offered while disabled: %+v", candidate)
		}
	}
}
