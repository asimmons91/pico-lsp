package analysis

import (
	"strconv"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

func tagFieldComparison(left, right syntax.Expr) (*syntax.FieldExpr, any, bool) {
	if field, value, ok := fieldAndLiteral(left, right); ok {
		return field, value, true
	}
	if field, value, ok := fieldAndLiteral(right, left); ok {
		return field, value, true
	}
	return nil, nil, false
}

func fieldAndLiteral(fieldExpr, literalExpr syntax.Expr) (*syntax.FieldExpr, any, bool) {
	field, ok := fieldExpr.(*syntax.FieldExpr)
	if !ok || field == nil || field.Name == nil {
		return nil, nil, false
	}
	if _, ok := field.Base.(*syntax.NameExpr); !ok {
		return nil, nil, false
	}
	switch literal := literalExpr.(type) {
	case *syntax.StringExpr:
		return field, literal.Value, true
	case *syntax.NumberExpr:
		text := literal.Text
		if strings.HasPrefix(strings.ToLower(text), "0x") || strings.HasPrefix(strings.ToLower(text), "-0x") {
			sign := int64(1)
			if text[0] == '-' {
				sign = -1
				text = text[1:]
			}
			value, err := strconv.ParseInt(strings.TrimPrefix(strings.ToLower(text), "0x"), 16, 64)
			if err != nil {
				return nil, nil, false
			}
			return field, sign * value, true
		}
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, nil, false
		}
		return field, value, true
	}
	return nil, nil, false
}

func (inf *inferer) invalidateFieldBase(base syntax.Expr, env map[*Symbol]types.Type) {
	for base != nil {
		switch expr := base.(type) {
		case *syntax.NameExpr:
			if sym := inf.symbolAt(expr); sym != nil {
				delete(env, sym)
			}
			return
		case *syntax.FieldExpr:
			base = expr.Base
		case *syntax.ParenExpr:
			base = expr.X
		default:
			return
		}
	}
}
