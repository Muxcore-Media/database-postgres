package server

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Muxcore-Media/core/pkg/contracts"
	databasev1 "github.com/Muxcore-Media/core/proto/gen/muxcore/database/v1"
	"github.com/Muxcore-Media/database-postgres/internal/db"
)

func newTestServer(t *testing.T) (*Server, context.Context) {
	t.Helper()
	d, err := db.Open(db.ConfigFromEnv())
	if err != nil {
		if os.Getenv("CI") == "true" {
			t.Fatalf("postgres unavailable in CI: %v", err)
		}
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	return New(d), context.Background()
}

func TestExecEmptyQuery(t *testing.T) {
	s, ctx := newTestServer(t)
	_, err := s.Exec(ctx, &databasev1.ExecRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code=%v want InvalidArgument", status.Code(err))
	}
}

func TestQueryEmptyQuery(t *testing.T) {
	s, ctx := newTestServer(t)
	_, err := s.Query(ctx, &databasev1.QueryRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code=%v want InvalidArgument", status.Code(err))
	}
}

func TestExecQueryRoundTrip(t *testing.T) {
	s, ctx := newTestServer(t)
	table := "srv_items_" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	if len(table) > 56 {
		table = table[len(table)-56:]
	}

	_, err := s.Exec(ctx, &databasev1.ExecRequest{
		Query: "CREATE TABLE " + table + " (id BIGSERIAL PRIMARY KEY, name TEXT, active BOOLEAN)",
	})
	if err != nil {
		t.Fatalf("Exec create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = s.Exec(ctx, &databasev1.ExecRequest{Query: "DROP TABLE IF EXISTS " + table})
	})

	_, err = s.Exec(ctx, &databasev1.ExecRequest{
		Query: "INSERT INTO " + table + " (name, active) VALUES ($1, $2)",
		Args: []*databasev1.Value{
			{Kind: &databasev1.Value_StringVal{StringVal: "alpha"}},
			{Kind: &databasev1.Value_BoolVal{BoolVal: true}},
		},
	})
	if err != nil {
		t.Fatalf("Exec insert: %v", err)
	}

	resp, err := s.Query(ctx, &databasev1.QueryRequest{Query: "SELECT * FROM " + table})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(resp.Columns) != 3 {
		t.Fatalf("columns=%v want 3", resp.Columns)
	}
	if resp.Columns[0] != "id" || resp.Columns[1] != "name" || resp.Columns[2] != "active" {
		t.Fatalf("columns=%v", resp.Columns)
	}
	if len(resp.Rows) != 1 {
		t.Fatalf("rows=%d want 1", len(resp.Rows))
	}
	row := resp.Rows[0]
	if row.Values[1].GetStringVal() != "alpha" {
		t.Fatalf("name=%q", row.Values[1].GetStringVal())
	}
	if !row.Values[2].GetBoolVal() {
		t.Fatalf("expected active=true, got %v", row.Values[2])
	}
}

func TestQueryTypedColumns(t *testing.T) {
	s, ctx := newTestServer(t)
	table := "srv_types_" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	if len(table) > 56 {
		table = table[len(table)-56:]
	}

	if _, err := s.Exec(ctx, &databasev1.ExecRequest{
		Query: "CREATE TABLE " + table + " (s TEXT, i INTEGER, ts TIMESTAMPTZ, blob BYTEA, n TEXT)",
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = s.Exec(ctx, &databasev1.ExecRequest{Query: "DROP TABLE IF EXISTS " + table})
	})

	ts := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	_, err := s.Exec(ctx, &databasev1.ExecRequest{
		Query: "INSERT INTO " + table + " (s, i, ts, blob, n) VALUES ($1, $2, $3, $4, NULL)",
		Args: []*databasev1.Value{
			{Kind: &databasev1.Value_StringVal{StringVal: "str"}},
			{Kind: &databasev1.Value_IntVal{IntVal: 42}},
			{Kind: &databasev1.Value_StringVal{StringVal: ts.Format(time.RFC3339Nano)}},
			{Kind: &databasev1.Value_BytesVal{BytesVal: []byte{1, 2, 3}}},
		},
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	resp, err := s.Query(ctx, &databasev1.QueryRequest{Query: "SELECT s, i, ts, blob, n FROM " + table})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	vals := resp.Rows[0].Values
	if vals[0].GetStringVal() != "str" {
		t.Fatalf("string=%v", vals[0])
	}
	if vals[1].GetIntVal() != 42 {
		t.Fatalf("int=%v", vals[1])
	}
	if vals[2].GetStringVal() == "" {
		t.Fatalf("timestamp=%v", vals[2])
	}
	if string(vals[3].GetBytesVal()) != string([]byte{1, 2, 3}) {
		t.Fatalf("bytes=%v", vals[3])
	}
	if !vals[4].GetNullVal() {
		t.Fatalf("null=%v", vals[4])
	}
}

func TestSyntaxErrorInvalidArgument(t *testing.T) {
	s, ctx := newTestServer(t)
	_, err := s.Exec(ctx, &databasev1.ExecRequest{Query: "SELEC 1"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code=%v want InvalidArgument", status.Code(err))
	}
}

func TestRollbackUnknownTarget(t *testing.T) {
	s, ctx := newTestServer(t)
	prefix := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	if len(prefix) > 40 {
		prefix = prefix[len(prefix)-40:]
	}

	_, err := s.Migrate(ctx, &databasev1.MigrateRequest{
		Migrations: []*databasev1.Migration{
			{Version: 1, Name: prefix + "_one", UpSql: "CREATE TABLE " + prefix + "_t1 (id BIGINT PRIMARY KEY)", DownSql: "DROP TABLE " + prefix + "_t1"},
			{Version: 3, Name: prefix + "_three", UpSql: "CREATE TABLE " + prefix + "_t3 (id BIGINT PRIMARY KEY)", DownSql: "DROP TABLE " + prefix + "_t3"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Rollback(ctx, &databasev1.RollbackRequest{TargetVersion: 2})
	if err == nil {
		t.Fatal("expected error for unknown target")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code=%v want InvalidArgument", status.Code(err))
	}
	if !strings.Contains(err.Error(), contracts.ErrMigrationTargetNotFound.Error()) {
		t.Fatalf("err=%v", err)
	}
}

func TestReplaceDatabaseDrain(t *testing.T) {
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

	s := New(d1)
	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		_, err := s.Query(ctx, &databasev1.QueryRequest{Query: "SELECT 1"})
		done <- err
	}()

	old := s.ReplaceDatabase(d2)
	if old != d1 {
		t.Fatal("expected old db")
	}
	s.Drain()
	_ = old.Close(ctx)

	if err := <-done; err != nil {
		t.Fatalf("query during swap: %v", err)
	}
}
