package lsp

import (
	"context"
	"sort"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// FoldingRanges implements textDocument/foldingRange.
func (s *Server) FoldingRanges(ctx context.Context, params *protocol.FoldingRangeParams) ([]protocol.FoldingRange, error) {
	if params == nil {
		return []protocol.FoldingRange{}, nil
	}
	return s.foldingRanges(ctx, params.TextDocument.URI), nil
}

type foldingKey struct {
	start, end uint32
}

func (s *Server) foldingRanges(ctx context.Context, u uri.URI) []protocol.FoldingRange {
	snap, fileID, ok := s.carts.SnapshotFor(ctx, u)
	if !ok {
		return []protocol.FoldingRange{}
	}
	file := snap.Cart.File(fileID)
	if file == nil || cart.IsP8PNG(file.URI) {
		return []protocol.FoldingRange{}
	}

	seen := make(map[foldingKey]struct{})
	var out []protocol.FoldingRange
	addLines := func(start, end uint32, kind protocol.FoldingRangeKind) {
		if end <= start {
			return
		}
		key := foldingKey{start: start, end: end}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		out = append(out, protocol.FoldingRange{StartLine: start, EndLine: end, Kind: kind})
	}
	addSpan := func(span source.Span, kind protocol.FoldingRangeKind) {
		if span.FileID != fileID || span.IsEmpty() {
			return
		}
		start, end := lineRange(file, span)
		addLines(start, end, kind)
	}

	syntax.Walk(snap.Chunk, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.FunctionExpr:
			addSpan(n.Span(), "")
		case *syntax.IfStmt:
			if n.SingleLine {
				return true
			}
			for _, branch := range n.Branches {
				addSpan(branch.Span().Merge(branch.Body.Span()), "")
			}
			if n.Else != nil {
				addSpan(source.Span{FileID: n.ElseSpan.FileID, Start: n.ElseSpan.Start, End: n.Else.Span().End}, "")
			}
		case *syntax.WhileStmt:
			if !n.SingleLine {
				addSpan(n.Span(), "")
			}
		case *syntax.RepeatStmt:
			addSpan(n.Span(), "")
		case *syntax.NumericForStmt:
			addSpan(n.Span(), "")
		case *syntax.GenericForStmt:
			addSpan(n.Span(), "")
		case *syntax.DoStmt:
			addSpan(n.Span(), "")
		case *syntax.TableExpr:
			addSpan(n.Span(), "")
		}
		return true
	})

	for _, comment := range snap.Comments {
		if comment.Span.FileID == fileID {
			addSpan(comment.Span, protocol.FoldingRangeKindComment)
		}
	}
	for _, token := range snap.Tokens {
		if token.Kind == syntax.TokenString && token.Span.FileID == fileID {
			addSpan(token.Span, "")
		}
	}

	if fileHasSuffix(file.URI, ".p8") {
		for _, tab := range snap.Cart.Tabs {
			addLines(uint32(tab.StartLine), uint32(tab.EndLine), protocol.FoldingRangeKindRegion)
		}
		for _, section := range snap.Cart.Sections {
			if section.Name == "lua" {
				continue
			}
			addLines(uint32(section.StartLine), uint32(section.EndLine), protocol.FoldingRangeKindRegion)
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].StartLine != out[j].StartLine {
			return out[i].StartLine < out[j].StartLine
		}
		return out[i].EndLine < out[j].EndLine
	})
	return out
}

func lineRange(file *cart.File, span source.Span) (start, end uint32) {
	startPos, _ := file.Index.Position(span.Start)
	endPos, _ := file.Index.Position(span.End)
	if endPos.Line > startPos.Line && endPos.Character == 0 {
		endPos.Line--
	}
	return startPos.Line, endPos.Line
}
