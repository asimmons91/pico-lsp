package syntax

import "testing"

func TestIsIdentifier(t *testing.T) {
	valid := []string{"x", "_x", "Player", "player_2", "a1", "é", "⬅️", "🅾️"}
	for _, name := range valid {
		if !IsIdentifier(name) {
			t.Errorf("IsIdentifier(%q) = false, want true", name)
		}
	}
	invalid := []string{"", "1x", "end", "END", "with-dash", "two words", "\xff"}
	for _, name := range invalid {
		if IsIdentifier(name) {
			t.Errorf("IsIdentifier(%q) = true, want false", name)
		}
	}
}
