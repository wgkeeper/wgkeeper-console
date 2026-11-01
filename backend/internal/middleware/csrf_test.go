package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSameOriginGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SameOriginGuard([]string{"http://localhost:5173"}))
	r.Any("/api/nodes", func(c *gin.Context) { c.Status(http.StatusOK) })

	tests := []struct {
		name        string
		method      string
		headers     map[string]string
		body        string
		contentType string
		want        int
	}{
		{"GET from another site passes", http.MethodGet, map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}, "", "", http.StatusOK},
		{"same-origin POST", http.MethodPost, map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "https://console.example"}, `{}`, "application/json", http.StatusOK},
		{"cross-site POST", http.MethodPost, map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}, `{}`, "application/json", http.StatusForbidden},
		{"same-site sibling POST", http.MethodPost, map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "https://other.console.example"}, `{}`, "application/json", http.StatusForbidden},
		{"cross-site DELETE without body", http.MethodDelete, map[string]string{"Sec-Fetch-Site": "cross-site"}, "", "", http.StatusForbidden},
		{"allowed dev origin", http.MethodPost, map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "http://localhost:5173"}, `{}`, "application/json", http.StatusOK},
		{"old browser, Origin matches Host", http.MethodPost, map[string]string{"Origin": "https://console.example"}, `{}`, "application/json", http.StatusOK},
		{"old browser, foreign Origin", http.MethodPost, map[string]string{"Origin": "https://evil.example"}, `{}`, "application/json", http.StatusForbidden},
		{"no browser headers (curl)", http.MethodPost, nil, `{}`, "application/json", http.StatusOK},
		{"form body", http.MethodPost, map[string]string{"Sec-Fetch-Site": "same-origin"}, `{"a":1}`, "text/plain", http.StatusUnsupportedMediaType},
		{"urlencoded body", http.MethodPost, nil, `a=1`, "application/x-www-form-urlencoded", http.StatusUnsupportedMediaType},
		{"JSON with charset", http.MethodPost, nil, `{}`, "application/json; charset=utf-8", http.StatusOK},
		{"POST without body", http.MethodPost, map[string]string{"Sec-Fetch-Site": "same-origin"}, "", "", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "https://console.example/api/nodes", strings.NewReader(tt.body))
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.want, rec.Body)
			}
		})
	}
}

func TestNoStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(NoStore())
	r.GET("/api/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/x", nil))
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}
