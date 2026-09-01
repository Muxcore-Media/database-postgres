package internal

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"

	"google.golang.org/grpc"

	"github.com/Muxcore-Media/core/pkg/contracts"
	modulesdk "github.com/Muxcore-Media/core/sdk/go/module"
	"github.com/Muxcore-Media/database-postgres/internal/db"
	"github.com/Muxcore-Media/database-postgres/internal/server"
)

const defaultGRPCAddr = "127.0.0.1:9701"

type Module struct {
	database *db.Database
	srv      *server.Server
	grpcSrv  *grpc.Server
	lis      net.Listener

	id       string
	cfgMu    sync.RWMutex
	dbCfg    db.Config
	grpcAddr string
}

type Config struct {
	ID       string
	DB       db.Config
	GRPCAddr string
}

func NewModule(cfg Config) *Module {
	if cfg.ID == "" {
		cfg.ID = "database-postgres"
	}
	if cfg.GRPCAddr == "" {
		cfg.GRPCAddr = defaultGRPCAddr
	}
	if v := os.Getenv("DATABASE_GRPC_ADDR"); v != "" {
		cfg.GRPCAddr = v
	}
	if cfg.DB.Host == "" && cfg.DB.URL == "" {
		cfg.DB = db.ConfigFromEnv()
	}
	return &Module{
		id:       cfg.ID,
		dbCfg:    cfg.DB,
		grpcAddr: cfg.GRPCAddr,
	}
}

func (m *Module) Info() contracts.ModuleInfo {
	return contracts.ModuleInfo{
		ID:           m.id,
		Name:         "Database Postgres",
		Version:      "0.1.2",
		Roles:        []string{"infrastructure"},
		Description:  "PostgreSQL database provider (pgx)",
		Author:       "MuxCore",
		Capabilities: []string{contracts.CapabilityDatabase, "database.postgres", "settings", "backupable"},
		HTTPAddr:     m.grpcAddr,
	}
}

func (m *Module) Init(ctx context.Context) error {
	m.cfgMu.RLock()
	cfg := m.dbCfg
	m.cfgMu.RUnlock()
	d, err := db.Open(cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	m.database = d
	m.srv = server.New(d)

	lis, err := net.Listen("tcp", m.grpcAddr)
	if err != nil {
		_ = d.Close(ctx)
		return fmt.Errorf("listen %s: %w", m.grpcAddr, err)
	}
	m.lis = lis
	slog.Info("database-postgres initialized", "addr", m.grpcAddr, "host", cfg.Host, "database", cfg.Database, "schema", cfg.EffectiveSchema())
	return nil
}

func (m *Module) Start(ctx context.Context) error {
	m.grpcSrv = grpc.NewServer()
	m.srv.RegisterWithGRPC(m.grpcSrv)
	modulesdk.RegisterSettings(m.grpcSrv, m.id, m)
	go func() {
		slog.Info("database-postgres gRPC service started", "addr", m.grpcAddr)
		if err := m.grpcSrv.Serve(m.lis); err != nil {
			slog.Error("database-postgres gRPC serve error", "error", err)
		}
	}()
	return nil
}

func (m *Module) Stop(ctx context.Context) error {
	if m.grpcSrv != nil {
		m.grpcSrv.GracefulStop()
	}
	if m.srv != nil {
		m.srv.Drain()
	}
	if m.database != nil {
		_ = m.database.Close(ctx)
	}
	slog.Info("database-postgres stopped")
	return nil
}

func (m *Module) Health(ctx context.Context) error {
	if m.database == nil {
		return fmt.Errorf("not initialized")
	}
	return m.database.Health(ctx)
}
