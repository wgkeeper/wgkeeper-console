// Package metrics exposes console metrics in Prometheus exposition format.
//
// The endpoint is opt-in (METRICS_PORT unset or 0 keeps it off). When on, it
// runs on its own listener with a mandatory bearer token, so the scrape secret
// never grants access to the console API.
//
// Cardinality discipline: every label comes from a fixed set (route template,
// status class, node operation, result, node status). Node IDs, peer IDs and
// raw paths are never labels.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Node operations, from the console's point of view.
const (
	OpHealth     = "health"
	OpStats      = "stats"
	OpListPeers  = "list_peers"
	OpGetPeer    = "get_peer"
	OpCreatePeer = "create_peer"
	OpDeletePeer = "delete_peer"
	OpRelease    = "release_check"
	OpOther      = "other"
)

// Results of a node request.
const (
	ResultSuccess      = "success"
	ResultUnauthorized = "error_unauthorized"
	ResultUpstream     = "error_upstream"
	ResultUnreachable  = "error_unreachable"
)

// Metrics is the registry-bound bundle of console metrics, built once at
// startup. Every method is safe on a nil *Metrics, which disables recording.
type Metrics struct {
	registry *prometheus.Registry

	// HTTPRequests counts API requests by method, route template and status class.
	HTTPRequests *prometheus.CounterVec
	// HTTPDuration measures API request latency by method and route template.
	HTTPDuration *prometheus.HistogramVec
	// NodeRequests counts requests the console makes to nodes (and GitHub),
	// by operation and result. Alert on error_unreachable and error_unauthorized.
	NodeRequests *prometheus.CounterVec
	// NodeRequestDuration measures how long nodes take to answer, by operation.
	NodeRequestDuration *prometheus.HistogramVec
}

// New builds the metrics bundle with Go runtime and process collectors.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		registry: reg,
		HTTPRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "wgkeeper_console_http_requests_total",
			Help: "API requests handled, by method, route template and status class.",
		}, []string{"method", "path", "status_class"}),
		HTTPDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "wgkeeper_console_http_request_duration_seconds",
			Help:    "API request latency, by method and route template.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),
		NodeRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "wgkeeper_console_node_requests_total",
			Help: "Requests to wgkeeper nodes and the release check, by operation and result.",
		}, []string{"op", "result"}),
		NodeRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "wgkeeper_console_node_request_duration_seconds",
			Help:    "Time nodes take to answer, by operation.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		}, []string{"op"}),
	}
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.HTTPRequests, m.HTTPDuration, m.NodeRequests, m.NodeRequestDuration,
	)
	return m
}

// Registry returns the registry backing the /metrics endpoint.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

// NodeCounter reports how many nodes are stored per status.
type NodeCounter interface {
	CountNodesByStatus() (map[string]int, error)
}

// BindNodeCounter adds the wgkeeper_console_nodes gauge, read from c at
// scrape time so it always matches the database.
func (m *Metrics) BindNodeCounter(c NodeCounter) {
	if m == nil {
		return
	}
	m.registry.MustRegister(&nodesCollector{
		counter: c,
		desc:    prometheus.NewDesc("wgkeeper_console_nodes", "Stored nodes by last known status.", []string{"status"}, nil),
	})
}

type nodesCollector struct {
	counter NodeCounter
	desc    *prometheus.Desc
}

func (c *nodesCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }

func (c *nodesCollector) Collect(ch chan<- prometheus.Metric) {
	counts, err := c.counter.CountNodesByStatus()
	if err != nil {
		ch <- prometheus.NewInvalidMetric(c.desc, err)
		return
	}
	for _, status := range []string{"online", "offline"} {
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, float64(counts[status]), status)
	}
}
