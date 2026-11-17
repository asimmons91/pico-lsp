// Package builtin embeds and loads the PICO-8 0.2.7 API stub. The stub is
// parsed with the same PICO-8 lexer and parser as carts so it cannot silently
// drift into unsupported syntax.
package builtin

import (
	_ "embed"
	"fmt"
	"sort"

	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// Version is the only API version supported in v1.
const Version = "0.2.7"

//go:embed pico8-0.2.7.lua
var Source string

// Kind classifies one built-in declaration.
type Kind int

const (
	KindFunction Kind = iota
	KindConstant
)

// Name is one built-in declaration.
type Name struct {
	Name string
	Kind Kind
	Span source.Span
}

// API is the parsed, immutable built-in API stub.
type API struct {
	Source string
	Chunk  *syntax.Chunk
	Names  []Name
	byName map[string]Name
	err    error
}

// Load parses the embedded stub. It returns an error when the stub would not
// load as a normal cart, which makes syntax drift a startup-visible failure.
func Load() (*API, error) {
	lexed := syntax.Lex([]byte(Source), nil)
	if len(lexed.Errors) > 0 {
		first := lexed.Errors[0]
		return nil, fmt.Errorf("builtin: lex PICO-8 %s API: %s at %d", Version, first.Message, first.Span.Start)
	}
	chunk, parseErrors := syntax.Parse(lexed.Tokens)
	if len(parseErrors) > 0 {
		first := parseErrors[0]
		return nil, fmt.Errorf("builtin: parse PICO-8 %s API: %s at %d", Version, first.Message, first.Span.Start)
	}
	if chunk == nil || chunk.Block == nil {
		return nil, fmt.Errorf("builtin: PICO-8 %s API stub is empty", Version)
	}

	api := &API{
		Source: Source,
		Chunk:  chunk,
		byName: make(map[string]Name),
	}
	for _, stmt := range chunk.Block.Stmts {
		switch st := stmt.(type) {
		case *syntax.FunctionDeclStmt:
			if len(st.Names) != 1 || st.Method {
				return nil, fmt.Errorf("builtin: %s API declaration is not a plain function", Version)
			}
			api.add(Name{Name: st.Names[0].Text, Kind: KindFunction, Span: st.Names[0].Span()})
		case *syntax.AssignStmt:
			for _, target := range st.Targets {
				name, ok := target.(*syntax.NameExpr)
				if !ok {
					return nil, fmt.Errorf("builtin: %s API assignment target is not a name", Version)
				}
				api.add(Name{Name: name.Text, Kind: KindConstant, Span: name.Span()})
			}
		case *syntax.EmptyStmt:
			// Comments leave no statements behind; empty statements are benign.
		default:
			return nil, fmt.Errorf("builtin: %s API contains unsupported top-level declaration %T", Version, stmt)
		}
	}
	if api.err != nil {
		return nil, api.err
	}
	if len(api.Names) < 100 {
		return nil, fmt.Errorf("builtin: %s API has only %d names", Version, len(api.Names))
	}
	sort.SliceStable(api.Names, func(i, j int) bool {
		return api.Names[i].Span.Start < api.Names[j].Span.Start
	})
	return api, nil
}

func (a *API) add(name Name) {
	a.Names = append(a.Names, name)
	if existing, ok := a.byName[name.Name]; ok {
		if a.err == nil {
			a.err = fmt.Errorf("builtin: duplicate %s declaration (%+v and %+v)", name.Name, existing.Span, name.Span)
		}
		return
	}
	a.byName[name.Name] = name
}

// Lookup returns the declaration for a built-in name. PICO-8 names are
// case-insensitive, so callers should pass an ASCII-lowercased name.
func (a *API) Lookup(name string) (Name, bool) {
	if a == nil {
		return Name{}, false
	}
	n, ok := a.byName[name]
	return n, ok
}

// SortedNames returns the declared names sorted for stable manifest checks.
func (a *API) SortedNames() []string {
	if a == nil {
		return nil
	}
	out := make([]string, 0, len(a.Names))
	for _, name := range a.Names {
		out = append(out, name.Name)
	}
	sort.Strings(out)
	return out
}
