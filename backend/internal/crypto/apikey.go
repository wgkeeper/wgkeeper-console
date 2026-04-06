package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

const encryptedPrefix = "enc:"

// Encrypt encrypts plaintext with AES-256-GCM using a random nonce.
// The returned string has the form "enc:<hex(nonce || ciphertext)>".
// key must be exactly 32 bytes.
func Encrypt(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("encrypt: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("encrypt: new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("encrypt: generate nonce: %w", err)
	}
	// Seal appends ciphertext + auth tag to nonce, producing nonce||ciphertext.
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encryptedPrefix + hex.EncodeToString(sealed), nil
}

// Decrypt decrypts a value produced by Encrypt.
// Returns an error if value does not start with the "enc:" prefix.
// key must be exactly 32 bytes.
func Decrypt(value string, key []byte) (string, error) {
	if !strings.HasPrefix(value, encryptedPrefix) {
		return "", errors.New("decrypt: value is not encrypted (missing enc: prefix)")
	}
	data, err := hex.DecodeString(strings.TrimPrefix(value, encryptedPrefix))
	if err != nil {
		return "", fmt.Errorf("decrypt: decode hex: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("decrypt: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("decrypt: new gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("decrypt: ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: open: %w", err)
	}
	return string(plaintext), nil
}
