package format

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asimmons91/pico-lsp/internal/syntax"
)

func TestCorpusFormatsIdempotently(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "carts")
	files := 0
	formatted := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".p8" {
			return nil
		}
		files++
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("%s: %v", path, readErr)
		}
		src := string(data)
		got, formatErr := P8(src, DefaultOptions())
		if formatErr != nil {
			// The corpus includes deliberately broken carts. A formatter must
			// refuse those rather than rewrite them.
			if formatErr != ErrSyntax {
				t.Fatalf("%s: unexpected format error: %v", path, formatErr)
			}
			return nil
		}
		formatted++
		if !sameTokens(t, codeSection(src), codeSection(got)) {
			t.Fatalf("%s: token stream changed", path)
		}
		if !sameComments(t, codeSection(src), codeSection(got)) {
			t.Fatalf("%s: comments changed", path)
		}
		again, secondErr := P8(got, DefaultOptions())
		if secondErr != nil {
			t.Fatalf("%s: second format: %v", path, secondErr)
		}
		if again != got {
			t.Fatalf("%s: formatting is not idempotent", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 50 || formatted == 0 {
		t.Fatalf("corpus files = %d, formatted = %d", files, formatted)
	}
	t.Logf("formatted %d of %d corpus carts", formatted, files)
}

func codeSection(src string) string {
	lines := splitLines(src)
	lua := -1
	for i, line := range lines {
		if name, ok := sectionName(line.text); ok && strings.EqualFold(name, "lua") {
			lua = i
			break
		}
	}
	if lua < 0 {
		return ""
	}
	end := len(lines)
	for i := lua + 1; i < len(lines); i++ {
		if _, ok := sectionName(lines[i].text); ok {
			end = i
			break
		}
	}
	start := lines[lua+1].start
	finish := len(src)
	if end < len(lines) {
		finish = lines[end].start
	}
	return src[start:finish]
}

func tokenText(src string) []string {
	lexed := syntax.Lex([]byte(src), nil)
	out := make([]string, 0, len(lexed.Tokens))
	for _, token := range lexed.Tokens {
		if token.Kind == syntax.TokenEOF || token.Kind == syntax.TokenComment || token.Kind == syntax.TokenInclude {
			continue
		}
		out = append(out, token.Kind.String()+":"+token.Text)
	}
	return out
}

func sameTokens(t *testing.T, before, after string) bool {
	t.Helper()
	a, b := tokenText(before), tokenText(after)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameComments(t *testing.T, before, after string) bool {
	t.Helper()
	a := syntax.Lex([]byte(before), nil).Comments
	b := syntax.Lex([]byte(after), nil).Comments
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Kind != b[i].Kind || strings.TrimSpace(a[i].Text) != strings.TrimSpace(b[i].Text) {
			return false
		}
	}
	return true
}
