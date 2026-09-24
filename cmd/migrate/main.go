package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"graph-code-challenge/internal/config"
	"graph-code-challenge/internal/migrator"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	database := flag.String("database", "", "postgres connection URL (overrides DATABASE_URL)")
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		usage()

		return errors.New("no command given")
	}

	dsn := *database
	if dsn == "" {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		dsn = cfg.DatabaseURL
	}

	m, err := migrator.New(dsn)
	if err != nil {
		return err
	}
	defer m.Close()

	switch command := args[0]; command {
	case "up":
		return up(m)
	case "down":
		return down(m)
	case "version":
		return version(m)
	default:
		usage()

		return fmt.Errorf("unknown command %q", command)
	}
}

func up(m *migrator.Migrator) error {
	switch err := m.Up(); {
	case errors.Is(err, migrator.ErrNoChange):
		fmt.Println("schema already up to date")
	case err != nil:
		return err
	default:
		fmt.Println("migrations applied")
	}

	return reportVersion(m)
}

func down(m *migrator.Migrator) error {
	switch err := m.Down(); {
	case errors.Is(err, migrator.ErrNoChange):
		fmt.Println("nothing to roll back")
	case err != nil:
		return err
	default:
		fmt.Println("rolled back one migration")
	}

	return reportVersion(m)
}

func version(m *migrator.Migrator) error {
	return reportVersion(m)
}

func reportVersion(m *migrator.Migrator) error {
	applied, dirty, err := m.Version()
	if err != nil {
		return err
	}

	if applied == 0 {
		fmt.Println("version: none applied")

		return nil
	}

	fmt.Printf("version: %d\n", applied)

	if dirty {
		fmt.Println("WARNING: schema is dirty — a migration failed partway.")
		fmt.Println("Repair the database by hand, then clear the dirty flag in schema_migrations.")
	}

	return nil
}

func usage() {
	fmt.Fprint(os.Stderr, `Apply the embedded SQL migrations to Postgres.

Usage:
  migrate [-database URL] up       apply every pending migration
  migrate [-database URL] down     roll back the most recently applied migration
  migrate [-database URL] version  print the applied version

The database URL is read from .env / the environment as DATABASE_URL.

Flags:
  -database URL   postgres connection URL (overrides DATABASE_URL)
`)
}
