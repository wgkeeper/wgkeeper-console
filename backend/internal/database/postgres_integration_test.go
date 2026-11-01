//go:build integration

package database

import (
	"os"
	"testing"

	"wg-keeper-backend/internal/crypto"
	"wg-keeper-backend/internal/models"
)

// TestPostgres runs against a real PostgreSQL server:
//
//	WGKEEPER_TEST_POSTGRES_URL=postgres://... go test -tags=integration ./internal/database/
//
// It covers what SQLite can't: the api_key column migration, the pgx simple
// protocol across restarts, and encrypted keys stored in a text column.
func TestPostgres(t *testing.T) {
	dbURL := os.Getenv("WGKEEPER_TEST_POSTGRES_URL")
	if dbURL == "" {
		t.Skip("WGKEEPER_TEST_POSTGRES_URL is not set")
	}
	loadDBConfig(t, dbURL)
	oldDB := DB
	t.Cleanup(func() { DB = oldDB })

	if err := Init(); err != nil {
		t.Fatalf("first Init() error = %v", err)
	}
	if err := DB.Exec("DELETE FROM nodes").Error; err != nil {
		t.Fatal(err)
	}
	node := models.Node{Name: "pg", Address: "https://pg.example", APIKey: "pg-node-key", Status: models.NodeStatusOnline}
	if err := DB.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	if err := Close(); err != nil {
		t.Fatal(err)
	}

	// A restart: migrations and the column widening run again on a live schema.
	if err := Init(); err != nil {
		t.Fatalf("Init() after restart error = %v", err)
	}
	t.Cleanup(func() { _ = Close() })

	var dataType string
	if err := DB.Raw(`SELECT data_type FROM information_schema.columns WHERE table_name = 'nodes' AND column_name = 'api_key'`).Scan(&dataType).Error; err != nil {
		t.Fatal(err)
	}
	if dataType != "text" {
		t.Fatalf("api_key column is %q, want text", dataType)
	}

	var raw string
	if err := DB.Raw("SELECT api_key FROM nodes WHERE id = ?", node.ID).Scan(&raw).Error; err != nil {
		t.Fatal(err)
	}
	if !crypto.IsEncrypted(raw) {
		t.Fatalf("api_key stored as %q, want ciphertext", raw)
	}
	var loaded models.Node
	if err := DB.First(&loaded, "id = ?", node.ID).Error; err != nil || loaded.APIKey != "pg-node-key" {
		t.Fatalf("node after restart: key %q, err %v", loaded.APIKey, err)
	}

	duplicate := models.Node{Name: "pg2", Address: node.Address, APIKey: "k", Status: models.NodeStatusOnline}
	if err := DB.Create(&duplicate).Error; err == nil {
		t.Fatal("a second node with the same address was stored")
	}
}
