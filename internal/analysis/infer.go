package analysis

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

const (
	maxInferencePasses = 4
	maxUnionMembers    = 8
)

type functionState struct {
	symbol         *Symbol
	fn             *types.Function
	returns        []types.Type
	explicitReturn bool
}

type inferer struct {
	snap    *cart.Snapshot
	result  *Result
	mode    TypeCheckingMode
	final   bool
	funcs   []*functionState
	param   map[*Symbol]types.Type
	calls   map[*Symbol]map[int][]types.Type
	visited map[string]bool
}

func inferTypes(snap *cart.Snapshot, result *Result, opts Options) {
	mode := opts.typeCheckingMode()
	if snap == nil || snap.Cart == nil || result == nil || mode == TypeCheckingOff {
		return
	}
	inf := &inferer{
		snap:   snap,
		result: result,
		mode:   mode,
		param:  make(map[*Symbol]types.Type),
	}
	// Seed explicit parameter types so the first pass sees them.
	for _, sym := range result.Table.Symbols {
		if sym != nil && sym.Kind == SymbolParam && sym.Type != nil {
			inf.param[sym] = sym.Type
		}
	}
	for pass := 0; pass < maxInferencePasses; pass++ {
		inf.beginPass()
		inf.inferBlock(snap.Chunk.Block, make(map[*Symbol]types.Type))
		if !inf.applyCallSites() && pass >= 1 {
			break
		}
	}
	inf.final = true
	inf.beginPass()
	inf.inferBlock(snap.Chunk.Block, make(map[*Symbol]types.Type))
	inf.applyCallSites()
	inf.emitDeprecated()
}

func (inf *inferer) beginPass() {
	inf.calls = make(map[*Symbol]map[int][]types.Type)
	inf.visited = make(map[string]bool)
	for _, sym := range inf.result.Table.Symbols {
		if sym == nil || sym.Builtin || sym.TypeExplicit {
			continue
		}
		if sym.Kind == SymbolFunction || sym.Kind == SymbolMethod {
			// Keep the previous pass's function signature so calls that
			// precede the declaration can converge across passes.
			continue
		}
		if sym.Kind == SymbolParam {
			if paramType, ok := inf.param[sym]; ok {
				sym.Type = paramType
			} else {
				sym.Type = nil
			}
			continue
		}
		sym.Type = nil
	}
}

func (inf *inferer) applyCallSites() bool {
	changed := false
	for callee, argsByIndex := range inf.calls {
		if callee == nil || callee.Builtin {
			continue
		}
		paramSymbols := inf.paramSymbols(callee)
		for index, paramSymbol := range paramSymbols {
			if paramSymbol == nil || paramSymbol.TypeExplicit {
				continue
			}
			var observed []types.Type
			if strings.EqualFold(paramSymbol.Name, "...") {
				for argIndex, argTypes := range argsByIndex {
					if argIndex >= index {
						observed = append(observed, argTypes...)
					}
				}
			} else {
				observed = argsByIndex[index]
			}
			if len(observed) == 0 {
				continue
			}
			var merged types.Type
			for _, observedType := range observed {
				merged = types.Merge(merged, types.Widen(types.First(observedType)))
			}
			if union, ok := merged.(types.Union); ok && len(union.Members) > maxUnionMembers {
				merged = types.AnyType
			}
			merged = types.UnionOf(splitUnion(merged)...)
			if types.Display(merged) == types.Display(paramSymbol.Type) {
				continue
			}
			inf.param[paramSymbol] = merged
			if !inf.final {
				paramSymbol.Type = merged
			}
			changed = true
		}
	}
	return changed
}

func splitUnion(t types.Type) []types.Type {
	if t == nil {
		return []types.Type{types.AnyType}
	}
	if union, ok := t.(types.Union); ok {
		return union.Members
	}
	return []types.Type{t}
}

func (inf *inferer) paramSymbols(callee *Symbol) []*Symbol {
	if callee == nil {
		return nil
	}
	byName := make(map[string]*Symbol)
	for _, sym := range inf.result.Table.Symbols {
		if sym != nil && sym.Kind == SymbolParam && sym.Function == callee {
			byName[CanonicalName(sym.Name)] = sym
		}
	}
	out := make([]*Symbol, 0, len(callee.Params))
	for _, name := range callee.Params {
		out = append(out, byName[CanonicalName(name)])
	}
	return out
}

func (inf *inferer) inferBlock(block *syntax.Block, env map[*Symbol]types.Type) {
	if block == nil {
		return
	}
	for _, stmt := range block.Stmts {
		inf.inferStmt(stmt, env)
	}
}

func (inf *inferer) inferStmt(stmt syntax.Stmt, env map[*Symbol]types.Type) {
	if stmt == nil {
		return
	}
	switch st := stmt.(type) {
	case *syntax.LocalStmt:
		inf.inferLocal(st, env)
	case *syntax.LocalFunctionStmt:
		inf.inferFunction(st.Func, env)
	case *syntax.AssignStmt:
		inf.inferAssign(st, env)
	case *syntax.CallStmt:
		if st.Call != nil {
			inf.inferExpr(st.Call, env)
		}
	case *syntax.DoStmt:
		inf.inferBlock(st.Body, cloneEnv(env))
	case *syntax.WhileStmt:
		inf.inferExpr(st.Cond, env)
		inf.inferBlock(st.Body, cloneEnv(env))
	case *syntax.RepeatStmt:
		bodyEnv := cloneEnv(env)
		inf.inferBlock(st.Body, bodyEnv)
		inf.inferExpr(st.Cond, bodyEnv)
	case *syntax.IfStmt:
		inf.inferIf(st, env)
	case *syntax.NumericForStmt:
		inf.inferExpr(st.Init, env)
		inf.inferExpr(st.Limit, env)
		if st.Step != nil {
			inf.inferExpr(st.Step, env)
		}
		loopEnv := cloneEnv(env)
		if sym := inf.symbolAt(st.Name); sym != nil {
			sym.Type = types.NumberType
			loopEnv[sym] = types.NumberType
		}
		inf.inferBlock(st.Body, loopEnv)
	case *syntax.GenericForStmt:
		inf.inferGenericFor(st, env)
	case *syntax.FunctionDeclStmt:
		inf.inferFunction(st.Func, env)
	case *syntax.ReturnStmt:
		inf.inferReturn(st, env)
	case *syntax.PrintStmt:
		for _, arg := range st.Args {
			inf.inferExpr(arg, env)
		}
	}
}

func (inf *inferer) inferLocal(st *syntax.LocalStmt, env map[*Symbol]types.Type) {
	valueTypes := make([]types.Type, len(st.Values))
	for i, value := range st.Values {
		valueTypes[i] = inf.inferExpr(value, env)
	}
	for i, name := range st.Names {
		sym := inf.symbolAt(name)
		if sym == nil {
			continue
		}
		valueType := multiValueAt(valueTypes, i)
		if sym.Doc != nil && sym.Doc.Enum != "" {
			if enum := inf.result.Enums[CanonicalName(sym.Doc.Enum)]; enum != nil {
				if table, ok := valueAtExpr(st.Values, i).(*syntax.TableExpr); ok {
					inf.fillEnum(enum, table, env)
				}
				valueType = enum
			}
		}
		declared := sym.Type
		if sym.TypeExplicit && declared != nil {
			if i < len(st.Values) {
				inf.checkAssignment(sym, declared, valueType, spanOfExpr(valueAtExpr(st.Values, i)))
				inf.attachTableFields(sym, valueAtExpr(st.Values, i), declared)
			}
			env[sym] = declared
			continue
		}
		valueType = types.Widen(valueType)
		sym.Type = valueType
		sym.TypeExplicit = false
		env[sym] = valueType
		inf.attachTableFields(sym, valueAtExpr(st.Values, i), valueType)
	}
}

func (inf *inferer) inferAssign(st *syntax.AssignStmt, env map[*Symbol]types.Type) {
	valueTypes := make([]types.Type, len(st.Values))
	for i, value := range st.Values {
		valueTypes[i] = inf.inferExpr(value, env)
	}
	if st.Op.Text != "=" && st.Op.Text != "" {
		for i, target := range st.Targets {
			targetType := inf.targetType(target, env)
			var valueType types.Type = types.AnyType
			if i < len(valueTypes) {
				valueType = firstType(valueTypes[i])
			}
			resultType := inf.binaryResultType(strings.TrimSuffix(st.Op.Text, "="), targetType, valueType)
			inf.assignTarget(target, resultType, env, spanOfExpr(target))
		}
		return
	}
	for i, target := range st.Targets {
		valueType := multiValueAt(valueTypes, i)
		if name, ok := target.(*syntax.NameExpr); ok {
			if sym := inf.symbolAt(name); sym != nil && sym.Doc != nil && sym.Doc.Enum != "" {
				if enum := inf.result.Enums[CanonicalName(sym.Doc.Enum)]; enum != nil {
					if table, ok := valueAtExpr(st.Values, i).(*syntax.TableExpr); ok {
						inf.fillEnum(enum, table, env)
					}
					valueType = enum
				}
			}
		}
		inf.assignTarget(target, valueType, env, spanOfExpr(valueAtExpr(st.Values, i)))
	}
}

func (inf *inferer) assignTarget(target syntax.Expr, valueType types.Type, env map[*Symbol]types.Type, valueSpan source.Span) {
	if target == nil {
		return
	}
	switch t := target.(type) {
	case *syntax.NameExpr:
		sym := inf.symbolAt(t)
		if sym == nil {
			return
		}
		declared := sym.Type
		if sym.TypeExplicit && declared != nil {
			declared = inf.applyCasts(sym, declared, t.Span())
			inf.checkAssignment(sym, declared, valueType, valueSpan)
			env[sym] = declared
			return
		}
		valueType = types.Widen(valueType)
		sym.Type = types.Merge(sym.Type, valueType)
		env[sym] = valueType
	case *syntax.FieldExpr:
		baseType := inf.inferExpr(t.Base, env)
		fieldSym := inf.fieldSymbol(t)
		fieldType := valueType
		if fieldSym != nil && fieldSym.TypeExplicit && fieldSym.Type != nil {
			inf.checkAssignment(fieldSym, fieldSym.Type, valueType, valueSpan)
			fieldType = fieldSym.Type
		}
		if fieldSym != nil {
			fieldSym.Type = types.Merge(fieldSym.Type, types.Widen(fieldType))
			fieldSym.TypeExplicit = fieldSym.TypeExplicit && fieldSym.Type != nil
		}
		inf.addRecordField(baseType, t.Name.Text, types.Widen(fieldType), false)
	case *syntax.IndexExpr:
		inf.inferExpr(t.Index, env)
		baseType := inf.inferExpr(t.Base, env)
		switch collection := types.Resolve(baseType).(type) {
		case types.Array:
			inf.checkAssignmentValue(collection.Elem, valueType, valueSpan)
		case types.Map:
			inf.checkAssignmentValue(collection.Elem, valueType, valueSpan)
		}
	}
}

func (inf *inferer) targetType(target syntax.Expr, env map[*Symbol]types.Type) types.Type {
	switch t := target.(type) {
	case *syntax.NameExpr:
		if sym := inf.symbolAt(t); sym != nil {
			return inf.symbolType(sym, t.Span(), env)
		}
	case *syntax.FieldExpr:
		return inf.inferExpr(t, env)
	}
	return types.AnyType
}

func (inf *inferer) inferIf(st *syntax.IfStmt, env map[*Symbol]types.Type) {
	if st == nil {
		return
	}
	base := env
	for i, branch := range st.Branches {
		condType := inf.inferExpr(branch.Cond, base)
		branchEnv := cloneEnv(base)
		inf.applyNarrow(branchEnv, branch.Cond, true)
		inf.inferBlock(branch.Body, branchEnv)
		if i == 0 && len(st.Branches) == 1 && st.Else == nil && blockAlwaysReturns(branch.Body) {
			elseEnv := cloneEnv(base)
			inf.applyNarrow(elseEnv, branch.Cond, false)
			mergeEnv(env, elseEnv)
		}
		_ = condType
	}
	if st.Else != nil {
		elseEnv := cloneEnv(base)
		for _, branch := range st.Branches {
			inf.applyNarrow(elseEnv, branch.Cond, false)
		}
		inf.inferBlock(st.Else, elseEnv)
	}
}

func (inf *inferer) inferGenericFor(st *syntax.GenericForStmt, env map[*Symbol]types.Type) {
	var iterType types.Type = types.AnyType
	if len(st.Values) > 0 {
		iterType = firstType(inf.inferExpr(st.Values[0], env))
	}
	name := ""
	if len(st.Values) > 0 {
		name = callName(st.Values[0])
	}
	loopTypes := iteratorLoopTypes(name, iterType)
	loopEnv := cloneEnv(env)
	for i, nameExpr := range st.Names {
		sym := inf.symbolAt(nameExpr)
		if sym == nil {
			continue
		}
		var loopType types.Type = types.AnyType
		if i < len(loopTypes) {
			loopType = loopTypes[i]
		}
		sym.Type = loopType
		loopEnv[sym] = loopType
	}
	inf.inferBlock(st.Body, loopEnv)
}

func (inf *inferer) inferReturn(st *syntax.ReturnStmt, env map[*Symbol]types.Type) {
	state := inf.currentFunction()
	if state == nil {
		for _, value := range st.Values {
			inf.inferExpr(value, env)
		}
		return
	}
	valueTypes := make([]types.Type, 0, len(st.Values))
	for _, value := range st.Values {
		valueTypes = append(valueTypes, inf.inferExpr(value, env))
	}
	if len(valueTypes) > 0 {
		if tuple, ok := valueTypes[len(valueTypes)-1].(types.Tuple); ok {
			valueTypes = append(valueTypes[:len(valueTypes)-1], tuple.Elems...)
		}
	}
	if inf.final && state.explicitReturn && state.fn != nil {
		for i, valueType := range valueTypes {
			if i >= len(state.fn.Returns.Elems) {
				break
			}
			if !types.Assignable(valueType, state.fn.Returns.Elems[i], inf.mode == TypeCheckingStrict) {
				span := source.Span{}
				if i < len(st.Values) {
					span = st.Values[i].Span()
				} else if len(st.Values) > 0 {
					span = st.Values[len(st.Values)-1].Span()
				}
				inf.report(CodeReturnTypeMismatch,
					fmt.Sprintf("cannot return %s, want %s", types.Display(valueType), types.Display(state.fn.Returns.Elems[i])),
					span)
			}
		}
	}
	if !state.explicitReturn {
		for i, valueType := range valueTypes {
			valueType = types.Widen(valueType)
			if i < len(state.returns) {
				state.returns[i] = types.Merge(state.returns[i], valueType)
			} else {
				state.returns = append(state.returns, valueType)
			}
		}
	}
}

func (inf *inferer) inferFunction(fn *syntax.FunctionExpr, env map[*Symbol]types.Type) types.Type {
	if fn == nil {
		return types.AnyType
	}
	functionSym := inf.result.funcSymbols[fn]
	fnType := &types.Function{}
	explicitReturns := false
	explicit := false
	if functionSym == nil {
		for i, param := range fn.Params {
			fnType.Params = setParamAt(fnType.Params, i, types.Param{Name: param.Text, Type: types.AnyType})
		}
		if fn.Vararg {
			fnType.Vararg = types.AnyType
		}
	}
	if functionSym != nil {
		if existing, ok := types.AsFunction(functionSym.Type); ok {
			fnType = existing
			explicit = functionSym.TypeExplicit
			explicitReturns = functionHasExplicitReturns(functionSym)
		}
		paramSymbols := inf.paramSymbols(functionSym)
		for i, param := range fn.Params {
			var paramSym *Symbol
			if i < len(paramSymbols) {
				paramSym = paramSymbols[i]
			}
			var paramType types.Type = types.AnyType
			if paramSym != nil && paramSym.Type != nil {
				paramType = paramSym.Type
			} else if i < len(fnType.Params) {
				paramType = fnType.Params[i].Type
			}
			optional := i < len(fnType.Params) && fnType.Params[i].Optional
			if optional && types.ContainsNil(paramType) {
				paramType = types.RemoveNil(paramType)
			}
			fnType.Params = setParamAt(fnType.Params, i, types.Param{
				Name:     param.Text,
				Type:     paramType,
				Optional: optional,
				Explicit: paramSym != nil && paramSym.TypeExplicit,
			})
		}
		if fn.Vararg {
			var vararg types.Type = types.AnyType
			if len(paramSymbols) > len(fn.Params) && paramSymbols[len(fn.Params)] != nil && paramSymbols[len(fn.Params)].Type != nil {
				vararg = paramSymbols[len(fn.Params)].Type
			}
			fnType.Vararg = vararg
		}
		if self := inf.selfSymbol(functionSym); self != nil {
			if owner := functionSym.Owner; owner != nil && owner.Type != nil {
				self.Type = owner.Type
			}
		}
	}
	if !explicitReturns {
		fnType.Returns = types.Tuple{}
	}
	state := &functionState{symbol: functionSym, fn: fnType, explicitReturn: explicitReturns}
	inf.funcs = append(inf.funcs, state)
	fnEnv := cloneEnv(env)
	if functionSym != nil {
		for _, paramSym := range inf.paramSymbols(functionSym) {
			if paramSym != nil && paramSym.Type != nil {
				fnEnv[paramSym] = paramSym.Type
			}
		}
	}
	inf.inferBlock(fn.Body, fnEnv)
	inf.funcs = inf.funcs[:len(inf.funcs)-1]
	if functionSym != nil {
		if !explicitReturns {
			fnType.Returns = types.TupleOf(state.returns...)
		}
		functionSym.Type = fnType
		functionSym.TypeExplicit = explicit
		setParamSymbolTypes(inf.result, functionSym, fnType)
	}
	return fnType
}

func functionHasExplicitReturns(sym *Symbol) bool {
	if sym == nil || sym.Doc == nil {
		return false
	}
	if len(sym.Doc.ParsedReturns) > 0 {
		return true
	}
	if fn, ok := types.AsFunction(sym.Doc.ParsedType); ok && len(fn.Returns.Elems) > 0 {
		return true
	}
	return false
}

func (inf *inferer) selfSymbol(function *Symbol) *Symbol {
	if function == nil {
		return nil
	}
	for _, sym := range inf.result.Table.Symbols {
		if sym != nil && sym.Kind == SymbolParam && sym.Function == function && strings.EqualFold(sym.Name, "self") {
			return sym
		}
	}
	return nil
}

func (inf *inferer) currentFunction() *functionState {
	if len(inf.funcs) == 0 {
		return nil
	}
	return inf.funcs[len(inf.funcs)-1]
}

func (inf *inferer) inferExpr(expr syntax.Expr, env map[*Symbol]types.Type) types.Type {
	if expr == nil {
		return types.AnyType
	}
	var typ types.Type
	switch e := expr.(type) {
	case *syntax.NilExpr:
		typ = types.NilType
	case *syntax.BoolExpr:
		typ = types.Literal{Value: e.Value}
	case *syntax.NumberExpr:
		if value, err := strconv.ParseFloat(e.Text, 64); err == nil {
			typ = types.Literal{Value: value}
		} else {
			typ = types.NumberType
		}
	case *syntax.StringExpr:
		typ = types.Literal{Value: e.Value}
	case *syntax.VarargExpr:
		typ = types.AnyType
		if state := inf.currentFunction(); state != nil && state.fn != nil && state.fn.Vararg != nil {
			typ = state.fn.Vararg
		}
	case *syntax.NameExpr:
		if sym := inf.symbolAt(e); sym != nil {
			typ = inf.symbolType(sym, e.Span(), env)
			if inf.final && !inf.result.meta[e.Span().FileID] && sym.Doc != nil && sym.Doc.Deprecated {
				occ := inf.result.OccurrenceAt(e.Span().FileID, e.Span().Start)
				if occ == nil || !occ.Decl {
					inf.report(CodeDeprecated, fmt.Sprintf("%s is deprecated", sym.Name), e.Span())
				}
			}
		} else {
			typ = types.AnyType
		}
	case *syntax.UnaryExpr:
		inf.inferExpr(e.X, env)
		if e.Op.Text == "not" {
			typ = types.BooleanType
		} else {
			typ = types.NumberType
		}
	case *syntax.BinaryExpr:
		typ = inf.inferBinary(e, env)
	case *syntax.TableExpr:
		typ = inf.inferTable(e, env)
	case *syntax.FunctionExpr:
		typ = inf.inferFunction(e, env)
	case *syntax.CallExpr:
		typ = inf.inferCall(e, env)
	case *syntax.MethodCallExpr:
		typ = inf.inferMethodCall(e, env)
	case *syntax.FieldExpr:
		typ = inf.inferField(e, env)
	case *syntax.IndexExpr:
		typ = inf.inferIndex(e, env)
	case *syntax.ParenExpr:
		typ = inf.inferExpr(e.X, env)
	default:
		typ = types.AnyType
	}
	return inf.applyAs(expr, typ)
}

func (inf *inferer) inferBinary(e *syntax.BinaryExpr, env map[*Symbol]types.Type) types.Type {
	left := firstType(inf.inferExpr(e.Left, env))
	switch e.Op.Text {
	case "and":
		rightEnv := cloneEnv(env)
		inf.applyNarrow(rightEnv, e.Left, true)
		right := inf.inferExpr(e.Right, rightEnv)
		return types.UnionOf(types.Truthy(left), right)
	case "or":
		rightEnv := cloneEnv(env)
		inf.applyNarrow(rightEnv, e.Left, false)
		right := inf.inferExpr(e.Right, rightEnv)
		return types.UnionOf(types.RemoveNil(types.Truthy(left)), right)
	}
	right := firstType(inf.inferExpr(e.Right, env))
	if inf.final && inf.mode == TypeCheckingStrict && requiresNonNil(e.Op.Text) {
		if types.ContainsNil(left) && !types.IsAny(left) {
			inf.report(CodeNeedCheckNil, fmt.Sprintf("%s may be nil", types.Display(left)), e.Left.Span())
		}
		if types.ContainsNil(right) && !types.IsAny(right) {
			inf.report(CodeNeedCheckNil, fmt.Sprintf("%s may be nil", types.Display(right)), e.Right.Span())
		}
	}
	return inf.binaryResultType(e.Op.Text, left, right)
}

func requiresNonNil(op string) bool {
	switch op {
	case "+", "-", "*", "/", "\\", "%", "^", "&", "|", "^^", "<<", ">>", ">>>", "<<>", ">><", "..":
		return true
	}
	return false
}

func (inf *inferer) binaryResultType(op string, left, right types.Type) types.Type {
	switch op {
	case "..":
		return types.StringType
	case "+", "-", "*", "/", "\\", "%", "^", "&", "|", "^^", "<<", ">>", ">>>", "<<>", ">><":
		return types.NumberType
	case "==", "~=", "!=", "<", "<=", ">", ">=":
		return types.BooleanType
	case "and", "or":
		return types.UnionOf(left, right)
	default:
		return types.AnyType
	}
}

func (inf *inferer) inferTable(e *syntax.TableExpr, env map[*Symbol]types.Type) types.Type {
	record := types.NewRecord("")
	var arrayElems []types.Type
	hasArray := false
	for _, field := range e.Fields {
		valueType := inf.inferExpr(field.Value, env)
		if field.Key == nil {
			hasArray = true
			arrayElems = append(arrayElems, types.Widen(firstType(valueType)))
			continue
		}
		if name, ok := field.Key.(*syntax.NameExpr); ok && !field.BracketKey {
			record.AddField(&types.Field{Name: name.Text, Type: types.Widen(firstType(valueType)), Decl: name.Span()})
			if sym := inf.fieldSymbolByName(e, name); sym != nil && !sym.TypeExplicit {
				sym.Type = types.Widen(firstType(valueType))
			}
			continue
		}
		if str, ok := field.Key.(*syntax.StringExpr); ok && field.BracketKey {
			record.AddField(&types.Field{Name: str.Value, Type: types.Widen(firstType(valueType)), Decl: str.Span()})
			continue
		}
		inf.inferExpr(field.Key, env)
	}
	if hasArray && len(record.Fields) == 0 {
		return types.Array{Elem: types.UnionOf(arrayElems...)}
	}
	return record
}

func (inf *inferer) inferField(e *syntax.FieldExpr, env map[*Symbol]types.Type) types.Type {
	base := firstType(inf.inferExpr(e.Base, env))
	if inf.final && inf.mode == TypeCheckingStrict && types.ContainsNil(base) && !types.IsAny(base) {
		inf.report(CodeNeedCheckNil, fmt.Sprintf("%s may be nil", types.Display(base)), e.Base.Span())
	}
	fieldType, ok, optional := types.LookupField(base, e.Name.Text)
	if sym := inf.fieldSymbol(e); sym != nil {
		if sym.Type != nil {
			fieldType = sym.Type
			ok = true
			optional = sym.Doc != nil && sym.Doc.Optional
		} else if ok {
			if optional {
				sym.Type = types.Optional(fieldType)
			} else {
				sym.Type = fieldType
			}
		}
	}
	if !ok {
		if inf.final && !types.IsAny(base) {
			record, isRecord := types.Resolve(base).(*types.Record)
			if isRecord && (inf.mode == TypeCheckingStrict || record.Exact) {
				inf.report(CodeUndefinedField, fmt.Sprintf("field %q does not exist on %s", e.Name.Text, types.Display(base)), e.Name.Span())
			}
		}
		return types.AnyType
	}
	if optional {
		return types.Optional(fieldType)
	}
	return fieldType
}

func (inf *inferer) inferIndex(e *syntax.IndexExpr, env map[*Symbol]types.Type) types.Type {
	base := firstType(inf.inferExpr(e.Base, env))
	if inf.final && inf.mode == TypeCheckingStrict && types.ContainsNil(base) && !types.IsAny(base) {
		inf.report(CodeNeedCheckNil, fmt.Sprintf("%s may be nil", types.Display(base)), e.Base.Span())
	}
	switch value := types.Resolve(base).(type) {
	case types.Array:
		return value.Elem
	case types.Map:
		return value.Elem
	case *types.Record:
		if str, ok := e.Index.(*syntax.StringExpr); ok {
			if fieldType, ok, optional := types.LookupField(value, str.Value); ok {
				if optional {
					return types.Optional(fieldType)
				}
				return fieldType
			}
		}
	}
	return types.AnyType
}

func (inf *inferer) inferCall(e *syntax.CallExpr, env map[*Symbol]types.Type) types.Type {
	callee := inf.calleeSymbol(e.Func)
	argTypes := make([]types.Type, len(e.Args))
	for i, arg := range e.Args {
		argTypes[i] = inf.inferExpr(arg, env)
	}
	name := callName(e.Func)
	if callee != nil {
		inf.recordCall(callee, argTypes)
	}
	if callee == nil || callee.Builtin {
		if typ, ok := inf.specialCall(name, e.Args, argTypes, env); ok {
			return typ
		}
	}
	if callee != nil {
		inf.checkCall(callee, argTypes, e.Args)
		if fn, ok := types.AsFunction(callee.Type); ok {
			return callReturnType(fn)
		}
	}
	return types.AnyType
}

func (inf *inferer) inferMethodCall(e *syntax.MethodCallExpr, env map[*Symbol]types.Type) types.Type {
	receiver := firstType(inf.inferExpr(e.Receiver, env))
	if inf.final && inf.mode == TypeCheckingStrict && types.ContainsNil(receiver) && !types.IsAny(receiver) {
		inf.report(CodeNeedCheckNil, fmt.Sprintf("%s may be nil", types.Display(receiver)), e.Receiver.Span())
	}
	var method *Symbol
	if e.Method != nil {
		method = inf.symbolAt(e.Method)
	}
	argTypes := make([]types.Type, len(e.Args))
	for i, arg := range e.Args {
		argTypes[i] = inf.inferExpr(arg, env)
	}
	if method != nil {
		inf.recordCall(method, argTypes)
		inf.checkCall(method, argTypes, e.Args)
		if fn, ok := types.AsFunction(method.Type); ok {
			return callReturnType(fn)
		}
		if fieldType, ok, _ := types.LookupField(receiver, e.Method.Text); ok {
			if fn, ok := types.AsFunction(fieldType); ok {
				if method.Type == nil {
					method.Type = fn
				}
				return callReturnType(fn)
			}
		}
	}
	return types.AnyType
}

func (inf *inferer) calleeSymbol(expr syntax.Expr) *Symbol {
	switch e := expr.(type) {
	case *syntax.NameExpr:
		return inf.symbolAt(e)
	case *syntax.FieldExpr:
		return inf.fieldSymbol(e)
	case *syntax.ParenExpr:
		return inf.calleeSymbol(e.X)
	}
	return nil
}

func (inf *inferer) recordCall(callee *Symbol, args []types.Type) {
	if callee == nil || callee.Builtin {
		return
	}
	if inf.calls[callee] == nil {
		inf.calls[callee] = make(map[int][]types.Type)
	}
	for i, argType := range args {
		inf.calls[callee][i] = append(inf.calls[callee][i], firstType(argType))
	}
}

func (inf *inferer) checkCall(callee *Symbol, args []types.Type, argExprs []syntax.Expr) {
	if !inf.final || callee == nil || callee.Builtin || callee.TypeExplicit == false {
		return
	}
	fn, ok := types.AsFunction(callee.Type)
	if !ok {
		return
	}
	for i, argType := range args {
		if i >= len(fn.Params) {
			if fn.Vararg != nil {
				continue
			}
			continue
		}
		param := fn.Params[i]
		if !param.Explicit {
			continue
		}
		paramType := param.Type
		if param.Optional {
			paramType = types.Optional(paramType)
		}
		if types.IsAny(argType) || types.IsAny(paramType) {
			continue
		}
		if !types.Assignable(firstType(argType), paramType, inf.mode == TypeCheckingStrict) {
			span := source.Span{}
			if i < len(argExprs) {
				span = argExprs[i].Span()
			}
			inf.report(CodeParamTypeMismatch,
				fmt.Sprintf("argument %d is %s, want %s", i+1, types.Display(firstType(argType)), types.Display(paramType)),
				span)
		}
	}
}

func (inf *inferer) specialCall(name string, args []syntax.Expr, argTypes []types.Type, env map[*Symbol]types.Type) (types.Type, bool) {
	switch name {
	case "all", "ipairs":
		if len(argTypes) > 0 {
			return types.AnyType, true
		}
	case "pairs":
		return types.AnyType, true
	case "count":
		return types.NumberType, true
	case "deli":
		if len(argTypes) > 1 && !types.Assignable(argTypes[1], types.NumberType, inf.mode == TypeCheckingStrict) && inf.final {
			inf.report(CodeParamTypeMismatch,
				fmt.Sprintf("deli index is %s, want number", types.Display(argTypes[1])),
				args[1].Span())
		}
		return types.AnyType, true
	case "pack":
		return types.Array{Elem: types.AnyType}, true
	case "unpack", "select":
		if len(argTypes) > 0 {
			return iterElement(argTypes[0]), true
		}
		return types.AnyType, true
	case "foreach":
		if len(args) > 1 && len(argTypes) > 0 {
			if elem := iterElement(argTypes[0]); elem != nil {
				if callee := inf.calleeSymbol(args[1]); callee != nil {
					inf.recordCall(callee, []types.Type{elem})
				}
			}
		}
		return types.AnyType, true
	case "add", "del":
		if len(argTypes) > 1 {
			if elem := iterElement(argTypes[0]); elem != nil && !types.IsAny(elem) {
				if !types.Assignable(argTypes[1], elem, inf.mode == TypeCheckingStrict) && inf.final {
					inf.report(CodeParamTypeMismatch,
						fmt.Sprintf("%s expects %s, got %s", name, types.Display(elem), types.Display(argTypes[1])),
						args[1].Span())
				}
			}
		}
		return types.AnyType, true
	case "type":
		return types.StringType, true
	case "tostr", "chr", "sub":
		return types.StringType, true
	case "tonum", "ord":
		return types.NumberType, true
	case "split":
		return types.Array{Elem: types.StringType}, true
	}
	return nil, false
}

func (inf *inferer) checkAssignment(sym *Symbol, declared, valueType types.Type, valueSpan source.Span) {
	if !inf.final || declared == nil || valueType == nil || types.IsAny(declared) || types.IsAny(valueType) {
		return
	}
	if inf.mode == TypeCheckingStrict {
		if record, ok := types.Resolve(declared).(*types.Record); ok {
			if valueRecord, ok := types.Resolve(valueType).(*types.Record); ok {
				missing := missingFields(record, valueRecord)
				if len(missing) > 0 {
					inf.report(CodeMissingFields, fmt.Sprintf("missing fields: %s", strings.Join(missing, ", ")), valueSpan)
					return
				}
			}
		}
	}
	if !types.Assignable(valueType, declared, inf.mode == TypeCheckingStrict) {
		inf.report(CodeAssignTypeMismatch,
			fmt.Sprintf("cannot assign %s to %s", types.Display(valueType), types.Display(declared)), valueSpan)
	}
}

func (inf *inferer) checkAssignmentValue(want, got types.Type, span source.Span) {
	if !inf.final || want == nil || got == nil || types.IsAny(want) || types.IsAny(got) {
		return
	}
	if !types.Assignable(got, want, inf.mode == TypeCheckingStrict) {
		inf.report(CodeAssignTypeMismatch,
			fmt.Sprintf("cannot assign %s to %s", types.Display(got), types.Display(want)), span)
	}
}

func missingFields(want, got *types.Record) []string {
	if want == nil || got == nil {
		return nil
	}
	var missing []string
	for _, field := range types.Fields(want) {
		if field.Optional {
			continue
		}
		if _, ok, _ := types.LookupField(got, field.Name); !ok {
			missing = append(missing, field.Name)
		}
	}
	return missing
}

func (inf *inferer) addRecordField(base types.Type, name string, fieldType types.Type, optional bool) {
	base = types.Resolve(base)
	record, ok := base.(*types.Record)
	if !ok || record == nil || record.Exact {
		return
	}
	record.AddField(&types.Field{Name: name, Type: fieldType, Optional: optional})
}

func (inf *inferer) attachTableFields(sym *Symbol, expr syntax.Expr, typ types.Type) {
	table, ok := expr.(*syntax.TableExpr)
	if !ok || sym == nil {
		return
	}
	record, ok := types.Resolve(typ).(*types.Record)
	if !ok {
		return
	}
	for _, field := range table.Fields {
		name, ok := field.Key.(*syntax.NameExpr)
		if !ok || field.BracketKey {
			continue
		}
		fieldSym := inf.fieldSymbolByName(table, name)
		if fieldSym == nil || fieldSym.TypeExplicit {
			continue
		}
		if fieldType, ok, _ := types.LookupField(record, name.Text); ok {
			fieldSym.Type = fieldType
		}
	}
}

func (inf *inferer) fillEnum(enum *types.Record, table *syntax.TableExpr, env map[*Symbol]types.Type) {
	if enum == nil || table == nil {
		return
	}
	for _, field := range table.Fields {
		name, ok := field.Key.(*syntax.NameExpr)
		if !ok || field.BracketKey {
			continue
		}
		literal := inf.literalType(field.Value, env)
		enum.AddField(&types.Field{Name: name.Text, Type: literal, Decl: name.Span()})
	}
}

func (inf *inferer) literalType(expr syntax.Expr, env map[*Symbol]types.Type) types.Type {
	switch e := expr.(type) {
	case *syntax.NumberExpr:
		if value, err := strconv.ParseFloat(e.Text, 64); err == nil {
			return types.Literal{Value: value}
		}
		return types.NumberType
	case *syntax.StringExpr:
		return types.Literal{Value: e.Value}
	case *syntax.BoolExpr:
		return types.Literal{Value: e.Value}
	}
	return firstType(inf.inferExpr(expr, env))
}

func (inf *inferer) symbolType(sym *Symbol, span source.Span, env map[*Symbol]types.Type) types.Type {
	if sym == nil {
		return types.AnyType
	}
	base := sym.Type
	if env != nil {
		if narrowed, ok := env[sym]; ok && narrowed != nil {
			base = narrowed
		}
	}
	if base == nil {
		base = types.AnyType
	}
	return inf.applyCasts(sym, base, span)
}

func (inf *inferer) applyCasts(sym *Symbol, base types.Type, span source.Span) types.Type {
	if sym == nil || len(inf.result.Casts) == 0 {
		return base
	}
	line := inf.lineOf(span)
	for _, cast := range inf.result.Casts {
		if cast.FileID != span.FileID || canonical(cast.Name) != sym.Canonical || cast.Line > line {
			continue
		}
		switch cast.Mode {
		case CastSet:
			base = cast.Type
		case CastAdd:
			base = types.UnionOf(base, cast.Type)
		case CastRemove:
			base = types.Remove(base, cast.Type)
		}
	}
	return base
}

func (inf *inferer) applyAs(expr syntax.Expr, typ types.Type) types.Type {
	if expr == nil || len(inf.result.AsCasts) == 0 {
		return typ
	}
	span := expr.Span()
	line := inf.lineOf(span)
	best := -1
	var chosen types.Type
	for _, as := range inf.result.AsCasts {
		if as.FileID != span.FileID || as.Line != line || as.Span.Start < span.End {
			continue
		}
		if best < 0 || as.Span.Start < best {
			best = as.Span.Start
			chosen = as.Type
		}
	}
	if chosen != nil {
		return chosen
	}
	return typ
}

func (inf *inferer) applyNarrow(env map[*Symbol]types.Type, cond syntax.Expr, truthy bool) {
	if cond == nil || env == nil {
		return
	}
	switch e := cond.(type) {
	case *syntax.NameExpr:
		sym := inf.symbolAt(e)
		if sym == nil {
			return
		}
		base := inf.symbolType(sym, e.Span(), env)
		if truthy {
			env[sym] = types.Truthy(base)
		} else {
			if types.ContainsNil(base) {
				env[sym] = types.NilType
			}
		}
	case *syntax.UnaryExpr:
		if e.Op.Text == "not" {
			inf.applyNarrow(env, e.X, !truthy)
		}
	case *syntax.ParenExpr:
		inf.applyNarrow(env, e.X, truthy)
	case *syntax.BinaryExpr:
		inf.applyNarrowBinary(env, e, truthy)
	}
}

func (inf *inferer) applyNarrowBinary(env map[*Symbol]types.Type, e *syntax.BinaryExpr, truthy bool) {
	switch e.Op.Text {
	case "and":
		if truthy {
			inf.applyNarrow(env, e.Left, true)
			inf.applyNarrow(env, e.Right, true)
		}
	case "or":
		if !truthy {
			inf.applyNarrow(env, e.Left, false)
			inf.applyNarrow(env, e.Right, false)
		}
	case "==", "~=", "!=":
		if name, nilSide := nilComparison(e.Left, e.Right); name != nil {
			if sym := inf.symbolAt(name); sym != nil {
				base := inf.symbolType(sym, name.Span(), env)
				isEqual := e.Op.Text == "=="
				if truthy == isEqual {
					if nilSide {
						env[sym] = types.NilType
					}
				} else {
					env[sym] = types.RemoveNil(base)
				}
				return
			}
		}
	}
	notEqual := e.Op.Text == "~=" || e.Op.Text == "!="
	if !(e.Op.Text == "==" && truthy) && !(notEqual && !truthy) {
		return
	}
	call, literal := typeCall(e.Left, e.Right)
	if call == nil || literal == "" {
		return
	}
	sym := inf.symbolAt(call.Args[0].(*syntax.NameExpr))
	if sym == nil {
		return
	}
	var narrowed types.Type
	switch literal {
	case "number":
		narrowed = types.NumberType
	case "string":
		narrowed = types.StringType
	case "boolean":
		narrowed = types.BooleanType
	case "table":
		narrowed = types.TableType
	case "function":
		narrowed = types.FunctionType
	}
	if narrowed == nil {
		return
	}
	env[sym] = narrowed
}

func (inf *inferer) fieldSymbol(expr *syntax.FieldExpr) *Symbol {
	if expr == nil || expr.Name == nil {
		return nil
	}
	return inf.symbolAt(expr.Name)
}

func (inf *inferer) fieldSymbolByName(table *syntax.TableExpr, name *syntax.NameExpr) *Symbol {
	if table == nil || name == nil {
		return nil
	}
	return inf.symbolAt(name)
}

func (inf *inferer) symbolAt(name *syntax.NameExpr) *Symbol {
	if name == nil {
		return nil
	}
	occ := inf.result.OccurrenceAt(name.Span().FileID, name.Span().Start)
	if occ == nil {
		return nil
	}
	return occ.Symbol
}

func (inf *inferer) report(code, message string, span source.Span) {
	if span.IsEmpty() {
		return
	}
	inf.result.addTypeDiagnostic(inf.mode, code, message, span, SeverityWarning)
}

func (inf *inferer) lineOf(span source.Span) int {
	file := inf.snap.Cart.File(span.FileID)
	if file == nil || file.Index == nil {
		return -1
	}
	pos, err := file.Index.Position(span.Start)
	if err != nil {
		return -1
	}
	return int(pos.Line)
}

func (inf *inferer) emitDeprecated() {
	if inf.mode == TypeCheckingOff {
		return
	}
	for _, sym := range inf.result.Table.Symbols {
		if sym == nil || sym.Doc == nil || !sym.Doc.Deprecated {
			continue
		}
		for _, span := range sym.Uses {
			inf.result.addTypeDiagnostic(inf.mode, CodeDeprecated, fmt.Sprintf("%s is deprecated", sym.Name), span, SeverityHint)
		}
	}
}

func callReturnType(fn *types.Function) types.Type {
	if fn == nil {
		return types.AnyType
	}
	if len(fn.Returns.Elems) > 1 || fn.Returns.Vararg != nil {
		return fn.Returns
	}
	return types.First(fn.Returns)
}

func multiValueAt(values []types.Type, target int) types.Type {
	if target < len(values) {
		value := values[target]
		if tuple, ok := value.(types.Tuple); ok {
			return typeAt(tuple, 0)
		}
		if target == len(values)-1 {
			if tuple, ok := values[len(values)-1].(types.Tuple); ok {
				return typeAt(tuple, 0)
			}
		}
		return value
	}
	if len(values) > 0 {
		if tuple, ok := values[len(values)-1].(types.Tuple); ok {
			offset := target - (len(values) - 1)
			return typeAt(tuple, offset)
		}
	}
	return types.NilType
}

func typeAt(t types.Type, index int) types.Type {
	if tuple, ok := t.(types.Tuple); ok {
		if index < len(tuple.Elems) {
			return tuple.Elems[index]
		}
		if tuple.Vararg != nil {
			return tuple.Vararg
		}
		return types.NilType
	}
	return firstType(t)
}

func firstType(t types.Type) types.Type {
	return types.First(t)
}

func spanOfExpr(expr syntax.Expr) source.Span {
	if expr == nil {
		return source.Span{}
	}
	return expr.Span()
}

func valueAtExpr(values []syntax.Expr, index int) syntax.Expr {
	if index >= 0 && index < len(values) {
		return values[index]
	}
	return nil
}

func callName(expr syntax.Expr) string {
	switch e := expr.(type) {
	case *syntax.NameExpr:
		return canonical(e.Text)
	case *syntax.FieldExpr:
		return canonical(e.Name.Text)
	case *syntax.ParenExpr:
		return callName(e.X)
	}
	return ""
}

func canonical(name string) string {
	return CanonicalName(name)
}

func iteratorLoopTypes(name string, iter types.Type) []types.Type {
	elem := iterElement(iter)
	switch name {
	case "all":
		return []types.Type{elem}
	case "ipairs":
		return []types.Type{types.NumberType, elem}
	case "pairs":
		return []types.Type{types.AnyType, elem}
	}
	return nil
}

func iterElement(t types.Type) types.Type {
	switch value := types.Resolve(t).(type) {
	case types.Array:
		return value.Elem
	case types.Map:
		return value.Elem
	case *types.Record:
		var out []types.Type
		for _, field := range types.Fields(value) {
			out = append(out, field.Type)
		}
		if len(out) > 0 {
			return types.UnionOf(out...)
		}
	case types.Union:
		var out []types.Type
		for _, member := range value.Members {
			elem := iterElement(member)
			if !types.IsAny(elem) {
				out = append(out, elem)
			}
		}
		if len(out) > 0 {
			return types.UnionOf(out...)
		}
	}
	return types.AnyType
}

func nilComparison(left, right syntax.Expr) (*syntax.NameExpr, bool) {
	if name, ok := left.(*syntax.NameExpr); ok {
		if _, ok := right.(*syntax.NilExpr); ok {
			return name, true
		}
	}
	if name, ok := right.(*syntax.NameExpr); ok {
		if _, ok := left.(*syntax.NilExpr); ok {
			return name, true
		}
	}
	return nil, false
}

func typeCall(left, right syntax.Expr) (*syntax.CallExpr, string) {
	call, ok := left.(*syntax.CallExpr)
	if !ok {
		call, ok = right.(*syntax.CallExpr)
		if !ok {
			return nil, ""
		}
	}
	literal, ok := right.(*syntax.StringExpr)
	if !ok {
		literal, ok = left.(*syntax.StringExpr)
		if !ok {
			return nil, ""
		}
	}
	if callName(call.Func) != "type" || len(call.Args) != 1 {
		return nil, ""
	}
	if _, ok := call.Args[0].(*syntax.NameExpr); !ok {
		return nil, ""
	}
	return call, literal.Value
}

func blockAlwaysReturns(block *syntax.Block) bool {
	if block == nil || len(block.Stmts) == 0 {
		return false
	}
	_, ok := block.Stmts[len(block.Stmts)-1].(*syntax.ReturnStmt)
	return ok
}

func cloneEnv(env map[*Symbol]types.Type) map[*Symbol]types.Type {
	out := make(map[*Symbol]types.Type, len(env))
	for sym, typ := range env {
		out[sym] = typ
	}
	return out
}

func mergeEnv(dst, src map[*Symbol]types.Type) {
	for sym, typ := range src {
		dst[sym] = typ
	}
}

func setParamAt(params []types.Param, index int, param types.Param) []types.Param {
	for len(params) <= index {
		params = append(params, types.Param{})
	}
	params[index] = param
	return params
}
