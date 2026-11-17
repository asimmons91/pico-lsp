package analysis

import (
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

func analyzeTextLint(t *testing.T, text string, lint LintOptions) *Result {
	t.Helper()
	u := uri.File("/test/lint.lua")
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
	return Analyze(snap, testBuiltins(t), Options{UndefinedGlobal: SeverityWarning, Lint: lint})
}

func lintAll(severity Severity) LintOptions {
	return LintOptions{
		CallbackPrecedence:  severity,
		CallbackParams:      severity,
		NoStdlib:            severity,
		DuplicateTableField: severity,
		UnreachableCode:     severity,
		UnusedLocal:         severity,
		ShadowedLocal:       severity,
		EmptyBlock:          severity,
		GlobalInFunction:    severity,
	}
}

func lintCodes(result *Result) []string {
	var codes []string
	for _, diag := range result.Diagnostics {
		codes = append(codes, diag.Code)
	}
	return codes
}

func TestM5LifecycleLint(t *testing.T) {
	text := `function _update(a)
end
function _update60()
end
`
	result := analyzeTextLint(t, text, lintAll(SeverityHint))
	if !hasDiagnostic(result, CodeCallbackPrecedence) || !hasDiagnostic(result, CodeCallbackParams) {
		t.Fatalf("callback diagnostics = %v", lintCodes(result))
	}
	for _, diag := range result.Diagnostics {
		if diag.Code == CodeCallbackPrecedence && diag.Severity != SeverityHint {
			t.Fatalf("precedence severity = %v", diag.Severity)
		}
	}
}

func TestM5NoStdlibRespectsUserDefinitions(t *testing.T) {
	text := `require("x")
math.sin(1)
local function mine() end
mine()
`
	result := analyzeTextLint(t, text, LintOptions{NoStdlib: SeverityHint})
	if countCode(result, CodeNoStdlib) != 2 {
		t.Fatalf("stdlib diagnostics = %v", lintCodes(result))
	}
	text = `function require(name) end
require("x")
`
	result = analyzeTextLint(t, text, LintOptions{NoStdlib: SeverityHint})
	if hasDiagnostic(result, CodeNoStdlib) {
		t.Fatalf("user require definition produced stdlib hint: %v", lintCodes(result))
	}
}

func TestM5StyleLints(t *testing.T) {
	text := `local unused = 1
local shadow = 1
do
  local shadow = 2
end
local duplicate = {a = 1, a = 2}
function f()
  local value = 1
  return value
  local after = 2
end
function empty()
end
`
	result := analyzeTextLint(t, text, lintAll(SeverityHint))
	for _, code := range []string{
		CodeUnusedLocal,
		CodeShadowedLocal,
		CodeDuplicateTableField,
		CodeUnreachableCode,
		CodeEmptyBlock,
	} {
		if !hasDiagnostic(result, code) {
			t.Errorf("missing %s in %v", code, lintCodes(result))
		}
	}
}

func TestM5GlobalInFunctionLint(t *testing.T) {
	text := `function f()
  global_value = 1
end
`
	result := analyzeTextLint(t, text, LintOptions{GlobalInFunction: SeverityHint})
	if !hasDiagnostic(result, CodeGlobalInFunction) {
		t.Fatalf("global diagnostics = %v", lintCodes(result))
	}
}

func TestM5LintOffByDefault(t *testing.T) {
	text := `require("x")
local t = {a = 1, a = 2}
`
	result := analyzeTextLint(t, text, LintOptions{})
	for _, code := range lintCodes(result) {
		if strings.HasPrefix(code, "pico8-") || code == CodeDuplicateTableField || code == CodeNoStdlib {
			t.Fatalf("lint fired with zero options: %v", lintCodes(result))
		}
	}
}

func countCode(result *Result, code string) int {
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == code {
			count++
		}
	}
	return count
}

func TestM5UnusedLocalIgnoresGlobalsAndFields(t *testing.T) {
	text := `function global_fn()
end
local record = {}
function record.method()
end
`
	result := analyzeTextLint(t, text, LintOptions{UnusedLocal: SeverityHint})
	if hasDiagnostic(result, CodeUnusedLocal) {
		t.Fatalf("unused-local fired for globals or fields: %v", lintCodes(result))
	}
}

func TestM5DuplicateFieldKeyCaseRules(t *testing.T) {
	result := analyzeTextLint(t, `local t = {["A"] = 1, ["a"] = 2}`, LintOptions{DuplicateTableField: SeverityHint})
	if hasDiagnostic(result, CodeDuplicateTableField) {
		t.Fatalf("string keys are case-sensitive: %v", lintCodes(result))
	}
	result = analyzeTextLint(t, `local t = {alpha = 1, ALPHA = 2}`, LintOptions{DuplicateTableField: SeverityHint})
	if !hasDiagnostic(result, CodeDuplicateTableField) {
		t.Fatalf("identifier keys should fold: %v", lintCodes(result))
	}
}
