package refactor

import (
	"fmt"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// AddMissingFields inserts required exact-class fields into the table literal
// flagged by a missing-fields diagnostic. Open classes never produce this fix
// because the diagnostic is only emitted for exact classes (or strict mode).
func AddMissingFields(ctx *Context, span source.Span, fields []types.FieldInfo) *Action {
	if !ctx.valid() || span.FileID != ctx.FileID || len(fields) == 0 {
		return nil
	}
	table := ctx.tableAtSpan(span)
	if table == nil {
		return nil
	}
	identifiers, stringKeys := tableFieldNames(table)
	var missing []types.FieldInfo
	var names []string
	for _, field := range fields {
		if field.Name == "" || !validIdentifier(field.Name) {
			// A partial edit would leave the missing-fields diagnostic
			// unresolved, so refuse the quickfix entirely.
			return nil
		}
		canonical := analysis.CanonicalName(field.Name)
		if identifiers[canonical] || stringKeys[field.Name] {
			continue
		}
		identifiers[canonical] = true
		missing = append(missing, field)
		names = append(names, field.Name)
	}
	if len(missing) == 0 {
		return nil
	}

	text := ctx.currentText()
	openLine := ctx.lineStart(table.Open.Start)
	closeLine := ctx.lineStart(table.Close.Start)
	if openLine == closeLine && ctx.hasComment(table.Open.Start, table.Close.End) {
		return nil
	}
	if openLine == closeLine {
		var parts []string
		for _, field := range missing {
			parts = append(parts, field.Name+" = "+placeholderFor(field.Type))
		}
		insert := strings.Join(parts, ", ")
		if len(table.Fields) > 0 {
			body := strings.TrimSpace(text[table.Open.End:table.Close.Start])
			if !strings.HasSuffix(body, ",") && !strings.HasSuffix(body, ";") {
				insert = ", " + insert
			}
		}
		return &Action{
			Title: "Add missing fields: " + strings.Join(names, ", "),
			Kind:  KindAddMissingFields,
			Edits: []Edit{{FileID: ctx.FileID, Start: table.Close.Start, End: table.Close.Start, NewText: insert}},
		}
	}

	fieldIndent := ctx.indentAt(table.Open.Start) + ctx.indentUnit()
	var b strings.Builder
	for _, field := range missing {
		b.WriteString(fieldIndent)
		b.WriteString(field.Name)
		b.WriteString(" = ")
		b.WriteString(placeholderFor(field.Type))
		b.WriteString(",\n")
	}
	insertOffset := ctx.lineStart(table.Close.Start)
	return &Action{
		Title: "Add missing fields: " + strings.Join(names, ", "),
		Kind:  KindAddMissingFields,
		Edits: []Edit{{
			FileID:  ctx.FileID,
			Start:   insertOffset,
			End:     insertOffset,
			NewText: b.String(),
		}},
	}
}

func (c *Context) tableAtSpan(span source.Span) *syntax.TableExpr {
	for _, node := range c.nodes() {
		table, ok := node.(*syntax.TableExpr)
		if !ok || table == nil {
			continue
		}
		tableSpan := table.Span()
		if tableSpan == span {
			return table
		}
	}
	// The diagnostic span is expression-based, but tolerate a selection that
	// only covers the interior of the literal.
	for _, node := range c.nodes() {
		table, ok := node.(*syntax.TableExpr)
		if !ok || table == nil {
			continue
		}
		tableSpan := table.Span()
		if tableSpan.FileID == span.FileID && tableSpan.Start <= span.Start && tableSpan.End >= span.End {
			return table
		}
	}
	return nil
}

func tableFieldNames(table *syntax.TableExpr) (identifiers, stringKeys map[string]bool) {
	identifiers = make(map[string]bool)
	stringKeys = make(map[string]bool)
	if table == nil {
		return identifiers, stringKeys
	}
	for _, field := range table.Fields {
		switch key := field.Key.(type) {
		case *syntax.NameExpr:
			if !field.BracketKey {
				identifiers[analysis.CanonicalName(key.Text)] = true
			}
		case *syntax.StringExpr:
			if field.BracketKey {
				stringKeys[key.Value] = true
			}
		}
	}
	return identifiers, stringKeys
}

func placeholderFor(t types.Type) string {
	if t == nil {
		return "nil"
	}
	t = types.Resolve(t)
	switch v := t.(type) {
	case types.Primitive:
		switch v.Kind {
		case types.KindBoolean:
			return "false"
		case types.KindNumber, types.KindInteger:
			return "0"
		case types.KindString:
			return `""`
		case types.KindTable:
			return "{}"
		case types.KindFunction:
			return "function() end"
		default:
			return "nil"
		}
	case types.Union:
		for _, member := range v.Members {
			if types.IsNil(member) {
				return "nil"
			}
		}
		if len(v.Members) > 0 {
			return placeholderFor(v.Members[0])
		}
		return "nil"
	case types.Array, types.Map, *types.Record:
		return "{}"
	case types.Literal:
		switch value := v.Value.(type) {
		case string:
			return fmt.Sprintf("%q", value)
		case bool:
			if value {
				return "true"
			}
			return "false"
		default:
			return fmt.Sprint(value)
		}
	default:
		return "nil"
	}
}
