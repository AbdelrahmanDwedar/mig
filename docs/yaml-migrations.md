# YAML Migrations

`mig` supports three migration file formats: raw `.sql`, structured `.json`
(see [JSON Migrations](json-migrations.md)), and structured `.yaml`/`.yml` —
the same op vocabulary as JSON, written as YAML. All three coexist in the
same `migrations/` directory, applied in chronological (filename) order.

## 1. Overview

YAML migrations use the **exact same op vocabulary, `internal/sqlgen`
structs, and dialect rendering** as JSON migrations — a YAML file is
converted to the equivalent JSON internally before being compiled to SQL, so
every op, column type, error message, and dialect limitation documented in
[JSON Migrations](json-migrations.md) applies unchanged to YAML. This doc
only covers what's YAML-specific: syntax, gotchas, and worked examples.

**Format detection** is by extension only: `.yaml` and `.yml` are both
recognized (no content sniffing). `mig create --format yaml` scaffolds
`.yaml` as the canonical extension; hand-authored or renamed `.yml` files
work identically.

**Format selection for new files** follows the same precedence as JSON:
`--format` flag > `migrations.parser` in `mig.yml` > default `sql`.

```bash
# One-off override — scaffolds a YAML migration regardless of mig.yml
mig create add_users --format yaml
```

```yaml
# mig.yml — project-wide default so every `mig create` scaffolds YAML
migrations:
  parser: yaml
  dir: migrations
```

The [JSON Schema](json-migrations.schema.json) written for the JSON envelope
applies equally to the YAML envelope, since both compile through the same
structure. Most YAML-aware editors (e.g. VS Code's YAML extension) can
validate against a JSON Schema via a leading comment:

```yaml
# yaml-language-server: $schema=./docs/json-migrations.schema.json
up: []
down: []
```

## 2. File shape

Every YAML migration is a single top-level mapping with `up`/`down` keys, in
the same shape as the JSON envelope, just written as YAML:

```yaml
up:   # ops, run top to bottom
down: # ops, run top to bottom
```

Both `up` and `down` are required — an empty `down: []` is valid and, same
as JSON/SQL, makes rollback a silent no-op.

## 3. Op reference

Identical to the JSON op reference — see
[JSON Migrations §3 Full op reference](json-migrations.md#3-full-op-reference)
and [§4 Column type reference](json-migrations.md#4-column-type-reference).
Every field name, type, and requirement is the same; only the surface syntax
differs (YAML mapping/sequence instead of JSON object/array).

## 4. Worked examples

**Minimal table:**

```yaml
up:
  - op: create_table
    table: tags
    columns:
      - name: id
        type: integer
        primary_key: true
        auto_increment: true
      - name: name
        type: string
        length: 100
        nullable: false
down:
  - op: drop_table
    table: tags
    if_exists: true
```

**Unique index + named foreign key with `on_delete`/`on_update`:**

```yaml
up:
  - op: create_table
    table: posts
    columns:
      - name: id
        type: integer
        primary_key: true
        auto_increment: true
      - name: slug
        type: string
        length: 255
      - name: author_id
        type: integer
        nullable: false
    indexes:
      - name: idx_posts_slug
        columns: [slug]
        unique: true
    foreign_keys:
      - name: fk_posts_author
        columns: [author_id]
        ref_table: users
        ref_columns: [id]
        on_delete: cascade
        on_update: cascade
down:
  - op: drop_table
    table: posts
    if_exists: true
```

**Adding/dropping/renaming a column on an existing table:**

```yaml
up:
  - op: add_column
    table: users
    column:
      name: nickname
      type: string
      length: 50
  - op: rename_column
    table: users
    from: nickname
    to: display_name
down:
  - op: drop_column
    table: users
    column: display_name
```

**Raw SQL escape hatch (identical `sql` op as JSON):**

```yaml
up:
  - op: sql
    query: CREATE VIEW active_users AS SELECT * FROM users WHERE active = true
down:
  - op: sql
    query: DROP VIEW active_users
```

**Reusing a column definition with a YAML anchor** — a convenience JSON
doesn't have: define a shared shape once, reference it from multiple ops.

```yaml
up:
  - op: create_table
    table: widgets
    columns:
      - &id_column
        name: id
        type: integer
        primary_key: true
        auto_increment: true
      - name: name
        type: string
        length: 100
  - op: create_table
    table: gadgets
    columns:
      - *id_column
      - name: label
        type: string
        length: 100
down:
  - op: drop_table
    table: gadgets
    if_exists: true
  - op: drop_table
    table: widgets
    if_exists: true
```

## 5. YAML-specific gotchas

These don't exist for JSON and are worth knowing before hand-authoring:

- **Quote ambiguous scalars.** YAML's implicit typing turns unquoted
  `yes`/`no`/`on`/`off`/`true`/`false` into booleans and bare numeric-looking
  tokens into numbers. If a table, column, or index name could be mistaken
  for one of these (rare, but e.g. a column literally named `on`), quote it:
  `name: "on"`. A real type mismatch (e.g. a boolean landing in a field that
  expects a string) fails loudly with a normal Go unmarshal type error — it
  does not silently corrupt the migration.
- **Tabs are illegal indentation.** YAML requires spaces; a file with a tab
  character in its indentation fails to parse with a syntax error before it
  ever reaches the op layer.
- **Multi-document files are rejected.** A YAML file containing a second
  `---`-separated document is treated as invalid — `mig` expects exactly one
  `up`/`down` envelope per file.
- **Anchors and aliases (`&name`/`*name`) are supported** and can reduce
  repetition across ops in the same file (see the worked example above) —
  they're resolved before the migration is compiled, so the resulting SQL is
  identical either way.
- **Empty files parse as an empty migration**, not an error — same as an
  empty `.sql` file or `{"up": [], "down": []}` in JSON, both `up` and `down`
  are treated as empty op lists.

## 6. Known limitations

Identical to JSON's — see
[JSON Migrations §6 Known limitations](json-migrations.md#6-known-limitations)
(these are dialect-level restrictions, e.g. SQLite's `alter_column`/foreign
key/constraint support, not format-level ones).

## 7. Error reference

Identical error vocabulary to JSON (see
[JSON Migrations §7 Error reference](json-migrations.md#7-error-reference)),
with one addition: a YAML syntax error (bad indentation, tabs, multi-document
files) surfaces as `invalid YAML migration: <parser detail>` before any op
is even inspected.
