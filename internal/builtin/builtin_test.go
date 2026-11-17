package builtin

import (
	"bufio"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestStubMatchesManifest(t *testing.T) {
	api, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	path := filepath.Join("..", "..", "testdata", "builtin", "pico8-0.2.7.txt")
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open manifest: %v", err)
	}
	defer file.Close()

	var manifest []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name != "" && !strings.HasPrefix(name, "#") {
			manifest = append(manifest, name)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	slices.Sort(manifest)

	got := api.SortedNames()
	if !slices.Equal(got, manifest) {
		t.Fatalf("stub names differ from manifest\nstub:     %v\nmanifest: %v", got, manifest)
	}
}

func TestStubHasNoStandardLibrary(t *testing.T) {
	api, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	forbidden := []string{"require", "string", "table", "math", "io", "os", "coroutine", "package", "utf8", "debug"}
	for _, name := range forbidden {
		if _, ok := api.Lookup(name); ok {
			t.Errorf("stub defines standard-library name %q", name)
		}
	}
}

func TestStubCoversKeyAPISurface(t *testing.T) {
	api, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, name := range []string{
		"_init", "_update", "_update60", "_draw",
		"btn_left", "btn_right", "btn_up", "btn_down", "btn_o", "btn_x",
		"⬅️", "➡️", "⬆️", "⬇️", "🅾️", "❎",
		"btn", "btnp", "spr", "sspr", "circfill", "rrect", "rrectfill",
		"sfx", "music", "map", "mget", "mset", "peek", "poke",
		"add", "all", "foreach", "split", "sub", "tostr", "tonum",
	} {
		if _, ok := api.Lookup(name); !ok {
			t.Errorf("stub is missing %q", name)
		}
	}
}
