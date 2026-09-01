package internal

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Muxcore-Media/database-postgres/internal/db"
)

// ExportState implements contracts.Backupable via pg_dump plain SQL when available.
func (m *Module) ExportState(ctx context.Context) ([]byte, error) {
	m.cfgMu.RLock()
	cfg := m.dbCfg
	m.cfgMu.RUnlock()
	return exportPostgres(ctx, cfg)
}

// ImportState restores a pg_dump plain SQL snapshot.
func (m *Module) ImportState(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty backup payload")
	}
	m.cfgMu.RLock()
	cfg := m.dbCfg
	m.cfgMu.RUnlock()

	if m.srv != nil {
		m.srv.Drain()
	}
	if m.database != nil {
		_ = m.database.Close(ctx)
		m.database = nil
	}

	if err := restorePostgres(ctx, cfg, data); err != nil {
		return err
	}

	d, err := db.Open(cfg)
	if err != nil {
		return fmt.Errorf("reopen database: %w", err)
	}
	if m.srv != nil {
		old := m.srv.ReplaceDatabase(d)
		if old != nil {
			_ = old.Close(ctx)
		}
	}
	m.database = d
	return nil
}

func exportPostgres(ctx context.Context, cfg db.Config) ([]byte, error) {
	if dump, err := pgDump(ctx, cfg); err == nil && len(dump) > 0 {
		return dump, nil
	}
	return sqlDumpGo(ctx, cfg)
}

func pgDump(ctx context.Context, cfg db.Config) ([]byte, error) {
	path, err := exec.LookPath("pg_dump")
	if err != nil {
		return nil, err
	}
	args := []string{
		"--no-owner",
		"--no-acl",
		"--format=plain",
		"--dbname", cfg.DSN(),
	}
	if schema := cfg.EffectiveSchema(); schema != "" && schema != "public" {
		args = append(args, "-n", schema)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = pgToolEnv(cfg)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pg_dump: %w", err)
	}
	return out, nil
}

func restorePostgres(ctx context.Context, cfg db.Config, data []byte) error {
	if _, err := exec.LookPath("psql"); err == nil {
		cmd := exec.CommandContext(ctx, "psql", "--dbname", cfg.DSN(), "-v", "ON_ERROR_STOP=1")
		cmd.Env = pgToolEnv(cfg)
		cmd.Stdin = bytes.NewReader(data)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("psql restore: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	return restoreSQLGo(ctx, cfg, data)
}

func pgToolEnv(cfg db.Config) []string {
	env := []string{
		"PGHOST=" + cfg.Host,
		"PGPORT=" + cfg.Port,
		"PGUSER=" + cfg.User,
		"PGPASSWORD=" + cfg.Password,
		"PGDATABASE=" + cfg.Database,
		"PGSSLMODE=" + cfg.SSLMode,
	}
	if cfg.URL != "" {
		env = append(env, "DATABASE_URL="+cfg.URL)
	}
	return env
}

func sqlDumpGo(ctx context.Context, cfg db.Config) ([]byte, error) {
	d, err := db.Open(cfg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = d.Close(ctx) }()

	schema := d.Schema()
	var tables []string
	rows, err := d.Query(ctx, `
		SELECT tablename FROM pg_tables
		WHERE schemaname = $1 AND tablename NOT LIKE 'pg_%'
		ORDER BY tablename`, schema)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return nil, err
		}
		tables = append(tables, name)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	var buf strings.Builder
	fmt.Fprintf(&buf, "-- muxcore database-postgres backup schema=%s\n", schema)
	for _, table := range tables {
		colRows, err := d.Query(ctx, `
			SELECT column_name, data_type
			FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = $2
			ORDER BY ordinal_position`, schema, table)
		if err != nil {
			return nil, fmt.Errorf("columns %s: %w", table, err)
		}
		var cols []string
		for colRows.Next() {
			var col, typ string
			if err := colRows.Scan(&col, &typ); err != nil {
				_ = colRows.Close()
				return nil, err
			}
			cols = append(cols, fmt.Sprintf("%s %s", quoteIdent(col), pgType(typ)))
		}
		_ = colRows.Close()
		if len(cols) == 0 {
			continue
		}
		fmt.Fprintf(&buf, "CREATE TABLE IF NOT EXISTS %s (%s);\n", quoteIdent(table), strings.Join(cols, ", "))

		dataRows, err := d.Query(ctx, fmt.Sprintf("SELECT * FROM %s", quoteIdent(table)))
		if err != nil {
			return nil, fmt.Errorf("dump %s: %w", table, err)
		}
		colNames, err := dataRows.Columns()
		if err != nil {
			_ = dataRows.Close()
			return nil, err
		}
		for dataRows.Next() {
			values := make([]any, len(colNames))
			ptrs := make([]any, len(colNames))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if err := dataRows.Scan(ptrs...); err != nil {
				_ = dataRows.Close()
				return nil, err
			}
			var literals []string
			for _, v := range values {
				literals = append(literals, sqlLiteral(v))
			}
			quotedCols := make([]string, len(colNames))
			for i, c := range colNames {
				quotedCols[i] = quoteIdent(c)
			}
			fmt.Fprintf(&buf, "INSERT INTO %s (%s) VALUES (%s);\n",
				quoteIdent(table), strings.Join(quotedCols, ", "), strings.Join(literals, ", "))
		}
		if err := dataRows.Close(); err != nil {
			return nil, err
		}
	}
	return []byte(buf.String()), nil
}

func restoreSQLGo(ctx context.Context, cfg db.Config, data []byte) error {
	d, err := db.Open(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close(ctx) }()

	for _, stmt := range splitSQLStatements(string(data)) {
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}
		if _, err := d.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("restore stmt: %w", err)
		}
	}
	return nil
}

func splitSQLStatements(sql string) []string {
	var out []string
	for _, part := range strings.Split(sql, ";\n") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func pgType(informationSchemaType string) string {
	switch informationSchemaType {
	case "integer":
		return "INTEGER"
	case "bigint":
		return "BIGINT"
	case "text":
		return "TEXT"
	case "boolean":
		return "BOOLEAN"
	case "timestamp with time zone":
		return "TIMESTAMPTZ"
	default:
		return strings.ToUpper(informationSchemaType)
	}
}

func sqlLiteral(v any) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case string:
		return "'" + strings.ReplaceAll(val, "'", "''") + "'"
	case []byte:
		return "'" + strings.ReplaceAll(string(val), "'", "''") + "'"
	case bool:
		if val {
			return "TRUE"
		}
		return "FALSE"
	case int64, int32, int16, int:
		return fmt.Sprintf("%v", val)
	case float64, float32:
		return fmt.Sprintf("%v", val)
	default:
		return "'" + strings.ReplaceAll(fmt.Sprint(val), "'", "''") + "'"
	}
}
