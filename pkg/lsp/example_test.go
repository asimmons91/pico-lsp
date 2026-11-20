package lsp_test

import (
	"context"
	"fmt"
	"net"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/pkg/lsp"
)

func ExampleServer() {
	serverPipe, clientPipe := net.Pipe()

	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	serverConn := lsp.New(lsp.Options{}).Serve(serverCtx, jsonrpc2.NewStream(serverPipe))
	defer func() { _ = serverConn.Close() }()

	clientCtx, cancelClient := context.WithCancel(context.Background())
	defer cancelClient()
	_, clientConn, client := protocol.NewClient(clientCtx, &protocol.UnimplementedClient{}, jsonrpc2.NewStream(clientPipe))
	defer func() { _ = clientConn.Close() }()

	ctx := context.Background()
	result, err := client.Initialize(ctx, &protocol.InitializeParams{})
	if err != nil {
		panic(err)
	}
	fmt.Println(result.ServerInfo.Name, result.Capabilities.SemanticTokensProvider != nil)

	if err := client.Shutdown(ctx); err != nil {
		panic(err)
	}
	// Output:
	// pico-lsp true
}
