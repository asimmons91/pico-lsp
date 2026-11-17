package types

import (
	"strings"
	"testing"
)

func genericRecord(name string, params ...*GenericParam) *Record {
	record := NewRecord(name)
	record.TypeParams = params
	return record
}

func TestM10InstantiateGenericRecord(t *testing.T) {
	box := genericRecord("Box", &GenericParam{ID: 1, Name: "T"})
	box.AddField(&Field{Name: "value", Type: box.TypeParams[0]})
	applied, err := Apply(box, []Type{NumberType})
	if err != nil {
		t.Fatal(err)
	}
	appliedRecord, ok := applied.(*Record)
	if !ok {
		t.Fatalf("applied type = %T", applied)
	}
	if got := Display(appliedRecord); got != "Box<number>" {
		t.Fatalf("display = %q", got)
	}
	fieldType, ok, _ := LookupField(appliedRecord, "value")
	if !ok || Display(fieldType) != "number" {
		t.Fatalf("value type = %s", Display(fieldType))
	}
}

func TestM10NestedInstantiation(t *testing.T) {
	box := genericRecord("Box", &GenericParam{ID: 1, Name: "T"})
	box.AddField(&Field{Name: "value", Type: box.TypeParams[0]})
	inner, err := Apply(box, []Type{NumberType})
	if err != nil {
		t.Fatal(err)
	}
	outer, err := Apply(box, []Type{inner})
	if err != nil {
		t.Fatal(err)
	}
	if got := Display(outer); got != "Box<Box<number>>" {
		t.Fatalf("nested display = %q", got)
	}
	fieldType, ok, _ := LookupField(outer, "value")
	if !ok || Display(fieldType) != "Box<number>" {
		t.Fatalf("nested value = %s", Display(fieldType))
	}
}

func TestM10GenericDefaultAndArity(t *testing.T) {
	box := genericRecord("Box", &GenericParam{ID: 1, Name: "T", Default: StringType})
	missing, err := Apply(box, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := Display(missing.(*Record).TypeArgs[0]); got != "string" {
		t.Fatalf("default arg = %q", got)
	}
	applied, err := Apply(box, []Type{NilType})
	if err != nil {
		t.Fatal(err)
	}
	if got := Display(applied.(*Record).TypeArgs[0]); got != "nil" {
		t.Fatalf("explicit nil arg = %q", got)
	}
	if _, err := Apply(box, []Type{NumberType, StringType}); err == nil {
		t.Fatal("too many type arguments accepted")
	}
	if _, err := Apply(NewRecord("Plain"), []Type{NumberType}); err == nil {
		t.Fatal("non-generic type accepted arguments")
	}
}

func TestM10BindFunctionGenerics(t *testing.T) {
	box := genericRecord("Box", &GenericParam{ID: 1, Name: "T"})
	box.AddField(&Field{Name: "value", Type: box.TypeParams[0]})
	param := box.TypeParams[0]
	fn := &Function{
		Generics: []*GenericParam{param},
		Params:   []Param{{Name: "value", Type: param}},
		Returns:  TupleOf(box),
	}
	specialized := BindFunction(fn, []Type{NumberType})
	if len(specialized.Generics) != 0 {
		t.Fatal("specialized function kept generic parameters")
	}
	if got := Display(specialized.Returns.Elems[0]); got != "Box<number>" {
		t.Fatalf("specialized return = %q", got)
	}
	if got := Display(specialized.Params[0].Type); got != "number" {
		t.Fatalf("specialized param = %q", got)
	}
}

func TestM10NarrowByTag(t *testing.T) {
	circle := NewRecord("Circle")
	circle.AddField(&Field{Name: "kind", Type: Literal{Value: "circle"}})
	circle.AddField(&Field{Name: "radius", Type: NumberType})
	square := NewRecord("Square")
	square.AddField(&Field{Name: "kind", Type: Literal{Value: "square"}})
	square.AddField(&Field{Name: "side", Type: NumberType})
	shape := UnionOf(circle, square)

	narrowed := NarrowByTag(shape, "kind", "circle", true)
	if got := Display(narrowed); got != "Circle" {
		t.Fatalf("positive narrowing = %q", got)
	}
	narrowed = NarrowByTag(shape, "kind", "circle", false)
	if got := Display(narrowed); got != "Square" {
		t.Fatalf("negative narrowing = %q", got)
	}
	if narrowed := NarrowByTag(shape, "radius", "circle", true); narrowed != nil {
		t.Fatalf("non-tag field narrowed to %s", Display(narrowed))
	}
}

func TestM10OperatorLookup(t *testing.T) {
	vec := NewRecord("Vec")
	vec.Operators = map[string]*Operator{
		"add": {Name: "add", Operand: vec, Result: vec},
	}
	child := NewRecord("VecChild")
	child.Parents = []*Record{vec}
	op, ok := LookupOperator(child, "add")
	if !ok || op.Result != vec {
		t.Fatalf("inherited operator = %+v, %v", op, ok)
	}
	applied, err := Apply(genericOperatorRecord(), []Type{NumberType})
	if err != nil {
		t.Fatal(err)
	}
	op, ok = LookupOperator(applied, "add")
	if !ok || Display(op.Result) != "Op<number>" {
		t.Fatalf("substituted operator = %s", Display(op.Result))
	}
}

func genericOperatorRecord() *Record {
	param := &GenericParam{ID: 9, Name: "T"}
	record := genericRecord("Op", param)
	record.Operators = map[string]*Operator{
		"add": {Name: "add", Operand: record, Result: record},
	}
	return record
}

func TestM10GenericDisplayTruncates(t *testing.T) {
	box := genericRecord("Box", &GenericParam{ID: 1, Name: "T"})
	box.AddField(&Field{Name: "value", Type: box.TypeParams[0]})
	var deep Type = NumberType
	for i := 0; i < 40; i++ {
		applied, err := Apply(box, []Type{deep})
		if err != nil {
			t.Fatal(err)
		}
		deep = applied
	}
	got := Display(deep)
	if !strings.Contains(got, "...") {
		t.Fatalf("deep generic display did not truncate: %q", got)
	}
	if len(got) > 400 {
		t.Fatalf("deep generic display too long: %d bytes", len(got))
	}
}

func TestM10RecursiveGenericRecord(t *testing.T) {
	node := genericRecord("Node", &GenericParam{ID: 1, Name: "T"})
	node.AddField(&Field{Name: "value", Type: node.TypeParams[0]})
	node.AddField(&Field{Name: "next", Type: node})
	applied := Instantiate(node, []Type{NumberType})
	next, ok, _ := LookupField(applied, "next")
	if !ok || Display(next) != "Node<number>" {
		t.Fatalf("recursive generic field = %s", Display(next))
	}
}
