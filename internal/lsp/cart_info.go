package lsp

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/config"
)

const (
	TokenLimit = 8192
	CharLimit  = 65535

	CommandShowCartInfo = "pico8.showCartInfo"
	RequestCartInfo     = "pico8/cartInfo"

	cartNotFoundCode jsonrpc2.Code = -32001
)

// CartInfo is the pico8/cartInfo response and the data used by the budget lens.
type CartInfo struct {
	RootURI    uri.URI       `json:"rootUri"`
	Tokens     int           `json:"tokens"`
	TokenLimit int           `json:"tokenLimit"`
	Chars      int           `json:"chars"`
	CharLimit  int           `json:"charLimit"`
	Files      []FileInfo    `json:"files,omitempty"`
	Includes   []IncludeInfo `json:"includes,omitempty"`
	Tabs       []TabInfo     `json:"tabs,omitempty"`
}

// FileInfo reports one source file's contribution to the assembled cart.
type FileInfo struct {
	URI    uri.URI `json:"uri"`
	Tokens int     `json:"tokens"`
	Chars  int     `json:"chars"`
	Bytes  int     `json:"bytes"`
}

// IncludeInfo reports one #include directive.
type IncludeInfo struct {
	Path       string  `json:"path"`
	URI        uri.URI `json:"uri,omitempty"`
	Tokens     int     `json:"tokens,omitempty"`
	Chars      int     `json:"chars,omitempty"`
	Bytes      int     `json:"bytes,omitempty"`
	Missing    bool    `json:"missing,omitempty"`
	Nested     bool    `json:"nested,omitempty"`
	Unreadable bool    `json:"unreadable,omitempty"`
}

// TabInfo reports one .p8 tab.
type TabInfo struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
}

func cartInfoFor(snap *cart.Snapshot) CartInfo {
	if snap == nil || snap.Cart == nil {
		return CartInfo{TokenLimit: TokenLimit, CharLimit: CharLimit}
	}
	c := snap.Cart
	info := CartInfo{
		RootURI:    c.RootURI,
		Tokens:     c.Tokens,
		TokenLimit: TokenLimit,
		Chars:      c.Chars,
		CharLimit:  CharLimit,
	}
	for _, file := range c.Files {
		info.Files = append(info.Files, FileInfo{
			URI:    file.URI,
			Tokens: c.FileTokens[file.ID],
			Chars:  c.FileChars[file.ID],
			Bytes:  len(file.Text),
		})
	}
	for _, include := range c.Includes {
		entry := IncludeInfo{
			Path:       include.Path,
			URI:        include.ResolvedURI,
			Missing:    include.Missing,
			Nested:     include.Nested,
			Unreadable: include.Unreadable,
		}
		if id := c.FileID(include.ResolvedURI); id >= 0 {
			if file := c.File(id); file != nil {
				entry.Bytes = len(file.Text)
				entry.Tokens = c.FileTokens[id]
				entry.Chars = c.FileChars[id]
			}
		}
		info.Includes = append(info.Includes, entry)
	}
	for _, tab := range c.Tabs {
		info.Tabs = append(info.Tabs, TabInfo{
			Index:     tab.Index,
			Name:      tab.Name,
			StartLine: tab.StartLine,
			EndLine:   tab.EndLine,
		})
	}
	return info
}

func (s *Server) cartInfoRequest(ctx context.Context, params protocol.LSPAny) (any, error) {
	var request struct {
		TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	}
	if err := protocol.Unmarshal(params, &request); err != nil {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "pico8/cartInfo: invalid params")
	}
	if request.TextDocument.URI == "" {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "pico8/cartInfo: missing textDocument.uri")
	}
	snap, _, ok := s.carts.SnapshotFor(ctx, request.TextDocument.URI)
	if !ok {
		return nil, jsonrpc2.Errorf(cartNotFoundCode, "cart-not-found: %s", request.TextDocument.URI.String())
	}
	return cartInfoFor(snap), nil
}

// ExecuteCommand handles workspace/executeCommand.
func (s *Server) ExecuteCommand(ctx context.Context, params *protocol.ExecuteCommandParams) (protocol.LSPAny, error) {
	if params == nil {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "workspace/executeCommand: missing params")
	}
	switch params.Command {
	case CommandShowCartInfo:
		result, err := s.showCartInfo(ctx, params)
		if err != nil {
			return nil, err
		}
		data, err := protocol.Marshal(result)
		if err != nil {
			return nil, err
		}
		return protocol.LSPAny(data), nil
	default:
		return nil, jsonrpc2.Errorf(jsonrpc2.MethodNotFound, "unknown command %q", params.Command)
	}
}

func (s *Server) showCartInfo(ctx context.Context, params *protocol.ExecuteCommandParams) (any, error) {
	if params == nil || len(params.Arguments) == 0 {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "%s: missing cart URI", CommandShowCartInfo)
	}
	var raw string
	if err := protocol.Unmarshal(params.Arguments[0], &raw); err != nil || raw == "" {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "%s: invalid cart URI", CommandShowCartInfo)
	}
	u := uri.URI(raw)
	snap, _, ok := s.carts.SnapshotFor(ctx, u)
	if !ok {
		return nil, jsonrpc2.Errorf(cartNotFoundCode, "cart-not-found: %s", u.String())
	}
	info := cartInfoFor(snap)
	message := cartInfoSummary(info)
	if client := s.Client(); client != nil {
		kind := protocol.MessageTypeInfo
		if info.Tokens > info.TokenLimit || info.Chars > info.CharLimit {
			kind = protocol.MessageTypeWarning
		}
		_ = client.ShowMessage(ctx, &protocol.ShowMessageParams{Type: kind, Message: message})
	}
	return info, nil
}

func cartInfoSummary(info CartInfo) string {
	summary := fmt.Sprintf("%d / %d tokens · %d / %d chars",
		info.Tokens, info.TokenLimit, info.Chars, info.CharLimit)
	if largest := largestFile(info.Files, false); largest != nil && len(info.Files) > 1 {
		summary += fmt.Sprintf("\nLargest file: %s (%d tokens, %d chars)",
			filepath.Base(largest.URI.FsPath()), largest.Tokens, largest.Chars)
	}
	return summary
}

func largestContributor(snap *cart.Snapshot, byTokens bool) *cart.File {
	if snap == nil || snap.Cart == nil || len(snap.Cart.Files) < 2 {
		return nil
	}
	var best *cart.File
	bestCount := 0
	for _, file := range snap.Cart.Files {
		if file == nil || file.ID == 0 {
			continue
		}
		count := snap.Cart.FileChars[file.ID]
		if byTokens {
			count = snap.Cart.FileTokens[file.ID]
		}
		if best == nil || count > bestCount {
			best, bestCount = file, count
		}
	}
	return best
}

func largestFile(files []FileInfo, byTokens bool) *FileInfo {
	if len(files) == 0 {
		return nil
	}
	best := &files[0]
	for i := range files {
		candidate := &files[i]
		value := candidate.Chars
		bestValue := best.Chars
		if byTokens {
			value = candidate.Tokens
			bestValue = best.Tokens
		}
		if value > bestValue {
			best = candidate
		}
	}
	return best
}

func cartLimitDiagnostics(snap *cart.Snapshot, settings config.Settings) []protocol.Diagnostic {
	if snap == nil || snap.Cart == nil || !settings.Limits.Diagnostics {
		return nil
	}
	file := snap.Cart.File(0)
	if file == nil || file.Index == nil {
		return nil
	}
	rootRange := file.Index.Range(0, 0)
	var out []protocol.Diagnostic
	add := func(count, limit int, code, unit string, byTokens bool) {
		if count < limit*9/10 {
			return
		}
		severity := protocol.DiagnosticSeverityWarning
		if count > limit {
			severity = protocol.DiagnosticSeverityError
		}
		diag := protocol.Diagnostic{
			Range:    rootRange,
			Severity: severity,
			Code:     protocol.String(code),
			Source:   protocol.NewOptional(DiagnosticSource),
			Message:  protocol.String(fmt.Sprintf("%d / %d %s", count, limit, unit)),
		}
		if contributor := largestContributor(snap, byTokens); contributor != nil {
			contributorRange := protocol.Range{}
			if contributor.Index != nil {
				contributorRange = contributor.Index.Range(0, 0)
			}
			contributorCount := snap.Cart.FileChars[contributor.ID]
			if byTokens {
				contributorCount = snap.Cart.FileTokens[contributor.ID]
			}
			diag.RelatedInformation = []protocol.DiagnosticRelatedInformation{{
				Location: protocol.Location{URI: contributor.URI, Range: contributorRange},
				Message: fmt.Sprintf("largest contributor: %s (%d %s)",
					filepath.Base(contributor.URI.FsPath()), contributorCount, unit),
			}}
		}
		out = append(out, diag)
	}
	add(snap.Cart.Tokens, TokenLimit, "token-limit", "tokens", true)
	add(snap.Cart.Chars, CharLimit, "char-limit", "chars", false)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Code.(protocol.String) < out[j].Code.(protocol.String) })
	return out
}
