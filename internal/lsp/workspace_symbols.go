package lsp

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
)

const (
	maxWorkspaceSymbolResults = 500
	maxWorkspaceSymbolFiles   = 2000
	workspaceSymbolTimeout    = 5 * time.Second
)

// Symbols implements workspace/symbol with a lazy, bounded scan.
func (s *Server) Symbols(ctx context.Context, params *protocol.WorkspaceSymbolParams) (protocol.WorkspaceSymbolResult, error) {
	if params == nil {
		return protocol.SymbolInformationSlice{}, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, protocol.ErrRequestCancelled
	}
	// The scan can take seconds. Release the read loop so cancellation
	// notifications and edits can be processed while it runs.
	jsonrpc2.Async(ctx)

	query := analysis.CanonicalName(params.Query)
	deadline := time.Now().Add(workspaceSymbolTimeout)

	roots, scanLimited := s.workspaceSymbolRoots(ctx, deadline)
	results := make([]protocol.SymbolInformation, 0)
	seen := make(map[string]struct{})
	truncated := scanLimited
	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return nil, protocol.ErrRequestCancelled
		}
		if len(results) >= maxWorkspaceSymbolResults {
			truncated = true
			break
		}
		if time.Now().After(deadline) {
			truncated = true
			break
		}
		snap, semantics, err := s.carts.LoadRoot(root)
		if err != nil {
			s.log.Debug("workspace symbol cart load failed", "root", root.String(), "error", err)
			continue
		}
		for _, sym := range semantics.Table.Symbols {
			if sym.Builtin || !workspaceSymbolKind(sym.Kind) {
				continue
			}
			if !fuzzyMatch(query, sym.Canonical) || sym.DeclSpan.IsEmpty() {
				continue
			}
			file := snap.Cart.File(sym.DeclSpan.FileID)
			if file == nil || file.Index == nil {
				continue
			}
			location := protocol.Location{
				URI:   file.URI,
				Range: file.Index.Range(sym.DeclSpan.Start, sym.DeclSpan.End),
			}
			key := sym.Name + "\x00" + location.URI.String() + "\x00" +
				fmt.Sprintf("%d:%d", location.Range.Start.Line, location.Range.Start.Character)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			container := workspaceContainer(snap, root, sym.DeclSpan)
			results = append(results, protocol.SymbolInformation{
				BaseSymbolInformation: protocol.BaseSymbolInformation{
					Name:          sym.Name,
					Kind:          protocolSymbolKind(sym.Kind),
					ContainerName: &container,
				},
				Location: location,
			})
			if len(results) >= maxWorkspaceSymbolResults {
				truncated = true
				break
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, protocol.ErrRequestCancelled
	}
	if truncated {
		s.log.Warn("workspace symbol results truncated",
			"count", len(results),
			"limit", maxWorkspaceSymbolResults,
			"files", maxWorkspaceSymbolFiles,
			"timeout", workspaceSymbolTimeout,
			"scanLimited", scanLimited,
		)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Name != results[j].Name {
			return results[i].Name < results[j].Name
		}
		if results[i].Location.URI != results[j].Location.URI {
			return results[i].Location.URI.String() < results[j].Location.URI.String()
		}
		if results[i].Location.Range.Start.Line != results[j].Location.Range.Start.Line {
			return results[i].Location.Range.Start.Line < results[j].Location.Range.Start.Line
		}
		return results[i].Location.Range.Start.Character < results[j].Location.Range.Start.Character
	})
	return protocol.SymbolInformationSlice(results), nil
}

func workspaceSymbolKind(kind analysis.SymbolKind) bool {
	switch kind {
	case analysis.SymbolGlobal, analysis.SymbolFunction, analysis.SymbolMethod, analysis.SymbolLabel, analysis.SymbolClass:
		return true
	default:
		return false
	}
}

func (s *Server) workspaceSymbolRoots(ctx context.Context, deadline time.Time) ([]uri.URI, bool) {
	folders := s.WorkspaceFolders()
	if len(folders) == 0 {
		return s.openDocumentRoots(ctx), false
	}

	resolver := s.carts.Resolver()
	seen := make(map[uri.URI]struct{})
	limited := false
	add := func(root uri.URI) {
		if root != "" {
			seen[root] = struct{}{}
		}
	}

	var carts, luas []string
	files := 0
	for _, folder := range folders {
		if ctx.Err() != nil || time.Now().After(deadline) {
			limited = true
			break
		}
		root := folder.FsPath()
		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if ctx.Err() != nil || time.Now().After(deadline) {
				limited = true
				return fs.SkipAll
			}
			if entry.IsDir() {
				name := entry.Name()
				if path != root && (strings.HasPrefix(name, ".") || name == "node_modules") {
					return fs.SkipDir
				}
				return nil
			}
			files++
			if files > maxWorkspaceSymbolFiles {
				limited = true
				return fs.SkipAll
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".p8":
				carts = append(carts, path)
			case ".lua":
				luas = append(luas, path)
			}
			return nil
		})
	}

	// Load carts first so their included .lua files resolve to the cart rather
	// than appearing as duplicate standalone roots.
	for _, path := range carts {
		add(uri.File(path))
	}
	for _, path := range luas {
		if ctx.Err() != nil || time.Now().After(deadline) {
			limited = true
			break
		}
		add(resolver.Resolve(uri.File(path)))
	}

	out := make([]uri.URI, 0, len(seen))
	for root := range seen {
		out = append(out, root)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out, limited
}

func (s *Server) openDocumentRoots(ctx context.Context) []uri.URI {
	resolver := s.carts.Resolver()
	seen := make(map[uri.URI]struct{})
	for _, u := range s.docs.List() {
		if ctx.Err() != nil {
			break
		}
		seen[resolver.Resolve(u)] = struct{}{}
	}
	out := make([]uri.URI, 0, len(seen))
	for root := range seen {
		out = append(out, root)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func workspaceContainer(snap *cart.Snapshot, root uri.URI, span source.Span) string {
	base := filepath.Base(root.FsPath())
	if snap == nil || snap.Cart == nil {
		return base
	}
	file := snap.Cart.File(span.FileID)
	if file == nil {
		return base
	}
	if !strings.HasSuffix(strings.ToLower(file.URI.Path()), ".p8") {
		// Included .lua files are part of the cart; show their path relative
		// to the cart directory rather than only the cart name.
		if rel, err := filepath.Rel(filepath.Dir(root.FsPath()), file.URI.FsPath()); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
		return base
	}
	line := 0
	if file.Index != nil {
		pos, err := file.Index.Position(span.Start)
		if err == nil {
			line = int(pos.Line)
		}
	}
	for _, tab := range snap.Cart.Tabs {
		if line >= tab.StartLine && line <= tab.EndLine {
			return base + " / " + tab.Name
		}
	}
	return base
}

func fuzzyMatch(query, name string) bool {
	if query == "" {
		return true
	}
	if strings.Contains(name, query) {
		return true
	}
	i := 0
	for j := 0; j < len(name) && i < len(query); j++ {
		if name[j] == query[i] {
			i++
		}
	}
	return i == len(query)
}
