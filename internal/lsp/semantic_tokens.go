package lsp

import (
	"context"
	"sort"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

var semanticTokenTypes = []string{
	"namespace", "type", "class", "enum", "interface", "struct", "typeParameter",
	"parameter", "variable", "property", "enumMember", "event", "function", "method",
	"macro", "keyword", "modifier", "comment", "string", "number", "regexp", "operator",
}

var semanticTokenModifiers = []string{
	"declaration", "definition", "readonly", "static", "deprecated", "defaultLibrary",
	"documentation",
}

const maxSemanticTokens = 20000

const (
	stNamespace = iota
	stType
	stClass
	stEnum
	stInterface
	stStruct
	stTypeParameter
	stParameter
	stVariable
	stProperty
	stEnumMember
	stEvent
	stFunction
	stMethod
	stMacro
	stKeyword
	stModifier
	stComment
	stString
	stNumber
	stRegexp
	stOperator
)

const (
	smDeclaration = 1 << iota
	smDefinition
	smReadonly
	smStatic
	smDeprecated
	smDefaultLibrary
	smDocumentation
)

type semanticToken struct {
	line   uint32
	start  uint32
	length uint32
	kind   uint32
	mods   uint32
}

func semanticTokensOptions() *protocol.SemanticTokensOptions {
	return &protocol.SemanticTokensOptions{
		Legend: protocol.SemanticTokensLegend{
			TokenTypes:     append([]string(nil), semanticTokenTypes...),
			TokenModifiers: append([]string(nil), semanticTokenModifiers...),
		},
		Full: protocol.Boolean(true),
	}
}

// SemanticTokensFull implements textDocument/semanticTokens/full.
func (s *Server) SemanticTokensFull(ctx context.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	if params == nil {
		return &protocol.SemanticTokens{Data: []uint32{}}, nil
	}
	nav, ok := s.navigationFor(ctx, params.TextDocument.URI)
	if !ok {
		return &protocol.SemanticTokens{Data: []uint32{}}, nil
	}
	data, truncated := semanticTokensData(nav)
	if truncated {
		s.log.Warn("truncating semantic tokens", "uri", nav.file.URI.String(), "max", maxSemanticTokens)
	}
	return &protocol.SemanticTokens{Data: data}, nil
}

func semanticTokensData(nav *navContext) ([]uint32, bool) {
	if nav == nil || nav.file == nil || nav.snap == nil {
		return []uint32{}, false
	}
	tokens := make([]semanticToken, 0)
	for _, token := range nav.snap.Tokens {
		if token.Span.FileID != nav.fileID || token.Span.IsEmpty() {
			continue
		}
		kind, mods := uint32(stVariable), uint32(0)
		switch token.Kind {
		case syntax.TokenKeyword:
			kind = stKeyword
		case syntax.TokenString:
			kind = stString
		case syntax.TokenNumber:
			kind = stNumber
		case syntax.TokenPunct:
			kind = stOperator
		case syntax.TokenInclude:
			// Root #include directives are consumed by the flattener, so the
			// authoritative source is Cart.Includes below. Nested directives
			// that survive into the flattened text are emitted there too.
			continue
		case syntax.TokenName:
			if occ := nav.semantics.OccurrenceAt(nav.fileID, token.Span.Start); occ != nil && occ.Symbol != nil {
				kind, mods = semanticSymbolKind(occ.Symbol, occ.Decl)
			}
		default:
			continue
		}
		tokens = append(tokens, spansToSemanticTokens(nav, token.Span, kind, mods)...)
	}
	for _, comment := range nav.snap.Comments {
		if comment.Span.FileID != nav.fileID || comment.Span.IsEmpty() {
			continue
		}
		tokens = append(tokens, spansToSemanticTokens(nav, comment.Span, stComment, 0)...)
	}
	for _, include := range nav.snap.Cart.Includes {
		if include.DirectiveSpan.FileID != nav.fileID {
			continue
		}
		if span, ok := includePathSpan(nav.file.Text, include.DirectiveSpan); ok {
			tokens = append(tokens, spansToSemanticTokens(nav, span, stString, smDocumentation)...)
		}
	}

	sort.SliceStable(tokens, func(i, j int) bool {
		if tokens[i].line != tokens[j].line {
			return tokens[i].line < tokens[j].line
		}
		if tokens[i].start != tokens[j].start {
			return tokens[i].start < tokens[j].start
		}
		return tokens[i].length < tokens[j].length
	})
	truncated := false
	if len(tokens) > maxSemanticTokens {
		tokens = tokens[:maxSemanticTokens]
		truncated = true
	}

	data := make([]uint32, 0, len(tokens)*5)
	prevLine, prevStart, prevLength := uint32(0), uint32(0), uint32(0)
	for i, token := range tokens {
		if i > 0 {
			if token.line < prevLine || (token.line == prevLine && (token.start < prevStart || token.start < prevStart+prevLength)) {
				continue
			}
		}
		deltaLine := token.line - prevLine
		deltaStart := token.start
		if deltaLine == 0 {
			deltaStart = token.start - prevStart
		}
		data = append(data, deltaLine, deltaStart, token.length, token.kind, token.mods)
		prevLine, prevStart, prevLength = token.line, token.start, token.length
	}
	return data, truncated
}

func semanticSymbolKind(sym *analysis.Symbol, declaration bool) (uint32, uint32) {
	kind := uint32(stVariable)
	mods := uint32(0)
	if declaration {
		mods |= smDeclaration
	}
	if sym.Builtin {
		mods |= smDefaultLibrary
		if strings.HasPrefix(sym.Signature, "function ") {
			kind = stFunction
		}
	} else {
		switch sym.Kind {
		case analysis.SymbolParam:
			kind = stParameter
		case analysis.SymbolFunction:
			kind = stFunction
		case analysis.SymbolMethod:
			kind = stMethod
		case analysis.SymbolField:
			kind = stProperty
		case analysis.SymbolLabel:
			kind = stNamespace
		case analysis.SymbolClass:
			kind = stClass
		case analysis.SymbolGlobal:
			if strings.HasPrefix(sym.Signature, "function ") {
				kind = stFunction
			}
		}
	}
	if sym.Doc != nil && sym.Doc.Deprecated {
		mods |= smDeprecated
	}
	return kind, mods
}

// includePathSpan narrows a `#include "path"` directive span to the path text
// so only the path is highlighted as a documented string.
func includePathSpan(text string, directive source.Span) (source.Span, bool) {
	if directive.IsEmpty() || directive.Start < 0 || directive.End > len(text) {
		return source.Span{}, false
	}
	i := directive.Start
	for i < directive.End && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	if !strings.HasPrefix(text[i:directive.End], "#include") {
		return source.Span{}, false
	}
	i += len("#include")
	for i < directive.End && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	quoted := i < directive.End && (text[i] == '"' || text[i] == '\'')
	var quote byte
	if quoted {
		quote = text[i]
		i++
	}
	start := i
	for i < directive.End && text[i] != '\r' && text[i] != '\n' {
		if quoted && text[i] == quote {
			break
		}
		i++
	}
	if i <= start {
		return source.Span{}, false
	}
	return source.Span{FileID: directive.FileID, Start: start, End: i}, true
}

func spansToSemanticTokens(nav *navContext, span source.Span, kind, mods uint32) []semanticToken {
	if nav == nil || nav.file == nil || nav.file.Index == nil || span.IsEmpty() {
		return nil
	}
	index := nav.file.Index
	text := nav.file.Text
	start, err := index.Position(span.Start)
	if err != nil {
		return nil
	}
	end, err := index.Position(span.End)
	if err != nil {
		return nil
	}
	out := make([]semanticToken, 0, end.Line-start.Line+1)
	for line := start.Line; line <= end.Line; line++ {
		lineStart, err := index.Offset(protocol.Position{Line: line, Character: 0})
		if err != nil {
			continue
		}
		lineEnd, err := index.Offset(protocol.Position{Line: line, Character: ^uint32(0)})
		if err != nil || lineEnd < lineStart {
			continue
		}
		segStart := span.Start
		if segStart < lineStart {
			segStart = lineStart
		}
		segEnd := span.End
		if segEnd > lineEnd {
			segEnd = lineEnd
		}
		if segStart < 0 || segEnd > len(text) || segEnd <= segStart {
			continue
		}
		startPos, err := index.Position(segStart)
		if err != nil {
			continue
		}
		endPos, err := index.Position(segEnd)
		if err != nil || endPos.Character <= startPos.Character {
			continue
		}
		out = append(out, semanticToken{
			line:   startPos.Line,
			start:  startPos.Character,
			length: endPos.Character - startPos.Character,
			kind:   kind,
			mods:   mods,
		})
	}
	return out
}
