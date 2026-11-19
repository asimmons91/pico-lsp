package lsp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/config"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/testutil"
)

func TestCartLimitDiagnosticsBoundaries(t *testing.T) {
	text := "x=1\n"
	u := uri.File("/tmp/cart.p8")
	file := &cart.File{ID: 0, URI: u, Text: text, Index: source.New([]byte(text), protocol.PositionEncodingKindUTF16)}
	makeSnap := func(tokens, chars int) *cart.Snapshot {
		return &cart.Snapshot{Cart: &cart.Cart{RootURI: u, Files: []*cart.File{file}, Tokens: tokens, Chars: chars}}
	}
	settings := config.Default()
	cases := []struct {
		name   string
		tokens int
		chars  int
		codes  []string
		errors bool
	}{
		{name: "below", tokens: TokenLimit*9/10 - 1, chars: CharLimit*9/10 - 1},
		{name: "warn boundary", tokens: TokenLimit * 9 / 10, chars: CharLimit * 9 / 10, codes: []string{"char-limit", "token-limit"}},
		{name: "at limit warning", tokens: TokenLimit, chars: CharLimit, codes: []string{"char-limit", "token-limit"}},
		{name: "over limit error", tokens: TokenLimit + 1, chars: CharLimit + 1, codes: []string{"char-limit", "token-limit"}, errors: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := cartLimitDiagnostics(makeSnap(tc.tokens, tc.chars), settings)
			var codes []string
			for _, diag := range diags {
				codes = append(codes, string(diag.Code.(protocol.String)))
				if tc.errors == false && diag.Severity == protocol.DiagnosticSeverityError {
					t.Fatalf("unexpected error severity: %+v", diag)
				}
				if tc.errors && diag.Severity != protocol.DiagnosticSeverityError {
					t.Fatalf("expected error severity: %+v", diag)
				}
			}
			if strings.Join(codes, ",") != strings.Join(tc.codes, ",") {
				t.Fatalf("codes = %v, want %v", codes, tc.codes)
			}
		})
	}
	settings.Limits.Diagnostics = false
	if diags := cartLimitDiagnostics(makeSnap(TokenLimit+1, CharLimit+1), settings); len(diags) != 0 {
		t.Fatalf("disabled limit diagnostics = %+v", diags)
	}
}

func TestCodeLensBudget(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nx = 1\n"
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	snap, _, ok := h.srv.carts.SnapshotFor(ctx, root)
	if !ok {
		t.Fatal("missing snapshot")
	}
	lenses, err := h.server.CodeLens(ctx, &protocol.CodeLensParams{TextDocument: protocol.TextDocumentIdentifier{URI: root}})
	if err != nil {
		t.Fatalf("code lens: %v", err)
	}
	if len(lenses) != 1 {
		t.Fatalf("lenses = %+v", lenses)
	}
	compressed, _ := snap.CompressedSize()
	want := fmt.Sprintf("%d / %d tokens · %d / %d chars · %d / %d compressed",
		snap.Cart.Tokens, TokenLimit, snap.Cart.Chars, CharLimit, compressed, CompressedLimit)
	if lenses[0].Command.Title != want {
		t.Fatalf("lens = %q, want %q", lenses[0].Command.Title, want)
	}
	if lenses[0].Command.Command != CommandShowCartInfo || len(lenses[0].Command.Arguments) != 1 {
		t.Fatalf("lens command = %+v", lenses[0].Command)
	}
	var arg string
	if err := protocol.Unmarshal(lenses[0].Command.Arguments[0], &arg); err != nil || arg != root.String() {
		t.Fatalf("lens argument = %q, err=%v", arg, err)
	}

	includeText := "function included() end\n"
	include := writeTemp(t, dir, "included.lua", includeText)
	text2 := strings.Replace(text, "x = 1", "#include \"included.lua\"\nx = 1", 1)
	root2 := writeTemp(t, dir, "cart2.p8", text2)
	openDoc(t, h, ctx, root2, text2)
	lenses, err = h.server.CodeLens(ctx, &protocol.CodeLensParams{TextDocument: protocol.TextDocumentIdentifier{URI: root2}})
	if err != nil || len(lenses) != 1 {
		t.Fatalf("include-root lens = %+v, err=%v", lenses, err)
	}

	// Included files do not get a lens.
	openDoc(t, h, ctx, include, includeText)
	lenses, err = h.server.CodeLens(ctx, &protocol.CodeLensParams{TextDocument: protocol.TextDocumentIdentifier{URI: include}})
	if err != nil || len(lenses) != 0 {
		t.Fatalf("include lens = %+v, err=%v", lenses, err)
	}
}

func TestCodeLensDisabled(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nx = 1\n"
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	h := initializeWithOptions(t, `{"pico8":{"limits":{"codeLens":false}}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	lenses, err := h.server.CodeLens(ctx, &protocol.CodeLensParams{TextDocument: protocol.TextDocumentIdentifier{URI: root}})
	if err != nil || len(lenses) != 0 {
		t.Fatalf("disabled lenses = %+v, err=%v", lenses, err)
	}
}

func TestCartInfoRequestAndCommand(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nx = 1\n"
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	params, _ := protocol.Marshal(map[string]any{"textDocument": map[string]string{"uri": root.String()}})
	result, err := h.srv.Request(ctx, RequestCartInfo, protocol.LSPAny(params))
	if err != nil {
		t.Fatalf("cartInfo: %v", err)
	}
	info, ok := result.(CartInfo)
	if !ok {
		t.Fatalf("cartInfo result = %T", result)
	}
	if info.RootURI != root || info.TokenLimit != TokenLimit || info.CharLimit != CharLimit || len(info.Files) != 1 {
		t.Fatalf("cartInfo = %+v", info)
	}

	var wireInfo CartInfo
	if _, err := h.clientConn.Call(ctx, RequestCartInfo, map[string]any{
		"textDocument": map[string]string{"uri": root.String()},
	}, &wireInfo); err != nil {
		t.Fatalf("cartInfo over the wire: %v", err)
	}
	if wireInfo.RootURI != root || wireInfo.Tokens != info.Tokens {
		t.Fatalf("wire cartInfo = %+v, want %+v", wireInfo, info)
	}

	commandArgs, _ := protocol.Marshal(root.String())
	raw, err := h.server.ExecuteCommand(ctx, &protocol.ExecuteCommandParams{
		Command:   CommandShowCartInfo,
		Arguments: []protocol.LSPAny{commandArgs},
	})
	if err != nil {
		t.Fatalf("showCartInfo: %v", err)
	}
	var commandInfo CartInfo
	if err := protocol.Unmarshal(raw, &commandInfo); err != nil {
		t.Fatalf("decode command result: %v", err)
	}
	if commandInfo.RootURI != root {
		t.Fatalf("command info = %+v", commandInfo)
	}
	select {
	case message := <-h.client.messages:
		if !strings.Contains(message.Message, "tokens") {
			t.Fatalf("message = %+v", message)
		}
	case <-time.After(time.Second):
		t.Fatal("no showMessage")
	}

	unknown := uri.File(dir + "/nope.p8")
	params, _ = protocol.Marshal(map[string]any{"textDocument": map[string]string{"uri": unknown.String()}})
	if _, err := h.server.Request(ctx, RequestCartInfo, protocol.LSPAny(params)); err == nil {
		t.Fatal("unknown cart request succeeded")
	}
}

func TestM5CodeActions(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
local function move(dx, dy)
  return dx + dy
end
move(1, 2)
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	actions, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        protocol.Range{Start: protocol.Position{Line: 3}, End: protocol.Position{Line: 3, Character: 4}},
	})
	if err != nil {
		t.Fatalf("code action: %v", err)
	}
	var annotation *protocol.CodeAction
	for _, action := range actions {
		candidate, ok := action.(*protocol.CodeAction)
		if ok && strings.Contains(candidate.Title, "annotations") {
			annotation = candidate
			break
		}
	}
	if annotation == nil {
		t.Fatalf("actions = %+v", actions)
	}
	edit := annotation.Edit
	if edit == nil || len(edit.Changes[root]) != 1 {
		t.Fatalf("annotation edit = %+v", edit)
	}
	insert := edit.Changes[root][0].NewText
	for _, want := range []string{"---@param dx number", "---@param dy number", "---@return number"} {
		if !strings.Contains(insert, want) {
			t.Fatalf("annotation edit = %q, missing %q", insert, want)
		}
	}

	diag := protocol.Diagnostic{
		Range:    protocol.Range{Start: protocol.Position{Line: 3}, End: protocol.Position{Line: 3}},
		Severity: protocol.DiagnosticSeverityWarning,
		Code:     protocol.String("assign-type-mismatch"),
	}
	actions, err = h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        diag.Range,
		Context:      protocol.CodeActionContext{Diagnostics: []protocol.Diagnostic{diag}},
	})
	if err != nil {
		t.Fatalf("suppression action: %v", err)
	}
	found := false
	for _, action := range actions {
		candidate, ok := action.(*protocol.CodeAction)
		if ok && strings.Contains(candidate.Title, "Suppress") {
			found = true
			if candidate.Edit == nil || !strings.Contains(candidate.Edit.Changes[root][0].NewText, "disable-next-line") {
				t.Fatalf("suppression edit = %+v", candidate.Edit)
			}
		}
	}
	if !found {
		t.Fatalf("missing suppression action in %+v", actions)
	}

	syntaxDiag := protocol.Diagnostic{
		Range:    protocol.Range{Start: protocol.Position{Line: 1}, End: protocol.Position{Line: 1}},
		Severity: protocol.DiagnosticSeverityError,
		Code:     protocol.String("syntax"),
	}
	actions, err = h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        syntaxDiag.Range,
		Context:      protocol.CodeActionContext{Diagnostics: []protocol.Diagnostic{syntaxDiag}},
	})
	if err != nil {
		t.Fatalf("syntax code action: %v", err)
	}
	for _, action := range actions {
		if candidate, ok := action.(*protocol.CodeAction); ok && strings.Contains(candidate.Title, "Suppress") {
			t.Fatalf("syntax diagnostic got a suppression action: %+v", candidate)
		}
	}

	quickfix := []protocol.CodeActionKind{protocol.CodeActionKindQuickFix}
	actions, err = h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        protocol.Range{Start: protocol.Position{Line: 3}, End: protocol.Position{Line: 3, Character: 4}},
		Context: protocol.CodeActionContext{
			Only:        quickfix,
			Diagnostics: []protocol.Diagnostic{diag},
		},
	})
	if err != nil {
		t.Fatalf("filtered code action: %v", err)
	}
	for _, action := range actions {
		if candidate, ok := action.(*protocol.CodeAction); ok && strings.Contains(candidate.Title, "annotations") {
			t.Fatalf("annotation refactor returned for quickfix-only request: %+v", candidate)
		}
	}
}

func TestM5CreateMissingIncludeAction(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib/new.lua\"\nx = 1\n"
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
		if code, _ := params.Diagnostics[i].Code.(protocol.String); string(code) == "include-missing" {
			missing = &params.Diagnostics[i]
			break
		}
	}
	if missing == nil {
		t.Fatalf("diagnostics = %+v", params.Diagnostics)
	}
	if missing.Range.Start.Character != 10 || missing.Range.End.Character != 21 {
		t.Fatalf("missing include range = %+v, want path range 10..21", missing.Range)
	}
	actions, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        missing.Range,
		Context:      protocol.CodeActionContext{Diagnostics: []protocol.Diagnostic{*missing}},
	})
	if err != nil {
		t.Fatalf("create include action: %v", err)
	}
	found := false
	for _, action := range actions {
		candidate, ok := action.(*protocol.CodeAction)
		if ok && strings.Contains(candidate.Title, "Create missing") {
			found = true
			if candidate.Edit == nil || len(candidate.Edit.DocumentChanges) != 1 {
				t.Fatalf("create include edit = %+v", candidate.Edit)
			}
		}
	}
	if !found {
		t.Fatalf("missing create action in %+v", actions)
	}
}

func TestM5WatchedFileDeletionMarksIncludeMissing(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib.lua\"\nx = 1\n"
	libText := "function lib() end\n"
	root := writeTemp(t, dir, "cart.p8", text)
	lib := writeTemp(t, dir, "lib.lua", libText)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	if err := os.Remove(lib.FsPath()); err != nil {
		t.Fatal(err)
	}
	if err := h.srv.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{{URI: lib, Type: protocol.FileChangeTypeDeleted}},
	}); err != nil {
		t.Fatalf("watched files: %v", err)
	}
	snap, _, ok := h.srv.carts.SnapshotFor(ctx, root)
	if !ok {
		t.Fatal("missing snapshot")
	}
	found := false
	for _, include := range snap.Cart.Includes {
		if include.Path == "lib.lua" && include.Missing {
			found = true
		}
	}
	if !found {
		t.Fatalf("includes = %+v, want missing lib.lua", snap.Cart.Includes)
	}
}

func TestM5LintDiagnosticsInLSP(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nfunction _update() end\nfunction _update60() end\n"
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	h := initializeWithOptions(t, "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	params := waitDiagnostic(t, h.client, root)
	found := false
	for _, diag := range params.Diagnostics {
		if code, _ := diag.Code.(protocol.String); string(code) == "pico8-callback-precedence" {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v", params.Diagnostics)
	}

	disabledRoot := writeTemp(t, t.TempDir(), "cart.p8", text)
	h2 := initializeWithOptions(t, `{"pico8":{"diagnostics":{"callbackPrecedence":"off"}}}`)
	openDoc(t, h2, ctx, disabledRoot, text)
	_, semantics, _, ok := h2.srv.carts.SemanticsFor(ctx, disabledRoot)
	if !ok {
		t.Fatal("missing disabled semantics")
	}
	for _, diag := range semantics.Diagnostics {
		if diag.Code == "pico8-callback-precedence" {
			t.Fatalf("disabled lint still emitted: %+v", diag)
		}
	}
}

func TestM5IncludeDiagnosticVariants(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "shared/lib.lua", "function shared() end\n")
	writeTemp(t, dir, "data.txt", "not lua\n")
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"../shared/lib.lua\"\n#include \"data.txt\"\n#include \"../shared/lib.lua\"\nx = 1\n"
	root := writeTemp(t, dir, "cart.p8", text)
	h := initializeWithOptions(t, `{"pico8":{"diagnostics":{"includeDuplicate":"hint"}}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	params := waitDiagnostic(t, h.client, root)
	codes := diagnosticCodes(params)
	for _, code := range []string{"include-outside-workspace", "include-unusual-type", "include-duplicate"} {
		if !containsString(codes, code) {
			t.Errorf("diagnostics = %v, missing %s", codes, code)
		}
	}
}

func TestM5WatchedFileCreateAndChange(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib.lua\"\nx = 1\n"
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	snap, _, ok := h.srv.carts.SnapshotFor(ctx, root)
	if !ok || len(snap.Cart.Includes) == 0 || !snap.Cart.Includes[0].Missing {
		t.Fatalf("initial includes = %+v", snap.Cart.Includes)
	}

	lib := writeTemp(t, dir, "lib.lua", "function lib() end\n")
	if err := h.srv.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{{URI: lib, Type: protocol.FileChangeTypeCreated}},
	}); err != nil {
		t.Fatalf("created watched file: %v", err)
	}
	snap, _, ok = h.srv.carts.SnapshotFor(ctx, root)
	if !ok || snap.Cart.Includes[0].Missing {
		t.Fatalf("after create includes = %+v", snap.Cart.Includes)
	}

	if err := os.WriteFile(lib.FsPath(), []byte("function broken(\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := h.srv.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{{URI: lib, Type: protocol.FileChangeTypeChanged}},
	}); err != nil {
		t.Fatalf("changed watched file: %v", err)
	}
	snap, fileID, ok := h.srv.carts.SnapshotFor(ctx, lib)
	if !ok {
		t.Fatal("missing snapshot after change")
	}
	if fileID != 1 || len(snap.Errors) == 0 {
		t.Fatalf("snapshot root=%s fileID=%d errors=%+v", snap.Cart.RootURI, fileID, snap.Errors)
	}

}

func TestCartInfoForIncludedFile(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib.lua\"\nx = 1\n"
	root := writeTemp(t, dir, "cart.p8", text)
	lib := writeTemp(t, dir, "lib.lua", "function lib() end\n")
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	params, _ := protocol.Marshal(map[string]any{"textDocument": map[string]string{"uri": lib.String()}})
	result, err := h.srv.Request(ctx, RequestCartInfo, protocol.LSPAny(params))
	if err != nil {
		t.Fatalf("cartInfo included file: %v", err)
	}
	info := result.(CartInfo)
	if info.RootURI != root || len(info.Includes) != 1 {
		t.Fatalf("included cartInfo = %+v", info)
	}
}

func TestM5WatchedFilesRegistration(t *testing.T) {
	h := newHarness(t, newTestClient())
	dynamic := true
	h.initialize(t, &protocol.InitializeParams{
		Capabilities: protocol.ClientCapabilities{
			Workspace: &protocol.WorkspaceClientCapabilities{
				DidChangeWatchedFiles: &protocol.DidChangeWatchedFilesClientCapabilities{DynamicRegistration: &dynamic},
			},
		},
	})
	select {
	case params := <-h.client.registrations:
		if len(params.Registrations) != 1 || params.Registrations[0].Method != protocol.MethodWorkspaceDidChangeWatchedFiles {
			t.Fatalf("registrations = %+v", params.Registrations)
		}
	case <-time.After(time.Second):
		t.Fatal("file watcher registration not sent")
	}
}

func TestM5CartInfoPerformance(t *testing.T) {
	testutil.SkipUnlessPerf(t)
	if raceDetectorEnabled || testing.CoverMode() != "" {
		t.Skip("timing targets are not meaningful under the race detector or coverage instrumentation")
	}
	text := largeCartText()
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	snap, _, ok := h.srv.carts.SnapshotFor(ctx, root)
	if !ok {
		t.Fatal("missing snapshot")
	}
	measure := func(name string, fn func()) time.Duration {
		const iterations = 50
		samples := make([]time.Duration, 0, iterations)
		for i := 0; i < iterations; i++ {
			start := time.Now()
			fn()
			samples = append(samples, time.Since(start))
		}
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		best := samples[0]
		t.Logf("%s: best %v, median %v", name, best, samples[len(samples)/2])
		return best
	}
	lens := measure("code lens", func() {
		if _, err := h.srv.CodeLens(ctx, &protocol.CodeLensParams{TextDocument: protocol.TextDocumentIdentifier{URI: root}}); err != nil {
			t.Fatal(err)
		}
	})
	info := measure("cart info", func() { _ = cartInfoFor(snap) })
	if lens > time.Millisecond || info > time.Millisecond {
		t.Fatalf("cart info targets missed: lens=%v info=%v", lens, info)
	}
}

func TestM5CreateIncludeRefusesOutsideWorkspace(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"../outside/missing.lua\"\nx = 1\n"
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	params := waitDiagnostic(t, h.client, root)
	var missing *protocol.Diagnostic
	for i := range params.Diagnostics {
		if code, _ := params.Diagnostics[i].Code.(protocol.String); string(code) == "include-missing" {
			missing = &params.Diagnostics[i]
			break
		}
	}
	if missing == nil {
		t.Fatalf("diagnostics = %+v", params.Diagnostics)
	}
	actions, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        missing.Range,
		Context:      protocol.CodeActionContext{Diagnostics: []protocol.Diagnostic{*missing}},
	})
	if err != nil {
		t.Fatalf("code action: %v", err)
	}
	for _, action := range actions {
		if candidate, ok := action.(*protocol.CodeAction); ok && strings.Contains(candidate.Title, "Create missing") {
			t.Fatalf("offered create action outside workspace: %+v", candidate)
		}
	}
}

func TestCartLimitRelatedInformation(t *testing.T) {
	rootText := "x=1\n"
	includeText := "y=2\n"
	rootURI := uri.File("/tmp/related/cart.p8")
	includeURI := uri.File("/tmp/related/lib.lua")
	rootFile := &cart.File{ID: 0, URI: rootURI, Text: rootText, Index: source.New([]byte(rootText), protocol.PositionEncodingKindUTF16)}
	includeFile := &cart.File{ID: 1, URI: includeURI, Text: includeText, Index: source.New([]byte(includeText), protocol.PositionEncodingKindUTF16)}
	snap := &cart.Snapshot{Cart: &cart.Cart{
		RootURI:    rootURI,
		Files:      []*cart.File{rootFile, includeFile},
		Tokens:     TokenLimit + 1,
		Chars:      CharLimit + 1,
		FileTokens: map[source.FileID]int{0: 10, 1: TokenLimit},
		FileChars:  map[source.FileID]int{0: 10, 1: CharLimit},
	}}
	diags := cartLimitDiagnostics(snap, config.Default())
	if len(diags) != 2 {
		t.Fatalf("diagnostics = %+v", diags)
	}
	for _, diag := range diags {
		if len(diag.RelatedInformation) != 1 {
			t.Fatalf("related information = %+v", diag.RelatedInformation)
		}
		if diag.RelatedInformation[0].Location.URI != includeURI {
			t.Fatalf("related URI = %s", diag.RelatedInformation[0].Location.URI)
		}
	}
}

func TestM5NestedIncludeNotDoubleReported(t *testing.T) {
	dir := t.TempDir()
	libText := "#include \"nested-missing.lua\"\nfunction lib() end\n"
	writeTemp(t, dir, "lib.lua", libText)
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib.lua\"\nx = 1\n"
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	params := waitDiagnostic(t, h.client, uri.File(filepath.Join(dir, "lib.lua")))
	codes := diagnosticCodes(params)
	if !containsString(codes, "include-nested") {
		t.Fatalf("diagnostics = %v, want include-nested", codes)
	}
	if containsString(codes, "include-missing") {
		t.Fatalf("nested directive reported as missing: %v", codes)
	}
}

func TestM5WatchedUnknownFileIgnored(t *testing.T) {
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	unknown := uri.File(filepath.Join(t.TempDir(), "unrelated.lua"))
	if err := h.srv.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{{URI: unknown, Type: protocol.FileChangeTypeCreated}},
	}); err != nil {
		t.Fatalf("watched files: %v", err)
	}
	h.srv.carts.mu.Lock()
	_, cached := h.srv.carts.byRoot[unknown]
	h.srv.carts.mu.Unlock()
	if cached {
		t.Fatal("unknown watched file was analyzed")
	}
}

func TestM5WatchedMissingP8IncludeAppears(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib.p8\"\nx = 1\n"
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	snap, _, ok := h.srv.carts.SnapshotFor(ctx, root)
	if !ok || len(snap.Cart.Includes) != 1 || !snap.Cart.Includes[0].Missing {
		t.Fatalf("initial includes = %+v", snap.Cart.Includes)
	}

	libText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nfunction lib() end\n"
	lib := writeTemp(t, dir, "lib.p8", libText)
	if err := h.srv.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{{URI: lib, Type: protocol.FileChangeTypeCreated}},
	}); err != nil {
		t.Fatalf("created watched file: %v", err)
	}
	snap, _, ok = h.srv.carts.SnapshotFor(ctx, root)
	if !ok {
		t.Fatal("missing snapshot after create")
	}
	if snap.Cart.Includes[0].Missing {
		t.Fatalf("include still missing after create: %+v", snap.Cart.Includes[0])
	}
	if snap.Cart.Tokens == 0 || len(snap.Cart.Files) < 2 {
		t.Fatalf("cart was not reanalyzed: tokens=%d files=%d", snap.Cart.Tokens, len(snap.Cart.Files))
	}
}

func TestM5CartLimitDiagnosticsPublished(t *testing.T) {
	var b strings.Builder
	b.WriteString("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n")
	for i := 0; i < 11000; i++ {
		b.WriteString("x = 1\n")
	}
	text := b.String()
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	params := waitDiagnostic(t, h.client, root)
	codes := diagnosticCodes(params)
	for _, code := range []string{"token-limit", "char-limit"} {
		if !containsString(codes, code) {
			t.Fatalf("diagnostics = %v, missing %s", codes, code)
		}
	}
}

func TestM5WatcherRegistrationAndConfigurationPullTogether(t *testing.T) {
	client := newTestClient()
	client.configValues = []protocol.LSPAny{
		protocol.LSPAny([]byte(`{"typeChecking":"strict"}`)),
	}
	h := newHarness(t, client)
	dynamic := true
	h.initialize(t, &protocol.InitializeParams{
		Capabilities: protocol.ClientCapabilities{
			Workspace: &protocol.WorkspaceClientCapabilities{
				Configuration:         boolPtr(true),
				DidChangeWatchedFiles: &protocol.DidChangeWatchedFilesClientCapabilities{DynamicRegistration: &dynamic},
			},
		},
	})

	select {
	case params := <-h.client.registrations:
		if len(params.Registrations) != 1 || params.Registrations[0].Method != protocol.MethodWorkspaceDidChangeWatchedFiles {
			t.Fatalf("registrations = %+v", params.Registrations)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("file watcher registration not sent")
	}
	select {
	case <-h.client.configCalls:
	case <-time.After(2 * time.Second):
		t.Fatal("configuration pull not sent")
	}
	waitFor(t, "pulled configuration", func() bool {
		return h.srv.cfg.Settings().TypeChecking == config.TypeCheckingStrict
	})
}

func TestM5SharedLuaOnlyOwnerPublishesDiagnostics(t *testing.T) {
	dir := t.TempDir()
	sharedText := "local x = 1\nreturn x\nprint(x)\nuses_root_only()\n"
	shared := writeTemp(t, dir, "shared.lua", sharedText)
	aText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"shared.lua\"\nfunction uses_root_only() end\n"
	bText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"shared.lua\"\n"
	a := writeTemp(t, dir, "a.p8", aText)
	b := writeTemp(t, dir, "b.p8", bText)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, a, aText)
	waitFor(t, "owner analysis", func() bool {
		h.srv.carts.mu.Lock()
		owner := h.srv.carts.byFile[shared]
		h.srv.carts.mu.Unlock()
		return owner == a
	})
	waitFor(t, "owner diagnostics", func() bool {
		h.srv.diags.mu.Lock()
		_, ok := h.srv.diags.last[shared]
		h.srv.diags.mu.Unlock()
		return ok
	})

	openDoc(t, h, ctx, b, bText)
	waitFor(t, "non-owner analysis", func() bool {
		h.srv.carts.mu.Lock()
		_, ok := h.srv.carts.byRoot[b]
		h.srv.carts.mu.Unlock()
		return ok
	})
	if _, err := h.server.Hover(ctx, &protocol.HoverParams{}); err != nil {
		t.Fatalf("barrier: %v", err)
	}

	h.srv.carts.mu.Lock()
	owner := h.srv.carts.byFile[shared]
	h.srv.carts.mu.Unlock()
	if owner != a {
		t.Fatalf("shared owner = %s, want %s", owner, a)
	}
	h.srv.diags.mu.Lock()
	last := h.srv.diags.last[shared]
	h.srv.diags.mu.Unlock()
	for _, diag := range last.items {
		if code, _ := diag.Code.(protocol.String); string(code) == "undefined-global" {
			t.Fatalf("non-owner diagnostics published: %+v", last.items)
		}
	}

	if err := h.server.DidClose(ctx, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: a},
	}); err != nil {
		t.Fatalf("close owner: %v", err)
	}
	if err := os.Remove(a.FsPath()); err != nil {
		t.Fatal(err)
	}
	if err := h.srv.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{{URI: a, Type: protocol.FileChangeTypeDeleted}},
	}); err != nil {
		t.Fatalf("delete owner: %v", err)
	}
	waitFor(t, "ownership after owner deletion", func() bool {
		h.srv.carts.mu.Lock()
		owner := h.srv.carts.byFile[shared]
		h.srv.carts.mu.Unlock()
		return owner == b
	})
	waitFor(t, "new owner diagnostics", func() bool {
		h.srv.diags.mu.Lock()
		last := h.srv.diags.last[shared]
		h.srv.diags.mu.Unlock()
		for _, diag := range last.items {
			if code, _ := diag.Code.(protocol.String); string(code) == "undefined-global" {
				return true
			}
		}
		return false
	})
}

func TestM5BudgetLensOmittedWhenCountsUnavailable(t *testing.T) {
	text := "not a png cart"
	root := writeTemp(t, t.TempDir(), "cart.p8.png", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	lenses, err := h.server.CodeLens(ctx, &protocol.CodeLensParams{TextDocument: protocol.TextDocumentIdentifier{URI: root}})
	if err != nil {
		t.Fatalf("code lens: %v", err)
	}
	if len(lenses) != 0 {
		t.Fatalf("unavailable-count lens = %+v", lenses)
	}
}

func TestM5CartInfoTabIndexesAreZeroBased(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nx = 1\n-->8\ny = 2\n"
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	params, _ := protocol.Marshal(map[string]any{"textDocument": map[string]string{"uri": root.String()}})
	result, err := h.srv.Request(ctx, RequestCartInfo, protocol.LSPAny(params))
	if err != nil {
		t.Fatalf("cartInfo: %v", err)
	}
	info := result.(CartInfo)
	if len(info.Tabs) != 2 || info.Tabs[0].Index != 0 || info.Tabs[1].Index != 1 {
		t.Fatalf("tabs = %+v, want zero-based indexes", info.Tabs)
	}
	if info.Tabs[0].StartLine != 3 || info.Tabs[0].EndLine != 3 {
		t.Fatalf("tab 0 = %+v, want inclusive 3..3", info.Tabs[0])
	}
	if info.Tabs[1].StartLine != 4 || info.Tabs[1].EndLine != 5 {
		t.Fatalf("tab 1 = %+v, want marker line 4 through 5", info.Tabs[1])
	}
}

func TestM5CreateIncludeActionInAnotherWorkspaceFolder(t *testing.T) {
	base := t.TempDir()
	gameDir := filepath.Join(base, "game")
	sharedDir := filepath.Join(base, "shared")
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"../shared/missing.lua\"\nx = 1\n"
	root := writeTemp(t, gameDir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{
		WorkspaceFolders: protocol.NewNullable([]protocol.WorkspaceFolder{
			{URI: uri.File(gameDir), Name: "game"},
			{URI: uri.File(sharedDir), Name: "shared"},
		}),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	params := waitDiagnostic(t, h.client, root)
	codes := diagnosticCodes(params)
	if containsString(codes, "include-outside-workspace") {
		t.Fatalf("diagnostics = %v, unexpected outside-workspace warning", codes)
	}
	var missing *protocol.Diagnostic
	for i := range params.Diagnostics {
		if code, _ := params.Diagnostics[i].Code.(protocol.String); string(code) == "include-missing" {
			missing = &params.Diagnostics[i]
			break
		}
	}
	if missing == nil {
		t.Fatalf("diagnostics = %+v, missing include-missing", params.Diagnostics)
	}
	actions, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Range:        missing.Range,
		Context:      protocol.CodeActionContext{Diagnostics: []protocol.Diagnostic{*missing}},
	})
	if err != nil {
		t.Fatalf("code action: %v", err)
	}
	for _, action := range actions {
		if candidate, ok := action.(*protocol.CodeAction); ok && strings.Contains(candidate.Title, "Create missing") {
			return
		}
	}
	t.Fatalf("missing create action in %+v", actions)
}

func TestM5IncludePathRangeWithApostrophe(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib's file.lua\"\nx = 1\n"
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	params := waitDiagnostic(t, h.client, root)
	for _, diag := range params.Diagnostics {
		if code, _ := diag.Code.(protocol.String); string(code) != "include-missing" {
			continue
		}
		if diag.Range.Start.Character != 10 || diag.Range.End.Character != 24 {
			t.Fatalf("include path range = %+v, want 10..24", diag.Range)
		}
		return
	}
	t.Fatalf("diagnostics = %+v, missing include-missing", params.Diagnostics)
}
