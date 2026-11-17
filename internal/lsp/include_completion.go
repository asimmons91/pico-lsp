package lsp

import (
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"go.lsp.dev/protocol"
)

const (
	maxIncludeFiles       = 500
	maxIncludeDepth       = 6
	includeCompletionKind = protocol.CompletionItemKindFile
)

func (s *Server) includeCompletions(nav *navContext, offset int, linePrefix string) []protocol.CompletionItem {
	pathStart, typed, ok := includePathContext(linePrefix)
	if !ok {
		return []protocol.CompletionItem{}
	}
	pathStart += offset - len(linePrefix)
	if pathStart < 0 || pathStart > offset {
		return []protocol.CompletionItem{}
	}
	normalized := strings.ReplaceAll(typed, "\\", "/")
	if strings.ContainsAny(normalized, `"'`) || strings.HasPrefix(normalized, "/") {
		return []protocol.CompletionItem{}
	}

	root := filepath.Dir(nav.snap.Cart.RootURI.FsPath())
	target := filepath.Join(root, filepath.FromSlash(path.Dir(normalized)))
	if !pathWithin(s.includeAllowedRoot(root), target) {
		return []protocol.CompletionItem{}
	}
	dirPart, basePart := path.Split(normalized)
	current := filepath.Clean(nav.file.URI.FsPath())
	files := includeFiles(target, maxIncludeFiles)
	items := make([]protocol.CompletionItem, 0, len(files))
	for _, relative := range files {
		if relative == "" || filepath.Clean(filepath.Join(target, filepath.FromSlash(relative))) == current {
			continue
		}
		if basePart != "" && !strings.HasPrefix(relative, basePart) {
			continue
		}
		insert := dirPart + relative
		items = append(items, protocol.CompletionItem{
			Label:            insert,
			Kind:             includeCompletionKind,
			Detail:           protocol.NewOptional("Lua include"),
			FilterText:       protocol.NewOptional(insert),
			SortText:         protocol.NewOptional(insert),
			TextEdit:         &protocol.TextEdit{Range: nav.file.Index.Range(pathStart, offset), NewText: insert},
			InsertTextFormat: protocol.InsertTextFormatPlainText,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Label < items[j].Label
	})
	if len(items) > maxIncludeFiles {
		items = items[:maxIncludeFiles]
	}
	return items
}

func isIncludeKeywordContext(linePrefix string) bool {
	trimmed := strings.TrimLeft(linePrefix, " \t")
	if trimmed == "" || trimmed[0] != '#' || strings.HasPrefix(trimmed, "#include") {
		return false
	}
	return strings.HasPrefix("#include", trimmed)
}

func (s *Server) includeKeywordCompletions(nav *navContext, offset int, linePrefix string) []protocol.CompletionItem {
	trimmed := strings.TrimLeft(linePrefix, " \t")
	start := offset - len(trimmed)
	for _, snippet := range completionSnippets {
		if snippet.Label != "#include" {
			continue
		}
		return []protocol.CompletionItem{{
			Label:            snippet.Label,
			Kind:             protocol.CompletionItemKindSnippet,
			Detail:           protocol.NewOptional(snippet.Detail),
			FilterText:       protocol.NewOptional("#include"),
			SortText:         protocol.NewOptional("0_#include"),
			TextEdit:         &protocol.TextEdit{Range: nav.file.Index.Range(start, offset), NewText: snippet.Insert},
			InsertTextFormat: protocol.InsertTextFormatSnippet,
		}}
	}
	return []protocol.CompletionItem{}
}

func includePathContext(linePrefix string) (int, string, bool) {
	trimmed := strings.TrimLeft(linePrefix, " \t")
	if !strings.HasPrefix(trimmed, "#include") {
		return 0, "", false
	}
	if len(trimmed) > len("#include") && trimmed[len("#include")] != ' ' && trimmed[len("#include")] != '\t' {
		return 0, "", false
	}
	start := len(linePrefix) - len(trimmed) + len("#include")
	for start < len(linePrefix) && (linePrefix[start] == ' ' || linePrefix[start] == '\t') {
		start++
	}
	if start < len(linePrefix) && (linePrefix[start] == '"' || linePrefix[start] == '\'') {
		start++
	}
	return start, linePrefix[start:], true
}

// includeAllowedRoot returns the highest directory include completion may
// traverse. Workspace folders widen the boundary; without one, includes stay
// inside the cart directory.
func (s *Server) includeAllowedRoot(root string) string {
	allowed := filepath.Clean(root)
	for _, folder := range s.WorkspaceFolders() {
		candidate := filepath.Clean(folder.FsPath())
		if pathWithin(candidate, root) && len(candidate) < len(allowed) {
			allowed = candidate
		}
	}
	return allowed
}

func pathWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func includeFiles(root string, limit int) []string {
	var out []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if path == root {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil || strings.Count(rel, string(filepath.Separator)) >= maxIncludeDepth {
				return fs.SkipDir
			}
			if strings.HasPrefix(entry.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if len(out) >= limit {
			return fs.SkipAll
		}
		if strings.ToLower(filepath.Ext(path)) != ".lua" {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out
}
