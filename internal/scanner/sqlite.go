package scanner

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

// sqliteScanner reads schema via sqlite_master and the PRAGMA family
// (table_info, index_list, index_info, foreign_key_list). SQLite has no
// user-facing sequences (Scan always returns an empty Sequences map) and no
// PRAGMA for CHECK constraints, so Table.Checks is always empty for now —
// see docs/scanners.md's limitations section.
type sqliteScanner struct {
	db *sql.DB
}

func (s *sqliteScanner) Scan(ctx context.Context) (*Schema, error) {
	schema := &Schema{
		Tables:    map[string]*Table{},
		Views:     map[string]*View{},
		Triggers:  map[string]*Trigger{},
		Sequences: map[string]*Sequence{},
	}

	names, err := s.tableNames(ctx)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		t, err := s.scanTable(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("scanning table %q: %w", name, err)
		}
		schema.Tables[name] = t
	}

	if err := s.scanViews(ctx, schema); err != nil {
		return nil, err
	}
	if err := s.scanTriggers(ctx, schema); err != nil {
		return nil, err
	}

	return schema, nil
}

func (s *sqliteScanner) tableNames(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name != '_migrations'
		ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (s *sqliteScanner) scanTable(ctx context.Context, name string) (*Table, error) {
	createSQL, err := s.tableSQL(ctx, name)
	if err != nil {
		return nil, err
	}
	uniqueCols, indexes, err := s.scanIndexes(ctx, name)
	if err != nil {
		return nil, err
	}
	cols, err := s.scanColumns(ctx, name, createSQL, uniqueCols)
	if err != nil {
		return nil, err
	}
	fks, err := s.scanForeignKeys(ctx, name)
	if err != nil {
		return nil, err
	}
	return &Table{Name: name, Columns: cols, Indexes: indexes, ForeignKeys: fks}, nil
}

func (s *sqliteScanner) tableSQL(ctx context.Context, name string) (string, error) {
	var createSQL sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?`, name,
	).Scan(&createSQL)
	return createSQL.String, err
}

func (s *sqliteScanner) scanColumns(ctx context.Context, table, createSQL string, uniqueCols map[string]bool) ([]Column, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%q)`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rawCol struct {
		name, ctype string
		notnull, pk int
		dflt        sql.NullString
	}
	var raw []rawCol
	pkCount := 0
	for rows.Next() {
		var cid int
		var rc rawCol
		if err := rows.Scan(&cid, &rc.name, &rc.ctype, &rc.notnull, &rc.dflt, &rc.pk); err != nil {
			return nil, err
		}
		if rc.pk > 0 {
			pkCount++
		}
		raw = append(raw, rc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// SQLite only truly auto-increments rowid when the column is declared
	// "INTEGER PRIMARY KEY AUTOINCREMENT" — the literal keyword matters
	// (mirrors sqliteDialect.CreateTable's own auto-increment rendering).
	hasAutoincrementKeyword := strings.Contains(strings.ToUpper(createSQL), "AUTOINCREMENT")

	cols := make([]Column, 0, len(raw))
	for _, rc := range raw {
		nullable := rc.notnull == 0
		col := Column{
			Column: sqlgen.Column{
				Name:       rc.name,
				PrimaryKey: rc.pk > 0,
				Nullable:   &nullable,
				Unique:     uniqueCols[rc.name],
			},
		}

		if abstract, length, precision, scale, ok := sqlgen.AbstractType("sqlite", rc.ctype); ok {
			col.Type = abstract
			col.Length = length
			col.Precision = precision
			col.Scale = scale
		} else {
			col.Type = RawType
			col.NativeType = rc.ctype
		}

		if pkCount == 1 && rc.pk == 1 && hasAutoincrementKeyword && strings.EqualFold(rc.ctype, "INTEGER") {
			col.AutoIncrement = true
		}

		if rc.dflt.Valid {
			// dflt_value is already raw SQL text as stored in the schema
			// (a quoted literal or an expression) — see docs/scanners.md:
			// scanned defaults are always represented as Expr, never
			// reverse-parsed into a typed Literal.
			col.Default = &sqlgen.Default{Expr: rc.dflt.String}
		}

		cols = append(cols, col)
	}
	return cols, nil
}

// scanIndexes classifies each of the table's indexes (via PRAGMA index_list)
// into either a single-column-uniqueness fact folded onto the owning
// Column (uniqueCols), or a displayed Index entry. SQLite creates indexes
// implicitly for PRIMARY KEY ('pk' origin, always redundant with
// Column.PrimaryKey) and UNIQUE column/table constraints ('u' origin,
// carrying an auto-generated name like "sqlite_autoindex_users_1" that
// isn't something the user chose) — a single-column 'u' index folds into
// Column.Unique instead of being shown as a named index; a multi-column one
// can't be expressed as a per-column flag, so it's still shown despite the
// unhelpful name. Explicit 'c' (CREATE INDEX) entries are always shown.
func (s *sqliteScanner) scanIndexes(ctx context.Context, table string) (map[string]bool, []Index, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`PRAGMA index_list(%q)`, table))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	type rawIdx struct {
		name, origin string
		unique       bool
	}
	var raws []rawIdx
	for rows.Next() {
		var seq int
		var name, origin string
		var unique, partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, nil, err
		}
		raws = append(raws, rawIdx{name: name, origin: origin, unique: unique == 1})
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	uniqueCols := map[string]bool{}
	var indexes []Index
	for _, ri := range raws {
		if ri.origin == "pk" {
			continue
		}
		cols, err := s.indexColumns(ctx, ri.name)
		if err != nil {
			return nil, nil, err
		}
		if ri.origin == "u" && len(cols) == 1 {
			uniqueCols[cols[0]] = true
			continue
		}
		indexes = append(indexes, Index{
			Index: sqlgen.Index{Name: ri.name, Columns: cols, Unique: ri.unique},
		})
	}
	return uniqueCols, indexes, nil
}

func (s *sqliteScanner) indexColumns(ctx context.Context, indexName string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`PRAGMA index_info(%q)`, indexName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var seqno, cid int
		var name sql.NullString
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			return nil, err
		}
		if name.Valid {
			cols = append(cols, name.String)
		}
	}
	return cols, rows.Err()
}

func (s *sqliteScanner) scanForeignKeys(ctx context.Context, table string) ([]sqlgen.ForeignKey, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`PRAGMA foreign_key_list(%q)`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type row struct {
		id, seq                      int
		refTable, from, to           string
		onUpdate, onDelete, matchStr string
	}
	var raws []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.seq, &r.refTable, &r.from, &r.to, &r.onUpdate, &r.onDelete, &r.matchStr); err != nil {
			return nil, err
		}
		raws = append(raws, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	byID := map[int]*sqlgen.ForeignKey{}
	var order []int
	for _, r := range raws {
		fk, ok := byID[r.id]
		if !ok {
			fk = &sqlgen.ForeignKey{RefTable: r.refTable}
			if r.onDelete != "" && !strings.EqualFold(r.onDelete, "NO ACTION") {
				fk.OnDelete = strings.ToUpper(r.onDelete)
			}
			if r.onUpdate != "" && !strings.EqualFold(r.onUpdate, "NO ACTION") {
				fk.OnUpdate = strings.ToUpper(r.onUpdate)
			}
			byID[r.id] = fk
			order = append(order, r.id)
		}
		fk.Columns = append(fk.Columns, r.from)
		fk.RefColumns = append(fk.RefColumns, r.to)
	}

	fks := make([]sqlgen.ForeignKey, 0, len(order))
	for _, id := range order {
		fks = append(fks, *byID[id])
	}
	return fks, nil
}

func (s *sqliteScanner) scanViews(ctx context.Context, schema *Schema) error {
	rows, err := s.db.QueryContext(ctx, `SELECT name, sql FROM sqlite_master WHERE type = 'view' ORDER BY name`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var def sql.NullString
		if err := rows.Scan(&name, &def); err != nil {
			return err
		}
		schema.Views[name] = &View{Name: name, Definition: def.String}
	}
	return rows.Err()
}

var sqliteTriggerRe = regexp.MustCompile(`(?i)CREATE\s+TRIGGER\s+(?:IF\s+NOT\s+EXISTS\s+)?\S+\s+(BEFORE|AFTER|INSTEAD\s+OF)\s+(INSERT|UPDATE|DELETE)`)

func (s *sqliteScanner) scanTriggers(ctx context.Context, schema *Schema) error {
	rows, err := s.db.QueryContext(ctx, `SELECT name, tbl_name, sql FROM sqlite_master WHERE type = 'trigger' ORDER BY name`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name, tbl string
		var def sql.NullString
		if err := rows.Scan(&name, &tbl, &def); err != nil {
			return err
		}
		trig := &Trigger{Name: name, Table: tbl, Definition: def.String}
		if m := sqliteTriggerRe.FindStringSubmatch(def.String); m != nil {
			trig.Timing = strings.ToUpper(m[1])
			trig.Event = strings.ToUpper(m[2])
		}
		schema.Triggers[name] = trig
	}
	return rows.Err()
}
