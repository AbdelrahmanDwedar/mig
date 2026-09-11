# mig — AI Agent Skill

This document is written for AI coding agents (Claude, Cursor, Copilot, etc.)
so they can use the `mig` CLI correctly in this project without trial and
error. If you are an agent working in a project that uses `mig`, read this
before running any `mig` command.

**Trigger conditions**: "create a migration", "add column/table X",
"drop/rename table/column X", "add an index/foreign key", "scaffold a
migration", "apply/run migrations", "rollback migrations", "reset/fresh the
database", "migration status", or any schema change in a project that has a
`mig.yml`. For "add/drop/rename table|column|index|foreign key" requests
specifically, prefer `mig create --template` (see [Create a
migration](#create-a-migration)) over hand-writing SQL.

`mig` is a standalone database-migration CLI. It is not an ORM and does not
generate SQL from a schema definition — you write the SQL by hand in
generated migration files, and `mig` tracks/applies/rolls them back. It is
driver-agnostic across PostgreSQL/MySQL/SQLite.

## Step 0 — verify mig is available

```bash
mig --help
```

If not on PATH and you're in the `mig` repo itself:

```bash
go build -o /usr/local/bin/mig ./cmd/mig
```

## Machine-readable output (`--json`)

Every command accepts a global `--json` flag. When set, the command prints
exactly one JSON line to stdout instead of prose, and prints nothing else:

```json
{"success": true, "data": <command-specific>, "error": ""}
```

On failure: `{"success": false, "data": null, "error": "<message>"}`, exit
code 1. **Prefer `--json` when driving `mig` programmatically** — plain-text
output has no stable format and may change wording across versions.

| Command | `data` shape on success |
|---|---|
| `setup` | `{"driver","dbname","dir","created"}` — `created` is `false` if `mig.yml` already existed (setup no-ops, still exit 0) |
| `create <name>` | `{"file": "<path>"}` — extension is `.sql` or `.json` depending on the resolved format, see [Create a migration](#create-a-migration) |
| `migrate` | `{"applied": [<filenames in order applied>]}` |
| `rollback` | `{"rolled_back": [<filenames in order rolled back>]}` |
| `reset` | `{"rolled_back": [<filenames>]}` |
| `fresh`/`refresh` | `{"rolled_back": [<filenames>], "applied": [<filenames>]}` |
| `status` | `[{"name": "<filename>", "status": "Applied"\|"Pending"}, ...]` |

There is no distinct exit-code taxonomy — every failure is exit 1 with a
message in `error`; there is no way to distinguish e.g. "config not found"
from "SQL syntax error" except by parsing the message text.

## One-time setup per project

```bash
mig setup --driver postgresql --dbname mydb --dir migrations --json
```

`--driver`, `--dbname`, and `--dir` must **all** be passed to get
non-interactive behavior — if any one is omitted, `mig setup` drops into
interactive `promptui` prompts (select driver / type dbname / type dir),
which will hang a non-interactive agent session. `setup` is safe to call
speculatively: if `mig.yml` already exists it just prints a message and
exits 0 (`created: false` in JSON) rather than erroring or overwriting.

This writes `mig.yml` in the current directory and creates the migrations
dir. All other `mig` commands hardcode `mig.yml` as a relative path — **you
must run every command from the directory containing `mig.yml`**; there is
no `--config` flag to point elsewhere.

### `mig.yml` format

```yaml
database:
  driver: postgresql   # postgresql | mysql | sqlite
  host: localhost
  port: 5432
  user: user
  password: password
  dbname: mydatabase   # for sqlite this is the .db file path, not a DB name
migrations:
  parser: sql          # sql | json — default format for `mig create`; see below
  dir: migrations
```

- Values support Docker Compose-style interpolation: `${VAR}`,
  `${VAR:-default}`, `${VAR:?error message if unset/empty}`.
- A `.env` file is auto-loaded. Discovery order: walk up from `mig.yml`'s
  directory looking for a `.git` folder and load `.env` from that root; if
  no `.git` is found anywhere up the tree, fall back to `.env` in the
  current working directory instead. Put `.env` at the git root of the
  project, not next to `mig.yml`, if they differ.
- `sqlite` driver: `host`/`port`/`user`/`password` are irrelevant, only
  `dbname` (the file path) matters.

## Create a migration

```bash
mig create <name> --json
```

Generates `<dir>/<YYYY_MM_DD_HHMMSS>_<name>.<ext>` (name arg is required,
exactly one positional arg). `<ext>` and the file's content depend entirely
on which of the three ways below you invoke `create` — **decide which one
before running the command**, since they produce different file shapes and
you can't cheaply convert between them after the fact:

| You want... | Run | Produces |
|---|---|---|
| A scaffolded, dialect-correct migration for a common op (create/drop table, add/drop column, add/drop index, add/drop FK, rename) | `mig create <name> --template <op> ...` (see below) | Real SQL or the equivalent JSON op — **prefer this whenever the op fits the [10 supported templates](templates.md#2-supported-templates)** |
| Full manual control over raw SQL | `mig create <name> --format sql` (or omit `--format`, it's the default) | Empty `-- +migrate Up`/`Down` boilerplate you fill in yourself |
| Full manual control but as structured, portable ops (views/triggers/anything `--template` doesn't cover, or multiple ops in one file) | `mig create <name> --format json` | Empty `{"up": [], "down": []}` — write ops by hand, see [json-migrations.md](json-migrations.md) |

Empty `.sql` boilerplate:

```sql
-- +migrate Up
-- SQL queries for UP migration here

-- +migrate Down
-- SQL queries for DOWN migration here
```

**SQL parser is naive**: it matches any line *containing the substring*
`+migrate Up` / `+migrate Down`, not anchored to `--` comment syntax. Don't
put that literal text anywhere else in the file (e.g. in a string literal or
a comment about the syntax itself) or parsing breaks. Both sections should
be filled in — an empty Down section is not an error, it just makes rollback
a silent no-op (the table/column stays, tracking row is deleted).

Format resolution precedence (for both `--format` and `--template`
invocations): `--format` flag > `migrations.parser` in `mig.yml` > `sql`
default. If you don't know the project's default, either pass `--format`
explicitly or run `mig create ... --json` and read the returned file's
extension back rather than assuming.

### Scaffolding common ops with `--template` (prefer this over hand-writing SQL)

For the 10 ops it covers, `--template` is strictly better than hand-writing
a migration: it renders real SQL through the actual dialect code path (no
risk of a typo'd `AUTO_INCREMENT`/`SERIAL`/`AUTOINCREMENT`), and derives the
down side for you. Reach for it first; fall back to plain `--format
sql`/`json` only for what it can't express (views, triggers, `alter_column`,
`add_constraint`/`drop_constraint`, or anything needing the raw `sql` op).

**Copy-paste one-liners** (swap in real table/column names — every example
below is a complete, runnable command):

```bash
# create_table — --columns is "name:type[(len)|(prec,scale)][:modifier]*", comma-separated
mig create add_users --template create_table --table users \
  --columns "id:bigint:pk:auto,email:string(255):unique,created_at:timestamp:notnull:defaultexpr=NOW()" --json

# drop_table
mig create drop_users --template drop_table --table users --if-exists --json

# add_column — --columns takes exactly ONE column spec here (not a list)
mig create add_users_plan --template add_column --table users \
  --columns "plan:string(50):default=free" --json

# drop_column
mig create drop_users_plan --template drop_column --table users --column plan --json

# rename_column
mig create rename_users_email --template rename_column --table users --column email --to email_address --json

# rename_table
mig create rename_users --template rename_table --table users --to accounts --json

# add_index (--index-name auto-derived from columns if omitted)
mig create add_users_email_idx --template add_index --table users --index "email" --index-unique --json

# drop_index
mig create drop_users_email_idx --template drop_index --table users --index-name idx_email --json

# add_foreign_key (--fk-column is repeatable for composite keys; --fk-name auto-derived if omitted)
mig create add_posts_author_fk --template add_foreign_key --table posts \
  --fk-column author_id --fk-references "users(id)" --fk-on-delete cascade --json

# drop_foreign_key
mig create drop_posts_author_fk --template drop_foreign_key --table posts --fk-name fk_author_id_users --json
```

**Reversibility**: `create_table`/`add_column`/`add_index`/`add_foreign_key`/
`rename_table`/`rename_column` derive a fully-working `down` automatically —
nothing to fix up. `drop_table`/`drop_column`/`drop_index`/
`drop_foreign_key` **cannot** derive a working `down` (the original shape
they destroy isn't in the drop invocation's flags), so their generated file
has a placeholder in the down section:

```sql
-- TODO: cannot auto-reverse drop_table on "users"; write the down migration manually
```

**You must read the generated file and replace that TODO line yourself**
before treating the migration as reversible — don't apply-then-forget for
these four ops. (In `.json` output it's `{"op": "sql", "query": "-- TODO: ..."}`,
which still parses/applies fine as a no-op — it just won't actually reverse
anything until you replace it.)

**Errors an agent should recognize and handle, not just surface to the user:**

| Error substring | Meaning | Fix |
|---|---|---|
| `unknown template "..."` | Typo'd `--template` value | Check spelling against the [supported templates table](templates.md#2-supported-templates) |
| `--template <op> requires --table` | `--table` omitted | Every template needs `--table` |
| `--template create_table requires --columns` / `add_column requires --columns` | Missing/empty `--columns` | Add at least one column spec |
| `--template add_column takes exactly one column, got N` | Comma-separated list passed to `add_column`'s `--columns` | `add_column` takes one column per invocation — run it N times for N columns |
| `--template rename_table requires --to` / `rename_column requires ... --to` | Missing `--to` | Add the new name |
| `--template drop_index requires --index-name` | `add_index`'s auto-derived name convention doesn't apply to drops — you must know the exact existing index name | Look it up (e.g. from the migration that created it) rather than guessing |
| `invalid column spec "...": unknown type "..."` | Type isn't one of the [abstract types](json-migrations.md#4-column-type-reference) | Use `string`/`text`/`integer`/`bigint`/`boolean`/`uuid`/`timestamp`/`date`/`decimal`/`json`/`float` |
| `sqlite does not support ...` | Targeting a SQLite `mig.yml` with `add_foreign_key`/`drop_foreign_key`/`alter_column`/constraints | Bake FKs into `create_table` instead, or use `--format json` with the raw `sql` op — see [Known limitations](json-migrations.md#6-known-limitations) |
| `requires a mig.yml to resolve the database dialect` | `--template` with `--format sql` (or the resolved default) run before `mig setup` | Run `mig setup` first, or add `--format json` to skip dialect resolution |

Full flag reference and the `--columns` mini-DSL grammar in detail:
**[docs/templates.md](templates.md)**.

this is not one big transaction for the whole batch. If file 3 of 5 pending
fails, files 1–2 are already committed and will show as `Applied`; files 4–5
are never attempted. Fix file 3 and re-run `mig migrate` — it will correctly
skip 1–2 and retry from 3.

## Rollback / Reset / Fresh

```bash
mig rollback --json                        # undo the last 1 (default)
mig rollback --steps 3 --json              # undo last N
mig rollback --migration <name-substr> --json  # undo one match by substring
mig reset --json                           # undo everything, dev only
mig fresh --json                           # reset then re-migrate, dev only
mig refresh --json                         # alias of fresh
```

- `--steps` and `--migration` are mutually exclusive (enforced before run —
  passing both is a hard error, not a warning).
- `--migration <substr>` does **not** target "the most recent match". It
  scans all applied migrations sorted ascending by filename and rolls back
  the **first** one whose filename contains the substring. If two applied
  migrations both match the substring, you get the older one, not the newer
  one — be as specific as possible in the substring to avoid ambiguity.
- Rollback/reset order is purely by sorted filename (timestamp prefix).
  There is a `batch` column in the tracking table, but every migration ever
  applied gets `batch = 1` — nothing in mig actually groups by batch despite
  what `ARCHITECTURE.md` implies. Don't rely on batch semantics.
- **Never run `reset`/`fresh` against production** — both roll back (and
  `fresh` then re-applies) every migration unconditionally.

## Status

```bash
mig status --json
```

Returns every `.sql`/`.json` file in the migrations dir with `Applied` or
`Pending` — purely a filename-vs-tracking-table diff, doesn't touch schema
state directly.

## Inspect the live database schema

```bash
mig inspect --json
```

Reads the actual database (not the migration files) and returns everything
in it — tables, columns, indexes, foreign keys, checks, views, triggers,
sequences — as structured JSON (`mig inspect` with no flag prints a
human-readable tree instead). Useful for confirming a migration actually
produced the schema you expect, or for checking what's already there before
writing a new migration. It does not compare against migration files — it
only reports what the database currently looks like. See
[`docs/scanners.md`](scanners.md) for the full output shape and its known
limitations (e.g. no CHECK constraint visibility on SQLite, single-schema
scope on Postgres).

## Workflow: making a schema change end to end

**If the change is one of the 10 supported templates** (the common case —
adding/dropping a table/column/index/FK, or a rename):

1. `mig create <descriptive_name> --template <op> --table <t> ... --json` →
   get the file path back.
2. If it's a destructive op (`drop_table`/`drop_column`/`drop_index`/
   `drop_foreign_key`), open the file and replace the `-- TODO: cannot
   auto-reverse ...` line with a real down migration. Additive/rename ops
   need no manual edit.
3. `mig migrate --json` → confirm the filename appears in `applied`.
4. `mig status --json` → confirm it now shows `Applied`.

**Otherwise** (views, triggers, `alter_column`, constraints, or anything
else outside the template set):

1. `mig create <descriptive_name> --json` (add `--format json` for
   structured ops, omit/`--format sql` for raw SQL) → get the file path back.
2. Write the Up and its exact inverse as the Down, in whichever format you chose.
3. `mig migrate --json` → confirm the filename appears in `applied`.
4. `mig status --json` → confirm it now shows `Applied`.

## Gotchas

- Run from the directory containing `mig.yml` — there's no way to point
  elsewhere.
- Migrations are tracked by **filename** in the `_migrations` table. Renaming
  or deleting an already-applied file breaks tracking (mig will think it's a
  new pending migration, or lose the ability to roll it back by name).
- No locking: don't run `mig migrate`/`rollback`/`reset` concurrently against
  the same database.
- Supported drivers are PostgreSQL, MySQL, and SQLite only (`internal/db/factory.go`);
  passing any other `driver` value in `mig.yml` fails at runtime with
  "unsupported driver: X".
- `migrations.parser` in `mig.yml` accepts `sql` or `json` only — anything
  else errors at runtime (this only sets the *default* for `mig create`;
  `.sql` and `.json` files always coexist and both apply regardless of it).
- `--template` renders through the real per-dialect code, so switching
  `mig.yml`'s `database.driver` changes what SQL a future `--template`
  invocation produces — always re-check `database.driver` before scaffolding
  if you're not sure which project/environment you're in.
- `add_column`'s `--columns` takes exactly **one** column spec per
  invocation, unlike `create_table`'s `--columns` which takes a
  comma-separated list — run `create` once per column if you need to add
  several.
- Auto-generated names (`--index-name` for `add_index`, `--fk-name` for
  `add_foreign_key`) are **not** remembered anywhere — if you didn't pass
  an explicit name and need to `drop_index`/`drop_foreign_key` it later,
  re-derive the same auto-generated name (`idx_<col1>_<col2>...` /
  `fk_<col1>_..._<ref_table>`) or read it back out of the generated file.
