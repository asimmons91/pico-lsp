package lsp

import (
	"sort"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/analysis"
)

var annotationKeywords = []string{
	"param", "return", "class", "field", "type", "alias", "enum", "meta",
	"deprecated", "cast", "diagnostic",
}

var annotationTypes = []string{
	"any", "boolean", "function", "integer", "nil", "number", "string",
	"table", "thread",
}

func annotationCompletions(nav *navContext, offset int, linePrefix string) []protocol.CompletionItem {
	trimmed := strings.TrimLeft(linePrefix, " \t")
	if !strings.HasPrefix(trimmed, "---@") {
		return []protocol.CompletionItem{}
	}
	rest := trimmed[len("---@"):]
	if rest == "" || !strings.ContainsAny(rest, " \t") {
		return annotationKeywordItems(nav, offset, rest)
	}

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return []protocol.CompletionItem{}
	}
	keyword := fields[0]
	lastSpace := strings.LastIndexAny(rest, " \t")
	if lastSpace < 0 {
		return nil
	}
	partial := rest[lastSpace+1:]
	start := offset - len(partial)

	switch keyword {
	case "class", "alias":
		return annotationNameItems(nav, start, offset, partial, annotationClassNames(nav))
	case "type", "return":
		return annotationNameItems(nav, start, offset, partial, annotationTypeNames(nav))
	case "param":
		if len(fields) >= 2 {
			return annotationNameItems(nav, start, offset, partial, annotationTypeNames(nav))
		}
	case "field":
		if len(fields) >= 2 {
			return annotationNameItems(nav, start, offset, partial, annotationTypeNames(nav))
		}
	}
	return []protocol.CompletionItem{}
}

func annotationKeywordItems(nav *navContext, offset int, partial string) []protocol.CompletionItem {
	start := offset - len(partial)
	prefix := analysis.CanonicalName(partial)
	items := make([]protocol.CompletionItem, 0, len(annotationKeywords))
	for _, keyword := range annotationKeywords {
		if prefix != "" && !strings.HasPrefix(analysis.CanonicalName(keyword), prefix) {
			continue
		}
		kind := protocol.CompletionItemKindKeyword
		switch keyword {
		case "class", "alias":
			kind = protocol.CompletionItemKindClass
		}
		items = append(items, protocol.CompletionItem{
			Label:      keyword,
			Kind:       kind,
			Detail:     protocol.NewOptional("---@" + keyword),
			FilterText: protocol.NewOptional(keyword),
			SortText:   protocol.NewOptional(analysis.CanonicalName(keyword)),
			TextEdit: &protocol.TextEdit{
				Range:   nav.file.Index.Range(start, offset),
				NewText: keyword,
			},
			InsertTextFormat: protocol.InsertTextFormatPlainText,
		})
	}
	return items
}

func annotationNameItems(nav *navContext, start, offset int, partial string, names []string) []protocol.CompletionItem {
	prefix := analysis.CanonicalName(partial)
	items := make([]protocol.CompletionItem, 0, len(names))
	for _, name := range names {
		if prefix != "" && !strings.HasPrefix(analysis.CanonicalName(name), prefix) {
			continue
		}
		kind := protocol.CompletionItemKindClass
		items = append(items, protocol.CompletionItem{
			Label:      name,
			Kind:       kind,
			FilterText: protocol.NewOptional(name),
			SortText:   protocol.NewOptional(analysis.CanonicalName(name)),
			TextEdit: &protocol.TextEdit{
				Range:   nav.file.Index.Range(start, offset),
				NewText: name,
			},
			InsertTextFormat: protocol.InsertTextFormatPlainText,
		})
	}
	return items
}

func annotationClassNames(nav *navContext) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(name string) {
		name = strings.TrimSuffix(name, ":")
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if nav != nil && nav.snap != nil {
		for _, name := range analysis.AnnotationNames(nav.snap.Comments) {
			add(name)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return analysis.CanonicalName(out[i]) < analysis.CanonicalName(out[j])
	})
	return out
}

func annotationTypeNames(nav *navContext) []string {
	names := append([]string(nil), annotationTypes...)
	names = append(names, annotationClassNames(nav)...)
	sort.SliceStable(names, func(i, j int) bool {
		return analysis.CanonicalName(names[i]) < analysis.CanonicalName(names[j])
	})
	return names
}
