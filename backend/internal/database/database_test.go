package database

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"wg-keeper-backend/internal/config"
)

func TestSQLitePoolConfigForcesSingleWriter(t *testing.T) {
	sqlDB := newTestSQLDB(t)
	t.Cleanup(func() { _ = sqlDB.Close() })

	applyPoolConfig(sqlDB, "sqlite", config.DatabaseConfig{
		MaxOpenConns:    20,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: time.Minute,
	})

	stats := sqlDB.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Fatalf("expected sqlite max open conns to be 1, got %d", stats.MaxOpenConnections)
	}
}

func TestIsPostgresURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{input: "postgres://localhost/db", want: true},
		{input: "postgresql://localhost/db", want: true},
		{input: "file:./dev.db", want: false},
	}

	for _, tt := range tests {
		if got := isPostgresURL(tt.input); got != tt.want {
			t.Fatalf("isPostgresURL(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestSQLiteDSN(t *testing.T) {
	dsn, err := sqliteDSN("file:./testdata/dev.db")
	if err != nil {
		t.Fatalf("sqliteDSN() error = %v", err)
	}
	if dsn != "./testdata/dev.db" {
		t.Fatalf("dsn = %q, want ./testdata/dev.db", dsn)
	}
}

func BenchmarkApplyPoolConfig(b *testing.B) {
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		b.Fatalf("sql.Open() error = %v", err)
	}
	b.Cleanup(func() { _ = sqlDB.Close() })

	cfg := config.DatabaseConfig{
		MaxOpenConns:    20,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: time.Minute,
	}

	for i := 0; i < b.N; i++ {
		applyPoolConfig(sqlDB, "postgres", cfg)
	}
}
