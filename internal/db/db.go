package db

import "database/sql"

type Driver interface {
	Connect() error
	Close() error
	Conn() *sql.DB
	EnsureMigrationsTable() error
	GetAppliedMigrations() ([]string, error)
	ApplyMigration(name, upSQL string) error
	RollbackMigration(name, downSQL string) error
}
