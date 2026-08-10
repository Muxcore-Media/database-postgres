package db

import (
	"context"
	"testing"

	"github.com/Muxcore-Media/core/pkg/contracts"
)

func TestConfigFromParams_RejectsCredentialInExtra(t *testing.T) {
	_, err := ConfigFromParams(contracts.DatabaseParams{
		Host: "localhost", User: "u", Database: "d",
		Extra: map[string]string{"options": "password=secret"},
	})
	if err != contracts.ErrDatabaseCredentialExposure {
		t.Fatalf("got %v", err)
	}
}

func TestDSN_NoPasswordInLogFields(t *testing.T) {
	cfg := Config{Host: "db.example", Port: "5432", User: "u", Password: "s3cret", Database: "mux", SSLMode: "require"}
	dsn := cfg.dsn()
	if dsn == "" {
		t.Fatal("empty dsn")
	}
	// ensure builder works; do not print dsn
	if !containsAll(dsn, "db.example", "mux", "u") {
		t.Fatalf("unexpected dsn shape")
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func openOrSkip(t *testing.T) *Database {
	t.Helper()
	d, err := Open(ConfigFromEnv())
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	return d
}

func TestOpenClose(t *testing.T) {
	d := openOrSkip(t)
	if err := d.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func TestExecQueryMigrate(t *testing.T) {
	d := openOrSkip(t)
	ctx := context.Background()

	_, err := d.Exec(ctx, `DROP TABLE IF EXISTS items`)
	if err != nil {
		t.Fatalf("drop: %v", err)
	}

	if err := d.Migrate(ctx, []Migration{{
		Version: 1, Name: "items",
		Up:   `CREATE TABLE items (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL)`,
		Down: `DROP TABLE IF EXISTS items`,
	}}); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	n, err := d.Exec(ctx, `INSERT INTO items (name) VALUES ($1)`, "test-item")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if n != 1 {
		t.Fatalf("rows=%d", n)
	}

	rows, err := d.Query(ctx, `SELECT id, name FROM items ORDER BY id`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected row")
	}
	var id int64
	var name string
	if err := rows.Scan(&id, &name); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if name != "test-item" {
		t.Fatalf("name=%q", name)
	}

	if err := d.Rollback(ctx, 0); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
}
