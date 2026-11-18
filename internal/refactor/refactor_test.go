package refactor

import (
	"os"
	"path/filepath"
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

func TestExtractFunctionKeepsDocsOnOriginalStatement(t *testing.T) {
	text := "---@type number\nlocal value = a + b\n"
	ctx := testContext(t, text)
	start := offsetOf(t, text, "a + b")
	action := ExtractFunction(ctx, start, start+len("a + b"))
	if action == nil {
		t.Fatal("extract returned nil")
	}
	out := applyEdits(t, text, action.Edits)
	want := "local function extracted()\n  return a + b\nend\n---@type number\nlocal value = extracted()\n"
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

func TestExtractFunctionRefusesNonExpressionPositions(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		needle string
	}{
		{name: "assignment target", text: "x = 1\n", needle: "x"},
		{name: "table key", text: "local t = {x = 1}\n", needle: "x = 1"},
		{name: "field name", text: "local t = {}\nt.x = 1\n", needle: "x = 1"},
		{name: "parameter", text: "local function f(a) return a end\n", needle: "a"},
		{name: "local function name", text: "local function f() end\n", needle: "f"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := testContext(t, test.text)
			start := offsetOf(t, test.text, test.needle)
			end := start + len(test.needle)
			if test.name == "table key" {
				// Select just the key, not the value.
				end = start + 1
			}
			if test.name == "field name" {
				start = offsetOf(t, test.text, "t.x") + 2
				end = start + 1
			}
			if test.name == "parameter" {
				start = offsetOf(t, test.text, "(a)") + 1
				end = start + 1
			}
			if action := ExtractFunction(ctx, start, end); action != nil {
				t.Fatalf("extract accepted a non-expression position: %+v", action)
			}
		})
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

func TestTopLevelIncludedFileActions(t *testing.T) {
	dir := t.TempDir()
	rootText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nlocal root = 1\n#include \"lib.lua\"\n"
	libText := "local a = 1\nscore = 0\nprint(a + 2)\n"
	rootPath := filepath.Join(dir, "cart.p8")
	libPath := filepath.Join(dir, "lib.lua")
	if err := os.WriteFile(rootPath, []byte(rootText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(libPath, []byte(libText), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err := cart.NewLoader(nil, "", nil).Load(uri.File(rootPath))
	if err != nil {
		t.Fatal(err)
	}
	result := analysis.Analyze(snap, nil, analysis.Options{
		UndefinedGlobal: analysis.SeverityWarning,
		TypeChecking:    analysis.TypeCheckingBasic,
	})
	fileID := snap.Cart.FileID(uri.File(libPath))
	file := snap.Cart.File(fileID)
	if file == nil {
		t.Fatal("included file missing from cart")
	}
	ctx := &Context{Snap: snap, Result: result, FileID: fileID, File: file}

	start := offsetOf(t, libText, "a + 2")
	if ctx.parentBlock(ctx.enclosingStatement(start)) != snap.Chunk.Block {
		t.Fatal("top-level included statement was not attached to the chunk block")
	}
	extract := ExtractFunction(ctx, start, start+len("a + 2"))
	if extract == nil {
		t.Fatal("extract function was not offered in a top-level included file")
	}
	for _, edit := range extract.Edits {
		if edit.FileID != fileID {
			t.Fatalf("extract edit targeted file %d, want %d", edit.FileID, fileID)
		}
	}

	local := GlobalToLocal(ctx, offsetOf(t, libText, "score"))
	if local == nil {
		t.Fatal("global-to-local was not offered in a top-level included file")
	}
	if len(local.Edits) != 1 || local.Edits[0].FileID != fileID {
		t.Fatalf("local edit = %+v", local.Edits)
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

func TestRemoveUnusedRefusesMetaFile(t *testing.T) {
	text := "---@meta\nlocal unused = 1\n"
	ctx := testContext(t, text)
	if !ctx.Result.IsMetaFile(ctx.FileID) {
		t.Fatal("test context was not recognized as a meta file")
	}
	if action := RemoveUnused(ctx, offsetOf(t, text, "unused")); action != nil {
		t.Fatalf("remove unused was offered in a meta file: %+v", action)
	}
}

func TestRemoveUnusedProtectsSpacedTypeTags(t *testing.T) {
	text := "--- @class Used\n---@field x number\n---@type Used\nlocal unused = 1\n"
	ctx := testContext(t, text)
	if len(ctx.Result.Classes) == 0 {
		t.Fatal("spaced @class annotation was not parsed")
	}
	if action := RemoveUnused(ctx, offsetOf(t, text, "unused")); action != nil {
		t.Fatalf("remove unused deleted a protected type declaration: %+v", action)
	}
}

func TestRemoveUnusedRemovesAttachedDocs(t *testing.T) {
	text := "---@type number\nlocal unused = 1\nlocal used = 2\nprint(used)\n"
	ctx := testContext(t, text)
	action := RemoveUnused(ctx, offsetOf(t, text, "unused"))
	if action == nil {
		t.Fatal("remove unused returned nil")
	}
	out := applyEdits(t, text, action.Edits)
	if strings.Contains(out, "---@type number") || strings.Contains(out, "unused") {
		t.Fatalf("result kept the removed declaration's docs: %q", out)
	}
	if !strings.Contains(out, "local used = 2") {
		t.Fatalf("result changed unrelated code: %q", out)
	}
}

func TestRemoveUnusedRefusesWriteOnlyLocal(t *testing.T) {
	text := "local unused\nunused = 1\n"
	ctx := testContext(t, text)
	if action := RemoveUnused(ctx, offsetOf(t, text, "unused")); action != nil {
		t.Fatalf("remove unused changed a local write into a global write: %+v", action)
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

func TestAddMissingFieldsRefusesUnrepresentableField(t *testing.T) {
	text := "---@class (exact) Bad\n---@field bad-name number\n---@field ok number\n---@type Bad\nlocal v = {}\n"
	ctx := testContext(t, text)
	if len(ctx.Result.MissingFieldFixes) == 0 {
		t.Fatalf("no missing-field fix recorded: %+v", ctx.Result.Diagnostics)
	}
	fix := ctx.Result.MissingFieldFixes[0]
	if action := AddMissingFields(ctx, fix.Span, fix.Fields); action != nil {
		t.Fatalf("partial add-missing-fields action was offered: %+v", action)
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
