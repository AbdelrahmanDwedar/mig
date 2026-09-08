package scanner

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
	"github.com/lib/pq"
)

const pgDefaultVarcharLen = 255

// postgresScanner reads schema from pg_catalog/information_schema, scoped
// to the "public" schema (multi-schema support is a known limitation — see
// docs/scanners.md).
type postgresScanner struct {
	db *sql.DB
}

func (s *postgresScanner) Scan(ctx context.Context) (*Schema, error) {
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
	if err := s.scanSequences(ctx, schema); err != nil {
		return nil, err
	}

	return schema, nil
}

func (s *postgresScanner) tableNames(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE' AND table_name <> '_migrations'
		ORDER BY table_name`)
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

func (s *postgresScanner) scanTable(ctx context.Context, name string) (*Table, error) {
	pks, err := s.primaryKeyColumns(ctx, name)
	if err != nil {
		return nil, err
	}
	uniqueCols, indexes, err := s.scanIndexes(ctx, name)
	if err != nil {
		return nil, err
	}
	cols, err := s.scanColumns(ctx, name, pks, uniqueCols)
	if err != nil {
		return nil, err
	}
	fks, err := s.scanForeignKeys(ctx, name)
	if err != nil {
		return nil, err
	}
	checks, err := s.scanChecks(ctx, name)
	if err != nil {
		return nil, err
	}
	return &Table{Name: name, Columns: cols, Indexes: indexes, ForeignKeys: fks, Checks: checks}, nil
}

func (s *postgresScanner) primaryKeyColumns(ctx context.Context, table string) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON kcu.constraint_name = tc.constraint_name AND kcu.table_schema = tc.table_schema
		WHERE tc.table_schema = 'public' AND tc.table_name = $1 AND tc.constraint_type = 'PRIMARY KEY'`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pks := map[string]bool{}
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		pks[col] = true
	}
	return pks, rows.Err()
}

// pgNativeType reconstructs the native type string sqlgen.NativeColumnType
// would render for this column from Postgres's information_schema
// representation, so it can be resolved back to an abstract type via
// sqlgen.AbstractType. ok is false for types with no portable equivalent
// (arrays, enums, domains, and other Postgres-specific types) — callers
// fall back to the raw udt_name/data_type.
func pgNativeType(dataType string, charLen, numPrec, numScale sql.NullInt64) (string, bool) {
	switch dataType {
	case "character varying":
		n := pgDefaultVarcharLen
		if charLen.Valid {
			n = int(charLen.Int64)
		}
		return fmt.Sprintf("VARCHAR(%d)", n), true
	case "text":
		return "TEXT", true
	case "integer":
		return "INTEGER", true
	case "bigint":
		return "BIGINT", true
	case "boolean":
		return "BOOLEAN", true
	case "uuid":
		return "UUID", true
	case "timestamp without time zone", "timestamp with time zone":
		return "TIMESTAMP", true
	case "date":
		return "DATE", true
	case "numeric":
		p, sc := 10, 0
		if numPrec.Valid {
			p = int(numPrec.Int64)
		}
		if numScale.Valid {
			sc = int(numScale.Int64)
		}
		return fmt.Sprintf("NUMERIC(%d,%d)", p, sc), true
	case "jsonb":
		return "JSONB", true
	case "double precision":
		return "DOUBLE PRECISION", true
	default:
		return "", false
	}
}

func (s *postgresScanner) scanColumns(ctx context.Context, table string, pks, uniqueCols map[string]bool) ([]Column, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT column_name, data_type, udt_name, character_maximum_length,
		       numeric_precision, numeric_scale, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []Column
	for rows.Next() {
		var name, dataType, udtName, isNullable string
		var charLen, numPrec, numScale sql.NullInt64
		var colDefault sql.NullString
		if err := rows.Scan(&name, &dataType, &udtName, &charLen, &numPrec, &numScale, &isNullable, &colDefault); err != nil {
			return nil, err
		}

		nullable := isNullable == "YES"
		col := Column{
			Column: sqlgen.Column{
				Name:       name,
				PrimaryKey: pks[name],
				Nullable:   &nullable,
				Unique:     uniqueCols[name],
			},
		}

		if native, mapped := pgNativeType(dataType, charLen, numPrec, numScale); mapped {
			if abstract, length, precision, scale, ok := sqlgen.AbstractType("postgresql", native); ok {
				col.Type = abstract
				col.Length = length
				col.Precision = precision
				col.Scale = scale
			} else {
				col.Type = RawType
				col.NativeType = native
			}
		} else {
			col.Type = RawType
			col.NativeType = udtName
		}

		if colDefault.Valid {
			// SERIAL/IDENTITY columns surface as a nextval(...) default —
			// that's sequence machinery, not a user-authored default, so it
			// becomes AutoIncrement instead of Default (matching how
			// CreateTableOp expects auto_increment to be expressed).
			if strings.Contains(colDefault.String, "nextval(") {
				col.AutoIncrement = true
			} else {
				col.Default = &sqlgen.Default{Expr: colDefault.String}
			}
		}

		cols = append(cols, col)
	}
	return cols, rows.Err()
}

// scanIndexes classifies each of the table's non-PK indexes into either a
// single-column-uniqueness fact folded onto the owning Column (uniqueCols)
// or a displayed Index entry. A single-column unique index only folds into
// Column.Unique when it's a plain btree index with no partial predicate —
// otherwise folding it would silently drop the Method/Partial detail the
// user actually cares about seeing (see the "different databases have
// different index features" rationale in docs/scanners.md), so it's shown
// as an Index instead.
func (s *postgresScanner) scanIndexes(ctx context.Context, table string) (map[string]bool, []Index, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			ic.relname AS index_name,
			am.amname AS method,
			ix.indisunique AS is_unique,
			ix.indisprimary AS is_primary,
			pg_get_expr(ix.indpred, ix.indrelid) AS partial_pred,
			array(
				SELECT a.attname
				FROM unnest(ix.indkey) WITH ORDINALITY AS k(attnum, ord)
				JOIN pg_attribute a ON a.attrelid = ix.indrelid AND a.attnum = k.attnum
				ORDER BY k.ord
			) AS columns
		FROM pg_index ix
		JOIN pg_class ic ON ic.oid = ix.indexrelid
		JOIN pg_class tc ON tc.oid = ix.indrelid
		JOIN pg_am am ON am.oid = ic.relam
		WHERE tc.relname = $1 AND tc.relnamespace = 'public'::regnamespace
		ORDER BY ic.relname`, table)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	uniqueCols := map[string]bool{}
	var indexes []Index
	for rows.Next() {
		var name, method string
		var isUnique, isPrimary bool
		var partial sql.NullString
		var cols pq.StringArray
		if err := rows.Scan(&name, &method, &isUnique, &isPrimary, &partial, &cols); err != nil {
			return nil, nil, err
		}
		if isPrimary {
			// Already represented via Column.PrimaryKey.
			continue
		}
		if isUnique && len(cols) == 1 && method == "btree" && !partial.Valid {
			uniqueCols[cols[0]] = true
			continue
		}
		idx := Index{Index: sqlgen.Index{Name: name, Columns: cols, Unique: isUnique}, Method: method}
		if partial.Valid {
			idx.Partial = partial.String
		}
		indexes = append(indexes, idx)
	}
	return uniqueCols, indexes, rows.Err()
}

func (s *postgresScanner) scanForeignKeys(ctx context.Context, table string) ([]sqlgen.ForeignKey, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			con.conname,
			con.confupdtype,
			con.confdeltype,
			array(
				SELECT a.attname
				FROM unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord)
				JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum
				ORDER BY k.ord
			) AS columns,
			ref.relname AS ref_table,
			array(
				SELECT a.attname
				FROM unnest(con.confkey) WITH ORDINALITY AS k(attnum, ord)
				JOIN pg_attribute a ON a.attrelid = con.confrelid AND a.attnum = k.attnum
				ORDER BY k.ord
			) AS ref_columns
		FROM pg_constraint con
		JOIN pg_class c ON c.oid = con.conrelid
		JOIN pg_class ref ON ref.oid = con.confrelid
		WHERE con.contype = 'f' AND c.relname = $1 AND c.relnamespace = 'public'::regnamespace
		ORDER BY con.conname`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []sqlgen.ForeignKey
	for rows.Next() {
		var name string
		var onUpdate, onDelete rune
		var cols, refCols pq.StringArray
		var refTable string
		if err := rows.Scan(&name, &onUpdate, &onDelete, &cols, &refTable, &refCols); err != nil {
			return nil, err
		}
		fks = append(fks, sqlgen.ForeignKey{
			Name:       name,
			Columns:    cols,
			RefTable:   refTable,
			RefColumns: refCols,
			OnDelete:   pgConstraintAction(onDelete),
			OnUpdate:   pgConstraintAction(onUpdate),
		})
	}
	return fks, rows.Err()
}

// pgConstraintAction maps a pg_constraint confupdtype/confdeltype char to
// the referential action keyword; returns "" for 'a' (NO ACTION), the
// implicit default not worth rendering explicitly.
func pgConstraintAction(c rune) string {
	switch c {
	case 'r':
		return "RESTRICT"
	case 'c':
		return "CASCADE"
	case 'n':
		return "SET NULL"
	case 'd':
		return "SET DEFAULT"
	default:
		return ""
	}
}

func (s *postgresScanner) scanChecks(ctx context.Context, table string) ([]sqlgen.Check, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT con.conname, pg_get_constraintdef(con.oid)
		FROM pg_constraint con
		JOIN pg_class c ON c.oid = con.conrelid
		WHERE con.contype = 'c' AND c.relname = $1 AND c.relnamespace = 'public'::regnamespace
		ORDER BY con.conname`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []sqlgen.Check
	for rows.Next() {
		var name, def string
		if err := rows.Scan(&name, &def); err != nil {
			return nil, err
		}
		checks = append(checks, sqlgen.Check{Name: name, Expr: pgCheckExpr(def)})
	}
	return checks, rows.Err()
}

// pgCheckExpr strips pg_get_constraintdef's "CHECK (...)" wrapper, leaving
// just the inner expression (Check.Expr's expected shape).
func pgCheckExpr(def string) string {
	const prefix, suffix = "CHECK (", ")"
	if strings.HasPrefix(def, prefix) && strings.HasSuffix(def, suffix) {
		return def[len(prefix) : len(def)-len(suffix)]
	}
	return def
}

func (s *postgresScanner) scanViews(ctx context.Context, schema *Schema) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT table_name, view_definition FROM information_schema.views
		WHERE table_schema = 'public' ORDER BY table_name`)
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

func (s *postgresScanner) scanTriggers(ctx context.Context, schema *Schema) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT trigger_name, event_object_table, action_timing, event_manipulation, action_statement
		FROM information_schema.triggers
		WHERE trigger_schema = 'public'
		ORDER BY trigger_name`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name, table, timing, event, action string
		if err := rows.Scan(&name, &table, &timing, &event, &action); err != nil {
			return err
		}
		// information_schema.triggers has one row per event for a trigger
		// declared on multiple events (e.g. "BEFORE INSERT OR UPDATE") —
		// combine them under a single Trigger entry.
		if existing, ok := schema.Triggers[name]; ok {
			existing.Event = existing.Event + " OR " + event
			continue
		}
		schema.Triggers[name] = &Trigger{Name: name, Table: table, Timing: timing, Event: event, Definition: action}
	}
	return rows.Err()
}

func (s *postgresScanner) scanSequences(ctx context.Context, schema *Schema) error {
	// Exclude sequences owned by a column (SERIAL/IDENTITY backing
	// sequences, deptype 'a') — those are already represented via
	// Column.AutoIncrement, not standalone user sequences.
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.relname
		FROM pg_class c
		WHERE c.relkind = 'S' AND c.relnamespace = 'public'::regnamespace
		AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.objid = c.oid AND d.deptype = 'a')
		ORDER BY c.relname`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, name := range names {
		var start, increment, minValue, maxValue int64
		err := s.db.QueryRowContext(ctx, `
			SELECT start_value, increment_by, min_value, max_value
			FROM pg_sequences WHERE schemaname = 'public' AND sequencename = $1`, name,
		).Scan(&start, &increment, &minValue, &maxValue)
		if err != nil {
			return fmt.Errorf("scanning sequence %q: %w", name, err)
		}
		schema.Sequences[name] = &Sequence{Name: name, Start: start, Increment: increment, MinValue: minValue, MaxValue: maxValue}
	}
	return nil
}
