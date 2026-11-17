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

// Diagnostic is one analysis diagnostic before LSP conversion.
type Diagnostic struct {
	Code     string
	Message  string
	Span     source.Span
	Severity Severity
}
