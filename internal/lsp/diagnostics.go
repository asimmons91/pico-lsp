package lsp

import (
	"context"
	"log/slog"
	"reflect"
	"slices"
	"sync"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Diagnostics publishes push diagnostics through the LSP client. It
// deduplicates consecutive identical publications and truncates to the
// configured problem limit.
type Diagnostics struct {
	client      func() protocol.Client
	maxProblems func() int
	log         *slog.Logger

	mu      sync.Mutex
	enabled bool
	last    map[uri.URI]publishedDiagnostics
}

type publishedDiagnostics struct {
	version *int32
	items   []protocol.Diagnostic
}

// NewDiagnostics creates a publisher. The client and maxProblems providers are
// evaluated at publish time so the server can be wired up before its
// connection exists.
func NewDiagnostics(client func() protocol.Client, maxProblems func() int, log *slog.Logger) *Diagnostics {
	if log == nil {
		log = slog.Default()
	}
	return &Diagnostics{
		client:      client,
		maxProblems: maxProblems,
		log:         log,
		last:        make(map[uri.URI]publishedDiagnostics),
	}
}

// Enable starts publishing. It must be called after the client sends the
// initialized notification.
func (d *Diagnostics) Enable() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.enabled = true
}

// Publish sends diagnostics for uri. Before initialized it is a no-op. Passing
// an empty slice clears the URI's diagnostics.
func (d *Diagnostics) Publish(ctx context.Context, u uri.URI, version *int32, items []protocol.Diagnostic) {
	if items == nil {
		items = []protocol.Diagnostic{}
	}

	max := 0
	if d.maxProblems != nil {
		max = d.maxProblems()
	}
	if max > 0 && len(items) > max {
		d.log.Warn("truncating diagnostics", "uri", u.String(), "count", len(items), "max", max)
		items = items[:max]
	}
	items = slices.Clone(items)

	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.enabled {
		return
	}

	if previous, ok := d.last[u]; ok && sameVersion(previous.version, version) && reflect.DeepEqual(previous.items, items) {
		return
	}

	client := d.client()
	if client == nil {
		return
	}

	params := &protocol.PublishDiagnosticsParams{
		URI:         u,
		Diagnostics: items,
	}
	if version != nil {
		params.Version = protocol.NewOptional(*version)
	}

	if err := client.PublishDiagnostics(ctx, params); err != nil {
		d.log.Warn("failed to publish diagnostics", "uri", u.String(), "error", err)
		return
	}

	d.last[u] = publishedDiagnostics{
		version: cloneVersion(version),
		items:   items,
	}
}

// Has reports whether a non-clearing diagnostics set was last published for u.
func (d *Diagnostics) Has(u uri.URI) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.last[u]
	return ok
}

// Clear removes any diagnostics for uri.
func (d *Diagnostics) Clear(ctx context.Context, u uri.URI) {
	d.Publish(ctx, u, nil, []protocol.Diagnostic{})
}

func sameVersion(a, b *int32) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func cloneVersion(v *int32) *int32 {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}
