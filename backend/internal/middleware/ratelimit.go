package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	loginWindowDuration = 15 * time.Minute
	loginMaxFailures    = 10
	loginBlockDuration  = 15 * time.Minute
	loginCleanupPeriod  = 5 * time.Minute
)

// API rate limiter constants: 120 requests per minute per IP.
const (
	apiWindowDuration = time.Minute
	apiMaxRequests    = 120
	apiCleanupPeriod  = 5 * time.Minute
)

type apiEntry struct {
	count       int
	windowStart time.Time
}

// APIRateLimiter enforces a per-IP fixed-window request limit on protected
// API endpoints. All state is in-memory and resets on restart.
type APIRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*apiEntry
}

func NewAPIRateLimiter() *APIRateLimiter {
	rl := &APIRateLimiter{entries: make(map[string]*apiEntry)}
	go rl.cleanupLoop()
	return rl
}

func (rl *APIRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(apiCleanupPeriod)
	defer ticker.Stop()
	for range ticker.C {
		rl.cleanup()
	}
}

func (rl *APIRateLimiter) cleanup() {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for ip, e := range rl.entries {
		if now.After(e.windowStart.Add(apiWindowDuration)) {
			delete(rl.entries, ip)
		}
	}
}

// allow returns whether the request is permitted. When denied, the second
// return value is the seconds remaining until the IP's current fixed window
// resets — used to populate a precise Retry-After header.
func (rl *APIRateLimiter) allow(ip string) (bool, int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	e, ok := rl.entries[ip]
	if !ok || now.After(e.windowStart.Add(apiWindowDuration)) {
		rl.entries[ip] = &apiEntry{count: 1, windowStart: now}
		return true, 0
	}
	e.count++
	if e.count <= apiMaxRequests {
		return true, 0
	}
	secs := int(time.Until(e.windowStart.Add(apiWindowDuration)).Seconds())
	if secs < 1 {
		secs = 1
	}
	return false, secs
}

// APIRateLimit returns a Gin middleware that limits each IP to apiMaxRequests
// per apiWindowDuration across all protected routes.
func APIRateLimit(rl *APIRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed, retryAfter := rl.allow(c.ClientIP())
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.JSON(http.StatusTooManyRequests, gin.H{"ok": false, "error": "too_many_requests"})
			c.Abort()
			return
		}
		c.Next()
	}
}

type loginEntry struct {
	failures     int
	windowStart  time.Time
	blockedUntil time.Time
}

// LoginRateLimiter tracks failed login attempts per IP using a fixed window.
// After loginMaxFailures failures within loginWindowDuration, the IP is blocked
// for loginBlockDuration. All state is in-memory and does not persist restarts.
type LoginRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*loginEntry
}

func NewLoginRateLimiter() *LoginRateLimiter {
	rl := &LoginRateLimiter{
		entries: make(map[string]*loginEntry),
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *LoginRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(loginCleanupPeriod)
	defer ticker.Stop()
	for range ticker.C {
		rl.cleanup()
	}
}

func (rl *LoginRateLimiter) cleanup() {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for ip, e := range rl.entries {
		windowExpired := now.After(e.windowStart.Add(loginWindowDuration))
		blockExpired := e.blockedUntil.IsZero() || now.After(e.blockedUntil)
		if windowExpired && blockExpired {
			delete(rl.entries, ip)
		}
	}
}

// blockedFor returns how many seconds the IP is still blocked, or 0 if not blocked.
func (rl *LoginRateLimiter) blockedFor(ip string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	e, ok := rl.entries[ip]
	if !ok {
		return 0
	}
	if e.blockedUntil.IsZero() {
		return 0
	}
	secs := int(time.Until(e.blockedUntil).Seconds())
	if secs <= 0 {
		return 0
	}
	return secs
}

func (rl *LoginRateLimiter) recordFailure(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	e, ok := rl.entries[ip]
	if !ok {
		e = &loginEntry{windowStart: now}
		rl.entries[ip] = e
	}
	// Reset window if expired
	if now.After(e.windowStart.Add(loginWindowDuration)) {
		e.failures = 0
		e.windowStart = now
		e.blockedUntil = time.Time{}
	}
	e.failures++
	if e.failures >= loginMaxFailures {
		e.blockedUntil = now.Add(loginBlockDuration)
	}
}

func (rl *LoginRateLimiter) recordSuccess(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.entries, ip)
}

// LoginRateLimit returns a Gin middleware that should wrap the login handler.
// It blocks IPs that have exceeded the failure threshold, and records the
// outcome of each login attempt based on the handler's response status.
func LoginRateLimit(rl *LoginRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if secs := rl.blockedFor(ip); secs > 0 {
			c.Header("Retry-After", strconv.Itoa(secs))
			c.JSON(http.StatusTooManyRequests, gin.H{"ok": false, "error": "too_many_requests"})
			c.Abort()
			return
		}

		c.Next()

		switch c.Writer.Status() {
		case http.StatusOK:
			rl.recordSuccess(ip)
		case http.StatusUnauthorized:
			rl.recordFailure(ip)
		}
	}
}
