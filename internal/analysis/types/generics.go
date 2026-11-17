package types

import (
	"fmt"
	"strings"
)

// GenericParam is a named type parameter scoped to one annotation.
type GenericParam struct {
	ID      int
	Name    string
	Default Type
}

func (*GenericParam) typeNode() {}

// Operator is one explicit LuaCATS operator annotation on a class.
type Operator struct {
	Name    string
	Operand Type
	Result  Type
	// Call holds the full function signature for the call operator.
	Call *Function
}

// Apply instantiates a generic record with args. Missing trailing arguments use
// the generic default, then any.
func Apply(base Type, args []Type) (Type, error) {
	base = Resolve(base)
	record, ok := base.(*Record)
	if !ok {
		if len(args) == 0 {
			return base, nil
		}
		return nil, fmt.Errorf("type %s is not generic", Display(base))
	}
	if len(record.TypeParams) == 0 {
		if len(args) == 0 {
			return record, nil
		}
		return nil, fmt.Errorf("type %s is not generic", Display(record))
	}
	if len(args) > len(record.TypeParams) {
		return nil, fmt.Errorf("type %s expects %d type arguments, got %d", record.Name, len(record.TypeParams), len(args))
	}
	return Instantiate(record, args), nil
}

// Instantiate returns a specialized copy of a generic record with its fields,
// parents, and operators substituted for args.
func Instantiate(record *Record, args []Type) *Record {
	return instantiateWithState(record, args, makeSubstituteState())
}

type substituteState struct {
	records map[string]*Record
}

func makeSubstituteState() *substituteState {
	return &substituteState{records: make(map[string]*Record)}
}

func argumentKey(t Type) string {
	switch v := t.(type) {
	case nil:
		return "nil"
	case *GenericParam:
		return fmt.Sprintf("g%p", v)
	case *Record:
		return fmt.Sprintf("r%p", v)
	case *Alias:
		return fmt.Sprintf("a%p", v)
	case Primitive:
		return fmt.Sprintf("p%d", v.Kind)
	case Literal:
		return fmt.Sprintf("l%v:%T", v.Value, v.Value)
	}
	return Display(t)
}

func instantiationKey(record *Record, args []Type) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%p", record)
	for _, arg := range args {
		b.WriteByte('|')
		b.WriteString(argumentKey(arg))
	}
	return b.String()
}

func instantiateWithState(record *Record, args []Type, state *substituteState) *Record {
	if record == nil {
		return nil
	}
	template := record
	if record.Template != nil {
		template = record.Template
	}
	if len(template.TypeParams) == 0 {
		return record
	}
	if state == nil {
		state = makeSubstituteState()
	}
	resolved := make([]Type, len(template.TypeParams))
	binding := make(map[*GenericParam]Type, len(template.TypeParams))
	for i, param := range template.TypeParams {
		var arg Type = AnyType
		if i < len(args) && args[i] != nil {
			arg = args[i]
		} else if param != nil && param.Default != nil {
			arg = param.Default
		}
		resolved[i] = arg
		binding[param] = arg
	}
	key := instantiationKey(template, resolved)
	if existing := state.records[key]; existing != nil {
		return existing
	}
	copyRecord := &Record{
		Name:       template.Name,
		Open:       template.Open,
		Exact:      template.Exact,
		Enum:       template.Enum,
		TypeParams: template.TypeParams,
		TypeArgs:   resolved,
		Template:   template,
	}
	state.records[key] = copyRecord
	for _, field := range template.Fields {
		if field == nil {
			continue
		}
		copyRecord.AddField(&Field{
			Name:      field.Name,
			Type:      substituteWithState(field.Type, binding, state, 0),
			Optional:  field.Optional,
			Decl:      field.Decl,
			StringKey: field.StringKey,
			Inherited: field.Inherited,
		})
	}
	for _, parent := range template.Parents {
		if parent == nil {
			continue
		}
		if substituted, ok := substituteWithState(parent, binding, state, 0).(*Record); ok && substituted != nil {
			copyRecord.Parents = append(copyRecord.Parents, substituted)
		}
	}
	for name, op := range template.Operators {
		if op == nil {
			continue
		}
		if copyRecord.Operators == nil {
			copyRecord.Operators = make(map[string]*Operator)
		}
		copyRecord.Operators[name] = &Operator{
			Name:    op.Name,
			Operand: substituteWithState(op.Operand, binding, state, 0),
			Result:  substituteWithState(op.Result, binding, state, 0),
			Call:    substituteFunctionWithState(op.Call, binding, state, 0),
		}
	}
	return copyRecord
}

// SubstituteFor applies a receiver's generic arguments to t. It is used when a
// member comes from a generic class template.
func SubstituteFor(receiver Type, t Type) Type {
	record, ok := Resolve(receiver).(*Record)
	if !ok || record == nil || record.Template == nil {
		return t
	}
	return Substitute(t, recordBinding(record))
}

// Substitute replaces generic parameters in t using binding. Unbound parameters
// fall back to their default, then any.
func Substitute(t Type, binding map[*GenericParam]Type) Type {
	return substituteWithState(t, binding, nil, 0)
}

func substituteWithState(t Type, binding map[*GenericParam]Type, state *substituteState, depth int) Type {
	if t == nil {
		return nil
	}
	if depth > maxTypeDepth {
		return AnyType
	}
	switch v := t.(type) {
	case *GenericParam:
		if v == nil {
			return AnyType
		}
		if bound, ok := binding[v]; ok && bound != nil {
			return substituteWithState(bound, binding, state, depth+1)
		}
		if v.Default != nil {
			return substituteWithState(v.Default, binding, state, depth+1)
		}
		return AnyType
	case Primitive, Literal, *TypeVar:
		return t
	case Union:
		members := make([]Type, 0, len(v.Members))
		for _, member := range v.Members {
			members = append(members, substituteWithState(member, binding, state, depth+1))
		}
		return UnionOf(members...)
	case Array:
		return Array{Elem: substituteWithState(v.Elem, binding, state, depth+1)}
	case Map:
		return Map{
			Key:  substituteWithState(v.Key, binding, state, depth+1),
			Elem: substituteWithState(v.Elem, binding, state, depth+1),
		}
	case Tuple:
		elems := make([]Type, 0, len(v.Elems))
		for _, elem := range v.Elems {
			elems = append(elems, substituteWithState(elem, binding, state, depth+1))
		}
		return Tuple{Elems: elems, Vararg: substituteWithState(v.Vararg, binding, state, depth+1)}
	case *Alias:
		if v == nil {
			return AnyType
		}
		return &Alias{Name: v.Name, Underlying: substituteWithState(v.Underlying, binding, state, depth+1)}
	case *Function:
		if v == nil {
			return AnyType
		}
		return substituteFunctionWithState(v, binding, state, depth)
	case Function:
		return substituteFunctionWithState(&v, binding, state, depth)
	case *Record:
		if v == nil {
			return AnyType
		}
		if len(v.TypeArgs) > 0 || len(v.TypeParams) > 0 {
			template := v
			if v.Template != nil {
				template = v.Template
			}
			args := make([]Type, 0, len(template.TypeParams))
			for i, param := range template.TypeParams {
				arg := Type(AnyType)
				if i < len(v.TypeArgs) && v.TypeArgs[i] != nil {
					arg = substituteWithState(v.TypeArgs[i], binding, state, depth+1)
				} else if bound, ok := binding[param]; ok && bound != nil {
					arg = substituteWithState(bound, binding, state, depth+1)
				} else if param != nil && param.Default != nil {
					arg = substituteWithState(param.Default, binding, state, depth+1)
				}
				args = append(args, arg)
			}
			if state != nil {
				if existing := state.records[instantiationKey(template, args)]; existing != nil {
					return existing
				}
			}
			return instantiateWithState(template, args, state)
		}
		return v
	}
	return t
}

func substituteFunctionWithState(fn *Function, binding map[*GenericParam]Type, state *substituteState, depth int) *Function {
	if fn == nil {
		return nil
	}
	out := &Function{Generics: fn.Generics}
	out.Params = make([]Param, 0, len(fn.Params))
	for _, param := range fn.Params {
		param.Type = substituteWithState(param.Type, binding, state, depth+1)
		out.Params = append(out.Params, param)
	}
	out.Returns = Tuple{
		Elems:  make([]Type, 0, len(fn.Returns.Elems)),
		Vararg: substituteWithState(fn.Returns.Vararg, binding, state, depth+1),
	}
	for _, elem := range fn.Returns.Elems {
		out.Returns.Elems = append(out.Returns.Elems, substituteWithState(elem, binding, state, depth+1))
	}
	out.Vararg = substituteWithState(fn.Vararg, binding, state, depth+1)
	return out
}

// BindFunction specializes a generic function's parameter and return types
// from call argument types. Unbound generics fall back to defaults or any.
func BindFunction(fn *Function, args []Type) *Function {
	if fn == nil || len(fn.Generics) == 0 {
		return fn
	}
	binding := make(map[*GenericParam]Type, len(fn.Generics))
	for i, param := range fn.Params {
		if i >= len(args) {
			break
		}
		bindType(param.Type, args[i], binding, 0)
	}
	if fn.Vararg != nil {
		for i := len(fn.Params); i < len(args); i++ {
			bindType(fn.Vararg, args[i], binding, 0)
		}
	}
	specialized := substituteFunctionWithState(fn, binding, makeSubstituteState(), 0)
	specialized.Generics = nil
	return specialized
}

func bindType(pattern, actual Type, binding map[*GenericParam]Type, depth int) {
	if pattern == nil || actual == nil || depth > maxTypeDepth {
		return
	}
	switch p := pattern.(type) {
	case *GenericParam:
		if p == nil {
			return
		}
		actual = Widen(actual)
		if existing, ok := binding[p]; ok {
			if !Identical(existing, actual) {
				binding[p] = UnionOf(existing, actual)
			}
			return
		}
		binding[p] = actual
		return
	case *Alias:
		if p != nil {
			bindType(p.Underlying, actual, binding, depth+1)
		}
		return
	case *Record:
		if p == nil {
			return
		}
		actualRecord, ok := Resolve(actual).(*Record)
		if !ok || actualRecord == nil {
			return
		}
		if len(p.TypeArgs) > 0 && len(actualRecord.TypeArgs) > 0 {
			if CanonicalName(p.Name) != CanonicalName(actualRecord.Name) {
				return
			}
			for i := range p.TypeArgs {
				if i >= len(actualRecord.TypeArgs) {
					break
				}
				bindType(p.TypeArgs[i], actualRecord.TypeArgs[i], binding, depth+1)
			}
			return
		}
		for key, field := range p.Fields {
			if field == nil {
				continue
			}
			if actualField := recordField(actualRecord, key); actualField != nil {
				bindType(field.Type, actualField.Type, binding, depth+1)
			}
		}
		return
	case Array:
		if actualArray, ok := Resolve(actual).(Array); ok {
			bindType(p.Elem, actualArray.Elem, binding, depth+1)
		}
		return
	case Map:
		if actualMap, ok := Resolve(actual).(Map); ok {
			bindType(p.Key, actualMap.Key, binding, depth+1)
			bindType(p.Elem, actualMap.Elem, binding, depth+1)
		}
		return
	case *Function:
		actualFn, ok := AsFunction(actual)
		if !ok {
			return
		}
		for i := range p.Params {
			if i >= len(actualFn.Params) {
				break
			}
			bindType(p.Params[i].Type, actualFn.Params[i].Type, binding, depth+1)
		}
		for i := range p.Returns.Elems {
			if i >= len(actualFn.Returns.Elems) {
				break
			}
			bindType(p.Returns.Elems[i], actualFn.Returns.Elems[i], binding, depth+1)
		}
		return
	case Tuple:
		actualTuple, ok := Resolve(actual).(Tuple)
		if !ok {
			return
		}
		for i := range p.Elems {
			if i >= len(actualTuple.Elems) {
				break
			}
			bindType(p.Elems[i], actualTuple.Elems[i], binding, depth+1)
		}
		return
	case Union:
		for _, member := range p.Members {
			if _, isGeneric := member.(*GenericParam); isGeneric {
				bindType(member, actual, binding, depth+1)
			}
		}
	}
}

func recordBinding(record *Record) map[*GenericParam]Type {
	if record == nil || record.Template == nil {
		return nil
	}
	binding := make(map[*GenericParam]Type)
	for i, param := range record.Template.TypeParams {
		if param == nil {
			continue
		}
		if i < len(record.TypeArgs) && record.TypeArgs[i] != nil {
			binding[param] = record.TypeArgs[i]
		} else if param.Default != nil {
			binding[param] = param.Default
		} else {
			binding[param] = AnyType
		}
	}
	return binding
}

// LookupOperator returns the explicit operator annotation named name on t.
func LookupOperator(t Type, name string) (*Operator, bool) {
	return lookupOperator(Resolve(t), name, 0)
}

func lookupOperator(t Type, name string, depth int) (*Operator, bool) {
	if t == nil || depth > maxTypeDepth {
		return nil, false
	}
	record, ok := t.(*Record)
	if !ok || record == nil {
		return nil, false
	}
	if op := record.Operators[name]; op != nil {
		return op, true
	}
	if record.Template != nil {
		if op := record.Template.Operators[name]; op != nil {
			binding := recordBinding(record)
			return &Operator{
				Name:    op.Name,
				Operand: Substitute(op.Operand, binding),
				Result:  Substitute(op.Result, binding),
				Call:    substituteFunctionWithState(op.Call, binding, makeSubstituteState(), 0),
			}, true
		}
	}
	for _, parent := range record.Parents {
		if op, ok := lookupOperator(parent, name, depth+1); ok {
			return op, true
		}
	}
	return nil, false
}
