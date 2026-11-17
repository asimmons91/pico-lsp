package lsp

import (
	"context"
	"strings"
	"unicode/utf8"

	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

type callSite struct {
	open   source.Span
	close  source.Span
	callee *analysis.Symbol
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
			site := &callSite{open: call.Open, close: call.Close}
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
			site := &callSite{open: call.Open, close: call.Close}
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

	active := topLevelCommaCount(nav, call.open.End, offset)
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

func topLevelCommaCount(nav *navContext, start, offset int) int {
	if nav == nil || nav.file == nil || start >= offset || offset > len(nav.file.Text) {
		return 0
	}
	text := nav.file.Text
	skip := nav.skipSpans()
	count := 0
	depth := 0
	for i := start; i < offset; {
		for _, span := range skip {
			if span.FileID == nav.fileID && i >= span.Start && i < span.End {
				i = span.End
				break
			}
		}
		if i >= offset {
			break
		}
		if i < 0 || i >= len(text) {
			break
		}
		switch text[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				count++
			}
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		if size <= 0 {
			size = 1
		}
		i += size
	}
	return count
}

func (n *navContext) skipSpans() []source.Span {
	if n == nil || n.snap == nil {
		return nil
	}
	spans := make([]source.Span, 0)
	for _, token := range n.snap.Tokens {
		if token.Span.FileID == n.fileID && token.Kind == syntax.TokenString {
			spans = append(spans, token.Span)
		}
	}
	for _, comment := range n.snap.Comments {
		if comment.Span.FileID == n.fileID {
			spans = append(spans, comment.Span)
		}
	}
	return spans
}
