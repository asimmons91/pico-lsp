// Package p8scii maps between PICO-8's 256-character P8SCII encoding and
// Unicode strings. The mapping follows PICO-8 0.2.7's charset and matches
// shrinko8's k_charset.
package p8scii

import "unicode/utf8"

// charset maps each P8SCII byte to its Unicode glyph.
var charset = [256]string{
	"\x00", "\u00b9", "\u00b2", "\u00b3", "\u2074", "\u2075", "\u2076", "\u2077",
	"\u2078", "\t", "\n", "\u1d47", "\u1d9c", "\r", "\u1d49", "\u1da0",
	"\u25ae", "\u25a0", "\u25a1", "\u2059", "\u2058", "\u2016", "\u25c0", "\u25b6",
	"\u300c", "\u300d", "\u00a5", "\u2022", "\u3001", "\u3002", "\u309b", "\u309c",
	" ", "!", "\"", "#", "$", "%", "&", "'",
	"(", ")", "*", "+", ",", "-", ".", "/",
	"0", "1", "2", "3", "4", "5", "6", "7",
	"8", "9", ":", ";", "<", "=", ">", "?",
	"@", "A", "B", "C", "D", "E", "F", "G",
	"H", "I", "J", "K", "L", "M", "N", "O",
	"P", "Q", "R", "S", "T", "U", "V", "W",
	"X", "Y", "Z", "[", "\\", "]", "^", "_",
	"`", "a", "b", "c", "d", "e", "f", "g",
	"h", "i", "j", "k", "l", "m", "n", "o",
	"p", "q", "r", "s", "t", "u", "v", "w",
	"x", "y", "z", "{", "|", "}", "~", "\u25cb",
	"\u2588", "\u2592", "\U0001f431", "\u2b07\ufe0f", "\u2591", "\u273d", "\u25cf", "\u2665",
	"\u2609", "\uc6c3", "\u2302", "\u2b05\ufe0f", "\U0001f610", "\u266a", "\U0001f17e\ufe0f", "\u25c6",
	"\u2026", "\u27a1\ufe0f", "\u2605", "\u29d7", "\u2b06\ufe0f", "\u02c7", "\u2227", "\u274e",
	"\u25a4", "\u25a5", "\u3042", "\u3044", "\u3046", "\u3048", "\u304a", "\u304b",
	"\u304d", "\u304f", "\u3051", "\u3053", "\u3055", "\u3057", "\u3059", "\u305b",
	"\u305d", "\u305f", "\u3061", "\u3064", "\u3066", "\u3068", "\u306a", "\u306b",
	"\u306c", "\u306d", "\u306e", "\u306f", "\u3072", "\u3075", "\u3078", "\u307b",
	"\u307e", "\u307f", "\u3080", "\u3081", "\u3082", "\u3084", "\u3086", "\u3088",
	"\u3089", "\u308a", "\u308b", "\u308c", "\u308d", "\u308f", "\u3092", "\u3093",
	"\u3063", "\u3083", "\u3085", "\u3087", "\u30a2", "\u30a4", "\u30a6", "\u30a8",
	"\u30aa", "\u30ab", "\u30ad", "\u30af", "\u30b1", "\u30b3", "\u30b5", "\u30b7",
	"\u30b9", "\u30bb", "\u30bd", "\u30bf", "\u30c1", "\u30c4", "\u30c6", "\u30c8",
	"\u30ca", "\u30cb", "\u30cc", "\u30cd", "\u30ce", "\u30cf", "\u30d2", "\u30d5",
	"\u30d8", "\u30db", "\u30de", "\u30df", "\u30e0", "\u30e1", "\u30e2", "\u30e4",
	"\u30e6", "\u30e8", "\u30e9", "\u30ea", "\u30eb", "\u30ec", "\u30ed", "\u30ef",
	"\u30f2", "\u30f3", "\u30c3", "\u30e3", "\u30e5", "\u30e7", "\u25dc", "\u25dd",
}

// unicap maps the Unicode mathematical italic letters PICO-8 uses for
// uppercase glyphs when copying text, back to their ASCII P8SCII indexes.
var unicap = [26]string{
	"\U0001d622", "\U0001d623", "\U0001d624", "\U0001d625", "\U0001d626",
	"\U0001d627", "\U0001d628", "\U0001d629", "\U0001d62a", "\U0001d62b",
	"\U0001d62c", "\U0001d62d", "\U0001d62e", "\U0001d62f", "\U0001d630",
	"\U0001d631", "\U0001d632", "\U0001d633", "\U0001d634", "\U0001d635",
	"\U0001d636", "\U0001d637", "\U0001d638", "\U0001d639", "\U0001d63a",
	"\U0001d63b",
}

var encodeMap = func() map[string]byte {
	m := make(map[string]byte, len(charset)+len(unicap))
	for i, glyph := range charset {
		m[glyph] = byte(i)
	}
	for i, glyph := range unicap {
		m[glyph] = byte('A' + i)
	}
	return m
}()

// Decode converts P8SCII bytes to Unicode text.
func Decode(code []byte) string {
	out := make([]byte, 0, len(code))
	for _, c := range code {
		out = append(out, charset[c]...)
	}
	return string(out)
}

// Encode converts Unicode text to the P8SCII bytes PICO-8 would store.
// Unknown characters are reinterpreted as their UTF-8 bytes, matching
// PICO-8/shrinko8 behavior.
func Encode(text string) []byte {
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError && size == 1 {
			out = append(out, text[i])
			i++
			continue
		}
		if r < utf8.RuneSelf {
			out = append(out, byte(r))
			i += size
			continue
		}
		if b, ok := encodeMap[text[i:i+size]]; ok {
			out = append(out, b)
			i += size
			continue
		}
		if i+size < len(text) {
			_, size2 := utf8.DecodeRuneInString(text[i+size:])
			if b, ok := encodeMap[text[i:i+size+size2]]; ok {
				out = append(out, b)
				i += size + size2
				continue
			}
		}
		out = append(out, text[i:i+size]...)
		i += size
	}
	return out
}
