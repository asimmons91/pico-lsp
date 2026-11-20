package lsp_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/config"
	internallsp "github.com/asimmons91/pico-lsp/internal/lsp"
	"github.com/asimmons91/pico-lsp/internal/version"
	"github.com/asimmons91/pico-lsp/pkg/lsp"
)

type testClient struct {
	protocol.UnimplementedClient

	diagnostics  chan *protocol.PublishDiagnosticsParams
	configCalls  chan *protocol.ConfigurationParams
	configValues []protocol.LSPAny
	configErr    error
}

func newTestClient() *testClient {
	return &testClient{
		diagnostics: make(chan *protocol.PublishDiagnosticsParams, 16),
		configCalls: make(chan *protocol.ConfigurationParams, 4),
	}
}

func (c *testClient) PublishDiagnostics(_ context.Context, params *protocol.PublishDiagnosticsParams) error {
	select {
	case c.diagnostics <- params:
	default:
	}
	return nil
}

func (c *testClient) Configuration(_ context.Context, params *protocol.ConfigurationParams) ([]protocol.LSPAny, error) {
	select {
	case c.configCalls <- params:
	default:
	}
	return c.configValues, c.configErr
}

type harness struct {
	t          *testing.T
	client     *testClient
	clientAPI  protocol.Server
	clientConn jsonrpc2.Conn
	serverConn jsonrpc2.Conn

	serverCancel context.CancelFunc
	clientCancel context.CancelFunc
	stopped      bool
}

func newHarness(t *testing.T, opts lsp.Options, client *testClient) *harness {
	t.Helper()

	serverPipe, clientPipe := net.Pipe()
	serverCtx, serverCancel := context.WithCancel(context.Background())
	serverConn := lsp.New(opts).Serve(serverCtx, jsonrpc2.NewStream(serverPipe))

	clientCtx, clientCancel := context.WithCancel(context.Background())
	_, clientConn, clientAPI := protocol.NewClient(clientCtx, client, jsonrpc2.NewStream(clientPipe))

	h := &harness{
		t:            t,
		client:       client,
		clientAPI:    clientAPI,
		clientConn:   clientConn,
		serverConn:   serverConn,
		serverCancel: serverCancel,
		clientCancel: clientCancel,
	}
	t.Cleanup(h.stop)
	return h
}

func (h *harness) initialize(params *protocol.InitializeParams) *protocol.InitializeResult {
	h.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := h.clientAPI.Initialize(ctx, params)
	if err != nil {
		h.t.Fatalf("initialize: %v", err)
	}
	if err := h.clientAPI.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		h.t.Fatalf("initialized: %v", err)
	}
	return result
}

func (h *harness) stop() {
	if h.stopped {
		return
	}
	h.stopped = true
	h.clientCancel()
	h.serverCancel()

	done := make(chan struct{})
	go func() {
		_ = h.clientConn.Close()
		_ = h.serverConn.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		h.t.Error("timed out closing the in-memory LSP harness")
	}
}

func (h *harness) waitDiagnostics(u uri.URI) *protocol.PublishDiagnosticsParams {
	h.t.Helper()

	deadline := time.After(5 * time.Second)
	for {
		select {
		case params := <-h.client.diagnostics:
			if params.URI == u {
				return params
			}
		case <-deadline:
			h.t.Fatalf("timed out waiting for diagnostics on %s", u)
		}
	}
}

func (h *harness) waitConfigCall() {
	h.t.Helper()

	select {
	case <-h.client.configCalls:
	case <-time.After(5 * time.Second):
		h.t.Fatal("timed out waiting for workspace/configuration")
	}
}

func (h *harness) waitCodeLens(u uri.URI, want int) {
	h.t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		lenses, err := h.clientAPI.CodeLens(ctx, &protocol.CodeLensParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
		})
		cancel()
		if err != nil {
			h.t.Fatalf("code lens: %v", err)
		}
		if len(lenses) == want {
			return
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("code lens count = %d, want %d", len(lenses), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func initializeParams(dir string, options string) *protocol.InitializeParams {
	root := uri.File(dir)
	params := &protocol.InitializeParams{
		Capabilities: protocol.ClientCapabilities{
			Workspace: &protocol.WorkspaceClientCapabilities{
				WorkspaceFolders: boolPtr(true),
			},
		},
		WorkspaceFolders: protocol.NewNullable([]protocol.WorkspaceFolder{{URI: root, Name: "game"}}),
	}
	if options != "" {
		params.InitializationOptions = protocol.LSPAny(options)
	}
	return params
}

func boolPtr(value bool) *bool { return &value }

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func writeCart(t *testing.T, dir, name, text string) (uri.URI, string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return uri.File(path), path
}

func openCart(t *testing.T, h *harness, u uri.URI, text string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.clientAPI.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
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

func TestInMemoryServerRoundTrip(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nlocal x = 1\nprintt(x)\n"
	cartURI, _ := writeCart(t, dir, "game.p8", text)
	client := newTestClient()
	h := newHarness(t, lsp.Options{}, client)

	params := initializeParams(dir, `{"pico8":{"limits":{"codeLens":false}}}`)
	got := h.initialize(params)

	// The in-process server advertises exactly what the stdio server does.
	stdio := internallsp.New(discardLogger(), config.New(discardLogger()))
	want, err := stdio.Initialize(context.Background(), params)
	if err != nil {
		t.Fatalf("stdio initialize: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("initialize result differs from stdio server:\n got: %+v\nwant: %+v", got, want)
	}
	if got.ServerInfo.Name != "pico-lsp" {
		t.Fatalf("ServerInfo.Name = %q", got.ServerInfo.Name)
	}
	if value, ok := got.ServerInfo.Version.Get(); !ok || value != version.Current() {
		t.Fatalf("ServerInfo.Version = %q, %v; want %q", value, ok, version.Current())
	}
	if got.Capabilities.PositionEncoding != protocol.PositionEncodingKindUTF16 {
		t.Fatalf("PositionEncoding = %q, want UTF-16", got.Capabilities.PositionEncoding)
	}

	openCart(t, h, cartURI, text)
	diagnostics := h.waitDiagnostics(cartURI)
	if len(diagnostics.Diagnostics) == 0 {
		t.Fatal("didOpen published no diagnostics")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tokens, err := h.clientAPI.SemanticTokensFull(ctx, &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: cartURI},
	})
	if err != nil {
		t.Fatalf("semantic tokens: %v", err)
	}
	if tokens == nil || len(tokens.Data) == 0 {
		t.Fatal("semantic tokens returned no data")
	}

	// The initializationOptions setting disables the cart-budget code lens.
	h.waitCodeLens(cartURI, 0)

	if err := h.clientAPI.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	h.stop()
	select {
	case <-h.serverConn.Done():
	default:
		t.Fatal("server connection is still running after shutdown and close")
	}
}

func TestWorkspaceConfiguration(t *testing.T) {
	t.Run("applies a setting", func(t *testing.T) {
		dir := t.TempDir()
		text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nx = 1\nprintt(x)\n"
		cartURI, _ := writeCart(t, dir, "game.p8", text)
		client := newTestClient()
		client.configValues = []protocol.LSPAny{protocol.LSPAny(`{"limits":{"codeLens":false}}`)}
		h := newHarness(t, lsp.Options{}, client)

		params := initializeParams(dir, "")
		params.Capabilities.Workspace.Configuration = boolPtr(true)
		h.initialize(params)
		h.waitConfigCall()

		openCart(t, h, cartURI, text)
		h.waitDiagnostics(cartURI)
		h.waitCodeLens(cartURI, 0)
	})

	t.Run("invalid values fall back with a log entry", func(t *testing.T) {
		dir := t.TempDir()
		text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nx = 1\nprintt(x)\n"
		cartURI, _ := writeCart(t, dir, "game.p8", text)
		client := newTestClient()
		client.configValues = []protocol.LSPAny{protocol.LSPAny(`{"limits":{"codeLens":"invalid"}}`)}

		logs := &lockedBuffer{}
		logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
		h := newHarness(t, lsp.Options{Logger: logger}, client)

		params := initializeParams(dir, "")
		params.Capabilities.Workspace.Configuration = boolPtr(true)
		h.initialize(params)
		h.waitConfigCall()
		waitForLog(t, logs, "configuration pull failed")

		openCart(t, h, cartURI, text)
		h.waitDiagnostics(cartURI)
		h.waitCodeLens(cartURI, 1)
	})
}

func TestLogging(t *testing.T) {
	t.Run("supplied logger receives records", func(t *testing.T) {
		logs := &lockedBuffer{}
		logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
		h := newHarness(t, lsp.Options{Logger: logger}, newTestClient())
		h.initialize(initializeParams(t.TempDir(), ""))
		h.stop()

		if logs.String() == "" {
			t.Fatal("supplied logger received no records")
		}
		if !bytes.Contains([]byte(logs.String()), []byte("initialized")) {
			t.Fatalf("log output = %q, want an initialized record", logs.String())
		}
	})

	t.Run("nil logger writes nothing to stdout", func(t *testing.T) {
		oldStdout := os.Stdout
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout = writer
		defer func() {
			os.Stdout = oldStdout
			_ = reader.Close()
			_ = writer.Close()
		}()

		h := newHarness(t, lsp.Options{}, newTestClient())
		h.initialize(initializeParams(t.TempDir(), ""))
		h.stop()

		os.Stdout = oldStdout
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		output, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if len(output) != 0 {
			t.Fatalf("nil logger wrote %q to stdout", output)
		}
	})
}

func TestConcurrentRequestsAndCancellation(t *testing.T) {
	dir := t.TempDir()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nlocal speed = 2\nfunction _update()\n  speed = speed + 1\nend\n"
	cartURI, _ := writeCart(t, dir, "game.p8", text)
	h := newHarness(t, lsp.Options{}, newTestClient())
	h.initialize(initializeParams(dir, ""))
	openCart(t, h, cartURI, text)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var failures []error
	record := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		failures = append(failures, err)
		mu.Unlock()
	}

	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				_, err := h.clientAPI.SemanticTokensFull(ctx, &protocol.SemanticTokensParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: cartURI},
				})
				record(err)
				_, err = h.clientAPI.Hover(ctx, &protocol.HoverParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: cartURI},
					Position:     protocol.Position{Line: 4, Character: 3},
				})
				record(err)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		canceled, cancelRequest := context.WithCancel(context.Background())
		cancelRequest()
		_, err := h.clientAPI.DocumentSymbol(canceled, &protocol.DocumentSymbolParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: cartURI},
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			record(err)
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("concurrent requests timed out")
	}
	if len(failures) > 0 {
		t.Fatalf("concurrent request failures: %v", failures)
	}
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func waitForLog(t *testing.T, logs *lockedBuffer, want string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if bytes.Contains([]byte(logs.String()), []byte(want)) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for log %q; logs: %s", want, logs.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
