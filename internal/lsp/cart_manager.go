package lsp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// CartManager loads and caches the cart that owns each open document and
// publishes its diagnostics.
type CartManager struct {
	docs  *DocumentStore
	diags *Diagnostics
	log   *slog.Logger

	mu        sync.Mutex
	analyzeMu sync.Mutex
	loader    *cart.Loader
	resolver  *cart.Resolver
	byRoot    map[uri.URI]*cartEntry
	byFile    map[uri.URI]uri.URI
}

type cartEntry struct {
	snap  *cart.Snapshot
	files map[uri.URI]struct{}
}

func NewCartManager(docs *DocumentStore, diags *Diagnostics, log *slog.Logger) *CartManager {
	if log == nil {
		log = slog.Default()
	}
	m := &CartManager{
		docs:   docs,
		diags:  diags,
		log:    log,
		byRoot: make(map[uri.URI]*cartEntry),
		byFile: make(map[uri.URI]uri.URI),
	}
	m.loader = cart.NewLoader(m.provider(), protocol.PositionEncodingKindUTF16, log)
	m.resolver = cart.NewResolver(m.loader)
	return m
}

func (m *CartManager) SetEncoding(encoding protocol.PositionEncodingKind) {
	m.analyzeMu.Lock()
	defer m.analyzeMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loader.Encoding = encoding
	m.resolver = cart.NewResolver(m.loader)
	m.byRoot = make(map[uri.URI]*cartEntry)
	m.byFile = make(map[uri.URI]uri.URI)
}

func (m *CartManager) provider() cart.Provider {
	return cart.ProviderFuncs{
		ReadFunc: m.readFile,
		ListFunc: m.listDir,
	}
}

func (m *CartManager) readFile(u uri.URI) ([]byte, error) {
	if text, ok := m.docs.Text(u); ok {
		return []byte(text), nil
	}
	return os.ReadFile(u.FsPath())
}

func (m *CartManager) listDir(u uri.URI) ([]uri.URI, error) {
	seen := make(map[uri.URI]struct{})
	var out []uri.URI
	add := func(candidate uri.URI) {
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}

	entries, err := os.ReadDir(u.FsPath())
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				add(uri.File(filepath.Join(u.FsPath(), entry.Name())))
			}
		}
	}
	dir := filepath.Clean(u.FsPath())
	for _, open := range m.docs.List() {
		if filepath.Dir(open.FsPath()) == dir {
			add(open)
		}
	}
	if err != nil && len(out) == 0 {
		return nil, err
	}
	return out, nil
}

// Analyze resolves the owning cart for u, reloads it, and publishes
// diagnostics for every file in the cart.
func (m *CartManager) Analyze(ctx context.Context, u uri.URI) {
	m.analyze(ctx, u, true)
}

// AnalyzeChange re-analyzes u after an edit. Ownership only depends on `.p8`
// include lists, so editing a `.lua` file does not invalidate the resolver's
// cached ownership.
func (m *CartManager) AnalyzeChange(ctx context.Context, u uri.URI) {
	invalidate := strings.HasSuffix(strings.ToLower(u.Path()), ".p8")
	m.analyze(ctx, u, invalidate)
}

func (m *CartManager) analyze(ctx context.Context, u uri.URI, invalidate bool) {
	m.analyzeMu.Lock()
	defer m.analyzeMu.Unlock()

	m.mu.Lock()
	resolver := m.resolver
	m.mu.Unlock()
	if invalidate {
		resolver.Invalidate()
	}
	root := resolver.Resolve(u)

	snap, err := m.loader.Load(root)
	if err != nil {
		m.mu.Lock()
		previous := m.byRoot[root]
		m.mu.Unlock()
		m.handleLoadError(ctx, u, root, previous, err)
		return
	}

	entry := &cartEntry{
		snap:  snap,
		files: make(map[uri.URI]struct{}, len(snap.Cart.Files)),
	}
	m.mu.Lock()
	previous := m.byRoot[root]
	for _, file := range snap.Cart.Files {
		entry.files[file.URI] = struct{}{}
		m.byFile[file.URI] = root
	}
	if previous != nil {
		for file := range previous.files {
			if _, ok := entry.files[file]; !ok && m.byFile[file] == root {
				delete(m.byFile, file)
			}
		}
	}
	m.byRoot[root] = entry
	m.mu.Unlock()

	m.publish(ctx, snap, previous)
}

// AnalyzeOpen re-analyzes every open document. It is used after the
// initialized notification, because clients may send didOpen before that point
// and diagnostics are intentionally gated until initialization completes.
func (m *CartManager) AnalyzeOpen(ctx context.Context) {
	for _, u := range m.docs.List() {
		m.analyze(ctx, u, false)
	}
}

// Closed handles didClose. If a standalone document was never saved and has no
// owning cart, its diagnostics and cache entry are removed instead of being
// kept as a stale snapshot.
func (m *CartManager) Closed(ctx context.Context, u uri.URI) {
	m.mu.Lock()
	resolver := m.resolver
	m.mu.Unlock()
	resolver.Invalidate()
	root := resolver.Resolve(u)

	if root == u {
		if _, err := os.Stat(u.FsPath()); err != nil {
			m.mu.Lock()
			var removed []uri.URI
			if entry := m.byRoot[u]; entry != nil {
				for file := range entry.files {
					removed = append(removed, file)
				}
			}
			delete(m.byRoot, u)
			for file, owner := range m.byFile {
				if owner == u {
					delete(m.byFile, file)
				}
			}
			m.mu.Unlock()
			for _, file := range removed {
				if file == u {
					m.diags.Clear(ctx, file)
					continue
				}
				if _, open := m.docs.Get(file); open {
					m.analyze(ctx, file, false)
				} else {
					m.diags.Clear(ctx, file)
				}
			}
			m.diags.Clear(ctx, u)
			return
		}
	}
	m.Analyze(ctx, u)
}

func (m *CartManager) handleLoadError(ctx context.Context, u, root uri.URI, previous *cartEntry, err error) {
	m.log.Warn("cart load failed", "file", u.String(), "root", root.String(), "error", err)
	if previous != nil {
		// Keep the previous navigation snapshot but surface the failure on the
		// cart root.
		diag := protocol.Diagnostic{
			Message:  protocol.String(fmt.Sprintf("unable to reload cart: %v", err)),
			Severity: protocol.DiagnosticSeverityError,
			Code:     protocol.String(cart.CodeIncludeUnreadable),
			Source:   protocol.NewOptional(DiagnosticSource),
		}
		version := m.documentVersion(root)
		m.diags.Publish(ctx, root, version, []protocol.Diagnostic{diag})
		return
	}
	m.diags.Clear(ctx, u)
}

func (m *CartManager) documentVersion(u uri.URI) *int32 {
	if doc, ok := m.docs.Get(u); ok {
		v := doc.Version
		return &v
	}
	return nil
}

func (m *CartManager) publish(ctx context.Context, snap *cart.Snapshot, previous *cartEntry) {
	items := make(map[uri.URI][]protocol.Diagnostic)
	add := func(span source.Span, severity protocol.DiagnosticSeverity, code, message string) {
		file := snap.Cart.File(span.FileID)
		if file == nil {
			return
		}
		items[file.URI] = append(items[file.URI], newDiagnostic(file.Index, span.Start, span.End, severity, code, message))
	}

	for _, diag := range snap.Cart.Diagnostics {
		severity := protocol.DiagnosticSeverityError
		if diag.Severity == cart.SeverityWarning {
			severity = protocol.DiagnosticSeverityWarning
		}
		add(diag.Span, severity, diag.Code, diag.Message)
	}
	for _, parseErr := range snap.Errors {
		add(parseErr.Span, protocol.DiagnosticSeverityError, syntax.DiagnosticCode, parseErr.Message)
	}

	for _, file := range snap.Cart.Files {
		fileItems := items[file.URI]
		if len(fileItems) == 0 {
			if m.diags.Has(file.URI) {
				m.diags.Clear(ctx, file.URI)
			}
			continue
		}
		sort.SliceStable(fileItems, func(i, j int) bool {
			if fileItems[i].Range.Start.Line != fileItems[j].Range.Start.Line {
				return fileItems[i].Range.Start.Line < fileItems[j].Range.Start.Line
			}
			return fileItems[i].Range.Start.Character < fileItems[j].Range.Start.Character
		})
		m.diags.Publish(ctx, file.URI, m.documentVersion(file.URI), fileItems)
	}

	if previous != nil {
		for file := range previous.files {
			if snap.Cart.FileID(file) < 0 && m.diags.Has(file) {
				m.diags.Clear(ctx, file)
			}
		}
	}
}

// SnapshotFor returns the cached cart snapshot containing u, loading it on
// demand if necessary.
func (m *CartManager) SnapshotFor(ctx context.Context, u uri.URI) (*cart.Snapshot, source.FileID, bool) {
	m.mu.Lock()
	resolver := m.resolver
	m.mu.Unlock()
	root := resolver.Resolve(u)

	m.mu.Lock()
	entry := m.byRoot[root]
	m.mu.Unlock()

	// A file can move between standalone and cart ownership. If the cached
	// standalone snapshot no longer matches the open document, reload it.
	if entry != nil && root == u && len(entry.snap.Cart.Files) > 0 {
		if text, ok := m.docs.Text(u); ok && entry.snap.Cart.Files[0].Text != text {
			entry = nil
		}
	}
	if entry == nil {
		m.analyze(ctx, u, false)
		m.mu.Lock()
		entry = m.byRoot[root]
		m.mu.Unlock()
	}
	if entry == nil {
		return nil, -1, false
	}
	id := entry.snap.Cart.FileID(u)
	if id < 0 {
		return nil, -1, false
	}
	return entry.snap, id, true
}
