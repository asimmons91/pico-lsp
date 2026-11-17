package lsp

import (
	"context"

	"go.lsp.dev/protocol"
)

// References implements textDocument/references across every file in the cart.
// Built-ins intentionally return no references.
func (s *Server) References(ctx context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	if params == nil {
		return []protocol.Location{}, nil
	}
	nav, ok := s.navigationFor(ctx, params.TextDocument.URI)
	if !ok {
		return []protocol.Location{}, nil
	}
	offset, ok := nav.offset(params.Position)
	if !ok {
		return []protocol.Location{}, nil
	}
	occ := nav.occurrenceAt(offset)
	if occ == nil || occ.Symbol == nil || occ.Symbol.Builtin {
		return []protocol.Location{}, nil
	}
	spans := nav.semantics.References(occ.Symbol, params.Context.IncludeDeclaration)
	return nav.locations(spans), nil
}
