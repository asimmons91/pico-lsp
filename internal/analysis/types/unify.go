package types

import (
	"sort"

	"github.com/asimmons91/pico-lsp/internal/source"
)

const maxTypeDepth = 32

// Resolve follows aliases and bound type variables.
func Resolve(t Type) Type {
	seenAlias := make(map[*Alias]bool)
	for i := 0; i < maxTypeDepth; i++ {
		switch v := t.(type) {
		case *Alias:
			if v == nil || v.Underlying == nil || seenAlias[v] {
				return AnyType
			}
			seenAlias[v] = true
			t = v.Underlying
		case *TypeVar:
			if v == nil || v.Bound == nil {
				return AnyType
			}
			t = v.Bound
		default:
			return t
		}
	}
	return AnyType
}

// Assignable reports whether src can be assigned to dst. When either side is
// any, the answer is always true.
func Assignable(src, dst Type, strict bool) bool {
	return assignable(src, dst, strict, 0)
}

func assignable(src, dst Type, strict bool, depth int) bool {
	if depth > maxTypeDepth {
		return true
	}
	src = Resolve(src)
	dst = Resolve(dst)
	if IsAny(src) || IsAny(dst) || src == nil || dst == nil {
		return true
	}
	if srcUnion, ok := src.(Union); ok {
		if len(srcUnion.Members) == 0 {
			return true
		}
		for _, member := range srcUnion.Members {
			if !assignable(member, dst, strict, depth+1) {
				return false
			}
		}
		return true
	}
	if dstUnion, ok := dst.(Union); ok {
		for _, member := range dstUnion.Members {
			if assignable(src, member, strict, depth+1) {
				return true
			}
		}
		return false
	}
	switch d := dst.(type) {
	case Primitive:
		return assignablePrimitive(src, d, strict, depth)
	case Literal:
		srcLit, ok := src.(Literal)
		if !ok {
			return false
		}
		return literalEqual(srcLit.Value, d.Value)
	case Array:
		switch s := src.(type) {
		case Array:
			return assignable(s.Elem, d.Elem, strict, depth+1)
		case *Record:
			return s != nil && len(s.Fields) == 0
		case Primitive:
			return s.Kind == KindTable || s.Kind == KindAny
		}
		return false
	case Map:
		switch s := src.(type) {
		case Map:
			return assignable(s.Key, d.Key, strict, depth+1) && assignable(s.Elem, d.Elem, strict, depth+1)
		case *Record:
			if s == nil || !assignable(StringType, d.Key, strict, depth+1) {
				return false
			}
			for _, field := range s.Fields {
				if !assignable(field.Type, d.Elem, strict, depth+1) {
					return false
				}
			}
			return true
		case Primitive:
			return s.Kind == KindTable || s.Kind == KindAny
		}
		return false
	case *Record:
		return assignableRecord(src, d, strict, depth)
	case Function:
		return assignableFunction(src, &d, depth)
	case *Function:
		return assignableFunction(src, d, depth)
	case Tuple:
		return assignableTuple(src, d, strict, depth)
	case *TypeVar:
		if d.Bound == nil {
			return true
		}
		return assignable(src, d.Bound, strict, depth+1)
	}
	return false
}

func assignablePrimitive(src Type, dst Primitive, strict bool, depth int) bool {
	switch dst.Kind {
	case KindAny:
		return true
	case KindNil:
		return IsNil(src)
	case KindBoolean:
		if lit, ok := src.(Literal); ok {
			_, isBool := lit.Value.(bool)
			return isBool
		}
		p, ok := src.(Primitive)
		return ok && p.Kind == KindBoolean
	case KindNumber, KindInteger:
		if lit, ok := src.(Literal); ok {
			switch lit.Value.(type) {
			case int, int64, float64:
				return true
			}
		}
		p, ok := src.(Primitive)
		return ok && (p.Kind == KindNumber || p.Kind == KindInteger)
	case KindString:
		if lit, ok := src.(Literal); ok {
			_, isString := lit.Value.(string)
			return isString
		}
		p, ok := src.(Primitive)
		return ok && p.Kind == KindString
	case KindTable:
		switch s := src.(type) {
		case Primitive:
			return s.Kind == KindTable
		case *Record, Record, Array, Map:
			return true
		}
		return false
	case KindFunction:
		switch src := src.(type) {
		case *Function, Function:
			return true
		case Primitive:
			return src.Kind == KindFunction
		}
		return false
	case KindThread:
		p, ok := src.(Primitive)
		return ok && p.Kind == KindThread
	case KindUserdata:
		p, ok := src.(Primitive)
		return ok && p.Kind == KindUserdata
	}
	return false
}

func assignableRecord(src Type, dst *Record, strict bool, depth int) bool {
	if dst == nil {
		return true
	}
	if dst.Enum {
		for _, field := range dst.Fields {
			if field != nil && assignable(src, field.Type, strict, depth+1) {
				return true
			}
		}
	}
	if srcRecord, ok := src.(*Record); ok && srcRecord != nil {
		if dst.Name != "" && srcRecord.Name != "" && CanonicalName(dst.Name) != CanonicalName(srcRecord.Name) && !recordInherits(srcRecord, dst) {
			return false
		}
		if dst.Exact && len(srcRecord.Fields) > 0 {
			for key := range srcRecord.Fields {
				if recordField(dst, key) == nil {
					return false
				}
			}
		}
		return recordFieldsAssignable(srcRecord, dst, strict, depth)
	}
	if srcRecord, ok := src.(Record); ok {
		return assignableRecord(&srcRecord, dst, strict, depth)
	}
	if p, ok := src.(Primitive); ok && p.Kind == KindTable {
		return true
	}
	return false
}

func recordInherits(src, dst *Record) bool {
	return recordInheritsDepth(src, dst, 0)
}

func recordInheritsDepth(src, dst *Record, depth int) bool {
	if src == nil || dst == nil || depth > maxTypeDepth {
		return false
	}
	for _, parent := range src.Parents {
		if parent == nil {
			continue
		}
		if CanonicalName(parent.Name) == CanonicalName(dst.Name) || recordInheritsDepth(parent, dst, depth+1) {
			return true
		}
	}
	return false
}

func recordFieldsAssignable(src, dst *Record, strict bool, depth int) bool {
	if depth > maxTypeDepth {
		return true
	}
	for key, dstField := range dst.Fields {
		srcField := recordField(src, key)
		if srcField == nil {
			if dstField.Optional || !strict {
				continue
			}
			return false
		}
		targetType := dstField.Type
		if dstField.Optional {
			targetType = Optional(targetType)
		}
		if !assignable(srcField.Type, targetType, strict, depth+1) {
			return false
		}
	}
	for _, parent := range dst.Parents {
		if !recordFieldsAssignable(src, parent, strict, depth+1) {
			return false
		}
	}
	return true
}

func assignableFunction(src Type, dst *Function, depth int) bool {
	if dst == nil {
		return true
	}
	var srcFn *Function
	switch s := src.(type) {
	case *Function:
		srcFn = s
	case Function:
		srcFn = &s
	case Primitive:
		if s.Kind == KindFunction {
			return true
		}
	}
	if srcFn == nil {
		return false
	}
	if len(srcFn.Returns.Elems) > 0 && len(dst.Returns.Elems) > 0 {
		for i, ret := range dst.Returns.Elems {
			if i >= len(srcFn.Returns.Elems) {
				break
			}
			if !assignable(srcFn.Returns.Elems[i], ret, false, depth+1) {
				return false
			}
		}
	}
	return true
}

func assignableTuple(src Type, dst Tuple, strict bool, depth int) bool {
	srcTuple, ok := src.(Tuple)
	if !ok {
		if len(dst.Elems) == 0 {
			return true
		}
		return assignable(src, dst.Elems[0], strict, depth+1)
	}
	for i, elem := range dst.Elems {
		if i >= len(srcTuple.Elems) {
			if !optionalTupleTail(srcTuple) {
				return false
			}
			continue
		}
		if !assignable(srcTuple.Elems[i], elem, strict, depth+1) {
			return false
		}
	}
	return true
}

func optionalTupleTail(tuple Tuple) bool {
	return tuple.Vararg != nil
}

func literalEqual(a, b any) bool {
	if a == b {
		return true
	}
	return Display(Literal{Value: a}) == Display(Literal{Value: b})
}

// FieldInfo describes one member available on a type.
type FieldInfo struct {
	Name      string
	Type      Type
	Optional  bool
	Ambiguous bool
	Decl      source.Span
}

// Fields returns the member names available on t. Unions expose the union of
// their members and mark fields missing from any member as ambiguous.
func Fields(t Type) []FieldInfo {
	return fields(t, 0)
}

func fields(t Type, depth int) []FieldInfo {
	if depth > maxTypeDepth {
		return nil
	}
	t = Resolve(t)
	switch v := t.(type) {
	case *Record:
		return recordFields(v)
	case Record:
		return recordFields(&v)
	case Union:
		return unionFields(v, depth)
	case *Alias:
		if v == nil {
			return nil
		}
		return fields(v.Underlying, depth+1)
	}
	return nil
}

func recordFields(record *Record) []FieldInfo {
	if record == nil {
		return nil
	}
	seen := make(map[string]FieldInfo)
	var add func(*Record, int)
	add = func(current *Record, depth int) {
		if current == nil || depth > maxTypeDepth {
			return
		}
		for key, field := range current.Fields {
			if field == nil {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = FieldInfo{
				Name:     field.Name,
				Type:     field.Type,
				Optional: field.Optional,
				Decl:     field.Decl,
			}
		}
		for _, parent := range current.Parents {
			add(parent, depth+1)
		}
	}
	add(record, 0)
	out := make([]FieldInfo, 0, len(seen))
	for _, field := range seen {
		out = append(out, field)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return CanonicalName(out[i].Name) < CanonicalName(out[j].Name)
	})
	return out
}

func unionFields(union Union, depth int) []FieldInfo {
	byName := make(map[string]FieldInfo)
	missing := make(map[string]int)
	for _, member := range union.Members {
		if IsNil(member) {
			continue
		}
		for _, field := range fields(member, depth+1) {
			key := CanonicalName(field.Name)
			existing, ok := byName[key]
			if !ok {
				byName[key] = field
				missing[key] = 0
				continue
			}
			existing.Type = UnionOf(existing.Type, field.Type)
			existing.Optional = existing.Optional || field.Optional
			existing.Ambiguous = existing.Ambiguous || field.Ambiguous
			byName[key] = existing
		}
	}
	for key := range byName {
		// Count how many members omitted the field. This is enough to mark
		// ambiguity for completion without walking the graph again.
		for _, member := range union.Members {
			if IsNil(member) {
				continue
			}
			record, ok := Resolve(member).(*Record)
			if !ok || recordField(record, key) == nil {
				missing[key]++
			}
		}
		field := byName[key]
		field.Ambiguous = field.Ambiguous || missing[key] > 0
		byName[key] = field
	}
	out := make([]FieldInfo, 0, len(byName))
	for _, field := range byName {
		out = append(out, field)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return CanonicalName(out[i].Name) < CanonicalName(out[j].Name)
	})
	return out
}

// LookupField returns the member named name on t. Optional is true when the
// field is declared optional; callers should treat the result as `T | nil`
// when Optional is true.
func LookupField(t Type, name string) (Type, bool, bool) {
	return field(t, CanonicalName(name), 0)
}

func field(t Type, canonical string, depth int) (Type, bool, bool) {
	if depth > maxTypeDepth {
		return AnyType, false, false
	}
	t = Resolve(t)
	switch v := t.(type) {
	case *Record:
		if field := recordField(v, canonical); field != nil {
			return field.Type, true, field.Optional
		}
	case Record:
		if field := recordField(&v, canonical); field != nil {
			return field.Type, true, field.Optional
		}
	case Union:
		var out []Type
		optional := false
		for _, member := range v.Members {
			if IsNil(member) {
				continue
			}
			memberType, ok, opt := field(member, canonical, depth+1)
			if !ok {
				return AnyType, false, false
			}
			out = append(out, memberType)
			optional = optional || opt
		}
		if len(out) == 0 {
			return AnyType, false, false
		}
		return UnionOf(out...), true, optional
	}
	return AnyType, false, false
}

func recordField(record *Record, canonical string) *Field {
	return recordFieldDepth(record, canonical, 0)
}

func recordFieldDepth(record *Record, canonical string, depth int) *Field {
	if record == nil || depth > maxTypeDepth {
		return nil
	}
	if field, ok := record.Fields[canonical]; ok {
		return field
	}
	for _, parent := range record.Parents {
		if field := recordFieldDepth(parent, canonical, depth+1); field != nil {
			return field
		}
	}
	return nil
}

// CallResult returns the first result type for a call expression.
func CallResult(fn Type) Type {
	fn = Resolve(fn)
	switch v := fn.(type) {
	case *Function:
		return First(v.Returns)
	case Function:
		return First(v.Returns)
	}
	return AnyType
}

// AsFunction returns the function type behind t, if any.
func AsFunction(t Type) (*Function, bool) {
	t = Resolve(t)
	switch v := t.(type) {
	case *Function:
		return v, true
	case Function:
		return &v, true
	}
	return nil, false
}

// Merge joins the types observed for one symbol across assignments.
func Merge(a, b Type) Type {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if IsAny(a) || IsAny(b) {
		return AnyType
	}
	return UnionOf(a, b)
}

// IsUnion reports whether t is a union with more than one member.
func IsUnion(t Type) bool {
	union, ok := Resolve(t).(Union)
	return ok && len(union.Members) > 1
}
