package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/config"
)

func TestLimitSeverityThresholds(t *testing.T) {
	cases := []struct {
		count  int
		show   bool
		severe protocol.DiagnosticSeverity
	}{
		{CompressedWarning - 1, false, 0},
		{CompressedWarning, true, protocol.DiagnosticSeverityWarning},
		{CompressedLimit, true, protocol.DiagnosticSeverityWarning},
		{CompressedLimit + 1, true, protocol.DiagnosticSeverityError},
	}
	for _, tc := range cases {
		severity, show := limitSeverity(tc.count, CompressedWarning, CompressedLimit)
		if show != tc.show || severity != tc.severe {
			t.Errorf("limitSeverity(%d) = %v/%v, want %v/%v", tc.count, severity, show, tc.severe, tc.show)
		}
	}
}

func TestBudgetStatusThresholds(t *testing.T) {
	cases := []struct {
		size int
		want string
	}{
		{CompressedWarning - 1, BudgetStatusOK},
		{CompressedWarning, BudgetStatusWarning},
		{CompressedLimit, BudgetStatusWarning},
		{CompressedLimit + 1, BudgetStatusError},
	}
	for _, tc := range cases {
		if got := budgetStatus(tc.size, CompressedWarning, CompressedLimit); got != tc.want {
			t.Errorf("budgetStatus(%d) = %q, want %q", tc.size, got, tc.want)
		}
	}
}

// compressedBudgetText builds a valid cart whose payload is a single
// high-entropy comment. The compressor falls back to the raw size for
// incompressible text, giving deterministic budget thresholds.
func compressedBudgetText(payload int) string {
	var b strings.Builder
	b.WriteString("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n--")
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rng := rand.New(rand.NewSource(17))
	for i := 0; i < payload; i++ {
		b.WriteByte(alphabet[rng.Intn(len(alphabet))])
	}
	b.WriteString("\nx=1\n")
	return b.String()
}

func requestCartInfo(t *testing.T, h *harness, ctx context.Context, u uri.URI) CartInfo {
	t.Helper()
	params, _ := protocol.Marshal(map[string]any{"textDocument": map[string]string{"uri": u.String()}})
	result, err := h.srv.Request(ctx, RequestCartInfo, protocol.LSPAny(params))
	if err != nil {
		t.Fatalf("cartInfo: %v", err)
	}
	info, ok := result.(CartInfo)
	if !ok {
		t.Fatalf("cartInfo result = %T", result)
	}
	return info
}

func TestCompressedBudgetCartInfoAndLens(t *testing.T) {
	tests := []struct {
		name       string
		payload    int
		status     string
		lensSuffix string
	}{
		{"below", 1000, BudgetStatusOK, ""},
		{"warning", 15000, BudgetStatusWarning, " (warning)"},
		{"error", 20000, BudgetStatusError, " (over limit)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text := compressedBudgetText(tc.payload)
			root := writeTemp(t, t.TempDir(), "cart.p8", text)
			h := newHarness(t, newTestClient())
			h.initialize(t, &protocol.InitializeParams{})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			openDoc(t, h, ctx, root, text)

			snap, _, ok := h.srv.carts.SnapshotFor(ctx, root)
			if !ok {
				t.Fatal("missing snapshot")
			}
			size, available := snap.CompressedSize()
			if !available {
				t.Fatal("compressed size unavailable")
			}
			if size <= tc.payload {
				t.Fatalf("compressed size %d should exceed the %d-byte payload", size, tc.payload)
			}

			info := requestCartInfo(t, h, ctx, root)
			if info.Compressed != size || info.CompressedLimit != CompressedLimit ||
				info.CompressedWarning != CompressedWarning || info.CompressedStatus != tc.status {
				t.Fatalf("cartInfo = %+v (size %d)", info, size)
			}

			lenses, err := h.server.CodeLens(ctx, &protocol.CodeLensParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: root},
			})
			if err != nil || len(lenses) != 1 {
				t.Fatalf("lenses = %+v, err=%v", lenses, err)
			}
			want := fmt.Sprintf("%d / %d compressed%s", size, CompressedLimit, tc.lensSuffix)
			if !strings.Contains(lenses[0].Command.Title, want) {
				t.Fatalf("lens = %q, want %q", lenses[0].Command.Title, want)
			}
		})
	}
}

func TestCompressedLimitDiagnostics(t *testing.T) {
	tests := []struct {
		name     string
		payload  int
		severity protocol.DiagnosticSeverity
	}{
		{"below", 1000, 0},
		{"warning", 15000, protocol.DiagnosticSeverityWarning},
		{"error", 20000, protocol.DiagnosticSeverityError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text := compressedBudgetText(tc.payload)
			root := writeTemp(t, t.TempDir(), "cart.p8", text)
			h := newHarness(t, newTestClient())
			h.initialize(t, &protocol.InitializeParams{})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			openDoc(t, h, ctx, root, text)

			snap, semantics, _, ok := h.srv.carts.SemanticsFor(ctx, root)
			if !ok {
				t.Fatal("missing snapshot")
			}
			items := h.srv.carts.collectDiagnostics(snap, semantics)
			var compressed *protocol.Diagnostic
			for i := range items[root] {
				if code, _ := items[root][i].Code.(protocol.String); string(code) == "compressed-size-limit" {
					compressed = &items[root][i]
					break
				}
			}
			if tc.severity == 0 {
				if compressed != nil {
					t.Fatalf("unexpected compressed diagnostic: %+v", compressed)
				}
				return
			}
			if compressed == nil {
				t.Fatalf("diagnostics = %+v, missing compressed-size-limit", items[root])
			}
			if compressed.Severity != tc.severity {
				t.Fatalf("severity = %v, want %v", compressed.Severity, tc.severity)
			}
			if !strings.Contains(string(compressed.Message.(protocol.String)), "compressed bytes") {
				t.Fatalf("message = %q", compressed.Message)
			}
		})
	}
}

func TestCompressedLimitDiagnosticsWire(t *testing.T) {
	text := compressedBudgetText(15000)
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	params := waitDiagnostic(t, h.client, root)
	if !containsString(diagnosticCodes(params), "compressed-size-limit") {
		t.Fatalf("diagnostics = %v, want compressed-size-limit", diagnosticCodes(params))
	}
}

func TestCompressedBudgetStandaloneLua(t *testing.T) {
	text := "x=1\n"
	root := writeTemp(t, t.TempDir(), "standalone.lua", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	info := requestCartInfo(t, h, ctx, root)
	if info.CompressedStatus != BudgetStatusUnavailable || info.Compressed != 0 {
		t.Fatalf("standalone cartInfo = %+v", info)
	}
	lenses, err := h.server.CodeLens(ctx, &protocol.CodeLensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
	})
	if err != nil || len(lenses) != 1 {
		t.Fatalf("lenses = %+v, err=%v", lenses, err)
	}
	if !strings.Contains(lenses[0].Command.Title, "compressed n/a") {
		t.Fatalf("lens = %q, want compressed n/a", lenses[0].Command.Title)
	}
}

func TestCompressedBudgetDisabled(t *testing.T) {
	text := compressedBudgetText(20000)
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	h := initializeWithOptions(t, `{"pico8":{"limits":{"codeLens":false,"diagnostics":false}}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	lenses, err := h.server.CodeLens(ctx, &protocol.CodeLensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
	})
	if err != nil || len(lenses) != 0 {
		t.Fatalf("lenses = %+v, err=%v", lenses, err)
	}
	if diags := cartLimitDiagnostics(mustSnapshot(t, h, ctx, root), h.srv.cfg.Settings()); len(diags) != 0 {
		t.Fatalf("disabled limit diagnostics = %+v", diags)
	}
}

func mustSnapshot(t *testing.T, h *harness, ctx context.Context, u uri.URI) *cart.Snapshot {
	t.Helper()
	snap, _, ok := h.srv.carts.SnapshotFor(ctx, u)
	if !ok {
		t.Fatal("missing snapshot")
	}
	return snap
}

func TestCompressedBudgetOversized(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n--" + strings.Repeat("x", 70000) + "\n"
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	info := requestCartInfo(t, h, ctx, root)
	if info.CompressedStatus != BudgetStatusUnavailable || info.Compressed != 0 {
		t.Fatalf("oversized cartInfo = %+v", info)
	}
	snap, semantics, _, ok := h.srv.carts.SemanticsFor(ctx, root)
	if !ok {
		t.Fatal("missing snapshot")
	}
	items := h.srv.carts.collectDiagnostics(snap, semantics)
	codes := make([]string, 0, len(items[root]))
	for _, diag := range items[root] {
		code, _ := diag.Code.(protocol.String)
		codes = append(codes, string(code))
		if string(code) == "compressed-size-limit" {
			t.Fatalf("oversized cart published compressed-size-limit: %+v", diag)
		}
	}
	if !containsString(codes, "char-limit") {
		t.Fatalf("diagnostics = %v, want char-limit", codes)
	}
}

func TestCorpusCompressedBudgets(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "p8compress", "corpus.json"))
	if err != nil {
		t.Fatalf("read corpus sizes: %v", err)
	}
	type corpusEntry struct {
		Path       string `json:"path"`
		Compressed int    `json:"compressed"`
	}
	var expected []corpusEntry
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("decode corpus sizes: %v", err)
	}
	want := make(map[string]int, len(expected))
	for _, entry := range expected {
		want[entry.Path] = entry.Compressed
	}

	root := filepath.Join("..", "..", "testdata", "carts")
	loader := cart.NewLoader(cart.DiskProvider{}, "", nil)
	seen := make(map[string]bool)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".p8" {
			return nil
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		snap, err := loader.Load(uri.File(abs))
		if err != nil {
			t.Errorf("%s: load: %v", path, err)
			return nil
		}
		size, ok := snap.CompressedSize()
		if !ok {
			t.Errorf("%s: compressed size unavailable", path)
			return nil
		}
		rel, _ := filepath.Rel(filepath.Join("..", ".."), path)
		rel = filepath.ToSlash(rel)
		seen[rel] = true
		if size != want[rel] {
			t.Errorf("%s: compressed size %d, want %d", rel, size, want[rel])
		}
		if size >= CompressedWarning {
			t.Errorf("%s: compressed size %d is at or above the warning threshold", path, size)
		}
		for _, diag := range cartLimitDiagnostics(snap, config.Default()) {
			if code, _ := diag.Code.(protocol.String); string(code) == "compressed-size-limit" {
				t.Errorf("%s: compressed-size-limit diagnostic for %d bytes", path, size)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) < 50 {
		t.Fatalf("corpus has %d carts, want at least 50", len(seen))
	}
	for path := range want {
		if !seen[path] {
			t.Errorf("corpus fixture %s was not visited", path)
		}
	}
}

func TestCompressedSizePerformance(t *testing.T) {
	if raceDetectorEnabled || runtime.GOOS != "linux" {
		t.Skip("timing targets are not meaningful under the race detector")
	}
	var b strings.Builder
	b.WriteString("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n")
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&b, "value_%03d = %d + circfill(%d, 2, 3, 4)\n", i, i, i)
	}
	b.WriteString("-- ")
	for b.Len() < 64*1024 {
		b.WriteString("padding padding padding padding padding padding ")
	}
	b.WriteString("\nfunction _update()\n  local speed = 1\nend\n")
	text := b.String()
	if len(text) > 65535 {
		text = text[:65535]
	}
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	loader := cart.NewLoader(cart.DiskProvider{}, "", nil)
	best := time.Hour
	for i := 0; i < 5; i++ {
		snap, err := loader.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		size, ok := snap.CompressedSize()
		elapsed := time.Since(start)
		if !ok {
			t.Fatal("compressed size unavailable")
		}
		if size == 0 {
			t.Fatal("compressed size is zero")
		}
		if elapsed < best {
			best = elapsed
		}
	}
	t.Logf("64 KB cart compression: best %v (target 10ms)", best)
	if best > 10*time.Millisecond {
		t.Errorf("compression best %v exceeds 10ms", best)
	}
}
