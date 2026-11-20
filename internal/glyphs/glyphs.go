// Package glyphs describes PICO-8 P8SCII glyphs and user-defined glyph
// snippets. It owns the built-in names and aliases shared by completion and the
// glyph picker; encoding stays in internal/p8scii.
package glyphs

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/p8scii"
)

// Context controls where a glyph may be inserted.
type Context string

const (
	// ContextBoth allows insertion in code and strings.
	ContextBoth Context = "both"
	// ContextString allows insertion only inside string literals.
	ContextString Context = "string"
)

// Definition is the user-configurable form of a custom glyph for pico8.glyphs.
type Definition struct {
	Name        string   `json:"name"`
	Glyph       string   `json:"glyph,omitempty"`
	Byte        *int     `json:"byte,omitempty"`
	Text        string   `json:"text,omitempty"`
	Context     Context  `json:"context,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	Description string   `json:"description,omitempty"`
}

// Entry is a resolved glyph in the server catalog.
type Entry struct {
	Name string `json:"name"`
	Text string `json:"text"`
	// StringText is the source spelling to insert inside a string literal when
	// it differs from the canonical glyph text, for example \" and \\.
	StringText  string   `json:"stringText,omitempty"`
	Byte        byte     `json:"byte"`
	Aliases     []string `json:"aliases,omitempty"`
	Category    string   `json:"category"`
	Description string   `json:"description,omitempty"`
	Context     Context  `json:"context"`
	Custom      bool     `json:"custom,omitempty"`
}

// Trigger returns the default completion trigger for the entry.
func (e Entry) Trigger() string {
	return "p8_" + asciiLower(e.Name)
}

// AllTriggers returns the default trigger followed by explicit aliases.
func (e Entry) AllTriggers() []string {
	triggers := make([]string, 0, len(e.Aliases)+1)
	triggers = append(triggers, e.Trigger())
	for _, alias := range e.Aliases {
		if alias != "" && asciiLower(alias) != e.Trigger() {
			triggers = append(triggers, alias)
		}
	}
	return triggers
}

type builtinSpec struct {
	byte        byte
	name        string
	category    string
	description string
	aliases     []string
}

// asciiStringText holds the source spellings that differ inside Lua strings.
var asciiStringText = map[byte]string{
	'"':  `\"`,
	'\\': `\\`,
}

var builtinSpecs = []builtinSpec{
	{0x10, "rect_vertical", "symbol", "Vertical rectangle", []string{"p8-rect-vertical"}},
	{0x11, "square_filled", "symbol", "Filled square", []string{"p8-square-filled"}},
	{0x12, "square_hollow", "symbol", "Hollow square", []string{"p8-square-hollow"}},
	{0x13, "dot5", "symbol", "Five-dot pattern", []string{"p8-dot5"}},
	{0x14, "dot4", "symbol", "Four-dot pattern", []string{"p8-dot4"}},
	{0x15, "pause", "symbol", "Pause bars", []string{"p8-pause"}},
	{0x16, "back", "symbol", "Back arrow", []string{"p8-back"}},
	{0x17, "forward", "symbol", "Forward arrow", []string{"p8-forward"}},
	{0x18, "jp_start_quote", "symbol", "Japanese starting quote", []string{"p8-jp-start-quote"}},
	{0x19, "jp_end_quote", "symbol", "Japanese ending quote", []string{"p8-jp-end-quote"}},
	{0x1a, "yen", "symbol", "Yen sign", []string{"p8-yen"}},
	{0x1b, "interpunct", "symbol", "Interpunct", []string{"p8-interpunct"}},
	{0x1c, "jp_comma", "symbol", "Japanese comma", []string{"p8-jp-comma"}},
	{0x1d, "jp_fullstop", "symbol", "Japanese full stop", []string{"p8-jp-fullstop"}},
	{0x1e, "jp_dakuten", "symbol", "Japanese dakuten", []string{"p8-jp-dakuten"}},
	{0x1f, "jp_handakuten", "symbol", "Japanese handakuten", []string{"p8-jp-handakuten"}},
	{0x7f, "circle_hollow", "symbol", "Hollow circle", []string{"p8-circ-hollow"}},
	{0x80, "rect", "symbol", "Rectangle", []string{"p8-rect"}},
	{0x81, "checkerboard", "symbol", "Checkerboard", []string{"p8-checkerboard"}},
	{0x82, "jelpi", "symbol", "Jelpi cat", []string{"p8-jelpi"}},
	{0x83, "down", "button", "Down button", []string{"p8-down"}},
	{0x84, "dot_pattern", "symbol", "Dot pattern", []string{"p8-dot-pattern"}},
	{0x85, "star_throwing", "symbol", "Throwing star", []string{"p8-star-throwing"}},
	{0x86, "ball", "symbol", "Ball", []string{"p8-ball"}},
	{0x87, "heart", "symbol", "Heart", []string{"p8-heart"}},
	{0x88, "eye", "symbol", "Eye", []string{"p8-eye"}},
	{0x89, "man", "symbol", "Man", []string{"p8-man"}},
	{0x8a, "house", "symbol", "House", []string{"p8-house"}},
	{0x8b, "left", "button", "Left button", []string{"p8-left"}},
	{0x8c, "face", "symbol", "Face", []string{"p8-face"}},
	{0x8d, "note", "symbol", "Musical note", []string{"p8-note"}},
	{0x8e, "o", "button", "O button", []string{"p8-o"}},
	{0x8f, "diamond", "symbol", "Diamond", []string{"p8-diamond"}},
	{0x90, "ellipsis", "symbol", "Ellipsis", []string{"p8-ellipsis"}},
	{0x91, "right", "button", "Right button", []string{"p8-right"}},
	{0x92, "star5", "symbol", "Five-pointed star", []string{"p8-star-5p"}},
	{0x93, "hourglass", "symbol", "Hourglass", []string{"p8-hourglass"}},
	{0x94, "up", "button", "Up button", []string{"p8-up"}},
	{0x95, "birds", "symbol", "Birds", []string{"p8-birds"}},
	{0x96, "sawtooth", "symbol", "Sawtooth", []string{"p8-sawtooth"}},
	{0x97, "x", "button", "X button", []string{"p8-x-key"}},
	{0x98, "horiz_lines", "symbol", "Horizontal lines", []string{"p8-horiz-lines"}},
	{0x99, "vert_lines", "symbol", "Vertical lines", []string{"p8-vert-lines"}},
	{0xfe, "left_arc", "symbol", "Left arc", []string{"p8-left-arc"}},
	{0xff, "right_arc", "symbol", "Right arc", []string{"p8-right-arc"}},
}

type controlSpec struct {
	byte        byte
	name        string
	escape      string
	description string
}

var controlSpecs = []controlSpec{
	{0x01, "repeat", `\*`, "Repeat the next character"},
	{0x02, "background", `\#`, "Draw a solid background"},
	{0x03, "cursor_x", `\-`, "Move the cursor horizontally"},
	{0x04, "cursor_y", `\|`, "Move the cursor vertically"},
	{0x05, "cursor", `\+`, "Move the cursor"},
	{0x06, "special", `\^`, "Run a P8SCII special command"},
	{0x07, "audio", `\a`, "Run an audio command"},
	{0x0c, "foreground", `\f`, "Set the foreground color"},
	{0x0e, "font_custom", `\14`, "Switch to the custom font at 0x5600"},
	{0x0f, "font_default", `\15`, "Switch to the default font"},
}

// asciiSpecs covers printable ASCII 0x20-0x7e. Uppercase ASCII is the puny
// font's lowercase-shaped glyph set; lowercase ASCII is the standard font.
var asciiSpecs = buildASCIISpecs()

// kanaSpecs covers the complete Hiragana and Katakana ranges 0x9a-0xfd.
var kanaSpecs = buildKanaSpecs()

var builtinEntries = buildBuiltins()

func buildBuiltins() []Entry {
	specs := make([]builtinSpec, 0, len(builtinSpecs)+len(asciiSpecs)+len(kanaSpecs)+len(controlSpecs))
	specs = append(specs, builtinSpecs...)
	specs = append(specs, asciiSpecs...)
	specs = append(specs, kanaSpecs...)

	entries := make([]Entry, 0, len(specs)+len(controlSpecs))
	for _, spec := range specs {
		entries = append(entries, Entry{
			Name:        spec.name,
			Text:        p8scii.Decode([]byte{spec.byte}),
			StringText:  asciiStringText[spec.byte],
			Byte:        spec.byte,
			Aliases:     append([]string(nil), spec.aliases...),
			Category:    spec.category,
			Description: spec.description,
			Context:     ContextBoth,
		})
	}
	for _, spec := range controlSpecs {
		entries = append(entries, Entry{
			Name:        spec.name,
			Text:        spec.escape,
			Byte:        spec.byte,
			Category:    "control",
			Description: spec.description,
			Context:     ContextString,
			Custom:      false,
		})
	}
	return entries
}

func buildASCIISpecs() []builtinSpec {
	specs := make([]builtinSpec, 0, 95)
	for b := byte(0x20); b <= 0x7e; b++ {
		name, description := asciiGlyph(b)
		specs = append(specs, builtinSpec{
			byte:        b,
			name:        name,
			category:    asciiCategory(b),
			description: description,
		})
	}
	return specs
}

func asciiCategory(b byte) string {
	switch {
	case b >= 'A' && b <= 'Z':
		return "puny"
	case b >= 'a' && b <= 'z':
		return "ascii"
	default:
		return "ascii"
	}
}

func asciiGlyph(b byte) (string, string) {
	if b >= '0' && b <= '9' {
		return "digit_" + string(b), "Digit " + string(b)
	}
	if b >= 'a' && b <= 'z' {
		return "letter_" + string(b), "Letter " + string(b)
	}
	if b >= 'A' && b <= 'Z' {
		lower := string(b - 'A' + 'a')
		return "puny_" + lower, "Puny " + string(b)
	}
	if name, ok := asciiPunctuation[b]; ok {
		return name.name, name.description
	}
	return fmt.Sprintf("ascii_%02x", b), fmt.Sprintf("ASCII 0x%02x", b)
}

var asciiPunctuation = map[byte]struct {
	name        string
	description string
}{
	0x20: {"space", "Space"},
	0x21: {"punct_exclam", "Exclamation mark"},
	0x22: {"punct_quotedbl", "Quotation mark"},
	0x23: {"punct_numbersign", "Number sign"},
	0x24: {"punct_dollar", "Dollar sign"},
	0x25: {"punct_percent", "Percent sign"},
	0x26: {"punct_ampersand", "Ampersand"},
	0x27: {"punct_apostrophe", "Apostrophe"},
	0x28: {"punct_parenleft", "Left parenthesis"},
	0x29: {"punct_parenright", "Right parenthesis"},
	0x2a: {"punct_asterisk", "Asterisk"},
	0x2b: {"punct_plus", "Plus sign"},
	0x2c: {"punct_comma", "Comma"},
	0x2d: {"punct_hyphen", "Hyphen-minus"},
	0x2e: {"punct_period", "Full stop"},
	0x2f: {"punct_slash", "Solidus"},
	0x3a: {"punct_colon", "Colon"},
	0x3b: {"punct_semicolon", "Semicolon"},
	0x3c: {"punct_less", "Less-than sign"},
	0x3d: {"punct_equal", "Equals sign"},
	0x3e: {"punct_greater", "Greater-than sign"},
	0x3f: {"punct_question", "Question mark"},
	0x40: {"punct_at", "Commercial at"},
	0x5b: {"punct_bracketleft", "Left square bracket"},
	0x5c: {"punct_backslash", "Reverse solidus"},
	0x5d: {"punct_bracketright", "Right square bracket"},
	0x5e: {"punct_asciicircum", "Circumflex accent"},
	0x5f: {"punct_underscore", "Low line"},
	0x60: {"punct_grave", "Grave accent"},
	0x7b: {"punct_braceleft", "Left curly bracket"},
	0x7c: {"punct_bar", "Vertical line"},
	0x7d: {"punct_braceright", "Right curly bracket"},
	0x7e: {"punct_asciitilde", "Tilde"},
}

var kanaLabels = []string{
	"a", "i", "u", "e", "o",
	"ka", "ki", "ku", "ke", "ko",
	"sa", "shi", "su", "se", "so",
	"ta", "chi", "tsu", "te", "to",
	"na", "ni", "nu", "ne", "no",
	"ha", "hi", "fu", "he", "ho",
	"ma", "mi", "mu", "me", "mo",
	"ya", "yu", "yo",
	"ra", "ri", "ru", "re", "ro",
	"wa", "wo", "n",
	"small_tsu", "small_ya", "small_yu", "small_yo",
}

func buildKanaSpecs() []builtinSpec {
	specs := make([]builtinSpec, 0, len(kanaLabels)*2)
	for i, label := range kanaLabels {
		specs = append(specs, builtinSpec{
			byte:        0x9a + byte(i),
			name:        "hiragana_" + label,
			category:    "kana",
			description: "Hiragana " + strings.ReplaceAll(label, "_", " "),
		})
	}
	for i, label := range kanaLabels {
		specs = append(specs, builtinSpec{
			byte:        0xcc + byte(i),
			name:        "katakana_" + label,
			category:    "kana",
			description: "Katakana " + strings.ReplaceAll(label, "_", " "),
		})
	}
	return specs
}

// Builtins returns a copy of the built-in catalog.
func Builtins() []Entry {
	return cloneEntries(builtinEntries)
}

// Build merges user definitions into the built-in catalog. Invalid definitions
// are skipped and reported as warnings. Custom entries are applied in order;
// later definitions win on a name or trigger collision.
func Build(custom []Definition) ([]Entry, []string) {
	warnings := make([]string, 0)
	customEntries := make([]Entry, 0, len(custom))
	customByName := make(map[string]int, len(custom))
	for i, definition := range custom {
		entry, err := NormalizeDefinition(definition)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("entry %d: %v", i+1, err))
			continue
		}
		if index, ok := customByName[entry.Name]; ok {
			// A later custom definition wins without duplicating the name.
			customEntries[index] = entry
			continue
		}
		customByName[entry.Name] = len(customEntries)
		customEntries = append(customEntries, entry)
	}

	// Custom entries always sort after built-ins so their names and aliases
	// win trigger collisions regardless of where the built-in was declared.
	entries := make([]Entry, 0, len(builtinEntries)+len(customEntries))
	for _, builtin := range builtinEntries {
		if _, overridden := customByName[builtin.Name]; overridden {
			warnings = append(warnings, fmt.Sprintf("%q overrides a built-in glyph", builtin.Name))
			continue
		}
		entries = append(entries, builtin)
	}
	entries = append(entries, customEntries...)

	// Later entries win when two entries claim the same trigger. Report the
	// conflict only for custom entries so built-in duplicates stay testable.
	owners := make(map[string]string)
	for _, entry := range entries {
		for _, trigger := range entry.AllTriggers() {
			key := asciiLower(trigger)
			if owner, ok := owners[key]; ok && owner != entry.Name && entry.Custom {
				warnings = append(warnings, fmt.Sprintf("%q trigger %q overrides %q", entry.Name, trigger, owner))
			}
			owners[key] = entry.Name
		}
	}
	return entries, warnings
}

// NormalizeDefinition validates a user definition and resolves its canonical
// text and P8SCII byte.
func NormalizeDefinition(definition Definition) (Entry, error) {
	name := asciiLower(strings.TrimSpace(definition.Name))
	if !validName(name) {
		return Entry{}, fmt.Errorf("name must match [a-z0-9_]+")
	}

	fields := 0
	if definition.Byte != nil {
		fields++
	}
	if definition.Glyph != "" {
		fields++
	}
	if definition.Text != "" {
		fields++
	}
	if fields != 1 {
		return Entry{}, fmt.Errorf("entry %q must set exactly one of byte, glyph, or text", name)
	}

	context := definition.Context
	if context == "" {
		context = ContextBoth
	}
	if context != ContextBoth && context != ContextString {
		return Entry{}, fmt.Errorf("entry %q has invalid context %q", name, context)
	}

	aliases, err := normalizeAliases(name, definition.Aliases)
	if err != nil {
		return Entry{}, err
	}

	if definition.Text != "" {
		if context != ContextString {
			return Entry{}, fmt.Errorf("entry %q text entries require context %q", name, ContextString)
		}
		value, ok := parseControlEscape(definition.Text)
		if !ok {
			return Entry{}, fmt.Errorf("entry %q text is not a P8SCII control escape", name)
		}
		return Entry{
			Name:        name,
			Text:        definition.Text,
			Byte:        value,
			Aliases:     aliases,
			Category:    "control",
			Description: description(definition.Description, "P8SCII control 0x%02x", value),
			Context:     ContextString,
			Custom:      true,
		}, nil
	}

	var value byte
	if definition.Byte != nil {
		if *definition.Byte < 0 || *definition.Byte > 255 {
			return Entry{}, fmt.Errorf("entry %q byte must be between 0 and 255", name)
		}
		value = byte(*definition.Byte)
	} else {
		encoded := p8scii.Encode(definition.Glyph)
		if len(encoded) != 1 {
			return Entry{}, fmt.Errorf("entry %q glyph must encode to exactly one P8SCII byte", name)
		}
		value = encoded[0]
	}
	if value < 0x10 {
		return Entry{}, fmt.Errorf("entry %q uses control byte 0x%02x; use text instead", name, value)
	}

	text := p8scii.Decode([]byte{value})
	encoded := p8scii.Encode(text)
	if len(encoded) != 1 || encoded[0] != value {
		return Entry{}, fmt.Errorf("entry %q does not round-trip through P8SCII", name)
	}
	return Entry{
		Name:        name,
		Text:        text,
		Byte:        value,
		Aliases:     aliases,
		Category:    "custom",
		Description: description(definition.Description, "P8SCII 0x%02x", value),
		Context:     context,
		Custom:      true,
	}, nil
}

// FindByName resolves an entry name or trigger, case-insensitively. Later
// entries win so custom definitions override built-in aliases.
func FindByName(entries []Entry, name string) (Entry, bool) {
	key := asciiLower(strings.TrimSpace(name))
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if asciiLower(entry.Name) == key {
			return entry, true
		}
		for _, trigger := range entry.AllTriggers() {
			if asciiLower(trigger) == key {
				return entry, true
			}
		}
	}
	return Entry{}, false
}

// FindByByte resolves an entry with a P8SCII byte. Later entries win so custom
// definitions override built-in entries that share a slot.
func FindByByte(entries []Entry, value byte) (Entry, bool) {
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Byte == value {
			return entries[i], true
		}
	}
	return Entry{}, false
}

func normalizeAliases(name string, aliases []string) ([]string, error) {
	out := make([]string, 0, len(aliases))
	seen := map[string]struct{}{"p8_" + name: {}}
	for _, raw := range aliases {
		alias := asciiLower(strings.TrimSpace(raw))
		if !validAlias(alias) {
			return nil, fmt.Errorf("entry %q alias %q must start with p8_ or p8- and contain only ASCII letters, digits, _ or - (p8_ aliases cannot contain -)", name, raw)
		}
		if _, ok := seen[alias]; ok {
			continue
		}
		seen[alias] = struct{}{}
		out = append(out, alias)
	}
	return out, nil
}

func validName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}

func validAlias(alias string) bool {
	if strings.HasPrefix(alias, "p8_") {
		if strings.Contains(alias[len("p8_"):], "-") {
			return false
		}
		alias = alias[len("p8_"):]
	} else if strings.HasPrefix(alias, "p8-") {
		alias = alias[len("p8-"):]
	} else {
		return false
	}
	if alias == "" || len(alias) > 64 {
		return false
	}
	for i := 0; i < len(alias); i++ {
		c := alias[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			continue
		}
		return false
	}
	return true
}

func parseControlEscape(text string) (byte, bool) {
	switch text {
	case `\0`:
		return 0, true
	case `\*`:
		return 1, true
	case `\#`:
		return 2, true
	case `\-`:
		return 3, true
	case `\|`:
		return 4, true
	case `\+`:
		return 5, true
	case `\^`:
		return 6, true
	case `\a`:
		return 7, true
	case `\b`:
		return 8, true
	case `\t`:
		return 9, true
	case `\n`:
		return 10, true
	case `\v`:
		return 11, true
	case `\f`:
		return 12, true
	case `\r`:
		return 13, true
	}
	if len(text) < 2 || len(text) > 4 || text[0] != '\\' {
		return 0, false
	}
	for i := 1; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return 0, false
		}
	}
	value, err := strconv.Atoi(text[1:])
	if err != nil || value < 0 || value > 255 {
		return 0, false
	}
	return byte(value), true
}

func description(value, format string, args ...any) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fmt.Sprintf(format, args...)
}

func asciiLower(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		builder.WriteByte(c)
	}
	return builder.String()
}

func cloneEntries(entries []Entry) []Entry {
	out := make([]Entry, len(entries))
	copy(out, entries)
	for i := range out {
		out[i].Aliases = append([]string(nil), out[i].Aliases...)
	}
	return out
}
