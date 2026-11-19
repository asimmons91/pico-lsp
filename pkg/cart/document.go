// Package cart reads and writes PICO-8 text .p8 carts for external Go
// programs.
//
// Parse is pure: it never reads the filesystem, resolves #include directives,
// or flattens includes. ReplaceCode changes only the __lua__ body and preserves
// every other byte. Resolver owns filesystem-aware include ownership and is
// separate from parsing.
package cart

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	internalcart "github.com/asimmons91/pico-lsp/internal/cart"
)

// Tab describes one code tab in a .p8 cart.
type Tab struct {
	// Index is zero-based, matching pico8/cartInfo and pico-editor.
	Index int
	// Name is the text after -->8, or "Tab N" when empty.
	Name string
	// StartLine is the first content line, zero-based and full-cart.
	StartLine int
	// EndLine is exclusive, zero-based and full-cart.
	EndLine int
	// MarkerLine is the hidden -->8 line, or -1 for the first tab.
	MarkerLine int
}

// Section describes one .p8 section.
type Section struct {
	Name string
	// Known reports whether Name is a standard PICO-8 section.
	Known bool
	// StartLine is the section header line, zero-based.
	StartLine int
	// EndLine is exclusive, zero-based and full-cart.
	EndLine int
}

// Include describes one lexical #include directive in the cart code body.
type Include struct {
	// Path is the unquoted path without a .p8/.p8.png tab selector.
	Path string
	// ResolvedPath is the cleaned lexical path relative to the cart.
	ResolvedPath string
	// Tab is the zero-based .p8 tab selector, or -1 when there is none.
	Tab int
	// Line is the zero-based full-cart directive line.
	Line int
}

// Document is an immutable parsed .p8 cart.
type Document struct {
	path               string
	data               []byte
	code               string
	hasCode            bool
	codeStart          int
	codeEnd            int
	codeStartLine      int
	codeUsesCRLF       bool
	headerUnterminated bool
	tabs               []Tab
	sections           []Section
	includes           []Include
}

// Parse parses a text .p8 cart. The path supplies the extension, lexical
// include base, and error context; Parse never reads or writes the filesystem.
//
// A cart without a __lua__ section parses for inspection. Missing include files
// do not fail parsing because includes are not resolved here.
func Parse(path string, data []byte) (*Document, error) {
	cleaned := filepath.Clean(path)
	if !strings.EqualFold(filepath.Ext(cleaned), ".p8") {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, cleaned)
	}
	if len(data) > internalcart.MaxFileSize {
		return nil, fmt.Errorf("%w: %d bytes", ErrTooLarge, len(data))
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, fmt.Errorf("%w: %s", ErrInvalidEncoding, cleaned)
	}

	owned := append([]byte(nil), data...)
	inspection := internalcart.InspectText(string(owned))
	if line, err := parseIssue(inspection); err != nil {
		return nil, &ParseError{Line: line, Err: err}
	}

	doc := &Document{
		path:               cleaned,
		data:               owned,
		code:               inspection.Code,
		hasCode:            inspection.HasCode,
		codeStart:          inspection.CodeStart,
		codeEnd:            inspection.CodeEnd,
		codeStartLine:      inspection.CodeStartLine,
		codeUsesCRLF:       inspection.CodeUsesCRLF,
		headerUnterminated: inspection.HeaderUnterminated,
	}
	for _, section := range inspection.Sections {
		doc.sections = append(doc.sections, Section{
			Name:      section.Name,
			Known:     section.Known,
			StartLine: section.StartLine,
			EndLine:   section.EndLine + 1,
		})
	}
	for _, tab := range inspection.Tabs {
		doc.tabs = append(doc.tabs, Tab{
			Index:      tab.Index,
			Name:       tab.Name,
			StartLine:  tab.StartLine,
			EndLine:    tab.EndLine,
			MarkerLine: tab.MarkerLine,
		})
	}
	for _, include := range inspection.Includes {
		doc.includes = append(doc.includes, Include{
			Path:         include.Path,
			ResolvedPath: resolveLexical(cleaned, include.Path),
			Tab:          include.Tab,
			Line:         include.Line,
		})
	}
	return doc, nil
}

// parseIssue returns the earliest line-numbered parse problem.
func parseIssue(inspection internalcart.Inspection) (int, error) {
	line := -1
	var err error
	if len(inspection.MalformedLines) > 0 {
		line = inspection.MalformedLines[0]
		err = ErrMalformedSection
	}
	if inspection.DuplicateLuaLine >= 0 && (line < 0 || inspection.DuplicateLuaLine < line) {
		line = inspection.DuplicateLuaLine
		err = ErrDuplicateLua
	}
	return line, err
}

func resolveLexical(cartPath, includePath string) string {
	slashed := strings.ReplaceAll(includePath, "\\", "/")
	return filepath.Clean(filepath.Join(filepath.Dir(cartPath), filepath.FromSlash(slashed)))
}

// Path returns the cleaned path passed to Parse.
func (d *Document) Path() string {
	return d.path
}

// Code returns the exact bytes between the __lua__ header line and the next
// section header or EOF. CRLF and LF are preserved. A cart without __lua__
// returns an empty string.
func (d *Document) Code() string {
	return d.code
}

// HasCode reports whether the cart has a __lua__ section.
func (d *Document) HasCode() bool {
	return d.hasCode
}

// CodeStartLine returns the zero-based full-cart line where the code body
// begins, or 0 when HasCode is false.
func (d *Document) CodeStartLine() int {
	return d.codeStartLine
}

// Tabs returns a copy of the cart tabs in file order. It is nil when HasCode is
// false.
func (d *Document) Tabs() []Tab {
	if d.tabs == nil {
		return nil
	}
	out := make([]Tab, len(d.tabs))
	copy(out, d.tabs)
	return out
}

// Sections returns a copy of all sections in file order, including unknown
// sections.
func (d *Document) Sections() []Section {
	if d.sections == nil {
		return nil
	}
	out := make([]Section, len(d.sections))
	copy(out, d.sections)
	return out
}

// Includes returns a copy of the lexical include directives in source order.
func (d *Document) Includes() []Include {
	if d.includes == nil {
		return nil
	}
	out := make([]Include, len(d.includes))
	copy(out, d.includes)
	return out
}
