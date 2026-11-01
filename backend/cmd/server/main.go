package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/database"
	"wg-keeper-backend/internal/logging"
	"wg-keeper-backend/internal/metrics"
	"wg-keeper-backend/internal/server"
	"wg-keeper-backend/internal/services"

	"github.com/gin-gonic/gin"

	_ "wg-keeper-backend/docs"
)

// @title        wgkeeper console API
// @version      2.0.0
// @description  WireGuard management console API

// @contact.name  wgkeeper console
// @contact.url   https://github.com/wgkeeper/wgkeeper-console

// @license.name  AGPL-3.0
// @license.url   https://www.gnu.org/licenses/agpl-3.0.html

// @BasePath  /api

// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name wg_session

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(logging.New(os.Stdout, cfg.LogFormat, level))

	if err := database.Init(); err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			slog.Warn("close database", "err", err)
		}
	}()
	sqlDB, err := database.DB.DB()
	if err != nil {
		return fmt.Errorf("database handle: %w", err)
	}

	if cfg.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	slog.Info("starting", "debug", cfg.Debug, "docs", cfg.DocsEnabled, "env", cfg.Environment, "metrics", cfg.Metrics.Enabled())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var m *metrics.Metrics
	svc := services.NewDashboardService()
	metricsDone := make(chan error, 1)
	if cfg.Metrics.Enabled() {
		m = metrics.New()
		m.BindNodeCounter(svc)
		svc.WithTransport(m.InstrumentTransport(services.SharedTransport()))
		metricsSrv := metrics.NewServer(":"+strconv.Itoa(cfg.Metrics.Port), cfg.Metrics.Token, m)
		slog.Info("metrics enabled", "port", cfg.Metrics.Port)
		go func() { metricsDone <- metrics.Run(ctx, metricsSrv) }()
	} else {
		close(metricsDone)
	}

	router, stopRouter, err := server.NewRouter(cfg, server.Deps{
		DB:         sqlDB,
		Service:    svc,
		Metrics:    m,
		StaticRoot: "./public",
	})
	if err != nil {
		return err
	}
	defer stopRouter()

	httpServer := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           router,
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		MaxHeaderBytes:    1 << 13, // 8KB
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", cfg.Server.Port)
		serveErr <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
	case <-ctx.Done():
		slog.Info("shutting down")
	}

	// In-flight requests get ShutdownTimeout to finish; the deferred calls then
	// stop the rate limiters and close the database.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down server: %w", err)
	}
	if err := <-metricsDone; err != nil {
		slog.Warn("metrics server", "err", err)
	}
	return nil
}
