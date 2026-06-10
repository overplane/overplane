package telemetrytest_test

import (
	"context"
	"testing"

	"github.com/overplane/overplane/internal/platform/telemetry/telemetrytest"
)

func TestCollectorLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c, err := telemetrytest.StartGRPC(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr() == "" {
		t.Fatal("missing addr")
	}
	cancel()
	_ = c.Close()
}
