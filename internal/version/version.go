// Package version reports the pico-lsp build version.
package version

import (
	"runtime/debug"
	"strings"
)

// Version is set at build time with:
//
//	go build -ldflags "-X github.com/asimmons91/pico-lsp/internal/version.Version=v1.0.0"
//
// Development builds report "dev" or, when Go stamped a VCS build, a
// pseudo-version. When Version is left at its default and the binary was
// installed from a module version, Current falls back to Go's build information
// so `go install ...@v1.0.0` also reports a release version.
var Version = "dev"

// Current returns the version string advertised to LSP clients.
func Current() string {
	value := Version
	if value == "" || value == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			value = info.Main.Version
		}
	}
	switch value {
	case "", "devel", "(devel)":
		return "dev"
	}
	if !strings.HasPrefix(value, "v") {
		return "v" + value
	}
	return value
}
