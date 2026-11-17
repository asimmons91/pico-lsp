package lsp

import (
	"sort"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/analysis/types"
)

var annotationKeywords = []string{
	"param", "return", "class", "field", "type", "alias", "enum", "meta",
	"deprecated", "cast", "diagnostic", "generic", "overload", "operator",
}

var luaOperatorCompletionNames = []string{
	"add", "band", "bnot", "bor", "bxor", "call", "concat", "div", "eq",
	"idiv", "le", "len", "lt", "mod", "mul", "pow", "rotl", "rotr", "shl",
	"shr", "sub", "unm",
}

var annotationTypes = []string{
	"any", "boolean", "function", "integer", "nil", "number", "string",
	"table", "thread", "userdata",
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
	case "operator":
		if strings.Contains(rest, "(") {
			return annotationNameItems(nav, start, offset, partial, annotationTypeNames(nav))
		}
		return annotationNameItems(nav, start, offset, partial, luaOperatorCompletionNames)
	case "generic":
		return annotationNameItems(nav, start, offset, partial, []string{"T", "K", "V"})
	case "overload":
		return annotationOverloadItems(nav, start, offset, partial)
	}
	return []protocol.CompletionItem{}
}

// annotationSnippetItems offers the M3 doc-block snippets when the cursor is
// on a bare `---@` prefix. They replace the annotation prefix so clients never
// end up with a doubled `---@`.
func annotationSnippetItems(nav *navContext, offset int, linePrefix string) []protocol.CompletionItem {
	trimmed := strings.TrimLeft(linePrefix, " \t")
	if !strings.HasPrefix(trimmed, "---@") || strings.TrimSpace(strings.TrimPrefix(trimmed, "---@")) != "" {
		return nil
	}
	start := offset - len(trimmed)
	items := make([]protocol.CompletionItem, 0, 2)
	for _, snippet := range completionSnippets {
		if snippet.Label != "class annotation" && snippet.Label != "function docs" {
			continue
		}
		items = append(items, protocol.CompletionItem{
			Label:            snippet.Label,
			Kind:             protocol.CompletionItemKindSnippet,
			Detail:           protocol.NewOptional(snippet.Detail),
			FilterText:       protocol.NewOptional(snippet.Label),
			SortText:         protocol.NewOptional("5_" + snippet.Label),
			TextEdit:         &protocol.TextEdit{Range: nav.file.Index.Range(start, offset), NewText: snippet.Insert},
			InsertTextFormat: protocol.InsertTextFormatSnippet,
		})
	}
	return items
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

func annotationOverloadItems(nav *navContext, start, offset int, partial string) []protocol.CompletionItem {
	if strings.TrimSpace(partial) != "" {
		return nil
	}
	item := protocol.CompletionItem{
		Label:            "fun(...)",
		Kind:             protocol.CompletionItemKindSnippet,
		Detail:           protocol.NewOptional("overload signature"),
		FilterText:       protocol.NewOptional("fun"),
		TextEdit:         &protocol.TextEdit{Range: nav.file.Index.Range(start, offset), NewText: "fun(${1:args}): ${2:returns}"},
		InsertTextFormat: protocol.InsertTextFormatSnippet,
	}
	return []protocol.CompletionItem{item}
}

func annotationClassNames(nav *navContext) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(name string) {
		name = strings.TrimSuffix(name, ":")
		if name == "" {
			return
		}
		key := analysis.CanonicalName(name)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	if nav != nil && nav.semantics != nil {
		for _, typ := range nav.semantics.TypeNames() {
			if record, ok := types.Resolve(typ).(*types.Record); ok && record != nil && record.Name != "" {
				add(record.Name)
				continue
			}
			add(types.Display(typ))
		}
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
