package services

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// A node, or a proxy in front of it, that answers with a redirect must not get
// the console to resend the API key to another host.
func TestNodeRequestsDoNotFollowRedirects(t *testing.T) {
	var leaked atomic.Value
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked.Store(r.Header.Get("X-API-KEY"))
		w.WriteHeader(http.StatusOK)
	}))
	defer attacker.Close()

	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, attacker.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer node.Close()

	// The calls may fail; all that matters is where the key went.
	s := NewDashboardService()
	_, _ = s.fetchStatsData(node.URL, "node-api-key")
	_, _ = s.CheckNode(node.URL, "node-api-key")

	if v, _ := leaked.Load().(string); v != "" {
		t.Fatalf("the API key was sent to the redirect target: %q", v)
	}
}
