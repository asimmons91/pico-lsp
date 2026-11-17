package cart

import (
	"testing"

	"github.com/asimmons91/pico-lsp/internal/source"
)

func TestFlatPosition(t *testing.T) {
	cart := &Cart{Segments: []Segment{
		{FlatStart: 0, FlatEnd: 5, FileID: 0, FileOffset: 10},
		{FlatStart: 5, FlatEnd: 12, FileID: 1, FileOffset: 0},
	}}
	cases := []struct {
		name   string
		fileID source.FileID
		offset int
		want   int
		ok     bool
	}{
		{name: "first file start", fileID: 0, offset: 10, want: 0, ok: true},
		{name: "first file middle", fileID: 0, offset: 12, want: 2, ok: true},
		{name: "first file end", fileID: 0, offset: 15, want: 5, ok: true},
		{name: "first file before", fileID: 0, offset: 9, ok: false},
		{name: "second file start", fileID: 1, offset: 0, want: 5, ok: true},
		{name: "second file end", fileID: 1, offset: 7, want: 12, ok: true},
		{name: "second file after", fileID: 1, offset: 8, ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := cart.FlatPosition(tc.fileID, tc.offset)
			if ok != tc.ok || (ok && got != tc.want) {
				t.Fatalf("FlatPosition(%d, %d) = (%d, %v), want (%d, %v)", tc.fileID, tc.offset, got, ok, tc.want, tc.ok)
			}
		})
	}
	if _, ok := (*Cart)(nil).FlatPosition(0, 0); ok {
		t.Fatal("nil cart returned a position")
	}
}
