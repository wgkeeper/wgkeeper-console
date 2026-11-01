package models

import (
	"database/sql/driver"
	"fmt"

	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/crypto"
)

// SecretString is a string column encrypted at rest with the key derived from
// SECRET_KEY. It is encrypted on every write (Create, Save, Updates) and
// decrypted on read, so the value in memory is always plaintext.
//
// A value written while no key was configured is plaintext in the database;
// it is read back as is, and EncryptLegacySecrets encrypts it once a key is set.
type SecretString string

// Value implements driver.Valuer.
func (s SecretString) Value() (driver.Value, error) {
	key := config.Current().APIKeyEncryptionKey
	if len(key) == 0 {
		return string(s), nil
	}
	return crypto.Encrypt(string(s), key)
}

// Scan implements sql.Scanner.
func (s *SecretString) Scan(src any) error {
	var raw string
	switch v := src.(type) {
	case string:
		raw = v
	case []byte:
		raw = string(v)
	case nil:
		*s = ""
		return nil
	default:
		return fmt.Errorf("secret string: unsupported type %T", src)
	}

	if !crypto.IsEncrypted(raw) {
		*s = SecretString(raw)
		return nil
	}
	key := config.Current().APIKeyEncryptionKey
	if len(key) == 0 {
		return fmt.Errorf("secret string: value is encrypted but SECRET_KEY is not set")
	}
	plain, err := crypto.Decrypt(raw, key)
	if err != nil {
		return fmt.Errorf("secret string: %w (was SECRET_KEY changed?)", err)
	}
	*s = SecretString(plain)
	return nil
}
