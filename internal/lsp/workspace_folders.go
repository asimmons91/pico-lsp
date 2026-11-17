package lsp

import (
	"context"
	"sort"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func initializeFolders(params *protocol.InitializeParams) []uri.URI {
	if params == nil {
		return nil
	}
	var out []uri.URI
	if folders, ok := params.WorkspaceFolders.Get(); ok {
		for _, folder := range folders {
			if folder.URI.IsFile() {
				out = append(out, folder.URI)
			}
		}
	}
	if len(out) == 0 && params.RootURI != nil && params.RootURI.IsFile() {
		out = append(out, *params.RootURI)
	}
	if len(out) == 0 {
		if path, ok := params.RootPath.Get(); ok && path != "" {
			out = append(out, uri.File(path))
		}
	}
	return dedupeFolders(out)
}

func dedupeFolders(folders []uri.URI) []uri.URI {
	seen := make(map[uri.URI]struct{}, len(folders))
	out := make([]uri.URI, 0, len(folders))
	for _, folder := range folders {
		if _, ok := seen[folder]; ok {
			continue
		}
		seen[folder] = struct{}{}
		out = append(out, folder)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].String() < out[j].String()
	})
	return out
}

func (s *Server) setWorkspaceFolders(folders []uri.URI) {
	s.workspaceMu.Lock()
	s.workspaceFolders = dedupeFolders(folders)
	s.workspaceMu.Unlock()
}

// WorkspaceFolders returns a copy of the configured workspace folders.
func (s *Server) WorkspaceFolders() []uri.URI {
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()
	return append([]uri.URI(nil), s.workspaceFolders...)
}

// DidChangeWorkspaceFolders updates the lazily scanned workspace roots.
func (s *Server) DidChangeWorkspaceFolders(ctx context.Context, params *protocol.DidChangeWorkspaceFoldersParams) error {
	if params == nil {
		return nil
	}
	current := s.WorkspaceFolders()
	removed := make(map[uri.URI]struct{}, len(params.Event.Removed))
	for _, folder := range params.Event.Removed {
		removed[folder.URI] = struct{}{}
	}
	next := make([]uri.URI, 0, len(current)+len(params.Event.Added))
	for _, folder := range current {
		if _, ok := removed[folder]; !ok {
			next = append(next, folder)
		}
	}
	for _, folder := range params.Event.Added {
		if folder.URI.IsFile() {
			next = append(next, folder.URI)
		}
	}
	s.setWorkspaceFolders(next)
	s.log.Debug("workspace folders changed", "count", len(next))
	s.carts.AnalyzeOpen(ctx)
	return nil
}
