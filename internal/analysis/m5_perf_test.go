package analysis

import (
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/asimmons91/pico-lsp/internal/testutil"
)

func TestM5AnalysisPerformance(t *testing.T) {
	testutil.SkipUnlessPerf(t)
	if analysisRaceDetectorEnabled || runtime.GOOS != "linux" || testing.CoverMode() != "" {
		t.Skip("timing targets are not meaningful under the race detector or coverage instrumentation")
	}
	text := typeInferenceCartText()
	snap := typeInferenceSnapshot(t, text)
	builtins := testBuiltins(t)
	lint := LintOptions{
		CallbackPrecedence:  SeverityHint,
		CallbackParams:      SeverityWarning,
		NoStdlib:            SeverityHint,
		DuplicateTableField: SeverityHint,
		UnreachableCode:     SeverityHint,
	}
	const iterations = 10
	samples := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		_ = Analyze(snap, builtins, Options{UndefinedGlobal: SeverityWarning, TypeChecking: TypeCheckingBasic, Lint: lint})
		samples = append(samples, time.Since(start))
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	median := samples[len(samples)/2]
	best := samples[0]
	t.Logf("full analysis + diagnostics: median %v, best %v (target 15ms)", median, best)
	if best > 15*time.Millisecond {
		t.Fatalf("best analysis time %v exceeds 15ms (median %v)", best, median)
	}
}
