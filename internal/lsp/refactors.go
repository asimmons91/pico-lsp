package lsp

import (
	"context"
	"fmt"
	"sort"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/refactor"
)

// CommandRemoveUnused applies a confirmed remove-unused refactor through
// workspace/applyEdit. The code action itself carries no edit so editors can
// ask for confirmation before invoking the command.
const CommandRemoveUnused = "pico8.removeUnused"

type removeUnusedArgument struct {
	URI   string `json:"uri"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

func (s *Server) refactorActions(nav *navContext, diagnostics []protocol.Diagnostic, start, end int) []protocol.CommandOrCodeAction {
	if nav == nil || nav.file == nil {
		return nil
	}
	rctx := navContextForRefactor(nav)
	var actions []protocol.CommandOrCodeAction
	seen := make(map[string]bool)
	addAction := func(action *protocol.CodeAction) {
		if action == nil {
			return
		}
		key := action.Title
		if action.Kind != nil {
			key = string(*action.Kind) + "\x00" + key
		}
		if seen[key] {
			return
		}
		seen[key] = true
		actions = append(actions, action)
	}
	add := func(action *refactor.Action) {
		if action == nil {
			return
		}
		addAction(s.codeActionForRefactor(nav, action, nil))
	}

	for i := range diagnostics {
		diag := diagnostics[i]
		switch diagnosticCode(diag) {
		case analysis.CodeMissingFields:
			addAction(s.missingFieldsAction(nav, rctx, diag))
		case analysis.CodeUnusedLocal:
			diagStart, _, err := nav.file.Index.RangeOffsets(diag.Range)
			if err != nil {
				continue
			}
			if action := refactor.RemoveUnused(rctx, diagStart); action != nil {
				addAction(s.codeActionForRefactor(nav, action, []protocol.Diagnostic{diag}))
			}
		}
	}
	if action := refactor.ExtractFunction(rctx, start, end); action != nil {
		add(action)
	}
	if action := refactorAt(rctx, start, end, refactor.GenerateClass); action != nil {
		add(action)
	}
	if action := refactorAtOccurrence(rctx, start, end, refactor.GlobalToLocal); action != nil {
		add(action)
	}
	if action := refactorAt(rctx, start, end, refactor.ConvertShorthand); action != nil {
		add(action)
	}
	if action := refactorAtOccurrence(rctx, start, end, refactor.RemoveUnused); action != nil {
		add(action)
	}
	return actions
}

func trimRefactorRange(text string, start, end int) (int, int) {
	if start < 0 {
		start = 0
	}
	if end > len(text) {
		end = len(text)
	}
	for start < end && isRefactorSpace(text[start]) {
		start++
	}
	for end > start && isRefactorSpace(text[end-1]) {
		end--
	}
	return start, end
}

func isRefactorSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

// refactorAtOccurrence finds the first name occurrence inside a selection. It
// handles selections that begin at line indentation or cover a whole line.
func refactorAtOccurrence(ctx *refactor.Context, start, end int, fn func(*refactor.Context, int) *refactor.Action) *refactor.Action {
	if action := fn(ctx, start); action != nil {
		return action
	}
	for _, occ := range ctx.Result.Occurrences(ctx.FileID) {
		if occ.Span.Start < start || occ.Span.End > end {
			continue
		}
		if action := fn(ctx, occ.Span.Start); action != nil {
			return action
		}
	}
	if end > start {
		return fn(ctx, end-1)
	}
	return nil
}

// refactorAt tries the selection start and, for whole-line selections over a
// table literal, the last selected byte. Only class generation uses this: a
// table value can begin in the middle of a declaration line.
func refactorAt(ctx *refactor.Context, start, end int, fn func(*refactor.Context, int) *refactor.Action) *refactor.Action {
	if action := fn(ctx, start); action != nil {
		return action
	}
	if end > start {
		return fn(ctx, end-1)
	}
	return nil
}

func (s *Server) missingFieldsAction(nav *navContext, rctx *refactor.Context, diag protocol.Diagnostic) *protocol.CodeAction {
	if nav == nil || nav.semantics == nil || nav.file == nil || nav.file.Index == nil {
		return nil
	}
	start, end, err := nav.file.Index.RangeOffsets(diag.Range)
	if err != nil {
		return nil
	}
	for _, fix := range nav.semantics.MissingFieldFixes {
		if fix.Span.FileID != nav.fileID {
			continue
		}
		if fix.Span.Start > end || fix.Span.End < start {
			continue
		}
		action := refactor.AddMissingFields(rctx, fix.Span, fix.Fields)
		if action != nil {
			return s.codeActionForRefactor(nav, action, []protocol.Diagnostic{diag})
		}
	}
	return nil
}

func navContextForRefactor(nav *navContext) *refactor.Context {
	if nav == nil {
		return nil
	}
	return &refactor.Context{
		Snap:   nav.snap,
		Result: nav.semantics,
		FileID: nav.fileID,
		File:   nav.file,
	}
}

func (s *Server) codeActionForRefactor(nav *navContext, action *refactor.Action, diagnostics []protocol.Diagnostic) *protocol.CodeAction {
	if nav == nil || action == nil {
		return nil
	}
	kind := protocol.CodeActionKind(action.Kind)
	result := &protocol.CodeAction{
		Title:       action.Title,
		Kind:        &kind,
		Diagnostics: diagnostics,
	}
	if action.Kind == refactor.KindAddMissingFields {
		preferred := true
		result.IsPreferred = &preferred
	}
	if action.Confirmation != nil {
		argument := removeUnusedArgument{
			URI:   nav.file.URI.String(),
			Start: action.Confirmation.Span.Start,
			End:   action.Confirmation.Span.End,
		}
		raw, err := protocol.Marshal(argument)
		if err != nil {
			return nil
		}
		result.Command = protocol.Command{
			Title:     action.Title,
			Command:   CommandRemoveUnused,
			Arguments: []protocol.LSPAny{protocol.LSPAny(raw)},
		}
		return result
	}
	edit, ok := s.workspaceEdit(nav, action.Edits)
	if !ok {
		return nil
	}
	result.Edit = edit
	return result
}

func (s *Server) workspaceEdit(nav *navContext, edits []refactor.Edit) (*protocol.WorkspaceEdit, bool) {
	if nav == nil || nav.snap == nil || nav.snap.Cart == nil || len(edits) == 0 {
		return nil, false
	}
	changes := make(map[uri.URI][]protocol.TextEdit)
	for _, edit := range edits {
		file := nav.snap.Cart.File(edit.FileID)
		if file == nil || file.Index == nil {
			return nil, false
		}
		changes[file.URI] = append(changes[file.URI], protocol.TextEdit{
			Range:   file.Index.Range(edit.Start, edit.End),
			NewText: edit.NewText,
		})
	}
	for _, list := range changes {
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].Range.Start.Line != list[j].Range.Start.Line {
				return list[i].Range.Start.Line < list[j].Range.Start.Line
			}
			return list[i].Range.Start.Character < list[j].Range.Start.Character
		})
	}
	return &protocol.WorkspaceEdit{Changes: changes}, true
}

func (s *Server) applyRemoveUnused(ctx context.Context, params *protocol.ExecuteCommandParams) (any, error) {
	if params == nil || len(params.Arguments) == 0 {
		return nil, fmt.Errorf("%s: missing argument", CommandRemoveUnused)
	}
	var argument removeUnusedArgument
	if err := protocol.Unmarshal(params.Arguments[0], &argument); err != nil || argument.URI == "" {
		return nil, fmt.Errorf("%s: invalid argument", CommandRemoveUnused)
	}
	u := uri.URI(argument.URI)
	nav, ok := s.navigationFor(ctx, u)
	if !ok || nav.file == nil || nav.file.Index == nil {
		return nil, fmt.Errorf("%s: no analysis for %s", CommandRemoveUnused, argument.URI)
	}
	if !s.snapshotFresh(nav) {
		return nil, fmt.Errorf("%s: analysis is stale", CommandRemoveUnused)
	}
	action := refactor.RemoveUnused(navContextForRefactor(nav), argument.Start)
	if action == nil || action.Confirmation == nil ||
		action.Confirmation.Span.Start != argument.Start || action.Confirmation.Span.End != argument.End {
		return nil, fmt.Errorf("%s: symbol is no longer an unused local", CommandRemoveUnused)
	}
	edit, ok := s.workspaceEdit(nav, action.Edits)
	if !ok {
		return nil, fmt.Errorf("%s: could not build workspace edit", CommandRemoveUnused)
	}
	client := s.Client()
	if client == nil {
		return nil, fmt.Errorf("%s: no client connection", CommandRemoveUnused)
	}
	label := action.Title
	result, err := client.ApplyEdit(ctx, &protocol.ApplyWorkspaceEditParams{
		Label: &label,
		Edit:  *edit,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", CommandRemoveUnused, err)
	}
	if result != nil && !result.Applied {
		reason := "client refused the edit"
		if result.FailureReason != nil && *result.FailureReason != "" {
			reason = *result.FailureReason
		}
		return nil, fmt.Errorf("%s: %s", CommandRemoveUnused, reason)
	}
	return map[string]any{"applied": true}, nil
}

// snapshotFresh reports whether every open file in the analyzed snapshot still
// has the analyzed text. Code actions are never offered from stale analysis.
func (s *Server) snapshotFresh(nav *navContext) bool {
	if nav == nil || nav.snap == nil || nav.snap.Cart == nil {
		return false
	}
	for _, file := range nav.snap.Cart.Files {
		if text, ok := s.docs.Text(file.URI); ok && text != file.Text {
			return false
		}
	}
	return true
}
