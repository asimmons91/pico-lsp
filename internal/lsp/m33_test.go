package lsp

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/source"
)

func TestM33Capabilities(t *testing.T) {
	h := newHarness(t, newTestClient())
	result := h.initialize(t, &protocol.InitializeParams{})

	rangeProvider, ok := result.Capabilities.DocumentRangeFormattingProvider.(protocol.Boolean)
	if !ok || !bool(rangeProvider) {
		t.Fatalf("documentRangeFormattingProvider = %T, want true", result.Capabilities.DocumentRangeFormattingProvider)
	}
	if result.Capabilities.DocumentLinkProvider == nil {
		t.Fatal("documentLinkProvider is nil")
	}
}

func m33FormattingRange(startLine, startChar, endLine, endChar uint32) protocol.Range {
	return protocol.Range{
		Start: protocol.Position{Line: startLine, Character: startChar},
		End:   protocol.Position{Line: endLine, Character: endChar},
	}
}

func m33FormattingParams(u uri.URI, r protocol.Range) *protocol.DocumentRangeFormattingParams {
	return &protocol.DocumentRangeFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
		Range:        r,
		Options:      protocol.FormattingOptions{TabSize: 2, InsertSpaces: true},
	}
}

func TestM33RangeFormattingSelectedLines(t *testing.T) {
	text := strings.Join([]string{
		"pico-8 cartridge // pico-8.com",
		"version 42",
		"__lua__",
		"function f(a,b)",
		"if (a) then",
		"x=1",
		"else",
		"x=2",
		"end",
		"end",
		"after=3",
		"__gfx__",
		"0000000000000000",
	}, "\n") + "\n"
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	edits, err := h.server.RangeFormatting(ctx, m33FormattingParams(root, m33FormattingRange(4, 0, 8, 1)))
	if err != nil {
		t.Fatalf("range formatting: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("edits = %+v", edits)
	}
	if edits[0].Range.Start.Line != 4 || edits[0].Range.End.Line != 8 {
		t.Fatalf("edit range = %+v, want lines 4-8", edits[0].Range)
	}
	for _, want := range []string{"  if (a) then", "    x = 1", "  else", "    x = 2", "  end"} {
		if !strings.Contains(edits[0].NewText, want) {
			t.Fatalf("range edit missing %q:\n%s", want, edits[0].NewText)
		}
	}
	applied := applyProtocolEdit(t, text, edits[0])
	appliedLines := strings.Split(applied, "\n")
	if appliedLines[3] != "function f(a,b)" {
		t.Fatalf("unselected function line changed: %q", appliedLines[3])
	}
	if appliedLines[10] != "after=3" {
		t.Fatalf("unselected line changed: %q", appliedLines[10])
	}
	if appliedLines[4] != "  if (a) then" || appliedLines[5] != "    x = 1" {
		t.Fatalf("selected lines not formatted:\n%s", applied)
	}
}

func TestM33RangeFormattingSingleAndPartialLines(t *testing.T) {
	text := "function f()\nif (x) then\ny=1\nend\nend\nafter=2\n"
	root := writeTemp(t, t.TempDir(), "lib.lua", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	single, err := h.server.RangeFormatting(ctx, m33FormattingParams(root, m33FormattingRange(2, 0, 2, 2)))
	if err != nil {
		t.Fatalf("single-line range formatting: %v", err)
	}
	if len(single) != 1 || single[0].Range.Start.Line != 2 || single[0].Range.End.Line != 2 {
		t.Fatalf("single-line edits = %+v", single)
	}
	if single[0].NewText != "    y = 1" {
		t.Fatalf("single-line text = %q", single[0].NewText)
	}

	partial, err := h.server.RangeFormatting(ctx, m33FormattingParams(root, m33FormattingRange(1, 2, 2, 2)))
	if err != nil {
		t.Fatalf("partial-line range formatting: %v", err)
	}
	if len(partial) != 1 || partial[0].Range.Start.Line != 1 || partial[0].Range.End.Line != 2 {
		t.Fatalf("partial-line edits = %+v", partial)
	}
	if !strings.Contains(partial[0].NewText, "if (x) then") || !strings.Contains(partial[0].NewText, "y = 1") {
		t.Fatalf("partial-line text = %q", partial[0].NewText)
	}

	empty, err := h.server.RangeFormatting(ctx, m33FormattingParams(root, m33FormattingRange(2, 0, 2, 0)))
	if err != nil {
		t.Fatalf("empty range formatting: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("empty range edits = %+v, want none", empty)
	}
}

func TestM33RangeFormattingRefusesLineCountChange(t *testing.T) {
	text := "x=1\n\n\n"
	root := writeTemp(t, t.TempDir(), "lib.lua", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	_, err := h.server.RangeFormatting(ctx, m33FormattingParams(root, m33FormattingRange(0, 0, 0, 1)))
	var rpcErr *jsonrpc2.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != requestFailedCode {
		t.Fatalf("line-count error = %v, want RequestFailed", err)
	}
	if !strings.Contains(rpcErr.Message, "whole document") {
		t.Fatalf("line-count message = %q", rpcErr.Message)
	}
}

func TestM33RangeFormattingFinalNewline(t *testing.T) {
	text := "x=1"
	root := writeTemp(t, t.TempDir(), "lib.lua", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	edits, err := h.server.RangeFormatting(ctx, m33FormattingParams(root, m33FormattingRange(0, 0, 0, 1)))
	if err != nil {
		t.Fatalf("range formatting: %v", err)
	}
	if len(edits) != 1 || edits[0].NewText != "x = 1\n" {
		t.Fatalf("edits = %+v, want final newline", edits)
	}
	if got := applyProtocolEdit(t, text, edits[0]); got != "x = 1\n" {
		t.Fatalf("applied = %q", got)
	}
}

func TestM33RangeFormattingErrors(t *testing.T) {
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var rpcErr *jsonrpc2.Error
	if _, err := h.server.RangeFormatting(ctx, nil); !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.InvalidParams {
		t.Fatalf("nil params error = %v", err)
	}
	if _, err := h.server.RangeFormatting(ctx, &protocol.DocumentRangeFormattingParams{}); !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.InvalidParams {
		t.Fatalf("empty URI error = %v", err)
	}
	missing := uri.File(filepath.Join(t.TempDir(), "not-open.lua"))
	if _, err := h.server.RangeFormatting(ctx, m33FormattingParams(missing, m33FormattingRange(0, 0, 0, 1))); !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.InvalidParams {
		t.Fatalf("unopened document error = %v", err)
	}

	syntaxText := "function broken(\n"
	syntaxRoot := writeTemp(t, t.TempDir(), "broken.lua", syntaxText)
	openDoc(t, h, ctx, syntaxRoot, syntaxText)
	if _, err := h.server.RangeFormatting(ctx, m33FormattingParams(syntaxRoot, m33FormattingRange(0, 0, 0, 1))); !errors.As(err, &rpcErr) || rpcErr.Code != requestFailedCode {
		t.Fatalf("syntax error = %v", err)
	}

	missingText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"missing.lua\"\nx=1\n"
	missingRoot := writeTemp(t, t.TempDir(), "missing.p8", missingText)
	openDoc(t, h, ctx, missingRoot, missingText)
	if _, err := h.server.RangeFormatting(ctx, m33FormattingParams(missingRoot, m33FormattingRange(4, 0, 4, 1))); !errors.As(err, &rpcErr) || rpcErr.Code != requestFailedCode {
		t.Fatalf("missing include error = %v", err)
	}

	if _, err := h.server.RangeFormatting(ctx, m33FormattingParams(uri.File("/tmp/cart.p8.png"), m33FormattingRange(0, 0, 0, 1))); !errors.As(err, &rpcErr) || rpcErr.Code != requestFailedCode {
		t.Fatalf("png error = %v", err)
	}
	decoded := decodeCartForTest(t, h, writeP8PNG(t, filepath.Join(t.TempDir(), "cart.p8.png"), "x=1\n"))
	if _, err := h.server.RangeFormatting(ctx, m33FormattingParams(decoded.VirtualURI, m33FormattingRange(0, 0, 0, 1))); !errors.As(err, &rpcErr) || rpcErr.Code != requestFailedCode {
		t.Fatalf("virtual png error = %v", err)
	}
}

func TestM33RangeFormattingIncludedLuaWithMissingSibling(t *testing.T) {
	dir := t.TempDir()
	rootText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"missing.lua\"\n#include \"lib.lua\"\n"
	root := writeTemp(t, dir, "cart.p8", rootText)
	libText := "function f()\nx=1\nend\n"
	lib := writeTemp(t, dir, "lib.lua", libText)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)
	openDoc(t, h, ctx, lib, libText)

	edits, err := h.server.RangeFormatting(ctx, m33FormattingParams(lib, m33FormattingRange(0, 0, 1, 1)))
	if err != nil {
		t.Fatalf("included lua range formatting: %v", err)
	}
	if len(edits) != 1 || !strings.Contains(edits[0].NewText, "function f()") {
		t.Fatalf("edits = %+v", edits)
	}
}

type m33GoldenLink struct {
	Range   protocol.Range `json:"range"`
	Tooltip string         `json:"tooltip,omitempty"`
	Target  string         `json:"target"`
}

func TestM33DocumentLinks(t *testing.T) {
	dir := t.TempDir()
	rootText := strings.Join([]string{
		"pico-8 cartridge // pico-8.com",
		"version 42",
		"__lua__",
		`#include "lib/player.lua"`,
		`#include 'lib/other.lua'`,
		"#include   unquoted.lua",
		`#include "tabs.p8:2"`,
		`#include "missing.lua"`,
		`#include "nested.lua"`,
		"print(1)",
	}, "\n") + "\n"
	root := writeTemp(t, dir, "cart.p8", rootText)
	playerText := "#include \"nested-child.lua\"\nfunction player() end\n"
	writeTemp(t, dir, "lib/player.lua", playerText)
	writeTemp(t, dir, "lib/other.lua", "function other() end\n")
	writeTemp(t, dir, "unquoted.lua", "function u() end\n")
	writeTemp(t, dir, "nested.lua", "function nested() end\n")
	writeTemp(t, dir, "lib/nested-child.lua", "function child() end\n")
	tabsText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\ntab0=1\n-->8\ntab1=2\n-->8\ntab2=3\n"
	writeTemp(t, dir, "tabs.p8", tabsText)

	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, rootText)

	links, err := h.server.DocumentLink(ctx, &protocol.DocumentLinkParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
	})
	if err != nil {
		t.Fatalf("document links: %v", err)
	}
	if len(links) != 5 {
		t.Fatalf("links = %+v, want 5", links)
	}

	expected := map[string]struct {
		span    string
		tooltip string
	}{
		"lib/player.lua": {span: `"lib/player.lua"`, tooltip: "Open lib/player.lua"},
		"lib/other.lua":  {span: `'lib/other.lua'`, tooltip: "Open lib/other.lua"},
		"unquoted.lua":   {span: "unquoted.lua", tooltip: "Open unquoted.lua"},
		"tabs.p8":        {span: `"tabs.p8:2"`, tooltip: "Open tabs.p8 (tab 2)"},
		"nested.lua":     {span: `"nested.lua"`, tooltip: "Open nested.lua"},
	}
	index := source.New([]byte(rootText), protocol.PositionEncodingKindUTF16)
	golden := make([]m33GoldenLink, 0, len(links))
	for _, link := range links {
		if link.Target == nil {
			t.Fatal("link target is nil")
		}
		rel, relErr := filepath.Rel(dir, link.Target.FsPath())
		if relErr != nil {
			t.Fatalf("target relative path: %v", relErr)
		}
		rel = filepath.ToSlash(rel)
		want, ok := expected[rel]
		if !ok {
			t.Fatalf("unexpected target %q", rel)
		}
		start, end, offsetErr := index.RangeOffsets(link.Range)
		if offsetErr != nil {
			t.Fatalf("link range: %v", offsetErr)
		}
		if got := rootText[start:end]; got != want.span {
			t.Fatalf("%s span = %q, want %q", rel, got, want.span)
		}
		tooltip := ""
		if link.Tooltip != nil {
			tooltip = *link.Tooltip
		}
		if tooltip != want.tooltip {
			t.Fatalf("%s tooltip = %q, want %q", rel, tooltip, want.tooltip)
		}
		golden = append(golden, m33GoldenLink{Range: link.Range, Tooltip: tooltip, Target: rel})
	}
	goldenJSON(t, "m33/document_links.json", golden)

	openDoc(t, h, ctx, uri.File(filepath.Join(dir, "lib", "player.lua")), playerText)
	nestedLinks, err := h.server.DocumentLink(ctx, &protocol.DocumentLinkParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filepath.Join(dir, "lib", "player.lua"))},
	})
	if err != nil {
		t.Fatalf("nested document links: %v", err)
	}
	if len(nestedLinks) != 0 {
		t.Fatalf("nested links = %+v, want none", nestedLinks)
	}

	decoded := decodeCartForTest(t, h, writeP8PNG(t, filepath.Join(dir, "png.p8.png"), "x=1\n"))
	pngLinks, err := h.server.DocumentLink(ctx, &protocol.DocumentLinkParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: decoded.VirtualURI},
	})
	if err != nil {
		t.Fatalf("png document links: %v", err)
	}
	if len(pngLinks) != 0 {
		t.Fatalf("png links = %+v, want none", pngLinks)
	}
}

func TestM33DocumentLinkErrors(t *testing.T) {
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var rpcErr *jsonrpc2.Error
	if _, err := h.server.DocumentLink(ctx, nil); !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.InvalidParams {
		t.Fatalf("nil params error = %v", err)
	}
	if _, err := h.server.DocumentLink(ctx, &protocol.DocumentLinkParams{}); !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.InvalidParams {
		t.Fatalf("empty URI error = %v", err)
	}
	unopened := uri.File(filepath.Join(t.TempDir(), "not-open.lua"))
	if _, err := h.server.DocumentLink(ctx, &protocol.DocumentLinkParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: unopened},
	}); !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.InvalidParams {
		t.Fatalf("unopened document error = %v", err)
	}
}

func m33CodeLensCapabilities(refresh bool) protocol.ClientCapabilities {
	return protocol.ClientCapabilities{
		Workspace: &protocol.WorkspaceClientCapabilities{
			CodeLens: &protocol.CodeLensWorkspaceClientCapabilities{
				RefreshSupport: boolPtr(refresh),
			},
		},
	}
}

func TestM33CodeLensRefreshThrottled(t *testing.T) {
	client := newTestClient()
	h := newHarness(t, client)
	h.srv.codeLensRefreshDelay = 10 * time.Millisecond
	h.initialize(t, &protocol.InitializeParams{Capabilities: m33CodeLensCapabilities(true)})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	text := "x=1\n"
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	openDoc(t, h, ctx, root, text)
	select {
	case <-client.codeLensRefresh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for code lens refresh")
	}

	before := client.codeLensRefreshHits.Load()
	for version := int32(2); version <= 4; version++ {
		if err := h.server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
			TextDocument: protocol.VersionedTextDocumentIdentifier{
				TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: root},
				Version:                version,
			},
			ContentChanges: []protocol.TextDocumentContentChangeEvent{
				&protocol.TextDocumentContentChangeWholeDocument{Text: text},
			},
		}); err != nil {
			t.Fatalf("didChange: %v", err)
		}
	}
	select {
	case <-client.codeLensRefresh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for throttled code lens refresh")
	}
	time.Sleep(100 * time.Millisecond)
	if got := client.codeLensRefreshHits.Load(); got != before+1 {
		t.Fatalf("refresh count = %d, want %d", got, before+1)
	}

	h.exit(t, true)
}

func TestM33CodeLensRefreshOnlyWhenSupportedAndEnabled(t *testing.T) {
	t.Run("unsupported", func(t *testing.T) {
		client := newTestClient()
		h := newHarness(t, client)
		h.srv.codeLensRefreshDelay = 10 * time.Millisecond
		h.initialize(t, &protocol.InitializeParams{})
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		text := "x=1\n"
		openDoc(t, h, ctx, writeTemp(t, t.TempDir(), "cart.p8", text), text)
		time.Sleep(100 * time.Millisecond)
		if got := client.codeLensRefreshHits.Load(); got != 0 {
			t.Fatalf("refresh count = %d, want 0", got)
		}
		h.exit(t, true)
	})

	t.Run("disabled setting", func(t *testing.T) {
		client := newTestClient()
		h := newHarness(t, client)
		h.srv.codeLensRefreshDelay = 10 * time.Millisecond
		h.initialize(t, &protocol.InitializeParams{
			Capabilities:          m33CodeLensCapabilities(true),
			InitializationOptions: protocol.LSPAny([]byte(`{"pico8":{"limits":{"codeLens":false}}}`)),
		})
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		text := "x=1\n"
		openDoc(t, h, ctx, writeTemp(t, t.TempDir(), "cart.p8", text), text)
		time.Sleep(100 * time.Millisecond)
		if got := client.codeLensRefreshHits.Load(); got != 0 {
			t.Fatalf("refresh count = %d, want 0", got)
		}
		h.exit(t, true)
	})
}

func TestM33CodeLensRefreshStopsOnShutdown(t *testing.T) {
	client := newTestClient()
	h := newHarness(t, client)
	h.srv.codeLensRefreshDelay = 200 * time.Millisecond
	h.initialize(t, &protocol.InitializeParams{Capabilities: m33CodeLensCapabilities(true)})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	text := "x=1\n"
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	openDoc(t, h, ctx, root, text)
	select {
	case <-client.codeLensRefresh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial code lens refresh")
	}

	if err := h.server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: root},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangeWholeDocument{Text: text},
		},
	}); err != nil {
		t.Fatalf("didChange: %v", err)
	}
	h.srv.refreshMu.Lock()
	pending := len(h.srv.refreshTimers)
	h.srv.refreshMu.Unlock()
	if pending == 0 {
		t.Fatal("no refresh timer was pending before shutdown")
	}
	if err := h.server.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	before := client.codeLensRefreshHits.Load()
	time.Sleep(300 * time.Millisecond)
	if got := client.codeLensRefreshHits.Load(); got != before {
		t.Fatalf("refresh count after shutdown = %d, want %d", got, before)
	}
	h.exit(t, false)
}
