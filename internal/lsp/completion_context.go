package lsp

import (
	"strings"
	"unicode/utf8"

	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

func (n *navContext) linePrefix(offset int) (int, string) {
	if n == nil || n.file == nil || n.file.Index == nil {
		return 0, ""
	}
	pos, err := n.file.Index.Position(offset)
	if err != nil {
		return offset, ""
	}
	lineStart, err := n.file.Index.Offset(protocol.Position{Line: pos.Line, Character: 0})
	if err != nil || lineStart < 0 || lineStart > offset {
		return offset, ""
	}
	return lineStart, n.file.Text[lineStart:offset]
}

func (n *navContext) tokenAt(offset int) *syntax.Token {
	if n == nil || n.snap == nil {
		return nil
	}
	for i := range n.snap.Tokens {
		token := &n.snap.Tokens[i]
		if token.Span.FileID != n.fileID {
			continue
		}
		if token.Span.Contains(offset) || (offset > 0 && token.Span.Contains(offset-1)) {
			return token
		}
	}
	return nil
}

func (n *navContext) inComment(offset int) bool {
	if n == nil || n.snap == nil {
		return false
	}
	for _, comment := range n.snap.Comments {
		if comment.Span.FileID != n.fileID {
			continue
		}
		if comment.Span.Contains(offset) || (offset > 0 && comment.Span.Contains(offset-1)) {
			return true
		}
	}
	return false
}

func (n *navContext) inString(offset int) bool {
	token := n.tokenAt(offset)
	return token != nil && token.Kind == syntax.TokenString
}

func identifierPrefixStart(text string, offset int) int {
	if offset > len(text) {
		offset = len(text)
	}
	start := offset
	for start > 0 {
		c := text[start-1]
		if c < utf8.RuneSelf {
			if !isIdentifierByte(c) {
				break
			}
			start--
			continue
		}
		// Keep non-ASCII glyphs together by scanning bytes until a non-ASCII
		// boundary. The lexer accepts any valid non-ASCII rune in an identifier.
		start--
		for start > 0 && !utf8.RuneStart(text[start]) {
			start--
		}
	}
	return start
}

func isIdentifierByte(c byte) bool {
	return c == '_' || c == 0x1e || c == 0x1f ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// glyphPrefix reports whether text immediately before offset is a P8SCII glyph
// trigger. Triggers are ASCII, so the scan stops at the first non-trigger byte.
func glyphPrefix(text string, offset int) (start int, prefix string, ok bool) {
	if offset > len(text) {
		offset = len(text)
	}
	start = offset
	for start > 0 {
		c := text[start-1]
		if c >= utf8.RuneSelf || !isGlyphTriggerByte(c) {
			break
		}
		start--
	}
	prefix = strings.ToLower(text[start:offset])
	if strings.HasPrefix(prefix, "p8_") && strings.Contains(prefix, "-") {
		// A hyphen after p8_ is subtraction, not part of a glyph trigger.
		return 0, "", false
	}
	if strings.HasPrefix(prefix, "p8_") || strings.HasPrefix(prefix, "p8-") {
		return start, prefix, true
	}
	return 0, "", false
}

func isGlyphTriggerByte(c byte) bool {
	return c == '_' || c == '-' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func precedingMemberSeparator(text string, prefixStart int) (int, bool, bool) {
	i := prefixStart - 1
	for i >= 0 && (text[i] == ' ' || text[i] == '\t') {
		i--
	}
	if i >= 0 && text[i] == '.' {
		return i, false, true
	}
	if i >= 0 && text[i] == ':' {
		return i, true, true
	}
	return 0, false, false
}

func (n *navContext) receiverBefore(separator int) *analysis.Symbol {
	if n == nil || n.semantics == nil || separator < 0 || separator > len(n.file.Text) {
		return nil
	}
	occs := n.semantics.Occurrences(n.fileID)
	for i := len(occs) - 1; i >= 0; i-- {
		occ := &occs[i]
		if occ.Span.Start >= separator || occ.Span.End > separator {
			continue
		}
		if !isOnlySpace(n.file.Text[occ.Span.End:separator]) {
			break
		}
		if occ.Symbol == nil {
			break
		}
		if typ, ok := n.semantics.FlowType(occ.Span); ok && typ != nil && !types.IsAny(typ) {
			receiver := *occ.Symbol
			receiver.Type = typ
			return resolveAlias(&receiver)
		}
		return resolveAlias(occ.Symbol)
	}
	// The parser drops statements that are not assignments or calls. While the
	// user is typing `self.` on its own line, there is no occurrence for the
	// receiver inside that statement. Resolve a simple identifier immediately
	// before the separator from the enclosing lexical scope instead.
	if name, ok := textReceiverName(n.file.Text, separator); ok {
		if scope := n.semantics.ScopeAt(n.fileID, separator); scope != nil {
			if sym := scope.Lookup(analysis.CanonicalName(name)); sym != nil {
				return resolveAlias(sym)
			}
		}
	}
	return nil
}

// textReceiverName returns the identifier directly before a member separator,
// rejecting dotted chains because the last field is not a scope name.
func textReceiverName(text string, separator int) (string, bool) {
	if separator <= 0 || separator > len(text) {
		return "", false
	}
	i := separator
	for i > 0 && (text[i-1] == ' ' || text[i-1] == '\t') {
		i--
	}
	end := i
	for i > 0 && isIdentifierByte(text[i-1]) {
		i--
	}
	if i == end {
		return "", false
	}
	if i > 0 && (text[i-1] == '.' || text[i-1] == ':') {
		return "", false
	}
	return text[i:end], true
}

func isOnlySpace(text string) bool {
	for _, r := range text {
		if r != ' ' && r != '\t' {
			return false
		}
	}
	return true
}

func resolveAlias(sym *analysis.Symbol) *analysis.Symbol {
	seen := make(map[*analysis.Symbol]struct{})
	for sym != nil && sym.Alias != nil {
		if _, ok := seen[sym]; ok {
			break
		}
		seen[sym] = struct{}{}
		sym = sym.Alias
	}
	return sym
}

func isIncludeLine(linePrefix string) bool {
	trimmed := strings.TrimLeft(linePrefix, " \t")
	return strings.HasPrefix(trimmed, "#include") &&
		(len(trimmed) == len("#include") || trimmed[len("#include")] == ' ' || trimmed[len("#include")] == '\t')
}

func (n *navContext) onIncludeLine(offset int) bool {
	if n == nil {
		return false
	}
	_, linePrefix := n.linePrefix(offset)
	return isIncludeLine(linePrefix)
}
