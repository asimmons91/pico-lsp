package lsp

import (
	"context"
	"strings"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// DidChangeWatchedFiles re-analyzes carts affected by filesystem changes.
func (s *Server) DidChangeWatchedFiles(ctx context.Context, params *protocol.DidChangeWatchedFilesParams) error {
	if params == nil {
		return nil
	}
	for _, change := range params.Changes {
		if change.URI == "" {
			continue
		}
		lower := strings.ToLower(change.URI.Path())
		if strings.HasSuffix(lower, ".p8.png") {
			s.watchedPNG(ctx, change.URI, change.Type)
			continue
		}
		if !strings.HasSuffix(lower, ".p8") && !strings.HasSuffix(lower, ".lua") {
			continue
		}
		if change.Type == protocol.FileChangeTypeDeleted {
			s.carts.Closed(ctx, change.URI)
			continue
		}
		s.carts.WatchedChange(ctx, change.URI)
	}
	return nil
}

func (s *Server) watchedPNG(ctx context.Context, source uri.URI, kind protocol.FileChangeType) {
	virtual := virtualCartURI(source)
	s.png.invalidate(source)
	if kind == protocol.FileChangeTypeDeleted {
		s.docs.Close(virtual)
		s.carts.Forget(ctx, virtual)
		s.carts.RemoveVirtualText(virtual)
		s.diags.Clear(ctx, virtual)
		return
	}
	entry, err := s.decodePNG(source)
	if err != nil {
		s.log.Warn("failed to decode watched PNG cart", "uri", source.String(), "error", err)
		return
	}
	if doc, ok := s.docs.Get(virtual); ok {
		s.docs.Open(virtual, "pico8", doc.Version+1, entry.result.Text)
		s.carts.Analyze(ctx, virtual)
	}
}

func (s *Server) registerFileWatchers(ctx context.Context) {
	if !s.watchedFilesDynamic {
		return
	}
	client := s.Client()
	if client == nil {
		return
	}
	options, err := protocol.Marshal(protocol.DidChangeWatchedFilesRegistrationOptions{
		Watchers: []protocol.FileSystemWatcher{
			{GlobPattern: protocol.Pattern("**/*.p8")},
			{GlobPattern: protocol.Pattern("**/*.p8.png")},
			{GlobPattern: protocol.Pattern("**/*.lua")},
		},
	})
	if err != nil {
		s.log.Warn("failed to encode file watcher registration", "error", err)
		return
	}
	watchCtx, cancel := context.WithTimeout(jsonrpc2.DetachContext(ctx), 5*time.Second)
	defer cancel()
	err = client.RegisterCapability(watchCtx, &protocol.RegistrationParams{
		Registrations: []protocol.Registration{{
			ID:              "pico8-watched-files",
			Method:          protocol.MethodWorkspaceDidChangeWatchedFiles,
			RegisterOptions: protocol.LSPAny(options),
		}},
	})
	if err != nil {
		s.log.Warn("failed to register file watchers", "error", err)
	}
}
