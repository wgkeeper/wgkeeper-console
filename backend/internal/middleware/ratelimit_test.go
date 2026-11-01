package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gin-gonic/gin"
)

func TestClientKey(t *testing.T) {
	tests := []struct{ ip, want string }{
		{"203.0.113.7", "203.0.113.7"},
		{"::ffff:203.0.113.7", "203.0.113.7"},
		{"2001:db8:1:2:aaaa::1", "2001:db8:1:2::/64"},
		{"2001:db8:1:2:ffff:ffff:ffff:ffff", "2001:db8:1:2::/64"},
		{"2001:db8:1:3::1", "2001:db8:1:3::/64"},
		{"not-an-ip", "not-an-ip"},
	}
	for _, tt := range tests {
		if got := clientKey(tt.ip); got != tt.want {
			t.Errorf("clientKey(%q) = %q, want %q", tt.ip, got, tt.want)
		}
	}
}

// loginRouter wraps a handler that answers with the status queued for the
// next request, and counts how often the handler actually ran.
func loginRouter(rl *LoginRateLimiter, status *atomic.Int32, calls *atomic.Int32) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login", LoginRateLimit(rl), func(c *gin.Context) {
		calls.Add(1)
		c.Status(int(status.Load()))
	})
	return r
}

func login(r http.Handler, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestLoginRateLimitBlocksAfterFailures(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rl := NewLoginRateLimiter()
		defer rl.Close()
		var status, calls atomic.Int32
		status.Store(http.StatusUnauthorized)
		r := loginRouter(rl, &status, &calls)

		for i := range loginMaxFailures {
			if rec := login(r, "198.51.100.1:1234"); rec.Code != http.StatusUnauthorized {
				t.Fatalf("attempt %d: status = %d, want 401", i+1, rec.Code)
			}
		}
		rec := login(r, "198.51.100.1:1234")
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("after %d failures: status = %d, want 429", loginMaxFailures, rec.Code)
		}
		if calls.Load() != loginMaxFailures {
			t.Fatalf("handler ran %d times, want %d: a blocked attempt must not reach the password check", calls.Load(), loginMaxFailures)
		}
		retry, _ := strconv.Atoi(rec.Header().Get("Retry-After"))
		if retry < int(loginBlockDuration.Seconds())-1 || retry > int(loginBlockDuration.Seconds()) {
			t.Fatalf("Retry-After = %d, want about %d", retry, int(loginBlockDuration.Seconds()))
		}

		// Another client is not affected.
		if rec := login(r, "198.51.100.2:1234"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("other client: status = %d, want 401", rec.Code)
		}

		// The block lifts after loginBlockDuration.
		time.Sleep(loginBlockDuration + time.Second)
		status.Store(http.StatusOK)
		if rec := login(r, "198.51.100.1:1234"); rec.Code != http.StatusOK {
			t.Fatalf("after the block: status = %d, want 200", rec.Code)
		}
	})
}

func TestLoginRateLimitCountsOnlyFailures(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rl := NewLoginRateLimiter()
		defer rl.Close()
		var status, calls atomic.Int32
		r := loginRouter(rl, &status, &calls)
		const addr = "198.51.100.1:1234"

		status.Store(http.StatusUnauthorized)
		for range loginMaxFailures - 1 {
			login(r, addr)
		}
		// Bad requests and server errors don't count.
		for _, code := range []int{http.StatusBadRequest, http.StatusInternalServerError} {
			status.Store(int32(code))
			if rec := login(r, addr); rec.Code != code {
				t.Fatalf("status = %d, want %d", rec.Code, code)
			}
		}
		// A success resets the counter, so the next failures start from zero.
		status.Store(http.StatusOK)
		login(r, addr)
		status.Store(http.StatusUnauthorized)
		for i := range loginMaxFailures {
			if rec := login(r, addr); rec.Code != http.StatusUnauthorized {
				t.Fatalf("failure %d after a success: status = %d, want 401", i+1, rec.Code)
			}
		}
	})
}

func TestLoginRateLimitHoldsUnderParallelBurst(t *testing.T) {
	rl := NewLoginRateLimiter()
	defer rl.Close()

	// Handlers block until released, as a slow bcrypt check would, so every
	// request is in flight at the same time.
	release := make(chan struct{})
	var calls atomic.Int32
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login", LoginRateLimit(rl), func(c *gin.Context) {
		calls.Add(1)
		<-release
		c.Status(http.StatusUnauthorized)
	})

	const burst = 50
	var wg sync.WaitGroup
	codes := make(chan int, burst)
	for range burst {
		wg.Go(func() { codes <- login(r, "198.51.100.1:1234").Code })
	}
	// Wait until the reservations are taken, then let the handlers finish.
	deadline := time.Now().Add(5 * time.Second)
	for calls.Load() < loginMaxFailures && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	close(codes)

	if got := calls.Load(); got != loginMaxFailures {
		t.Fatalf("%d of %d parallel attempts reached the password check, want %d", got, burst, loginMaxFailures)
	}
	refused := 0
	for code := range codes {
		if code == http.StatusTooManyRequests {
			refused++
		}
	}
	if refused != burst-loginMaxFailures {
		t.Fatalf("refused %d attempts, want %d", refused, burst-loginMaxFailures)
	}
}

func TestLoginRateLimitGroupsIPv6ByPrefix(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rl := NewLoginRateLimiter()
		defer rl.Close()
		var status, calls atomic.Int32
		status.Store(http.StatusUnauthorized)
		r := loginRouter(rl, &status, &calls)

		// Every attempt from a different address in the same /64.
		for i := range loginMaxFailures {
			login(r, fmt.Sprintf("[2001:db8:1:2::%x]:1234", i+1))
		}
		if rec := login(r, "[2001:db8:1:2::ffff]:1234"); rec.Code != http.StatusTooManyRequests {
			t.Fatalf("rotating addresses inside one /64: status = %d, want 429", rec.Code)
		}
		if rec := login(r, "[2001:db8:1:3::1]:1234"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("another /64: status = %d, want 401", rec.Code)
		}
	})
}

func TestLoginRateLimiterBoundsMemory(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rl := NewLoginRateLimiter()
		defer rl.Close()
		for i := range loginMaxEntries {
			key := fmt.Sprintf("10.%d.%d.%d", i>>16&0xff, i>>8&0xff, i&0xff)
			if secs := rl.begin(key); secs != 0 {
				t.Fatalf("entry %d refused before the limit", i)
			}
			rl.finish(key, http.StatusUnauthorized)
		}
		if secs := rl.begin("192.0.2.1"); secs == 0 {
			t.Fatal("a new client was accepted with the table full")
		}

		// Entries expire, and new clients are accepted again.
		time.Sleep(loginWindowDuration + time.Second)
		if secs := rl.begin("192.0.2.1"); secs != 0 {
			t.Fatalf("after expiry: refused for %ds", secs)
		}
	})
}

func TestAPIRateLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rl := NewAPIRateLimiter()
		defer rl.Close()
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.GET("/nodes", APIRateLimit(rl), func(c *gin.Context) { c.Status(http.StatusOK) })
		get := func(addr string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodGet, "/nodes", nil)
			req.RemoteAddr = addr
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			return rec
		}

		for i := range apiMaxRequests {
			if rec := get("198.51.100.1:1"); rec.Code != http.StatusOK {
				t.Fatalf("request %d: status = %d, want 200", i+1, rec.Code)
			}
		}
		rec := get("198.51.100.1:1")
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d: status = %d, want 429", apiMaxRequests+1, rec.Code)
		}
		if retry, _ := strconv.Atoi(rec.Header().Get("Retry-After")); retry < 1 || retry > 60 {
			t.Fatalf("Retry-After = %d, want 1..60", retry)
		}
		if rec := get("198.51.100.2:1"); rec.Code != http.StatusOK {
			t.Fatalf("other client: status = %d, want 200", rec.Code)
		}

		time.Sleep(apiWindowDuration + time.Second)
		if rec := get("198.51.100.1:1"); rec.Code != http.StatusOK {
			t.Fatalf("next window: status = %d, want 200", rec.Code)
		}
	})
}

func TestLoginRateLimitIgnoresSpoofedForwardedFor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rl := NewLoginRateLimiter()
		defer rl.Close()
		var status, calls atomic.Int32
		status.Store(http.StatusUnauthorized)
		r := loginRouter(rl, &status, &calls)
		if err := r.SetTrustedProxies(nil); err != nil {
			t.Fatal(err)
		}

		for i := range loginMaxFailures {
			req := httptest.NewRequest(http.MethodPost, "/login", nil)
			req.RemoteAddr = "198.51.100.1:1234"
			req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i+1))
			r.ServeHTTP(httptest.NewRecorder(), req)
		}
		if rec := login(r, "198.51.100.1:1234"); rec.Code != http.StatusTooManyRequests {
			t.Fatalf("spoofed X-Forwarded-For without a trusted proxy got its own buckets: status = %d", rec.Code)
		}
	})
}

func TestLoginRateLimitPanicDoesNotResetCounter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rl := NewLoginRateLimiter()
		defer rl.Close()
		var panicNext atomic.Bool
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.Use(gin.CustomRecovery(func(c *gin.Context, _ any) { c.AbortWithStatus(http.StatusInternalServerError) }))
		r.POST("/login", LoginRateLimit(rl), func(c *gin.Context) {
			if panicNext.Load() {
				panic("boom")
			}
			c.Status(http.StatusUnauthorized)
		})

		for range loginMaxFailures - 1 {
			login(r, "198.51.100.1:1234")
		}
		panicNext.Store(true)
		if rec := login(r, "198.51.100.1:1234"); rec.Code != http.StatusInternalServerError {
			t.Fatalf("panicking handler: status = %d, want 500", rec.Code)
		}
		panicNext.Store(false)
		login(r, "198.51.100.1:1234")
		if rec := login(r, "198.51.100.1:1234"); rec.Code != http.StatusTooManyRequests {
			t.Fatalf("a panic reset the failure counter: status = %d, want 429", rec.Code)
		}
	})
}
