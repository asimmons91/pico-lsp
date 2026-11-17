package lsp

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/builtin"
	"github.com/asimmons91/pico-lsp/internal/cart"
)

const maxHoverMarkdown = 8000

func symbolCode(sym *analysis.Symbol) string {
	if sym == nil {
		return ""
	}
	if fn, ok := types.AsFunction(sym.Type); ok {
		return typedFunctionSignature(sym, fn)
	}
	if sym.Kind == analysis.SymbolField && sym.Owner != nil {
		if record, ok := types.Resolve(sym.Owner.Type).(*types.Record); ok && record != nil && record.Name != "" {
			return record.Name + "." + sym.Name
		}
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

func effectiveSymbolType(sym *analysis.Symbol) types.Type {
	if sym == nil {
		return nil
	}
	if sym.Type != nil {
		return sym.Type
	}
	if sym.Kind == analysis.SymbolField && sym.Owner != nil && sym.Owner.Type != nil {
		if fieldType, ok, optional := types.LookupField(sym.Owner.Type, sym.Name); ok {
			if optional {
				return types.Optional(fieldType)
			}
			return fieldType
		}
	}
	return nil
}

func namedRecord(t types.Type) *types.Record {
	record, ok := types.Resolve(t).(*types.Record)
	if !ok || record == nil || record.Name == "" {
		return nil
	}
	return record
}

func typedFunctionSignature(sym *analysis.Symbol, fn *types.Function) string {
	if sym == nil || fn == nil {
		return ""
	}
	name := sym.Name
	if strings.HasPrefix(sym.Signature, "function ") {
		if index := strings.Index(sym.Signature, "("); index > len("function ") {
			name = strings.TrimPrefix(sym.Signature[:index], "function ")
		}
	}
	params := make([]string, 0, len(fn.Params)+1)
	for _, param := range fn.Params {
		label := param.Name
		if label == "" {
			label = "arg"
		}
		if param.Optional {
			label += "?"
		}
		label += ": " + types.Display(param.Type)
		params = append(params, label)
	}
	if fn.Vararg != nil {
		params = append(params, "...: "+types.Display(fn.Vararg))
	}
	signature := "function " + name + "(" + strings.Join(params, ", ") + ")"
	if len(fn.Returns.Elems) == 1 && fn.Returns.Vararg == nil {
		signature += ": " + types.Display(fn.Returns.Elems[0])
	} else if len(fn.Returns.Elems) > 1 {
		signature += ": " + types.Display(fn.Returns)
	}
	return signature
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
	} else if typ := effectiveSymbolType(sym); typ != nil {
		if _, isFunction := types.AsFunction(typ); !isFunction {
			b.WriteString("\n\n**Type:** `")
			b.WriteString(types.Display(typ))
			b.WriteString("`")
		}
	}
	if record := namedRecord(effectiveSymbolType(sym)); record != nil {
		fields := types.Fields(record)
		if len(fields) > 0 {
			b.WriteString("\n\n**Fields:**")
			for i, field := range fields {
				if i >= 8 {
					b.WriteString("\n- ...")
					break
				}
				b.WriteString("\n- `")
				b.WriteString(field.Name)
				b.WriteString("`: ")
				b.WriteString("`")
				b.WriteString(types.Display(field.Type))
				b.WriteString("`")
				if field.Optional {
					b.WriteString(" optional")
				}
			}
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
	if sym == nil {
		return ""
	}
	if sym.Doc == nil {
		if typ := effectiveSymbolType(sym); typ != nil && !types.IsAny(typ) {
			return "**Type:** `" + types.Display(typ) + "`"
		}
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
	} else if typ := effectiveSymbolType(sym); typ != nil {
		if _, isFunction := types.AsFunction(typ); !isFunction {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString("**Type:** `")
			b.WriteString(types.Display(typ))
			b.WriteString("`")
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
