package cart

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// Load reads and analyzes the cart rooted at rootURI.
func (l *Loader) Load(rootURI uri.URI) (*Snapshot, error) {
	if IsP8PNG(rootURI) {
		text, _ := l.Provider.Read(rootURI)
		return l.emptySnapshot(rootURI, string(text), "PICO-8 PNG carts are not supported yet; save as .p8."), nil
	}

	data, err := l.Provider.Read(rootURI)
	if err != nil {
		return nil, fmt.Errorf("cart: read %s: %w", rootURI.String(), err)
	}
	text := string(data)
	root := &File{
		ID:    0,
		URI:   rootURI,
		Text:  text,
		Index: source.New(data, l.Encoding),
	}
	cart := &Cart{
		RootURI: rootURI,
		Files:   []*File{root},
	}

	if len(data) > l.maxSize() || !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		cart.Diagnostics = append(cart.Diagnostics, Diagnostic{
			Code:     CodeIncludeUnreadable,
			Message:  "cart is binary or larger than the supported size",
			Span:     source.Span{FileID: 0},
			Severity: SeverityError,
		})
		return &Snapshot{Cart: cart, Chunk: &syntax.Chunk{Block: &syntax.Block{}}}, nil
	}

	var code []lineRef
	trimFinalTerminator := false
	if strings.HasSuffix(strings.ToLower(rootURI.Path()), ".p8") {
		sections, codeLines, version := parseSections(text)
		cart.Sections = sections
		cart.Version = version
		cart.Tabs = buildTabs(text, sections, codeLines, root.Index)
		code = codeLines
		trimFinalTerminator = luaFollowedBySection(sections)
	} else {
		code = splitLines(text)
		if len(code) > 0 && strings.HasPrefix(text, "\ufeff") {
			code[0].start += len("\ufeff")
		}
	}

	f := &flattener{
		loader:  l,
		rootDir: filepath.Dir(rootURI.FsPath()),
		files:   cart.Files,
		byURI:   map[uri.URI]source.FileID{rootURI: 0},
	}
	f.appendLines(text, code, 0, trimFinalTerminator)

	cart.Files = f.files
	cart.Flat = f.flat
	cart.Segments = f.segments
	cart.Includes = f.includes
	cart.Diagnostics = append(cart.Diagnostics, f.diags...)
	cart.Chars = CountChars(cart.Flat)

	lexed := syntax.Lex(cart.Flat, cart.Span)
	cart.Tokens = CountTokens(lexed.Tokens)
	chunk, parseErrors := syntax.Parse(lexed.Tokens)

	errors := make([]syntax.Error, 0, len(lexed.Errors)+len(parseErrors))
	errors = append(errors, lexed.Errors...)
	errors = append(errors, parseErrors...)

	return &Snapshot{
		Cart:     cart,
		Tokens:   lexed.Tokens,
		Comments: lexed.Comments,
		Index:    syntax.NewCommentIndex(lexed.Comments),
		Chunk:    chunk,
		Errors:   errors,
	}, nil
}

// luaFollowedBySection reports whether the __lua__ section is followed by
// another section. PICO-8 joins root code lines with newlines, so the final
// line terminator before a following section is not part of the assembled
// program's character count.
func luaFollowedBySection(sections []Section) bool {
	for i, section := range sections {
		if section.Name == "lua" {
			return i+1 < len(sections)
		}
	}
	return false
}

func (l *Loader) emptySnapshot(rootURI uri.URI, text, message string) *Snapshot {
	root := &File{
		ID:    0,
		URI:   rootURI,
		Text:  text,
		Index: source.New([]byte(text), l.Encoding),
	}
	cart := &Cart{
		RootURI: rootURI,
		Files:   []*File{root},
		Diagnostics: []Diagnostic{{
			Code:     CodeIncludeUnreadable,
			Message:  message,
			Span:     source.Span{FileID: 0},
			Severity: SeverityError,
		}},
	}
	return &Snapshot{Cart: cart, Chunk: &syntax.Chunk{Block: &syntax.Block{}}}
}

func buildTabs(text string, sections []Section, code []lineRef, index *source.LineIndex) []Tab {
	if len(code) == 0 {
		for _, section := range sections {
			if section.Name == "lua" {
				return []Tab{{
					Index:     1,
					Name:      "Tab 1",
					StartLine: section.StartLine,
					EndLine:   section.StartLine,
					SpanVal:   section.HeaderSpan,
				}}
			}
		}
		return nil
	}

	lineNumber := func(offset int) int {
		pos, err := index.Position(offset)
		if err != nil {
			return 0
		}
		return int(pos.Line)
	}

	type marker struct {
		line    int
		ref     lineRef
		name    string
		content string
	}
	var markers []marker
	for _, line := range code {
		content := lineContent(text, line)
		left := strings.TrimLeft(content, " \t")
		if !strings.HasPrefix(left, "-->8") {
			continue
		}
		name := strings.TrimSpace(left[len("-->8"):])
		markers = append(markers, marker{
			line:    lineNumber(line.start),
			ref:     line,
			name:    name,
			content: content,
		})
	}

	firstLine := lineNumber(code[0].start)
	lastLine := lineNumber(code[len(code)-1].start)
	lastEnd := contentEnd(text, code[len(code)-1])

	tabs := make([]Tab, 0, len(markers)+1)
	startLine := firstLine
	startSpan := source.Span{FileID: 0, Start: code[0].start, End: code[0].start}
	endLine := lastLine
	spanEnd := lastEnd
	if len(markers) > 0 {
		endLine = markers[0].line - 1
		spanEnd = markers[0].ref.start
		if endLine < startLine {
			endLine = startLine
			spanEnd = startSpan.Start
		}
	}
	tabs = append(tabs, Tab{
		Index:     1,
		Name:      "Tab 1",
		StartLine: startLine,
		EndLine:   endLine,
		SpanVal:   source.Span{FileID: 0, Start: startSpan.Start, End: spanEnd},
	})

	for i, m := range markers {
		name := m.name
		if name == "" {
			name = fmt.Sprintf("Tab %d", i+2)
		}
		end := lastLine
		spanEnd := lastEnd
		if i+1 < len(markers) {
			end = markers[i+1].line - 1
			spanEnd = markers[i+1].ref.start
		}
		if end < m.line {
			end = m.line
			spanEnd = m.ref.start
		}
		tabs = append(tabs, Tab{
			Index:      i + 2,
			Name:       name,
			MarkerSpan: source.Span{FileID: 0, Start: m.ref.start, End: contentEnd(text, m.ref)},
			StartLine:  m.line,
			EndLine:    end,
			SpanVal:    source.Span{FileID: 0, Start: m.ref.start, End: spanEnd},
		})
	}
	return tabs
}
