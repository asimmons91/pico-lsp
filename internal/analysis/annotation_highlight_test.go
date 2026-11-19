package analysis

import (
	"testing"

	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

func TestAnnotationSegments(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []AnnotationSegment
	}{
		{
			name: "class",
			text: "---@class Ship",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 9), Role: AnnotationTag},
				{Span: annotationSpan(10, 14), Role: AnnotationClassName},
			},
		},
		{
			name: "class parents",
			text: "---@class Enemy: Entity, Base",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 9), Role: AnnotationTag},
				{Span: annotationSpan(10, 15), Role: AnnotationClassName},
				{Span: annotationSpan(17, 23), Role: AnnotationType},
				{Span: annotationSpan(25, 29), Role: AnnotationType},
			},
		},
		{
			name: "class generic",
			text: "---@class Box<T>: Base<T>",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 9), Role: AnnotationTag},
				{Span: annotationSpan(10, 13), Role: AnnotationClassName},
				{Span: annotationSpan(14, 15), Role: AnnotationGenericName},
				{Span: annotationSpan(18, 22), Role: AnnotationType},
				{Span: annotationSpan(23, 24), Role: AnnotationType},
			},
		},
		{
			name: "class trailing prose",
			text: "---@class Ship main entity",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 9), Role: AnnotationTag},
				{Span: annotationSpan(10, 14), Role: AnnotationClassName},
			},
		},
		{
			name: "class constrained generic",
			text: "---@class Inventory<T: table<string, Item>>: Container",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 9), Role: AnnotationTag},
				{Span: annotationSpan(10, 19), Role: AnnotationClassName},
				{Span: annotationSpan(20, 21), Role: AnnotationGenericName},
				{Span: annotationSpan(23, 28), Role: AnnotationType},
				{Span: annotationSpan(29, 35), Role: AnnotationType},
				{Span: annotationSpan(37, 41), Role: AnnotationType},
				{Span: annotationSpan(45, 54), Role: AnnotationType},
			},
		},
		{
			name: "class unicode",
			text: "---@class Café: Base",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 9), Role: AnnotationTag},
				{Span: annotationSpan(10, 15), Role: AnnotationClassName},
				{Span: annotationSpan(17, 21), Role: AnnotationType},
			},
		},
		{
			name: "class unicode colon",
			text: "---@class Éditeur: Base",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 9), Role: AnnotationTag},
				{Span: annotationSpan(10, 18), Role: AnnotationClassName},
				{Span: annotationSpan(20, 24), Role: AnnotationType},
			},
		},
		{
			name: "class exact",
			text: "---@class Ship (exact)",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 9), Role: AnnotationTag},
				{Span: annotationSpan(10, 14), Role: AnnotationClassName},
			},
		},
		{
			name: "field",
			text: "---@field spd number",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 9), Role: AnnotationTag},
				{Span: annotationSpan(10, 13), Role: AnnotationFieldName},
				{Span: annotationSpan(14, 20), Role: AnnotationType},
			},
		},
		{
			name: "field scope",
			text: "---@field [string] spd? number",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 9), Role: AnnotationTag},
				{Span: annotationSpan(11, 17), Role: AnnotationScope},
				{Span: annotationSpan(19, 23), Role: AnnotationFieldName},
				{Span: annotationSpan(24, 30), Role: AnnotationType},
			},
		},
		{
			name: "field public scope",
			text: "---@field [public] hp number",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 9), Role: AnnotationTag},
				{Span: annotationSpan(11, 17), Role: AnnotationScope},
				{Span: annotationSpan(19, 21), Role: AnnotationFieldName},
				{Span: annotationSpan(22, 28), Role: AnnotationType},
			},
		},
		{
			name: "param trailing prose",
			text: "---@param dx? number horizontal delta",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 9), Role: AnnotationTag},
				{Span: annotationSpan(10, 13), Role: AnnotationParamName},
				{Span: annotationSpan(14, 20), Role: AnnotationType},
			},
		},
		{
			name: "return trailing prose",
			text: "---@return boolean ok",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 10), Role: AnnotationTag},
				{Span: annotationSpan(11, 18), Role: AnnotationType},
			},
		},
		{
			name: "return list",
			text: "---@return number, string",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 10), Role: AnnotationTag},
				{Span: annotationSpan(11, 17), Role: AnnotationType},
				{Span: annotationSpan(19, 25), Role: AnnotationType},
			},
		},
		{
			name: "return description comma",
			text: "---@return number the count, or zero",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 10), Role: AnnotationTag},
				{Span: annotationSpan(11, 17), Role: AnnotationType},
			},
		},
		{
			name: "record type",
			text: "---@type {x: number, name: string}",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 8), Role: AnnotationTag},
				{Span: annotationSpan(10, 11), Role: AnnotationType},
				{Span: annotationSpan(13, 19), Role: AnnotationType},
				{Span: annotationSpan(21, 25), Role: AnnotationType},
				{Span: annotationSpan(27, 33), Role: AnnotationType},
			},
		},
		{
			name: "alias literals",
			text: `---@alias Mode "left" | "right"`,
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 9), Role: AnnotationTag},
				{Span: annotationSpan(10, 14), Role: AnnotationTypeName},
				{Span: annotationSpan(15, 21), Role: AnnotationString},
				{Span: annotationSpan(24, 31), Role: AnnotationString},
			},
		},
		{
			name: "alias unicode",
			text: `---@alias Café "x"`,
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 9), Role: AnnotationTag},
				{Span: annotationSpan(10, 15), Role: AnnotationTypeName},
				{Span: annotationSpan(16, 19), Role: AnnotationString},
			},
		},
		{
			name: "enum",
			text: "---@enum Direction",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 8), Role: AnnotationTag},
				{Span: annotationSpan(9, 18), Role: AnnotationEnumName},
			},
		},
		{
			name: "generic",
			text: "---@generic T, K: number",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 11), Role: AnnotationTag},
				{Span: annotationSpan(12, 13), Role: AnnotationGenericName},
				{Span: annotationSpan(15, 16), Role: AnnotationGenericName},
				{Span: annotationSpan(18, 24), Role: AnnotationType},
			},
		},
		{
			name: "generic default",
			text: "---@generic T = number",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 11), Role: AnnotationTag},
				{Span: annotationSpan(12, 13), Role: AnnotationGenericName},
				{Span: annotationSpan(16, 22), Role: AnnotationType},
			},
		},
		{
			name: "cast",
			text: "---@cast v string",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 8), Role: AnnotationTag},
				{Span: annotationSpan(9, 10), Role: AnnotationVariableName},
				{Span: annotationSpan(11, 17), Role: AnnotationType},
			},
		},
		{
			name: "cast field target",
			text: "---@cast t.x string",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 8), Role: AnnotationTag},
				{Span: annotationSpan(9, 12), Role: AnnotationVariableName},
				{Span: annotationSpan(13, 19), Role: AnnotationType},
			},
		},
		{
			name: "cast polarities",
			text: "---@cast v +string, -nil",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 8), Role: AnnotationTag},
				{Span: annotationSpan(9, 10), Role: AnnotationVariableName},
				{Span: annotationSpan(12, 18), Role: AnnotationType},
				{Span: annotationSpan(21, 24), Role: AnnotationType},
			},
		},
		{
			name: "operator call params",
			text: "---@operator call(x: number, y: string): boolean",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 12), Role: AnnotationTag},
				{Span: annotationSpan(13, 17), Role: AnnotationOperatorName},
				{Span: annotationSpan(18, 19), Role: AnnotationParamName},
				{Span: annotationSpan(21, 27), Role: AnnotationType},
				{Span: annotationSpan(29, 30), Role: AnnotationParamName},
				{Span: annotationSpan(32, 38), Role: AnnotationType},
				{Span: annotationSpan(41, 48), Role: AnnotationType},
			},
		},
		{
			name: "operator",
			text: "---@operator add(Vec): Vec",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 12), Role: AnnotationTag},
				{Span: annotationSpan(13, 16), Role: AnnotationOperatorName},
				{Span: annotationSpan(17, 20), Role: AnnotationType},
				{Span: annotationSpan(23, 26), Role: AnnotationType},
			},
		},
		{
			name: "operator function operand",
			text: "---@operator add(fun(): number): string",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 12), Role: AnnotationTag},
				{Span: annotationSpan(13, 16), Role: AnnotationOperatorName},
				{Span: annotationSpan(17, 20), Role: AnnotationType},
				{Span: annotationSpan(24, 30), Role: AnnotationType},
				{Span: annotationSpan(33, 39), Role: AnnotationType},
			},
		},
		{
			name: "deprecated",
			text: "---@deprecated",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 14), Role: AnnotationTag},
			},
		},
		{
			name: "unknown tag",
			text: "---@unknown thing",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 11), Role: AnnotationTag},
			},
		},
		{name: "summary", text: "--- Moves the ship."},
		{name: "plain comment", text: "--@class Ship", want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			comment := syntax.Comment{
				Kind: syntax.CommentDoc,
				Text: tc.text,
				Span: source.Span{FileID: 2, Start: 100, End: 100 + len(tc.text)},
				Line: 4,
			}
			got := AnnotationSegments(comment)
			if len(got) != len(tc.want) {
				t.Fatalf("segments = %+v, want %+v", got, tc.want)
			}
			for i := range got {
				want := tc.want[i]
				want.Span.FileID = comment.Span.FileID
				want.Span.Start += comment.Span.Start
				want.Span.End += comment.Span.Start
				if got[i] != want {
					t.Fatalf("segment %d = %+v, want %+v", i, got[i], want)
				}
			}
		})
	}
}

func annotationSpan(start, end int) source.Span {
	return source.Span{Start: start, End: end}
}
