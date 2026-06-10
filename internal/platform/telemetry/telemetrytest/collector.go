package telemetrytest

import (
	"context"
	"net"
)

type Collector struct {
	Listener net.Listener
	Spans    []string
	Metrics  []string
}

func StartGRPC(ctx context.Context) (*Collector, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	c := &Collector{Listener: l}
	go func() {
		<-ctx.Done()
		_ = l.Close()
	}()
	return c, nil
}

func (c *Collector) Addr() string {
	if c == nil || c.Listener == nil {
		return ""
	}
	return c.Listener.Addr().String()
}

func (c *Collector) Close() error {
	if c == nil || c.Listener == nil {
		return nil
	}
	return c.Listener.Close()
}
