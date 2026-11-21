package refactor

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/builtin"
	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

var (
	tokenBuiltinsOnce sync.Once
	tokenBuiltins     map[string]*analysis.Symbol
)

func loadTokenBuiltins(t *testing.T) map[string]*analysis.Symbol {
	t.Helper()
	tokenBuiltinsOnce.Do(func() {
		api, err := builtin.Load()
		if err != nil {
			t.Fatalf("load builtins: %v", err)
		}
		tokenBuiltins = analysis.BuildBuiltins(api)
	})
	return tokenBuiltins
}

func tokenSavingTestContext(t *testing.T, text string) *Context {
	t.Helper()
	u := uri.File("/test/main.lua")
	file := &cart.File{ID: 0, URI: u, Text: text, Index: source.New([]byte(text), protocol.PositionEncodingKindUTF16)}
	c := &cart.Cart{RootURI: u, Files: []*cart.File{file}}
	lexed := syntax.Lex([]byte(text), func(start, end int) source.Span {
		return source.Span{FileID: 0, Start: start, End: end}
	})
	chunk, parseErrors := syntax.Parse(lexed.Tokens)
	if len(lexed.Errors) > 0 || len(parseErrors) > 0 {
		t.Fatalf("parse failed: lex=%v parse=%v", lexed.Errors, parseErrors)
	}
	snap := &cart.Snapshot{Cart: c, Chunk: chunk, Comments: lexed.Comments, Index: syntax.NewCommentIndex(lexed.Comments)}
	result := analysis.Analyze(snap, loadTokenBuiltins(t), analysis.Options{
		UndefinedGlobal: analysis.SeverityWarning,
		TypeChecking:    analysis.TypeCheckingBasic,
		Lint: analysis.LintOptions{
			UnusedLocal: analysis.SeverityHint,
		},
	})
	return &Context{Snap: snap, Result: result, FileID: 0, File: file}
}

func findTokenSavingAction(actions []*Action, kind string) *Action {
	for _, action := range actions {
		if action != nil && action.Kind == kind {
			return action
		}
	}
	return nil
}

func applyAction(t *testing.T, text string, action *Action) string {
	t.Helper()
	if action == nil {
		t.Fatal("action is nil")
	}
	return applyEdits(t, text, action.Edits)
}

func TestTokenSavingPrint(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    string
		savings int
	}{
		{name: "one argument", text: "print(score)\n", want: "?score\n", savings: 1},
		{name: "multiple arguments", text: "print(a, b)\n", want: "?a, b\n", savings: 1},
		{name: "nested expression", text: "print(a + b * 2)\n", want: "?a + b * 2\n", savings: 1},
		{name: "spacing preserved", text: "print( a , b )\n", want: "? a , b \n", savings: 1},
		{name: "shorthand if body", text: "if (x) print(a)\n", want: "if (x) ?a\n", savings: 1},
		{name: "shorthand while body", text: "while (x) print(a)\n", want: "while (x) ?a\n", savings: 1},
		{name: "parenthesized final call", text: "print((f()))\n", want: "?(f())\n", savings: 1},
		{name: "indented", text: "  print(a)\n", want: "  ?a\n", savings: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tokenSavingTestContext(t, tt.text)
			action := TokenSavingPrint(ctx, offsetOf(t, tt.text, "print"))
			if action == nil {
				t.Fatal("TokenSavingPrint returned nil")
			}
			if action.Kind != KindTokenSavingPrint {
				t.Fatalf("kind = %q", action.Kind)
			}
			if action.TokenSavings != tt.savings {
				t.Fatalf("savings = %d, want %d", action.TokenSavings, tt.savings)
			}
			wantTitle := "Use ? print shorthand (saves 1 token)"
			if tt.savings != 1 {
				wantTitle = "Use ? print shorthand (saves " + strconv.Itoa(tt.savings) + " tokens)"
			}
			if action.Title != wantTitle {
				t.Fatalf("title = %q, want %q", action.Title, wantTitle)
			}
			if got := applyAction(t, tt.text, action); got != tt.want {
				t.Fatalf("result = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTokenSavingPrintRefusals(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{name: "zero arguments", text: "print()\n"},
		{name: "zero arguments with spaces", text: "print( )\n"},
		{name: "shadowed local", text: "local print = f\nprint(a)\n"},
		{name: "reassigned global", text: "print = f\nprint(a)\n"},
		{name: "parameter", text: "local function f(print)\n  print(a)\nend\n"},
		{name: "value use", text: "local x = print(a)\n"},
		{name: "return", text: "return print(a)\n"},
		{name: "nested call", text: "f(print(a))\n"},
		{name: "method call", text: "t:print(a)\n"},
		{name: "multi-line call", text: "print(\n  a)\n"},
		{name: "trailing statement", text: "print(a) print(b)\n"},
		{name: "trailing semicolon", text: "print(a);\n"},
		{name: "trailing comment", text: "print(a) -- note\n"},
		{name: "comment inside", text: "print(a --[[note]] + b)\n"},
		{name: "same-line else", text: "if (x) print(a) else print(b)\n"},
		{name: "final multi-value call", text: "print(f())\n"},
		{name: "final vararg", text: "local function f(...)\n  print(...)\nend\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tokenSavingTestContext(t, tt.text)
			offset := offsetOf(t, tt.text, "print")
			action := TokenSavingPrint(ctx, offset)
			if tt.name == "same-line else" {
				// The final print is the only safe candidate.
				last := strings.LastIndex(tt.text, "print")
				action = TokenSavingPrint(ctx, last)
				if action == nil {
					t.Fatal("final print action returned nil")
				}
				first := TokenSavingPrint(ctx, offset)
				if first != nil {
					t.Fatalf("first print offered action: %+v", first)
				}
				return
			}
			if action != nil {
				t.Fatalf("action offered: %+v", action)
			}
		})
	}
}

func TestTokenSavingCompoundEveryOperator(t *testing.T) {
	tests := []struct {
		op       string
		compound string
	}{
		{"+", "+="}, {"-", "-="}, {"*", "*="}, {"/", "/="}, {"\\", "\\="},
		{"%", "%="}, {"^", "^="}, {"..", "..="}, {"&", "&="}, {"|", "|="},
		{"^^", "^^="}, {"<<", "<<="}, {">>", ">>="}, {">>>", ">>>="},
		{"<<>", "<<>="}, {">><", ">><="},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			text := "x = x " + tt.op + " 1\n"
			want := "x " + tt.compound + " 1\n"
			ctx := tokenSavingTestContext(t, text)
			action := TokenSavingCompound(ctx, offsetOf(t, text, "x ="))
			if action == nil {
				t.Fatal("TokenSavingCompound returned nil")
			}
			if action.Kind != KindTokenSavingCompound {
				t.Fatalf("kind = %q", action.Kind)
			}
			if action.TokenSavings != 2 {
				t.Fatalf("savings = %d, want 2", action.TokenSavings)
			}
			if wantTitle := "Use compound assignment " + tt.compound + " (saves 2 tokens)"; action.Title != wantTitle {
				t.Fatalf("title = %q, want %q", action.Title, wantTitle)
			}
			if got := applyAction(t, text, action); got != want {
				t.Fatalf("result = %q, want %q", got, want)
			}
		})
	}
}

func TestTokenSavingCompoundTargets(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    string
		savings int
	}{
		{
			name:    "simple name",
			text:    "score = score + 1\n",
			want:    "score += 1\n",
			savings: 2,
		},
		{
			name:    "static field",
			text:    "local t = {x = 1}\nt.x = t.x + 1\n",
			want:    "local t = {x = 1}\nt.x += 1\n",
			savings: 3,
		},
		{
			name:    "nested static field",
			text:    "local self = {stats = {hp = 1}}\nself.stats.hp = self.stats.hp * 2\n",
			want:    "local self = {stats = {hp = 1}}\nself.stats.hp *= 2\n",
			savings: 4,
		},
		{
			name:    "method receiver field",
			text:    "local t = {}\nfunction t:m()\n  self.hp = self.hp + 1\nend\n",
			want:    "local t = {}\nfunction t:m()\n  self.hp += 1\nend\n",
			savings: 3,
		},
		{
			name:    "parenthesized value",
			text:    "x = x + (a * b)\n",
			want:    "x += (a * b)\n",
			savings: 2,
		},
		{
			name:    "trailing comment preserved",
			text:    "x = x + 1 -- note\n",
			want:    "x += 1 -- note\n",
			savings: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tokenSavingTestContext(t, tt.text)
			action := findTokenSavingAction(TokenSavingActions(ctx, 0, len(tt.text)), KindTokenSavingCompound)
			if action == nil {
				t.Fatal("TokenSavingCompound returned nil")
			}
			if action.TokenSavings != tt.savings {
				t.Fatalf("savings = %d, want %d", action.TokenSavings, tt.savings)
			}
			if got := applyAction(t, tt.text, action); got != tt.want {
				t.Fatalf("result = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTokenSavingCompoundRefusals(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{name: "mismatched left operand", text: "x = y + 1\n"},
		{name: "reversed operands", text: "x = 1 + x\n"},
		{name: "mismatched field", text: "local t = {x = 1, y = 2}\nt.x = t.y + 1\n"},
		{name: "bracket index", text: "local t = {1}\nt[1] = t[1] + 1\n"},
		{name: "string index", text: "local t = {x = 1}\nt[\"x\"] = t[\"x\"] + 1\n"},
		{name: "dynamic receiver", text: "f().x = f().x + 1\n"},
		{name: "multiple assignment", text: "x, y = x + 1, y\n"},
		{name: "multiple values", text: "x = x + 1, 2\n"},
		{name: "existing compound", text: "x += 1\n"},
		{name: "no binary operator", text: "x = x\n"},
		{name: "unsupported operator", text: "x = x == 1\n"},
		{name: "comment inside", text: "x = x --[[note]] + 1\n"},
		{name: "multi-line", text: "x =\n  x + 1\n"},
		{name: "method call target", text: "t:m()\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tokenSavingTestContext(t, tt.text)
			if action := findTokenSavingAction(TokenSavingActions(ctx, 0, len(tt.text)), KindTokenSavingCompound); action != nil {
				t.Fatalf("action offered: %+v", action)
			}
		})
	}
}

func TestTokenSavingPeek(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		call    string
		want    string
		savings int
	}{
		{name: "peek", text: "local a = peek(0x5f00)\n", call: "peek(", want: "local a = @0x5f00\n", savings: 1},
		{name: "peek2", text: "local a = peek2(addr)\n", call: "peek2(", want: "local a = %addr\n", savings: 1},
		{name: "peek4", text: "local a = peek4(addr)\n", call: "peek4(", want: "local a = $addr\n", savings: 1},
		{name: "nested expression", text: "local a = 1 + peek(addr)\n", call: "peek(", want: "local a = 1 + @addr\n", savings: 1},
		{name: "unary argument", text: "local a = peek(-1)\n", call: "peek(", want: "local a = @-1\n", savings: 1},
		{name: "exponent argument", text: "local a = peek(a ^ b)\n", call: "peek(", want: "local a = @a ^ b\n", savings: 1},
		{name: "call argument", text: "local a = peek(f())\n", call: "peek(", want: "local a = @f()\n", savings: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tokenSavingTestContext(t, tt.text)
			action := TokenSavingPeek(ctx, offsetOf(t, tt.text, tt.call))
			if action == nil {
				t.Fatal("TokenSavingPeek returned nil")
			}
			if action.Kind != KindTokenSavingPeek {
				t.Fatalf("kind = %q", action.Kind)
			}
			if action.TokenSavings != tt.savings {
				t.Fatalf("savings = %d, want %d", action.TokenSavings, tt.savings)
			}
			if got := applyAction(t, tt.text, action); got != tt.want {
				t.Fatalf("result = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTokenSavingPeekRefusals(t *testing.T) {
	tests := []struct {
		name string
		text string
		call string
	}{
		{name: "bare statement", text: "peek(1)\n", call: "peek("},
		{name: "shadowed local", text: "local peek = f\nlocal a = peek(1)\n", call: "peek(1)"},
		{name: "reassigned global", text: "peek = f\nlocal a = peek(1)\n", call: "peek(1)"},
		{name: "field function", text: "local a = t.peek(1)\n", call: "peek("},
		{name: "wrong arity zero", text: "local a = peek()\n", call: "peek("},
		{name: "wrong arity two", text: "local a = peek(a, b)\n", call: "peek("},
		{name: "low precedence addition", text: "local a = peek(a + b)\n", call: "peek("},
		{name: "low precedence multiplication", text: "local a = peek(a * b)\n", call: "peek("},
		{name: "low precedence and", text: "local a = peek(a and b)\n", call: "peek("},
		{name: "comment inside", text: "local a = peek(a --[[note]])\n", call: "peek("},
		{name: "multi-line", text: "local a = peek(\n  a)\n", call: "peek("},
		{name: "poke is not peek", text: "local a = poke(1)\n", call: "poke("},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tokenSavingTestContext(t, tt.text)
			if action := TokenSavingPeek(ctx, offsetOf(t, tt.text, tt.call)); action != nil {
				t.Fatalf("action offered: %+v", action)
			}
		})
	}
}

func TestTokenSavingActionsSelectionAndOrder(t *testing.T) {
	text := "x = x + 1\nprint(x)\nlocal a = peek(x)\n"
	ctx := tokenSavingTestContext(t, text)
	actions := TokenSavingActions(ctx, 0, len(text))
	if len(actions) != 3 {
		t.Fatalf("actions = %d, want 3: %+v", len(actions), actions)
	}
	wantKinds := []string{KindTokenSavingCompound, KindTokenSavingPrint, KindTokenSavingPeek}
	wantStarts := []int{offsetOf(t, text, "x ="), offsetOf(t, text, "print"), offsetOf(t, text, "peek")}
	for i, want := range wantKinds {
		if actions[i].Kind != want {
			t.Fatalf("action %d kind = %q, want %q", i, actions[i].Kind, want)
		}
		if actions[i].Edits[0].Start != wantStarts[i] {
			t.Fatalf("action %d start = %d, want %d", i, actions[i].Edits[0].Start, wantStarts[i])
		}
	}

	cursor := offsetOf(t, text, "print")
	actions = TokenSavingActions(ctx, cursor, cursor)
	if len(actions) != 1 || actions[0].Kind != KindTokenSavingPrint {
		t.Fatalf("cursor actions = %+v", actions)
	}

	// A selection around only the peek call excludes the enclosing statement.
	start := offsetOf(t, text, "peek(x)")
	actions = TokenSavingActions(ctx, start, start+len("peek(x)"))
	if len(actions) != 1 || actions[0].Kind != KindTokenSavingPeek {
		t.Fatalf("selection actions = %+v", actions)
	}

	// A print call whose final argument is another call stays refused until
	// multi-value expansion has a parity fixture.
	nested := "print(peek(x))\n"
	ctx = tokenSavingTestContext(t, nested)
	cursor = offsetOf(t, nested, "peek")
	actions = TokenSavingActions(ctx, cursor, cursor)
	if len(actions) != 1 || actions[0].Kind != KindTokenSavingPeek {
		t.Fatalf("nested multi-value actions = %+v", actions)
	}

	// Non-call nesting keeps both independent candidates.
	nested = "print(1 + peek(x))\n"
	ctx = tokenSavingTestContext(t, nested)
	cursor = offsetOf(t, nested, "peek")
	actions = TokenSavingActions(ctx, cursor, cursor)
	if len(actions) != 2 {
		t.Fatalf("nested actions = %+v", actions)
	}
	if actions[0].Kind != KindTokenSavingPrint || actions[1].Kind != KindTokenSavingPeek {
		t.Fatalf("nested kinds = %q, %q", actions[0].Kind, actions[1].Kind)
	}
	if len(actions[0].Edits) != 1 || len(actions[1].Edits) != 1 {
		t.Fatalf("actions must not merge edits: %+v", actions)
	}
}

func TestTokenSavingMetaFileRefused(t *testing.T) {
	text := "---@meta\nprint(x)\n"
	ctx := tokenSavingTestContext(t, text)
	if actions := TokenSavingActions(ctx, 0, len(text)); len(actions) != 0 {
		t.Fatalf("meta actions = %+v", actions)
	}
}

func TestTokenSavingTokenDeltaHelpers(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"x = -1", 3},
		{"x += 1", 3},
		{"x = x + 1", 5},
		{"print(a) -- note", 3},
		{"---@type number\nlocal x", 1},
	}
	for _, tt := range tests {
		got, ok := fragmentTokenCount(tt.text)
		if !ok || got != tt.want {
			t.Fatalf("fragmentTokenCount(%q) = %d, %v; want %d", tt.text, got, ok, tt.want)
		}
	}
}

func TestTokenSavingCorpusActionsAreSafe(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "carts")
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".p8" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}
	if len(files) < 50 {
		t.Fatalf("corpus has %d carts, want at least 50", len(files))
	}
	builtins := loadTokenBuiltins(t)
	loader := cart.NewLoader(cart.DiskProvider{}, "", nil)
	checked := 0
	for _, path := range files {
		abs, err := filepath.Abs(path)
		if err != nil {
			t.Fatalf("abs: %v", err)
		}
		rootURI := uri.File(abs)
		snap, err := loader.Load(rootURI)
		if err != nil {
			t.Fatalf("%s: load: %v", path, err)
		}
		result := analysis.Analyze(snap, builtins, analysis.Options{
			UndefinedGlobal: analysis.SeverityWarning,
			TypeChecking:    analysis.TypeCheckingBasic,
			Lint: analysis.LintOptions{
				UnusedLocal: analysis.SeverityHint,
			},
		})
		beforeCodes := diagnosticCodeCounts(result.Diagnostics)
		for _, file := range snap.Cart.Files {
			ctx := &Context{Snap: snap, Result: result, FileID: file.ID, File: file}
			actions := TokenSavingActions(ctx, 0, len(file.Text))
			for _, action := range actions {
				checked++
				if len(action.Edits) != 1 {
					t.Fatalf("%s %s: edits = %+v", path, action.Title, action.Edits)
				}
				edit := action.Edits[0]
				if edit.FileID != file.ID || edit.Start < 0 || edit.End > len(file.Text) || edit.Start > edit.End {
					t.Fatalf("%s %s: invalid edit %+v", path, action.Title, edit)
				}
				edited := file.Text[:edit.Start] + edit.NewText + file.Text[edit.End:]
				overlay := overlayProvider{DiskProvider: cart.DiskProvider{}, uri: file.URI, data: []byte(edited)}
				next, err := cart.NewLoader(overlay, "", nil).Load(rootURI)
				if err != nil {
					t.Fatalf("%s %s: reload: %v", path, action.Title, err)
				}
				if len(next.Errors) != 0 {
					t.Fatalf("%s %s: syntax errors after edit: %+v", path, action.Title, next.Errors)
				}
				if got := snap.Cart.Tokens - next.Cart.Tokens; got != action.TokenSavings {
					t.Fatalf("%s %s: token delta = %d, want %d", path, action.Title, got, action.TokenSavings)
				}
				nextResult := analysis.Analyze(next, builtins, analysis.Options{
					UndefinedGlobal: analysis.SeverityWarning,
					TypeChecking:    analysis.TypeCheckingBasic,
					Lint: analysis.LintOptions{
						UnusedLocal: analysis.SeverityHint,
					},
				})
				for code, count := range diagnosticCodeCounts(nextResult.Diagnostics) {
					if count > beforeCodes[code] {
						t.Fatalf("%s %s: new diagnostic %s (%d > %d)", path, action.Title, code, count, beforeCodes[code])
					}
				}
				assertIncludeCountsUnchanged(t, snap, next)
			}
		}
	}
	if checked == 0 {
		t.Fatal("corpus produced no token-saving actions")
	}
	t.Logf("checked %d token-saving actions", checked)
}

type overlayProvider struct {
	cart.DiskProvider
	uri  uri.URI
	data []byte
}

func (p overlayProvider) Read(u uri.URI) ([]byte, error) {
	if u == p.uri {
		return p.data, nil
	}
	return p.DiskProvider.Read(u)
}

func (p overlayProvider) Size(u uri.URI) (int64, error) {
	if u == p.uri {
		return int64(len(p.data)), nil
	}
	return p.DiskProvider.Size(u)
}

func (p overlayProvider) ReadLimit(u uri.URI, max int) ([]byte, error) {
	if u == p.uri {
		data := p.data
		if max >= 0 && len(data) > max+1 {
			data = data[:max+1]
		}
		return data, nil
	}
	return p.DiskProvider.ReadLimit(u, max)
}

func diagnosticCodeCounts(diagnostics []analysis.Diagnostic) map[string]int {
	out := make(map[string]int)
	for _, diag := range diagnostics {
		out[diag.Code]++
	}
	return out
}

func assertIncludeCountsUnchanged(t *testing.T, before, after *cart.Snapshot) {
	t.Helper()
	count := func(snap *cart.Snapshot) map[string]int {
		out := make(map[string]int)
		for _, include := range snap.Cart.Includes {
			if include.DirectiveSpan.IsEmpty() {
				continue
			}
			file := snap.Cart.File(include.DirectiveSpan.FileID)
			if file == nil {
				continue
			}
			out[file.Text[include.DirectiveSpan.Start:include.DirectiveSpan.End]]++
		}
		return out
	}
	want, got := count(before), count(after)
	if len(want) != len(got) {
		t.Fatalf("include directives changed: %v -> %v", want, got)
	}
	for directive, count := range want {
		if got[directive] != count {
			t.Fatalf("include directive %q count changed: %d -> %d", directive, count, got[directive])
		}
	}
}
