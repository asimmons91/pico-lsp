package lsp

import (
	"context"

	"go.lsp.dev/protocol"
)

// Definition implements textDocument/definition for locals, globals,
// functions, methods, fields, labels, and include paths. Built-ins have no
// user-navigable definition in v1.
func (s *Server) Definition(ctx context.Context, params *protocol.DefinitionParams) (protocol.DefinitionResult, error) {
	if params == nil {
		return protocol.LocationSlice{}, nil
	}
	nav, ok := s.navigationFor(ctx, params.TextDocument.URI)
	if !ok {
		return protocol.LocationSlice{}, nil
	}
	offset, ok := nav.offset(params.Position)
	if !ok {
		return protocol.LocationSlice{}, nil
	}

	if include := nav.includeAt(offset); include != nil {
		if loc, ok := nav.includeTarget(include); ok {
			return protocol.LocationSlice{loc}, nil
		}
		return protocol.LocationSlice{}, nil
	}

	occ := nav.occurrenceAt(offset)
	if occ == nil || occ.Symbol == nil || occ.Symbol.Builtin {
		return protocol.LocationSlice{}, nil
	}
	return protocol.LocationSlice(nav.locations(nav.semantics.Definitions(occ.Symbol))), nil
}
