package analysis

import "github.com/asimmons91/pico-lsp/internal/source"

// Severity is the configured severity for an analysis diagnostic.
type Severity int

const (
	SeverityOff Severity = iota
	SeverityHint
	SeverityInfo
	SeverityWarning
	SeverityError
)

// Diagnostic codes owned by the analysis layer.
const (
	CodeUndefinedGlobal = "undefined-global"
	CodeUnknownLabel    = "unknown-label"
	CodeDuplicateLabel  = "duplicate-label"
)

// Type-related diagnostic codes.
const (
	CodeAnnotationInvalid      = "annotation-invalid"
	CodeAnnotationUnknownType  = "annotation-unknown-type"
	CodeAnnotationDuplicate    = "annotation-duplicate"
	CodeAnnotationUnknownParam = "annotation-unknown-param"
	CodeAnnotationMissingName  = "annotation-missing-name"

	CodeAssignTypeMismatch = "assign-type-mismatch"
	CodeParamTypeMismatch  = "param-type-mismatch"
	CodeReturnTypeMismatch = "return-type-mismatch"
	CodeUndefinedField     = "undefined-field"
	CodeNeedCheckNil       = "need-check-nil"
	CodeMissingFields      = "missing-fields"
	CodeOperatorMismatch   = "operator-type-mismatch"
	CodeDeprecated         = "deprecated"
	CodeTypeUnknown        = "type-unknown"
)

// PICO-8 lint diagnostic codes.
const (
	CodeCallbackPrecedence  = "pico8-callback-precedence"
	CodeCallbackParams      = "pico8-callback-params"
	CodeNoStdlib            = "pico8-no-stdlib"
	CodeDuplicateTableField = "duplicate-table-field"
	CodeUnreachableCode     = "unreachable-code"
	CodeUnusedLocal         = "unused-local"
	CodeShadowedLocal       = "shadowed-local"
	CodeEmptyBlock          = "empty-block"
	CodeGlobalInFunction    = "global-in-function"
)

// Diagnostic is one analysis diagnostic before LSP conversion.
type Diagnostic struct {
	Code     string
	Message  string
	Span     source.Span
	Severity Severity
}
