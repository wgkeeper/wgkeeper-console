package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
)

// BenchmarkAPIRateLimiterAllow measures the shared mutex on every protected
// request, with one busy client and with many clients.
func BenchmarkAPIRateLimiterAllow(b *testing.B) {
	for _, clients := range []int{1, 10000} {
		b.Run(fmt.Sprintf("clients=%d", clients), func(b *testing.B) {
			rl := NewAPIRateLimiter()
			defer rl.Close()
			keys := make([]string, clients)
			for i := range keys {
				keys[i] = fmt.Sprintf("10.%d.%d.%d", i>>16&0xff, i>>8&0xff, i&0xff)
			}
			var n atomic.Uint64
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					rl.allow(keys[n.Add(1)%uint64(clients)])
				}
			})
		})
	}
}

func BenchmarkLoginRateLimiterBeginFinish(b *testing.B) {
	rl := NewLoginRateLimiter()
	defer rl.Close()
	b.ReportAllocs()
	for b.Loop() {
		if rl.begin("203.0.113.7") == 0 {
			rl.finish("203.0.113.7", http.StatusOK)
		}
	}
}

func BenchmarkClientKey(b *testing.B) {
	for _, ip := range []string{"203.0.113.7", "2001:db8:1:2:aaaa:bbbb:cccc:dddd"} {
		b.Run(ip, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				clientKey(ip)
			}
		})
	}
}

// BenchmarkSameOriginGuard is on every /api request.
func BenchmarkSameOriginGuard(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SameOriginGuard(nil))
	r.POST("/api/nodes", func(c *gin.Context) { c.Status(http.StatusOK) })
	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "https://console.example/api/nodes", strings.NewReader(`{}`))
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
}
