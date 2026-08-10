package internal

import (
	"testing"
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
