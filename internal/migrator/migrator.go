package migrator

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgxdriver "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"

	"graph-code-challenge/migrations"
)

var ErrNoChange = migrate.ErrNoChange

type Migrator struct {
	migrate *migrate.Migrate
	db      *sql.DB
}

func New(dsn string) (*Migrator, error) {
	if dsn == "" {
		return nil, errors.New("migrator: empty database dsn")
	}

	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("migrator: read embedded migrations: %w", err)
	}

	db, err := sql.Open("pgx/v5", dsn)
	if err != nil {
		return nil, fmt.Errorf("migrator: open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()

		return nil, fmt.Errorf("migrator: connect to database: %w", err)
	}

	driver, err := pgxdriver.WithInstance(db, &pgxdriver.Config{})
	if err != nil {
		db.Close()

		return nil, fmt.Errorf("migrator: init postgres driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		db.Close()

		return nil, fmt.Errorf("migrator: init migrator: %w", err)
	}

	return &Migrator{migrate: m, db: db}, nil
}

func (m *Migrator) Up() error {
	if err := m.migrate.Up(); err != nil {
		return fmt.Errorf("migrator: up: %w", err)
	}

	return nil
}

func (m *Migrator) Down() error {
	if _, _, err := m.migrate.Version(); errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("migrator: down: %w", ErrNoChange)
	}

	if err := m.migrate.Steps(-1); err != nil {
		return fmt.Errorf("migrator: down: %w", err)
	}

	return nil
}

func (m *Migrator) Version() (version uint, dirty bool, err error) {
	version, dirty, err = m.migrate.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, fmt.Errorf("migrator: version: %w", err)
	}

	return version, dirty, nil
}

func (m *Migrator) Close() error {
	sourceErr, dbErr := m.migrate.Close()

	return errors.Join(sourceErr, dbErr)
}
