package main

import (
	"context"
	"encoding/json"
	"flag"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/config"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/mcpserver"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() { os.Exit(run()) }
func run() int {
	check := flag.Bool("check-catalog", false, "validate catalog without listening or calling business APIs")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(logger)
	c, err := config.Load()
	if err != nil {
		logger.Error(err.Error())
		return 1
	}
	o, err := config.ReadOverrides(c.OverridesFile)
	if err != nil {
		logger.Error(err.Error())
		return 1
	}
	pc, err := pgxpool.ParseConfig(c.DatabaseURL)
	if err != nil {
		logger.Error("invalid database configuration")
		return 1
	}
	pc.MaxConns = c.MaxConns
	pc.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
	pool, err := pgxpool.NewWithConfig(context.Background(), pc)
	if err != nil {
		logger.Error("database pool initialization failed")
		return 1
	}
	defer pool.Close()
	manager := catalog.NewManager(catalog.PostgresLoader(pool, c.QueryTimeout), o, c.MaxAge, *check)
	if *check {
		if err = manager.Refresh(context.Background()); err != nil {
			logger.Error("catalog check failed", "reason", err.Error())
			return 1
		}
		json.NewEncoder(os.Stdout).Encode(manager.Status())
		return 0
	}
	client, err := gateway.New(c.GatewayURL, c.GatewayTimeout, c.ConnectTimeout, c.MaxResponse, c.MaxResult)
	if err != nil {
		logger.Error(err.Error())
		return 1
	}
	defer client.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	handler := mcpserver.New(manager, client, mcpserver.Options{MaxBody: c.MaxBody, MaxCalls: c.MaxCalls, Hosts: c.Hosts, Origins: c.Origins, Logger: logger})
	server := &http.Server{Addr: c.Listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	refreshDone := make(chan struct{})
	go func() {
		defer close(refreshDone)
		ticker := time.NewTicker(c.Refresh)
		defer ticker.Stop()
		for {
			if err := manager.Refresh(ctx); err != nil {
				logger.Error("catalog refresh failed", "reason", err.Error())
			} else {
				logger.Info("catalog verified", "status", manager.Status())
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	logger.Info("MCP listening", "address", c.Listen)
	select {
	case err = <-done:
		if err != http.ErrServerClosed {
			logger.Error("HTTP server stopped")
			return 1
		}
	case <-ctx.Done():
	}
	manager.Drain()
	shutdown, cancel := context.WithTimeout(context.Background(), c.ShutdownTimeout)
	defer cancel()
	if err = server.Shutdown(shutdown); err != nil {
		server.Close()
		logger.Error("shutdown deadline exceeded; in-flight execution may have completed")
		return 1
	}
	<-refreshDone
	return 0
}
