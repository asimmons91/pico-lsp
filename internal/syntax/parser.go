package syntax

import (
	"fmt"

	"github.com/asimmons91/pico-lsp/internal/source"
)

const maxParseErrors = 100

// Parse builds an error-recovering AST from a token stream. The stream must
// end with a TokenEOF token.
func Parse(tokens []Token) (*Chunk, []Error) {
	p := &parser{tokens: tokens}
	block := p.parseBlock(nil)
	return &Chunk{Block: block}, p.errors
}

type parser struct {
	tokens  []Token
	pos     int
	errors  []Error
	tooMany bool
}

func (p *parser) current() Token {
	if p.pos >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.pos]
}

func (p *parser) peek(n int) Token {
	i := p.pos + n
	if i < 0 || i >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[i]
}

func (p *parser) eof() bool { return p.current().Kind == TokenEOF }

func (p *parser) advance() Token {
	tok := p.current()
	if p.pos < len(p.tokens)-1 {
		p.pos++
	}
	return tok
}

func (p *parser) check(text string) bool { return p.current().Is(text) }

func (p *parser) match(text string) (Token, bool) {
	if p.check(text) {
		return p.advance(), true
	}
	return Token{}, false
}

func (p *parser) expect(text string) Token {
	if p.check(text) {
		return p.advance()
	}
	tok := p.current()
	p.errorAt(tok, fmt.Sprintf("expected %q, found %s", text, describeToken(tok)))
	return Token{Span: tok.Span, Line: tok.Line}
}

func (p *parser) parseName() *NameExpr {
	tok := p.current()
	if tok.Kind != TokenName {
		p.errorAt(tok, fmt.Sprintf("expected name, found %s", describeToken(tok)))
		return nil
	}
	p.advance()
	return &NameExpr{NodeSpan: NodeSpan{tok.Span}, Text: tok.Text}
}

func (p *parser) errorAt(tok Token, message string) {
	p.errorSpan(tok.Span, message)
}

func (p *parser) errorSpan(span source.Span, message string) {
	if p.tooMany {
		return
	}
	if len(p.errors) >= maxParseErrors-1 {
		p.tooMany = true
		p.errors = append(p.errors, Error{Span: span, Message: "too many syntax errors; stopped reporting"})
		return
	}
	p.errors = append(p.errors, Error{Span: span, Message: message})
}

func describeToken(tok Token) string {
	if tok.Kind == TokenEOF {
		return "end of input"
	}
	if tok.Text == "" {
		return tok.Kind.String()
	}
	return fmt.Sprintf("%q", tok.Text)
}

func (p *parser) parseBlock(stop func(Token) bool) *Block {
	start := p.current().Span
	block := &Block{}
	for !p.eof() && (stop == nil || !stop(p.current())) {
		before := p.pos
		stmt := p.parseStatement()
		if stmt != nil {
			block.Stmts = append(block.Stmts, stmt)
		}
		if p.pos == before {
			p.errorAt(p.current(), "unexpected token")
			p.advance()
		}
	}
	block.SpanVal = start
	if len(block.Stmts) > 0 {
		block.SpanVal = block.SpanVal.Merge(block.Stmts[len(block.Stmts)-1].Span())
	}
	return block
}

func (p *parser) parseStatement() Stmt {
	tok := p.current()
	if tok.Kind == TokenInclude {
		p.errorAt(tok, "unexpected #include directive; nested includes are not supported")
		p.advance()
		return nil
	}
	if tok.Kind == TokenInvalid {
		p.errorAt(tok, "invalid token")
		p.advance()
		return nil
	}
	if tok.Kind == TokenTabMarker {
		p.advance()
		return nil
	}
	if tok.Kind == TokenPunct {
		switch tok.Text {
		case ";":
			p.advance()
			return &EmptyStmt{NodeSpan: NodeSpan{tok.Span}, Semicolon: tok.Span}
		case "?":
			return p.parsePrint()
		case "::":
			return p.parseLabel()
		}
	}
	if tok.Kind == TokenKeyword {
		switch tok.Value {
		case "local":
			return p.parseLocal()
		case "function":
			return p.parseFunctionDecl()
		case "if":
			return p.parseIf(false)
		case "while":
			return p.parseWhile(false)
		case "repeat":
			return p.parseRepeat()
		case "for":
			return p.parseFor()
		case "do":
			return p.parseDo()
		case "return":
			return p.parseReturn()
		case "break":
			p.advance()
			return &BreakStmt{NodeSpan: NodeSpan{tok.Span}, Break: tok.Span}
		case "goto":
			return p.parseGoto()
		}
	}
	return p.parseExprStatement()
}

func (p *parser) parseExprStatement() Stmt {
	start := p.current()
	targets := []Expr{p.parseUnary()}
	if targets[0] == nil {
		p.recover()
		return nil
	}

	for p.check(",") {
		p.advance()
		target := p.parseUnary()
		if target == nil {
			break
		}
		targets = append(targets, target)
	}

	op := p.current()
	if op.Kind == TokenPunct && (op.Text == "=" || isCompoundAssign(op.Text)) {
		p.advance()
		values := p.parseExprList()
		for _, target := range targets {
			if !isAssignable(target) {
				if unary, ok := target.(*UnaryExpr); ok && isPeekOp(unary.Op.Text) {
					p.errorSpan(target.Span(), "peek operators are read-only")
				} else {
					p.errorSpan(target.Span(), "cannot assign to this expression")
				}
			}
		}
		if len(targets) > 1 && op.Text != "=" {
			p.errorAt(op, "compound assignment requires a single target")
		}
		if len(values) == 0 {
			p.errorAt(op, "expected value after assignment")
		}
		return &AssignStmt{
			NodeSpan: NodeSpan{spanFrom(start, lastSpan(targets, values, op))},
			Targets:  targets,
			Op:       op,
			Values:   values,
		}
	}

	if len(targets) > 1 {
		p.errorAt(op, "expected assignment after expression list")
	}
	if _, ok := targets[0].(*CallExpr); ok {
		return &CallStmt{NodeSpan: NodeSpan{targets[0].Span()}, Call: targets[0]}
	}
	if _, ok := targets[0].(*MethodCallExpr); ok {
		return &CallStmt{NodeSpan: NodeSpan{targets[0].Span()}, Call: targets[0]}
	}

	p.errorSpan(targets[0].Span(), "expected assignment or function call")
	return nil
}

func lastSpan(targets []Expr, values []Expr, op Token) source.Span {
	end := op.Span
	if len(values) > 0 {
		end = values[len(values)-1].Span()
	}
	if len(targets) > 0 {
		end = end.Merge(targets[len(targets)-1].Span())
	}
	return end
}

func spanFrom(start Token, end source.Span) source.Span {
	if start.Span.FileID != end.FileID {
		return start.Span
	}
	return start.Span.Merge(end)
}

func isAssignable(expr Expr) bool {
	switch expr.(type) {
	case *NameExpr, *FieldExpr, *IndexExpr:
		return true
	default:
		return false
	}
}

func isPeekOp(op string) bool { return op == "@" || op == "%" || op == "$" }

func isCompoundAssign(op string) bool {
	switch op {
	case "+=", "-=", "*=", "/=", "\\=", "%=", "^=", "..=", "&=", "|=", "^^=", "<<=", ">>=", ">>>=", "<<>=", ">><=":
		return true
	default:
		return false
	}
}

func (p *parser) parseLocal() Stmt {
	local := p.advance()
	if p.check("function") {
		fn := p.advance()
		name := p.parseName()
		body := p.parseFunctionBody(fn)
		if name == nil || body == nil {
			return nil
		}
		return &LocalFunctionStmt{
			NodeSpan: NodeSpan{spanFrom(local, body.Span())},
			Local:    local.Span,
			Function: fn.Span,
			Name:     name,
			Func:     body,
		}
	}

	var names []*NameExpr
	for {
		name := p.parseName()
		if name == nil {
			break
		}
		names = append(names, name)
		if !p.check(",") {
			break
		}
		p.advance()
	}
	if len(names) == 0 {
		p.errorAt(p.current(), "expected local name")
		return nil
	}

	var values []Expr
	if p.check("=") {
		eq := p.advance()
		values = p.parseExprList()
		if len(values) == 0 {
			p.errorAt(eq, "expected value after local declaration")
		}
	}
	end := names[len(names)-1].Span()
	if len(values) > 0 {
		end = values[len(values)-1].Span()
	}
	return &LocalStmt{
		NodeSpan: NodeSpan{spanFrom(local, end)},
		Local:    local.Span,
		Names:    names,
		Values:   values,
	}
}

func (p *parser) parseFunctionDecl() Stmt {
	fn := p.advance()
	var names []*NameExpr
	name := p.parseName()
	if name == nil {
		return nil
	}
	names = append(names, name)
	for p.check(".") {
		p.advance()
		next := p.parseName()
		if next == nil {
			break
		}
		names = append(names, next)
	}
	method := false
	if p.check(":") {
		p.advance()
		method = true
		next := p.parseName()
		if next == nil {
			return nil
		}
		names = append(names, next)
	}

	body := p.parseFunctionBody(fn)
	if body == nil {
		return nil
	}
	return &FunctionDeclStmt{
		NodeSpan: NodeSpan{spanFrom(fn, body.Span())},
		Function: fn.Span,
		Names:    names,
		Method:   method,
		Func:     body,
	}
}

func (p *parser) parseFunctionBody(function Token) *FunctionExpr {
	if !p.check("(") {
		p.expect("(")
	}
	var params []*NameExpr
	vararg := false
	if p.check("(") {
		p.advance()
		for !p.eof() && !p.check(")") {
			if p.check("...") {
				p.advance()
				vararg = true
				if p.check(",") {
					p.advance()
				}
				break
			}
			name := p.parseName()
			if name == nil {
				p.advance()
				continue
			}
			params = append(params, name)
			if p.check(",") {
				p.advance()
				continue
			}
			break
		}
		p.expect(")")
	}

	body := p.parseBlock(func(tok Token) bool { return tok.Is("end") })
	end := p.expect("end")
	return &FunctionExpr{
		NodeSpan: NodeSpan{spanFrom(function, end.Span)},
		Params:   params,
		Vararg:   vararg,
		Body:     body,
		End:      end.Span,
	}
}

func (p *parser) parseIf(lineBound bool) Stmt {
	ifTok := p.advance()
	cond := p.parseExpr(0)
	if cond == nil {
		p.recover()
		return nil
	}

	if !p.check("then") {
		if _, ok := cond.(*ParenExpr); ok && !lineBound {
			return p.parseShorthandIf(ifTok, cond)
		}
		if lineBound {
			p.errorAt(p.current(), "expected a statement, found standard if on shorthand line")
		}
	}

	thenTok := p.expect("then")
	branch := &IfBranch{
		NodeSpan: NodeSpan{spanFrom(ifTok, cond.Span())},
		Keyword:  ifTok.Span,
		Cond:     cond,
		Then:     thenTok.Span,
		Body:     p.parseBlock(func(tok Token) bool { return tok.Is("elseif") || tok.Is("else") || tok.Is("end") }),
	}
	branches := []*IfBranch{branch}

	for p.check("elseif") {
		kw := p.advance()
		cond := p.parseExpr(0)
		if cond == nil {
			break
		}
		thenTok := p.expect("then")
		branch := &IfBranch{
			NodeSpan: NodeSpan{spanFrom(kw, cond.Span())},
			Keyword:  kw.Span,
			Cond:     cond,
			Then:     thenTok.Span,
			Body:     p.parseBlock(func(tok Token) bool { return tok.Is("elseif") || tok.Is("else") || tok.Is("end") }),
			ElseIf:   true,
		}
		branches = append(branches, branch)
	}

	var elseBlock *Block
	var elseSpan source.Span
	if p.check("else") {
		kw := p.advance()
		elseSpan = kw.Span
		elseBlock = p.parseBlock(func(tok Token) bool { return tok.Is("end") })
	}

	end := p.expect("end")
	return &IfStmt{
		NodeSpan: NodeSpan{spanFrom(ifTok, end.Span)},
		Branches: branches,
		Else:     elseBlock,
		ElseSpan: elseSpan,
		End:      end.Span,
	}
}

func (p *parser) parseShorthandIf(ifTok Token, cond Expr) Stmt {
	lineNo := ifTok.Line
	body := p.parseLineBody(lineNo)
	if len(body.Stmts) == 0 {
		p.errorAt(p.current(), "expected statement on the same line as shorthand if")
	}
	branch := &IfBranch{
		NodeSpan: NodeSpan{cond.Span()},
		Keyword:  ifTok.Span,
		Cond:     cond,
		Body:     body,
	}
	stmt := &IfStmt{
		NodeSpan:   NodeSpan{cond.Span().Merge(body.Span())},
		Branches:   []*IfBranch{branch},
		SingleLine: true,
	}
	if p.check("else") && p.current().Line == lineNo {
		kw := p.advance()
		stmt.ElseSpan = kw.Span
		stmt.Else = p.parseLineBody(lineNo)
		stmt.SpanVal = stmt.SpanVal.Merge(stmt.Else.Span())
	}
	return stmt
}

func (p *parser) parseWhile(lineBound bool) Stmt {
	whileTok := p.advance()
	cond := p.parseExpr(0)
	if cond == nil {
		p.recover()
		return nil
	}
	if !p.check("do") {
		if _, ok := cond.(*ParenExpr); ok && !lineBound {
			return p.parseShorthandWhile(whileTok, cond)
		}
		if lineBound {
			p.errorAt(p.current(), "expected a statement, found standard while on shorthand line")
		}
	}
	doTok := p.expect("do")
	body := p.parseBlock(func(tok Token) bool { return tok.Is("end") })
	end := p.expect("end")
	return &WhileStmt{
		NodeSpan: NodeSpan{spanFrom(whileTok, end.Span)},
		While:    whileTok.Span,
		Cond:     cond,
		Do:       doTok.Span,
		Body:     body,
		End:      end.Span,
	}
}

func (p *parser) parseShorthandWhile(whileTok Token, cond Expr) Stmt {
	lineNo := whileTok.Line
	body := p.parseLineBody(lineNo)
	if len(body.Stmts) == 0 {
		p.errorAt(p.current(), "expected statement on the same line as shorthand while")
	}
	return &WhileStmt{
		NodeSpan:   NodeSpan{cond.Span().Merge(body.Span())},
		While:      whileTok.Span,
		Cond:       cond,
		Body:       body,
		SingleLine: true,
	}
}

func (p *parser) parseLineBody(line int) *Block {
	start := p.current().Span
	block := &Block{NodeSpan: NodeSpan{start}}
	for !p.eof() && p.current().Line == line && !lineBodyStop(p.current()) {
		before := p.pos
		stmt := p.parseLineStatement()
		if stmt != nil {
			block.Stmts = append(block.Stmts, stmt)
		}
		if p.pos == before {
			p.advance()
			break
		}
	}
	if len(block.Stmts) > 0 {
		block.SpanVal = block.SpanVal.Merge(block.Stmts[len(block.Stmts)-1].Span())
	}
	return block
}

func (p *parser) parseLineStatement() Stmt {
	tok := p.current()
	if tok.Is("?") {
		return p.parsePrint()
	}
	if tok.Kind == TokenKeyword {
		switch tok.Value {
		case "local":
			return p.parseLocal()
		case "break":
			p.advance()
			return &BreakStmt{NodeSpan: NodeSpan{tok.Span}, Break: tok.Span}
		case "return":
			return p.parseReturn()
		case "goto":
			return p.parseGoto()
		case "if":
			return p.parseIf(true)
		case "while":
			return p.parseWhile(true)
		}
	}
	if tok.Is("::") {
		return p.parseLabel()
	}
	return p.parseExprStatement()
}

func lineBodyStop(tok Token) bool {
	if tok.Kind == TokenEOF {
		return true
	}
	if tok.Kind == TokenInclude {
		return true
	}
	if tok.Kind == TokenKeyword {
		switch tok.Value {
		case "else", "elseif", "end", "until":
			return true
		}
	}
	return false
}

func (p *parser) parseRepeat() Stmt {
	repeatTok := p.advance()
	body := p.parseBlock(func(tok Token) bool { return tok.Is("until") })
	untilTok := p.expect("until")
	cond := p.parseExpr(0)
	end := untilTok.Span
	if cond != nil {
		end = end.Merge(cond.Span())
	}
	return &RepeatStmt{
		NodeSpan: NodeSpan{spanFrom(repeatTok, end)},
		Repeat:   repeatTok.Span,
		Body:     body,
		Until:    untilTok.Span,
		Cond:     cond,
	}
}

func (p *parser) parseDo() Stmt {
	doTok := p.advance()
	body := p.parseBlock(func(tok Token) bool { return tok.Is("end") })
	end := p.expect("end")
	return &DoStmt{
		NodeSpan: NodeSpan{spanFrom(doTok, end.Span)},
		Do:       doTok.Span,
		Body:     body,
		End:      end.Span,
	}
}

func (p *parser) parseFor() Stmt {
	forTok := p.advance()
	name := p.parseName()
	if name == nil {
		p.recover()
		return nil
	}

	if p.check("=") {
		p.advance()
		init := p.parseExpr(0)
		p.expect(",")
		limit := p.parseExpr(0)
		var step Expr
		if p.check(",") {
			p.advance()
			step = p.parseExpr(0)
		}
		doTok := p.expect("do")
		body := p.parseBlock(func(tok Token) bool { return tok.Is("end") })
		end := p.expect("end")
		return &NumericForStmt{
			NodeSpan: NodeSpan{spanFrom(forTok, end.Span)},
			For:      forTok.Span,
			Name:     name,
			Init:     init,
			Limit:    limit,
			Step:     step,
			Do:       doTok.Span,
			Body:     body,
			End:      end.Span,
		}
	}

	names := []*NameExpr{name}
	for p.check(",") {
		p.advance()
		next := p.parseName()
		if next == nil {
			break
		}
		names = append(names, next)
	}
	inTok := p.expect("in")
	values := p.parseExprList()
	doTok := p.expect("do")
	body := p.parseBlock(func(tok Token) bool { return tok.Is("end") })
	end := p.expect("end")
	return &GenericForStmt{
		NodeSpan: NodeSpan{spanFrom(forTok, end.Span)},
		For:      forTok.Span,
		Names:    names,
		In:       inTok.Span,
		Values:   values,
		Do:       doTok.Span,
		Body:     body,
		End:      end.Span,
	}
}

func (p *parser) parseReturn() Stmt {
	ret := p.advance()
	var values []Expr
	if !p.eof() && !blockTerminator(p.current()) && !p.check(";") {
		values = p.parseExprList()
	}
	if p.check(";") {
		p.advance()
	}
	end := ret.Span
	if len(values) > 0 {
		end = values[len(values)-1].Span()
	}
	return &ReturnStmt{
		NodeSpan: NodeSpan{spanFrom(ret, end)},
		Return:   ret.Span,
		Values:   values,
	}
}

func (p *parser) parseGoto() Stmt {
	gotoTok := p.advance()
	name := p.parseName()
	if name == nil {
		return nil
	}
	return &GotoStmt{
		NodeSpan: NodeSpan{spanFrom(gotoTok, name.Span())},
		Goto:     gotoTok.Span,
		Name:     name,
	}
}

func (p *parser) parseLabel() Stmt {
	open := p.advance()
	name := p.parseName()
	close := p.expect("::")
	return &LabelStmt{
		NodeSpan: NodeSpan{spanFrom(open, close.Span)},
		Open:     open.Span,
		Name:     name,
		Close:    close.Span,
	}
}

func (p *parser) parsePrint() Stmt {
	q := p.advance()
	var args []Expr
	for !p.eof() && p.current().Line == q.Line && !lineBodyStop(p.current()) {
		if p.check("?") {
			break
		}
		arg := p.parseExpr(0)
		if arg == nil {
			break
		}
		args = append(args, arg)
		if !p.check(",") {
			break
		}
		p.advance()
	}
	end := q.Span
	if len(args) > 0 {
		end = args[len(args)-1].Span()
	}
	return &PrintStmt{
		NodeSpan: NodeSpan{spanFrom(q, end)},
		Question: q.Span,
		Args:     args,
	}
}

func (p *parser) parseExprList() []Expr {
	var list []Expr
	for {
		if p.eof() || p.check(")") || p.check("]") || p.check("}") || p.check(";") {
			break
		}
		expr := p.parseExpr(0)
		if expr == nil {
			break
		}
		list = append(list, expr)
		if !p.check(",") {
			break
		}
		p.advance()
	}
	return list
}

func (p *parser) parseExpr(minPrec int) Expr {
	left := p.parseUnary()
	if left == nil {
		return nil
	}
	for {
		tok := p.current()
		prec, rightAssoc, ok := binaryPrecedence(tok)
		if !ok || prec < minPrec {
			break
		}
		p.advance()
		nextMin := prec + 1
		if rightAssoc {
			nextMin = prec
		}
		right := p.parseExpr(nextMin)
		if right == nil {
			return left
		}
		left = &BinaryExpr{
			NodeSpan: NodeSpan{left.Span().Merge(right.Span())},
			Op:       tok,
			Left:     left,
			Right:    right,
		}
	}
	return left
}

func (p *parser) parseUnary() Expr {
	tok := p.current()
	if isUnaryOp(tok) {
		p.advance()
		x := p.parseExpr(unaryPrecedence)
		if x == nil {
			return nil
		}
		return &UnaryExpr{
			NodeSpan: NodeSpan{spanFrom(tok, x.Span())},
			Op:       tok,
			X:        x,
		}
	}
	return p.parseSuffixed()
}

func (p *parser) parseSuffixed() Expr {
	expr := p.parsePrimary()
	if expr == nil {
		return nil
	}

	for {
		switch {
		case p.check("."):
			p.advance()
			name := p.parseName()
			if name == nil {
				return expr
			}
			expr = &FieldExpr{
				NodeSpan: NodeSpan{expr.Span().Merge(name.Span())},
				Base:     expr,
				Name:     name,
			}
		case p.check("["):
			open := p.advance()
			index := p.parseExpr(0)
			close := p.expect("]")
			end := close.Span
			if index != nil {
				end = index.Span().Merge(close.Span)
			}
			expr = &IndexExpr{
				NodeSpan: NodeSpan{spanFrom(open, end)},
				Base:     expr,
				Index:    index,
			}
		case p.check(":"):
			p.advance()
			name := p.parseName()
			if name == nil {
				return expr
			}
			args, open, close := p.parseArgs()
			end := name.Span()
			if len(args) > 0 {
				end = end.Merge(args[len(args)-1].Span())
			} else if !close.IsEmpty() {
				end = end.Merge(close)
			}
			expr = &MethodCallExpr{
				NodeSpan: NodeSpan{expr.Span().Merge(end)},
				Receiver: expr,
				Method:   name,
				Args:     args,
				Open:     open,
				Close:    close,
			}
		case p.startsArgs():
			args, open, close := p.parseArgs()
			end := expr.Span()
			if len(args) > 0 {
				end = args[len(args)-1].Span()
			} else if !close.IsEmpty() {
				end = close
			}
			expr = &CallExpr{
				NodeSpan: NodeSpan{expr.Span().Merge(end)},
				Func:     expr,
				Args:     args,
				Open:     open,
				Close:    close,
			}
		default:
			return expr
		}
	}
}

func (p *parser) startsArgs() bool {
	tok := p.current()
	return tok.Is("(") || tok.Is("{") || tok.Kind == TokenString
}

func (p *parser) parseArgs() (args []Expr, open, close source.Span) {
	if p.check("(") {
		openTok := p.advance()
		open = openTok.Span
		args = p.parseExprList()
		closeTok := p.expect(")")
		close = closeTok.Span
		return args, open, close
	}
	if p.check("{") {
		table := p.parseTable()
		if table != nil {
			args = []Expr{table}
			open = table.Open
			close = table.Close
		}
		return args, open, close
	}
	if p.current().Kind == TokenString {
		tok := p.advance()
		expr := &StringExpr{
			NodeSpan: NodeSpan{tok.Span},
			Text:     tok.Text,
			Value:    tok.Value,
		}
		return []Expr{expr}, tok.Span, tok.Span
	}
	return nil, source.Span{}, source.Span{}
}

func (p *parser) parsePrimary() Expr {
	tok := p.current()
	switch tok.Kind {
	case TokenName:
		p.advance()
		return &NameExpr{NodeSpan: NodeSpan{tok.Span}, Text: tok.Text}
	case TokenNumber:
		p.advance()
		return &NumberExpr{NodeSpan: NodeSpan{tok.Span}, Text: tok.Text}
	case TokenString:
		p.advance()
		return &StringExpr{NodeSpan: NodeSpan{tok.Span}, Text: tok.Text, Value: tok.Value}
	case TokenKeyword:
		switch tok.Value {
		case "nil":
			p.advance()
			return &NilExpr{NodeSpan: NodeSpan{tok.Span}}
		case "true":
			p.advance()
			return &BoolExpr{NodeSpan: NodeSpan{tok.Span}, Value: true}
		case "false":
			p.advance()
			return &BoolExpr{NodeSpan: NodeSpan{tok.Span}, Value: false}
		case "function":
			p.advance()
			body := p.parseFunctionBody(tok)
			return body
		}
	case TokenPunct:
		switch tok.Text {
		case "...":
			p.advance()
			return &VarargExpr{NodeSpan: NodeSpan{tok.Span}}
		case "{":
			return p.parseTable()
		case "(":
			open := p.advance()
			expr := p.parseExpr(0)
			close := p.expect(")")
			end := close.Span
			if expr != nil {
				end = expr.Span().Merge(close.Span)
			}
			return &ParenExpr{
				NodeSpan: NodeSpan{spanFrom(open, end)},
				X:        expr,
				Open:     open.Span,
				Close:    close.Span,
			}
		}
	}

	p.errorAt(tok, fmt.Sprintf("expected expression, found %s", describeToken(tok)))
	return nil
}

func (p *parser) parseTable() *TableExpr {
	open := p.expect("{")
	table := &TableExpr{
		NodeSpan: NodeSpan{open.Span},
		Open:     open.Span,
	}
	for !p.eof() && !p.check("}") {
		before := p.pos
		var field *TableField

		if p.check("[") {
			p.advance()
			key := p.parseExpr(0)
			p.expect("]")
			p.expect("=")
			value := p.parseExpr(0)
			if key != nil && value != nil {
				field = &TableField{
					NodeSpan:   NodeSpan{key.Span().Merge(value.Span())},
					Key:        key,
					BracketKey: true,
					Value:      value,
				}
			}
		} else if p.current().Kind == TokenName && p.peek(1).Is("=") {
			name := p.parseName()
			p.advance() // =
			value := p.parseExpr(0)
			if name != nil && value != nil {
				field = &TableField{
					NodeSpan: NodeSpan{name.Span().Merge(value.Span())},
					Key:      name,
					Value:    value,
				}
			}
		} else {
			value := p.parseExpr(0)
			if value != nil {
				field = &TableField{
					NodeSpan: NodeSpan{value.Span()},
					Value:    value,
				}
			}
		}

		if field != nil {
			table.Fields = append(table.Fields, field)
		}
		if p.check(",") || p.check(";") {
			p.advance()
			continue
		}
		if !p.check("}") {
			p.errorAt(p.current(), "expected ',' or '}' in table constructor")
			p.recoverTo(",", ";", "}")
			if p.check(",") || p.check(";") {
				p.advance()
			}
		}
		if p.pos == before {
			p.advance()
		}
	}

	closeTok := p.expect("}")
	table.Close = closeTok.Span
	table.SpanVal = table.SpanVal.Merge(closeTok.Span)
	return table
}

func (p *parser) recover() {
	if !p.eof() && !blockTerminator(p.current()) {
		p.advance()
	}
	p.recoverTo(",")
}

func (p *parser) recoverTo(stops ...string) {
	stopSet := make(map[string]struct{}, len(stops))
	for _, s := range stops {
		stopSet[s] = struct{}{}
	}
	for !p.eof() {
		if _, ok := stopSet[p.current().Text]; ok {
			return
		}
		if blockTerminator(p.current()) || statementStart(p.current()) {
			return
		}
		p.advance()
	}
}

func blockTerminator(tok Token) bool {
	if tok.Kind != TokenKeyword {
		return false
	}
	switch tok.Value {
	case "else", "elseif", "end", "until":
		return true
	}
	return false
}

func statementStart(tok Token) bool {
	if tok.Kind == TokenKeyword {
		switch tok.Value {
		case "local", "function", "if", "for", "while", "repeat", "return", "break", "goto", "do":
			return true
		}
	}
	if tok.Kind == TokenInclude || tok.Kind == TokenTabMarker {
		return true
	}
	return tok.Is("?") || tok.Is("::") || tok.Is(";")
}

func isUnaryOp(tok Token) bool {
	if tok.Kind == TokenKeyword {
		return tok.Value == "not"
	}
	if tok.Kind != TokenPunct {
		return false
	}
	switch tok.Text {
	case "-", "~", "#", "@", "%", "$":
		return true
	}
	return false
}

const unaryPrecedence = 11

func binaryPrecedence(tok Token) (prec int, rightAssoc bool, ok bool) {
	if tok.Kind == TokenKeyword {
		switch tok.Value {
		case "or":
			return 1, false, true
		case "and":
			return 2, false, true
		}
		return 0, false, false
	}
	if tok.Kind != TokenPunct {
		return 0, false, false
	}
	switch tok.Text {
	case "<", ">", "<=", ">=", "==", "~=", "!=":
		return 3, false, true
	case "|":
		return 4, false, true
	case "^^", "~":
		return 5, false, true
	case "&":
		return 6, false, true
	case "<<", ">>", ">>>", "<<>", ">><":
		return 7, false, true
	case "..":
		return 8, true, true
	case "+", "-":
		return 9, false, true
	case "*", "/", "%", "\\":
		return 10, false, true
	case "^":
		return 12, true, true
	default:
		return 0, false, false
	}
}
