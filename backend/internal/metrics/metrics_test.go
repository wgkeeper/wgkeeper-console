package metrics

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNodeOp(t *testing.T) {
	tests := []struct {
		method, rawURL, want string
	}{
		{http.MethodGet, "https://node.example/readyz", OpHealth},
		{http.MethodGet, "https://node.example/stats", OpStats},
		{http.MethodGet, "https://node.example/stats/", OpStats},
		{http.MethodGet, "https://node.example/peers?offset=0&limit=50", OpListPeers},
		{http.MethodPost, "https://node.example/peers", OpCreatePeer},
		{http.MethodGet, "https://node.example/peers/3f7a2c1e", OpGetPeer},
		{http.MethodDelete, "https://node.example/peers/3f7a2c1e", OpDeletePeer},
		{http.MethodGet, "https://api.github.com/repos/wgkeeper/wgkeeper-node/releases/latest", OpRelease},
		{http.MethodGet, "https://node.example/something/else", OpOther},
	}
	for _, tt := range tests {
		u, err := url.Parse(tt.rawURL)
		if err != nil {
			t.Fatal(err)
		}
		if got := nodeOp(&http.Request{Method: tt.method, URL: u}); got != tt.want {
			t.Errorf("nodeOp(%s %s) = %q, want %q", tt.method, tt.rawURL, got, tt.want)
		}
	}
}

func TestInstrumentTransport(t *testing.T) {
	m := New()
	answers := []func() (*http.Response, error){
		func() (*http.Response, error) { return &http.Response{StatusCode: 200, Body: http.NoBody}, nil },
		func() (*http.Response, error) { return &http.Response{StatusCode: 401, Body: http.NoBody}, nil },
		func() (*http.Response, error) { return &http.Response{StatusCode: 503, Body: http.NoBody}, nil },
		func() (*http.Response, error) { return nil, errors.New("connection refused") },
	}
	i := 0
	rt := m.InstrumentTransport(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		resp, err := answers[i]()
		i++
		return resp, err
	}))
	for range answers {
		req, _ := http.NewRequest(http.MethodGet, "https://node.example/stats", nil)
		if resp, err := rt.RoundTrip(req); err == nil {
			_ = resp.Body.Close()
		}
	}

	for result, want := range map[string]float64{ResultSuccess: 1, ResultUnauthorized: 1, ResultUpstream: 1, ResultUnreachable: 1} {
		if got := testutil.ToFloat64(m.NodeRequests.WithLabelValues(OpStats, result)); got != want {
			t.Errorf("node_requests_total{op=stats,result=%s} = %v, want %v", result, got, want)
		}
	}
	if got := testutil.CollectAndCount(m.NodeRequestDuration); got != 1 {
		t.Errorf("duration series = %d, want 1 (one op)", got)
	}
}

func TestNilMetricsAreNoOps(t *testing.T) {
	var m *Metrics
	base := roundTripperFunc(func(*http.Request) (*http.Response, error) { return nil, nil })
	if rt := m.InstrumentTransport(base); rt == nil {
		t.Fatal("nil metrics must return the transport unchanged")
	}
	m.BindNodeCounter(fakeCounter{})
	if m.GinMiddleware() == nil {
		t.Fatal("nil metrics must return a no-op middleware")
	}
}

type fakeCounter struct {
	counts map[string]int
	err    error
}

func (f fakeCounter) CountNodesByStatus() (map[string]int, error) { return f.counts, f.err }

func TestNodesGauge(t *testing.T) {
	m := New()
	m.BindNodeCounter(fakeCounter{counts: map[string]int{"online": 3, "offline": 1}})
	want := `
# HELP wgkeeper_console_nodes Stored nodes by last known status.
# TYPE wgkeeper_console_nodes gauge
wgkeeper_console_nodes{status="offline"} 1
wgkeeper_console_nodes{status="online"} 3
`
	if err := testutil.GatherAndCompare(m.Registry(), strings.NewReader(want), "wgkeeper_console_nodes"); err != nil {
		t.Fatal(err)
	}
}
