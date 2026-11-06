package scanner

import (
	"context"
	"database/sql"
	"fmt"
)

type Scanner interface {
	Scan(ctx context.Context) (*Schema, error)
}

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
