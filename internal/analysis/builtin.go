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
		out[canonical] = sym
	}
	return out
}
