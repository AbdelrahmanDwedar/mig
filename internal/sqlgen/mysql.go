package sqlgen

import (
	"fmt"
	"strings"
)

type mysqlDialect struct{}

func (d *mysqlDialect) Name() string { return "mysql" }

func (d *mysqlDialect) CreateTable(op CreateTableOp) ([]string, error) {
	return buildCreateTable("mysql", op, func(col Column, pks []string) (string, bool, error) {
		if col.Type != "integer" && col.Type != "bigint" {
			return "", false, fmt.Errorf("auto_increment is only supported for integer/bigint columns, got %q on %q", col.Type, col.Name)
		}
		native, err := NativeColumnType("mysql", col.Type, col.Length, col.Precision, col.Scale)
		if err != nil {
			return "", false, err
		}
		return fmt.Sprintf("%s %s AUTO_INCREMENT", quoteIdent("mysql", col.Name), native), false, nil
	})
}

func (d *mysqlDialect) DropTable(op DropTableOp) ([]string, error) {
	return []string{dropTableStatement("mysql", op)}, nil
}

func (d *mysqlDialect) RenameTable(op RenameTableOp) ([]string, error) {
	return []string{renameTableStatement("mysql", op)}, nil
}

func (d *mysqlDialect) AddColumn(op AddColumnOp) ([]string, error) {
	stmt, err := addColumnStatement("mysql", op.Table, op.Column)
	if err != nil {
		return nil, err
	}
	return []string{stmt}, nil
}

func (d *mysqlDialect) DropColumn(op DropColumnOp) ([]string, error) {
	return []string{dropColumnStatement("mysql", op.Table, op.Column)}, nil
}

func (d *mysqlDialect) RenameColumn(op RenameColumnOp) ([]string, error) {
	return []string{renameColumnStatement("mysql", op.Table, op.From, op.To)}, nil
}

// AlterColumn uses MODIFY COLUMN, which redefines the entire column — MySQL
// has no standalone "ALTER COLUMN ... TYPE" clause. "type" must therefore be
// specified even when only nullable/default is changing.
func (d *mysqlDialect) AlterColumn(op AlterColumnOp) ([]string, error) {
	if op.Type == nil {
		return nil, fmt.Errorf("mysql alter_column requires \"type\" to be specified (MODIFY COLUMN redefines the full column)")
	}
	length, precision, scale := 0, 0, 0
	if op.Length != nil {
		length = *op.Length
	}
	if op.Precision != nil {
		precision = *op.Precision
	}
	if op.Scale != nil {
		scale = *op.Scale
	}
	native, err := NativeColumnType("mysql", *op.Type, length, precision, scale)
	if err != nil {
		return nil, err
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s %s", quoteIdent("mysql", op.Table), quoteIdent("mysql", op.Column), native))
	if op.Nullable != nil && !*op.Nullable {
		b.WriteString(" NOT NULL")
	}
	if op.Default != nil {
		rendered, err := renderDefault(op.Default)
		if err != nil {
			return nil, err
		}
		b.WriteString(" DEFAULT ")
		b.WriteString(rendered)
	}
	return []string{b.String()}, nil
}

func (d *mysqlDialect) AddIndex(op AddIndexOp) ([]string, error) {
	return []string{createIndexStatement("mysql", op.Table, op.Index)}, nil
}

func (d *mysqlDialect) DropIndex(op DropIndexOp) ([]string, error) {
	return []string{dropIndexStatement("mysql", op.Table, op.Name)}, nil
}

func (d *mysqlDialect) AddForeignKey(op AddForeignKeyOp) ([]string, error) {
	return []string{fmt.Sprintf("ALTER TABLE %s ADD %s", quoteIdent("mysql", op.Table), foreignKeyClause("mysql", op.ForeignKey))}, nil
}

func (d *mysqlDialect) DropForeignKey(op DropForeignKeyOp) ([]string, error) {
	return []string{fmt.Sprintf("ALTER TABLE %s DROP FOREIGN KEY %s", quoteIdent("mysql", op.Table), quoteIdent("mysql", op.Name))}, nil
}

func (d *mysqlDialect) AddConstraint(op AddConstraintOp) ([]string, error) {
	return []string{fmt.Sprintf("ALTER TABLE %s ADD %s", quoteIdent("mysql", op.Table), checkClause("mysql", op.Check))}, nil
}

func (d *mysqlDialect) DropConstraint(op DropConstraintOp) ([]string, error) {
	return []string{fmt.Sprintf("ALTER TABLE %s DROP CHECK %s", quoteIdent("mysql", op.Table), quoteIdent("mysql", op.Name))}, nil
}
