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
	CodeDeprecated         = "deprecated"
	CodeTypeUnknown        = "type-unknown"
)

// Diagnostic is one analysis diagnostic before LSP conversion.
type Diagnostic struct {
	Code     string
	Message  string
	Span     source.Span
	Severity Severity
}
