package format

import (
	"flag"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/asimmons91/pico-lsp/internal/testutil"
)

func TestFormatPerformance(t *testing.T) {
	if fuzz := flag.Lookup("test.fuzz"); fuzz != nil && fuzz.Value.String() != "" {
		t.Skip("timing checks are not meaningful under fuzz instrumentation")
	}
	testutil.SkipUnlessPerf(t)
	var b strings.Builder
	for i := 0; i < 1300; i++ {
		b.WriteString("function f(x,y)\nif (x>0) then\nreturn x+y\nend\nend\n")
	}
	src := b.String()
	const iterations = 10
	samples := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		if _, err := Lua(src, DefaultOptions()); err != nil {
			t.Fatalf("format: %v", err)
		}
		samples = append(samples, time.Since(start))
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	best := samples[0]
	t.Logf("format %d bytes: best %v, median %v (target 25ms)", len(src), best, samples[len(samples)/2])
	if best > 25*time.Millisecond {
		t.Fatalf("best format time %v exceeds 25ms", best)
	}
}
