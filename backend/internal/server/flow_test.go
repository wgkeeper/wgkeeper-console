package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/database"
	"wg-keeper-backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const fakeNodeKey = "node-api-key-for-tests"

// fakeNode implements the parts of the wgkeeper-node API the console uses.
type fakeNode struct {
	mu          sync.Mutex
	peers       map[string]map[string]any
	lastRequest map[string]any
}

func newFakeNode(t *testing.T) (*fakeNode, *httptest.Server) {
	t.Helper()
	n := &fakeNode{peers: map[string]map[string]any{}}
	srv := httptest.NewServer(http.HandlerFunc(n.serve))
	t.Cleanup(srv.Close)
	return n, srv
}

func (n *fakeNode) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/readyz" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Header.Get("X-API-Key") != fakeNodeKey {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")

	switch {
	case r.URL.Path == "/stats":
		_, _ = fmt.Fprintf(w, `{"service":{"name":"wgkeeper-node","version":"2.0.0"},"wireguard":{"listenPort":51820},"peers":{"possible":253,"issued":%d}}`, len(n.peers))
	case r.URL.Path == "/peers" && r.Method == http.MethodPost:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		n.lastRequest = body
		id, _ := body["peerId"].(string)
		peer := map[string]any{"peerId": id, "publicKey": "pub-" + id, "allowedIPs": []string{"10.66.0.2/32"}, "active": false, "createdAt": "2026-11-01T00:00:00Z"}
		n.peers[id] = peer
		_ = json.NewEncoder(w).Encode(map[string]any{
			"server": map[string]any{"publicKey": "server-public-key", "listenPort": 51820},
			"peer":   map[string]any{"peerId": id, "publicKey": "pub-" + id, "privateKey": "peer-private-key", "presharedKey": "peer-psk", "allowedIPs": []string{"10.66.0.2/32"}},
		})
	case r.URL.Path == "/peers" && r.Method == http.MethodGet:
		data := make([]map[string]any, 0, len(n.peers))
		for _, p := range n.peers {
			data = append(data, p)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "meta": map[string]any{"totalItems": len(data)}})
	case strings.HasPrefix(r.URL.Path, "/peers/"):
		id := strings.TrimPrefix(r.URL.Path, "/peers/")
		peer, ok := n.peers[id]
		if !ok {
			http.Error(w, `{"error":"peer not found"}`, http.StatusNotFound)
			return
		}
		if r.Method == http.MethodDelete {
			delete(n.peers, id)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = json.NewEncoder(w).Encode(peer)
	default:
		http.NotFound(w, r)
	}
}

// apiClient is a browser stand-in: it keeps the session cookie.
type apiClient struct {
	t    *testing.T
	base string
	http *http.Client
}

func (c *apiClient) do(method, path, body string) (int, string) {
	c.t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(c.t.Context(), method, c.base+path, r)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(data)
}

func (c *apiClient) expect(method, path, body string, want int) string {
	c.t.Helper()
	got, resp := c.do(method, path, body)
	if got != want {
		c.t.Fatalf("%s %s = %d, want %d: %s", method, path, got, want, resp)
	}
	return resp
}

// TestAPIFlow walks the whole console API as the UI uses it, against a fake
// node: first login, the forced password change, adding a node, peers and
// configs, refresh, deletion and logout.
func TestAPIFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "test")
	t.Setenv("SECRET_KEY", strings.Repeat("cd", 32))
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open("file:apiflow?mode=memory&cache=shared"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := models.EnsureDefaultAdmin(db, "admin", "bootstrap-password"); err != nil {
		t.Fatal(err)
	}
	oldDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = oldDB })
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	router, stop, err := NewRouter(cfg, Deps{DB: sqlDB, StaticRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
	console := httptest.NewServer(router)
	t.Cleanup(console.Close)

	node, nodeSrv := newFakeNode(t)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &apiClient{t: t, base: console.URL, http: &http.Client{Jar: jar}}

	// First login with the bootstrap password; nodes stay closed until it changes.
	c.expect(http.MethodGet, "/api/me", "", http.StatusUnauthorized)
	c.expect(http.MethodPost, "/api/login", `{"username":"admin","password":"wrong"}`, http.StatusUnauthorized)
	if resp := c.expect(http.MethodPost, "/api/login", `{"username":"admin","password":"bootstrap-password"}`, http.StatusOK); !strings.Contains(resp, `"mustChangePassword":true`) {
		t.Fatalf("login response %s, want mustChangePassword", resp)
	}
	c.expect(http.MethodGet, "/api/nodes", "", http.StatusForbidden)
	c.expect(http.MethodPost, "/api/change-password", `{"newPassword":"a-much-better-password"}`, http.StatusOK)
	if resp := c.expect(http.MethodGet, "/api/me", "", http.StatusOK); !strings.Contains(resp, `"mustChangePassword":false`) {
		t.Fatalf("/api/me after the change = %s", resp)
	}

	// Check and add the node.
	add := func(key string) string {
		return fmt.Sprintf(`{"address":%q,"apiKey":%q}`, nodeSrv.URL, key)
	}
	c.expect(http.MethodPost, "/api/nodes/check", add("wrong-key"), http.StatusUnprocessableEntity)
	if resp := c.expect(http.MethodPost, "/api/nodes/check", add(fakeNodeKey), http.StatusOK); !strings.Contains(resp, `"version":"2.0.0"`) {
		t.Fatalf("check = %s", resp)
	}
	c.expect(http.MethodPost, "/api/nodes", `{"address":"not a url","apiKey":"x"}`, http.StatusBadRequest)
	var created struct{ ID string }
	if err := json.Unmarshal([]byte(c.expect(http.MethodPost, "/api/nodes", add(fakeNodeKey), http.StatusOK)), &created); err != nil || created.ID == "" {
		t.Fatalf("create node: id %q, err %v", created.ID, err)
	}
	c.expect(http.MethodPost, "/api/nodes", add(fakeNodeKey), http.StatusConflict)
	if resp := c.expect(http.MethodGet, "/api/nodes", "", http.StatusOK); !strings.Contains(resp, `"status":"online"`) || !strings.Contains(resp, `"total":1`) {
		t.Fatalf("node list = %s", resp)
	}
	nodePath := "/api/nodes/" + created.ID
	c.expect(http.MethodGet, nodePath+"/stats", "", http.StatusOK)

	// Generate a config: the node is asked to create the peer.
	peerID := uuid.NewString()
	q := url.Values{
		"peerId":          {peerID},
		"dns":             {"9.9.9.9, 2620:fe::fe"},
		"addressFamilies": {"IPv4,bogus"},
		"expiresAt":       {"2030-01-01T00:00:00Z"},
	}
	conf := c.expect(http.MethodPost, nodePath+"/config?"+q.Encode(), "", http.StatusOK)
	nodeHost := strings.TrimPrefix(nodeSrv.URL, "http://")
	host, _, _ := strings.Cut(nodeHost, ":")
	for _, want := range []string{"PrivateKey = peer-private-key", "Address = 10.66.0.2/32", "DNS = 9.9.9.9\n", "Endpoint = " + host + ":51820"} {
		if !strings.Contains(conf, want) {
			t.Fatalf("config lacks %q:\n%s", want, conf)
		}
	}
	node.mu.Lock()
	sent := node.lastRequest
	node.mu.Unlock()
	if sent["expiresAt"] != "2030-01-01T00:00:00Z" || fmt.Sprint(sent["addressFamilies"]) != "[IPv4]" {
		t.Fatalf("node got %v, want expiresAt and only the valid address family", sent)
	}
	c.expect(http.MethodPost, nodePath+"/config?peerId="+peerID+"&expiresAt=tomorrow", "", http.StatusBadRequest)
	c.expect(http.MethodPost, nodePath+"/config", "", http.StatusBadRequest)
	c.expect(http.MethodPost, "/api/nodes/missing/config?peerId="+peerID, "", http.StatusNotFound)

	// Peers.
	if resp := c.expect(http.MethodGet, nodePath+"/peers", "", http.StatusOK); !strings.Contains(resp, peerID) {
		t.Fatalf("peer list = %s", resp)
	}
	c.expect(http.MethodGet, nodePath+"/peers/"+peerID, "", http.StatusOK)
	c.expect(http.MethodGet, nodePath+"/peers/"+uuid.NewString(), "", http.StatusNotFound)
	if resp := c.expect(http.MethodDelete, nodePath+"/peers?peerId="+peerID, "", http.StatusOK); !strings.Contains(resp, `"deleted":true`) {
		t.Fatalf("delete peer = %s", resp)
	}
	if resp := c.expect(http.MethodDelete, nodePath+"/peers?peerId="+peerID, "", http.StatusOK); !strings.Contains(resp, `"notFound":true`) {
		t.Fatalf("second delete = %s", resp)
	}

	// Refresh, delete the node, log out.
	c.expect(http.MethodPost, "/api/nodes/refresh", "", http.StatusOK)
	c.expect(http.MethodDelete, nodePath, "", http.StatusOK)
	c.expect(http.MethodGet, nodePath+"/stats", "", http.StatusNotFound)
	c.expect(http.MethodPost, "/api/logout", "", http.StatusOK)
	c.expect(http.MethodGet, "/api/me", "", http.StatusUnauthorized)
}

func TestStaticFilesInProduction(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"index.html":    "<!doctype html><title>wgkeeper console</title>",
		"robots.txt":    "User-agent: *\nDisallow: /",
		"favicon.svg":   "<svg/>",
		"assets/app.js": "console.log(1)",
		"theme-init.js": "",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerStatic(r, root, true)

	get := func(path, accept string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	if rec := get("/", ""); rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-cache, must-revalidate" {
		t.Fatalf("/ = %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	if rec := get("/nodes/abc", "text/html"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "wgkeeper console") {
		t.Fatalf("SPA route = %d", rec.Code)
	}
	for _, path := range []string{"/assets/app.js", "/robots.txt", "/favicon.svg"} {
		if rec := get(path, ""); rec.Code != http.StatusOK {
			t.Fatalf("%s = %d", path, rec.Code)
		}
	}
	if rec := get("/api/unknown", "text/html"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown API path = %d, want 404 not the SPA", rec.Code)
	}
	if rec := get("/missing.png", "image/png"); rec.Code != http.StatusNotFound {
		t.Fatalf("missing asset = %d, want 404", rec.Code)
	}
}

func TestDevModeHasNoFrontend(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerStatic(r, t.TempDir(), false)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "frontend_dev_server_only") {
		t.Fatalf("dev mode / = %d %s", rec.Code, rec.Body)
	}
}
