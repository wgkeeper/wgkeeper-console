package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"wg-keeper-backend/internal/database"
	"wg-keeper-backend/internal/models"
	"wg-keeper-backend/internal/services"

	"github.com/gin-gonic/gin"
)

func TestWriteNodeError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{"missing node", services.ErrNodeNotFound, http.StatusNotFound, `"error":"node_not_found"`},
		{"wrapped missing node", fmt.Errorf("lookup: %w", services.ErrNodeNotFound), http.StatusNotFound, `"node_not_found"`},
		{"key rejected", &services.UpstreamError{Status: http.StatusUnauthorized}, http.StatusBadGateway, `"error":"invalid_api_key"`},
		{"node error", &services.UpstreamError{Status: http.StatusInternalServerError}, http.StatusBadGateway, `"error":"peers_unavailable","ok":false,"status":500`},
		{"unreachable", fmt.Errorf("%w: refused", services.ErrNodeUnreachable), http.StatusBadGateway, `"error":"peers_unavailable"`},
		{"node timeout", context.DeadlineExceeded, http.StatusBadGateway, `"peers_unavailable"`},
		{"client gone", context.Canceled, http.StatusServiceUnavailable, ``},
		{"database error", errors.New("database is locked"), http.StatusInternalServerError, `{"ok":false}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/nodes/n1/peers", nil)
			writeNodeError(c, tt.err, "peers_unavailable")
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Fatalf("body = %s, want it to contain %s", rec.Body, tt.wantBody)
			}
		})
	}
}

func dashboardWithNode(t *testing.T) *DashboardHandler {
	t.Helper()
	db := openHandlersTestDB(t)
	oldDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = oldDB })
	node := models.Node{ID: "node-1", Name: "n1", Address: "http://node.test:51821", APIKey: "secret", Status: models.NodeStatusOnline}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	return NewDashboardHandler()
}

func serve(h gin.HandlerFunc, method, route, target string) *httptest.ResponseRecorder {
	r := gin.New()
	r.Handle(method, route, h)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestGetNodePeersUnreachableIsNotAnEmptyList(t *testing.T) {
	h := dashboardWithNode(t)
	restore := stubDashboardRoundTripper(t, func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})
	defer restore()

	rec := serve(h.GetNodePeers, http.MethodGet, "/api/nodes/:id/peers", "/api/nodes/node-1/peers")
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "peers_unavailable") {
		t.Fatalf("unreachable node: status = %d body = %s, want 502 peers_unavailable", rec.Code, rec.Body)
	}

	rec = serve(h.GetNodePeers, http.MethodGet, "/api/nodes/:id/peers", "/api/nodes/missing/peers")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing node: status = %d, want 404", rec.Code)
	}
}

func TestGetNodePeerDetailStatuses(t *testing.T) {
	h := dashboardWithNode(t)
	var status int
	restore := stubDashboardRoundTripper(t, func(*http.Request) (*http.Response, error) {
		return dashboardJSONResponse(status, `{}`)
	})
	defer restore()

	for _, tt := range []struct {
		node     string
		upstream int
		want     int
		body     string
	}{
		{"node-1", http.StatusNotFound, http.StatusNotFound, "peer_not_found"},
		{"node-1", http.StatusInternalServerError, http.StatusBadGateway, "peer_unavailable"},
		{"missing", http.StatusOK, http.StatusNotFound, "node_not_found"},
	} {
		status = tt.upstream
		rec := serve(h.GetNodePeerDetail, http.MethodGet, "/api/nodes/:id/peers/:peerId", "/api/nodes/"+tt.node+"/peers/p1")
		if rec.Code != tt.want || !strings.Contains(rec.Body.String(), tt.body) {
			t.Errorf("node %s, upstream %d: status = %d body = %s, want %d %s", tt.node, tt.upstream, rec.Code, rec.Body, tt.want, tt.body)
		}
	}
}

func TestDeleteNodeStatuses(t *testing.T) {
	h := dashboardWithNode(t)

	if rec := serve(h.DeleteNode, http.MethodDelete, "/api/nodes/:id", "/api/nodes/missing"); rec.Code != http.StatusNotFound {
		t.Fatalf("missing node: status = %d, want 404", rec.Code)
	}

	sqlDB, err := database.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	if rec := serve(h.DeleteNode, http.MethodDelete, "/api/nodes/:id", "/api/nodes/node-1"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("database down: status = %d, want 500 (not a misleading 404)", rec.Code)
	}
}
