package lsp

import (
	"context"
	"sort"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// PrepareRename implements textDocument/prepareRename. It returns the range
// and current spelling only when the symbol can be renamed safely.
func (s *Server) PrepareRename(ctx context.Context, params *protocol.PrepareRenameParams) (protocol.PrepareRenameResult, error) {
	if params == nil {
		return nil, nil
	}
	nav, sym, occ, ok := s.renameTarget(ctx, params.TextDocument.URI, params.Position)
	if !ok || !nav.semantics.Renameable(sym) {
		return nil, nil
	}
	return &protocol.PrepareRenamePlaceholder{
		Range:       nav.file.Index.Range(occ.Span.Start, occ.Span.End),
		Placeholder: occ.Text,
	}, nil
}

// Rename implements textDocument/rename. Unsafe cases return an error rather
// than a partial edit.
func (s *Server) Rename(ctx context.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	if params == nil {
		return nil, jsonrpc2.NewError(jsonrpc2.InvalidParams, "rename: missing params")
	}
	nav, sym, _, ok := s.renameTarget(ctx, params.TextDocument.URI, params.Position)
	if !ok || !nav.semantics.Renameable(sym) {
		return nil, jsonrpc2.NewError(jsonrpc2.InvalidRequest, "rename: symbol cannot be renamed safely")
	}
	if !validRenameName(sym, params.NewName) {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "rename: %q is not a valid PICO-8 identifier", params.NewName)
	}

	newCanonical := analysis.CanonicalName(params.NewName)
	if conflict := nav.semantics.RenameConflict(sym, newCanonical); conflict != nil {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidRequest, "rename: %q would collide with %q", params.NewName, conflict.Name)
	}

	occurrences := nav.semantics.OccurrencesForSymbol(sym)
	newText := typedRenameText(params.NewName, occurrences)
	changes := make(map[uri.URI][]protocol.TextEdit)
	seen := make(map[source.Span]struct{})

	for _, occ := range occurrences {
		if _, ok := seen[occ.Span]; ok {
			continue
		}
		seen[occ.Span] = struct{}{}
		file := nav.snap.Cart.File(occ.Span.FileID)
		if file == nil || file.Index == nil {
			continue
		}
		if occ.Text == newText {
			continue
		}
		changes[file.URI] = append(changes[file.URI], protocol.TextEdit{
			Range:   file.Index.Range(occ.Span.Start, occ.Span.End),
			NewText: newText,
		})
	}

	for u := range changes {
		edits := changes[u]
		sort.SliceStable(edits, func(i, j int) bool {
			if edits[i].Range.Start.Line != edits[j].Range.Start.Line {
				return edits[i].Range.Start.Line < edits[j].Range.Start.Line
			}
			return edits[i].Range.Start.Character < edits[j].Range.Start.Character
		})
		changes[u] = edits
	}
	return &protocol.WorkspaceEdit{Changes: changes}, nil
}

func (s *Server) renameTarget(ctx context.Context, u uri.URI, pos protocol.Position) (*navContext, *analysis.Symbol, *analysis.Occurrence, bool) {
	nav, ok := s.navigationFor(ctx, u)
	if !ok {
		return nil, nil, nil, false
	}
	offset, ok := nav.offset(pos)
	if !ok {
		return nil, nil, nil, false
	}
	if nav.includeAt(offset) != nil {
		return nil, nil, nil, false
	}
	occ := nav.occurrenceAt(offset)
	if occ == nil || occ.Symbol == nil {
		return nil, nil, nil, false
	}
	return nav, occ.Symbol, occ, true
}

// validRenameName accepts PICO-8 identifiers for code symbols and qualified
// names such as `Player.State` for annotation type declarations.
func validRenameName(sym *analysis.Symbol, name string) bool {
	if !analysis.IsTypeSymbol(sym) {
		return syntax.IsIdentifier(name)
	}
	if name == "" {
		return false
	}
	for _, part := range strings.Split(name, ".") {
		if !syntax.IsIdentifier(part) {
			return false
		}
	}
	return true
}

// typedRenameText preserves a consistently uppercase spelling. Mixed or
// lowercase spellings use the rename request's spelling as typed.
func typedRenameText(newName string, occurrences []analysis.Occurrence) string {
	sawUpper, sawLower := false, false
	for _, occ := range occurrences {
		for i := 0; i < len(occ.Text); i++ {
			c := occ.Text[i]
			if c >= 'A' && c <= 'Z' {
				sawUpper = true
			}
			if c >= 'a' && c <= 'z' {
				sawLower = true
			}
		}
	}
	if sawUpper && !sawLower {
		return asciiUpper(newName)
	}
	return newName
}

func asciiUpper(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		b.WriteByte(c)
	}
	return b.String()
}
