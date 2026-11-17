package lsp

import (
	"context"
	"errors"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/format"
)

const requestFailedCode jsonrpc2.Code = -32803

// Formatting implements textDocument/formatting with a conservative,
// AST-guided formatter.
func (s *Server) Formatting(ctx context.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	if params == nil || params.TextDocument.URI == "" {
		return nil, nil
	}
	u := params.TextDocument.URI
	if isVirtualCartURI(u) {
		return nil, jsonrpc2.Errorf(requestFailedCode,
			"cannot format read-only decoded .p8.png carts")
	}
	lower := strings.ToLower(u.Path())
	if strings.HasSuffix(lower, ".p8.png") {
		return nil, jsonrpc2.Errorf(requestFailedCode,
			"cannot format .p8.png carts; export a text .p8 first")
	}
	nav, ok := s.navigationFor(ctx, u)
	if !ok || nav.file == nil || nav.file.Index == nil {
		return nil, nil
	}

	if nav.snap.Cart.RootURI == u {
		for _, include := range nav.snap.Cart.Includes {
			if include.Missing || include.Unreadable {
				return nil, jsonrpc2.Errorf(requestFailedCode,
					"cannot format cart with missing or unreadable includes")
			}
		}
	}

	settings := s.cfg.Settings()
	opts := format.DefaultOptions()
	opts.MaxBlankLines = settings.Format.MaxBlankLines
	if params.Options.TabSize > 0 {
		opts.TabSize = int(params.Options.TabSize)
	}
	if params.Options.TabSize > 0 || params.Options.InsertSpaces {
		opts.InsertSpaces = params.Options.InsertSpaces
	}
	if params.Options.TrimTrailingWhitespace != nil {
		opts.TrimTrailingWhitespace = *params.Options.TrimTrailingWhitespace
	}
	if params.Options.InsertFinalNewline != nil {
		opts.InsertFinalNewline = *params.Options.InsertFinalNewline
	}
	if params.Options.TrimFinalNewlines != nil {
		opts.TrimFinalNewlines = *params.Options.TrimFinalNewlines
	}

	formatted, err := format.Document(nav.file.Text, opts, strings.HasSuffix(lower, ".p8"))
	if errors.Is(err, format.ErrSyntax) {
		return nil, jsonrpc2.Errorf(requestFailedCode,
			"cannot format document with syntax errors; fix the errors first")
	}
	if err != nil {
		return nil, jsonrpc2.Errorf(requestFailedCode, "cannot format document safely: %v", err)
	}
	if formatted == nav.file.Text {
		return []protocol.TextEdit{}, nil
	}
	return []protocol.TextEdit{{
		Range:   nav.file.Index.Range(0, len(nav.file.Text)),
		NewText: formatted,
	}}, nil
}
