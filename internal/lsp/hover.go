package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// Hover implements textDocument/hover.
func (s *Server) Hover(ctx context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	if params == nil {
		return nil, nil
	}
	nav, ok := s.navigationFor(ctx, params.TextDocument.URI)
	if !ok {
		return nil, nil
	}
	offset, ok := nav.offset(params.Position)
	if !ok {
		return nil, nil
	}

	if include := nav.includeAt(offset); include != nil {
		return hoverResult(nav, include.DirectiveSpan, includeMarkdown(nav, include)), nil
	}
	if token := nav.tokenAt(offset); token != nil {
		switch token.Kind {
		case syntax.TokenString, syntax.TokenComment:
			return nil, nil
		case syntax.TokenInclude:
			return nil, nil
		case syntax.TokenPunct:
			switch token.Text {
			case "?", "@", "%", "$":
				// tokenAt also matches the end boundary of a token. When an
				// identifier starts immediately after one of these operators,
				// the identifier's docs take precedence at its first byte.
				if occ := nav.occurrenceAt(offset); occ != nil && occ.Symbol != nil {
					return hoverResult(nav, occ.Span, occurrenceMarkdown(nav, occ)), nil
				}
				if token.Text == "?" {
					return hoverResult(nav, token.Span, shorthandMarkdown()), nil
				}
				return hoverResult(nav, token.Span, peekMarkdown(token.Text)), nil
			}
		}
	}

	occ := nav.occurrenceAt(offset)
	if occ == nil || occ.Symbol == nil {
		return nil, nil
	}
	return hoverResult(nav, occ.Span, occurrenceMarkdown(nav, occ)), nil
}

func occurrenceMarkdown(nav *navContext, occ *analysis.Occurrence) string {
	if occ == nil || occ.Symbol == nil {
		return ""
	}
	if typ, ok := nav.semantics.FlowType(occ.Span); ok && typ != nil {
		return symbolMarkdownWithType(nav, occ.Symbol, typ)
	}
	return symbolMarkdown(nav, occ.Symbol)
}

func hoverResult(nav *navContext, span source.Span, markdown string) *protocol.Hover {
	if markdown == "" {
		return nil
	}
	result := &protocol.Hover{
		Contents: &protocol.MarkupContent{
			Kind:  protocol.MarkupKindMarkdown,
			Value: markdown,
		},
	}
	if nav != nil && nav.file != nil && nav.file.Index != nil {
		r := nav.file.Index.Range(span.Start, span.End)
		result.Range = &r
	}
	return result
}
