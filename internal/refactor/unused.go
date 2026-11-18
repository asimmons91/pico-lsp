package refactor

import (
	"fmt"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// RemoveUnused removes the declaration of an unused local or local function.
// The action carries a Confirmation so the server executes it through a client
// command instead of applying the edit implicitly.
func RemoveUnused(ctx *Context, offset int) *Action {
	if !ctx.valid() {
		return nil
	}
	occ := ctx.Result.OccurrenceAt(ctx.FileID, offset)
	if occ == nil || occ.Symbol == nil {
		return nil
	}
	sym := occ.Symbol
	if !ctx.Result.IsUnusedLocal(sym) {
		return nil
	}
	for _, candidate := range ctx.Result.OccurrencesForSymbol(sym) {
		if candidate.Decl {
			continue
		}
		// A write-only local would become a global write if the declaration
		// were removed, so removal is not behavior-preserving.
		return nil
	}
	stmt := ctx.enclosingStatement(sym.DeclSpan.Start)
	if stmt == nil {
		return nil
	}

	switch st := stmt.(type) {
	case *syntax.LocalStmt:
		if len(st.Names) != 1 {
			return nil
		}
		if len(st.Values) > 1 {
			return nil
		}
		for _, value := range st.Values {
			if !pureInitializer(value) {
				return nil
			}
		}
	case *syntax.LocalFunctionStmt:
		if st.Name != nil && analysis.CanonicalName(st.Name.Text) != sym.Canonical {
			return nil
		}
	default:
		return nil
	}

	// A dynamically resolved same-name use would be captured (or exposed) by
	// removing the declaration, so refuse rather than guessing.
	for _, candidate := range ctx.Result.Occurrences(ctx.FileID) {
		if candidate.Symbol != nil || analysis.CanonicalName(candidate.Text) != sym.Canonical {
			continue
		}
		if candidate.Span.Start > sym.DeclSpan.Start {
			return nil
		}
	}

	start, ok := ctx.statementLineStart(stmt)
	if !ok {
		return nil
	}
	if docStart := ctx.docBlockStart(stmt.Span().Start); docStart >= 0 && docStart < start {
		block := ctx.currentText()[docStart:start]
		if strings.Contains(block, "---@class") || strings.Contains(block, "---@alias") ||
			strings.Contains(block, "---@enum") {
			// Type declarations may be referenced elsewhere even when the
			// variable itself is unused.
			return nil
		}
		start = docStart
	}
	lineEnd := ctx.lineEnd(stmt.Span().End)
	if strings.TrimSpace(ctx.currentText()[stmt.Span().End:lineEnd]) != "" {
		return nil
	}
	end := lineEnd
	text := ctx.currentText()
	if end < len(text) && text[end] == '\r' {
		end++
	}
	if end < len(text) && text[end] == '\n' {
		end++
	}

	label := "local"
	if sym.Kind == analysis.SymbolFunction {
		label = "function"
	}
	return &Action{
		Title: fmt.Sprintf("Remove unused %s %q", label, sym.Name),
		Kind:  KindRemoveUnused,
		Edits: []Edit{{FileID: ctx.FileID, Start: start, End: end, NewText: ""}},
		Confirmation: &Confirmation{
			Span: sym.DeclSpan,
		},
	}
}

func pureInitializer(expr syntax.Expr) bool {
	switch e := expr.(type) {
	case nil:
		return true
	case *syntax.NilExpr, *syntax.BoolExpr, *syntax.NumberExpr, *syntax.StringExpr,
		*syntax.VarargExpr, *syntax.NameExpr, *syntax.FunctionExpr:
		return true
	case *syntax.UnaryExpr:
		return pureInitializer(e.X)
	case *syntax.BinaryExpr:
		return pureInitializer(e.Left) && pureInitializer(e.Right)
	case *syntax.ParenExpr:
		return pureInitializer(e.X)
	case *syntax.TableExpr:
		for _, field := range e.Fields {
			if field.Key != nil && !pureInitializer(field.Key) {
				return false
			}
			if !pureInitializer(field.Value) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
