package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"wg-keeper-backend/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTestService opens a private database (with driver errors translated, as
// in production) holding one node at address.
func newTestService(t *testing.T, address string) (*DashboardService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if address != "" {
		node := models.Node{ID: "node-1", Name: "n1", Address: address, APIKey: "secret", Status: models.NodeStatusOnline}
		if err := db.Create(&node).Error; err != nil {
			t.Fatal(err)
		}
	}
	return &DashboardService{db: db, latestReleaseTTL: 10 * time.Minute}, db
}

func TestGetNodePeersReportsNodeFailures(t *testing.T) {
	big := `{"data":[` + strings.Repeat(`{"peerId":"p","publicKey":"k"},`, maxNodeResponseBytes/30) + `{}]}`
	tests := []struct {
		name   string
		answer func() (*http.Response, error)
		check  func(t *testing.T, err error)
	}{
		{"connection refused", func() (*http.Response, error) { return nil, errors.New("dial tcp: connection refused") },
			func(t *testing.T, err error) {
				if !errors.Is(err, ErrNodeUnreachable) {
					t.Fatalf("error = %v, want ErrNodeUnreachable", err)
				}
			}},
		{"server error", func() (*http.Response, error) {
			return jsonResponse(http.StatusInternalServerError, `{"error":"boom"}`)
		},
			func(t *testing.T, err error) {
				var up *UpstreamError
				if !errors.As(err, &up) || up.Status != http.StatusInternalServerError || up.Unauthorized() {
					t.Fatalf("error = %v, want UpstreamError 500", err)
				}
			}},
		{"rejected API key", func() (*http.Response, error) { return jsonResponse(http.StatusUnauthorized, ``) },
			func(t *testing.T, err error) {
				var up *UpstreamError
				if !errors.As(err, &up) || !up.Unauthorized() {
					t.Fatalf("error = %v, want an unauthorized UpstreamError", err)
				}
			}},
		{"bad JSON", func() (*http.Response, error) { return jsonResponse(http.StatusOK, `{"data":`) },
			func(t *testing.T, err error) {
				if !errors.Is(err, ErrNodeUnreachable) {
					t.Fatalf("error = %v, want ErrNodeUnreachable", err)
				}
			}},
		{"oversized response", func() (*http.Response, error) { return jsonResponse(http.StatusOK, big) },
			func(t *testing.T, err error) {
				if !errors.Is(err, ErrNodeUnreachable) || !errors.Is(err, errResponseTooLarge) {
					t.Fatalf("error = %v, want a too-large ErrNodeUnreachable", err)
				}
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestService(t, "http://node.test:51821")
			restore := stubSharedRoundTripper(t, func(*http.Request) (*http.Response, error) { return tt.answer() })
			defer restore()

			got, err := svc.GetNodePeers(t.Context(), "node-1", 0, 50)
			if got != nil {
				t.Fatalf("a failed node must not look like an empty peer list, got %+v", got)
			}
			tt.check(t, err)
		})
	}
}

func TestGetNodePeersFiltersBlankRows(t *testing.T) {
	svc, _ := newTestService(t, "http://node.test:51821")
	restore := stubSharedRoundTripper(t, func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("X-API-Key"); got != "secret" {
			t.Errorf("X-API-Key = %q, want secret", got)
		}
		if req.Header.Get("X_API_KEY") != "" {
			t.Error("the API key is sent twice")
		}
		return jsonResponse(http.StatusOK, `{"data":[{"peerId":"a","publicKey":"ka"},{"peerId":"","publicKey":"kb"},{"peerId":"c","publicKey":" "}],"meta":{"totalItems":3}}`)
	})
	defer restore()

	got, err := svc.GetNodePeers(t.Context(), "node-1", 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Data) != 1 || got.Data[0].PeerID != "a" {
		t.Fatalf("got %+v, want only peer a", got.Data)
	}
}

func TestGetNodePeerDetailNotFound(t *testing.T) {
	svc, _ := newTestService(t, "http://node.test:51821")
	restore := stubSharedRoundTripper(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.EscapedPath() != "/peers/a%2Fb" {
			t.Errorf("path = %q, want the peer ID escaped", req.URL.EscapedPath())
		}
		return jsonResponse(http.StatusNotFound, `{"error":"peer not found"}`)
	})
	defer restore()

	_, err := svc.GetNodePeerDetail(t.Context(), "node-1", "a/b")
	var up *UpstreamError
	if !errors.As(err, &up) || up.Status != http.StatusNotFound {
		t.Fatalf("error = %v, want UpstreamError 404", err)
	}
	if _, err := svc.GetNodePeerDetail(t.Context(), "missing", "a"); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("missing node: error = %v, want ErrNodeNotFound", err)
	}
}

func TestFetchStatsDataRetriesOnlyOnNotFound(t *testing.T) {
	tests := []struct {
		name      string
		answer    func() (*http.Response, error)
		wantCalls int32
	}{
		{"unreachable node is not retried", func() (*http.Response, error) { return nil, errors.New("timeout") }, 1},
		{"server error is not retried", func() (*http.Response, error) { return jsonResponse(http.StatusInternalServerError, ``) }, 1},
		{"missing /stats falls back to /stats/", func() (*http.Response, error) { return jsonResponse(http.StatusNotFound, ``) }, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			restore := stubSharedRoundTripper(t, func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				return tt.answer()
			})
			defer restore()

			if _, err := NewDashboardService().fetchStatsData(t.Context(), "http://node.test:51821", "secret"); err != nil {
				t.Fatal(err)
			}
			if got := calls.Load(); got != tt.wantCalls {
				t.Fatalf("node was called %d times, want %d", got, tt.wantCalls)
			}
		})
	}
}

func TestConfigEndpointBracketsIPv6(t *testing.T) {
	const peer = `{"server":{"publicKey":"spk","listenPort":51820},"peer":{"peerId":"p","privateKey":"priv","presharedKey":"psk","allowedIPs":["10.0.0.2/32","fd00::2/128"]}}`
	for _, tt := range []struct{ address, want string }{
		{"https://[fd00::1]:51821", "Endpoint = [fd00::1]:51820"},
		{"https://vpn.example.com:51821", "Endpoint = vpn.example.com:51820"},
		{"http://203.0.113.5:51821", "Endpoint = 203.0.113.5:51820"},
	} {
		t.Run(tt.address, func(t *testing.T) {
			restore := stubSharedRoundTripper(t, func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, peer)
			})
			defer restore()

			cfg, err := NewDashboardService().fetchConfigData(t.Context(), tt.address, "secret", "p", nil, nil, nil)
			if err != nil || cfg.Data == nil {
				t.Fatalf("fetchConfigData() = (%+v, %v)", cfg, err)
			}
			if !strings.Contains(*cfg.Data, tt.want+"\n") {
				t.Fatalf("config does not contain %q:\n%s", tt.want, *cfg.Data)
			}
		})
	}
}

func TestDeletePeerUnreachable(t *testing.T) {
	svc, _ := newTestService(t, "http://node.test:51821")
	restore := stubSharedRoundTripper(t, func(*http.Request) (*http.Response, error) { return nil, errors.New("refused") })
	defer restore()
	if _, err := svc.DeletePeer(t.Context(), "node-1", "p"); !errors.Is(err, ErrNodeUnreachable) {
		t.Fatalf("error = %v, want ErrNodeUnreachable", err)
	}
}

func TestCreateNodeDuplicateIsErrNodeExists(t *testing.T) {
	svc, _ := newTestService(t, "")
	restore := stubSharedRoundTripper(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{}`)
	})
	defer restore()

	input := struct {
		Address string
		APIKey  string
	}{"http://node.test:51821", "secret"}
	if _, err := svc.CreateNode(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateNode(t.Context(), input); !errors.Is(err, ErrNodeExists) {
		t.Fatalf("second create: error = %v, want ErrNodeExists", err)
	}
}

func TestLatestReleaseFailureIsCached(t *testing.T) {
	t.Setenv("LATEST_NODE_RELEASE_OVERRIDE", "")
	var calls atomic.Int32
	restore := stubSharedRoundTripper(t, func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return jsonResponse(http.StatusForbidden, `{"message":"rate limited"}`)
	})
	defer restore()

	svc, _ := newTestService(t, "")
	if _, err := svc.GetLatestNodeRelease(t.Context()); err == nil {
		t.Fatal("expected an error from the failed lookup")
	}
	for range 5 {
		if _, err := svc.GetLatestNodeRelease(t.Context()); err == nil {
			t.Fatal("expected an error while backing off")
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("GitHub was called %d times, want 1 during the backoff", got)
	}
}

func TestLatestReleaseLookupsAreShared(t *testing.T) {
	t.Setenv("LATEST_NODE_RELEASE_OVERRIDE", "")
	var calls atomic.Int32
	release := make(chan struct{})
	restore := stubSharedRoundTripper(t, func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		<-release
		return jsonResponse(http.StatusOK, `{"tag_name":"v2.0.0","html_url":"https://example.com"}`)
	})
	defer restore()

	svc, _ := newTestService(t, "")
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if r, err := svc.GetLatestNodeRelease(t.Context()); err != nil || r.Version != "2.0.0" {
				t.Errorf("GetLatestNodeRelease() = (%v, %v)", r, err)
			}
		})
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if got := calls.Load(); got != 1 {
		t.Fatalf("GitHub was called %d times for 10 concurrent requests, want 1", got)
	}
}

func TestRefreshStatusesKeepsStatusOfNodesCutOffByDeadline(t *testing.T) {
	svc, db := newTestService(t, "http://fast.test:51821")
	slow := models.Node{ID: "node-slow", Name: "slow", Address: "http://slow.test:51821", APIKey: "secret", Status: models.NodeStatusOnline}
	if err := db.Create(&slow).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.Node{}).Where("id = ?", "node-1").Update("status", models.NodeStatusOffline).Error; err != nil {
		t.Fatal(err)
	}
	svc.sweepTimeout = 300 * time.Millisecond

	restore := stubSharedRoundTripper(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "slow.test:51821" {
			<-req.Context().Done() // never answers before the sweep deadline
			return nil, req.Context().Err()
		}
		if strings.HasSuffix(req.URL.Path, "/readyz") {
			return jsonResponse(http.StatusOK, ``)
		}
		return jsonResponse(http.StatusOK, `{"service":{"version":"2.0.0"}}`)
	})
	defer restore()

	start := time.Now()
	if err := svc.RefreshStatuses(t.Context()); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("refresh took %v, want it bounded by the sweep timeout", elapsed)
	}

	var fast, stillSlow models.Node
	db.First(&fast, "id = ?", "node-1")
	db.First(&stillSlow, "id = ?", "node-slow")
	if fast.Status != models.NodeStatusOnline || fast.Version == nil || *fast.Version != "2.0.0" {
		t.Fatalf("fast node = %s %v, want online 2.0.0", fast.Status, fast.Version)
	}
	if stillSlow.Status != models.NodeStatusOnline {
		t.Fatalf("a node cut off by the deadline was marked %s, want its last status kept", stillSlow.Status)
	}
}

func TestRefreshStatusesSharesConcurrentSweeps(t *testing.T) {
	svc, _ := newTestService(t, "http://node.test:51821")
	var readyz atomic.Int32
	release := make(chan struct{})
	restore := stubSharedRoundTripper(t, func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/readyz") {
			readyz.Add(1)
			<-release
			return jsonResponse(http.StatusOK, ``)
		}
		return jsonResponse(http.StatusOK, `{}`)
	})
	defer restore()

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			if err := svc.RefreshStatuses(context.Background()); err != nil {
				t.Error(err)
			}
		})
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if got := readyz.Load(); got != 1 {
		t.Fatalf("node checked %d times for 5 concurrent refreshes, want 1", got)
	}
}
