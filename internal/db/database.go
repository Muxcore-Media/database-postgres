package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Muxcore-Media/core/pkg/contracts"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const defaultConnectTimeout = 10 * time.Second

type Database struct {
	mu     sync.Mutex
	db     *sql.DB
	schema string
}

type Rows struct {
	rows *sql.Rows
}

func (r *Rows) Next() bool             { return r.rows.Next() }
func (r *Rows) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *Rows) Close() error           { return r.rows.Close() }

func (r *Rows) Columns() ([]string, error) {
	return r.rows.Columns()
}

func (r *Rows) Err() error {
	return r.rows.Err()
}

type Tx struct {
	tx *sql.Tx
}

func (t *Tx) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	result, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("postgres exec: %w", err)
	}
	return result.RowsAffected()
}

func (t *Tx) Query(ctx context.Context, query string, args ...any) (*Rows, error) {
	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres query: %w", err)
	}
	return &Rows{rows: rows}, nil
}

// Config holds connection settings (password never logged).
type Config struct {
	Host           string
	Port           string
	User           string
	Password       string
	Database       string
	SSLMode        string
	Schema         string
	URL            string // optional full URL; takes precedence when set
	ConnectTimeout time.Duration
	MaxOpenConns   int
	MaxIdleConns   int
}

func (c Config) EffectiveSchema() string {
	if s := strings.TrimSpace(c.Schema); s != "" {
		return s
	}
	return "public"
}

func (c Config) effectiveSchema() string {
	return c.EffectiveSchema()
}

func ConfigFromEnv() Config {
	cfg := Config{
		Host:     envOr("PGHOST", "localhost"),
		Port:     envOr("PGPORT", "5432"),
		User:     envOr("PGUSER", "muxcore"),
		Password: os.Getenv("PGPASSWORD"),
		Database: envOr("PGDATABASE", "muxcore"),
		SSLMode:  envOr("PGSSLMODE", "disable"),
		Schema:   envOr("PGSCHEMA", "public"),
		URL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
	}
	if v := strings.TrimSpace(os.Getenv("PGCONNECT_TIMEOUT")); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			cfg.ConnectTimeout = time.Duration(secs) * time.Second
		}
	}
	if v := strings.TrimSpace(os.Getenv("PGPOOL_MAX_OPEN")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxOpenConns = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("PGPOOL_MAX_IDLE")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.MaxIdleConns = n
		}
	}
	return cfg
}

func ConfigFromParams(p contracts.DatabaseParams) (Config, error) {
	for k, v := range p.Extra {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "password") || strings.Contains(lk, "passwd") ||
			strings.Contains(strings.ToLower(v), "password=") {
			return Config{}, contracts.ErrDatabaseCredentialExposure
		}
	}
	host := p.Host
	port := "5432"
	if h, pr, ok := strings.Cut(p.Host, ":"); ok && !strings.HasPrefix(p.Host, "/") {
		host, port = h, pr
	}
	ssl := "disable"
	schema := "public"
	var connectTimeout time.Duration
	if p.Extra != nil {
		if v := p.Extra["sslmode"]; v != "" {
			ssl = v
		}
		if v := p.Extra["schema"]; v != "" {
			schema = v
		}
		if v := p.Extra["search_path"]; v != "" {
			schema = v
		}
		if v := p.Extra["connect_timeout"]; v != "" {
			if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
				connectTimeout = time.Duration(secs) * time.Second
			}
		}
	}
	return Config{
		Host:           host,
		Port:           port,
		User:           p.User,
		Password:       p.Password,
		Database:       p.Database,
		SSLMode:        ssl,
		Schema:         schema,
		ConnectTimeout: connectTimeout,
	}, nil
}

func (c Config) DSN() string {
	return c.dsn()
}

func (c Config) dsn() string {
	if c.URL != "" {
		return c.URL
	}
	u := url.URL{
		Scheme: "postgres",
		Path:   "/" + c.Database,
	}
	if strings.HasPrefix(c.Host, "/") {
		u.Host = "localhost"
	} else {
		u.Host = netHost(c.Host, c.Port)
	}
	if c.User != "" {
		if c.Password != "" {
			u.User = url.UserPassword(c.User, c.Password)
		} else {
			u.User = url.User(c.User)
		}
	}
	q := url.Values{}
	q.Set("sslmode", c.SSLMode)
	if strings.HasPrefix(c.Host, "/") {
		q.Set("host", c.Host)
	}
	if c.ConnectTimeout > 0 {
		q.Set("connect_timeout", strconv.Itoa(int(c.ConnectTimeout.Seconds())))
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func netHost(host, port string) string {
	if port == "" {
		return host
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = net.JoinHostPort(host, port)
		return host
	}
	if strings.HasPrefix(host, "[") {
		return host + ":" + port
	}
	return host + ":" + port
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func (c Config) connectTimeout() time.Duration {
	if c.ConnectTimeout > 0 {
		return c.ConnectTimeout
	}
	return defaultConnectTimeout
}

func (c Config) maxOpenConns() int {
	if c.MaxOpenConns > 0 {
		return c.MaxOpenConns
	}
	return 10
}

func (c Config) maxIdleConns() int {
	if c.MaxIdleConns > 0 {
		return c.MaxIdleConns
	}
	return 5
}

func Open(cfg Config) (*Database, error) {
	if cfg.SSLMode == "disable" {
		slog.Warn("database: PGSSLMODE=disable sends credentials in plaintext; use require or verify-full in production")
	}
	dsn := cfg.dsn()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(cfg.maxOpenConns())
	db.SetMaxIdleConns(cfg.maxIdleConns())

	schema := cfg.effectiveSchema()
	d := &Database{db: db, schema: schema}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.connectTimeout())
	defer cancel()
	if err := d.initSchema(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres init schema: %w", err)
	}
	if err := d.Health(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres health check: %w", err)
	}
	slog.Info("database: opened PostgreSQL",
		"host", cfg.Host, "port", cfg.Port, "database", cfg.Database, "schema", schema, "user", cfg.User)
	return d, nil
}

func (d *Database) Schema() string {
	return d.schema
}

func (d *Database) DB() *sql.DB {
	return d.db
}

func (d *Database) initSchema(ctx context.Context) error {
	if d.schema == "public" {
		return nil
	}
	if _, err := d.db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS "+quoteIdent(d.schema)); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if _, err := d.db.ExecContext(ctx, "SET search_path TO "+quoteIdent(d.schema)); err != nil {
		return fmt.Errorf("set search_path: %w", err)
	}
	return nil
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func (d *Database) migrationsTable() string {
	if d.schema == "public" {
		return "_migrations"
	}
	return quoteIdent(d.schema) + "._migrations"
}

func (d *Database) Close(_ context.Context) error {
	return d.db.Close()
}

func (d *Database) Health(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

func (d *Database) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	result, err := d.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("postgres exec: %w", err)
	}
	return result.RowsAffected()
}

func (d *Database) Query(ctx context.Context, query string, args ...any) (*Rows, error) {
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres query: %w", err)
	}
	return &Rows{rows: rows}, nil
}

func (d *Database) Transaction(ctx context.Context, fn func(tx *Tx) error) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres begin tx: %w", err)
	}
	if err := fn(&Tx{tx: tx}); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			slog.Error("postgres rollback failed", "error", rbErr)
		}
		return err
	}
	return tx.Commit()
}

type Migration struct {
	Version int
	Name    string
	Up      string
	Down    string
}

func createMigrationsTable(ctx context.Context, db *sql.DB, table string) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		up_sql TEXT NOT NULL DEFAULT '',
		down_sql TEXT NOT NULL DEFAULT '',
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`, table))
	return err
}

func (d *Database) Migrate(ctx context.Context, migrations []Migration) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	table := d.migrationsTable()
	if err := createMigrationsTable(ctx, d.db, table); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	for _, m := range migrations {
		var exists int
		err := d.db.QueryRowContext(ctx,
			fmt.Sprintf("SELECT 1 FROM %s WHERE version = $1", table), m.Version).Scan(&exists)
		if err == nil {
			slog.Debug("migration already applied", "version", m.Version, "name", m.Name)
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("check migration %d: %w", m.Version, err)
		}

		slog.Info("applying migration", "version", m.Version, "name", m.Name)
		tx, err := d.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration tx %d: %w", m.Version, err)
		}
		if _, err := tx.ExecContext(ctx, m.Up); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d %q: %w", m.Version, m.Name, err)
		}
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf("INSERT INTO %s (version, name, up_sql, down_sql) VALUES ($1, $2, $3, $4)", table),
			m.Version, m.Name, m.Up, m.Down); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.Version, err)
		}
	}
	return nil
}

func (d *Database) Rollback(ctx context.Context, targetVersion int) error {
	if targetVersion < 0 {
		return fmt.Errorf("target version must be >= 0")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	table := d.migrationsTable()
	if err := createMigrationsTable(ctx, d.db, table); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	var maxVersion sql.NullInt64
	if err := d.db.QueryRowContext(ctx, fmt.Sprintf("SELECT MAX(version) FROM %s", table)).Scan(&maxVersion); err != nil {
		return fmt.Errorf("query max migration version: %w", err)
	}
	if !maxVersion.Valid || int(maxVersion.Int64) <= targetVersion {
		return nil
	}

	if targetVersion > 0 {
		var exists int
		err := d.db.QueryRowContext(ctx,
			fmt.Sprintf("SELECT 1 FROM %s WHERE version = $1", table), targetVersion).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return contracts.ErrMigrationTargetNotFound
		}
		if err != nil {
			return fmt.Errorf("check target version %d: %w", targetVersion, err)
		}
	}

	type migInfo struct {
		Version int
		Name    string
		DownSQL string
	}

	rows, err := d.db.QueryContext(ctx,
		fmt.Sprintf("SELECT version, name, down_sql FROM %s WHERE version > $1 ORDER BY version DESC", table),
		targetVersion)
	if err != nil {
		return fmt.Errorf("query migrations for rollback: %w", err)
	}

	var toRollback []migInfo
	for rows.Next() {
		var m migInfo
		if err := rows.Scan(&m.Version, &m.Name, &m.DownSQL); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan migration: %w", err)
		}
		toRollback = append(toRollback, m)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close migration rows: %w", err)
	}

	for _, m := range toRollback {
		slog.Info("rolling back migration", "version", m.Version, "name", m.Name)
		if m.DownSQL == "" {
			slog.Warn("migration has no down SQL; catalog row retained", "version", m.Version, "name", m.Name)
			continue
		}
		tx, err := d.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin rollback tx %d: %w", m.Version, err)
		}
		if _, err := tx.ExecContext(ctx, m.DownSQL); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("rollback migration %d: %w", m.Version, err)
		}
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf("DELETE FROM %s WHERE version = $1", table), m.Version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("delete migration record %d: %w", m.Version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit rollback %d: %w", m.Version, err)
		}
	}
	return nil
}
