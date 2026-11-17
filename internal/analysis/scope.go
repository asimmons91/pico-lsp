package analysis

import (
	"sort"

	"github.com/asimmons91/pico-lsp/internal/source"
)

// Scope is one lexical scope. A scope may contain several declarations with
// the same canonical name; lookup returns the most recent one.
type Scope struct {
	Parent  *Scope
	symbols map[string][]*Symbol

	// Span is the approximate original-file extent used for completion scope
	// selection. Empty spans are ignored.
	Span source.Span

	// Labels are function-scoped in Lua/PICO-8. Only function and chunk scopes
	// use this map.
	Labels map[string]*Symbol
}

// NewScope creates a scope with parent.
func NewScope(parent *Scope) *Scope {
	return &Scope{
		Parent:  parent,
		symbols: make(map[string][]*Symbol),
	}
}

// Declare adds sym to s.
func (s *Scope) Declare(sym *Symbol) {
	if s == nil || sym == nil {
		return
	}
	key := sym.Canonical
	if key == "" {
		key = CanonicalName(sym.Name)
	}
	s.symbols[key] = append(s.symbols[key], sym)
}

// Lookup finds the most recent declaration visible from s.
func (s *Scope) Lookup(canonical string) *Symbol {
	for scope := s; scope != nil; scope = scope.Parent {
		if list := scope.symbols[canonical]; len(list) > 0 {
			return list[len(list)-1]
		}
	}
	return nil
}

// VisibleSymbols returns declarations visible from s, with closer scopes and
// newer declarations taking precedence. The result is sorted by canonical name
// for deterministic completion output.
func (s *Scope) VisibleSymbols() []*Symbol {
	return s.visibleSymbolsBefore(nil)
}

// VisibleSymbolsAt returns the declarations visible at offset in fileID. A
// declaration later in the same file is not visible yet, so an outer
// declaration with the same name is returned until that point. Cross-file
// ordering needs cart metadata; use Result.VisibleSymbolsAt for that.
func (s *Scope) VisibleSymbolsAt(fileID source.FileID, offset int) []*Symbol {
	return s.visibleSymbolsBefore(func(candidate *Symbol) bool {
		return candidate.DeclSpan.FileID != fileID || candidate.DeclSpan.Start <= offset
	})
}

func (s *Scope) visibleSymbolsBefore(visible func(*Symbol) bool) []*Symbol {
	seen := make(map[string]struct{})
	var out []*Symbol
	for scope := s; scope != nil; scope = scope.Parent {
		for canonical, list := range scope.symbols {
			if _, ok := seen[canonical]; ok {
				continue
			}
			if len(list) == 0 {
				continue
			}
			chosen := list[len(list)-1]
			if visible != nil {
				chosen = nil
				for i := len(list) - 1; i >= 0; i-- {
					candidate := list[i]
					if visible(candidate) {
						chosen = candidate
						break
					}
				}
				if chosen == nil {
					// The inner declaration is not visible yet. Keep looking
					// for an outer declaration with the same name.
					continue
				}
			}
			seen[canonical] = struct{}{}
			out = append(out, chosen)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Canonical < out[j].Canonical
	})
	return out
}

// IsAncestorOf reports whether s is ancestor or equal to other.
func (s *Scope) IsAncestorOf(other *Scope) bool {
	for scope := other; scope != nil; scope = scope.Parent {
		if scope == s {
			return true
		}
	}
	return false
}
