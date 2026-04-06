package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/database"
	"wg-keeper-backend/internal/models"
	"wg-keeper-backend/internal/services"
)

func TestDashboardHandlerGetNodesAndDelete(t *testing.T) {
	t.Setenv("LATEST_NODE_RELEASE_OVERRIDE", "1.1.1")
	db := openDashboardTestDB(t)
	oldDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = oldDB })

	version := "1.1.0"
	node := models.Node{
		ID:        "node-1",
		Name:      "wg-http://127.0.0.1:8080",
		Address:   "http://127.0.0.1:8080",
		APIKey:    "secret",
		Status:    models.NodeStatusOnline,
		Version:   &version,
		CreatedAt: time.Unix(1700000000, 0).UTC(),
		UpdatedAt: time.Unix(1700000100, 0).UTC(),
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("db.Create() error = %v", err)
	}

	handler := NewDashboardHandler()

	t.Run("get nodes", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/nodes?page=1&limit=12", nil)
		handler.GetNodes(ctx)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if body := rec.Body.String(); body == "" || !contains(body, `"isOutdated":true`) || !contains(body, `"latestVersion":"1.1.1"`) {
			t.Fatalf("unexpected body: %s", body)
		}
	})

	t.Run("delete node bad request", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/nodes/", nil)
		handler.DeleteNode(ctx)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("delete node success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Params = gin.Params{{Key: "id", Value: "node-1"}}
		ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/nodes/node-1", nil)
		handler.DeleteNode(ctx)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

func TestDashboardHandlerInputValidation(t *testing.T) {
	db := openDashboardTestDB(t)
	oldDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = oldDB })

	handler := NewDashboardHandler()

	tests := []struct {
		name       string
		method     string
		path       string
		paramKey   string
		paramValue string
		invoke     func(*DashboardHandler, *gin.Context)
		wantStatus int
	}{
		{name: "stats missing id", method: http.MethodGet, path: "/api/nodes//stats", invoke: (*DashboardHandler).GetNodeStats, wantStatus: http.StatusBadRequest},
		{name: "config missing peer id", method: http.MethodGet, path: "/api/nodes/node-1/config", paramKey: "id", paramValue: "node-1", invoke: (*DashboardHandler).GetNodeConfig, wantStatus: http.StatusBadRequest},
		{name: "peers missing id", method: http.MethodGet, path: "/api/nodes//peers", invoke: (*DashboardHandler).GetNodePeers, wantStatus: http.StatusBadRequest},
		{name: "peer detail missing params", method: http.MethodGet, path: "/api/nodes//peers/", invoke: (*DashboardHandler).GetNodePeerDetail, wantStatus: http.StatusBadRequest},
		{name: "delete peer missing query", method: http.MethodDelete, path: "/api/nodes/node-1/peers", paramKey: "id", paramValue: "node-1", invoke: (*DashboardHandler).DeletePeer, wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest(tt.method, tt.path, nil)
			if tt.paramKey != "" {
				ctx.Params = gin.Params{{Key: tt.paramKey, Value: tt.paramValue}}
			}
			tt.invoke(handler, ctx)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestDashboardHandlerCreateAndCheckNode(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	_, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	db := openDashboardTestDB(t)
	oldDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = oldDB })

	restore := stubDashboardRoundTripper(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/readyz":
			return dashboardJSONResponse(http.StatusOK, "")
		case "/stats":
			return dashboardJSONResponse(http.StatusOK, `{"service":{"name":"wgkeeper-node","version":"1.1.0"}}`)
		default:
			return dashboardJSONResponse(http.StatusNotFound, "")
		}
	})
	defer restore()

	handler := NewDashboardHandler()

	t.Run("check invalid body", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/nodes/check", bytes.NewBufferString("{"))
		ctx.Request.Header.Set("Content-Type", "application/json")
		handler.CheckNode(ctx)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("check success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = newDashboardJSONRequest(t, http.MethodPost, "/api/nodes/check", CheckNodeRequest{
			Address: "http://node.example.com:8080", APIKey: "secret",
		})
		handler.CheckNode(ctx)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if !strings.Contains(rec.Body.String(), `"version":"1.1.0"`) {
			t.Fatalf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("create invalid input", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = newDashboardJSONRequest(t, http.MethodPost, "/api/nodes", CreateNodeRequest{
			Address: "not-a-url", APIKey: "",
		})
		handler.CreateNode(ctx)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("create success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = newDashboardJSONRequest(t, http.MethodPost, "/api/nodes", CreateNodeRequest{
			Address: "http://node.example.com:8080", APIKey: "secret",
		})
		handler.CreateNode(ctx)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("create duplicate", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = newDashboardJSONRequest(t, http.MethodPost, "/api/nodes", CreateNodeRequest{
			Address: "http://node.example.com:8080", APIKey: "secret",
		})
		handler.CreateNode(ctx)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
		}
	})
}

func TestDashboardHandlerRefreshNodesEmptyDB(t *testing.T) {
	db := openDashboardTestDB(t)
	oldDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = oldDB })

	handler := NewDashboardHandler()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/nodes/refresh", nil)

	handler.RefreshNodes(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func openDashboardTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}

func contains(haystack string, needle string) bool {
	return strings.Contains(haystack, needle)
}

func newDashboardJSONRequest(t *testing.T, method string, path string, body any) *http.Request {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func stubDashboardRoundTripper(t *testing.T, fn func(req *http.Request) (*http.Response, error)) func() {
	t.Helper()

	prev := services.SetSharedHTTPRoundTripperForTests(roundTripFunc(fn))
	return func() {
		services.SetSharedHTTPRoundTripperForTests(prev)
	}
}

func dashboardJSONResponse(status int, body string) (*http.Response, error) {
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
