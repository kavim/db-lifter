# db-lift

High-performance CLI tool for restoring MySQL dump files into Docker containers using zero-copy streaming and a modern terminal UI.

## Why

Restoring large MySQL dumps (multi-GB) into Docker containers is slow and memory-intensive with naive approaches. `db-lift` solves this by streaming the dump file directly into the container's `mysql` process through OS pipes — no intermediate buffers, no temp files, no wasted RAM.

## Features

- **Zero-copy streaming** — the dump file is piped directly from disk into `docker exec -i ... mysql` with no in-memory buffering
- **Optimized MySQL settings** — automatically wraps the import with `SET FOREIGN_KEY_CHECKS=0`, `SET UNIQUE_CHECKS=0`, and `SET AUTOCOMMIT=0` for maximum throughput, then commits and re-enables at the end
- **Real-time progress bar** — tracks bytes transferred against total file size with a high-refresh Bubbletea TUI
- **Spinner states** — visual feedback during the DROP/CREATE DATABASE phase
- **Graceful shutdown** — captures `SIGINT`/`SIGTERM` to cleanly terminate the Docker process and close pipes
- **Configurable** — accepts CLI flags or environment variables (with `.env` file support)

## Prerequisites

- [Go 1.25+](https://go.dev/dl/) (see `go.mod`; for building from source)
- [Docker](https://docs.docker.com/get-docker/) running with a MySQL container
- A `.sql` dump file to restore

## Installation

### Build from source

```bash
git clone https://github.com/kevinmacielmedeiros/db-lift.git
cd db-lift
make build
```

The binary is compiled to `bin/db-lift`. It is a static binary with no external dependencies — copy it anywhere on your `PATH`.

### Run directly

```bash
go run ./cmd/db-lift -f dump.sql -c mysql-container -d mydb
```

## Usage

```
db-lift [flags]
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--file` | `-f` | | Path to the `.sql` dump file **(required)** |
| `--container` | `-c` | | Docker container name or ID running MySQL **(required)** |
| `--database` | `-d` | | Target database name **(required)** — letters, digits, `_`, `-`; max 64 chars |
| `--user` | `-u` | `root` | MySQL user (same character rules as database name) |
| `--password` | `-p` | | MySQL password (prefer `DB_LIFT_PASSWORD` / `.env`) |
| `--recreate-database` | | `false` | **Destructive:** drop and recreate the database before import |
| `--no-tui` | | `false` | Plain log output (also when `stdout` is not a TTY or `CI` is set) |
| `--timeout` | | `0` | Max duration for the whole operation (`0` = no limit) |
| `--env` | `-e` | | Load configuration from an env file (e.g. `--env .env`) |
| `--version` | | | Print version and exit |

### Examples

Restore with explicit flags (drop + recreate database, then import):

```bash
db-lift --recreate-database -f ./backup.sql -c mysql-dev -u root -p secret -d my_app
```

Import only into an existing empty database (no drop):

```bash
db-lift -f ./backup.sql -c mysql-dev -d my_app
```

Restore using a `.env` file:

```bash
db-lift --env .env
```

Mix both — flags override values from the env file:

```bash
db-lift --env .env -f /data/other-dump.sql
```

## Configuration via `.env` file

When you pass `--env`, `db-lift` loads the specified file and maps its variables to the corresponding flags. Any flag explicitly set on the command line takes precedence.

| Variable | Maps to |
|----------|---------|
| `DB_LIFT_FILE` | `--file` |
| `DB_LIFT_CONTAINER` | `--container` |
| `DB_LIFT_USER` | `--user` |
| `DB_LIFT_PASSWORD` | `--password` |
| `DB_LIFT_DATABASE` | `--database` |
| `DB_LIFT_RECREATE` | `true` / `1` enables `--recreate-database` if the flag was not set |
| `DB_LIFT_NO_TUI` | `true` / `1` enables `--no-tui` if the flag was not set |

Copy `.env.example` to `.env` and edit; **do not commit** `.env` (it is gitignored).

Example `.env` file:

```env
DB_LIFT_FILE=./dumps/dump.sql
DB_LIFT_CONTAINER=mysql-dev
DB_LIFT_USER=root
DB_LIFT_PASSWORD=secret
DB_LIFT_DATABASE=my_app
DB_LIFT_RECREATE=true
```

Environment variables already set in your shell are also picked up (even without `--env`). The priority order is: **CLI flags > env vars > defaults**.

## How it works

```
  dump file          ProgressReader           docker exec -i … mysql
 (on disk)     ->   (counts bytes)     ->    stdin: preamble + SQL + epilogue
 |
                           v
                    Bubbletea TUI or plain logs
```

1. **Init** — verifies the Docker container is running
2. **Drop/Create** — only with `--recreate-database` (or `DB_LIFT_RECREATE`); otherwise skipped
3. **Stream** — streams the dump into `mysql` in the container with session tuning (progress or byte updates)
4. **Done** — success summary with elapsed time, or an error with details

The TUI and I/O run on separate goroutines so the interface stays responsive regardless of I/O throughput.

## Project structure

```
.
├── cmd/db-lift/          # CLI entry point (Cobra, signal handling)
│   └── main.go
├── internal/
│   ├── docker/           # Docker + MySQL exec (no shell for stream)
│   ├── progress/         # Thread-safe byte-tracking io.Reader
│   ├── restore/          # Restore pipeline orchestration
│   └── tui/              # Bubbletea terminal UI
├── .github/workflows/    # CI (test, vet, build)
├── go.mod
├── go.sum
├── .env.example          # Template for local config (copy to .env)
└── Makefile
```

## Makefile targets


| Target                | Description                               |
| --------------------- | ----------------------------------------- |
| `make build`          | Compile a static binary to `bin/db-lift`  |
| `make run ARGS="..."` | Run with `go run` (pass flags via `ARGS`) |
| `make clean`          | Remove the `bin/` directory               |
| `make lint`           | Run `golangci-lint`                       |


## License

MIT