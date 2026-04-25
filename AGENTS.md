# Goliath — Codebase Reference

## Project Overview

Goliath is a self-hosted RSS aggregator with a Go backend and React/TypeScript frontend. It exposes two feed reader–compatible APIs (Fever and Google Reader) alongside a gRPC admin interface, and continuously fetches feeds in the background.

## Repository Structure

```
goliath/
├── backend/              # Go backend
│   ├── goliath.go        # Main binary entry point
│   ├── api/              # REST API handlers (Fever, Google Reader)
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
├── schema/               # SQL schema files
│   ├── base.sql          # Base schema
│   ├── latest.sql        # Current schema
│   └── migrations/       # Numbered migration files (v1–v19)
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
| `backend/api/greader.go` | Google Reader API subset handler |
| `backend/auth/` | Cookie-based session auth; MD5 API key (`md5(user:pass)`) |
| `backend/fetch/` | Feed fetching loop, HTML sanitisation (bluemonday), favicon extraction, cuckoo-filter deduplication |
| `backend/cache/` | In-process retrieval cache persisted to DB on shutdown |
| `backend/admin/` | gRPC admin service; protobuf types in `proto/` |
| `backend/opml/` | OPML import/export via CLI flags |

### Notable dependencies

- `github.com/jrupac/rss` — RSS parsing
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

Schema migrations live in `schema/migrations/` (v1–v19). Apply via `goliath-cli migrate-schema`.

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

CLI commands: feed CRUD, mute-word management, schema migration, compose operations, SQL shell.

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

## CI/CD

GitHub Actions (`.github/workflows/ci.yml`) runs on push to `master`:
1. Compile protobufs
2. `go test ./backend/...`
3. `bun run test` (frontend)

## API Compatibility

| API | Mount path | Protocol |
|-----|-----------|----------|
| Fever v3 | `/fever/` | HTTP JSON |
| Google Reader | `/greader/` | HTTP JSON |
| Admin | port 9997 | gRPC/Protobuf |
| Image proxy | `/cache` | HTTP reverse proxy |
