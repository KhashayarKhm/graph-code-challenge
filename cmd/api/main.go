package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"graph-code-challenge/internal/config"
	"graph-code-challenge/internal/delivery/httpserver"
	"graph-code-challenge/internal/delivery/httpserver/taskhandler"
	"graph-code-challenge/internal/repository/postgres"
	"graph-code-challenge/internal/repository/postgres/postgrestask"
	"graph-code-challenge/internal/service/taskservice"
	"graph-code-challenge/internal/validator/taskvalidator"
)

const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := postgres.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	taskSvc := taskservice.New(postgrestask.New(db))
	handler := taskhandler.New(taskSvc, taskvalidator.New())

	server := httpserver.New(cfg, handler)
	server.Setup()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)

	go func() {
		fmt.Println("listening on", cfg.Addr())
		serveErr <- server.Serve()
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		stop()
		fmt.Println("shutdown signal received, draining connections")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	if err := <-serveErr; err != nil {
		return err
	}

	fmt.Println("shutdown complete")

	return nil
}
