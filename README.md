# graph-code-challenge

Task Manager microservice — Golang developer hiring evaluation for GRAPH.

Work in progress. Architecture, API reference and design trade-offs are still to be written.

## Running the tests

Unit tests need nothing but Go:

```sh
make test-unit
```

Integration tests run against a real Postgres, in a **separate database** from the
development one, because they apply migrations and delete rows. Start Postgres,
create that database once, then run them:

```sh
docker compose up -d --wait postgres
docker compose exec -T postgres createdb -U postgres task_manager_test
make test-integration
```

The `createdb` step is a one-off per volume — the database persists in the
`postgres-data` volume, so it is only needed on a fresh checkout or after
`docker compose down -v`. If it already exists, `createdb` prints
`database "task_manager_test" already exists` and nothing is harmed.

Connection details come from `.env.test`, which `make test-integration` passes as
an absolute `ENV_FILE` (`go test` runs each package from its own directory, so a
relative `.env` lookup would miss the repo root). The tests refuse to run unless
the database name ends in `_test`, so a mis-set `ENV_FILE` fails loudly instead of
migrating the development database.

For combined unit + integration coverage as a single merged figure:

```sh
make coverage
```

## Migrations

Migrations are applied deliberately, never on API startup:

```sh
go run ./cmd/migrate up       # apply everything pending
go run ./cmd/migrate down     # roll back the most recent migration
go run ./cmd/migrate version  # print the applied version
```

The SQL is embedded in the binary, so the command is self-contained. It reads
`DATABASE_URL` from `.env` or the environment; `-database` overrides it.
