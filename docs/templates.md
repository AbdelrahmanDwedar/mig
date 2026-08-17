# Migration Templates

`mig create` can scaffold common schema operations directly — creating a
table, adding a column, adding an index, and so on — instead of writing an
empty migration and filling it in by hand. Pass `--template <op>` plus the
flags that op needs, and `mig` generates a complete, valid migration file in
whichever format (`.sql` or `.json`) is already configured.

## 1. Overview

Templates build directly on the same op vocabulary and dialect machinery
[JSON migrations](json-migrations.md) use ([`internal/sqlgen`](../internal/sqlgen)),
so there's no separate scaffolding logic to keep in sync:

- For **`.sql`** output, the exact `sqlgen.Dialect` methods used at
  migrate-time render the scaffold — the generated SQL is guaranteed correct
  for your configured driver (Postgres/MySQL/SQLite), not independently
  hand-written text that could drift from what actually gets executed.
- For **`.json`** output, the same op is serialized straight into the
  `{"op": ...}` envelope the JSON parser already consumes — dialect
  resolution still happens later, at migrate-time, exactly as it does for
  hand-written JSON migrations.

Both the `down` side and the scaffold content are generated automatically —
you don't need `--format` beyond what `mig create` already resolves (see
[format precedence](json-migrations.md#1-overview)).

```bash
mig create add_users --template create_table \
  --table users \
  --columns "id:bigint:pk:auto,email:string(255):unique"
```

## 2. Supported templates

| Template | Required flags | Notes |
|---|---|---|
| `create_table` | `--table`, `--columns` | `--if-not-exists` optional |
| `drop_table` | `--table` | `--if-exists` optional |
| `add_column` | `--table`, `--columns` (exactly one column) | |
| `drop_column` | `--table`, `--column` | |
| `rename_column` | `--table`, `--column` (old name), `--to` (new name) | |
| `rename_table` | `--table` (old name), `--to` (new name) | |
| `add_index` | `--table`, `--index` | `--index-name`, `--index-unique` optional |
| `drop_index` | `--table`, `--index-name` | |
| `add_foreign_key` | `--table`, `--fk-column` (repeatable), `--fk-references` | `--fk-on-delete`, `--fk-on-update`, `--fk-name` optional |
| `drop_foreign_key` | `--table`, `--fk-name` | |

`add_constraint`/`drop_constraint` and the raw `sql` escape hatch aren't
scaffoldable templates — write those migrations by hand (or with `--format
json`, that op directly).

## 3. Column spec mini-DSL (`--columns`)

Comma-separated fields, each `name:type[(args)][:modifier]*`:

```bash
--columns "id:bigint:pk:auto,email:string(255):unique,price:decimal(10,2):default=0"
```

- **Type**: one of the abstract types in the
  [column type reference](json-migrations.md#4-column-type-reference)
  (`string`, `text`, `integer`, `bigint`, `boolean`, `uuid`, `timestamp`,
  `date`, `decimal`, `json`, `float`).
- **Length / precision,scale**: parenthesized after the type —
  `string(255)` sets length, `decimal(10,2)` sets precision and scale. Omit
  to fall back to the same defaults JSON migrations use (`255` for string
  length, `10`/`0` for decimal precision/scale).
- **Modifiers** (any order, colon-separated):

  | Modifier | Effect |
  |---|---|
  | `pk` | primary key |
  | `unique` | unique constraint |
  | `auto` | auto-increment (see the [per-dialect rules](json-migrations.md#auto-increment-behavior-per-dialect) — same constraints apply here) |
  | `null` | explicitly nullable |
  | `notnull` | explicitly not-null |
  | `default=X` | literal default — `X` is auto-detected as bool (`true`/`false`), number, or string |
  | `defaultexpr=X` | verbatim SQL expression default, e.g. `defaultexpr=NOW()` |

```bash
--columns "created_at:timestamp:notnull:defaultexpr=NOW(),plan:string(50):default=free"
```

For `add_column`, `--columns` takes exactly one column field (no commas).

## 4. Index and foreign key flags

```bash
# add_index
mig create idx_users_email --template add_index \
  --table users --index "email" --index-unique
# --index-name is optional — auto-derived as idx_<col1>_<col2>... if omitted

# drop_index
mig create drop_idx --template drop_index --table users --index-name idx_users_email

# add_foreign_key (composite keys: repeat --fk-column)
mig create fk_posts_author --template add_foreign_key \
  --table posts --fk-column author_id --fk-references "users(id)" --fk-on-delete cascade
# --fk-name is optional — auto-derived as fk_<col1>_<col2>..._<ref_table> if omitted

# drop_foreign_key
mig create drop_fk --template drop_foreign_key --table posts --fk-name fk_posts_author
```

## 5. Reversibility

Flags always describe the **up** direction; `down` is derived automatically,
following one rule for every template:

- **Additive and rename ops** (`create_table`, `add_column`, `add_index`,
  `add_foreign_key`, `rename_table`, `rename_column`) are mechanically
  invertible from the up flags alone — e.g. `create_table --table users ...`
  produces `drop_table` on the down side with no extra input needed.
- **Destructive ops** (`drop_table`, `drop_column`, `drop_index`,
  `drop_foreign_key`) can't recover the shape they destroy from a "drop"
  invocation's flags alone, so their `down` is a TODO stub you fill in by
  hand:

  ```sql
  -- TODO: cannot auto-reverse drop_table on "users"; write the down migration manually
  ```

  (In `.json` output this is a schema-valid `{"op": "sql", "query": "-- TODO: ..."}`
  entry, so the file still parses and applies — the down side is just a
  no-op comment until you replace it.)

## 6. Worked example

```bash
mig create add_users --template create_table \
  --table users --columns "id:bigint:pk:auto,email:string(255):unique"
```

Generates (Postgres):

```sql
-- +migrate Up
CREATE TABLE "users" (
  "id" BIGSERIAL,
  "email" VARCHAR(255) UNIQUE,
  PRIMARY KEY ("id")
);

-- +migrate Down
DROP TABLE "users";
```

Or with `--format json`:

```json
{
  "up": [
    {
      "op": "create_table",
      "table": "users",
      "columns": [
        { "name": "id", "type": "bigint", "primary_key": true, "auto_increment": true },
        { "name": "email", "type": "string", "length": 255, "unique": true }
      ]
    }
  ],
  "down": [
    { "op": "drop_table", "table": "users" }
  ]
}
```

## 7. Errors

Since `.sql` templates render through the real `sqlgen.Dialect`, an op your
configured driver can't express surfaces the dialect's actual error at
`create` time instead of generating broken SQL — e.g.:

```bash
mig create add_fk --template add_foreign_key \
  --table posts --fk-column user_id --fk-references "users(id)"
# Error: up: sqlite does not support adding foreign keys after table
# creation; define them in create_table or use the "sql" escape hatch
```

See the [Known limitations](json-migrations.md#6-known-limitations) section
of the JSON migrations doc — the same per-dialect gaps apply here, since
templates call the same `Dialect` methods.

Other common errors are missing-required-flag messages, one per template
(e.g. `--template create_table requires --columns`, `--template
rename_table requires --to <new name>`) — self-explanatory and specific to
the flag that's missing.

`--template` with `--format sql` requires a `mig.yml` in the current
directory to resolve the database dialect (`mig setup` first, or pass
`--format json` if you don't need SQL output yet).
