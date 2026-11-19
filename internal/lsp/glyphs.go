package lsp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/glyphs"
)

const (
	// RequestGlyphs is the custom request that returns the P8SCII catalog.
	RequestGlyphs = "pico8/glyphs"
	// CommandInsertGlyph is the command that inserts a P8SCII glyph.
	CommandInsertGlyph = "pico8.insertGlyph"
)

type glyphsParams struct {
	TextDocument *protocol.TextDocumentIdentifier `json:"textDocument,omitempty"`
	Position     *protocol.Position               `json:"position,omitempty"`
}

// GlyphsResult is the pico8/glyphs response.
type GlyphsResult struct {
	Glyphs []glyphs.Entry `json:"glyphs"`
}

type insertGlyphArgument struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	Position     protocol.Position               `json:"position"`
	Name         string                          `json:"name,omitempty"`
	Byte         *int                            `json:"byte,omitempty"`
}

type insertGlyphResult struct {
	Applied bool                    `json:"applied"`
	Edit    *protocol.WorkspaceEdit `json:"edit,omitempty"`
}

func (s *Server) glyphCompletions(nav *navContext, offset, start int, prefix string) []protocol.CompletionItem {
	entries, _ := glyphs.Build(s.cfg.Settings().Glyphs)
	triggers := make(map[string]glyphs.Entry)
	order := make([]string, 0)
	for _, entry := range entries {
		for _, trigger := range entry.AllTriggers() {
			key := analysis.CanonicalName(trigger)
			if _, ok := triggers[key]; !ok {
				order = append(order, key)
			}
			// Later entries win trigger collisions so custom entries can
			// override built-in aliases.
			triggers[key] = entry
		}
	}
	sort.Strings(order)

	canonicalPrefix := analysis.CanonicalName(prefix)
	inString := nav.inString(offset)
	items := make([]protocol.CompletionItem, 0)
	for _, trigger := range order {
		if !strings.HasPrefix(trigger, canonicalPrefix) {
			continue
		}
		entry := triggers[trigger]
		if entry.Context == glyphs.ContextString && !inString {
			continue
		}
		items = append(items, glyphCompletionItem(nav, entry, trigger, start, offset))
	}
	if len(items) > maxCompletionItems {
		items = items[:maxCompletionItems]
	}
	return items
}

func glyphCompletionItem(nav *navContext, entry glyphs.Entry, trigger string, start, end int) protocol.CompletionItem {
	detail := fmt.Sprintf("P8SCII 0x%02x · %s", entry.Byte, entry.Description)
	if entry.Category == "control" {
		detail = fmt.Sprintf("P8SCII control 0x%02x · %s", entry.Byte, entry.Description)
	}
	filter := strings.Join([]string{
		trigger,
		entry.Name,
		entry.Text,
		fmt.Sprintf("0x%02x", entry.Byte),
	}, " ")
	return protocol.CompletionItem{
		Label:            entry.Text + "  " + entry.Trigger(),
		Kind:             protocol.CompletionItemKindConstant,
		Detail:           protocol.NewOptional(detail),
		Documentation:    &protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: glyphDocumentation(entry)},
		FilterText:       protocol.NewOptional(filter),
		SortText:         protocol.NewOptional("5_" + analysis.CanonicalName(entry.Name)),
		TextEdit:         &protocol.TextEdit{Range: nav.file.Index.Range(start, end), NewText: entry.Text},
		InsertTextFormat: protocol.InsertTextFormatPlainText,
	}
}

func glyphDocumentation(entry glyphs.Entry) string {
	var builder strings.Builder
	builder.WriteString("```text\n")
	builder.WriteString(entry.Text)
	builder.WriteString("\n```\n\n")
	fmt.Fprintf(&builder, "P8SCII byte `0x%02x` · %s", entry.Byte, entry.Category)
	if entry.Description != "" {
		builder.WriteString(" · ")
		builder.WriteString(entry.Description)
	}
	builder.WriteString("\n\nUnicode: `")
	builder.WriteString(glyphCodepoints(entry.Text))
	builder.WriteString("`")
	if entry.Context == glyphs.ContextString {
		builder.WriteString("\n\nString-only control code.")
	}
	if entry.Custom {
		builder.WriteString("\n\nCustom entries insert the canonical Unicode spelling for the selected P8SCII byte. A runtime custom font may draw a different shape.")
	}
	return builder.String()
}

func glyphCodepoints(text string) string {
	parts := make([]string, 0, len(text))
	for _, r := range text {
		parts = append(parts, fmt.Sprintf("U+%04X", r))
	}
	return strings.Join(parts, " ")
}

func (s *Server) glyphsRequest(ctx context.Context, params protocol.LSPAny) (any, error) {
	var request glyphsParams
	if len(params) > 0 && strings.TrimSpace(string(params)) != "null" {
		if err := protocol.Unmarshal(params, &request); err != nil {
			return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "%s: invalid params", RequestGlyphs)
		}
	}
	if (request.TextDocument == nil) != (request.Position == nil) {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "%s: textDocument and position must be supplied together", RequestGlyphs)
	}

	entries, _ := glyphs.Build(s.cfg.Settings().Glyphs)
	if request.TextDocument != nil {
		nav, ok := s.navigationFor(ctx, request.TextDocument.URI)
		if !ok {
			return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "%s: no analysis for %s", RequestGlyphs, request.TextDocument.URI)
		}
		offset, ok := nav.offset(*request.Position)
		if !ok {
			return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "%s: invalid position", RequestGlyphs)
		}
		if !nav.inCode(offset) || nav.onIncludeLine(offset) {
			return GlyphsResult{Glyphs: []glyphs.Entry{}}, nil
		}
		inString := nav.inString(offset)
		filtered := make([]glyphs.Entry, 0, len(entries))
		for _, entry := range entries {
			if entry.Context == glyphs.ContextString && !inString {
				continue
			}
			filtered = append(filtered, entry)
		}
		entries = filtered
	}
	return GlyphsResult{Glyphs: entries}, nil
}

func (s *Server) insertGlyph(ctx context.Context, params *protocol.ExecuteCommandParams) (any, error) {
	if params == nil || len(params.Arguments) == 0 {
		return nil, fmt.Errorf("%s: missing argument", CommandInsertGlyph)
	}
	var argument insertGlyphArgument
	if err := protocol.Unmarshal(params.Arguments[0], &argument); err != nil || argument.TextDocument.URI == "" {
		return nil, fmt.Errorf("%s: invalid argument", CommandInsertGlyph)
	}
	if (argument.Name == "") == (argument.Byte == nil) {
		return nil, fmt.Errorf("%s: supply exactly one of name or byte", CommandInsertGlyph)
	}
	if argument.Byte != nil && (*argument.Byte < 0 || *argument.Byte > 255) {
		return nil, fmt.Errorf("%s: byte must be between 0 and 255", CommandInsertGlyph)
	}

	u := argument.TextDocument.URI
	if isVirtualCartURI(u) || isPNGURI(u) {
		return nil, fmt.Errorf("%s: %s is read-only", CommandInsertGlyph, u)
	}
	if _, ok := s.docs.Get(u); !ok {
		return nil, fmt.Errorf("%s: %s is not open", CommandInsertGlyph, u)
	}
	nav, ok := s.navigationFor(ctx, u)
	if !ok || nav.file == nil || nav.file.Index == nil {
		return nil, fmt.Errorf("%s: no analysis for %s", CommandInsertGlyph, u)
	}
	if !s.snapshotFresh(nav) {
		return nil, fmt.Errorf("%s: analysis is stale", CommandInsertGlyph)
	}
	offset, ok := nav.offset(argument.Position)
	if !ok {
		return nil, fmt.Errorf("%s: invalid position", CommandInsertGlyph)
	}
	if !nav.inCode(offset) {
		return nil, fmt.Errorf("%s: cannot insert a glyph outside Lua code or a string", CommandInsertGlyph)
	}
	if nav.onIncludeLine(offset) {
		return nil, fmt.Errorf("%s: cannot insert a glyph in an include path", CommandInsertGlyph)
	}

	entries, _ := glyphs.Build(s.cfg.Settings().Glyphs)
	var entry glyphs.Entry
	if argument.Name != "" {
		entry, ok = glyphs.FindByName(entries, argument.Name)
	} else {
		entry, ok = glyphs.FindByByte(entries, byte(*argument.Byte))
	}
	if !ok {
		return nil, fmt.Errorf("%s: unknown glyph", CommandInsertGlyph)
	}
	if entry.Context == glyphs.ContextString && !nav.inString(offset) {
		return nil, fmt.Errorf("%s: %s is only valid inside a string", CommandInsertGlyph, entry.Name)
	}

	start := offset
	if triggerStart, _, ok := glyphPrefix(nav.file.Text, offset); ok {
		start = triggerStart
	}
	edit := protocol.WorkspaceEdit{
		Changes: map[uri.URI][]protocol.TextEdit{
			u: {{
				Range:   nav.file.Index.Range(start, offset),
				NewText: entry.Text,
			}},
		},
	}

	client := s.Client()
	if s.clientSupportsApplyEdit() && client != nil {
		jsonrpc2.Async(ctx)
		label := "Insert P8SCII glyph"
		result, err := client.ApplyEdit(ctx, &protocol.ApplyWorkspaceEditParams{
			Label: &label,
			Edit:  edit,
		})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", CommandInsertGlyph, err)
		}
		if result != nil && !result.Applied {
			reason := "client refused the edit"
			if result.FailureReason != nil && *result.FailureReason != "" {
				reason = *result.FailureReason
			}
			return nil, fmt.Errorf("%s: %s", CommandInsertGlyph, reason)
		}
		return insertGlyphResult{Applied: true}, nil
	}
	return insertGlyphResult{Applied: false, Edit: &edit}, nil
}

func (s *Server) clientSupportsApplyEdit() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applyEdit
}
