package main

import (
	"context"
	"fmt"
	"os"

	"graph-code-challenge/internal/config"
	"graph-code-challenge/internal/delivery/httpserver"
	"graph-code-challenge/internal/delivery/httpserver/taskhandler"
	"graph-code-challenge/internal/repository/postgres"
	"graph-code-challenge/internal/repository/postgres/postgrestask"
	"graph-code-challenge/internal/service/taskservice"
	"graph-code-challenge/internal/validator/taskvalidator"
)

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

	fmt.Println("listening on", cfg.Addr())

	return server.Serve()
}
