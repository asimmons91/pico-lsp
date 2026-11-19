package cart

import "github.com/asimmons91/pico-lsp/internal/source"

// alignLinesToLSP converts P8SCII line numbers to the LSP line space used by
// cartInfo, folding, document symbols, and workspace symbols. The scanner
// treats only LF and CRLF as terminators, but LSP also treats a lone CR as one.
func alignLinesToLSP(cart *Cart, index *source.LineIndex, text string) {
	if index == nil {
		return
	}
	for i := range cart.Tabs {
		tab := &cart.Tabs[i]
		tab.StartLine = lspLine(index, tab.ContentStart)
		tab.EndLine = lspExclusiveLine(index, tab.ContentStart, tab.ContentEnd)
		if tab.MarkerOffset >= 0 {
			tab.MarkerLine = lspLine(index, tab.MarkerOffset)
		}
	}
	for i := range cart.Sections {
		section := &cart.Sections[i]
		section.StartLine = lspLine(index, section.HeaderSpan.Start)
		if section.SpanVal.End >= len(text) {
			section.EndLine = lspLine(index, len(text))
		} else {
			section.EndLine = lspLine(index, section.SpanVal.End-1)
		}
	}
}

func lspLine(index *source.LineIndex, offset int) int {
	pos, err := index.Position(offset)
	if err != nil {
		return 0
	}
	return int(pos.Line)
}

func lspExclusiveLine(index *source.LineIndex, start, end int) int {
	if end <= start {
		return lspLine(index, start)
	}
	return lspLine(index, end-1) + 1
}
