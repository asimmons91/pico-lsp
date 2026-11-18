package refactor

import (
	"fmt"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

type classField struct {
	Name string
	Type string
}

// GenerateClass inserts a contiguous `---@class` block above the declaration
// whose table literal contains offset. Only identifier-named fields are
// emitted; array and dynamic-key fields are left to explicit annotations.
func GenerateClass(ctx *Context, offset int) *Action {
	if !ctx.valid() {
		return nil
	}
	table := ctx.innermostTable(offset)
	if table == nil {
		return nil
	}
	stmt := ctx.enclosingStatement(table.Span().Start)
	if stmt == nil {
		return nil
	}

	var name string
	declStart := stmt.Span().Start
	switch st := stmt.(type) {
	case *syntax.LocalStmt:
		if len(st.Names) != 1 || len(st.Values) != 1 || st.Values[0] != table {
			return nil
		}
		name = st.Names[0].Text
	case *syntax.AssignStmt:
		if len(st.Targets) != 1 || len(st.Values) != 1 || st.Values[0] != table || st.Op.Text != "=" {
			return nil
		}
		target, ok := st.Targets[0].(*syntax.NameExpr)
		if !ok {
			return nil
		}
		name = target.Text
	default:
		return nil
	}
	if name == "" {
		return nil
	}
	if ctx.Result.Classes[analysis.CanonicalName(name)] != nil {
		return nil
	}

	fields := classFields(ctx, table)
	insertOffset, ok := ctx.statementLineStart(stmt)
	if !ok {
		return nil
	}
	if docStart := ctx.docBlockStart(declStart); docStart >= 0 && docStart < insertOffset {
		if strings.Contains(ctx.currentText()[docStart:declStart], "---@class") {
			return nil
		}
		insertOffset = docStart
	}
	indent := ctx.indentAt(insertOffset)

	var b strings.Builder
	b.WriteString(indent)
	b.WriteString("---@class ")
	b.WriteString(name)
	b.WriteString("\n")
	for _, field := range fields {
		b.WriteString(indent)
		b.WriteString("---@field ")
		b.WriteString(field.Name)
		b.WriteByte(' ')
		b.WriteString(field.Type)
		b.WriteString("\n")
	}
	return &Action{
		Title: fmt.Sprintf("Generate ---@class %s from this table", name),
		Kind:  KindGenerateClass,
		Edits: []Edit{{
			FileID:  ctx.FileID,
			Start:   insertOffset,
			End:     insertOffset,
			NewText: b.String(),
		}},
	}
}

func classFields(ctx *Context, table *syntax.TableExpr) []classField {
	if table == nil {
		return nil
	}
	var out []classField
	index := make(map[string]int)
	for _, field := range table.Fields {
		name := ""
		fieldType := "any"
		switch key := field.Key.(type) {
		case *syntax.NameExpr:
			if field.BracketKey {
				continue
			}
			name = key.Text
			if occ := ctx.Result.OccurrenceAt(ctx.FileID, key.Span().Start); occ != nil && occ.Symbol != nil {
				if occ.Symbol.Doc != nil && occ.Symbol.Doc.Type != "" {
					fieldType = annotationTypeText(occ.Symbol.Doc.ParsedType, occ.Symbol.Doc.Type)
				} else {
					fieldType = annotationTypeText(occ.Symbol.Type, "")
				}
			}
		case *syntax.StringExpr:
			// String keys do not case-fold. Represent a valid identifier as an
			// any-typed field rather than pretending the case is preserved.
			if field.BracketKey && validIdentifier(key.Value) {
				name = key.Value
			} else {
				continue
			}
		default:
			continue
		}
		if name == "" {
			continue
		}
		canonical := analysis.CanonicalName(name)
		if existing, ok := index[canonical]; ok {
			if out[existing].Type != fieldType {
				out[existing].Type = "any"
			}
			continue
		}
		index[canonical] = len(out)
		out = append(out, classField{Name: name, Type: fieldType})
	}
	return out
}

func annotationTypeText(t types.Type, annotation string) string {
	if annotation != "" {
		annotation = strings.TrimSpace(annotation)
		if annotation != "" && !strings.ContainsAny(annotation, "\r\n") {
			return annotation
		}
	}
	if t == nil || types.IsAny(t) {
		return "any"
	}
	text := strings.TrimSpace(types.Display(types.Widen(t)))
	if text == "" || strings.ContainsAny(text, "{}[]") {
		return "any"
	}
	switch text {
	case "any", "nil":
		return "any"
	}
	return text
}

func (c *Context) docBlockStart(offset int) int {
	if c == nil || c.Snap == nil {
		return -1
	}
	last := -1
	for i := len(c.Snap.Comments) - 1; i >= 0; i-- {
		comment := c.Snap.Comments[i]
		if comment.Span.FileID != c.FileID || comment.Kind != syntax.CommentDoc || comment.Span.End > offset {
			continue
		}
		last = i
		break
	}
	if last < 0 {
		return -1
	}
	first := last
	for first > 0 {
		previous := c.Snap.Comments[first-1]
		current := c.Snap.Comments[first]
		if previous.Span.FileID != c.FileID || previous.Kind != syntax.CommentDoc {
			break
		}
		if !c.commentsAdjacent(previous.Span, current.Span) {
			break
		}
		first--
	}
	if !c.commentAdjacentToOffset(c.Snap.Comments[last].Span, offset) {
		return -1
	}
	return c.lineStart(c.Snap.Comments[first].Span.Start)
}

func (c *Context) commentsAdjacent(previous, current source.Span) bool {
	text := c.currentText()
	if previous.End < 0 || current.Start > len(text) || previous.End > current.Start {
		return false
	}
	gap := text[previous.End:current.Start]
	return strings.TrimSpace(gap) == "" && strings.Count(gap, "\n") == 1
}

func (c *Context) commentAdjacentToOffset(comment source.Span, offset int) bool {
	text := c.currentText()
	if comment.End < 0 || offset > len(text) || comment.End > offset {
		return false
	}
	gap := text[comment.End:offset]
	return strings.TrimSpace(gap) == "" && strings.Count(gap, "\n") == 1
}
