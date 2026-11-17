package lsp

import (
	"context"

	"go.lsp.dev/protocol"
)

// The handlers below are placeholders for later milestones. They are wired in
// now so features can replace them individually, but their capabilities are not
// advertised until they return real results.

// DocumentSymbol is implemented in M1.
func (s *Server) DocumentSymbol(ctx context.Context, params *protocol.DocumentSymbolParams) (protocol.DocumentSymbolResult, error) {
	return nil, nil
}

// Definition is implemented in M2.
func (s *Server) Definition(ctx context.Context, params *protocol.DefinitionParams) (protocol.DefinitionResult, error) {
	return nil, nil
}

// References is implemented in M2.
func (s *Server) References(ctx context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	return []protocol.Location{}, nil
}

// Hover is implemented in M3.
func (s *Server) Hover(ctx context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	return nil, nil
}

// Completion is implemented in M3.
func (s *Server) Completion(ctx context.Context, params *protocol.CompletionParams) (protocol.CompletionResult, error) {
	return nil, nil
}

// SignatureHelp is implemented in M3.
func (s *Server) SignatureHelp(ctx context.Context, params *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
	return nil, nil
}

// FoldingRanges is implemented in M1.
func (s *Server) FoldingRanges(ctx context.Context, params *protocol.FoldingRangeParams) ([]protocol.FoldingRange, error) {
	return []protocol.FoldingRange{}, nil
}

// SemanticTokensFull is implemented in M3.
func (s *Server) SemanticTokensFull(ctx context.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	return &protocol.SemanticTokens{Data: []uint32{}}, nil
}

// CodeAction is implemented in M5.
func (s *Server) CodeAction(ctx context.Context, params *protocol.CodeActionParams) ([]protocol.CommandOrCodeAction, error) {
	return []protocol.CommandOrCodeAction{}, nil
}

// CodeLens is implemented in M5.
func (s *Server) CodeLens(ctx context.Context, params *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	return []protocol.CodeLens{}, nil
}

// Symbols handles workspace/symbol. It is implemented in M2.
func (s *Server) Symbols(ctx context.Context, params *protocol.WorkspaceSymbolParams) (protocol.WorkspaceSymbolResult, error) {
	return nil, nil
}

// ExecuteCommand handles workspace/executeCommand. It is implemented in M5.
func (s *Server) ExecuteCommand(ctx context.Context, params *protocol.ExecuteCommandParams) (protocol.LSPAny, error) {
	return nil, nil
}
