package lsp

import (
	"context"
	"errors"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/asimmons91/pico-lsp/internal/format"
)

// RangeFormatting implements textDocument/rangeFormatting by formatting the
// whole document for context and returning edits only for complete lines
// covered by the request.
func (s *Server) RangeFormatting(ctx context.Context, params *protocol.DocumentRangeFormattingParams) ([]protocol.TextEdit, error) {
	if params == nil || params.TextDocument.URI == "" {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "textDocument/rangeFormatting: invalid params")
	}

	u := canonicalVirtualURI(params.TextDocument.URI)
	if isVirtualCartURI(u) {
		return nil, jsonrpc2.Errorf(requestFailedCode,
			"cannot format read-only decoded .p8.png carts")
	}
	lower := strings.ToLower(u.Path())
	if strings.HasSuffix(lower, ".p8.png") {
		return nil, jsonrpc2.Errorf(requestFailedCode,
			"cannot format .p8.png carts; export a text .p8 first")
	}
	if !u.IsFile() {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "textDocument/rangeFormatting: document is not a file")
	}
	if _, open := s.docs.Get(u); !open {
		return nil, jsonrpc2.Errorf(jsonrpc2.InvalidParams, "textDocument/rangeFormatting: document is not open")
	}

	nav, ok := s.navigationFor(ctx, u)
	if !ok || nav.file == nil || nav.file.Index == nil {
		return nil, jsonrpc2.Errorf(requestFailedCode, "textDocument/rangeFormatting: document is not available")
	}

	// Mirror document formatting: only the cart root owns the include list, so
	// an included .lua file can still be formatted when a sibling is missing.
	if nav.file.URI == nav.snap.Cart.RootURI {
		for _, include := range nav.snap.Cart.Includes {
			if include.Missing || include.Unreadable {
				return nil, jsonrpc2.Errorf(requestFailedCode,
					"cannot format cart with missing or unreadable includes")
			}
		}
	}

	opts := s.formattingOptions(params.Options)
	formatted, err := format.Document(nav.file.Text, opts, strings.HasSuffix(lower, ".p8"))
	if errors.Is(err, format.ErrSyntax) {
		return nil, jsonrpc2.Errorf(requestFailedCode,
			"cannot format document with syntax errors; fix the errors first")
	}
	if err != nil {
		return nil, jsonrpc2.Errorf(requestFailedCode, "cannot format document safely: %v", err)
	}
	if formatted == nav.file.Text {
		return []protocol.TextEdit{}, nil
	}

	originalLines := splitRangeLines(nav.file.Text)
	formattedLines := splitRangeLines(formatted)
	if len(originalLines) != len(formattedLines) {
		return nil, jsonrpc2.Errorf(requestFailedCode,
			"range formatting would change the line count; format the whole document instead")
	}

	startLine, endLine, ok := expandRangeLines(params.Range, len(originalLines))
	if !ok {
		return []protocol.TextEdit{}, nil
	}
	if !rangeLinesChanged(originalLines, formattedLines, startLine, endLine) {
		return []protocol.TextEdit{}, nil
	}

	startOffset := originalLines[startLine].start
	lastOriginal := originalLines[endLine-1]
	endOffset := lastOriginal.start + len(lastOriginal.text)
	formattedStart := formattedLines[startLine].start
	lastFormatted := formattedLines[endLine-1]
	formattedEnd := lastFormatted.start + len(lastFormatted.text)

	if endLine == len(originalLines) && (lastOriginal.eol != "") != (lastFormatted.eol != "") {
		// The selected range includes the final line and the formatter
		// deliberately adds or trims its terminator. Include that terminator in
		// the edit; otherwise a content-only edit would leave the old one in
		// place (or fail to add the new one).
		endOffset += len(lastOriginal.eol)
		if lastFormatted.eol != "" {
			formattedEnd += len(lastFormatted.eol)
		}
	}

	return []protocol.TextEdit{{
		Range:   nav.file.Index.Range(startOffset, endOffset),
		NewText: formatted[formattedStart:formattedEnd],
	}}, nil
}

// expandRangeLines turns an LSP range into complete line indexes. The end line
// is included when the request ends inside it. An empty result means there are
// no complete lines to format.
func expandRangeLines(r protocol.Range, lineCount int) (start, end int, ok bool) {
	if lineCount <= 0 {
		return 0, 0, false
	}
	start = int(r.Start.Line)
	end = int(r.End.Line)
	if r.End.Character > 0 {
		end++
	}
	if start > end || start >= lineCount {
		return 0, 0, false
	}
	if end > lineCount {
		end = lineCount
	}
	if end <= start {
		return 0, 0, false
	}
	return start, end, true
}

func rangeLinesChanged(original, formatted []rangeLine, start, end int) bool {
	for i := start; i < end; i++ {
		if original[i].text != formatted[i].text {
			return true
		}
	}
	if end == len(original) {
		// The final terminator is outside a content-only edit, so only treat a
		// presence change as a difference when the formatter changed it.
		return (original[end-1].eol == "") != (formatted[end-1].eol == "")
	}
	return false
}

type rangeLine struct {
	text  string
	eol   string
	start int
}

// splitRangeLines mirrors the formatter's line splitting so line counts agree.
func splitRangeLines(src string) []rangeLine {
	var out []rangeLine
	for i := 0; i < len(src); {
		start := i
		for i < len(src) && src[i] != '\n' && src[i] != '\r' {
			i++
		}
		contentEnd := i
		if i < len(src) {
			if src[i] == '\r' {
				i++
				if i < len(src) && src[i] == '\n' {
					i++
				}
			} else {
				i++
			}
		}
		out = append(out, rangeLine{text: src[start:contentEnd], eol: src[contentEnd:i], start: start})
	}
	return out
}
