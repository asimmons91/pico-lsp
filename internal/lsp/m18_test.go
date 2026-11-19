package lsp

import (
	"context"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func cartInfoTabs(t *testing.T, h *harness, ctx context.Context, root uri.URI) []TabInfo {
	t.Helper()
	params, _ := protocol.Marshal(map[string]any{"textDocument": map[string]string{"uri": root.String()}})
	result, err := h.srv.Request(ctx, RequestCartInfo, protocol.LSPAny(params))
	if err != nil {
		t.Fatalf("cartInfo: %v", err)
	}
	return result.(CartInfo).Tabs
}

func TestM18LoneCRTabLinesUseLSPPositions(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\ns = \"a\rb\"\n-->8 second\nfunction f() end\n__gfx__\n0000\n"
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	tabs := cartInfoTabs(t, h, ctx, root)
	if len(tabs) != 2 {
		t.Fatalf("tabs = %+v", tabs)
	}
	if tabs[0].StartLine != 3 || tabs[0].EndLine != 4 {
		t.Fatalf("tab 0 = %+v, want LSP lines 3..4", tabs[0])
	}
	if tabs[1].StartLine != 5 || tabs[1].EndLine != 6 {
		t.Fatalf("tab 1 = %+v, want LSP lines 5..6", tabs[1])
	}

	folds, err := h.server.FoldingRanges(ctx, &protocol.FoldingRangeParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
	})
	if err != nil {
		t.Fatalf("foldingRanges: %v", err)
	}
	if !hasFold(folds, 3, 4) || !hasFold(folds, 5, 6) {
		t.Fatalf("folds = %+v, want tab folds 3..4 and 5..6", folds)
	}

	result, err := h.server.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
	})
	if err != nil {
		t.Fatalf("documentSymbol: %v", err)
	}
	symbols := result.(protocol.DocumentSymbolSlice)
	var second *protocol.DocumentSymbol
	for i := range symbols {
		if symbols[i].Name == "second" {
			second = &symbols[i]
		}
	}
	if second == nil {
		t.Fatalf("symbols = %v, missing second tab", symbolNames(symbols))
	}
	if !containsString(symbolNames(second.Children), "f") {
		t.Fatalf("second tab children = %v, want f", symbolNames(second.Children))
	}
}

func TestM18CartInfoPreservesHistoricalTabLines(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []TabInfo
	}{
		{
			name: "empty body",
			text: "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n__gfx__\n0000\n",
			want: []TabInfo{{Index: 0, Name: "Tab 1", StartLine: 2, EndLine: 2}},
		},
		{
			name: "last tab at EOF",
			text: "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nx = 1\n-->8\ny = 2\n",
			want: []TabInfo{
				{Index: 0, Name: "Tab 1", StartLine: 3, EndLine: 3},
				{Index: 1, Name: "Tab 2", StartLine: 4, EndLine: 6},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := writeTemp(t, t.TempDir(), "cart.p8", tc.text)
			h := newHarness(t, newTestClient())
			h.initialize(t, &protocol.InitializeParams{})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			openDoc(t, h, ctx, root, tc.text)

			got := cartInfoTabs(t, h, ctx, root)
			if len(got) != len(tc.want) {
				t.Fatalf("tabs = %+v, want %+v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("tab %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func hasFold(folds []protocol.FoldingRange, start, end uint32) bool {
	for _, fold := range folds {
		if fold.StartLine == start && fold.EndLine == end {
			return true
		}
	}
	return false
}
