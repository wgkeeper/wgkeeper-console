package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/models"
)

// DB is the global GORM database connection.
var DB *gorm.DB

// Init opens a database connection based on DATABASE_URL and runs migrations.
//
// Driver selection:
//   - "postgres://" or "postgresql://" prefix → PostgreSQL (pgx)
//   - "file:<path>" or empty                  → SQLite (default ./wgkeeper-console.db)
func Init() error {
	dbURL := config.Current().Database.URL

	dialector, driver, err := dialectorFromURL(dbURL)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	slog.Info("database", "driver", driver)

	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: gormLogger(),
	})
	if err != nil {
		return fmt.Errorf("database: open: %w", err)
	}

	if err := models.AutoMigrate(DB); err != nil {
		return fmt.Errorf("database: migrate: %w", err)
	}

	if err := widenAPIKeyColumn(DB, driver); err != nil {
		return fmt.Errorf("database: widen api_key: %w", err)
	}

	cfg := config.Current()
	if err := models.EnsureDefaultAdmin(
		DB,
		cfg.Bootstrap.AdminUsername,
		cfg.Bootstrap.AdminPassword,
	); err != nil {
		return fmt.Errorf("database: seed: %w", err)
	}

	if err := configurePool(DB, driver, cfg.Database); err != nil {
		return fmt.Errorf("database: pool: %w", err)
	}

	return nil
}

// dialectorFromURL returns the GORM dialector and a human-readable driver name
// inferred from the URL scheme.
func dialectorFromURL(dbURL string) (gorm.Dialector, string, error) {
	if isPostgresURL(dbURL) {
		// PreferSimpleProtocol disables pgx prepared-statement caching, which
		// otherwise causes "prepared statement already exists" errors on restart
		// and makes HasTable unreliable (leading to spurious "relation already exists").
		dialector := postgres.New(postgres.Config{
			DSN:                  dbURL,
			PreferSimpleProtocol: true,
		})
		return dialector, "postgres", nil
	}

	dsn, err := sqliteDSN(dbURL)
	if err != nil {
		return nil, "", fmt.Errorf("sqlite: %w", err)
	}
	return sqlite.Open(dsn), "sqlite", nil
}

// gormLogger returns Silent logger in production and Info logger when DEBUG=true.
func gormLogger() logger.Interface {
	if config.Current().Debug {
		return logger.Default.LogMode(logger.Info)
	}
	return logger.Default.LogMode(logger.Silent)
}

// isPostgresURL reports whether the URL targets a PostgreSQL server.
func isPostgresURL(url string) bool {
	return strings.HasPrefix(url, "postgres://") ||
		strings.HasPrefix(url, "postgresql://")
}

// sqliteDSN strips the optional "file:" prefix from a SQLite URL and ensures
// the parent directory exists.
func sqliteDSN(dbURL string) (string, error) {
	dsn := strings.TrimPrefix(dbURL, "file:")
	dir := filepath.Dir(dsn)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("create dir %q: %w", dir, err)
		}
	}
	return dsn, nil
}

// widenAPIKeyColumn alters api_key to TEXT in PostgreSQL if it was previously
// created as varchar(255). SQLite is dynamically typed and needs no alteration.
func widenAPIKeyColumn(db *gorm.DB, driver string) error {
	if driver != "postgres" {
		return nil
	}
	return db.Exec("ALTER TABLE nodes ALTER COLUMN api_key TYPE text").Error
}

func configurePool(db *gorm.DB, driver string, cfg config.DatabaseConfig) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	applyPoolConfig(sqlDB, driver, cfg)
	return nil
}

func applyPoolConfig(sqlDB *sql.DB, driver string, cfg config.DatabaseConfig) {
	maxOpen := cfg.MaxOpenConns
	maxIdle := cfg.MaxIdleConns

	if driver == "sqlite" {
		maxOpen = 1
		maxIdle = 1
	}

	if maxOpen > 0 {
		sqlDB.SetMaxOpenConns(maxOpen)
	}
	if maxIdle >= 0 {
		sqlDB.SetMaxIdleConns(maxIdle)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
}
