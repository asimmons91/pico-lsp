package analysis

// Scope is one lexical scope. A scope may contain several declarations with
// the same canonical name; lookup returns the most recent one.
type Scope struct {
	Parent  *Scope
	symbols map[string][]*Symbol

	// Labels are function-scoped in Lua/PICO-8. Only function and chunk scopes
	// use this map.
	Labels map[string]*Symbol
}

// NewScope creates a scope with parent.
func NewScope(parent *Scope) *Scope {
	return &Scope{
		Parent:  parent,
		symbols: make(map[string][]*Symbol),
	}
}

// Declare adds sym to s.
func (s *Scope) Declare(sym *Symbol) {
	if s == nil || sym == nil {
		return
	}
	key := sym.Canonical
	if key == "" {
		key = CanonicalName(sym.Name)
	}
	s.symbols[key] = append(s.symbols[key], sym)
}

// Lookup finds the most recent declaration visible from s.
func (s *Scope) Lookup(canonical string) *Symbol {
	for scope := s; scope != nil; scope = scope.Parent {
		if list := scope.symbols[canonical]; len(list) > 0 {
			return list[len(list)-1]
		}
	}
	return nil
}

// IsAncestorOf reports whether s is ancestor or equal to other.
func (s *Scope) IsAncestorOf(other *Scope) bool {
	for scope := other; scope != nil; scope = scope.Parent {
		if scope == s {
			return true
		}
	}
	return false
}
