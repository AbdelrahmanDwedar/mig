# Schema Scanners and `mig inspect`

`mig inspect` reads a live database's actual schema and prints everything it
finds: tables, columns, indexes, foreign keys, check constraints, views,
triggers, and sequences. It is read-only — it never generates or runs SQL,
and it does not compare what it finds against your migration files. That
comparison (`mig diff`) is a separate, not-yet-built feature; `mig inspect`
is the introspection layer it will eventually build on.

## 1. Overview

```bash
mig inspect          # human-readable tree
mig inspect --json   # structured output
```

Both read `mig.yml` the same way every other `mig` command does (`database`
for connection details) and print the schema of whatever database that
config points at.

## 2. The Scanner abstraction

Introspection is a third concern in `mig`'s architecture, parallel to (and
independent of) the two that already exist:

| Concern | Package | Job |
|---|---|---|
| Connect / run migrations | `internal/db` (`Driver`) | Open a connection, apply/rollback SQL |
| Render SQL for an op | `internal/sqlgen` (`Dialect`) | Turn a portable op into dialect-native SQL |
| Read the live schema | `internal/scanner` (`Scanner`) | Turn a live database into a `Schema` value |

`Driver` never introspects, and `Scanner` never generates SQL — it only
reads. `Driver` exposes its open connection via `Conn() *sql.DB` for
`Scanner` to use directly; `scanner.New(driverName, conn)` picks the
concrete scanner the same way `db.NewDriver` and `sqlgen.New` pick their
concrete implementations, keyed off the same `database.driver` string from
`mig.yml`.

## 3. The Schema model

`Table.Columns`, `Table.ForeignKeys`, and `Table.Checks` reuse the exact
same portable types the JSON migration format uses —
[`sqlgen.Column`](json-migrations.md#3-full-op-reference),
`sqlgen.ForeignKey`, and `sqlgen.Check`. A scanned schema is meant to be fed
directly into future diff/generate work without a translation layer, so
there's no separate, parallel set of "scanner types" for the portable parts.

`Table.Indexes` wraps `sqlgen.Index` with fields a portable index
definition can't express, because **different databases have genuinely
different index features** — Postgres has index methods (btree, gin, gist,
brin), partial indexes, and expression indexes; MySQL has FULLTEXT and
SPATIAL indexes:

```go
type Index struct {
    sqlgen.Index
    Method  string            // e.g. "btree", "gin", "fulltext"
    Partial string            // partial-index predicate, if any
    Extra   map[string]string // anything else worth surfacing
}
```

A single-column unique constraint with no such extras (a plain btree,
non-partial unique index) folds into `Column.Unique` instead of appearing
as a separate `Index` entry — this matches how you'd actually author it in
a JSON migration (`"unique": true` on the column). A single-column unique
index that *does* carry dialect-specific detail (a partial predicate, a
non-default method) stays a visible `Index` entry instead, so that detail
isn't silently dropped. Composite unique constraints always stay as `Index`
entries, since `Column.Unique` can't express more than one column.

## 4. The raw-native fallback

Not every native column type has a portable abstract equivalent — an enum,
an array type, `hstore`, or any other dialect-specific extension. When a
scanned column's native type doesn't reverse-map cleanly, `Column.Type` is
set to the sentinel `"raw"` and the actual native type string is preserved
on `Column.NativeType`, instead of guessing or erroring:

```json
{
  "name": "tags",
  "type": "raw",
  "native_type": "text[]"
}
```

The reverse mapping is `sqlgen.AbstractType(dialect, nativeType)`, the
inverse of `sqlgen.NativeColumnType` — see the
[column type reference](json-migrations.md#4-column-type-reference) for the
abstract type table both directions share.

**Column defaults** are always represented as `Default.Expr` (a raw SQL
expression string), never reverse-parsed into a typed `Default.Literal` —
catalogs report defaults as raw SQL text (`'foo'::varchar`,
`CURRENT_TIMESTAMP`, `0`), and distinguishing "quoted literal" from
"expression" from that text alone isn't reliable across dialects.

## 5. `--json` output shape

```json
{
  "tables": {
    "users": {
      "name": "users",
      "columns": [
        {"name": "id", "type": "integer", "primary_key": true, "auto_increment": true},
        {"name": "email", "type": "string", "length": 255, "nullable": false, "unique": true}
      ],
      "indexes": [],
      "foreign_keys": [],
      "checks": []
    }
  },
  "views": {
    "active_users": {"name": "active_users", "definition": "SELECT * FROM users WHERE ..."}
  },
  "triggers": {
    "trg_users_touch": {"name": "trg_users_touch", "table": "users", "timing": "AFTER", "event": "UPDATE"}
  },
  "sequences": {}
}
```

Not every dialect has every object kind — **SQLite and MySQL have no
user-facing sequences**, so `sequences` is always `{}` there. `_migrations`
(mig's own bookkeeping table) is always excluded from `tables`.

## 6. Known limitations

- **Single schema only.** Postgres scanning is scoped to the `public`
  schema; multi-schema databases are not yet supported.
- **No stored procedures/functions, materialized views, or table
  partitioning.** These aren't modeled — they're invisible to `mig inspect`
  for now, the same way `docs/json-migrations.md` treats unmodeled DDL:
  the escape hatch is dropping to raw SQL in your migrations, not scanning.
- **SQLite has no PRAGMA for CHECK constraints**, so `Table.Checks` is
  always empty on SQLite even if the table actually has one (SQLite still
  enforces it — `mig inspect` just can't see it via PRAGMA introspection).
- **SQLite's type affinity loses information.** `bigint`/`integer` both
  render as `INTEGER`, and `uuid`/`json`/`text` all render as `TEXT` — a
  bare SQLite `INTEGER` column reverse-maps to `"integer"` and a bare `TEXT`
  column reverse-maps to `"text"` by convention (see `typeOrder` in
  `internal/sqlgen/typemap.go`), not because the original abstract type was
  recoverable from the catalog.
- **Foreign key names on SQLite** are never populated — SQLite's
  `PRAGMA foreign_key_list` doesn't surface a constraint name.
