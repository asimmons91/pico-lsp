package lsp

import (
	"fmt"
	"path/filepath"
	"strings"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/config"
	"github.com/asimmons91/pico-lsp/internal/source"
)

type includeDiagnostic struct {
	span     source.Span
	severity protocol.DiagnosticSeverity
	code     string
	message  string
}

func narrowIncludeDiagnosticSpan(snap *cart.Snapshot, span source.Span) source.Span {
	if snap == nil || snap.Cart == nil {
		return span
	}
	file := snap.Cart.File(span.FileID)
	if file == nil {
		return span
	}
	for _, include := range snap.Cart.Includes {
		if include.DirectiveSpan != span {
			continue
		}
		if pathSpan, ok := includePathSpan(file.Text, include.DirectiveSpan); ok {
			return pathSpan
		}
	}
	return span
}

func (m *CartManager) includeDiagnostics(snap *cart.Snapshot, settings config.Settings) []includeDiagnostic {
	if snap == nil || snap.Cart == nil {
		return nil
	}
	rootDir := filepath.Dir(snap.Cart.RootURI.FsPath())
	allowedRoots := append([]uri.URI(nil), m.workspaceFolders()...)
	allowedRoots = append(allowedRoots, uri.File(rootDir))
	var out []includeDiagnostic

	outsideSeverity, outsideOK := diagnosticSeverity(settings.Diagnostics.IncludeOutsideWorkspace)
	unusualSeverity, unusualOK := diagnosticSeverity(settings.Diagnostics.IncludeUnusualType)
	duplicateSeverity, duplicateOK := diagnosticSeverity(settings.Diagnostics.IncludeDuplicate)
	seen := make(map[string]struct{})

	for _, include := range snap.Cart.Includes {
		if include.Nested {
			// The loader already reports nested directives; they are pasted
			// literally rather than expanded, so extra checks would be noise.
			continue
		}
		file := snap.Cart.File(include.DirectiveSpan.FileID)
		if file == nil || file.Index == nil {
			continue
		}
		span := include.DirectiveSpan
		if pathSpan, ok := includePathSpan(file.Text, include.DirectiveSpan); ok {
			span = pathSpan
		}
		resolved := include.ResolvedURI.FsPath()
		if resolved != "" {
			if outsideOK && !withinAllowedRoots(allowedRoots, resolved) {
				out = append(out, includeDiagnostic{
					span:     span,
					severity: outsideSeverity,
					code:     "include-outside-workspace",
					message:  fmt.Sprintf("include %q resolves outside the workspace: %s", include.Path, resolved),
				})
			}
			if unusualOK && !isUsualInclude(resolved) {
				out = append(out, includeDiagnostic{
					span:     span,
					severity: unusualSeverity,
					code:     "include-unusual-type",
					message:  fmt.Sprintf("include target is not a .lua or .p8 file: %s", include.Path),
				})
			}
			key := filepath.Clean(resolved)
			if _, exists := seen[key]; exists {
				if duplicateOK {
					out = append(out, includeDiagnostic{
						span:     span,
						severity: duplicateSeverity,
						code:     "include-duplicate",
						message:  fmt.Sprintf("duplicate include of %s", include.Path),
					})
				}
			} else {
				seen[key] = struct{}{}
			}
		}
	}
	return out
}

func withinAllowedRoots(roots []uri.URI, target string) bool {
	for _, root := range roots {
		if pathWithin(root.FsPath(), target) {
			return true
		}
	}
	return false
}

func isUsualInclude(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".lua") || strings.HasSuffix(lower, ".p8")
}

func diagnosticSeverity(value string) (protocol.DiagnosticSeverity, bool) {
	if value == config.SeverityOff {
		return 0, false
	}
	return analysisSeverity(configuredSeverity(value)), true
}
