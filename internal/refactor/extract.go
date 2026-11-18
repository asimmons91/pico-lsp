package refactor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// ExtractFunction extracts a single expression or statement into a local
// function in the enclosing block. It refuses selections that would need
// cross-boundary control flow, writes to captured locals, or vararg/nested
// function semantics.
func ExtractFunction(ctx *Context, start, end int) *Action {
	if !ctx.valid() {
		return nil
	}
	text := ctx.currentText()
	start, end = trimSelection(text, start, end)
	node := ctx.exactNode(start, end)
	if node == nil {
		return nil
	}

	refused := false
	syntax.Walk(node, func(child syntax.Node) bool {
		switch child := child.(type) {
		case *syntax.FunctionExpr, *syntax.FunctionDeclStmt,
			*syntax.LabelStmt, *syntax.BreakStmt, *syntax.GotoStmt,
			*syntax.ReturnStmt, *syntax.VarargExpr:
			refused = true
			return false
		case *syntax.StringExpr:
			// Reindenting a multi-line long string would change its value.
			if strings.ContainsAny(spanText(text, child.Span()), "\r\n") {
				refused = true
				return false
			}
		}
		return true
	})
	if refused {
		return nil
	}

	var selectedStatement syntax.Stmt
	var selectedExpr syntax.Expr
	switch n := node.(type) {
	case syntax.Stmt:
		selectedStatement = n
	case syntax.Expr:
		selectedExpr = n
	default:
		return nil
	}
	if selectedStatement != nil {
		switch selectedStatement.(type) {
		case *syntax.LocalStmt, *syntax.LocalFunctionStmt, *syntax.ReturnStmt,
			*syntax.BreakStmt, *syntax.GotoStmt, *syntax.LabelStmt,
			*syntax.EmptyStmt:
			return nil
		}
	}
	if selectedExpr != nil && expandsMultipleValues(ctx, selectedExpr) {
		return nil
	}

	baseIndent := ctx.indentAt(start)
	bodyIndent := baseIndent + ctx.indentUnit()

	// Captured locals become parameters. Writing a capture from inside the new
	// function would change behavior, so such selections are refused.
	params, ok := extractCaptures(ctx, start, end)
	if !ok {
		return nil
	}

	anchor := ctx.enclosingStatement(start)
	if selectedStatement != nil {
		anchor = selectedStatement
	}
	if anchor == nil {
		return nil
	}
	for {
		parent := ctx.smallestContainingStmt(anchor.Span(), anchor)
		if parent == nil {
			break
		}
		singleLine := false
		switch parent := parent.(type) {
		case *syntax.IfStmt:
			singleLine = parent.SingleLine
		case *syntax.WhileStmt:
			singleLine = parent.SingleLine
		}
		if !singleLine {
			break
		}
		anchor = parent
	}
	block := ctx.parentBlock(anchor)
	if block == nil {
		return nil
	}
	insertOffset := anchor.Span().Start
	lineStart := ctx.lineStart(insertOffset)
	if strings.TrimSpace(text[lineStart:insertOffset]) != "" {
		// A multi-line function cannot be inserted in the middle of a line
		// without disturbing neighboring statements.
		return nil
	}
	insertOffset = lineStart
	indent := ctx.indentAt(insertOffset)

	name := extractedName(ctx)
	call := name + "(" + strings.Join(paramNames(params), ", ") + ")"

	body := spanText(text, source.Span{FileID: ctx.FileID, Start: start, End: end})
	body = reindent(body, baseIndent, bodyIndent)
	if selectedExpr != nil {
		body = indentFirstLine(body, bodyIndent, "return ")
	}

	var b strings.Builder
	b.WriteString(indent)
	b.WriteString("local function ")
	b.WriteString(name)
	b.WriteString("(")
	b.WriteString(strings.Join(paramNames(params), ", "))
	b.WriteString(")\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(indent)
	b.WriteString("end\n")

	if insertOffset == start {
		// The insertion and replacement meet at a line boundary. Return one
		// combined edit so clients never see touching ranges.
		return &Action{
			Title: "Extract selection to local function",
			Kind:  KindExtractFunction,
			Edits: []Edit{{
				FileID:  ctx.FileID,
				Start:   start,
				End:     end,
				NewText: b.String() + indent + call,
			}},
		}
	}
	return &Action{
		Title: "Extract selection to local function",
		Kind:  KindExtractFunction,
		Edits: []Edit{
			{FileID: ctx.FileID, Start: insertOffset, End: insertOffset, NewText: b.String()},
			{FileID: ctx.FileID, Start: start, End: end, NewText: call},
		},
	}
}

func indentFirstLine(text, indent, prefix string) string {
	if !strings.HasPrefix(text, indent) {
		return indent + prefix + strings.TrimLeft(text, " \t")
	}
	return indent + prefix + text[len(indent):]
}

func trimSelection(text string, start, end int) (int, int) {
	if start < 0 {
		start = 0
	}
	if end > len(text) {
		end = len(text)
	}
	for start < end && isSpace(text[start]) {
		start++
	}
	for end > start && isSpace(text[end-1]) {
		end--
	}
	return start, end
}

func isSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

// expandsMultipleValues reports whether replacing the expression with a
// single-return call could change a multi-value context. It is intentionally
// conservative: calls in ambiguous positions are refused.
func expandsMultipleValues(ctx *Context, expr syntax.Expr) bool {
	switch expr.(type) {
	case *syntax.CallExpr, *syntax.MethodCallExpr, *syntax.VarargExpr:
	default:
		return false
	}
	parent := ctx.parentOf(expr)
	switch p := parent.(type) {
	case *syntax.ParenExpr, *syntax.BinaryExpr, *syntax.UnaryExpr,
		*syntax.FieldExpr, *syntax.IndexExpr:
		return false
	case *syntax.LocalStmt:
		return len(p.Names) != 1 || len(p.Values) != 1
	case *syntax.AssignStmt:
		return len(p.Targets) != 1 || len(p.Values) != 1 || p.Op.Text != "="
	case *syntax.CallExpr:
		if len(p.Args) == 0 {
			return false
		}
		return p.Args[len(p.Args)-1].Span() == expr.Span()
	case *syntax.MethodCallExpr:
		if len(p.Args) == 0 {
			return false
		}
		return p.Args[len(p.Args)-1].Span() == expr.Span()
	default:
		return true
	}
}

func extractCaptures(ctx *Context, start, end int) ([]*analysis.Symbol, bool) {
	selection := source.Span{FileID: ctx.FileID, Start: start, End: end}
	var out []*analysis.Symbol
	seen := make(map[*analysis.Symbol]bool)
	for _, occ := range ctx.Result.Occurrences(ctx.FileID) {
		if occ.Symbol == nil || occ.Symbol.Builtin {
			continue
		}
		if occ.Span.Start < selection.Start || occ.Span.End > selection.End {
			continue
		}
		sym := occ.Symbol
		if sym.Owner != nil || sym.Kind == analysis.SymbolField || sym.Kind == analysis.SymbolGlobal {
			continue
		}
		if symbolDeclaredWithin(sym, selection) {
			continue
		}
		if occ.Write {
			return nil, false
		}
		if seen[sym] {
			continue
		}
		seen[sym] = true
		out = append(out, sym)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return firstOccurrence(ctx, out[i], selection) < firstOccurrence(ctx, out[j], selection)
	})
	return out, true
}

func symbolDeclaredWithin(sym *analysis.Symbol, span source.Span) bool {
	if sym == nil {
		return false
	}
	for _, decl := range sym.Decls {
		if decl.FileID == span.FileID && decl.Start >= span.Start && decl.End <= span.End {
			return true
		}
	}
	return sym.DeclSpan.FileID == span.FileID && sym.DeclSpan.Start >= span.Start && sym.DeclSpan.End <= span.End
}

func firstOccurrence(ctx *Context, sym *analysis.Symbol, span source.Span) int {
	best := int(^uint(0) >> 1)
	for _, occ := range ctx.Result.OccurrencesForSymbol(sym) {
		if occ.Span.FileID != span.FileID || occ.Span.Start < span.Start || occ.Span.End > span.End {
			continue
		}
		if occ.Span.Start < best {
			best = occ.Span.Start
		}
	}
	return best
}

func paramNames(params []*analysis.Symbol) []string {
	out := make([]string, 0, len(params))
	for _, param := range params {
		out = append(out, param.Name)
	}
	return out
}

func extractedName(ctx *Context) string {
	used := make(map[string]bool)
	for _, sym := range ctx.Result.Table.Symbols {
		if sym != nil {
			used[sym.Canonical] = true
		}
	}
	for _, file := range ctx.Snap.Cart.Files {
		for _, occ := range ctx.Result.Occurrences(file.ID) {
			if occ.Text != "" {
				used[analysis.CanonicalName(occ.Text)] = true
			}
		}
	}
	base := "extracted"
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s%d", base, i)
		if !used[candidate] {
			return candidate
		}
	}
}
