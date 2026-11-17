package p8png

import (
	"os"
	"sort"
	"testing"
	"time"
)

func TestDecodePerformance(t *testing.T) {
	data, err := os.ReadFile("../../testdata/p8png/smoke.p8.png")
	if err != nil {
		t.Fatal(err)
	}
	const iterations = 20
	samples := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		if _, err := Decode(data); err != nil {
			t.Fatal(err)
		}
		samples = append(samples, time.Since(start))
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	best := samples[0]
	t.Logf("PNG decode: best %v, median %v (target 50ms)", best, samples[len(samples)/2])
	if best > 50*time.Millisecond {
		t.Fatalf("PNG decode best %v exceeds 50ms", best)
	}
}
