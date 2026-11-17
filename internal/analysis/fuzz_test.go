package analysis

import (
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

func FuzzAnalyze(f *testing.F) {
	for _, seed := range []string{
		"",
		"local x = x\n",
		"function f(a, ...) return a end\n",
		"t = {x = 1}; function t:m() return self end\n",
		"goto missing\n::a::\n",
		"repeat local x = 1 until x\n",
		"#include \"nope.lua\"\n",
		"if (btn(0)) x += 1\n",
		"---@class A: B\n---@class B: A\nlocal a = {}\n",
		"---@class (exact) P\n---@field x number\n---@type P\nlocal p = {x = 1}\n",
		"---@class C: (exact) D\n---@class D\nlocal c = {}\n",
		"local p = setmetatable({}, {__index = C})\n",
		"---@param x number?\nlocal function f(x)\n  assert(x)\n  if not x then return end\n  local y = x or 1\nend\n",
	} {
		f.Add(seed)
	}

	builtins := testBuiltins(f)
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 128*1024 {
			t.Skip()
		}
		u := uri.File("/fuzz/cart.lua")
		file := &cart.File{
			ID:    0,
			URI:   u,
			Text:  text,
			Index: source.New([]byte(text), protocol.PositionEncodingKindUTF16),
		}
		lexed := syntax.Lex([]byte(text), func(start, end int) source.Span {
			return source.Span{FileID: 0, Start: start, End: end}
		})
		chunk, _ := syntax.Parse(lexed.Tokens)
		snap := &cart.Snapshot{
			Cart:     &cart.Cart{RootURI: u, Files: []*cart.File{file}},
			Chunk:    chunk,
			Comments: lexed.Comments,
			Index:    syntax.NewCommentIndex(lexed.Comments),
		}
		_ = Analyze(snap, builtins, Options{UndefinedGlobal: SeverityWarning})
	})
}
