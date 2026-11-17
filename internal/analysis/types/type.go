// Package types implements the gradual PICO-8/LuaCATS type model used by
// analysis and the LSP layer.
package types

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/source"
)

// Type is one node in the type graph.
type Type interface {
	typeNode()
}

// PrimitiveKind identifies a built-in type.
type PrimitiveKind int

const (
	KindAny PrimitiveKind = iota
	KindNil
	KindBoolean
	KindNumber
	KindInteger
	KindString
	KindTable
	KindFunction
	KindThread
	KindUserdata
)

// Primitive is a built-in scalar or unparameterized type.
type Primitive struct {
	Kind PrimitiveKind
}

func (Primitive) typeNode() {}

var (
	AnyType      = Primitive{Kind: KindAny}
	NilType      = Primitive{Kind: KindNil}
	BooleanType  = Primitive{Kind: KindBoolean}
	NumberType   = Primitive{Kind: KindNumber}
	IntegerType  = Primitive{Kind: KindInteger}
	StringType   = Primitive{Kind: KindString}
	TableType    = Primitive{Kind: KindTable}
	FunctionType = Primitive{Kind: KindFunction}
	ThreadType   = Primitive{Kind: KindThread}
	UserdataType = Primitive{Kind: KindUserdata}
)

// Literal is a singleton type such as "left", 3, true, or nil.
type Literal struct {
	Value any
}

func (Literal) typeNode() {}

// Union is a canonical union; members are unique and sorted.
type Union struct {
	Members []Type
}

func (Union) typeNode() {}

// Array is a homogeneous array type.
type Array struct {
	Elem Type
}

func (Array) typeNode() {}

// Map is a key/value table type.
type Map struct {
	Key  Type
	Elem Type
}

func (Map) typeNode() {}

// Record is a structural table/class type.
type Record struct {
	Name    string
	Fields  map[string]*Field // canonical name -> field
	Open    bool
	Exact   bool
	Enum    bool
	Parents []*Record
}

func (Record) typeNode() {}

// Field is one record field.
type Field struct {
	Name     string
	Type     Type
	Optional bool
	Decl     source.Span
	// StringKey marks a field created from a table string key. Identifier
	// fields fold case; string keys remain case-sensitive.
	StringKey bool
	// Inherited marks a field or method declared on the class table. Such
	// members are available through the metatable and are not required in
	// instance literals.
	Inherited bool
}

// Function is a callable type.
type Function struct {
	Params  []Param
	Returns Tuple
	Vararg  Type
}

func (Function) typeNode() {}

// Param is one declared function parameter.
type Param struct {
	Name     string
	Type     Type
	Optional bool
	Doc      string
	Explicit bool
}

// Tuple is a multiple-return type. Vararg is the tail element type, if any.
type Tuple struct {
	Elems  []Type
	Vararg Type
}

func (Tuple) typeNode() {}

// Alias is a named reference to another type.
type Alias struct {
	Name       string
	Underlying Type
}

func (Alias) typeNode() {}

// TypeVar is an inference variable. Bound is nil while unresolved.
type TypeVar struct {
	ID    int
	Bound Type
}

func (TypeVar) typeNode() {}

// NewRecord creates an open, empty record. Named records are open unless Exact
// is set by the annotation parser.
func NewRecord(name string) *Record {
	return &Record{Name: name, Fields: make(map[string]*Field), Open: true}
}

// AddField adds or replaces a field on r.
func (r *Record) AddField(field *Field) {
	if r == nil || field == nil {
		return
	}
	if field.Name == "" {
		return
	}
	if r.Fields == nil {
		r.Fields = make(map[string]*Field)
	}
	key := CanonicalName(field.Name)
	if field.StringKey {
		// String keys are case-sensitive, so keep them out of the folded
		// identifier namespace.
		key = stringKeyPrefix + field.Name
	}
	if key == "" {
		return
	}
	if existing := r.Fields[key]; existing != nil && existing.Type != nil && existing.Name != "" {
		// Keep the first declaration's metadata but fill in missing type info.
		if existing.Type == nil || IsAny(existing.Type) {
			existing.Type = field.Type
		}
		return
	}
	r.Fields[key] = field
}

// stringKeyPrefix keeps case-sensitive string-key fields out of the folded
// identifier namespace in Record.Fields.
const stringKeyPrefix = "\x00"

// CanonicalName ASCII-lowercases a type/field name, matching PICO-8 folding.
func CanonicalName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b.WriteByte(c)
	}
	return b.String()
}

// UnionOf flattens and canonicalizes a union. any absorbs every other member.
func UnionOf(members ...Type) Type {
	var flat []Type
	var add func(Type)
	add = func(t Type) {
		switch v := t.(type) {
		case nil:
			return
		case Union:
			for _, member := range v.Members {
				add(member)
			}
		case *Union:
			if v != nil {
				for _, member := range v.Members {
					add(member)
				}
			}
		case *Alias:
			if v == nil {
				return
			}
			flat = append(flat, t)
		default:
			flat = append(flat, t)
		}
	}
	for _, member := range members {
		add(member)
	}
	seen := make(map[string]Type)
	for _, member := range flat {
		if IsAny(member) {
			return AnyType
		}
		key := Display(member)
		if _, ok := seen[key]; !ok {
			seen[key] = member
		}
	}
	out := make([]Type, 0, len(seen))
	for _, member := range seen {
		out = append(out, member)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return Display(out[i]) < Display(out[j])
	})
	if len(out) == 0 {
		return AnyType
	}
	if len(out) == 1 {
		return out[0]
	}
	return Union{Members: out}
}

// Optional returns t | nil.
func Optional(t Type) Type {
	return UnionOf(t, NilType)
}

// Remove returns t with the members of remove removed. It is used by @cast -T.
func Remove(t Type, remove Type) Type {
	base := Resolve(t)
	if union, ok := base.(Union); ok {
		out := make([]Type, 0, len(union.Members))
		for _, member := range union.Members {
			if !Assignable(member, remove, false) {
				out = append(out, member)
			}
		}
		return UnionOf(out...)
	}
	if Assignable(base, remove, false) {
		return AnyType
	}
	return base
}

// RemoveNil returns t with nil removed, or any when nothing remains.
func RemoveNil(t Type) Type {
	switch v := t.(type) {
	case Union:
		out := make([]Type, 0, len(v.Members))
		for _, member := range v.Members {
			if !IsNil(member) {
				out = append(out, member)
			}
		}
		if len(out) == 0 {
			return NilType
		}
		return UnionOf(out...)
	case *Alias:
		if v != nil && v.Underlying != nil {
			return RemoveNil(v.Underlying)
		}
	}
	if IsNil(t) {
		return NilType
	}
	return t
}

// IsAny reports whether t is the top type.
func IsAny(t Type) bool {
	switch v := t.(type) {
	case Primitive:
		return v.Kind == KindAny
	case *Alias:
		return v == nil || v.Underlying == nil || IsAny(v.Underlying)
	case *TypeVar:
		return v == nil || v.Bound == nil || IsAny(v.Bound)
	case nil:
		return true
	default:
		return false
	}
}

// IsNil reports whether t is exactly nil.
func IsNil(t Type) bool {
	switch v := t.(type) {
	case Primitive:
		return v.Kind == KindNil
	case *Alias:
		return v != nil && v.Underlying != nil && IsNil(v.Underlying)
	default:
		return false
	}
}

// ContainsNil reports whether t includes nil.
func ContainsNil(t Type) bool {
	switch v := t.(type) {
	case Primitive:
		return v.Kind == KindNil
	case Union:
		for _, member := range v.Members {
			if ContainsNil(member) {
				return true
			}
		}
	case *Alias:
		return v != nil && v.Underlying != nil && ContainsNil(v.Underlying)
	case *TypeVar:
		return v != nil && v.Bound != nil && ContainsNil(v.Bound)
	}
	return false
}

// Truthy returns the part of t that survives an `if t then` test. When no
// member survives, it returns boolean to keep narrowing conservative.
func Truthy(t Type) Type {
	if part := TruthyPart(t); part != nil {
		return part
	}
	return BooleanType
}

// TruthyPart returns the truthy members of t, or nil when none survive.
func TruthyPart(t Type) Type {
	t = Resolve(t)
	switch v := t.(type) {
	case Union:
		out := make([]Type, 0, len(v.Members))
		for _, member := range v.Members {
			if part := TruthyPart(member); part != nil {
				out = append(out, part)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return UnionOf(out...)
	case Primitive:
		switch v.Kind {
		case KindAny:
			return AnyType
		case KindNil:
			return nil
		default:
			return t
		}
	case Literal:
		if value, ok := v.Value.(bool); ok && !value {
			return nil
		}
		return t
	case nil:
		return nil
	default:
		return t
	}
}

// Falsy returns the nil and false members of t, or nil when t is always truthy.
func Falsy(t Type) Type {
	t = Resolve(t)
	switch v := t.(type) {
	case Union:
		out := make([]Type, 0, len(v.Members))
		for _, member := range v.Members {
			if part := Falsy(member); part != nil {
				out = append(out, part)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return UnionOf(out...)
	case Primitive:
		switch v.Kind {
		case KindAny:
			return AnyType
		case KindNil:
			return t
		case KindBoolean:
			return t
		default:
			return nil
		}
	case Literal:
		if value, ok := v.Value.(bool); ok && !value {
			return t
		}
		return nil
	case nil:
		return nil
	default:
		return nil
	}
}

// Widen converts literal types to their primitive counterparts for display
// and ordinary inference. Enum member types are kept literal by callers that
// need them.
func Widen(t Type) Type {
	switch v := t.(type) {
	case Literal:
		switch v.Value.(type) {
		case string:
			return StringType
		case bool:
			return BooleanType
		default:
			return NumberType
		}
	case Union:
		out := make([]Type, 0, len(v.Members))
		for _, member := range v.Members {
			out = append(out, Widen(member))
		}
		return UnionOf(out...)
	}
	return t
}

// First returns the first value of a tuple, or t itself.
func First(t Type) Type {
	if tuple, ok := t.(Tuple); ok {
		if len(tuple.Elems) > 0 {
			return tuple.Elems[0]
		}
		if tuple.Vararg != nil {
			return tuple.Vararg
		}
		return AnyType
	}
	return t
}

// TupleOf builds a canonical tuple.
func TupleOf(elems ...Type) Tuple {
	return Tuple{Elems: elems}
}

const maxDisplayDepth = 24

// Display renders t in LuaCATS syntax.
func Display(t Type) string {
	return displayType(t, &displayState{seen: make(map[*Record]bool)})
}

type displayState struct {
	seen  map[*Record]bool
	depth int
}

func displayType(t Type, state *displayState) string {
	if t == nil {
		return "any"
	}
	if state.depth >= maxDisplayDepth {
		return "..."
	}
	state.depth++
	defer func() { state.depth-- }()
	switch v := t.(type) {
	case Primitive:
		switch v.Kind {
		case KindAny:
			return "any"
		case KindNil:
			return "nil"
		case KindBoolean:
			return "boolean"
		case KindNumber:
			return "number"
		case KindInteger:
			return "integer"
		case KindString:
			return "string"
		case KindTable:
			return "table"
		case KindFunction:
			return "function"
		case KindThread:
			return "thread"
		case KindUserdata:
			return "userdata"
		default:
			return "any"
		}
	case Literal:
		switch value := v.Value.(type) {
		case string:
			return strconv.Quote(value)
		case bool:
			if value {
				return "true"
			}
			return "false"
		case nil:
			return "nil"
		default:
			return fmt.Sprint(value)
		}
	case Union:
		return displayUnion(v.Members, state)
	case Array:
		return parenForPostfix(v.Elem, state) + "[]"
	case Map:
		return "table<" + displayType(v.Key, state) + ", " + displayType(v.Elem, state) + ">"
	case *Record:
		if v == nil {
			return "table"
		}
		if v.Name != "" {
			return v.Name
		}
		if state.seen[v] {
			return "{...}"
		}
		return displayRecord(v, state)
	case Record:
		return displayRecord(&v, state)
	case *Function:
		if v == nil {
			return "function"
		}
		return displayFunction(v, state)
	case Function:
		return displayFunction(&v, state)
	case Tuple:
		parts := make([]string, 0, len(v.Elems)+1)
		for _, elem := range v.Elems {
			parts = append(parts, displayType(elem, state))
		}
		if v.Vararg != nil {
			parts = append(parts, "..."+displayType(v.Vararg, state))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *Alias:
		if v == nil {
			return "any"
		}
		if v.Name != "" {
			return v.Name
		}
		return displayType(v.Underlying, state)
	case *TypeVar:
		if v == nil || v.Bound == nil {
			return "any"
		}
		return displayType(v.Bound, state)
	default:
		return "any"
	}
}

func displayUnion(members []Type, state *displayState) string {
	var nonNil []Type
	hasNil := false
	for _, member := range members {
		if IsNil(member) {
			hasNil = true
			continue
		}
		nonNil = append(nonNil, member)
	}
	if hasNil && len(nonNil) == 1 {
		return parenForUnion(nonNil[0], state) + "?"
	}
	parts := make([]string, 0, len(members))
	for _, member := range members {
		parts = append(parts, displayType(member, state))
	}
	if len(parts) == 0 {
		return "any"
	}
	if len(parts) > 8 {
		parts = append(parts[:8], "...")
	}
	return strings.Join(parts, " | ")
}

func displayRecord(record *Record, state *displayState) string {
	if record == nil {
		return "table"
	}
	if state.seen[record] {
		return "{...}"
	}
	state.seen[record] = true
	defer delete(state.seen, record)
	keys := make([]string, 0, len(record.Fields))
	for key := range record.Fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		field := record.Fields[key]
		label := field.Name
		if field.Optional {
			label += "?"
		}
		parts = append(parts, label+": "+displayType(field.Type, state))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func displayFunction(fn *Function, state *displayState) string {
	parts := make([]string, 0, len(fn.Params)+1)
	for _, param := range fn.Params {
		label := param.Name
		if label == "" {
			label = "arg"
		}
		if param.Optional {
			label += "?"
		}
		label += ": " + displayType(param.Type, state)
		parts = append(parts, label)
	}
	if fn.Vararg != nil {
		parts = append(parts, "...: "+displayType(fn.Vararg, state))
	}
	out := "fun(" + strings.Join(parts, ", ") + ")"
	if len(fn.Returns.Elems) == 1 && fn.Returns.Vararg == nil {
		out += ": " + displayType(fn.Returns.Elems[0], state)
	} else if len(fn.Returns.Elems) > 1 {
		out += ": " + displayType(fn.Returns, state)
	}
	return out
}

func parenForPostfix(t Type, state *displayState) string {
	switch t.(type) {
	case Union:
		return "(" + displayType(t, state) + ")"
	case *Function, Function:
		return "(" + displayType(t, state) + ")"
	default:
		return displayType(t, state)
	}
}

func parenForUnion(t Type, state *displayState) string {
	switch t.(type) {
	case Union:
		return "(" + displayType(t, state) + ")"
	}
	return displayType(t, state)
}
