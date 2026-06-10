package telemetry

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/overplane/overplane/internal/platform/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Providers struct {
	Tracer trace.Tracer
	Meter  metric.Meter

	enabled  bool
	shutdown func(context.Context) error
}

var (
	globalMu sync.RWMutex
	global   = noop()
)

func Init(ctx context.Context) (*Providers, error) {
	endpoint := strings.TrimPrefix(strings.TrimPrefix(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"), "http://"), "https://")
	if endpoint == "" {
		p := noop()
		setGlobal(p)
		return p, nil
	}
	hostname, _ := os.Hostname()
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", "overplane"),
			attribute.String("service.version", version.Version),
			attribute.String("service.instance.id", hostname),
		),
	)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, err
	}
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExporter), sdktrace.WithResource(res))
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
	)
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	p := &Providers{
		Tracer:  tp.Tracer("github.com/overplane/overplane"),
		Meter:   mp.Meter("github.com/overplane/overplane"),
		enabled: true,
		shutdown: func(ctx context.Context) error {
			if err := tp.Shutdown(ctx); err != nil {
				_ = mp.Shutdown(ctx)
				return err
			}
			return mp.Shutdown(ctx)
		},
	}
	setGlobal(p)
	return p, nil
}

func (p *Providers) StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return p.Tracer.Start(ctx, name)
}

func (p *Providers) Enabled() bool {
	return p != nil && p.enabled
}

func (p *Providers) Shutdown(ctx context.Context) error {
	if p == nil || p.shutdown == nil {
		return nil
	}
	return p.shutdown(ctx)
}

func Global() *Providers {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return global
}

func setGlobal(p *Providers) {
	globalMu.Lock()
	defer globalMu.Unlock()
	global = p
}

func noop() *Providers {
	return &Providers{Tracer: otel.Tracer("noop"), Meter: otel.Meter("noop")}
}
