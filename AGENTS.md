# Goliath — Codebase Reference

## Project Overview

Goliath is a self-hosted RSS aggregator with a Go backend and React/TypeScript frontend. It exposes two feed reader–compatible APIs (Fever and GReader) alongside a gRPC admin interface, and continuously fetches feeds in the background.

## Conventions

### Code comments

Comments must be freestanding and timeless. A comment outlives whatever
motivated it, so it should describe why the code is the way it is in general
terms, not what was found or changed.

Do not put in a comment:

- dates, or anything anchored to "recently" / "now" / "as of";
- references to external resources, including files in this repo (`docs/`,
  `plans/`, "see the spec");
- named third-party clients that merely happen to be what was tested — say
  "clients" and describe the behavior;
- benchmark numbers, latencies, percentiles, or counts from a specific run;
- history of the code ("X used to do Y", "this was broken because...").

That detail belongs in the commit message.

## Repository Structure

```
goliath/
├── backend/              # Go backend
│   ├── goliath.go        # Main binary entry point
│   ├── api/              # REST API handlers (Fever, GReader)
│   ├── auth/             # Auth middleware and login/logout
│   ├── cache/            # Retrieval cache layer
│   ├── fetch/            # Feed fetching, parsing, deduplication
│   ├── models/           # Domain models (User, Feed, Folder, Article)
│   ├── opml/             # OPML import/export
│   ├── storage/          # Database interface + CockroachDB implementation
│   └── admin/            # gRPC admin service (protobuf-generated)
├── frontend/             # React + TypeScript frontend
│   ├── src/
│   │   ├── App.tsx       # Main application component (class-based)
│   │   ├── components/   # ArticleList, FolderFeedList, ArticleCard, etc.
│   │   ├── api/          # API client interfaces and implementations
│   │   ├── models/       # Frontend data models (ContentTree)
│   │   ├── themes/       # CSS themes (default.css, dark.css)
│   │   └── utils/        # Helpers, type definitions, lossless JSON
│   ├── vite.config.js    # Vite config with PWA plugin
│   └── package.json
├── cli/                  # goliath-cli admin CLI (Go)
├── backend/schema/       # SQL schema files
│   ├── base.sql          # Base schema
│   ├── latest.sql        # Current schema
│   └── vNN_*.sql         # Numbered migration files
├── proto/                # Protobuf definitions for admin gRPC service
├── config-dist.ini       # Configuration template
├── compose.yaml          # Docker Compose (prod/dev/debug profiles)
├── Dockerfile            # Multi-stage build (backend, frontend, cli targets)
├── Makefile              # Build and install CLI
└── go.work               # Go workspace (multi-module)
```

## Backend

**Language:** Go 1.25.0  
**HTTP server:** stdlib `net/http` (no framework)  
**Default ports:** HTTP `9999`, Prometheus metrics `9998`, gRPC admin `9997`

### Key packages

| Package | Responsibility |
|---------|---------------|
| `backend/goliath.go` | Startup: initialises DB, starts HTTP server, metrics server, feed-fetch goroutine, graceful shutdown |
| `backend/storage/db.go` | `Database` interface — all DB operations |
| `backend/storage/crdb.go` | CockroachDB implementation (no ORM, raw SQL) |
| `backend/storage/gc.go` | Background garbage collection of old articles |
| `backend/api/fever.go` | Fever API v3 handler |
| `backend/api/greader.go` | GReader API subset handler |
| `backend/auth/` | Cookie-based session auth; MD5 API key (`md5(user:pass)`) |
| `backend/fetch/` | Feed fetching loop, HTML sanitisation (bluemonday), favicon extraction, cuckoo-filter deduplication |
| `backend/cache/` | In-process retrieval cache persisted to DB on shutdown |
| `backend/admin/` | gRPC admin service; protobuf types in `proto/` |
| `backend/opml/` | OPML import/export via CLI flags |

### Notable dependencies

- `github.com/jrupac/rss/v2` — feed fetching and parsing (RSS, Atom, JSON Feed)
- `github.com/PuerkitoBio/goquery` — HTML parsing
- `github.com/microcosm-cc/bluemonday` — HTML sanitisation
- `github.com/mat/besticon/v3` — favicon extraction
- `github.com/seiflotfy/cuckoofilter` — probabilistic deduplication
- `github.com/kljensen/snowball` — stemming for dedup
- `github.com/prometheus/client_golang` — Prometheus metrics
- `golang/glog` — structured logging

## Frontend

**Framework:** React 18 + TypeScript  
**Build tool:** Vite 7  
**Package manager:** Bun  
**UI library:** Material-UI (MUI) v7 + Emotion  
**Testing:** Vitest + Happy-DOM  
**Linting/formatting:** Oxlint + Prettier

### Running tests

```bash
cd frontend && bun run test
```

### Dev server

```bash
cd frontend && bun run start   # http://localhost:3000
```

### Production build

```bash
cd frontend && bun run build   # outputs to frontend/build/
```

The backend serves the built static files from `/` and `/static/`.

### Notable frontend details

- `App.tsx` is a large class component managing global state with optimistic updates.
- Custom lossless JSON handling (`utils/`) for large-number precision (feed IDs from CockroachDB are 64-bit integers).
- PWA support via Workbox; LRU cache for in-memory data structures.
- Theme switching (`t` key), keyboard shortcuts for navigation.

## Database

**Engine:** CockroachDB (PostgreSQL-compatible distributed SQL)  
**Default DSN:** `postgresql://goliath@localhost:26257/goliath?sslmode=disable`

### Key tables

| Table | Purpose |
|-------|---------|
| `UserTable` | Users — UUID PK, username, API key, password hash |
| `Folder` / `FolderChildren` | Feed folders and hierarchy |
| `Feed` | Feed metadata, URL, favicon |
| `Article` | Articles — content, read/saved status |
| `UserPrefs` | Per-user mute words, unmuted feed list |
| `RetrievalCache` | Persisted fetch-state cache |

Schema migrations live in `backend/schema/` as `vNN_<description>.sql`. Each
declares in its leading comments whether binaries built before it keep running
once it is applied (`-- older-binaries: compatible` or `incompatible`). A new
migration is mirrored in `latest.sql`, which also records the version it stamps
a new database with. The `SchemaVersion` table records which migrations a
database has had; the backend refuses to start on a database behind its newest
migration, or ahead of it on one declared incompatible.

`goliath-cli migrate-schema` applies pending migrations: it stops the
application, checkpoints the database, applies and records each migration, and
restarts, printing the `goliath-cli rollback-schema` command that undoes it.
Checkpoints are CockroachDB backups; list them with
`goliath-cli list-checkpoints`. `--database` points these commands at a copy,
for rehearsing.

## Configuration

Copy `config-dist.ini` to `config.ini` (git-ignored). Sections:

| Section | Key settings |
|---------|-------------|
| `[general]` | `port`, `metrics_port`, `public_folder` |
| `[storage]` | `db_uri`, GC settings |
| `[fetcher]` | HTML sanitisation, favicon, article parsing options |
| `[opml]` | Import/export paths |

## Build and Deployment

### Docker Compose profiles

```bash
docker-compose --profile prod  up   # single production container
docker-compose --profile dev   up   # hot-reload frontend + backend, mounted volumes
docker-compose --profile debug up   # adds delve debugger on port 40000
```

### CLI (goliath-cli)

```bash
make build    # Docker build → ./dist/goliath-cli
make install  # installs to /usr/local/bin/goliath-cli
```

CLI commands: user management (`add-user`, `list-users`, `delete-user`,
`restore-user`), sessions
and password changes, feed CRUD, mute-word management, schema migration and
rollback, deploys (`upgrade`, `rollback`), compose operations, SQL shell.

`goliath-cli upgrade` is how a deployment is updated: it fast-forwards the
checkout, rebuilds the CLI and continues in it, builds the new image while the
old one runs, checkpoints, migrates, starts, and waits for `/version` to report
the new build and schema. The replaced image stays tagged `:previous`, and
`goliath-cli rollback` returns to it, restoring the checkpoint too when a
migration since would stop the old binary. What was deployed is logged in
`.goliath/deploys.jsonl` on the host.

### Admin gRPC

```bash
grpc_cli call localhost:9997 AdminService.GetFeeds 'Username: "user"'
```

## Testing

```bash
# Backend
go test ./backend/...

# Frontend
cd frontend && bun run test

# Lint frontend
cd frontend && bun run lint
```

Tests named `*_live_test.go` need a real CockroachDB and are skipped without
one. Most take `GOLIATH_TEST_DB`, the connection string of a throwaway
database.

The multi-user end-to-end test (`backend/e2e_live_test.go`) runs a whole
server in-process -- HTTP handlers, admin service, fetch scheduler -- against a
database it creates and drops, with a local server standing in for feed
publishers. It needs a cluster connection that can create databases:

```bash
cd backend
# Correctness, with the race detector
GOLIATH_E2E_CRDB='postgresql://root@localhost:26257/defaultdb?sslmode=disable' \
  go test -race -run TestMultiUserEndToEnd -v .
# Users start with a share of an existing database's feeds and articles
GOLIATH_E2E_SEED_DB=<database> ...
# Sizing run: 100 users x 500 feeds; take timings without -race
GOLIATH_E2E_SCALE=large GOLIATH_E2E_REPORT=report.json ... -timeout 120m
```

It ends with a table of per-request latencies.

Nothing a test does should reach a real feed publisher: fetching is per user,
so many test users multiply the traffic to every site they share.
`backend/feedtest` serves synthetic feeds from a local address for that, with
items published on demand and requests counted per feed. `backend/devseed`
copies a database's feeds and articles to test users, moving each feed onto
that server. For trying clients by hand,
`go run ./devseed/seedtesters -db <new> -source <database>` (from `backend/`)
builds such a database, prints the users' credentials and the flags to point a
locally run server at it, and serves the feeds until interrupted.

## CI/CD

GitHub Actions (`.github/workflows/ci.yml`) runs on push to `master`:
1. Compile protobufs
2. `go test ./backend/...`
3. `bun run test` (frontend)

## API Compatibility

| API | Mount path | Protocol |
|-----|-----------|----------|
| Fever v3 | `/fever/` | HTTP JSON |
| GReader | `/greader/` | HTTP JSON |
| Admin | port 9997 | gRPC/Protobuf |
| Image proxy | `/cache` | HTTP reverse proxy |
