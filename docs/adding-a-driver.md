# Adding a New Database Driver

Supporting a new database in `mig` means implementing three independent
axes, not one. There is no single `Driver` interface to satisfy — connection
handling, SQL rendering, and schema introspection are deliberately separate
concerns, each with its own per-dialect factory keyed off the same
`database.driver` string from `mig.yml`:

| Axis | Package | Factory | Interface |
|---|---|---|---|
| Connect / run migrations | `internal/db` | `db.NewDriver(cfg)` | `Driver` |
| Render SQL for an op | `internal/sqlgen` | `sqlgen.New(driver)` | `Dialect` |
| Read the live schema | `internal/scanner` | `scanner.New(driver, conn)` | `Scanner` |

They're separate because they change for different reasons and at
different rates: connection/DSN details rarely change, SQL rendering rules
change per-dialect-quirk, and introspection queries change per catalog
format. Bolting introspection onto `Driver`, for example, would mean every
`Driver` implementer (including `MockDriver`, used purely for migration-
execution tests) has to also know how to read a schema it may never
introspect. See [`scanners.md`](scanners.md#2-the-scanner-abstraction) for
the fuller rationale.

Below, `<name>` stands for your new dialect's driver string (e.g. `"mssql"`).

## 1. `internal/db`: connect and execute migrations

- Add `internal/db/<name>.go` implementing `Driver`:
  `Connect`, `Close`, `Conn() *sql.DB`, `EnsureMigrationsTable`,
  `GetAppliedMigrations`, `ApplyMigration`, `RollbackMigration`. Copy
  `postgres.go` or `mysql.go` as a starting point — they differ only in DSN
  construction, placeholder style (`$1` vs `?`), and the `_migrations` table
  DDL (UUID/`gen_random_uuid()` vs `CHAR(36)` + Go-generated UUID).
- Register the new driver string in `db.NewDriver`'s switch
  (`internal/db/factory.go`).

## 2. `internal/sqlgen`: render SQL for each op

- Add `internal/sqlgen/<name>.go` implementing `Dialect` (one method per op:
  `CreateTable`, `AddColumn`, `AddIndex`, etc. — see `dialect.go`). Most of
  the assembly logic is shared via `buildCreateTable` and the other helpers
  in `common.go`; only auto-increment rendering (`autoIncrementRenderer`)
  and any dialect-specific quirks (like SQLite's inability to `ALTER TABLE`
  for FKs/constraints — see
  [`json-migrations.md`'s Known Limitations](json-migrations.md#6-known-limitations))
  need dialect-specific code.
- Add the dialect's native type templates to `typeMap` in
  `internal/sqlgen/typemap.go` for every abstract type (`string`, `text`,
  `integer`, `bigint`, `boolean`, `uuid`, `timestamp`, `date`, `decimal`,
  `json`, `float`). If any of your dialect's native renderings collide with
  another abstract type's rendering for the *same* dialect (the way
  SQLite's `bigint`/`integer` both render as `INTEGER`), add the resolution
  order to `typeOrder`'s doc comment and note the ambiguity in
  [`scanners.md`'s Known Limitations](scanners.md#6-known-limitations) —
  reverse-mapping (`AbstractType`) can't do better than the forward mapping
  allows.
- Register the driver string in `sqlgen.New`'s switch (`dialect.go`).

## 3. `internal/scanner`: read the live schema

- Add `internal/scanner/<name>.go` implementing `Scanner`
  (`Scan(ctx) (*Schema, error)`). Follow the composition pattern used by
  the Postgres/MySQL/SQLite scanners: small private helper methods per
  object kind (`scanColumns`, `scanIndexes`, `scanForeignKeys`,
  `scanChecks`, `scanViews`, `scanTriggers`, `scanSequences`) called from
  `Scan`, rather than one large query.
- **A driver need not support every object kind.** SQLite and MySQL have no
  user-facing sequences; SQLite has no PRAGMA for CHECK constraints. Return
  an empty map (or nil slice) for anything your dialect's catalog doesn't
  expose — never error just because an object kind doesn't apply.
- Write a `<name>NativeType(...)` helper that reconstructs the native type
  string `sqlgen.NativeColumnType` would have produced from however your
  dialect's catalog reports types, then pass that through
  `sqlgen.AbstractType(driverName, native)` to resolve the abstract type.
  This keeps `typeMap` the single source of truth for both directions
  instead of a second, parallel type-mapping table. Fall back to
  `scanner.RawType` + the actual native type string (see
  [`scanners.md` §4](scanners.md#4-the-raw-native-fallback)) for anything
  that doesn't map — don't guess.
- Register the driver string in `scanner.New`'s switch (`scanner.go`).
- **If your new database isn't relational SQL at all** (the Cassandra/CQL
  case originally motivating this three-axis split) — don't force it
  through `sqlgen.Column`/`Index`/`ForeignKey`. Those types encode a
  relational shape (columns, foreign keys, indexes over column lists) that
  a wide-column or document store doesn't share. It's fine, and expected,
  for such a driver's `Scanner` to return its own `Schema`-adjacent types
  instead of reusing the SQL-shaped ones — the `Scanner` interface only
  requires *some* schema representation, not this particular one.

## 4. `mig.yml` / config

- No explicit allow-list currently validates `database.driver` beyond the
  three factory switches above — each factory's `default:` case already
  returns `unsupported driver: %q` for an unrecognized string, so adding the
  driver to all three switches is what makes a `mig.yml` naming it work.
- If the driver name appears anywhere user-facing (CLI help text, the
  `mig setup --driver` flag description in `cmd/mig/main.go`), update it
  there too.

## 5. Testing checklist

Mirror [`scanners.md`'s testing approach](scanners.md) and the existing
per-dialect test files under `tests/db/`, `tests/sqlgen/`, `tests/scanner/`:

- `tests/db/<name>_test.go`: `Connect`/`Close`, `Conn()` returns a usable
  `*sql.DB`, and (if a live server isn't available in CI, following the
  existing Postgres/MySQL test pattern) an unreachable-server case
  asserting the DB-hitting methods fail cleanly rather than hang.
- `tests/sqlgen/<name>_test.go`: one test per op, plus a `typemap_test.go`
  contribution — extend `TestAbstractType_RoundTrip` with your dialect's
  entries (and its `typeOrder` collisions, if any).
- `tests/scanner/<name>_test.go`: seed a schema via raw SQL (not through
  `mig`'s own migration runner — keep the test from circularly validating
  `mig` against itself) covering every abstract type, a composite PK, a
  single- and multi-column unique constraint, a foreign key with non-default
  `ON DELETE`/`ON UPDATE`, a check constraint, a view, a trigger, a
  sequence (if supported), and an empty database.
- Add your dialect to the cross-dialect consistency test (seed the same
  logical schema across all supported dialects, assert equality after
  zeroing dialect-specific fields) so a scanner that reports the same
  reality differently per dialect gets caught immediately.

## 6. Worked example

The SQLite scanner (`internal/scanner/sqlite.go`) is the most recently
added and the most idiosyncratic (PRAGMA-based rather than
`information_schema`-based, implicit auto-increment detection via a keyword
search in the stored `CREATE TABLE` text, no CHECK constraint
introspection) — if your new database is also not a straightforward
`information_schema` catalog, it's the closer model to copy from. If it is
a standard `information_schema` catalog, `internal/scanner/mysql.go` is the
simpler starting point.
