package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/crypto"
)

func loadSecretConfig(t *testing.T, secretKey string) {
	t.Helper()
	loadSecretConfigTB(t, secretKey)
}

func loadSecretConfigTB(t testing.TB, secretKey string) {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("SECRET_KEY", secretKey)
	if _, err := config.Load(); err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
}

func openSecretTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}

// rawAPIKey reads the column as stored, bypassing SecretString.
func rawAPIKey(t *testing.T, db *gorm.DB, id string) string {
	t.Helper()
	var raw string
	if err := db.Raw("SELECT api_key FROM nodes WHERE id = ?", id).Scan(&raw).Error; err != nil {
		t.Fatal(err)
	}
	return raw
}

var testKey = strings.Repeat("cd", 32)

func TestSecretStringEncryptsOnEveryWrite(t *testing.T) {
	loadSecretConfig(t, testKey)
	db := openSecretTestDB(t)

	node := Node{Name: "n1", Address: "https://n1.example", APIKey: "plain-key", Status: NodeStatusOnline}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	if node.APIKey != "plain-key" {
		t.Fatalf("Create changed the in-memory key to %q", node.APIKey)
	}
	created := rawAPIKey(t, db, node.ID)
	if !crypto.IsEncrypted(created) || strings.Contains(created, "plain-key") {
		t.Fatalf("Create stored %q, want ciphertext", created)
	}

	var loaded Node
	if err := db.First(&loaded, "id = ?", node.ID).Error; err != nil {
		t.Fatal(err)
	}
	if loaded.APIKey != "plain-key" {
		t.Fatalf("read back %q, want plain-key", loaded.APIKey)
	}

	t.Run("Save", func(t *testing.T) {
		loaded.APIKey = "rotated-key"
		if err := db.Save(&loaded).Error; err != nil {
			t.Fatal(err)
		}
		if raw := rawAPIKey(t, db, node.ID); !crypto.IsEncrypted(raw) || strings.Contains(raw, "rotated-key") {
			t.Fatalf("Save stored %q, want ciphertext", raw)
		}
	})

	t.Run("Updates", func(t *testing.T) {
		if err := db.Model(&Node{}).Where("id = ?", node.ID).Updates(map[string]any{"api_key": SecretString("map-key")}).Error; err != nil {
			t.Fatal(err)
		}
		if raw := rawAPIKey(t, db, node.ID); !crypto.IsEncrypted(raw) {
			t.Fatalf("Updates stored %q, want ciphertext", raw)
		}
	})

	t.Run("status update leaves the key alone", func(t *testing.T) {
		before := rawAPIKey(t, db, node.ID)
		if err := db.Model(&loaded).Updates(map[string]any{"status": NodeStatusOffline}).Error; err != nil {
			t.Fatal(err)
		}
		if after := rawAPIKey(t, db, node.ID); after != before {
			t.Fatal("updating the status rewrote the API key")
		}
	})

	t.Run("same key, different ciphertext", func(t *testing.T) {
		other := Node{Name: "n2", Address: "https://n2.example", APIKey: "plain-key", Status: NodeStatusOnline}
		if err := db.Create(&other).Error; err != nil {
			t.Fatal(err)
		}
		if rawAPIKey(t, db, other.ID) == created {
			t.Fatal("two nodes with the same key got the same ciphertext")
		}
	})

	t.Run("never serialized", func(t *testing.T) {
		out, err := json.Marshal(loaded)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(string(out)), "apikey") {
			t.Fatalf("node JSON contains the API key: %s", out)
		}
	})
}

func TestSecretStringReadsLegacyPlaintext(t *testing.T) {
	loadSecretConfig(t, testKey)
	db := openSecretTestDB(t)
	if err := db.Exec("INSERT INTO nodes (id, name, address, api_key, status, created_at, updated_at) VALUES ('legacy', 'old', 'https://old.example', 'legacy-key', 'online', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)").Error; err != nil {
		t.Fatal(err)
	}
	node := Node{Name: "new", Address: "https://new.example", APIKey: "new-key", Status: NodeStatusOnline}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}

	// One plaintext row must not break the whole query.
	var nodes []Node
	if err := db.Order("name").Find(&nodes).Error; err != nil {
		t.Fatalf("Find with a plaintext row: %v", err)
	}
	if len(nodes) != 2 || nodes[1].APIKey != "legacy-key" {
		t.Fatalf("got %+v, want the legacy key read as is", nodes)
	}

	n, err := EncryptLegacySecrets(db)
	if err != nil {
		t.Fatalf("EncryptLegacySecrets() error = %v", err)
	}
	if n != 1 {
		t.Fatalf("encrypted %d rows, want 1", n)
	}
	if raw := rawAPIKey(t, db, "legacy"); !crypto.IsEncrypted(raw) {
		t.Fatalf("legacy key still stored as %q", raw)
	}
	var legacy Node
	if err := db.First(&legacy, "id = ?", "legacy").Error; err != nil || legacy.APIKey != "legacy-key" {
		t.Fatalf("after migration: key %q, err %v", legacy.APIKey, err)
	}
	if n, err := EncryptLegacySecrets(db); err != nil || n != 0 {
		t.Fatalf("second run encrypted %d rows (err %v), want 0", n, err)
	}
}

func TestSecretStringWrongKeyFailsLoudly(t *testing.T) {
	loadSecretConfig(t, testKey)
	db := openSecretTestDB(t)
	node := Node{Name: "n1", Address: "https://n1.example", APIKey: "plain-key", Status: NodeStatusOnline}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}

	loadSecretConfig(t, strings.Repeat("ef", 32))
	var loaded Node
	err := db.First(&loaded, "id = ?", node.ID).Error
	if err == nil || !strings.Contains(err.Error(), "SECRET_KEY") {
		t.Fatalf("reading with another SECRET_KEY: error = %v, want a SECRET_KEY hint", err)
	}
}
