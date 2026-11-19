package cart

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	internalcart "github.com/asimmons91/pico-lsp/internal/cart"
)

// Source supplies files and directory listings for Resolver. ReadDir returns
// full paths to the entries in a directory.
type Source interface {
	ReadFile(path string) ([]byte, error)
	ReadDir(path string) ([]string, error)
}

// DiskSource reads from the local filesystem.
type DiskSource struct{}

// ReadFile reads path from disk.
func (DiskSource) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// ReadDir returns the full paths of entries in dir.
func (DiskSource) ReadDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, filepath.Join(dir, entry.Name()))
	}
	return out, nil
}

// Resolver determines which cart owns a .lua file.
type Resolver struct {
	source Source

	mu    sync.Mutex
	cache map[string]string
}

// NewResolver creates a Resolver. A nil Source uses DiskSource.
func NewResolver(source Source) *Resolver {
	if source == nil {
		source = DiskSource{}
	}
	return &Resolver{source: source, cache: make(map[string]string)}
}

// Owner returns the cleaned path of a .p8 file or the nearest ancestor .p8
// that includes a .lua file. A standalone .lua file returns its own cleaned
// path. Owner only returns an error for an invalid path; unreadable carts are
// skipped. Results are cached until Invalidate.
//
// Resolver is safe for concurrent use.
func (r *Resolver) Owner(path string) (string, error) {
	cleaned, isCart, err := ownerPath(path)
	if err != nil || isCart {
		return cleaned, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cache == nil {
		r.cache = make(map[string]string)
	}
	if owner, ok := r.cache[cleaned]; ok {
		return owner, nil
	}
	owner := r.findOwner(cleaned)
	r.cache[cleaned] = owner
	return owner, nil
}

// Invalidate clears cached ownership results after a file tree or buffer
// change.
func (r *Resolver) Invalidate() {
	r.mu.Lock()
	r.cache = make(map[string]string)
	r.mu.Unlock()
}

func (r *Resolver) findOwner(target string) string {
	source := r.src()
	dir := filepath.Dir(target)
	for {
		entries, err := source.ReadDir(dir)
		if err == nil {
			sort.Strings(entries)
			for _, candidate := range entries {
				if !strings.EqualFold(filepath.Ext(candidate), ".p8") {
					continue
				}
				if r.cartOwns(candidate, target) {
					return filepath.Clean(candidate)
				}
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

func (r *Resolver) src() Source {
	if r.source == nil {
		return DiskSource{}
	}
	return r.source
}

func (r *Resolver) cartOwns(candidate, target string) bool {
	data, err := r.src().ReadFile(candidate)
	if err != nil || len(data) > internalcart.MaxFileSize {
		return false
	}
	doc, err := Parse(candidate, data)
	if err != nil {
		return false
	}
	for _, include := range doc.Includes() {
		if include.ResolvedPath == target {
			return true
		}
	}
	return false
}

func ownerPath(path string) (string, bool, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", false, fmt.Errorf("%w: %q", ErrInvalidPath, path)
	}
	cleaned := filepath.Clean(path)
	lower := strings.ToLower(cleaned)
	switch {
	case strings.HasSuffix(lower, ".p8"):
		return cleaned, true, nil
	case strings.HasSuffix(lower, ".lua"):
		return cleaned, false, nil
	default:
		return "", false, fmt.Errorf("%w: %q", ErrInvalidPath, path)
	}
}
