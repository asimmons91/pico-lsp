package lsp

import (
	"testing"

	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
)

func TestWorkspaceContainerShowsParentIncludePaths(t *testing.T) {
	root := uri.File("/workspace/carts/cart.p8")
	included := &cart.File{ID: 1, URI: uri.File("/workspace/shared/lib.lua")}
	c := &cart.Cart{
		RootURI: root,
		Files: []*cart.File{
			{ID: 0, URI: root},
			included,
		},
	}
	snap := &cart.Snapshot{Cart: c}
	got := workspaceContainer(snap, root, source.Span{FileID: 1, Start: 0, End: 1})
	if got != "../shared/lib.lua" {
		t.Fatalf("workspaceContainer = %q, want ../shared/lib.lua", got)
	}
}
