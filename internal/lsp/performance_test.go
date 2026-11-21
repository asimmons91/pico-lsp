package lsp

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/testutil"
)

func largeCartText() string {
	var b strings.Builder
	b.WriteString("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n")
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&b, "value_%03d = %d + circfill(%d, 2, 3, 4)\n", i, i, i)
	}
	b.WriteString("-- ")
	for b.Len() < 64*1024 {
		b.WriteString("padding padding padding padding padding padding ")
	}
	b.WriteString("\nfunction _update()\n  local speed = 1\n  score = score + 1\n  \nend\n")
	return b.String()
}

func TestPerformanceTargets(t *testing.T) {
	testutil.SkipUnlessPerf(t)
	if raceDetectorEnabled || testing.CoverMode() != "" {
		t.Skip("timing targets are not meaningful under the race detector or coverage instrumentation")
	}
	if runtime.GOOS != "linux" {
		t.Skip("timing targets are measured on Linux runners")
	}
	text := largeCartText()
	if len(text) < 64*1024 {
		t.Fatalf("cart size = %d, want at least 64 KB", len(text))
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
		const iterations = 25
		samples := make([]time.Duration, 0, iterations)
		for i := 0; i < iterations; i++ {
			start := time.Now()
			fn()
			samples = append(samples, time.Since(start))
		}
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		best := samples[0]
		median := samples[len(samples)/2]
		t.Logf("%s: best %v, median %v across %d runs (target %v)", name, best, median, iterations, target)
		if best > target {
			t.Errorf("%s best %v exceeds target %v", name, best, target)
		}
		return best
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

	actionPos := positionAt(t, text, "circfill(", 0)
	measure("code actions 64 KB", 10*time.Millisecond, func() {
		if _, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Range:        protocol.Range{Start: actionPos, End: actionPos},
		}); err != nil {
			t.Fatalf("code actions: %v", err)
		}
	})

	tokenPos := positionAt(t, text, "score = score + 1", 0)
	measure("token-saving code actions 64 KB", 10*time.Millisecond, func() {
		if _, err := h.server.CodeAction(ctx, &protocol.CodeActionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: root},
			Range:        protocol.Range{Start: tokenPos, End: tokenPos},
		}); err != nil {
			t.Fatalf("token-saving code actions: %v", err)
		}
	})
}
