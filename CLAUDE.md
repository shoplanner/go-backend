# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

Build system is [Task](https://taskfile.dev) (`taskfile.yml`). Note: `task`, `golangci-lint` and `swag` are not
necessarily installed on PATH — the plain `go` equivalents are listed alongside.

```bash
task generate        # swag init (-g cmd/backend/main.go) + go generate ./...
task build           # runs generate, then: go build -ldflags="-w -s" -o bin/backend cmd/backend/main.go
task run             # builds and runs: bin/backend --config config/backend.yml
task lint            # golangci-lint run
task fmt             # swag fmt + go fmt ./...
task export          # docker buildx image -> shoplanner.tar

go build ./...                       # compile everything (needs CGO_ENABLED=1, see below)
go test ./...                        # all tests
go test ./pkg/hashing/ -run TestHash  # single test
go test ./pkg/hashing/ -run 'TestHash/TestName' -v   # single case inside a testify suite
```

`CGO_ENABLED=1` and a C toolchain are required: the SQLite driver is `mattn/go-sqlite3` (pulled in via
`gorm.io/driver/sqlite`). The Dockerfile installs `build-base sqlite-dev` for this reason.

Code generators are Go tools declared in `go.mod` (`tool` directives) and invoked through Python shims in `tools/`,
so `python3` is also required for `task generate`:

- `tools/goenum.py` — runs `go-enum` on files carrying `//go:generate python $GOENUM`. Triggered by `// ENUM(...)`
  comments above an integer type (see `internal/backend/list/model.go`). Output: `*.enum.gen.go.go` — generated,
  never edit.
- `tools/sqlc_helper.py` — copies `config/sqlc.yaml` into the repo package, runs `sqlc generate`, deletes the copy.
  Triggered by `//go:generate python $SQLC_HELPER` in repo packages that have `schema.sql` + `query.sql`
  (`internal/backend/user/repo`, `internal/backend/shopmap/repo`). Output goes to that package's `sqlgen/`.
  The sqlc engine is still configured as `mysql` even though the app now runs on SQLite.

Both shims depend on env vars exported by `taskfile.yml` (`PROJECT_ROOT`, `GOENUM`, `SQLC_HELPER`), so run generation
via `task generate`, not bare `go generate`.

## Runtime configuration

Two separate sources, both required:

- **YAML file** (`--config`, default `/etc/backend.yaml`; repo copy at `config/backend.yml`) — listener host/port/net
  and JWT token lifetimes. Parsed by `internal/backend/config.ParseConfig`.
- **Environment** — `DB_PATH` (SQLite file) and `AUTH_PRIVATE_KEY` (a *path* to a PEM file holding a PKCS#8 ECDSA
  private key, despite the name). Parsed by `config.ParseEnv`. `LOG_NO_SYSLOG=1` sends logs to stderr instead of
  syslog (see `cmd/backend/logger_syslog.go` / `logger_windows.go`, split by build tag).

`README.md` is stale: it documents a MySQL/Dolt + Redis deployment and a `task deps` target that no longer exist.
Trust `internal/backend/config/config.go` and `config/systemd/shoplanner.service` instead.

## Architecture

Single Gin HTTP server, `cmd/backend/main.go`, which is the only wiring point: it opens both a `database/sql` handle
and a GORM handle over the *same* SQLite file, constructs every repo → service → API handler by hand, and registers
them on the router. There is no DI container; adding a feature means adding a block to `main`.

Each domain under `internal/backend/<domain>/` follows the same four-part layout:

- `model.go` — domain types, enums (`// ENUM(...)`), invariants. Package name is the bare domain (`list`, `user`).
- `repo/` — persistence. Two coexisting styles: **sqlc-generated** queries (`user`, `shopmap` — files still named
  `dolt.go` for historical reasons) and **GORM** with `AutoMigrate` (`product`, `favorite`, `list`). Repos create
  their own tables at construction time (`InitUsers`, `AutoMigrate`, …), so there are no migration files.
- `service/` — business logic. Services declare the repo interface they need *locally* (consumer-side interfaces,
  e.g. `type repo interface {...}` in `list/service/service.go`); repos never import services.
- `api/` — Gin handlers, one `RegisterREST(group, service, log)` per domain.

Auth is layered in `main` by registration order: `auth` and `user` routes are registered on `/api/v1` *before*
`apiGroup.Use(jwtMiddleware.Middleware())`, so everything registered after it is authenticated. Handlers read the
caller via `authAPI.GetUserID(ctx)`, which the middleware puts in the Gin context. Access/refresh tokens are ES256
JWTs (`auth/provider/jwt.go`); `auth/repo/redis.go` exists but is currently not wired into `main`.

Real-time list updates: `list/service` keeps an in-memory `map[providerID]*eventProvider` of per-(user, list)
channels; mutations fan out `list.Event` values and `list/api/websocket.go` streams them over
`GET /api/v1/lists/:id/ws`. This state is process-local — the service is not horizontally scalable as written.

### Shared conventions (`pkg/`)

- `pkg/myerr` — the five sentinel errors (`ErrInvalidArgument`, `ErrNotFound`, `ErrAlreadyExists`, `ErrForbidden`,
  `ErrInternal`). Services wrap these with `fmt.Errorf("%w: ...")`; `pkg/api/rest/rerr.BaseHandler.HandleError`
  maps them to HTTP status codes. Return one of these from a service or the API will report 500.
- `pkg/api/rest/rerr` — embed `rerr.BaseHandler` in every handler struct for `HandleError`/`Decode`, and use
  `rerr.PathID[T]`/`rerr.QueryID[T]` for UUID params.
- `pkg/id` — `id.ID[T]` is a phantom-typed UUID; `pkg/date` does the same for `CreateDate[T]`/`UpdateDate[T]`.
  Cross-domain references are typed (`id.ID[user.User]`), so keep them typed rather than passing raw strings.
- Optional values use `mo.Option[T]` (`samber/mo`) in domain models, with `swaggertype`/`x-nullable` tags so Swagger
  still renders them. `samber/lo` is the standard slice-mapping helper.
- Logging is `zerolog`, passed down by value; each layer does `log.With().Str("component", "...").Logger()`.

### Swagger

Handlers carry `@Summary`/`@Router`/`@Security ApiKeyAuth` annotations; `task generate` regenerates `docs/`
(`docs.go`, `swagger.json`, `swagger.yaml` — generated, do not hand-edit). The UI is served at
`/api/v1/swagger/index.html`. The auth header is literally `Auth: Bearer <token>`, not `Authorization`.

## Linting

`.golangci.yml` enables a very large linter set (`exhaustruct`, `wrapcheck`, `err113`, `gochecknoglobals`, `mnd`,
`lll` at 120 chars, …). Practical consequences when writing code here:

- `exhaustruct` — struct literals must list every field, which is why zero-valued fields are spelled out explicitly
  throughout the codebase. Match that style.
- `wrapcheck` — wrap every error crossing a package boundary with `fmt.Errorf(...: %w)`.
- `err113` — no dynamic `errors.New` at call sites; define sentinels or wrap `myerr` values.
- `nolintlint` requires an explanation comment on `//nolint` directives (except `funlen`, `gocognit`, `lll`).
- `testpackage` — tests live in `package <pkg>_test`; existing tests use `stretchr/testify` suites.

`.golangci.bck.yml` is an unused backup; ignore it.
