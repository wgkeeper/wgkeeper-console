package middleware

import (
	"net/http"
	"net/netip"
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
	// loginMaxEntries bounds the memory the limiter can use. When it is full,
	// logins from new addresses are refused until old entries expire.
	loginMaxEntries = 10000
)

// clientKey is the bucket a client IP is counted in. An IPv6 client usually
// controls a whole /64, so counting each address separately would hand it an
// unlimited number of buckets.
func clientKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	addr = addr.Unmap()
	if addr.Is4() {
		return addr.String()
	}
	prefix, err := addr.Prefix(64)
	if err != nil {
		return ip
	}
	return prefix.String()
}

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
	stop    chan struct{}
	once    sync.Once
}

func NewAPIRateLimiter() *APIRateLimiter {
	rl := &APIRateLimiter{entries: make(map[string]*apiEntry), stop: make(chan struct{})}
	go rl.cleanupLoop()
	return rl
}

// Close stops the background cleanup.
func (rl *APIRateLimiter) Close() {
	rl.once.Do(func() { close(rl.stop) })
}

func (rl *APIRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(apiCleanupPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stop:
			return
		}
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
	secs := max(int(time.Until(e.windowStart.Add(apiWindowDuration)).Seconds()), 1)
	return false, secs
}

// APIRateLimit returns a Gin middleware that limits each IP to apiMaxRequests
// per apiWindowDuration across all protected routes.
func APIRateLimit(rl *APIRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed, retryAfter := rl.allow(clientKey(c.ClientIP()))
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
	pending      int // attempts that passed begin and have not finished yet
	windowStart  time.Time
	blockedUntil time.Time
}

// LoginRateLimiter tracks failed login attempts per client using a fixed
// window. After loginMaxFailures failures within loginWindowDuration, the
// client is blocked for loginBlockDuration. An attempt reserves its slot before
// the password is checked, so a burst of parallel requests cannot run more than
// loginMaxFailures guesses. All state is in-memory and does not survive restarts.
type LoginRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*loginEntry
	stop    chan struct{}
	once    sync.Once
}

func NewLoginRateLimiter() *LoginRateLimiter {
	rl := &LoginRateLimiter{
		entries: make(map[string]*loginEntry),
		stop:    make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Close stops the background cleanup.
func (rl *LoginRateLimiter) Close() {
	rl.once.Do(func() { close(rl.stop) })
}

func (rl *LoginRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(loginCleanupPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			rl.cleanupLocked(time.Now())
			rl.mu.Unlock()
		case <-rl.stop:
			return
		}
	}
}

func (rl *LoginRateLimiter) cleanupLocked(now time.Time) {
	for key, e := range rl.entries {
		windowExpired := now.After(e.windowStart.Add(loginWindowDuration))
		blockExpired := e.blockedUntil.IsZero() || now.After(e.blockedUntil)
		if windowExpired && blockExpired && e.pending == 0 {
			delete(rl.entries, key)
		}
	}
}

// begin reserves a login attempt for key. It returns 0 when the attempt may
// go ahead, or the number of seconds to wait before retrying.
func (rl *LoginRateLimiter) begin(key string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()

	e, ok := rl.entries[key]
	if !ok {
		if len(rl.entries) >= loginMaxEntries {
			rl.cleanupLocked(now)
			if len(rl.entries) >= loginMaxEntries {
				return int(loginCleanupPeriod.Seconds())
			}
		}
		e = &loginEntry{windowStart: now}
		rl.entries[key] = e
	}

	if !e.blockedUntil.IsZero() {
		if now.Before(e.blockedUntil) {
			return max(int(time.Until(e.blockedUntil).Seconds()), 1)
		}
		e.blockedUntil = time.Time{}
		e.failures = 0
		e.windowStart = now
	}
	if now.After(e.windowStart.Add(loginWindowDuration)) {
		e.failures = 0
		e.windowStart = now
	}
	// Attempts still in flight count as failures until they finish, so the
	// limit holds even when many requests arrive at once.
	if e.failures+e.pending >= loginMaxFailures {
		return 1
	}
	e.pending++
	return 0
}

// finish records the outcome of an attempt reserved by begin.
func (rl *LoginRateLimiter) finish(key string, status int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	e, ok := rl.entries[key]
	if !ok {
		return
	}
	e.pending = max(e.pending-1, 0)
	switch status {
	case http.StatusOK:
		if e.pending == 0 {
			delete(rl.entries, key)
			return
		}
		e.failures = 0
	case http.StatusUnauthorized:
		e.failures++
		if e.failures >= loginMaxFailures {
			e.blockedUntil = time.Now().Add(loginBlockDuration)
		}
	}
}

// LoginRateLimit returns a Gin middleware that should wrap the login handler.
// It refuses clients that have used up their attempts and records the outcome
// of each login based on the handler's response status: 401 counts as a
// failure, 200 resets the counter, anything else (400, 500) is not counted.
func LoginRateLimit(rl *LoginRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := clientKey(c.ClientIP())
		if secs := rl.begin(key); secs > 0 {
			c.Header("Retry-After", strconv.Itoa(secs))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"ok": false, "error": "too_many_requests"})
			return
		}
		defer func() {
			// A handler that panicked never set its status; don't let the
			// default 200 reset the counter. Re-panic for gin.Recovery.
			if r := recover(); r != nil {
				rl.finish(key, 0)
				panic(r)
			}
			rl.finish(key, c.Writer.Status())
		}()
		c.Next()
	}
}
