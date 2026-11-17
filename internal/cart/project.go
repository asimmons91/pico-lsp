package cart

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"go.lsp.dev/uri"
)

// Resolver determines which cart owns a document. A .p8 owns itself; a .lua
// file belongs to the nearest ancestor .p8 that includes it, or is a
// standalone one-file cart.
type Resolver struct {
	loader *Loader
	mu     sync.Mutex
	cache  map[uri.URI]uri.URI
}

// NewResolver creates an on-demand ownership resolver.
func NewResolver(loader *Loader) *Resolver {
	return &Resolver{loader: loader, cache: make(map[uri.URI]uri.URI)}
}

// Resolve returns the root URI for u.
func (r *Resolver) Resolve(u uri.URI) uri.URI {
	if !isLuaURI(u) {
		return u
	}

	r.mu.Lock()
	if root, ok := r.cache[u]; ok {
		r.mu.Unlock()
		return root
	}
	r.mu.Unlock()

	root := r.findOwner(u)
	r.mu.Lock()
	r.cache[u] = root
	r.mu.Unlock()
	return root
}

// Invalidate clears cached ownership after an open document or cart changes.
func (r *Resolver) Invalidate() {
	r.mu.Lock()
	r.cache = make(map[uri.URI]uri.URI)
	r.mu.Unlock()
}

func (r *Resolver) findOwner(target uri.URI) uri.URI {
	dir := filepath.Dir(target.FsPath())
	for {
		candidates, err := r.loader.Provider.List(uri.File(dir))
		if err == nil {
			sort.Slice(candidates, func(i, j int) bool {
				return candidates[i].String() < candidates[j].String()
			})
			var matches []uri.URI
			for _, candidate := range candidates {
				if !strings.HasSuffix(strings.ToLower(candidate.Path()), ".p8") {
					continue
				}
				if r.cartIncludes(candidate, target) {
					matches = append(matches, candidate)
				}
			}
			if len(matches) > 0 {
				if len(matches) > 1 {
					r.loader.Log.Debug("multiple carts include file; using nearest",
						"file", target.String(), "chosen", matches[0].String(), "count", len(matches))
				}
				return matches[0]
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return target
}

func (r *Resolver) cartIncludes(cartURI, target uri.URI) bool {
	if size, ok := r.loader.sizeOf(cartURI); ok && size > int64(r.loader.maxSize()) {
		return false
	}
	max := r.loader.maxSize()
	data, err := r.loader.readFile(cartURI, max)
	if err != nil || len(data) > max {
		return false
	}
	text := string(data)
	var code []lineRef
	if strings.HasSuffix(strings.ToLower(cartURI.Path()), ".p8") {
		_, code, _ = parseSections(text)
	} else {
		code = splitLines(text)
	}
	dir := filepath.Dir(cartURI.FsPath())
	var state scanState
	for _, line := range code {
		content := lineContent(text, line)
		if path, ok := includeDirective(content); ok && state.outside() {
			resolved, err := resolveInclude(dir, path)
			if err == nil && resolved == target {
				return true
			}
			continue
		}
		state.scan(content)
	}
	return false
}

func isLuaURI(u uri.URI) bool {
	return strings.HasSuffix(strings.ToLower(u.Path()), ".lua")
}
