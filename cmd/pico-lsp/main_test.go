package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asimmons91/pico-lsp/internal/version"
)

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeFrame(t *testing.T, w io.Writer, body string) {
	t.Helper()

	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		t.Fatalf("write frame header: %v", err)
	}
	if _, err := io.WriteString(w, body); err != nil {
		t.Fatalf("write frame body: %v", err)
	}
}

func readFrame(t *testing.T, r *bufio.Reader) []byte {
	t.Helper()

	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("read frame header: %v", err)
		}
		if line == "\r\n" {
			break
		}
		if !strings.HasSuffix(line, "\r\n") {
			t.Fatalf("stdout line is not CRLF terminated: %q", line)
		}

		key, value, ok := strings.Cut(strings.TrimSuffix(line, "\r\n"), ":")
		if !ok {
			t.Fatalf("stdout output is not an LSP header: %q", line)
		}
		if key == "Content-Length" {
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				t.Fatalf("invalid Content-Length %q: %v", value, err)
			}
		}
	}
	if length < 0 {
		t.Fatal("frame is missing Content-Length")
	}

	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		t.Fatalf("read frame body: %v", err)
	}
	if !json.Valid(body) {
		t.Fatalf("frame body is not valid JSON: %q", body)
	}
	return body
}

func decodeResponse(t *testing.T, body []byte) rpcResponse {
	t.Helper()

	var resp rpcResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.JSONRPC != "2.0" {
		t.Fatalf("jsonrpc = %q, want 2.0", resp.JSONRPC)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error response: %+v", resp.Error)
	}
	return resp
}

func runWithTimeout(t *testing.T, ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	t.Helper()

	done := make(chan int, 1)
	go func() {
		done <- run(ctx, args, stdin, stdout, stderr)
	}()

	select {
	case code := <-done:
		return code
	case <-time.After(5 * time.Second):
		t.Fatal("run did not return within 5s")
		return -1
	}
}

type canceledContextNoDone struct{}

func (canceledContextNoDone) Deadline() (time.Time, bool) { return time.Time{}, false }
func (canceledContextNoDone) Done() <-chan struct{}       { return nil }
func (canceledContextNoDone) Err() error                  { return context.Canceled }
func (canceledContextNoDone) Value(any) any               { return nil }

func TestChooseExit(t *testing.T) {
	closed := func() <-chan struct{} {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	never := make(chan struct{})

	canceledCtx, cancelCanceled := context.WithCancel(context.Background())
	cancelCanceled()
	defer cancelCanceled()

	no := func() bool { return false }
	yes := func() bool { return true }

	tests := []struct {
		name         string
		ctx          context.Context
		exitSignal   <-chan struct{}
		connDone     <-chan struct{}
		exitCode     func() int
		exited       func() bool
		shuttingDown func() bool
		wantCode     int
		wantCause    exitCause
	}{
		{
			name:       "exit notification",
			ctx:        context.Background(),
			exitSignal: closed(),
			connDone:   never,
			exitCode:   func() int { return 0 },
			exited:     yes, shuttingDown: yes,
			wantCode: 0, wantCause: exitFromClient,
		},
		{
			name:       "exit without shutdown",
			ctx:        context.Background(),
			exitSignal: closed(),
			connDone:   never,
			exitCode:   func() int { return 1 },
			exited:     yes, shuttingDown: yes,
			wantCode: 1, wantCause: exitFromClient,
		},
		{
			name:       "context canceled",
			ctx:        canceledCtx,
			exitSignal: never,
			connDone:   never,
			exitCode:   func() int { return 1 },
			exited:     no, shuttingDown: no,
			wantCode: 0, wantCause: exitFromSignal,
		},
		{
			name:       "connection closed with canceled parent",
			ctx:        canceledContextNoDone{},
			exitSignal: never,
			connDone:   closed(),
			exitCode:   func() int { return 1 },
			exited:     no, shuttingDown: no,
			wantCode: 0, wantCause: exitFromSignal,
		},
		{
			name:       "connection closed after shutdown",
			ctx:        context.Background(),
			exitSignal: never,
			connDone:   closed(),
			exitCode:   func() int { return 1 },
			exited:     no, shuttingDown: yes,
			wantCode: 0, wantCause: exitFromClient,
		},
		{
			name:       "connection closed unexpectedly",
			ctx:        context.Background(),
			exitSignal: never,
			connDone:   closed(),
			exitCode:   func() int { return 0 },
			exited:     no, shuttingDown: no,
			wantCode: 1, wantCause: exitFromConnection,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, cause := chooseExit(tt.ctx, tt.exitSignal, tt.connDone, tt.exitCode, tt.exited, tt.shuttingDown)
			if code != tt.wantCode || cause != tt.wantCause {
				t.Fatalf("chooseExit = (%d,%v), want (%d,%v)", code, cause, tt.wantCode, tt.wantCause)
			}
		})
	}
}

func TestRunSpeaksOnlyLSPOnStdout(t *testing.T) {
	inR, inW := net.Pipe()
	outR, outW := net.Pipe()
	var stderr bytes.Buffer

	deadline := time.Now().Add(10 * time.Second)
	_ = inW.SetWriteDeadline(deadline)
	_ = outR.SetReadDeadline(deadline)

	done := make(chan int, 1)
	go func() {
		done <- run(context.Background(), nil, inR, outW, &stderr)
	}()

	reader := bufio.NewReader(outR)

	writeFrame(t, inW, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{}}}`)
	initResp := decodeResponse(t, readFrame(t, reader))

	var initResult struct {
		Capabilities struct {
			PositionEncoding string `json:"positionEncoding"`
			TextDocumentSync struct {
				OpenClose bool `json:"openClose"`
				Change    int  `json:"change"`
				Save      struct {
					IncludeText bool `json:"includeText"`
				} `json:"save"`
			} `json:"textDocumentSync"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(initResp.Result, &initResult); err != nil {
		t.Fatalf("decode initialize result: %v", err)
	}
	if initResult.Capabilities.PositionEncoding != "utf-16" {
		t.Fatalf("positionEncoding = %q, want utf-16", initResult.Capabilities.PositionEncoding)
	}
	if !initResult.Capabilities.TextDocumentSync.OpenClose {
		t.Fatal("openClose = false, want true")
	}
	if initResult.Capabilities.TextDocumentSync.Change != 2 {
		t.Fatalf("change = %d, want 2 (incremental)", initResult.Capabilities.TextDocumentSync.Change)
	}
	if initResult.ServerInfo.Name != "pico-lsp" {
		t.Fatalf("serverInfo.name = %q, want pico-lsp", initResult.ServerInfo.Name)
	}
	if initResult.ServerInfo.Version != version.Current() {
		t.Fatalf("serverInfo.version = %q, want %q", initResult.ServerInfo.Version, version.Current())
	}

	writeFrame(t, inW, `{"jsonrpc":"2.0","method":"initialized","params":{}}`)
	writeFrame(t, inW, `{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///tmp/cart.p8","languageId":"pico8","version":1,"text":"print('hi')"}}}`)
	writeFrame(t, inW, `{"jsonrpc":"2.0","id":2,"method":"shutdown","params":null}`)
	decodeResponse(t, readFrame(t, reader))
	writeFrame(t, inW, `{"jsonrpc":"2.0","method":"exit","params":null}`)

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server exit")
	}

	_ = inW.Close()
	_ = outW.Close()
	_ = outR.SetReadDeadline(time.Now().Add(time.Second))

	extra, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read leftover stdout: %v", err)
	}
	if len(extra) != 0 {
		t.Fatalf("non-frame output on stdout after exit: %q", extra)
	}
}

func TestRunCancellationDuringSessionExitsZero(t *testing.T) {
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		inR, inW := net.Pipe()
		outR, outW := net.Pipe()

		deadline := time.Now().Add(5 * time.Second)
		_ = inW.SetWriteDeadline(deadline)
		_ = outR.SetReadDeadline(deadline)

		done := make(chan int, 1)
		go func() {
			done <- run(ctx, nil, inR, outW, io.Discard)
		}()

		reader := bufio.NewReader(outR)
		writeFrame(t, inW, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{}}}`)
		decodeResponse(t, readFrame(t, reader))

		cancel()
		// Make conn.Done ready as well: a signal closes the context, but a
		// blocked pipe read does not observe cancellation on its own.
		_ = inW.Close()

		select {
		case code := <-done:
			if code != 0 {
				t.Fatalf("iteration %d: exit code = %d, want 0", i, code)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("iteration %d: timed out waiting for cancellation exit", i)
		}

		_ = outW.Close()
	}
}

type recordingCloser struct {
	r      io.Reader
	w      io.Writer
	closed atomic.Bool
}

func (c *recordingCloser) Read(p []byte) (int, error)  { return c.r.Read(p) }
func (c *recordingCloser) Write(p []byte) (int, error) { return c.w.Write(p) }
func (c *recordingCloser) Close() error {
	c.closed.Store(true)
	return nil
}

func TestRunSignalClosesTransport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	inR, inW := net.Pipe()
	outR, outW := net.Pipe()

	in := &recordingCloser{r: inR}
	out := &recordingCloser{w: outW}

	done := make(chan int, 1)
	go func() {
		done <- run(ctx, nil, in, out, io.Discard)
	}()

	reader := bufio.NewReader(outR)
	_ = inW.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_ = outR.SetReadDeadline(time.Now().Add(5 * time.Second))

	writeFrame(t, inW, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{}}}`)
	decodeResponse(t, readFrame(t, reader))

	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for cancellation exit")
	}

	if !in.closed.Load() || !out.closed.Load() {
		t.Fatalf("transport closers called: in=%v out=%v", in.closed.Load(), out.closed.Load())
	}

	_ = inW.Close()
	_ = inR.Close()
	_ = outR.Close()
	_ = outW.Close()
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := runWithTimeout(t, context.Background(), []string{"--version"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got, want := stdout.String(), version.Current()+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := runWithTimeout(t, context.Background(), []string{"--help"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Usage of pico-lsp") {
		t.Fatalf("stderr = %q, want usage text", stderr.String())
	}
}

func TestRunInvalidLogLevel(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := runWithTimeout(t, context.Background(), []string{"--log-level", "loud"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "invalid log level") {
		t.Fatalf("stderr = %q, want invalid log level message", stderr.String())
	}
}

func TestRunCanceledContextExitsPromptly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	inR, inW := net.Pipe()
	outR, outW := net.Pipe()

	start := time.Now()
	code := runWithTimeout(t, ctx, nil, inR, outW, io.Discard)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("run took %v after cancellation, want prompt exit", elapsed)
	}

	_ = inW.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := inW.Write([]byte("x")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("server-side stdin was not closed: %v", err)
	}

	_ = inW.Close()
	_ = outW.Close()
	_ = outR.Close()
}

func TestRunLogFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pico-lsp.log")
	var stdout, stderr bytes.Buffer

	code := runWithTimeout(t, context.Background(), []string{"--log-file", path}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 for EOF without exit", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(data), "pico-lsp started") {
		t.Fatalf("log file = %q, want startup message", data)
	}
}

func writeTestP8PNG(t *testing.T, code string) string {
	t.Helper()
	payload := make([]byte, 160*205)
	copy(payload[0x4300:], []byte(code))
	img := image.NewNRGBA(image.Rect(0, 0, 160, 205))
	for y := 0; y < 205; y++ {
		for x := 0; x < 160; x++ {
			b := payload[y*160+x]
			img.SetNRGBA(x, y, color.NRGBA{
				R: 0xfc | (b>>4)&3,
				G: 0xfc | (b>>2)&3,
				B: 0xfc | b&3,
				A: 0xfc | (b>>6)&3,
			})
		}
	}
	path := filepath.Join(t.TempDir(), "cart.p8.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunDecodePNG(t *testing.T) {
	path := writeTestP8PNG(t, "x=1\nfunction f()\nend\n")
	var stdout, stderr bytes.Buffer
	code := runWithTimeout(t, context.Background(), []string{"decode", path}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "function f()") || !strings.Contains(stdout.String(), "__lua__") {
		t.Fatalf("decoded stdout = %q", stdout.String())
	}

	out := filepath.Join(t.TempDir(), "decoded.p8")
	stdout.Reset()
	stderr.Reset()
	code = runWithTimeout(t, context.Background(), []string{"decode", "--output", out, path}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("output decode exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "function f()") {
		t.Fatalf("decoded file = %q", data)
	}
}

func TestRunDecodeRejectsInvalidPNG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.p8.png")
	if err := os.WriteFile(path, []byte("bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runWithTimeout(t, context.Background(), []string{"decode", path}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
