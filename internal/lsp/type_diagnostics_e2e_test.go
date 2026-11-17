package lsp

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
)

func initializeWithOptions(t *testing.T, options string) *harness {
	t.Helper()
	h := newHarness(t, newTestClient())
	params := &protocol.InitializeParams{}
	if options != "" {
		params.InitializationOptions = protocol.LSPAny([]byte(options))
	}
	h.initialize(t, params)
	return h
}

func diagnosticCodes(params *protocol.PublishDiagnosticsParams) []string {
	out := make([]string, 0, len(params.Diagnostics))
	for _, diag := range params.Diagnostics {
		if code, ok := diag.Code.(protocol.String); ok {
			out = append(out, string(code))
		}
	}
	return out
}

func TestEndToEndTypeDiagnostics(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@param dx number
local function move(dx) end
move("left")
`
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := initializeWithOptions(t, `{"pico8":{"typeChecking":"basic"}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	params := waitDiagnostic(t, h.client, root)
	codes := diagnosticCodes(params)
	if !containsString(codes, "param-type-mismatch") {
		t.Fatalf("diagnostic codes = %v", codes)
	}
	for _, diag := range params.Diagnostics {
		if code, ok := diag.Code.(protocol.String); ok && string(code) == "param-type-mismatch" {
			if diag.Severity != protocol.DiagnosticSeverityWarning {
				t.Fatalf("severity = %v", diag.Severity)
			}
		}
	}
}

func TestEndToEndTypeCheckingModesAndSuppression(t *testing.T) {
	text := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@param p {x: number}?
local function use(p)
  local value = p.x
end
---@type {x: number}
local value = {}
---@type number
---@diagnostic disable-next-line: assign-type-mismatch
local suppressed = "bad"
`
	cases := []struct {
		name     string
		options  string
		wantCode string
		absent   []string
	}{
		{name: "off", options: `{"pico8":{"typeChecking":"off"}}`},
		{name: "basic", options: `{"pico8":{"typeChecking":"basic"}}`},
		{name: "strict", options: `{"pico8":{"typeChecking":"strict"}}`, wantCode: "need-check-nil", absent: []string{"assign-type-mismatch"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			root := writeTemp(t, dir, "cart.p8", text)
			h := initializeWithOptions(t, tc.options)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			openDoc(t, h, ctx, root, text)
			_, semantics, _, ok := h.srv.carts.SemanticsFor(ctx, root)
			if !ok {
				t.Fatal("missing semantics")
			}
			var codes []string
			for _, diag := range semantics.Diagnostics {
				codes = append(codes, diag.Code)
			}
			if tc.wantCode != "" && !containsString(codes, tc.wantCode) {
				t.Fatalf("diagnostics = %v, missing %s", codes, tc.wantCode)
			}
			for _, absent := range tc.absent {
				if containsString(codes, absent) {
					t.Fatalf("diagnostics = %v, unexpected %s", codes, absent)
				}
			}
			if tc.name == "off" {
				for _, code := range codes {
					if strings.HasPrefix(code, "annotation-") || strings.Contains(code, "type") || code == "need-check-nil" {
						t.Fatalf("off diagnostics = %v", codes)
					}
				}
			}
			for _, code := range codes {
				if code == "assign-type-mismatch" && strings.Contains(text, "suppressed") {
					// The only remaining assignment mismatch would be on the
					// suppressed declaration; the test text has no other one.
					t.Fatalf("suppressed diagnostic leaked: %v", codes)
				}
			}
		})
	}
}

func TestEndToEndTypeDiagnosticInInclude(t *testing.T) {
	dir := t.TempDir()
	cartText := `pico-8 cartridge // pico-8.com
version 42
__lua__
---@class Player
---@field x number
#include "lib.lua"
`
	libText := `---@type Player
local p = {x = 1}
p.x = "bad"
`
	root := writeTemp(t, dir, "cart.p8", cartText)
	lib := writeTemp(t, dir, "lib.lua", libText)
	h := initializeWithOptions(t, `{"pico8":{"typeChecking":"basic"}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, cartText)
	params := waitDiagnostic(t, h.client, lib)
	codes := diagnosticCodes(params)
	if !containsString(codes, "assign-type-mismatch") {
		t.Fatalf("include diagnostic codes = %v", codes)
	}
	if params.Diagnostics[0].Range.Start.Line != 2 {
		t.Fatalf("include diagnostic range = %+v, want line 2", params.Diagnostics[0].Range)
	}
}

func TestEndToEndTypeDiagnosticUTF16Range(t *testing.T) {
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n---@type number\nlocal x = \"é\"\n"
	dir := t.TempDir()
	root := writeTemp(t, dir, "cart.p8", text)
	h := initializeWithOptions(t, `{"pico8":{"typeChecking":"basic"}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	openDoc(t, h, ctx, root, text)
	params := waitDiagnostic(t, h.client, root)
	for _, diag := range params.Diagnostics {
		code, _ := diag.Code.(protocol.String)
		if string(code) != "assign-type-mismatch" {
			continue
		}
		length := diag.Range.End.Character - diag.Range.Start.Character
		if length != 3 {
			t.Fatalf("diagnostic UTF-16 length = %d, want 3 (%+v)", length, diag.Range)
		}
		return
	}
	t.Fatalf("missing assign-type-mismatch in %v", diagnosticCodes(params))
}
