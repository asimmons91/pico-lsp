package analysis

import (
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

func TestM9InheritanceLookupAndCompatibility(t *testing.T) {
	text := `---@class Entity
---@field x number
---@field hp number
---@field label? string
---@class Enemy: Entity
---@field damage number

---@type Enemy
local e = {x = 1, hp = 2, damage = 3}
local ex = e.x
local ed = e.damage
local el = e.label

---@param ent Entity
function use(ent)
  local entx = ent.x
end
use(e)

---@type Entity
local base = e
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	for _, code := range []string{CodeAssignTypeMismatch, CodeParamTypeMismatch, CodeReturnTypeMismatch, CodeMissingFields} {
		if hasDiagnostic(result, code) {
			t.Fatalf("inheritance produced %s: %+v", code, result.Diagnostics)
		}
	}
	for name, want := range map[string]string{
		"ex":   "number",
		"ed":   "number",
		"el":   "string?",
		"entx": "number",
	} {
		sym := symbolByName(result, name)
		if sym == nil {
			t.Fatalf("missing symbol %s", name)
		}
		if got := types.Display(sym.Type); got != want {
			t.Errorf("%s type = %q, want %q", name, got, want)
		}
	}
	enemy := result.Classes["enemy"]
	if enemy == nil || enemy.Record == nil || len(enemy.Record.Parents) != 1 || enemy.Record.Parents[0].Name != "Entity" {
		t.Fatalf("enemy parents = %+v", enemy)
	}
	if _, ok, _ := types.LookupField(enemy.Record, "hp"); !ok {
		t.Fatal("inherited hp missing from Enemy")
	}
}

func TestM9InheritanceDiagnostics(t *testing.T) {
	t.Run("unknown parent", func(t *testing.T) {
		text := `---@class Child: Missing
local c = {}
`
		result := analyzeTextMode(t, text, TypeCheckingBasic)
		diag := findDiagnostic(result, CodeAnnotationInvalid)
		if diag == nil || !strings.Contains(diag.Message, "unknown parent class") {
			t.Fatalf("diagnostics = %+v", result.Diagnostics)
		}
	})

	t.Run("non class parent", func(t *testing.T) {
		text := `---@alias Mode string
---@class Child: Mode
local c = {}
`
		result := analyzeTextMode(t, text, TypeCheckingBasic)
		diag := findDiagnostic(result, CodeAnnotationInvalid)
		if diag == nil || !strings.Contains(diag.Message, "not a parent class") {
			t.Fatalf("diagnostics = %+v", result.Diagnostics)
		}
	})

	t.Run("cycle", func(t *testing.T) {
		text := `---@class Alpha: Beta
---@class Beta: Alpha
local a = {}
local b = {}
`
		result := analyzeTextMode(t, text, TypeCheckingBasic)
		count := 0
		for _, diag := range result.Diagnostics {
			if diag.Code == CodeAnnotationInvalid && strings.Contains(diag.Message, "cycle") {
				count++
				if !strings.Contains(diag.Message, "Alpha") || !strings.Contains(diag.Message, "Beta") {
					t.Fatalf("cycle message = %q", diag.Message)
				}
			}
		}
		if count != 1 {
			t.Fatalf("cycle diagnostics = %d in %+v", count, result.Diagnostics)
		}
	})

	t.Run("self parent", func(t *testing.T) {
		text := `---@class Selfish: Selfish
local s = {}
`
		result := analyzeTextMode(t, text, TypeCheckingBasic)
		diag := findDiagnostic(result, CodeAnnotationInvalid)
		if diag == nil || !strings.Contains(diag.Message, "inherit itself") {
			t.Fatalf("diagnostics = %+v", result.Diagnostics)
		}
	})

	t.Run("invalid exact placement", func(t *testing.T) {
		text := `---@class Child: Parent (exact)
---@field x number
---@class Parent
local c = {}
`
		result := analyzeTextMode(t, text, TypeCheckingBasic)
		diag := findDiagnostic(result, CodeAnnotationInvalid)
		if diag == nil || !strings.Contains(diag.Message, "(exact) placement") {
			t.Fatalf("diagnostics = %+v", result.Diagnostics)
		}
	})
}

func TestM9ExactClasses(t *testing.T) {
	text := `---@class (exact) Point
---@field x number
---@field y number
---@field note? string
local Point = {}

---@type Point
local ok = {x = 1, y = 2}

---@type Point
local missing = {x = 1}

---@type Point
local extra = {x = 1, y = 2, z = 3}

---@class (exact) Labeled: Point
---@field label string
---@type Labeled
local inheritedOk = {x = 1, y = 2, label = "ok"}

---@type Labeled
local inheritedMissing = {x = 1, label = "ok", note = "n"}

---@class (exact) Clicker
---@field hits number
local Clicker = {}
function Clicker:click()
  self.hits += 1
end
---@type Clicker
local clicker = {hits = 0}
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	missingDiag := findDiagnostic(result, CodeMissingFields)
	if missingDiag == nil || !strings.Contains(missingDiag.Message, "y") {
		t.Fatalf("missing fields diagnostics = %+v", result.Diagnostics)
	}
	countMissing := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == CodeMissingFields {
			countMissing++
		}
	}
	if countMissing != 2 {
		t.Fatalf("missing-fields count = %d in %+v", countMissing, result.Diagnostics)
	}
	extraDiag := findDiagnostic(result, CodeAssignTypeMismatch)
	if extraDiag == nil || !strings.Contains(extraDiag.Message, `field "z" does not exist`) {
		t.Fatalf("extra field diagnostics = %+v", result.Diagnostics)
	}
	for _, diag := range result.Diagnostics {
		if diag.Code == CodeMissingFields && strings.Contains(diag.Message, "note") {
			t.Fatalf("optional field reported missing: %+v", diag)
		}
	}
}

func TestM9ExactClassDefinitionLiteralIsNotAnInstance(t *testing.T) {
	text := `---@class (exact) Point
---@field x number
---@field y number
local Point = {}
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if hasDiagnostic(result, CodeMissingFields) || hasDiagnostic(result, CodeAssignTypeMismatch) {
		t.Fatalf("class definition table produced diagnostics: %+v", result.Diagnostics)
	}
	point := symbolByName(result, "Point")
	if point == nil || types.Display(point.Type) != "Point" {
		t.Fatalf("Point type = %v", point)
	}
}

func TestM9MetatableInference(t *testing.T) {
	text := `---@class Point
---@field x number
---@field y number
local Point = {}

local a = setmetatable({}, {__index = Point})
local ax = a.x

local b = setmetatable({}, Point)
local by = b.y

local mt = {__index = Point}
local c = setmetatable({}, mt)
local cx = c.x

---@return Point
local function indexer(t, k) return Point end
local d = setmetatable({}, {__index = indexer})
local dx = d.x

local ambiguous = setmetatable({}, {})
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	for name, want := range map[string]string{
		"a":  "Point",
		"b":  "Point",
		"c":  "Point",
		"d":  "Point",
		"ax": "number",
		"by": "number",
		"cx": "number",
		"dx": "number",
	} {
		sym := symbolByName(result, name)
		if sym == nil {
			t.Fatalf("missing symbol %s", name)
		}
		if got := types.Display(sym.Type); got != want {
			t.Errorf("%s type = %q, want %q", name, got, want)
		}
	}
	if ambiguous := symbolByName(result, "ambiguous"); ambiguous == nil || strings.Contains(types.Display(ambiguous.Type), "Point") {
		t.Fatalf("ambiguous setmetatable inferred a class: %v", ambiguous)
	}
}

func TestM9Narrowing(t *testing.T) {
	text := `---@class Player
---@field hp number

---@param p Player?
function guarded(p)
  local ga = p
  assert(p)
  local gb = p
  local ghp = p.hp
  local gc = p or {hp = 1}
  local gd = p and p.hp
end

---@param q Player?
function assigned(q)
  q = q or {hp = 1}
  local aa = q
end

---@param r Player?
function reset(r)
  assert(r)
  r = nil
  local ra = r
end

---@param p Player?
function earlyReturns(p)
  if not p then return end
  local ea = p
  if p == nil then return end
  local eb = p
end

---@param v number|string
function typeNarrow(v)
  if type(v) == "number" then
    local ta = v
  else
    local tb = v
  end
  if type(v) ~= "number" then return end
  local tc = v
end

---@param p Player?
function assertedValue(p)
  local va = assert(p)
  local vb = va
end

---@param p Player?
function castNarrow(p)
  ---@cast p Player
  local ca = p
end
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	for name, want := range map[string]string{
		"ga":  "Player?",
		"gb":  "Player",
		"ghp": "number",
		"gc":  "Player",
		"gd":  "number",
		"aa":  "Player",
		"ra":  "nil",
		"ea":  "Player",
		"eb":  "Player",
		"ta":  "number",
		"tb":  "string",
		"tc":  "number",
		"va":  "Player",
		"vb":  "Player",
		"ca":  "Player",
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

func TestM9NarrowingScopeIsolation(t *testing.T) {
	text := `---@class Player
---@field hp number

---@param p Player?
function outer(p)
  local oa = p
  local function inner()
    assert(p)
    local narrowed = p
  end
  local ob = p
end

---@param p Player?
function loopNarrow(p)
  while p do
    local loopInside = p
    p = nil
  end
  local loopAfter = p
end

---@param p Player?
function branchMerge(p, flag)
  if flag then
    p = nil
  end
  local mergeAfter = p
end

---@param p Player?
function branchReturn(p)
  if p then
    local thenValue = p
  else
    return
  end
  local returnAfter = p
end
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	// The inner function's assertion must not leak to the outer scope.
	if got := types.Display(symbolByName(result, "oa").Type); got != "Player?" {
		t.Errorf("oa type = %q, want Player?", got)
	}
	if got := types.Display(symbolByName(result, "ob").Type); got != "Player?" {
		t.Errorf("ob type = %q, want Player? (narrowing leaked across a function)", got)
	}
	if got := types.Display(symbolByName(result, "loopInside").Type); got != "Player" {
		t.Errorf("loopInside type = %q, want Player", got)
	}
	if got := types.Display(symbolByName(result, "loopAfter").Type); got != "Player?" {
		t.Errorf("loopAfter type = %q, want Player?", got)
	}
	if got := types.Display(symbolByName(result, "mergeAfter").Type); got != "Player?" {
		t.Errorf("mergeAfter type = %q, want Player?", got)
	}
	if got := types.Display(symbolByName(result, "thenValue").Type); got != "Player" {
		t.Errorf("thenValue type = %q, want Player", got)
	}
	if got := types.Display(symbolByName(result, "returnAfter").Type); got != "Player" {
		t.Errorf("returnAfter type = %q, want Player", got)
	}
}

func TestM9CaseSensitiveStringKeys(t *testing.T) {
	text := `---@class Info
---@field health number

---@type Info
local i = {}
local lower = i["health"]
local upper = i["Health"]

local t = {Health = 1}
local dot = t.health
local stringLower = t["health"]
local stringUpper = t["Health"]
local literal = {["Foo"] = 2}
local exactString = literal["Foo"]
local foldedString = literal["foo"]
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	for name, want := range map[string]string{
		"lower":        "number",
		"upper":        "any",
		"dot":          "number",
		"stringLower":  "number",
		"stringUpper":  "any",
		"exactString":  "number",
		"foldedString": "any",
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

func TestM9InheritedMethods(t *testing.T) {
	text := `---@class Entity
---@field hp number
local Entity = {}
---@param amount number
function Entity:heal(amount)
  self.hp += amount
end

---@class Enemy: Entity
---@field damage number
local Enemy = {}
local e = setmetatable({hp = 1, damage = 2}, {__index = Enemy})
e:heal(3)
e:heal("bad")
e.heal("worse")
local obj = setmetatable({}, {__index = Enemy})
---@type fun(x: string)
obj.heal = function(x) return x end
obj:heal("ok")
local method = e.heal
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	paramDiagnostics := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == CodeParamTypeMismatch {
			paramDiagnostics++
		}
	}
	if paramDiagnostics != 2 {
		t.Fatalf("inherited method argument mismatches = %d, want 2: %+v", paramDiagnostics, result.Diagnostics)
	}
	enemy := result.Classes["enemy"]
	if enemy == nil || enemy.Record == nil {
		t.Fatal("missing Enemy class")
	}
	methodType, ok, _ := types.LookupField(enemy.Record, "heal")
	if !ok {
		t.Fatal("inherited method heal missing from Enemy")
	}
	fn, ok := types.AsFunction(methodType)
	if !ok {
		t.Fatalf("heal type = %s, want function", types.Display(methodType))
	}
	if len(fn.Params) != 1 || fn.Params[0].Name != "amount" {
		t.Fatalf("heal params = %+v", fn.Params)
	}
	if got := types.Display(symbolByName(result, "method").Type); got != "fun(amount: any)" && !strings.HasPrefix(got, "fun(") {
		t.Fatalf("method type = %q", got)
	}
	info, ok := types.LookupFieldInfo(enemy.Record, "heal")
	if !ok || info.Owner == nil || info.Owner.Name != "Entity" {
		t.Fatalf("heal owner = %+v", info.Owner)
	}

	off := analyzeTextMode(t, text, TypeCheckingOff)
	offEnemy := off.Classes["enemy"]
	if offEnemy == nil || offEnemy.Record == nil {
		t.Fatal("missing off-mode Enemy class")
	}
	if _, ok, _ := types.LookupField(offEnemy.Record, "heal"); !ok {
		t.Fatal("off mode dropped inherited method lookup")
	}
}

func TestM9AnnotatedFixture(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "annotated", "objects.p8"))
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

func TestM9ParseClassHeader(t *testing.T) {
	cases := []struct {
		input  string
		name   string
		exact  bool
		parent []string
		valid  bool
	}{
		{input: " Point", name: "Point", valid: true},
		{input: " (exact) Point", name: "Point", exact: true, valid: true},
		{input: " Point (exact)", name: "Point", exact: true, valid: true},
		{input: " Child: Parent", name: "Child", parent: []string{"Parent"}, valid: true},
		{input: " Child: Parent, Other", name: "Child", parent: []string{"Parent", "Other"}, valid: true},
		{input: " Child: Parent<T>", name: "Child", parent: []string{"Parent"}, valid: true},
		{input: " (exact) Child: Parent", name: "Child", exact: true, parent: []string{"Parent"}, valid: true},
		{input: " Child: Parent (exact)", name: "Child", parent: []string{"Parent"}, valid: false},
		{input: " (exact)", name: "", valid: true},
		{input: " (exact) (exact) Point", name: "Point", exact: true, valid: false},
	}
	for _, tc := range cases {
		name, exact, parents, valid := parseClassHeader(tc.input)
		if name != tc.name || exact != tc.exact || valid != tc.valid {
			t.Errorf("parseClassHeader(%q) = %q, %v, %v, %v", tc.input, name, exact, parents, valid)
			continue
		}
		if len(parents) != len(tc.parent) {
			t.Errorf("parseClassHeader(%q) parents = %v, want %v", tc.input, parents, tc.parent)
			continue
		}
		for i := range parents {
			if parents[i] != tc.parent[i] {
				t.Errorf("parseClassHeader(%q) parents = %v, want %v", tc.input, parents, tc.parent)
				break
			}
		}
	}
}

func TestM9AliasParentAndOverriddenField(t *testing.T) {
	text := `---@class Base
---@field value number
---@alias BaseRef Base
---@class Child: BaseRef
---@field value integer
---@type Child
local c = {value = 1}
local v = c.value
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	child := result.Classes["child"]
	if child == nil || child.Record == nil || len(child.Record.Parents) != 1 || child.Record.Parents[0].Name != "Base" {
		t.Fatalf("child parents = %+v", child)
	}
	if got := types.Display(symbolByName(result, "v").Type); got != "integer" {
		t.Fatalf("overridden field type = %q, want integer", got)
	}
}

func TestM9AnnotationNamesHandleExactAndParents(t *testing.T) {
	comments := []syntax.Comment{
		{Kind: syntax.CommentDoc, Text: "---@class (exact) Point", Line: 0},
		{Kind: syntax.CommentDoc, Text: "---@class Enemy: Entity", Line: 1},
		{Kind: syntax.CommentDoc, Text: "---@alias Mode string", Line: 2},
	}
	names := AnnotationNames(comments)
	want := []string{"Point", "Enemy", "Mode"}
	if len(names) != len(want) {
		t.Fatalf("annotation names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("annotation names = %v, want %v", names, want)
		}
	}
}
