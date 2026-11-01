package metrics

import (
	"net/http"
	"strings"
	"time"
)

// InstrumentTransport wraps rt so every request the console sends to a node
// (or to GitHub for the release check) is counted and timed. Returns rt
// unchanged on a nil *Metrics.
func (m *Metrics) InstrumentTransport(rt http.RoundTripper) http.RoundTripper {
	if m == nil {
		return rt
	}
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		op := nodeOp(req)
		start := time.Now()
		resp, err := rt.RoundTrip(req)
		m.NodeRequestDuration.WithLabelValues(op).Observe(time.Since(start).Seconds())
		m.NodeRequests.WithLabelValues(op, nodeResult(resp, err)).Inc()
		return resp, err
	})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// nodeOp maps a request to a fixed operation name from the node API's routes.
func nodeOp(req *http.Request) string {
	if req.URL.Host == "api.github.com" {
		return OpRelease
	}
	path := strings.TrimSuffix(req.URL.Path, "/")
	switch {
	case path == "/readyz":
		return OpHealth
	case path == "/stats":
		return OpStats
	case path == "/peers" && req.Method == http.MethodGet:
		return OpListPeers
	case path == "/peers" && req.Method == http.MethodPost:
		return OpCreatePeer
	case strings.HasPrefix(path, "/peers/") && req.Method == http.MethodGet:
		return OpGetPeer
	case strings.HasPrefix(path, "/peers/") && req.Method == http.MethodDelete:
		return OpDeletePeer
	}
	return OpOther
}

func nodeResult(resp *http.Response, err error) string {
	switch {
	case err != nil:
		return ResultUnreachable
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return ResultUnauthorized
	case resp.StatusCode >= 400:
		return ResultUpstream
	}
	return ResultSuccess
}
