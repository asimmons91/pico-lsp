// Package version reports the pico-lsp build version.
package version

// Version is set at build time with:
//
//	go build -ldflags "-X github.com/asimmons91/pico-lsp/internal/version.Version=v1.0.0"
//
// Development builds report "dev".
var Version = "dev"

// Current returns the version string advertised to LSP clients.
func Current() string {
	if Version == "" {
		return "dev"
	}
	return Version
}
