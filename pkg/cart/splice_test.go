package cart_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/asimmons91/pico-lsp/pkg/cart"
)

func TestReplaceCodeRoundTripsFixtures(t *testing.T) {
	fixtures := []string{
		"all_sections.p8",
		"blank_lines.p8",
		"bom.p8",
		"consecutive_markers.p8",
		"crlf.p8",
		"empty_body.p8",
		"indented_marker.p8",
		"includes.p8",
		"leading_marker.p8",
		"lone_cr.p8",
		"no_trailing_newline.p8",
		"unknown_sections.p8",
		"unterminated_header.p8",
	}
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			original := readFixture(t, name)
			doc, err := cart.Parse("game.p8", original)
			if err != nil {
				t.Fatal(err)
			}
			if !doc.HasCode() {
				t.Fatal("fixture has no code section")
			}
			out, err := doc.ReplaceCode(doc.Code())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out, original) {
				t.Fatalf("round-trip changed bytes:\n got %q\nwant %q", out, original)
			}
		})
	}
}

func TestReplaceCodeOnlyChangesBody(t *testing.T) {
	original := readFixture(t, "all_sections.p8")
	doc, err := cart.Parse("game.p8", original)
	if err != nil {
		t.Fatal(err)
	}
	out, err := doc.ReplaceCode("new=1\n")
	if err != nil {
		t.Fatal(err)
	}
	start := bytes.Index(original, []byte("__lua__\n")) + len("__lua__\n")
	end := start + bytes.Index(original[start:], []byte("__gfx__\n"))
	want := append([]byte(nil), original[:start]...)
	want = append(want, "new=1\n"...)
	want = append(want, original[end:]...)
	if !bytes.Equal(out, want) {
		t.Fatalf("ReplaceCode changed bytes outside the body:\n got %q\nwant %q", out, want)
	}
	if !bytes.HasPrefix(out, []byte("pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n")) {
		t.Fatal("header changed")
	}
	if !bytes.Contains(out, []byte("__gfx__\n0000\n")) {
		t.Fatal("following section changed")
	}
}

func TestReplaceCodeCRLF(t *testing.T) {
	doc := parseFixture(t, "crlf.p8")
	out, err := doc.ReplaceCode("x = 1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("__lua__\r\nx = 1\r\n__gfx__")) {
		t.Fatalf("output = %q, want CRLF body with terminator", out)
	}
	start := bytes.Index(out, []byte("__lua__\r\n")) + len("__lua__\r\n")
	end := start + bytes.Index(out[start:], []byte("__gfx__"))
	if string(out[start:end]) != "x = 1\r\n" {
		t.Fatalf("body = %q, want CRLF-only line endings", out[start:end])
	}
}

func TestReplaceCodeMirrorsCRLFIntoMixedInput(t *testing.T) {
	original := []byte("pico-8 cartridge // pico-8.com\r\nversion 42\r\n__lua__\r\na=1\r\n__gfx__\r\n0000\r\n")
	doc, err := cart.Parse("game.p8", original)
	if err != nil {
		t.Fatal(err)
	}
	out, err := doc.ReplaceCode("b=2\nc=3")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("__lua__\r\nb=2\r\nc=3\r\n__gfx__")) {
		t.Fatalf("output = %q, want normalized CRLF body", out)
	}
}

func TestReplaceCodeEmptyBody(t *testing.T) {
	doc := parseFixture(t, "empty_body.p8")
	out, err := doc.ReplaceCode("a=1\n")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("__lua__\na=1\n__gfx__")) {
		t.Fatalf("output = %q, want code directly before __gfx__", out)
	}
	out, err = doc.ReplaceCode("")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, readFixture(t, "empty_body.p8")) {
		t.Fatalf("empty replacement changed bytes: %q", out)
	}
}

func TestReplaceCodeAddsNewlineBeforeFollowingSection(t *testing.T) {
	doc := parseFixture(t, "empty_body.p8")
	out, err := doc.ReplaceCode("a=1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("__lua__\na=1\n__gfx__")) {
		t.Fatalf("output = %q, want a newline before __gfx__", out)
	}
}

func TestReplaceCodeNoFollowingSection(t *testing.T) {
	doc := parseFixture(t, "no_trailing_newline.p8")
	out, err := doc.ReplaceCode("b=2")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(out, []byte("__lua__\nb=2")) {
		t.Fatalf("output = %q, want no invented trailing newline", out)
	}
}

func TestReplaceCodeUnterminatedHeader(t *testing.T) {
	original := readFixture(t, "unterminated_header.p8")
	doc, err := cart.Parse("game.p8", original)
	if err != nil {
		t.Fatal(err)
	}
	out, err := doc.ReplaceCode("x = 1")
	if !errors.Is(err, cart.ErrUnterminatedHeader) {
		t.Fatalf("err = %v, want ErrUnterminatedHeader", err)
	}
	if out != nil {
		t.Fatalf("out = %q, want nil", out)
	}
	out, err = doc.ReplaceCode("")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, original) {
		t.Fatalf("empty replacement changed bytes: %q", out)
	}
}

func TestReplaceCodeNoCodeSection(t *testing.T) {
	doc := parseFixture(t, "no_lua.p8")
	out, err := doc.ReplaceCode("x = 1")
	if !errors.Is(err, cart.ErrNoCodeSection) {
		t.Fatalf("err = %v, want ErrNoCodeSection", err)
	}
	if out != nil {
		t.Fatalf("out = %q, want nil", out)
	}
}

func TestReplaceCodePreservesLoneCR(t *testing.T) {
	original := readFixture(t, "lone_cr.p8")
	doc, err := cart.Parse("game.p8", original)
	if err != nil {
		t.Fatal(err)
	}
	code := "s = \"a\rb\"\n"
	out, err := doc.ReplaceCode(code)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("a\rb")) {
		t.Fatalf("output = %q, want lone CR preserved", out)
	}
	if !bytes.Equal(out, original) {
		t.Fatalf("output = %q, want original %q", out, original)
	}
}

func TestReplaceCodeDoesNotMutateDocument(t *testing.T) {
	doc := parseFixture(t, "all_sections.p8")
	before := doc.Code()
	_, err := doc.ReplaceCode("changed=1\n")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Code() != before {
		t.Fatal("ReplaceCode mutated the document")
	}
}

func TestReplaceCodeRepeatedCalls(t *testing.T) {
	doc := parseFixture(t, "empty_body.p8")
	first, err := doc.ReplaceCode("a=1\n")
	if err != nil {
		t.Fatal(err)
	}
	second, err := doc.ReplaceCode("b=2\n")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(first, []byte("b=2")) || bytes.Contains(second, []byte("a=1")) {
		t.Fatal("repeated replacements leaked state")
	}
	if !strings.Contains(string(first), "a=1\n") {
		t.Fatalf("first = %q", first)
	}
}
