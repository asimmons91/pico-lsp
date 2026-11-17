package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func TestRunSpeaksOnlyLSPOnStdout(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	var stderr bytes.Buffer

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

	extra, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read leftover stdout: %v", err)
	}
	if len(extra) != 0 {
		t.Fatalf("non-frame output on stdout after exit: %q", extra)
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run(context.Background(), []string{"--version"}, strings.NewReader(""), &stdout, &stderr)
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

	code := run(context.Background(), []string{"--help"}, strings.NewReader(""), &stdout, &stderr)
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

	code := run(context.Background(), []string{"--log-level", "loud"}, strings.NewReader(""), &stdout, &stderr)
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

func TestRunLogFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pico-lsp.log")
	var stdout, stderr bytes.Buffer

	code := run(context.Background(), []string{"--log-file", path}, strings.NewReader(""), &stdout, &stderr)
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
