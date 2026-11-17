package syntax

import (
	"strings"
	"testing"
)

func parseText(t *testing.T, text string) (*Chunk, []Error) {
	t.Helper()
	lexed := Lex([]byte(text), nil)
	chunk, errs := Parse(lexed.Tokens)
	errs = append(lexed.Errors, errs...)
	return chunk, errs
}

func oneStmt(t *testing.T, text string) Stmt {
	t.Helper()
	chunk, errs := parseText(t, text)
	if len(errs) != 0 {
		t.Fatalf("errors = %v", errs)
	}
	if chunk.Block == nil || len(chunk.Block.Stmts) != 1 {
		t.Fatalf("statements = %d, want 1", len(chunk.Block.Stmts))
	}
	return chunk.Block.Stmts[0]
}

func exprText(e Expr) string {
	switch x := e.(type) {
	case nil:
		return "<nil>"
	case *NilExpr:
		return "nil"
	case *BoolExpr:
		if x.Value {
			return "true"
		}
		return "false"
	case *NumberExpr:
		return x.Text
	case *StringExpr:
		return x.Text
	case *VarargExpr:
		return "..."
	case *NameExpr:
		return x.Text
	case *UnaryExpr:
		return x.Op.Text + "(" + exprText(x.X) + ")"
	case *BinaryExpr:
		return "(" + exprText(x.Left) + " " + x.Op.Text + " " + exprText(x.Right) + ")"
	case *FieldExpr:
		return exprText(x.Base) + "." + x.Name.Text
	case *IndexExpr:
		return exprText(x.Base) + "[" + exprText(x.Index) + "]"
	case *CallExpr:
		return exprText(x.Func) + "(" + exprListText(x.Args) + ")"
	case *MethodCallExpr:
		return exprText(x.Receiver) + ":" + x.Method.Text + "(" + exprListText(x.Args) + ")"
	case *FunctionExpr:
		return "function(" + nameListText(x.Params) + ")..." + "end"
	case *TableExpr:
		return "{table}"
	case *ParenExpr:
		return "(" + exprText(x.X) + ")"
	default:
		return "<expr>"
	}
}

func exprListText(list []Expr) string {
	parts := make([]string, len(list))
	for i, e := range list {
		parts[i] = exprText(e)
	}
	return strings.Join(parts, ",")
}

func nameListText(names []*NameExpr) string {
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = n.Text
	}
	return strings.Join(parts, ",")
}

func TestParseExpressionPrecedence(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{"1+2*3", "(1 + (2 * 3))"},
		{"1*2+3", "((1 * 2) + 3)"},
		{"2^3^4", "(2 ^ (3 ^ 4))"},
		{"-2^2", "-((2 ^ 2))"},
		{"a..b..c", "(a .. (b .. c))"},
		{"a and b or c", "((a and b) or c)"},
		{"a | b ^^ c & d", "(a | (b ^^ (c & d)))"},
		{"a << b + c", "(a << (b + c))"},
		{"a < b + c and d", "((a < (b + c)) and d)"},
		{"not a == b", "(not(a) == b)"},
		{"%a + b", "(%(a) + b)"},
		{"a % b", "(a % b)"},
		{"@a.b", "@(a.b)"},
		{"$a", "$(a)"},
		{"#t", "#(t)"},
		{"~a", "~(a)"},
		{"a != b", "(a != b)"},
		{"a <<> b", "(a <<> b)"},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			stmt := oneStmt(t, "x="+tt.text)
			assign, ok := stmt.(*AssignStmt)
			if !ok {
				t.Fatalf("stmt = %T, want assign", stmt)
			}
			if got := exprText(assign.Values[0]); got != tt.want {
				t.Fatalf("expr = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestParseSuffixedExpressions(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{"f(1)", "f(1)"},
		{"f(1)(2)", "f(1)(2)"},
		{"a.b[1].c(3)", "a.b[1].c(3)"},
		{"a:b(1)", "a:b(1)"},
		{"a:b():c(2)", "a:b():c(2)"},
		{`print"hi"`, `print("hi")`},
		{"f{}", "f({table})"},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			stmt := oneStmt(t, tt.text)
			call, ok := stmt.(*CallStmt)
			if !ok {
				t.Fatalf("stmt = %T, want call", stmt)
			}
			if got := exprText(call.Call); got != tt.want {
				t.Fatalf("expr = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestParseStatements(t *testing.T) {
	tests := []struct {
		text string
		kind string
	}{
		{"local a, b = 1, 2", "*syntax.LocalStmt"},
		{"local function f(x) return x end", "*syntax.LocalFunctionStmt"},
		{"a, b.c, d[1] = 1, 2, 3", "*syntax.AssignStmt"},
		{"t.x += 1", "*syntax.AssignStmt"},
		{"f()", "*syntax.CallStmt"},
		{"do a=1 end", "*syntax.DoStmt"},
		{"while x do a=1 end", "*syntax.WhileStmt"},
		{"repeat a=1 until x", "*syntax.RepeatStmt"},
		{"if x then a=1 elseif y then b=2 else c=3 end", "*syntax.IfStmt"},
		{"for i=1,10 do a=i end", "*syntax.NumericForStmt"},
		{"for k,v in pairs(t) do a=k end", "*syntax.GenericForStmt"},
		{"function a.b:c(x, ...) return x end", "*syntax.FunctionDeclStmt"},
		{"return 1, 2;", "*syntax.ReturnStmt"},
		{"break", "*syntax.BreakStmt"},
		{"goto done", "*syntax.GotoStmt"},
		{"::done::", "*syntax.LabelStmt"},
		{"?a,b", "*syntax.PrintStmt"},
		{";", "*syntax.EmptyStmt"},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			stmt := oneStmt(t, tt.text)
			if got := typeName(stmt); got != tt.kind {
				t.Fatalf("kind = %s, want %s", got, tt.kind)
			}
		})
	}
}

func typeName(v any) string {
	switch v.(type) {
	case *LocalStmt:
		return "*syntax.LocalStmt"
	case *LocalFunctionStmt:
		return "*syntax.LocalFunctionStmt"
	case *AssignStmt:
		return "*syntax.AssignStmt"
	case *CallStmt:
		return "*syntax.CallStmt"
	case *DoStmt:
		return "*syntax.DoStmt"
	case *WhileStmt:
		return "*syntax.WhileStmt"
	case *RepeatStmt:
		return "*syntax.RepeatStmt"
	case *IfStmt:
		return "*syntax.IfStmt"
	case *NumericForStmt:
		return "*syntax.NumericForStmt"
	case *GenericForStmt:
		return "*syntax.GenericForStmt"
	case *FunctionDeclStmt:
		return "*syntax.FunctionDeclStmt"
	case *ReturnStmt:
		return "*syntax.ReturnStmt"
	case *BreakStmt:
		return "*syntax.BreakStmt"
	case *GotoStmt:
		return "*syntax.GotoStmt"
	case *LabelStmt:
		return "*syntax.LabelStmt"
	case *PrintStmt:
		return "*syntax.PrintStmt"
	case *EmptyStmt:
		return "*syntax.EmptyStmt"
	default:
		return "<unknown>"
	}
}

func TestParseTableConstructors(t *testing.T) {
	stmt := oneStmt(t, "t = {1, x=2, [k]=3; 4,}")
	assign := stmt.(*AssignStmt)
	table, ok := assign.Values[0].(*TableExpr)
	if !ok {
		t.Fatalf("value = %T, want table", assign.Values[0])
	}
	if len(table.Fields) != 4 {
		t.Fatalf("fields = %d, want 4", len(table.Fields))
	}
	if table.Fields[1].Key == nil || table.Fields[1].Key.(*NameExpr).Text != "x" {
		t.Fatalf("field 1 = %+v", table.Fields[1])
	}
	if !table.Fields[2].BracketKey {
		t.Fatal("field 2 should have a bracket key")
	}
}

func TestParseFunctionDeclarationPaths(t *testing.T) {
	stmt := oneStmt(t, "function a.b:c(x, ...) return x end")
	decl := stmt.(*FunctionDeclStmt)
	if !decl.Method {
		t.Fatal("method = false, want true")
	}
	if len(decl.Names) != 3 || decl.Names[0].Text != "a" || decl.Names[1].Text != "b" || decl.Names[2].Text != "c" {
		t.Fatalf("names = %v", nameListText(decl.Names))
	}
	if !decl.Func.Vararg || len(decl.Func.Params) != 1 {
		t.Fatalf("params/vararg = %v/%v", decl.Func.Params, decl.Func.Vararg)
	}
}

func TestParseShorthandIf(t *testing.T) {
	stmt := oneStmt(t, "if (x) y=1")
	ifStmt := stmt.(*IfStmt)
	if !ifStmt.SingleLine || len(ifStmt.Branches) != 1 || len(ifStmt.Branches[0].Body.Stmts) != 1 {
		t.Fatalf("if = %+v", ifStmt)
	}
}

func TestParseShorthandIfElseSameLine(t *testing.T) {
	stmt := oneStmt(t, "if (x) y=1 else z=2")
	ifStmt := stmt.(*IfStmt)
	if ifStmt.Else == nil || len(ifStmt.Else.Stmts) != 1 {
		t.Fatalf("else = %+v", ifStmt.Else)
	}
}

func TestParseShorthandIfDoesNotConsumeNextLineElse(t *testing.T) {
	chunk, errs := parseText(t, "if (x) y=1\nelse z=2")
	if len(errs) == 0 {
		t.Fatal("expected a stray-else error")
	}
	if len(chunk.Block.Stmts) < 1 {
		t.Fatal("missing shorthand if")
	}
	ifStmt, ok := chunk.Block.Stmts[0].(*IfStmt)
	if !ok || !ifStmt.SingleLine || ifStmt.Else != nil {
		t.Fatalf("if = %+v", chunk.Block.Stmts[0])
	}
}

func TestParseShorthandIfInsideStandardIf(t *testing.T) {
	chunk, errs := parseText(t, `if a then
  if (x) y=1
else
  z=2
end`)
	if len(errs) != 0 {
		t.Fatalf("errors = %v", errs)
	}
	outer := chunk.Block.Stmts[0].(*IfStmt)
	if outer.SingleLine || outer.Else == nil {
		t.Fatalf("outer if = %+v", outer)
	}
	inner := outer.Branches[0].Body.Stmts[0].(*IfStmt)
	if !inner.SingleLine || inner.Else != nil {
		t.Fatalf("inner if = %+v", inner)
	}
}

func TestParseNestedShorthandOnOneLine(t *testing.T) {
	chunk, errs := parseText(t, "if (x) if (y) z=1")
	if len(errs) != 0 {
		t.Fatalf("errors = %v", errs)
	}
	outer := chunk.Block.Stmts[0].(*IfStmt)
	if !outer.SingleLine || len(outer.Branches[0].Body.Stmts) != 1 {
		t.Fatalf("outer = %+v", outer)
	}
	inner, ok := outer.Branches[0].Body.Stmts[0].(*IfStmt)
	if !ok || !inner.SingleLine {
		t.Fatalf("inner = %T/%+v", outer.Branches[0].Body.Stmts[0], inner)
	}

	chunk, errs = parseText(t, "while (x) if (y) z=1")
	if len(errs) != 0 {
		t.Fatalf("errors = %v", errs)
	}
	while := chunk.Block.Stmts[0].(*WhileStmt)
	if !while.SingleLine || len(while.Body.Stmts) != 1 {
		t.Fatalf("while = %+v", while)
	}
}

func TestParseShorthandWhile(t *testing.T) {
	stmt := oneStmt(t, "while (x) y=1")
	while := stmt.(*WhileStmt)
	if !while.SingleLine || len(while.Body.Stmts) != 1 {
		t.Fatalf("while = %+v", while)
	}
}

func TestParseShorthandWhileNoBody(t *testing.T) {
	chunk, errs := parseText(t, "while (x)\ny=1")
	if len(errs) == 0 {
		t.Fatal("expected shorthand body error")
	}
	while, ok := chunk.Block.Stmts[0].(*WhileStmt)
	if !ok || !while.SingleLine || len(while.Body.Stmts) != 0 {
		t.Fatalf("while = %+v", chunk.Block.Stmts[0])
	}
	if len(chunk.Block.Stmts) != 2 {
		t.Fatalf("statements = %d, want while + assignment", len(chunk.Block.Stmts))
	}
}

func TestParsePrintShorthand(t *testing.T) {
	tests := []string{
		"?x",
		"?x,y",
		"if (x) ?y",
		"while (x) ?y",
	}
	for _, text := range tests {
		t.Run(text, func(t *testing.T) {
			chunk, errs := parseText(t, text)
			if len(errs) != 0 {
				t.Fatalf("errors = %v", errs)
			}
			if len(chunk.Block.Stmts) != 1 {
				t.Fatalf("statements = %d", len(chunk.Block.Stmts))
			}
		})
	}
}

func TestParsePeekAssignmentError(t *testing.T) {
	_, errs := parseText(t, "@x = 1")
	if len(errs) == 0 {
		t.Fatal("expected read-only peek error")
	}
	found := false
	for _, err := range errs {
		if strings.Contains(err.Message, "read-only") {
			found = true
		}
	}
	if !found {
		t.Fatalf("errors = %v, want read-only", errs)
	}
}

func TestParseMissingEndRecovers(t *testing.T) {
	chunk, errs := parseText(t, "function f()\n  a=1")
	if len(errs) == 0 {
		t.Fatal("expected missing-end error")
	}
	if len(chunk.Block.Stmts) != 1 {
		t.Fatalf("statements = %d, want partial function", len(chunk.Block.Stmts))
	}
	if _, ok := chunk.Block.Stmts[0].(*FunctionDeclStmt); !ok {
		t.Fatalf("stmt = %T", chunk.Block.Stmts[0])
	}
}

func TestParseTruncatedAtEveryByte(t *testing.T) {
	program := `function f(a, b)
  local t = {x=1, [a]=b}
  if (a) ?t.x else t.x += 1
  for i=1,3 do t[i] = i end
  return t
end
while (a) a += 1
`
	for i := 0; i <= len(program); i++ {
		lexed := Lex([]byte(program[:i]), nil)
		chunk, errs := Parse(lexed.Tokens)
		if chunk == nil {
			t.Fatalf("offset %d: nil chunk", i)
		}
		if len(errs) > maxParseErrors {
			t.Fatalf("offset %d: %d parse errors, want <= %d", i, len(errs), maxParseErrors)
		}
	}
}

func TestParseChainedComparisonAndBitwise(t *testing.T) {
	stmt := oneStmt(t, "x = a < b & c")
	assign := stmt.(*AssignStmt)
	want := "(a < (b & c))"
	if got := exprText(assign.Values[0]); got != want {
		t.Fatalf("expr = %s, want %s", got, want)
	}

	stmt = oneStmt(t, "x = a | b ^^ c")
	assign = stmt.(*AssignStmt)
	want = "(a | (b ^^ c))"
	if got := exprText(assign.Values[0]); got != want {
		t.Fatalf("expr = %s, want %s", got, want)
	}
}
