package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/builtin"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

const maxCompletionItems = 1000

type completionData struct {
	Kind string `json:"kind"`
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	URI  string `json:"uri,omitempty"`
}

type completionContext struct {
	nav         *navContext
	offset      int
	lineStart   int
	linePrefix  string
	prefixStart int
	prefix      string
	scope       *analysis.Scope
}

// Completion implements textDocument/completion.
func (s *Server) Completion(ctx context.Context, params *protocol.CompletionParams) (protocol.CompletionResult, error) {
	if params == nil {
		return &protocol.CompletionList{Items: []protocol.CompletionItem{}}, nil
	}
	nav, ok := s.navigationFor(ctx, params.TextDocument.URI)
	if !ok {
		return &protocol.CompletionList{Items: []protocol.CompletionItem{}}, nil
	}
	offset, ok := nav.offset(params.Position)
	if !ok {
		return &protocol.CompletionList{Items: []protocol.CompletionItem{}}, nil
	}
	trigger := ""
	if params.Context.TriggerKind == protocol.CompletionTriggerKindTriggerCharacter && params.Context.TriggerCharacter != nil {
		trigger = *params.Context.TriggerCharacter
	}
	items, incomplete := s.completions(nav, offset, trigger)
	return &protocol.CompletionList{IsIncomplete: incomplete, Items: items}, nil
}

// CompletionResolve attaches documentation and signatures without changing an
// item's insertion text.
func (s *Server) CompletionResolve(ctx context.Context, item *protocol.CompletionItem) (*protocol.CompletionItem, error) {
	if item == nil || item.Data == nil {
		return item, nil
	}
	var data completionData
	if err := json.Unmarshal(item.Data, &data); err != nil {
		return item, nil
	}
	switch data.Kind {
	case "builtin":
		if s.builtins == nil {
			return item, nil
		}
		name, ok := s.builtins.Lookup(analysis.CanonicalName(data.Name))
		if !ok {
			return item, nil
		}
		item.Detail = protocol.NewOptional(name.Signature)
		if doc := builtinMarkdown(name); doc != "" {
			item.Documentation = &protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: doc}
		}
	case "symbol":
		nav, ok := s.navigationFor(ctx, uri.URI(data.URI))
		if !ok {
			return item, nil
		}
		if data.ID < 0 || data.ID >= len(nav.semantics.Table.Symbols) {
			return item, nil
		}
		sym := nav.semantics.Table.Symbols[data.ID]
		// Resolve data can outlive the document that produced it. If the
		// symbol at this index has been replaced, return the item unchanged.
		if sym == nil || sym.Canonical != analysis.CanonicalName(data.Name) {
			return item, nil
		}
		item.Detail = protocol.NewOptional(symbolCode(sym))
		doc := symbolDocumentation(sym)
		if source := declarationSource(nav, sym); source != "" {
			if doc != "" {
				doc += "\n\n"
			}
			doc += "*Declared in " + source + "*"
		}
		if doc != "" {
			item.Documentation = &protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: doc}
		}
	}
	return item, nil
}

func (s *Server) completions(nav *navContext, offset int, trigger string) ([]protocol.CompletionItem, bool) {
	if nav == nil || nav.file == nil {
		return nil, false
	}
	if token := nav.tokenAt(offset); token != nil && (token.Kind == syntax.TokenString || token.Kind == syntax.TokenInclude) {
		// Include paths are handled below; ordinary strings are not completion
		// contexts.
		if token.Kind == syntax.TokenString {
			return nil, false
		}
	}

	lineStart, linePrefix := nav.linePrefix(offset)
	if nav.inString(offset) {
		return nil, false
	}
	if nav.inComment(offset) || strings.HasPrefix(strings.TrimLeft(linePrefix, " \t"), "---@") {
		items := annotationCompletions(nav, offset, linePrefix)
		items = append(items, annotationSnippetItems(nav, offset, linePrefix)...)
		return items, false
	}
	if isIncludeKeywordContext(linePrefix) {
		return s.includeKeywordCompletions(nav, offset, linePrefix), false
	}
	if isIncludeLine(linePrefix) {
		return s.includeCompletions(nav, offset, linePrefix), false
	}

	prefixStart := identifierPrefixStart(nav.file.Text, offset)
	prefix := nav.file.Text[prefixStart:offset]
	if trigger == "@" || trigger == "#" {
		// Those characters trigger annotation/include completion above. At
		// every other position they are the peek and length operators, which
		// are not identifier contexts.
		return nil, false
	}
	if separator, method, ok := precedingMemberSeparator(nav.file.Text, prefixStart); ok {
		receiver := nav.receiverBefore(separator)
		return s.memberCompletions(nav, receiver, method, prefixStart, offset, prefix)
	}

	context := &completionContext{
		nav:         nav,
		offset:      offset,
		lineStart:   lineStart,
		linePrefix:  linePrefix,
		prefixStart: prefixStart,
		prefix:      prefix,
	}
	context.scope = nav.semantics.ScopeAt(nav.fileID, offset)
	return s.identifierCompletions(context)
}

func (s *Server) identifierCompletions(context *completionContext) ([]protocol.CompletionItem, bool) {
	nav := context.nav
	seen := make(map[string]struct{})
	items := make([]protocol.CompletionItem, 0)
	canonicalPrefix := analysis.CanonicalName(context.prefix)

	add := func(item protocol.CompletionItem, canonical string, group int) {
		if canonical == "" {
			return
		}
		if _, ok := seen[canonical]; ok {
			return
		}
		if canonicalPrefix != "" && !strings.HasPrefix(canonical, canonicalPrefix) {
			return
		}
		seen[canonical] = struct{}{}
		item.SortText = protocol.NewOptional(fmt.Sprintf("%d_%s", group, canonical))
		items = append(items, item)
	}

	if context.scope != nil {
		for _, sym := range nav.semantics.VisibleSymbolsAt(context.scope, nav.fileID, context.offset) {
			if sym == nil || sym.Owner != nil || sym.Kind == analysis.SymbolLabel {
				continue
			}
			add(s.makeSymbolItem(nav, sym, context.prefixStart, context.offset), sym.Canonical, 0)
		}
	}

	globals := make([]*analysis.Symbol, 0, len(nav.semantics.Table.Globals))
	for _, sym := range nav.semantics.Table.Globals {
		globals = append(globals, sym)
	}
	sort.SliceStable(globals, func(i, j int) bool {
		return globals[i].Canonical < globals[j].Canonical
	})
	for _, sym := range globals {
		add(s.makeSymbolItem(nav, sym, context.prefixStart, context.offset), sym.Canonical, 1)
	}

	if s.builtins != nil {
		for _, name := range s.builtins.Names {
			canonical := analysis.CanonicalName(name.Name)
			add(s.makeBuiltinItem(nav, name, context.prefixStart, context.offset), canonical, 2)
		}
	}

	for _, keyword := range completionKeywords {
		add(s.makeTextItem(nav, keyword, protocol.CompletionItemKindKeyword, "", keyword, false, context.prefixStart, context.offset), analysis.CanonicalName(keyword), 3)
	}
	for _, snippet := range completionSnippets {
		add(s.makeTextItem(nav, snippet.Label, protocol.CompletionItemKindSnippet, snippet.Detail, snippet.Insert, true, context.prefixStart, context.offset), analysis.CanonicalName(snippet.Label), 4)
	}

	if len(items) > maxCompletionItems {
		s.log.Warn("truncating completion items", "count", len(items), "max", maxCompletionItems)
		items = items[:maxCompletionItems]
	}
	return items, len(items) >= maxCompletionItems
}

func (s *Server) memberCompletions(nav *navContext, receiver *analysis.Symbol, method bool, prefixStart, offset int, prefix string) ([]protocol.CompletionItem, bool) {
	receiver = resolveAlias(receiver)
	if receiver == nil || len(receiver.Fields) == 0 {
		return nil, false
	}
	canonicalPrefix := analysis.CanonicalName(prefix)
	items := make([]protocol.CompletionItem, 0, len(receiver.Fields))
	for _, field := range receiver.Fields {
		if field == nil {
			continue
		}
		if method && field.Kind != analysis.SymbolMethod {
			continue
		}
		if canonicalPrefix != "" && !strings.HasPrefix(field.Canonical, canonicalPrefix) {
			continue
		}
		kind := protocol.CompletionItemKindField
		if field.Kind == analysis.SymbolMethod || field.Kind == analysis.SymbolFunction || field.Signature != "" {
			kind = protocol.CompletionItemKindMethod
		}
		items = append(items, s.makeSymbolItemWithKind(nav, field, prefixStart, offset, kind))
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Label < items[j].Label
	})
	if len(items) > maxCompletionItems {
		items = items[:maxCompletionItems]
	}
	return items, len(items) >= maxCompletionItems
}

func (s *Server) makeSymbolItem(nav *navContext, sym *analysis.Symbol, start, end int) protocol.CompletionItem {
	kind := protocol.CompletionItemKindVariable
	if sym.Signature != "" {
		kind = protocol.CompletionItemKindFunction
	}
	switch sym.Kind {
	case analysis.SymbolFunction:
		kind = protocol.CompletionItemKindFunction
	case analysis.SymbolMethod:
		kind = protocol.CompletionItemKindMethod
	case analysis.SymbolField:
		kind = protocol.CompletionItemKindField
	case analysis.SymbolParam:
		kind = protocol.CompletionItemKindVariable
	case analysis.SymbolGlobal:
		kind = protocol.CompletionItemKindVariable
	}
	return s.makeSymbolItemWithKind(nav, sym, start, end, kind)
}

func (s *Server) makeSymbolItemWithKind(nav *navContext, sym *analysis.Symbol, start, end int, kind protocol.CompletionItemKind) protocol.CompletionItem {
	data, _ := json.Marshal(completionData{Kind: "symbol", ID: sym.ID, Name: sym.Name, URI: nav.file.URI.String()})
	item := s.makeTextItem(nav, sym.Name, kind, symbolCode(sym), sym.Name, false, start, end)
	item.Data = protocol.LSPAny(data)
	item.FilterText = protocol.NewOptional(sym.Name)
	item.SortText = protocol.NewOptional(sym.Canonical)
	if sym.Doc != nil && sym.Doc.Deprecated {
		item.Tags = append(item.Tags, protocol.CompletionItemTagDeprecated)
	}
	return item
}

func (s *Server) makeBuiltinItem(nav *navContext, name builtin.Name, start, end int) protocol.CompletionItem {
	kind := protocol.CompletionItemKindFunction
	if name.Kind == builtin.KindConstant {
		kind = protocol.CompletionItemKindConstant
	}
	data, _ := json.Marshal(completionData{Kind: "builtin", Name: name.Name})
	item := s.makeTextItem(nav, name.Name, kind, name.Signature, name.Name, false, start, end)
	item.Data = protocol.LSPAny(data)
	item.FilterText = protocol.NewOptional(name.Name)
	if name.Docs != nil && name.Docs.Deprecated {
		item.Tags = append(item.Tags, protocol.CompletionItemTagDeprecated)
	}
	return item
}

func (s *Server) makeTextItem(nav *navContext, label string, kind protocol.CompletionItemKind, detail, insert string, snippet bool, start, end int) protocol.CompletionItem {
	format := protocol.InsertTextFormatPlainText
	if snippet {
		format = protocol.InsertTextFormatSnippet
	}
	item := protocol.CompletionItem{
		Label:            label,
		Kind:             kind,
		FilterText:       protocol.NewOptional(label),
		TextEdit:         &protocol.TextEdit{Range: nav.file.Index.Range(start, end), NewText: insert},
		InsertTextFormat: format,
	}
	if detail != "" {
		item.Detail = protocol.NewOptional(detail)
	}
	return item
}

var completionKeywords = []string{
	"and", "break", "do", "else", "elseif", "end", "false", "for",
	"function", "goto", "if", "in", "local", "nil", "not", "or",
	"repeat", "return", "then", "true", "until", "while",
}

type completionSnippet struct {
	Label  string
	Detail string
	Insert string
}

var completionSnippets = []completionSnippet{
	{Label: "if then", Detail: "if statement", Insert: "if ${1:condition} then\n\t$0\nend"},
	{Label: "if else", Detail: "if/else statement", Insert: "if ${1:condition} then\n\t${2:-- then}\nelse\n\t$0\nend"},
	{Label: "for numeric", Detail: "numeric for loop", Insert: "for ${1:i}=${2:1},${3:n} do\n\t$0\nend"},
	{Label: "for generic", Detail: "generic for loop", Insert: "for ${1:x} in all(${2:t}) do\n\t$0\nend"},
	{Label: "while loop", Detail: "while loop", Insert: "while ${1:condition} do\n\t$0\nend"},
	{Label: "function declaration", Detail: "function declaration", Insert: "function ${1:name}(${2})\n\t$0\nend"},
	{Label: "anonymous function", Detail: "anonymous function", Insert: "function(${1})\n\t$0\nend"},
	{Label: "_init", Detail: "init callback", Insert: "function _init()\n\t$0\nend"},
	{Label: "_update", Detail: "update callback", Insert: "function _update()\n\t$0\nend"},
	{Label: "_update60", Detail: "60fps update callback", Insert: "function _update60()\n\t$0\nend"},
	{Label: "_draw", Detail: "draw callback", Insert: "function _draw()\n\t$0\nend"},
	{Label: "class annotation", Detail: "LuaCATS class block", Insert: "---@class ${1:Name}\n---@field ${2:name} ${3:any}\nlocal ${1:Name} = {}"},
	{Label: "function docs", Detail: "LuaCATS function documentation", Insert: "---${1:Description}\n---@param ${2:name} ${3:any}\n---@return ${4:any}\nfunction ${5:name}(${2:name})\n\t$0\nend"},
	{Label: "#include", Detail: "include another Lua file", Insert: "#include \"${1:lib/file.lua}\""},
}
