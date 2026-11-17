package analysis

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func annotatedTypeCartText() string {
	var b strings.Builder
	b.WriteString("---@class Base\n---@field x number\n---@field hp number\nlocal Base = {}\n")
	b.WriteString("---@param dx number\nfunction Base:move(dx)\n  self.x += dx\nend\n")
	b.WriteString("---@class Child: Base\n---@field y number\nlocal Child = {}\n")
	for i := 0; i < 450; i++ {
		fmt.Fprintf(&b, "local obj_%03d = setmetatable({x = %d, y = %d}, {__index = Child})\n", i, i, i)
	}
	for i := 0; i < 120; i++ {
		fmt.Fprintf(&b, "---@param p Child?\nlocal function use_%03d(p)\n  assert(p)\n  local before_%03d = p\n  local x_%03d = p.x\n  if not p then return end\n  local after_%03d = p or obj_000\n  return after_%03d.x\nend\n", i, i, i, i, i)
	}
	b.WriteString("-- ")
	for b.Len() < 64*1024 {
		b.WriteString("annotated padding padding padding padding padding padding ")
	}
	return b.String()
}

func TestM9AnnotatedInferencePerformance(t *testing.T) {
	if analysisRaceDetectorEnabled || runtime.GOOS != "linux" {
		t.Skip("timing targets are not meaningful under the race detector")
	}
	text := annotatedTypeCartText()
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
		t.Logf("%s annotated inference: median %v, best %v (target %v)", tc.mode, median, best, tc.target)
		if best > tc.target {
			t.Errorf("%s annotated inference best %v exceeds target %v (median %v)", tc.mode, best, tc.target, median)
		}
	}
}
