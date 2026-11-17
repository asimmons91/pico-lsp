package lsp

import (
	"context"

	"go.lsp.dev/protocol"
)

// The handlers below are placeholders for later milestones. They are wired in
// now so features can replace them individually, but their capabilities are not
// advertised until they return real results.

// CodeAction is implemented in M5.
func (s *Server) CodeAction(ctx context.Context, params *protocol.CodeActionParams) ([]protocol.CommandOrCodeAction, error) {
	return []protocol.CommandOrCodeAction{}, nil
}

// CodeLens is implemented in M5.
func (s *Server) CodeLens(ctx context.Context, params *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	return []protocol.CodeLens{}, nil
}

// ExecuteCommand handles workspace/executeCommand. It is implemented in M5.
func (s *Server) ExecuteCommand(ctx context.Context, params *protocol.ExecuteCommandParams) (protocol.LSPAny, error) {
	return nil, nil
}
