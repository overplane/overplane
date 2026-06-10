package telemetry_test

import (
	"context"
	"testing"

	"github.com/overplane/overplane/internal/platform/telemetry"
)

func TestNoopTelemetry(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	p, err := telemetry.Init(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p.Enabled() {
		t.Fatal("noop telemetry should be disabled")
	}
	ctx, span := p.StartSpan(context.Background(), "unit")
	span.End()
	if ctx == nil || telemetry.Global() == nil {
		t.Fatal("telemetry not initialized")
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}
