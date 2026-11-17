package config

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/go-json-experiment/json/jsontext"
	"go.lsp.dev/protocol"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeClient struct {
	protocol.UnimplementedClient

	values []protocol.LSPAny
	err    error
	params *protocol.ConfigurationParams
}

func (f *fakeClient) Configuration(ctx context.Context, params *protocol.ConfigurationParams) ([]protocol.LSPAny, error) {
	f.params = params
	return f.values, f.err
}

func lspValue(s string) protocol.LSPAny {
	return jsontext.Value(s)
}

func TestDefaults(t *testing.T) {
	got := New(testLogger()).Settings()
	want := Default()
	if got != want {
		t.Fatalf("settings = %+v, want %+v", got, want)
	}
}

func TestApplyInitializationOptions(t *testing.T) {
	m := New(testLogger())
	m.ApplyInitializationOptions(lspValue(`{
		"pico8": {
			"typeChecking": "strict",
			"maxProblems": 25,
			"unknownKey": true
		},
		"other": {"ignored": true}
	}`))

	got := m.Settings()
	if got.TypeChecking != TypeCheckingStrict {
		t.Fatalf("typeChecking = %q, want strict", got.TypeChecking)
	}
	if got.MaxProblems != 25 {
		t.Fatalf("maxProblems = %d, want 25", got.MaxProblems)
	}
	if got.APIVersion != DefaultAPIVersion {
		t.Fatalf("apiVersion = %q, want %q", got.APIVersion, DefaultAPIVersion)
	}
	if got.Diagnostics.UndefinedGlobal != DefaultUndefinedGlobal {
		t.Fatalf("undefinedGlobal = %q, want %q", got.Diagnostics.UndefinedGlobal, DefaultUndefinedGlobal)
	}
}

func TestInvalidValuesFallBackToDefaults(t *testing.T) {
	m := New(testLogger())
	m.ApplyInitializationOptions(lspValue(`{
		"pico8": {
			"apiVersion": "9.9.9",
			"typeChecking": "turbo",
			"maxProblems": -1,
			"diagnostics": {
				"undefinedGlobal": "loud",
				"unusedLocal": "hint"
			}
		}
	}`))

	got := m.Settings()
	if got.APIVersion != DefaultAPIVersion {
		t.Fatalf("apiVersion = %q, want default", got.APIVersion)
	}
	if got.TypeChecking != DefaultTypeChecking {
		t.Fatalf("typeChecking = %q, want default", got.TypeChecking)
	}
	if got.MaxProblems != DefaultMaxProblems {
		t.Fatalf("maxProblems = %d, want default", got.MaxProblems)
	}
	if got.Diagnostics.UndefinedGlobal != DefaultUndefinedGlobal {
		t.Fatalf("undefinedGlobal = %q, want default", got.Diagnostics.UndefinedGlobal)
	}
	if got.Diagnostics.UnusedLocal != SeverityHint {
		t.Fatalf("unusedLocal = %q, want hint", got.Diagnostics.UnusedLocal)
	}
}

func TestZeroMaxProblemsDisablesTruncation(t *testing.T) {
	m := New(testLogger())
	m.ApplyInitializationOptions(lspValue(`{"pico8":{"maxProblems":0}}`))

	if got := m.Settings().MaxProblems; got != 0 {
		t.Fatalf("maxProblems = %d, want 0 (unlimited)", got)
	}
}

func TestMalformedInitializationOptionsAreIgnored(t *testing.T) {
	m := New(testLogger())
	m.ApplyInitializationOptions(lspValue(`"not an object"`))
	m.ApplyInitializationOptions(lspValue(`{"pico8": 12}`))
	m.ApplyInitializationOptions(lspValue(``))

	if got := m.Settings(); got != Default() {
		t.Fatalf("settings changed after malformed options: %+v", got)
	}
}

func TestPullOverridesInitializationOptions(t *testing.T) {
	m := New(testLogger())
	m.ApplyInitializationOptions(lspValue(`{"pico8":{"typeChecking":"strict"}}`))

	client := &fakeClient{values: []protocol.LSPAny{lspValue(`{"typeChecking":"off","maxProblems":7}`)}}
	m.SetPullSupported(true)
	if err := m.Pull(context.Background(), client); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	got := m.Settings()
	if got.TypeChecking != TypeCheckingOff {
		t.Fatalf("typeChecking = %q, want off", got.TypeChecking)
	}
	if got.MaxProblems != 7 {
		t.Fatalf("maxProblems = %d, want 7", got.MaxProblems)
	}
	if client.params == nil || len(client.params.Items) != 1 {
		t.Fatalf("configuration params = %+v, want one item", client.params)
	}
	section := client.params.Items[0].Section
	if section == nil || *section != Section {
		t.Fatalf("section = %v, want %q", section, Section)
	}
}

func TestPullEmptyValueFallsBackToInitializationOptions(t *testing.T) {
	m := New(testLogger())
	m.ApplyInitializationOptions(lspValue(`{"pico8":{"typeChecking":"strict","maxProblems":25}}`))
	m.SetPullSupported(true)

	for name, values := range map[string][]protocol.LSPAny{
		"empty": nil,
		"null":  {lspValue(`null`)},
		"blank": {lspValue(`{}`)},
	} {
		t.Run(name, func(t *testing.T) {
			m.ApplyInitializationOptions(lspValue(`{"pico8":{"typeChecking":"strict","maxProblems":25}}`))
			m.store(Default())
			if err := m.Pull(context.Background(), &fakeClient{values: values}); err != nil {
				t.Fatalf("Pull: %v", err)
			}
			got := m.Settings()
			if got.TypeChecking != TypeCheckingStrict {
				t.Fatalf("typeChecking = %q, want strict from initializationOptions", got.TypeChecking)
			}
			if got.MaxProblems != 25 {
				t.Fatalf("maxProblems = %d, want 25 from initializationOptions", got.MaxProblems)
			}
		})
	}
}

func TestPullPreservesInitializationOptionsForMissingFields(t *testing.T) {
	m := New(testLogger())
	m.ApplyInitializationOptions(lspValue(`{"pico8":{"maxProblems":25,"diagnostics":{"unusedLocal":"hint"}}}`))
	m.SetPullSupported(true)

	if err := m.Pull(context.Background(), &fakeClient{
		values: []protocol.LSPAny{lspValue(`{"typeChecking":"strict"}`)},
	}); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	got := m.Settings()
	if got.TypeChecking != TypeCheckingStrict {
		t.Fatalf("typeChecking = %q, want strict from pull", got.TypeChecking)
	}
	if got.MaxProblems != 25 {
		t.Fatalf("maxProblems = %d, want 25 from initializationOptions", got.MaxProblems)
	}
	if got.Diagnostics.UnusedLocal != SeverityHint {
		t.Fatalf("unusedLocal = %q, want hint from initializationOptions", got.Diagnostics.UnusedLocal)
	}
}

func TestPullErrorKeepsPreviousSettings(t *testing.T) {
	m := New(testLogger())
	m.ApplyInitializationOptions(lspValue(`{"pico8":{"typeChecking":"strict"}}`))
	m.SetPullSupported(true)

	err := m.Pull(context.Background(), &fakeClient{err: errors.New("boom")})
	if err == nil {
		t.Fatal("Pull succeeded, want error")
	}
	if got := m.Settings().TypeChecking; got != TypeCheckingStrict {
		t.Fatalf("typeChecking = %q, want strict", got)
	}
}

func TestPullWithoutSupportIsNoOp(t *testing.T) {
	m := New(testLogger())
	client := &fakeClient{values: []protocol.LSPAny{lspValue(`{"typeChecking":"strict"}`)}}

	if err := m.Pull(context.Background(), client); err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if client.params != nil {
		t.Fatal("Pull called workspace/configuration without client support")
	}
	if got := m.Settings().TypeChecking; got != DefaultTypeChecking {
		t.Fatalf("typeChecking = %q, want default", got)
	}
}

func TestApplyDidChange(t *testing.T) {
	m := New(testLogger())
	m.ApplyDidChange(lspValue(`{"pico8":{"diagnostics":{"unusedLocal":"warning"}}}`))

	got := m.Settings()
	if got.Diagnostics.UnusedLocal != SeverityWarning {
		t.Fatalf("unusedLocal = %q, want warning", got.Diagnostics.UnusedLocal)
	}
	if got.TypeChecking != DefaultTypeChecking {
		t.Fatalf("typeChecking = %q, want default", got.TypeChecking)
	}
}
