package lsp

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/config"
)

func TestDispatchCancelsAsyncRequest(t *testing.T) {
	s := New(quietLogger(), config.New(quietLogger()))
	if _, err := s.Initialize(context.Background(), &protocol.InitializeParams{}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	serverPipe, clientPipe := net.Pipe()
	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()

	started := make(chan struct{})
	handlerErr := make(chan error, 1)
	next := func(ctx context.Context, req *jsonrpc2.Request) (any, error) {
		if req.Method() != protocol.MethodWorkspaceSymbol {
			return nil, jsonrpc2.ErrMethodNotFound
		}
		// Match a long-running handler that releases the read loop.
		jsonrpc2.Async(ctx)
		close(started)
		<-ctx.Done()
		handlerErr <- ctx.Err()
		return nil, protocol.ErrRequestCancelled
	}
	serverConn := jsonrpc2.NewConn(jsonrpc2.NewStream(serverPipe))
	serverConn.Go(serverCtx, s.dispatch(next))
	defer func() { _ = serverConn.Close() }()

	clientCtx, cancelClient := context.WithCancel(context.Background())
	defer cancelClient()
	_, clientConn, clientServer := protocol.NewClient(clientCtx, newTestClient(), jsonrpc2.NewStream(clientPipe))
	defer func() { _ = clientConn.Close() }()

	callCtx, cancelCall := context.WithCancel(context.Background())
	callErr := make(chan error, 1)
	go func() {
		_, err := clientServer.Symbols(callCtx, &protocol.WorkspaceSymbolParams{Query: ""})
		callErr <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("workspace symbol request did not start")
	}
	cancelCall()

	select {
	case err := <-handlerErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("handler context error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("workspace symbol handler was not canceled")
	}
	select {
	case err := <-callErr:
		if err == nil {
			t.Fatal("workspace symbol call succeeded after cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("workspace symbol call did not return")
	}
}

func TestHandleCancelCancelsRegisteredRequest(t *testing.T) {
	s := New(quietLogger(), config.New(quietLogger()))

	for _, tc := range []struct {
		name   string
		id     jsonrpc2.ID
		params string
	}{
		{name: "numeric id", id: jsonrpc2.NewNumberID(42), params: `{"id":42}`},
		{name: "string id", id: jsonrpc2.NewStringID("abc"), params: `{"id":"abc"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			s.registerCancel(tc.id, cancel)
			s.handleCancel(jsonrpc2.RawMessage(tc.params))
			if !errors.Is(ctx.Err(), context.Canceled) {
				t.Fatalf("context error = %v, want context.Canceled", ctx.Err())
			}
			s.unregisterCancel(tc.id)
		})
	}
}

func TestHandleCancelIgnoresUnknownAndMalformed(t *testing.T) {
	s := New(quietLogger(), config.New(quietLogger()))
	s.handleCancel(jsonrpc2.RawMessage(`{"id":99}`))
	s.handleCancel(jsonrpc2.RawMessage(`{`))
	s.handleCancel(jsonrpc2.RawMessage(`{"id":{}}`))
}

func TestRecoverPanicRequest(t *testing.T) {
	s := New(quietLogger(), config.New(quietLogger()))
	var result any
	var err error

	func() {
		defer s.recoverPanic("textDocument/hover", true, &result, &err)
		panic("boom")
	}()

	var rpcErr *jsonrpc2.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error = %v, want *jsonrpc2.Error", err)
	}
	if rpcErr.Code != jsonrpc2.InternalError {
		t.Fatalf("code = %d, want InternalError", rpcErr.Code)
	}
	if result != nil {
		t.Fatalf("result = %v, want nil", result)
	}
}

func TestRecoverPanicNotification(t *testing.T) {
	s := New(quietLogger(), config.New(quietLogger()))
	var result any
	var err error

	func() {
		defer s.recoverPanic("textDocument/didChange", false, &result, &err)
		panic("boom")
	}()

	if err != nil {
		t.Fatalf("error = %v, want nil for notification", err)
	}
	if result != nil {
		t.Fatalf("result = %v, want nil", result)
	}
}

func TestLifecycleChecks(t *testing.T) {
	s := New(quietLogger(), config.New(quietLogger()))

	if allow, err := s.checkLifecycle(protocol.MethodInitialize, true); !allow || err != nil {
		t.Fatalf("initialize before start: allow=%v err=%v", allow, err)
	}

	allow, err := s.checkLifecycle(protocol.MethodTextDocumentHover, true)
	if allow {
		t.Fatal("request before initialize was allowed")
	}
	var rpcErr *jsonrpc2.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.ServerNotInitialized {
		t.Fatalf("error = %v, want ServerNotInitialized", err)
	}

	allow, err = s.checkLifecycle(protocol.MethodTextDocumentDidOpen, false)
	if allow || err != nil {
		t.Fatalf("notification before initialize: allow=%v err=%v, want dropped", allow, err)
	}

	srv := New(quietLogger(), config.New(quietLogger()))
	if _, err := srv.Initialize(context.Background(), &protocol.InitializeParams{}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	if allow, err := srv.checkLifecycle(protocol.MethodTextDocumentHover, true); !allow || err != nil {
		t.Fatalf("request after initialize: allow=%v err=%v", allow, err)
	}
	if allow, err := srv.checkLifecycle(protocol.MethodInitialize, true); allow {
		t.Fatal("second initialize was allowed")
	} else if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.InvalidRequest {
		t.Fatalf("second initialize error = %v, want InvalidRequest", err)
	}

	if err := srv.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if allow, _ := srv.checkLifecycle(protocol.MethodTextDocumentHover, true); allow {
		t.Fatal("request after shutdown was allowed")
	}
	if allow, _ := srv.checkLifecycle(protocol.MethodTextDocumentDidOpen, false); allow {
		t.Fatal("notification after shutdown was allowed")
	}
}

func TestShutdownBeforeInitializeFails(t *testing.T) {
	s := New(quietLogger(), config.New(quietLogger()))
	err := s.Shutdown(context.Background())
	var rpcErr *jsonrpc2.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.InvalidRequest {
		t.Fatalf("error = %v, want InvalidRequest", err)
	}
}

func TestExitCodes(t *testing.T) {
	t.Run("after shutdown", func(t *testing.T) {
		s := New(quietLogger(), config.New(quietLogger()))
		if _, err := s.Initialize(context.Background(), &protocol.InitializeParams{}); err != nil {
			t.Fatal(err)
		}
		if err := s.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := s.Exit(context.Background()); err != nil {
			t.Fatal(err)
		}
		<-s.ExitSignal()
		if got := s.ExitCode(); got != 0 {
			t.Fatalf("exit code = %d, want 0", got)
		}
	})

	t.Run("without shutdown", func(t *testing.T) {
		s := New(quietLogger(), config.New(quietLogger()))
		if err := s.Exit(context.Background()); err != nil {
			t.Fatal(err)
		}
		<-s.ExitSignal()
		if got := s.ExitCode(); got != 1 {
			t.Fatalf("exit code = %d, want 1", got)
		}
	})
}

func TestInitializeCapabilities(t *testing.T) {
	s := New(quietLogger(), config.New(quietLogger()))
	res, err := s.Initialize(context.Background(), &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	if res.Capabilities.PositionEncoding != protocol.PositionEncodingKindUTF16 {
		t.Fatalf("position encoding = %q, want utf-16", res.Capabilities.PositionEncoding)
	}

	syncOptions, ok := res.Capabilities.TextDocumentSync.(*protocol.TextDocumentSyncOptions)
	if !ok {
		t.Fatalf("textDocumentSync = %T, want *TextDocumentSyncOptions", res.Capabilities.TextDocumentSync)
	}
	if syncOptions.OpenClose == nil || !*syncOptions.OpenClose {
		t.Fatal("openClose not enabled")
	}
	if syncOptions.Change == nil || *syncOptions.Change != protocol.TextDocumentSyncKindIncremental {
		t.Fatalf("change = %v, want incremental", syncOptions.Change)
	}
	save, ok := syncOptions.Save.(*protocol.SaveOptions)
	if !ok {
		t.Fatalf("save = %T, want *SaveOptions", syncOptions.Save)
	}
	if save.IncludeText == nil || *save.IncludeText {
		t.Fatal("includeText should be false")
	}

	if _, ok := res.Capabilities.DocumentSymbolProvider.(protocol.Boolean); !ok {
		t.Fatalf("documentSymbolProvider = %T, want Boolean", res.Capabilities.DocumentSymbolProvider)
	}
	if _, ok := res.Capabilities.FoldingRangeProvider.(protocol.Boolean); !ok {
		t.Fatalf("foldingRangeProvider = %T, want Boolean", res.Capabilities.FoldingRangeProvider)
	}
	for name, provider := range map[string]any{
		"definitionProvider":        res.Capabilities.DefinitionProvider,
		"referencesProvider":        res.Capabilities.ReferencesProvider,
		"documentHighlightProvider": res.Capabilities.DocumentHighlightProvider,
		"workspaceSymbolProvider":   res.Capabilities.WorkspaceSymbolProvider,
		"inlayHintProvider":         res.Capabilities.InlayHintProvider,
	} {
		if _, ok := provider.(protocol.Boolean); !ok {
			t.Errorf("%s = %T, want Boolean", name, provider)
		}
	}
	if _, ok := res.Capabilities.CodeActionProvider.(protocol.Boolean); !ok {
		t.Fatalf("codeActionProvider = %T, want Boolean", res.Capabilities.CodeActionProvider)
	}
	if res.Capabilities.CodeLensProvider == nil {
		t.Fatal("codeLensProvider not advertised")
	}
	if !containsString(res.Capabilities.ExecuteCommandProvider.Commands, CommandShowCartInfo) {
		t.Fatalf("executeCommandProvider = %+v", res.Capabilities.ExecuteCommandProvider)
	}
	rename, ok := res.Capabilities.RenameProvider.(*protocol.RenameOptions)
	if !ok || rename.PrepareProvider == nil || !*rename.PrepareProvider {
		t.Fatalf("renameProvider = %T, want prepare support", res.Capabilities.RenameProvider)
	}

	if res.Capabilities.Workspace == nil ||
		res.Capabilities.Workspace.WorkspaceFolders == nil ||
		res.Capabilities.Workspace.WorkspaceFolders.Supported == nil ||
		!*res.Capabilities.Workspace.WorkspaceFolders.Supported {
		t.Fatal("workspace folders are not advertised")
	}
	if notifications, ok := res.Capabilities.Workspace.WorkspaceFolders.ChangeNotifications.(protocol.Boolean); !ok || !bool(notifications) {
		t.Fatalf("workspace folder change notifications = %T, want true", res.Capabilities.Workspace.WorkspaceFolders.ChangeNotifications)
	}

	if res.ServerInfo.Name != "pico-lsp" {
		t.Fatalf("server name = %q", res.ServerInfo.Name)
	}
	if _, ok := res.ServerInfo.Version.Get(); !ok {
		t.Fatal("server version is missing")
	}
}

func TestChooseEncoding(t *testing.T) {
	tests := []struct {
		name string
		gen  *protocol.GeneralClientCapabilities
		want protocol.PositionEncodingKind
	}{
		{name: "nil general", want: protocol.PositionEncodingKindUTF16},
		{name: "empty", gen: &protocol.GeneralClientCapabilities{}, want: protocol.PositionEncodingKindUTF16},
		{
			name: "utf32 only",
			gen:  &protocol.GeneralClientCapabilities{PositionEncodings: []protocol.PositionEncodingKind{protocol.PositionEncodingKindUTF32}},
			want: protocol.PositionEncodingKindUTF16,
		},
		{
			name: "utf16 and utf8",
			gen:  &protocol.GeneralClientCapabilities{PositionEncodings: []protocol.PositionEncodingKind{protocol.PositionEncodingKindUTF16, protocol.PositionEncodingKindUTF8}},
			want: protocol.PositionEncodingKindUTF8,
		},
		{
			name: "utf8",
			gen:  &protocol.GeneralClientCapabilities{PositionEncodings: []protocol.PositionEncodingKind{protocol.PositionEncodingKindUTF8}},
			want: protocol.PositionEncodingKindUTF8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := &protocol.InitializeParams{}
			if tt.gen != nil {
				params.Capabilities.General = tt.gen
			}
			if got := chooseEncoding(params); got != tt.want {
				t.Fatalf("chooseEncoding = %q, want %q", got, tt.want)
			}
		})
	}
	if got := chooseEncoding(nil); got != protocol.PositionEncodingKindUTF16 {
		t.Fatalf("chooseEncoding(nil) = %q, want utf-16", got)
	}
}

func TestInitializeWithUTF8Negotiation(t *testing.T) {
	s := New(quietLogger(), config.New(quietLogger()))
	res, err := s.Initialize(context.Background(), &protocol.InitializeParams{
		Capabilities: protocol.ClientCapabilities{
			General: &protocol.GeneralClientCapabilities{
				PositionEncodings: []protocol.PositionEncodingKind{protocol.PositionEncodingKindUTF16, protocol.PositionEncodingKindUTF8},
			},
		},
	})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if res.Capabilities.PositionEncoding != protocol.PositionEncodingKindUTF8 {
		t.Fatalf("position encoding = %q, want utf-8", res.Capabilities.PositionEncoding)
	}
	if s.Encoding() != protocol.PositionEncodingKindUTF8 {
		t.Fatalf("server encoding = %q, want utf-8", s.Encoding())
	}
	if s.docs.Encoding() != protocol.PositionEncodingKindUTF8 {
		t.Fatalf("document encoding = %q, want utf-8", s.docs.Encoding())
	}
}
