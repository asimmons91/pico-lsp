package refactor

import (
	"sort"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

func testContext(t *testing.T, text string) *Context {
	t.Helper()
	return testContextMode(t, text, analysis.TypeCheckingBasic)
}

func testContextMode(t *testing.T, text string, mode analysis.TypeCheckingMode) *Context {
	t.Helper()
	u := uri.File("/test/main.lua")
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
	result := analysis.Analyze(snap, nil, analysis.Options{
		UndefinedGlobal: analysis.SeverityWarning,
		TypeChecking:    mode,
		Lint: analysis.LintOptions{
			UnusedLocal: analysis.SeverityHint,
		},
	})
	return &Context{Snap: snap, Result: result, FileID: 0, File: file}
}

func offsetOf(t *testing.T, text, needle string) int {
	t.Helper()
	index := strings.Index(text, needle)
	if index < 0 {
		t.Fatalf("missing %q in %q", needle, text)
	}
	return index
}

func applyEdits(t *testing.T, text string, edits []Edit) string {
	t.Helper()
	sorted := append([]Edit(nil), edits...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Start != sorted[j].Start {
			return sorted[i].Start > sorted[j].Start
		}
		return sorted[i].End > sorted[j].End
	})
	out := text
	for _, edit := range sorted {
		if edit.Start < 0 || edit.End > len(out) || edit.Start > edit.End {
			t.Fatalf("invalid edit %+v for text length %d", edit, len(out))
		}
		out = out[:edit.Start] + edit.NewText + out[edit.End:]
	}
	return out
}

func TestExtractFunctionExpression(t *testing.T) {
	text := "local a = 1\nlocal b = 2\nlocal c = a + b\n"
	ctx := testContext(t, text)
	start := offsetOf(t, text, "a + b")
	action := ExtractFunction(ctx, start, start+len("a + b"))
	if action == nil {
		t.Fatal("extract returned nil")
	}
	if action.Kind != KindExtractFunction {
		t.Fatalf("kind = %q", action.Kind)
	}
	out := applyEdits(t, text, action.Edits)
	for _, want := range []string{
		"local function extracted(a, b)",
		"return a + b",
		"local c = extracted(a, b)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("result missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "local function") > strings.Index(out, "local c =") {
		t.Fatalf("function was not inserted before call site:\n%s", out)
	}
}

func TestExtractFunctionStatement(t *testing.T) {
	text := "local a = 1\nprint(a)\n"
	ctx := testContext(t, text)
	start := offsetOf(t, text, "print(a)")
	action := ExtractFunction(ctx, start, start+len("print(a)"))
	if action == nil {
		t.Fatal("extract returned nil")
	}
	out := applyEdits(t, text, action.Edits)
	want := "local a = 1\nlocal function extracted(a)\n  print(a)\nend\nextracted(a)\n"
	if out != want {
		t.Fatalf("result = %q, want %q", out, want)
	}
}

func TestExtractFunctionRefusesWriteToCapture(t *testing.T) {
	text := "local x = 0\nx = x + 1\n"
	ctx := testContext(t, text)
	start := offsetOf(t, text, "x = x + 1")
	if action := ExtractFunction(ctx, start, start+len("x = x + 1")); action != nil {
		t.Fatalf("extract accepted a write to a captured local: %+v", action)
	}
}

func TestExtractFunctionRefusesMultilineString(t *testing.T) {
	text := "local s = [[line one\n  line two]]\nprint(s)\n"
	ctx := testContext(t, text)
	start := offsetOf(t, text, "[[line one")
	end := start + len("[[line one\n  line two]]")
	if action := ExtractFunction(ctx, start, end); action != nil {
		t.Fatalf("extract reindented a multi-line string: %+v", action)
	}
}

func TestExtractFunctionRefusesMidLineStatement(t *testing.T) {
	text := "first() second()\n"
	ctx := testContext(t, text)
	start := offsetOf(t, text, "second()")
	if action := ExtractFunction(ctx, start, start+len("second()")); action != nil {
		t.Fatalf("extract split a shared line: %+v", action)
	}
}

func TestExtractFunctionRefusesMultiValueCall(t *testing.T) {
	text := "local function f() return 1, 2 end\nlocal a, b = f()\n"
	ctx := testContext(t, text)
	start := offsetOf(t, text, "f()")
	if action := ExtractFunction(ctx, start, start+len("f()")); action != nil {
		t.Fatalf("extract accepted a multi-value call: %+v", action)
	}
}

func TestGlobalToLocal(t *testing.T) {
	text := "score = 0\nfunction tick() score += 1 end\n"
	ctx := testContext(t, text)
	action := GlobalToLocal(ctx, offsetOf(t, text, "score"))
	if action == nil {
		t.Fatal("global-to-local returned nil")
	}
	out := applyEdits(t, text, action.Edits)
	if !strings.HasPrefix(out, "local score = 0\n") {
		t.Fatalf("result = %q", out)
	}
	if action.Kind != KindGlobalToLocal {
		t.Fatalf("kind = %q", action.Kind)
	}
}

func TestGlobalToLocalPreservesIndentation(t *testing.T) {
	text := "  score = 0\n"
	ctx := testContext(t, text)
	action := GlobalToLocal(ctx, offsetOf(t, text, "score"))
	if action == nil {
		t.Fatal("global-to-local returned nil")
	}
	out := applyEdits(t, text, action.Edits)
	if out != "  local score = 0\n" {
		t.Fatalf("result = %q", out)
	}
}

func TestGlobalToLocalRefusesNestedDeclarationWithOutsideUse(t *testing.T) {
	text := "if flag then\n  score = 0\nend\nprint(score)\n"
	ctx := testContext(t, text)
	if action := GlobalToLocal(ctx, offsetOf(t, text, "score")); action != nil {
		t.Fatalf("global-to-local scoped a local into a block: %+v", action)
	}
}

func TestGlobalToLocalRefusesForwardReference(t *testing.T) {
	text := "function tick() score += 1 end\nscore = 0\n"
	ctx := testContext(t, text)
	if action := GlobalToLocal(ctx, offsetOf(t, text, "score")); action != nil {
		t.Fatalf("global-to-local accepted a forward function reference: %+v", action)
	}
}

func TestGlobalToLocalAllowsRecursiveFunction(t *testing.T) {
	text := "function walk() walk() end\n"
	ctx := testContext(t, text)
	action := GlobalToLocal(ctx, offsetOf(t, text, "walk"))
	if action == nil {
		t.Fatal("global-to-local refused a recursive function")
	}
	out := applyEdits(t, text, action.Edits)
	if !strings.HasPrefix(out, "local function walk()") {
		t.Fatalf("result = %q", out)
	}
}

func TestGenerateClass(t *testing.T) {
	text := "local p = {x = 1, y = 2}\n"
	ctx := testContext(t, text)
	start := offsetOf(t, text, "{x")
	action := GenerateClass(ctx, start)
	if action == nil {
		t.Fatal("generate class returned nil")
	}
	out := applyEdits(t, text, action.Edits)
	want := "---@class p\n---@field x number\n---@field y number\nlocal p = {x = 1, y = 2}\n"
	if out != want {
		t.Fatalf("result = %q, want %q", out, want)
	}
}

func TestGenerateClassConservativeFieldTypes(t *testing.T) {
	text := "local p = {pos = {x = 1}, name = \"p\", [1] = true, [\"odd-key\"] = 1}\n"
	ctx := testContext(t, text)
	action := GenerateClass(ctx, offsetOf(t, text, "{pos"))
	if action == nil {
		t.Fatal("generate class returned nil")
	}
	out := applyEdits(t, text, action.Edits)
	want := "---@class p\n---@field pos any\n---@field name string\nlocal p = {pos = {x = 1}, name = \"p\", [1] = true, [\"odd-key\"] = 1}\n"
	if out != want {
		t.Fatalf("result = %q, want %q", out, want)
	}
}

func TestGenerateClassInsertsAboveDocs(t *testing.T) {
	text := "---@type table\nlocal p = {x = 1}\n"
	ctx := testContext(t, text)
	action := GenerateClass(ctx, offsetOf(t, text, "{x"))
	if action == nil {
		t.Fatal("generate class returned nil")
	}
	out := applyEdits(t, text, action.Edits)
	want := "---@class p\n---@field x number\n---@type table\nlocal p = {x = 1}\n"
	if out != want {
		t.Fatalf("result = %q, want %q", out, want)
	}
}

func TestShorthandConversions(t *testing.T) {
	standard := "if x then\n  play()\nend\n"
	ctx := testContext(t, standard)
	action := ConvertShorthand(ctx, offsetOf(t, standard, "if"))
	if action == nil {
		t.Fatal("collapse returned nil")
	}
	out := applyEdits(t, standard, action.Edits)
	if out != "if (x) play()\n" {
		t.Fatalf("collapsed = %q", out)
	}

	shorthand := "if (x) play()\n"
	ctx = testContext(t, shorthand)
	action = ConvertShorthand(ctx, 0)
	if action == nil {
		t.Fatal("expand returned nil")
	}
	out = applyEdits(t, shorthand, action.Edits)
	if out != "if x then\n  play()\nend\n" {
		t.Fatalf("expanded = %q", out)
	}
}

func TestShorthandExpandPreservesNextLineElse(t *testing.T) {
	text := "if a then\n  if (b) play()\nelse\n  stop()\nend\n"
	ctx := testContext(t, text)
	start := offsetOf(t, text, "if (b)")
	action := ConvertShorthand(ctx, start)
	if action == nil {
		t.Fatal("expand returned nil")
	}
	out := applyEdits(t, text, action.Edits)
	want := "if a then\n  if b then\n    play()\n  end\nelse\n  stop()\nend\n"
	if out != want {
		t.Fatalf("result = %q, want %q", out, want)
	}
}

func TestShorthandCollapseLeavesOuterNextLineElse(t *testing.T) {
	text := "if a then\n  if b then\n    play()\n  end\nelse\n  stop()\nend\n"
	ctx := testContext(t, text)
	start := offsetOf(t, text, "if b")
	action := ConvertShorthand(ctx, start)
	if action == nil {
		t.Fatal("collapse returned nil")
	}
	out := applyEdits(t, text, action.Edits)
	want := "if a then\n  if (b) play()\nelse\n  stop()\nend\n"
	if out != want {
		t.Fatalf("result = %q, want %q", out, want)
	}
}

func TestShorthandExpandPreservesTabIndentation(t *testing.T) {
	text := "\tif (x) play()\n"
	ctx := testContext(t, text)
	action := ConvertShorthand(ctx, 1)
	if action == nil {
		t.Fatal("expand returned nil")
	}
	out := applyEdits(t, text, action.Edits)
	want := "\tif x then\n\t\tplay()\n\tend\n"
	if out != want {
		t.Fatalf("result = %q, want %q", out, want)
	}
}

func TestShorthandCollapseRefusesLongLine(t *testing.T) {
	text := "if x then\n  " + strings.Repeat("a", maxShorthandLine) + "()\nend\n"
	ctx := testContext(t, text)
	if action := ConvertShorthand(ctx, 0); action != nil {
		t.Fatalf("collapse accepted an over-long line: %+v", action)
	}
}

func TestShorthandConversionRefusesComments(t *testing.T) {
	text := "if x then\n  -- keep me\n  play()\nend\n"
	ctx := testContext(t, text)
	if action := ConvertShorthand(ctx, offsetOf(t, text, "if")); action != nil {
		t.Fatalf("collapse accepted a comment: %+v", action)
	}
}

func TestRemoveUnused(t *testing.T) {
	text := "local used = 1\nlocal unused = 2\nprint(used)\n"
	ctx := testContext(t, text)
	start := offsetOf(t, text, "unused")
	action := RemoveUnused(ctx, start)
	if action == nil {
		t.Fatal("remove unused returned nil")
	}
	if action.Confirmation == nil {
		t.Fatal("remove unused did not request confirmation")
	}
	out := applyEdits(t, text, action.Edits)
	if strings.Contains(out, "unused") {
		t.Fatalf("result still contains unused declaration: %q", out)
	}
	if !strings.Contains(out, "local used = 1") || !strings.Contains(out, "print(used)") {
		t.Fatalf("result changed unrelated code: %q", out)
	}
}

func TestRemoveUnusedRefusesImpureInitializer(t *testing.T) {
	text := "local unused = make_thing()\n"
	ctx := testContext(t, text)
	if action := RemoveUnused(ctx, offsetOf(t, text, "unused")); action != nil {
		t.Fatalf("remove unused removed a call initializer: %+v", action)
	}
}

func TestAddMissingFieldsKeepsStringKeyCaseSensitivity(t *testing.T) {
	text := "---@class (exact) Point\n---@field x number\n---@type Point\nlocal p = {[\"X\"] = 0}\n"
	ctx := testContext(t, text)
	if len(ctx.Result.MissingFieldFixes) == 0 {
		t.Fatalf("no missing-field fix recorded: %+v", ctx.Result.Diagnostics)
	}
	fix := ctx.Result.MissingFieldFixes[0]
	action := AddMissingFields(ctx, fix.Span, fix.Fields)
	if action == nil {
		t.Fatal("add missing fields returned nil")
	}
	out := applyEdits(t, text, action.Edits)
	if !strings.Contains(out, "[\"X\"] = 0, x = 0") {
		t.Fatalf("result = %q", out)
	}
}

func TestAddMissingFieldsDoesNotTouchOpenClasses(t *testing.T) {
	text := "---@class Point\n---@field x number\n---@type Point\nlocal p = {}\n"
	ctx := testContextMode(t, text, analysis.TypeCheckingStrict)
	if len(ctx.Result.MissingFieldFixes) != 0 {
		t.Fatalf("open class recorded a fix: %+v", ctx.Result.MissingFieldFixes)
	}
}

func TestAddMissingFieldsSingleLineTrailingComma(t *testing.T) {
	text := "---@class (exact) Point\n---@field x number\n---@field y number\n---@type Point\nlocal p = {x = 0,}\n"
	ctx := testContext(t, text)
	if len(ctx.Result.MissingFieldFixes) == 0 {
		t.Fatalf("no missing-field fix recorded: %+v", ctx.Result.Diagnostics)
	}
	fix := ctx.Result.MissingFieldFixes[0]
	action := AddMissingFields(ctx, fix.Span, fix.Fields)
	if action == nil {
		t.Fatal("add missing fields returned nil")
	}
	out := applyEdits(t, text, action.Edits)
	if !strings.Contains(out, "local p = {x = 0,y = 0}") {
		t.Fatalf("result = %q", out)
	}
}

func TestAddMissingFields(t *testing.T) {
	text := "---@class (exact) Point\n---@field x number\n---@field y number\n---@type Point\nlocal p = {}\n"
	ctx := testContext(t, text)
	if len(ctx.Result.MissingFieldFixes) == 0 {
		t.Fatalf("no missing-field fix recorded: %+v", ctx.Result.Diagnostics)
	}
	fix := ctx.Result.MissingFieldFixes[0]
	action := AddMissingFields(ctx, fix.Span, fix.Fields)
	if action == nil {
		t.Fatal("add missing fields returned nil")
	}
	out := applyEdits(t, text, action.Edits)
	for _, want := range []string{"x = 0", "y = 0"} {
		if !strings.Contains(out, want) {
			t.Fatalf("result missing %q:\n%s", want, out)
		}
	}
	if action.Kind != KindAddMissingFields {
		t.Fatalf("kind = %q", action.Kind)
	}
}
