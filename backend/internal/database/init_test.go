package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/models"
)

func loadDBConfig(t *testing.T, dbURL string) {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("SECRET_KEY", strings.Repeat("ef", 32))
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "bootstrap-password")
	t.Setenv("DATABASE_URL", dbURL)
	if _, err := config.Load(); err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
}

// TestInitSQLiteSurvivesRestart opens a fresh SQLite file, stores a node, then
// opens it again the way a restart does.
func TestInitSQLiteSurvivesRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	loadDBConfig(t, "file:"+filepath.Join(dir, "console.db"))
	oldDB := DB
	t.Cleanup(func() { DB = oldDB })

	if err := Init(); err != nil {
		t.Fatalf("first Init() error = %v", err)
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o750 {
		t.Fatalf("database directory: %v, mode %v, want 0750", err, info.Mode().Perm())
	}
	node := models.Node{Name: "n1", Address: "https://n1.example", APIKey: "node-key", Status: models.NodeStatusOnline}
	if err := DB.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	if err := Close(); err != nil {
		t.Fatal(err)
	}

	if err := Init(); err != nil {
		t.Fatalf("Init() after restart error = %v", err)
	}
	t.Cleanup(func() { _ = Close() })

	var users int64
	if err := DB.Model(&models.User{}).Count(&users).Error; err != nil || users != 1 {
		t.Fatalf("users after restart = %d (err %v), want the one bootstrap admin", users, err)
	}
	var loaded models.Node
	if err := DB.First(&loaded, "id = ?", node.ID).Error; err != nil || loaded.APIKey != "node-key" {
		t.Fatalf("node after restart: key %q, err %v", loaded.APIKey, err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	if got := sqlDB.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("SQLite pool allows %d connections, want 1 writer", got)
	}
}

func TestDialectorFromURL(t *testing.T) {
	tests := []struct {
		url, driver string
	}{
		{"postgres://u:p@db:5432/console", "postgres"},
		{"postgresql://u:p@db/console?sslmode=disable", "postgres"},
		{"file:" + filepath.Join(t.TempDir(), "x.db"), "sqlite"},
	}
	for _, tt := range tests {
		d, driver, err := dialectorFromURL(tt.url)
		if err != nil || d == nil || driver != tt.driver {
			t.Errorf("dialectorFromURL(%q) = (%v, %q, %v), want driver %q", tt.url, d, driver, err, tt.driver)
		}
	}
}

func TestCloseWithoutDatabase(t *testing.T) {
	oldDB := DB
	DB = nil
	t.Cleanup(func() { DB = oldDB })
	if err := Close(); err != nil {
		t.Fatalf("Close() with no database = %v, want nil", err)
	}
}
