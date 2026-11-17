package lsp

import (
	"context"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/analysis/types"
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
	if call == nil || call.callee == nil {
		return nil, nil
	}
	if len(call.callee.FunctionCandidates()) == 0 && call.callee.Signature == "" {
		return nil, nil
	}
	active := -1
	if params.Context.IsRetrigger && params.Context.ActiveSignatureHelp.ActiveSignature != nil {
		active = int(*params.Context.ActiveSignatureHelp.ActiveSignature)
	}
	return buildSignatureHelp(nav, call, offset, active), nil
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

func buildSignatureHelp(nav *navContext, call *callSite, offset, activeOverride int) *protocol.SignatureHelp {
	sym := call.callee
	if sym == nil {
		return nil
	}
	candidates := sym.FunctionCandidates()
	activeParameter := activeParameterIndex(nav, call, offset)
	provided := len(call.args)
	callArgs := nav.semantics.CallArguments(call.open)
	signatures := make([]protocol.SignatureInformation, 0, max(1, len(candidates)))
	if len(candidates) > 0 {
		for _, fn := range candidates {
			signatures = append(signatures, signatureInformation(sym, fn, activeParameter))
		}
	} else {
		params := make([]protocol.ParameterInformation, 0, len(sym.Params))
		for _, name := range sym.Params {
			param := protocol.ParameterInformation{Label: protocol.String(name)}
			if doc := paramDocumentation(sym, name); doc != "" {
				param.Documentation = &protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: doc}
			}
			params = append(params, param)
		}
		info := protocol.SignatureInformation{Label: sym.Signature, Parameters: params}
		if summary := docSummary(sym); summary != "" {
			info.Documentation = &protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: summary}
		}
		signatures = append(signatures, info)
	}
	activeSignature := chooseActiveSignature(candidates, callArgs, provided, activeParameter, activeOverride)
	help := &protocol.SignatureHelp{
		Signatures:      signatures,
		ActiveSignature: &activeSignature,
	}
	if int(activeSignature) < len(signatures) {
		params := signatures[activeSignature].Parameters
		if len(params) > 0 {
			active := activeParameter
			if active >= len(params) {
				active = len(params) - 1
			}
			help.ActiveParameter = protocol.NewNullable(uint32(active))
		}
	}
	return help
}

func signatureInformation(sym *analysis.Symbol, fn *types.Function, active int) protocol.SignatureInformation {
	params := make([]protocol.ParameterInformation, 0, len(fn.Params)+1)
	for _, param := range fn.Params {
		label := param.Name
		if label == "" {
			label = "arg"
		}
		info := protocol.ParameterInformation{Label: protocol.String(label)}
		if doc := typedParamDocumentation(param); doc != "" {
			info.Documentation = &protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: doc}
		}
		params = append(params, info)
	}
	if fn.Vararg != nil {
		info := protocol.ParameterInformation{Label: protocol.String("...")}
		info.Documentation = &protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: "`" + types.Display(fn.Vararg) + "`"}
		params = append(params, info)
	}
	info := protocol.SignatureInformation{
		Label:      typedFunctionSignature(sym, fn),
		Parameters: params,
	}
	if summary := docSummary(sym); summary != "" {
		info.Documentation = &protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: summary}
	}
	if len(params) > 0 {
		active = clamp(active, 0, len(params)-1)
		info.ActiveParameter = protocol.NewNullable(uint32(active))
	}
	return info
}

func chooseActiveSignature(candidates []*types.Function, args []types.Type, provided, activeParameter, override int) uint32 {
	if len(candidates) == 0 {
		return 0
	}
	if override >= 0 && override < len(candidates) {
		return uint32(override)
	}
	best := 0
	bestScore := int(^uint(0) >> 1)
	for i, fn := range candidates {
		score := signatureScore(fn, args, provided, activeParameter)
		if score < bestScore {
			best = i
			bestScore = score
		}
	}
	return uint32(best)
}

func signatureScore(fn *types.Function, args []types.Type, provided, activeParameter int) int {
	if fn == nil {
		return 10000
	}
	fn = types.BindFunction(fn, args)
	score := 0
	for i, arg := range args {
		if i >= len(fn.Params) {
			continue
		}
		target := fn.Params[i].Type
		if fn.Params[i].Optional {
			target = types.Optional(target)
		}
		if types.IsAny(target) || types.IsAny(arg) {
			score += 4
			continue
		}
		if !types.Assignable(arg, target, false) {
			score += 100
		}
	}
	if activeParameter >= len(fn.Params) {
		if fn.Vararg != nil {
			score += 2
		} else {
			score += 100
		}
	}
	required := 0
	for _, param := range fn.Params {
		if !param.Optional {
			required++
		}
	}
	if provided < required {
		score += (required - provided) * 8
	}
	extras := provided - len(fn.Params)
	if extras > 0 {
		if fn.Vararg == nil {
			score += extras * 8
		} else {
			score += extras * 2
		}
	}
	return score
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func typedParamDocumentation(param types.Param) string {
	parts := make([]string, 0, 2)
	if param.Type != nil {
		parts = append(parts, "`"+types.Display(param.Type)+"`")
	}
	if param.Doc != "" {
		parts = append(parts, markdownInline(param.Doc))
	}
	return strings.Join(parts, " ")
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

// activeParameterIndex counts top-level comma tokens before offset. Argument
// spans exclude commas inside nested calls, tables, function bodies, comments,
// and strings, while the token scan ensures a comma after trailing whitespace
// still advances the parameter at the comma itself, not at the whitespace
// before it.
func activeParameterIndex(nav *navContext, call *callSite, offset int) int {
	if nav == nil || nav.snap == nil || call == nil {
		return 0
	}
	active := 0
	for i := range nav.snap.Tokens {
		token := &nav.snap.Tokens[i]
		if token.Span.FileID != nav.fileID || token.Span.Start < call.open.Start || token.Span.Start >= offset {
			continue
		}
		if token.Kind != syntax.TokenPunct || token.Text != "," {
			continue
		}
		if spanContains(call.args, token.Span.Start) {
			continue
		}
		active++
	}
	return active
}

func spanContains(spans []source.Span, offset int) bool {
	for _, span := range spans {
		if span.Contains(offset) {
			return true
		}
	}
	return false
}
