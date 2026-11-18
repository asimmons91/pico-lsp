package lsp

import (
	"context"
	"fmt"

	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/cart"
)

// CodeLens implements textDocument/codeLens with the cart budget lens.
func (s *Server) CodeLens(ctx context.Context, params *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	if params == nil {
		return nil, nil
	}
	settings := s.cfg.Settings()
	if !settings.Limits.CodeLens {
		return nil, nil
	}
	nav, ok := s.navigationFor(ctx, params.TextDocument.URI)
	if !ok || nav.snap == nil || nav.snap.Cart == nil || nav.file == nil || !nav.snap.Cart.CountsAvailable {
		return nil, nil
	}
	if nav.file.URI != nav.snap.Cart.RootURI {
		return nil, nil
	}
	label := cartBudgetLabel(nav.snap)
	args, _ := protocol.Marshal(nav.snap.Cart.RootURI.String())
	return []protocol.CodeLens{{
		Range: protocol.Range{Start: protocol.Position{}, End: protocol.Position{}},
		Command: protocol.Command{
			Title:     label,
			Command:   CommandShowCartInfo,
			Arguments: []protocol.LSPAny{args},
		},
	}}, nil
}

func cartBudgetLabel(snap *cart.Snapshot) string {
	if snap == nil || snap.Cart == nil {
		return ""
	}
	label := fmt.Sprintf("%d / %d tokens · %d / %d chars · ",
		snap.Cart.Tokens, TokenLimit, snap.Cart.Chars, CharLimit)
	size, ok := snap.CompressedSize()
	if !ok {
		return label + "compressed n/a"
	}
	label += fmt.Sprintf("%d / %d compressed", size, CompressedLimit)
	switch {
	case size > CompressedLimit:
		label += " (over limit)"
	case size >= CompressedWarning:
		label += " (warning)"
	}
	return label
}
