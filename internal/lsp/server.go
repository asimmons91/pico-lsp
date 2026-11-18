package lsp

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/builtin"
	"github.com/asimmons91/pico-lsp/internal/config"
	"github.com/asimmons91/pico-lsp/internal/version"
)

// errUnhandledMethod is returned by Server.Request for methods that are not
// part of the LSP. The dispatcher converts it to MethodNotFound for requests
// and silently drops it for notifications.
var errUnhandledMethod = errors.New("unhandled LSP method")

// protocolMethods is the set of methods known to the LSP metamodel. Methods
// outside this set are rejected before protocol.ServerHandler decodes their
// params; otherwise a parameterless custom request would fail with a parse
// error instead of MethodNotFound. Later milestones add pico8/... methods here
// when they implement custom requests.
var protocolMethods = func() map[string]struct{} {
	methods := make(map[string]struct{}, len(protocol.Methods)+1)
	for _, info := range protocol.Methods {
		methods[info.Method] = struct{}{}
	}
	methods[RequestCartInfo] = struct{}{}
	methods[RequestDecodeCart] = struct{}{}
	return methods
}()

// lspCodec mirrors protocol's unexported LSP codec. protocol.Marshal and
// protocol.Unmarshal are the exported union-aware codec core; the wrapper only
// adds the raw-message pass-through required by jsonrpc2.
type lspCodec struct{}

func (lspCodec) Marshal(v any) ([]byte, error) {
	switch m := v.(type) {
	case jsonrpc2.RawMessage:
		if m == nil {
			return []byte("null"), nil
		}
		return m, nil
	case *jsonrpc2.RawMessage:
		if m == nil || *m == nil {
			return []byte("null"), nil
		}
		return *m, nil
	}
	return protocol.Marshal(v)
}

func (lspCodec) Unmarshal(data []byte, v any) error {
	if p, ok := v.(*jsonrpc2.RawMessage); ok {
		b := make(jsonrpc2.RawMessage, len(data))
		copy(b, data)
		*p = b
		return nil
	}
	return protocol.Unmarshal(data, v)
}

type lifecycleState int

const (
	stateCreated lifecycleState = iota
	stateInitialized
	stateShutdown
	stateExited
)

// Server implements the LSP server. It embeds protocol.UnimplementedServer so
// later milestones can add features one handler at a time.
type Server struct {
	protocol.UnimplementedServer

	log      *slog.Logger
	docs     *DocumentStore
	cfg      *config.Manager
	diags    *Diagnostics
	carts    *CartManager
	png      *pngCache
	builtins *builtin.API

	client atomic.Value // protocol.Client

	mu                 sync.Mutex
	state              lifecycleState
	encoding           protocol.PositionEncodingKind
	trace              protocol.TraceValue
	diagnosticPull     bool
	diagnosticRelated  bool
	warnedPullFallback bool

	workspaceMu      sync.Mutex
	workspaceFolders []uri.URI

	watchedFilesDynamic bool

	cancelMu sync.Mutex
	cancels  map[jsonrpc2.ID]context.CancelFunc

	pullMu sync.Mutex

	exitCode int
	exitDone chan struct{}
	exitOnce sync.Once
}

// New creates a server. If cfg is nil, a manager with defaults is created.
func New(log *slog.Logger, cfg *config.Manager) *Server {
	if log == nil {
		log = slog.Default()
	}
	if cfg == nil {
		cfg = config.New(log)
	}

	s := &Server{
		log:      log,
		cfg:      cfg,
		encoding: protocol.PositionEncodingKindUTF16,
		trace:    protocol.TraceValueOff,
		exitDone: make(chan struct{}),
		cancels:  make(map[jsonrpc2.ID]context.CancelFunc),
	}
	s.docs = NewDocumentStore(s.encoding, log)
	s.png = newPNGCache()
	s.diags = NewDiagnostics(
		func() protocol.Client { return s.Client() },
		func() int { return s.cfg.Settings().MaxProblems },
		log,
	)

	var builtins map[string]*analysis.Symbol
	api, err := builtin.Load()
	if err != nil {
		// Keep the server usable without built-ins. Every other feature must
		// continue to work when the embedded stub is broken.
		log.Error("failed to load PICO-8 built-in API; built-in symbols are disabled", "error", err)
	} else {
		s.builtins = api
		builtins = analysis.BuildBuiltins(api)
	}

	s.carts = NewCartManager(s.docs, s.diags, func() config.Settings { return s.cfg.Settings() }, s.WorkspaceFolders, builtins, log)
	return s
}

// Serve starts an LSP connection over stream. It returns the connection so the
// caller can wait for it to finish and close it during shutdown.
//
// It mirrors protocol.NewServer but wraps the handler with lifecycle checks,
// panic recovery, and unknown-method handling, which protocol.NewServer does
// not expose.
func (s *Server) Serve(ctx context.Context, stream jsonrpc2.Stream) jsonrpc2.Conn {
	next := protocol.ServerHandler(s, jsonrpc2.MethodNotFoundHandler)
	return s.serve(ctx, stream, next)
}

func (s *Server) serve(ctx context.Context, stream jsonrpc2.Stream, next jsonrpc2.Handler) jsonrpc2.Conn {
	conn := jsonrpc2.NewConn(stream, jsonrpc2.WithCodec(lspCodec{}))
	s.client.Store(protocol.ClientDispatcher(conn))

	conn.Go(ctx, s.dispatch(next))
	return conn
}

// Client returns the dispatcher used to call the editor, or nil before Serve.
func (s *Server) Client() protocol.Client {
	v := s.client.Load()
	if v == nil {
		return nil
	}
	client, _ := v.(protocol.Client)
	return client
}

// Document returns a snapshot of an open document.
func (s *Server) Document(u uri.URI) (Document, bool) {
	return s.docs.Get(u)
}

// Encoding returns the negotiated position encoding.
func (s *Server) Encoding() protocol.PositionEncodingKind {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.encoding
}

// ExitSignal is closed when the client sends the exit notification.
func (s *Server) ExitSignal() <-chan struct{} { return s.exitDone }

// ExitCode reports the process exit code requested by the client. It is valid
// after ExitSignal is closed.
func (s *Server) ExitCode() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exitCode
}

// Exited reports whether the exit notification has been handled.
func (s *Server) Exited() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == stateExited
}

// ShuttingDown reports whether shutdown has been received.
func (s *Server) ShuttingDown() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == stateShutdown || s.state == stateExited
}

// Diagnostics exposes the publisher for tests and later milestones.
func (s *Server) Diagnostics() *Diagnostics { return s.diags }

// cancellableRequestContext preserves the jsonrpc2 request values (notably
// the Async release token) while exposing the per-request cancellation channel.
type cancellableRequestContext struct {
	context.Context
	values context.Context
}

func (c cancellableRequestContext) Value(key any) any {
	return c.values.Value(key)
}

func (s *Server) registerCancel(id jsonrpc2.ID, cancel context.CancelFunc) {
	s.cancelMu.Lock()
	if s.cancels == nil {
		s.cancels = make(map[jsonrpc2.ID]context.CancelFunc)
	}
	s.cancels[id] = cancel
	s.cancelMu.Unlock()
}

func (s *Server) unregisterCancel(id jsonrpc2.ID) {
	s.cancelMu.Lock()
	delete(s.cancels, id)
	s.cancelMu.Unlock()
}

func (s *Server) handleCancel(raw jsonrpc2.RawMessage) {
	var params protocol.CancelParams
	if err := protocol.Unmarshal(raw, &params); err != nil {
		s.log.Debug("ignoring malformed cancellation request", "error", err)
		return
	}

	var id jsonrpc2.ID
	switch value := params.ID.(type) {
	case protocol.Integer:
		id = jsonrpc2.NewNumberID(int64(value))
	case protocol.String:
		id = jsonrpc2.NewStringID(string(value))
	default:
		s.log.Debug("ignoring cancellation request with invalid id")
		return
	}

	s.cancelMu.Lock()
	cancel := s.cancels[id]
	s.cancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// dispatch adds lifecycle checks, panic recovery, cancellation, and request
// logging around every protocol handler.
func (s *Server) dispatch(next jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, req *jsonrpc2.Request) (result any, err error) {
		method := strings.Clone(req.Method())
		isCall := req.IsCall()
		start := time.Now()

		defer func() {
			s.log.Debug("lsp request",
				"method", method,
				"call", isCall,
				"duration", time.Since(start),
				"error", err,
			)
		}()
		defer s.recoverPanic(method, isCall, &result, &err)

		if allow, lifeErr := s.checkLifecycle(method, isCall); !allow {
			if !isCall {
				if lifeErr != nil {
					s.log.Warn("ignoring notification rejected by lifecycle", "method", method, "error", lifeErr)
				}
				return nil, nil
			}
			return nil, lifeErr
		}

		if _, ok := protocolMethods[method]; !ok {
			if isCall {
				return nil, jsonrpc2.Errorf(jsonrpc2.MethodNotFound, "method not found: %s", method)
			}
			s.log.Debug("ignoring unknown notification", "method", method)
			return nil, nil
		}

		if method == protocol.MethodCancelRequest {
			s.handleCancel(req.Params())
			return nil, nil
		}

		if isCall {
			id := req.ID()
			parent := jsonrpc2.DetachContext(ctx)
			if parent == nil {
				parent = context.Background()
			}
			cancelCtx, cancel := context.WithCancel(parent)
			s.registerCancel(id, cancel)
			ctx = cancellableRequestContext{Context: cancelCtx, values: ctx}
			defer func() {
				s.unregisterCancel(id)
				cancel()
			}()
		}

		result, err = next(ctx, req)
		if errors.Is(err, errUnhandledMethod) {
			if isCall {
				return nil, jsonrpc2.Errorf(jsonrpc2.MethodNotFound, "method not found: %s", method)
			}
			s.log.Debug("ignoring unknown notification", "method", method)
			return nil, nil
		}
		if !isCall && err != nil {
			// A notification has no response. Returning an error would make the
			// JSON-RPC layer tear down the connection, so malformed notifications
			// are logged and ignored.
			s.log.Warn("ignoring failed notification", "method", method, "error", err)
			return nil, nil
		}
		if isCall && errors.Is(err, jsonrpc2.ErrParse) {
			// The protocol decoder reports parameter shape errors as ParseError.
			// Envelope-level parse failures never reach a handler, so this is an
			// invalid-params error from the LSP perspective.
			s.log.Debug("invalid request params", "method", method)
			return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "%s: invalid params", method)
		}
		return result, err
	}
}

// recoverPanic turns a handler panic into an InternalError response for
// requests and a logged no-op for notifications.
func (s *Server) recoverPanic(method string, isCall bool, result *any, err *error) {
	r := recover()
	if r == nil {
		return
	}

	s.log.Error("recovered panic in LSP handler",
		"method", method,
		"panic", r,
		"stack", string(debug.Stack()),
	)

	if isCall {
		*result = nil
		*err = jsonrpc2.Errorf(jsonrpc2.InternalError, "internal error handling %s", method)
		return
	}
	*err = nil
}

// checkLifecycle enforces the LSP initialization and shutdown sequence.
func (s *Server) checkLifecycle(method string, isCall bool) (allow bool, err error) {
	switch method {
	case protocol.MethodInitialize:
		if !isCall {
			s.log.Debug("dropping initialize notification")
			return false, nil
		}
		s.mu.Lock()
		state := s.state
		s.mu.Unlock()
		if state != stateCreated {
			return false, jsonrpc2.Errorf(jsonrpc2.InvalidRequest, "server is already initialized")
		}
		return true, nil

	case protocol.MethodExit, protocol.MethodSetTrace:
		return true, nil

	case protocol.MethodShutdown:
		if !isCall {
			s.log.Debug("dropping shutdown notification")
			return false, nil
		}
	}

	s.mu.Lock()
	state := s.state
	s.mu.Unlock()

	switch state {
	case stateCreated:
		if isCall {
			return false, jsonrpc2.NewError(jsonrpc2.ServerNotInitialized, "server not initialized")
		}
		s.log.Debug("dropping notification before initialize", "method", method)
		return false, nil

	case stateShutdown, stateExited:
		if isCall {
			return false, jsonrpc2.Errorf(jsonrpc2.InvalidRequest, "server is shutting down")
		}
		s.log.Debug("dropping notification after shutdown", "method", method)
		return false, nil
	}

	return true, nil
}

// Initialize negotiates capabilities and position encoding.
func (s *Server) Initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	if params == nil {
		return nil, jsonrpc2.NewError(jsonrpc2.InvalidParams, "initialize: missing params")
	}

	encoding := chooseEncoding(params)
	pull := supportsConfiguration(params)
	diagnosticPull, diagnosticRelated := supportsPullDiagnostics(params)

	s.docs.SetEncoding(encoding)
	s.carts.SetEncoding(encoding)
	s.cfg.SetPullSupported(pull)
	s.cfg.ApplyInitializationOptions(params.InitializationOptions)

	s.mu.Lock()
	if s.state != stateCreated {
		s.mu.Unlock()
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidRequest, "server is already initialized")
	}
	s.encoding = encoding
	s.state = stateInitialized
	s.diagnosticPull = diagnosticPull
	s.diagnosticRelated = diagnosticRelated
	if params.Trace != "" {
		s.trace = params.Trace
	}
	s.mu.Unlock()
	s.applyDiagnosticTransport(ctx)

	s.setWorkspaceFolders(initializeFolders(params))
	if params.Capabilities.Workspace != nil && params.Capabilities.Workspace.DidChangeWatchedFiles != nil {
		capability := params.Capabilities.Workspace.DidChangeWatchedFiles
		s.watchedFilesDynamic = capability.DynamicRegistration != nil && *capability.DynamicRegistration
	}

	clientName := params.ClientInfo.Name
	clientVersion, _ := params.ClientInfo.Version.Get()
	s.log.Info("initialized",
		"client", clientName,
		"clientVersion", clientVersion,
		"encoding", encoding,
		"configurationPull", pull,
	)

	change := protocol.TextDocumentSyncKindIncremental
	syncOptions := &protocol.TextDocumentSyncOptions{
		OpenClose: boolPtr(true),
		Change:    &change,
		Save:      &protocol.SaveOptions{IncludeText: boolPtr(false)},
	}

	capabilities := protocol.ServerCapabilities{
		PositionEncoding: encoding,
		TextDocumentSync: syncOptions,
		CompletionProvider: &protocol.CompletionOptions{
			TriggerCharacters: []string{".", ":", "@", "#"},
			ResolveProvider:   boolPtr(true),
		},
		HoverProvider:              protocol.Boolean(true),
		SignatureHelpProvider:      &protocol.SignatureHelpOptions{TriggerCharacters: []string{"(", ","}, RetriggerCharacters: []string{","}},
		DocumentFormattingProvider: protocol.Boolean(true),
		CodeActionProvider:         protocol.Boolean(true),
		CodeLensProvider:           &protocol.CodeLensOptions{},
		ExecuteCommandProvider:     protocol.ExecuteCommandOptions{Commands: []string{CommandShowCartInfo, CommandRemoveUnused}},
		DefinitionProvider:         protocol.Boolean(true),
		ReferencesProvider:         protocol.Boolean(true),
		DocumentHighlightProvider:  protocol.Boolean(true),
		DocumentSymbolProvider:     protocol.Boolean(true),
		FoldingRangeProvider:       protocol.Boolean(true),
		WorkspaceSymbolProvider:    protocol.Boolean(true),
		RenameProvider:             &protocol.RenameOptions{PrepareProvider: boolPtr(true)},
		SemanticTokensProvider:     semanticTokensOptions(),
		InlayHintProvider:          protocol.Boolean(true),
		Workspace: &protocol.WorkspaceOptions{
			WorkspaceFolders: &protocol.WorkspaceFoldersServerCapabilities{
				Supported:           boolPtr(true),
				ChangeNotifications: protocol.Boolean(true),
			},
		},
	}
	if diagnosticPull {
		capabilities.DiagnosticProvider = &protocol.DiagnosticOptions{
			InterFileDependencies: true,
			WorkspaceDiagnostics:  true,
		}
	}

	return &protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: protocol.ServerInfo{
			Name:    "pico-lsp",
			Version: protocol.NewOptional(version.Current()),
		},
	}, nil
}

// applyDiagnosticTransport reconciles push publication with the configured mode
// and the client capability. Pull mode without client support falls back to
// push so a configuration mistake cannot hide every diagnostic.
func (s *Server) applyDiagnosticTransport(ctx context.Context) {
	settings := s.cfg.Settings()
	s.mu.Lock()
	supported := s.diagnosticPull
	warned := s.warnedPullFallback
	if settings.Diagnostics.Mode == config.DiagnosticsModePull && !supported {
		s.warnedPullFallback = true
	}
	s.mu.Unlock()

	if settings.Diagnostics.Mode == config.DiagnosticsModePull && !supported && !warned {
		s.log.Warn("pico8.diagnostics.mode is pull but the client does not support pull diagnostics; falling back to push")
	}

	push := settings.Diagnostics.Mode == config.DiagnosticsModePush ||
		!supported
	s.diags.SetPushAllowed(ctx, push)
}

func (s *Server) relatedDocumentsSupported() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.diagnosticRelated
}

func supportsPullDiagnostics(params *protocol.InitializeParams) (pull, related bool) {
	if params == nil || params.Capabilities.TextDocument == nil {
		return false, false
	}
	capability := params.Capabilities.TextDocument.Diagnostic
	if capability == nil {
		return false, false
	}
	related = capability.RelatedDocumentSupport != nil && *capability.RelatedDocumentSupport
	return true, related
}

// Initialized enables diagnostics and pulls configuration without blocking the
// initialize request.
func (s *Server) Initialized(ctx context.Context, params *protocol.InitializedParams) error {
	s.diags.Enable()
	s.log.Debug("client initialized notification received")
	s.carts.AnalyzeOpen(ctx)
	if s.Client() != nil && (s.watchedFilesDynamic || s.cfg.SupportsPull()) {
		// Both registration and configuration pull are server-initiated
		// requests. Release the read loop once so their responses can arrive
		// while this handler is still running.
		jsonrpc2.Async(ctx)
	}
	s.registerFileWatchers(ctx)
	s.pullConfiguration(ctx)
	return nil
}

// Shutdown marks the server as shutting down.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	state := s.state
	if state == stateCreated {
		s.mu.Unlock()
		return jsonrpc2.Errorf(jsonrpc2.InvalidRequest, "shutdown before initialize")
	}
	if state != stateExited {
		s.state = stateShutdown
	}
	s.mu.Unlock()

	s.log.Info("shutdown requested")
	return nil
}

// Exit records the process exit code. It exits with 0 only when shutdown was
// received first.
func (s *Server) Exit(ctx context.Context) error {
	code := 1

	s.mu.Lock()
	if s.state == stateShutdown {
		code = 0
	}
	s.state = stateExited
	s.exitCode = code
	s.mu.Unlock()

	s.exitOnce.Do(func() { close(s.exitDone) })

	s.log.Info("exit notification received", "code", code)
	return nil
}

// SetTrace stores the client's requested trace level. Detailed trace logging is
// intentionally minimal in M0.
func (s *Server) SetTrace(ctx context.Context, params *protocol.SetTraceParams) error {
	if params == nil {
		return nil
	}

	s.mu.Lock()
	s.trace = params.Value
	s.mu.Unlock()

	s.log.Debug("trace level changed", "value", params.Value)
	return nil
}

// DidChangeConfiguration pulls configuration when possible and otherwise
// applies settings pushed by the client.
func (s *Server) DidChangeConfiguration(ctx context.Context, params *protocol.DidChangeConfigurationParams) error {
	if s.cfg.SupportsPull() {
		if s.Client() != nil {
			// Release the read loop before the server-initiated pull request.
			jsonrpc2.Async(ctx)
		}
		s.pullConfiguration(ctx)
		return nil
	}
	if params != nil {
		s.cfg.ApplyDidChange(params.Settings)
		s.applyDiagnosticTransport(ctx)
		s.carts.AnalyzeOpen(ctx)
	}
	return nil
}

// Request handles non-standard methods. $/cancelRequest is handled by dispatch
// before this method is reached.
func (s *Server) Request(ctx context.Context, method string, params any) (any, error) {
	switch method {
	case RequestCartInfo:
		raw, ok := params.(protocol.LSPAny)
		if !ok {
			return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "%s: invalid params", method)
		}
		return s.cartInfoRequest(ctx, raw)
	case RequestDecodeCart:
		raw, ok := params.(protocol.LSPAny)
		if !ok {
			return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "%s: invalid params", method)
		}
		return s.decodeCartRequest(ctx, raw)
	default:
		return nil, errUnhandledMethod
	}
}

func (s *Server) pullConfiguration(ctx context.Context) {
	if !s.cfg.SupportsPull() {
		return
	}
	client := s.Client()
	if client == nil {
		return
	}

	s.pullMu.Lock()
	defer s.pullMu.Unlock()

	pullCtx, cancel := context.WithTimeout(jsonrpc2.DetachContext(ctx), 5*time.Second)
	defer cancel()

	if err := s.cfg.Pull(pullCtx, client); err != nil {
		s.log.Warn("configuration pull failed", "error", err)
		return
	}
	// Diagnostics and navigation depend on the configured severity and
	// transport, so reconcile both before republishing.
	s.applyDiagnosticTransport(ctx)
	s.carts.AnalyzeOpen(ctx)
}

func chooseEncoding(params *protocol.InitializeParams) protocol.PositionEncodingKind {
	if params != nil && params.Capabilities.General != nil {
		for _, encoding := range params.Capabilities.General.PositionEncodings {
			if encoding == protocol.PositionEncodingKindUTF8 {
				return protocol.PositionEncodingKindUTF8
			}
		}
	}
	return protocol.PositionEncodingKindUTF16
}

func supportsConfiguration(params *protocol.InitializeParams) bool {
	return params != nil &&
		params.Capabilities.Workspace != nil &&
		params.Capabilities.Workspace.Configuration != nil &&
		*params.Capabilities.Workspace.Configuration
}

func boolPtr(v bool) *bool { return &v }
