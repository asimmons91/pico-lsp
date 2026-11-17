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
	defer func() { _ = file.Close() }()

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
	forbidden := []string{"require", "string", "table", "math", "io", "os", "coroutine", "package", "utf8", "debug", "tostring", "tonumber"}
	for _, name := range forbidden {
		if _, ok := api.Lookup(name); ok {
			t.Errorf("stub defines standard-library name %q", name)
		}
	}
}

func TestStubMetadata(t *testing.T) {
	api, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	circ, ok := api.Lookup("circfill")
	if !ok {
		t.Fatal("missing circfill")
	}
	if !strings.Contains(circ.Signature, "[col]") {
		t.Fatalf("circfill signature = %q, want optional col", circ.Signature)
	}
	if circ.Docs == nil || strings.TrimSpace(circ.Docs.Summary) == "" {
		t.Fatalf("circfill docs = %+v, want summary", circ.Docs)
	}
	if len(circ.Params) != 4 || len(circ.Docs.Params) != 4 {
		t.Fatalf("circfill params = %v, docs = %+v, want four", circ.Params, circ.Docs.Params)
	}
	if !circ.Docs.Params[3].Optional {
		t.Fatalf("last circfill param = %+v, want optional", circ.Docs.Params[3])
	}
	if circ.Docs.Params[0].Type != "number" || circ.Docs.Params[3].Type != "number" {
		t.Fatalf("circfill param types = %+v, want numbers", circ.Docs.Params)
	}

	load, ok := api.Lookup("load")
	if !ok || load.Docs == nil || !strings.Contains(load.Signature, "[breadcrumb]") {
		t.Fatalf("load = %+v, want optional breadcrumb", load)
	}
	callback, ok := api.Lookup("_update")
	if !ok || callback.Docs == nil || strings.TrimSpace(callback.Docs.Summary) == "" {
		t.Fatalf("_update = %+v, want docs", callback)
	}
	costatus, ok := api.Lookup("costatus")
	if !ok || costatus.Docs == nil || len(costatus.Docs.Params) != 1 || costatus.Docs.Params[0].Type != "thread" {
		t.Fatalf("costatus param docs = %+v, want thread", costatus)
	}
}

func TestStubMetadataComplete(t *testing.T) {
	api, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, name := range api.Names {
		switch name.Kind {
		case KindFunction:
			if !strings.HasPrefix(name.Signature, "function "+name.Name+"(") {
				t.Fatalf("%s signature = %q", name.Name, name.Signature)
			}
			if name.Docs == nil || strings.TrimSpace(name.Docs.Summary) == "" {
				t.Fatalf("%s has no summary", name.Name)
			}
			if len(name.Params) != len(name.Docs.Params) {
				t.Fatalf("%s params = %v, docs = %+v", name.Name, name.Params, name.Docs.Params)
			}
			for i, param := range name.Docs.Params {
				if name.Params[i] != param.Name && param.Name != "..." {
					t.Fatalf("%s param %d = %q, docs = %q", name.Name, i, name.Params[i], param.Name)
				}
			}
		case KindConstant:
			if name.Docs == nil || strings.TrimSpace(name.Docs.Summary) == "" {
				t.Fatalf("%s has no summary", name.Name)
			}
			if name.Docs.Type != "number" {
				t.Fatalf("%s type = %q, want number", name.Name, name.Docs.Type)
			}
			if !strings.Contains(name.Signature, " = ") {
				t.Fatalf("%s signature = %q, want assignment", name.Name, name.Signature)
			}
		default:
			t.Fatalf("%s has unknown kind %v", name.Name, name.Kind)
		}
	}
}

func TestStubHasNoLuaStandardLibraryModules(t *testing.T) {
	api, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, name := range []string{
		"require", "loadfile", "dofile", "io", "os", "string", "table", "math",
		"coroutine", "debug", "utf8", "package",
	} {
		if _, ok := api.Lookup(name); ok {
			t.Fatalf("built-in API contains Lua standard library name %q", name)
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

func TestStubMetadataIncludesVariadicParameters(t *testing.T) {
	api, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, name := range []string{"poke", "chr", "coresume"} {
		entry, ok := api.Lookup(name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if !strings.HasSuffix(entry.Signature, "...)") {
			t.Fatalf("%s signature = %q, want variadic", name, entry.Signature)
		}
		if len(entry.Params) == 0 || entry.Params[len(entry.Params)-1] != "..." {
			t.Fatalf("%s params = %v, want trailing ...", name, entry.Params)
		}
	}
}
