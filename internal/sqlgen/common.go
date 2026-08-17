package sqlgen

import (
	"fmt"
	"strconv"
	"strings"
)

// quoteIdent quotes a table/column/index/constraint identifier for the
// given dialect. MySQL uses backticks; Postgres and SQLite use double quotes.
func quoteIdent(dialectName, ident string) string {
	if dialectName == "mysql" {
		return "`" + ident + "`"
	}
	return `"` + ident + `"`
}

func quoteIdentList(dialectName string, idents []string) string {
	quoted := make([]string, len(idents))
	for i, id := range idents {
		quoted[i] = quoteIdent(dialectName, id)
	}
	return strings.Join(quoted, ", ")
}

func renderLiteral(v interface{}) (string, error) {
	switch val := v.(type) {
	case string:
		return "'" + strings.ReplaceAll(val, "'", "''") + "'", nil
	case bool:
		if val {
			return "TRUE", nil
		}
		return "FALSE", nil
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64), nil
	case nil:
		return "NULL", nil
	default:
		return "", fmt.Errorf("unsupported default literal type: %T", v)
	}
}

func renderDefault(d *Default) (string, error) {
	if d == nil {
		return "", nil
	}
	if d.Expr != "" {
		return d.Expr, nil
	}
	return renderLiteral(d.Literal)
}

// columnClause renders "name TYPE [NOT NULL] [DEFAULT ...] [UNIQUE]" for a
// column that is not the dialect's special-cased auto-increment primary key.
func columnClause(dialectName string, col Column) (string, error) {
	native, err := NativeColumnType(dialectName, col.Type, col.Length, col.Precision, col.Scale)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(quoteIdent(dialectName, col.Name))
	b.WriteString(" ")
	b.WriteString(native)

	nullable := true
	if col.Nullable != nil {
		nullable = *col.Nullable
	}
	if !nullable {
		b.WriteString(" NOT NULL")
	}

	if col.Default != nil {
		rendered, err := renderDefault(col.Default)
		if err != nil {
			return "", err
		}
		b.WriteString(" DEFAULT ")
		b.WriteString(rendered)
	}

	if col.Unique {
		b.WriteString(" UNIQUE")
	}

	return b.String(), nil
}

func primaryKeyColumns(cols []Column) []string {
	var pks []string
	for _, c := range cols {
		if c.PrimaryKey {
			pks = append(pks, c.Name)
		}
	}
	return pks
}

func foreignKeyClause(dialectName string, fk ForeignKey) string {
	var b strings.Builder
	if fk.Name != "" {
		b.WriteString("CONSTRAINT ")
		b.WriteString(quoteIdent(dialectName, fk.Name))
		b.WriteString(" ")
	}
	b.WriteString("FOREIGN KEY (")
	b.WriteString(quoteIdentList(dialectName, fk.Columns))
	b.WriteString(") REFERENCES ")
	b.WriteString(quoteIdent(dialectName, fk.RefTable))
	b.WriteString(" (")
	b.WriteString(quoteIdentList(dialectName, fk.RefColumns))
	b.WriteString(")")
	if fk.OnDelete != "" {
		b.WriteString(" ON DELETE ")
		b.WriteString(strings.ToUpper(fk.OnDelete))
	}
	if fk.OnUpdate != "" {
		b.WriteString(" ON UPDATE ")
		b.WriteString(strings.ToUpper(fk.OnUpdate))
	}
	return b.String()
}

func checkClause(dialectName string, c Check) string {
	var b strings.Builder
	if c.Name != "" {
		b.WriteString("CONSTRAINT ")
		b.WriteString(quoteIdent(dialectName, c.Name))
		b.WriteString(" ")
	}
	b.WriteString("CHECK (")
	b.WriteString(c.Expr)
	b.WriteString(")")
	return b.String()
}

func createIndexStatement(dialectName, table string, idx Index) string {
	unique := ""
	if idx.Unique {
		unique = "UNIQUE "
	}
	return fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)",
		unique, quoteIdent(dialectName, idx.Name), quoteIdent(dialectName, table), quoteIdentList(dialectName, idx.Columns))
}

// dropIndexStatement renders a DROP INDEX statement. MySQL requires the
// table name (indexes are table-scoped); Postgres/SQLite index names are
// schema/database-scoped and don't take one.
func dropIndexStatement(dialectName, table, name string) string {
	if dialectName == "mysql" {
		return fmt.Sprintf("DROP INDEX %s ON %s", quoteIdent(dialectName, name), quoteIdent(dialectName, table))
	}
	return fmt.Sprintf("DROP INDEX %s", quoteIdent(dialectName, name))
}

func dropTableStatement(dialectName string, op DropTableOp) string {
	ifExists := ""
	if op.IfExists {
		ifExists = "IF EXISTS "
	}
	return fmt.Sprintf("DROP TABLE %s%s", ifExists, quoteIdent(dialectName, op.Table))
}

func renameTableStatement(dialectName string, op RenameTableOp) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME TO %s", quoteIdent(dialectName, op.From), quoteIdent(dialectName, op.To))
}

func addColumnStatement(dialectName, table string, col Column) (string, error) {
	def, err := columnClause(dialectName, col)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", quoteIdent(dialectName, table), def), nil
}

func dropColumnStatement(dialectName, table, column string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", quoteIdent(dialectName, table), quoteIdent(dialectName, column))
}

func renameColumnStatement(dialectName, table, from, to string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", quoteIdent(dialectName, table), quoteIdent(dialectName, from), quoteIdent(dialectName, to))
}

// autoIncrementRenderer renders the column-definition clause for a column
// with auto_increment set. It returns whether a table-level PRIMARY KEY
// constraint should be suppressed (SQLite's INTEGER PRIMARY KEY AUTOINCREMENT
// already declares the PK inline; Postgres/MySQL still need it separately
// when the PK has more than one column... though for SQLite composite PKs
// with auto_increment are rejected entirely by the renderer itself).
type autoIncrementRenderer func(col Column, pkColumns []string) (clause string, suppressPKConstraint bool, err error)

// buildCreateTable is the shared CREATE TABLE assembly used by all three
// dialects; only auto-increment column rendering differs between them.
func buildCreateTable(dialectName string, op CreateTableOp, renderAutoIncrement autoIncrementRenderer) ([]string, error) {
	pks := primaryKeyColumns(op.Columns)
	suppressPK := false

	var colDefs []string
	for _, col := range op.Columns {
		if col.AutoIncrement {
			clause, suppress, err := renderAutoIncrement(col, pks)
			if err != nil {
				return nil, err
			}
			colDefs = append(colDefs, clause)
			if suppress {
				suppressPK = true
			}
			continue
		}
		def, err := columnClause(dialectName, col)
		if err != nil {
			return nil, err
		}
		colDefs = append(colDefs, def)
	}

	if len(pks) > 0 && !suppressPK {
		colDefs = append(colDefs, fmt.Sprintf("PRIMARY KEY (%s)", quoteIdentList(dialectName, pks)))
	}
	for _, fk := range op.ForeignKeys {
		colDefs = append(colDefs, foreignKeyClause(dialectName, fk))
	}
	for _, chk := range op.Checks {
		colDefs = append(colDefs, checkClause(dialectName, chk))
	}

	ifNotExists := ""
	if op.IfNotExists {
		ifNotExists = "IF NOT EXISTS "
	}
	stmt := fmt.Sprintf("CREATE TABLE %s%s (\n  %s\n)",
		ifNotExists, quoteIdent(dialectName, op.Table), strings.Join(colDefs, ",\n  "))

	statements := []string{stmt}
	for _, idx := range op.Indexes {
		statements = append(statements, createIndexStatement(dialectName, op.Table, idx))
	}
	return statements, nil
}
