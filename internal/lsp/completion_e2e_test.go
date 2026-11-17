package lsp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

const m3CompletionCart = `pico-8 cartridge // pico-8.com
version 42
__lua__
---@class Player
---@field x number
---@field y number
---@field score? number
local player = {x = 10, y = 20, speed = 1}
player.hp = 3

---@type Player
local other = {}

--- Move the player.
---@param dx number
---@param dy? number
---@return number
function move(dx, dy)
  return dx + (dy or 0)
end

---@deprecated
function old() end

function player:step(dx)
  self.x = self.x + dx
end

function _update()
  local speed = 2
  
  player:step(1)
  circfill(player.x, player.y, 2, 7)
  other.score = 1
  nosuch.field = 1
end
`

func completionItems(t *testing.T, h *harness, ctx context.Context, u uri.URI, position protocol.Position) []protocol.CompletionItem {
	t.Helper()
	result, err := h.server.Completion(ctx, &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     position,
		},
	})
	if err != nil {
		t.Fatalf("completion: %v", err)
	}
	switch r := result.(type) {
	case *protocol.CompletionList:
		return r.Items
	case protocol.CompletionItemSlice:
		return r
	case nil:
		return nil
	default:
		t.Fatalf("completion result type = %T", result)
		return nil
	}
}

func findCompletion(items []protocol.CompletionItem, label string) *protocol.CompletionItem {
	for i := range items {
		if items[i].Label == label {
			return &items[i]
		}
	}
	return nil
}

func itemTextEdit(t *testing.T, item protocol.CompletionItem) protocol.TextEdit {
	t.Helper()
	if item.TextEdit == nil {
		t.Fatalf("%s has no text edit", item.Label)
	}
	edit, ok := item.TextEdit.(*protocol.TextEdit)
	if !ok || edit == nil {
		t.Fatalf("%s text edit type = %T", item.Label, item.TextEdit)
	}
	return *edit
}

func completionLabels(items []protocol.CompletionItem) []string {
	labels := make([]string, 0, len(items))
	for _, item := range items {
		labels = append(labels, item.Label)
	}
	return labels
}

type goldenCompletionItem struct {
	Label            string                       `json:"label"`
	Kind             protocol.CompletionItemKind  `json:"kind"`
	Detail           string                       `json:"detail,omitempty"`
	FilterText       string                       `json:"filterText,omitempty"`
	SortText         string                       `json:"sortText,omitempty"`
	InsertTextFormat protocol.InsertTextFormat    `json:"insertTextFormat"`
	Tags             []protocol.CompletionItemTag `json:"tags,omitempty"`
	TextEdit         *protocol.TextEdit           `json:"textEdit,omitempty"`
}

func goldenCompletionItems(t *testing.T, items []protocol.CompletionItem) []goldenCompletionItem {
	t.Helper()
	out := make([]goldenCompletionItem, 0, len(items))
	for _, item := range items {
		entry := goldenCompletionItem{
			Label:            item.Label,
			Kind:             item.Kind,
			InsertTextFormat: item.InsertTextFormat,
			Tags:             item.Tags,
		}
		if detail, ok := item.Detail.Get(); ok {
			entry.Detail = detail
		}
		if filter, ok := item.FilterText.Get(); ok {
			entry.FilterText = filter
		}
		if sortText, ok := item.SortText.Get(); ok {
			entry.SortText = sortText
		}
		if item.TextEdit != nil {
			edit := itemTextEdit(t, item)
			entry.TextEdit = &edit
		}
		out = append(out, entry)
	}
	return out
}

func TestEndToEndCompletionBuiltinsOrderingAndResolve(t *testing.T) {
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", m3CompletionCart)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, m3CompletionCart)

	// Prefix filtering and built-in metadata.
	pos := positionAt(t, m3CompletionCart, "circfill(player.x", len("circ"))
	items := completionItems(t, h, ctx, root, pos)
	labels := completionLabels(items)
	if len(labels) != 2 || !containsString(labels, "circ") || !containsString(labels, "circfill") {
		t.Fatalf("circ completion labels = %v, want circ and circfill", labels)
	}
	circfill := findCompletion(items, "circfill")
	if circfill == nil {
		t.Fatal("missing circfill completion")
	}
	if circfill.Kind != protocol.CompletionItemKindFunction {
		t.Fatalf("circfill kind = %v, want function", circfill.Kind)
	}
	detail, _ := circfill.Detail.Get()
	if !strings.Contains(detail, "[col]") {
		t.Fatalf("circfill detail = %q, want optional parameter", detail)
	}
	// The replacement range covers only the typed prefix.
	circfillEdit := itemTextEdit(t, *circfill)
	if circfillEdit.NewText != "circfill" {
		t.Fatalf("circfill text edit = %+v", circfillEdit)
	}
	if circfillEdit.Range.Start.Character != pos.Character-4 || circfillEdit.Range.End != pos {
		t.Fatalf("circfill range = %+v, want prefix range ending at %+v", circfillEdit.Range, pos)
	}
	var data completionData
	if err := json.Unmarshal(circfill.Data, &data); err != nil {
		t.Fatalf("completion data: %v", err)
	}
	if data.Kind != "builtin" || !strings.EqualFold(data.Name, "circfill") {
		t.Fatalf("completion data = %+v", data)
	}

	// Resolve attaches docs without changing insertion text.
	before := *circfill
	resolved, err := h.server.CompletionResolve(ctx, &before)
	if err != nil {
		t.Fatalf("completion resolve: %v", err)
	}
	resolvedEdit := itemTextEdit(t, *resolved)
	if resolvedEdit != circfillEdit {
		t.Fatalf("resolve changed insertion: before %+v after %+v", circfillEdit, resolvedEdit)
	}
	if resolved.Documentation == nil {
		t.Fatal("resolve did not attach documentation")
	}
	doc, ok := resolved.Documentation.(*protocol.MarkupContent)
	if !ok {
		t.Fatalf("documentation type = %T", resolved.Documentation)
	}
	for _, want := range []string{"circfill", "Draw a filled circle", "PICO-8 0.2.7 API", "col"} {
		if !strings.Contains(doc.Value, want) {
			t.Fatalf("circfill docs missing %q:\n%s", want, doc.Value)
		}
	}

	goldenJSON(t, "completion/builtin-prefix.json", goldenCompletionItems(t, items))

	// Locals sort before globals, then built-ins, then keywords. Snippets are
	// a separate group after keywords.
	blank := strings.Index(m3CompletionCart, "  \n  player:step(1)")
	pos = positionAt(t, m3CompletionCart, "  player:step(1)", 2)
	if blank < 0 {
		t.Fatal("completion blank line not found")
	}
	items = completionItems(t, h, ctx, root, pos)
	local := findCompletion(items, "speed")
	global := findCompletion(items, "move")
	builtin := findCompletion(items, "circ")
	keyword := findCompletion(items, "and")
	snippet := findCompletion(items, "if then")
	deprecated := findCompletion(items, "old")
	for name, item := range map[string]*protocol.CompletionItem{
		"local speed":  local,
		"global move":  global,
		"builtin circ": builtin,
		"keyword and":  keyword,
		"snippet":      snippet,
		"deprecated":   deprecated,
	} {
		if item == nil {
			t.Fatalf("missing completion item %q", name)
		}
	}
	index := func(item *protocol.CompletionItem) int {
		for i := range items {
			if &items[i] == item {
				return i
			}
		}
		return -1
	}
	if !(index(local) < index(global) && index(global) < index(builtin) && index(builtin) < index(keyword) && index(keyword) < index(snippet)) {
		t.Fatalf("completion order local=%d global=%d builtin=%d keyword=%d snippet=%d",
			index(local), index(global), index(builtin), index(keyword), index(snippet))
	}
	if len(deprecated.Tags) != 1 || deprecated.Tags[0] != protocol.CompletionItemTagDeprecated {
		t.Fatalf("deprecated completion tags = %v", deprecated.Tags)
	}
	for item, prefix := range map[*protocol.CompletionItem]string{
		local:      "0_",
		global:     "1_",
		builtin:    "2_",
		deprecated: "1_",
		keyword:    "3_",
		snippet:    "4_",
	} {
		sortText, _ := item.SortText.Get()
		if !strings.HasPrefix(sortText, prefix) {
			t.Fatalf("%s sortText = %q, want prefix %q", item.Label, sortText, prefix)
		}
	}

	// User docs arrive on resolve as well.
	before = *global
	resolved, err = h.server.CompletionResolve(ctx, &before)
	if err != nil {
		t.Fatalf("resolve move: %v", err)
	}
	doc, ok = resolved.Documentation.(*protocol.MarkupContent)
	if !ok {
		t.Fatalf("move documentation type = %T", resolved.Documentation)
	}
	for _, want := range []string{"Move the player.", "dx", "dy", "optional", "Declared in"} {
		if !strings.Contains(doc.Value, want) {
			t.Fatalf("move docs missing %q:\n%s", want, doc.Value)
		}
	}
}

func TestEndToEndCompletionMembersAndScope(t *testing.T) {
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", m3CompletionCart)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, m3CompletionCart)

	// Dot completion returns fields and methods.
	pos := positionAt(t, m3CompletionCart, "player.x, player.y", len("player."))
	items := completionItems(t, h, ctx, root, pos)
	labels := completionLabels(items)
	for _, want := range []string{"hp", "speed", "score", "step", "x", "y"} {
		if !containsString(labels, want) {
			t.Fatalf("player. labels = %v, missing %q", labels, want)
		}
	}
	if item := findCompletion(items, "step"); item == nil || item.Kind != protocol.CompletionItemKindMethod {
		t.Fatalf("step item = %+v, want method", item)
	}
	if item := findCompletion(items, "x"); item == nil || item.Kind != protocol.CompletionItemKindField {
		t.Fatalf("x item = %+v, want field", item)
	}
	goldenJSON(t, "completion/member-dot.json", goldenCompletionItems(t, items))

	// Colon completion only returns methods.
	pos = positionAt(t, m3CompletionCart, "player:step(1)", len("player:"))
	items = completionItems(t, h, ctx, root, pos)
	if labels := completionLabels(items); len(labels) != 1 || labels[0] != "step" {
		t.Fatalf("player: labels = %v, want step only", labels)
	}

	// self resolves to the method receiver.
	pos = positionAt(t, m3CompletionCart, "self.x = self.x", len("self."))
	items = completionItems(t, h, ctx, root, pos)
	if item := findCompletion(items, "x"); item == nil {
		t.Fatalf("self. labels = %v, missing x", completionLabels(items))
	}

	// A ---@type annotation attaches shallow class fields.
	pos = positionAt(t, m3CompletionCart, "other.score", len("other."))
	items = completionItems(t, h, ctx, root, pos)
	if item := findCompletion(items, "score"); item == nil {
		t.Fatalf("other. labels = %v, missing class field score", completionLabels(items))
	}
	if item := findCompletion(items, "score"); item != nil {
		detail, _ := item.Detail.Get()
		if !strings.Contains(detail, "score") {
			t.Fatalf("class field detail = %q", detail)
		}
	}

	// Unknown receivers return an empty list, not every symbol.
	pos = positionAt(t, m3CompletionCart, "nosuch.field", len("nosuch."))
	items = completionItems(t, h, ctx, root, pos)
	if len(items) != 0 {
		t.Fatalf("unknown receiver completion = %v, want empty", completionLabels(items))
	}

	// A local declared later in the same scope does not hide a visible local
	// declared earlier.
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nlocal x = 1\nprint(x)\nlocal x = 2\n"
	root2 := writeTemp(t, dir, "shadow.p8", text)
	openDoc(t, h, ctx, root2, text)
	pos = positionAt(t, text, "print(x)", len("print("))
	items = completionItems(t, h, ctx, root2, pos)
	if item := findCompletion(items, "x"); item == nil {
		t.Fatalf("shadowed completion = %v, missing earlier x", completionLabels(items))
	}
}

func TestEndToEndCompletionSnippetsAnnotationsAndIncludes(t *testing.T) {
	dir := t.TempDir()
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@class Player
---@field x number
---@ty
local player = {}
---@param name P
function use(name) end
#inc
#include ""
print(player)
`
	root := writeTemp(t, dir, "cart.p8", text)
	writeTemp(t, dir, "lib/player.lua", "function make_player() end\n")
	writeTemp(t, dir, "lib/util.lua", "function util() end\n")
	writeTemp(t, dir, ".hidden/secret.lua", "print(1)\n")
	writeTemp(t, dir, "README.md", "not lua\n")

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	// Snippet completion includes a snippet edit covering the typed prefix.
	pos := positionAt(t, text, "#inc", len("#inc"))
	items := completionItems(t, h, ctx, root, pos)
	includeSnippet := findCompletion(items, "#include")
	if includeSnippet == nil {
		t.Fatalf("#inc completion = %v, missing #include", completionLabels(items))
	}
	if includeSnippet.InsertTextFormat != protocol.InsertTextFormatSnippet {
		t.Fatalf("#include format = %v, want snippet", includeSnippet.InsertTextFormat)
	}
	includeEdit := itemTextEdit(t, *includeSnippet)
	if !strings.Contains(includeEdit.NewText, "${1:") {
		t.Fatalf("#include text edit = %+v", includeEdit)
	}

	// Annotation keyword completion in a ---@ comment.
	pos = positionAt(t, text, "---@ty", len("---@ty"))
	items = completionItems(t, h, ctx, root, pos)
	if findCompletion(items, "type") == nil {
		t.Fatalf("---@ty completion = %v, missing type", completionLabels(items))
	}
	goldenJSON(t, "completion/annotation-keyword.json", goldenCompletionItems(t, items))

	// Annotation type completion offers classes declared in the cart.
	pos = positionAt(t, text, "---@param name P", len("---@param name P"))
	items = completionItems(t, h, ctx, root, pos)
	if findCompletion(items, "Player") == nil {
		t.Fatalf("---@param type completion = %v, missing Player", completionLabels(items))
	}
	goldenJSON(t, "completion/annotation-type.json", goldenCompletionItems(t, items))

	// Include path completion lists nested .lua files only.
	pos = positionAt(t, text, `#include ""`, len(`#include "`))
	items = completionItems(t, h, ctx, root, pos)
	labels := completionLabels(items)
	for _, want := range []string{"lib/player.lua", "lib/util.lua"} {
		if !containsString(labels, want) {
			t.Fatalf("include labels = %v, missing %q", labels, want)
		}
	}
	for _, unwanted := range []string{"README.md", ".hidden/secret.lua", "cart.p8"} {
		if containsString(labels, unwanted) {
			t.Fatalf("include labels = %v, should exclude %q", labels, unwanted)
		}
	}
	item := findCompletion(items, "lib/player.lua")
	if item == nil {
		t.Fatalf("include item = %+v", item)
	}
	edit := itemTextEdit(t, *item)
	if edit.Range.Start.Character != uint32(len(`#include "`)) || edit.NewText != "lib/player.lua" {
		t.Fatalf("include edit = %+v, want path replacement after quote", edit)
	}
	goldenJSON(t, "completion/include-path.json", goldenCompletionItems(t, items))

	// A typed ./ prefix is preserved.
	text2 := strings.Replace(text, `#include ""`, `#include "./"`, 1)
	root2 := writeTemp(t, dir, "dot/cart.p8", text2)
	writeTemp(t, dir, "dot/lib/player.lua", "function make_player() end\n")
	openDoc(t, h, ctx, root2, text2)
	pos = positionAt(t, text2, `#include "./"`, len(`#include "./`))
	items = completionItems(t, h, ctx, root2, pos)
	item = findCompletion(items, "./lib/player.lua")
	if item == nil {
		t.Fatalf("./ include labels = %v", completionLabels(items))
	}
	if edit := itemTextEdit(t, *item); edit.NewText != "./lib/player.lua" {
		t.Fatalf("./ include edit = %+v, want preserved prefix", edit)
	}
}

func TestIncludeCompletionNavigatesUpWithinWorkspace(t *testing.T) {
	workspace := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"../shared/li\"\nprint(1)\n"
	root := writeTemp(t, workspace, "carts/cart.p8", text)
	writeTemp(t, workspace, "shared/lib.lua", "function shared() end\n")
	writeTemp(t, workspace, "shared/util.lua", "function util() end\n")

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{
		WorkspaceFoldersInitializeParams: protocol.WorkspaceFoldersInitializeParams{
			WorkspaceFolders: protocol.NewNullable([]protocol.WorkspaceFolder{{URI: uri.File(workspace), Name: "workspace"}}),
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	pos := positionAt(t, text, `#include "../shared/li"`, len(`#include "../shared/li`))
	items := completionItems(t, h, ctx, root, pos)
	if item := findCompletion(items, "../shared/lib.lua"); item == nil {
		t.Fatalf("upward include labels = %v, missing ../shared/lib.lua", completionLabels(items))
	}

	// An include path that escapes the workspace folder is not offered.
	outside := strings.Replace(text, `../shared/li`, `../../escape`, 1)
	escapeRoot := writeTemp(t, workspace, "carts/escape.p8", outside)
	openDoc(t, h, ctx, escapeRoot, outside)
	pos = positionAt(t, outside, `#include "../../escape"`, len(`#include "../../escape`))
	if items := completionItems(t, h, ctx, escapeRoot, pos); len(items) != 0 {
		t.Fatalf("outside-workspace include = %v, want empty", completionLabels(items))
	}

	// Absolute include paths are not PICO-8 include paths.
	absolute := strings.Replace(text, `../shared/li`, `/etc/`, 1)
	absoluteRoot := writeTemp(t, workspace, "carts/absolute.p8", absolute)
	openDoc(t, h, ctx, absoluteRoot, absolute)
	pos = positionAt(t, absolute, `#include "/etc/"`, len(`#include "/etc/`))
	if items := completionItems(t, h, ctx, absoluteRoot, pos); len(items) != 0 {
		t.Fatalf("absolute include = %v, want empty", completionLabels(items))
	}
}

func TestCompletionSnippetSetCoversM3Idioms(t *testing.T) {
	required := []string{
		"if then", "if else", "for numeric", "for generic", "while",
		"function", "anonymous function", "_init", "_update", "_update60",
		"_draw", "class annotation", "function docs", "#include",
	}
	seen := make(map[string]bool)
	for _, snippet := range completionSnippets {
		seen[snippet.Label] = true
		if snippet.Detail == "" || snippet.Insert == "" {
			t.Fatalf("snippet %q is incomplete: %+v", snippet.Label, snippet)
		}
		if snippet.Label != "#include" && !strings.Contains(snippet.Insert, "$") {
			// Every M3 snippet exposes at least one tab stop. The #include
			// snippet intentionally has no final tab stop.
			t.Fatalf("snippet %q has no placeholders: %q", snippet.Label, snippet.Insert)
		}
	}
	for _, label := range required {
		if !seen[label] {
			t.Fatalf("missing snippet %q", label)
		}
	}
}

func TestCompletionSnippetsAreNotOfferedInStringsOrComments(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n-- comment if\nlocal value = \"if then\"\n"
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	for _, pos := range []protocol.Position{
		positionAt(t, text, "-- comment if", len("-- comment if")),
		positionAt(t, text, `"if then"`, len(`"if then`)),
	} {
		if items := completionItems(t, h, ctx, root, pos); len(items) != 0 {
			t.Fatalf("completion at %+v = %v, want empty", pos, completionLabels(items))
		}
	}
}

func TestIncludeCompletionRespectsFileLimits(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		name := filepath.Join("lib", "f"+string(rune('a'+i))+".lua")
		writeTemp(t, dir, name, "")
	}
	if got := includeFiles(dir, 3); len(got) != 3 {
		t.Fatalf("includeFiles(limit=3) = %v, want 3", got)
	}
}
