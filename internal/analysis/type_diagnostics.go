package analysis

import (
	"sort"
	"strings"
)

func isTypeDiagnosticCode(code string) bool {
	switch code {
	case CodeAnnotationInvalid, CodeAnnotationUnknownType, CodeAnnotationDuplicate,
		CodeAnnotationUnknownParam, CodeAnnotationMissingName,
		CodeAssignTypeMismatch, CodeParamTypeMismatch, CodeReturnTypeMismatch,
		CodeUndefinedField, CodeNeedCheckNil, CodeMissingFields, CodeDeprecated,
		CodeTypeUnknown:
		return true
	default:
		return false
	}
}

func applyTypeDiagnosticSuppressions(result *Result, mode TypeCheckingMode) {
	if result == nil || mode == TypeCheckingOff || len(result.DiagnosticDirectives) == 0 {
		return
	}
	directives := append([]DiagnosticDirective(nil), result.DiagnosticDirectives...)
	sort.SliceStable(directives, func(i, j int) bool {
		if directives[i].FileID != directives[j].FileID {
			return directives[i].FileID < directives[j].FileID
		}
		if directives[i].Line != directives[j].Line {
			return directives[i].Line < directives[j].Line
		}
		return directives[i].Span.Start < directives[j].Span.Start
	})

	out := result.Diagnostics[:0]
	for _, diag := range result.Diagnostics {
		if isTypeDiagnosticCode(diag.Code) && suppressedByDirectives(result, directives, diag) {
			continue
		}
		out = append(out, diag)
	}
	result.Diagnostics = out
}

func suppressedByDirectives(result *Result, directives []DiagnosticDirective, diag Diagnostic) bool {
	line := result.diagLine(diag)
	if line < 0 {
		return false
	}
	disabledAll := false
	disabled := make(map[string]bool)
	for _, directive := range directives {
		if directive.FileID != diag.Span.FileID {
			continue
		}
		switch directive.Action {
		case "disable-line":
			if directive.Line == line && directiveMatchesCode(directive, diag.Code) {
				return true
			}
			continue
		case "disable-next-line":
			if directive.Line+1 == line && directiveMatchesCode(directive, diag.Code) {
				return true
			}
			continue
		}
		if directive.Line > line {
			break
		}
		switch directive.Action {
		case "disable":
			if len(directive.Codes) == 0 || containsCode(directive.Codes, "*") {
				disabledAll = true
				continue
			}
			for _, code := range directive.Codes {
				disabled[code] = true
			}
		case "enable":
			if len(directive.Codes) == 0 || containsCode(directive.Codes, "*") {
				disabledAll = false
				disabled = make(map[string]bool)
				continue
			}
			for _, code := range directive.Codes {
				delete(disabled, code)
			}
		}
	}
	return disabledAll || disabled[diag.Code]
}

func directiveMatchesCode(directive DiagnosticDirective, code string) bool {
	if len(directive.Codes) == 0 || containsCode(directive.Codes, "*") {
		return true
	}
	return containsCode(directive.Codes, code)
}

func containsCode(codes []string, code string) bool {
	for _, candidate := range codes {
		if strings.EqualFold(candidate, code) {
			return true
		}
	}
	return false
}

func (r *Result) diagLine(diag Diagnostic) int {
	if r == nil {
		return -1
	}
	if line, ok := r.diagLines[diag]; ok {
		return line
	}
	return -1
}
