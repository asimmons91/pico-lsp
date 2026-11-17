package analysis

import (
	"strings"
	"testing"

	"github.com/asimmons91/pico-lsp/internal/syntax"
)

func TestParseDocComments(t *testing.T) {
	comments := []syntax.Comment{
		{Kind: syntax.CommentDoc, Text: "--- A player object.", Line: 0},
		{Kind: syntax.CommentDoc, Text: "---@type Player", Line: 1},
		{Kind: syntax.CommentDoc, Text: "---@param dx? number horizontal delta", Line: 2},
		{Kind: syntax.CommentDoc, Text: "---@return boolean ok", Line: 3},
		{Kind: syntax.CommentDoc, Text: "---@deprecated", Line: 4},
	}
	doc := ParseDocComments(comments)
	if doc == nil {
		t.Fatal("no docs")
	}
	if doc.Summary != "A player object." {
		t.Fatalf("summary = %q", doc.Summary)
	}
	if doc.Type != "Player" {
		t.Fatalf("type = %q", doc.Type)
	}
	if len(doc.Params) != 1 || doc.Params[0].Name != "dx" || !doc.Params[0].Optional || doc.Params[0].Doc != "horizontal delta" {
		t.Fatalf("params = %+v", doc.Params)
	}
	if len(doc.Returns) != 1 || doc.Returns[0] != "boolean ok" {
		t.Fatalf("returns = %+v", doc.Returns)
	}
	if !doc.Deprecated {
		t.Fatal("deprecated = false")
	}
}

func TestDocsDoNotLeakAcrossInterveningCode(t *testing.T) {
	text := "--- First docs.\nfunction first() end\n\nlocal value = 1\n\nfunction second() end\n"
	result, _ := analyzeText(t, text)
	first := result.Table.Globals["first"]
	second := result.Table.Globals["second"]
	if first == nil || first.Doc == nil || first.Doc.Summary != "First docs." {
		t.Fatalf("first docs = %+v", first)
	}
	if second == nil {
		t.Fatal("missing second")
	}
	if second.Doc != nil {
		t.Fatalf("second inherited docs: %+v", second.Doc)
	}
}

func TestAnnotationNamesAreUnique(t *testing.T) {
	comments := []syntax.Comment{
		{Kind: syntax.CommentDoc, Text: "---@class Player", Line: 0},
		{Kind: syntax.CommentDoc, Text: "---@alias Mode string", Line: 1},
		{Kind: syntax.CommentDoc, Text: "---@class Player", Line: 2},
	}
	names := AnnotationNames(comments)
	if len(names) != 2 || names[0] != "Player" || names[1] != "Mode" {
		t.Fatalf("annotation names = %v", names)
	}
}

func TestVisibleSymbolsAtRespectsDeclarationOrder(t *testing.T) {
	text := strings.Join([]string{
		"local x = 1",
		"do",
		"  print(x)",
		"  local x = 2",
		"  print(x)",
		"end",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)

	innerDecl := strings.Index(text, "local x = 2")
	afterOffset := innerDecl + strings.Index(text[innerDecl:], "print(x)")
	before := result.ScopeAt(0, strings.Index(text, "print(x)"))
	after := result.ScopeAt(0, afterOffset)
	if before == nil || after == nil {
		t.Fatal("missing scopes")
	}
	if before == result.chunkScope {
		t.Fatal("expected an inner block scope")
	}

	findX := func(scope *Scope, offset int) *Symbol {
		for _, sym := range scope.VisibleSymbolsAt(0, offset) {
			if sym.Canonical == "x" {
				return sym
			}
		}
		return nil
	}
	outer := findX(before, strings.Index(text, "print(x)")+len("print("))
	inner := findX(after, afterOffset+len("print("))
	if outer == nil || inner == nil {
		t.Fatalf("x not visible: outer=%v inner=%v", outer, inner)
	}
	if outer.DeclSpan.Start != strings.Index(text, "x = 1") {
		t.Fatalf("before declaration resolved to inner x at %d", outer.DeclSpan.Start)
	}
	if inner.DeclSpan.Start != strings.Index(text, "x = 2") {
		t.Fatalf("after declaration resolved to %d, want inner x", inner.DeclSpan.Start)
	}
}
