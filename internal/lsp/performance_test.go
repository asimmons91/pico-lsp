package lsp

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
)

func largeCartText() string {
	var b strings.Builder
	b.WriteString("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n")
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&b, "value_%03d = %d + circfill(%d, 2, 3, 4)\n", i, i, i)
	}
	b.WriteString("-- ")
	for b.Len() < 60*1024 {
		b.WriteString("padding padding padding padding padding padding ")
	}
	b.WriteString("\nfunction _update()\n  local speed = 1\n  \nend\n")
	return b.String()
}

func TestPerformanceTargets(t *testing.T) {
	if raceDetectorEnabled {
		t.Skip("timing targets are not meaningful under the race detector")
	}
	text := largeCartText()
	if len(text) < 60*1024 {
		t.Fatalf("cart size = %d, want at least 60 KB", len(text))
	}
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	completionPos := positionAt(t, text, "  local speed = 1", 2)
	// Warm the cart cache and any lazily built indexes.
	if _, err := h.server.Completion(ctx, &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Position:     completionPos,
		},
	}); err != nil {
		t.Fatalf("warm completion: %v", err)
	}

	measure := func(name string, target time.Duration, fn func()) time.Duration {
		const iterations = 10
		start := time.Now()
		for i := 0; i < iterations; i++ {
			fn()
		}
		elapsed := time.Since(start) / iterations
		t.Logf("%s: %v (target %v)", name, elapsed, target)
		if elapsed > target {
			t.Errorf("%s took %v, target %v", name, elapsed, target)
		}
		return elapsed
	}

	measure("completion 64 KB", 5*time.Millisecond, func() {
		if _, err := h.server.Completion(ctx, &protocol.CompletionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: root},
				Position:     completionPos,
			},
		}); err != nil {
			t.Fatalf("completion: %v", err)
		}
	})

	hoverPos := positionAt(t, text, "circfill(", 2)
	measure("hover 64 KB", 2*time.Millisecond, func() {
		if _, err := h.server.Hover(ctx, &protocol.HoverParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: root},
				Position:     hoverPos,
			},
		}); err != nil {
			t.Fatalf("hover: %v", err)
		}
	})

	measure("semantic tokens 64 KB", 10*time.Millisecond, func() {
		if _, err := h.server.SemanticTokensFull(ctx, &protocol.SemanticTokensParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
		}); err != nil {
			t.Fatalf("semantic tokens: %v", err)
		}
	})
}
