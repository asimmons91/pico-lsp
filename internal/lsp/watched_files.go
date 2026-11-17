package lsp

import (
	"context"
	"strings"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
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
