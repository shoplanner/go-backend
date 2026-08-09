# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

Build system is [Task](https://taskfile.dev) (`taskfile.yml`). Note: `task`, `golangci-lint` and `swag` are not
necessarily installed on PATH — the plain `go` equivalents are listed alongside.

```bash
task generate        # swag init (-g cmd/backend/main.go) + go generate ./...
task build           # runs generate, then: go build -ldflags="-w -s" -o bin/backend cmd/backend/main.go
task run             # builds and runs: bin/backend --config config/backend.yml
task test            # go test ./... -race
task test:update     # regenerates internal/backend/functest/testdata (see Regression suite)
task lint            # golangci-lint run
task fmt             # swag fmt + go fmt ./...
task export          # docker buildx image -> shoplanner.tar

go build ./...                       # compile everything (needs CGO_ENABLED=1, see below)
go test ./...                        # all tests
go test ./pkg/hashing/ -run TestHash  # single test
go test ./pkg/hashing/ -run 'TestHash/TestName' -v   # single case inside a testify suite
```

`CGO_ENABLED=1` and a C toolchain are required: the SQLite driver is `mattn/go-sqlite3`. The Dockerfile
installs `build-base sqlite-dev` for this reason.

Code generators are Go tools declared in `go.mod` (`tool` directives) and invoked through Python shims in `tools/`,
so `python3` is also required for `task generate`:

- `tools/goenum.py` — runs `go-enum` on files carrying `//go:generate python $GOENUM`. Triggered by `// ENUM(...)`
  comments above an integer type (see `internal/backend/list/model.go`). Output: `*.enum.gen.go.go` — generated,
  never edit.
- `tools/sqlc_helper.py` — copies `config/sqlc.yaml` into the repo package, runs `sqlc generate`, deletes the copy.
  Triggered by `//go:generate python $SQLC_HELPER` in every repo package; each has `schema.sql` + `query.sql` and
  gets its own `sqlgen/`. The sqlc engine is `sqlite`, which has no `:copyfrom` — bulk inserts are plain `:exec`
  queries in a loop inside a transaction (see `shopmap/repo/sqlite.go`).

  Two sqlc limitations shape the code. It generates **nothing** for a `CREATE INDEX` statement, so every unique
  index is additionally spelled out as a `createIndexQuery` const next to the repo and executed with a raw
  `ExecContext`; the statement is duplicated in `schema.sql` so that file stays the whole schema. And it types a
  column without `NOT NULL` as `sql.Null*` even when nothing ever writes a NULL there, which the `overrides` block
  in `config/sqlc.yaml` pins back to plain Go types — that file is copied into every package, so the overrides are
  the union across all of them.

Both shims depend on env vars exported by `taskfile.yml` (`PROJECT_ROOT`, `GOENUM`, `SQLC_HELPER`), so run generation
via `task generate`, not bare `go generate`.

## Runtime configuration

Two separate sources, both required:

- **YAML file** (`--config`, default `/etc/backend.yaml`; repo copy at `config/backend.yml`) — listener host/port/net
  and JWT token lifetimes. Parsed by `internal/backend/config.ParseConfig`.
- **Environment** — `DB_PATH` (SQLite file) and `AUTH_PRIVATE_KEY` (a *path* to a PEM file holding a PKCS#8 ECDSA
  private key, despite the name). Parsed by `config.ParseEnv`. `LOG_NO_SYSLOG=1` sends logs to stderr instead of
  syslog (see `cmd/backend/logger_syslog.go` / `logger_windows.go`, split by build tag).

`internal/backend/config/config.go` and `config/systemd/shoplanner.service` are the authority on what is actually
read at startup.

## Architecture

Single Gin HTTP server, `cmd/backend/main.go`, which is the only wiring point: it opens one `database/sql` handle
over the SQLite file, constructs every repo → service → API handler by hand, and registers them on the router.
There is no DI container; adding a feature means adding a block to `main`. Construction order matters — `user`
first, because the member tables of `favorite` and `list` declare foreign keys against `users`.

Each domain under `internal/backend/<domain>/` follows the same four-part layout:

- `model.go` — domain types, enums (`// ENUM(...)`), invariants. Package name is the bare domain (`list`, `user`).
- `repo/` — persistence: sqlc-generated queries over `database/sql`, in `sqlite.go`. Repos create their own
  tables at construction time from the `Init*` queries in their `schema.sql`, so there are no migration files
  and every `CREATE TABLE` is `IF NOT EXISTS`.

  A repo only owns the tables in its own `schema.sql`. When one domain needs to read another's rows — `list` and
  `favorite` both embed products, `list` members carry a login — it calls an exported loader on the owning
  package: `productRepo.Load(ctx, db, ids, LoadOptions{…})` and `userRepo.LoadLogins(ctx, db, ids)`. Both take a
  `sqlgen.DBTX` rather than a `*sql.DB`, which is what lets the caller pass its own `*sql.Tx` and read everything
  inside one transaction. That is the replacement for GORM's nested `Preload`s, and `LoadOptions` exists because
  the read paths genuinely differ: a list state's product is loaded with its category and forms, its *replacement*
  product without forms, and the favorites write path loads products bare.
- `service/` — business logic. Services declare the repo interface they need *locally* (consumer-side interfaces,
  e.g. `type repo interface {...}` in `list/service/service.go`); repos never import services.
- `api/` — Gin handlers, one `RegisterREST(group, service, log)` per domain.

Auth is layered in `main` by registration order: `auth` and `user` routes are registered on `/api/v1` *before*
`apiGroup.Use(jwtMiddleware.Middleware())`, so everything registered after it is authenticated. Handlers read the
caller via `authAPI.GetUserID(ctx)`, which the middleware puts in the Gin context. Access/refresh tokens are ES256
JWTs (`auth/provider/jwt.go`) and are fully stateless: there is no token store, so `Logout` is a no-op and
issued tokens cannot be revoked before they expire.

Real-time list updates: `list/service` keeps an in-memory `map[providerID]*eventProvider` of per-(user, list)
channels; mutations fan out `list.Event` values and `list/api/websocket.go` streams them over
`GET /api/v1/lists/:id/ws`. This state is process-local — the service is not horizontally scalable as written.

### Shared conventions (`pkg/`)

- `pkg/mysqlite` — maps `sqlite3.Error` constraint failures onto `myerr` sentinels (`GetType`,
  `IsUniqueViolation`, `IsForeignKeyViolation`). Use it wherever a driver error has to become a status code.
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

## Regression suite (`internal/backend/functest`)

`internal/backend/functest` is a cross-domain functional suite written to make the GORM → sqlc migration
verifiable, and kept afterwards as the storage regression suite. It wires the whole backend minus HTTP exactly
as `cmd/backend/main.go` does — one `database/sql` handle over one temp SQLite file — and drives every service
against it. Nothing is mocked.

Two committed artifact sets under `testdata/` are the actual contract:

- `schema.sql` — the physical schema the repo constructors produce, regenerated with `task test:update`
  (`go test ./internal/backend/functest/ -update`) and **reviewed by hand**. There are no migration files in this
  project, so this snapshot is the only written-down description of the on-disk format. A diff means either the
  new schema is compatible with deployed files, or a migration is owed. `normalizeDDL` in `fixtures_test.go`
  strips identifier quoting, collapses whitespace and sorts constraint clauses, so the comparison is about
  structure rather than about which generator wrote the statement.
- `golden/*.json` — canonicalised snapshots of domain models, also regenerated by `-update`. UUIDs become
  `<uuid:N>` (stable per value, so relationships stay visible) and timestamps become `<ts>`. These catch what
  hand-written assertions miss: nil vs empty slice, `mo.None` vs `mo.Some("")`, a dropped association, a
  reordered collection.

- `legacy/gorm_v1.sql` is different in kind: a byte-exact dump of a database written by the **pre-migration**
  GORM code, and **frozen input that nothing regenerates**. `legacy_test.go` loads it and reads it back through
  the current services; that is what proves the sqlc code still reads data already on users' disks. The
  generator that produced it was deleted along with GORM on purpose — it drove the scenario through the repos,
  so keeping it would mean `-update` quietly replacing the evidence with a re-recording of current behaviour.
  Never hand-edit this file and never reintroduce a generator for it.

`TestLegacyAndFreshDatabasesHaveTheSameShape` compares a legacy database against a fresh one column by column
via `PRAGMA table_info`. `users` is the one accepted exception: GORM's `AutoMigrate` used to rebuild that table
with its `NOT NULL`s dropped and uniqueness moved into `idx_users_login`, and `CREATE TABLE IF NOT EXISTS`
cannot undo it — so deployed files keep the loose shape while fresh ones get the strict sqlc DDL. Both enforce
the same constraints, and the repo creates `idx_users_login` explicitly so uniqueness is indexed either way.

Known bugs are covered by a pair of tests: `..._CurrentBehaviour` is green and pins today's behaviour, and
`..._Desired` is `t.Skip`ped with a pointer to its partner. When one gets fixed, the pair flips. Do not "fix" a
`_CurrentBehaviour` test to look correct — its whole job is to fail loudly if storage work changes it by accident.

## Linting

`.golangci.yml` enables a very large linter set (`exhaustruct`, `wrapcheck`, `err113`, `gochecknoglobals`, `mnd`,
`lll` at 120 chars, …). Practical consequences when writing code here:

- `exhaustruct` — struct literals must list every field, which is why zero-valued fields are spelled out explicitly
  throughout the codebase. Match that style.
- `wrapcheck` — wrap every error crossing a package boundary with `fmt.Errorf(...: %w)`.
- `err113` — no dynamic `errors.New` at call sites; define sentinels or wrap `myerr` values.
- `nolintlint` requires an explanation comment on `//nolint` directives (except `funlen`, `gocognit`, `lll`).
- `testpackage` — tests live in `package <pkg>_test`; existing tests use `stretchr/testify` suites.
- On `_test.go` the following are switched off: `bodyclose`, `cyclop`, `dupl`, `err113`, `exhaustruct`, `funlen`,
  `gochecknoglobals`, `goconst`, `gosec`, `maintidx`, `mnd`, `noctx`, `wrapcheck`. `exhaustruct` in particular
  would otherwise force every nested field of `list.ProductList` to be spelled out in each fixture.
