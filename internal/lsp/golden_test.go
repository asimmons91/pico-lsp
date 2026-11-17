package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goldenPath returns a path under testdata/lsp. Tests run from internal/lsp.
func goldenPath(name string) string {
	return filepath.Join("..", "..", "testdata", "lsp", filepath.FromSlash(name))
}

func goldenBytes(t *testing.T, name string, got []byte) {
	t.Helper()
	path := goldenPath(name)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", name, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run UPDATE_GOLDEN=1 go test ./internal/lsp)", name, err)
	}
	wantText := strings.ReplaceAll(string(want), "\r\n", "\n")
	gotText := strings.ReplaceAll(string(got), "\r\n", "\n")
	if wantText == gotText {
		return
	}
	t.Fatalf("golden %s mismatch (run UPDATE_GOLDEN=1 go test ./internal/lsp)\n%s", name, firstDifference(wantText, gotText))
}

func goldenJSON(t *testing.T, name string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal golden %s: %v", name, err)
	}
	data = append(data, '\n')
	goldenBytes(t, name, data)
}

func goldenText(t *testing.T, name, got string) {
	t.Helper()
	goldenBytes(t, name, []byte(got))
}

func firstDifference(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			return fmt.Sprintf("first difference at line %d:\nwant: %q\ngot:  %q", i+1, w, g)
		}
	}
	return "content differs"
}
