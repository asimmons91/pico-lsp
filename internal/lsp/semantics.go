package lsp

import (
	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/config"
)

func analysisOptions(settings config.Settings) analysis.Options {
	return analysis.Options{
		UndefinedGlobal: configuredSeverity(settings.Diagnostics.UndefinedGlobal),
	}
}

func configuredSeverity(value string) analysis.Severity {
	switch value {
	case config.SeverityOff:
		return analysis.SeverityOff
	case config.SeverityHint:
		return analysis.SeverityHint
	case config.SeverityInfo:
		return analysis.SeverityInfo
	case config.SeverityError:
		return analysis.SeverityError
	default:
		return analysis.SeverityWarning
	}
}

func analysisSeverity(severity analysis.Severity) protocol.DiagnosticSeverity {
	switch severity {
	case analysis.SeverityError:
		return protocol.DiagnosticSeverityError
	case analysis.SeverityInfo:
		return protocol.DiagnosticSeverityInformation
	case analysis.SeverityHint:
		return protocol.DiagnosticSeverityHint
	default:
		return protocol.DiagnosticSeverityWarning
	}
}

func protocolSymbolKind(kind analysis.SymbolKind) protocol.SymbolKind {
	switch kind {
	case analysis.SymbolFunction:
		return protocol.SymbolKindFunction
	case analysis.SymbolMethod:
		return protocol.SymbolKindMethod
	case analysis.SymbolField:
		return protocol.SymbolKindField
	case analysis.SymbolLabel:
		return protocol.SymbolKindKey
	case analysis.SymbolClass:
		return protocol.SymbolKindClass
	case analysis.SymbolParam:
		return protocol.SymbolKindVariable
	default:
		return protocol.SymbolKindVariable
	}
}
