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
// configured problem limit. Pull-capable clients can disable push publication;
// switching transports clears or republishes the tracked set so editors never
// keep stale diagnostics from the other transport.
type Diagnostics struct {
	client      func() protocol.Client
	maxProblems func() int
	log         *slog.Logger

	mu          sync.Mutex
	enabled     bool
	pushAllowed bool
	last        map[uri.URI]publishedDiagnostics
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
		pushAllowed: true,
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

// SetPushAllowed enables or disables push publication. Disabling pushes
// immediately clears every diagnostic set this publisher sent, so a move to
// pull diagnostics does not leave stale squiggles behind. Enabling pushes
// forgets the deduplication cache; the next analysis publishes one
// authoritative set.
func (d *Diagnostics) SetPushAllowed(ctx context.Context, allowed bool) {
	d.mu.Lock()
	if d.pushAllowed == allowed {
		d.mu.Unlock()
		return
	}
	d.pushAllowed = allowed
	if allowed {
		d.last = make(map[uri.URI]publishedDiagnostics)
		d.mu.Unlock()
		return
	}

	// Clear diagnostics while holding the lock so a concurrent publish cannot
	// race between the transport switch and the clear.
	for u, previous := range d.last {
		if len(previous.items) == 0 {
			delete(d.last, u)
			continue
		}
		if d.sendLocked(ctx, u, nil, []protocol.Diagnostic{}) {
			delete(d.last, u)
		}
	}
	d.mu.Unlock()
}

// Truncate applies the configured per-document problem limit. It always
// returns a copy so callers cannot mutate the publisher's or report's data.
func (d *Diagnostics) Truncate(items []protocol.Diagnostic) []protocol.Diagnostic {
	return d.truncate(items, "")
}

func (d *Diagnostics) truncateFor(u uri.URI, items []protocol.Diagnostic) []protocol.Diagnostic {
	return d.truncate(items, u.String())
}

func (d *Diagnostics) truncate(items []protocol.Diagnostic, u string) []protocol.Diagnostic {
	if items == nil {
		items = []protocol.Diagnostic{}
	}

	max := 0
	if d.maxProblems != nil {
		max = d.maxProblems()
	}
	if max > 0 && len(items) > max {
		d.log.Warn("truncating diagnostics", "uri", u, "count", len(items), "max", max)
		items = items[:max]
	}
	return slices.Clone(items)
}

// Publish sends diagnostics for uri. Before initialized or when push
// publication is disabled it is a no-op. Passing an empty slice clears the
// URI's diagnostics.
func (d *Diagnostics) Publish(ctx context.Context, u uri.URI, version *int32, items []protocol.Diagnostic) {
	items = d.truncate(items, u.String())

	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.enabled || !d.pushAllowed {
		return
	}

	if previous, ok := d.last[u]; ok && sameVersion(previous.version, version) && reflect.DeepEqual(previous.items, items) {
		return
	}

	if d.sendLocked(ctx, u, version, items) {
		d.last[u] = publishedDiagnostics{
			version: cloneVersion(version),
			items:   items,
		}
	}
}

// sendLocked publishes one diagnostics payload. The caller must hold d.mu.
func (d *Diagnostics) sendLocked(ctx context.Context, u uri.URI, version *int32, items []protocol.Diagnostic) bool {
	client := d.client()
	if client == nil {
		return false
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
		return false
	}
	return true
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
