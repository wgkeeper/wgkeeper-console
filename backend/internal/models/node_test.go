package models

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEnsureDefaultAdminRequiresPassword(t *testing.T) {
	db := openTestDB(t)

	if err := EnsureDefaultAdmin(db, "admin", ""); err == nil {
		t.Fatalf("expected error when bootstrap password is missing")
	}
}

func TestEnsureDefaultAdminCreatesConfiguredAdmin(t *testing.T) {
	db := openTestDB(t)

	if err := EnsureDefaultAdmin(db, "root", "super-secret-password"); err != nil {
		t.Fatalf("EnsureDefaultAdmin() error = %v", err)
	}

	var user User
	if err := db.Where("username = ?", "root").First(&user).Error; err != nil {
		t.Fatalf("failed to load created admin: %v", err)
	}
	if !user.IsAdmin {
		t.Fatalf("expected seeded user to be admin")
	}
	if !user.MustChangePassword {
		t.Fatalf("expected seeded user to require password change")
	}
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}
