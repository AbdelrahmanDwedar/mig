package sqlgen

import "fmt"

// Dialect renders JSON migration ops into dialect-native SQL statements.
// One op may expand into multiple statements (e.g. create_table plus its
// inline indexes/foreign keys as separate ALTER/CREATE INDEX statements).
type Dialect interface {
	Name() string
	CreateTable(op CreateTableOp) ([]string, error)
	DropTable(op DropTableOp) ([]string, error)
	RenameTable(op RenameTableOp) ([]string, error)
	AddColumn(op AddColumnOp) ([]string, error)
	DropColumn(op DropColumnOp) ([]string, error)
	RenameColumn(op RenameColumnOp) ([]string, error)
	AlterColumn(op AlterColumnOp) ([]string, error)
	AddIndex(op AddIndexOp) ([]string, error)
	DropIndex(op DropIndexOp) ([]string, error)
	AddForeignKey(op AddForeignKeyOp) ([]string, error)
	DropForeignKey(op DropForeignKeyOp) ([]string, error)
	AddConstraint(op AddConstraintOp) ([]string, error)
	DropConstraint(op DropConstraintOp) ([]string, error)
}

// New returns the sqlgen.Dialect for a mig database driver name
// ("postgresql", "mysql", "sqlite" — see internal/db.NewDriver).
func New(driver string) (Dialect, error) {
	switch driver {
	case "postgresql":
		return &postgresDialect{}, nil
	case "mysql":
		return &mysqlDialect{}, nil
	case "sqlite":
		return &sqliteDialect{}, nil
	default:
		return nil, fmt.Errorf("unsupported driver for JSON migrations: %q", driver)
	}
}
