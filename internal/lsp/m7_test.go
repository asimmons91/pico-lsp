package lsp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func TestFormattingCapability(t *testing.T) {
	h := newHarness(t, newTestClient())
	result := h.initialize(t, &protocol.InitializeParams{})
	if _, ok := result.Capabilities.DocumentFormattingProvider.(protocol.Boolean); !ok {
		t.Fatalf("documentFormattingProvider = %T, want Boolean", result.Capabilities.DocumentFormattingProvider)
	}
}

func TestEndToEndFormattingP8(t *testing.T) {
	text := strings.Join([]string{
		"pico-8 cartridge // pico-8.com",
		"version 42",
		"__lua__",
		`#include "lib.lua"`,
		"function f(a,b)",
		"if (a) then",
		"x=1",
		"else",
		"x=2",
		"end",
		"if (x) x+=1",
		"end",
		"__gfx__",
		"0000000000000000",
	}, "\n") + "\n"
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	writeTemp(t, dir, "lib.lua", "function lib() end\n")
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	edits, err := h.server.Formatting(ctx, &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Options:      protocol.FormattingOptions{TabSize: 2, InsertSpaces: true},
	})
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("edits = %+v", edits)
	}
	formatted := edits[0].NewText
	for _, want := range []string{
		"function f(a, b)",
		"    x = 1",
		"    x = 2",
		"  if (x) x += 1",
		"__gfx__\n0000000000000000\n",
		`#include "lib.lua"`,
	} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("formatted cart missing %q:\n%s", want, formatted)
		}
	}
	if strings.Contains(formatted, "__gfx__\n0000\n") {
		t.Fatalf("non-code section was modified:\n%s", formatted)
	}

	if err := h.server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: root},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangeWholeDocument{Text: formatted},
		},
	}); err != nil {
		t.Fatalf("apply formatting: %v", err)
	}
	again, err := h.server.Formatting(ctx, &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Options:      protocol.FormattingOptions{TabSize: 2, InsertSpaces: true},
	})
	if err != nil {
		t.Fatalf("second format: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("already formatted document returned edits: %+v", again)
	}
}

func TestEndToEndFormattingLuaTabs(t *testing.T) {
	text := "function f()\nx=1\nend\n"
	dir := t.TempDir()
	root := writeTemp(t, dir, "lib.lua", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	edits, err := h.server.Formatting(ctx, &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Options:      protocol.FormattingOptions{TabSize: 2, InsertSpaces: false},
	})
	if err != nil {
		t.Fatalf("format lua: %v", err)
	}
	if len(edits) != 1 || edits[0].NewText != "function f()\n\tx = 1\nend\n" {
		t.Fatalf("lua edit = %+v", edits)
	}
}

func TestEndToEndFormattingRefusesSyntaxErrors(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nfunction broken(\n"
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	_, err := h.server.Formatting(ctx, &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Options:      protocol.FormattingOptions{TabSize: 2, InsertSpaces: true},
	})
	var rpcErr *jsonrpc2.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != requestFailedCode {
		t.Fatalf("format error = %v, want RequestFailed", err)
	}
}

func TestEndToEndFormattingRefusesMissingInclude(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"missing.lua\"\nx=1\n"
	root := writeTemp(t, t.TempDir(), "cart.p8", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	_, err := h.server.Formatting(ctx, &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Options:      protocol.FormattingOptions{TabSize: 2, InsertSpaces: true},
	})
	var rpcErr *jsonrpc2.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != requestFailedCode {
		t.Fatalf("format error = %v, want RequestFailed", err)
	}
}

func TestFormattingRefusesPNG(t *testing.T) {
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := h.server.Formatting(ctx, &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///tmp/cart.p8.png"},
		Options:      protocol.FormattingOptions{TabSize: 2, InsertSpaces: true},
	})
	var rpcErr *jsonrpc2.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != requestFailedCode {
		t.Fatalf("format png error = %v, want RequestFailed", err)
	}
}

func TestEndToEndFormattingLuaSingleFinalNewline(t *testing.T) {
	text := "x=1\n\n\n"
	dir := t.TempDir()
	root := writeTemp(t, dir, "lib.lua", text)
	h := newHarness(t, newTestClient())
	h.initialize(t, &protocol.InitializeParams{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)

	edits, err := h.server.Formatting(ctx, &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Options:      protocol.FormattingOptions{TabSize: 2, InsertSpaces: true},
	})
	if err != nil {
		t.Fatalf("format lua: %v", err)
	}
	if len(edits) != 1 || edits[0].NewText != "x = 1\n" {
		t.Fatalf("lua edit = %+v, want one final newline", edits)
	}

	// An explicit false preserves the client's choice to keep extra newlines.
	edits, err = h.server.Formatting(ctx, &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: root},
		Options: protocol.FormattingOptions{
			TabSize:           2,
			InsertSpaces:      true,
			TrimFinalNewlines: boolPtr(false),
		},
	})
	if err != nil {
		t.Fatalf("format lua with preserved newlines: %v", err)
	}
	if len(edits) != 1 || edits[0].NewText != "x = 1\n\n\n" {
		t.Fatalf("lua preserved edit = %+v", edits)
	}
}
