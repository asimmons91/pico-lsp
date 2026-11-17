package lsp

import (
	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/config"
)

func analysisOptions(settings config.Settings) analysis.Options {
	return analysis.Options{
		UndefinedGlobal: configuredSeverity(settings.Diagnostics.UndefinedGlobal),
		TypeChecking:    configuredTypeChecking(settings.TypeChecking),
		Lint: analysis.LintOptions{
			CallbackPrecedence:  configuredSeverity(settings.Diagnostics.CallbackPrecedence),
			CallbackParams:      configuredSeverity(settings.Diagnostics.CallbackParams),
			NoStdlib:            configuredSeverity(settings.Diagnostics.NoStdlib),
			DuplicateTableField: configuredSeverity(settings.Diagnostics.DuplicateTableField),
			UnreachableCode:     configuredSeverity(settings.Diagnostics.UnreachableCode),
			UnusedLocal:         configuredSeverity(settings.Diagnostics.UnusedLocal),
			ShadowedLocal:       configuredSeverity(settings.Diagnostics.ShadowedLocal),
			EmptyBlock:          configuredSeverity(settings.Diagnostics.EmptyBlock),
			GlobalInFunction:    configuredSeverity(settings.Diagnostics.GlobalInFunction),
		},
	}
}

func configuredTypeChecking(value string) analysis.TypeCheckingMode {
	switch value {
	case config.TypeCheckingOff:
		return analysis.TypeCheckingOff
	case config.TypeCheckingStrict:
		return analysis.TypeCheckingStrict
	default:
		return analysis.TypeCheckingBasic
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
