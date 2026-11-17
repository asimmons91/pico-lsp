package types

// NarrowByTag narrows a union of records to members whose tag field can or
// cannot equal value. It returns nil when t has no literal tag field and no
// narrowing is possible.
func NarrowByTag(t Type, field string, value any, positive bool) Type {
	if value == nil || field == "" {
		return nil
	}
	resolved := Resolve(t)
	switch record := resolved.(type) {
	case *Record:
		values, ok := tagValues(record, field)
		if !ok {
			return nil
		}
		matches := literalInSet(values, value)
		if positive {
			if matches {
				return record
			}
			return AnyType
		}
		if matches && len(values) == 1 {
			return AnyType
		}
		return record
	case Union:
		var narrowed []Type
		for _, member := range record.Members {
			if IsNil(member) {
				continue
			}
			memberRecord, ok := Resolve(member).(*Record)
			if !ok || memberRecord == nil {
				return nil
			}
			values, ok := tagValues(memberRecord, field)
			if !ok {
				return nil
			}
			matches := literalInSet(values, value)
			if positive {
				if matches {
					narrowed = append(narrowed, memberRecord)
				}
				continue
			}
			if matches && len(values) == 1 {
				continue
			}
			narrowed = append(narrowed, memberRecord)
		}
		if len(narrowed) == 0 {
			return AnyType
		}
		return UnionOf(narrowed...)
	}
	return nil
}

func tagValues(record *Record, field string) ([]any, bool) {
	if record == nil || field == "" {
		return nil, false
	}
	canonical := CanonicalName(field)
	tag := recordField(record, canonical)
	if tag == nil || tag.Optional {
		return nil, false
	}
	return literalValues(Resolve(tag.Type))
}

func literalValues(t Type) ([]any, bool) {
	switch value := t.(type) {
	case Literal:
		return []any{value.Value}, true
	case Union:
		out := make([]any, 0, len(value.Members))
		for _, member := range value.Members {
			literal, ok := Resolve(member).(Literal)
			if !ok {
				return nil, false
			}
			out = append(out, literal.Value)
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	}
	return nil, false
}

func literalInSet(values []any, candidate any) bool {
	for _, value := range values {
		if literalEqual(value, candidate) {
			return true
		}
	}
	return false
}
