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

// TokenClaims holds the data embedded in the signed session token.
type TokenClaims struct {
	UserID    string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// CreateToken signs a new session token for the given user.
// Token format: base64url(payload) + "." + hex(hmac-sha256(payload, secret))
func CreateToken(user models.User) (string, error) {
	cfg := config.Current()
	ttl := cfg.Auth.SessionTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	now := time.Now()
	claims := TokenClaims{
		UserID:    user.ID,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encoded := base64.RawURLEncoding.EncodeToString(payload)
	sig := signPayload(encoded, cfg.Auth.SessionSecret)
	return encoded + "." + sig, nil
}

// VerifyToken validates the token signature and expiry, then returns the claims.
func VerifyToken(token string) (*TokenClaims, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid token format")
	}

	encoded, sig := parts[0], parts[1]
	cfg := config.Current()

	expected := signPayload(encoded, cfg.Auth.SessionSecret)
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return nil, errors.New("invalid token signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("invalid token encoding")
	}

	var claims TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errors.New("invalid token payload")
	}

	if time.Now().Unix() > claims.ExpiresAt {
		return nil, errors.New("token expired")
	}

	return &claims, nil
}

// ShouldRefresh returns true when less than half of the session TTL remains.
func ShouldRefresh(claims *TokenClaims) bool {
	cfg := config.Current()
	ttl := cfg.Auth.SessionTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return time.Until(time.Unix(claims.ExpiresAt, 0)) < ttl/2
}

func signPayload(encoded string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(encoded))
	return hex.EncodeToString(mac.Sum(nil))
}
