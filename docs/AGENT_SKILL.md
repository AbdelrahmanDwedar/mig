# mig — AI Agent Skill

This document is written for AI coding agents (Claude, Cursor, Copilot, etc.)
so they can use the `mig` CLI correctly in this project without trial and
error. If you are an agent working in a project that uses `mig`, read this
before running any `mig` command.

**Trigger conditions**: "create a migration", "add column/table X",
"apply/run migrations", "rollback migrations", "reset/fresh the database",
"migration status", or any schema change in a project that has a `mig.yml`.

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
| `create <name>` | `{"file": "<path>"}` |
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
  parser: sql          # only "sql" exists; anything else errors at runtime
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

Generates `<dir>/<YYYY_MM_DD_HHMMSS>_<name>.sql` (name arg is required,
exactly one positional arg) with this boilerplate to fill in by hand:

```sql
-- +migrate Up
-- SQL queries for UP migration here

-- +migrate Down
-- SQL queries for DOWN migration here
```

**Parser is naive**: it matches any line *containing the substring*
`+migrate Up` / `+migrate Down`, not anchored to `--` comment syntax. Don't
put that literal text anywhere else in the file (e.g. in a string literal or
a comment about the syntax itself) or parsing breaks. Both sections should
be filled in — an empty Down section is not an error, it just makes rollback
a silent no-op (the table/column stays, tracking row is deleted).

## Apply pending migrations

```bash
mig migrate --json
```

Applies all pending `.sql` files in the migrations dir, in filename-sorted
(i.e. timestamp) order. Each file is applied in **its own transaction** —
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

Returns every `.sql` file in the migrations dir with `Applied` or `Pending`
— purely a filename-vs-tracking-table diff, doesn't touch schema state
directly.

## Workflow: making a schema change end to end

1. `mig create <descriptive_name> --json` → get the file path back.
2. Write the Up SQL and its exact inverse as the Down SQL.
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
- Only `parser: sql` exists today — don't set anything else in `mig.yml`.
