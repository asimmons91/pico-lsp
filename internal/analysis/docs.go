package analysis

import (
	"strings"
	"unicode"

	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// ParamDoc is one LuaCATS parameter annotation.
type ParamDoc struct {
	Name       string
	Type       string
	ParsedType types.Type
	Doc        string
	Optional   bool
}

// DocComment is the subset of LuaCATS docs used by M3 hover and completion.
// Type semantics are intentionally shallow here; M4 owns type checking.
type DocComment struct {
	Span          source.Span
	Summary       string
	Type          string
	Class         string
	Enum          string
	Params        []ParamDoc
	Returns       []string
	ParsedType    types.Type
	ParsedReturns []types.Type
	Deprecated    bool
	Optional      bool
}

// ClassField is one shallow `---@field` declaration.
type ClassField struct {
	Name       string
	Type       string
	ParsedType types.Type
	Doc        string
	Optional   bool
	Span       source.Span
}

// Class is the class metadata used for member completion and type checking.
type Class struct {
	Name     string
	Fields   []ClassField
	Record   *types.Record
	Exact    bool
	Parents  []string
	DeclSpan source.Span
}

// ParseDocComments extracts summaries and common LuaCATS tags from contiguous
// `---` comments. It intentionally ignores unknown tags.
func ParseDocComments(comments []syntax.Comment) *DocComment {
	if len(comments) == 0 {
		return nil
	}
	doc := &DocComment{}
	var summary []string
	for _, comment := range comments {
		if comment.Kind != syntax.CommentDoc {
			continue
		}
		if doc.Span.IsEmpty() {
			doc.Span = comment.Span
		}
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(comment.Text), "---"))
		if text == "" {
			if len(summary) > 0 {
				summary = append(summary, "")
			}
			continue
		}
		if strings.HasPrefix(text, "@") {
			parseDocTag(doc, text)
			continue
		}
		summary = append(summary, text)
	}
	doc.Summary = strings.TrimSpace(strings.Join(summary, "\n"))
	return doc
}

func parseDocTag(doc *DocComment, text string) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return
	}
	switch fields[0] {
	case "@param":
		if len(fields) < 2 {
			return
		}
		rest := strings.TrimSpace(text[len("@param"):])
		name, rest := cutFirstField(rest)
		if name == "" {
			return
		}
		typeText, paramDoc := splitTrailingType(rest)
		param := ParamDoc{
			Name:     strings.TrimSuffix(name, "?"),
			Optional: strings.HasSuffix(name, "?"),
			Type:     typeText,
			Doc:      paramDoc,
		}
		doc.Params = append(doc.Params, param)
	case "@type":
		rest := strings.TrimSpace(text[len("@type"):])
		if rest != "" {
			typeText, _ := splitTrailingType(rest)
			doc.Type = typeText
		}
	case "@class":
		name, _, _, _ := parseClassHeader(text[len("@class"):])
		if name != "" {
			doc.Class = name
		}
	case "@enum":
		if len(fields) > 1 {
			doc.Enum = strings.TrimSuffix(fields[1], ":")
		}
	case "@return":
		if len(fields) > 1 {
			doc.Returns = append(doc.Returns, strings.Join(fields[1:], " "))
		}
	case "@deprecated":
		doc.Deprecated = true
	}
}

func cutFirstField(text string) (string, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ""
	}
	index := strings.IndexAny(text, " \t")
	if index < 0 {
		return text, ""
	}
	return text[:index], strings.TrimSpace(text[index:])
}

func attachDocs(snap *cart.Snapshot, result *Result) {
	if snap == nil || snap.Index == nil || result == nil || result.Table == nil {
		return
	}
	result.Classes = collectClasses(snap)
	for _, sym := range result.Table.Symbols {
		if sym == nil || sym.Builtin || sym.DeclSpan.IsEmpty() {
			continue
		}
		if sym.Kind == SymbolParam {
			// Function docs describe parameters; avoid attaching the function
			// block above the declaration line to each parameter symbol.
			continue
		}
		file := snap.Cart.File(sym.DeclSpan.FileID)
		if file == nil || file.Index == nil {
			continue
		}
		doc := ParseDocComments(docCommentsFor(file, snap.Comments, sym.DeclSpan.Start))
		if sym.Kind == SymbolField {
			// A field using a doc block above a call line is not a
			// declaration, and fields embedded in a table/assignment line do
			// not own a doc block above that line. Only a field that starts its
			// own line can carry general docs; an explicit @type annotation is
			// still associated with an assignment target.
			if sym.InTableConstructor && !fieldAtLineStart(file, sym.DeclSpan) {
				continue
			}
			if len(sym.Decls) == 0 || (!fieldAtLineStart(file, sym.DeclSpan) && (doc == nil || doc.Type == "")) {
				continue
			}
		}
		sym.Doc = doc
	}
	attachClassFields(result)
}

func collectClasses(snap *cart.Snapshot) map[string]*Class {
	if snap == nil {
		return nil
	}
	classes := make(map[string]*Class)
	var current *Class
	var previous *syntax.Comment
	for i := range snap.Comments {
		comment := snap.Comments[i]
		if comment.Kind != syntax.CommentDoc {
			current = nil
			previous = nil
			continue
		}
		if previous != nil && !commentsAdjacent(snap, *previous, comment) {
			current = nil
		}
		previous = &snap.Comments[i]
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(comment.Text), "---"))
		fields := strings.Fields(text)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "@class":
			if len(fields) < 2 {
				current = nil
				continue
			}
			name, exact, parents, _ := parseClassHeader(text[len("@class"):])
			if name == "" {
				current = nil
				continue
			}
			canonical := CanonicalName(name)
			current = classes[canonical]
			if current == nil {
				current = &Class{Name: name}
				classes[canonical] = current
			}
			current.Exact = current.Exact || exact
			if len(parents) > 0 {
				current.Parents = parents
			}
			if current.DeclSpan.IsEmpty() {
				current.DeclSpan = comment.Span
			}
		case "@field":
			if current == nil {
				current = nil
				continue
			}
			field, ok := parseFieldAnnotation(text[len("@field"):], comment.Span)
			if !ok {
				current = nil
				continue
			}
			current.Fields = append(current.Fields, field)
		default:
			if strings.HasPrefix(text, "@") {
				current = nil
			}
		}
	}
	return classes
}

func parseClassHeader(rest string) (name string, exact bool, parents []string, exactValid bool) {
	rest = strings.TrimSpace(rest)
	exactValid = true
	namePart := rest
	parentPart := ""
	if idx := strings.Index(rest, ":"); idx >= 0 {
		namePart = rest[:idx]
		parentPart = rest[idx+1:]
	}
	if strings.Contains(namePart, "(exact)") {
		exact = true
		if strings.Count(namePart, "(exact)") > 1 {
			exactValid = false
		}
		namePart = strings.ReplaceAll(namePart, "(exact)", " ")
	}
	if strings.Contains(parentPart, "(exact)") {
		// (exact) belongs with the class name, before any parent list.
		exact = false
		exactValid = false
		parentPart = strings.ReplaceAll(parentPart, "(exact)", " ")
	}
	if parentPart != "" {
		parents = splitTypeNames(parentPart)
	}
	fields := strings.Fields(namePart)
	if len(fields) == 0 {
		return "", false, parents, exactValid
	}
	return strings.TrimSuffix(fields[0], ":"), exact, parents, exactValid
}

func splitTypeNames(text string) []string {
	text = strings.ReplaceAll(text, "(", " ")
	text = strings.ReplaceAll(text, ")", " ")
	parts := strings.Split(text, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.Fields(strings.TrimSpace(part))
		if len(name) == 0 {
			continue
		}
		// Generic arguments are parsed and ignored in M9.
		candidate := name[0]
		if idx := strings.Index(candidate, "<"); idx >= 0 {
			candidate = candidate[:idx]
		}
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			out = append(out, candidate)
		}
	}
	return out
}

func parseFieldAnnotation(rest string, span source.Span) (ClassField, bool) {
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ClassField{}, false
	}
	index := 0
	if strings.HasPrefix(fields[0], "[") && strings.HasSuffix(fields[0], "]") {
		index++
	}
	if index >= len(fields) {
		return ClassField{}, false
	}
	name := fields[index]
	if name == "" {
		return ClassField{}, false
	}
	typeText, doc := splitTrailingType(strings.Join(fields[index+1:], " "))
	return ClassField{
		Name:     strings.TrimSuffix(name, "?"),
		Optional: strings.HasSuffix(name, "?"),
		Type:     typeText,
		Doc:      doc,
		Span:     span,
	}, true
}

// splitTrailingType separates a leading type expression from trailing prose
// without requiring the cart's type namespace.
func splitTrailingType(text string) (string, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ""
	}
	_, end, err := parseTypePrefix(text, func(string) (types.Type, bool) {
		return types.AnyType, true
	})
	if err != nil || end <= 0 {
		fields := strings.Fields(text)
		if len(fields) <= 1 {
			return text, ""
		}
		return fields[0], strings.Join(fields[1:], " ")
	}
	return strings.TrimSpace(text[:end]), strings.TrimSpace(text[end:])
}

func commentsAdjacent(snap *cart.Snapshot, previous, current syntax.Comment) bool {
	if previous.Span.FileID != current.Span.FileID || snap == nil || snap.Cart == nil {
		return false
	}
	file := snap.Cart.File(current.Span.FileID)
	if file == nil || previous.Span.End < 0 || current.Span.Start > len(file.Text) || previous.Span.End > current.Span.Start {
		return false
	}
	gap := file.Text[previous.Span.End:current.Span.Start]
	return onlyWhitespace(gap) && strings.Count(gap, "\n") == 1
}

func attachClassFields(result *Result) {
	if result == nil || result.Table == nil || len(result.Classes) == 0 {
		return
	}
	for _, sym := range result.Table.Symbols {
		if sym == nil || sym.Builtin || sym.Doc == nil {
			continue
		}
		className := sym.Doc.Class
		if className == "" {
			className = classTypeName(sym.Doc.Type)
		}
		class := result.Classes[CanonicalName(className)]
		if class == nil {
			continue
		}
		if sym.Fields == nil {
			sym.Fields = make(map[string]*Symbol)
		}
		for _, field := range class.Fields {
			canonical := CanonicalName(field.Name)
			if canonical == "" {
				continue
			}
			if existing := sym.Fields[canonical]; existing != nil {
				applyClassFieldDoc(existing, field)
				continue
			}
			fieldSym := result.Table.NewSymbol(SymbolField, field.Name, field.Span, nil)
			fieldSym.Owner = sym
			applyClassFieldDoc(fieldSym, field)
			sym.Fields[canonical] = fieldSym
		}
	}
}

func applyClassFieldDoc(sym *Symbol, field ClassField) {
	if sym == nil {
		return
	}
	if sym.Doc == nil {
		if field.Type == "" && field.Doc == "" && !field.Optional {
			return
		}
		sym.Doc = &DocComment{}
	}
	if sym.Doc.Type == "" {
		sym.Doc.Type = field.Type
	}
	if sym.Doc.Summary == "" {
		sym.Doc.Summary = field.Doc
	}
	sym.Doc.Optional = sym.Doc.Optional || field.Optional
}

// fieldAtLineStart reports whether only indentation precedes span on its line.
// Table fields on their own line can own a doc block; fields embedded in a
// local declaration, table literal, or assignment line cannot.
func fieldAtLineStart(file *cart.File, span source.Span) bool {
	if file == nil || file.Index == nil || span.IsEmpty() {
		return false
	}
	pos, err := file.Index.Position(span.Start)
	if err != nil {
		return false
	}
	lineStart, err := file.Index.Offset(protocol.Position{Line: pos.Line, Character: 0})
	if err != nil || lineStart < 0 || lineStart > span.Start {
		return false
	}
	for _, r := range file.Text[lineStart:span.Start] {
		if r != ' ' && r != '\t' {
			return false
		}
	}
	return true
}

// attachParamDocs copies matching `---@param` metadata onto parameter symbols.
// The function-level doc block is still attached to the function itself.
func attachParamDocs(result *Result) {
	if result == nil || result.Table == nil {
		return
	}
	for _, sym := range result.Table.Symbols {
		if sym == nil || sym.Kind != SymbolParam || sym.Function == nil || sym.Function.Doc == nil {
			continue
		}
		for _, param := range sym.Function.Doc.Params {
			if !strings.EqualFold(param.Name, sym.Name) {
				continue
			}
			if sym.Doc == nil {
				sym.Doc = &DocComment{}
			}
			if sym.Doc.Type == "" {
				sym.Doc.Type = param.Type
			}
			if sym.Doc.Summary == "" {
				sym.Doc.Summary = param.Doc
			}
			sym.Doc.Optional = sym.Doc.Optional || param.Optional
			break
		}
	}
}

func classTypeName(value string) string {
	value = strings.TrimSpace(value)
	for i, r := range value {
		if r == '_' || r == '.' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		return value[:i]
	}
	return value
}

// docCommentsFor returns the contiguous doc-comment block directly above
// offset, allowing only whitespace between comments and the declaration. It
// works on mapped byte spans so it is correct for flattened .p8 carts and
// includes, where lexer line numbers are relative to the flattened program.
func docCommentsFor(file *cart.File, comments []syntax.Comment, offset int) []syntax.Comment {
	if file == nil || offset < 0 || offset > len(file.Text) {
		return nil
	}
	lastIndex := -1
	for i := len(comments) - 1; i >= 0; i-- {
		comment := comments[i]
		if comment.Span.FileID != file.ID || comment.Span.End > offset || comment.Kind != syntax.CommentDoc {
			continue
		}
		lastIndex = i
		break
	}
	if lastIndex < 0 {
		return nil
	}
	pos, err := file.Index.Position(offset)
	if err != nil {
		return nil
	}
	lineStart, err := file.Index.Offset(protocol.Position{Line: pos.Line, Character: 0})
	if err != nil || lineStart < 0 || lineStart > offset {
		return nil
	}
	last := comments[lastIndex]
	if !docCommentAdjacent(file.Text, last.Span.End, lineStart) {
		return nil
	}
	block := []syntax.Comment{last}
	for i := lastIndex - 1; i >= 0; i-- {
		comment := comments[i]
		if comment.Span.FileID != file.ID || comment.Kind != syntax.CommentDoc {
			break
		}
		if !docCommentAdjacent(file.Text, comment.Span.End, block[0].Span.Start) {
			break
		}
		block = append(block, syntax.Comment{})
		copy(block[1:], block[:len(block)-1])
		block[0] = comment
	}
	return block
}

func onlyWhitespace(text string) bool {
	for _, r := range text {
		switch r {
		case ' ', '\t', '\r', '\n':
		default:
			return false
		}
	}
	return true
}

// docCommentAdjacent reports whether a doc-comment block is contiguous with
// the text that follows it. A blank line breaks the block.
func docCommentAdjacent(text string, from, to int) bool {
	if from < 0 || to < from || to > len(text) {
		return false
	}
	gap := text[from:to]
	return onlyWhitespace(gap) && strings.Count(gap, "\n") == 1
}

// AnnotationNames returns shallow class and alias names from `---@class` and
// `---@alias` comments for M3 annotation completion.
func AnnotationNames(comments []syntax.Comment) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, comment := range comments {
		if comment.Kind != syntax.CommentDoc {
			continue
		}
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(comment.Text), "---"))
		fields := strings.Fields(text)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "@class", "@alias":
			add(strings.TrimSuffix(fields[1], ":"))
		}
	}
	return out
}
