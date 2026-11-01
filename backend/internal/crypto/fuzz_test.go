package crypto

import (
	"bytes"
	"testing"
)

var fuzzKey = bytes.Repeat([]byte{0x42}, 32)

// FuzzEncryptDecrypt checks the round trip for any plaintext, and that
// changing any single byte of the ciphertext makes decryption fail.
func FuzzEncryptDecrypt(f *testing.F) {
	for _, seed := range []string{"", "node-api-key", "caf\u00e9 \u65e5\u672c", "\x00\xff", string(bytes.Repeat([]byte("k"), 1024))} {
		f.Add(seed, uint16(0))
	}

	f.Fuzz(func(t *testing.T, plaintext string, flip uint16) {
		encrypted, err := Encrypt(plaintext, fuzzKey)
		if err != nil {
			t.Fatalf("Encrypt() error = %v", err)
		}
		if !IsEncrypted(encrypted) {
			t.Fatalf("ciphertext %q has no prefix", encrypted)
		}
		got, err := Decrypt(encrypted, fuzzKey)
		if err != nil || got != plaintext {
			t.Fatalf("round trip: got (%q, %v), want %q", got, err, plaintext)
		}

		// Tamper with one hex character after the prefix.
		body := []byte(encrypted[len(encryptedPrefix):])
		i := int(flip) % len(body)
		if body[i] == '0' {
			body[i] = '1'
		} else {
			body[i] = '0'
		}
		if _, err := Decrypt(encryptedPrefix+string(body), fuzzKey); err == nil {
			t.Fatalf("tampered ciphertext at %d decrypted without error", i)
		}
	})
}

// FuzzDecrypt feeds arbitrary stored values to Decrypt: it must never panic,
// and only values carrying the prefix can decrypt.
func FuzzDecrypt(f *testing.F) {
	valid, err := Encrypt("seed", fuzzKey)
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range []string{valid, "enc:", "enc:zz", "enc:00", "plain-key", "", valid[:len(valid)-2]} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if _, err := Decrypt(value, fuzzKey); err == nil && !IsEncrypted(value) {
			t.Fatalf("decrypted a value without the prefix: %q", value)
		}
	})
}
