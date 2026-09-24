# graph-code-challenge

Task Manager microservice — Golang developer hiring evaluation for GRAPH.

Work in progress. Architecture, API reference and design trade-offs are still to be written.

## Running with Docker

```sh
docker compose up --build -d
docker compose exec api /migrate up
curl http://localhost:8080/healthz
```

That brings up `postgres`, `redis` and `api` on port 8080; the API waits for both
data stores to report healthy before it starts. Migrations stay a deliberate,
separate step — never run on API startup, and no migration container runs behind
your back on `up` — so apply them once with the second command. It runs the
`/migrate` binary that the same image already carries, inside the API container,
so it reuses the API's `DATABASE_URL` and reaches Postgres by its compose
hostname. (`exec` needs no shell in the image: Docker runs the binary directly.)
`go run ./cmd/migrate up` from the host does the same job against
`localhost:5432`.

The image is multi-stage: `golang:1.26-alpine` builds static binaries, and the
final stage is `gcr.io/distroless/static-debian12:nonroot` — no shell, no package
manager, running as an unprivileged user. Because there is no shell, the API's
compose healthcheck cannot be a `wget`/`curl` one-liner; a third tiny binary,
`/healthcheck`, ships in the image and exits non-zero unless `/healthz` answers
200.

Rebuild after a code change with `docker compose up --build -d api`, and tear the
stack down with `docker compose down` (add `-v` to drop the database volume too).

## Caching

`GET /tasks/:id` and `GET /tasks` are served through a cache-aside layer in
`internal/repository/redis/redistask`. It is a decorator implementing the same
`taskservice.Repository` interface as the Postgres adapter and wrapping it, so no
layer above the repository knows caching exists — it is composed in `main.go` and
nowhere else. Set `REDIS_URL` to enable it; leave it empty and the API talks
straight to Postgres.

Single tasks are trivial to invalidate: one key per id (`task:{id}`), dropped on
that id's update or delete.

Lists are the hard case. Every status/assignee/cursor/limit combination is its own
key, and given "task 42 changed" there is no way to ask Redis which of those keys
are affected — the payloads are opaque to it, and membership *after* a write isn't
derivable from membership *before* it (flip a task from `pending` to `done` and it
must leave every `status=pending` page **and** join `status=done` pages it was
never in). Scanning the keyspace on every write is O(keyspace); a reverse index of
"which pages contain task 42" needs one index per filter dimension, each with its
own TTL bookkeeping, and a bug in it serves silently wrong data indefinitely.

So this implementation doesn't look for the keys — it **orphans** them. Each page
key carries a generation read from `tasks:list:gen`:

```
tasks:list:{gen}:{status}:{assignee}:{cursor}:{limit}
```

Any write `INCR`s that counter. Nothing is enumerated and nothing is deleted; the
old keys simply become unreachable, because no later read will ever construct a
key with the previous generation again. They fall out on their own TTL.

Every key expires, the counter included. Pages live for `REDIS_TTL` (default 60s);
the counter lives ten times that, because it has to outlive the pages it stamped —
if it expired first it would reset to 0 while generation-0 pages were still alive
and bring them back. `INCR` doesn't refresh a TTL, so the `EXPIRE` travels with it
in one pipelined round trip.

Every cache operation is best-effort: a Redis error makes a read fall through to
Postgres and a write proceed anyway, because a cache must never be able to take
the API down. Invalidation runs on `context.WithoutCancel`, so a client that
disconnects the instant after its write still gets its stale entries dropped.

### When this is the wrong design

Retiring every page on every write is deliberate over-invalidation, so the list
cache is worth exactly the repeat reads that land between two consecutive writes:

- **Read-heavy** — a board polling `GET /tasks` every few seconds against a
  handful of writes a minute. A generation survives tens of seconds and serves
  hundreds of reads. Clear win.
- **Write-heavy** — writes arriving about as often as reads. The hit rate
  collapses toward zero while every request still pays `GET gen` + `GET page` +
  `SET page`. Strictly worse than no cache at all.

This service assumes the first. If that stopped holding, the two ways out are
**TTL-only caching with no invalidation** (accept 5–10s of staleness, drop the
counter entirely, and write rate stops mattering — what most listing endpoints
actually do) or **per-filter generations** (`tasks:list:gen:{status}:{assignee}`),
which keeps the hit rate up under writes but has to bump the counters for both the
task's old and new status/assignee, so an update must read the task first.

## Running the tests

Unit tests need nothing but Go:

```sh
make test-unit
```

Integration tests run against a real Postgres and a real Redis, both kept
**separate** from the development ones — a `task_manager_test` database, because
they apply migrations and delete rows, and Redis index 15, because they flush it.
Start both, create that database once, then run them:

```sh
docker compose up -d --wait postgres redis
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
the database name ends in `_test` and the Redis index is not 0, so a mis-set
`ENV_FILE` fails loudly instead of migrating the development database or flushing
the development cache.

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
