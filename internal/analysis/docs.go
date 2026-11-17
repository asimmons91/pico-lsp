package analysis

import (
	"strings"
	"unicode"

	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// ParamDoc is one LuaCATS parameter annotation.
type ParamDoc struct {
	Name     string
	Type     string
	Doc      string
	Optional bool
}

// DocComment is the subset of LuaCATS docs used by M3 hover and completion.
// Type semantics are intentionally shallow here; M4 owns type checking.
type DocComment struct {
	Summary    string
	Type       string
	Class      string
	Params     []ParamDoc
	Returns    []string
	Deprecated bool
}

// ClassField is one shallow `---@field` declaration.
type ClassField struct {
	Name     string
	Type     string
	Doc      string
	Optional bool
	Span     source.Span
}

// Class is the shallow class metadata M3 uses for member completion.
type Class struct {
	Name   string
	Fields []ClassField
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
		if len(fields) < 3 {
			return
		}
		name := strings.TrimSuffix(fields[1], "?")
		param := ParamDoc{
			Name:     name,
			Optional: strings.HasSuffix(fields[1], "?"),
			Type:     fields[2],
		}
		if len(fields) > 3 {
			param.Doc = strings.Join(fields[3:], " ")
		}
		doc.Params = append(doc.Params, param)
	case "@type":
		if len(fields) > 1 {
			doc.Type = strings.Join(fields[1:], " ")
		}
	case "@class":
		if len(fields) > 1 {
			doc.Class = strings.TrimSuffix(fields[1], ":")
		}
	case "@return":
		if len(fields) > 1 {
			doc.Returns = append(doc.Returns, strings.Join(fields[1:], " "))
		}
	case "@deprecated":
		doc.Deprecated = true
	}
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
		sym.Doc = ParseDocComments(docCommentsFor(file, snap.Comments, sym.DeclSpan.Start))
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
			name := strings.TrimSuffix(fields[1], ":")
			canonical := CanonicalName(name)
			current = classes[canonical]
			if current == nil {
				current = &Class{Name: name}
				classes[canonical] = current
			}
		case "@field":
			if current == nil || len(fields) < 3 {
				current = nil
				continue
			}
			field := ClassField{
				Name:     strings.TrimSuffix(fields[1], "?"),
				Optional: strings.HasSuffix(fields[1], "?"),
				Type:     fields[2],
				Span:     comment.Span,
			}
			if len(fields) > 3 {
				field.Doc = strings.Join(fields[3:], " ")
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
			if _, exists := sym.Fields[canonical]; exists {
				continue
			}
			fieldSym := result.Table.NewSymbol(SymbolField, field.Name, field.Span, nil)
			fieldSym.Owner = sym
			if field.Type != "" || field.Doc != "" {
				fieldSym.Doc = &DocComment{Type: field.Type, Summary: field.Doc}
			}
			sym.Fields[canonical] = fieldSym
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
	var docs []syntax.Comment
	for _, comment := range comments {
		if comment.Span.FileID != file.ID || comment.Kind != syntax.CommentDoc || comment.Span.End > offset {
			continue
		}
		docs = append(docs, comment)
	}
	if len(docs) == 0 {
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
	last := docs[len(docs)-1]
	if !onlyWhitespace(file.Text[last.Span.End:lineStart]) {
		return nil
	}
	block := []syntax.Comment{last}
	for i := len(docs) - 2; i >= 0; i-- {
		comment := docs[i]
		if !onlyWhitespace(file.Text[comment.Span.End:block[0].Span.Start]) {
			break
		}
		block = append([]syntax.Comment{comment}, block...)
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
			add(fields[1])
		}
	}
	return out
}
