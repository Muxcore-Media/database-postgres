package internal

import (
	"context"
	"fmt"
	"strings"

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
			Description: "Postgres host (PGHOST)",
			Group:       "Connection",
		},
		{
			Key:         "port",
			Label:       "Port",
			Type:        contracts.SettingTypeString,
			Value:       cfg.Port,
			Default:     "5432",
			Description: "Postgres port (PGPORT)",
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
			Key:         "sslmode",
			Label:       "SSL Mode",
			Type:        contracts.SettingTypeSelect,
			Value:       cfg.SSLMode,
			Default:     "disable",
			Description: "libpq sslmode (PGSSLMODE)",
			Group:       "Connection",
			Options:     []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"},
		},
	}
}

func (m *Module) updateSetting(key, value string) error {
	value = strings.TrimSpace(value)
	m.cfgMu.RLock()
	cfg := m.dbCfg
	m.cfgMu.RUnlock()

	switch key {
	case "database_url", "DATABASE_URL":
		// Empty or UI mask means "leave unchanged".
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
	case "sslmode", "PGSSLMODE":
		switch value {
		case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
			cfg.SSLMode = value
			cfg.URL = ""
		default:
			return fmt.Errorf("invalid sslmode %q", value)
		}
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
		_ = old.Close(context.Background())
	}
	return nil
}
