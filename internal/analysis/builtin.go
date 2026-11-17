package analysis

import "github.com/asimmons91/pico-lsp/internal/builtin"

// BuildBuiltins converts the parsed API stub into shared built-in symbols.
// The returned map is immutable after construction and can be shared by every
// cart analysis in the server.
func BuildBuiltins(api *builtin.API) map[string]*Symbol {
	if api == nil {
		return nil
	}
	table := &SymbolTable{}
	out := make(map[string]*Symbol, len(api.Names))
	for _, name := range api.Names {
		canonical := CanonicalName(name.Name)
		if _, exists := out[canonical]; exists {
			continue
		}
		sym := table.NewSymbol(SymbolBuiltin, name.Name, name.Span, nil)
		sym.Builtin = true
		sym.Signature = name.Signature
		sym.Params = append([]string(nil), name.Params...)
		if name.Docs != nil {
			doc := &DocComment{
				Summary:    name.Docs.Summary,
				Type:       name.Docs.Type,
				Returns:    append([]string(nil), name.Docs.Returns...),
				Deprecated: name.Docs.Deprecated,
			}
			for _, param := range name.Docs.Params {
				doc.Params = append(doc.Params, ParamDoc{
					Name:     param.Name,
					Type:     param.Type,
					Doc:      param.Doc,
					Optional: param.Optional,
				})
			}
			sym.Doc = doc
		}
		out[canonical] = sym
	}
	return out
}
