package lsp

import (
	"context"
	"fmt"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/cart"
)

// DocumentLink implements textDocument/documentLink for resolved #include
// directives in the requested file.
func (s *Server) DocumentLink(ctx context.Context, params *protocol.DocumentLinkParams) ([]protocol.DocumentLink, error) {
	if params == nil || params.TextDocument.URI == "" {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "textDocument/documentLink: invalid params")
	}

	u := canonicalVirtualURI(params.TextDocument.URI)
	if isVirtualCartURI(u) || isPNGURI(u) {
		return []protocol.DocumentLink{}, nil
	}
	if !u.IsFile() {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "textDocument/documentLink: document is not a file")
	}
	if _, open := s.docs.Get(u); !open {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "textDocument/documentLink: document is not open")
	}

	nav, ok := s.navigationFor(ctx, u)
	if !ok || nav.snap == nil || nav.snap.Cart == nil || nav.file == nil || nav.file.Index == nil {
		return nil, jsonrpc2.Errorf(requestFailedCode, "textDocument/documentLink: document is not available")
	}

	links := make([]protocol.DocumentLink, 0, 4)
	seen := make(map[string]struct{})
	includes := nav.snap.Cart.Includes
	if len(includes) > cart.MaxIncludes {
		includes = includes[:cart.MaxIncludes]
	}
	for i := range includes {
		include := &includes[i]
		if include.Nested || include.Missing || include.Unreadable || include.ResolvedURI == "" {
			continue
		}
		if include.DirectiveSpan.FileID != nav.fileID {
			continue
		}

		span := include.PathSpan
		if span.FileID != nav.fileID || span.IsEmpty() {
			// Older snapshots and malformed directives may not carry a path
			// span. The directive span is still a useful link target.
			span = include.DirectiveSpan
		}
		if span.FileID != nav.fileID || span.IsEmpty() {
			continue
		}

		r := nav.file.Index.Range(span.Start, span.End)
		target := include.ResolvedURI
		key := linkKey(r, target)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}

		tooltip := "Open " + includeDisplayPath(*include)
		if include.Tab >= 0 {
			tooltip += fmt.Sprintf(" (tab %d)", include.Tab)
		}
		links = append(links, protocol.DocumentLink{
			Range:   r,
			Target:  &target,
			Tooltip: &tooltip,
		})
	}
	return links, nil
}

func includeDisplayPath(include cart.Include) string {
	if include.Tab < 0 {
		return include.Path
	}
	if i := strings.LastIndex(include.Path, ":"); i >= 0 {
		return include.Path[:i]
	}
	return include.Path
}

func linkKey(r protocol.Range, target uri.URI) string {
	return fmt.Sprintf("%d:%d:%d:%d:%s",
		r.Start.Line, r.Start.Character, r.End.Line, r.End.Character, target.String())
}
