package format

import (
	"strings"
	"testing"
)

func TestLuaFormatting(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "basic indentation and spacing",
			in:   "function f(a,b)\nif (a) then\nx=1\nelse\nx=2\nend\nend\n",
			want: "function f(a, b)\n  if (a) then\n    x = 1\n  else\n    x = 2\n  end\nend\n",
		},
		{
			name: "shorthand stays line bounded",
			in:   "if (btn(0)) x+=1\nwhile (x<10) x+=1\n?print(x)\n",
			want: "if (btn(0)) x+=1\nwhile (x<10) x+=1\n?print(x)\n",
		},
		{
			name: "comments and annotations",
			in:   "---@class Foo\nfunction f()\n-- body\nx=1\n-- before end\nend\n",
			want: "---@class Foo\nfunction f()\n  -- body\n  x = 1\n  -- before end\nend\n",
		},
		{
			name: "tables and continuations",
			in:   "local t={\na=1,\nb={c=2},\n}\n",
			want: "local t = {\n  a = 1,\n  b = {c = 2},\n}\n",
		},
		{
			name: "unary and binary operators",
			in:   "x=-1\ny=@addr\nz=#t\na=x-1\n",
			want: "x = -1\ny = @addr\nz = #t\na = x - 1\n",
		},
		{
			name: "elseif and loops",
			in:   "if a then\nx=1\nelseif b then\nx=2\nelse\nx=3\nend\nfor i=1,10 do\ny=i\nend\nrepeat\nz=1\nuntil z>3\n",
			want: "if a then\n  x = 1\nelseif b then\n  x = 2\nelse\n  x = 3\nend\nfor i = 1, 10 do\n  y = i\nend\nrepeat\n  z = 1\nuntil z > 3\n",
		},
		{
			name: "calls fields and tables",
			in:   "obj:method(1,2)\nt[1]=x\nlocal f=function(a) return a end\n",
			want: "obj:method(1, 2)\nt[1] = x\nlocal f = function(a) return a end\n",
		},
		{
			name: "string and line comment",
			in:   "x=\"-- not a comment\" -- comment\n// other\n",
			want: "x = \"-- not a comment\"  -- comment\n// other\n",
		},
		{
			name: "multiline strings and block comments preserved",
			in:   "x=[[\n  keep  this\n]]\n--[[ block\n comment ]]\ny=1\n",
			want: "x=[[\n  keep  this\n]]\n--[[ block\n comment ]]\ny = 1\n",
		},
		{
			name: "blank line cap",
			in:   "x=1\n\n\n\ny=2\n",
			want: "x = 1\n\n\ny = 2\n",
		},
		{
			name: "extra final newlines",
			in:   "x=1\n\n\n",
			want: "x = 1\n",
		},
		{
			name: "extra crlf final newlines",
			in:   "x=1\r\n\r\n",
			want: "x = 1\r\n",
		},
		{
			name: "crlf preserved",
			in:   "x=1\r\ny=2\r\n",
			want: "x = 1\r\ny = 2\r\n",
		},
		{
			name: "tab indentation",
			in:   "function f()\nx=1\nend\n",
			want: "function f()\n\tx = 1\nend\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := DefaultOptions()
			if tc.name == "tab indentation" {
				opts.InsertSpaces = false
			}
			got, err := Lua(tc.in, opts)
			if err != nil {
				t.Fatalf("Lua: %v", err)
			}
			if got != tc.want {
				t.Fatalf("formatted:\n%q\nwant:\n%q", got, tc.want)
			}
			again, err := Lua(got, opts)
			if err != nil {
				t.Fatalf("second format: %v", err)
			}
			if again != got {
				t.Fatalf("not idempotent:\nfirst:  %q\nsecond: %q", got, again)
			}
		})
	}
}

func TestLuaSyntaxError(t *testing.T) {
	for _, src := range []string{
		"function f(\n",
		"x = ",
		"#include \"nested.lua\"\n",
	} {
		if _, err := Lua(src, DefaultOptions()); err == nil {
			t.Fatalf("Lua(%q) succeeded, want syntax error", src)
		}
	}
}

func TestLuaLineEndings(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "lone cr", in: "x=1\ry=2\r", want: "x = 1\ry = 2\r"},
		{name: "no final newline", in: "x=1", want: "x = 1\n"},
		{name: "no final newline disabled", in: "x=1", want: "x = 1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := DefaultOptions()
			if tc.name == "no final newline disabled" {
				opts.InsertFinalNewline = false
			}
			got, err := Lua(tc.in, opts)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLuaFinalNewlineOptions(t *testing.T) {
	opts := DefaultOptions()
	opts.InsertFinalNewline = false
	got, err := Lua("x=1", opts)
	if err != nil {
		t.Fatal(err)
	}
	if got != "x = 1" {
		t.Fatalf("without final newline = %q", got)
	}

	opts = DefaultOptions()
	opts.TrimFinalNewlines = true
	got, err = Lua("x=1\n\n", opts)
	if err != nil {
		t.Fatal(err)
	}
	if got != "x = 1\n" {
		t.Fatalf("trimmed final newlines = %q", got)
	}

	opts = DefaultOptions()
	opts.TrimFinalNewlines = false
	got, err = Lua("x=1\n\n", opts)
	if err != nil {
		t.Fatal(err)
	}
	if got != "x = 1\n" {
		t.Fatalf("insertFinalNewline should keep one final newline = %q", got)
	}

	opts = DefaultOptions()
	opts.InsertFinalNewline = false
	opts.TrimFinalNewlines = false
	got, err = Lua("x=1\n\n", opts)
	if err != nil {
		t.Fatal(err)
	}
	if got != "x = 1\n\n" {
		t.Fatalf("preserved final newlines = %q", got)
	}

	opts = DefaultOptions()
	opts.InsertFinalNewline = false
	opts.TrimFinalNewlines = true
	got, err = Lua("x=1\n\n", opts)
	if err != nil {
		t.Fatal(err)
	}
	if got != "x = 1\n" {
		t.Fatalf("trimmed without insertion = %q", got)
	}
}

func TestP8FormattingPreservesSections(t *testing.T) {
	src := strings.Join([]string{
		"pico-8 cartridge // pico-8.com",
		"version 42",
		"__lua__",
		`#include "lib.lua"`,
		"function f()",
		"x=1",
		"if (x) x+=1",
		"end",
		"-->8",
		"function g()",
		"y=2",
		"end",
		"__gfx__",
		"0000000000000000",
	}, "\n") + "\n"
	got, err := P8(src, DefaultOptions())
	if err != nil {
		t.Fatalf("P8: %v", err)
	}
	for _, want := range []string{
		"__gfx__\n0000000000000000\n",
		`#include "lib.lua"`,
		"function f()\n  x = 1\n  if (x) x += 1\nend",
		"-->8\nfunction g()\n  y = 2\nend",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted cart missing %q:\n%s", want, got)
		}
	}
	again, err := P8(got, DefaultOptions())
	if err != nil {
		t.Fatalf("second P8: %v", err)
	}
	if again != got {
		t.Fatalf("P8 not idempotent:\nfirst:\n%s\nsecond:\n%s", got, again)
	}
}

func TestOptionsDefaults(t *testing.T) {
	opts := DefaultOptions()
	if opts.TabSize != 2 || !opts.InsertSpaces || !opts.InsertFinalNewline || !opts.TrimFinalNewlines || opts.MaxBlankLines != 2 {
		t.Fatalf("defaults = %+v", opts)
	}
}

func TestP8CRLFAndFinalNewline(t *testing.T) {
	src := "pico-8 cartridge // pico-8.com\r\nversion 42\r\n__lua__\r\nfunction f()\r\nx=1\r\nend\r\n__gfx__\r\n0000"
	got, err := P8(src, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "function f()\r\n  x = 1\r\nend\r\n__gfx__\r\n0000") {
		t.Fatalf("CRLF P8 output:\n%q", got)
	}
	withoutCRLF := strings.ReplaceAll(got, "\r\n", "")
	if strings.ContainsAny(withoutCRLF, "\r\n") {
		t.Fatalf("mixed line endings:\n%q", got)
	}
}
