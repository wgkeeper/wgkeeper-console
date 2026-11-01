package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/models"
)

const testSecretKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

func loadTestConfig(t *testing.T, secret string) {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("SECRET_KEY", secret)
	t.Setenv("SESSION_TTL", "24h")
	if _, err := config.Load(); err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
}

// signClaims builds a correctly signed token from arbitrary claims, so tests
// can reach the checks that run after the signature.
func signClaims(t *testing.T, claims any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return encoded + "." + signPayload(encoded, config.Current().Auth.SessionSecret)
}

func TestCreateAndVerifyToken(t *testing.T) {
	loadTestConfig(t, testSecretKey)
	user := models.User{ID: "user-1", SessionVersion: 3}

	token, expires, err := CreateToken(user)
	if err != nil {
		t.Fatalf("CreateToken() error = %v", err)
	}
	claims, err := VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	if claims.UserID != "user-1" || claims.Version != 3 {
		t.Fatalf("claims = %+v, want sub user-1 and ver 3", claims)
	}
	if claims.AuthTime != claims.IssuedAt {
		t.Fatalf("a new session must start at issue time: auth=%d iat=%d", claims.AuthTime, claims.IssuedAt)
	}
	if got := time.Unix(claims.ExpiresAt, 0); !got.Equal(expires.Truncate(time.Second)) {
		t.Fatalf("returned expiry %v does not match token expiry %v", expires, got)
	}
}

func TestVerifyTokenRejects(t *testing.T) {
	loadTestConfig(t, testSecretKey)
	now := time.Now()
	valid, _, err := CreateToken(models.User{ID: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	payload, sig, _ := strings.Cut(valid, ".")

	flipped := []byte(sig)
	if flipped[0] == 'a' {
		flipped[0] = 'b'
	} else {
		flipped[0] = 'a'
	}

	otherPayload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"admin-2","iat":1,"exp":9999999999,"auth":1}`))

	tests := []struct {
		name    string
		token   string
		wantErr error
	}{
		{"empty", "", ErrInvalidToken},
		{"only dot", ".", ErrInvalidToken},
		{"no dot", "abc", ErrInvalidToken},
		{"extra part", valid + ".x", ErrInvalidToken},
		{"payload swapped, old signature", otherPayload + "." + sig, ErrInvalidToken},
		{"signature flipped", payload + "." + string(flipped), ErrInvalidToken},
		{"signature upper-cased", payload + "." + strings.ToUpper(sig), ErrInvalidToken},
		{"signed payload not base64", "!!!." + signPayload("!!!", config.Current().Auth.SessionSecret), ErrInvalidToken},
		{"signed payload not JSON", signedRaw("not json"), ErrInvalidToken},
		{"no subject", signClaims(t, TokenClaims{IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Hour).Unix(), AuthTime: now.Unix()}), ErrInvalidToken},
		{"expired", signClaims(t, TokenClaims{UserID: "u", IssuedAt: now.Add(-2 * time.Hour).Unix(), ExpiresAt: now.Add(-time.Second).Unix(), AuthTime: now.Add(-2 * time.Hour).Unix()}), ErrTokenExpired},
		{"session older than the maximum age", signClaims(t, TokenClaims{UserID: "u", IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Hour).Unix(), AuthTime: now.Add(-MaxSessionAge - time.Minute).Unix()}), ErrTokenExpired},
		{"token from before login times", signClaims(t, map[string]any{"sub": "u", "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()}), ErrTokenExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := VerifyToken(tt.token); !errors.Is(err, tt.wantErr) {
				t.Fatalf("VerifyToken() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func signedRaw(payload string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return encoded + "." + signPayload(encoded, config.Current().Auth.SessionSecret)
}

func TestVerifyTokenRejectsOtherSecret(t *testing.T) {
	loadTestConfig(t, testSecretKey)
	token, _, err := CreateToken(models.User{ID: "user-1"})
	if err != nil {
		t.Fatal(err)
	}

	loadTestConfig(t, strings.Repeat("ff", 32))
	if _, err := VerifyToken(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("token signed with another SECRET_KEY: error = %v, want ErrInvalidToken", err)
	}
}

func TestShouldRefresh(t *testing.T) {
	loadTestConfig(t, testSecretKey)
	ttl := 24 * time.Hour
	now := time.Now()

	tests := []struct {
		name   string
		claims TokenClaims
		want   bool
	}{
		{"fresh token", TokenClaims{ExpiresAt: now.Add(ttl).Unix(), AuthTime: now.Unix()}, false},
		{"more than half left", TokenClaims{ExpiresAt: now.Add(ttl/2 + time.Minute).Unix(), AuthTime: now.Unix()}, false},
		{"less than half left", TokenClaims{ExpiresAt: now.Add(ttl/2 - time.Minute).Unix(), AuthTime: now.Add(-ttl).Unix()}, true},
		{"already at the maximum age", TokenClaims{ExpiresAt: now.Add(time.Hour).Unix(), AuthTime: now.Add(-MaxSessionAge + time.Hour).Unix()}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldRefresh(&tt.claims); got != tt.want {
				t.Fatalf("ShouldRefresh() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRefreshTokenKeepsLoginTimeAndCapsExpiry(t *testing.T) {
	loadTestConfig(t, testSecretKey)
	now := time.Now()
	user := models.User{ID: "user-1", SessionVersion: 1}

	t.Run("keeps the original login time", func(t *testing.T) {
		old := &TokenClaims{UserID: user.ID, AuthTime: now.Add(-48 * time.Hour).Unix()}
		token, _, err := RefreshToken(user, old)
		if err != nil {
			t.Fatal(err)
		}
		claims, err := VerifyToken(token)
		if err != nil {
			t.Fatal(err)
		}
		if claims.AuthTime != old.AuthTime {
			t.Fatalf("AuthTime = %d, want %d", claims.AuthTime, old.AuthTime)
		}
		if claims.Version != 1 {
			t.Fatalf("Version = %d, want the user's current version 1", claims.Version)
		}
	})

	t.Run("never expires past the maximum age", func(t *testing.T) {
		authTime := now.Add(-MaxSessionAge + time.Hour)
		_, expires, err := RefreshToken(user, &TokenClaims{UserID: user.ID, AuthTime: authTime.Unix()})
		if err != nil {
			t.Fatal(err)
		}
		limit := authTime.Add(MaxSessionAge)
		if expires.After(limit) {
			t.Fatalf("expiry %v is past the session limit %v", expires, limit)
		}
	})
}
