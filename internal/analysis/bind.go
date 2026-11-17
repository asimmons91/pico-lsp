package analysis

import (
	"fmt"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// TypeCheckingMode controls type diagnostics.
type TypeCheckingMode string

const (
	TypeCheckingOff    TypeCheckingMode = "off"
	TypeCheckingBasic  TypeCheckingMode = "basic"
	TypeCheckingStrict TypeCheckingMode = "strict"
)

// Options controls analysis diagnostics.
type Options struct {
	UndefinedGlobal Severity
	TypeChecking    TypeCheckingMode
}

// typeCheckingMode returns the effective mode, defaulting to basic.
func (o Options) typeCheckingMode() TypeCheckingMode {
	switch o.TypeChecking {
	case TypeCheckingOff, TypeCheckingStrict:
		return o.TypeChecking
	default:
		return TypeCheckingBasic
	}
}

// Analyze binds one cart snapshot. builtins may be nil, which disables
// built-in name resolution while keeping every other feature working.
func Analyze(snap *cart.Snapshot, builtins map[string]*Symbol, opts Options) *Result {
	result := &Result{
		Table:       NewSymbolTable(builtins),
		meta:        metaFiles(snap),
		funcSymbols: make(map[*syntax.FunctionExpr]*Symbol),
	}
	if snap == nil || snap.Cart == nil || snap.Chunk == nil {
		result.finish()
		return result
	}
	result.cart = snap.Cart

	chunkScope := NewScope(nil)
	chunkScope.Labels = make(map[string]*Symbol)
	result.chunkScope = chunkScope
	table := result.Table

	// Globals are collected before any use is resolved, so a function can be
	// called before its definition and includes can refer to each other.
	for _, decl := range prebindGlobals(snap.Chunk) {
		canonical := CanonicalName(decl.name)
		sym := table.Globals[canonical]
		if sym == nil {
			sym = table.NewSymbol(SymbolGlobal, decl.name, decl.span, chunkScope)
			table.Globals[canonical] = sym
		}
		if decl.function {
			sym.Kind = SymbolFunction
		}
	}

	b := &binder{
		result:      result,
		table:       table,
		opts:        opts,
		labels:      []*labelScope{{labels: chunkScope.Labels, function: chunkScope}},
		selfAliases: make(map[*Symbol]*Symbol),
	}
	b.collectLabels(snap.Chunk.Block, chunkScope)
	b.bindBlock(snap.Chunk.Block, chunkScope)
	attachDocs(snap, result)
	attachParamDocs(result)
	attachTypeAnnotations(snap, result, opts)
	result.finish()
	inferTypes(snap, result, opts)
	result.finish()
	return result
}

func metaFiles(snap *cart.Snapshot) map[source.FileID]bool {
	out := make(map[source.FileID]bool)
	if snap == nil {
		return out
	}
	for _, comment := range snap.Comments {
		if comment.Kind != syntax.CommentDoc {
			continue
		}
		if strings.TrimSpace(comment.Text) == "---@meta" {
			out[comment.Span.FileID] = true
		}
	}
	return out
}

// Pre-pass: collect globals without resolving any uses. The pre-binder still
// tracks local declarations so an assignment to a local is not mistaken for a
// global. Its scopes are intentionally separate from the binding pass.
type pendingGlobal struct {
	name     string
	span     source.Span
	function bool
}

type preScope struct {
	parent *preScope
	names  map[string]struct{}
}

func newPreScope(parent *preScope) *preScope {
	return &preScope{parent: parent, names: make(map[string]struct{})}
}

func (s *preScope) declare(name string) {
	s.names[CanonicalName(name)] = struct{}{}
}

func (s *preScope) has(name string) bool {
	canonical := CanonicalName(name)
	for scope := s; scope != nil; scope = scope.parent {
		if _, ok := scope.names[canonical]; ok {
			return true
		}
	}
	return false
}

type prebinder struct {
	globals []pendingGlobal
}

func prebindGlobals(chunk *syntax.Chunk) []pendingGlobal {
	if chunk == nil {
		return nil
	}
	p := &prebinder{}
	scope := newPreScope(nil)
	p.preBlock(chunk.Block, scope)
	return p.globals
}

func (p *prebinder) addGlobal(name *syntax.NameExpr, function bool) {
	if name == nil {
		return
	}
	p.globals = append(p.globals, pendingGlobal{
		name:     name.Text,
		span:     name.Span(),
		function: function,
	})
}

func (p *prebinder) preBlock(block *syntax.Block, scope *preScope) {
	if block == nil {
		return
	}
	for _, stmt := range block.Stmts {
		p.preStmt(stmt, scope)
	}
}

func (p *prebinder) preStmt(stmt syntax.Stmt, scope *preScope) {
	switch st := stmt.(type) {
	case *syntax.LocalStmt:
		for _, value := range st.Values {
			p.preExpr(value, scope)
		}
		for _, name := range st.Names {
			scope.declare(name.Text)
		}
	case *syntax.LocalFunctionStmt:
		scope.declare(st.Name.Text)
		p.preFunction(st.Func, scope, false)
	case *syntax.AssignStmt:
		for _, target := range st.Targets {
			p.preTarget(target, scope)
		}
		for _, value := range st.Values {
			p.preExpr(value, scope)
		}
	case *syntax.CallStmt:
		p.preExpr(st.Call, scope)
	case *syntax.DoStmt:
		p.preBlock(st.Body, newPreScope(scope))
	case *syntax.WhileStmt:
		p.preExpr(st.Cond, scope)
		p.preBlock(st.Body, newPreScope(scope))
	case *syntax.RepeatStmt:
		bodyScope := newPreScope(scope)
		p.preBlock(st.Body, bodyScope)
		p.preExpr(st.Cond, bodyScope)
	case *syntax.IfStmt:
		for _, branch := range st.Branches {
			p.preExpr(branch.Cond, scope)
			p.preBlock(branch.Body, newPreScope(scope))
		}
		if st.Else != nil {
			p.preBlock(st.Else, newPreScope(scope))
		}
	case *syntax.NumericForStmt:
		p.preExpr(st.Init, scope)
		p.preExpr(st.Limit, scope)
		if st.Step != nil {
			p.preExpr(st.Step, scope)
		}
		loop := newPreScope(scope)
		loop.declare(st.Name.Text)
		p.preBlock(st.Body, newPreScope(loop))
	case *syntax.GenericForStmt:
		for _, value := range st.Values {
			p.preExpr(value, scope)
		}
		loop := newPreScope(scope)
		for _, name := range st.Names {
			loop.declare(name.Text)
		}
		p.preBlock(st.Body, newPreScope(loop))
	case *syntax.FunctionDeclStmt:
		if len(st.Names) == 1 && !scope.has(st.Names[0].Text) {
			p.addGlobal(st.Names[0], true)
		}
		p.preFunction(st.Func, scope, st.Method)
	case *syntax.ReturnStmt:
		for _, value := range st.Values {
			p.preExpr(value, scope)
		}
	case *syntax.PrintStmt:
		for _, arg := range st.Args {
			p.preExpr(arg, scope)
		}
	}
}

func (p *prebinder) preFunction(fn *syntax.FunctionExpr, scope *preScope, method bool) {
	if fn == nil {
		return
	}
	fnScope := newPreScope(scope)
	if method {
		fnScope.declare("self")
	}
	for _, param := range fn.Params {
		fnScope.declare(param.Text)
	}
	p.preBlock(fn.Body, newPreScope(fnScope))
}

func (p *prebinder) preTarget(expr syntax.Expr, scope *preScope) {
	switch t := expr.(type) {
	case *syntax.NameExpr:
		if !scope.has(t.Text) {
			p.addGlobal(t, false)
		}
	case *syntax.FieldExpr:
		p.preExpr(t.Base, scope)
	case *syntax.IndexExpr:
		p.preExpr(t.Base, scope)
		p.preExpr(t.Index, scope)
	default:
		p.preExpr(expr, scope)
	}
}

func (p *prebinder) preExpr(expr syntax.Expr, scope *preScope) {
	switch e := expr.(type) {
	case nil:
		return
	case *syntax.UnaryExpr:
		p.preExpr(e.X, scope)
	case *syntax.BinaryExpr:
		p.preExpr(e.Left, scope)
		p.preExpr(e.Right, scope)
	case *syntax.FieldExpr:
		p.preExpr(e.Base, scope)
	case *syntax.IndexExpr:
		p.preExpr(e.Base, scope)
		p.preExpr(e.Index, scope)
	case *syntax.CallExpr:
		p.preExpr(e.Func, scope)
		for _, arg := range e.Args {
			p.preExpr(arg, scope)
		}
	case *syntax.MethodCallExpr:
		p.preExpr(e.Receiver, scope)
		for _, arg := range e.Args {
			p.preExpr(arg, scope)
		}
	case *syntax.FunctionExpr:
		p.preFunction(e, scope, false)
	case *syntax.TableExpr:
		for _, field := range e.Fields {
			if field.Key != nil {
				p.preExpr(field.Key, scope)
			}
			p.preExpr(field.Value, scope)
		}
	case *syntax.ParenExpr:
		p.preExpr(e.X, scope)
	}
}

// Binding pass.

type labelScope struct {
	labels   map[string]*Symbol
	function *Scope
}

type binder struct {
	result      *Result
	table       *SymbolTable
	opts        Options
	labels      []*labelScope
	selfAliases map[*Symbol]*Symbol
}

// valueSymbol resolves an implicit method self to the receiver for field
// lookup. The self symbol itself is still used for navigation and highlights.
func (b *binder) valueSymbol(sym *Symbol) *Symbol {
	if sym == nil {
		return nil
	}
	if receiver := b.selfAliases[sym]; receiver != nil {
		return receiver
	}
	return sym
}

func (b *binder) bindBlock(block *syntax.Block, scope *Scope) {
	if block == nil {
		return
	}
	if scope != nil && scope.Span.IsEmpty() {
		span := block.Span()
		if !span.IsEmpty() {
			scope.Span = span
			b.result.scopes = append(b.result.scopes, scope)
		}
	}
	for _, stmt := range block.Stmts {
		b.bindStmt(stmt, scope)
	}
}

func (b *binder) bindStmt(stmt syntax.Stmt, scope *Scope) {
	switch st := stmt.(type) {
	case *syntax.LocalStmt:
		b.bindLocal(st, scope)
	case *syntax.LocalFunctionStmt:
		sym := b.table.NewSymbol(SymbolFunction, st.Name.Text, st.Name.Span(), scope)
		scope.Declare(sym)
		b.addDecl(st.Name, sym, scope)
		setFunctionInfo(sym, st.Name.Text, st.Func)
		b.bindFunction(st.Func, scope, false, nil, source.Span{}, sym)
	case *syntax.AssignStmt:
		b.bindAssign(st, scope)
	case *syntax.CallStmt:
		b.bindExpr(st.Call, scope)
	case *syntax.DoStmt:
		b.bindBlock(st.Body, NewScope(scope))
	case *syntax.WhileStmt:
		b.bindExpr(st.Cond, scope)
		b.bindBlock(st.Body, NewScope(scope))
	case *syntax.RepeatStmt:
		bodyScope := NewScope(scope)
		b.bindBlock(st.Body, bodyScope)
		b.bindExpr(st.Cond, bodyScope)
	case *syntax.IfStmt:
		for _, branch := range st.Branches {
			b.bindExpr(branch.Cond, scope)
			b.bindBlock(branch.Body, NewScope(scope))
		}
		if st.Else != nil {
			b.bindBlock(st.Else, NewScope(scope))
		}
	case *syntax.NumericForStmt:
		b.bindExpr(st.Init, scope)
		b.bindExpr(st.Limit, scope)
		if st.Step != nil {
			b.bindExpr(st.Step, scope)
		}
		loopScope := NewScope(scope)
		sym := b.table.NewSymbol(SymbolLocal, st.Name.Text, st.Name.Span(), loopScope)
		loopScope.Declare(sym)
		b.addDecl(st.Name, sym, loopScope)
		b.bindBlock(st.Body, NewScope(loopScope))
	case *syntax.GenericForStmt:
		for _, value := range st.Values {
			b.bindExpr(value, scope)
		}
		loopScope := NewScope(scope)
		for _, name := range st.Names {
			sym := b.table.NewSymbol(SymbolLocal, name.Text, name.Span(), loopScope)
			loopScope.Declare(sym)
			b.addDecl(name, sym, loopScope)
		}
		b.bindBlock(st.Body, NewScope(loopScope))
	case *syntax.FunctionDeclStmt:
		b.bindFunctionDecl(st, scope)
	case *syntax.ReturnStmt:
		for _, value := range st.Values {
			b.bindExpr(value, scope)
		}
	case *syntax.GotoStmt:
		b.bindGoto(st, scope)
	case *syntax.LabelStmt:
		// Labels were declared and indexed when the function was entered.
	case *syntax.PrintStmt:
		for _, arg := range st.Args {
			b.bindExpr(arg, scope)
		}
	}
}

func (b *binder) bindLocal(st *syntax.LocalStmt, scope *Scope) {
	symbols := make([]*Symbol, len(st.Names))
	for i, name := range st.Names {
		kind := SymbolLocal
		if valueAt(st.Values, i) != nil {
			if _, ok := valueAt(st.Values, i).(*syntax.FunctionExpr); ok {
				kind = SymbolFunction
			}
		}
		symbols[i] = b.table.NewSymbol(kind, name.Text, name.Span(), scope)
	}

	// Initializers are evaluated before the new locals become visible, so
	// local x = x binds the right-hand x to the outer/global symbol.
	for i, value := range st.Values {
		if value == nil {
			continue
		}
		var target *Symbol
		if i < len(symbols) {
			target = symbols[i]
		}
		if fn, ok := value.(*syntax.FunctionExpr); ok {
			if target != nil {
				setFunctionInfo(target, st.Names[i].Text, fn)
			}
			b.bindFunction(fn, scope, false, nil, source.Span{}, target)
			continue
		}
		if table, ok := value.(*syntax.TableExpr); ok && target != nil {
			b.bindTable(table, target, scope)
			continue
		}
		b.bindExpr(value, scope)
	}

	for i, name := range st.Names {
		scope.Declare(symbols[i])
		b.addDecl(name, symbols[i], scope)
	}
}

func (b *binder) bindAssign(st *syntax.AssignStmt, scope *Scope) {
	targets := make([]*Symbol, len(st.Targets))
	for i, target := range st.Targets {
		targets[i] = b.bindTarget(target, scope)
	}
	for i, value := range st.Values {
		if value == nil {
			continue
		}
		if table, ok := value.(*syntax.TableExpr); ok && i < len(targets) && targets[i] != nil {
			b.bindTable(table, b.valueSymbol(targets[i]), scope)
			continue
		}
		if fn, ok := value.(*syntax.FunctionExpr); ok && i < len(targets) && targets[i] != nil {
			target := targets[i]
			if target.Kind == SymbolGlobal || target.Kind == SymbolLocal || target.Kind == SymbolField {
				target.Kind = SymbolFunction
			}
			setFunctionInfo(target, target.Name, fn)
			b.bindFunction(fn, scope, false, nil, source.Span{}, target)
			continue
		}
		b.bindExpr(value, scope)
	}
}

func valueAt(values []syntax.Expr, i int) syntax.Expr {
	if i >= 0 && i < len(values) {
		return values[i]
	}
	return nil
}

func setFunctionInfo(sym *Symbol, displayName string, fn *syntax.FunctionExpr) {
	if sym == nil || fn == nil || sym.Signature != "" {
		return
	}
	params := make([]string, 0, len(fn.Params)+1)
	for _, param := range fn.Params {
		params = append(params, param.Text)
	}
	if fn.Vararg {
		params = append(params, "...")
	}
	sym.Params = params
	sym.Signature = "function " + displayName + "(" + strings.Join(params, ", ") + ")"
}

func functionDisplayName(names []*syntax.NameExpr, method bool) string {
	if len(names) == 0 {
		return ""
	}
	var b strings.Builder
	for i, name := range names {
		if i > 0 {
			if method && i == len(names)-1 {
				b.WriteByte(':')
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteString(name.Text)
	}
	return b.String()
}

func (b *binder) bindFunctionDecl(st *syntax.FunctionDeclStmt, scope *Scope) {
	if len(st.Names) == 0 {
		b.bindFunction(st.Func, scope, st.Method, nil, source.Span{}, nil)
		return
	}
	if len(st.Names) == 1 {
		name := st.Names[0]
		var sym *Symbol
		if existing := scope.Lookup(CanonicalName(name.Text)); existing != nil {
			// function f() is sugar for f = function(). It updates a visible
			// local rather than creating a global.
			sym = existing
			if sym.Kind == SymbolLocal {
				sym.Kind = SymbolFunction
			}
			b.addUse(name, sym, scope, true)
		} else {
			sym = b.declareGlobal(name, scope)
			sym.Kind = SymbolFunction
		}
		setFunctionInfo(sym, name.Text, st.Func)
		b.bindFunction(st.Func, scope, st.Method, nil, name.Span(), sym)
		return
	}

	receiver := b.resolveName(st.Names[0], scope)
	selfReceiver := receiver
	for i := 1; i < len(st.Names); i++ {
		name := st.Names[i]
		last := i == len(st.Names)-1
		if receiver == nil {
			continue
		}
		if last {
			kind := SymbolFunction
			if st.Method {
				kind = SymbolMethod
			}
			receiver = b.declareField(receiver, name, scope, kind)
			continue
		}
		if existing := fieldOf(receiver, name); existing != nil {
			b.addUse(name, existing, scope, false)
			receiver = existing
		} else {
			receiver = b.declareField(receiver, name, scope, SymbolField)
		}
		if st.Method {
			selfReceiver = receiver
		}
	}
	if receiver != nil {
		setFunctionInfo(receiver, functionDisplayName(st.Names, st.Method), st.Func)
	}
	selfSpan := st.Names[len(st.Names)-1].Span()
	b.bindFunction(st.Func, scope, st.Method, selfReceiver, selfSpan, receiver)
}

func (b *binder) bindFunction(fn *syntax.FunctionExpr, outer *Scope, method bool, receiver *Symbol, selfSpan source.Span, functionSym *Symbol) {
	if fn == nil {
		return
	}
	if functionSym != nil {
		b.result.funcSymbols[fn] = functionSym
	}
	fnScope := NewScope(outer)
	fnScope.Labels = make(map[string]*Symbol)
	if span := fn.Span(); !span.IsEmpty() {
		fnScope.Span = span
		b.result.scopes = append(b.result.scopes, fnScope)
	}

	if method {
		self := b.table.NewSymbol(SymbolParam, "self", selfSpan, fnScope)
		self.DeclSpan = selfSpan
		self.SelectionSpan = selfSpan
		self.Function = functionSym
		self.Decls = nil // implicit; no occurrence at the method name
		fnScope.Declare(self)
		if receiver != nil {
			self.Alias = receiver
			b.selfAliases[self] = receiver
		}
	}
	for _, param := range fn.Params {
		sym := b.table.NewSymbol(SymbolParam, param.Text, param.Span(), fnScope)
		sym.Function = functionSym
		fnScope.Declare(sym)
		b.addDecl(param, sym, fnScope)
	}
	if fn.Vararg {
		span := fn.End
		if span.IsEmpty() {
			span = fn.Span()
		}
		// The vararg symbol is not a normal lexical declaration, but inference
		// needs it to constrain the tail type from call sites.
		sym := b.table.NewSymbol(SymbolParam, "...", span, fnScope)
		sym.Function = functionSym
	}

	labels := &labelScope{labels: fnScope.Labels, function: fnScope}
	b.labels = append(b.labels, labels)
	b.collectLabels(fn.Body, fnScope)
	b.bindBlock(fn.Body, NewScope(fnScope))
	b.labels = b.labels[:len(b.labels)-1]
}

func (b *binder) bindGoto(st *syntax.GotoStmt, scope *Scope) {
	labels := b.currentLabels()
	if labels == nil || st.Name == nil {
		return
	}
	sym := labels.labels[CanonicalName(st.Name.Text)]
	occ := Occurrence{
		Span:       st.Name.Span(),
		Text:       st.Name.Text,
		Scope:      scope,
		LabelScope: labels.function,
	}
	if sym == nil {
		b.result.addOccurrence(occ)
		b.addDiagnostic(CodeUnknownLabel, fmt.Sprintf("unknown label %q", st.Name.Text), st.Name.Span(), SeverityWarning)
		return
	}
	occ.Symbol = sym
	b.result.addOccurrence(occ)
}

func (b *binder) currentLabels() *labelScope {
	if len(b.labels) == 0 {
		return nil
	}
	return b.labels[len(b.labels)-1]
}

func (b *binder) collectLabels(block *syntax.Block, fnScope *Scope) {
	labels := b.currentLabels()
	if labels == nil || block == nil {
		return
	}
	var walk func(*syntax.Block)
	walk = func(block *syntax.Block) {
		if block == nil {
			return
		}
		for _, stmt := range block.Stmts {
			switch st := stmt.(type) {
			case *syntax.LabelStmt:
				if st.Name == nil {
					continue
				}
				canonical := CanonicalName(st.Name.Text)
				sym := labels.labels[canonical]
				if sym == nil {
					sym = b.table.NewSymbol(SymbolLabel, st.Name.Text, st.Name.Span(), fnScope)
					sym.Scope = fnScope
					labels.labels[canonical] = sym
					b.addDecl(st.Name, sym, fnScope)
					continue
				}
				b.addDiagnostic(CodeDuplicateLabel, fmt.Sprintf("duplicate label %q", st.Name.Text), st.Name.Span(), SeverityWarning)
				b.addDecl(st.Name, sym, fnScope)
			case *syntax.DoStmt:
				walk(st.Body)
			case *syntax.WhileStmt:
				walk(st.Body)
			case *syntax.RepeatStmt:
				walk(st.Body)
			case *syntax.IfStmt:
				for _, branch := range st.Branches {
					walk(branch.Body)
				}
				walk(st.Else)
			case *syntax.NumericForStmt:
				walk(st.Body)
			case *syntax.GenericForStmt:
				walk(st.Body)
			}
		}
	}
	walk(block)
}

func (b *binder) bindTarget(target syntax.Expr, scope *Scope) *Symbol {
	switch t := target.(type) {
	case *syntax.NameExpr:
		canonical := CanonicalName(t.Text)
		if sym := scope.Lookup(canonical); sym != nil {
			b.addUse(t, sym, scope, true)
			return sym
		}
		return b.declareGlobal(t, scope)
	case *syntax.FieldExpr:
		receiver := b.valueSymbol(b.bindExpr(t.Base, scope))
		if receiver == nil || t.Name == nil {
			return nil
		}
		if existing := fieldOf(receiver, t.Name); existing != nil {
			if len(existing.Decls) == 0 {
				b.addDecl(t.Name, existing, scope)
			} else {
				b.addUse(t.Name, existing, scope, true)
			}
			return existing
		}
		return b.declareField(receiver, t.Name, scope, SymbolField)
	case *syntax.IndexExpr:
		receiver := b.valueSymbol(b.bindExpr(t.Base, scope))
		if _, ok := t.Index.(*syntax.StringExpr); ok {
			// A string-keyed access is not a dot field: do not navigate into
			// the string literal, and make field rename on this receiver safe
			// by refusing it.
			if receiver != nil && !receiver.Builtin {
				receiver.DynamicFields = true
			}
			return nil
		}
		b.bindExpr(t.Index, scope)
		if receiver != nil && !receiver.Builtin {
			receiver.DynamicFields = true
		}
		return nil
	default:
		b.bindExpr(target, scope)
		return nil
	}
}

func (b *binder) bindExpr(expr syntax.Expr, scope *Scope) *Symbol {
	switch e := expr.(type) {
	case nil:
		return nil
	case *syntax.NameExpr:
		return b.resolveName(e, scope)
	case *syntax.FieldExpr:
		receiver := b.valueSymbol(b.bindExpr(e.Base, scope))
		return b.fieldUse(receiver, e.Name, scope)
	case *syntax.IndexExpr:
		receiver := b.valueSymbol(b.bindExpr(e.Base, scope))
		if _, ok := e.Index.(*syntax.StringExpr); ok {
			if receiver != nil && !receiver.Builtin {
				receiver.DynamicFields = true
			}
			return nil
		}
		b.bindExpr(e.Index, scope)
		if receiver != nil && !receiver.Builtin {
			receiver.DynamicFields = true
		}
		return nil
	case *syntax.CallExpr:
		b.bindExpr(e.Func, scope)
		for _, arg := range e.Args {
			b.bindExpr(arg, scope)
		}
		return nil
	case *syntax.MethodCallExpr:
		receiver := b.valueSymbol(b.bindExpr(e.Receiver, scope))
		b.fieldUse(receiver, e.Method, scope)
		for _, arg := range e.Args {
			b.bindExpr(arg, scope)
		}
		return nil
	case *syntax.UnaryExpr:
		b.bindExpr(e.X, scope)
		return nil
	case *syntax.BinaryExpr:
		b.bindExpr(e.Left, scope)
		b.bindExpr(e.Right, scope)
		return nil
	case *syntax.FunctionExpr:
		b.bindFunction(e, scope, false, nil, source.Span{}, nil)
		return nil
	case *syntax.TableExpr:
		b.bindTable(e, nil, scope)
		return nil
	case *syntax.ParenExpr:
		return b.bindExpr(e.X, scope)
	default:
		return nil
	}
}

func (b *binder) bindTable(table *syntax.TableExpr, receiver *Symbol, scope *Scope) {
	if table == nil {
		return
	}
	for _, field := range table.Fields {
		var fieldSym *Symbol
		if !field.BracketKey {
			if key, ok := field.Key.(*syntax.NameExpr); ok {
				if receiver != nil {
					fieldSym = b.declareField(receiver, key, scope, SymbolField)
				}
			} else {
				b.bindExpr(field.Key, scope)
			}
		} else if _, ok := field.Key.(*syntax.StringExpr); ok {
			if receiver != nil && !receiver.Builtin {
				receiver.DynamicFields = true
			}
		} else {
			if receiver != nil && !receiver.Builtin {
				receiver.DynamicFields = true
			}
			b.bindExpr(field.Key, scope)
		}

		if fieldSym != nil {
			if fn, ok := field.Value.(*syntax.FunctionExpr); ok {
				setFunctionInfo(fieldSym, fieldSym.Name, fn)
				b.bindFunction(fn, scope, false, nil, source.Span{}, fieldSym)
				continue
			}
		}
		if nested, ok := field.Value.(*syntax.TableExpr); ok && fieldSym != nil {
			b.bindTable(nested, fieldSym, scope)
			continue
		}
		b.bindExpr(field.Value, scope)
	}
}

func (b *binder) resolveName(name *syntax.NameExpr, scope *Scope) *Symbol {
	if name == nil {
		return nil
	}
	canonical := CanonicalName(name.Text)
	if sym := scope.Lookup(canonical); sym != nil {
		b.addUse(name, sym, scope, false)
		return sym
	}
	if sym := b.table.Globals[canonical]; sym != nil {
		b.addUse(name, sym, scope, false)
		return sym
	}
	if sym := b.table.Builtins[canonical]; sym != nil {
		b.addUse(name, sym, scope, false)
		return sym
	}
	b.addUnresolved(name, scope)
	return nil
}

func (b *binder) declareGlobal(name *syntax.NameExpr, scope *Scope) *Symbol {
	canonical := CanonicalName(name.Text)
	sym := b.table.Globals[canonical]
	if sym == nil {
		sym = b.table.NewSymbol(SymbolGlobal, name.Text, name.Span(), scope)
		b.table.Globals[canonical] = sym
	}
	b.addDecl(name, sym, scope)
	return sym
}

func (b *binder) declareField(receiver *Symbol, name *syntax.NameExpr, scope *Scope, kind SymbolKind) *Symbol {
	if receiver == nil || name == nil {
		return nil
	}
	return b.declareFieldAt(receiver, name.Text, name.Span(), scope, kind)
}

func (b *binder) declareFieldAt(receiver *Symbol, name string, span source.Span, scope *Scope, kind SymbolKind) *Symbol {
	if receiver == nil || receiver.Builtin {
		// Built-in symbols are shared by every cart and must stay immutable.
		return nil
	}
	if receiver.Fields == nil {
		receiver.Fields = make(map[string]*Symbol)
	}
	canonical := CanonicalName(name)
	sym := receiver.Fields[canonical]
	if sym == nil {
		sym = b.table.NewSymbol(kind, name, span, scope)
		sym.Owner = receiver
		receiver.Fields[canonical] = sym
		b.addDeclAt(name, span, sym, scope)
		return sym
	}
	if kind != SymbolField {
		sym.Kind = kind
	}
	b.addDeclAt(name, span, sym, scope)
	return sym
}

func fieldOf(receiver *Symbol, name *syntax.NameExpr) *Symbol {
	if receiver == nil || name == nil || receiver.Fields == nil {
		return nil
	}
	return receiver.Fields[CanonicalName(name.Text)]
}

// fieldUse resolves a dot/method access. If the field has not been declared
// yet, it creates a provisional field symbol so a later declaration can merge
// with uses that appeared earlier in the cart. Provisional fields have no
// declarations and therefore cannot be renamed on their own.
func (b *binder) fieldUse(receiver *Symbol, name *syntax.NameExpr, scope *Scope) *Symbol {
	if receiver == nil || name == nil || receiver.Builtin {
		return nil
	}
	if receiver.Fields == nil {
		receiver.Fields = make(map[string]*Symbol)
	}
	canonical := CanonicalName(name.Text)
	field := receiver.Fields[canonical]
	if field == nil {
		field = b.table.NewSymbol(SymbolField, name.Text, name.Span(), scope)
		field.Owner = receiver
		receiver.Fields[canonical] = field
	}
	b.addUse(name, field, scope, false)
	return field
}

func (b *binder) addDecl(name *syntax.NameExpr, sym *Symbol, scope *Scope) {
	if name == nil {
		return
	}
	b.addDeclAt(name.Text, name.Span(), sym, scope)
}

func (b *binder) addDeclAt(text string, span source.Span, sym *Symbol, scope *Scope) {
	if sym == nil {
		return
	}
	b.result.addOccurrence(Occurrence{
		Span:   span,
		Text:   text,
		Symbol: sym,
		Scope:  scope,
		Write:  true,
		Decl:   true,
	})
}

func (b *binder) addUse(name *syntax.NameExpr, sym *Symbol, scope *Scope, write bool) {
	if name == nil {
		return
	}
	b.addUseAt(name.Text, name.Span(), sym, scope, write)
}

func (b *binder) addUseAt(text string, span source.Span, sym *Symbol, scope *Scope, write bool) {
	if sym == nil {
		return
	}
	b.result.addOccurrence(Occurrence{
		Span:   span,
		Text:   text,
		Symbol: sym,
		Scope:  scope,
		Write:  write,
	})
}

func (b *binder) addUnresolved(name *syntax.NameExpr, scope *Scope) {
	if name == nil {
		return
	}
	b.result.addOccurrence(Occurrence{
		Span:  name.Span(),
		Text:  name.Text,
		Scope: scope,
	})
	b.addDiagnostic(
		CodeUndefinedGlobal,
		fmt.Sprintf("undefined global %q", name.Text),
		name.Span(),
		b.opts.UndefinedGlobal,
	)
}

func (b *binder) addDiagnostic(code, message string, span source.Span, severity Severity) {
	if severity == SeverityOff {
		return
	}
	if code == CodeUndefinedGlobal && b.result.meta[span.FileID] {
		return
	}
	b.result.Diagnostics = append(b.result.Diagnostics, Diagnostic{
		Code:     code,
		Message:  message,
		Span:     span,
		Severity: severity,
	})
}
