// Package glyphs exposes PICO-8's built-in P8SCII glyph catalog and the custom
// glyph definitions used by pico8.glyphs.
//
// The catalog is the same data that backs pico8/glyphs and glyph completion in
// the language server; it is not a second copy. Entries describe insertion
// text, the P8SCII byte, aliases, and whether a glyph is valid in code, inside
// a string, or both.
//
// The package is a thin, stable facade over the server's canonical catalog. It
// is additive from v1.8.0 and does not change the LSP server.
package glyphs

import internalglyphs "github.com/asimmons91/pico-lsp/internal/glyphs"

// Context controls where a glyph may be inserted.
type Context = internalglyphs.Context

const (
	// ContextBoth allows insertion in code and strings.
	ContextBoth Context = internalglyphs.ContextBoth
	// ContextString allows insertion only inside string literals.
	ContextString Context = internalglyphs.ContextString
)

// Definition is the user-configurable form of a custom glyph for pico8.glyphs.
// Exactly one of Byte, Glyph, or Text must be set.
type Definition = internalglyphs.Definition

// Entry is a resolved glyph in the server catalog. Its JSON tags are part of
// the public protocol contract used by pico8/glyphs.
type Entry = internalglyphs.Entry

// Builtins returns a copy of the built-in catalog in declaration order.
// Callers may mutate the slice and its entries without affecting later calls.
func Builtins() []Entry {
	return internalglyphs.Builtins()
}

// Build merges custom definitions into the built-in catalog. Invalid
// definitions are skipped and reported as warnings. Custom entries are applied
// in order; later definitions win on a name or trigger collision.
func Build(custom []Definition) ([]Entry, []string) {
	return internalglyphs.Build(custom)
}

// NormalizeDefinition validates a custom definition and resolves its canonical
// text and P8SCII byte.
func NormalizeDefinition(definition Definition) (Entry, error) {
	return internalglyphs.NormalizeDefinition(definition)
}

// FindByName resolves an entry name or trigger, case-insensitively. Later
// entries win so custom definitions override built-in aliases.
func FindByName(entries []Entry, name string) (Entry, bool) {
	return internalglyphs.FindByName(entries, name)
}

// FindByByte resolves an entry with a P8SCII byte. Later entries win so custom
// definitions override built-in entries that share a slot.
func FindByByte(entries []Entry, value byte) (Entry, bool) {
	return internalglyphs.FindByByte(entries, value)
}
