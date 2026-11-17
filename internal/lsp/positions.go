package lsp

import (
	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/source"
)

const (
	// DiagnosticSource is attached to every diagnostic published by pico-lsp.
	DiagnosticSource = "pico-lsp"
)

// spanRange converts a byte range to an LSP range.
func spanRange(index *source.LineIndex, start, end int) protocol.Range {
	return index.Range(start, end)
}

// newDiagnostic builds a diagnostic whose range is derived from byte offsets.
func newDiagnostic(index *source.LineIndex, start, end int, severity protocol.DiagnosticSeverity, code, message string) protocol.Diagnostic {
	return protocol.Diagnostic{
		Range:    spanRange(index, start, end),
		Severity: severity,
		Code:     protocol.String(code),
		Source:   protocol.NewOptional(DiagnosticSource),
		Message:  protocol.String(message),
	}
}
