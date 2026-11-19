package analysis

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// AnnotationRole classifies one highlightable part of a LuaCATS doc comment.
type AnnotationRole uint8

const (
	// AnnotationTag is a tag name such as `@class` or `@param`.
	AnnotationTag AnnotationRole = iota
	// AnnotationClassName is the name declared by `@class`.
	AnnotationClassName
	// AnnotationEnumName is the name declared by `@enum`.
	AnnotationEnumName
	// AnnotationTypeName is the name declared by `@alias`.
	AnnotationTypeName
	// AnnotationFieldName is the field name in `@field`.
	AnnotationFieldName
	// AnnotationParamName is the parameter name in `@param`.
	AnnotationParamName
	// AnnotationGenericName is a generic parameter name in `@generic`.
	AnnotationGenericName
	// AnnotationOperatorName is the operator name in `@operator`.
	AnnotationOperatorName
	// AnnotationType is an identifier or keyword inside a type expression.
	AnnotationType
	// AnnotationString is a string literal inside a type expression.
	AnnotationString
	// AnnotationNumber is a number literal inside a type expression.
	AnnotationNumber
)

// AnnotationSegment is one role-tagged byte range inside a doc comment. Spans
// are in original-file coordinates and never overlap.
type AnnotationSegment struct {
	Span source.Span
	Role AnnotationRole
}

// AnnotationSegments splits a `---@` doc comment into semantic-highlight
// segments. The `---` prefix, whitespace, and trailing prose keep their normal
// comment styling because they are not returned. Unknown tags still produce a
// tag segment; their arguments are left as comments.
func AnnotationSegments(comment syntax.Comment) []AnnotationSegment {
	if comment.Kind != syntax.CommentDoc || comment.Span.IsEmpty() {
		return nil
	}
	text := comment.Text
	if !strings.HasPrefix(text, "---") {
		return nil
	}
	i := len("---")
	for i < len(text) && isAnnotationSpace(text[i]) {
		i++
	}
	if i >= len(text) || text[i] != '@' {
		return nil
	}
	tagStart := i
	i++
	nameStart := i
	for i < len(text) && isAnnotationNameStart(text[i]) {
		i++
	}
	if i == nameStart {
		return nil
	}
	tag := strings.ToLower(text[nameStart:i])

	var segments []AnnotationSegment
	add := func(role AnnotationRole, start, end int) {
		start, end = clampAnnotationRange(text, start, end)
		if end <= start {
			return
		}
		segments = append(segments, AnnotationSegment{
			Span: source.Span{
				FileID: comment.Span.FileID,
				Start:  comment.Span.Start + start,
				End:    comment.Span.Start + end,
			},
			Role: role,
		})
	}
	add(AnnotationTag, tagStart, i)

	rest := i
	for rest < len(text) && isAnnotationSpace(text[rest]) {
		rest++
	}

	switch tag {
	case "class":
		nameStart, nameEnd := nextAnnotationWord(text, rest)
		if nameStart < nameEnd && strings.EqualFold(text[nameStart:nameEnd], "(exact)") {
			nameStart, nameEnd = nextAnnotationWord(text, nameEnd)
		}
		if nameStart < nameEnd {
			end := annotationNameEnd(text, nameStart, nameEnd)
			add(AnnotationClassName, nameStart, end)
			addClassAnnotationTypes(add, text, end)
		}
	case "enum":
		nameStart, nameEnd := nextAnnotationWord(text, rest)
		if nameStart < nameEnd {
			add(AnnotationEnumName, nameStart, annotationNameEnd(text, nameStart, nameEnd))
		}
	case "alias":
		nameStart, nameEnd := nextAnnotationWord(text, rest)
		if nameStart < nameEnd {
			nameEnd = annotationNameEnd(text, nameStart, nameEnd)
			add(AnnotationTypeName, nameStart, nameEnd)
			addAnnotationTypes(add, text, nameEnd, len(text))
		}
	case "field":
		nameStart := rest
		if nameStart < len(text) && text[nameStart] == '[' {
			if close := strings.IndexByte(text[nameStart:], ']'); close >= 0 {
				close += nameStart
				addAnnotationTypes(add, text, nameStart+1, close)
				nameStart = close + 1
			}
		}
		nameStart, nameEnd := nextAnnotationWord(text, nameStart)
		if nameStart < nameEnd {
			add(AnnotationFieldName, nameStart, nameEnd)
			addAnnotationTypes(add, text, nameEnd, len(text))
		}
	case "param":
		nameStart, nameEnd := nextAnnotationWord(text, rest)
		if nameStart < nameEnd {
			add(AnnotationParamName, nameStart, nameEnd)
			addAnnotationTypes(add, text, nameEnd, len(text))
		}
	case "type", "return", "cast", "overload":
		addAnnotationTypes(add, text, rest, len(text))
	case "generic":
		addGenericAnnotationTypes(add, text, rest, len(text))
	case "operator":
		nameStart, nameEnd := nextAnnotationWord(text, rest)
		if nameStart < nameEnd {
			opEnd := annotationNameEnd(text, nameStart, nameEnd)
			add(AnnotationOperatorName, nameStart, opEnd)
			i := opEnd
			for i < len(text) && isAnnotationSpace(text[i]) {
				i++
			}
			if i < len(text) && text[i] == '(' {
				if close := strings.IndexByte(text[i:], ')'); close >= 0 {
					close += i
					addAnnotationTypes(add, text, i+1, close)
					i = close + 1
				}
			}
			for i < len(text) && isAnnotationSpace(text[i]) {
				i++
			}
			if i < len(text) && text[i] == ':' {
				i++
			}
			if i < len(text) {
				addAnnotationTypes(add, text, i, len(text))
			}
		}
	}
	return segments
}

func clampAnnotationRange(text string, start, end int) (int, int) {
	if start < 0 {
		start = 0
	}
	if end > len(text) {
		end = len(text)
	}
	if start > end {
		start = end
	}
	return start, end
}

func isAnnotationSpace(c byte) bool {
	return c == ' ' || c == '\t'
}

func isAnnotationNameStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func nextAnnotationWord(text string, i int) (start, end int) {
	for i < len(text) && isAnnotationSpace(text[i]) {
		i++
	}
	start = i
	for i < len(text) && !isAnnotationSpace(text[i]) {
		i++
	}
	return start, i
}

// annotationNameEnd trims declaration suffixes such as `?`, `:` and `<...>`
// from a word so the name itself is highlighted independently.
func annotationNameEnd(text string, start, end int) int {
	i := start
	for i < end {
		c := text[i]
		if isAnnotationNameStart(c) || c == '.' {
			i++
			continue
		}
		break
	}
	if i == start {
		return end
	}
	return i
}

func addClassAnnotationTypes(add func(AnnotationRole, int, int), text string, start int) {
	for start < len(text) && isAnnotationSpace(text[start]) {
		start++
	}
	if strings.HasPrefix(text[start:], "(exact)") {
		start += len("(exact)")
		for start < len(text) && isAnnotationSpace(text[start]) {
			start++
		}
	}
	if start < len(text) && text[start] == '<' {
		if close := strings.IndexByte(text[start:], '>'); close >= 0 {
			close += start
			addGenericAnnotationTypes(add, text, start+1, close)
			start = close + 1
		}
	}
	for start < len(text) && isAnnotationSpace(text[start]) {
		start++
	}
	if start >= len(text) || text[start] != ':' {
		return
	}
	start++
	// Parent classes are type expressions. Split top-level commas so each
	// parent is parsed on its own.
	for start < len(text) {
		for start < len(text) && (isAnnotationSpace(text[start]) || text[start] == ',') {
			start++
		}
		if start >= len(text) || text[start] == ')' {
			return
		}
		end := annotationTypeEnd(text, start, len(text))
		if end <= start {
			return
		}
		addAnnotationTypeTokens(add, text, start, end)
		start = end
		for start < len(text) && isAnnotationSpace(text[start]) {
			start++
		}
		if start < len(text) && text[start] == ',' {
			start++
			continue
		}
		return
	}
}

func addAnnotationTypes(add func(AnnotationRole, int, int), text string, start, end int) {
	start, end = clampAnnotationRange(text, start, end)
	for start < end && isAnnotationSpace(text[start]) {
		start++
	}
	if start >= end {
		return
	}
	typeEnd := annotationTypeEnd(text, start, end)
	if typeEnd <= start {
		return
	}
	addAnnotationTypeTokens(add, text, start, typeEnd)
}

// annotationTypeEnd returns the end of the leading type expression. It uses the
// annotation type parser with permissive name resolution because highlighting
// must work before a full analysis pass has resolved classes.
func annotationTypeEnd(text string, start, limit int) int {
	if start < 0 || start >= limit || limit > len(text) {
		return start
	}
	_, end, err := parseTypePrefix(text[start:limit], func(string) (types.Type, bool) {
		return types.AnyType, true
	})
	if err != nil || end <= 0 {
		return start
	}
	end += start
	for end > start && isAnnotationSpace(text[end-1]) {
		end--
	}
	return end
}

func addAnnotationTypeTokens(add func(AnnotationRole, int, int), text string, start, end int) {
	start, end = clampAnnotationRange(text, start, end)
	for i := start; i < end; {
		switch {
		case text[i] == '"' || text[i] == '\'':
			next := scanAnnotationString(text, i, end)
			add(AnnotationString, i, next)
			i = next
		case text[i] == '(' && strings.HasPrefix(text[i:end], "(exact)"):
			i += len("(exact)")
		case isAnnotationNumberStart(text[i]):
			next := scanAnnotationNumber(text, i, end)
			if next <= i {
				i++
				continue
			}
			add(AnnotationNumber, i, next)
			i = next
		case isAnnotationIdentifierStart(text, i):
			next := scanAnnotationIdentifier(text, i, end)
			add(AnnotationType, i, next)
			i = next
		default:
			i++
		}
	}
}

func addGenericAnnotationTypes(add func(AnnotationRole, int, int), text string, start, end int) {
	i := start
	for i < end {
		for i < end && (isAnnotationSpace(text[i]) || text[i] == ',') {
			i++
		}
		if i >= end {
			return
		}
		nameStart := i
		if !isAnnotationIdentifierStart(text, i) {
			return
		}
		i = scanAnnotationIdentifier(text, i, end)
		add(AnnotationGenericName, nameStart, i)
		for i < end && isAnnotationSpace(text[i]) {
			i++
		}
		if i < end && text[i] == ':' {
			i++
			typeStart := i
			typeEnd := annotationTypeEnd(text, typeStart, end)
			addAnnotationTypeTokens(add, text, typeStart, typeEnd)
			i = typeEnd
			for i < end && text[i] != ',' {
				i++
			}
		}
	}
}

func scanAnnotationString(text string, start, end int) int {
	quote := text[start]
	i := start + 1
	for i < end {
		if text[i] == '\\' {
			i += 2
			continue
		}
		if text[i] == quote {
			return i + 1
		}
		i++
	}
	return end
}

func isAnnotationNumberStart(c byte) bool {
	return c == '-' || c == '+' || (c >= '0' && c <= '9')
}

func scanAnnotationNumber(text string, start, end int) int {
	i := start
	if i < end && (text[i] == '-' || text[i] == '+') {
		i++
	}
	if i+1 < end && text[i] == '0' && (text[i+1] == 'x' || text[i+1] == 'X') {
		i += 2
		for i < end && isHexDigit(text[i]) {
			i++
		}
		return i
	}
	dot := false
	for i < end {
		c := text[i]
		if c >= '0' && c <= '9' {
			i++
			continue
		}
		if c == '.' && !dot && i+1 < end && text[i+1] >= '0' && text[i+1] <= '9' {
			dot = true
			i++
			continue
		}
		break
	}
	return i
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isAnnotationIdentifierStart(text string, i int) bool {
	if i < 0 || i >= len(text) {
		return false
	}
	c := text[i]
	if c < utf8.RuneSelf {
		return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
	}
	r, _ := utf8.DecodeRuneInString(text[i:])
	return r != utf8.RuneError && (unicode.IsLetter(r) || r == '_')
}

func scanAnnotationIdentifier(text string, start, end int) int {
	i := start
	for i < end {
		c := text[i]
		if c < utf8.RuneSelf {
			if c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' {
				i++
				continue
			}
			break
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError || !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			break
		}
		i += size
	}
	return i
}
