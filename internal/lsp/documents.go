package lsp

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/source"
)

// Document is an immutable snapshot of an open document. Requests receive a
// copy and never mutate the store directly.
type Document struct {
	URI        uri.URI
	LanguageID string
	Version    int32
	Text       string
	Index      *source.LineIndex
}

// DocumentStore tracks open documents and applies full or incremental edits.
// It is safe for concurrent use.
type DocumentStore struct {
	mu       sync.RWMutex
	docs     map[uri.URI]Document
	encoding protocol.PositionEncodingKind
	log      *slog.Logger
}

// NewDocumentStore creates an empty store using encoding for LSP columns.
func NewDocumentStore(encoding protocol.PositionEncodingKind, log *slog.Logger) *DocumentStore {
	if log == nil {
		log = slog.Default()
	}
	return &DocumentStore{
		docs:     make(map[uri.URI]Document),
		encoding: normalizeEncoding(encoding),
		log:      log,
	}
}

// SetEncoding updates the negotiated position encoding and rebuilds indexes for
// any documents that are already open.
func (s *DocumentStore) SetEncoding(encoding protocol.PositionEncodingKind) {
	encoding = normalizeEncoding(encoding)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.encoding == encoding {
		return
	}
	s.encoding = encoding
	for u, doc := range s.docs {
		doc.Index = source.New([]byte(doc.Text), encoding)
		s.docs[u] = doc
	}
}

// Encoding returns the active position encoding.
func (s *DocumentStore) Encoding() protocol.PositionEncodingKind {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.encoding
}

// Open adds or replaces a document.
func (s *DocumentStore) Open(u uri.URI, languageID string, version int32, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.docs[u] = Document{
		URI:        u,
		LanguageID: languageID,
		Version:    version,
		Text:       text,
		Index:      source.New([]byte(text), s.encoding),
	}
}

// Change applies content changes in order and stores the client's resulting
// version.
func (s *DocumentStore) Change(u uri.URI, version int32, changes []protocol.TextDocumentContentChangeEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, ok := s.docs[u]
	if !ok {
		return fmt.Errorf("document is not open")
	}
	if version <= doc.Version {
		s.log.Warn("ignoring stale document change",
			"uri", u.String(), "current", doc.Version, "received", version)
		return fmt.Errorf("stale document version %d (current %d)", version, doc.Version)
	}

	text := doc.Text
	index := doc.Index
	for i, change := range changes {
		next, err := applyChange(text, index, change)
		if err != nil {
			return fmt.Errorf("change %d: %w", i, err)
		}
		text = next
		index = source.New([]byte(text), s.encoding)
	}

	doc.Version = version
	doc.Text = text
	doc.Index = index
	s.docs[u] = doc
	return nil
}

// Save updates an open document's text when the client includes it in
// textDocument/didSave. It reports whether an open overlay was updated.
func (s *DocumentStore) Save(u uri.URI, text *string) bool {
	if text == nil {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	doc, ok := s.docs[u]
	if !ok {
		return false
	}

	doc.Text = *text
	doc.Index = source.New([]byte(*text), s.encoding)
	s.docs[u] = doc
	return true
}

// Close removes the open overlay for u.
func (s *DocumentStore) Close(u uri.URI) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.docs[u]; !ok {
		return false
	}
	delete(s.docs, u)
	return true
}

// Get returns a consistent snapshot of an open document.
func (s *DocumentStore) Get(u uri.URI) (Document, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc, ok := s.docs[u]
	return doc, ok
}

// Text returns the current text for an open document.
func (s *DocumentStore) Text(u uri.URI) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc, ok := s.docs[u]
	return doc.Text, ok
}

// List returns the URIs of all open documents.
func (s *DocumentStore) List() []uri.URI {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]uri.URI, 0, len(s.docs))
	for u := range s.docs {
		out = append(out, u)
	}
	return out
}

// Count returns the number of open documents.
func (s *DocumentStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.docs)
}

func applyChange(text string, index *source.LineIndex, change protocol.TextDocumentContentChangeEvent) (string, error) {
	switch c := change.(type) {
	case *protocol.TextDocumentContentChangeWholeDocument:
		return c.Text, nil

	case *protocol.TextDocumentContentChangePartial:
		start, end, err := index.RangeOffsets(c.Range)
		if err != nil {
			return "", err
		}

		var b strings.Builder
		b.Grow(len(text) - (end - start) + len(c.Text))
		b.WriteString(text[:start])
		b.WriteString(c.Text)
		b.WriteString(text[end:])
		return b.String(), nil

	case nil:
		return "", fmt.Errorf("missing content change")

	default:
		return "", fmt.Errorf("unsupported content change %T", change)
	}
}

func normalizeEncoding(encoding protocol.PositionEncodingKind) protocol.PositionEncodingKind {
	switch encoding {
	case protocol.PositionEncodingKindUTF8,
		protocol.PositionEncodingKindUTF16,
		protocol.PositionEncodingKindUTF32:
		return encoding
	default:
		return protocol.PositionEncodingKindUTF16
	}
}
