package analysis

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

func typeInferenceCartText() string {
	var b strings.Builder
	for i := 0; i < 700; i++ {
		fmt.Fprintf(&b, "local value_%03d = %d + #tostr(%d)\n", i, i, i)
	}
	b.WriteString("local t = {")
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&b, "field_%d = %d,", i, i)
	}
	b.WriteString("}\n")
	b.WriteString("local function combine(a, b)\n  return a + b\nend\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&b, "local sum_%03d = combine(%d, %d)\n", i, i, i+1)
	}
	b.WriteString("for i, v in ipairs(t) do\n  local x = i + v\nend\n")
	b.WriteString("-- ")
	for b.Len() < 64*1024 {
		b.WriteString("padding padding padding padding padding padding ")
	}
	return b.String()
}

func typeInferenceSnapshot(t *testing.T, text string) *cart.Snapshot {
	t.Helper()
	u := uri.File("/test/type-perf.p8")
	file := &cart.File{ID: 0, URI: u, Text: text, Index: source.New([]byte(text), protocol.PositionEncodingKindUTF16)}
	c := &cart.Cart{RootURI: u, Files: []*cart.File{file}}
	lexed := syntax.Lex([]byte(text), nil)
	chunk, errs := syntax.Parse(lexed.Tokens)
	if len(lexed.Errors) > 0 || len(errs) > 0 {
		t.Fatalf("parse failed: %v %v", lexed.Errors, errs)
	}
	return &cart.Snapshot{Cart: c, Chunk: chunk, Comments: lexed.Comments, Index: syntax.NewCommentIndex(lexed.Comments)}
}

func TestM4InferencePerformance(t *testing.T) {
	if analysisRaceDetectorEnabled {
		t.Skip("timing targets are not meaningful under the race detector")
	}
	text := typeInferenceCartText()
	if len(text) < 64*1024 {
		t.Fatalf("cart size = %d, want at least 64 KB", len(text))
	}
	snap := typeInferenceSnapshot(t, text)
	builtins := testBuiltins(t)
	for _, tc := range []struct {
		mode   TypeCheckingMode
		target time.Duration
	}{
		{mode: TypeCheckingBasic, target: 10 * time.Millisecond},
		{mode: TypeCheckingStrict, target: 20 * time.Millisecond},
	} {
		const iterations = 15
		samples := make([]time.Duration, 0, iterations)
		for i := 0; i < iterations; i++ {
			start := time.Now()
			_ = Analyze(snap, builtins, Options{UndefinedGlobal: SeverityWarning, TypeChecking: tc.mode})
			samples = append(samples, time.Since(start))
		}
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		median := samples[len(samples)/2]
		best := samples[0]
		t.Logf("%s inference: median %v, best %v (target %v)", tc.mode, median, best, tc.target)
		// Package tests run in parallel with other packages, so use the best
		// sample for the CI gate. The median is logged for local benchmarking.
		if best > tc.target {
			t.Errorf("%s inference best %v exceeds target %v (median %v)", tc.mode, best, tc.target, median)
		}
	}
}
