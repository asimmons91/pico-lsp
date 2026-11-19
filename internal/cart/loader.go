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
		return l.emptySnapshot(rootURI, "", "PICO-8 PNG carts are not supported yet; save as .p8."), nil
	}

	if size, ok := l.sizeOf(rootURI); ok && size > int64(l.maxSize()) {
		return l.emptySnapshot(rootURI, "", fmt.Sprintf("cart is larger than the supported size (%d bytes)", l.maxSize())), nil
	}

	data, err := l.readFile(rootURI, l.maxSize())
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
		cart.Tabs = buildTabs(text, sections, codeLines)
		alignLinesToLSP(cart, root.Index, text)
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
	cart.FileChars = CountCharsBySegment(cart.Segments, cart.Flat)

	lexed := syntax.Lex(cart.Flat, cart.Span)
	cart.Tokens = CountTokens(lexed.Tokens)
	cart.FileTokens = CountTokensByFile(lexed.Tokens)
	cart.CountsAvailable = true
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

func buildTabs(text string, sections []Section, code []lineRef) []Tab {
	var lua *Section
	for i := range sections {
		if sections[i].Name == "lua" {
			lua = &sections[i]
			break
		}
	}
	if lua == nil {
		return nil
	}

	codeText := ""
	if lua.ContentStart < lua.SpanVal.End {
		codeText = text[lua.ContentStart:lua.SpanVal.End]
	}
	codeStartLine := lua.ContentStartLine
	codeEndLine := codeStartLine + countCodeLines(codeText)
	lastEnd := lua.ContentStart
	if len(code) > 0 {
		lastEnd = contentEnd(text, code[len(code)-1])
	}

	type marker struct {
		line int
		ref  lineRef
		name string
	}
	var markers []marker
	for i, line := range code {
		lineNo := codeStartLine + i
		if lineNo >= codeEndLine {
			break
		}
		content := lineContent(text, line)
		if !strings.HasPrefix(content, "-->8") {
			continue
		}
		markers = append(markers, marker{
			line: lineNo,
			ref:  line,
			name: strings.TrimSpace(content[len("-->8"):]),
		})
	}

	tab0End := codeEndLine
	tab0ContentEnd := lua.SpanVal.End
	tab0SpanEnd := lastEnd
	if len(markers) > 0 {
		tab0End = markers[0].line
		tab0ContentEnd = markers[0].ref.start
		tab0SpanEnd = markers[0].ref.start
	}
	if tab0End < codeStartLine {
		tab0End = codeStartLine
	}
	if tab0SpanEnd < lua.ContentStart {
		tab0SpanEnd = lua.ContentStart
	}
	tab0Span := source.Span{FileID: 0, Start: lua.ContentStart, End: tab0SpanEnd}
	if codeText == "" {
		tab0Span = lua.HeaderSpan
	}

	tabs := make([]Tab, 0, len(markers)+1)
	tabs = append(tabs, Tab{
		Index:        0,
		Name:         "Tab 1",
		MarkerLine:   -1,
		MarkerOffset: -1,
		ContentStart: lua.ContentStart,
		ContentEnd:   tab0ContentEnd,
		StartLine:    codeStartLine,
		EndLine:      tab0End,
		SpanVal:      tab0Span,
	})
	for i, m := range markers {
		start := m.line + 1
		end := codeEndLine
		contentEndOffset := lua.SpanVal.End
		spanEnd := lastEnd
		if i+1 < len(markers) {
			end = markers[i+1].line
			contentEndOffset = markers[i+1].ref.start
			spanEnd = markers[i+1].ref.start
		}
		if end < start {
			end = start
		}
		name := m.name
		if name == "" {
			name = fmt.Sprintf("Tab %d", i+2)
		}
		tabs = append(tabs, Tab{
			Index:        i + 1,
			Name:         name,
			MarkerLine:   m.line,
			MarkerOffset: m.ref.start,
			ContentStart: m.ref.end,
			ContentEnd:   contentEndOffset,
			MarkerSpan:   source.Span{FileID: 0, Start: m.ref.start, End: contentEnd(text, m.ref)},
			StartLine:    start,
			EndLine:      end,
			SpanVal:      source.Span{FileID: 0, Start: m.ref.start, End: spanEnd},
		})
	}
	return tabs
}
