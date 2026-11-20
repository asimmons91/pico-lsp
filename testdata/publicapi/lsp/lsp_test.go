package publicapi

import (
	"context"
	"net"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/pkg/glyphs"
	"github.com/asimmons91/pico-lsp/pkg/lsp"
)

type client struct {
	protocol.UnimplementedClient
}

func TestInProcessServerRoundTrip(t *testing.T) {
	serverPipe, clientPipe := net.Pipe()
	serverCtx, cancelServer := context.WithCancel(context.Background())
	serverConn := lsp.New(lsp.Options{}).Serve(serverCtx, jsonrpc2.NewStream(serverPipe))

	clientCtx, cancelClient := context.WithCancel(context.Background())
	_, clientConn, clientAPI := protocol.NewClient(clientCtx, &client{}, jsonrpc2.NewStream(clientPipe))

	t.Cleanup(func() {
		cancelClient()
		cancelServer()
		done := make(chan struct{})
		go func() {
			_ = clientConn.Close()
			_ = serverConn.Close()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("timed out closing the in-process server")
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	root := uri.File(t.TempDir())
	result, err := clientAPI.Initialize(ctx, &protocol.InitializeParams{
		Capabilities: protocol.ClientCapabilities{
			Workspace: &protocol.WorkspaceClientCapabilities{},
		},
		WorkspaceFolders: protocol.NewNullable([]protocol.WorkspaceFolder{{URI: root, Name: "workspace"}}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ServerInfo.Name != "pico-lsp" {
		t.Fatalf("ServerInfo = %+v", result.ServerInfo)
	}
	if result.Capabilities.SemanticTokensProvider == nil {
		t.Fatal("semantic token capability is missing")
	}

	if err := clientAPI.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		t.Fatal(err)
	}
	if err := clientAPI.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestGlyphCatalog(t *testing.T) {
	entries := glyphs.Builtins()
	if len(entries) == 0 {
		t.Fatal("Builtins returned no entries")
	}
	heart, ok := glyphs.FindByName(entries, "p8_heart")
	if !ok || heart.Byte != 0x87 || heart.Text != "♥" {
		t.Fatalf("FindByName = %+v, %v", heart, ok)
	}
	if left, ok := glyphs.FindByByte(entries, 0x8b); !ok || left.Name != "left" {
		t.Fatalf("FindByByte = %+v, %v", left, ok)
	}

	entries[0].Name = "changed"
	if glyphs.Builtins()[0].Name == "changed" {
		t.Fatal("Builtins did not return a copy")
	}

	byteValue := 0x80
	custom, warnings := glyphs.Build([]glyphs.Definition{{
		Name:        "logo",
		Byte:        &byteValue,
		Description: "custom font slot",
	}})
	if len(warnings) != 0 {
		t.Fatalf("Build warnings = %v", warnings)
	}
	logo, ok := glyphs.FindByName(custom, "p8_logo")
	if !ok || logo.Byte != 0x80 || !logo.Custom || logo.Context != glyphs.ContextBoth {
		t.Fatalf("custom logo = %+v, %v", logo, ok)
	}
}
