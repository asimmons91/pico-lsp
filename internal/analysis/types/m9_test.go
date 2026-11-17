package types

import "testing"

func TestM9ExactRequiresInheritedFieldsInBasic(t *testing.T) {
	parent := NewRecord("Parent")
	parent.AddField(&Field{Name: "x", Type: NumberType})
	child := NewRecord("Child")
	child.Exact = true
	child.Parents = []*Record{parent}
	child.AddField(&Field{Name: "y", Type: NumberType})

	partial := NewRecord("")
	partial.AddField(&Field{Name: "x", Type: NumberType})
	if Assignable(partial, child, false) {
		t.Fatal("exact child accepted a literal missing its own field")
	}

	missingParent := NewRecord("")
	missingParent.AddField(&Field{Name: "y", Type: NumberType})
	if Assignable(missingParent, child, false) {
		t.Fatal("exact child accepted a literal missing an inherited field")
	}

	complete := NewRecord("")
	complete.AddField(&Field{Name: "x", Type: NumberType})
	complete.AddField(&Field{Name: "y", Type: NumberType})
	if !Assignable(complete, child, false) {
		t.Fatal("exact child rejected a complete literal")
	}

	open := NewRecord("Open")
	open.Parents = []*Record{parent}
	open.AddField(&Field{Name: "y", Type: NumberType})
	if !Assignable(partial, open, false) {
		t.Fatal("open class required missing fields in basic mode")
	}
}

func TestM9FieldInfoReportsDeclaringClass(t *testing.T) {
	parent := NewRecord("Parent")
	parent.AddField(&Field{Name: "x", Type: NumberType})
	child := NewRecord("Child")
	child.Parents = []*Record{parent}
	child.AddField(&Field{Name: "y", Type: NumberType})

	info, ok := LookupFieldInfo(child, "x")
	if !ok {
		t.Fatal("missing inherited field x")
	}
	if info.Owner != parent {
		t.Fatalf("x owner = %v, want Parent", info.Owner)
	}
	info, ok = LookupFieldInfo(child, "y")
	if !ok || info.Owner != child {
		t.Fatalf("y owner = %+v, want Child", info)
	}
}

func TestM9StringKeysAreCaseSensitive(t *testing.T) {
	record := NewRecord("Info")
	record.AddField(&Field{Name: "health", Type: NumberType})
	record.AddField(&Field{Name: "Foo", Type: StringType, StringKey: true})

	if _, ok, _ := LookupFieldExact(record, "health"); !ok {
		t.Fatal("identifier field did not match its folded key")
	}
	if _, ok, _ := LookupFieldExact(record, "Health"); ok {
		t.Fatal("identifier field matched a differently cased string key")
	}
	if typ, ok, _ := LookupFieldExact(record, "Foo"); !ok || Display(typ) != "string" {
		t.Fatal("string key did not match exactly")
	}
	if _, ok, _ := LookupFieldExact(record, "foo"); ok {
		t.Fatal("string key matched a differently cased key")
	}
	if _, ok, _ := LookupField(record, "Foo"); ok {
		t.Fatal("dot lookup matched a string key")
	}
}

func TestM9TruthinessParts(t *testing.T) {
	if got := Display(TruthyPart(Optional(NumberType))); got != "number" {
		t.Fatalf("truthy optional = %q", got)
	}
	if got := Display(Falsy(Optional(NumberType))); got != "nil" {
		t.Fatalf("falsy optional = %q", got)
	}
	if TruthyPart(Literal{Value: false}) != nil {
		t.Fatal("false literal has a truthy part")
	}
	if got := Display(Falsy(Literal{Value: false})); got != "false" {
		t.Fatalf("falsy false literal = %q", got)
	}
	if got := Display(Falsy(BooleanType)); got != "boolean" {
		t.Fatalf("falsy boolean = %q", got)
	}
	if Falsy(NumberType) != nil || Falsy(StringType) != nil {
		t.Fatal("number/string should be always truthy")
	}
}
