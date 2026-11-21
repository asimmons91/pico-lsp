package analysis

import (
	"sort"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// annotationPrimitiveTypes are keywords handled by the type parser before it
// consults the cart's named-type namespace. They never refer to user types.
var annotationPrimitiveTypes = map[string]struct{}{
	"nil":      {},
	"any":      {},
	"boolean":  {},
	"integer":  {},
	"number":   {},
	"string":   {},
	"table":    {},
	"function": {},
	"thread":   {},
	"userdata": {},
	"true":     {},
	"false":    {},
	"fun":      {},
}

// IsPrimitiveTypeName reports whether name is reserved by the LuaCATS type
// grammar and cannot be used as a user type name.
func IsPrimitiveTypeName(name string) bool {
	_, ok := annotationPrimitiveTypes[CanonicalName(name)]
	return ok
}

// bindAnnotationNames adds navigation occurrences for LuaCATS declarations and
// references that do not live in code: type declarations, type references,
// class fields, and cast targets. Parameter annotations are bound by
// attachParamDocs because they need the function's parameter symbols.
func bindAnnotationNames(snap *cart.Snapshot, result *Result) {
	if snap == nil || snap.Cart == nil || result == nil || result.Table == nil {
		return
	}
	bindTypeDeclarationSymbols(result)
	bindCastNames(result)
	bindTypeReferences(snap, result)
}

// bindTypeDeclarationSymbols creates symbols for `---@alias` and `---@enum`
// declarations and finalizes the type-name namespace. Class symbols were
// created by bindClassSymbols before class fields were attached.
func bindTypeDeclarationSymbols(result *Result) {
	if result == nil || result.Table == nil {
		return
	}
	if result.TypeSymbols == nil {
		result.TypeSymbols = make(map[string]*Symbol)
	}
	for canonical, class := range result.Classes {
		sym := result.TypeSymbols[canonical]
		if sym == nil || class == nil || class.Record == nil {
			continue
		}
		sym.Type = class.Record
		sym.TypeExplicit = true
	}
	aliasNames := make([]string, 0, len(result.Aliases))
	for canonical := range result.Aliases {
		aliasNames = append(aliasNames, canonical)
	}
	sort.Strings(aliasNames)
	for _, canonical := range aliasNames {
		alias := result.Aliases[canonical]
		if alias == nil {
			continue
		}
		span := result.aliasNameSpan[canonical]
		if span.IsEmpty() {
			continue
		}
		sym := result.Table.NewSymbol(SymbolClass, alias.Name, span, nil)
		sym.Type = alias
		sym.TypeExplicit = true
		result.TypeSymbols[canonical] = sym
		result.addOccurrence(Occurrence{
			Span:       span,
			Text:       alias.Name,
			Symbol:     sym,
			Decl:       true,
			Write:      true,
			Annotation: true,
		})
	}
	enumNames := make([]string, 0, len(result.Enums))
	for canonical := range result.Enums {
		enumNames = append(enumNames, canonical)
	}
	sort.Strings(enumNames)
	for _, canonical := range enumNames {
		enum := result.Enums[canonical]
		if enum == nil {
			continue
		}
		span := result.enumNameSpan[canonical]
		if span.IsEmpty() {
			continue
		}
		sym := result.Table.NewSymbol(SymbolClass, enum.Name, span, nil)
		sym.Type = enum
		sym.TypeExplicit = true
		result.TypeSymbols[canonical] = sym
		result.addOccurrence(Occurrence{
			Span:       span,
			Text:       enum.Name,
			Symbol:     sym,
			Decl:       true,
			Write:      true,
			Annotation: true,
		})
	}
}

// bindCastNames resolves `---@cast name` targets to the visible local, global,
// or parameter they annotate.
func bindCastNames(result *Result) {
	if result == nil || result.Table == nil {
		return
	}
	seen := make(map[source.Span]struct{})
	for _, cast := range result.Casts {
		if cast.Name == "" || cast.NameSpan.IsEmpty() {
			continue
		}
		if _, ok := seen[cast.NameSpan]; ok {
			continue
		}
		sym, scope := symbolVisibleAt(result, cast.FileID, cast.NameSpan.Start, cast.Name)
		if sym == nil || sym.Builtin {
			continue
		}
		seen[cast.NameSpan] = struct{}{}
		result.addOccurrence(Occurrence{
			Span:       cast.NameSpan,
			Text:       cast.Name,
			Symbol:     sym,
			Scope:      scope,
			Annotation: true,
		})
	}
}

// bindTypeReferences resolves type identifiers in annotation type expressions
// to `---@class`, `---@alias`, and `---@enum` declarations. Generic parameter
// names are scoped to their annotation context and never bind to a same-named
// type declaration.
func bindTypeReferences(snap *cart.Snapshot, result *Result) {
	if snap == nil || snap.Cart == nil || result == nil || len(result.TypeSymbols) == 0 {
		return
	}
	blockGenerics := result.annotationBlockGenerics()
	var generics map[string]struct{}
	var previous *syntax.Comment
	var blockStart source.Span
	for i := range snap.Comments {
		comment := &snap.Comments[i]
		if comment.Kind == syntax.CommentBlock {
			result.bindTypeSegments(snap, asAnnotationSegments(*comment), nil)
			generics = nil
			previous = nil
			blockStart = source.Span{}
			continue
		}
		if comment.Kind != syntax.CommentDoc {
			generics = nil
			previous = nil
			blockStart = source.Span{}
			continue
		}
		if previous != nil && !commentsAdjacent(snap, *previous, *comment) {
			generics = nil
			blockStart = source.Span{}
		}
		if blockStart.IsEmpty() {
			blockStart = comment.Span
			generics = cloneGenericSet(blockGenerics[blockStart])
		}
		previous = comment

		segments := AnnotationSegments(*comment)
		if len(segments) == 0 {
			continue
		}
		// Generic declarations scope every later line in the same block,
		// including a class header such as `---@class Box<T>`.
		for _, segment := range segments {
			if segment.Role != AnnotationGenericName {
				continue
			}
			name := CanonicalName(annotationSpanText(snap, segment.Span))
			if name == "" {
				continue
			}
			if generics == nil {
				generics = make(map[string]struct{})
			}
			generics[name] = struct{}{}
		}
		result.bindTypeSegments(snap, segments, generics)
	}
}

// annotationBlockGenerics maps the first comment span of an annotation block
// to the generic parameters visible in that block. Function and method docs
// see their own `@generic` names and, for methods, their class's generics.
func (r *Result) annotationBlockGenerics() map[source.Span]map[string]struct{} {
	out := make(map[source.Span]map[string]struct{})
	add := func(span source.Span, names ...string) {
		if r == nil || span.IsEmpty() {
			return
		}
		for _, name := range names {
			canonical := CanonicalName(name)
			if canonical == "" {
				continue
			}
			set := out[span]
			if set == nil {
				set = make(map[string]struct{})
				out[span] = set
			}
			set[canonical] = struct{}{}
		}
	}
	if r == nil || r.Table == nil {
		return out
	}
	for _, class := range r.Classes {
		if class == nil {
			continue
		}
		names := make([]string, 0, len(class.GenericDecls))
		for _, decl := range class.GenericDecls {
			names = append(names, decl.Name)
		}
		add(class.DeclSpan, names...)
	}
	for _, sym := range r.Table.Symbols {
		if sym == nil || sym.Doc == nil || sym.Doc.Span.IsEmpty() {
			continue
		}
		names := make([]string, 0, len(sym.Doc.Generics))
		for _, decl := range sym.Doc.Generics {
			names = append(names, decl.Name)
		}
		if sym.Owner != nil {
			if class := r.classForSymbol(sym.Owner); class != nil {
				for _, decl := range class.GenericDecls {
					names = append(names, decl.Name)
				}
			}
		}
		add(sym.Doc.Span, names...)
	}
	return out
}

func cloneGenericSet(source map[string]struct{}) map[string]struct{} {
	if len(source) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(source))
	for name := range source {
		out[name] = struct{}{}
	}
	return out
}

func (r *Result) bindTypeSegments(snap *cart.Snapshot, segments []AnnotationSegment, generics map[string]struct{}) {
	for _, segment := range segments {
		if segment.Role != AnnotationType {
			continue
		}
		if annotationNameFollowedByColonInSnapshot(snap, segment.Span) {
			// Record fields and function-type parameter names share the
			// AnnotationType token role but are not type references.
			continue
		}
		text := annotationSpanText(snap, segment.Span)
		canonical := CanonicalName(text)
		if canonical == "" {
			continue
		}
		if _, primitive := annotationPrimitiveTypes[canonical]; primitive {
			continue
		}
		if _, generic := generics[canonical]; generic {
			continue
		}
		sym := r.TypeSymbols[canonical]
		if sym == nil {
			continue
		}
		r.addOccurrence(Occurrence{
			Span:       segment.Span,
			Text:       text,
			Symbol:     sym,
			Annotation: true,
		})
	}
}

func symbolVisibleAt(result *Result, fileID source.FileID, offset int, name string) (*Symbol, *Scope) {
	if result == nil || result.Table == nil {
		return nil, nil
	}
	canonical := CanonicalName(name)
	if canonical == "" {
		return nil, nil
	}
	scope := result.ScopeAt(fileID, offset)
	if scope != nil {
		for _, sym := range result.VisibleSymbolsAt(scope, fileID, offset) {
			if sym != nil && sym.Canonical == canonical {
				return sym, scope
			}
		}
	}
	if sym := result.Table.Globals[canonical]; sym != nil {
		return sym, scope
	}
	return nil, scope
}

func annotationSpanText(snap *cart.Snapshot, span source.Span) string {
	if snap == nil || snap.Cart == nil || span.IsEmpty() {
		return ""
	}
	file := snap.Cart.File(span.FileID)
	if file == nil || span.Start < 0 || span.End > len(file.Text) || span.Start > span.End {
		return ""
	}
	return file.Text[span.Start:span.End]
}

// annotationNameFollowedByColonInSnapshot reports whether a token is a record
// field or function-type parameter name, optionally with a `?` marker, rather
// than a type reference.
func annotationNameFollowedByColonInSnapshot(snap *cart.Snapshot, span source.Span) bool {
	if snap == nil || snap.Cart == nil || span.IsEmpty() {
		return false
	}
	file := snap.Cart.File(span.FileID)
	if file == nil || span.End < 0 || span.End > len(file.Text) {
		return false
	}
	text := file.Text
	i := span.End
	for i < len(text) && isAnnotationSpace(text[i]) {
		i++
	}
	if i < len(text) && text[i] == '?' {
		i++
		for i < len(text) && isAnnotationSpace(text[i]) {
			i++
		}
	}
	return i < len(text) && text[i] == ':'
}

// asAnnotationSegments extracts type-expression token spans from a block
// comment such as `--[[@as Player]]`.
func asAnnotationSegments(comment syntax.Comment) []AnnotationSegment {
	text := comment.Text
	lower := strings.ToLower(text)
	idx := strings.Index(lower, "@as")
	if idx < 0 {
		return nil
	}
	start := idx + len("@as")
	for start < len(text) && isAnnotationSpace(text[start]) {
		start++
	}
	limit := len(text)
	if end := strings.LastIndex(text, "]]"); end > start {
		limit = end
	}
	if end := strings.LastIndex(text, "]=]"); end > start {
		limit = end
	}
	end := annotationTypeEnd(text, start, limit)
	if end <= start {
		return nil
	}
	var segments []AnnotationSegment
	add := func(role AnnotationRole, from, to int) {
		from, to = clampAnnotationRange(text, from, to)
		if to <= from {
			return
		}
		segments = append(segments, AnnotationSegment{
			Span: source.Span{
				FileID: comment.Span.FileID,
				Start:  comment.Span.Start + from,
				End:    comment.Span.Start + to,
			},
			Role: role,
		})
	}
	addAnnotationTypeTokens(add, text, start, end)
	return segments
}
