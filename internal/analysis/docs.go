package analysis

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

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
	// NameSpan is the byte range of the parameter name in the annotation.
	NameSpan source.Span
}

// DocComment is the subset of LuaCATS docs used by M3 hover and completion.
// Type semantics are intentionally shallow here; M4 owns type checking.
type DocComment struct {
	Span            source.Span
	Summary         string
	Type            string
	Class           string
	Enum            string
	Params          []ParamDoc
	Returns         []string
	ParsedType      types.Type
	ParsedReturns   []types.Type
	Generics        []GenericDecl
	Overloads       []string
	ParsedGenerics  []*types.GenericParam
	ParsedOverloads []*types.Function
	Deprecated      bool
	Optional        bool
}

// ClassField is one shallow `---@field` declaration.
type ClassField struct {
	Name       string
	Type       string
	ParsedType types.Type
	Doc        string
	Optional   bool
	Span       source.Span
	// NameSpan is the byte range of the field name in the annotation.
	NameSpan source.Span
}

// Class is the class metadata used for member completion and type checking.
type Class struct {
	Name         string
	Fields       []ClassField
	Record       *types.Record
	Exact        bool
	Parents      []string
	GenericDecls []GenericDecl
	Operators    []ClassOperator
	DeclSpan     source.Span
	// NameSpan is the byte range of the first class name in `---@class`.
	NameSpan source.Span
	// NameSpans records every `---@class` name span when a class is declared
	// more than once.
	NameSpans []source.Span
}

// GenericDecl is one parsed generic parameter declaration.
type GenericDecl struct {
	Name    string
	Default string
}

// ClassOperator is one parsed operator annotation before type resolution.
type ClassOperator struct {
	Name    string
	Operand string
	Result  string
	Span    source.Span
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
			before := len(doc.Params)
			parseDocTag(doc, text)
			if len(doc.Params) > before {
				doc.Params[len(doc.Params)-1].NameSpan = trimAnnotationNameSpan(
					comment, annotationRoleSpan(comment, AnnotationParamName), "?")
			}
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
	case "@generic":
		doc.Generics = append(doc.Generics, parseGenericDeclList(text[len("@generic"):])...)
	case "@overload":
		rest := strings.TrimSpace(text[len("@overload"):])
		if rest != "" {
			doc.Overloads = append(doc.Overloads, rest)
		}
	case "@class":
		name, _, _, _, _ := parseClassHeader(text[len("@class"):])
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
	bindClassSymbols(snap, result)
	attachClassFields(result)
}

func collectClasses(snap *cart.Snapshot) map[string]*Class {
	if snap == nil {
		return nil
	}
	classes := make(map[string]*Class)
	var current *Class
	var previous *syntax.Comment
	var pendingGenerics []GenericDecl
	for i := range snap.Comments {
		comment := snap.Comments[i]
		if comment.Kind != syntax.CommentDoc {
			current = nil
			previous = nil
			pendingGenerics = nil
			continue
		}
		if previous != nil && !commentsAdjacent(snap, *previous, comment) {
			current = nil
			pendingGenerics = nil
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
			name, exact, parents, generics, _ := parseClassHeader(text[len("@class"):])
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
			if len(generics) > 0 {
				current.GenericDecls = generics
			}
			if len(pendingGenerics) > 0 {
				current.GenericDecls = append(current.GenericDecls, pendingGenerics...)
				pendingGenerics = nil
			}
			current.GenericDecls = dedupeGenericDecls(current.GenericDecls)
			if current.DeclSpan.IsEmpty() {
				current.DeclSpan = comment.Span
			}
			if span := annotationRoleSpan(comment, AnnotationClassName); !span.IsEmpty() {
				if current.NameSpan.IsEmpty() {
					current.NameSpan = span
				}
				current.NameSpans = appendUniqueSpan(current.NameSpans, span)
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
			field.NameSpan = trimAnnotationNameSpan(
				comment, annotationRoleSpan(comment, AnnotationFieldName), "?")
			current.Fields = append(current.Fields, field)
		case "@operator":
			if current == nil {
				continue
			}
			op, ok := parseOperatorAnnotation(text[len("@operator"):], comment.Span)
			if !ok {
				current = nil
				continue
			}
			current.Operators = append(current.Operators, op)
		case "@generic":
			decls := parseGenericDeclList(text[len("@generic"):])
			if current == nil {
				pendingGenerics = append(pendingGenerics, decls...)
				continue
			}
			current.GenericDecls = append(current.GenericDecls, decls...)
			current.GenericDecls = dedupeGenericDecls(current.GenericDecls)
		default:
			if strings.HasPrefix(text, "@") {
				current = nil
				pendingGenerics = nil
			}
		}
	}
	return classes
}

func parseClassHeader(rest string) (name string, exact bool, parents []string, generics []GenericDecl, exactValid bool) {
	rest = strings.TrimSpace(rest)
	exactValid = true
	namePart := rest
	parentPart := ""
	if idx := splitTopLevelColon(rest); idx >= 0 {
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
		parents = splitTypeExpressions(parentPart)
	}
	namePart = strings.TrimSpace(namePart)
	if namePart == "" {
		return "", false, parents, nil, exactValid
	}
	end := 0
	for end < len(namePart) {
		r, size := utf8.DecodeRuneInString(namePart[end:])
		if r == '_' || r == '.' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			end += size
			continue
		}
		break
	}
	name = namePart[:end]
	remaining := strings.TrimSpace(namePart[end:])
	if strings.HasPrefix(remaining, "<") {
		generics = parseGenericDecls(remaining)
	}
	return name, exact, parents, generics, exactValid
}

func splitTopLevelColon(text string) int {
	depth := 0
	for i, r := range text {
		switch r {
		case '<', '{', '[', '(':
			depth++
		case '>', '}', ']', ')':
			if depth > 0 {
				depth--
			}
		case ':':
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func parseGenericDecls(text string) []GenericDecl {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "<") {
		return nil
	}
	close := strings.LastIndex(text, ">")
	if close < 0 {
		return nil
	}
	inner := text[1:close]
	parts := splitTypeExpressions(inner)
	out := make([]GenericDecl, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		decl := GenericDecl{}
		if idx := strings.Index(part, "="); idx >= 0 {
			decl.Default = strings.TrimSpace(part[idx+1:])
			part = strings.TrimSpace(part[:idx])
		}
		if idx := strings.Index(part, ":"); idx >= 0 {
			part = strings.TrimSpace(part[:idx])
		}
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		decl.Name = fields[0]
		if decl.Name != "" {
			out = append(out, decl)
		}
	}
	return out
}

func dedupeGenericDecls(decls []GenericDecl) []GenericDecl {
	if len(decls) < 2 {
		return decls
	}
	seen := make(map[string]int, len(decls))
	out := decls[:0]
	for _, decl := range decls {
		key := CanonicalName(decl.Name)
		if key == "" {
			continue
		}
		if index, ok := seen[key]; ok {
			if out[index].Default == "" && decl.Default != "" {
				out[index].Default = decl.Default
			}
			continue
		}
		seen[key] = len(out)
		out = append(out, decl)
	}
	return out
}

func parseGenericDeclList(text string) []GenericDecl {
	parts := splitTypeExpressions(text)
	out := make([]GenericDecl, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		decl := GenericDecl{}
		if idx := strings.Index(part, "="); idx >= 0 {
			decl.Default = strings.TrimSpace(part[idx+1:])
			part = strings.TrimSpace(part[:idx])
		}
		if idx := strings.Index(part, ":"); idx >= 0 {
			part = strings.TrimSpace(part[:idx])
		}
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		decl.Name = fields[0]
		if decl.Name != "" {
			out = append(out, decl)
		}
	}
	return out
}

func splitTypeExpressions(text string) []string {
	var out []string
	depth := 0
	start := 0
	for i, r := range text {
		switch r {
		case '<', '{', '[', '(':
			depth++
		case '>', '}', ']', ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				if part := strings.TrimSpace(text[start:i]); part != "" {
					out = append(out, part)
				}
				start = i + 1
			}
		}
	}
	if part := strings.TrimSpace(text[start:]); part != "" {
		out = append(out, part)
	}
	return out
}

func parseOperatorAnnotation(rest string, span source.Span) (ClassOperator, bool) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return ClassOperator{}, false
	}
	end := strings.IndexAny(rest, "( \t:")
	if end < 0 {
		end = len(rest)
	}
	name := strings.TrimSpace(rest[:end])
	if name == "" {
		return ClassOperator{}, false
	}
	op := ClassOperator{Name: strings.ToLower(name), Span: span}
	rest = strings.TrimSpace(rest[end:])
	if strings.HasPrefix(rest, "(") {
		close := strings.Index(rest, ")")
		if close < 0 {
			return op, true
		}
		op.Operand = strings.TrimSpace(rest[1:close])
		rest = strings.TrimSpace(rest[close+1:])
	}
	if idx := strings.Index(rest, ":"); idx >= 0 {
		op.Result = strings.TrimSpace(rest[idx+1:])
	}
	return op, true
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

// bindClassSymbols creates one declaration symbol per `---@class` and one
// canonical symbol per `---@field`. Runtime receiver fields link to the
// canonical field through Symbol.ClassField so rename and references cover the
// annotation and every annotated receiver together.
func bindClassSymbols(snap *cart.Snapshot, result *Result) {
	if result == nil || result.Table == nil || len(result.Classes) == 0 {
		return
	}
	if result.TypeSymbols == nil {
		result.TypeSymbols = make(map[string]*Symbol)
	}
	if result.ClassFields == nil {
		result.ClassFields = make(map[string]map[string]*Symbol)
	}
	names := make([]string, 0, len(result.Classes))
	for name := range result.Classes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, canonical := range names {
		class := result.Classes[canonical]
		if class == nil {
			continue
		}
		span := class.NameSpan
		if span.IsEmpty() {
			continue
		}
		classSym := result.TypeSymbols[canonical]
		if classSym == nil {
			classSym = result.Table.NewSymbol(SymbolClass, class.Name, span, nil)
			// The class name doubles as the receiver type for generic
			// resolution on canonical `---@field` symbols.
			classSym.Doc = &DocComment{Class: class.Name}
			result.TypeSymbols[canonical] = classSym
		}
		nameSpans := class.NameSpans
		if len(nameSpans) == 0 {
			nameSpans = []source.Span{span}
		}
		for _, nameSpan := range nameSpans {
			text := annotationSpanText(snap, nameSpan)
			if text == "" {
				text = class.Name
			}
			result.addOccurrence(Occurrence{
				Span:       nameSpan,
				Text:       text,
				Symbol:     classSym,
				Decl:       true,
				Write:      true,
				Annotation: true,
			})
		}
		if classSym.Fields == nil {
			classSym.Fields = make(map[string]*Symbol)
		}
		fields := result.ClassFields[canonical]
		if fields == nil {
			fields = make(map[string]*Symbol)
			result.ClassFields[canonical] = fields
		}
		for i := range class.Fields {
			field := &class.Fields[i]
			key := CanonicalName(field.Name)
			if key == "" {
				continue
			}
			fieldSym := fields[key]
			if fieldSym != nil {
				applyClassFieldDoc(fieldSym, *field)
				if !field.NameSpan.IsEmpty() {
					text := annotationSpanText(snap, field.NameSpan)
					if text == "" {
						text = field.Name
					}
					result.addOccurrence(Occurrence{
						Span:       field.NameSpan,
						Text:       text,
						Symbol:     fieldSym,
						Decl:       true,
						Write:      true,
						Annotation: true,
					})
				}
				continue
			}
			fieldSpan := field.NameSpan
			if fieldSpan.IsEmpty() {
				continue
			}
			fieldSym = result.Table.NewSymbol(SymbolField, field.Name, fieldSpan, nil)
			fieldSym.Owner = classSym
			fieldSym.FromClass = true
			text := annotationSpanText(snap, fieldSpan)
			if text == "" {
				text = field.Name
			}
			result.addOccurrence(Occurrence{
				Span:       fieldSpan,
				Text:       text,
				Symbol:     fieldSym,
				Decl:       true,
				Write:      true,
				Annotation: true,
			})
			fields[key] = fieldSym
			classSym.Fields[key] = fieldSym
			applyClassFieldDoc(fieldSym, *field)
		}
	}
}

func attachClassFields(result *Result) {
	if result == nil || result.Table == nil || len(result.Classes) == 0 {
		return
	}
	for _, sym := range result.Table.Symbols {
		if sym == nil || sym.Builtin || sym.Doc == nil || isClassFieldDeclaration(sym) {
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
		classCanonical := CanonicalName(className)
		canonicalFields := result.ClassFields[classCanonical]
		for _, field := range class.Fields {
			canonical := CanonicalName(field.Name)
			if canonical == "" {
				continue
			}
			classField := canonicalFields[canonical]
			if existing := sym.Fields[canonical]; existing != nil {
				if existing == classField {
					continue
				}
				existing.FromClass = true
				if existing.ClassField == nil {
					existing.ClassField = classField
				}
				applyClassFieldDoc(existing, field)
				continue
			}
			fieldSpan := field.NameSpan
			if fieldSpan.IsEmpty() {
				fieldSpan = field.Span
			}
			fieldSym := result.Table.NewSymbol(SymbolField, field.Name, fieldSpan, nil)
			fieldSym.Owner = sym
			fieldSym.FromClass = true
			fieldSym.ClassField = classField
			applyClassFieldDoc(fieldSym, field)
			sym.Fields[canonical] = fieldSym
		}
		// Link fields inherited from parent classes so annotation rename and
		// references reach the parent's `---@field` declaration too.
		for key, fieldSym := range sym.Fields {
			if fieldSym == nil || fieldSym.ClassField != nil || isClassFieldDeclaration(fieldSym) {
				continue
			}
			if parentField := ancestorClassField(result, classCanonical, key); parentField != nil {
				fieldSym.ClassField = parentField
			}
		}
	}
}

func ancestorClassField(result *Result, classCanonical, fieldCanonical string) *Symbol {
	if result == nil || classCanonical == "" || fieldCanonical == "" {
		return nil
	}
	seen := make(map[string]struct{})
	queue := []string{classCanonical}
	for len(queue) > 0 {
		canonical := queue[0]
		queue = queue[1:]
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		class := result.Classes[canonical]
		if class == nil {
			continue
		}
		for _, parent := range class.Parents {
			parentCanonical := CanonicalName(parent)
			if base := classTypeName(parent); base != "" {
				parentCanonical = CanonicalName(base)
			}
			if fields := result.ClassFields[parentCanonical]; fields != nil {
				if field := fields[fieldCanonical]; field != nil {
					return field
				}
			}
			queue = append(queue, parentCanonical)
		}
	}
	return nil
}

func isClassFieldDeclaration(sym *Symbol) bool {
	return sym != nil && sym.Kind == SymbolField && sym.Owner != nil && sym.Owner.Kind == SymbolClass
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
		matched := false
		for _, param := range sym.Function.Doc.Params {
			if !strings.EqualFold(param.Name, sym.Name) {
				continue
			}
			if !param.NameSpan.IsEmpty() {
				result.addOccurrence(Occurrence{
					Span:       param.NameSpan,
					Text:       param.Name,
					Symbol:     sym,
					Scope:      sym.Scope,
					Annotation: true,
				})
			}
			if matched {
				continue
			}
			matched = true
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
		}
	}
}

func classTypeName(value string) string {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "|{}[]()") {
		return ""
	}
	for i, r := range value {
		if r == '_' || r == '.' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		if r == '<' {
			return ""
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
		case "@class":
			name, _, _, _, _ := parseClassHeader(text[len("@class"):])
			add(name)
		case "@alias":
			add(strings.TrimSuffix(fields[1], ":"))
		}
	}
	return out
}
