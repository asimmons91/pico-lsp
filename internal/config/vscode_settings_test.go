package config

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const vscodeManifestPath = "../../editors/vscode/package.json"

type vscodeManifest struct {
	Contributes struct {
		Configuration json.RawMessage `json:"configuration"`
	} `json:"contributes"`
}

type vscodeConfiguration struct {
	Properties map[string]vscodeProperty `json:"properties"`
}

type vscodeProperty struct {
	Type                 string                    `json:"type"`
	Enum                 []json.RawMessage         `json:"enum"`
	Default              json.RawMessage           `json:"default"`
	Minimum              *float64                  `json:"minimum"`
	Scope                string                    `json:"scope"`
	Description          string                    `json:"description"`
	MarkdownDescription  string                    `json:"markdownDescription"`
	Properties           map[string]vscodeProperty `json:"properties"`
	AdditionalProperties *bool                     `json:"additionalProperties"`
}

// TestVSCodeSettingsMatchConfig keeps the VS Code settings schema in sync with
// the server's authoritative defaults and enums in internal/config.
func TestVSCodeSettingsMatchConfig(t *testing.T) {
	manifest := loadVSCodeProperties(t)

	leaves := make(map[string]vscodeProperty)
	for name, property := range manifest {
		flattenVSCodeProperties(t, name, property, leaves)
	}

	serverDefaults := make(map[string]any)
	serverObjects := make(map[string]any)
	collectServerDefaults(t, Section, reflect.ValueOf(Default()), serverDefaults, serverObjects)

	extensionOnly := map[string]struct{}{
		"pico8.serverPath":   {},
		"pico8.autoDownload": {},
		"pico8.attachLua":    {},
		"pico8.trace.server": {},
	}

	for name, want := range serverDefaults {
		property, ok := leaves[name]
		if !ok {
			t.Errorf("%s is a server setting in internal/config but is missing from editors/vscode/package.json; add it to the VS Code schema and internal/config together", name)
			continue
		}
		if property.Description == "" && property.MarkdownDescription == "" {
			t.Errorf("%s has no description in editors/vscode/package.json", name)
		}
		if strings.Count(name, ".") == 1 && property.Scope != "window" {
			t.Errorf("%s has scope %q, want window", name, property.Scope)
		}
		if got, wantType := property.Type, schemaType(want); got != wantType {
			t.Errorf("%s has type %q, want %q", name, got, wantType)
		}
		assertDefault(t, name, property, want)
	}

	for name, want := range serverObjects {
		property, ok := manifest[name]
		if !ok || len(property.Properties) == 0 {
			continue
		}
		assertDefault(t, name, property, want)
	}

	for name := range leaves {
		if _, ok := serverDefaults[name]; ok {
			continue
		}
		if _, ok := extensionOnly[name]; ok {
			continue
		}
		t.Errorf("%s is declared in editors/vscode/package.json but is neither a server setting in internal/config nor a known extension-only setting; update internal/config or the extension-only allowlist", name)
	}

	for _, name := range []string{
		"pico8.limits",
		"pico8.format",
		"pico8.diagnostics",
		"pico8.lint",
		"pico8.refactors",
	} {
		property, ok := manifest[name]
		if !ok {
			continue
		}
		if property.Scope != "window" {
			t.Errorf("%s has scope %q, want window", name, property.Scope)
		}
	}

	serverPath, ok := leaves["pico8.serverPath"]
	if !ok {
		t.Error("pico8.serverPath is missing from editors/vscode/package.json")
	} else if serverPath.Scope != "machine" {
		t.Errorf("pico8.serverPath has scope %q, want machine", serverPath.Scope)
	}

	for name := range extensionOnly {
		if name == "pico8.serverPath" {
			continue
		}
		property, ok := leaves[name]
		if ok && property.Scope != "window" {
			t.Errorf("%s has scope %q, want window", name, property.Scope)
		}
	}

	checkEnums(t, leaves)

	for _, name := range []string{"pico8.maxProblems", "pico8.format.maxBlankLines"} {
		property, ok := leaves[name]
		if !ok {
			continue
		}
		if property.Minimum == nil || *property.Minimum != 0 {
			t.Errorf("%s must declare minimum: 0", name)
		}
	}
}

func loadVSCodeProperties(t *testing.T) map[string]vscodeProperty {
	t.Helper()

	raw, err := os.ReadFile(vscodeManifestPath)
	if err != nil {
		t.Fatalf("read VS Code manifest %s: %v", vscodeManifestPath, err)
	}
	var manifest vscodeManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("parse VS Code manifest %s: %v", vscodeManifestPath, err)
	}

	configuration := bytes.TrimSpace(manifest.Contributes.Configuration)
	if len(configuration) == 0 || bytes.Equal(configuration, []byte("null")) {
		t.Fatalf("%s has no contributes.configuration", vscodeManifestPath)
	}

	var blocks []vscodeConfiguration
	if configuration[0] == '[' {
		if err := json.Unmarshal(configuration, &blocks); err != nil {
			t.Fatalf("parse contributes.configuration as an array: %v", err)
		}
	} else {
		var block vscodeConfiguration
		if err := json.Unmarshal(configuration, &block); err != nil {
			t.Fatalf("parse contributes.configuration as an object: %v", err)
		}
		blocks = append(blocks, block)
	}

	properties := make(map[string]vscodeProperty)
	for _, block := range blocks {
		for name, property := range block.Properties {
			if _, exists := properties[name]; exists {
				t.Errorf("%s is declared more than once in contributes.configuration", name)
			}
			properties[name] = property
		}
	}
	if len(properties) == 0 {
		t.Fatalf("%s declares no configuration properties", vscodeManifestPath)
	}
	return properties
}

func flattenVSCodeProperties(t *testing.T, name string, property vscodeProperty, out map[string]vscodeProperty) {
	t.Helper()

	if len(property.Properties) > 0 {
		if property.Type != "object" {
			t.Errorf("%s: nested settings must have type object, got %q", name, property.Type)
		}
		if property.AdditionalProperties == nil || *property.AdditionalProperties {
			t.Errorf("%s: nested settings must set additionalProperties: false", name)
		}
		for childName, child := range property.Properties {
			flattenVSCodeProperties(t, name+"."+childName, child, out)
		}
		return
	}
	out[name] = property
}

func collectServerDefaults(
	t *testing.T,
	prefix string,
	value reflect.Value,
	leaves map[string]any,
	objects map[string]any,
) {
	t.Helper()

	if value.Kind() == reflect.Struct {
		objects[prefix] = value.Interface()
		typ := value.Type()
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			jsonName, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if jsonName == "" || jsonName == "-" {
				continue
			}
			collectServerDefaults(t, prefix+"."+jsonName, value.Field(i), leaves, objects)
		}
		return
	}
	leaves[prefix] = value.Interface()
}

func assertDefault(t *testing.T, name string, property vscodeProperty, want any) {
	t.Helper()

	if len(property.Default) == 0 {
		t.Errorf("%s has no default in editors/vscode/package.json; add the default from internal/config.Default()", name)
		return
	}
	wantJSON, err := defaultJSON(want)
	if err != nil {
		t.Fatalf("%s: encode expected default: %v", name, err)
	}
	gotJSON, err := compactJSON(property.Default)
	if err != nil {
		t.Errorf("%s: invalid default in editors/vscode/package.json: %v", name, err)
		return
	}

	var got, wantValue any
	if err := json.Unmarshal(gotJSON, &got); err != nil {
		t.Errorf("%s: decode default in editors/vscode/package.json: %v", name, err)
		return
	}
	if err := json.Unmarshal(wantJSON, &wantValue); err != nil {
		t.Fatalf("%s: decode expected default: %v", name, err)
	}
	if !reflect.DeepEqual(got, wantValue) {
		t.Errorf("%s default = %s, want %s (from internal/config.Default()); update internal/config and editors/vscode/package.json together", name, gotJSON, wantJSON)
	}
}

func defaultJSON(value any) ([]byte, error) {
	rv := reflect.ValueOf(value)
	if rv.IsValid() && rv.Kind() == reflect.Slice && rv.IsNil() {
		return []byte("[]"), nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return compactJSON(raw)
}

func compactJSON(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func schemaType(value any) string {
	switch reflect.TypeOf(value).Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.String:
		return "string"
	case reflect.Slice, reflect.Array:
		return "array"
	default:
		return reflect.TypeOf(value).Kind().String()
	}
}

func checkEnums(t *testing.T, leaves map[string]vscodeProperty) {
	t.Helper()

	severities := []string{SeverityOff, SeverityHint, SeverityInfo, SeverityWarning, SeverityError}
	expected := map[string][]string{
		"pico8.apiVersion":       {DefaultAPIVersion},
		"pico8.typeChecking":     {TypeCheckingOff, TypeCheckingBasic, TypeCheckingStrict},
		"pico8.diagnostics.mode": {DiagnosticsModeAuto, DiagnosticsModePush, DiagnosticsModePull},
	}
	for name := range leaves {
		if strings.HasPrefix(name, "pico8.diagnostics.") && name != "pico8.diagnostics.mode" {
			expected[name] = severities
		}
	}

	for name, want := range expected {
		property, ok := leaves[name]
		if !ok {
			continue
		}
		got := make([]string, 0, len(property.Enum))
		for _, raw := range property.Enum {
			var value string
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Errorf("%s has a non-string enum value: %s", name, raw)
				continue
			}
			got = append(got, value)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s enum = %v, want %v; keep the VS Code schema and internal/config constants in sync", name, got, want)
		}
	}
}
