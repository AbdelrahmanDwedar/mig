package sqlgen

import "fmt"

// sqliteDialect implements the known v1 limitations documented in the JSON
// migrations design: no alter_column, and no post-creation foreign
// key/constraint mutation — SQLite's ALTER TABLE doesn't support either.
// Callers are directed to the "sql" escape hatch or to bake the desired
// shape into create_table.
type sqliteDialect struct{}

func (d *sqliteDialect) Name() string { return "sqlite" }

func (d *sqliteDialect) CreateTable(op CreateTableOp) ([]string, error) {
	return buildCreateTable("sqlite", op, func(col Column, pks []string) (string, bool, error) {
		if len(pks) != 1 || pks[0] != col.Name {
			return "", false, fmt.Errorf("sqlite auto_increment column %q must be the sole primary key column (composite primary keys are not supported)", col.Name)
		}
		if col.Type != "integer" && col.Type != "bigint" {
			return "", false, fmt.Errorf("sqlite auto_increment is only supported for integer/bigint columns, got %q on %q", col.Type, col.Name)
		}
		return fmt.Sprintf("%s INTEGER PRIMARY KEY AUTOINCREMENT", quoteIdent("sqlite", col.Name)), true, nil
	})
}

func (d *sqliteDialect) DropTable(op DropTableOp) ([]string, error) {
	return []string{dropTableStatement("sqlite", op)}, nil
}

func (d *sqliteDialect) RenameTable(op RenameTableOp) ([]string, error) {
	return []string{renameTableStatement("sqlite", op)}, nil
}

func (d *sqliteDialect) AddColumn(op AddColumnOp) ([]string, error) {
	stmt, err := addColumnStatement("sqlite", op.Table, op.Column)
	if err != nil {
		return nil, err
	}
	return []string{stmt}, nil
}

func (d *sqliteDialect) DropColumn(op DropColumnOp) ([]string, error) {
	return []string{dropColumnStatement("sqlite", op.Table, op.Column)}, nil
}

func (d *sqliteDialect) RenameColumn(op RenameColumnOp) ([]string, error) {
	return []string{renameColumnStatement("sqlite", op.Table, op.From, op.To)}, nil
}

func (d *sqliteDialect) AlterColumn(op AlterColumnOp) ([]string, error) {
	return nil, fmt.Errorf("sqlite does not support alter_column (type/nullable/default changes); bake the desired shape into create_table or use the \"sql\" escape hatch")
}

func (d *sqliteDialect) AddIndex(op AddIndexOp) ([]string, error) {
	return []string{createIndexStatement("sqlite", op.Table, op.Index)}, nil
}

func (d *sqliteDialect) DropIndex(op DropIndexOp) ([]string, error) {
	return []string{dropIndexStatement("sqlite", op.Table, op.Name)}, nil
}

func (d *sqliteDialect) AddForeignKey(op AddForeignKeyOp) ([]string, error) {
	return nil, fmt.Errorf("sqlite does not support adding foreign keys after table creation; define them in create_table or use the \"sql\" escape hatch")
}

func (d *sqliteDialect) DropForeignKey(op DropForeignKeyOp) ([]string, error) {
	return nil, fmt.Errorf("sqlite does not support dropping foreign keys; use the \"sql\" escape hatch (rebuild-table pattern)")
}

func (d *sqliteDialect) AddConstraint(op AddConstraintOp) ([]string, error) {
	return nil, fmt.Errorf("sqlite does not support adding constraints after table creation; define them in create_table or use the \"sql\" escape hatch")
}

func (d *sqliteDialect) DropConstraint(op DropConstraintOp) ([]string, error) {
	return nil, fmt.Errorf("sqlite does not support dropping constraints; use the \"sql\" escape hatch (rebuild-table pattern)")
}
