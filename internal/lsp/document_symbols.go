package lsp

import (
	"context"
	"sort"
	"strings"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// DocumentSymbol implements textDocument/documentSymbol.
func (s *Server) DocumentSymbol(ctx context.Context, params *protocol.DocumentSymbolParams) (protocol.DocumentSymbolResult, error) {
	if params == nil {
		return protocol.DocumentSymbolSlice{}, nil
	}
	return s.documentSymbols(ctx, params.TextDocument.URI), nil
}

type outlineNode struct {
	name     string
	detail   string
	kind     protocol.SymbolKind
	span     source.Span
	sel      source.Span
	children []*outlineNode
}

func (s *Server) documentSymbols(ctx context.Context, u uri.URI) protocol.DocumentSymbolResult {
	snap, fileID, ok := s.carts.SnapshotFor(ctx, u)
	if !ok {
		return protocol.DocumentSymbolSlice{}
	}
	file := snap.Cart.File(fileID)
	if file == nil {
		return protocol.DocumentSymbolSlice{}
	}
	if cart.IsP8PNG(file.URI) {
		return protocol.DocumentSymbolSlice{}
	}

	rootNodes := collectOutline(snap.Chunk.Block, true)
	filtered := filterOutlineByFile(rootNodes, fileID)

	modules := make([]*outlineNode, 0)
	for _, include := range snap.Cart.Includes {
		if include.DirectiveSpan.FileID != fileID {
			continue
		}
		name := include.Path
		if name == "" {
			name = "(include)"
		}
		modules = append(modules, &outlineNode{
			name: name,
			kind: protocol.SymbolKindModule,
			span: include.DirectiveSpan,
			sel:  include.DirectiveSpan,
		})
	}

	isP8 := fileHasSuffix(file.URI, ".p8")
	if !isP8 {
		out := make([]protocol.DocumentSymbol, 0, len(filtered)+len(modules))
		for _, node := range filtered {
			out = append(out, toDocumentSymbol(file, node))
		}
		for _, module := range modules {
			out = append(out, toDocumentSymbol(file, module))
		}
		sortDocumentSymbols(out)
		return protocol.DocumentSymbolSlice(out)
	}

	tabs := snap.Cart.Tabs
	tabNodes := make([]*outlineNode, 0, len(tabs))
	tabByLine := make([]struct {
		start, end int
		node       *outlineNode
	}, len(tabs))
	for _, tab := range tabs {
		node := &outlineNode{
			name: tab.Name,
			kind: protocol.SymbolKindNamespace,
			span: tab.SpanVal,
			sel:  tab.MarkerSpan,
		}
		if node.sel.IsEmpty() {
			node.sel = tab.SpanVal
		}
		tabNodes = append(tabNodes, node)
	}
	for i, tab := range tabs {
		tabByLine[i].start = tab.StartLine
		tabByLine[i].end = tab.EndLine
		tabByLine[i].node = tabNodes[i]
	}

	var top []*outlineNode
	place := func(node *outlineNode) {
		line := lineOf(file, node.span.Start)
		for i := range tabByLine {
			if line >= tabByLine[i].start && line <= tabByLine[i].end {
				tabByLine[i].node.children = append(tabByLine[i].node.children, node)
				return
			}
		}
		top = append(top, node)
	}
	for _, node := range filtered {
		place(node)
	}
	for _, module := range modules {
		place(module)
	}

	out := make([]protocol.DocumentSymbol, 0, len(tabNodes)+len(top))
	for _, tab := range tabNodes {
		if len(tab.children) == 0 && tab.span.IsEmpty() {
			continue
		}
		out = append(out, toDocumentSymbol(file, tab))
	}
	for _, node := range top {
		out = append(out, toDocumentSymbol(file, node))
	}
	sortDocumentSymbols(out)
	return protocol.DocumentSymbolSlice(out)
}

// filterOutlineByFile keeps nodes declared in fileID and prunes children from
// other files. When a declaration in another file wraps declarations in
// fileID (for example, an include pasted inside a function), those declarations
// are promoted to the caller's level so per-file outlines never leak spans
// across files.
func filterOutlineByFile(nodes []*outlineNode, fileID source.FileID) []*outlineNode {
	var out []*outlineNode
	for _, node := range nodes {
		if node.span.FileID == fileID {
			copyNode := *node
			copyNode.children = filterOutlineByFile(node.children, fileID)
			out = append(out, &copyNode)
			continue
		}
		out = append(out, filterOutlineByFile(node.children, fileID)...)
	}
	return out
}

func collectOutline(block *syntax.Block, allowLocals bool) []*outlineNode {
	if block == nil {
		return nil
	}
	var out []*outlineNode
	for _, stmt := range block.Stmts {
		switch st := stmt.(type) {
		case *syntax.LocalStmt:
			if allowLocals {
				for i, name := range st.Names {
					kind := protocol.SymbolKindVariable
					if _, ok := valueAt(st.Values, i).(*syntax.FunctionExpr); ok {
						kind = protocol.SymbolKindFunction
					}
					out = append(out, &outlineNode{
						name: name.Text,
						kind: kind,
						span: st.Span(),
						sel:  name.Span(),
					})
				}
			}
		case *syntax.LocalFunctionStmt:
			node := functionOutline(st.Name.Text, st.Name.Span(), st.Span(), st.Func)
			out = append(out, node)
		case *syntax.FunctionDeclStmt:
			name := ""
			if len(st.Names) > 0 {
				name = st.Names[len(st.Names)-1].Text
			}
			kind := protocol.SymbolKindFunction
			if st.Method {
				kind = protocol.SymbolKindMethod
			}
			node := &outlineNode{
				name:     name,
				detail:   functionDetail(st.Names, st.Method),
				kind:     kind,
				span:     st.Span(),
				sel:      selectionOf(st.Names, st.Span()),
				children: collectOutline(st.Func.Body, false),
			}
			out = append(out, node)
		case *syntax.AssignStmt:
			for i, target := range st.Targets {
				if node := assignmentOutline(target, valueAt(st.Values, i), st.Span()); node != nil {
					out = append(out, node)
				}
			}
		case *syntax.LabelStmt:
			if st.Name != nil {
				out = append(out, &outlineNode{
					name: st.Name.Text,
					kind: protocol.SymbolKindKey,
					span: st.Span(),
					sel:  st.Name.Span(),
				})
			}
		case *syntax.DoStmt:
			out = append(out, collectOutline(st.Body, false)...)
		case *syntax.WhileStmt:
			out = append(out, collectOutline(st.Body, false)...)
		case *syntax.RepeatStmt:
			out = append(out, collectOutline(st.Body, false)...)
		case *syntax.IfStmt:
			for _, branch := range st.Branches {
				out = append(out, collectOutline(branch.Body, false)...)
			}
			out = append(out, collectOutline(st.Else, false)...)
		case *syntax.NumericForStmt:
			out = append(out, collectOutline(st.Body, false)...)
		case *syntax.GenericForStmt:
			out = append(out, collectOutline(st.Body, false)...)
		}
	}
	return out
}

func functionOutline(name string, sel source.Span, span source.Span, fn *syntax.FunctionExpr) *outlineNode {
	return &outlineNode{
		name:     name,
		kind:     protocol.SymbolKindFunction,
		span:     span,
		sel:      sel,
		children: collectOutline(fn.Body, false),
	}
}

func assignmentOutline(target syntax.Expr, value syntax.Expr, span source.Span) *outlineNode {
	switch t := target.(type) {
	case *syntax.NameExpr:
		kind := protocol.SymbolKindVariable
		if _, ok := value.(*syntax.FunctionExpr); ok {
			kind = protocol.SymbolKindFunction
		}
		return &outlineNode{name: t.Text, kind: kind, span: span, sel: t.Span()}
	case *syntax.FieldExpr:
		kind := protocol.SymbolKindField
		if _, ok := value.(*syntax.FunctionExpr); ok {
			kind = protocol.SymbolKindMethod
		}
		return &outlineNode{name: t.Name.Text, kind: kind, span: span, sel: t.Name.Span()}
	default:
		return nil
	}
}

func valueAt(values []syntax.Expr, i int) syntax.Expr {
	if i >= 0 && i < len(values) {
		return values[i]
	}
	return nil
}

func functionDetail(names []*syntax.NameExpr, method bool) string {
	if len(names) == 0 {
		return ""
	}
	detail := "function "
	if method {
		detail += "method "
	}
	for i, name := range names {
		if i > 0 {
			if method && i == len(names)-1 {
				detail += ":"
			} else {
				detail += "."
			}
		}
		detail += name.Text
	}
	return detail
}

func selectionOf(names []*syntax.NameExpr, fallback source.Span) source.Span {
	if len(names) == 0 {
		return fallback
	}
	return names[len(names)-1].Span()
}

func toDocumentSymbol(file *cart.File, node *outlineNode) protocol.DocumentSymbol {
	rangeVal := file.Index.Range(node.span.Start, node.span.End)
	selection := file.Index.Range(node.sel.Start, node.sel.End)
	if selection.Start.Line < rangeVal.Start.Line || selection.Start.Line > rangeVal.End.Line {
		selection = rangeVal
	}
	symbol := protocol.DocumentSymbol{
		Name:           node.name,
		Kind:           node.kind,
		Range:          rangeVal,
		SelectionRange: selection,
	}
	if node.detail != "" {
		detail := node.detail
		symbol.Detail = &detail
	}
	for _, child := range node.children {
		symbol.Children = append(symbol.Children, toDocumentSymbol(file, child))
	}
	return symbol
}

func sortDocumentSymbols(symbols []protocol.DocumentSymbol) {
	sort.SliceStable(symbols, func(i, j int) bool {
		if symbols[i].Range.Start.Line != symbols[j].Range.Start.Line {
			return symbols[i].Range.Start.Line < symbols[j].Range.Start.Line
		}
		return symbols[i].Range.Start.Character < symbols[j].Range.Start.Character
	})
}

func lineOf(file *cart.File, offset int) int {
	pos, err := file.Index.Position(offset)
	if err != nil {
		return 0
	}
	return int(pos.Line)
}

func fileHasSuffix(u uri.URI, suffix string) bool {
	return strings.HasSuffix(strings.ToLower(u.Path()), suffix)
}
