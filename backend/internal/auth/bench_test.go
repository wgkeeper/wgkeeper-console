package auth

import (
	"testing"

	"wg-keeper-backend/internal/models"
)

func BenchmarkCreateToken(b *testing.B) {
	loadTestConfig(b, testSecretKey)
	user := models.User{ID: "user-1", SessionVersion: 1}
	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := CreateToken(user); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkVerifyToken is on every authenticated request.
func BenchmarkVerifyToken(b *testing.B) {
	loadTestConfig(b, testSecretKey)
	token, _, err := CreateToken(models.User{ID: "user-1", SessionVersion: 1})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := VerifyToken(token); err != nil {
			b.Fatal(err)
		}
	}
}
