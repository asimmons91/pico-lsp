// Package cart reads PICO-8 cartridges, resolves includes, and builds the
// flattened program used by the lexer and parser.
package cart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

const (
	// MaxFileSize bounds one cart or included file.
	MaxFileSize = 1 << 20
	// MaxFlatSize bounds the assembled program. It is far above PICO-8's
	// 65535-character limit but stops repeated includes from exhausting memory.
	MaxFlatSize = 1 << 20
	// MaxIncludes bounds the number of directives in one cart.
	MaxIncludes = 512
)

// Severity is the default severity for a cart-level diagnostic.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
)

// Diagnostic is a cart/include diagnostic before LSP conversion.
type Diagnostic struct {
	Code     string
	Message  string
	Span     source.Span
	Severity Severity
}

// Diagnostic codes owned by the cart layer.
const (
	CodeIncludeMissing    = "include-missing"
	CodeIncludeNested     = "include-nested"
	CodeIncludeUnreadable = "include-unreadable"
)

// File is one original source file in a cart. FileID is the index in
// Cart.Files.
type File struct {
	ID    source.FileID
	URI   uri.URI
	Text  string
	Index *source.LineIndex
}

// Segment maps a contiguous flattened byte range to an original file.
type Segment struct {
	FlatStart  int
	FlatEnd    int
	FileID     source.FileID
	FileOffset int
}

// Include records one #include directive.
type Include struct {
	Path          string
	ResolvedURI   uri.URI
	DirectiveSpan source.Span
	FileSpan      source.Span
	Missing       bool
	Nested        bool
	Unreadable    bool
}

// Tab is a code tab in a .p8 cart. The marker line remains a comment in the
// flattened program.
type Tab struct {
	Index      int
	Name       string
	MarkerSpan source.Span
	StartLine  int
	EndLine    int
	SpanVal    source.Span
}

// Section is a .p8 section. SpanVal covers the header through its content.
type Section struct {
	Name         string
	Known        bool
	StartLine    int
	EndLine      int
	HeaderSpan   source.Span
	ContentStart int
	ContentEnd   int
	SpanVal      source.Span
}

// Cart is the flattened cart and its source map.
type Cart struct {
	RootURI     uri.URI
	Files       []*File
	Segments    []Segment
	Includes    []Include
	Tabs        []Tab
	Sections    []Section
	Flat        []byte
	Tokens      int
	Chars       int
	Version     int
	Diagnostics []Diagnostic
}

// Snapshot is a cart plus its lexed and parsed representation.
type Snapshot struct {
	Cart     *Cart
	Tokens   []syntax.Token
	Comments []syntax.Comment
	Index    *syntax.CommentIndex
	Chunk    *syntax.Chunk
	Errors   []syntax.Error
}

// Provider supplies file contents and directory listings, checking open
// document overlays before disk where applicable.
type Provider interface {
	Read(uri.URI) ([]byte, error)
	List(uri.URI) ([]uri.URI, error)
}

// SizeProvider optionally reports a file's size before it is read, so callers
// can reject oversized input without allocating it.
type SizeProvider interface {
	Size(uri.URI) (int64, error)
}

// ProviderFuncs adapts functions into a Provider.
type ProviderFuncs struct {
	ReadFunc func(uri.URI) ([]byte, error)
	ListFunc func(uri.URI) ([]uri.URI, error)
	SizeFunc func(uri.URI) (int64, error)
}

func (p ProviderFuncs) Read(u uri.URI) ([]byte, error) {
	if p.ReadFunc == nil {
		return nil, fmt.Errorf("cart: no reader")
	}
	return p.ReadFunc(u)
}

func (p ProviderFuncs) List(u uri.URI) ([]uri.URI, error) {
	if p.ListFunc == nil {
		return nil, fmt.Errorf("cart: no directory lister")
	}
	return p.ListFunc(u)
}

func (p ProviderFuncs) Size(u uri.URI) (int64, error) {
	if p.SizeFunc == nil {
		return 0, fmt.Errorf("cart: no size provider")
	}
	return p.SizeFunc(u)
}

// DiskProvider reads files and directories from the local filesystem.
type DiskProvider struct{}

func (DiskProvider) Read(u uri.URI) ([]byte, error) {
	info, err := os.Stat(u.FsPath())
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	return os.ReadFile(u.FsPath())
}

func (DiskProvider) Size(u uri.URI) (int64, error) {
	info, err := os.Stat(u.FsPath())
	if err != nil {
		return 0, err
	}
	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("not a regular file")
	}
	return info.Size(), nil
}

func (DiskProvider) List(u uri.URI) ([]uri.URI, error) {
	entries, err := os.ReadDir(u.FsPath())
	if err != nil {
		return nil, err
	}
	out := make([]uri.URI, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		out = append(out, uri.File(filepath.Join(u.FsPath(), entry.Name())))
	}
	return out, nil
}

// Loader builds cart snapshots.
type Loader struct {
	Provider    Provider
	Encoding    protocol.PositionEncodingKind
	Log         interface{ Debug(string, ...any) }
	MaxFileSize int
	MaxFlatSize int
	MaxIncludes int
}

// NewLoader creates a loader. A nil provider uses the local filesystem.
func NewLoader(provider Provider, encoding protocol.PositionEncodingKind, log interface{ Debug(string, ...any) }) *Loader {
	if provider == nil {
		provider = DiskProvider{}
	}
	if log == nil {
		log = discardLogger{}
	}
	if encoding == "" {
		encoding = protocol.PositionEncodingKindUTF16
	}
	return &Loader{
		Provider:    provider,
		Encoding:    encoding,
		Log:         log,
		MaxFileSize: MaxFileSize,
	}
}

type discardLogger struct{}

func (discardLogger) Debug(string, ...any) {}

// File returns the file for id.
func (c *Cart) File(id source.FileID) *File {
	if c == nil || int(id) < 0 || int(id) >= len(c.Files) {
		return nil
	}
	return c.Files[id]
}

// FileID returns the id for u, or -1.
func (c *Cart) FileID(u uri.URI) source.FileID {
	for _, file := range c.Files {
		if file.URI == u {
			return file.ID
		}
	}
	return -1
}

// Span maps a flattened byte range to an original file. Ranges that cross a
// segment boundary are clamped to the segment containing the start.
// sizeOf returns a known file size when the provider supports it.
func (l *Loader) sizeOf(u uri.URI) (int64, bool) {
	sp, ok := l.Provider.(SizeProvider)
	if !ok {
		return 0, false
	}
	size, err := sp.Size(u)
	if err != nil {
		return 0, false
	}
	return size, true
}

func (c *Cart) Span(start, end int) source.Span {
	if c == nil || len(c.Segments) == 0 {
		return source.Span{Start: start, End: end}
	}
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}

	i := 0
	for i < len(c.Segments) && c.Segments[i].FlatEnd <= start {
		i++
	}
	if i >= len(c.Segments) {
		i = len(c.Segments) - 1
	}
	seg := c.Segments[i]
	if start > seg.FlatEnd {
		start = seg.FlatEnd
	}
	if end > seg.FlatEnd {
		end = seg.FlatEnd
	}
	fileStart := seg.FileOffset + (start - seg.FlatStart)
	fileEnd := fileStart + (end - start)
	if fileStart < 0 {
		fileStart = 0
	}
	if fileEnd < fileStart {
		fileEnd = fileStart
	}
	if file := c.File(seg.FileID); file != nil {
		if fileStart > len(file.Text) {
			fileStart = len(file.Text)
		}
		if fileEnd > len(file.Text) {
			fileEnd = len(file.Text)
		}
	}
	return source.Span{FileID: seg.FileID, Start: fileStart, End: fileEnd}
}

// IsP8PNG reports whether a URI names a PNG cartridge.
func IsP8PNG(u uri.URI) bool {
	return strings.HasSuffix(strings.ToLower(u.Path()), ".p8.png")
}
