package internal

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Muxcore-Media/core/pkg/contracts"
	modulesdk "github.com/Muxcore-Media/core/sdk/go/module"
	"github.com/Muxcore-Media/database-postgres/internal/db"
)

func (m *Module) Settings() []contracts.SettingDef {
	return m.settingsDefs()
}

func (m *Module) UpdateSetting(key, value string) error {
	return m.updateSetting(key, value)
}

func (m *Module) settingsDefs() []contracts.SettingDef {
	m.cfgMu.RLock()
	cfg := m.dbCfg
	m.cfgMu.RUnlock()
	return []contracts.SettingDef{
		{
			Key:         "database_url",
			Label:       "Database URL",
			Type:        contracts.SettingTypeSecret,
			Value:       modulesdk.MaskSecret(cfg.URL),
			Default:     "",
			Description: "Full Postgres URL (DATABASE_URL); when set, overrides host/user fields",
			Group:       "Connection",
		},
		{
			Key:         "host",
			Label:       "Host",
			Type:        contracts.SettingTypeString,
			Value:       cfg.Host,
			Default:     "localhost",
			Description: "Postgres host (PGHOST); use a path such as /run/postgresql for unix sockets",
			Group:       "Connection",
		},
		{
			Key:         "port",
			Label:       "Port",
			Type:        contracts.SettingTypeString,
			Value:       cfg.Port,
			Default:     "5432",
			Description: "Postgres port (PGPORT); ignored for unix-socket hosts",
			Group:       "Connection",
		},
		{
			Key:         "user",
			Label:       "User",
			Type:        contracts.SettingTypeString,
			Value:       cfg.User,
			Default:     "muxcore",
			Description: "Postgres user (PGUSER)",
			Group:       "Connection",
		},
		{
			Key:         "password",
			Label:       "Password",
			Type:        contracts.SettingTypeSecret,
			Value:       modulesdk.MaskSecret(cfg.Password),
			Default:     "",
			Description: "Postgres password (PGPASSWORD)",
			Group:       "Connection",
		},
		{
			Key:         "database",
			Label:       "Database",
			Type:        contracts.SettingTypeString,
			Value:       cfg.Database,
			Default:     "muxcore",
			Description: "Database name (PGDATABASE)",
			Group:       "Connection",
		},
		{
			Key:         "schema",
			Label:       "Schema",
			Type:        contracts.SettingTypeString,
			Value:       cfg.EffectiveSchema(),
			Default:     "public",
			Description: "Postgres schema / search_path (PGSCHEMA); isolates _migrations per sidecar on shared clusters",
			Group:       "Connection",
		},
		{
			Key:         "sslmode",
			Label:       "SSL Mode",
			Type:        contracts.SettingTypeSelect,
			Value:       cfg.SSLMode,
			Default:     "disable",
			Description: "libpq sslmode (PGSSLMODE); disable logs a production warning",
			Group:       "Connection",
			Options:     []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"},
		},
		{
			Key:         "connect_timeout",
			Label:       "Connect Timeout (seconds)",
			Type:        contracts.SettingTypeString,
			Value:       timeoutSettingValue(cfg.ConnectTimeout),
			Default:     "10",
			Description: "Seconds to wait for initial Ping (PGCONNECT_TIMEOUT)",
			Group:       "Pool",
		},
		{
			Key:         "pool_max_open",
			Label:       "Pool Max Open",
			Type:        contracts.SettingTypeString,
			Value:       intSettingValue(cfg.MaxOpenConns, 10),
			Default:     "10",
			Description: "database/sql MaxOpenConns (PGPOOL_MAX_OPEN)",
			Group:       "Pool",
		},
		{
			Key:         "pool_max_idle",
			Label:       "Pool Max Idle",
			Type:        contracts.SettingTypeString,
			Value:       intSettingValue(cfg.MaxIdleConns, 5),
			Default:     "5",
			Description: "database/sql MaxIdleConns (PGPOOL_MAX_IDLE)",
			Group:       "Pool",
		},
	}
}

func timeoutSettingValue(d time.Duration) string {
	if d <= 0 {
		return "10"
	}
	return strconv.Itoa(int(d.Seconds()))
}

func intSettingValue(n, def int) string {
	if n > 0 {
		return strconv.Itoa(n)
	}
	return strconv.Itoa(def)
}

func (m *Module) updateSetting(key, value string) error {
	value = strings.TrimSpace(value)
	m.cfgMu.RLock()
	cfg := m.dbCfg
	m.cfgMu.RUnlock()

	switch key {
	case "database_url", "DATABASE_URL":
		if value == "" || value == modulesdk.MaskSecret("x") {
			return nil
		}
		cfg.URL = value
	case "host", "PGHOST":
		if value == "" {
			return fmt.Errorf("host must not be empty")
		}
		cfg.Host = value
		cfg.URL = ""
	case "port", "PGPORT":
		if value == "" {
			return fmt.Errorf("port must not be empty")
		}
		cfg.Port = value
		cfg.URL = ""
	case "user", "PGUSER":
		cfg.User = value
		cfg.URL = ""
	case "password", "PGPASSWORD":
		if value == "" || value == modulesdk.MaskSecret("x") {
			return nil
		}
		cfg.Password = value
		cfg.URL = ""
	case "database", "PGDATABASE":
		if value == "" {
			return fmt.Errorf("database must not be empty")
		}
		cfg.Database = value
		cfg.URL = ""
	case "schema", "PGSCHEMA", "search_path":
		if value == "" {
			return fmt.Errorf("schema must not be empty")
		}
		cfg.Schema = value
		cfg.URL = ""
	case "sslmode", "PGSSLMODE":
		switch value {
		case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
			cfg.SSLMode = value
			cfg.URL = ""
			if value == "disable" {
				slog.Warn("database-postgres: sslmode disable is dev-only; use require in production")
			}
		default:
			return fmt.Errorf("invalid sslmode %q", value)
		}
	case "connect_timeout", "PGCONNECT_TIMEOUT":
		secs, err := strconv.Atoi(value)
		if err != nil || secs <= 0 {
			return fmt.Errorf("connect_timeout must be a positive integer")
		}
		cfg.ConnectTimeout = time.Duration(secs) * time.Second
		cfg.URL = ""
	case "pool_max_open", "PGPOOL_MAX_OPEN":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return fmt.Errorf("pool_max_open must be a positive integer")
		}
		cfg.MaxOpenConns = n
	case "pool_max_idle", "PGPOOL_MAX_IDLE":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return fmt.Errorf("pool_max_idle must be >= 0")
		}
		cfg.MaxIdleConns = n
	default:
		return fmt.Errorf("unknown setting %q", key)
	}
	return m.applyDBConfig(cfg)
}

func (m *Module) applyDBConfig(cfg db.Config) error {
	m.cfgMu.Lock()
	defer m.cfgMu.Unlock()
	if m.srv == nil {
		m.dbCfg = cfg
		return nil
	}
	d, err := db.Open(cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	old := m.srv.ReplaceDatabase(d)
	m.database = d
	m.dbCfg = cfg
	if old != nil {
		m.srv.Drain()
		_ = old.Close(context.Background())
	}
	return nil
}
