package lsp

import (
	"testing"

	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/source"
)

func TestNewDiagnosticUsesUTF16Columns(t *testing.T) {
	text := []byte("⬅️x = 1 +")
	index := source.New(text, protocol.PositionEncodingKindUTF16)
	xOffset := len("⬅️")
	diag := newDiagnostic(index, xOffset, xOffset+1, protocol.DiagnosticSeverityError, "syntax", "bad")

	if diag.Range.Start.Line != 0 {
		t.Fatalf("line = %d, want 0", diag.Range.Start.Line)
	}
	if diag.Range.Start.Character != 2 {
		t.Fatalf("character = %d, want 2 UTF-16 units before x", diag.Range.Start.Character)
	}
	if diag.Range.End.Character != 3 {
		t.Fatalf("end character = %d, want 3", diag.Range.End.Character)
	}
}
