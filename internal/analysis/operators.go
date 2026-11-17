package analysis

import (
	"fmt"

	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/source"
)

var binaryOperatorNames = map[string]string{
	"+": "add", "-": "sub", "*": "mul", "/": "div", "\\": "idiv",
	"%": "mod", "^": "pow", "..": "concat",
	"==": "eq", "~=": "eq", "!=": "eq",
	"<": "lt", "<=": "le", ">": "lt", ">=": "le",
	"&": "band", "|": "bor", "^^": "bxor",
	"<<": "shl", ">>": "shr", ">>>": "shr",
	"<<>": "rotl", ">><": "rotr",
}

var unaryOperatorNames = map[string]string{
	"-": "unm",
	"~": "bnot",
	"#": "len",
}

func (inf *inferer) operatorResult(op string, left, right types.Type, leftSpan, rightSpan source.Span) (types.Type, bool) {
	name, ok := binaryOperatorNames[op]
	if !ok || types.IsAny(left) && types.IsAny(right) {
		return nil, false
	}
	if !operatorReceiver(left) && !operatorReceiver(right) {
		return nil, false
	}
	swap := op == ">" || op == ">="
	primary, secondary := left, right
	primarySpan, secondarySpan := leftSpan, rightSpan
	if swap {
		primary, secondary = right, left
		primarySpan, secondarySpan = rightSpan, leftSpan
	}
	if typ, ok := inf.operatorType(name, primary, secondary, primarySpan, secondarySpan); ok {
		return typ, true
	}
	if typ, ok := inf.operatorType(name, secondary, primary, secondarySpan, primarySpan); ok {
		return typ, true
	}
	return nil, false
}

func (inf *inferer) operatorType(name string, handler, other types.Type, handlerSpan, otherSpan source.Span) (types.Type, bool) {
	op, ok := types.LookupOperator(handler, name)
	if !ok || op == nil {
		return nil, false
	}
	if op.Operand != nil && !types.IsAny(other) && !types.Assignable(other, op.Operand, false) {
		inf.reportHint(CodeOperatorMismatch,
			fmt.Sprintf("operator %s expects %s, got %s", name, types.Display(op.Operand), types.Display(other)),
			otherSpan)
	}
	if op.Result != nil {
		return op.Result, true
	}
	return nil, false
}

func (inf *inferer) unaryOperatorResult(op string, operand types.Type, span source.Span) (types.Type, bool) {
	name, ok := unaryOperatorNames[op]
	if !ok || !operatorReceiver(operand) {
		return nil, false
	}
	annotation, ok := types.LookupOperator(operand, name)
	if !ok || annotation == nil {
		return nil, false
	}
	if annotation.Operand != nil && !types.IsAny(operand) && !types.Assignable(operand, annotation.Operand, false) {
		inf.reportHint(CodeOperatorMismatch,
			fmt.Sprintf("operator %s expects %s, got %s", name, types.Display(annotation.Operand), types.Display(operand)),
			span)
	}
	if annotation.Result != nil {
		return annotation.Result, true
	}
	return nil, false
}

func operatorReceiver(t types.Type) bool {
	switch types.Resolve(t).(type) {
	case *types.Record, types.Record:
		return true
	}
	return false
}

func (inf *inferer) reportHint(code, message string, span source.Span) {
	if span.IsEmpty() {
		return
	}
	inf.result.addTypeDiagnostic(inf.mode, code, message, span, SeverityHint)
}
