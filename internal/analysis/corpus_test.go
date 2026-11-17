package analysis

import (
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/builtin"
	"github.com/asimmons91/pico-lsp/internal/cart"
)

func corpusCarts(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", "carts")
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".p8" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}
	return files
}

// TestCorpusAnalyzes runs the binder over the M1 corpus. It is primarily a
// panic and span-sanity guard; diagnostic counts are logged so noisy patterns
// can be investigated without making the corpus a brittle golden test.
func TestCorpusAnalyzes(t *testing.T) {
	files := corpusCarts(t)
	if len(files) < 50 {
		t.Fatalf("corpus has %d carts, want at least 50", len(files))
	}

	loader := cart.NewLoader(nil, "", nil)
	builtins := testBuiltins(t)
	totalDiagnostics := 0
	for _, path := range files {
		abs, err := filepath.Abs(path)
		if err != nil {
			t.Fatalf("abs: %v", err)
		}
		start := time.Now()
		snap, err := loader.Load(uri.File(abs))
		if err != nil {
			t.Errorf("%s: load: %v", path, err)
			continue
		}
		result := Analyze(snap, builtins, Options{UndefinedGlobal: SeverityWarning})
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Errorf("%s: analyze took %v", path, elapsed)
		}
		totalDiagnostics += len(result.Diagnostics)
		for i, diag := range result.Diagnostics {
			if isTypeDiagnosticCode(diag.Code) {
				t.Errorf("%s: unannotated corpus produced type diagnostic %s: %s", path, diag.Code, diag.Message)
			}
			if i >= 3 {
				break
			}
			file := snap.Cart.File(diag.Span.FileID)
			name := ""
			if file != nil {
				name = file.URI.String()
			}
			t.Logf("%s: %s %s:%d %s", path, diag.Code, name, diag.Span.Start, diag.Message)
		}
		for _, occ := range result.occs {
			file := snap.Cart.File(occ.Span.FileID)
			if file == nil {
				t.Errorf("%s: occurrence has unknown file %d", path, occ.Span.FileID)
				continue
			}
			if occ.Span.Start < 0 || occ.Span.End < occ.Span.Start || occ.Span.End > len(file.Text) {
				t.Errorf("%s: occurrence span out of bounds: %+v (len %d)", path, occ.Span, len(file.Text))
			}
		}
	}
	t.Logf("analyzed %d carts, %d diagnostics", len(files), totalDiagnostics)
}

func BenchmarkAnalyzeCorpusCart(b *testing.B) {
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "carts", "apa64__try-pico8", "topdown1.p8"))
	if err != nil {
		b.Fatal(err)
	}
	loader := cart.NewLoader(nil, "", nil)
	snap, err := loader.Load(uri.File(path))
	if err != nil {
		b.Fatal(err)
	}
	api, err := builtin.Load()
	if err != nil {
		b.Fatal(err)
	}
	builtins := BuildBuiltins(api)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Analyze(snap, builtins, Options{UndefinedGlobal: SeverityWarning})
	}
}

func TestM5CorpusLintDefaults(t *testing.T) {
	files := corpusCarts(t)
	loader := cart.NewLoader(nil, "", nil)
	builtins := testBuiltins(t)
	lint := LintOptions{
		CallbackPrecedence:  SeverityHint,
		CallbackParams:      SeverityWarning,
		NoStdlib:            SeverityHint,
		DuplicateTableField: SeverityHint,
		UnreachableCode:     SeverityHint,
	}
	counts := make(map[string]int)
	for _, path := range files {
		abs, err := filepath.Abs(path)
		if err != nil {
			t.Fatalf("abs: %v", err)
		}
		snap, err := loader.Load(uri.File(abs))
		if err != nil {
			t.Fatalf("%s: load: %v", path, err)
		}
		result := Analyze(snap, builtins, Options{Lint: lint})
		for _, diag := range result.Diagnostics {
			switch diag.Code {
			case CodeCallbackPrecedence, CodeCallbackParams, CodeNoStdlib, CodeDuplicateTableField, CodeUnreachableCode:
				counts[diag.Code]++
			}
		}
	}
	t.Logf("M5 lint diagnostics across %d corpus carts: %v", len(files), counts)
	if counts[CodeNoStdlib] > 0 || counts[CodeDuplicateTableField] > 0 || counts[CodeUnreachableCode] > 0 {
		t.Fatalf("corpus lint noise: %v", counts)
	}
	if counts[CodeCallbackParams] > len(files)/10 {
		t.Fatalf("callback parameter lint fired too often: %v", counts)
	}
}
