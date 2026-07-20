package xrayapi

import (
	"context"
	"net"
	"testing"

	statsservice "github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type fakeStatsServer struct {
	statsservice.UnimplementedStatsServiceServer
	values map[string]int64
}

func (f *fakeStatsServer) GetStats(_ context.Context, req *statsservice.GetStatsRequest) (*statsservice.GetStatsResponse, error) {
	v, ok := f.values[req.Name]
	if !ok {
		return &statsservice.GetStatsResponse{}, nil
	}
	return &statsservice.GetStatsResponse{
		Stat: &statsservice.Stat{Name: req.Name, Value: v},
	}, nil
}

func newTestClient(t *testing.T, srv *fakeStatsServer) (*Client, func()) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer()
	statsservice.RegisterStatsServiceServer(gs, srv)
	go gs.Serve(lis)

	dialer := func(_ context.Context, _ string) (net.Conn, error) { return lis.Dial() }
	// passthrough:/// keeps grpc.NewClient from running the default dns
	// resolver on the fake "bufnet" target; the custom dialer handles the
	// bufconn listener directly.
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	c := &Client{addr: "bufnet", conn: conn, client: statsservice.NewStatsServiceClient(conn)}
	return c, func() {
		_ = c.Close()
		gs.Stop()
		_ = lis.Close()
	}
}

func TestCounters_ReturnsValues(t *testing.T) {
	srv := &fakeStatsServer{values: map[string]int64{
		"outbound>>>proxy>>>traffic>>>uplink":   1024,
		"outbound>>>proxy>>>traffic>>>downlink": 2048,
	}}
	c, cleanup := newTestClient(t, srv)
	defer cleanup()

	up, down, err := c.Counters(context.Background())
	if err != nil {
		t.Fatalf("Counters: %v", err)
	}
	if up != 1024 || down != 2048 {
		t.Fatalf("up=%d down=%d, want 1024 2048", up, down)
	}
}

func TestCounters_MissingStatsReturnsZero(t *testing.T) {
	srv := &fakeStatsServer{values: map[string]int64{}}
	c, cleanup := newTestClient(t, srv)
	defer cleanup()

	up, down, err := c.Counters(context.Background())
	if err != nil {
		t.Fatalf("Counters: %v", err)
	}
	if up != 0 || down != 0 {
		t.Fatalf("up=%d down=%d, want 0 0", up, down)
	}
}

func TestClose_Idempotent(t *testing.T) {
	srv := &fakeStatsServer{}
	c, cleanup := newTestClient(t, srv)
	defer cleanup()

	if err := c.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestNew_LazyDial(t *testing.T) {
	// New() with an unreachable address should NOT fail until Counters is called.
	c := New("127.0.0.1:1") // port 1 is reserved, dial will fail
	defer c.Close()
	// New itself returns successfully — that's the lazy contract.
	if c.addr != "127.0.0.1:1" {
		t.Fatalf("addr=%q", c.addr)
	}
}
