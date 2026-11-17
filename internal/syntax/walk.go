package syntax

import "github.com/asimmons91/pico-lsp/internal/source"

// Span returns the span of the chunk's top-level block.
func (c *Chunk) Span() source.Span {
	if c == nil || c.Block == nil {
		return source.Span{}
	}
	return c.Block.Span()
}

// Walk visits root and its children in source order. If fn returns false, the
// node's children are skipped.
func Walk(root Node, fn func(Node) bool) {
	if root == nil || fn == nil {
		return
	}
	if !fn(root) {
		return
	}

	switch n := root.(type) {
	case *Chunk:
		Walk(n.Block, fn)
	case *Block:
		for _, stmt := range n.Stmts {
			Walk(stmt, fn)
		}
	case *LocalStmt:
		for _, name := range n.Names {
			Walk(name, fn)
		}
		for _, value := range n.Values {
			Walk(value, fn)
		}
	case *LocalFunctionStmt:
		Walk(n.Name, fn)
		Walk(n.Func, fn)
	case *AssignStmt:
		for _, target := range n.Targets {
			Walk(target, fn)
		}
		for _, value := range n.Values {
			Walk(value, fn)
		}
	case *CallStmt:
		Walk(n.Call, fn)
	case *DoStmt:
		Walk(n.Body, fn)
	case *WhileStmt:
		Walk(n.Cond, fn)
		Walk(n.Body, fn)
	case *RepeatStmt:
		Walk(n.Body, fn)
		Walk(n.Cond, fn)
	case *IfStmt:
		for _, branch := range n.Branches {
			Walk(branch, fn)
		}
		if n.Else != nil {
			Walk(n.Else, fn)
		}
	case *IfBranch:
		Walk(n.Cond, fn)
		Walk(n.Body, fn)
	case *NumericForStmt:
		Walk(n.Name, fn)
		Walk(n.Init, fn)
		Walk(n.Limit, fn)
		if n.Step != nil {
			Walk(n.Step, fn)
		}
		Walk(n.Body, fn)
	case *GenericForStmt:
		for _, name := range n.Names {
			Walk(name, fn)
		}
		for _, value := range n.Values {
			Walk(value, fn)
		}
		Walk(n.Body, fn)
	case *FunctionDeclStmt:
		for _, name := range n.Names {
			Walk(name, fn)
		}
		Walk(n.Func, fn)
	case *ReturnStmt:
		for _, value := range n.Values {
			Walk(value, fn)
		}
	case *GotoStmt:
		Walk(n.Name, fn)
	case *LabelStmt:
		Walk(n.Name, fn)
	case *PrintStmt:
		for _, arg := range n.Args {
			Walk(arg, fn)
		}
	case *UnaryExpr:
		Walk(n.X, fn)
	case *BinaryExpr:
		Walk(n.Left, fn)
		Walk(n.Right, fn)
	case *FieldExpr:
		Walk(n.Base, fn)
		Walk(n.Name, fn)
	case *IndexExpr:
		Walk(n.Base, fn)
		Walk(n.Index, fn)
	case *CallExpr:
		Walk(n.Func, fn)
		for _, arg := range n.Args {
			Walk(arg, fn)
		}
	case *MethodCallExpr:
		Walk(n.Receiver, fn)
		Walk(n.Method, fn)
		for _, arg := range n.Args {
			Walk(arg, fn)
		}
	case *FunctionExpr:
		for _, param := range n.Params {
			Walk(param, fn)
		}
		Walk(n.Body, fn)
	case *TableExpr:
		for _, field := range n.Fields {
			Walk(field, fn)
		}
	case *TableField:
		if n.Key != nil {
			Walk(n.Key, fn)
		}
		Walk(n.Value, fn)
	case *ParenExpr:
		Walk(n.X, fn)
	}
}
