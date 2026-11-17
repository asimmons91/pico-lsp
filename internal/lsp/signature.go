package lsp

import (
	"context"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

type callSite struct {
	open   source.Span
	close  source.Span
	callee *analysis.Symbol
	args   []source.Span
}

// SignatureHelp implements textDocument/signatureHelp.
func (s *Server) SignatureHelp(ctx context.Context, params *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
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
	call := nav.innermostCall(offset)
	if call == nil || call.callee == nil || call.callee.Signature == "" {
		return nil, nil
	}
	return buildSignatureHelp(nav, call, offset), nil
}

func (n *navContext) innermostCall(offset int) *callSite {
	if n == nil || n.snap == nil || n.semantics == nil {
		return nil
	}
	var best *callSite
	syntax.Walk(n.snap.Chunk, func(node syntax.Node) bool {
		switch call := node.(type) {
		case *syntax.CallExpr:
			if !n.callContains(call.Open, call.Close, offset) {
				return true
			}
			site := &callSite{open: call.Open, close: call.Close, args: exprSpans(call.Args)}
			switch fn := call.Func.(type) {
			case *syntax.NameExpr:
				site.callee = n.symbolAtSpan(fn.Span())
			case *syntax.FieldExpr:
				site.callee = n.symbolAtSpan(fn.Name.Span())
			}
			if best == nil || call.Open.Start > best.open.Start {
				best = site
			}
		case *syntax.MethodCallExpr:
			if !n.callContains(call.Open, call.Close, offset) {
				return true
			}
			site := &callSite{open: call.Open, close: call.Close, args: exprSpans(call.Args)}
			if call.Method != nil {
				site.callee = n.symbolAtSpan(call.Method.Span())
			}
			if best == nil || call.Open.Start > best.open.Start {
				best = site
			}
		}
		return true
	})
	return best
}

func (n *navContext) callContains(open, close source.Span, offset int) bool {
	if open.FileID != n.fileID || open.IsEmpty() || offset < open.Start {
		return false
	}
	if open.Start >= len(n.file.Text) || n.file.Text[open.Start] != '(' {
		return false
	}
	return close.IsEmpty() || offset <= close.Start
}

func (n *navContext) symbolAtSpan(span source.Span) *analysis.Symbol {
	occ := n.semantics.OccurrenceAt(n.fileID, span.Start)
	if occ == nil {
		return nil
	}
	return occ.Symbol
}

func buildSignatureHelp(nav *navContext, call *callSite, offset int) *protocol.SignatureHelp {
	sym := call.callee
	if sym == nil {
		return nil
	}
	params := make([]protocol.ParameterInformation, 0, len(sym.Params))
	for _, name := range sym.Params {
		param := protocol.ParameterInformation{Label: protocol.String(name)}
		if doc := paramDocumentation(sym, name); doc != "" {
			param.Documentation = &protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: doc}
		}
		params = append(params, param)
	}

	active := activeParameterIndex(call.args, offset)
	if len(params) > 0 && active >= len(params) {
		active = len(params) - 1
	}
	info := protocol.SignatureInformation{
		Label:      sym.Signature,
		Parameters: params,
	}
	if summary := docSummary(sym); summary != "" {
		info.Documentation = &protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: summary}
	}
	if len(params) > 0 {
		info.ActiveParameter = protocol.NewNullable(uint32(active))
	}
	activeSignature := uint32(0)
	help := &protocol.SignatureHelp{
		Signatures:      []protocol.SignatureInformation{info},
		ActiveSignature: &activeSignature,
	}
	if len(params) > 0 {
		help.ActiveParameter = protocol.NewNullable(uint32(active))
	}
	return help
}

func paramDocumentation(sym *analysis.Symbol, name string) string {
	if sym == nil || sym.Doc == nil {
		return ""
	}
	for _, param := range sym.Doc.Params {
		if !strings.EqualFold(param.Name, name) {
			continue
		}
		parts := make([]string, 0, 2)
		if param.Type != "" {
			parts = append(parts, "`"+param.Type+"`")
		}
		if param.Doc != "" {
			parts = append(parts, markdownInline(param.Doc))
		}
		return strings.Join(parts, " ")
	}
	return ""
}

func docSummary(sym *analysis.Symbol) string {
	if sym == nil || sym.Doc == nil {
		return ""
	}
	return markdownInline(strings.TrimSpace(sym.Doc.Summary))
}

func exprSpans(exprs []syntax.Expr) []source.Span {
	spans := make([]source.Span, 0, len(exprs))
	for _, expr := range exprs {
		if expr == nil {
			continue
		}
		spans = append(spans, expr.Span())
	}
	return spans
}

// activeParameterIndex derives the active argument from parsed argument spans
// rather than raw text. A text-only comma scan miscounts commas inside nested
// function bodies, which are not argument separators of the enclosing call.
func activeParameterIndex(args []source.Span, offset int) int {
	active := 0
	for i, span := range args {
		if span.IsEmpty() {
			continue
		}
		if offset <= span.End {
			active = i
			break
		}
		active = i + 1
	}
	return active
}
