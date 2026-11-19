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
			name: "field index optional",
			text: "---@field [string] spd? number",
			want: []AnnotationSegment{
				{Span: annotationSpan(3, 9), Role: AnnotationTag},
				{Span: annotationSpan(11, 17), Role: AnnotationType},
				{Span: annotationSpan(19, 23), Role: AnnotationFieldName},
				{Span: annotationSpan(24, 30), Role: AnnotationType},
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
