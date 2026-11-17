// Command pico-lsp is the PICO-8 language server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.lsp.dev/jsonrpc2"

	"github.com/asimmons91/pico-lsp/internal/config"
	"github.com/asimmons91/pico-lsp/internal/lsp"
	"github.com/asimmons91/pico-lsp/internal/version"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run wires flags, logging, and the stdio transport. It is separated from main
// so tests can exercise the complete process without spawning a subprocess.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pico-lsp", flag.ContinueOnError)
	fs.SetOutput(stderr)

	logFile := fs.String("log-file", "", "write logs to `PATH` instead of stderr")
	logLevel := fs.String("log-level", "info", "log verbosity: debug, info, warn, error")
	showVersion := fs.Bool("version", false, "print the version and exit")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "pico-lsp: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, version.Current())
		return 0
	}

	level, err := parseLogLevel(*logLevel)
	if err != nil {
		fmt.Fprintf(stderr, "pico-lsp: %v\n", err)
		return 2
	}

	logWriter := stderr
	var logHandle *os.File
	if *logFile != "" {
		logHandle, err = os.OpenFile(*logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintf(stderr, "pico-lsp: open log file: %v\n", err)
			return 1
		}
		defer logHandle.Close()
		logWriter = logHandle
	}

	logger := slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{Level: level}))

	srv := lsp.New(logger, config.New(logger))
	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream := jsonrpc2.NewStream(&stdioTransport{in: stdin, out: stdout})
	conn := srv.Serve(connCtx, stream)
	logger.Info("pico-lsp started", "version", version.Current())

	code, cause := chooseExit(ctx, srv.ExitSignal(), conn.Done(), srv.ExitCode, srv.Exited, srv.ShuttingDown)
	signaled := cause == exitFromSignal
	switch cause {
	case exitFromSignal:
		logger.Info("exiting on signal")
	case exitFromConnection:
		if err := conn.Err(); err != nil {
			logger.Error("connection closed", "error", err)
		} else {
			logger.Info("connection closed by client")
		}
	}

	cancel()
	if signaled {
		// Close the transport without waiting for the read loop, which may be
		// blocked on a pipe read that close cannot interrupt. Process exit will
		// terminate the remaining goroutine.
		_ = stream.Close()
		go func() { _ = conn.Close() }()
	} else {
		closeConn(conn, time.Second)
	}
	logger.Info("pico-lsp stopped", "code", code)
	return code
}

// exitCause records which channel ended the server loop.
type exitCause int

const (
	exitFromClient exitCause = iota
	exitFromSignal
	exitFromConnection
)

// chooseExit decides the process exit code. It is separated from run so the
// signal/connection race can be tested deterministically.
func chooseExit(
	ctx context.Context,
	exitSignal, connDone <-chan struct{},
	exitCode func() int,
	exited, shuttingDown func() bool,
) (code int, cause exitCause) {
	select {
	case <-exitSignal:
		return exitCode(), exitFromClient
	case <-ctx.Done():
		return 0, exitFromSignal
	case <-connDone:
		if ctx.Err() != nil {
			// A signal cancels both contexts; which channel the select observes
			// is nondeterministic. Treat a canceled parent as the signal path.
			return 0, exitFromSignal
		}
		switch {
		case exited():
			return exitCode(), exitFromClient
		case shuttingDown():
			return 0, exitFromClient
		default:
			return 1, exitFromConnection
		}
	}
}

func parseLogLevel(value string) (slog.Level, error) {
	switch strings.ToLower(value) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level %q (want debug, info, warn, or error)", value)
	}
}

func closeConn(conn jsonrpc2.Conn, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		_ = conn.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
	}
}

// stdioTransport adapts separate stdin/stdout handles to the io.ReadWriteCloser
// required by jsonrpc2. Closing it closes the underlying handles so a blocked
// protocol read unblocks during shutdown.
type stdioTransport struct {
	in  io.Reader
	out io.Writer
}

func (t *stdioTransport) Read(p []byte) (int, error)  { return t.in.Read(p) }
func (t *stdioTransport) Write(p []byte) (int, error) { return t.out.Write(p) }

func (t *stdioTransport) Close() error {
	var err error
	if closer, ok := t.in.(io.Closer); ok {
		if closeErr := closer.Close(); closeErr != nil {
			err = closeErr
		}
	}
	if closer, ok := t.out.(io.Closer); ok {
		if closeErr := closer.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}
