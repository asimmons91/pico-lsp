package cart

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/source"
)

type mapProvider struct {
	files map[uri.URI]string
}

func newMapProvider() *mapProvider {
	return &mapProvider{files: map[uri.URI]string{}}
}

func (p *mapProvider) Read(u uri.URI) ([]byte, error) {
	text, ok := p.files[u]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(text), nil
}

func (p *mapProvider) List(u uri.URI) ([]uri.URI, error) {
	dir := filepath.Clean(u.FsPath())
	var out []uri.URI
	for file := range p.files {
		if filepath.Dir(file.FsPath()) == dir {
			out = append(out, file)
		}
	}
	return out, nil
}

func loadText(t *testing.T, provider *mapProvider, root uri.URI, text string) *Snapshot {
	t.Helper()
	provider.files[root] = text
	snap, err := NewLoader(provider, "", nil).Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return snap
}

func TestLoadP8SectionsAndTabs(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	p := newMapProvider()
	text := "pico-8 cartridge // pico-8.com\r\nversion 42\r\n__lua__\r\n-- one\r\nprint(1)\r\n-->8 second\r\n-- two\r\nfunction f() end\r\n__gfx__\r\n0000\r\n__mystery__\r\nxyz\r\n"
	snap := loadText(t, p, root, text)
	cart := snap.Cart

	if cart.Version != 42 {
		t.Fatalf("version = %d, want 42", cart.Version)
	}
	if cart.Files[0].Text != text {
		t.Fatal("root text was not preserved")
	}
	if len(cart.Sections) != 3 {
		t.Fatalf("sections = %+v, want 3", cart.Sections)
	}
	if cart.Sections[0].Name != "lua" || !cart.Sections[0].Known {
		t.Fatalf("section 0 = %+v", cart.Sections[0])
	}
	if cart.Sections[2].Name != "mystery" || cart.Sections[2].Known {
		t.Fatalf("section 2 = %+v", cart.Sections[2])
	}

	if len(cart.Tabs) != 2 {
		t.Fatalf("tabs = %+v, want 2", cart.Tabs)
	}
	if cart.Tabs[0].Name != "Tab 1" || cart.Tabs[0].StartLine != 3 || cart.Tabs[0].EndLine != 4 {
		t.Fatalf("tab 1 = %+v", cart.Tabs[0])
	}
	if cart.Tabs[1].Name != "second" || cart.Tabs[1].StartLine != 5 {
		t.Fatalf("tab 2 = %+v", cart.Tabs[1])
	}

	if !strings.Contains(string(cart.Flat), "-->8 second") {
		t.Fatalf("flat = %q, want tab marker preserved as code comment", cart.Flat)
	}
	if snap.Chunk == nil || len(snap.Chunk.Block.Stmts) == 0 {
		t.Fatal("no parsed statements")
	}
	if cart.Tokens == 0 || cart.Chars == 0 {
		t.Fatalf("counts = %d/%d", cart.Tokens, cart.Chars)
	}
}

func TestLoadStandaloneLua(t *testing.T) {
	root := uri.File("/tmp/p8/main.lua")
	p := newMapProvider()
	snap := loadText(t, p, root, "print(1)\n")
	if len(snap.Cart.Sections) != 0 || len(snap.Cart.Tabs) != 0 {
		t.Fatalf("sections/tabs = %+v/%+v", snap.Cart.Sections, snap.Cart.Tabs)
	}
	if string(snap.Cart.Flat) != "print(1)\n" {
		t.Fatalf("flat = %q", snap.Cart.Flat)
	}
}

func TestLoadIncludesAndSourceMap(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	inc := uri.File("/tmp/p8/lib/a.lua")
	p := newMapProvider()
	p.files[inc] = "local a=1\nprint(a)"
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nbefore=1\n#include \"lib/a.lua\"\nafter=2\n"
	snap := loadText(t, p, root, text)
	cart := snap.Cart

	if len(cart.Includes) != 1 {
		t.Fatalf("includes = %+v", cart.Includes)
	}
	include := cart.Includes[0]
	if include.Missing || include.Nested || include.Unreadable {
		t.Fatalf("include = %+v", include)
	}
	if include.FileSpan.FileID != 1 || include.FileSpan.End != len(p.files[inc]) {
		t.Fatalf("file span = %+v", include.FileSpan)
	}
	if len(cart.Files) != 2 || cart.Files[1].URI != inc {
		t.Fatalf("files = %+v", cart.Files)
	}
	if len(cart.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", cart.Diagnostics)
	}
	if cart.Tokens != 12 || cart.Chars != 36 {
		t.Fatalf("counts = %d/%d, want 12/36", cart.Tokens, cart.Chars)
	}

	flat := string(cart.Flat)
	if !strings.Contains(flat, "before=1\nlocal a=1\nprint(a)\nafter=2\n") {
		t.Fatalf("flat = %q", flat)
	}
	// The include content must map to the included file, not the root.
	var incToken *source.Span
	for i := range snap.Tokens {
		tok := &snap.Tokens[i]
		if tok.Text == "print" || (len(tok.Text) > 5 && strings.HasPrefix(tok.Text, "print")) {
			incToken = &tok.Span
			break
		}
	}
	if incToken == nil || incToken.FileID != 1 {
		t.Fatalf("include token span = %+v", incToken)
	}
	if include.DirectiveSpan.FileID != 0 || !strings.Contains(text[include.DirectiveSpan.Start:include.DirectiveSpan.End], "#include") {
		t.Fatalf("directive span = %+v", include.DirectiveSpan)
	}
}

func TestIncludeMissing(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	p := newMapProvider()
	snap := loadText(t, p, root, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"nope.lua\"\na=1\n")
	if len(snap.Cart.Includes) != 1 || !snap.Cart.Includes[0].Missing {
		t.Fatalf("includes = %+v", snap.Cart.Includes)
	}
	if len(snap.Cart.Diagnostics) != 1 || snap.Cart.Diagnostics[0].Code != CodeIncludeMissing {
		t.Fatalf("diagnostics = %+v", snap.Cart.Diagnostics)
	}
	if !strings.Contains(string(snap.Cart.Flat), "a=1") {
		t.Fatalf("flat = %q, want code after missing include", snap.Cart.Flat)
	}
}

func TestIncludeNested(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	inc := uri.File("/tmp/p8/a.lua")
	p := newMapProvider()
	p.files[inc] = "#include \"b.lua\"\nprint(1)\n"
	snap := loadText(t, p, root, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"a.lua\"\n")
	var nested *Include
	for i := range snap.Cart.Includes {
		if snap.Cart.Includes[i].Nested {
			nested = &snap.Cart.Includes[i]
		}
	}
	if nested == nil {
		t.Fatalf("includes = %+v", snap.Cart.Includes)
	}
	if nested.DirectiveSpan.FileID != 1 {
		t.Fatalf("nested span = %+v", nested.DirectiveSpan)
	}
	if len(snap.Cart.Files) != 2 {
		t.Fatalf("files = %+v, want root + a.lua only", snap.Cart.Files)
	}
	if !strings.Contains(string(snap.Cart.Flat), "#include \"b.lua\"") {
		t.Fatalf("flat = %q, want nested directive preserved", snap.Cart.Flat)
	}
	foundNested := false
	for _, diag := range snap.Cart.Diagnostics {
		if diag.Code == CodeIncludeNested {
			foundNested = true
		}
	}
	if !foundNested {
		t.Fatalf("diagnostics = %+v", snap.Cart.Diagnostics)
	}
	if len(snap.Errors) == 0 {
		t.Fatal("expected a parser error for the pasted nested directive")
	}
}

func TestIncludeDuplicate(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	inc := uri.File("/tmp/p8/a.lua")
	p := newMapProvider()
	p.files[inc] = "print(1)\n"
	snap := loadText(t, p, root, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"a.lua\"\n#include \"a.lua\"\n")
	if len(snap.Cart.Includes) != 2 || len(snap.Cart.Files) != 2 {
		t.Fatalf("includes/files = %+v/%+v", snap.Cart.Includes, snap.Cart.Files)
	}
	if strings.Count(string(snap.Cart.Flat), "print(1)") != 2 {
		t.Fatalf("flat = %q, want duplicate content", snap.Cart.Flat)
	}
	if snap.Cart.Includes[0].FileSpan.FileID != snap.Cart.Includes[1].FileSpan.FileID {
		t.Fatal("duplicate includes should share a FileID")
	}
}

func TestIncludeInsideLongStringOrComment(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	p := newMapProvider()
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n--[[\n#include \"a.lua\"\n]]\n// #include \"b.lua\"\n-- #include \"c.lua\"\nprint([[#include d.lua]])\n"
	snap := loadText(t, p, root, text)
	if len(snap.Cart.Includes) != 0 {
		t.Fatalf("includes = %+v, want none", snap.Cart.Includes)
	}
}

func TestIncludeWindowsPathAndQuotes(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	inc := uri.File("/tmp/p8/lib/a.lua")
	p := newMapProvider()
	p.files[inc] = "x=1"
	snap := loadText(t, p, root, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n  #include 'lib\\a.lua'\n")
	if len(snap.Cart.Includes) != 1 || snap.Cart.Includes[0].Missing {
		t.Fatalf("includes = %+v", snap.Cart.Includes)
	}
	if string(snap.Cart.Flat) != "x=1\n" {
		t.Fatalf("flat = %q, want included content plus the directive line terminator", snap.Cart.Flat)
	}
}

func TestIncludePathWithSpaces(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	inc := uri.File("/tmp/p8/lib/included space.lua")
	p := newMapProvider()
	p.files[inc] = "x=1\n"
	snap := loadText(t, p, root, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib/included space.lua\"\n")
	if len(snap.Cart.Includes) != 1 || snap.Cart.Includes[0].Missing {
		t.Fatalf("includes = %+v", snap.Cart.Includes)
	}
	if string(snap.Cart.Flat) != "x=1\n\n" {
		t.Fatalf("flat = %q, want included content plus the directive line terminator", snap.Cart.Flat)
	}
}

func TestP8NoTrailingNewline(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	p := newMapProvider()
	snap := loadText(t, p, root, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\na=1")
	if string(snap.Cart.Flat) != "a=1" {
		t.Fatalf("flat = %q", snap.Cart.Flat)
	}
	if snap.Cart.Tokens != 3 || snap.Cart.Chars != 3 {
		t.Fatalf("counts = %d/%d, want 3/3", snap.Cart.Tokens, snap.Cart.Chars)
	}
	if len(snap.Errors) != 0 {
		t.Fatalf("errors = %+v", snap.Errors)
	}
}

func TestIncludeUnreadable(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	inc := uri.File("/tmp/p8/a.lua")
	p := newMapProvider()
	p.files[inc] = string([]byte{'a', 0, 'b'})
	snap := loadText(t, p, root, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"a.lua\"\na=1\n")
	if len(snap.Cart.Includes) != 1 || !snap.Cart.Includes[0].Unreadable {
		t.Fatalf("includes = %+v", snap.Cart.Includes)
	}
	found := false
	for _, diag := range snap.Cart.Diagnostics {
		if diag.Code == CodeIncludeUnreadable {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v", snap.Cart.Diagnostics)
	}
}

func TestP8PNGDiagnostic(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8.png")
	p := newMapProvider()
	p.files[root] = "PNG"
	snap := loadText(t, p, root, "PNG")
	if len(snap.Cart.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v", snap.Cart.Diagnostics)
	}
	if !strings.Contains(snap.Cart.Diagnostics[0].Message, "PICO-8 PNG carts are not supported") {
		t.Fatalf("message = %q", snap.Cart.Diagnostics[0].Message)
	}
	if len(snap.Cart.Flat) != 0 {
		t.Fatalf("flat = %q, want empty", snap.Cart.Flat)
	}
}

func TestRootReadError(t *testing.T) {
	p := newMapProvider()
	_, err := NewLoader(p, "", nil).Load(uri.File("/tmp/p8/missing.p8"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want not-exist", err)
	}
}

func TestSourceMapExactOffsets(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	inc := uri.File("/tmp/p8/a.lua")
	p := newMapProvider()
	p.files[inc] = "inside=1\n"
	rootText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nbefore=1\n#include \"a.lua\"\nafter=2\n"
	snap := loadText(t, p, root, rootText)

	want := map[string]source.Span{
		"before": {FileID: 0, Start: strings.Index(rootText, "before"), End: strings.Index(rootText, "before") + 6},
		"inside": {FileID: 1, Start: 0, End: 6},
		"after":  {FileID: 0, Start: strings.Index(rootText, "after"), End: strings.Index(rootText, "after") + 5},
	}
	for _, tok := range snap.Tokens {
		if span, ok := want[tok.Text]; ok {
			if tok.Span != span {
				t.Errorf("token %q span = %+v, want %+v", tok.Text, tok.Span, span)
			}
		}
	}
}

func TestSourceMapRoundTripTokens(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	inc := uri.File("/tmp/p8/a.lua")
	p := newMapProvider()
	p.files[inc] = "included=1\n"
	snap := loadText(t, p, root, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\nroot=1\n#include \"a.lua\"\n")
	for _, tok := range snap.Tokens {
		if tok.Kind == 0 { // EOF
			continue
		}
		file := snap.Cart.File(tok.Span.FileID)
		if file == nil {
			t.Fatalf("token %q has no file: %+v", tok.Text, tok.Span)
		}
		if tok.Span.End > len(file.Text) || tok.Span.Start < 0 {
			t.Fatalf("token %q span out of bounds: %+v len=%d", tok.Text, tok.Span, len(file.Text))
		}
	}
}

func TestLoadEmptyAndHeaderOnlyCarts(t *testing.T) {
	root := uri.File("/tmp/p8/empty.p8")
	p := newMapProvider()
	snap := loadText(t, p, root, "pico-8 cartridge // pico-8.com\nversion 42\n")
	if len(snap.Cart.Flat) != 0 || snap.Cart.Tokens != 0 || snap.Cart.Chars != 0 {
		t.Fatalf("cart = %+v, want empty", snap.Cart)
	}
	if len(snap.Cart.Tabs) != 0 || len(snap.Cart.Sections) != 0 {
		t.Fatalf("tabs/sections = %+v/%+v, want none", snap.Cart.Tabs, snap.Cart.Sections)
	}
	if len(snap.Errors) != 0 {
		t.Fatalf("errors = %+v", snap.Errors)
	}
}

func TestLoadRootBinary(t *testing.T) {
	root := uri.File("/tmp/p8/binary.p8")
	p := newMapProvider()
	snap := loadText(t, p, root, string([]byte{'p', 'n', 'g', 0, 'x'}))
	if len(snap.Cart.Flat) != 0 {
		t.Fatalf("flat = %q, want empty", snap.Cart.Flat)
	}
	found := false
	for _, diag := range snap.Cart.Diagnostics {
		if diag.Code == CodeIncludeUnreadable {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v", snap.Cart.Diagnostics)
	}
}

func TestIncludeOversized(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	inc := uri.File("/tmp/p8/a.lua")
	p := newMapProvider()
	p.files[root] = "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"a.lua\"\n"
	p.files[inc] = strings.Repeat("x", 200)
	loader := NewLoader(p, "", nil)
	loader.MaxFileSize = 100
	snap, err := loader.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Cart.Includes) != 1 || !snap.Cart.Includes[0].Unreadable {
		t.Fatalf("includes = %+v", snap.Cart.Includes)
	}
}

func TestIncludeWithoutTrailingNewlineClampsSpans(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	inc := uri.File("/tmp/p8/a.lua")
	p := newMapProvider()
	p.files[inc] = "x=[[abc"
	snap := loadText(t, p, root, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"a.lua\"\n")
	for _, tok := range snap.Tokens {
		file := snap.Cart.File(tok.Span.FileID)
		if file == nil {
			t.Fatalf("token %q has no file", tok.Text)
		}
		if tok.Span.Start < 0 || tok.Span.End > len(file.Text) {
			t.Fatalf("token %q span = %+v, file length %d", tok.Text, tok.Span, len(file.Text))
		}
	}
	var str *source.Span
	for i := range snap.Tokens {
		if snap.Tokens[i].Kind.String() == "string" {
			str = &snap.Tokens[i].Span
		}
	}
	if str == nil || str.End != len("x=[[abc") {
		t.Fatalf("string span = %+v, want end %d", str, len("x=[[abc"))
	}
}

func TestP8IncludeAllTabs(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	other := uri.File("/tmp/p8/other.p8")
	p := newMapProvider()
	p.files[other] = "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\na=1\n-->8\nb=2\n__gfx__\n"
	snap := loadText(t, p, root, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"other.p8\"\n")
	if len(snap.Cart.Includes) != 1 || snap.Cart.Includes[0].Missing || snap.Cart.Includes[0].Unreadable {
		t.Fatalf("includes = %+v", snap.Cart.Includes)
	}
	if got := string(snap.Cart.Flat); got != "a=1\n-->8\nb=2\n" {
		t.Fatalf("flat = %q", got)
	}
	if snap.Cart.Tokens != 6 {
		t.Fatalf("tokens = %d, want 6", snap.Cart.Tokens)
	}
	if snap.Cart.Chars != 13 {
		t.Fatalf("chars = %d, want 13", snap.Cart.Chars)
	}
	if snap.Cart.Includes[0].FileSpan.FileID != 1 || snap.Cart.Includes[0].FileSpan.End == 0 {
		t.Fatalf("file span = %+v", snap.Cart.Includes[0].FileSpan)
	}
}

func TestP8IncludeSelectedTab(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	other := uri.File("/tmp/p8/other.p8")
	p := newMapProvider()
	p.files[other] = "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\na=1\n-->8\nb=2\n-->8\nc=3\n__gfx__\n"
	snap := loadText(t, p, root, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include other.p8:0\n#include other.p8:1\n#include other.p8:2\n")
	if len(snap.Cart.Includes) != 3 {
		t.Fatalf("includes = %+v", snap.Cart.Includes)
	}
	if got := string(snap.Cart.Flat); got != "a=1\nb=2\nc=3\n" {
		t.Fatalf("flat = %q", got)
	}
	if snap.Cart.Tokens != 9 {
		t.Fatalf("tokens = %d, want 9", snap.Cart.Tokens)
	}
	if snap.Cart.Chars != 12 {
		t.Fatalf("chars = %d, want 12", snap.Cart.Chars)
	}
}

func TestP8IncludeTabOutOfRange(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	other := uri.File("/tmp/p8/other.p8")
	p := newMapProvider()
	p.files[other] = "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\na=1\n__gfx__\n"
	snap := loadText(t, p, root, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include other.p8:1\n")
	found := false
	for _, diag := range snap.Cart.Diagnostics {
		if diag.Code == CodeIncludeUnreadable && strings.Contains(diag.Message, "tab 1 not found") {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v", snap.Cart.Diagnostics)
	}
	if len(snap.Cart.Flat) != 0 {
		t.Fatalf("flat = %q, want empty", snap.Cart.Flat)
	}
}

func TestIncludeShortStringContinuation(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	inc := uri.File("/tmp/p8/a.lua")
	p := newMapProvider()
	p.files[inc] = "a=1\n"
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\ns='line1\\\n#include \"a.lua\"\\\nline3'\n"
	snap := loadText(t, p, root, text)
	if len(snap.Cart.Includes) != 0 {
		t.Fatalf("includes = %+v, want none", snap.Cart.Includes)
	}
	if len(snap.Errors) != 0 {
		t.Fatalf("errors = %+v", snap.Errors)
	}
	if !strings.Contains(string(snap.Cart.Flat), "#include \"a.lua\"") {
		t.Fatalf("flat = %q", snap.Cart.Flat)
	}
}

func TestIncludeBOM(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	inc := uri.File("/tmp/p8/a.lua")
	p := newMapProvider()
	p.files[inc] = "\ufefffunction f() return 1 end\n"
	snap := loadText(t, p, root, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"a.lua\"\n")
	if len(snap.Errors) != 0 {
		t.Fatalf("errors = %+v", snap.Errors)
	}
	if !strings.HasPrefix(string(snap.Cart.Flat), "function f()") {
		t.Fatalf("flat = %q, want BOM stripped", snap.Cart.Flat)
	}
	for _, tok := range snap.Tokens {
		if tok.Text == "function" && (tok.Span.FileID != 1 || tok.Span.Start != len("\ufeff")) {
			t.Fatalf("function span = %+v, want include offset %d", tok.Span, len("\ufeff"))
		}
	}
}

func TestStandaloneLuaBOM(t *testing.T) {
	root := uri.File("/tmp/p8/main.lua")
	p := newMapProvider()
	snap := loadText(t, p, root, "\ufefffunction f() return 1 end\n")
	if len(snap.Errors) != 0 {
		t.Fatalf("errors = %+v", snap.Errors)
	}
	if strings.HasPrefix(string(snap.Cart.Flat), "\ufeff") {
		t.Fatalf("flat = %q, want BOM stripped", snap.Cart.Flat)
	}
}

func TestCharCountLoneCRAndIncludeEnding(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	p := newMapProvider()
	cr := loadText(t, p, root, "pico-8 cartridge // pico-8.com\rversion 42\r__lua__\ra=1\r__gfx__\r")
	if cr.Cart.Chars != 3 {
		t.Fatalf("CR chars = %d, want 3", cr.Cart.Chars)
	}

	inc := uri.File("/tmp/p8/a.lua")
	p2 := newMapProvider()
	p2.files[inc] = "b=2\n"
	incSnap := loadText(t, p2, root, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\na=1\n#include \"a.lua\"\n__gfx__\n")
	if incSnap.Cart.Chars != 8 {
		t.Fatalf("include chars = %d, want 8", incSnap.Cart.Chars)
	}
	if incSnap.Cart.Tokens != 6 {
		t.Fatalf("include tokens = %d, want 6", incSnap.Cart.Tokens)
	}
}

func TestFlattenLimits(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	inc := uri.File("/tmp/p8/a.lua")
	p := newMapProvider()
	p.files[inc] = "b=2\n"
	p.files[root] = "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"a.lua\"\n#include \"a.lua\"\na=1\n"
	loader := NewLoader(p, "", nil)
	loader.MaxIncludes = 1
	snap, err := loader.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, diag := range snap.Cart.Diagnostics {
		if diag.Code == CodeIncludeUnreadable && strings.Contains(diag.Message, "too many #include") {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v", snap.Cart.Diagnostics)
	}

	p2 := newMapProvider()
	p2.files[inc] = strings.Repeat("x=1\n", 100)
	p2.files[root] = "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"a.lua\"\n"
	loader = NewLoader(p2, "", nil)
	loader.MaxFlatSize = 50
	snap, err = loader.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, diag := range snap.Cart.Diagnostics {
		if diag.Code == CodeIncludeUnreadable && strings.Contains(diag.Message, "flattened cart exceeds") {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v", snap.Cart.Diagnostics)
	}
	if len(snap.Cart.Flat) > 50 {
		t.Fatalf("flat = %d bytes, want <= 50", len(snap.Cart.Flat))
	}
}

func TestP8IncludeTokenSourceMap(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	other := uri.File("/tmp/p8/other.p8")
	p := newMapProvider()
	otherText := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\na=1\n-->8\nb=2\n-->8\nc=3\n__gfx__\n"
	p.files[other] = otherText
	snap := loadText(t, p, root, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include other.p8:0\n#include other.p8:1\n#include other.p8:2\n")
	want := map[string]int{
		"a": strings.Index(otherText, "a=1"),
		"b": strings.Index(otherText, "b=2"),
		"c": strings.Index(otherText, "c=3"),
	}
	for _, tok := range snap.Tokens {
		start, ok := want[tok.Text]
		if !ok {
			continue
		}
		if tok.Span.FileID != 1 || tok.Span.Start != start {
			t.Errorf("token %q span = %+v, want file 1 offset %d", tok.Text, tok.Span, start)
		}
	}
}

func TestDiskProviderRejectsNonRegularFiles(t *testing.T) {
	if _, err := (DiskProvider{}).Read(uri.File(t.TempDir())); err == nil {
		t.Fatal("reading a directory should fail")
	}
}

func TestP8IncludeWithBOMKeepsCodeOffsets(t *testing.T) {
	root := uri.File("/tmp/p8/cart.p8")
	other := uri.File("/tmp/p8/other.p8")
	p := newMapProvider()
	otherText := "\ufeffpico-8 cartridge // pico-8.com\nversion 42\n__lua__\na=1\n__gfx__\n"
	p.files[other] = otherText
	snap := loadText(t, p, root, "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include other.p8\n")
	if len(snap.Errors) != 0 {
		t.Fatalf("errors = %+v", snap.Errors)
	}
	want := strings.Index(otherText, "a=1")
	for _, tok := range snap.Tokens {
		if tok.Text == "a" && (tok.Span.FileID != 1 || tok.Span.Start != want) {
			t.Fatalf("a span = %+v, want file 1 offset %d", tok.Span, want)
		}
	}
}
