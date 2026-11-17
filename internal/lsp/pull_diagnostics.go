package lsp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/config"
)

// errDiagnosticDocument marks a diagnostic pull for a document that is neither
// open nor a registered virtual cart.
var errDiagnosticDocument = errors.New("diagnostic document is not available")

// maxWorkspaceDiagnosticDocuments bounds workspace pull reports. Workspace
// diagnostics cover open carts only; unopened carts are deliberately excluded.
const maxWorkspaceDiagnosticDocuments = 500

type pullFileDiagnostics struct {
	uri      uri.URI
	version  *int32
	items    []protocol.Diagnostic
	resultID string
}

// HasDocument reports whether u is an open document or a registered virtual
// cart. Pull requests for anything else are client errors.
func (m *CartManager) HasDocument(u uri.URI) bool {
	if _, ok := m.docs.Get(u); ok {
		return true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.virtual[u]
	return ok
}

// PullReport builds a full or unchanged report for u. Related documents are
// the other files owned by the same cart snapshot, with their original URIs.
func (m *CartManager) PullReport(ctx context.Context, u uri.URI, previousResultID *string) (protocol.DocumentDiagnosticReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, protocol.ErrRequestCancelled
	}

	entry, fileID, ok := m.entryFor(ctx, u)
	if !ok || entry.snap == nil || entry.snap.Cart == nil {
		return nil, fmt.Errorf("%w: %s", errDiagnosticDocument, u.String())
	}
	if err := ctx.Err(); err != nil {
		return nil, protocol.ErrRequestCancelled
	}

	reports := m.diagnosticReports(entry.snap, entry.semantics)
	requested, ok := reports[u]
	if !ok {
		file := entry.snap.Cart.File(fileID)
		if file == nil {
			return nil, fmt.Errorf("%w: %s", errDiagnosticDocument, u.String())
		}
		requested = m.fileDiagnostics(file, nil, m.settings())
	}

	related := make(map[uri.URI]protocol.FullDocumentDiagnosticReportOrUnchangedDocumentDiagnosticReport)
	for other, report := range reports {
		if other == u {
			continue
		}
		related[other] = &protocol.FullDocumentDiagnosticReport{
			Kind:     string(protocol.DocumentDiagnosticReportKindFull),
			ResultID: stringPtr(report.resultID),
			Items:    report.items,
		}
	}

	if previousResultID != nil && *previousResultID == requested.resultID {
		return &protocol.RelatedUnchangedDocumentDiagnosticReport{
			UnchangedDocumentDiagnosticReport: protocol.UnchangedDocumentDiagnosticReport{
				Kind:     string(protocol.DocumentDiagnosticReportKindUnchanged),
				ResultID: requested.resultID,
			},
			RelatedDocuments: related,
		}, nil
	}

	return &protocol.RelatedFullDocumentDiagnosticReport{
		FullDocumentDiagnosticReport: protocol.FullDocumentDiagnosticReport{
			Kind:     string(protocol.DocumentDiagnosticReportKindFull),
			ResultID: stringPtr(requested.resultID),
			Items:    requested.items,
		},
		RelatedDocuments: related,
	}, nil
}

// WorkspaceReport builds reports for every open cart. progress, when non-nil,
// receives batches as carts are processed so the caller can stream partial
// results. The returned report always contains the complete item list.
func (m *CartManager) WorkspaceReport(
	ctx context.Context,
	previous []protocol.PreviousResultId,
	progress func([]protocol.WorkspaceDocumentDiagnosticReport) error,
) (*protocol.WorkspaceDiagnosticReport, error) {
	previousIDs := make(map[uri.URI]string, len(previous))
	for _, item := range previous {
		previousIDs[item.URI] = item.Value
	}

	open := m.docs.List()
	sort.Slice(open, func(i, j int) bool { return open[i].String() < open[j].String() })

	seenRoots := make(map[uri.URI]struct{})
	items := make([]protocol.WorkspaceDocumentDiagnosticReport, 0)
	for _, u := range open {
		if err := ctx.Err(); err != nil {
			return nil, protocol.ErrRequestCancelled
		}

		entry, _, ok := m.entryFor(ctx, canonicalVirtualURI(u))
		if err := ctx.Err(); err != nil {
			return nil, protocol.ErrRequestCancelled
		}
		if !ok || entry.snap == nil || entry.snap.Cart == nil {
			continue
		}
		root := entry.snap.Cart.RootURI
		if _, seen := seenRoots[root]; seen {
			continue
		}
		seenRoots[root] = struct{}{}

		reports := m.diagnosticReports(entry.snap, entry.semantics)
		batch := make([]protocol.WorkspaceDocumentDiagnosticReport, 0, len(reports))
		for _, report := range sortedReports(reports) {
			if len(items)+len(batch) >= maxWorkspaceDiagnosticDocuments {
				m.log.Warn("truncating workspace diagnostics", "limit", maxWorkspaceDiagnosticDocuments)
				break
			}
			batch = append(batch, workspaceReport(report, previousIDs[report.uri]))
		}
		items = append(items, batch...)
		if progress != nil && len(batch) > 0 {
			if err := progress(batch); err != nil {
				return nil, err
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, protocol.ErrRequestCancelled
		}
		if len(items) >= maxWorkspaceDiagnosticDocuments {
			m.log.Warn("truncating workspace diagnostics", "limit", maxWorkspaceDiagnosticDocuments)
			break
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		return workspaceReportURI(items[i]) < workspaceReportURI(items[j])
	})
	return &protocol.WorkspaceDiagnosticReport{Items: items}, nil
}

func (m *CartManager) diagnosticReports(snap *cart.Snapshot, semantics *analysis.Result) map[uri.URI]pullFileDiagnostics {
	reports := make(map[uri.URI]pullFileDiagnostics)
	if snap == nil || snap.Cart == nil {
		return reports
	}
	items := m.collectDiagnostics(snap, semantics)
	settings := m.settings()
	for _, file := range snap.Cart.Files {
		if !m.ownsFile(snap.Cart.RootURI, file.URI) {
			continue
		}
		reports[file.URI] = m.fileDiagnostics(file, m.diags.Truncate(items[file.URI]), settings)
	}
	return reports
}

func (m *CartManager) fileDiagnostics(file *cart.File, items []protocol.Diagnostic, settings config.Settings) pullFileDiagnostics {
	if items == nil {
		items = []protocol.Diagnostic{}
	}
	return pullFileDiagnostics{
		uri:      file.URI,
		version:  m.documentVersion(file.URI),
		items:    items,
		resultID: diagnosticResultID(file, items, settings),
	}
}

func sortedReports(reports map[uri.URI]pullFileDiagnostics) []pullFileDiagnostics {
	out := make([]pullFileDiagnostics, 0, len(reports))
	for _, report := range reports {
		out = append(out, report)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].uri.String() < out[j].uri.String() })
	return out
}

func workspaceReport(report pullFileDiagnostics, previousResultID string) protocol.WorkspaceDocumentDiagnosticReport {
	if previousResultID != "" && previousResultID == report.resultID {
		return &protocol.WorkspaceUnchangedDocumentDiagnosticReport{
			UnchangedDocumentDiagnosticReport: protocol.UnchangedDocumentDiagnosticReport{
				Kind:     string(protocol.DocumentDiagnosticReportKindUnchanged),
				ResultID: report.resultID,
			},
			URI:     report.uri,
			Version: report.version,
		}
	}
	return &protocol.WorkspaceFullDocumentDiagnosticReport{
		FullDocumentDiagnosticReport: protocol.FullDocumentDiagnosticReport{
			Kind:     string(protocol.DocumentDiagnosticReportKindFull),
			ResultID: stringPtr(report.resultID),
			Items:    report.items,
		},
		URI:     report.uri,
		Version: report.version,
	}
}

func workspaceReportURI(report protocol.WorkspaceDocumentDiagnosticReport) string {
	switch r := report.(type) {
	case *protocol.WorkspaceFullDocumentDiagnosticReport:
		return r.URI.String()
	case *protocol.WorkspaceUnchangedDocumentDiagnosticReport:
		return r.URI.String()
	default:
		return ""
	}
}

// diagnosticResultID derives a stable ID from the file contents, the
// diagnostics, and every setting that can change diagnostic output. Transport
// mode is excluded: moving between push and pull does not change diagnostics.
func diagnosticResultID(file *cart.File, items []protocol.Diagnostic, settings config.Settings) string {
	settings.Diagnostics.Mode = ""

	h := sha256.New()
	_, _ = h.Write([]byte("pico-lsp-diagnostics-v1\x00"))
	_, _ = h.Write([]byte(file.URI.String()))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(file.Text))
	_, _ = h.Write([]byte{0})
	if data, err := json.Marshal(settings); err == nil {
		_, _ = h.Write(data)
	}
	_, _ = h.Write([]byte{0})
	for _, item := range items {
		if data, err := protocol.Marshal(item); err == nil {
			_, _ = h.Write(data)
		}
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func stringPtr(value string) *string { return &value }
