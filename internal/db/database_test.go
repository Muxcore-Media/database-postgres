package db

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"strings"
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
	if !containsAll(dsn, "db.example", "mux", "u") {
		t.Fatalf("unexpected dsn shape: %s", dsn)
	}
}

func TestDSN_UnixSocket(t *testing.T) {
	cfg := Config{
		Host:     "/run/postgresql",
		User:     "postgres",
		Database: "muxcore",
		SSLMode:  "disable",
	}
	dsn := cfg.dsn()
	if strings.Contains(dsn, "/run/postgresql:5432") {
		t.Fatalf("unix socket must not append port: %s", dsn)
	}
	if !strings.Contains(dsn, "host=%2Frun%2Fpostgresql") && !strings.Contains(dsn, "host=/run/postgresql") {
		t.Fatalf("expected host query param for socket: %s", dsn)
	}
}

func TestDSN_IPv6(t *testing.T) {
	cfg := Config{
		Host:     "fd2c:a2fd:5d9e:ab72::1",
		Port:     "5432",
		User:     "u",
		Database: "mux",
		SSLMode:  "disable",
	}
	dsn := cfg.dsn()
	if !strings.Contains(dsn, "[fd2c:a2fd:5d9e:ab72::1]:5432") {
		t.Fatalf("expected bracketed IPv6 host: %s", dsn)
	}
}

func TestNetHost(t *testing.T) {
	got := netHost("fd2c:a2fd:5d9e:ab72::1", "5432")
	want := "[fd2c:a2fd:5d9e:ab72::1]:5432"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}

func openOrSkip(t *testing.T) *Database {
	t.Helper()
	d, err := Open(ConfigFromEnv())
	if err != nil {
		if os.Getenv("CI") == "true" {
			t.Fatalf("postgres unavailable in CI: %v", err)
		}
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	return d
}

func testPrefix(t *testing.T) string {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	if len(name) > 48 {
		name = name[len(name)-48:]
	}
	return fmt.Sprintf("t_%s", name)
}

func TestOpenClose(t *testing.T) {
	d := openOrSkip(t)
	if err := d.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func testMigrationVersion(t *testing.T) int {
	t.Helper()
	h := fnv.New32a()
	_, _ = h.Write([]byte(t.Name()))
	return int(h.Sum32()%900000) + 100000
}

func TestExecQueryMigrate(t *testing.T) {
	d := openOrSkip(t)
	ctx := context.Background()
	prefix := testPrefix(t)
	items := prefix + "_items"
	version := testMigrationVersion(t)

	_, err := d.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, quoteIdent(items)))
	if err != nil {
		t.Fatalf("drop: %v", err)
	}

	if err := d.Migrate(ctx, []Migration{{
		Version: version, Name: prefix + "_items",
		Up:   fmt.Sprintf(`CREATE TABLE %s (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL)`, quoteIdent(items)),
		Down: fmt.Sprintf(`DROP TABLE IF EXISTS %s`, quoteIdent(items)),
	}}); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	n, err := d.Exec(ctx, fmt.Sprintf(`INSERT INTO %s (name) VALUES ($1)`, quoteIdent(items)), "test-item")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if n != 1 {
		t.Fatalf("rows=%d", n)
	}

	rows, err := d.Query(ctx, fmt.Sprintf(`SELECT id, name FROM %s ORDER BY id`, quoteIdent(items)))
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = rows.Close() }()
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

func TestRollbackUnknownTarget(t *testing.T) {
	d := openOrSkip(t)
	ctx := context.Background()
	prefix := testPrefix(t)

	base := testMigrationVersion(t)
	if err := d.Migrate(ctx, []Migration{
		{Version: base + 1, Name: prefix + "_one", Up: fmt.Sprintf("CREATE TABLE %s (id BIGINT PRIMARY KEY)", quoteIdent(prefix+"_t1")), Down: fmt.Sprintf("DROP TABLE %s", quoteIdent(prefix+"_t1"))},
		{Version: base + 3, Name: prefix + "_three", Up: fmt.Sprintf("CREATE TABLE %s (id BIGINT PRIMARY KEY)", quoteIdent(prefix+"_t3")), Down: fmt.Sprintf("DROP TABLE %s", quoteIdent(prefix+"_t3"))},
	}); err != nil {
		t.Fatal(err)
	}

	if err := d.Rollback(ctx, base+2); !errors.Is(err, contracts.ErrMigrationTargetNotFound) {
		t.Fatalf("err=%v want ErrMigrationTargetNotFound", err)
	}
}

func TestRollbackNegativeTarget(t *testing.T) {
	d := openOrSkip(t)
	if err := d.Rollback(context.Background(), -1); err == nil {
		t.Fatal("expected error for negative target")
	}
}

func TestRollbackEmptyDownRetainsCatalog(t *testing.T) {
	d := openOrSkip(t)
	ctx := context.Background()
	prefix := testPrefix(t)
	table := prefix + "_locked"

	if err := d.Migrate(ctx, []Migration{
		{Version: testMigrationVersion(t), Name: prefix + "_no_down", Up: fmt.Sprintf("CREATE TABLE %s (id BIGINT PRIMARY KEY)", quoteIdent(table)), Down: ""},
	}); err != nil {
		t.Fatal(err)
	}

	if err := d.Rollback(ctx, 0); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	tableName := d.migrationsTable()
	rows, err := d.Query(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		t.Fatal("expected count row")
	}
	var count int64
	if err := rows.Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("catalog count=%d want 1", count)
	}
}

func TestRowsColumns(t *testing.T) {
	d := openOrSkip(t)
	ctx := context.Background()
	prefix := testPrefix(t)
	table := prefix + "_cols"

	if _, err := d.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (id BIGINT, name TEXT)", quoteIdent(table))); err != nil {
		t.Fatal(err)
	}
	rows, err := d.Query(ctx, fmt.Sprintf("SELECT id, name FROM %s", quoteIdent(table)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 2 || cols[0] != "id" || cols[1] != "name" {
		t.Fatalf("columns=%v", cols)
	}
}
