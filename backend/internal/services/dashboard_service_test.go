package services

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"wg-keeper-backend/internal/models"
)

func TestParseComparableVersion(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   [3]int
		wantOK bool
	}{
		{name: "plain semver", input: "1.1.0", want: [3]int{1, 1, 0}, wantOK: true},
		{name: "tagged semver", input: "v2.3.4", want: [3]int{2, 3, 4}, wantOK: true},
		{name: "short semver", input: "1.2", want: [3]int{1, 2, 0}, wantOK: true},
		{name: "prerelease", input: "1.2.3-beta.1", want: [3]int{1, 2, 3}, wantOK: true},
		{name: "invalid", input: "main", want: [3]int{}, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseComparableVersion(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("parseComparableVersion(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("parseComparableVersion(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsVersionOutdated(t *testing.T) {
	version := func(raw string) *string { return &raw }

	tests := []struct {
		name    string
		current *string
		latest  *LatestNodeRelease
		want    bool
	}{
		{
			name:    "older version is outdated",
			current: version("1.0.0"),
			latest:  &LatestNodeRelease{Version: "1.1.0"},
			want:    true,
		},
		{
			name:    "same version is current",
			current: version("v1.1.0"),
			latest:  &LatestNodeRelease{Version: "1.1.0"},
			want:    false,
		},
		{
			name:    "newer version is not outdated",
			current: version("1.2.0"),
			latest:  &LatestNodeRelease{Version: "1.1.0"},
			want:    false,
		},
		{
			name:    "missing node version is ignored",
			current: nil,
			latest:  &LatestNodeRelease{Version: "1.1.0"},
			want:    false,
		},
		{
			name:    "invalid node version is ignored",
			current: version("dev"),
			latest:  &LatestNodeRelease{Version: "1.1.0"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsVersionOutdated(tt.current, tt.latest); got != tt.want {
				t.Fatalf("IsVersionOutdated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStripTrailingSlash(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http://127.0.0.1:8080", "http://127.0.0.1:8080"},
		{"https://example.com:443/", "https://example.com:443"},
		{"https://example.com:443///", "https://example.com:443"},
	}
	for _, tt := range tests {
		if got := stripTrailingSlash(tt.input); got != tt.want {
			t.Fatalf("stripTrailingSlash(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPreviewLatestNodeReleaseOverride(t *testing.T) {
	t.Setenv("LATEST_NODE_RELEASE_OVERRIDE", "v1.2.3")
	release := previewLatestNodeReleaseOverride()
	if release == nil {
		t.Fatalf("expected override release")
	}
	if release.Version != "1.2.3" {
		t.Fatalf("release version = %q, want 1.2.3", release.Version)
	}
}

func TestFindNodeHelpers(t *testing.T) {
	db := openServiceTestDB(t)
	version := "1.0.0"
	node := models.Node{
		ID:      "node-1",
		Name:    "wg-http://127.0.0.1:9999",
		Address: "http://127.0.0.1:9999",
		APIKey:  "secret",
		Status:  models.NodeStatusOnline,
		Version: &version,
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("db.Create() error = %v", err)
	}

	svc := &DashboardService{db: db}
	foundByID, err := svc.FindNodeByID("node-1")
	if err != nil || foundByID == nil {
		t.Fatalf("FindNodeByID() error = %v, node = %v", err, foundByID)
	}

	foundByAddress, err := svc.FindNodeByAddress("http://127.0.0.1:9999")
	if err != nil || foundByAddress == nil {
		t.Fatalf("FindNodeByAddress() error = %v, node = %v", err, foundByAddress)
	}
}

func TestCheckNodeAndCreateNode(t *testing.T) {
	restore := stubSharedRoundTripper(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/readyz":
			return jsonResponse(http.StatusOK, "")
		case "/stats":
			return jsonResponse(http.StatusOK, `{"service":{"name":"wgkeeper-node","version":"1.1.0"},"peers":{"possible":10,"issued":2,"active":1}}`)
		default:
			return jsonResponse(http.StatusNotFound, "")
		}
	})
	defer restore()

	db := openServiceTestDB(t)
	svc := &DashboardService{db: db}

	health, err := svc.CheckNode("http://example.com:8080", "secret")
	if err != nil {
		t.Fatalf("CheckNode() error = %v", err)
	}
	if health.Status != models.NodeStatusOnline {
		t.Fatalf("status = %q, want online", health.Status)
	}

	node, err := svc.CreateNode(struct {
		Address string
		APIKey  string
	}{
		Address: "http://example.com:8080", APIKey: "secret",
	})
	if err != nil {
		t.Fatalf("CreateNode() error = %v", err)
	}
	if node.Address != "http://example.com:8080" {
		t.Fatalf("address = %q, want http://example.com:8080", node.Address)
	}
}

func TestCheckHealthInvalidAPIKey(t *testing.T) {
	restore := stubSharedRoundTripper(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/readyz" {
			return jsonResponse(http.StatusUnauthorized, "")
		}
		return jsonResponse(http.StatusNotFound, "")
	})
	defer restore()

	svc := &DashboardService{}
	health, err := svc.CheckNode("http://example.com:8080", "wrong")
	if err != nil {
		t.Fatalf("CheckNode() error = %v", err)
	}
	if health.Error != "invalid_api_key" {
		t.Fatalf("error = %q, want invalid_api_key", health.Error)
	}
}

func TestFetchStatsDataFallsBackToTrailingSlash(t *testing.T) {
	restore := stubSharedRoundTripper(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/stats":
			return jsonResponse(http.StatusNotFound, "")
		case "/stats/":
			return jsonResponse(http.StatusOK, `{"service":{"version":"1.2.0"}}`)
		default:
			return jsonResponse(http.StatusNotFound, "")
		}
	})
	defer restore()

	svc := &DashboardService{}
	resp, err := svc.fetchStatsData("http://example.com:8080", "secret")
	if err != nil {
		t.Fatalf("fetchStatsData() error = %v", err)
	}
	if resp == nil || resp.Data == nil || resp.Data.Service == nil || resp.Data.Service.Version == nil {
		t.Fatalf("expected stats data with service version")
	}
	if *resp.Endpoint != "/stats/" {
		t.Fatalf("endpoint = %q, want /stats/", *resp.Endpoint)
	}
}

func TestRefreshStatusesUpdatesStoredVersion(t *testing.T) {
	restore := stubSharedRoundTripper(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/readyz":
			return jsonResponse(http.StatusOK, "")
		case "/stats":
			return jsonResponse(http.StatusOK, `{"service":{"version":"1.2.0"}}`)
		default:
			return jsonResponse(http.StatusNotFound, "")
		}
	})
	defer restore()

	db := openServiceTestDB(t)
	initialVersion := "1.0.0"
	node := models.Node{
		ID:      "node-2",
		Name:    "wg-http://refresh.example.com:8080",
		Address: "http://refresh.example.com:8080",
		APIKey:  "secret",
		Status:  models.NodeStatusOffline,
		Version: &initialVersion,
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("db.Create() error = %v", err)
	}

	svc := &DashboardService{db: db}
	if err := svc.RefreshStatuses(); err != nil {
		t.Fatalf("RefreshStatuses() error = %v", err)
	}

	var updated models.Node
	if err := db.Where("id = ?", "node-2").First(&updated).Error; err != nil {
		t.Fatalf("db.First() error = %v", err)
	}
	if updated.Status != models.NodeStatusOnline {
		t.Fatalf("status = %q, want online", updated.Status)
	}
	if updated.Version == nil || *updated.Version != "1.2.0" {
		t.Fatalf("version = %v, want 1.2.0", updated.Version)
	}
}

func BenchmarkParseComparableVersion(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, ok := parseComparableVersion("v1.2.3-beta.4"); !ok {
			b.Fatalf("parseComparableVersion() returned ok=false")
		}
	}
}

func BenchmarkIsVersionOutdated(b *testing.B) {
	current := "1.1.0"
	latest := &LatestNodeRelease{Version: "1.1.1"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !IsVersionOutdated(&current, latest) {
			b.Fatalf("IsVersionOutdated() returned false")
		}
	}
}

func BenchmarkStripTrailingSlash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = stripTrailingSlash("https://127.0.0.1:8080/")
	}
}

func openServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}

func stubSharedRoundTripper(t *testing.T, fn func(req *http.Request) (*http.Response, error)) func() {
	t.Helper()

	prev := sharedHTTPRoundTripper
	sharedHTTPRoundTripper = roundTripFunc(fn)
	return func() {
		sharedHTTPRoundTripper = prev
	}
}

func jsonResponse(status int, body string) (*http.Response, error) {
	resp := &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	if body != "" {
		resp.Header.Set("Content-Type", "application/json")
	}
	return resp, nil
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
