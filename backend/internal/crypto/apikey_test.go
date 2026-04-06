package crypto

import (
	"strings"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	plaintext := "my-super-secret-api-key"

	encrypted, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if !strings.HasPrefix(encrypted, "enc:") {
		t.Fatalf("encrypted value missing enc: prefix: %q", encrypted)
	}
	if encrypted == plaintext {
		t.Fatalf("Encrypt() returned unchanged plaintext")
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptProducesUniqueCiphertexts(t *testing.T) {
	key := make([]byte, 32)
	plaintext := "same-plaintext"

	a, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	b, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if a == b {
		t.Fatalf("two Encrypt calls produced identical ciphertext (nonce not random)")
	}
}

func TestDecryptPlaintextFails(t *testing.T) {
	key := make([]byte, 32)
	if _, err := Decrypt("raw-plaintext-no-prefix", key); err == nil {
		t.Fatalf("Decrypt() of plaintext expected error, got nil")
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	keyA := make([]byte, 32)
	keyB := make([]byte, 32)
	for i := range keyB {
		keyB[i] = 0xff
	}

	encrypted, err := Encrypt("secret", keyA)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if _, err := Decrypt(encrypted, keyB); err == nil {
		t.Fatalf("Decrypt() with wrong key expected error, got nil")
	}
}
