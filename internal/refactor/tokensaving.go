package refactor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// Token-saving code-action kinds.
const (
	KindTokenSavingPrint    = "refactor.rewrite.token-saving.print"
	KindTokenSavingCompound = "refactor.rewrite.token-saving.compound"
	KindTokenSavingPeek     = "refactor.rewrite.token-saving.peek"
)

// maxTokenSavingCandidates bounds the action list returned for a large
// selection. Candidates are sorted deterministically before the limit is
// applied.
const maxTokenSavingCandidates = 100

var compoundOperators = map[string]string{
	"+":   "+=",
	"-":   "-=",
	"*":   "*=",
	"/":   "/=",
	"\\":  "\\=",
	"%":   "%=",
	"^":   "^=",
	"..":  "..=",
	"&":   "&=",
	"|":   "|=",
	"^^":  "^^=",
	"<<":  "<<=",
	">>":  ">>=",
	">>>": ">>>=",
	"<<>": "<<>=",
	">><": ">><=",
}

var peekOperators = map[string]string{
	"peek":  "@",
	"peek2": "%",
	"peek4": "$",
}

type tokenSavingCandidate struct {
	span    source.Span
	pattern int
	action  *Action
}

// TokenSavingActions returns token-saving candidates selected by start and end.
// A cursor (start == end) selects every candidate containing it. A non-empty
// selection selects only candidates wholly inside it. Each candidate produces
// its own action; edits are never merged.
func TokenSavingActions(ctx *Context, start, end int) []*Action {
	if !tokenSavingContext(ctx) {
		return nil
	}
	if start < 0 {
		start = 0
	}
	if end > len(ctx.currentText()) {
		end = len(ctx.currentText())
	}
	if start > end {
		start, end = end, start
	}

	var candidates []tokenSavingCandidate
	consider := func(span source.Span, pattern int, build func() *Action) {
		if !tokenCandidateSelected(span, start, end) {
			return
		}
		action := build()
		if action == nil {
			return
		}
		candidates = append(candidates, tokenSavingCandidate{span: span, pattern: pattern, action: action})
	}
	for _, node := range ctx.nodes() {
		switch n := node.(type) {
		case *syntax.CallStmt:
			call, ok := n.Call.(*syntax.CallExpr)
			if !ok {
				continue
			}
			consider(call.Span(), 0, func() *Action { return printAction(ctx, call) })
		case *syntax.AssignStmt:
			stmt := n
			consider(stmt.Span(), 1, func() *Action { return compoundAction(ctx, stmt) })
		case *syntax.CallExpr:
			call := n
			consider(call.Span(), 2, func() *Action { return peekAction(ctx, call) })
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.span.Start != right.span.Start {
			return left.span.Start < right.span.Start
		}
		if left.span.Len() != right.span.Len() {
			return left.span.Len() > right.span.Len()
		}
		if left.pattern != right.pattern {
			return left.pattern < right.pattern
		}
		return left.action.Title < right.action.Title
	})
	if len(candidates) > maxTokenSavingCandidates {
		candidates = candidates[:maxTokenSavingCandidates]
	}
	actions := make([]*Action, 0, len(candidates))
	for _, candidate := range candidates {
		actions = append(actions, candidate.action)
	}
	return actions
}

// TokenSavingPrint offers the PICO-8 `?` print shorthand for a complete
// `print(...)` statement.
func TokenSavingPrint(ctx *Context, offset int) *Action {
	return smallestTokenSavingAction(ctx, offset, func(ctx *Context, node syntax.Node) *Action {
		stmt, ok := node.(*syntax.CallStmt)
		if !ok {
			return nil
		}
		call, ok := stmt.Call.(*syntax.CallExpr)
		if !ok {
			return nil
		}
		return printAction(ctx, call)
	})
}

// TokenSavingCompound offers a compound assignment for `target = target op
// value`.
func TokenSavingCompound(ctx *Context, offset int) *Action {
	return smallestTokenSavingAction(ctx, offset, func(ctx *Context, node syntax.Node) *Action {
		stmt, ok := node.(*syntax.AssignStmt)
		if !ok {
			return nil
		}
		return compoundAction(ctx, stmt)
	})
}

// TokenSavingPeek offers `@`, `%`, or `$` for built-in peek calls.
func TokenSavingPeek(ctx *Context, offset int) *Action {
	return smallestTokenSavingAction(ctx, offset, func(ctx *Context, node syntax.Node) *Action {
		call, ok := node.(*syntax.CallExpr)
		if !ok {
			return nil
		}
		return peekAction(ctx, call)
	})
}

func smallestTokenSavingAction(ctx *Context, offset int, build func(*Context, syntax.Node) *Action) *Action {
	if !tokenSavingContext(ctx) {
		return nil
	}
	var best *Action
	bestLen := int(^uint(0) >> 1)
	for _, node := range ctx.nodes() {
		span := node.Span()
		if offset < span.Start || offset > span.End {
			continue
		}
		action := build(ctx, node)
		if action == nil {
			continue
		}
		if span.Len() < bestLen {
			best = action
			bestLen = span.Len()
		}
	}
	return best
}

func tokenSavingContext(ctx *Context) bool {
	return ctx != nil && ctx.valid() && ctx.Result != nil && !ctx.Result.IsMetaFile(ctx.FileID)
}

func tokenCandidateSelected(span source.Span, start, end int) bool {
	if start == end {
		return span.Start <= start && start <= span.End
	}
	return start <= span.Start && span.End <= end
}

func printAction(ctx *Context, call *syntax.CallExpr) *Action {
	if ctx == nil || call == nil {
		return nil
	}
	name, ok := call.Func.(*syntax.NameExpr)
	if !ok || analysis.CanonicalName(name.Text) != "print" {
		return nil
	}
	if !resolvedBuiltin(ctx, name, "print") {
		return nil
	}
	if len(call.Args) == 0 {
		// Zero-argument output and token accounting are not yet proven against
		// PICO-8, so the action stays unavailable.
		return nil
	}
	if !parenthesizedCall(ctx, call) {
		return nil
	}
	text := ctx.currentText()
	callText := spanText(text, call.Span())
	if callText == "" || strings.ContainsAny(callText, "\r\n") {
		return nil
	}
	if ctx.hasComment(call.Span().Start, call.Span().End) {
		return nil
	}
	if ctx.printPrecedesOnLine(call) {
		// An earlier `?` already consumes the rest of this line.
		return nil
	}
	// `?` consumes the rest of its physical line. Refuse anything after the
	// call, including a trailing comment, until PICO-8 parity is proven.
	lineEnd := ctx.lineEnd(call.Span().End)
	if strings.TrimSpace(text[call.Span().End:lineEnd]) != "" {
		return nil
	}
	if multiValueArg(call.Args[len(call.Args)-1]) {
		return nil
	}
	args := text[call.Open.End:call.Close.Start]
	trimmed := strings.TrimSpace(args)
	if trimmed == "" || strings.HasSuffix(trimmed, ",") {
		return nil
	}
	newText := "?" + args
	delta, ok := tokenDelta(ctx, call.Span(), newText)
	if !ok || delta <= 0 {
		return nil
	}
	return &Action{
		Title:        fmt.Sprintf("Use ? print shorthand (%s)", savingsText(delta)),
		Kind:         KindTokenSavingPrint,
		Edits:        []Edit{{FileID: ctx.FileID, Start: call.Span().Start, End: call.Span().End, NewText: newText}},
		TokenSavings: delta,
	}
}

func compoundAction(ctx *Context, stmt *syntax.AssignStmt) *Action {
	if ctx == nil || stmt == nil {
		return nil
	}
	if len(stmt.Targets) != 1 || len(stmt.Values) != 1 {
		return nil
	}
	if !stmt.Op.Is("=") {
		return nil
	}
	target := stmt.Targets[0]
	binary, ok := stmt.Values[0].(*syntax.BinaryExpr)
	if !ok {
		return nil
	}
	compound, ok := compoundOperators[binary.Op.Text]
	if !ok {
		return nil
	}
	if !staticLValue(target) || !sameLValue(ctx, target, binary.Left) {
		return nil
	}
	text := ctx.currentText()
	assignText := spanText(text, stmt.Span())
	if assignText == "" || strings.ContainsAny(assignText, "\r\n") {
		return nil
	}
	if ctx.hasComment(stmt.Span().Start, stmt.Span().End) {
		return nil
	}
	targetText := spanText(text, target.Span())
	rightText := spanText(text, binary.Right.Span())
	if targetText == "" || rightText == "" {
		return nil
	}
	newText := targetText + " " + compound + " " + rightText
	delta, ok := tokenDelta(ctx, stmt.Span(), newText)
	if !ok || delta <= 0 {
		return nil
	}
	return &Action{
		Title:        fmt.Sprintf("Use compound assignment %s (%s)", compound, savingsText(delta)),
		Kind:         KindTokenSavingCompound,
		Edits:        []Edit{{FileID: ctx.FileID, Start: stmt.Span().Start, End: stmt.Span().End, NewText: newText}},
		TokenSavings: delta,
	}
}

func peekAction(ctx *Context, call *syntax.CallExpr) *Action {
	if ctx == nil || call == nil {
		return nil
	}
	name, ok := call.Func.(*syntax.NameExpr)
	if !ok {
		return nil
	}
	canonical := analysis.CanonicalName(name.Text)
	op, ok := peekOperators[canonical]
	if !ok || !resolvedBuiltin(ctx, name, canonical) {
		return nil
	}
	if len(call.Args) != 1 {
		return nil
	}
	if !parenthesizedCall(ctx, call) {
		return nil
	}
	if !peekContextSafe(ctx, call) {
		return nil
	}
	text := ctx.currentText()
	callText := spanText(text, call.Span())
	if callText == "" || strings.ContainsAny(callText, "\r\n") {
		return nil
	}
	if ctx.hasComment(call.Span().Start, call.Span().End) {
		return nil
	}
	arg := call.Args[0]
	argText := spanText(text, arg.Span())
	if argText == "" {
		return nil
	}
	if b, ok := arg.(*syntax.BinaryExpr); ok {
		if prec, ok := syntax.BinaryPrecedence(b.Op); ok && prec < syntax.UnaryPrecedence {
			argText = "(" + argText + ")"
		}
	}
	newText := op + argText
	delta, ok := tokenDelta(ctx, call.Span(), newText)
	if !ok || delta <= 0 {
		return nil
	}
	return &Action{
		Title:        fmt.Sprintf("Use %s peek shorthand (%s)", op, savingsText(delta)),
		Kind:         KindTokenSavingPeek,
		Edits:        []Edit{{FileID: ctx.FileID, Start: call.Span().Start, End: call.Span().End, NewText: newText}},
		TokenSavings: delta,
	}
}

func (c *Context) printPrecedesOnLine(call *syntax.CallExpr) bool {
	if c == nil || call == nil {
		return false
	}
	if !c.printSpansBuilt {
		c.printSpansBuilt = true
		for _, node := range c.nodes() {
			print, ok := node.(*syntax.PrintStmt)
			if !ok {
				continue
			}
			c.printSpans = append(c.printSpans, print.Span())
		}
		sort.Slice(c.printSpans, func(i, j int) bool {
			return c.printSpans[i].Start < c.printSpans[j].Start
		})
	}
	lineStart := c.lineStart(call.Span().Start)
	index := sort.Search(len(c.printSpans), func(i int) bool {
		return c.printSpans[i].Start >= lineStart
	})
	for ; index < len(c.printSpans) && c.printSpans[index].Start < call.Span().Start; index++ {
		if c.printSpans[index].End <= call.Span().Start {
			return true
		}
	}
	return false
}

// peekContextSafe reports whether replacing the call with a prefix operator
// preserves the surrounding expression. A unary peek binds looser than `^` and
// tighter than suffixes, so a call used as a suffix receiver, call target, or
// the left operand of `^` would regroup.
func peekContextSafe(ctx *Context, call *syntax.CallExpr) bool {
	switch parent := ctx.parentOf(call).(type) {
	case *syntax.CallStmt:
		// `@x` is not a valid expression statement.
		return parent.Call != call
	case *syntax.FieldExpr:
		return parent.Base != call
	case *syntax.IndexExpr:
		return parent.Base != call
	case *syntax.CallExpr:
		return parent.Func != call
	case *syntax.MethodCallExpr:
		return parent.Receiver != call
	case *syntax.BinaryExpr:
		return parent.Left != call || !parent.Op.Is("^")
	default:
		return true
	}
}

// parenthesizedCall reports whether the call used `(args)`. Calls written as
// `f "x"` or `f {table}` also set Open/Close, but those spans cover the
// argument itself and cannot be used as argument delimiters.
func parenthesizedCall(ctx *Context, call *syntax.CallExpr) bool {
	if ctx == nil || call == nil || call.Open.IsEmpty() || call.Close.IsEmpty() {
		return false
	}
	text := ctx.currentText()
	return spanText(text, call.Open) == "(" && spanText(text, call.Close) == ")"
}

func multiValueArg(expr syntax.Expr) bool {
	switch expr.(type) {
	case *syntax.CallExpr, *syntax.MethodCallExpr, *syntax.VarargExpr:
		return true
	default:
		return false
	}
}

func staticLValue(expr syntax.Expr) bool {
	switch e := expr.(type) {
	case *syntax.NameExpr:
		return e.Text != ""
	case *syntax.FieldExpr:
		return e.Name != nil && staticLValue(e.Base)
	default:
		return false
	}
}

func sameLValue(ctx *Context, left, right syntax.Expr) bool {
	switch target := left.(type) {
	case *syntax.NameExpr:
		other, ok := right.(*syntax.NameExpr)
		if !ok || analysis.CanonicalName(target.Text) != analysis.CanonicalName(other.Text) {
			return false
		}
		return sameUserSymbol(symbolAt(ctx, target.Span().Start), symbolAt(ctx, other.Span().Start))
	case *syntax.FieldExpr:
		other, ok := right.(*syntax.FieldExpr)
		if !ok || target.Name == nil || other.Name == nil {
			return false
		}
		if analysis.CanonicalName(target.Name.Text) != analysis.CanonicalName(other.Name.Text) {
			return false
		}
		if !sameLValue(ctx, target.Base, other.Base) {
			return false
		}
		leftField := symbolAt(ctx, target.Name.Span().Start)
		rightField := symbolAt(ctx, other.Name.Span().Start)
		return sameFieldSymbol(leftField, rightField)
	default:
		return false
	}
}

func sameUserSymbol(left, right *analysis.Symbol) bool {
	return left != nil && left == right && !left.Builtin
}

func sameFieldSymbol(left, right *analysis.Symbol) bool {
	return left != nil && left == right && left.Owner != nil && !left.Builtin
}

func symbolAt(ctx *Context, offset int) *analysis.Symbol {
	if ctx == nil || ctx.Result == nil {
		return nil
	}
	occ := ctx.Result.OccurrenceAt(ctx.FileID, offset)
	if occ == nil {
		return nil
	}
	return occ.Symbol
}

func resolvedBuiltin(ctx *Context, name *syntax.NameExpr, canonical string) bool {
	if name == nil || analysis.CanonicalName(name.Text) != canonical {
		return false
	}
	sym := symbolAt(ctx, name.Span().Start)
	return sym != nil && sym.Builtin && sym.Kind == analysis.SymbolBuiltin && sym.Canonical == canonical
}

func savingsText(tokens int) string {
	if tokens == 1 {
		return "saves 1 token"
	}
	return fmt.Sprintf("saves %d tokens", tokens)
}

// tokenDelta returns the exact token-count change for replacing oldSpan with
// newText. Both sides are tokenized with the cart's PICO-8 rules; comments and
// annotations therefore never contribute.
func tokenDelta(ctx *Context, oldSpan source.Span, newText string) (int, bool) {
	oldText := spanText(ctx.currentText(), oldSpan)
	if oldText == "" || newText == "" {
		return 0, false
	}
	oldCount, ok := fragmentTokenCount(oldText)
	if !ok {
		return 0, false
	}
	newCount, ok := fragmentTokenCount(newText)
	if !ok {
		return 0, false
	}
	return oldCount - newCount, true
}

func fragmentTokenCount(text string) (int, bool) {
	lexed := syntax.Lex([]byte(text), func(start, end int) source.Span {
		return source.Span{Start: start, End: end}
	})
	if len(lexed.Errors) > 0 {
		return 0, false
	}
	for _, tok := range lexed.Tokens {
		if tok.Kind == syntax.TokenInvalid {
			return 0, false
		}
	}
	return cart.CountTokens(lexed.Tokens), true
}
