package models

import (
	"fmt"

	"gorm.io/gorm"

	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/crypto"
)

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&Node{}, &User{})
}

// EncryptLegacySecrets encrypts node API keys that were stored as plaintext,
// for example by an install that ran without SECRET_KEY before one was set.
// It returns how many rows it encrypted.
func EncryptLegacySecrets(db *gorm.DB) (int, error) {
	if len(config.Current().APIKeyEncryptionKey) == 0 {
		return 0, nil
	}

	var rows []struct {
		ID     string
		APIKey string
	}
	if err := db.Model(&Node{}).Select("id", "api_key").Scan(&rows).Error; err != nil {
		return 0, fmt.Errorf("load api keys: %w", err)
	}

	encrypted := 0
	for _, row := range rows {
		if crypto.IsEncrypted(row.APIKey) {
			continue
		}
		// Updating through SecretString encrypts the value on write.
		if err := db.Model(&Node{}).Where("id = ?", row.ID).
			Update("api_key", SecretString(row.APIKey)).Error; err != nil {
			return encrypted, fmt.Errorf("encrypt api key of node %s: %w", row.ID, err)
		}
		encrypted++
	}
	return encrypted, nil
}
