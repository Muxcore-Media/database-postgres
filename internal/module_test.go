package internal

import (
	"testing"

	"github.com/Muxcore-Media/core/pkg/contracts"
	modulesdk "github.com/Muxcore-Media/core/sdk/go/module"
	"github.com/Muxcore-Media/database-postgres/internal/db"
)

func TestModuleInfo(t *testing.T) {
	m := NewModule(Config{})
	info := m.Info()
	if info.ID != "database-postgres" {
		t.Fatalf("ID=%q", info.ID)
	}
	found := false
	for _, c := range info.Capabilities {
		if c == "database.postgres" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing database.postgres capability")
	}
}

func TestSettings_MaskSecretsAndDefaults(t *testing.T) {
	m := NewModule(Config{DB: db.Config{
		Host:     "db.example",
		Port:     "5433",
		User:     "app",
		Password: "s3cret",
		Database: "prod",
		SSLMode:  "require",
		URL:      "postgres://app:s3cret@db.example:5433/prod?sslmode=require",
	}})
	defs := m.Settings()
	byKey := map[string]contracts.SettingDef{}
	for _, d := range defs {
		byKey[d.Key] = d
	}
	if byKey["host"].Value != "db.example" {
		t.Fatalf("host=%q", byKey["host"].Value)
	}
	if byKey["password"].Value != modulesdk.MaskSecret("s3cret") {
		t.Fatalf("password not masked: %q", byKey["password"].Value)
	}
	if byKey["database_url"].Value != modulesdk.MaskSecret("x") {
		t.Fatalf("database_url not masked: %q", byKey["database_url"].Value)
	}
	if byKey["sslmode"].Type != contracts.SettingTypeSelect {
		t.Fatalf("sslmode type=%q", byKey["sslmode"].Type)
	}
}

func TestUpdateSetting_PreInitNoOpen(t *testing.T) {
	m := NewModule(Config{DB: db.Config{
		Host:     "localhost",
		Port:     "5432",
		User:     "muxcore",
		Database: "muxcore",
		SSLMode:  "disable",
	}})
	if err := m.UpdateSetting("host", "pg.internal"); err != nil {
		t.Fatal(err)
	}
	if err := m.UpdateSetting("sslmode", "prefer"); err != nil {
		t.Fatal(err)
	}
	if err := m.UpdateSetting("sslmode", "bogus"); err == nil {
		t.Fatal("expected invalid sslmode error")
	}
	if err := m.UpdateSetting("password", modulesdk.MaskSecret("x")); err != nil {
		t.Fatal(err)
	}
	defs := m.Settings()
	for _, d := range defs {
		switch d.Key {
		case "host":
			if d.Value != "pg.internal" {
				t.Fatalf("host=%q", d.Value)
			}
		case "sslmode":
			if d.Value != "prefer" {
				t.Fatalf("sslmode=%q", d.Value)
			}
		case "password":
			if d.Value != "" {
				t.Fatalf("password should stay empty after mask no-op, got %q", d.Value)
			}
		}
	}
}
