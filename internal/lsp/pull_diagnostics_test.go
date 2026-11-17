package lsp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/config"
)

func initializePullHarness(t *testing.T, related bool, options string) *harness {
	t.Helper()
	h := newHarness(t, newTestClient())
	params := &protocol.InitializeParams{
		Capabilities: protocol.ClientCapabilities{
			TextDocument: &protocol.TextDocumentClientCapabilities{
				Diagnostic: &protocol.DiagnosticClientCapabilities{
					RelatedDocumentSupport: boolPtr(related),
				},
			},
		},
	}
	if options != "" {
		params.InitializationOptions = protocol.LSPAny([]byte(options))
	}
	result := h.initialize(t, params)
	if result.Capabilities.DiagnosticProvider == nil {
		t.Fatal("pull diagnostics capability was not advertised")
	}
	return h
}

func pullTestCart(t *testing.T) (root, include uri.URI) {
	t.Helper()
	dir := t.TempDir()
	include = writeTemp(t, dir, "shared.lua", "shared_missing()\n")
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include shared.lua\nroot_missing()\n"
	root = writeTemp(t, dir, "cart.p8", text)
	return root, include
}

func diagnosticMessageCodes(items []protocol.Diagnostic) []string {
	out := make([]string, 0, len(items))
	for _, diag := range items {
		if code, ok := diag.Code.(protocol.String); ok {
			out = append(out, string(code))
		}
	}
	return out
}

func pullDocument(t *testing.T, h *harness, ctx context.Context, u uri.URI, previous *string) protocol.DocumentDiagnosticReport {
	t.Helper()
	report, err := h.server.Diagnostic(ctx, &protocol.DocumentDiagnosticParams{
		TextDocument:     protocol.TextDocumentIdentifier{URI: u},
		PreviousResultID: previous,
	})
	if err != nil {
		t.Fatalf("textDocument/diagnostic: %v", err)
	}
	return report
}

func assertNoPushDiagnostics(t *testing.T, client *testClient) {
	t.Helper()
	select {
	case params := <-client.diagnostics:
		t.Fatalf("unexpected push diagnostics for %s: %v", params.URI, diagnosticMessageCodes(params.Diagnostics))
	case <-time.After(150 * time.Millisecond):
	}
}

func TestPullDiagnosticsCapabilityAdvertisement(t *testing.T) {
	h := newHarness(t, newTestClient())
	result := h.initialize(t, &protocol.InitializeParams{})
	if result.Capabilities.DiagnosticProvider != nil {
		t.Fatal("pull diagnostics advertised without client support")
	}
	h.exit(t, true)

	h = initializePullHarness(t, true, "")
	if h.srv.diagnosticPull != true || h.srv.diagnosticRelated != true {
		t.Fatalf("diagnostic capabilities = pull:%v related:%v", h.srv.diagnosticPull, h.srv.diagnosticRelated)
	}
	h.exit(t, true)
}

func TestPullDiagnosticsFullReportAndRelatedDocuments(t *testing.T) {
	h := initializePullHarness(t, true, "")
	root, include := pullTestCart(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, mustReadFile(t, root))

	report := pullDocument(t, h, ctx, root, nil)
	full, ok := report.(*protocol.RelatedFullDocumentDiagnosticReport)
	if !ok {
		t.Fatalf("report type = %T, want RelatedFullDocumentDiagnosticReport", report)
	}
	if full.Kind != string(protocol.DocumentDiagnosticReportKindFull) {
		t.Fatalf("kind = %q, want full", full.Kind)
	}
	if full.ResultID == nil || *full.ResultID == "" {
		t.Fatal("full report has no result id")
	}
	if !containsString(diagnosticMessageCodes(full.Items), "undefined-global") {
		t.Fatalf("root diagnostic codes = %v, want undefined-global", diagnosticMessageCodes(full.Items))
	}
	if _, duplicate := full.RelatedDocuments[root]; duplicate {
		t.Fatal("relatedDocuments contains the requested document")
	}
	related, ok := full.RelatedDocuments[include]
	if !ok {
		t.Fatalf("relatedDocuments = %v, want %s", full.RelatedDocuments, include)
	}
	relatedFull, ok := related.(*protocol.FullDocumentDiagnosticReport)
	if !ok {
		t.Fatalf("related report type = %T, want FullDocumentDiagnosticReport", related)
	}
	if !containsString(diagnosticMessageCodes(relatedFull.Items), "undefined-global") {
		t.Fatalf("include diagnostic codes = %v, want undefined-global", diagnosticMessageCodes(relatedFull.Items))
	}

	assertNoPushDiagnostics(t, h.client)
	h.exit(t, true)
}

func TestPullDiagnosticsUnchangedAndStaleResultIDs(t *testing.T) {
	h := initializePullHarness(t, false, "")
	root, _ := pullTestCart(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	text := mustReadFile(t, root)
	openDoc(t, h, ctx, root, text)

	first := pullDocument(t, h, ctx, root, nil)
	firstFull, ok := first.(*protocol.RelatedFullDocumentDiagnosticReport)
	if !ok {
		t.Fatalf("first report type = %T, want full", first)
	}
	if firstFull.RelatedDocuments != nil {
		t.Fatal("relatedDocuments returned without related document support")
	}

	unchanged := pullDocument(t, h, ctx, root, firstFull.ResultID)
	unchangedReport, ok := unchanged.(*protocol.RelatedUnchangedDocumentDiagnosticReport)
	if !ok {
		t.Fatalf("second report type = %T, want unchanged", unchanged)
	}
	if unchangedReport.ResultID != *firstFull.ResultID {
		t.Fatalf("unchanged result id = %q, want %q", unchangedReport.ResultID, *firstFull.ResultID)
	}

	stale := "stale-result"
	fullAgain := pullDocument(t, h, ctx, root, &stale)
	if _, ok := fullAgain.(*protocol.RelatedFullDocumentDiagnosticReport); !ok {
		t.Fatalf("stale report type = %T, want full", fullAgain)
	}

	edited := text + "edited_missing()\n"
	if err := h.server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: root},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{wholeChange(edited)},
	}); err != nil {
		t.Fatalf("didChange: %v", err)
	}
	afterEdit := pullDocument(t, h, ctx, root, firstFull.ResultID)
	afterEditFull, ok := afterEdit.(*protocol.RelatedFullDocumentDiagnosticReport)
	if !ok {
		t.Fatalf("edited report type = %T, want full", afterEdit)
	}
	if *afterEditFull.ResultID == *firstFull.ResultID {
		t.Fatal("result id did not change after an edit")
	}

	h.exit(t, true)
}

func TestPullDiagnosticsUnknownDocument(t *testing.T) {
	h := initializePullHarness(t, false, "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.server.Diagnostic(ctx, &protocol.DocumentDiagnosticParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(t.TempDir(), "missing.p8"))},
	})
	var rpcErr *jsonrpc2.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.InvalidParams {
		t.Fatalf("unknown document error = %v, want InvalidParams", err)
	}

	if _, err := h.server.Diagnostic(ctx, &protocol.DocumentDiagnosticParams{}); err == nil {
		t.Fatal("missing textDocument.uri did not fail")
	}

	h.exit(t, true)
}

func TestPullDiagnosticsPushModePublishes(t *testing.T) {
	h := initializePullHarness(t, true, `{"pico8":{"diagnostics":{"mode":"push"}}}`)
	root, _ := pullTestCart(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, mustReadFile(t, root))

	pushed := waitDiagnostic(t, h.client, root)
	if !containsString(diagnosticCodes(pushed), "undefined-global") {
		t.Fatalf("pushed codes = %v, want undefined-global", diagnosticCodes(pushed))
	}

	report := pullDocument(t, h, ctx, root, nil)
	full, ok := report.(*protocol.RelatedFullDocumentDiagnosticReport)
	if !ok {
		t.Fatalf("report type = %T, want full", report)
	}
	if !slices.Equal(diagnosticMessageCodes(full.Items), diagnosticCodes(pushed)) {
		t.Fatalf("pull codes = %v, push codes = %v", diagnosticMessageCodes(full.Items), diagnosticCodes(pushed))
	}

	h.exit(t, true)
}

func TestPullDiagnosticsPullModeFallsBackWithoutCapability(t *testing.T) {
	h := newHarness(t, newTestClient())
	params := &protocol.InitializeParams{
		InitializationOptions: protocol.LSPAny([]byte(`{"pico8":{"diagnostics":{"mode":"pull"}}}`)),
	}
	h.initialize(t, params)

	root, _ := pullTestCart(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, mustReadFile(t, root))

	pushed := waitDiagnostic(t, h.client, root)
	if !containsString(diagnosticCodes(pushed), "undefined-global") {
		t.Fatalf("fallback pushed codes = %v, want undefined-global", diagnosticCodes(pushed))
	}

	h.exit(t, true)
}

func TestPullDiagnosticsTransportTransition(t *testing.T) {
	h := initializePullHarness(t, true, "")
	root, _ := pullTestCart(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, mustReadFile(t, root))
	assertNoPushDiagnostics(t, h.client)

	if err := h.server.DidChangeConfiguration(ctx, &protocol.DidChangeConfigurationParams{
		Settings: protocol.LSPAny([]byte(`{"pico8":{"diagnostics":{"mode":"push"}}}`)),
	}); err != nil {
		t.Fatalf("didChangeConfiguration push: %v", err)
	}
	pushed := waitDiagnostic(t, h.client, root)
	if !containsString(diagnosticCodes(pushed), "undefined-global") {
		t.Fatalf("pushed codes after transition = %v, want undefined-global", diagnosticCodes(pushed))
	}

	if err := h.server.DidChangeConfiguration(ctx, &protocol.DidChangeConfigurationParams{
		Settings: protocol.LSPAny([]byte(`{"pico8":{"diagnostics":{"mode":"pull"}}}`)),
	}); err != nil {
		t.Fatalf("didChangeConfiguration pull: %v", err)
	}
	cleared := waitDiagnostic(t, h.client, root)
	if len(cleared.Diagnostics) != 0 {
		t.Fatalf("clear published %d diagnostics, want 0", len(cleared.Diagnostics))
	}

	h.exit(t, true)
}

func TestPullDiagnosticsMaxProblemsPerDocument(t *testing.T) {
	h := initializePullHarness(t, true, `{"pico8":{"maxProblems":1,"diagnostics":{"mode":"pull"}}}`)
	dir := t.TempDir()
	include := writeTemp(t, dir, "shared.lua", "shared_one()\nshared_two()\n")
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include shared.lua\nroot_one()\nroot_two()\n"
	root := writeTemp(t, dir, "cart.p8", text)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	report := pullDocument(t, h, ctx, root, nil)
	full, ok := report.(*protocol.RelatedFullDocumentDiagnosticReport)
	if !ok {
		t.Fatalf("report type = %T, want full", report)
	}
	if len(full.Items) != 1 {
		t.Fatalf("root items = %d, want 1", len(full.Items))
	}
	related, ok := full.RelatedDocuments[include].(*protocol.FullDocumentDiagnosticReport)
	if !ok {
		t.Fatalf("related report type = %T, want full", full.RelatedDocuments[include])
	}
	if len(related.Items) != 1 {
		t.Fatalf("include items = %d, want 1", len(related.Items))
	}

	assertNoPushDiagnostics(t, h.client)
	h.exit(t, true)
}

func TestPullDiagnosticsVirtualCart(t *testing.T) {
	h := initializePullHarness(t, true, "")
	dir := t.TempDir()
	source := uri.File(filepath.Join(dir, "virtual.p8.png"))
	virtual := virtualCartURI(source)
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nvirtual_missing()\n"
	h.srv.carts.SetVirtualText(virtual, text)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	report := pullDocument(t, h, ctx, virtual, nil)
	full, ok := report.(*protocol.RelatedFullDocumentDiagnosticReport)
	if !ok {
		t.Fatalf("report type = %T, want full", report)
	}
	if !containsString(diagnosticMessageCodes(full.Items), "undefined-global") {
		t.Fatalf("virtual diagnostic codes = %v, want undefined-global", diagnosticMessageCodes(full.Items))
	}

	h.exit(t, true)
}

func TestWorkspaceDiagnosticsReportsOpenCarts(t *testing.T) {
	h := initializePullHarness(t, true, "")
	root, include := pullTestCart(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, mustReadFile(t, root))

	report, err := h.server.DiagnosticWorkspace(ctx, &protocol.WorkspaceDiagnosticParams{})
	if err != nil {
		t.Fatalf("workspace/diagnostic: %v", err)
	}
	if len(report.Items) != 2 {
		t.Fatalf("workspace items = %d, want 2", len(report.Items))
	}
	seen := make(map[uri.URI]bool)
	previous := make([]protocol.PreviousResultId, 0, len(report.Items))
	for _, item := range report.Items {
		full, ok := item.(*protocol.WorkspaceFullDocumentDiagnosticReport)
		if !ok {
			t.Fatalf("workspace report type = %T, want full", item)
		}
		seen[full.URI] = true
		if full.ResultID == nil || *full.ResultID == "" {
			t.Fatalf("workspace report for %s has no result id", full.URI)
		}
		previous = append(previous, protocol.PreviousResultId{URI: full.URI, Value: *full.ResultID})
	}
	if !seen[root] || !seen[include] {
		t.Fatalf("workspace URIs = %v, want root and include", seen)
	}

	again, err := h.server.DiagnosticWorkspace(ctx, &protocol.WorkspaceDiagnosticParams{PreviousResultIds: previous})
	if err != nil {
		t.Fatalf("workspace/diagnostic with previous ids: %v", err)
	}
	for _, item := range again.Items {
		if _, ok := item.(*protocol.WorkspaceUnchangedDocumentDiagnosticReport); !ok {
			t.Fatalf("workspace report type = %T, want unchanged", item)
		}
	}

	cancelled, cancelRequest := context.WithCancel(context.Background())
	cancelRequest()
	if _, err := h.srv.DiagnosticWorkspace(cancelled, &protocol.WorkspaceDiagnosticParams{}); !errors.Is(err, protocol.ErrRequestCancelled) {
		t.Fatalf("cancelled workspace error = %v (%T), want ErrRequestCancelled", err, err)
	}

	assertNoPushDiagnostics(t, h.client)
	h.exit(t, true)
}

func TestWorkspaceDiagnosticsPartialResults(t *testing.T) {
	h := initializePullHarness(t, true, "")
	root, _ := pullTestCart(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, mustReadFile(t, root))

	token := protocol.String("diagnostic-token")
	report, err := h.server.DiagnosticWorkspace(ctx, &protocol.WorkspaceDiagnosticParams{
		PartialResultParams: protocol.PartialResultParams{PartialResultToken: token},
	})
	if err != nil {
		t.Fatalf("workspace/diagnostic: %v", err)
	}
	if len(report.Items) != 0 {
		t.Fatalf("streamed report items = %d, want 0", len(report.Items))
	}

	select {
	case params := <-h.client.progress:
		if params.Token != token {
			t.Fatalf("progress token = %v, want %v", params.Token, token)
		}
		var partial protocol.WorkspaceDiagnosticReportPartialResult
		if err := protocol.Unmarshal(params.Value, &partial); err != nil {
			t.Fatalf("decode progress: %v", err)
		}
		if len(partial.Items) == 0 {
			t.Fatal("partial workspace report has no items")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for workspace diagnostic progress")
	}

	h.exit(t, true)
}

func TestDiagnosticResultIDDeterminism(t *testing.T) {
	file := &cart.File{URI: uri.File("/tmp/cart.p8"), Text: "print(1)"}
	items := []protocol.Diagnostic{diagnostic("problem")}
	settings := config.Default()

	first := diagnosticResultID(file, items, settings)
	if second := diagnosticResultID(file, items, settings); second != first {
		t.Fatalf("result id changed: %q != %q", second, first)
	}

	transportOnly := settings
	transportOnly.Diagnostics.Mode = config.DiagnosticsModePull
	if got := diagnosticResultID(file, items, transportOnly); got != first {
		t.Fatal("transport mode changed the result id")
	}

	severityChanged := settings
	severityChanged.Diagnostics.UndefinedGlobal = config.SeverityError
	if got := diagnosticResultID(file, items, severityChanged); got == first {
		t.Fatal("diagnostic setting change did not change the result id")
	}

	itemChanged := []protocol.Diagnostic{diagnostic("different")}
	if got := diagnosticResultID(file, itemChanged, settings); got == first {
		t.Fatal("diagnostic change did not change the result id")
	}

	textChanged := &cart.File{URI: file.URI, Text: "print(2)"}
	if got := diagnosticResultID(textChanged, items, settings); got == first {
		t.Fatal("file text change did not change the result id")
	}
}

func mustReadFile(t *testing.T, u uri.URI) string {
	t.Helper()
	data, err := os.ReadFile(u.FsPath())
	if err != nil {
		t.Fatalf("read %s: %v", u, err)
	}
	return string(data)
}
