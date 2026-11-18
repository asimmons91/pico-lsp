// Package testutil contains small helpers shared by tests.
package testutil

import (
	"os"
	"testing"
)

// SkipUnlessPerf skips wall-clock performance gates unless they are explicitly
// requested. Shared CI runners are too noisy for hard timing assertions; run
// PICO_LSP_PERF=1 locally or through the manual performance workflow.
func SkipUnlessPerf(t testing.TB) {
	t.Helper()
	if os.Getenv("PICO_LSP_PERF") != "1" {
		t.Skip("set PICO_LSP_PERF=1 to run wall-clock performance gates")
	}
}
