package refactor

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

func TestCorpusRefactorsKeepIncludesAndSourceCoordinates(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "carts")
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".p8" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}
	if len(files) < 50 {
		t.Fatalf("corpus has %d carts, want at least 50", len(files))
	}

	loader := cart.NewLoader(nil, "", nil)
	for _, path := range files {
		abs, err := filepath.Abs(path)
		if err != nil {
			t.Fatalf("abs: %v", err)
		}
		snap, err := loader.Load(uri.File(abs))
		if err != nil {
			t.Errorf("%s: load: %v", path, err)
			continue
		}
		result := analysis.Analyze(snap, nil, analysis.Options{
			UndefinedGlobal: analysis.SeverityWarning,
			TypeChecking:    analysis.TypeCheckingBasic,
			Lint: analysis.LintOptions{
				UnusedLocal: analysis.SeverityHint,
			},
		})
		for _, file := range snap.Cart.Files {
			ctx := &Context{Snap: snap, Result: result, FileID: file.ID, File: file}
			actions := corpusActions(ctx)
			for _, action := range actions {
				assertActionEdits(t, snap, file, action)
			}
		}
	}
}

func corpusActions(ctx *Context) []*Action {
	var actions []*Action
	seen := make(map[string]bool)
	add := func(action *Action) {
		if action == nil {
			return
		}
		key := action.Kind + "\x00" + action.Title
		if seen[key] {
			return
		}
		seen[key] = true
		actions = append(actions, action)
	}
	for _, sym := range ctx.Result.Table.Symbols {
		if sym == nil || sym.Builtin || sym.DeclSpan.FileID != ctx.FileID || sym.DeclSpan.IsEmpty() {
			continue
		}
		add(GlobalToLocal(ctx, sym.DeclSpan.Start))
		add(RemoveUnused(ctx, sym.DeclSpan.Start))
	}
	exprs := 0
	for _, node := range ctx.nodes() {
		switch node := node.(type) {
		case *syntax.TableExpr:
			add(GenerateClass(ctx, node.Open.Start))
		case *syntax.IfStmt:
			add(ConvertShorthand(ctx, node.Span().Start))
		case *syntax.WhileStmt:
			add(ConvertShorthand(ctx, node.Span().Start))
		case syntax.Expr:
			if exprs >= 25 {
				continue
			}
			span := node.Span()
			if span.Len() == 0 || span.Len() > 80 {
				continue
			}
			text := ctx.currentText()
			if strings.Contains(text[span.Start:span.End], "\n") {
				continue
			}
			exprs++
			add(ExtractFunction(ctx, span.Start, span.End))
		}
	}
	return actions
}

func assertActionEdits(t *testing.T, snap *cart.Snapshot, file *cart.File, action *Action) {
	t.Helper()
	texts := make(map[int]string)
	for _, member := range snap.Cart.Files {
		texts[int(member.ID)] = member.Text
	}
	before := make(map[int]int)
	for _, include := range snap.Cart.Includes {
		if include.DirectiveSpan.IsEmpty() {
			continue
		}
		directive := snap.Cart.File(include.DirectiveSpan.FileID)
		if directive == nil {
			continue
		}
		line := directive.Text[include.DirectiveSpan.Start:include.DirectiveSpan.End]
		before[int(include.DirectiveSpan.FileID)] += strings.Count(directive.Text, line)
	}
	byFile := make(map[int][]Edit)
	for _, edit := range action.Edits {
		member := snap.Cart.File(edit.FileID)
		if member == nil {
			t.Fatalf("%s: edit targets missing file %d", action.Title, edit.FileID)
		}
		if edit.Start < 0 || edit.End > len(member.Text) || edit.Start > edit.End {
			t.Fatalf("%s: invalid edit %+v in %s", action.Title, edit, member.URI)
		}
		byFile[int(edit.FileID)] = append(byFile[int(edit.FileID)], edit)
	}
	for id, edits := range byFile {
		sort.SliceStable(edits, func(i, j int) bool {
			if edits[i].Start != edits[j].Start {
				return edits[i].Start > edits[j].Start
			}
			return edits[i].End > edits[j].End
		})
		out := texts[id]
		for _, edit := range edits {
			out = out[:edit.Start] + edit.NewText + out[edit.End:]
		}
		texts[id] = out
	}
	for _, include := range snap.Cart.Includes {
		if include.DirectiveSpan.IsEmpty() {
			continue
		}
		directive := snap.Cart.File(include.DirectiveSpan.FileID)
		if directive == nil {
			continue
		}
		line := directive.Text[include.DirectiveSpan.Start:include.DirectiveSpan.End]
		if got := strings.Count(texts[int(include.DirectiveSpan.FileID)], line); got != before[int(include.DirectiveSpan.FileID)] {
			t.Fatalf("%s: include directive %q count changed from %d to %d", action.Title, line, before[int(include.DirectiveSpan.FileID)], got)
		}
	}
}
