package scanner

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

const mysqlDefaultVarcharLen = 255

// mysqlScanner reads schema from information_schema, scoped to the
// connection's current database (DATABASE()). MySQL has no user-facing
// sequences (Scan always returns an empty Sequences map).
type mysqlScanner struct {
	db *sql.DB
}

func (s *mysqlScanner) Scan(ctx context.Context) (*Schema, error) {
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

func (s *mysqlScanner) tableNames(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT TABLE_NAME FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE' AND TABLE_NAME <> '_migrations'
		ORDER BY TABLE_NAME`)
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

func (s *mysqlScanner) scanTable(ctx context.Context, name string) (*Table, error) {
	uniqueCols, indexes, err := s.scanIndexes(ctx, name)
	if err != nil {
		return nil, err
	}
	cols, err := s.scanColumns(ctx, name, uniqueCols)
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

// mysqlNativeType reconstructs the native type string sqlgen.NativeColumnType
// would render for this column from information_schema's representation, so
// it can be resolved back to an abstract type via sqlgen.AbstractType. ok is
// false for types with no portable equivalent (a tinyint of any width other
// than 1, a char of any length other than 36, ENUM/SET, and other
// MySQL-specific types) — callers fall back to the raw column type.
func mysqlNativeType(dataType, columnType string, charLen, numPrec, numScale sql.NullInt64) (string, bool) {
	switch dataType {
	case "varchar":
		n := mysqlDefaultVarcharLen
		if charLen.Valid {
			n = int(charLen.Int64)
		}
		return fmt.Sprintf("VARCHAR(%d)", n), true
	case "text":
		return "TEXT", true
	case "int":
		return "INT", true
	case "bigint":
		return "BIGINT", true
	case "tinyint":
		if strings.HasPrefix(columnType, "tinyint(1)") {
			return "TINYINT(1)", true
		}
		return "", false
	case "char":
		if columnType == "char(36)" {
			return "CHAR(36)", true
		}
		return "", false
	case "datetime":
		return "DATETIME", true
	case "date":
		return "DATE", true
	case "decimal":
		p, sc := 10, 0
		if numPrec.Valid {
			p = int(numPrec.Int64)
		}
		if numScale.Valid {
			sc = int(numScale.Int64)
		}
		return fmt.Sprintf("DECIMAL(%d,%d)", p, sc), true
	case "json":
		return "JSON", true
	case "double":
		return "DOUBLE", true
	default:
		return "", false
	}
}

func (s *mysqlScanner) scanColumns(ctx context.Context, table string, uniqueCols map[string]bool) ([]Column, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT COLUMN_NAME, DATA_TYPE, COLUMN_TYPE, CHARACTER_MAXIMUM_LENGTH,
		       NUMERIC_PRECISION, NUMERIC_SCALE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA, COLUMN_KEY
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []Column
	for rows.Next() {
		var name, dataType, columnType, isNullable, extra, columnKey string
		var charLen, numPrec, numScale sql.NullInt64
		var colDefault sql.NullString
		if err := rows.Scan(&name, &dataType, &columnType, &charLen, &numPrec, &numScale, &isNullable, &colDefault, &extra, &columnKey); err != nil {
			return nil, err
		}

		nullable := isNullable == "YES"
		col := Column{
			Column: sqlgen.Column{
				Name:          name,
				PrimaryKey:    columnKey == "PRI",
				Nullable:      &nullable,
				AutoIncrement: strings.Contains(extra, "auto_increment"),
				Unique:        uniqueCols[name],
			},
		}

		if native, mapped := mysqlNativeType(dataType, columnType, charLen, numPrec, numScale); mapped {
			if abstract, length, precision, scale, ok := sqlgen.AbstractType("mysql", native); ok {
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
			col.NativeType = columnType
		}

		if colDefault.Valid {
			col.Default = &sqlgen.Default{Expr: colDefault.String}
		}

		cols = append(cols, col)
	}
	return cols, rows.Err()
}

// scanIndexes classifies each of the table's indexes (via
// information_schema.STATISTICS) into either a single-column-uniqueness
// fact folded onto the owning Column (uniqueCols) or a displayed Index
// entry, mirroring the SQLite scanner's treatment of implicit unique
// indexes — a single-column unique index collapses to Column.Unique; a
// multi-column one can't be expressed as a per-column flag, so it's shown
// as an Index. The PRIMARY key's own index is always skipped (redundant
// with Column.PrimaryKey).
func (s *mysqlScanner) scanIndexes(ctx context.Context, table string) (map[string]bool, []Index, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT INDEX_NAME, NON_UNIQUE, COLUMN_NAME, INDEX_TYPE
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX`, table)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	type rawIdx struct {
		name, method string
		unique       bool
		columns      []string
	}
	order := []string{}
	byName := map[string]*rawIdx{}
	for rows.Next() {
		var name, column, indexType string
		var nonUnique int
		if err := rows.Scan(&name, &nonUnique, &column, &indexType); err != nil {
			return nil, nil, err
		}
		if name == "PRIMARY" {
			continue
		}
		ri, ok := byName[name]
		if !ok {
			ri = &rawIdx{name: name, method: strings.ToLower(indexType), unique: nonUnique == 0}
			byName[name] = ri
			order = append(order, name)
		}
		ri.columns = append(ri.columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	uniqueCols := map[string]bool{}
	indexes := make([]Index, 0, len(order))
	for _, name := range order {
		ri := byName[name]
		if ri.unique && len(ri.columns) == 1 && ri.method == "btree" {
			uniqueCols[ri.columns[0]] = true
			continue
		}
		indexes = append(indexes, Index{
			Index:  sqlgen.Index{Name: ri.name, Columns: ri.columns, Unique: ri.unique},
			Method: ri.method,
		})
	}
	return uniqueCols, indexes, nil
}

func (s *mysqlScanner) scanForeignKeys(ctx context.Context, table string) ([]sqlgen.ForeignKey, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT kcu.CONSTRAINT_NAME, kcu.COLUMN_NAME, kcu.REFERENCED_TABLE_NAME, kcu.REFERENCED_COLUMN_NAME,
		       rc.UPDATE_RULE, rc.DELETE_RULE
		FROM information_schema.KEY_COLUMN_USAGE kcu
		JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
			ON rc.CONSTRAINT_SCHEMA = kcu.CONSTRAINT_SCHEMA AND rc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME
		WHERE kcu.TABLE_SCHEMA = DATABASE() AND kcu.TABLE_NAME = ? AND kcu.REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY kcu.CONSTRAINT_NAME, kcu.ORDINAL_POSITION`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rawFK struct {
		name, refTable, updateRule, deleteRule string
		columns, refColumns                    []string
	}
	order := []string{}
	byName := map[string]*rawFK{}
	for rows.Next() {
		var name, column, refTable, refColumn, updateRule, deleteRule string
		if err := rows.Scan(&name, &column, &refTable, &refColumn, &updateRule, &deleteRule); err != nil {
			return nil, err
		}
		rf, ok := byName[name]
		if !ok {
			rf = &rawFK{name: name, refTable: refTable, updateRule: updateRule, deleteRule: deleteRule}
			byName[name] = rf
			order = append(order, name)
		}
		rf.columns = append(rf.columns, column)
		rf.refColumns = append(rf.refColumns, refColumn)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	fks := make([]sqlgen.ForeignKey, 0, len(order))
	for _, name := range order {
		rf := byName[name]
		fks = append(fks, sqlgen.ForeignKey{
			Name:       rf.name,
			Columns:    rf.columns,
			RefTable:   rf.refTable,
			RefColumns: rf.refColumns,
			OnDelete:   mysqlReferentialAction(rf.deleteRule),
			OnUpdate:   mysqlReferentialAction(rf.updateRule),
		})
	}
	return fks, nil
}

// mysqlReferentialAction normalizes information_schema.REFERENTIAL_CONSTRAINTS'
// UPDATE_RULE/DELETE_RULE, treating "NO ACTION" (the implicit default) as
// not worth rendering explicitly.
func mysqlReferentialAction(rule string) string {
	if rule == "" || strings.EqualFold(rule, "NO ACTION") {
		return ""
	}
	return rule
}

func (s *mysqlScanner) scanChecks(ctx context.Context, table string) ([]sqlgen.Check, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cc.CONSTRAINT_NAME, cc.CHECK_CLAUSE
		FROM information_schema.CHECK_CONSTRAINTS cc
		JOIN information_schema.TABLE_CONSTRAINTS tc
			ON tc.CONSTRAINT_SCHEMA = cc.CONSTRAINT_SCHEMA AND tc.CONSTRAINT_NAME = cc.CONSTRAINT_NAME
		WHERE tc.CONSTRAINT_SCHEMA = DATABASE() AND tc.TABLE_NAME = ? AND tc.CONSTRAINT_TYPE = 'CHECK'
		ORDER BY cc.CONSTRAINT_NAME`, table)
	if err != nil {
		// MySQL versions/forks without CHECK_CONSTRAINTS (pre-8.0.16) —
		// treat as "no check constraints" rather than a scan failure.
		return nil, nil
	}
	defer rows.Close()

	var checks []sqlgen.Check
	for rows.Next() {
		var name, clause string
		if err := rows.Scan(&name, &clause); err != nil {
			return nil, err
		}
		checks = append(checks, sqlgen.Check{Name: name, Expr: clause})
	}
	return checks, rows.Err()
}

func (s *mysqlScanner) scanViews(ctx context.Context, schema *Schema) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT TABLE_NAME, VIEW_DEFINITION FROM information_schema.VIEWS
		WHERE TABLE_SCHEMA = DATABASE() ORDER BY TABLE_NAME`)
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

func (s *mysqlScanner) scanTriggers(ctx context.Context, schema *Schema) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT TRIGGER_NAME, EVENT_OBJECT_TABLE, ACTION_TIMING, EVENT_MANIPULATION, ACTION_STATEMENT
		FROM information_schema.TRIGGERS
		WHERE TRIGGER_SCHEMA = DATABASE()
		ORDER BY TRIGGER_NAME`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name, table, timing, event, action string
		if err := rows.Scan(&name, &table, &timing, &event, &action); err != nil {
			return err
		}
		schema.Triggers[name] = &Trigger{Name: name, Table: table, Timing: timing, Event: event, Definition: action}
	}
	return rows.Err()
}
