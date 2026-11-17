package analysis

import (
	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/builtin"
)

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
		sym.Type = builtinSymbolType(name)
		sym.TypeExplicit = true
		out[canonical] = sym
	}
	return out
}

func builtinSymbolType(name builtin.Name) types.Type {
	if name.Kind == builtin.KindConstant {
		if name.Docs != nil && name.Docs.Type != "" {
			if parsed, err := parseType(name.Docs.Type, nil); err == nil {
				return parsed
			}
		}
		return types.NumberType
	}
	fn := &types.Function{}
	for _, paramName := range name.Params {
		var paramType types.Type = types.AnyType
		optional := false
		if name.Docs != nil {
			for _, doc := range name.Docs.Params {
				if equalFold(doc.Name, paramName) {
					if parsed, err := parseType(doc.Type, nil); err == nil {
						paramType = parsed
					}
					optional = doc.Optional
					break
				}
			}
		}
		if paramName == "..." {
			fn.Vararg = paramType
			continue
		}
		fn.Params = append(fn.Params, types.Param{Name: paramName, Type: paramType, Optional: optional, Explicit: true})
	}
	if name.Docs != nil {
		for _, raw := range name.Docs.Returns {
			if parsed, _, err := parseTypePrefix(raw, nil); err == nil {
				fn.Returns.Elems = append(fn.Returns.Elems, parsed)
			}
		}
	}
	if len(fn.Returns.Elems) == 0 {
		if fallback := builtinFallbackReturn(name.Name); fallback != nil {
			fn.Returns.Elems = []types.Type{fallback}
		}
	}
	return fn
}

func builtinFallbackReturn(name string) types.Type {
	switch CanonicalName(name) {
	case "btn", "btnp", "fget", "rawequal", "coresume":
		return types.BooleanType
	case "type", "tostr", "chr", "sub", "costatus":
		return types.StringType
	case "tonum", "ord", "count", "peek", "peek2", "peek4", "dget",
		"abs", "atan2", "band", "bnot", "bor", "bxor", "ceil", "cos",
		"flr", "lshr", "max", "mid", "min", "rawlen", "rnd", "rotl",
		"rotr", "shl", "shr", "sgn", "sin", "sqrt":
		return types.NumberType
	case "all", "pairs", "ipairs", "foreach":
		return types.FunctionType
	case "cocreate":
		return types.ThreadType
	case "split", "pack", "getmetatable":
		return types.TableType
	}
	return nil
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
