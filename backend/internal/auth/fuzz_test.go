package auth

import (
	"strings"
	"testing"
	"time"

	"wg-keeper-backend/internal/models"
)

// FuzzVerifyToken feeds arbitrary cookie values to the session verifier. It
// must never panic, and anything it accepts must be a token this secret
// actually signed, for a named user, that has not expired.
func FuzzVerifyToken(f *testing.F) {
	loadTestConfig(f, testSecretKey)
	valid, _, err := CreateToken(models.User{ID: "user-1", SessionVersion: 2})
	if err != nil {
		f.Fatal(err)
	}
	payload, sig, _ := strings.Cut(valid, ".")
	for _, seed := range []string{
		valid,
		payload + "." + strings.ToUpper(sig),
		payload + ".",
		"." + sig,
		valid + ".extra",
		"",
		".",
		"e30." + sig, // {} with a foreign signature
		strings.Repeat("A", 4096),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, token string) {
		claims, err := VerifyToken(token)
		if err != nil {
			return
		}
		encoded, gotSig, ok := strings.Cut(token, ".")
		if !ok || gotSig != signPayload(encoded, loadedSecret()) {
			t.Fatalf("accepted a token that this secret did not sign: %q", token)
		}
		if claims.UserID == "" {
			t.Fatalf("accepted a token without a subject: %q", token)
		}
		if time.Now().Unix() > claims.ExpiresAt {
			t.Fatalf("accepted an expired token: %+v", claims)
		}
	})
}
