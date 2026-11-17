package lsp

import (
	"context"
	"sort"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
)

type navContext struct {
	snap      *cart.Snapshot
	semantics *analysis.Result
	fileID    source.FileID
	file      *cart.File
}

func (s *Server) navigationFor(ctx context.Context, u uri.URI) (*navContext, bool) {
	u = canonicalVirtualURI(u)
	if u == "" || (!u.IsFile() && !isVirtualCartURI(u)) || u.Path() == "" || u.Path() == "/" {
		return nil, false
	}
	snap, semantics, fileID, ok := s.carts.SemanticsFor(ctx, u)
	if !ok || semantics == nil {
		return nil, false
	}
	file := snap.Cart.File(fileID)
	if file == nil {
		return nil, false
	}
	return &navContext{
		snap:      snap,
		semantics: semantics,
		fileID:    fileID,
		file:      file,
	}, true
}

func (n *navContext) offset(pos protocol.Position) (int, bool) {
	if n == nil || n.file == nil || n.file.Index == nil {
		return 0, false
	}
	offset, err := n.file.Index.Offset(pos)
	if err != nil {
		return 0, false
	}
	return offset, true
}

func (n *navContext) occurrenceAt(offset int) *analysis.Occurrence {
	if n == nil || n.semantics == nil {
		return nil
	}
	return n.semantics.OccurrenceAt(n.fileID, offset)
}

func (n *navContext) includeAt(offset int) *cart.Include {
	if n == nil || n.snap == nil {
		return nil
	}
	for i := range n.snap.Cart.Includes {
		include := &n.snap.Cart.Includes[i]
		span := include.DirectiveSpan
		if span.FileID != n.fileID {
			continue
		}
		if span.Contains(offset) {
			return include
		}
		if offset > 0 && span.Contains(offset-1) {
			return include
		}
	}
	return nil
}

func (n *navContext) location(span source.Span) (protocol.Location, bool) {
	if n == nil || n.snap == nil {
		return protocol.Location{}, false
	}
	file := n.snap.Cart.File(span.FileID)
	if file == nil || file.Index == nil {
		return protocol.Location{}, false
	}
	return protocol.Location{
		URI:   file.URI,
		Range: file.Index.Range(span.Start, span.End),
	}, true
}

func (n *navContext) includeTarget(include *cart.Include) (protocol.Location, bool) {
	if n == nil || include == nil || include.Missing || include.Unreadable {
		return protocol.Location{}, false
	}
	fileID := n.snap.Cart.FileID(include.ResolvedURI)
	if fileID < 0 {
		return protocol.Location{}, false
	}
	file := n.snap.Cart.File(fileID)
	if file == nil || file.Index == nil {
		return protocol.Location{}, false
	}
	start := include.FileSpan.Start
	if start < 0 || start > len(file.Text) {
		start = 0
	}
	return protocol.Location{
		URI:   file.URI,
		Range: file.Index.Range(start, start),
	}, true
}

func (n *navContext) locations(spans []source.Span) []protocol.Location {
	out := make([]protocol.Location, 0, len(spans))
	for _, span := range spans {
		if loc, ok := n.location(span); ok {
			out = append(out, loc)
		}
	}
	sortLocations(out)
	return out
}

func sortLocations(locations []protocol.Location) {
	sort.SliceStable(locations, func(i, j int) bool {
		if locations[i].URI != locations[j].URI {
			return locations[i].URI.String() < locations[j].URI.String()
		}
		if locations[i].Range.Start.Line != locations[j].Range.Start.Line {
			return locations[i].Range.Start.Line < locations[j].Range.Start.Line
		}
		return locations[i].Range.Start.Character < locations[j].Range.Start.Character
	})
}
