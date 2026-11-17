package lsp

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func writeP8PNG(t *testing.T, path, code string) uri.URI {
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	return uri.File(path)
}

func decodeCartForTest(t *testing.T, h *harness, source uri.URI) DecodeCartResult {
	t.Helper()
	params, _ := protocol.Marshal(map[string]any{"textDocument": map[string]string{"uri": source.String()}})
	result, err := h.srv.Request(context.Background(), RequestDecodeCart, protocol.LSPAny(params))
	if err != nil {
		t.Fatalf("decodeCart: %v", err)
	}
	return result.(DecodeCartResult)
}

func TestEndToEndDecodeP8PNG(t *testing.T) {
	code := "x=1\nfunction f()\nend\n"
	dir := t.TempDir()
	source := writeP8PNG(t, filepath.Join(dir, "cart.p8.png"), code)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	decoded := decodeCartForTest(t, h, source)
	if decoded.SourceURI != source {
		t.Fatalf("source = %s, want %s", decoded.SourceURI, source)
	}
	if !strings.Contains(decoded.Text, code) {
		t.Fatalf("decoded text = %q", decoded.Text)
	}
	if !isVirtualCartURI(decoded.VirtualURI) {
		t.Fatalf("virtual URI = %s", decoded.VirtualURI)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wire DecodeCartResult
	if _, err := h.clientConn.Call(ctx, RequestDecodeCart, map[string]any{
		"textDocument": map[string]string{"uri": source.String()},
	}, &wire); err != nil {
		t.Fatalf("decodeCart over the wire: %v", err)
	}
	if wire.VirtualURI != decoded.VirtualURI || wire.Text != decoded.Text {
		t.Fatalf("wire virtual=%q direct virtual=%q textEqual=%v", wire.VirtualURI, decoded.VirtualURI, wire.Text == decoded.Text)
	}

	defer cancel()
	openDoc(t, h, ctx, decoded.VirtualURI, decoded.Text)
	waitFor(t, "decoded cart analysis", func() bool {
		_, _, ok := h.srv.carts.SnapshotFor(ctx, decoded.VirtualURI)
		return ok
	})
	if _, err := h.server.Hover(ctx, &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: decoded.VirtualURI},
			Position:     protocol.Position{Line: 3, Character: 0},
		},
	}); err != nil {
		t.Fatalf("hover on decoded cart: %v", err)
	}

	params, _ := protocol.Marshal(map[string]any{"textDocument": map[string]string{"uri": source.String()}})
	info, err := h.srv.Request(ctx, RequestCartInfo, protocol.LSPAny(params))
	if err != nil {
		t.Fatalf("source cartInfo: %v", err)
	}
	cartInfo := info.(CartInfo)
	if cartInfo.RootURI != source || cartInfo.Tokens == 0 {
		t.Fatalf("source cartInfo = %+v", cartInfo)
	}

	_, err = h.srv.Formatting(ctx, &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: decoded.VirtualURI},
		Options:      protocol.FormattingOptions{TabSize: 2, InsertSpaces: true},
	})
	if err == nil {
		t.Fatal("formatting virtual cart succeeded")
	}

	commandArg, _ := protocol.Marshal(source.String())
	raw, err := h.srv.ExecuteCommand(ctx, &protocol.ExecuteCommandParams{
		Command:   CommandShowCartInfo,
		Arguments: []protocol.LSPAny{commandArg},
	})
	if err != nil {
		t.Fatalf("showCartInfo source: %v", err)
	}
	var commandInfo CartInfo
	if err := protocol.Unmarshal(raw, &commandInfo); err != nil || commandInfo.RootURI != source {
		t.Fatalf("showCartInfo info = %+v, err=%v", commandInfo, err)
	}

	writeP8PNG(t, source.FsPath(), "y=2\n")
	if err := h.srv.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{{URI: source, Type: protocol.FileChangeTypeChanged}},
	}); err != nil {
		t.Fatalf("watched PNG change: %v", err)
	}
	waitFor(t, "refreshed PNG virtual text", func() bool {
		doc, ok := h.srv.Document(decoded.VirtualURI)
		return ok && strings.Contains(doc.Text, "y=2")
	})
}

func TestDecodePNGCacheInvalidation(t *testing.T) {
	dir := t.TempDir()
	source := writeP8PNG(t, filepath.Join(dir, "cart.p8.png"), "x=1\n")
	h := newHarness(t, newTestClient())
	first := decodeCartForTest(t, h, source)
	if !strings.Contains(first.Text, "x=1") {
		t.Fatalf("first = %q", first.Text)
	}
	if err := os.WriteFile(source.FsPath(), []byte("not a png"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.srv.invalidatePNG(source)
	h.srv.carts.RemoveVirtualText(first.VirtualURI)
	params, _ := protocol.Marshal(map[string]any{"textDocument": map[string]string{"uri": source.String()}})
	if _, err := h.srv.Request(context.Background(), RequestDecodeCart, protocol.LSPAny(params)); err == nil {
		t.Fatal("invalid PNG decoded after cache invalidation")
	}
}

func TestDecodedPNGResolvesIncludes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lib.lua"), []byte("function lib() end\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := writeP8PNG(t, filepath.Join(dir, "cart.p8.png"), "#include \"lib.lua\"\nx=1\n")
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	decoded := decodeCartForTest(t, h, source)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, decoded.VirtualURI, decoded.Text)
	waitFor(t, "decoded include", func() bool {
		snap, _, ok := h.srv.carts.SnapshotFor(ctx, decoded.VirtualURI)
		if !ok || len(snap.Cart.Includes) != 1 {
			return false
		}
		return !snap.Cart.Includes[0].Missing && len(snap.Cart.Files) >= 2
	})
}
