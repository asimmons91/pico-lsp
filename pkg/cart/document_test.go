package cart_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asimmons91/pico-lsp/pkg/cart"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func parseFixture(t *testing.T, name string) *cart.Document {
	t.Helper()
	doc, err := cart.Parse(filepath.FromSlash("/work/game.p8"), readFixture(t, name))
	if err != nil {
		t.Fatalf("Parse(%s): %v", name, err)
	}
	return doc
}

func TestParseAllSections(t *testing.T) {
	doc := parseFixture(t, "all_sections.p8")
	if doc.Path() != filepath.Clean(filepath.FromSlash("/work/game.p8")) {
		t.Fatalf("Path() = %q", doc.Path())
	}
	if !doc.HasCode() {
		t.Fatal("HasCode() = false, want true")
	}
	if doc.CodeStartLine() != 3 {
		t.Fatalf("CodeStartLine() = %d, want 3", doc.CodeStartLine())
	}
	wantCode := "-- main\nprint(1)\n-->8 second\n-- second\nfunction f() end\n"
	if doc.Code() != wantCode {
		t.Fatalf("Code() = %q, want %q", doc.Code(), wantCode)
	}

	wantSections := []cart.Section{
		{Name: "lua", Known: true, StartLine: 2, EndLine: 8},
		{Name: "gfx", Known: true, StartLine: 8, EndLine: 10},
		{Name: "gff", Known: true, StartLine: 10, EndLine: 12},
		{Name: "label", Known: true, StartLine: 12, EndLine: 14},
		{Name: "map", Known: true, StartLine: 14, EndLine: 16},
		{Name: "sfx", Known: true, StartLine: 16, EndLine: 18},
		{Name: "music", Known: true, StartLine: 18, EndLine: 20},
		{Name: "mystery", Known: false, StartLine: 20, EndLine: 22},
	}
	if got := doc.Sections(); !equalSections(got, wantSections) {
		t.Fatalf("Sections() = %+v, want %+v", got, wantSections)
	}

	wantTabs := []cart.Tab{
		{Index: 0, Name: "Tab 1", StartLine: 3, EndLine: 5, MarkerLine: -1},
		{Index: 1, Name: "second", StartLine: 6, EndLine: 8, MarkerLine: 5},
	}
	if got := doc.Tabs(); !equalTabs(got, wantTabs) {
		t.Fatalf("Tabs() = %+v, want %+v", got, wantTabs)
	}
	if got := doc.Includes(); len(got) != 0 {
		t.Fatalf("Includes() = %+v, want none", got)
	}
}

func TestParseUnknownSections(t *testing.T) {
	doc := parseFixture(t, "unknown_sections.p8")
	want := []cart.Section{
		{Name: "lua", Known: true, StartLine: 2, EndLine: 4},
		{Name: "mystery", Known: false, StartLine: 4, EndLine: 6},
		{Name: "another", Known: false, StartLine: 6, EndLine: 8},
	}
	if got := doc.Sections(); !equalSections(got, want) {
		t.Fatalf("Sections() = %+v, want %+v", got, want)
	}
}

func TestParseBOM(t *testing.T) {
	doc := parseFixture(t, "bom.p8")
	if doc.Code() != "x=1\n" || doc.CodeStartLine() != 3 {
		t.Fatalf("Code/CodeStartLine = %q/%d", doc.Code(), doc.CodeStartLine())
	}
	if len(doc.Sections()) != 2 || doc.Sections()[0].Name != "lua" {
		t.Fatalf("Sections() = %+v", doc.Sections())
	}
}

func TestParseCRLF(t *testing.T) {
	doc := parseFixture(t, "crlf.p8")
	if !strings.Contains(doc.Code(), "\r\n") {
		t.Fatalf("Code() = %q, want CRLF", doc.Code())
	}
	if got := doc.Tabs(); len(got) != 2 || got[0].EndLine != 5 || got[1].MarkerLine != 5 {
		t.Fatalf("Tabs() = %+v", got)
	}
}

func TestParseLoneCRIsData(t *testing.T) {
	doc := parseFixture(t, "lone_cr.p8")
	if !strings.Contains(doc.Code(), "a\rb") {
		t.Fatalf("Code() = %q, want lone CR preserved", doc.Code())
	}
	if got := doc.Tabs(); len(got) != 1 || got[0].StartLine != 3 || got[0].EndLine != 4 {
		t.Fatalf("Tabs() = %+v", got)
	}
}

func TestParseEmptyBody(t *testing.T) {
	doc := parseFixture(t, "empty_body.p8")
	if !doc.HasCode() || doc.Code() != "" {
		t.Fatalf("HasCode/Code = %t/%q", doc.HasCode(), doc.Code())
	}
	if doc.CodeStartLine() != 3 {
		t.Fatalf("CodeStartLine() = %d, want 3", doc.CodeStartLine())
	}
	want := []cart.Tab{{Index: 0, Name: "Tab 1", StartLine: 3, EndLine: 3, MarkerLine: -1}}
	if got := doc.Tabs(); !equalTabs(got, want) {
		t.Fatalf("Tabs() = %+v, want %+v", got, want)
	}
}

func TestParseNoLuaSection(t *testing.T) {
	doc := parseFixture(t, "no_lua.p8")
	if doc.HasCode() || doc.Code() != "" || doc.CodeStartLine() != 0 {
		t.Fatalf("HasCode/Code/CodeStartLine = %t/%q/%d", doc.HasCode(), doc.Code(), doc.CodeStartLine())
	}
	if doc.Tabs() != nil {
		t.Fatalf("Tabs() = %+v, want nil", doc.Tabs())
	}
}

func TestParseUnterminatedHeader(t *testing.T) {
	doc := parseFixture(t, "unterminated_header.p8")
	if !doc.HasCode() || doc.Code() != "" || doc.CodeStartLine() != 3 {
		t.Fatalf("HasCode/Code/CodeStartLine = %t/%q/%d", doc.HasCode(), doc.Code(), doc.CodeStartLine())
	}
}

func TestParseLeadingAndConsecutiveMarkers(t *testing.T) {
	leading := parseFixture(t, "leading_marker.p8")
	wantLeading := []cart.Tab{
		{Index: 0, Name: "Tab 1", StartLine: 3, EndLine: 3, MarkerLine: -1},
		{Index: 1, Name: "first", StartLine: 4, EndLine: 5, MarkerLine: 3},
		{Index: 2, Name: "Tab 3", StartLine: 6, EndLine: 7, MarkerLine: 5},
	}
	if got := leading.Tabs(); !equalTabs(got, wantLeading) {
		t.Fatalf("leading Tabs() = %+v, want %+v", got, wantLeading)
	}

	consecutive := parseFixture(t, "consecutive_markers.p8")
	wantConsecutive := []cart.Tab{
		{Index: 0, Name: "Tab 1", StartLine: 3, EndLine: 4, MarkerLine: -1},
		{Index: 1, Name: "one", StartLine: 5, EndLine: 5, MarkerLine: 4},
		{Index: 2, Name: "two", StartLine: 6, EndLine: 7, MarkerLine: 5},
	}
	if got := consecutive.Tabs(); !equalTabs(got, wantConsecutive) {
		t.Fatalf("consecutive Tabs() = %+v, want %+v", got, wantConsecutive)
	}
}

func TestParseIndentedMarkerIsNotATab(t *testing.T) {
	doc := parseFixture(t, "indented_marker.p8")
	want := []cart.Tab{{Index: 0, Name: "Tab 1", StartLine: 3, EndLine: 5, MarkerLine: -1}}
	if got := doc.Tabs(); !equalTabs(got, want) {
		t.Fatalf("Tabs() = %+v, want %+v", got, want)
	}
}

func TestParseBlankLines(t *testing.T) {
	doc := parseFixture(t, "blank_lines.p8")
	if !strings.HasPrefix(doc.Code(), "\na=1\n\n-->8\n") || !strings.HasSuffix(doc.Code(), "b=2\n\n\n") {
		t.Fatalf("Code() = %q, want blank lines preserved", doc.Code())
	}
	want := []cart.Tab{
		{Index: 0, Name: "Tab 1", StartLine: 4, EndLine: 7, MarkerLine: -1},
		{Index: 1, Name: "Tab 2", StartLine: 8, EndLine: 12, MarkerLine: 7},
	}
	if got := doc.Tabs(); !equalTabs(got, want) {
		t.Fatalf("Tabs() = %+v, want %+v", got, want)
	}
}

func TestParseIncludes(t *testing.T) {
	doc := parseFixture(t, "includes.p8")
	base := filepath.Dir(filepath.Clean(filepath.FromSlash("/work/game.p8")))
	want := []cart.Include{
		{Path: "lib/a.lua", ResolvedPath: filepath.Join(base, "lib", "a.lua"), Tab: -1, Line: 3},
		{Path: "lib/with space.lua", ResolvedPath: filepath.Join(base, "lib", "with space.lua"), Tab: -1, Line: 4},
		{Path: "lib/plain.lua", ResolvedPath: filepath.Join(base, "lib", "plain.lua"), Tab: -1, Line: 5},
		{Path: "other.p8", ResolvedPath: filepath.Join(base, "other.p8"), Tab: 1, Line: 6},
		{Path: "lua:1.lua", ResolvedPath: filepath.Join(base, "lua:1.lua"), Tab: -1, Line: 7},
		{Path: "aftercomment.lua", ResolvedPath: filepath.Join(base, "aftercomment.lua"), Tab: -1, Line: 14},
	}
	if got := doc.Includes(); !equalIncludes(got, want) {
		t.Fatalf("Includes() = %+v, want %+v", got, want)
	}
	if !strings.Contains(doc.Code(), `#include "lib/a.lua"`) {
		t.Fatal("Code() should retain include directives")
	}
}

func TestParseMissingIncludeDoesNotFail(t *testing.T) {
	doc, err := cart.Parse(filepath.FromSlash("/work/game.p8"), []byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"missing.lua\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Includes(); len(got) != 1 || got[0].Path != "missing.lua" {
		t.Fatalf("Includes() = %+v", got)
	}
}

func TestParseBackslashIncludePath(t *testing.T) {
	data := []byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"lib\\a.lua\"\n")
	doc, err := cart.Parse(filepath.FromSlash("/work/game.p8"), data)
	if err != nil {
		t.Fatal(err)
	}
	includes := doc.Includes()
	if len(includes) != 1 {
		t.Fatalf("Includes() = %+v", includes)
	}
	want := filepath.Join(filepath.Dir(filepath.FromSlash("/work/game.p8")), "lib", "a.lua")
	if includes[0].ResolvedPath != want {
		t.Fatalf("ResolvedPath = %q, want %q", includes[0].ResolvedPath, want)
	}
}

func TestParseIncludeAtEOFWithoutNewline(t *testing.T) {
	data := []byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n#include \"last.lua\"")
	doc, err := cart.Parse(filepath.FromSlash("/work/game.p8"), data)
	if err != nil {
		t.Fatal(err)
	}
	includes := doc.Includes()
	if len(includes) != 1 || includes[0].Path != "last.lua" || includes[0].Line != 3 {
		t.Fatalf("Includes() = %+v", includes)
	}
}

func TestParseEmptyInput(t *testing.T) {
	doc, err := cart.Parse("game.p8", nil)
	if err != nil {
		t.Fatal(err)
	}
	if doc.HasCode() || doc.Tabs() != nil || doc.Code() != "" {
		t.Fatalf("empty cart = %+v", doc)
	}
}

func TestParseDuplicateLua(t *testing.T) {
	_, err := cart.Parse(filepath.FromSlash("/work/game.p8"), readFixture(t, "duplicate_lua.p8"))
	if !errors.Is(err, cart.ErrDuplicateLua) {
		t.Fatalf("err = %v, want ErrDuplicateLua", err)
	}
	var parseErr *cart.ParseError
	if !errors.As(err, &parseErr) || parseErr.Line != 4 {
		t.Fatalf("ParseError = %+v, want line 4", parseErr)
	}
}

func TestParseMalformedSection(t *testing.T) {
	data := []byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n__index = 1\n____\n__lu a__\n")
	_, err := cart.Parse(filepath.FromSlash("/work/game.p8"), data)
	if !errors.Is(err, cart.ErrMalformedSection) {
		t.Fatalf("err = %v, want ErrMalformedSection", err)
	}
	var parseErr *cart.ParseError
	if !errors.As(err, &parseErr) || parseErr.Line != 4 {
		t.Fatalf("ParseError = %+v, want line 4", parseErr)
	}

	ok := []byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n__index = 1\n__foo_bar__ = 2\n")
	if _, err := cart.Parse(filepath.FromSlash("/work/game.p8"), ok); err != nil {
		t.Fatalf("Lua identifiers should not be malformed sections: %v", err)
	}
}

func TestParseMalformedShortMarkers(t *testing.T) {
	for _, marker := range []string{"__", "___", "____"} {
		data := []byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n" + marker + "\n")
		if _, err := cart.Parse(filepath.FromSlash("/work/game.p8"), data); !errors.Is(err, cart.ErrMalformedSection) {
			t.Errorf("marker %q err = %v, want ErrMalformedSection", marker, err)
		}
	}
}

func TestParseUnsupportedFormat(t *testing.T) {
	for _, path := range []string{"game.p8.png", "game.lua", "", "game"} {
		if _, err := cart.Parse(path, nil); !errors.Is(err, cart.ErrUnsupportedFormat) {
			t.Errorf("Parse(%q) err = %v, want ErrUnsupportedFormat", path, err)
		}
	}
	if _, err := cart.Parse("GAME.P8", []byte("pico-8 cartridge // pico-8.com\n")); err != nil {
		t.Fatalf("uppercase extension: %v", err)
	}
}

func TestParseInvalidEncoding(t *testing.T) {
	for _, data := range [][]byte{{0xff, 0xfe}, []byte("a\x00b")} {
		if _, err := cart.Parse("game.p8", data); !errors.Is(err, cart.ErrInvalidEncoding) {
			t.Errorf("Parse(%v) err = %v, want ErrInvalidEncoding", data, err)
		}
	}
}

func TestParseTooLarge(t *testing.T) {
	data := bytes.Repeat([]byte("a"), (1<<20)+1)
	if _, err := cart.Parse("game.p8", data); !errors.Is(err, cart.ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

func TestDocumentSlicesAreCopies(t *testing.T) {
	doc := parseFixture(t, "includes.p8")
	doc.Tabs()[0].Name = "mutated"
	doc.Sections()[0].Name = "mutated"
	doc.Includes()[0].Path = "mutated"
	if doc.Tabs()[0].Name == "mutated" || doc.Sections()[0].Name == "mutated" || doc.Includes()[0].Path == "mutated" {
		t.Fatal("accessors expose internal slices")
	}
}

func equalTabs(a, b []cart.Tab) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalSections(a, b []cart.Section) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalIncludes(a, b []cart.Include) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
