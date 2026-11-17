package analysis

import (
	"testing"

	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/source"
)

func FuzzAnnotationSyntax(f *testing.F) {
	for _, seed := range []string{
		"number",
		"Box<number>",
		"Box<Box<number>>",
		"fun(x: number): string",
		"table<string, Box<number>>",
		"---@class Box<T>",
		"---@class (exact) Box<T = number>: Base<T>",
		"---@generic T, U = string",
		"---@overload fun(x: number): number",
		"---@operator add(Vec): Vec",
		"---@operator call(x: number): string",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 4096 {
			t.Skip()
		}
		resolver := func(name string) (types.Type, bool) {
			if name == "Box" {
				param := &types.GenericParam{ID: 1, Name: "T"}
				record := types.NewRecord("Box")
				record.TypeParams = []*types.GenericParam{param}
				record.AddField(&types.Field{Name: "value", Type: param})
				return record, true
			}
			return types.AnyType, true
		}
		_, _ = parseType(text, resolver)
		_, _, _, _, _ = parseClassHeader(text)
		_ = parseGenericDeclList(text)
		_, _ = parseOperatorAnnotation(text, source.Span{})
	})
}
