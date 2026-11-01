package models

import (
	"testing"
)

// BenchmarkSecretString covers the AES-GCM work done for every node row
// written or read.
func BenchmarkSecretString(b *testing.B) {
	b.Setenv("APP_ENV", "test")
	loadSecretConfigTB(b, testKey)
	secret := SecretString("0123456789abcdef0123456789abcdef")
	stored, err := secret.Value()
	if err != nil {
		b.Fatal(err)
	}

	b.Run("Value", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := secret.Value(); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Scan", func(b *testing.B) {
		b.ReportAllocs()
		var out SecretString
		for b.Loop() {
			if err := out.Scan(stored); err != nil {
				b.Fatal(err)
			}
		}
	})
}
