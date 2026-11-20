package lsp_test

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"

	"github.com/asimmons91/pico-lsp/pkg/lsp"
)

func FuzzMalformedFrames(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("garbage"))
	f.Add([]byte("Content-Length: 2\r\n\r\n{}"))
	f.Add([]byte("Content-Length: 999\r\n\r\n{"))
	f.Add([]byte("Content-Length: -1\r\n\r\n"))
	f.Add([]byte("Content-Length: 65\r\n\r\n{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{}}"))
	f.Add([]byte("Content-Length: 4\r\n\r\nnull"))
	f.Add([]byte("Content-Type: application/vscode-jsonrpc\r\n\r\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		serverPipe, clientPipe := net.Pipe()

		serverCtx, cancelServer := context.WithCancel(context.Background())
		serverConn := lsp.New(lsp.Options{}).Serve(serverCtx, jsonrpc2.NewStream(serverPipe))

		clientCtx, cancelClient := context.WithCancel(context.Background())
		clientConn := jsonrpc2.NewConn(jsonrpc2.NewStream(clientPipe))
		clientConn.Go(clientCtx, func(_ context.Context, req *jsonrpc2.Request) (any, error) {
			if req.IsCall() {
				// Fail server-initiated calls quickly instead of leaving the
				// handler to wait for the 5-second configuration timeout.
				return nil, jsonrpc2.Errorf(jsonrpc2.MethodNotFound, "fuzz client")
			}
			return nil, nil
		})

		var closeOnce sync.Once
		closeAll := func() {
			closeOnce.Do(func() {
				_ = clientConn.Close()
				_ = serverConn.Close()
			})
		}
		defer func() {
			cancelClient()
			cancelServer()
			go closeAll()
		}()

		written := make(chan struct{})
		go func() {
			_, _ = clientPipe.Write(data)
			close(written)
		}()
		select {
		case <-written:
		case <-time.After(time.Second):
			// The server may reject a malformed header before consuming the
			// whole frame. Closing the client side unblocks the writer.
		}

		done := make(chan struct{})
		go func() {
			closeAll()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("connection did not terminate for %s", frameSummary(data))
		}

		select {
		case <-serverConn.Done():
		default:
			t.Fatalf("server connection is still running for %s", frameSummary(data))
		}
		cancelClient()
		cancelServer()
	})
}

func frameSummary(data []byte) string {
	const limit = 64
	if len(data) <= limit {
		return fmt.Sprintf("%q", data)
	}
	return fmt.Sprintf("%q… (%d bytes)", data[:limit], len(data))
}
