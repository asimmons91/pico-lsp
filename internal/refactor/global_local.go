package refactor

import (
	"fmt"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

var lifecycleGlobals = map[string]bool{
	"_init": true, "_update": true, "_update60": true, "_draw": true,
}

// GlobalToLocal converts the global declaration at offset to a local when every
// read and write stays visible and no forward function reference depends on
// global scope.
func GlobalToLocal(ctx *Context, offset int) *Action {
	if !ctx.valid() {
		return nil
	}
	occ := ctx.Result.OccurrenceAt(ctx.FileID, offset)
	if occ == nil || occ.Symbol == nil {
		return nil
	}
	sym := occ.Symbol
	if sym.Builtin || sym.Owner != nil || lifecycleGlobals[sym.Canonical] {
		return nil
	}
	if sym.Kind != analysis.SymbolGlobal && sym.Kind != analysis.SymbolFunction {
		return nil
	}
	if ctx.Result.Table.Globals[sym.Canonical] != sym {
		return nil
	}

	decl, ok := ctx.globalDeclaration(sym)
	if !ok {
		return nil
	}
	stmt := ctx.enclosingStatement(decl.Start)
	if stmt == nil {
		return nil
	}
	if block := ctx.parentBlock(stmt); block == nil || block != ctx.Snap.Chunk.Block {
		// Moving a declaration inside a nested block would drop the local at
		// the end of that block for any uses outside it.
		return nil
	}
	if _, ok := ctx.statementLineStart(stmt); !ok {
		return nil
	}
	insertOffset := stmt.Span().Start

	declEnd := decl.End
	var selfFunc *syntax.FunctionExpr
	switch st := stmt.(type) {
	case *syntax.FunctionDeclStmt:
		if len(st.Names) != 1 || st.Method || analysis.CanonicalName(st.Names[0].Text) != sym.Canonical {
			return nil
		}
		selfFunc = st.Func
	case *syntax.AssignStmt:
		name, ok := st.Targets[0].(*syntax.NameExpr)
		if !ok || len(st.Targets) != 1 || st.Op.Text != "=" || analysis.CanonicalName(name.Text) != sym.Canonical {
			return nil
		}
		declEnd = st.Span().End
		// `local x = function() return x end` does not make the local visible
		// inside its own initializer. Refuse rather than changing recursion.
		if len(st.Values) == 1 {
			if _, isFunc := st.Values[0].(*syntax.FunctionExpr); isFunc {
				for _, candidate := range ctx.Result.Occurrences(ctx.FileID) {
					if candidate.Symbol == sym && candidate.Span != decl {
						return nil
					}
				}
			}
		}
	default:
		return nil
	}

	for _, candidate := range ctx.Result.OccurrencesForSymbol(sym) {
		if candidate.Span == decl {
			continue
		}
		if candidate.Span.FileID != ctx.FileID {
			// A local in one included file is not visible in another.
			return nil
		}
		if candidate.Span.Start < declEnd {
			return nil
		}
		if fn := ctx.enclosingFunction(candidate.Span.Start); fn != nil {
			if selfFunc != nil && fn == selfFunc && contains(fn.Span(), candidate.Span) {
				continue
			}
			if fn.Span().Start < declEnd {
				// A function defined before the declaration cannot close over a
				// local that does not exist yet.
				return nil
			}
		}
	}

	// Unresolved same-name occurrences would be captured by the new local (or
	// indicate that not every read resolves to this global).
	for _, file := range ctx.Snap.Cart.Files {
		for _, candidate := range ctx.Result.Occurrences(file.ID) {
			if candidate.Symbol != nil || analysis.CanonicalName(candidate.Text) != sym.Canonical {
				continue
			}
			return nil
		}
	}

	return &Action{
		Title: fmt.Sprintf("Convert global %q to local", sym.Name),
		Kind:  KindGlobalToLocal,
		Edits: []Edit{{
			FileID:  ctx.FileID,
			Start:   insertOffset,
			End:     insertOffset,
			NewText: "local ",
		}},
	}
}

func (c *Context) globalDeclaration(sym *analysis.Symbol) (source.Span, bool) {
	var best source.Span
	for _, span := range sym.Decls {
		if span.FileID != c.FileID {
			// Declarations in another file cannot be made local here.
			return source.Span{}, false
		}
		if best.IsEmpty() || span.Start < best.Start {
			best = span
		}
	}
	if best.IsEmpty() {
		best = sym.DeclSpan
	}
	if best.FileID != c.FileID || best.IsEmpty() {
		return source.Span{}, false
	}
	return best, true
}

// statementLineStart returns the start of the line when only indentation
// precedes stmt, so a prefix edit cannot disturb neighboring statements.
func (c *Context) statementLineStart(stmt syntax.Stmt) (int, bool) {
	start := stmt.Span().Start
	lineStart := c.lineStart(start)
	if strings.TrimSpace(c.currentText()[lineStart:start]) != "" {
		return 0, false
	}
	return lineStart, true
}

func (c *Context) enclosingFunction(offset int) *syntax.FunctionExpr {
	var best *syntax.FunctionExpr
	bestLen := int(^uint(0) >> 1)
	for _, node := range c.nodes() {
		fn, ok := node.(*syntax.FunctionExpr)
		if !ok || fn == nil {
			continue
		}
		span := fn.Span()
		if offset < span.Start || offset > span.End {
			continue
		}
		if length := span.Len(); length < bestLen {
			best = fn
			bestLen = length
		}
	}
	return best
}
