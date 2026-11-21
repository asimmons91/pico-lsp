// Package lsp runs the pico-lsp language server in-process for Go programs.
//
// External programs create a Server with New, supply their own
// [jsonrpc2.Stream], and call Serve. The server speaks the same LSP protocol as
// the pico-lsp stdio binary; capabilities and behavior do not change when it is
// embedded.
//
// The caller owns the stream and the connection returned by Serve. There is no
// transport helper and no separate shutdown API: cancel the context passed to
// Serve or close the returned connection to stop the server. Configuration is
// LSP-native, arriving through initializationOptions.pico8 and
// workspace/configuration; this package exposes no settings types.
//
// The package is additive from v1.6.0 and stable under semver.
package lsp

import (
	"context"
	"log/slog"

	"go.lsp.dev/jsonrpc2"

	"github.com/asimmons91/pico-lsp/internal/config"
	internallsp "github.com/asimmons91/pico-lsp/internal/lsp"
)

// Options configures a Server. The zero value is valid.
type Options struct {
	// Logger receives server log records. A nil Logger discards every record,
	// and the server never writes to stderr or stdout on its own.
	Logger *slog.Logger
}

// Server is an in-process pico-lsp server. One Server serves one connection;
// create a new Server for another connection.
type Server struct {
	inner *internallsp.Server
}

// New creates a server with default configuration and the given logger.
//
// Settings are supplied by the LSP client through initializationOptions.pico8
// and workspace/configuration. New always returns a usable server.
func New(opts Options) *Server {
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Server{
		inner: internallsp.New(logger, config.New(logger)),
	}
}

// Serve starts one LSP connection over stream and returns it. The caller owns
// the connection: wait on its Done channel, inspect Err, and close it to stop
// the server. Canceling ctx requests a graceful stop at the next frame
// boundary; closing the connection is the reliable way to unblock a reader.
//
// Both ctx and stream must be non-nil; passing nil is a programmer error and
// may panic. Protocol errors travel through the connection rather than through
// a return value.
func (s *Server) Serve(ctx context.Context, stream jsonrpc2.Stream) jsonrpc2.Conn {
	return s.inner.Serve(ctx, stream)
}
