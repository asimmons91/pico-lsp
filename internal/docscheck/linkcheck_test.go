// Package docscheck contains repository documentation checks.
package docscheck

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

var markdownLink = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

func TestLocalMarkdownLinks(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	files, err := markdownFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no Markdown files found")
	}

	anchors := make(map[string]map[string]bool)
	for _, file := range files {
		anchors[file] = headingAnchors(file)
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range proseLines(string(data)) {
			line.text = stripInlineCode(line.text)
			for _, match := range markdownLink.FindAllStringSubmatch(line.text, -1) {
				target := linkTarget(match[1])
				if target == "" || isExternal(target) {
					continue
				}
				if strings.HasPrefix(target, "#") {
					fragment := strings.TrimPrefix(target, "#")
					if !anchors[file][fragment] {
						t.Errorf("%s:%d: missing anchor %q", file, line.number, target)
					}
					continue
				}
				pathPart := target
				fragment := ""
				if index := strings.IndexByte(target, '#'); index >= 0 {
					pathPart = target[:index]
					fragment = target[index+1:]
				}
				resolved := filepath.Join(filepath.Dir(file), filepath.FromSlash(pathPart))
				if _, err := os.Stat(resolved); err != nil {
					t.Errorf("%s:%d: missing local link target %q", file, line.number, target)
					continue
				}
				if fragment != "" && strings.EqualFold(filepath.Ext(resolved), ".md") {
					if !anchors[resolved][fragment] {
						t.Errorf("%s:%d: missing anchor %q in %s", file, line.number, fragment, resolved)
					}
				}
			}
		}
	}
}

type proseLine struct {
	number int
	text   string
}

func proseLines(data string) []proseLine {
	var lines []proseLine
	inFence := false
	for index, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		lines = append(lines, proseLine{number: index + 1, text: line})
	}
	return lines
}

func markdownFiles(root string) ([]string, error) {
	var files []string
	for _, name := range []string{"README.md", "AGENTS.md", "CHANGELOG.md", "CONTRIBUTING.md", "SECURITY.md"} {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err == nil {
			files = append(files, path)
		}
	}
	for _, dir := range []string{"docs", "editors"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				switch entry.Name() {
				case "node_modules", ".vscode-test", "dist", ".git":
					return filepath.SkipDir
				}
				return nil
			}
			if strings.EqualFold(filepath.Ext(path), ".md") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func headingAnchors(path string) map[string]bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	anchors := make(map[string]bool)
	seen := make(map[string]int)
	for _, line := range proseLines(string(data)) {
		trimmed := strings.TrimSpace(line.text)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		trimmed = strings.TrimLeft(trimmed, "#")
		base := anchorSlug(trimmed)
		if base == "" {
			continue
		}
		anchor := base
		if count := seen[base]; count > 0 {
			anchor = base + "-" + strconv.Itoa(count)
		}
		seen[base]++
		anchors[anchor] = true
	}
	return anchors
}

func anchorSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_':
			out.WriteRune(r)
		case unicode.IsSpace(r):
			out.WriteByte('-')
		}
	}
	return out.String()
}

func stripInlineCode(line string) string {
	var out strings.Builder
	inCode := false
	for _, r := range line {
		if r == '`' {
			inCode = !inCode
			continue
		}
		if !inCode {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func linkTarget(raw string) string {
	raw = strings.TrimSpace(raw)
	if fields := strings.Fields(raw); len(fields) > 0 {
		raw = fields[0]
	}
	return strings.Trim(raw, "<>")
}

func isExternal(target string) bool {
	return strings.HasPrefix(target, "http://") ||
		strings.HasPrefix(target, "https://") ||
		strings.HasPrefix(target, "mailto:") ||
		strings.HasPrefix(target, "tel:")
}
