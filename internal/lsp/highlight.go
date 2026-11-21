package lsp

import (
	"context"
	"sort"

	"go.lsp.dev/protocol"
)

// DocumentHighlight implements textDocument/documentHighlight within the
// current file. Declarations and writes are reported as Write occurrences.
func (s *Server) DocumentHighlight(ctx context.Context, params *protocol.DocumentHighlightParams) ([]protocol.DocumentHighlight, error) {
	if params == nil {
		return []protocol.DocumentHighlight{}, nil
	}
	nav, ok := s.navigationFor(ctx, params.TextDocument.URI)
	if !ok {
		return []protocol.DocumentHighlight{}, nil
	}
	offset, ok := nav.offset(params.Position)
	if !ok {
		return []protocol.DocumentHighlight{}, nil
	}
	occ := nav.occurrenceAt(offset)
	if occ == nil || occ.Symbol == nil {
		return []protocol.DocumentHighlight{}, nil
	}

	out := make([]protocol.DocumentHighlight, 0)
	for _, candidate := range nav.semantics.Occurrences(nav.fileID) {
		if !nav.semantics.SameSymbol(candidate.Symbol, occ.Symbol) {
			continue
		}
		kind := protocol.DocumentHighlightKindRead
		if candidate.Write {
			kind = protocol.DocumentHighlightKindWrite
		}
		out = append(out, protocol.DocumentHighlight{
			Range: nav.file.Index.Range(candidate.Span.Start, candidate.Span.End),
			Kind:  kind,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Range.Start.Line != out[j].Range.Start.Line {
			return out[i].Range.Start.Line < out[j].Range.Start.Line
		}
		return out[i].Range.Start.Character < out[j].Range.Start.Character
	})
	return out, nil
}
