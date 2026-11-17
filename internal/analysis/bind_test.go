package analysis

import (
	"strings"
	"sync"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/builtin"
	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

var (
	builtinsOnce sync.Once
	builtinsMap  map[string]*Symbol
	builtinsErr  error
)

func testBuiltins(t testing.TB) map[string]*Symbol {
	t.Helper()
	builtinsOnce.Do(func() {
		api, err := builtin.Load()
		if err != nil {
			builtinsErr = err
			return
		}
		builtinsMap = BuildBuiltins(api)
	})
	if builtinsErr != nil {
		t.Fatalf("load builtins: %v", builtinsErr)
	}
	return builtinsMap
}

func analyzeText(t *testing.T, text string) (*Result, *cart.Snapshot) {
	t.Helper()
	u := uri.File("/test/cart.lua")
	file := &cart.File{
		ID:    0,
		URI:   u,
		Text:  text,
		Index: source.New([]byte(text), protocol.PositionEncodingKindUTF16),
	}
	c := &cart.Cart{RootURI: u, Files: []*cart.File{file}}
	lexed := syntax.Lex([]byte(text), func(start, end int) source.Span {
		return source.Span{FileID: 0, Start: start, End: end}
	})
	chunk, parseErrors := syntax.Parse(lexed.Tokens)
	if len(lexed.Errors) > 0 || len(parseErrors) > 0 {
		t.Fatalf("parse failed: lex=%v parse=%v", lexed.Errors, parseErrors)
	}
	snap := &cart.Snapshot{
		Cart:     c,
		Chunk:    chunk,
		Comments: lexed.Comments,
		Index:    syntax.NewCommentIndex(lexed.Comments),
	}
	return Analyze(snap, testBuiltins(t), Options{UndefinedGlobal: SeverityWarning}), snap
}

func symbolAtText(t *testing.T, result *Result, text, needle string) *Symbol {
	t.Helper()
	offset := strings.Index(text, needle)
	if offset < 0 {
		t.Fatalf("needle %q not found", needle)
	}
	occ := result.OccurrenceAt(0, offset)
	if occ == nil {
		t.Fatalf("no occurrence at %q (offset %d)", needle, offset)
	}
	return occ.Symbol
}

func diagnosticsWithCode(result *Result, code string) []Diagnostic {
	var out []Diagnostic
	for _, diag := range result.Diagnostics {
		if diag.Code == code {
			out = append(out, diag)
		}
	}
	return out
}

func TestScopeLocalInitializerSeesOuter(t *testing.T) {
	text := "x = 1\nlocal x = x\nprint(x)\n"
	result, _ := analyzeText(t, text)

	global := result.Table.Globals["x"]
	if global == nil {
		t.Fatal("missing global x")
	}
	localUse := result.OccurrenceAt(0, strings.Index(text, "print(x)")+len("print("))
	if localUse == nil || localUse.Symbol == nil {
		t.Fatal("missing local use")
	}
	if localUse.Symbol == global {
		t.Fatal("print(x) resolved to the global; want the local")
	}
	rhs := result.OccurrenceAt(0, strings.LastIndex(text, "= x")+2)
	if rhs == nil || rhs.Symbol != global {
		t.Fatalf("local initializer rhs = %+v, want global x", rhs)
	}
}

func TestScopeLocalFunctionRecurses(t *testing.T) {
	text := "local function f() return f() end\n"
	result, _ := analyzeText(t, text)
	decl := result.OccurrenceAt(0, strings.Index(text, "f()"))
	if decl == nil || decl.Symbol == nil {
		t.Fatal("missing f declaration")
	}
	use := result.OccurrenceAt(0, strings.LastIndex(text, "f()"))
	if use == nil || use.Symbol != decl.Symbol {
		t.Fatalf("recursive f = %+v, want declaration %+v", use, decl)
	}
}

func TestScopeLoopVariableDoesNotLeak(t *testing.T) {
	text := "for i = 1, 3 do end\nprint(i)\n"
	result, _ := analyzeText(t, text)
	if len(diagnosticsWithCode(result, CodeUndefinedGlobal)) != 1 {
		t.Fatalf("diagnostics = %+v, want one undefined-global", result.Diagnostics)
	}
}

func TestScopeRepeatConditionSeesBodyLocals(t *testing.T) {
	text := "repeat local x = 1 until x\n"
	result, _ := analyzeText(t, text)
	if len(diagnosticsWithCode(result, CodeUndefinedGlobal)) != 0 {
		t.Fatalf("diagnostics = %+v, want none", result.Diagnostics)
	}
}

func TestScopeBranchesDoNotLeak(t *testing.T) {
	text := "if true then local x = 1 end\nprint(x)\n"
	result, _ := analyzeText(t, text)
	if len(diagnosticsWithCode(result, CodeUndefinedGlobal)) != 1 {
		t.Fatalf("diagnostics = %+v, want one undefined-global", result.Diagnostics)
	}
}

func TestScopeMethodSelf(t *testing.T) {
	text := "t = {}\nfunction t:m() return self end\n"
	result, _ := analyzeText(t, text)
	if len(diagnosticsWithCode(result, CodeUndefinedGlobal)) != 0 {
		t.Fatalf("diagnostics = %+v, want none", result.Diagnostics)
	}
	tSym := result.Table.Globals["t"]
	if tSym == nil {
		t.Fatal("missing t")
	}
	m := tSym.Fields["m"]
	if m == nil || m.Kind != SymbolMethod {
		t.Fatalf("method m = %+v", m)
	}
	self := result.OccurrenceAt(0, strings.Index(text, "self end"))
	if self == nil || self.Symbol == nil || self.Symbol.Kind != SymbolParam {
		t.Fatalf("self occurrence = %+v, want param", self)
	}
}

func TestMethodSelfSharesReceiverFields(t *testing.T) {
	text := "t = {}\nfunction t:m()\n  self.x = 1\nend\nprint(t.x)\n"
	result, _ := analyzeText(t, text)
	tSym := result.Table.Globals["t"]
	if tSym == nil || tSym.Fields["x"] == nil {
		t.Fatalf("t fields = %v, want x created through self", fieldNames(tSym))
	}
	field := tSym.Fields["x"]
	occ := result.OccurrenceAt(0, strings.Index(text, "t.x")+len("t."))
	if occ == nil || occ.Symbol != field {
		t.Fatalf("t.x occurrence = %+v, want shared field %+v", occ, field)
	}
	if len(field.Decls) != 1 || len(field.Uses) != 1 {
		t.Fatalf("field = %+v, want one decl and one use", field)
	}
}

func TestGlobalsForwardReferenceAndMultipleDecls(t *testing.T) {
	text := "function f() return g() end\nfunction g() end\nx = 1\nx = 2\nprint(x)\n"
	result, _ := analyzeText(t, text)
	g := result.Table.Globals["g"]
	if g == nil {
		t.Fatal("missing g")
	}
	use := result.OccurrenceAt(0, strings.Index(text, "g()"))
	if use == nil || use.Symbol != g {
		t.Fatalf("g use = %+v, want global g", use)
	}
	x := result.Table.Globals["x"]
	if x == nil || len(x.Decls) != 2 {
		t.Fatalf("x decls = %+v, want two", x)
	}
}

func TestBuiltinShadowing(t *testing.T) {
	text := "btn = 1\nprint(btn)\n"
	result, _ := analyzeText(t, text)
	if len(diagnosticsWithCode(result, CodeUndefinedGlobal)) != 0 {
		t.Fatalf("diagnostics = %+v, want none", result.Diagnostics)
	}
	sym := symbolAtText(t, result, text, "btn = 1")
	if sym == nil || sym.Builtin {
		t.Fatalf("btn = %+v, want user global", sym)
	}
}

func TestCaseFolding(t *testing.T) {
	text := "Player = 1\nprint(player)\n"
	result, _ := analyzeText(t, text)
	if len(result.Table.Globals) != 1 {
		t.Fatalf("globals = %+v, want one", result.Table.Globals)
	}
	sym := result.Table.Globals["player"]
	if sym == nil || sym.Name != "Player" {
		t.Fatalf("symbol = %+v", sym)
	}
	if len(sym.Uses) != 1 {
		t.Fatalf("uses = %+v, want one", sym.Uses)
	}
}

func TestFieldsAndDynamicAccess(t *testing.T) {
	text := "t = {x = 1, inner = {y = 2}}\nprint(t.x, t.inner.y)\nt.z = 3\nfunction t:m() return self end\nt[k] = 4\n"
	result, _ := analyzeText(t, text)
	tSym := result.Table.Globals["t"]
	if tSym == nil {
		t.Fatal("missing t")
	}
	for _, name := range []string{"x", "inner", "z", "m"} {
		if tSym.Fields[name] == nil {
			t.Errorf("missing field %q in %v", name, fieldNames(tSym))
		}
	}
	if tSym.Fields["inner"].Fields["y"] == nil {
		t.Errorf("missing nested field y in %v", fieldNames(tSym.Fields["inner"]))
	}
	if !tSym.DynamicFields {
		t.Error("t.DynamicFields = false, want true")
	}
	if len(diagnosticsWithCode(result, CodeUndefinedGlobal)) != 1 {
		t.Errorf("diagnostics = %+v, want one undefined-global for k", result.Diagnostics)
	}
}

func fieldNames(sym *Symbol) []string {
	var out []string
	for name := range sym.Fields {
		out = append(out, name)
	}
	return out
}

func TestStringKeyedFieldIsNotAStringOccurrence(t *testing.T) {
	text := "t = {x = 1}\nprint(t[\"x\"])\nt[\"x\"] = 2\n"
	result, _ := analyzeText(t, text)
	tSym := result.Table.Globals["t"]
	if tSym == nil || !tSym.DynamicFields {
		t.Fatalf("t = %+v, want dynamic fields", tSym)
	}
	offset := strings.Index(text, `"x"`) + 1
	if occ := result.OccurrenceAt(0, offset); occ != nil {
		t.Fatalf("occurrence inside string = %+v, want nil", occ)
	}
}

func TestFieldOnUnknownReceiverIsNotInvented(t *testing.T) {
	text := "foo.x = 1\n"
	result, _ := analyzeText(t, text)
	if len(diagnosticsWithCode(result, CodeUndefinedGlobal)) != 1 {
		t.Fatalf("diagnostics = %+v, want one undefined-global", result.Diagnostics)
	}
	if len(result.Table.Symbols) != 0 {
		t.Fatalf("symbols = %+v, want none for an unknown receiver", result.Table.Symbols)
	}
}

func TestLabelsAndGoto(t *testing.T) {
	text := "goto later\n::later::\n"
	result, _ := analyzeText(t, text)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v, want none", result.Diagnostics)
	}
	label := symbolAtText(t, result, text, "later::")
	if label == nil || label.Kind != SymbolLabel || len(label.Uses) != 1 {
		t.Fatalf("label = %+v", label)
	}
}

func TestUnknownAndDuplicateLabels(t *testing.T) {
	text := "goto missing\n::a::\n::a::\n"
	result, _ := analyzeText(t, text)
	if len(diagnosticsWithCode(result, CodeUnknownLabel)) != 1 {
		t.Fatalf("unknown-label diagnostics = %+v", result.Diagnostics)
	}
	if len(diagnosticsWithCode(result, CodeDuplicateLabel)) != 1 {
		t.Fatalf("duplicate-label diagnostics = %+v", result.Diagnostics)
	}
}

func TestLabelsAreFunctionScoped(t *testing.T) {
	text := "::a::\nfunction f() goto a end\n"
	result, _ := analyzeText(t, text)
	if len(diagnosticsWithCode(result, CodeUnknownLabel)) != 1 {
		t.Fatalf("diagnostics = %+v, want unknown-label", result.Diagnostics)
	}
}

func TestUndefinedGlobalSuppressedInMeta(t *testing.T) {
	text := "---@meta\nfunction f() return missing end\n"
	result, _ := analyzeText(t, text)
	if len(diagnosticsWithCode(result, CodeUndefinedGlobal)) != 0 {
		t.Fatalf("diagnostics = %+v, want none", result.Diagnostics)
	}
}

func TestUndefinedGlobalSeverityOff(t *testing.T) {
	text := "print(missing)\n"
	u := uri.File("/test/cart.lua")
	file := &cart.File{ID: 0, URI: u, Text: text, Index: source.New([]byte(text), protocol.PositionEncodingKindUTF16)}
	c := &cart.Cart{RootURI: u, Files: []*cart.File{file}}
	lexed := syntax.Lex([]byte(text), nil)
	chunk, _ := syntax.Parse(lexed.Tokens)
	result := Analyze(&cart.Snapshot{Cart: c, Chunk: chunk, Comments: lexed.Comments}, testBuiltins(t), Options{UndefinedGlobal: SeverityOff})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v, want none", result.Diagnostics)
	}
}

func TestRenameConflictLocal(t *testing.T) {
	text := "local x = 1\ndo\n  local y = 2\n  print(x, y)\nend\n"
	result, _ := analyzeText(t, text)
	x := result.OccurrenceAt(0, strings.Index(text, "x = 1")).Symbol
	if conflict := result.RenameConflict(x, "y"); conflict == nil {
		t.Fatal("local x -> y should conflict with inner y")
	}
	y := result.OccurrenceAt(0, strings.Index(text, "y = 2")).Symbol
	if conflict := result.RenameConflict(y, "z"); conflict != nil {
		t.Fatalf("y -> z conflict = %+v, want none", conflict)
	}
}

func TestRenameConflictUnresolvedUse(t *testing.T) {
	text := "local x = 1\nprint(y)\n"
	result, _ := analyzeText(t, text)
	x := result.OccurrenceAt(0, strings.Index(text, "x = 1")).Symbol
	if conflict := result.RenameConflict(x, "y"); conflict == nil {
		t.Fatal("x -> y should conflict with the unresolved use")
	}
}

func TestRenameConflictBuiltinUse(t *testing.T) {
	text := "local x = 1\nprint(x)\n"
	result, _ := analyzeText(t, text)
	x := result.OccurrenceAt(0, strings.Index(text, "x = 1")).Symbol
	if conflict := result.RenameConflict(x, "print"); conflict == nil {
		t.Fatal("x -> print should conflict with the built-in use")
	}
}

func TestRenameConflictLocalFunction(t *testing.T) {
	text := "local function f() end\nlocal g = 1\n"
	result, _ := analyzeText(t, text)
	f := result.OccurrenceAt(0, strings.Index(text, "f()")).Symbol
	if conflict := result.RenameConflict(f, "g"); conflict == nil {
		t.Fatal("local function f -> g should conflict")
	}
}

func TestRenameConflictFieldFunctionWithDynamicAccess(t *testing.T) {
	text := "t = {}\nfunction t.x() end\nt[k] = 1\n"
	result, _ := analyzeText(t, text)
	tSym := result.Table.Globals["t"]
	if conflict := result.RenameConflict(tSym.Fields["x"], "y"); conflict == nil {
		t.Fatal("field function x -> y should conflict with dynamic access")
	}
}

func TestRenameConflictGlobalShadowedByLocal(t *testing.T) {
	text := "x = 1\nfunction f()\n  local y = 2\n  print(x, y)\nend\n"
	result, _ := analyzeText(t, text)
	x := result.Table.Globals["x"]
	if conflict := result.RenameConflict(x, "y"); conflict == nil {
		t.Fatal("global x -> y should conflict with local y")
	}
}

func TestRenameConflictGlobalAndField(t *testing.T) {
	text := "a = 1\nb = 2\nt = {x = 1, y = 2}\n"
	result, _ := analyzeText(t, text)
	a := result.Table.Globals["a"]
	if conflict := result.RenameConflict(a, "b"); conflict == nil {
		t.Fatal("a -> b should conflict")
	}
	tSym := result.Table.Globals["t"]
	if conflict := result.RenameConflict(tSym.Fields["x"], "y"); conflict == nil {
		t.Fatal("field x -> y should conflict")
	}
}
