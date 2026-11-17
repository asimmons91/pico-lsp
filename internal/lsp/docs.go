package lsp

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/builtin"
	"github.com/asimmons91/pico-lsp/internal/cart"
)

const maxHoverMarkdown = 8000

func symbolCode(sym *analysis.Symbol) string {
	if sym == nil {
		return ""
	}
	if sym.Signature != "" {
		return sym.Signature
	}
	switch sym.Kind {
	case analysis.SymbolGlobal:
		return "global " + sym.Name
	case analysis.SymbolLocal:
		return "local " + sym.Name
	case analysis.SymbolParam:
		return "parameter " + sym.Name
	case analysis.SymbolField:
		return "field " + sym.Name
	case analysis.SymbolMethod:
		return "method " + sym.Name
	case analysis.SymbolLabel:
		return "label " + sym.Name
	case analysis.SymbolClass:
		return "class " + sym.Name
	case analysis.SymbolBuiltin:
		return sym.Name
	default:
		return sym.Name
	}
}

func symbolMarkdown(nav *navContext, sym *analysis.Symbol) string {
	if sym == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("```lua\n")
	b.WriteString(symbolCode(sym))
	b.WriteString("\n```")
	if sym.Doc != nil && strings.TrimSpace(sym.Doc.Summary) != "" {
		b.WriteString("\n\n")
		b.WriteString(markdownInline(sym.Doc.Summary))
	}
	if sym.Doc != nil && sym.Doc.Type != "" {
		b.WriteString("\n\n**Type:** `")
		b.WriteString(sym.Doc.Type)
		b.WriteString("`")
		if sym.Doc.Optional {
			b.WriteString(" (optional)")
		}
	}
	if sym.Doc != nil && len(sym.Doc.Params) > 0 {
		b.WriteString("\n\n**Parameters**")
		for _, param := range sym.Doc.Params {
			b.WriteString("\n- `")
			b.WriteString(param.Name)
			b.WriteString("`")
			if param.Type != "" {
				b.WriteString(" (`")
				b.WriteString(param.Type)
				b.WriteString("`)")
			}
			if param.Optional {
				b.WriteString(" optional")
			}
			if param.Doc != "" {
				b.WriteString(": ")
				b.WriteString(markdownInline(param.Doc))
			}
		}
	}
	if sym.Doc != nil && len(sym.Doc.Returns) > 0 {
		b.WriteString("\n\n**Returns:** ")
		b.WriteString(strings.Join(sym.Doc.Returns, ", "))
	}
	if sym.Doc != nil && sym.Doc.Deprecated {
		b.WriteString("\n\n**Deprecated.**")
	}
	if sym.Builtin {
		b.WriteString("\n\nPICO-8 0.2.7 API")
	} else if nav != nil && nav.snap != nil {
		source := workspaceContainer(nav.snap, nav.snap.Cart.RootURI, sym.DeclSpan)
		if source != "" {
			b.WriteString("\n\n")
			b.WriteString(source)
		}
	}
	return truncateMarkdown(b.String())
}

func declarationSource(nav *navContext, sym *analysis.Symbol) string {
	if nav == nil || nav.snap == nil || sym == nil || sym.DeclSpan.IsEmpty() {
		return ""
	}
	return workspaceContainer(nav.snap, nav.snap.Cart.RootURI, sym.DeclSpan)
}

func symbolDocumentation(sym *analysis.Symbol) string {
	if sym == nil || sym.Doc == nil {
		return ""
	}
	var b strings.Builder
	if summary := strings.TrimSpace(sym.Doc.Summary); summary != "" {
		b.WriteString(markdownInline(summary))
	}
	if sym.Doc.Type != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("**Type:** `")
		b.WriteString(sym.Doc.Type)
		b.WriteString("`")
		if sym.Doc.Optional {
			b.WriteString(" (optional)")
		}
	}
	if len(sym.Doc.Params) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("**Parameters**")
		for _, param := range sym.Doc.Params {
			b.WriteString("\n- `")
			b.WriteString(param.Name)
			b.WriteString("`")
			if param.Type != "" {
				b.WriteString(" (`")
				b.WriteString(param.Type)
				b.WriteString("`)")
			}
			if param.Optional {
				b.WriteString(" optional")
			}
			if param.Doc != "" {
				b.WriteString(": ")
				b.WriteString(markdownInline(param.Doc))
			}
		}
	}
	if len(sym.Doc.Returns) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("**Returns:** ")
		b.WriteString(strings.Join(sym.Doc.Returns, ", "))
	}
	if sym.Doc.Deprecated {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("**Deprecated.**")
	}
	return truncateMarkdown(b.String())
}

// markdownInline escapes prose that could be mistaken for Markdown structure.
// Backticks are left intact so doc comments can use inline code.
func markdownInline(value string) string {
	return strings.NewReplacer("[", `\[`, "]", `\]`).Replace(value)
}

func truncateMarkdown(value string) string {
	if len(value) <= maxHoverMarkdown {
		return value
	}
	limit := maxHoverMarkdown
	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}
	return value[:limit] + "\n\nSee the [PICO-8 manual](https://www.lexaloffle.com/pico-8.php?page=manual) for the full documentation.\n\n*(truncated)*"
}

func builtinMarkdown(name builtin.Name) string {
	var b strings.Builder
	if name.Signature != "" {
		b.WriteString("```lua\n")
		b.WriteString(name.Signature)
		b.WriteString("\n```")
	}
	if name.Docs != nil {
		if summary := strings.TrimSpace(name.Docs.Summary); summary != "" {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(markdownInline(summary))
		}
		if name.Docs.Type != "" {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString("**Type:** `")
			b.WriteString(name.Docs.Type)
			b.WriteString("`")
		}
		if len(name.Docs.Params) > 0 {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString("**Parameters**")
			for _, param := range name.Docs.Params {
				b.WriteString("\n- `")
				b.WriteString(param.Name)
				b.WriteString("`")
				if param.Type != "" {
					b.WriteString(" (`")
					b.WriteString(param.Type)
					b.WriteString("`)")
				}
				if param.Optional {
					b.WriteString(" optional")
				}
				if param.Doc != "" {
					b.WriteString(": ")
					b.WriteString(markdownInline(param.Doc))
				}
			}
		}
		if len(name.Docs.Returns) > 0 {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString("**Returns:** ")
			b.WriteString(strings.Join(name.Docs.Returns, ", "))
		}
	}
	if b.Len() > 0 {
		b.WriteString("\n\nPICO-8 0.2.7 API")
	}
	return truncateMarkdown(b.String())
}

func includeMarkdown(nav *navContext, include *cart.Include) string {
	if include == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("```lua\n")
	b.WriteString("#include \"")
	b.WriteString(include.Path)
	b.WriteString("\"\n```")
	if include.Missing {
		b.WriteString("\n\nIncluded file is missing.")
	} else if include.Unreadable {
		b.WriteString("\n\nIncluded file is unreadable.")
	} else if include.ResolvedURI != "" {
		resolved := include.ResolvedURI.FsPath()
		if nav != nil && nav.snap != nil && nav.snap.Cart != nil {
			root := filepath.Dir(nav.snap.Cart.RootURI.FsPath())
			if relative, err := filepath.Rel(root, resolved); err == nil && !strings.HasPrefix(relative, "..") {
				resolved = filepath.ToSlash(relative)
			}
		}
		b.WriteString("\n\nResolved path: `")
		b.WriteString(resolved)
		b.WriteString("`")
	}
	b.WriteString("\n\nIncludes are flattened into the cart and are non-recursive.")
	return truncateMarkdown(b.String())
}

func shorthandMarkdown() string {
	return "```lua\n?expr\n```\n\n`?` is PICO-8 shorthand for `print(expr)`."
}

func peekMarkdown(operator string) string {
	name := "peek"
	switch operator {
	case "%":
		name = "peek2"
	case "$":
		name = "peek4"
	}
	return "```lua\n" + operator + "addr\n```\n\n`" + operator + "` reads a value from memory using `" + name + "()`."
}
