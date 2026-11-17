package analysis

import (
	"fmt"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

var lifecycleCallbacks = []string{"_init", "_update", "_update60", "_draw"}

var luaStandardLibraryNames = map[string]struct{}{
	"require": {}, "dofile": {}, "loadfile": {}, "load": {}, "loadstring": {},
	"string": {}, "table": {}, "math": {}, "io": {}, "os": {}, "package": {},
	"coroutine": {}, "debug": {}, "utf8": {},
}

func runLint(snap *cart.Snapshot, result *Result, opts LintOptions) {
	if snap == nil || result == nil || result.Table == nil {
		return
	}
	lintCallbacks(result, opts)
	lintNoStdlib(result, opts)
	if opts.DuplicateTableField != SeverityOff {
		lintDuplicateTableFields(snap, result, opts.DuplicateTableField)
	}
	if opts.UnreachableCode != SeverityOff {
		lintUnreachableCode(snap, result, opts.UnreachableCode)
	}
	if opts.UnusedLocal != SeverityOff {
		lintUnusedLocals(result, opts.UnusedLocal)
	}
	if opts.ShadowedLocal != SeverityOff {
		lintShadowedLocals(result, opts.ShadowedLocal)
	}
	if opts.EmptyBlock != SeverityOff {
		lintEmptyBlocks(snap, result, opts.EmptyBlock)
	}
	if opts.GlobalInFunction != SeverityOff {
		lintGlobalInFunction(result, opts.GlobalInFunction)
	}
}

func lintCallbacks(result *Result, opts LintOptions) {
	if opts.CallbackPrecedence != SeverityOff {
		update := result.Table.Globals["_update"]
		update60 := result.Table.Globals["_update60"]
		if update != nil && update60 != nil {
			result.addLintDiagnostic(opts.CallbackPrecedence, CodeCallbackPrecedence,
				"`_update60` is defined, so `_update` will not be called.", update.DeclSpan)
		}
	}
	if opts.CallbackParams != SeverityOff {
		for _, name := range lifecycleCallbacks {
			sym := result.Table.Globals[CanonicalName(name)]
			if sym == nil || len(sym.Params) == 0 {
				continue
			}
			result.addLintDiagnostic(opts.CallbackParams, CodeCallbackParams,
				fmt.Sprintf("PICO-8 calls `%s` with no arguments.", name), sym.DeclSpan)
		}
	}
}

func lintNoStdlib(result *Result, opts LintOptions) {
	if opts.NoStdlib == SeverityOff {
		return
	}
	for _, occ := range result.occs {
		if occ.Symbol != nil || occ.Span.IsEmpty() {
			continue
		}
		canonical := CanonicalName(occ.Text)
		if _, ok := luaStandardLibraryNames[canonical]; !ok {
			continue
		}
		if result.Table.Globals[canonical] != nil {
			// A user definition wins over the lint.
			continue
		}
		result.addLintDiagnostic(opts.NoStdlib, CodeNoStdlib,
			"PICO-8 does not include the Lua standard library; use the PICO-8 API.", occ.Span)
	}
}

func lintDuplicateTableFields(snap *cart.Snapshot, result *Result, severity Severity) {
	syntax.Walk(snap.Chunk, func(node syntax.Node) bool {
		table, ok := node.(*syntax.TableExpr)
		if !ok || table == nil {
			return true
		}
		seen := make(map[string]struct{})
		for _, field := range table.Fields {
			key, fold, ok := tableFieldKey(field)
			if !ok {
				continue
			}
			displayKey := key
			if fold {
				// PICO-8 folds identifiers, but string keys are case-sensitive.
				key = CanonicalName(key)
			}
			if _, exists := seen[key]; exists {
				result.addLintDiagnostic(severity, CodeDuplicateTableField,
					fmt.Sprintf("duplicate table field %q", displayKey), tableFieldKeySpan(field))
				continue
			}
			seen[key] = struct{}{}
		}
		return true
	})
}

func tableFieldKey(field *syntax.TableField) (key string, fold, ok bool) {
	if field == nil || field.Key == nil {
		return "", false, false
	}
	if name, isName := field.Key.(*syntax.NameExpr); isName && !field.BracketKey {
		return name.Text, true, true
	}
	if str, isString := field.Key.(*syntax.StringExpr); isString && field.BracketKey {
		return str.Value, false, true
	}
	return "", false, false
}

func tableFieldKeySpan(field *syntax.TableField) source.Span {
	if field == nil || field.Key == nil {
		if field == nil {
			return source.Span{}
		}
		return field.Span()
	}
	return field.Key.Span()
}

func lintUnreachableCode(snap *cart.Snapshot, result *Result, severity Severity) {
	checked := make(map[*syntax.Block]bool)
	syntax.Walk(snap.Chunk, func(node syntax.Node) bool {
		block, ok := node.(*syntax.Block)
		if !ok || block == nil || checked[block] {
			return true
		}
		checked[block] = true
		unreachable := false
		for _, stmt := range block.Stmts {
			if unreachable {
				switch stmt.(type) {
				case *syntax.EmptyStmt:
					continue
				case *syntax.LabelStmt:
					// A label is a valid goto target.
					unreachable = false
					continue
				}
				result.addLintDiagnostic(severity, CodeUnreachableCode,
					"statement is unreachable", stmt.Span())
				unreachable = false
			}
			switch stmt.(type) {
			case *syntax.ReturnStmt, *syntax.BreakStmt:
				unreachable = true
			}
		}
		return true
	})
}

func lintUnusedLocals(result *Result, severity Severity) {
	for _, sym := range result.Table.Symbols {
		if sym == nil || sym.Builtin || sym.DeclSpan.IsEmpty() || sym.Owner != nil {
			continue
		}
		if result.Table.Globals[sym.Canonical] == sym {
			// Global functions share SymbolFunction; they are not locals.
			continue
		}
		if sym.Kind != SymbolLocal && sym.Kind != SymbolFunction {
			continue
		}
		if strings.HasPrefix(sym.Name, "_") {
			continue
		}
		read := false
		for _, occ := range result.OccurrencesForSymbol(sym) {
			if !occ.Decl && !occ.Write {
				read = true
				break
			}
		}
		if !read {
			result.addLintDiagnostic(severity, CodeUnusedLocal,
				fmt.Sprintf("local %q is never read", sym.Name), sym.DeclSpan)
		}
	}
}

func lintShadowedLocals(result *Result, severity Severity) {
	for _, sym := range result.Table.Symbols {
		if sym == nil || sym.Builtin || sym.Scope == nil || sym.DeclSpan.IsEmpty() || sym.Owner != nil {
			continue
		}
		if result.Table.Globals[sym.Canonical] == sym {
			continue
		}
		if sym.Kind != SymbolLocal && sym.Kind != SymbolFunction && sym.Kind != SymbolParam {
			continue
		}
		shadowed := false
		for parent := sym.Scope.Parent; parent != nil && !shadowed; parent = parent.Parent {
			for _, other := range parent.symbols[sym.Canonical] {
				if other == nil || other == sym || other.Builtin {
					continue
				}
				result.addLintDiagnostic(severity, CodeShadowedLocal,
					fmt.Sprintf("%q shadows an outer declaration", sym.Name), sym.DeclSpan)
				shadowed = true
				break
			}
		}
	}
}

func lintEmptyBlocks(snap *cart.Snapshot, result *Result, severity Severity) {
	syntax.Walk(snap.Chunk, func(node syntax.Node) bool {
		block, ok := node.(*syntax.Block)
		if !ok || block == nil || len(block.Stmts) != 0 {
			return true
		}
		span := block.Span()
		if span.IsEmpty() {
			return true
		}
		result.addLintDiagnostic(severity, CodeEmptyBlock, "empty block", span)
		return true
	})
}

func lintGlobalInFunction(result *Result, severity Severity) {
	for _, occ := range result.occs {
		if !occ.Decl || occ.Symbol == nil || occ.Symbol.Kind != SymbolGlobal {
			continue
		}
		if scopeInFunction(result, occ.Scope) {
			result.addLintDiagnostic(severity, CodeGlobalInFunction,
				fmt.Sprintf("global %q is assigned inside a function", occ.Text), occ.Span)
		}
	}
}

func scopeInFunction(result *Result, scope *Scope) bool {
	for current := scope; current != nil; current = current.Parent {
		if result.functionScopes[current] {
			return true
		}
	}
	return false
}

func (r *Result) addLintDiagnostic(severity Severity, code, message string, span source.Span) {
	if r == nil || severity == SeverityOff || span.IsEmpty() || r.meta[span.FileID] {
		return
	}
	for _, existing := range r.Diagnostics {
		if existing.Code == code && existing.Span == span && existing.Message == message {
			return
		}
	}
	diag := Diagnostic{Code: code, Message: message, Span: span, Severity: severity}
	if r.diagLines == nil {
		r.diagLines = make(map[Diagnostic]int)
	}
	if r.cart != nil {
		if file := r.cart.File(span.FileID); file != nil && file.Index != nil {
			if pos, err := file.Index.Position(span.Start); err == nil {
				r.diagLines[diag] = int(pos.Line)
			}
		}
	}
	r.Diagnostics = append(r.Diagnostics, diag)
}
