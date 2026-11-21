package analysis

import (
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

func analyzeTextMode(t *testing.T, text string, mode TypeCheckingMode) *Result {
	t.Helper()
	u := uri.File("/test/cart.lua")
	file := &cart.File{ID: 0, URI: u, Text: text, Index: source.New([]byte(text), protocol.PositionEncodingKindUTF16)}
	c := &cart.Cart{RootURI: u, Files: []*cart.File{file}}
	lexed := syntax.Lex([]byte(text), func(start, end int) source.Span {
		return source.Span{FileID: 0, Start: start, End: end}
	})
	chunk, parseErrors := syntax.Parse(lexed.Tokens)
	if len(lexed.Errors) > 0 || len(parseErrors) > 0 {
		t.Fatalf("parse failed: lex=%v parse=%v", lexed.Errors, parseErrors)
	}
	snap := &cart.Snapshot{Cart: c, Chunk: chunk, Comments: lexed.Comments, Index: syntax.NewCommentIndex(lexed.Comments)}
	return Analyze(snap, testBuiltins(t), Options{UndefinedGlobal: SeverityWarning, TypeChecking: mode})
}

func findDiagnostic(result *Result, code string) *Diagnostic {
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Code == code {
			return &result.Diagnostics[i]
		}
	}
	return nil
}

func hasDiagnostic(result *Result, code string) bool {
	return findDiagnostic(result, code) != nil
}

func TestM4DemoAnnotationsAndInference(t *testing.T) {
	text := `---@class Player
---@field x number
---@field y number
---@field hp integer
---@field sprite integer

---@param p Player
---@param dx number
---@param dy number
---@return boolean moved
local function move(p, dx, dy)
    p.x += dx
    p.y += dy
    return dx != 0 or dy != 0
end

---@type Player
local player = { x = 64, y = 64, hp = 3, sprite = 1 }

if (btn(0)) move(player, -1, 0)
move(player, "left", 0)
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)

	move := result.Table.Globals["move"]
	if move == nil {
		// move is a local function in this snippet; find it by name.
		for _, sym := range result.Table.Symbols {
			if sym.Name == "move" {
				move = sym
				break
			}
		}
	}
	if move == nil {
		t.Fatal("missing move symbol")
	}
	fn, ok := types.AsFunction(move.Type)
	if !ok {
		t.Fatalf("move type = %t", move.Type)
	}
	if len(fn.Params) != 3 || types.Display(fn.Params[0].Type) != "Player" || types.Display(fn.Params[1].Type) != "number" {
		t.Fatalf("move params = %+v", fn.Params)
	}
	if len(fn.Returns.Elems) != 1 || types.Display(fn.Returns.Elems[0]) != "boolean" {
		t.Fatalf("move returns = %+v", fn.Returns)
	}
	player := result.Table.Globals["player"]
	if player == nil {
		for _, sym := range result.Table.Symbols {
			if sym.Name == "player" {
				player = sym
				break
			}
		}
	}
	if player == nil || types.Display(player.Type) != "Player" {
		t.Fatalf("player type = %v", player)
	}
	diag := findDiagnostic(result, CodeParamTypeMismatch)
	if diag == nil || !strings.Contains(diag.Message, `"left"`) {
		t.Fatalf("param diagnostic = %+v", diag)
	}
}

func TestM4AnnotationDiagnostics(t *testing.T) {
	text := `---@param x Missing
---@param x number
---@param nope number
function f(x) end

---@bogus thing
local y = 1

---@field
local z = 1

---@generic T
---@overload fun(x: number)
---@nodiscard
local deferred = 1
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	for _, code := range []string{
		CodeAnnotationUnknownType,
		CodeAnnotationDuplicate,
		CodeAnnotationUnknownParam,
		CodeAnnotationInvalid,
		CodeAnnotationMissingName,
	} {
		if !hasDiagnostic(result, code) {
			t.Errorf("missing %s in %+v", code, result.Diagnostics)
		}
	}
	for _, diag := range result.Diagnostics {
		if strings.Contains(diag.Message, "@generic") || strings.Contains(diag.Message, "@overload") || strings.Contains(diag.Message, "@nodiscard") {
			t.Fatalf("deferred annotation produced a diagnostic: %+v", diag)
		}
	}
}

func TestM4AliasesEnumsCastsAndSuppression(t *testing.T) {
	text := `---@alias Mode "left" | "right"
---@enum Direction
local Direction = {up = 1, down = 2}

---@type Mode
local m = "left"

---@type Direction
local d = Direction.up

local v = 1
---@cast v string
local s = v

local n = 1 --[[@as number | nil]]

---@type number
---@diagnostic disable-next-line: assign-type-mismatch
local suppressed = "x"

---@type number
local reported = "y"

local r = Direction.down
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if enum := result.Enums["direction"]; enum == nil || types.Display(enum) != "Direction" {
		t.Fatalf("missing enum Direction: %+v", result.Enums)
	}
	if got := types.Display(result.Aliases["mode"].Underlying); got != `"left" | "right"` && got != `"right" | "left"` {
		t.Fatalf("alias Mode = %q", got)
	}
	if display := types.Display(symbolByName(result, "m").Type); display != "Mode" {
		t.Fatalf("m type = %q", display)
	}
	if display := types.Display(symbolByName(result, "d").Type); display != "Direction" {
		t.Fatalf("d type = %q", display)
	}
	if display := types.Display(symbolByName(result, "s").Type); display != "string" {
		t.Fatalf("cast s type = %q", display)
	}
	if display := types.Display(symbolByName(result, "n").Type); display != "number?" {
		t.Fatalf("@as n type = %q", display)
	}
	if hasDiagnosticOnText(result, text, "suppressed") {
		t.Fatalf("suppressed diagnostic was emitted: %+v", result.Diagnostics)
	}
	if !hasDiagnosticOnText(result, text, "reported") {
		t.Fatalf("reported diagnostic missing: %+v", result.Diagnostics)
	}
}

func TestM4InferenceLocalsFunctionsAndIterators(t *testing.T) {
	text := `local x = 1
local s = "hi"
local t = {a = x, b = s}
local function add(a, b) return a + b end
local sum = add(1, 2)
local q = add
local u = t.a
local n = #t
local c = btn(0)
local values = {10, 20}
for i, v in ipairs(values) do
  local loop = v + i
end
local splits = split("a,b", ",")
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	want := map[string]string{
		"x":      "number",
		"s":      "string",
		"t":      "{a: number, b: string}",
		"add":    "fun(a: number, b: number): number",
		"sum":    "number",
		"q":      "fun(a: number, b: number): number",
		"u":      "number",
		"n":      "number",
		"c":      "boolean",
		"values": "number[]",
		"splits": "string[]",
	}
	for name, wantType := range want {
		sym := symbolByName(result, name)
		if sym == nil {
			t.Errorf("missing symbol %q", name)
			continue
		}
		if got := types.Display(sym.Type); got != wantType {
			t.Errorf("%s type = %q, want %q", name, got, wantType)
		}
	}
}

func TestM4IteratorValueMismatch(t *testing.T) {
	text := `local values = {10, 20}
add(values, 30)
add(values, "bad")
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if !hasDiagnostic(result, CodeParamTypeMismatch) {
		t.Fatalf("missing add element mismatch: %+v", result.Diagnostics)
	}
}

func TestM4Narrowing(t *testing.T) {
	text := `---@param x number?
---@param y number?
function f(x, y)
  if x then
    local a = x
  end
  if y == nil then return end
  local b = y
  if x ~= nil then
    local c = x
  end
  if type(y) == "number" then
    local d = y
  end
  if type(x) ~= "number" then return end
  local e = x
end
f(1, 2)
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	for name, want := range map[string]string{
		"a": "number",
		"b": "number",
		"c": "number",
		"d": "number",
		"e": "number",
	} {
		if got := types.Display(symbolByName(result, name).Type); got != want {
			t.Errorf("%s type = %q, want %q", name, got, want)
		}
	}
}

func TestM4DiagnosticsModes(t *testing.T) {
	text := `---@class Player (exact)
---@field x number
---@field hp number

---@param p Player?
function use(p)
  local a = p.x
end

---@type Player
local p = {x = 1}

local q = p.hp
local r = p.nope
`
	basic := analyzeTextMode(t, text, TypeCheckingBasic)
	if !hasDiagnostic(basic, CodeUndefinedField) {
		t.Fatalf("basic exact-class undefined field missing: %+v", basic.Diagnostics)
	}
	// M9 makes missing required fields a warning for explicitly exact classes
	// even in basic mode; nil checks remain strict-only.
	if !hasDiagnostic(basic, CodeMissingFields) {
		t.Fatalf("basic exact-class missing fields missing: %+v", basic.Diagnostics)
	}
	if hasDiagnostic(basic, CodeNeedCheckNil) {
		t.Fatalf("basic emitted strict nil diagnostics: %+v", basic.Diagnostics)
	}
	strict := analyzeTextMode(t, text, TypeCheckingStrict)
	if !hasDiagnostic(strict, CodeNeedCheckNil) || !hasDiagnostic(strict, CodeMissingFields) || !hasDiagnostic(strict, CodeUndefinedField) {
		t.Fatalf("strict diagnostics = %+v", strict.Diagnostics)
	}
	off := analyzeTextMode(t, text, TypeCheckingOff)
	for _, diag := range off.Diagnostics {
		if isTypeDiagnosticCode(diag.Code) {
			t.Fatalf("off emitted type diagnostic %+v", diag)
		}
	}
}

func TestM4AssignmentParamReturnAndDeprecated(t *testing.T) {
	text := `---@type number
local x = "bad"

---@param p number
function g(p) end
g("bad")

---@return number
function h() return "bad" end

---@deprecated
function old() end
old()
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	for _, code := range []string{CodeAssignTypeMismatch, CodeParamTypeMismatch, CodeReturnTypeMismatch, CodeDeprecated} {
		if !hasDiagnostic(result, code) {
			t.Errorf("missing %s in %+v", code, result.Diagnostics)
		}
	}
	deprecated := findDiagnostic(result, CodeDeprecated)
	if deprecated == nil || deprecated.Severity != SeverityHint {
		t.Fatalf("deprecated severity = %+v", deprecated)
	}
}

func TestM4MetaSuppressesTypeDiagnostics(t *testing.T) {
	text := `---@meta
---@type number
local x = "bad"
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	for _, diag := range result.Diagnostics {
		if isTypeDiagnosticCode(diag.Code) {
			t.Fatalf("meta file emitted %+v", diag)
		}
	}
}

func symbolByName(result *Result, name string) *Symbol {
	if result == nil || result.Table == nil {
		return nil
	}
	canonical := CanonicalName(name)
	var field *Symbol
	for _, sym := range result.Table.Symbols {
		if sym == nil || sym.Canonical != canonical {
			continue
		}
		if sym.Owner == nil {
			return sym
		}
		if field == nil {
			field = sym
		}
	}
	if field != nil {
		return field
	}
	for name, sym := range result.Table.Builtins {
		if name == canonical {
			return sym
		}
	}
	return nil
}

func hasDiagnosticOnText(result *Result, text, needle string) bool {
	offset := strings.Index(text, needle)
	if offset < 0 {
		return false
	}
	lineStart := strings.LastIndexByte(text[:offset], '\n') + 1
	lineEnd := strings.IndexByte(text[offset:], '\n')
	if lineEnd < 0 {
		lineEnd = len(text)
	} else {
		lineEnd += offset
	}
	for _, diag := range result.Diagnostics {
		if diag.Span.Start >= lineStart && diag.Span.Start < lineEnd {
			return true
		}
	}
	return false
}

func TestM4MultipleReturns(t *testing.T) {
	text := `---@return number, string
function pair()
  return 1, "x"
end
local a, b = pair()
local c = pair()
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	for name, want := range map[string]string{"a": "number", "b": "string", "c": "number"} {
		sym := symbolByName(result, name)
		if sym == nil {
			t.Fatalf("missing %s", name)
		}
		if got := types.Display(sym.Type); got != want {
			t.Errorf("%s type = %q, want %q", name, got, want)
		}
	}
}

func TestM4ClassInheritanceAndFieldScope(t *testing.T) {
	text := `---@class Base
---@field x number
---@class Child: Base
---@field [public] y? string

---@type Child
local c = {x = 1, y = "ok"}

---@param b Base
function use(b) end
use(c)
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if hasDiagnostic(result, CodeParamTypeMismatch) || hasDiagnostic(result, CodeAssignTypeMismatch) {
		t.Fatalf("inheritance produced type errors: %+v", result.Diagnostics)
	}
	child := result.Classes["child"]
	if child == nil || child.Record == nil {
		t.Fatal("missing Child class")
	}
	if len(child.Record.Parents) != 1 || child.Record.Parents[0].Name != "Base" {
		t.Fatalf("Child parents = %+v", child.Record.Parents)
	}
	if _, ok, _ := types.LookupField(child.Record, "y"); !ok {
		t.Fatal("optional field y missing")
	}
}

func TestM4FileDiagnosticSuppression(t *testing.T) {
	text := `---@diagnostic disable: assign-type-mismatch
---@type number
local first = "bad"
---@diagnostic enable: assign-type-mismatch
---@type number
local second = "bad"
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == CodeAssignTypeMismatch {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("assign diagnostics = %d, want 1 (%+v)", count, result.Diagnostics)
	}
}

func TestM4ClassFieldAssignmentMismatch(t *testing.T) {
	text := `---@class Player
---@field x number
---@type Player
local p = {x = 1}
p.x = "bad"
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if !hasDiagnostic(result, CodeAssignTypeMismatch) {
		t.Fatalf("missing field assignment mismatch: %+v", result.Diagnostics)
	}
}

func TestM4UninitializedExplicitLocal(t *testing.T) {
	text := "---@type number\nlocal x\n"
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if hasDiagnostic(result, CodeAssignTypeMismatch) {
		t.Fatalf("uninitialized annotated local warned: %+v", result.Diagnostics)
	}
}

func TestM4CastModesAndAsExpression(t *testing.T) {
	text := `local v = 1
---@cast v +string
local widened = v
---@cast v -number
local narrowed = v
---@type number
local forced = "bad" --[[@as number]]
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if got := types.Display(symbolByName(result, "widened").Type); got != "number | string" {
		t.Fatalf("widened type = %q", got)
	}
	if got := types.Display(symbolByName(result, "narrowed").Type); got != "string" {
		t.Fatalf("narrowed type = %q", got)
	}
	if hasDiagnostic(result, CodeAssignTypeMismatch) {
		t.Fatalf("@as did not override the expression type: %+v", result.Diagnostics)
	}
}

func TestM4UncalledFunctionParamAndRecursion(t *testing.T) {
	text := `local function unused(a)
  return a
end
local function recur(n)
  if n <= 0 then return 0 end
  return recur(n - 1)
end
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	unused := symbolByName(result, "unused")
	fn, ok := types.AsFunction(unused.Type)
	if !ok || len(fn.Params) != 1 || types.Display(fn.Params[0].Type) != "any" {
		t.Fatalf("unused type = %s", types.Display(unused.Type))
	}
	recur := symbolByName(result, "recur")
	if _, ok := types.AsFunction(recur.Type); !ok {
		t.Fatalf("recursive function type = %s", types.Display(recur.Type))
	}
}

func TestM4DisableLineSuppression(t *testing.T) {
	text := "---@type number\nlocal x = \"bad\" ---@diagnostic disable-line: assign-type-mismatch\n"
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if hasDiagnostic(result, CodeAssignTypeMismatch) {
		t.Fatalf("disable-line did not suppress: %+v", result.Diagnostics)
	}
}

func TestM4OptionalParamAcceptsNil(t *testing.T) {
	text := `---@param x number?
function f(x) end
f(nil)
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if hasDiagnostic(result, CodeParamTypeMismatch) {
		t.Fatalf("optional parameter rejected nil: %+v", result.Diagnostics)
	}
}

func TestM4ReturnTypeUnionAcrossStatements(t *testing.T) {
	text := `local function mixed(flag)
  if flag then return 1 end
  return "x"
end
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	mixed := symbolByName(result, "mixed")
	fn, ok := types.AsFunction(mixed.Type)
	if !ok || len(fn.Returns.Elems) != 1 {
		t.Fatalf("mixed returns = %s", types.Display(mixed.Type))
	}
	if got := types.Display(fn.Returns.Elems[0]); got != "number | string" {
		t.Fatalf("mixed return type = %q", got)
	}
}

func TestM4MapAndArrayAnnotations(t *testing.T) {
	text := `---@type table<string, number>
local m = {a = 1}
local mv = m["a"]
---@type number[]
local xs = {}
local xv = xs[1]
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if hasDiagnostic(result, CodeAssignTypeMismatch) {
		t.Fatalf("map/array assignment warned: %+v", result.Diagnostics)
	}
	if got := types.Display(symbolByName(result, "mv").Type); got != "number" {
		t.Fatalf("map value = %q", got)
	}
	if got := types.Display(symbolByName(result, "xv").Type); got != "number" {
		t.Fatalf("array value = %q", got)
	}
}

func TestM4ForwardGlobalFunctionInference(t *testing.T) {
	text := `local x = later(1)
function later(a)
  return a
end
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if got := types.Display(symbolByName(result, "x").Type); got != "number" {
		t.Fatalf("forward call type = %q, want number", got)
	}
}

func TestM4StrictNilArithmetic(t *testing.T) {
	text := `---@param x number?
function f(x)
  return x + 1
end
`
	basic := analyzeTextMode(t, text, TypeCheckingBasic)
	if hasDiagnostic(basic, CodeNeedCheckNil) {
		t.Fatalf("basic emitted need-check-nil: %+v", basic.Diagnostics)
	}
	strict := analyzeTextMode(t, text, TypeCheckingStrict)
	if !hasDiagnostic(strict, CodeNeedCheckNil) {
		t.Fatalf("strict missed nil arithmetic: %+v", strict.Diagnostics)
	}
}

func TestM4ReturnTupleFromCall(t *testing.T) {
	text := `---@return number, number
function pair() return 1, 2 end
---@return number, string
function f() return pair() end
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if !hasDiagnostic(result, CodeReturnTypeMismatch) {
		t.Fatalf("missing tuple return mismatch: %+v", result.Diagnostics)
	}
}

func TestM4DiagnosticDirectiveWithoutSpace(t *testing.T) {
	text := "---@type number\n---@diagnostic disable-next-line:assign-type-mismatch\nlocal x = \"bad\"\n"
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if hasDiagnostic(result, CodeAssignTypeMismatch) {
		t.Fatalf("compact @diagnostic did not suppress: %+v", result.Diagnostics)
	}
}

func TestM4VarargCallSiteInference(t *testing.T) {
	text := `local function first(a, ...)
  return ...
end
first(1, "x", 2)
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	fn, ok := types.AsFunction(symbolByName(result, "first").Type)
	if !ok || fn.Vararg == nil {
		t.Fatalf("first type = %s", types.Display(symbolByName(result, "first").Type))
	}
	if got := types.Display(fn.Vararg); got != "number | string" {
		t.Fatalf("vararg type = %q", got)
	}
}

func TestM4CyclicAliasesDoNotPanic(t *testing.T) {
	text := `---@alias A B
---@alias B A
---@type A
local x = 1
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	if result == nil {
		t.Fatal("nil result")
	}
}

func TestM4CyclicTableTypeDisplay(t *testing.T) {
	text := `local t = {}
t.self = t
`
	result := analyzeTextMode(t, text, TypeCheckingBasic)
	got := types.Display(symbolByName(result, "t").Type)
	if got == "" || len(got) > 500 {
		t.Fatalf("cyclic table display = %q", got)
	}
}

func TestM4StrictNilNarrowing(t *testing.T) {
	text := `---@param p {x: number}?
function f(p)
  if p then
    local a = p.x
  end
  if p == nil then return end
  local b = p.x
end
`
	result := analyzeTextMode(t, text, TypeCheckingStrict)
	if hasDiagnostic(result, CodeNeedCheckNil) {
		t.Fatalf("narrowed field access warned: %+v", result.Diagnostics)
	}
}
