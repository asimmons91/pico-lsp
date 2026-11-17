package lsp

import (
	"context"

	"go.lsp.dev/protocol"
)

// DidOpen stores a new open document.
func (s *Server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	if params == nil {
		return nil
	}

	doc := params.TextDocument
	if !doc.URI.IsFile() && !isVirtualCartURI(doc.URI) {
		s.log.Warn("ignoring non-file document", "uri", doc.URI.String())
		return nil
	}

	s.docs.Open(doc.URI, string(doc.LanguageID), doc.Version, doc.Text)
	s.carts.Analyze(ctx, doc.URI)
	s.log.Debug("document opened",
		"uri", doc.URI.String(),
		"languageId", doc.LanguageID,
		"version", doc.Version,
		"bytes", len(doc.Text),
	)
	return nil
}

// DidChange applies incremental or full content changes in order.
func (s *Server) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	if params == nil {
		return nil
	}

	u := params.TextDocument.URI
	if isVirtualCartURI(u) {
		s.log.Debug("ignoring change for read-only virtual cart", "uri", u.String())
		return nil
	}
	if !u.IsFile() {
		s.log.Warn("ignoring change for non-file document", "uri", u.String())
		return nil
	}

	if err := s.docs.Change(u, params.TextDocument.Version, params.ContentChanges); err != nil {
		s.log.Warn("ignoring invalid document change",
			"uri", u.String(),
			"version", params.TextDocument.Version,
			"error", err,
		)
		return nil
	}

	s.carts.AnalyzeChange(ctx, u)
	s.log.Debug("document changed",
		"uri", u.String(),
		"version", params.TextDocument.Version,
		"changes", len(params.ContentChanges),
	)
	return nil
}

// DidSave updates the overlay only when the client includes the saved text.
func (s *Server) DidSave(ctx context.Context, params *protocol.DidSaveTextDocumentParams) error {
	if params == nil || params.Text == nil {
		return nil
	}

	u := params.TextDocument.URI
	if isVirtualCartURI(u) || !u.IsFile() {
		s.log.Debug("ignoring save for read-only or non-file document", "uri", u.String())
		return nil
	}

	if s.docs.Save(u, params.Text) {
		s.carts.Analyze(ctx, u)
		s.log.Debug("document saved with text", "uri", u.String(), "bytes", len(*params.Text))
	} else {
		s.log.Debug("ignoring save text for unopened document", "uri", u.String())
	}
	return nil
}

// DidClose removes the open overlay. Later milestones may fall back to disk
// content for analysis when the file still exists.
func (s *Server) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	if params == nil {
		return nil
	}

	u := params.TextDocument.URI
	if s.docs.Close(u) {
		if isVirtualCartURI(u) {
			s.carts.Forget(ctx, u)
			s.carts.RemoveVirtualText(u)
		} else {
			s.carts.Closed(ctx, u)
		}
		s.log.Debug("document closed", "uri", u.String())
	} else {
		s.log.Debug("closed document was not open", "uri", u.String())
	}
	return nil
}
