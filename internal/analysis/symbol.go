// Package analysis binds the flattened cart AST to scopes and symbols and
// provides the navigation index used by the LSP layer.
package analysis

import (
	"sort"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// SymbolKind classifies a bound name.
type SymbolKind int

const (
	SymbolLocal SymbolKind = iota
	SymbolGlobal
	SymbolParam
	SymbolFunction
	SymbolMethod
	SymbolField
	SymbolLabel
	SymbolBuiltin
	SymbolClass
)

// String returns a stable name for tests and logs.
func (k SymbolKind) String() string {
	switch k {
	case SymbolLocal:
		return "local"
	case SymbolGlobal:
		return "global"
	case SymbolParam:
		return "param"
	case SymbolFunction:
		return "function"
	case SymbolMethod:
		return "method"
	case SymbolField:
		return "field"
	case SymbolLabel:
		return "label"
	case SymbolBuiltin:
		return "builtin"
	case SymbolClass:
		return "class"
	default:
		return "unknown"
	}
}

// Symbol is one bound declaration. Globals and fields may have several
// declarations and merge into one symbol.
type Symbol struct {
	ID            int
	Name          string // original spelling
	Canonical     string // ASCII-lowercased lookup key
	Kind          SymbolKind
	DeclSpan      source.Span
	SelectionSpan source.Span
	Scope         *Scope
	Builtin       bool

	Decls []source.Span
	Uses  []source.Span

	// Doc is the parsed LuaCATS/doc-comment metadata for user and built-in
	// declarations.
	Doc *DocComment
	// Signature is a user-facing signature such as `function move(p, dx, dy)`.
	Signature string
	// Params contains the declared parameter names, including `...` when used.
	Params []string
	// Function links parameter symbols, including implicit self, to the
	// function declaration that owns them.
	Function *Symbol
	// Type is the declared or inferred LuaCATS type for this symbol.
	Type types.Type
	// TypeExplicit is true when Type came from a user annotation rather than
	// inference.
	TypeExplicit bool

	// Fields maps canonical field names declared on this value. Owner is set
	// for field symbols and points at the receiver. Alias links implicit method
	// `self` to its receiver.
	Fields map[string]*Symbol
	Owner  *Symbol
	Alias  *Symbol

	// DynamicFields records an access such as t[k] on this value. Renaming a
	// field on this receiver is unsafe once this is set.
	DynamicFields bool
	// InTableConstructor marks fields declared inside a table literal, where a
	// doc block above the line documents the local or assignment target, not
	// the individual field.
	InTableConstructor bool
}

// CanonicalName ASCII-lowercases name. PICO-8 has no case distinction for
// letters; non-ASCII glyphs are left untouched.
func CanonicalName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b.WriteByte(c)
	}
	return b.String()
}

// Occurrence is one declaration or use of a symbol in the cart. A nil Symbol
// marks an unresolved read.
type Occurrence struct {
	Span   source.Span
	Text   string
	Symbol *Symbol
	Scope  *Scope
	Write  bool
	Decl   bool

	// LabelScope identifies a goto occurrence and the function whose labels it
	// targets. Labels are function-scoped, so this is not necessarily Scope or
	// one of its ancestors.
	LabelScope *Scope
}

// SymbolTable is the cart's symbol store.
type SymbolTable struct {
	Symbols  []*Symbol
	Globals  map[string]*Symbol
	Builtins map[string]*Symbol
}

// NewSymbolTable creates an empty table and merges builtin declarations.
func NewSymbolTable(builtins map[string]*Symbol) *SymbolTable {
	t := &SymbolTable{
		Globals:  make(map[string]*Symbol),
		Builtins: builtins,
	}
	return t
}

// NewSymbol creates and records a symbol. Callers are responsible for adding
// it to a scope or to Globals/Fields as appropriate.
func (t *SymbolTable) NewSymbol(kind SymbolKind, name string, span source.Span, scope *Scope) *Symbol {
	sym := &Symbol{
		ID:            len(t.Symbols),
		Name:          name,
		Canonical:     CanonicalName(name),
		Kind:          kind,
		DeclSpan:      span,
		SelectionSpan: span,
		Scope:         scope,
	}
	t.Symbols = append(t.Symbols, sym)
	return sym
}

// Result is the semantic model and diagnostics for one cart snapshot.
type Result struct {
	Table       *SymbolTable
	Diagnostics []Diagnostic

	// Classes holds `---@class` metadata used by member completion and type
	// checking.
	Classes map[string]*Class
	// Aliases holds `---@alias` declarations by canonical name.
	Aliases map[string]*types.Alias
	// Enums holds `---@enum` declarations by canonical name.
	Enums map[string]*types.Record

	typeNames   map[string]types.Type
	aliasRaw    map[string]string
	funcSymbols map[*syntax.FunctionExpr]*Symbol
	diagLines   map[Diagnostic]int

	// Casts, AsCasts, and DiagnosticDirectives are parsed annotation directives
	// used by inference and diagnostic suppression.
	Casts                []CastDirective
	AsCasts              []AsDirective
	DiagnosticDirectives []DiagnosticDirective

	// flowTypes records the narrowed type of expression occurrences at the end
	// of the final inference pass. Hover and completion use it to show
	// flow-sensitive types without re-running inference.
	flowTypes map[source.Span]types.Type
	// classFieldLinks connects field symbols declared on a class table to the
	// record fields used for inherited lookup and completion.
	classFieldLinks map[*Symbol]*types.Field
	// classFieldSymbols is the inverse of classFieldLinks, used to recover
	// annotations from an inherited field type.
	classFieldSymbols map[*types.Field]*Symbol
	// paramIndex maps a function symbol to its parameter symbols by canonical
	// name. It is built once after binding.
	paramIndex map[*Symbol]map[string]*Symbol

	occs           []Occurrence
	byFile         map[source.FileID][]Occurrence
	meta           map[source.FileID]bool
	scopes         []*Scope
	chunkScope     *Scope
	functionScopes map[*Scope]bool
	cart           *cart.Cart
}

// ScopeAt returns the innermost lexical scope containing offset. It falls back
// to the scope of the closest preceding occurrence, then to the chunk scope.
func (r *Result) ScopeAt(fileID source.FileID, offset int) *Scope {
	if r == nil {
		return nil
	}
	var best *Scope
	bestLen := int(^uint(0) >> 1)
	for _, scope := range r.scopes {
		if scope == nil || scope.Span.FileID != fileID || scope.Span.IsEmpty() {
			continue
		}
		if offset < scope.Span.Start || offset > scope.Span.End {
			continue
		}
		if length := scope.Span.Len(); length <= bestLen {
			best = scope
			bestLen = length
		}
	}
	if best != nil {
		return best
	}
	for i := len(r.occs) - 1; i >= 0; i-- {
		occ := &r.occs[i]
		if occ.Span.FileID != fileID || occ.Span.Start > offset {
			continue
		}
		if occ.Scope == nil {
			continue
		}
		// The nearest occurrence is a useful fallback for positions at a
		// boundary, but it can point into a function that has already ended.
		// Walk outward until a scope is known to contain the offset. Scopes in
		// another file cannot be checked against this offset, so keep them for
		// included code, where the enclosing span is mapped to the cart file.
		for scope := occ.Scope; scope != nil; scope = scope.Parent {
			if scope == r.chunkScope || scope.Span.FileID != fileID || scope.Span.IsEmpty() || offset <= scope.Span.End {
				return scope
			}
		}
		return r.chunkScope
	}
	return r.chunkScope
}

// VisibleSymbolsAt returns declarations visible at offset in fileID, accounting
// for the flattened order of included files. A declaration in another file is
// visible only when its include appears before the cursor.
func (r *Result) VisibleSymbolsAt(scope *Scope, fileID source.FileID, offset int) []*Symbol {
	if scope == nil {
		return nil
	}
	return scope.visibleSymbolsBefore(func(candidate *Symbol) bool {
		return r.declarationBefore(candidate.DeclSpan, fileID, offset)
	})
}

// TypeNames returns the named types declared in the cart, keyed canonically.
func (r *Result) TypeNames() map[string]types.Type {
	if r == nil || len(r.typeNames) == 0 {
		return nil
	}
	out := make(map[string]types.Type, len(r.typeNames))
	for name, typ := range r.typeNames {
		out[name] = typ
	}
	return out
}

// FlowType returns the narrowed type recorded for an expression occurrence.
func (r *Result) FlowType(span source.Span) (types.Type, bool) {
	if r == nil || len(r.flowTypes) == 0 {
		return nil, false
	}
	typ, ok := r.flowTypes[span]
	return typ, ok
}

func (r *Result) recordFlow(span source.Span, typ types.Type) {
	if r == nil || span.IsEmpty() || typ == nil {
		return
	}
	if r.flowTypes == nil {
		r.flowTypes = make(map[source.Span]types.Type)
	}
	r.flowTypes[span] = typ
}

// paramSymbolsFor returns the parameter symbols of function keyed by canonical
// name. The index is built once for the whole cart.
func (r *Result) paramSymbolsFor(function *Symbol) map[string]*Symbol {
	if r == nil || r.Table == nil || function == nil {
		return nil
	}
	if r.paramIndex == nil {
		r.paramIndex = make(map[*Symbol]map[string]*Symbol)
		for _, sym := range r.Table.Symbols {
			if sym == nil || sym.Kind != SymbolParam || sym.Function == nil {
				continue
			}
			byName := r.paramIndex[sym.Function]
			if byName == nil {
				byName = make(map[string]*Symbol)
				r.paramIndex[sym.Function] = byName
			}
			byName[CanonicalName(sym.Name)] = sym
		}
	}
	return r.paramIndex[function]
}

func (r *Result) lineForSpan(span source.Span) int {
	if r == nil || r.cart == nil {
		return -1
	}
	file := r.cart.File(span.FileID)
	if file == nil || file.Index == nil {
		return -1
	}
	pos, err := file.Index.Position(span.Start)
	if err != nil {
		return -1
	}
	return int(pos.Line)
}

func (r *Result) declarationBefore(decl source.Span, fileID source.FileID, offset int) bool {
	if decl.FileID == fileID {
		return decl.Start <= offset
	}
	if r == nil || r.cart == nil {
		return false
	}
	declFlat, ok := r.cart.FlatPosition(decl.FileID, decl.Start)
	if !ok {
		return false
	}
	currentFlat, ok := r.cart.FlatPosition(fileID, offset)
	if !ok {
		return false
	}
	return declFlat <= currentFlat
}

// Occurrences returns occurrences in fileID in source order.
func (r *Result) Occurrences(fileID source.FileID) []Occurrence {
	if r == nil {
		return nil
	}
	return r.byFile[fileID]
}

// OccurrencesForSymbol returns every declaration and use of sym.
func (r *Result) OccurrencesForSymbol(sym *Symbol) []Occurrence {
	if r == nil || sym == nil {
		return nil
	}
	out := make([]Occurrence, 0, len(sym.Decls)+len(sym.Uses))
	for _, occ := range r.occs {
		if occ.Symbol == sym {
			out = append(out, occ)
		}
	}
	return out
}

// OccurrenceAt returns the occurrence at or immediately before offset. A
// cursor at the end of an identifier resolves that identifier.
func (r *Result) OccurrenceAt(fileID source.FileID, offset int) *Occurrence {
	if r == nil {
		return nil
	}
	occs := r.byFile[fileID]
	if len(occs) == 0 {
		return nil
	}
	i := sort.Search(len(occs), func(i int) bool {
		return occs[i].Span.Start > offset
	}) - 1
	if i >= 0 && occs[i].Span.Contains(offset) {
		return &occs[i]
	}
	if offset > 0 {
		i = sort.Search(len(occs), func(i int) bool {
			return occs[i].Span.Start > offset-1
		}) - 1
		if i >= 0 && occs[i].Span.Contains(offset-1) {
			return &occs[i]
		}
	}
	return nil
}

// Definitions returns the declaration spans for sym. Built-ins have no
// user-navigable definitions in v1.
func (r *Result) Definitions(sym *Symbol) []source.Span {
	if r == nil || sym == nil || sym.Builtin {
		return nil
	}
	if len(sym.Decls) == 0 && sym.Kind == SymbolParam && sym.Canonical == "self" && !sym.DeclSpan.IsEmpty() {
		// Implicit method self points at the method name, but has no textual
		// declaration occurrence of its own.
		return []source.Span{sym.DeclSpan}
	}
	return sym.Decls
}

// References returns use spans and, optionally, declarations for sym.
// Built-ins return no references; reporting the whole stub would be noise.
func (r *Result) References(sym *Symbol, includeDecl bool) []source.Span {
	if r == nil || sym == nil || sym.Builtin {
		return nil
	}
	out := make([]source.Span, 0, len(sym.Uses)+len(sym.Decls))
	if includeDecl {
		out = append(out, sym.Decls...)
	}
	out = append(out, sym.Uses...)
	sortSpans(out)
	return out
}

func (r *Result) addOccurrence(occ Occurrence) {
	if occ.Span.IsEmpty() {
		return
	}
	if occ.Symbol != nil && !occ.Symbol.Builtin {
		if occ.Decl {
			first := len(occ.Symbol.Decls) == 0
			occ.Symbol.Decls = appendUniqueSpan(occ.Symbol.Decls, occ.Span)
			if first {
				occ.Symbol.DeclSpan = occ.Span
				occ.Symbol.SelectionSpan = occ.Span
			}
		} else {
			occ.Symbol.Uses = appendUniqueSpan(occ.Symbol.Uses, occ.Span)
		}
	}
	r.occs = append(r.occs, occ)
}

func (r *Result) finish() {
	sort.SliceStable(r.occs, func(i, j int) bool {
		a, b := r.occs[i].Span, r.occs[j].Span
		if a.FileID != b.FileID {
			return a.FileID < b.FileID
		}
		if a.Start != b.Start {
			return a.Start < b.Start
		}
		return a.End < b.End
	})
	r.byFile = make(map[source.FileID][]Occurrence)
	for _, occ := range r.occs {
		r.byFile[occ.Span.FileID] = append(r.byFile[occ.Span.FileID], occ)
	}
}

func appendUniqueSpan(spans []source.Span, span source.Span) []source.Span {
	for _, existing := range spans {
		if existing == span {
			return spans
		}
	}
	return append(spans, span)
}

func sortSpans(spans []source.Span) {
	sort.SliceStable(spans, func(i, j int) bool {
		if spans[i].FileID != spans[j].FileID {
			return spans[i].FileID < spans[j].FileID
		}
		if spans[i].Start != spans[j].Start {
			return spans[i].Start < spans[j].Start
		}
		return spans[i].End < spans[j].End
	})
}

// RenameConflict returns another symbol that would collide if sym were renamed
// to newCanonical, or nil when the rename is safe. Callers still need to refuse
// built-ins, dynamic fields, and invalid names.
func (r *Result) RenameConflict(sym *Symbol, newCanonical string) *Symbol {
	if r == nil || sym == nil || sym.Builtin || newCanonical == sym.Canonical {
		return nil
	}
	table := r.Table
	if sym.Owner != nil {
		if sym.Owner.DynamicFields {
			return sym
		}
		if other := sym.Owner.Fields[newCanonical]; other != nil && other != sym {
			return other
		}
		return nil
	}
	if table.Globals[sym.Canonical] == sym {
		if other := table.Globals[newCanonical]; other != nil && other != sym {
			return other
		}
		// A local declaration with the new name must not capture an existing
		// use or assignment of the global.
		for i := range r.occs {
			occ := &r.occs[i]
			if occ.Symbol != sym || occ.Scope == nil {
				continue
			}
			if other := occ.Scope.Lookup(newCanonical); other != nil && other != sym {
				return other
			}
		}
		return nil
	}
	if sym.Kind == SymbolLabel {
		if sym.Scope != nil {
			if other := sym.Scope.Labels[newCanonical]; other != nil && other != sym {
				return other
			}
		}
		for i := range r.occs {
			occ := &r.occs[i]
			if occ.Symbol != nil || occ.LabelScope == nil || occ.LabelScope != sym.Scope {
				continue
			}
			if CanonicalName(occ.Text) != newCanonical {
				continue
			}
			// An unresolved goto with the new name would start targeting this
			// renamed label.
			return &Symbol{Name: occ.Text, Canonical: newCanonical}
		}
		return nil
	}

	// Locals, parameters, and local functions can be shadowed by another
	// declaration or can capture an existing use inside their scope.
	for _, other := range table.Symbols {
		if other == sym || other.Builtin || other.Owner != nil || other.Kind == SymbolLabel || other.Canonical != newCanonical {
			continue
		}
		if table.Globals[other.Canonical] == other {
			// Global collisions are checked through uses below; a global with
			// no captured use does not conflict with a local.
			continue
		}
		if sym.Scope != nil && sym.Scope.IsAncestorOf(other.Scope) {
			return other
		}
	}
	for i := range r.occs {
		occ := &r.occs[i]
		if occ.Symbol == sym || CanonicalName(occ.Text) != newCanonical {
			continue
		}
		if occ.Symbol != nil && (occ.Symbol.Owner != nil || occ.Symbol.Kind == SymbolLabel) {
			// Field and label names live in separate namespaces.
			continue
		}
		if sym.Scope != nil && sym.Scope.IsAncestorOf(occ.Scope) {
			if occ.Symbol != nil {
				return occ.Symbol
			}
			// An unresolved or built-in use would be captured by the renamed
			// local, so the rename is not safe.
			return &Symbol{Name: occ.Text, Canonical: newCanonical}
		}
	}
	return nil
}
