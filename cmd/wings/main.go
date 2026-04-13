package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ocelik94/petrel-wings/internal/api"
	"github.com/ocelik94/petrel-wings/internal/config"
	wdocker "github.com/ocelik94/petrel-wings/internal/docker"
	"github.com/ocelik94/petrel-wings/internal/server"
)

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "", "path to wings config file")
	flag.Parse()

	if cfgPath == "" {
		cfgPath = os.Getenv("WINGS_CONFIG")
	}
	if cfgPath == "" {
		cfgPath = "config.yml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		panic(fmt.Errorf("loading config: %w", err))
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx := context.Background()
	dc, err := wdocker.NewClient(cfg.Docker.Socket)
	if err != nil {
		logger.Error("failed to create docker client", "error", err)
		os.Exit(1)
	}
	defer func() {
		if cerr := dc.Close(); cerr != nil {
			logger.Warn("failed to close docker client", "error", cerr)
		}
	}()

	if err := dc.Ping(ctx); err != nil {
		logger.Error("docker ping failed", "error", err)
		os.Exit(1)
	}

	manager := server.NewManager(cfg.DataPath, dc, cfg.Docker.Network)
	if err := manager.Initialize(ctx); err != nil {
		logger.Error("failed to initialize server manager", "error", err)
		os.Exit(1)
	}

	router := api.NewRouter(cfg.Token, manager, logger)
	addr := cfg.API.Host + ":" + cfg.API.Port
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("starting wings api", "addr", addr)
		if cfg.API.TLSCert != "" && cfg.API.TLSKey != "" {
			if err := httpServer.ListenAndServeTLS(cfg.API.TLSCert, cfg.API.TLSKey); err != nil && err != http.ErrServerClosed {
				logger.Error("server error", "error", err)
				os.Exit(1)
			}
			return
		}
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-sigCtx.Done()
	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown failed", "error", err)
	}
	if err := manager.Shutdown(shutdownCtx); err != nil {
		logger.Error("manager shutdown failed", "error", err)
	}
}
