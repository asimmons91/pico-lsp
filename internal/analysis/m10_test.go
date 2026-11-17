package analysis

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/cart"
)

func TestM10GenericFunctionsAndClasses(t *testing.T) {
	text := `---@class Box<T>
---@field value T

---@generic T
---@param value T
---@return Box<T>
local function box(value)
  return {value = value}
end

local numbers = box(1)
local strings = box("x")
local nv = numbers.value
local sv = strings.value

---@type Box<number>
local direct = {value = 3}
local dv = direct.value

---@type Box<Box<number>>
local nested = {value = {value = 4}}
local nestedValue = nested.value.value
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	for _, diag := range result.Diagnostics {
		if diag.Code == CodeAnnotationUnknownType {
			t.Fatalf("generic class produced an unknown-type diagnostic: %+v", diag)
		}
	}
	for name, want := range map[string]string{
		"numbers":     "Box<number>",
		"strings":     "Box<string>",
		"nv":          "number",
		"sv":          "string",
		"direct":      "Box<number>",
		"dv":          "number",
		"nested":      "Box<Box<number>>",
		"nestedValue": "number",
	} {
		sym := symbolByName(result, name)
		if sym == nil {
			t.Fatalf("missing symbol %s", name)
		}
		if got := types.Display(sym.Type); got != want {
			t.Errorf("%s type = %q, want %q", name, got, want)
		}
	}
}

func TestM10GenericShadowing(t *testing.T) {
	text := `---@class Box<T>
---@field value T

---@generic T
---@param value T
---@return Box<T>
local function box(value)
  return {value = value}
end

local result = box(1)
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if got := types.Display(symbolByName(result, "result").Type); got != "Box<number>" {
		t.Fatalf("result type = %q", got)
	}
	box := symbolByName(result, "box")
	fn, ok := types.AsFunction(box.Type)
	if !ok || len(fn.Generics) != 1 || fn.Generics[0].Name != "T" {
		t.Fatalf("box generics = %+v", fn)
	}
}

func TestM10GenericDefaults(t *testing.T) {
	text := `---@class Box<T = number>
---@field value T

---@type Box
local boxed = {}
local value = boxed.value
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if got := types.Display(symbolByName(result, "boxed").Type); got != "Box<number>" {
		t.Fatalf("boxed type = %q", got)
	}
	if got := types.Display(symbolByName(result, "value").Type); got != "number" {
		t.Fatalf("defaulted field = %q", got)
	}
}

func TestM10GenericParentInstantiation(t *testing.T) {
	text := `---@class Pair<K, V>
---@field key K
---@field value V
---@class NamedPair<V>: Pair<string, V>
---@field assigned boolean

---@type NamedPair<number>
local pair = {key = "a", value = 1, assigned = true}
local keyType = pair.key
local valueType = pair.value

---@type NamedPair<number>
local empty = {}
local emptyKey = empty.key
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if got := types.Display(symbolByName(result, "keyType").Type); got != "string" {
		t.Fatalf("key type = %q", got)
	}
	if got := types.Display(symbolByName(result, "valueType").Type); got != "number" {
		t.Fatalf("value type = %q", got)
	}
	if got := types.Display(symbolByName(result, "emptyKey").Type); got != "string" {
		t.Fatalf("inherited generic parent field = %q", got)
	}
}

func TestM10GenericDiagnostics(t *testing.T) {
	text := `---@class Box<T>
---@field value T

---@type Box<number, string>
local bad = {}
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	diag := findDiagnostic(result, CodeAnnotationInvalid)
	if diag == nil || !strings.Contains(diag.Message, "type arguments") {
		t.Fatalf("generic arity diagnostics = %+v", result.Diagnostics)
	}
}

func TestM10Overloads(t *testing.T) {
	text := `---@overload fun(x: number): number
---@overload fun(x: string): string
local function identity(x)
  return x
end

local n = identity(1)
local s = identity("x")
local unknown = identity(true)
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	for name, want := range map[string]string{
		"n":       "number",
		"s":       "string",
		"unknown": "any",
	} {
		sym := symbolByName(result, name)
		if sym == nil {
			t.Fatalf("missing symbol %s", name)
		}
		if got := types.Display(sym.Type); got != want {
			t.Errorf("%s type = %q, want %q", name, got, want)
		}
	}
	if hasDiagnostic(result, CodeParamTypeMismatch) {
		t.Fatalf("ambiguous overload produced a diagnostic: %+v", result.Diagnostics)
	}
}

func TestM10OverloadClearMismatch(t *testing.T) {
	text := `---@overload fun(x: number): number
---@overload fun(x: number, y: number): number
local function add(x, y)
  return x + y
end

add("bad")
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if !hasDiagnostic(result, CodeParamTypeMismatch) {
		t.Fatalf("clear overload mismatch missing: %+v", result.Diagnostics)
	}
}

func TestM10Operators(t *testing.T) {
	text := `---@class Vec
---@field x number
---@operator add(Vec): Vec
---@operator sub(Vec): Vec
---@operator unm(): Vec
---@operator eq(Vec): boolean
---@operator call(x: number): string
local Vec = {}

local a = setmetatable({x = 1}, {__index = Vec})
local b = setmetatable({x = 2}, {__index = Vec})
local sum = a + b
local difference = a - b
local negative = -a
local equal = a == b
local called = a(1)
local field = sum.x
local bad = a + 1
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	for name, want := range map[string]string{
		"sum":        "Vec",
		"difference": "Vec",
		"negative":   "Vec",
		"equal":      "boolean",
		"called":     "string",
		"field":      "number",
		"bad":        "Vec",
	} {
		sym := symbolByName(result, name)
		if sym == nil {
			t.Fatalf("missing symbol %s", name)
		}
		if got := types.Display(sym.Type); got != want {
			t.Errorf("%s type = %q, want %q", name, got, want)
		}
	}
	diag := findDiagnostic(result, CodeOperatorMismatch)
	if diag == nil || diag.Severity != SeverityHint {
		t.Fatalf("operator mismatch diagnostics = %+v", result.Diagnostics)
	}
}

func TestM10BuiltinsIgnoreOperators(t *testing.T) {
	text := "local x = 1 + 2\nlocal y = #\"abc\"\n"
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if got := types.Display(symbolByName(result, "x").Type); got != "number" {
		t.Fatalf("builtin add type = %q", got)
	}
	if got := types.Display(symbolByName(result, "y").Type); got != "number" {
		t.Fatalf("builtin length type = %q", got)
	}
}

func TestM10TaggedUnionNarrowing(t *testing.T) {
	text := `---@class Shape
---@field kind "circle" | "square"
---@class Circle: Shape
---@field kind "circle"
---@field radius number
---@class Square: Shape
---@field kind "square"
---@field side number

---@type Circle | Square
local shape = {}

if shape.kind == "circle" then
  local radius = shape.radius
  local kind = shape.kind
else
  local side = shape.side
end

if shape.kind ~= "square" then return end
local after = shape.side

if shape.kind == "circle" then
  shape.kind = "square"
  local mutated = shape.radius
end
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	for name, want := range map[string]string{
		"radius":  "number",
		"side":    "number",
		"after":   "number",
		"mutated": "any",
	} {
		sym := symbolByName(result, name)
		if sym == nil {
			t.Fatalf("missing symbol %s", name)
		}
		if got := types.Display(sym.Type); got != want {
			t.Errorf("%s type = %q, want %q", name, got, want)
		}
	}
	needle := "local kind = shape.kind"
	offset := strings.Index(text, needle) + len("local kind = shape.")
	if offset < 0 {
		t.Fatal("missing tag read")
	}
	occ := result.OccurrenceAt(0, offset)
	if occ == nil {
		t.Fatal("missing tag occurrence")
	}
	flow, ok := result.FlowType(occ.Span)
	if !ok || types.Display(flow) != `"circle"` {
		t.Fatalf("narrowed tag flow type = %s (%v)", types.Display(flow), ok)
	}
}

func TestM10TaggedUnionControlFlow(t *testing.T) {
	text := `---@class Shape
---@field kind "circle" | "square"
---@class Circle: Shape
---@field kind "circle"
---@field radius number
---@class Square: Shape
---@field kind "square"
---@field side number

---@param shape Circle | Square
local function inspect(shape)
  if shape.kind == "circle" and shape.kind ~= "square" then
    local both = shape.radius
  end
  while shape.kind == "circle" do
    local looped = shape.radius
    break
  end
  if shape.kind ~= "square" then return end
  local guarded = shape.side
end

---@class A
---@field kind 1
---@field ax number
---@class B
---@field kind 2
---@field bx number

---@type A | B
local item = {}
if item.kind == 1 then
  local ax = item.ax
end
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	for name, want := range map[string]string{
		"both":    "number",
		"looped":  "number",
		"guarded": "number",
		"ax":      "number",
	} {
		sym := symbolByName(result, name)
		if sym == nil {
			t.Fatalf("missing symbol %s", name)
		}
		if got := types.Display(sym.Type); got != want {
			t.Errorf("%s type = %q, want %q", name, got, want)
		}
	}
}

func TestM10InheritedOverloads(t *testing.T) {
	text := `---@class Base
local Base = {}
---@overload fun(x: number): number
---@overload fun(x: string): string
function Base:convert(x) end

---@class Child: Base
local Child = {}
local child = setmetatable({}, {__index = Child})
local n = child:convert(1)
local s = child:convert("x")
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if got := types.Display(symbolByName(result, "n").Type); got != "number" {
		t.Errorf("inherited number overload = %q", got)
	}
	if got := types.Display(symbolByName(result, "s").Type); got != "string" {
		t.Errorf("inherited string overload = %q", got)
	}
	if hasDiagnostic(result, CodeParamTypeMismatch) {
		t.Fatalf("inherited overloads produced a mismatch: %+v", result.Diagnostics)
	}
}

func TestM10GenericOperatorSubstitution(t *testing.T) {
	text := `---@class Num<T>
---@field value T
---@operator add(Num<T>): Num<T>
local Num = {}

---@type Num<number>
local a = {value = 1}
---@type Num<number>
local b = {value = 2}
local c = a + b
local value = c.value
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if got := types.Display(symbolByName(result, "c").Type); got != "Num<number>" {
		t.Fatalf("generic operator result = %q", got)
	}
	if got := types.Display(symbolByName(result, "value").Type); got != "number" {
		t.Fatalf("generic operator field = %q", got)
	}
}

func TestM10AnnotatedFixture(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "annotated", "m10.p8"))
	if err != nil {
		t.Fatal(err)
	}
	loader := cart.NewLoader(nil, "", nil)
	snap, err := loader.Load(uri.File(path))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	result := Analyze(snap, testBuiltins(t), Options{UndefinedGlobal: SeverityWarning, TypeChecking: TypeCheckingBasic})
	for _, diag := range result.Diagnostics {
		if isTypeDiagnosticCode(diag.Code) {
			t.Errorf("annotated fixture produced %s: %s", diag.Code, diag.Message)
		}
	}
}

func TestM10GenericFunctionDefault(t *testing.T) {
	text := `---@generic T = number
---@return T
local function make()
  return 1
end
local made = make()
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if got := types.Display(symbolByName(result, "made").Type); got != "number" {
		t.Fatalf("generic default return = %q", got)
	}
}

func TestM10GenericClassMethods(t *testing.T) {
	text := `---@class Box<T>
---@field value T
local Box = {}
---@return T
function Box:get()
  return self.value
end

---@type Box<number>
local boxed = {value = 1}
local got = boxed:get()
local direct = boxed.get
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if hasDiagnostic(result, CodeAnnotationUnknownType) {
		t.Fatalf("generic method produced an unknown-type diagnostic: %+v", result.Diagnostics)
	}
	if got := types.Display(symbolByName(result, "got").Type); got != "number" {
		t.Fatalf("generic method result = %q", got)
	}
	if got := types.Display(symbolByName(result, "direct").Type); !strings.Contains(got, "number") {
		t.Fatalf("generic method value = %q", got)
	}
}

func TestM10GenericOverload(t *testing.T) {
	text := `---@generic T
---@overload fun(x: T): T
local function identity(x)
  return x
end
local n = identity(1)
local s = identity("x")
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if got := types.Display(symbolByName(result, "n").Type); got != "number" {
		t.Errorf("generic overload number = %q", got)
	}
	if got := types.Display(symbolByName(result, "s").Type); got != "string" {
		t.Errorf("generic overload string = %q", got)
	}
}

func TestM10MalformedAnnotations(t *testing.T) {
	text := `---@class Bad
---@operator bogus(Bad): Bad
---@operator add(Bad)
---@generic
---@overload number
local value = 1
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if !hasDiagnostic(result, CodeAnnotationInvalid) {
		t.Fatalf("malformed generic/operator/overload annotations were accepted: %+v", result.Diagnostics)
	}
}

func TestM10GenericClassCycle(t *testing.T) {
	text := `---@class A<T>: B<T>
---@class B<T>: A<T>
local a = {}
local b = {}
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	diag := findDiagnostic(result, CodeAnnotationInvalid)
	if diag == nil || !strings.Contains(diag.Message, "cycle") {
		t.Fatalf("generic cycle diagnostics = %+v", result.Diagnostics)
	}
}

func TestM10ExactGenericInheritance(t *testing.T) {
	text := `---@class Pair<K, V>
---@field key K
---@field value V
---@class (exact) NamedPair<V>: Pair<string, V>
---@field assigned boolean

---@type NamedPair<number>
local ok = {key = "a", value = 1, assigned = true}

---@type NamedPair<number>
local bad = {key = "a", assigned = true}
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if hasDiagnosticOnText(result, text, "local ok =") {
		t.Fatalf("complete exact generic literal produced a diagnostic: %+v", result.Diagnostics)
	}
	if !hasDiagnosticOnText(result, text, "local bad =") {
		t.Fatalf("missing exact generic parent field was accepted: %+v", result.Diagnostics)
	}
}

func TestM10HexTaggedUnion(t *testing.T) {
	text := `---@class Red
---@field kind 0x1
---@field red number
---@class Blue
---@field kind 0x2
---@field blue number

---@type Red | Blue
local item = {}
if item.kind == 0x1 then
  local red = item.red
end
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if got := types.Display(symbolByName(result, "red").Type); got != "number" {
		t.Fatalf("hex tag narrowing = %q", got)
	}
}

func TestM10GenericInstanceFieldAssignment(t *testing.T) {
	text := `---@class Box<T>
---@field value T

---@type Box<number>
local boxed = {value = 1}
boxed.value = "bad"
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if !hasDiagnostic(result, CodeAssignTypeMismatch) {
		t.Fatalf("generic instance field assignment was not checked: %+v", result.Diagnostics)
	}
}

func TestM10GenericClassTag(t *testing.T) {
	text := `---@generic T
---@class Box
---@field value T
local Box = {}

---@type Box<number>
local boxed = {value = 1}
local value = boxed.value
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if hasDiagnostic(result, CodeAnnotationUnknownType) {
		t.Fatalf("class @generic tag produced unknown types: %+v", result.Diagnostics)
	}
	if got := types.Display(symbolByName(result, "value").Type); got != "number" {
		t.Fatalf("class @generic field = %q", got)
	}
}

func TestM10AllOperatorKinds(t *testing.T) {
	text := `---@class AllOps
---@operator add(AllOps): AllOps
---@operator sub(AllOps): AllOps
---@operator mul(AllOps): AllOps
---@operator div(AllOps): AllOps
---@operator idiv(AllOps): AllOps
---@operator mod(AllOps): AllOps
---@operator pow(AllOps): AllOps
---@operator unm(): AllOps
---@operator band(AllOps): AllOps
---@operator bor(AllOps): AllOps
---@operator bxor(AllOps): AllOps
---@operator bnot(): AllOps
---@operator shl(AllOps): AllOps
---@operator shr(AllOps): AllOps
---@operator rotl(AllOps): AllOps
---@operator rotr(AllOps): AllOps
---@operator concat(AllOps): string
---@operator len(): number
---@operator lt(AllOps): boolean
---@operator le(AllOps): boolean
---@operator eq(AllOps): boolean
---@operator call(): string
local AllOps = {}

local a = setmetatable({}, {__index = AllOps})
local b = setmetatable({}, {__index = AllOps})
local r01 = a + b
local r02 = a - b
local r03 = a * b
local r04 = a / b
local r05 = a \ b
local r06 = a % b
local r07 = a ^ b
local r08 = -a
local r09 = a & b
local r10 = a | b
local r11 = a ^^ b
local r12 = ~a
local r13 = a << b
local r14 = a >> b
local r15 = a <<> b
local r16 = a >>< b
local r17 = a .. b
local r18 = #a
local r19 = a < b
local r20 = a <= b
local r21 = a == b
local r22 = a > b
local r23 = a >= b
local r24 = a()
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	for i := 1; i <= 24; i++ {
		name := fmt.Sprintf("r%02d", i)
		sym := symbolByName(result, name)
		if sym == nil {
			t.Fatalf("missing symbol %s", name)
		}
		want := "AllOps"
		switch i {
		case 17, 24:
			want = "string"
		case 18:
			want = "number"
		case 19, 20, 21, 22, 23:
			want = "boolean"
		}
		if got := types.Display(sym.Type); got != want {
			t.Errorf("%s type = %q, want %q", name, got, want)
		}
	}
	if hasDiagnostic(result, CodeOperatorMismatch) {
		t.Fatalf("complete operator set produced a mismatch: %+v", result.Diagnostics)
	}
}
