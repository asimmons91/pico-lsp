package lsp

import (
	"context"
	"errors"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

type fakeDiagnosticClient struct {
	protocol.UnimplementedClient

	published []*protocol.PublishDiagnosticsParams
	err       error
}

func (f *fakeDiagnosticClient) PublishDiagnostics(ctx context.Context, params *protocol.PublishDiagnosticsParams) error {
	f.published = append(f.published, params)
	return f.err
}

func diagnostic(message string) protocol.Diagnostic {
	return protocol.Diagnostic{
		Range:    protocol.Range{},
		Severity: protocol.DiagnosticSeverityWarning,
		Code:     protocol.String("test"),
		Message:  protocol.String(message),
	}
}

func int32Ptr(v int32) *int32 { return &v }

func TestDiagnosticsPublishAndDeduplicate(t *testing.T) {
	client := &fakeDiagnosticClient{}
	maxProblems := 100
	d := NewDiagnostics(func() protocol.Client { return client }, func() int { return maxProblems }, quietLogger())
	u := uri.File("/tmp/cart.p8")
	ctx := context.Background()

	d.Publish(ctx, u, int32Ptr(1), []protocol.Diagnostic{diagnostic("first")})
	if len(client.published) != 0 {
		t.Fatal("published before Enable")
	}

	d.Enable()
	d.Publish(ctx, u, int32Ptr(1), []protocol.Diagnostic{diagnostic("first")})
	d.Publish(ctx, u, int32Ptr(1), []protocol.Diagnostic{diagnostic("first")})
	if len(client.published) != 1 {
		t.Fatalf("published %d times, want 1", len(client.published))
	}
	if got := client.published[0].Version; !got.IsZero() {
		version, _ := got.Get()
		if version != 1 {
			t.Fatalf("version = %d, want 1", version)
		}
	} else {
		t.Fatal("version was not published")
	}

	d.Publish(ctx, u, int32Ptr(2), []protocol.Diagnostic{diagnostic("first")})
	if len(client.published) != 2 {
		t.Fatalf("published %d times, want 2", len(client.published))
	}

	d.Publish(ctx, u, int32Ptr(2), []protocol.Diagnostic{diagnostic("second")})
	if len(client.published) != 3 {
		t.Fatalf("published %d times, want 3", len(client.published))
	}
}

func TestDiagnosticsClear(t *testing.T) {
	client := &fakeDiagnosticClient{}
	d := NewDiagnostics(func() protocol.Client { return client }, func() int { return 100 }, quietLogger())
	d.Enable()
	u := uri.File("/tmp/cart.p8")
	ctx := context.Background()

	d.Publish(ctx, u, int32Ptr(3), []protocol.Diagnostic{diagnostic("problem")})
	d.Clear(ctx, u)
	d.Clear(ctx, u)

	if len(client.published) != 2 {
		t.Fatalf("published %d times, want 2", len(client.published))
	}
	last := client.published[1]
	if len(last.Diagnostics) != 0 {
		t.Fatalf("clear published %d diagnostics, want 0", len(last.Diagnostics))
	}
	if !last.Version.IsZero() {
		t.Fatal("clear published a version, want none")
	}
}

func TestDiagnosticsTruncates(t *testing.T) {
	client := &fakeDiagnosticClient{}
	d := NewDiagnostics(func() protocol.Client { return client }, func() int { return 2 }, quietLogger())
	d.Enable()

	d.Publish(context.Background(), uri.File("/tmp/cart.p8"), nil, []protocol.Diagnostic{
		diagnostic("one"),
		diagnostic("two"),
		diagnostic("three"),
	})

	if len(client.published) != 1 {
		t.Fatalf("published %d times, want 1", len(client.published))
	}
	if got := len(client.published[0].Diagnostics); got != 2 {
		t.Fatalf("published %d diagnostics, want 2", got)
	}
}

func TestDiagnosticsNoLimit(t *testing.T) {
	client := &fakeDiagnosticClient{}
	d := NewDiagnostics(func() protocol.Client { return client }, func() int { return 0 }, quietLogger())
	d.Enable()

	items := make([]protocol.Diagnostic, 5)
	for i := range items {
		items[i] = diagnostic("problem")
	}
	d.Publish(context.Background(), uri.File("/tmp/cart.p8"), nil, items)

	if got := len(client.published[0].Diagnostics); got != 5 {
		t.Fatalf("published %d diagnostics, want 5", got)
	}
}

func TestDiagnosticsCopiesInput(t *testing.T) {
	client := &fakeDiagnosticClient{}
	d := NewDiagnostics(func() protocol.Client { return client }, func() int { return 100 }, quietLogger())
	d.Enable()

	items := []protocol.Diagnostic{diagnostic("original")}
	d.Publish(context.Background(), uri.File("/tmp/cart.p8"), nil, items)
	items[0] = diagnostic("mutated")

	if got := client.published[0].Diagnostics[0].Message; got != protocol.String("original") {
		t.Fatalf("published message = %v, want original", got)
	}
}

func TestDiagnosticsClientErrorDoesNotRecord(t *testing.T) {
	client := &fakeDiagnosticClient{err: errors.New("write failed")}
	d := NewDiagnostics(func() protocol.Client { return client }, func() int { return 100 }, quietLogger())
	d.Enable()
	u := uri.File("/tmp/cart.p8")
	ctx := context.Background()

	d.Publish(ctx, u, nil, []protocol.Diagnostic{diagnostic("first")})
	client.err = nil
	d.Publish(ctx, u, nil, []protocol.Diagnostic{diagnostic("first")})

	if len(client.published) != 2 {
		t.Fatalf("published %d times, want 2 after a failed publish", len(client.published))
	}
}

func TestDiagnosticsNilClient(t *testing.T) {
	d := NewDiagnostics(func() protocol.Client { return nil }, func() int { return 100 }, quietLogger())
	d.Enable()
	d.Publish(context.Background(), uri.File("/tmp/cart.p8"), nil, []protocol.Diagnostic{diagnostic("x")})
}
