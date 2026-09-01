package internal

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	databasev1 "github.com/Muxcore-Media/core/proto/gen/muxcore/database/v1"
	"github.com/Muxcore-Media/database-postgres/internal/db"
)

func TestBackupRoundTrip(t *testing.T) {
	ctx := context.Background()
	prefix := testPrefix(t)
	table := prefix + "_notes"

	m := NewModule(Config{DB: db.ConfigFromEnv(), GRPCAddr: "127.0.0.1:0"})
	if err := m.Init(ctx); err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	defer func() { _ = m.Stop(ctx) }()

	if _, err := m.database.Exec(ctx, "CREATE TABLE "+table+" (id BIGSERIAL PRIMARY KEY, body TEXT)"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = m.database.Exec(context.Background(), "DROP TABLE IF EXISTS "+table)
	})
	if _, err := m.database.Exec(ctx, "INSERT INTO "+table+" (body) VALUES ($1)", "hello"); err != nil {
		t.Fatal(err)
	}

	data, err := m.ExportState(ctx)
	if err != nil {
		t.Fatalf("ExportState: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty export")
	}

	if _, err := m.database.Exec(ctx, "DELETE FROM "+table); err != nil {
		t.Fatal(err)
	}

	if err := m.ImportState(ctx, data); err != nil {
		t.Fatalf("ImportState: %v", err)
	}

	rows, err := m.database.Query(ctx, "SELECT body FROM "+table)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		t.Fatal("expected row after restore")
	}
	var body string
	if err := rows.Scan(&body); err != nil {
		t.Fatal(err)
	}
	if body != "hello" {
		t.Fatalf("body=%q", body)
	}
}

func TestBackupEmptyPayload(t *testing.T) {
	m := NewModule(Config{DB: db.ConfigFromEnv(), GRPCAddr: "127.0.0.1:0"})
	ctx := context.Background()
	if err := m.ImportState(ctx, nil); err == nil {
		t.Fatal("expected error for empty payload")
	}
}

func TestConcurrentQueryDuringDBSwap(t *testing.T) {
	d1, err := db.Open(db.ConfigFromEnv())
	if err != nil {
		if os.Getenv("CI") == "true" {
			t.Fatalf("postgres unavailable in CI: %v", err)
		}
		t.Skipf("postgres unavailable: %v", err)
	}
	defer func() { _ = d1.Close(context.Background()) }()
	d2, err := db.Open(db.ConfigFromEnv())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d2.Close(context.Background()) }()

	ctx := context.Background()
	prefix := testPrefix(t)
	table := prefix + "_swap"

	m := NewModule(Config{DB: db.ConfigFromEnv(), GRPCAddr: "127.0.0.1:0"})
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Stop(ctx) }()

	if _, err := d1.Exec(ctx, "CREATE TABLE "+table+" (v INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if _, err := d1.Exec(ctx, "INSERT INTO "+table+" (v) VALUES (1)"); err != nil {
		t.Fatal(err)
	}
	if _, err := d2.Exec(ctx, "CREATE TABLE "+table+" (v INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if _, err := d2.Exec(ctx, "INSERT INTO "+table+" (v) VALUES (2)"); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.srv.Query(ctx, &databasev1.QueryRequest{Query: "SELECT v FROM " + table})
			errs <- err
		}()
	}

	time.Sleep(10 * time.Millisecond)
	old := m.srv.ReplaceDatabase(d2)
	m.database = d2
	if old != nil {
		m.srv.Drain()
		_ = old.Close(ctx)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
	}
}

func TestModuleInfoBackupable(t *testing.T) {
	m := NewModule(Config{})
	info := m.Info()
	found := false
	for _, c := range info.Capabilities {
		if c == "backupable" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("capabilities=%v", info.Capabilities)
	}
}

func TestDefaultGRPCAddrLoopback(t *testing.T) {
	m := NewModule(Config{})
	if m.grpcAddr != defaultGRPCAddr {
		t.Fatalf("addr=%q want %q", m.grpcAddr, defaultGRPCAddr)
	}
}

func testPrefix(t *testing.T) string {
	t.Helper()
	name := t.Name()
	if len(name) > 40 {
		name = name[len(name)-40:]
	}
	return "bk_" + name
}
