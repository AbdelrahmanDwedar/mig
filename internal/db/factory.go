package db

import (
	"fmt"
	"github.com/AbdelrahmanDwedar/mig/internal/config"
)

func NewDriver(cfg *config.DatabaseConfig) (Driver, error) {
	switch cfg.Driver {
	case "postgresql":
		return &PostgresDriver{config: cfg}, nil
	case "mysql":
		return &MySQLDriver{config: cfg}, nil
	case "sqlite":
		return &SQLiteDriver{config: cfg}, nil
	default:
		return nil, fmt.Errorf("unsupported driver: %s", cfg.Driver)
	}
}
