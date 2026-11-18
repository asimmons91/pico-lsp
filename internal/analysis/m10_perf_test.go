package analysis

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/asimmons91/pico-lsp/internal/testutil"
)

func m10AnnotatedCartText() string {
	var b strings.Builder
	b.WriteString("---@class Box<T>\n---@field value T\n")
	b.WriteString("---@generic T\n---@param value T\n---@return Box<T>\nlocal function box(value)\n  return {value = value}\nend\n")
	b.WriteString("---@overload fun(x: number): number\n---@overload fun(x: string): string\nlocal function pick(x)\n  return x\nend\n")
	b.WriteString("---@class Vec\n---@field x number\n---@operator add(Vec): Vec\n---@operator unm(): Vec\nlocal Vec = {}\n")
	b.WriteString("---@class Shape\n---@field kind \"circle\" | \"square\"\n---@class Circle: Shape\n---@field kind \"circle\"\n---@field radius number\n---@class Square: Shape\n---@field kind \"square\"\n---@field side number\n")
	b.WriteString("---@param shape Circle | Square\nlocal function shape_info(shape)\n  if shape.kind == \"circle\" then\n    local r = shape.radius\n  end\nend\n")
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&b, "local value_%03d = %d + #tostr(%d)\n", i, i, i)
	}
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&b, "local boxed_%03d = box(%d)\n", i, i)
		fmt.Fprintf(&b, "local picked_%03d = pick(%d)\n", i, i)
		fmt.Fprintf(&b, "local vec_a_%03d = setmetatable({x = %d}, {__index = Vec})\n", i, i)
		fmt.Fprintf(&b, "local vec_b_%03d = setmetatable({x = %d}, {__index = Vec})\n", i, i)
		fmt.Fprintf(&b, "local vec_sum_%03d = vec_a_%03d + vec_b_%03d\n", i, i, i)
	}
	b.WriteString("-- ")
	for b.Len() < 64*1024 {
		b.WriteString("m10 padding padding padding padding padding padding ")
	}
	return b.String()
}

func TestM10AnnotatedInferencePerformance(t *testing.T) {
	testutil.SkipUnlessPerf(t)
	if analysisRaceDetectorEnabled || runtime.GOOS != "linux" || testing.CoverMode() != "" {
		t.Skip("timing targets are not meaningful under the race detector or coverage instrumentation")
	}
	text := m10AnnotatedCartText()
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
		const iterations = 30
		_ = Analyze(snap, builtins, Options{UndefinedGlobal: SeverityWarning, TypeChecking: tc.mode})
		samples := make([]time.Duration, 0, iterations)
		for i := 0; i < iterations; i++ {
			start := time.Now()
			_ = Analyze(snap, builtins, Options{UndefinedGlobal: SeverityWarning, TypeChecking: tc.mode})
			samples = append(samples, time.Since(start))
		}
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		median := samples[len(samples)/2]
		best := samples[0]
		t.Logf("%s M10 annotated inference: median %v, best %v (target %v)", tc.mode, median, best, tc.target)
		if best > tc.target {
			t.Errorf("%s M10 annotated inference best %v exceeds target %v (median %v)", tc.mode, best, tc.target, median)
		}
	}
}
