package refactor

import (
	"fmt"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/syntax"
)

const maxShorthandLine = 120

// ConvertShorthand converts standard line-bounded statements to PICO-8
// shorthand and shorthand statements to standard `if ... then ... end` form.
// Only single-statement bodies without comments are converted.
func ConvertShorthand(ctx *Context, offset int) *Action {
	if !ctx.valid() {
		return nil
	}
	var target syntax.Stmt
	targetLen := int(^uint(0) >> 1)
	for _, node := range ctx.nodes() {
		stmt, ok := node.(syntax.Stmt)
		if !ok {
			continue
		}
		switch stmt.(type) {
		case *syntax.IfStmt, *syntax.WhileStmt:
		default:
			continue
		}
		span := stmt.Span()
		if offset < span.Start || offset > span.End {
			continue
		}
		if length := span.Len(); length < targetLen {
			target = stmt
			targetLen = length
		}
	}
	if target == nil {
		return nil
	}
	switch st := target.(type) {
	case *syntax.IfStmt:
		if st.SingleLine {
			return expandShorthandIf(ctx, st)
		}
		return collapseIf(ctx, st)
	case *syntax.WhileStmt:
		if st.SingleLine {
			return expandShorthandWhile(ctx, st)
		}
		return collapseWhile(ctx, st)
	default:
		return nil
	}
}

func collapseIf(ctx *Context, st *syntax.IfStmt) *Action {
	if st == nil || len(st.Branches) != 1 {
		return nil
	}
	branch := st.Branches[0]
	if branch == nil || branch.Body == nil || len(branch.Body.Stmts) != 1 {
		return nil
	}
	if ctx.hasComment(st.Span().Start, st.Span().End) {
		return nil
	}
	cond := shorthandCond(ctx, branch.Cond)
	if cond == "" {
		return nil
	}
	body := spanText(ctx.currentText(), branch.Body.Stmts[0].Span())
	if body == "" || strings.ContainsAny(body, "\r\n") {
		return nil
	}
	line := fmt.Sprintf("if (%s) %s", cond, strings.TrimSpace(body))
	if st.ElseSpan.Start != 0 || st.Else != nil {
		if st.Else == nil || len(st.Else.Stmts) != 1 {
			return nil
		}
		elseBody := spanText(ctx.currentText(), st.Else.Stmts[0].Span())
		if elseBody == "" || strings.ContainsAny(elseBody, "\r\n") {
			return nil
		}
		line += " else " + strings.TrimSpace(elseBody)
	}
	if len(ctx.indentAt(st.Span().Start))+len(line) > maxShorthandLine {
		return nil
	}
	return &Action{
		Title: "Convert to PICO-8 shorthand if",
		Kind:  KindShorthand,
		Edits: []Edit{{FileID: ctx.FileID, Start: st.Span().Start, End: st.Span().End, NewText: line}},
	}
}

func collapseWhile(ctx *Context, st *syntax.WhileStmt) *Action {
	if st == nil || st.Body == nil || len(st.Body.Stmts) != 1 {
		return nil
	}
	if ctx.hasComment(st.Span().Start, st.Span().End) {
		return nil
	}
	cond := shorthandCond(ctx, st.Cond)
	if cond == "" {
		return nil
	}
	body := spanText(ctx.currentText(), st.Body.Stmts[0].Span())
	if body == "" || strings.ContainsAny(body, "\r\n") {
		return nil
	}
	line := fmt.Sprintf("while (%s) %s", cond, strings.TrimSpace(body))
	if len(ctx.indentAt(st.Span().Start))+len(line) > maxShorthandLine {
		return nil
	}
	return &Action{
		Title: "Convert to PICO-8 shorthand while",
		Kind:  KindShorthand,
		Edits: []Edit{{FileID: ctx.FileID, Start: st.Span().Start, End: st.Span().End, NewText: line}},
	}
}

func shorthandCond(ctx *Context, cond syntax.Expr) string {
	if cond == nil {
		return ""
	}
	text := spanText(ctx.currentText(), cond.Span())
	if strings.ContainsAny(text, "\r\n") {
		return ""
	}
	if _, ok := cond.(*syntax.ParenExpr); ok {
		return text
	}
	return text
}

func expandShorthandIf(ctx *Context, st *syntax.IfStmt) *Action {
	if st == nil || len(st.Branches) != 1 {
		return nil
	}
	if ctx.shorthandAncestor(st) {
		return nil
	}
	if ctx.hasCommentOnLines(st.Span().Start, st.Span().End) {
		return nil
	}
	branch := st.Branches[0]
	if branch == nil || branch.Body == nil || len(branch.Body.Stmts) == 0 {
		return nil
	}
	cond := unwrappedCond(ctx, branch.Cond)
	if cond == "" || strings.ContainsAny(cond, "\r\n") {
		return nil
	}
	indent := ctx.indentAt(st.Span().Start)
	unit := ctx.indentUnit()
	lines := []string{"if " + cond + " then"}
	lines = append(lines, bodyLines(ctx, branch.Body, unit)...)
	if st.Else != nil || st.ElseSpan.Start != 0 {
		if st.Else == nil || len(st.Else.Stmts) == 0 {
			return nil
		}
		lines = append(lines, "else")
		lines = append(lines, bodyLines(ctx, st.Else, unit)...)
	}
	lines = append(lines, "end")
	if !linesFit(indent, lines) {
		return nil
	}
	return &Action{
		Title: "Convert to standard if statement",
		Kind:  KindShorthand,
		Edits: []Edit{{FileID: ctx.FileID, Start: st.Span().Start, End: st.Span().End, NewText: strings.Join(lines, "\n"+indent)}},
	}
}

func expandShorthandWhile(ctx *Context, st *syntax.WhileStmt) *Action {
	if st == nil || st.Body == nil || len(st.Body.Stmts) == 0 {
		return nil
	}
	if ctx.shorthandAncestor(st) {
		return nil
	}
	if ctx.hasCommentOnLines(st.Span().Start, st.Span().End) {
		return nil
	}
	cond := unwrappedCond(ctx, st.Cond)
	if cond == "" || strings.ContainsAny(cond, "\r\n") {
		return nil
	}
	indent := ctx.indentAt(st.Span().Start)
	unit := ctx.indentUnit()
	lines := []string{"while " + cond + " do"}
	lines = append(lines, bodyLines(ctx, st.Body, unit)...)
	lines = append(lines, "end")
	if !linesFit(indent, lines) {
		return nil
	}
	return &Action{
		Title: "Convert to standard while statement",
		Kind:  KindShorthand,
		Edits: []Edit{{FileID: ctx.FileID, Start: st.Span().Start, End: st.Span().End, NewText: strings.Join(lines, "\n"+indent)}},
	}
}

func unwrappedCond(ctx *Context, cond syntax.Expr) string {
	if cond == nil {
		return ""
	}
	if paren, ok := cond.(*syntax.ParenExpr); ok && paren.X != nil {
		return spanText(ctx.currentText(), paren.X.Span())
	}
	return spanText(ctx.currentText(), cond.Span())
}

func bodyLines(ctx *Context, block *syntax.Block, indent string) []string {
	if block == nil {
		return nil
	}
	text := ctx.currentText()
	var out []string
	for _, stmt := range block.Stmts {
		line := spanText(text, stmt.Span())
		if line == "" {
			continue
		}
		out = append(out, reindent(line, ctx.indentAt(stmt.Span().Start), indent))
	}
	return out
}

func linesFit(indent string, lines []string) bool {
	for _, line := range lines {
		if len(indent)+len(line) > maxShorthandLine {
			return false
		}
	}
	return true
}

func (c *Context) shorthandAncestor(stmt syntax.Stmt) bool {
	span := stmt.Span()
	for _, node := range c.nodes() {
		switch node := node.(type) {
		case *syntax.IfStmt:
			if node != stmt && node.SingleLine && contains(node.Span(), span) {
				return true
			}
		case *syntax.WhileStmt:
			if node != stmt && node.SingleLine && contains(node.Span(), span) {
				return true
			}
		}
	}
	return false
}
