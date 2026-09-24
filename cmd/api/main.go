package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"graph-code-challenge/internal/config"
	"graph-code-challenge/internal/delivery/httpserver"
	"graph-code-challenge/internal/delivery/httpserver/taskhandler"
	"graph-code-challenge/internal/logger"
	"graph-code-challenge/internal/logger/sloglogger"
	"graph-code-challenge/internal/metrics"
	"graph-code-challenge/internal/profiler"
	"graph-code-challenge/internal/repository/postgres"
	"graph-code-challenge/internal/repository/postgres/postgrestask"
	"graph-code-challenge/internal/repository/redis"
	"graph-code-challenge/internal/repository/redis/redistask"
	"graph-code-challenge/internal/service/taskservice"
	"graph-code-challenge/internal/validator/taskvalidator"
)

const (
	shutdownTimeout          = 10 * time.Second
	taskCountRefreshInterval = 15 * time.Second
	taskCountRefreshTimeout  = 2 * time.Second
)

type managedServer interface {
	Serve() error
	Shutdown(context.Context) error
}

type namedServer struct {
	name   string
	server managedServer
}

type serverResult struct {
	name string
	err  error
}

// @title Task Manager API
// @version 1.0
// @description REST API for creating, reading, updating, listing and soft-deleting tasks.
// @BasePath /
// @schemes http
func main() {
	log := sloglogger.New(os.Stdout, slog.LevelInfo)

	if err := run(log); err != nil {
		log.Error(context.Background(), "application stopped", "error", err)
		os.Exit(1)
	}
}

func run(log logger.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := postgres.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	var repo taskservice.Repository = postgrestask.New(db)

	if cfg.RedisURL != "" {
		cache, err := redis.New(context.Background(), cfg.RedisURL)
		if err != nil {
			return err
		}
		defer cache.Close()

		repo = redistask.New(repo, cache, cfg.RedisTTL, log)

		log.Info(context.Background(), "cache-aside enabled", "ttl", cfg.RedisTTL)
	}

	taskSvc := taskservice.New(repo)
	handler := taskhandler.New(taskSvc, taskvalidator.New(), log)

	appMetrics := metrics.New()

	server := httpserver.New(cfg, handler, appMetrics, log)
	server.Setup()

	servers := []namedServer{{name: "api", server: server}}
	if cfg.PProfEnabled {
		servers = append(servers, namedServer{name: "pprof", server: profiler.New(cfg.PProfAddr())})
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go refreshTaskGauge(ctx, log, appMetrics, taskSvc)

	serveResult := make(chan serverResult, len(servers))
	for _, current := range servers {
		go func() {
			serveResult <- serverResult{name: current.name, err: current.server.Serve()}
		}()
	}

	log.Info(context.Background(), "api listening", "address", cfg.Addr())
	if cfg.PProfEnabled {
		log.Info(context.Background(), "pprof listening", "address", cfg.PProfAddr())
	}

	var runErr error
	completedServers := 0

	select {
	case result := <-serveResult:
		completedServers = 1
		if result.err != nil {
			runErr = fmt.Errorf("%s: %w", result.name, result.err)
		} else {
			runErr = fmt.Errorf("%s server stopped unexpectedly", result.name)
		}
	case <-ctx.Done():
		stop()
		log.Info(context.Background(), "shutdown signal received, draining connections")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	for _, current := range servers {
		if err := current.server.Shutdown(shutdownCtx); err != nil {
			runErr = errors.Join(runErr, err)
		}
	}

	for range len(servers) - completedServers {
		result := <-serveResult
		if result.err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("%s: %w", result.name, result.err))
		}
	}

	if runErr != nil {
		return runErr
	}

	log.Info(context.Background(), "shutdown complete")

	return nil
}

func refreshTaskGauge(ctx context.Context, log logger.Logger, appMetrics *metrics.Metrics, reader metrics.TaskCountReader) {
	refresh := func() {
		refreshCtx, cancel := context.WithTimeout(ctx, taskCountRefreshTimeout)
		defer cancel()

		if err := appMetrics.RefreshTaskGauge(refreshCtx, reader); err != nil && ctx.Err() == nil {
			log.Warn(ctx, "task count metric refresh failed", "error", err)
		}
	}

	refresh()

	ticker := time.NewTicker(taskCountRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			refresh()
		case <-ctx.Done():
			return
		}
	}
}
