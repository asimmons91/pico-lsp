package syntax

import "github.com/asimmons91/pico-lsp/internal/source"

// Node is any AST node with an original-file span.
type Node interface {
	Span() source.Span
}

// NodeSpan is embedded by AST nodes.
type NodeSpan struct {
	SpanVal source.Span
}

// Span implements Node.
func (n NodeSpan) Span() source.Span { return n.SpanVal }

// Expr is an expression node.
type Expr interface {
	Node
	exprNode()
}

// Stmt is a statement node.
type Stmt interface {
	Node
	stmtNode()
}

// Chunk is the parsed top-level block.
type Chunk struct {
	Block *Block
}

// Block is a statement sequence. It can be empty.
type Block struct {
	NodeSpan
	Stmts []Stmt
}

type NilExpr struct{ NodeSpan }

type BoolExpr struct {
	NodeSpan
	Value bool
}

type NumberExpr struct {
	NodeSpan
	Text string
}

type StringExpr struct {
	NodeSpan
	Text  string
	Value string
}

type VarargExpr struct{ NodeSpan }

// NameExpr is an identifier use or declaration. Decls are identified by the
// statement that owns the name.
type NameExpr struct {
	NodeSpan
	Text string
}

type UnaryExpr struct {
	NodeSpan
	Op Token
	X  Expr
}

type BinaryExpr struct {
	NodeSpan
	Op          Token
	Left, Right Expr
}

type FieldExpr struct {
	NodeSpan
	Base Expr
	Name *NameExpr
}

type IndexExpr struct {
	NodeSpan
	Base  Expr
	Index Expr
}

type CallExpr struct {
	NodeSpan
	Func  Expr
	Args  []Expr
	Open  source.Span
	Close source.Span
}

type MethodCallExpr struct {
	NodeSpan
	Receiver Expr
	Method   *NameExpr
	Args     []Expr
	Open     source.Span
	Close    source.Span
}

type FunctionExpr struct {
	NodeSpan
	Params []*NameExpr
	Vararg bool
	Body   *Block
	End    source.Span
}

type TableExpr struct {
	NodeSpan
	Fields []*TableField
	Open   source.Span
	Close  source.Span
}

type TableField struct {
	NodeSpan
	// Key is nil for array-style fields. For name = value it is a NameExpr.
	// BracketKey distinguishes [expr] = value from a plain name key.
	Key        Expr
	BracketKey bool
	Value      Expr
}

type ParenExpr struct {
	NodeSpan
	X     Expr
	Open  source.Span
	Close source.Span
}

func (*NilExpr) exprNode()        {}
func (*BoolExpr) exprNode()       {}
func (*NumberExpr) exprNode()     {}
func (*StringExpr) exprNode()     {}
func (*VarargExpr) exprNode()     {}
func (*NameExpr) exprNode()       {}
func (*UnaryExpr) exprNode()      {}
func (*BinaryExpr) exprNode()     {}
func (*FieldExpr) exprNode()      {}
func (*IndexExpr) exprNode()      {}
func (*CallExpr) exprNode()       {}
func (*MethodCallExpr) exprNode() {}
func (*FunctionExpr) exprNode()   {}
func (*TableExpr) exprNode()      {}
func (*ParenExpr) exprNode()      {}

type LocalStmt struct {
	NodeSpan
	Local  source.Span
	Names  []*NameExpr
	Values []Expr
}

type LocalFunctionStmt struct {
	NodeSpan
	Local    source.Span
	Function source.Span
	Name     *NameExpr
	Func     *FunctionExpr
}

type AssignStmt struct {
	NodeSpan
	Targets []Expr
	Op      Token
	Values  []Expr
}

type CallStmt struct {
	NodeSpan
	Call Expr
}

type DoStmt struct {
	NodeSpan
	Do   source.Span
	Body *Block
	End  source.Span
}

type WhileStmt struct {
	NodeSpan
	While      source.Span
	Cond       Expr
	Do         source.Span
	Body       *Block
	End        source.Span
	SingleLine bool
}

type RepeatStmt struct {
	NodeSpan
	Repeat source.Span
	Body   *Block
	Until  source.Span
	Cond   Expr
}

type IfStmt struct {
	NodeSpan
	Branches   []*IfBranch
	Else       *Block
	ElseSpan   source.Span
	End        source.Span
	SingleLine bool
}

type IfBranch struct {
	NodeSpan
	Keyword source.Span
	Cond    Expr
	Then    source.Span
	Body    *Block
	ElseIf  bool
}

type NumericForStmt struct {
	NodeSpan
	For         source.Span
	Name        *NameExpr
	Init, Limit Expr
	Step        Expr
	Do          source.Span
	Body        *Block
	End         source.Span
}

type GenericForStmt struct {
	NodeSpan
	For    source.Span
	Names  []*NameExpr
	In     source.Span
	Values []Expr
	Do     source.Span
	Body   *Block
	End    source.Span
}

type FunctionDeclStmt struct {
	NodeSpan
	Function source.Span
	Names    []*NameExpr
	Method   bool
	Func     *FunctionExpr
}

type ReturnStmt struct {
	NodeSpan
	Return source.Span
	Values []Expr
}

type BreakStmt struct {
	NodeSpan
	Break source.Span
}

type GotoStmt struct {
	NodeSpan
	Goto source.Span
	Name *NameExpr
}

type LabelStmt struct {
	NodeSpan
	Open  source.Span
	Name  *NameExpr
	Close source.Span
}

type PrintStmt struct {
	NodeSpan
	Question source.Span
	Args     []Expr
}

type EmptyStmt struct {
	NodeSpan
	Semicolon source.Span
}

func (*LocalStmt) stmtNode()         {}
func (*LocalFunctionStmt) stmtNode() {}
func (*AssignStmt) stmtNode()        {}
func (*CallStmt) stmtNode()          {}
func (*DoStmt) stmtNode()            {}
func (*WhileStmt) stmtNode()         {}
func (*RepeatStmt) stmtNode()        {}
func (*IfStmt) stmtNode()            {}
func (*NumericForStmt) stmtNode()    {}
func (*GenericForStmt) stmtNode()    {}
func (*FunctionDeclStmt) stmtNode()  {}
func (*ReturnStmt) stmtNode()        {}
func (*BreakStmt) stmtNode()         {}
func (*GotoStmt) stmtNode()          {}
func (*LabelStmt) stmtNode()         {}
func (*PrintStmt) stmtNode()         {}
func (*EmptyStmt) stmtNode()         {}
