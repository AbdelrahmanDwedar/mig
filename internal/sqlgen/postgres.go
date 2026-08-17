package sqlgen

import (
	"fmt"
	"strings"
)

type postgresDialect struct{}

func (d *postgresDialect) Name() string { return "postgresql" }

func (d *postgresDialect) CreateTable(op CreateTableOp) ([]string, error) {
	return buildCreateTable("postgresql", op, func(col Column, pks []string) (string, bool, error) {
		var native string
		switch col.Type {
		case "integer":
			native = "SERIAL"
		case "bigint":
			native = "BIGSERIAL"
		default:
			return "", false, fmt.Errorf("auto_increment is only supported for integer/bigint columns, got %q on %q", col.Type, col.Name)
		}
		return fmt.Sprintf("%s %s", quoteIdent("postgresql", col.Name), native), false, nil
	})
}

func (d *postgresDialect) DropTable(op DropTableOp) ([]string, error) {
	return []string{dropTableStatement("postgresql", op)}, nil
}

func (d *postgresDialect) RenameTable(op RenameTableOp) ([]string, error) {
	return []string{renameTableStatement("postgresql", op)}, nil
}

func (d *postgresDialect) AddColumn(op AddColumnOp) ([]string, error) {
	stmt, err := addColumnStatement("postgresql", op.Table, op.Column)
	if err != nil {
		return nil, err
	}
	return []string{stmt}, nil
}

func (d *postgresDialect) DropColumn(op DropColumnOp) ([]string, error) {
	return []string{dropColumnStatement("postgresql", op.Table, op.Column)}, nil
}

func (d *postgresDialect) RenameColumn(op RenameColumnOp) ([]string, error) {
	return []string{renameColumnStatement("postgresql", op.Table, op.From, op.To)}, nil
}

func (d *postgresDialect) AlterColumn(op AlterColumnOp) ([]string, error) {
	ident := quoteIdent("postgresql", op.Column)
	var clauses []string

	if op.Type != nil {
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
		native, err := NativeColumnType("postgresql", *op.Type, length, precision, scale)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, fmt.Sprintf("ALTER COLUMN %s TYPE %s", ident, native))
	}
	if op.Nullable != nil {
		if *op.Nullable {
			clauses = append(clauses, fmt.Sprintf("ALTER COLUMN %s DROP NOT NULL", ident))
		} else {
			clauses = append(clauses, fmt.Sprintf("ALTER COLUMN %s SET NOT NULL", ident))
		}
	}
	if op.Default != nil {
		rendered, err := renderDefault(op.Default)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, fmt.Sprintf("ALTER COLUMN %s SET DEFAULT %s", ident, rendered))
	}
	if len(clauses) == 0 {
		return nil, fmt.Errorf("alter_column on %s.%s specifies no changes", op.Table, op.Column)
	}
	return []string{fmt.Sprintf("ALTER TABLE %s %s", quoteIdent("postgresql", op.Table), strings.Join(clauses, ", "))}, nil
}

func (d *postgresDialect) AddIndex(op AddIndexOp) ([]string, error) {
	return []string{createIndexStatement("postgresql", op.Table, op.Index)}, nil
}

func (d *postgresDialect) DropIndex(op DropIndexOp) ([]string, error) {
	return []string{dropIndexStatement("postgresql", op.Table, op.Name)}, nil
}

func (d *postgresDialect) AddForeignKey(op AddForeignKeyOp) ([]string, error) {
	return []string{fmt.Sprintf("ALTER TABLE %s ADD %s", quoteIdent("postgresql", op.Table), foreignKeyClause("postgresql", op.ForeignKey))}, nil
}

func (d *postgresDialect) DropForeignKey(op DropForeignKeyOp) ([]string, error) {
	return []string{fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s", quoteIdent("postgresql", op.Table), quoteIdent("postgresql", op.Name))}, nil
}

func (d *postgresDialect) AddConstraint(op AddConstraintOp) ([]string, error) {
	return []string{fmt.Sprintf("ALTER TABLE %s ADD %s", quoteIdent("postgresql", op.Table), checkClause("postgresql", op.Check))}, nil
}

func (d *postgresDialect) DropConstraint(op DropConstraintOp) ([]string, error) {
	return []string{fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s", quoteIdent("postgresql", op.Table), quoteIdent("postgresql", op.Name))}, nil
}
