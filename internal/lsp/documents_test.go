package lsp

import (
	"io"
	"log/slog"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func partialChange(startLine, startChar, endLine, endChar uint32, text string) protocol.TextDocumentContentChangeEvent {
	return &protocol.TextDocumentContentChangePartial{
		Range: protocol.Range{
			Start: protocol.Position{Line: startLine, Character: startChar},
			End:   protocol.Position{Line: endLine, Character: endChar},
		},
		Text: text,
	}
}

func wholeChange(text string) protocol.TextDocumentContentChangeEvent {
	return &protocol.TextDocumentContentChangeWholeDocument{Text: text}
}

func TestDocumentStoreOpenGetClose(t *testing.T) {
	store := NewDocumentStore(protocol.PositionEncodingKindUTF16, quietLogger())
	u := uri.File("/tmp/cart.p8")

	store.Open(u, "pico8", 1, "print('hi')")
	doc, ok := store.Get(u)
	if !ok {
		t.Fatal("document not found after Open")
	}
	if doc.Text != "print('hi')" || doc.Version != 1 || doc.LanguageID != "pico8" {
		t.Fatalf("unexpected document: %+v", doc)
	}
	if doc.Index == nil || doc.Index.Len() != len(doc.Text) {
		t.Fatalf("unexpected index: %+v", doc.Index)
	}
	if store.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", store.Count())
	}

	if !store.Close(u) {
		t.Fatal("Close returned false for open document")
	}
	if _, ok := store.Get(u); ok {
		t.Fatal("document still present after Close")
	}
	if store.Count() != 0 {
		t.Fatalf("Count() = %d, want 0", store.Count())
	}
}

func TestDocumentStoreIncrementalEdits(t *testing.T) {
	tests := []struct {
		name    string
		initial string
		changes []protocol.TextDocumentContentChangeEvent
		want    string
	}{
		{
			name:    "insert at start",
			initial: "world",
			changes: []protocol.TextDocumentContentChangeEvent{partialChange(0, 0, 0, 0, "hello ")},
			want:    "hello world",
		},
		{
			name:    "insert in middle",
			initial: "helloworld",
			changes: []protocol.TextDocumentContentChangeEvent{partialChange(0, 5, 0, 5, " ")},
			want:    "hello world",
		},
		{
			name:    "insert at eof",
			initial: "hello",
			changes: []protocol.TextDocumentContentChangeEvent{partialChange(0, 5, 0, 5, "!")},
			want:    "hello!",
		},
		{
			name:    "delete",
			initial: "hello world",
			changes: []protocol.TextDocumentContentChangeEvent{partialChange(0, 5, 0, 11, "")},
			want:    "hello",
		},
		{
			name:    "replace",
			initial: "hello world",
			changes: []protocol.TextDocumentContentChangeEvent{partialChange(0, 6, 0, 11, "pico")},
			want:    "hello pico",
		},
		{
			name:    "multi edit in order",
			initial: "abc",
			changes: []protocol.TextDocumentContentChangeEvent{
				partialChange(0, 1, 0, 1, "X"),
				partialChange(0, 3, 0, 3, "Y"),
			},
			want: "aXbYc",
		},
		{
			name:    "multi line replace",
			initial: "one\ntwo\nthree",
			changes: []protocol.TextDocumentContentChangeEvent{partialChange(0, 1, 2, 5, "X")},
			want:    "oX",
		},
		{
			name:    "full text despite incremental",
			initial: "old",
			changes: []protocol.TextDocumentContentChangeEvent{wholeChange("brand new")},
			want:    "brand new",
		},
		{
			name:    "crlf line endings",
			initial: "a\r\nb\r\nc",
			changes: []protocol.TextDocumentContentChangeEvent{partialChange(1, 0, 1, 1, "B")},
			want:    "a\r\nB\r\nc",
		},
		{
			name:    "emoji column",
			initial: "a😀b",
			changes: []protocol.TextDocumentContentChangeEvent{partialChange(0, 3, 0, 4, "X")},
			want:    "a😀X",
		},
		{
			name:    "empty file",
			initial: "",
			changes: []protocol.TextDocumentContentChangeEvent{partialChange(0, 0, 0, 0, "hello")},
			want:    "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewDocumentStore(protocol.PositionEncodingKindUTF16, quietLogger())
			u := uri.File("/tmp/edit.p8")
			store.Open(u, "pico8", 1, tt.initial)

			if err := store.Change(u, 2, tt.changes); err != nil {
				t.Fatalf("Change: %v", err)
			}
			doc, _ := store.Get(u)
			if doc.Text != tt.want {
				t.Fatalf("text = %q, want %q", doc.Text, tt.want)
			}
			if doc.Version != 2 {
				t.Fatalf("version = %d, want 2", doc.Version)
			}
		})
	}
}

func TestDocumentStoreUTF8Columns(t *testing.T) {
	store := NewDocumentStore(protocol.PositionEncodingKindUTF8, quietLogger())
	u := uri.File("/tmp/utf8.p8")
	store.Open(u, "pico8", 1, "a😀b")

	if err := store.Change(u, 2, []protocol.TextDocumentContentChangeEvent{
		partialChange(0, 1, 0, 5, "X"),
	}); err != nil {
		t.Fatalf("Change: %v", err)
	}

	doc, _ := store.Get(u)
	if doc.Text != "aXb" {
		t.Fatalf("text = %q, want aXb", doc.Text)
	}
}

func TestDocumentStoreRejectsInvalidChangeAtomically(t *testing.T) {
	store := NewDocumentStore(protocol.PositionEncodingKindUTF16, quietLogger())
	u := uri.File("/tmp/edit.p8")
	store.Open(u, "pico8", 1, "hello")

	err := store.Change(u, 2, []protocol.TextDocumentContentChangeEvent{
		partialChange(0, 0, 0, 0, "X"),
		nil,
	})
	if err == nil {
		t.Fatal("Change succeeded, want error")
	}

	doc, _ := store.Get(u)
	if doc.Text != "hello" || doc.Version != 1 {
		t.Fatalf("document changed after failed edit: %+v", doc)
	}
}

func TestDocumentStoreUnknownDocument(t *testing.T) {
	store := NewDocumentStore(protocol.PositionEncodingKindUTF16, quietLogger())
	u := uri.File("/tmp/missing.p8")

	if err := store.Change(u, 2, nil); err == nil {
		t.Fatal("Change succeeded for unknown document")
	}
	if store.Close(u) {
		t.Fatal("Close succeeded for unknown document")
	}
}

func TestDocumentStoreSave(t *testing.T) {
	store := NewDocumentStore(protocol.PositionEncodingKindUTF16, quietLogger())
	u := uri.File("/tmp/save.p8")
	store.Open(u, "pico8", 1, "old")

	text := "new text"
	if !store.Save(u, &text) {
		t.Fatal("Save returned false for open document")
	}
	doc, _ := store.Get(u)
	if doc.Text != "new text" {
		t.Fatalf("text = %q, want new text", doc.Text)
	}
	if doc.Version != 1 {
		t.Fatalf("version = %d, want 1", doc.Version)
	}

	if store.Save(u, nil) {
		t.Fatal("Save with nil text returned true")
	}
	if store.Save(uri.File("/tmp/other.p8"), &text) {
		t.Fatal("Save returned true for unopened document")
	}
}

func TestDocumentStoreSetEncodingRebuildsIndexes(t *testing.T) {
	store := NewDocumentStore(protocol.PositionEncodingKindUTF16, quietLogger())
	u := uri.File("/tmp/emoji.p8")
	store.Open(u, "pico8", 1, "a😀b")

	doc, _ := store.Get(u)
	pos, _ := doc.Index.Position(5)
	if pos.Character != 3 {
		t.Fatalf("utf-16 character = %d, want 3", pos.Character)
	}

	store.SetEncoding(protocol.PositionEncodingKindUTF8)
	doc, _ = store.Get(u)
	pos, _ = doc.Index.Position(5)
	if pos.Character != 5 {
		t.Fatalf("utf-8 character = %d, want 5", pos.Character)
	}
	if store.Encoding() != protocol.PositionEncodingKindUTF8 {
		t.Fatalf("Encoding() = %q, want utf-8", store.Encoding())
	}
}

func TestDocumentStoreVersionRegressionStillApplies(t *testing.T) {
	store := NewDocumentStore(protocol.PositionEncodingKindUTF16, quietLogger())
	u := uri.File("/tmp/version.p8")
	store.Open(u, "pico8", 5, "abc")

	if err := store.Change(u, 4, []protocol.TextDocumentContentChangeEvent{wholeChange("xyz")}); err != nil {
		t.Fatalf("Change: %v", err)
	}
	doc, _ := store.Get(u)
	if doc.Text != "xyz" || doc.Version != 4 {
		t.Fatalf("document = %+v, want text xyz version 4", doc)
	}
}
