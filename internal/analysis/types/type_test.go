package types

import (
	"testing"

	"github.com/asimmons91/pico-lsp/internal/source"
)

func TestDisplay(t *testing.T) {
	player := NewRecord("Player")
	player.AddField(&Field{Name: "x", Type: NumberType})
	fn := &Function{
		Params:  []Param{{Name: "p", Type: player}, {Name: "dx", Type: NumberType, Optional: true}},
		Returns: TupleOf(BooleanType),
	}
	cases := []struct {
		typ  Type
		want string
	}{
		{AnyType, "any"},
		{Optional(StringType), "string?"},
		{UnionOf(NumberType, StringType), "number | string"},
		{Array{Elem: NumberType}, "number[]"},
		{Map{Key: StringType, Elem: NumberType}, "table<string, number>"},
		{TupleOf(NumberType, StringType), "[number, string]"},
		{Literal{Value: "left"}, `"left"`},
		{Literal{Value: float64(3)}, "3"},
		{player, "Player"},
		{fn, "fun(p: Player, dx?: number): boolean"},
	}
	for _, tc := range cases {
		if got := Display(tc.typ); got != tc.want {
			t.Errorf("Display(%T) = %q, want %q", tc.typ, got, tc.want)
		}
	}
}

func TestUnionCanonicalization(t *testing.T) {
	union := UnionOf(StringType, NumberType, StringType, AnyType)
	if got := Display(union); got != "any" {
		t.Fatalf("any union = %q", got)
	}
	union = UnionOf(NumberType, StringType, NumberType)
	got := Display(union)
	if got != "number | string" {
		t.Fatalf("union display = %q", got)
	}
	optional := Optional(NewRecord("Player"))
	if !ContainsNil(optional) {
		t.Fatal("optional missing nil")
	}
	if got := Display(RemoveNil(optional)); got != "Player" {
		t.Fatalf("RemoveNil display = %q", got)
	}
}

func TestAssignable(t *testing.T) {
	player := NewRecord("Player")
	player.AddField(&Field{Name: "x", Type: NumberType})
	player.AddField(&Field{Name: "hp", Type: NumberType})
	partial := NewRecord("")
	partial.AddField(&Field{Name: "x", Type: NumberType})
	extra := NewRecord("")
	extra.AddField(&Field{Name: "x", Type: NumberType})
	extra.AddField(&Field{Name: "other", Type: StringType})

	cases := []struct {
		name   string
		src    Type
		dst    Type
		strict bool
		want   bool
	}{
		{"any source", AnyType, NumberType, false, true},
		{"any target", StringType, AnyType, false, true},
		{"literal number", Literal{Value: float64(3)}, NumberType, false, true},
		{"literal string", Literal{Value: "x"}, StringType, false, true},
		{"literal mismatch", Literal{Value: "x"}, NumberType, false, false},
		{"integer number", IntegerType, NumberType, false, true},
		{"optional nil", NilType, Optional(NumberType), false, true},
		{"partial record basic", partial, player, false, true},
		{"partial record strict", partial, player, true, false},
		{"record extra open", extra, player, false, true},
		{"function", &Function{Returns: TupleOf(BooleanType)}, FunctionType, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Assignable(tc.src, tc.dst, tc.strict); got != tc.want {
				t.Fatalf("Assignable(%s, %s, strict=%v) = %v, want %v", Display(tc.src), Display(tc.dst), tc.strict, got, tc.want)
			}
		})
	}

	withOptional := NewRecord("OptionalFields")
	withOptional.AddField(&Field{Name: "x", Type: NumberType, Optional: true})
	optionalValue := NewRecord("")
	optionalValue.AddField(&Field{Name: "x", Type: NilType})
	if !Assignable(optionalValue, withOptional, true) {
		t.Fatal("optional field rejected nil in strict mode")
	}
	record := NewRecord("")
	record.AddField(&Field{Name: "a", Type: NumberType})
	if Assignable(record, Map{Key: NumberType, Elem: NumberType}, false) {
		t.Fatal("string-keyed record accepted as table<number, number>")
	}

	exact := NewRecord("Exact")
	exact.Exact = true
	exact.AddField(&Field{Name: "x", Type: NumberType})
	if Assignable(extra, exact, false) {
		t.Fatal("exact record accepted an unknown field in basic mode")
	}
}

func TestFieldsFromUnionAndEnum(t *testing.T) {
	left := NewRecord("")
	left.AddField(&Field{Name: "a", Type: NumberType})
	left.AddField(&Field{Name: "shared", Type: NumberType})
	right := NewRecord("")
	right.AddField(&Field{Name: "b", Type: StringType})
	right.AddField(&Field{Name: "shared", Type: StringType})
	fields := Fields(UnionOf(left, right))
	byName := make(map[string]FieldInfo)
	for _, field := range fields {
		byName[field.Name] = field
	}
	if byName["shared"].Ambiguous {
		t.Fatal("shared field marked ambiguous although both members have it")
	}
	if !byName["a"].Ambiguous || !byName["b"].Ambiguous {
		t.Fatalf("union fields = %+v, want a/b ambiguous", fields)
	}

	enum := NewRecord("Direction")
	enum.Enum = true
	enum.AddField(&Field{Name: "up", Type: Literal{Value: float64(1)}, Decl: source.Span{}})
	if !Assignable(Literal{Value: float64(1)}, enum, false) {
		t.Fatal("enum member literal not assignable to enum")
	}
	if Assignable(Literal{Value: float64(2)}, enum, false) {
		t.Fatal("unknown enum literal accepted")
	}
}

func TestWiden(t *testing.T) {
	if got := Display(Widen(Literal{Value: "x"})); got != "string" {
		t.Fatalf("widen string literal = %q", got)
	}
	if got := Display(Widen(Optional(Literal{Value: "x"}))); got != "string?" {
		t.Fatalf("widen optional literal = %q", got)
	}
}

func TestCyclicRecordsTerminate(t *testing.T) {
	a := NewRecord("A")
	b := NewRecord("B")
	a.Parents = []*Record{b}
	b.Parents = []*Record{a}
	a.AddField(&Field{Name: "x", Type: NumberType})
	for i := 0; i < 2; i++ {
		_ = Fields(a)
		_ = Assignable(a, b, true)
		if _, ok, _ := LookupField(b, "x"); !ok {
			t.Fatal("LookupField did not find inherited field")
		}
	}
}
