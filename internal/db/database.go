package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/Muxcore-Media/core/pkg/contracts"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Database struct {
	mu sync.Mutex
	db *sql.DB
}

type Rows struct {
	rows *sql.Rows
}

func (r *Rows) Next() bool          { return r.rows.Next() }
func (r *Rows) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *Rows) Close() error        { return r.rows.Close() }

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
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
	URL      string // optional full URL; takes precedence when set
}

func ConfigFromEnv() Config {
	return Config{
		Host:     envOr("PGHOST", "localhost"),
		Port:     envOr("PGPORT", "5432"),
		User:     envOr("PGUSER", "muxcore"),
		Password: os.Getenv("PGPASSWORD"),
		Database: envOr("PGDATABASE", "muxcore"),
		SSLMode:  envOr("PGSSLMODE", "disable"),
		URL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
	}
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
	if h, pr, ok := strings.Cut(p.Host, ":"); ok {
		host, port = h, pr
	}
	ssl := "disable"
	if p.Extra != nil {
		if v := p.Extra["sslmode"]; v != "" {
			ssl = v
		}
	}
	return Config{
		Host:     host,
		Port:     port,
		User:     p.User,
		Password: p.Password,
		Database: p.Database,
		SSLMode:  ssl,
	}, nil
}

func (c Config) dsn() string {
	if c.URL != "" {
		return c.URL
	}
	u := url.URL{
		Scheme: "postgres",
		Host:   netHost(c.Host, c.Port),
		Path:   "/" + c.Database,
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
	u.RawQuery = q.Encode()
	return u.String()
}

func netHost(host, port string) string {
	if port == "" {
		return host
	}
	return host + ":" + port
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func Open(cfg Config) (*Database, error) {
	dsn := cfg.dsn()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	d := &Database{db: db}
	if err := d.Health(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres health check: %w", err)
	}
	slog.Info("database: opened PostgreSQL",
		"host", cfg.Host, "port", cfg.Port, "database", cfg.Database, "user", cfg.User)
	return d, nil
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

func createMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS _migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		up_sql TEXT NOT NULL DEFAULT '',
		down_sql TEXT NOT NULL DEFAULT '',
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	return err
}

func (d *Database) Migrate(ctx context.Context, migrations []Migration) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := createMigrationsTable(d.db); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	for _, m := range migrations {
		var exists int
		err := d.db.QueryRowContext(ctx, "SELECT 1 FROM _migrations WHERE version = $1", m.Version).Scan(&exists)
		if err == nil {
			slog.Debug("migration already applied", "version", m.Version, "name", m.Name)
			continue
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("check migration %d: %w", m.Version, err)
		}

		slog.Info("applying migration", "version", m.Version, "name", m.Name)
		if _, err := d.db.ExecContext(ctx, m.Up); err != nil {
			return fmt.Errorf("migration %d %q: %w", m.Version, m.Name, err)
		}
		if _, err := d.db.ExecContext(ctx,
			"INSERT INTO _migrations (version, name, up_sql, down_sql) VALUES ($1, $2, $3, $4)",
			m.Version, m.Name, m.Up, m.Down); err != nil {
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}
	}
	return nil
}

func (d *Database) Rollback(ctx context.Context, targetVersion int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := createMigrationsTable(d.db); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	type migInfo struct {
		Version int
		Name    string
		DownSQL string
	}

	rows, err := d.db.QueryContext(ctx,
		"SELECT version, name, down_sql FROM _migrations WHERE version > $1 ORDER BY version DESC",
		targetVersion)
	if err != nil {
		return fmt.Errorf("query migrations for rollback: %w", err)
	}

	var toRollback []migInfo
	for rows.Next() {
		var m migInfo
		if err := rows.Scan(&m.Version, &m.Name, &m.DownSQL); err != nil {
			rows.Close()
			return fmt.Errorf("scan migration: %w", err)
		}
		toRollback = append(toRollback, m)
	}
	rows.Close()

	for _, m := range toRollback {
		slog.Info("rolling back migration", "version", m.Version, "name", m.Name)
		if m.DownSQL != "" {
			if _, err := d.db.ExecContext(ctx, m.DownSQL); err != nil {
				return fmt.Errorf("rollback migration %d: %w", m.Version, err)
			}
		}
		if _, err := d.db.ExecContext(ctx, "DELETE FROM _migrations WHERE version = $1", m.Version); err != nil {
			return fmt.Errorf("delete migration record %d: %w", m.Version, err)
		}
	}
	return nil
}
