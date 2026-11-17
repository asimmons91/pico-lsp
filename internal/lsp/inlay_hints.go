package lsp

import (
	"context"
	"fmt"

	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/config"
)

// InlayHint implements textDocument/inlayHint for inferred local types.
func (s *Server) InlayHint(ctx context.Context, params *protocol.InlayHintParams) ([]protocol.InlayHint, error) {
	if params == nil {
		return nil, nil
	}
	settings := s.cfg.Settings()
	if !settings.InlayHints || settings.TypeChecking == config.TypeCheckingOff {
		return nil, nil
	}
	nav, ok := s.navigationFor(ctx, params.TextDocument.URI)
	if !ok || nav.file == nil || nav.file.Index == nil {
		return nil, nil
	}
	start, end, err := nav.file.Index.RangeOffsets(params.Range)
	if err != nil {
		return nil, nil
	}
	hints := make([]protocol.InlayHint, 0)
	seen := make(map[string]struct{})
	for _, sym := range nav.semantics.Table.Symbols {
		if sym == nil || sym.Kind != analysis.SymbolLocal || sym.TypeExplicit || sym.Type == nil {
			continue
		}
		if sym.DeclSpan.FileID != nav.fileID || sym.DeclSpan.Start < start || sym.DeclSpan.Start > end {
			continue
		}
		if types.IsAny(sym.Type) || types.IsNil(sym.Type) {
			continue
		}
		pos, err := nav.file.Index.Position(sym.DeclSpan.End)
		if err != nil {
			continue
		}
		label := ": " + types.Display(sym.Type)
		key := fmt.Sprintf("%d:%d:%s", pos.Line, pos.Character, label)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		hints = append(hints, protocol.InlayHint{
			Position:     pos,
			Label:        protocol.String(label),
			Kind:         protocol.InlayHintKindType,
			PaddingLeft:  boolPtr(false),
			PaddingRight: boolPtr(true),
		})
	}
	return hints, nil
}
