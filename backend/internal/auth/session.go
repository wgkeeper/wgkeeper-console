package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/models"
)

const SessionCookieName = "wg_session"

// MaxSessionAge caps how long a session can be kept alive by refreshing,
// counted from the login that started it. A longer SESSION_TTL raises it.
const MaxSessionAge = 7 * 24 * time.Hour

var (
	// ErrInvalidToken means the token is malformed, tampered with or signed
	// with another secret.
	ErrInvalidToken = errors.New("invalid session token")
	// ErrTokenExpired means the token is past its expiry or the session is
	// past MaxSessionAge.
	ErrTokenExpired = errors.New("session token expired")
)

// TokenClaims holds the data embedded in the signed session token.
type TokenClaims struct {
	UserID    string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	// AuthTime is when the user logged in; refreshes keep it unchanged.
	AuthTime int64 `json:"auth"`
	// Version must match the user's SessionVersion. Bumping that column on
	// logout or password change revokes every token issued before it.
	Version int `json:"ver"`
}

// CreateToken signs a token for a new session that starts now and returns it
// with its expiry.
// Token format: base64url(payload) + "." + hex(hmac-sha256(payload, secret))
func CreateToken(user models.User) (string, time.Time, error) {
	return issueToken(user, time.Now())
}

// RefreshToken signs a new token for an existing session. The session keeps
// its original login time, so refreshing never extends it past the maximum age.
func RefreshToken(user models.User, claims *TokenClaims) (string, time.Time, error) {
	return issueToken(user, time.Unix(claims.AuthTime, 0))
}

func issueToken(user models.User, authTime time.Time) (string, time.Time, error) {
	cfg := config.Current()
	ttl := sessionTTL(cfg)

	now := time.Now()
	expires := now.Add(ttl)
	if limit := authTime.Add(maxSessionAge(ttl)); expires.After(limit) {
		expires = limit
	}
	claims := TokenClaims{
		UserID:    user.ID,
		IssuedAt:  now.Unix(),
		ExpiresAt: expires.Unix(),
		AuthTime:  authTime.Unix(),
		Version:   user.SessionVersion,
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}

	encoded := base64.RawURLEncoding.EncodeToString(payload)
	sig := signPayload(encoded, cfg.Auth.SessionSecret)
	return encoded + "." + sig, time.Unix(claims.ExpiresAt, 0), nil
}

// VerifyToken validates the token signature, expiry and session age, then
// returns the claims. It does not check the session version; the caller
// compares it with the user's current SessionVersion.
func VerifyToken(token string) (*TokenClaims, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidToken
	}

	encoded, sig := parts[0], parts[1]
	cfg := config.Current()

	expected := signPayload(encoded, cfg.Auth.SessionSecret)
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return nil, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if claims.UserID == "" {
		return nil, ErrInvalidToken
	}

	now := time.Now()
	if now.Unix() > claims.ExpiresAt {
		return nil, ErrTokenExpired
	}
	// Tokens issued before sessions had a login time carry AuthTime 0 and
	// fail here, which logs those sessions out once.
	if now.After(time.Unix(claims.AuthTime, 0).Add(maxSessionAge(sessionTTL(cfg)))) {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}

// ShouldRefresh returns true when less than half of the session TTL remains
// and refreshing would still extend the session.
func ShouldRefresh(claims *TokenClaims) bool {
	ttl := sessionTTL(config.Current())
	expires := time.Unix(claims.ExpiresAt, 0)
	limit := time.Unix(claims.AuthTime, 0).Add(maxSessionAge(ttl))
	return time.Until(expires) < ttl/2 && expires.Before(limit)
}

func sessionTTL(cfg config.Config) time.Duration {
	if cfg.Auth.SessionTTL <= 0 {
		return 24 * time.Hour
	}
	return cfg.Auth.SessionTTL
}

func maxSessionAge(ttl time.Duration) time.Duration {
	return max(MaxSessionAge, ttl)
}

func signPayload(encoded string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(encoded))
	return hex.EncodeToString(mac.Sum(nil))
}
