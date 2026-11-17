package lsp

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/p8png"
)

const (
	virtualCartScheme               = "pico8-cart:"
	RequestDecodeCart               = "pico8/decodeCart"
	pngDecodeFailed   jsonrpc2.Code = -32002
)

type pngEntry struct {
	source  uri.URI
	virtual uri.URI
	result  p8png.Result
	mtime   time.Time
	size    int64
}

type pngCache struct {
	mu      sync.Mutex
	entries map[uri.URI]pngEntry
}

func newPNGCache() *pngCache {
	return &pngCache{entries: make(map[uri.URI]pngEntry)}
}

func isPNGURI(u uri.URI) bool {
	return u.IsFile() && strings.HasSuffix(strings.ToLower(u.Path()), ".p8.png")
}

func isVirtualCartURI(u uri.URI) bool {
	return u.Scheme() == "pico8-cart"
}

// canonicalVirtualURI normalizes percent-encoded path components such as the
// colon in a Windows drive letter, which JSON/URI encoders may escape.
func canonicalVirtualURI(u uri.URI) uri.URI {
	if !isVirtualCartURI(u) {
		return u
	}
	path := u.Path()
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	return uri.URI(virtualCartScheme + path)
}

func virtualCartURI(source uri.URI) uri.URI {
	path := source.Path()
	if strings.HasSuffix(strings.ToLower(path), ".png") {
		path = path[:len(path)-len(".png")]
	}
	return uri.URI(virtualCartScheme + path)
}

func (c *pngCache) lookup(source uri.URI) (pngEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[source]
	return entry, ok
}

func (c *pngCache) store(entry pngEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[entry.source] = entry
}

func (c *pngCache) invalidate(source uri.URI) (virtual uri.URI, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[source]
	if ok {
		delete(c.entries, source)
	}
	return entry.virtual, ok
}

func (s *Server) decodePNG(source uri.URI) (pngEntry, error) {
	if !isPNGURI(source) {
		return pngEntry{}, fmt.Errorf("not a .p8.png cart: %s", source)
	}
	info, err := os.Stat(source.FsPath())
	if err != nil {
		return pngEntry{}, err
	}
	if cached, ok := s.png.lookup(source); ok && cached.size == info.Size() && cached.mtime.Equal(info.ModTime()) {
		s.carts.SetVirtualText(cached.virtual, cached.result.Text)
		return cached, nil
	}
	result, err := p8png.DecodeFile(source.FsPath())
	if err != nil {
		return pngEntry{}, err
	}
	entry := pngEntry{
		source:  source,
		virtual: virtualCartURI(source),
		result:  result,
		mtime:   info.ModTime(),
		size:    info.Size(),
	}
	s.png.store(entry)
	s.carts.SetVirtualText(entry.virtual, result.Text)
	return entry, nil
}

func (s *Server) snapshotForCart(ctx context.Context, u uri.URI) (*cart.Snapshot, error) {
	u = canonicalVirtualURI(u)
	if isPNGURI(u) {
		entry, err := s.decodePNG(u)
		if err != nil {
			return nil, err
		}
		snapshot, _, err := s.carts.LoadRoot(entry.virtual)
		return snapshot, err
	}
	if isVirtualCartURI(u) {
		snapshot, _, ok := s.carts.SnapshotFor(ctx, canonicalVirtualURI(u))
		if !ok {
			return nil, fmt.Errorf("decoded cart is not available: %s", u)
		}
		return snapshot, nil
	}
	snapshot, _, ok := s.carts.SnapshotFor(ctx, u)
	if !ok {
		return nil, fmt.Errorf("cart not found: %s", u)
	}
	return snapshot, nil
}

type decodeCartParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
}

type DecodeCartResult struct {
	SourceURI  uri.URI       `json:"sourceUri"`
	VirtualURI uri.URI       `json:"virtualUri"`
	Text       string        `json:"text"`
	Version    int           `json:"version"`
	Tabs       []TabInfo     `json:"tabs,omitempty"`
	Includes   []IncludeInfo `json:"includes,omitempty"`
}

func (s *Server) decodeCartRequest(ctx context.Context, params protocol.LSPAny) (any, error) {
	var request decodeCartParams
	if err := protocol.Unmarshal(params, &request); err != nil || request.TextDocument.URI == "" {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "pico8/decodeCart: invalid params")
	}
	entry, err := s.decodePNG(request.TextDocument.URI)
	if err != nil {
		return nil, jsonrpc2.Errorf(pngDecodeFailed, "pico8/decodeCart: %v", err)
	}
	result := DecodeCartResult{
		SourceURI:  entry.source,
		VirtualURI: entry.virtual,
		Text:       entry.result.Text,
		Version:    entry.result.Version,
	}
	if snapshot, err := s.snapshotForCart(ctx, entry.virtual); err == nil && snapshot != nil {
		info := cartInfoFor(snapshot)
		result.Tabs = info.Tabs
		result.Includes = info.Includes
	}
	return result, nil
}

func (s *Server) invalidatePNG(source uri.URI) {
	virtual, ok := s.png.invalidate(source)
	if !ok {
		return
	}
	if text, ok := s.docs.Text(virtual); ok && text != "" {
		// Keep the virtual document alive but refresh its cache on the next
		// request. The helper's reload command replaces the buffer text.
		s.carts.RemoveVirtualText(virtual)
		return
	}
	s.carts.RemoveVirtualText(virtual)
}
