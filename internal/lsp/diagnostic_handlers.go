package lsp

import (
	"context"
	"errors"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// Diagnostic serves textDocument/diagnostic. Pull requests are answered in
// every mode; push publication is controlled separately.
func (s *Server) Diagnostic(ctx context.Context, params *protocol.DocumentDiagnosticParams) (protocol.DocumentDiagnosticReport, error) {
	if params == nil || params.TextDocument.URI == "" {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "textDocument/diagnostic: missing textDocument.uri")
	}
	if err := ctx.Err(); err != nil {
		return nil, protocol.ErrRequestCancelled
	}

	u := canonicalVirtualURI(params.TextDocument.URI)
	if !s.carts.HasDocument(u) {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "textDocument/diagnostic: document is not open: %s", u.String())
	}

	// A first pull can trigger a full cart load and analysis. Release the read
	// loop so cancellation and edits are processed while that work runs.
	jsonrpc2.Async(ctx)

	report, err := s.carts.PullReport(ctx, u, params.PreviousResultID)
	if err != nil {
		if errors.Is(err, errDiagnosticDocument) {
			return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "textDocument/diagnostic: %v", err)
		}
		return nil, err
	}

	if !s.relatedDocumentsSupported() {
		switch r := report.(type) {
		case *protocol.RelatedFullDocumentDiagnosticReport:
			r.RelatedDocuments = nil
		case *protocol.RelatedUnchangedDocumentDiagnosticReport:
			r.RelatedDocuments = nil
		}
	}
	return report, nil
}

// DiagnosticWorkspace serves workspace/diagnostic over open carts. Work is
// bounded and cancellable, and partial results are streamed when the client
// supplies a partial result token.
func (s *Server) DiagnosticWorkspace(ctx context.Context, params *protocol.WorkspaceDiagnosticParams) (*protocol.WorkspaceDiagnosticReport, error) {
	if params == nil {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "workspace/diagnostic: missing params")
	}
	if err := ctx.Err(); err != nil {
		return nil, protocol.ErrRequestCancelled
	}

	// Collecting several carts can take long enough that the client needs to
	// send cancellation or edits while the request runs.
	jsonrpc2.Async(ctx)

	client := s.Client()
	partial := params.PartialResultToken != nil && client != nil
	progress := func(batch []protocol.WorkspaceDocumentDiagnosticReport) error {
		if !partial {
			return nil
		}
		data, err := protocol.Marshal(&protocol.WorkspaceDiagnosticReportPartialResult{Items: batch})
		if err != nil {
			s.log.Warn("failed to encode workspace diagnostic partial result", "error", err)
			partial = false
			return nil
		}
		if err := client.Progress(ctx, &protocol.ProgressParams{
			Token: params.PartialResultToken,
			Value: protocol.LSPAny(data),
		}); err != nil {
			s.log.Warn("failed to send workspace diagnostic partial result", "error", err)
			partial = false
		}
		return nil
	}

	report, err := s.carts.WorkspaceReport(ctx, params.PreviousResultIds, progress)
	if err != nil {
		if errors.Is(err, protocol.ErrRequestCancelled) || ctx.Err() != nil {
			return nil, protocol.ErrRequestCancelled
		}
		return nil, err
	}
	if partial {
		// Everything was already streamed through $/progress.
		report.Items = []protocol.WorkspaceDocumentDiagnosticReport{}
	}
	return report, nil
}
