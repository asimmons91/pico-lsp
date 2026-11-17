package lsp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/analysis/types"
)

// CodeAction implements textDocument/codeAction.
func (s *Server) CodeAction(ctx context.Context, params *protocol.CodeActionParams) ([]protocol.CommandOrCodeAction, error) {
	if params == nil {
		return []protocol.CommandOrCodeAction{}, nil
	}
	if isVirtualCartURI(params.TextDocument.URI) {
		return []protocol.CommandOrCodeAction{}, nil
	}
	nav, ok := s.navigationFor(ctx, params.TextDocument.URI)
	if !ok || nav.file == nil || nav.file.Index == nil {
		return []protocol.CommandOrCodeAction{}, nil
	}
	var actions []protocol.CommandOrCodeAction
	for _, diag := range params.Context.Diagnostics {
		code := diagnosticCode(diag)
		if code == "" || !analysis.IsSuppressibleDiagnosticCode(code) {
			continue
		}
		if action := suppressDiagnosticAction(nav, diag, code); action != nil {
			actions = append(actions, action)
		}
	}
	if action := generateAnnotationsAction(nav, params.Range); action != nil {
		actions = append(actions, action)
	}
	for _, diag := range params.Context.Diagnostics {
		if diagnosticCode(diag) != "include-missing" {
			continue
		}
		if action := s.createMissingIncludeAction(nav, diag); action != nil {
			actions = append(actions, action)
		}
	}
	return filterCodeActions(actions, params.Context.Only), nil
}

func filterCodeActions(actions []protocol.CommandOrCodeAction, only []protocol.CodeActionKind) []protocol.CommandOrCodeAction {
	if len(only) == 0 {
		return actions
	}
	filtered := actions[:0]
	for _, action := range actions {
		candidate, ok := action.(*protocol.CodeAction)
		if !ok || candidate.Kind == nil {
			continue
		}
		kind := string(*candidate.Kind)
		for _, requested := range only {
			wanted := string(requested)
			if wanted == "" || kind == wanted || strings.HasPrefix(kind, wanted+".") {
				filtered = append(filtered, action)
				break
			}
		}
	}
	return filtered
}

func diagnosticCode(diag protocol.Diagnostic) string {
	code, _ := diag.Code.(protocol.String)
	return string(code)
}

func suppressDiagnosticAction(nav *navContext, diag protocol.Diagnostic, code string) *protocol.CodeAction {
	line := diag.Range.Start.Line
	offset, err := nav.file.Index.Offset(protocol.Position{Line: line, Character: 0})
	if err != nil {
		return nil
	}
	edit := protocol.TextEdit{
		Range:   nav.file.Index.Range(offset, offset),
		NewText: fmt.Sprintf("---@diagnostic disable-next-line: %s\n", code),
	}
	kind := protocol.CodeActionKindQuickFix
	preferred := true
	return &protocol.CodeAction{
		Title:       fmt.Sprintf("Suppress %s on this line", code),
		Kind:        &kind,
		Diagnostics: []protocol.Diagnostic{diag},
		IsPreferred: &preferred,
		Edit: &protocol.WorkspaceEdit{
			Changes: map[uri.URI][]protocol.TextEdit{nav.file.URI: {edit}},
		},
	}
}

func generateAnnotationsAction(nav *navContext, r protocol.Range) *protocol.CodeAction {
	if nav == nil || nav.file == nil || nav.file.Index == nil {
		return nil
	}
	line := r.Start.Line
	sym := functionSymbolAtLine(nav, line)
	if sym == nil || sym.Type == nil {
		return nil
	}
	fn, ok := types.AsFunction(sym.Type)
	if !ok || fn == nil {
		return nil
	}
	if sym.Doc != nil && sym.Doc.Type != "" {
		return nil
	}
	existingParams := make(map[string]struct{})
	existingReturns := false
	if sym.Doc != nil {
		for _, param := range sym.Doc.Params {
			existingParams[analysis.CanonicalName(param.Name)] = struct{}{}
		}
		existingReturns = len(sym.Doc.Returns) > 0
	}
	var lines []string
	for _, param := range fn.Params {
		if param.Name == "" || param.Type == nil || types.IsAny(param.Type) {
			continue
		}
		if _, ok := existingParams[analysis.CanonicalName(param.Name)]; ok {
			continue
		}
		name := param.Name
		if param.Optional {
			name += "?"
		}
		lines = append(lines, fmt.Sprintf("---@param %s %s", name, types.Display(param.Type)))
	}
	if !existingReturns {
		for _, ret := range fn.Returns.Elems {
			if ret == nil || types.IsAny(ret) {
				continue
			}
			lines = append(lines, "---@return "+types.Display(ret))
		}
	}
	if len(lines) == 0 {
		return nil
	}
	offset, err := nav.file.Index.Offset(protocol.Position{Line: line, Character: 0})
	if err != nil {
		return nil
	}
	kind := protocol.CodeActionKindRefactorRewrite
	return &protocol.CodeAction{
		Title: "Generate type annotations from inferred types",
		Kind:  &kind,
		Edit: &protocol.WorkspaceEdit{
			Changes: map[uri.URI][]protocol.TextEdit{
				nav.file.URI: {{
					Range:   nav.file.Index.Range(offset, offset),
					NewText: strings.Join(lines, "\n") + "\n",
				}},
			},
		},
	}
}

func functionSymbolAtLine(nav *navContext, line uint32) *analysis.Symbol {
	if nav == nil || nav.semantics == nil || nav.semantics.Table == nil {
		return nil
	}
	for _, sym := range nav.semantics.Table.Symbols {
		if sym == nil || sym.DeclSpan.FileID != nav.fileID || sym.DeclSpan.IsEmpty() {
			continue
		}
		if sym.Kind != analysis.SymbolFunction && sym.Kind != analysis.SymbolMethod {
			continue
		}
		pos, err := nav.file.Index.Position(sym.DeclSpan.Start)
		if err == nil && pos.Line == line {
			return sym
		}
	}
	return nil
}

func (s *Server) createMissingIncludeAction(nav *navContext, diag protocol.Diagnostic) *protocol.CodeAction {
	if nav == nil || nav.snap == nil || nav.snap.Cart == nil {
		return nil
	}
	start, end, err := nav.file.Index.RangeOffsets(diag.Range)
	if err != nil {
		return nil
	}
	for _, include := range nav.snap.Cart.Includes {
		if !include.Missing || include.ResolvedURI == "" || include.DirectiveSpan.FileID != nav.fileID {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(include.Path), ".lua") {
			continue
		}
		span := include.DirectiveSpan
		if pathSpan, ok := includePathSpan(nav.file.Text, include.DirectiveSpan); ok {
			span = pathSpan
		}
		if span.End <= start || span.Start >= end {
			continue
		}
		rootDir := filepath.Dir(nav.snap.Cart.RootURI.FsPath())
		allowedRoots := append([]uri.URI(nil), s.WorkspaceFolders()...)
		allowedRoots = append(allowedRoots, uri.File(rootDir))
		target := include.ResolvedURI.FsPath()
		if !withinAllowedRoots(allowedRoots, target) {
			continue
		}
		kind := protocol.CodeActionKindQuickFix
		preferred := true
		return &protocol.CodeAction{
			Title:       fmt.Sprintf("Create missing include file %s", include.Path),
			Kind:        &kind,
			Diagnostics: []protocol.Diagnostic{diag},
			IsPreferred: &preferred,
			Edit: &protocol.WorkspaceEdit{
				DocumentChanges: []protocol.DocumentChange{
					&protocol.CreateFile{Kind: "create", URI: include.ResolvedURI},
				},
			},
		}
	}
	return nil
}
