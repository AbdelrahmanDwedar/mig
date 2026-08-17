# JSON Migrations

`mig` supports two migration file formats: the original raw-`.sql` format
(`-- +migrate Up` / `-- +migrate Down` markers) and a structured JSON format.
Both are first-class — `.sql` and `.json` migration files can coexist in the
same `migrations/` directory, applied in chronological (filename) order
regardless of format.

## 1. Overview

**Why JSON exists:** raw SQL migrations are simple but give you no structure
to build on — no portable column types, no validation before you hit the
database, nothing an agent or tool can introspect without parsing SQL text.
The JSON format expresses the same schema changes (tables, columns, indexes,
foreign keys, constraints) as structured, portable data, translated to
correct native SQL per database driver, while still allowing a raw `sql` op
as an escape hatch for anything the structured DSL can't express.

**When to use `.sql` vs `.json`:** reach for `.json` for anything the
structured ops below cover — it's driver-portable and less error-prone. Drop
to `.sql` (or mix in a `{"op": "sql", ...}` step within a `.json` file) for
views, triggers, partial indexes, or anything else outside the DSL's scope.

**Format detection for existing files** is purely by extension: a migration
named `..._add_users.sql` is parsed as raw SQL, `..._add_users.json` is
parsed as structured JSON. There is no content sniffing.

**Format selection for new files**, i.e. what `mig create <name>` scaffolds,
follows this precedence:

1. `--format` flag, if explicitly passed on the `create` command.
2. `migrations.parser` in `mig.yml`.
3. Default: `sql`.

```bash
# One-off override — scaffolds a JSON migration regardless of mig.yml
mig create add_users --format json
```

```yaml
# mig.yml — project-wide default so every `mig create` scaffolds JSON
migrations:
  parser: json
  dir: migrations
```

```bash
# With the config above, this scaffolds JSON without needing the flag
mig create add_users
```

A JSON Schema is available at [`json-migrations.schema.json`](json-migrations.schema.json)
for editor/agent-side validation and autocomplete. It's a derived reference
artifact, not enforced by `mig` itself — point your editor or agent at it
with a documented (not required) convention:

```json
{
  "$schema": "./docs/json-migrations.schema.json",
  "up": [],
  "down": []
}
```

## 2. File shape

Every JSON migration is a single envelope:

```json
{
  "up": [ /* ops, run top to bottom */ ],
  "down": [ /* ops, run top to bottom */ ]
}
```

- Both `up` and `down` are **required arrays of ops** — there is no
  auto-derived rollback. Write the down migration explicitly, mirroring the
  existing `.sql` format's explicit `+migrate Down` section.
- Ops within each array execute strictly top-to-bottom, each compiling to one
  or more native SQL statements executed in the same transaction the driver
  already wraps migrations in.
- An empty `down: []` is valid — same as an empty `.sql` Down section, it
  makes rollback a silent no-op (schema stays, tracking row is deleted).

## 3. Full op reference

Every op is an object with an `"op"` discriminator field. Field tables below
list JSON keys; Go-side these map onto `internal/sqlgen` structs
(`CreateTableOp`, `ColumnOp`, etc. — see that package for the authoritative
shape).

### `create_table`

| Field | Type | Required | Default |
|---|---|---|---|
| `table` | string | yes | — |
| `if_not_exists` | bool | no | `false` |
| `columns` | array of [Column](#column-object) | yes | — |
| `indexes` | array of [Index](#index-object) | no | `[]` |
| `foreign_keys` | array of [ForeignKey](#foreignkey-object) | no | `[]` |
| `checks` | array of [Check](#check-object) | no | `[]` |

```json
{
  "op": "create_table",
  "table": "users",
  "columns": [
    { "name": "id", "type": "integer", "primary_key": true, "auto_increment": true },
    { "name": "email", "type": "string", "length": 255, "nullable": false, "unique": true }
  ],
  "indexes": [{ "name": "idx_users_email", "columns": ["email"], "unique": true }]
}
```

Generated SQL:

| Postgres | MySQL | SQLite |
|---|---|---|
| `CREATE TABLE "users" ( "id" SERIAL, "email" VARCHAR(255) NOT NULL UNIQUE, PRIMARY KEY ("id") ); CREATE UNIQUE INDEX "idx_users_email" ON "users" ("email")` | ``CREATE TABLE `users` ( `id` INT AUTO_INCREMENT, `email` VARCHAR(255) NOT NULL UNIQUE, PRIMARY KEY (`id`) ); CREATE UNIQUE INDEX `idx_users_email` ON `users` (`email`)`` | `CREATE TABLE "users" ( "id" INTEGER PRIMARY KEY AUTOINCREMENT, "email" VARCHAR(255) NOT NULL UNIQUE ); CREATE UNIQUE INDEX "idx_users_email" ON "users" ("email")` |

Note SQLite folds the auto-increment PK into one inline column clause
instead of a separate `PRIMARY KEY (...)` constraint — see
[Column type reference](#4-column-type-reference) for why.

### `drop_table`

| Field | Type | Required | Default |
|---|---|---|---|
| `table` | string | yes | — |
| `if_exists` | bool | no | `false` |

```json
{ "op": "drop_table", "table": "users", "if_exists": true }
```

All three dialects: `DROP TABLE IF EXISTS "users"` (backticks on MySQL).

### `rename_table`

| Field | Type | Required |
|---|---|---|
| `from` | string | yes |
| `to` | string | yes |

```json
{ "op": "rename_table", "from": "old_name", "to": "new_name" }
```

All three dialects: `ALTER TABLE "old_name" RENAME TO "new_name"`.

### `add_column`

| Field | Type | Required |
|---|---|---|
| `table` | string | yes |
| `column` | [Column](#column-object) | yes |

```json
{ "op": "add_column", "table": "organizations", "column": { "name": "plan", "type": "string", "length": 50, "default": { "literal": "free" } } }
```

All three dialects: `ALTER TABLE "organizations" ADD COLUMN "plan" VARCHAR(50) DEFAULT 'free'`.

### `drop_column`

| Field | Type | Required |
|---|---|---|
| `table` | string | yes |
| `column` | string | yes |

```json
{ "op": "drop_column", "table": "organizations", "column": "plan" }
```

All three dialects: `ALTER TABLE "organizations" DROP COLUMN "plan"`.

### `rename_column`

| Field | Type | Required |
|---|---|---|
| `table` | string | yes |
| `from` | string | yes |
| `to` | string | yes |

```json
{ "op": "rename_column", "table": "users", "from": "email", "to": "email_address" }
```

All three dialects: `ALTER TABLE "users" RENAME COLUMN "email" TO "email_address"`.

### `alter_column`

| Field | Type | Required | Default |
|---|---|---|---|
| `table` | string | yes | — |
| `column` | string | yes | — |
| `type` | string | no* | omitted = unchanged |
| `length` / `precision` / `scale` | int | no | see [type reference](#4-column-type-reference) |
| `nullable` | bool | no | omitted = unchanged |
| `default` | [Default](#default-object) | no | omitted = unchanged |

\* **MySQL requires `type`** even when only `nullable`/`default` is
changing — `MODIFY COLUMN` redefines the entire column, there's no
standalone "change nullability only" clause.

```json
{ "op": "alter_column", "table": "users", "column": "age", "type": "bigint", "nullable": false, "default": { "literal": 0 } }
```

| Postgres | MySQL |
|---|---|
| `ALTER TABLE "users" ALTER COLUMN "age" TYPE BIGINT, ALTER COLUMN "age" SET NOT NULL, ALTER COLUMN "age" SET DEFAULT 0` | ``ALTER TABLE `users` MODIFY COLUMN `age` BIGINT NOT NULL DEFAULT 0`` |

**SQLite does not support `alter_column`** — see
[Known limitations](#6-known-limitations).

### `add_index`

| Field | Type | Required |
|---|---|---|
| `table` | string | yes |
| `index` | [Index](#index-object) | yes |

```json
{ "op": "add_index", "table": "users", "index": { "name": "idx_users_created_at", "columns": ["created_at"] } }
```

Postgres/SQLite: `CREATE INDEX "idx_users_created_at" ON "users" ("created_at")`.
MySQL: same, with backticks. Supported on all three dialects, including
SQLite, post-table-creation — index statements are standalone, not
`ALTER TABLE`.

### `drop_index`

| Field | Type | Required |
|---|---|---|
| `table` | string | yes (MySQL only — indexes are table-scoped there) |
| `name` | string | yes |

```json
{ "op": "drop_index", "table": "users", "name": "idx_users_created_at" }
```

Postgres/SQLite: `DROP INDEX "idx_users_created_at"`.
MySQL: ``DROP INDEX `idx_users_created_at` ON `users` ``.

### `add_foreign_key`

| Field | Type | Required |
|---|---|---|
| `table` | string | yes |
| `foreign_key` | [ForeignKey](#foreignkey-object) | yes |

```json
{ "op": "add_foreign_key", "table": "posts", "foreign_key": { "name": "fk_posts_author", "columns": ["author_id"], "ref_table": "users", "ref_columns": ["id"], "on_delete": "cascade" } }
```

Postgres/MySQL: `ALTER TABLE "posts" ADD CONSTRAINT "fk_posts_author" FOREIGN KEY ("author_id") REFERENCES "users" ("id") ON DELETE CASCADE`.

**Not supported on SQLite post-creation** — bake foreign keys into
`create_table` instead. See [Known limitations](#6-known-limitations).

### `drop_foreign_key`

| Field | Type | Required |
|---|---|---|
| `table` | string | yes |
| `name` | string | yes |

```json
{ "op": "drop_foreign_key", "table": "posts", "name": "fk_posts_author" }
```

Postgres: `ALTER TABLE "posts" DROP CONSTRAINT "fk_posts_author"`.
MySQL: ``ALTER TABLE `posts` DROP FOREIGN KEY `fk_posts_author` ``.
Not supported on SQLite.

### `add_constraint`

| Field | Type | Required |
|---|---|---|
| `table` | string | yes |
| `check` | [Check](#check-object) | yes |

```json
{ "op": "add_constraint", "table": "users", "check": { "name": "chk_users_age_positive", "expr": "age >= 0" } }
```

Postgres/MySQL: `ALTER TABLE "users" ADD CONSTRAINT "chk_users_age_positive" CHECK (age >= 0)`.
Not supported on SQLite post-creation.

### `drop_constraint`

| Field | Type | Required |
|---|---|---|
| `table` | string | yes |
| `name` | string | yes |

```json
{ "op": "drop_constraint", "table": "users", "name": "chk_users_age_positive" }
```

Postgres: `ALTER TABLE "users" DROP CONSTRAINT "chk_users_age_positive"`.
MySQL: ``ALTER TABLE `users` DROP CHECK `chk_users_age_positive` ``.
Not supported on SQLite.

### `sql` (escape hatch)

| Field | Type | Required |
|---|---|---|
| `query` | string | yes |

```json
{ "op": "sql", "query": "CREATE MATERIALIZED VIEW active_users AS SELECT * FROM users WHERE is_active = TRUE" }
```

Passed through verbatim to the driver — no dialect translation, no
validation. Use for views, triggers, partial/expression indexes, or any
dialect-specific SQL the structured DSL doesn't model.

## 4. Column type reference

| Abstract | PostgreSQL | MySQL | SQLite |
|---|---|---|---|
| `string(len)` | `VARCHAR(len)` | `VARCHAR(len)` | `VARCHAR(len)` |
| `text` | `TEXT` | `TEXT` | `TEXT` |
| `integer` | `INTEGER` / `SERIAL` | `INT` / `INT AUTO_INCREMENT` | `INTEGER` |
| `bigint` | `BIGINT` / `BIGSERIAL` | `BIGINT` / `BIGINT AUTO_INCREMENT` | `INTEGER` |
| `boolean` | `BOOLEAN` | `TINYINT(1)` | `BOOLEAN` |
| `uuid` | `UUID` | `CHAR(36)` | `TEXT` |
| `timestamp` | `TIMESTAMP` | `DATETIME` | `DATETIME` |
| `date` | `DATE` | `DATE` | `DATE` |
| `decimal(p,s)` | `NUMERIC(p,s)` | `DECIMAL(p,s)` | `NUMERIC(p,s)` |
| `json` | `JSONB` | `JSON` | `TEXT` |
| `float` | `DOUBLE PRECISION` | `DOUBLE` | `REAL` |

### Column object

| Field | Type | Required | Default |
|---|---|---|---|
| `name` | string | yes | — |
| `type` | string | yes | — (one of the abstract types above) |
| `length` | int | no | `255` (only meaningful for `string`) |
| `precision` / `scale` | int | no | `10` / `0` (only meaningful for `decimal`) |
| `nullable` | bool | no | `true` |
| `primary_key` | bool | no | `false` |
| `unique` | bool | no | `false` |
| `auto_increment` | bool | no | `false` |
| `default` | [Default](#default-object) | no | none |

### Default object

```json
{ "literal": 0 }
{ "expr": "now()" }
```

- `literal` — a quoted/escaped literal value (string, bool, or number).
- `expr` — a verbatim SQL expression, e.g. `now()`, `gen_random_uuid()`.
  Exactly one of the two should be set.

### Auto-increment behavior per dialect

- **Postgres**: `integer`/`bigint` columns render as `SERIAL`/`BIGSERIAL`.
  Works fine as part of a composite primary key.
- **MySQL**: `integer`/`bigint` columns get `AUTO_INCREMENT` appended.
  Works fine as part of a composite primary key.
- **SQLite**: the auto-increment column **must be the sole primary key
  column** and must be `integer` or `bigint` — it renders inline as
  `INTEGER PRIMARY KEY AUTOINCREMENT` instead of a type + separate
  `PRIMARY KEY (...)` constraint. A composite primary key with
  `auto_increment: true` on one column is rejected with a validation error.

### Index object

| Field | Type | Required |
|---|---|---|
| `name` | string | yes |
| `columns` | array of string | yes |
| `unique` | bool | no (default `false`) |

### ForeignKey object

| Field | Type | Required |
|---|---|---|
| `name` | string | no (anonymous constraint if omitted) |
| `columns` | array of string | yes |
| `ref_table` | string | yes |
| `ref_columns` | array of string | yes |
| `on_delete` / `on_update` | string | no (`cascade`, `restrict`, `set null`, ...) |

### Check object

| Field | Type | Required |
|---|---|---|
| `name` | string | no |
| `expr` | string | yes — raw SQL boolean expression |

## 5. Worked examples

**Minimal table:**

```json
{
  "up": [{ "op": "create_table", "table": "tags", "columns": [
    { "name": "id", "type": "integer", "primary_key": true, "auto_increment": true },
    { "name": "name", "type": "string", "length": 100, "nullable": false }
  ]}],
  "down": [{ "op": "drop_table", "table": "tags", "if_exists": true }]
}
```

**Composite primary key:**

```json
{
  "up": [{ "op": "create_table", "table": "role_users", "columns": [
    { "name": "role_id", "type": "integer", "primary_key": true },
    { "name": "user_id", "type": "integer", "primary_key": true }
  ]}],
  "down": [{ "op": "drop_table", "table": "role_users", "if_exists": true }]
}
```

**Unique index + named foreign key with `on_delete`/`on_update`:**

```json
{
  "up": [{
    "op": "create_table",
    "table": "posts",
    "columns": [
      { "name": "id", "type": "integer", "primary_key": true, "auto_increment": true },
      { "name": "slug", "type": "string", "length": 255 },
      { "name": "author_id", "type": "integer", "nullable": false }
    ],
    "indexes": [{ "name": "idx_posts_slug", "columns": ["slug"], "unique": true }],
    "foreign_keys": [{ "name": "fk_posts_author", "columns": ["author_id"], "ref_table": "users", "ref_columns": ["id"], "on_delete": "cascade", "on_update": "cascade" }]
  }],
  "down": [{ "op": "drop_table", "table": "posts", "if_exists": true }]
}
```

**Check constraint:**

```json
{
  "up": [{
    "op": "create_table",
    "table": "accounts",
    "columns": [{ "name": "balance", "type": "decimal", "precision": 10, "scale": 2, "default": { "literal": 0 } }],
    "checks": [{ "name": "chk_accounts_balance_nonneg", "expr": "balance >= 0" }]
  }],
  "down": [{ "op": "drop_table", "table": "accounts", "if_exists": true }]
}
```

**Adding/dropping/renaming a column on an existing table:**

```json
{
  "up": [
    { "op": "add_column", "table": "users", "column": { "name": "nickname", "type": "string", "length": 50 } },
    { "op": "rename_column", "table": "users", "from": "nickname", "to": "display_name" }
  ],
  "down": [
    { "op": "drop_column", "table": "users", "column": "display_name" }
  ]
}
```

**`alter_column` changing type, nullable, and default** — errors on
SQLite, see [Known limitations](#6-known-limitations):

```json
{
  "up": [{ "op": "alter_column", "table": "users", "column": "age", "type": "bigint", "nullable": false, "default": { "literal": 0 } }],
  "down": [{ "op": "alter_column", "table": "users", "column": "age", "type": "integer", "nullable": true }]
}
```

**Adding and dropping an index/foreign key after table creation:**

```json
{
  "up": [
    { "op": "add_index", "table": "users", "index": { "name": "idx_users_created_at", "columns": ["created_at"] } },
    { "op": "add_foreign_key", "table": "posts", "foreign_key": { "name": "fk_posts_author", "columns": ["author_id"], "ref_table": "users", "ref_columns": ["id"] } }
  ],
  "down": [
    { "op": "drop_foreign_key", "table": "posts", "name": "fk_posts_author" },
    { "op": "drop_index", "table": "users", "name": "idx_users_created_at" }
  ]
}
```

**Using the `sql` escape hatch alongside structured ops:**

```json
{
  "up": [
    { "op": "add_column", "table": "organizations", "column": { "name": "plan", "type": "string", "length": 50, "default": { "literal": "free" } } },
    { "op": "sql", "query": "CREATE MATERIALIZED VIEW active_users AS SELECT * FROM users WHERE is_active = TRUE" }
  ],
  "down": [
    { "op": "sql", "query": "DROP MATERIALIZED VIEW IF EXISTS active_users" },
    { "op": "drop_column", "table": "organizations", "column": "plan" }
  ]
}
```

**A full multi-table migration:**

```json
{
  "up": [
    {
      "op": "create_table",
      "table": "users",
      "columns": [
        { "name": "id", "type": "uuid", "primary_key": true, "default": { "expr": "gen_random_uuid()" } },
        { "name": "email", "type": "string", "length": 255, "nullable": false, "unique": true },
        { "name": "balance", "type": "decimal", "precision": 10, "scale": 2, "default": { "literal": 0 } },
        { "name": "is_active", "type": "boolean", "nullable": false, "default": { "literal": true } },
        { "name": "created_at", "type": "timestamp", "nullable": false, "default": { "expr": "now()" } }
      ],
      "indexes": [{ "name": "idx_users_email", "columns": ["email"], "unique": true }]
    },
    {
      "op": "create_table",
      "table": "organizations",
      "columns": [
        { "name": "id", "type": "integer", "primary_key": true, "auto_increment": true },
        { "name": "name", "type": "string", "length": 255, "nullable": false }
      ]
    },
    { "op": "add_column", "table": "users", "column": { "name": "org_id", "type": "integer" } },
    { "op": "add_foreign_key", "table": "users", "foreign_key": { "name": "fk_users_org_id", "columns": ["org_id"], "ref_table": "organizations", "ref_columns": ["id"], "on_delete": "cascade" } }
  ],
  "down": [
    { "op": "drop_foreign_key", "table": "users", "name": "fk_users_org_id" },
    { "op": "drop_column", "table": "users", "column": "org_id" },
    { "op": "drop_table", "table": "organizations", "if_exists": true },
    { "op": "drop_table", "table": "users", "if_exists": true }
  ]
}
```

**`.sql` and `.json` coexisting in the same `migrations/` directory:**

```
migrations/
  2026_07_23_120000_create_organizations.sql
  2026_07_23_121000_add_users.json
```

`mig status` orders purely by filename (timestamp prefix), independent of
format:

```
2026_07_23_120000_create_organizations.sql: Applied
2026_07_23_121000_add_users.json: Applied
```

## 6. Known limitations

**SQLite** does not support, via `ALTER TABLE`:

- `alter_column` — changing a column's type, nullable-ness, or default
  after creation.
- `add_foreign_key` / `drop_foreign_key` — foreign keys can only be declared
  inline in `create_table`.
- `add_constraint` / `drop_constraint` — check constraints can only be
  declared inline in `create_table`.

**Workarounds**, in order of preference:

1. Bake the desired shape into `create_table` up front (define FKs/checks at
   table-creation time instead of adding them later).
2. Use the `sql` escape hatch with SQLite's manual rebuild-table pattern
   (`CREATE TABLE new`, `INSERT INTO new SELECT ...`, `DROP TABLE old`,
   `ALTER TABLE new RENAME TO old`) for changes needed after the fact.

`add_index` / `drop_index` are **not** affected — index statements are
standalone (`CREATE INDEX` / `DROP INDEX`), not `ALTER TABLE`, so they work
on SQLite post-creation like the other two dialects.

**MySQL's `alter_column`** requires `type` to be specified even when only
`nullable`/`default` is changing, because `MODIFY COLUMN` redefines the
entire column definition — there's no standalone "change nullability only"
clause in MySQL.

## 7. Error reference

All JSON-migration errors are plain-text messages (no structured error
codes yet — see the CLI's general machine-readability backlog for that).
Common ones:

| Error | Cause |
|---|---|
| `unknown migration op: "..."` | `"op"` doesn't match any of the 14 supported operations. |
| `migration op is missing required "op" field` | An entry in `up`/`down` has no `"op"` key. |
| `unsupported column type: "..."` | `"type"` isn't one of the abstract types in the [type reference](#4-column-type-reference). |
| `unsupported dialect: "..."` | Internal — `database.driver` in `mig.yml` isn't `postgresql`/`mysql`/`sqlite`. |
| `sqlite auto_increment column "..." must be the sole primary key column` | `auto_increment: true` set on a column that's part of a composite primary key on SQLite. |
| `sqlite auto_increment is only supported for integer/bigint columns` | `auto_increment: true` on a non-integer column. |
| `mysql alter_column requires "type" to be specified` | An `alter_column` op targeting MySQL omitted `"type"`. |
| `alter_column on <table>.<column> specifies no changes` (Postgres) | An `alter_column` op set none of `type`/`nullable`/`default`. |
| `sqlite does not support alter_column ...` | `alter_column` targeting a SQLite database — see [Known limitations](#6-known-limitations). |
| `sqlite does not support adding/dropping foreign keys/constraints ...` | `add_foreign_key`/`drop_foreign_key`/`add_constraint`/`drop_constraint` targeting SQLite. |
| `unsupported format: "..."` (must be "sql" or "json") | `--format` flag or `migrations.parser` in `mig.yml` set to something other than `sql`/`json`. |
