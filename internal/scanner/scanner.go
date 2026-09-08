package scanner

import (
	"context"
	"database/sql"
	"fmt"
)

// Scanner reads a live database's actual schema into a Schema value. It is a
// third concern alongside db.Driver (connect/execute) and sqlgen.Dialect
// (render SQL for an op): Scanner never produces SQL, and Driver never
// introspects — it only exposes the connection via Driver.Conn().
type Scanner interface {
	Scan(ctx context.Context) (*Schema, error)
}

// New returns the Scanner for a mig database driver name ("postgresql",
// "mysql", "sqlite" — the same strings accepted by db.NewDriver and
// sqlgen.New) against an already-open connection.
func New(driverName string, conn *sql.DB) (Scanner, error) {
	switch driverName {
	case "postgresql":
		return &postgresScanner{db: conn}, nil
	case "mysql":
		return &mysqlScanner{db: conn}, nil
	case "sqlite":
		return &sqliteScanner{db: conn}, nil
	default:
		return nil, fmt.Errorf("unsupported driver: %q", driverName)
	}
}
