package syntax

import (
	"bytes"
	"os"
	"testing"
)

func corpusSeeds(f *testing.F) {
	for _, name := range []string{
		"0xcafed00d__tac08/cart.p8",
		"apa64__try-pico8/oop1.p8",
		"brubsby__tweetcarts/truchet.p8",
		"jesstelford__pecs/example/particles.p8",
	} {
		data, err := os.ReadFile("../../testdata/carts/" + name)
		if err != nil {
			continue
		}
		if i := bytes.Index(data, []byte("__lua__\n")); i >= 0 {
			f.Add(data[i+len("__lua__\n"):])
		}
	}
}

func FuzzLexer(f *testing.F) {
	corpusSeeds(f)
	f.Add([]byte("local x = {1, 2, 3}\n?x\n"))
	f.Add([]byte("function f(a, ...) return a end\n"))
	f.Add([]byte("--[[ comment ]]\nif (x) y += 1\n"))
	f.Add([]byte{0xff, 0x00, '\n'})

	f.Fuzz(func(t *testing.T, data []byte) {
		res := Lex(data, nil)
		if len(res.Errors) > maxLexErrors {
			t.Fatalf("lex errors = %d, want <= %d", len(res.Errors), maxLexErrors)
		}
		if len(res.Tokens) == 0 || res.Tokens[len(res.Tokens)-1].Kind != TokenEOF {
			t.Fatal("token stream does not end with EOF")
		}
	})
}

func FuzzParser(f *testing.F) {
	corpusSeeds(f)
	f.Add([]byte("function f(a, b) local t={x=a,[b]=2} if (a) ?t.x return t end\n"))
	f.Add([]byte("for k,v in pairs(t) do if v then break end end\n"))
	f.Add([]byte("a=@b a+=1 t.x=2 f{} f\"s\"\n"))
	f.Add([]byte("if (x) y=1 else z=2\nwhile (x) ?y\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		lexed := Lex(data, nil)
		chunk, errs := Parse(lexed.Tokens)
		if chunk == nil {
			t.Fatal("nil chunk")
		}
		if len(errs) > maxParseErrors {
			t.Fatalf("parse errors = %d, want <= %d", len(errs), maxParseErrors)
		}
	})
}
