package lsp

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/config"
)

type testClient struct {
	protocol.UnimplementedClient

	configValues  []protocol.LSPAny
	configErr     error
	configCalls   chan *protocol.ConfigurationParams
	diagnostics   chan *protocol.PublishDiagnosticsParams
	messages      chan *protocol.ShowMessageParams
	registrations chan *protocol.RegistrationParams
	progress      chan *protocol.ProgressParams
}

func newTestClient() *testClient {
	return &testClient{
		configCalls:   make(chan *protocol.ConfigurationParams, 4),
		diagnostics:   make(chan *protocol.PublishDiagnosticsParams, 8),
		messages:      make(chan *protocol.ShowMessageParams, 8),
		registrations: make(chan *protocol.RegistrationParams, 4),
		progress:      make(chan *protocol.ProgressParams, 8),
	}
}

func (c *testClient) Configuration(ctx context.Context, params *protocol.ConfigurationParams) ([]protocol.LSPAny, error) {
	c.configCalls <- params
	return c.configValues, c.configErr
}

func (c *testClient) PublishDiagnostics(ctx context.Context, params *protocol.PublishDiagnosticsParams) error {
	c.diagnostics <- params
	return nil
}

func (c *testClient) ShowMessage(ctx context.Context, params *protocol.ShowMessageParams) error {
	c.messages <- params
	return nil
}

func (c *testClient) RegisterCapability(ctx context.Context, params *protocol.RegistrationParams) error {
	c.registrations <- params
	return nil
}

func (c *testClient) Progress(ctx context.Context, params *protocol.ProgressParams) error {
	c.progress <- params
	return nil
}

type harness struct {
	srv          *Server
	server       protocol.Server
	clientConn   jsonrpc2.Conn
	serverConn   jsonrpc2.Conn
	client       *testClient
	cancelServer context.CancelFunc
	cancelClient context.CancelFunc
}

func newHarness(t *testing.T, client *testClient) *harness {
	t.Helper()
	return newHarnessWithHandler(t, client, nil)
}

func newHarnessWithHandler(t *testing.T, client *testClient, wrap func(jsonrpc2.Handler) jsonrpc2.Handler) *harness {
	t.Helper()

	serverPipe, clientPipe := net.Pipe()
	srv := New(quietLogger(), config.New(quietLogger()))

	serverCtx, cancelServer := context.WithCancel(context.Background())
	next := protocol.ServerHandler(srv, jsonrpc2.MethodNotFoundHandler)
	if wrap != nil {
		next = wrap(next)
	}
	serverConn := srv.serve(serverCtx, jsonrpc2.NewStream(serverPipe), next)

	clientCtx, cancelClient := context.WithCancel(context.Background())
	_, clientConn, server := protocol.NewClient(clientCtx, client, jsonrpc2.NewStream(clientPipe))

	h := &harness{
		srv:          srv,
		server:       server,
		clientConn:   clientConn,
		serverConn:   serverConn,
		client:       client,
		cancelServer: cancelServer,
		cancelClient: cancelClient,
	}

	t.Cleanup(func() {
		h.cancelClient()
		h.cancelServer()

		done := make(chan struct{})
		go func() {
			_ = h.clientConn.Close()
			_ = h.serverConn.Close()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Log("timed out closing LSP harness")
		}
	})

	return h
}

func (h *harness) initialize(t *testing.T, params *protocol.InitializeParams) *protocol.InitializeResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := h.server.Initialize(ctx, params)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}

	if err := h.server.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		t.Fatalf("initialized: %v", err)
	}

	// Barrier: ensure the server processed the initialized notification before
	// the caller inspects diagnostics or configuration state.
	if _, err := h.server.Hover(ctx, &protocol.HoverParams{}); err != nil {
		t.Fatalf("initialized barrier: %v", err)
	}
	return result
}

func (h *harness) exit(t *testing.T, shutdown bool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if shutdown {
		if err := h.server.Shutdown(ctx); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}
	if err := h.server.Exit(ctx); err != nil {
		t.Fatalf("exit: %v", err)
	}

	select {
	case <-h.srv.ExitSignal():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for exit signal")
	}
}

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestEndToEndDocumentLifecycle(t *testing.T) {
	h := newHarness(t, newTestClient())
	result := h.initialize(t, &protocol.InitializeParams{})

	if result.Capabilities.PositionEncoding != protocol.PositionEncodingKindUTF16 {
		t.Fatalf("position encoding = %q, want utf-16", result.Capabilities.PositionEncoding)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	u := uri.File("/tmp/cart.p8")
	if err := h.server.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        u,
			LanguageID: protocol.LanguageKind("pico8"),
			Version:    1,
			Text:       "print('hi')",
		},
	}); err != nil {
		t.Fatalf("didOpen: %v", err)
	}
	waitFor(t, "document open", func() bool {
		doc, ok := h.srv.Document(u)
		return ok && doc.Text == "print('hi')" && doc.Version == 1
	})

	if err := h.server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: u},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			partialChange(0, 11, 0, 11, "!"),
		},
	}); err != nil {
		t.Fatalf("didChange: %v", err)
	}
	waitFor(t, "incremental change", func() bool {
		doc, ok := h.srv.Document(u)
		return ok && doc.Text == "print('hi')!" && doc.Version == 2
	})

	saved := "print('bye')"
	if err := h.server.DidSave(ctx, &protocol.DidSaveTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
		Text:         &saved,
	}); err != nil {
		t.Fatalf("didSave: %v", err)
	}
	waitFor(t, "save text", func() bool {
		doc, ok := h.srv.Document(u)
		return ok && doc.Text == saved
	})

	if err := h.server.DidClose(ctx, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
	}); err != nil {
		t.Fatalf("didClose: %v", err)
	}
	waitFor(t, "document close", func() bool {
		_, ok := h.srv.Document(u)
		return !ok
	})

	h.exit(t, true)
	if got := h.srv.ExitCode(); got != 0 {
		t.Fatalf("exit code = %d, want 0", got)
	}
}

func TestEndToEndUnknownRequestAndNotification(t *testing.T) {
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.server.Request(ctx, "pico8/doesNotExist", nil)
	var rpcErr *jsonrpc2.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.MethodNotFound {
		t.Fatalf("unknown request error = %v, want MethodNotFound", err)
	}

	if err := h.clientConn.Notify(ctx, "pico8/unknownNotification", map[string]any{"x": 1}); err != nil {
		t.Fatalf("unknown notification: %v", err)
	}

	// A hover request acts as a barrier: if the unknown notification had torn
	// down the connection, this request would fail.
	if _, err := h.server.Hover(ctx, &protocol.HoverParams{}); err != nil {
		t.Fatalf("hover after unknown notification: %v", err)
	}

	h.exit(t, true)
}

func TestEndToEndServerNotInitialized(t *testing.T) {
	h := newHarness(t, newTestClient())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.server.Hover(ctx, &protocol.HoverParams{})
	var rpcErr *jsonrpc2.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.ServerNotInitialized {
		t.Fatalf("error = %v, want ServerNotInitialized", err)
	}

	h.initialize(t, &protocol.InitializeParams{})
	h.exit(t, true)
}

func TestEndToEndInitializeNotificationIsIgnored(t *testing.T) {
	h := newHarness(t, newTestClient())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	initNotification := map[string]any{"capabilities": map[string]any{}}
	if err := h.clientConn.Notify(ctx, protocol.MethodInitialize, initNotification); err != nil {
		t.Fatalf("initialize notification: %v", err)
	}

	// The notification must not initialize the server, so this request succeeds.
	h.initialize(t, &protocol.InitializeParams{})

	if err := h.clientConn.Notify(ctx, protocol.MethodInitialize, initNotification); err != nil {
		t.Fatalf("duplicate initialize notification: %v", err)
	}
	if _, err := h.server.Hover(ctx, &protocol.HoverParams{}); err != nil {
		t.Fatalf("hover after duplicate initialize notification: %v", err)
	}

	_, err := h.server.Initialize(ctx, &protocol.InitializeParams{})
	var rpcErr *jsonrpc2.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.InvalidRequest {
		t.Fatalf("second initialize error = %v, want InvalidRequest", err)
	}

	h.exit(t, true)
}

func TestEndToEndShutdownNotificationIsIgnored(t *testing.T) {
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.clientConn.Notify(ctx, protocol.MethodShutdown, nil); err != nil {
		t.Fatalf("shutdown notification: %v", err)
	}
	if _, err := h.server.Hover(ctx, &protocol.HoverParams{}); err != nil {
		t.Fatalf("hover after shutdown notification: %v", err)
	}

	h.exit(t, true)
}

func TestEndToEndBadParamsDoNotKillConnection(t *testing.T) {
	h := newHarness(t, newTestClient())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var raw any
	if _, err := h.clientConn.Call(ctx, protocol.MethodInitialize, "not a params object", &raw); err == nil {
		t.Fatal("invalid initialize params succeeded, want error")
	} else {
		var rpcErr *jsonrpc2.Error
		if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.InvalidParams {
			t.Fatalf("invalid initialize error = %v, want InvalidParams", err)
		}
	}

	if _, err := h.server.Initialize(ctx, &protocol.InitializeParams{}); err != nil {
		t.Fatalf("initialize after invalid request: %v", err)
	}
	if err := h.server.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		t.Fatalf("initialized: %v", err)
	}

	if _, err := h.clientConn.Call(ctx, protocol.MethodTextDocumentHover, map[string]any{
		"textDocument": "not an object",
	}, &raw); err == nil {
		t.Fatal("invalid hover params succeeded, want error")
	} else {
		var rpcErr *jsonrpc2.Error
		if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.InvalidParams {
			t.Fatalf("invalid hover error = %v, want InvalidParams", err)
		}
	}

	if err := h.clientConn.Notify(ctx, protocol.MethodTextDocumentDidChange, map[string]any{
		"textDocument": "not an object",
	}); err != nil {
		t.Fatalf("malformed notification: %v", err)
	}
	if _, err := h.server.Hover(ctx, &protocol.HoverParams{}); err != nil {
		t.Fatalf("hover after malformed notification: %v", err)
	}

	h.exit(t, true)
}

func TestEndToEndPanicRecovery(t *testing.T) {
	var panicked atomic.Bool
	h := newHarnessWithHandler(t, newTestClient(), func(next jsonrpc2.Handler) jsonrpc2.Handler {
		return func(ctx context.Context, req *jsonrpc2.Request) (any, error) {
			if req.Method() == protocol.MethodTextDocumentReferences && panicked.CompareAndSwap(false, true) {
				panic("boom")
			}
			return next(ctx, req)
		}
	})

	var logs bytes.Buffer
	h.srv.log = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.server.References(ctx, &protocol.ReferenceParams{})
	var rpcErr *jsonrpc2.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.InternalError {
		t.Fatalf("panicking request error = %v, want InternalError", err)
	}

	if !strings.Contains(logs.String(), "recovered panic") {
		t.Fatalf("panic was not logged; log:\n%s", logs.String())
	}

	if _, err := h.server.References(ctx, &protocol.ReferenceParams{}); err != nil {
		t.Fatalf("references after recovered panic: %v", err)
	}

	h.exit(t, true)
}

func TestEndToEndConfigurationPull(t *testing.T) {
	client := newTestClient()
	client.configValues = []protocol.LSPAny{
		protocol.LSPAny([]byte(`{"typeChecking":"strict","maxProblems":7}`)),
	}

	h := newHarness(t, client)
	capabilities := protocol.ClientCapabilities{
		Workspace: &protocol.WorkspaceClientCapabilities{
			Configuration: boolPtr(true),
		},
	}
	h.initialize(t, &protocol.InitializeParams{Capabilities: capabilities})

	select {
	case params := <-client.configCalls:
		if len(params.Items) != 1 || params.Items[0].Section == nil || *params.Items[0].Section != config.Section {
			t.Fatalf("configuration params = %+v, want section pico8", params)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for workspace/configuration")
	}

	waitFor(t, "pulled configuration", func() bool {
		settings := h.srv.cfg.Settings()
		return settings.TypeChecking == config.TypeCheckingStrict && settings.MaxProblems == 7
	})

	h.exit(t, true)
}

func TestEndToEndNoConfigurationPullWithoutCapability(t *testing.T) {
	client := newTestClient()
	h := newHarness(t, client)
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := h.server.Hover(ctx, &protocol.HoverParams{}); err != nil {
		t.Fatalf("hover: %v", err)
	}
	if len(client.configCalls) != 0 {
		t.Fatal("workspace/configuration was called without client capability")
	}

	h.exit(t, true)
}

func TestEndToEndDiagnosticsGate(t *testing.T) {
	client := newTestClient()
	h := newHarness(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	u := uri.File("/tmp/cart.p8")
	h.srv.Diagnostics().Publish(ctx, u, nil, []protocol.Diagnostic{diagnostic("before init")})
	select {
	case <-client.diagnostics:
		t.Fatal("diagnostics published before initialized")
	case <-time.After(20 * time.Millisecond):
	}

	h.initialize(t, &protocol.InitializeParams{})
	h.srv.Diagnostics().Publish(ctx, u, int32Ptr(4), []protocol.Diagnostic{diagnostic("after init")})

	select {
	case params := <-client.diagnostics:
		if params.URI != u {
			t.Fatalf("diagnostic URI = %q, want %q", params.URI, u)
		}
		version, ok := params.Version.Get()
		if !ok || version != 4 {
			t.Fatalf("diagnostic version = %v, want 4", params.Version)
		}
		if len(params.Diagnostics) != 1 || params.Diagnostics[0].Message != protocol.String("after init") {
			t.Fatalf("diagnostics = %+v, want one 'after init' message", params.Diagnostics)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for diagnostics")
	}

	h.exit(t, true)
}

func TestEndToEndExitWithoutShutdown(t *testing.T) {
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	h.exit(t, false)

	if got := h.srv.ExitCode(); got != 1 {
		t.Fatalf("exit code = %d, want 1", got)
	}
}

func TestEndToEndNonFileDocumentIgnored(t *testing.T) {
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	u := uri.URI("untitled:cart")
	if err := h.server.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        u,
			LanguageID: protocol.LanguageKind("pico8"),
			Version:    1,
			Text:       "print(1)",
		},
	}); err != nil {
		t.Fatalf("didOpen: %v", err)
	}

	if _, ok := h.srv.Document(u); ok {
		t.Fatal("non-file document was stored")
	}

	h.exit(t, true)
}
