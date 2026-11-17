package format

import "testing"

func FuzzFormatLua(f *testing.F) {
	seeds := []string{
		"x=1\n",
		"function f(a,b)\nif (a) then\nx=1\nend\nend\n",
		"if (btn(0)) x+=1\n?print(x)\n",
		"local t={\na=1,\n-- comment\nb=2,\n}\n",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		got, err := Lua(src, DefaultOptions())
		if err != nil {
			if err != ErrSyntax {
				t.Fatalf("unexpected error: %v", err)
			}
			return
		}
		again, err := Lua(got, DefaultOptions())
		if err != nil {
			t.Fatalf("formatted output no longer parses: %v\ninput:\n%s\noutput:\n%s", err, src, got)
		}
		if again != got {
			t.Fatalf("formatting is not idempotent\ninput:\n%s\nfirst:\n%s\nsecond:\n%s", src, got, again)
		}
	})
}
