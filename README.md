# graph-code-challenge

Task Manager microservice — Golang developer hiring evaluation for GRAPH.

Work in progress. Architecture, API reference and design trade-offs are still to be written.

## Running with Docker

```sh
docker compose up --build -d
docker compose run --rm --entrypoint /migrate api up
curl http://localhost:8080/healthz
```

That brings up `postgres`, `redis` and `api` on port 8080; the API waits for both
data stores to report healthy before it starts. Migrations stay a deliberate,
separate step — never run on API startup — so apply them once with the second
command, which reuses the same image and the same `DATABASE_URL` as the API but
on the compose network. `go run ./cmd/migrate up` from the host does the same job
against `localhost:5432`.

The image is multi-stage: `golang:1.26-alpine` builds static binaries, and the
final stage is `gcr.io/distroless/static-debian12:nonroot` — no shell, no package
manager, running as an unprivileged user. Because there is no shell, the API's
compose healthcheck cannot be a `wget`/`curl` one-liner; a third tiny binary,
`/healthcheck`, ships in the image and exits non-zero unless `/healthz` answers
200.

Rebuild after a code change with `docker compose up --build -d api`, and tear the
stack down with `docker compose down` (add `-v` to drop the database volume too).

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
