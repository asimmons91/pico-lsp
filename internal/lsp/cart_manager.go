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

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/config"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// CartManager loads and caches the cart that owns each open document, binds
// its symbols, and publishes diagnostics.
type CartManager struct {
	docs             *DocumentStore
	diags            *Diagnostics
	log              *slog.Logger
	settings         func() config.Settings
	workspaceFolders func() []uri.URI

	mu        sync.Mutex
	analyzeMu sync.Mutex
	loader    *cart.Loader
	resolver  *cart.Resolver
	builtins  map[string]*analysis.Symbol
	byRoot    map[uri.URI]*cartEntry
	byFile    map[uri.URI]uri.URI
	virtual   map[uri.URI]string
}

type cartEntry struct {
	snap      *cart.Snapshot
	semantics *analysis.Result
	files     map[uri.URI]struct{}
}

func NewCartManager(docs *DocumentStore, diags *Diagnostics, settings func() config.Settings, workspaceFolders func() []uri.URI, builtins map[string]*analysis.Symbol, log *slog.Logger) *CartManager {
	if log == nil {
		log = slog.Default()
	}
	if settings == nil {
		settings = func() config.Settings { return config.Default() }
	}
	if workspaceFolders == nil {
		workspaceFolders = func() []uri.URI { return nil }
	}
	m := &CartManager{
		docs:             docs,
		diags:            diags,
		settings:         settings,
		workspaceFolders: workspaceFolders,
		builtins:         builtins,
		log:              log,
		byRoot:           make(map[uri.URI]*cartEntry),
		byFile:           make(map[uri.URI]uri.URI),
		virtual:          make(map[uri.URI]string),
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
		ReadFunc:      m.readFile,
		ListFunc:      m.listDir,
		SizeFunc:      m.fileSize,
		ReadLimitFunc: m.readFileLimit,
	}
}

func (m *CartManager) textFor(u uri.URI) (string, bool) {
	if text, ok := m.docs.Text(u); ok {
		return text, true
	}
	m.mu.Lock()
	text, ok := m.virtual[u]
	m.mu.Unlock()
	return text, ok
}

func (m *CartManager) fileSize(u uri.URI) (int64, error) {
	if text, ok := m.textFor(u); ok {
		return int64(len(text)), nil
	}
	return cart.DiskProvider{}.Size(u)
}

func (m *CartManager) readFile(u uri.URI) ([]byte, error) {
	if text, ok := m.textFor(u); ok {
		return []byte(text), nil
	}
	return cart.DiskProvider{}.Read(u)
}

func (m *CartManager) readFileLimit(u uri.URI, max int) ([]byte, error) {
	if text, ok := m.textFor(u); ok {
		data := []byte(text)
		if max >= 0 && len(data) > max+1 {
			data = data[:max+1]
		}
		return data, nil
	}
	return cart.DiskProvider{}.ReadLimit(u, max)
}

// SetVirtualText registers decoded text for a non-file pseudo URI.
func (m *CartManager) SetVirtualText(u uri.URI, text string) {
	m.mu.Lock()
	m.virtual[u] = text
	m.mu.Unlock()
}

// RemoveVirtualText removes a registered pseudo document.
func (m *CartManager) RemoveVirtualText(u uri.URI) {
	m.mu.Lock()
	delete(m.virtual, u)
	m.mu.Unlock()
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
	m.analyzeMu.Lock()
	defer m.analyzeMu.Unlock()
	invalidate := isCartURI(u)
	root := m.analyzeLocked(ctx, u, invalidate)
	m.refreshAfterLocked(ctx, u, invalidate, root)
}

// AnalyzeChange re-analyzes u after an edit. Ownership only depends on `.p8`
// include lists, so editing a `.lua` file does not invalidate the resolver's
// cached ownership.
func (m *CartManager) AnalyzeChange(ctx context.Context, u uri.URI) {
	m.analyzeMu.Lock()
	defer m.analyzeMu.Unlock()
	invalidate := isCartURI(u)
	root := m.analyzeLocked(ctx, u, invalidate)
	m.refreshAfterLocked(ctx, u, invalidate, root)
}

// WatchedChange handles a filesystem create/change/delete event. Ownership may
// have changed, so the resolver is always invalidated.
func (m *CartManager) WatchedChange(ctx context.Context, u uri.URI) {
	m.analyzeMu.Lock()
	defer m.analyzeMu.Unlock()
	m.mu.Lock()
	resolver := m.resolver
	m.mu.Unlock()
	resolver.Invalidate()

	if resolver.Resolve(u) == u && !m.watchedFileKnown(u) {
		// Unknown standalone files are ignored, but existing carts may include
		// the new path or gain a new owner, so refresh dependents.
		m.refreshAfterLocked(ctx, u, isCartURI(u), "")
		return
	}

	root := m.analyzeLocked(ctx, u, false)
	m.refreshAfterLocked(ctx, u, isCartURI(u), root)
}

func (m *CartManager) watchedFileKnown(u uri.URI) bool {
	if _, open := m.docs.Get(u); open {
		return true
	}
	m.mu.Lock()
	_, known := m.byFile[u]
	m.mu.Unlock()
	return known
}

func isCartURI(u uri.URI) bool {
	return strings.HasSuffix(strings.ToLower(u.Path()), ".p8")
}

func isLuaURI(u uri.URI) bool {
	return strings.HasSuffix(strings.ToLower(u.Path()), ".lua")
}

// ownsFile reports whether root is the publishing owner for file. A .p8 owns
// itself; a .lua file is owned by the nearest ancestor cart that includes it.
// If no cart includes a .lua file, the file owns itself. When a cart includes a
// shared file that another cart owns, it must not publish competing diagnostics.
func ownsFile(resolver *cart.Resolver, root, file uri.URI) bool {
	if root == file {
		return true
	}
	if resolver == nil || !isLuaURI(file) {
		return false
	}
	owner := resolver.Resolve(file)
	return owner == root || owner == file
}

func (m *CartManager) ownsFile(root, file uri.URI) bool {
	m.mu.Lock()
	resolver := m.resolver
	m.mu.Unlock()
	return ownsFile(resolver, root, file)
}

func (m *CartManager) analyze(ctx context.Context, u uri.URI, invalidate bool) uri.URI {
	m.analyzeMu.Lock()
	defer m.analyzeMu.Unlock()
	return m.analyzeLocked(ctx, u, invalidate)
}

func (m *CartManager) analyzeLocked(ctx context.Context, u uri.URI, invalidate bool) uri.URI {
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
		return root
	}

	entry := &cartEntry{
		snap:      snap,
		semantics: analysis.Analyze(snap, m.builtins, analysisOptions(m.settings())),
		files:     make(map[uri.URI]struct{}, len(snap.Cart.Files)),
	}
	owned := make(map[uri.URI]bool, len(snap.Cart.Files))
	for _, file := range snap.Cart.Files {
		owned[file.URI] = ownsFile(resolver, root, file.URI)
	}
	m.mu.Lock()
	previous := m.byRoot[root]
	for _, file := range snap.Cart.Files {
		entry.files[file.URI] = struct{}{}
		if owned[file.URI] {
			m.byFile[file.URI] = root
		} else if m.byFile[file.URI] == root {
			delete(m.byFile, file.URI)
		}
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

	m.publish(ctx, snap, entry.semantics, previous)
	return root
}

// AnalyzeOpen re-analyzes every open document. It is used after the
// initialized notification, because clients may send didOpen before that point
// and diagnostics are intentionally gated until initialization completes.
func (m *CartManager) AnalyzeOpen(ctx context.Context) {
	for _, u := range m.docs.List() {
		_ = m.analyze(ctx, u, false)
	}
}

// Forget removes a cached pseudo document and its diagnostics.
func (m *CartManager) Forget(ctx context.Context, u uri.URI) {
	m.analyzeMu.Lock()
	defer m.analyzeMu.Unlock()
	m.mu.Lock()
	entry := m.byRoot[u]
	delete(m.byRoot, u)
	for file, owner := range m.byFile {
		if owner == u {
			delete(m.byFile, file)
		}
	}
	m.mu.Unlock()
	if entry != nil {
		for file := range entry.files {
			m.diags.Clear(ctx, file)
		}
	}
}

// Closed handles didClose. If a standalone document was never saved and has no
// owning cart, its diagnostics and cache entry are removed instead of being
// kept as a stale snapshot.
func (m *CartManager) Closed(ctx context.Context, u uri.URI) {
	m.analyzeMu.Lock()
	defer m.analyzeMu.Unlock()

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
					m.analyzeLocked(ctx, file, false)
				} else if ownsFile(resolver, u, file) {
					m.diags.Clear(ctx, file)
				}
			}
			m.refreshAfterLocked(ctx, u, isCartURI(u), "")
			return
		}
	}
	root = m.analyzeLocked(ctx, u, false)
	m.refreshAfterLocked(ctx, u, isCartURI(u), root)
}

// refreshAfterLocked updates analyses that depended on a changed document.
// The caller must hold analyzeMu.
func (m *CartManager) refreshAfterLocked(ctx context.Context, changed uri.URI, cartChanged bool, analyzedRoot uri.URI) {
	if cartChanged {
		// A cart's include list can change ownership for open .lua files and
		// change which open cart should publish a shared file.
		for _, u := range m.docs.List() {
			if isLuaURI(u) || isCartURI(u) {
				m.analyzeLocked(ctx, u, false)
			}
		}
	}
	m.refreshDependentsLocked(ctx, changed, analyzedRoot)
}

const maxDependentRefreshes = 64

// refreshDependentsLocked reloads cached roots whose flattened snapshot
// includes changed, then walks newly updated roots the same way. The caller
// must hold analyzeMu.
func (m *CartManager) refreshDependentsLocked(ctx context.Context, changed, analyzedRoot uri.URI) {
	queue := []uri.URI{changed}
	seen := map[uri.URI]bool{changed: true}
	if analyzedRoot != "" && analyzedRoot != changed && !seen[analyzedRoot] {
		seen[analyzedRoot] = true
		queue = append(queue, analyzedRoot)
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, root := range m.dependents(cur) {
			if seen[root] {
				continue
			}
			seen[root] = true
			if len(seen) > maxDependentRefreshes {
				m.log.Warn("stopping dependent cart refresh", "file", changed.String(), "limit", maxDependentRefreshes)
				return
			}
			m.analyzeLocked(ctx, root, false)
			queue = append(queue, root)
		}
	}
}

func (m *CartManager) dependents(u uri.URI) []uri.URI {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []uri.URI
	for root, entry := range m.byRoot {
		if root == u {
			continue
		}
		for _, include := range entry.snap.Cart.Includes {
			if include.ResolvedURI == u {
				out = append(out, root)
				break
			}
		}
	}
	return out
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

// collectDiagnostics builds the canonical diagnostic set for one analyzed
// cart snapshot. Every diagnostic stays attached to its original file URI.
func (m *CartManager) collectDiagnostics(snap *cart.Snapshot, semantics *analysis.Result) map[uri.URI][]protocol.Diagnostic {
	items := make(map[uri.URI][]protocol.Diagnostic)
	if snap == nil || snap.Cart == nil {
		return items
	}
	settings := m.settings()
	items[snap.Cart.RootURI] = append(items[snap.Cart.RootURI], cartLimitDiagnostics(snap, settings)...)
	for _, diag := range m.includeDiagnostics(snap, settings) {
		file := snap.Cart.File(diag.span.FileID)
		if file == nil {
			continue
		}
		items[file.URI] = append(items[file.URI], newDiagnostic(file.Index, diag.span.Start, diag.span.End, diag.severity, diag.code, diag.message))
	}
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
		add(narrowIncludeDiagnosticSpan(snap, diag.Span), severity, diag.Code, diag.Message)
	}
	for _, parseErr := range snap.Errors {
		add(parseErr.Span, protocol.DiagnosticSeverityError, syntax.DiagnosticCode, parseErr.Message)
	}
	if semantics != nil {
		for _, diag := range semantics.Diagnostics {
			add(diag.Span, analysisSeverity(diag.Severity), diag.Code, diag.Message)
		}
	}

	for _, fileItems := range items {
		sort.SliceStable(fileItems, func(i, j int) bool {
			if fileItems[i].Range.Start.Line != fileItems[j].Range.Start.Line {
				return fileItems[i].Range.Start.Line < fileItems[j].Range.Start.Line
			}
			return fileItems[i].Range.Start.Character < fileItems[j].Range.Start.Character
		})
	}
	return items
}

func (m *CartManager) publish(ctx context.Context, snap *cart.Snapshot, semantics *analysis.Result, previous *cartEntry) {
	if snap == nil || snap.Cart == nil {
		return
	}
	items := m.collectDiagnostics(snap, semantics)

	for _, file := range snap.Cart.Files {
		if !m.ownsFile(snap.Cart.RootURI, file.URI) {
			m.log.Debug("skipping diagnostics for non-owned file",
				"file", file.URI.String(), "root", snap.Cart.RootURI.String())
			continue
		}
		fileItems := items[file.URI]
		if len(fileItems) == 0 {
			if m.diags.Has(file.URI) {
				m.diags.Clear(ctx, file.URI)
			}
			continue
		}
		m.diags.Publish(ctx, file.URI, m.documentVersion(file.URI), fileItems)
	}

	if previous != nil {
		for file := range previous.files {
			if snap.Cart.FileID(file) < 0 && m.ownsFile(snap.Cart.RootURI, file) && m.diags.Has(file) {
				m.diags.Clear(ctx, file)
			}
		}
	}
}

// SnapshotFor returns the cached cart snapshot containing u, loading it on
// demand if necessary.
func (m *CartManager) SnapshotFor(ctx context.Context, u uri.URI) (*cart.Snapshot, source.FileID, bool) {
	entry, id, ok := m.entryFor(ctx, u)
	if !ok {
		return nil, -1, false
	}
	return entry.snap, id, true
}

// SemanticsFor returns the cached snapshot and symbol model containing u.
func (m *CartManager) SemanticsFor(ctx context.Context, u uri.URI) (*cart.Snapshot, *analysis.Result, source.FileID, bool) {
	entry, id, ok := m.entryFor(ctx, u)
	if !ok {
		return nil, nil, -1, false
	}
	return entry.snap, entry.semantics, id, true
}

func (m *CartManager) entryFor(ctx context.Context, u uri.URI) (*cartEntry, source.FileID, bool) {
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
		_ = m.analyze(ctx, u, false)
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
	return entry, id, true
}

// LoadRoot loads and binds a cart without adding it to the open-document cache.
// Workspace symbol scans use it so they honor overlays without evicting cached
// analysis for open files.
func (m *CartManager) LoadRoot(root uri.URI) (*cart.Snapshot, *analysis.Result, error) {
	m.mu.Lock()
	loader := m.loader
	builtins := m.builtins
	settings := m.settings
	m.mu.Unlock()

	snap, err := loader.Load(root)
	if err != nil {
		return nil, nil, err
	}
	return snap, analysis.Analyze(snap, builtins, analysisOptions(settings())), nil
}

// Resolver returns the ownership resolver for workspace scans.
func (m *CartManager) Resolver() *cart.Resolver {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resolver
}
