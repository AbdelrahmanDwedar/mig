package db

import "database/sql"

type Driver interface {
	Connect() error
	Close() error
	// Conn returns the underlying *sql.DB opened by Connect, for callers
	// (e.g. schema scanners) that need direct read access to the connection.
	Conn() *sql.DB
	EnsureMigrationsTable() error
	GetAppliedMigrations() ([]string, error)
	ApplyMigration(name, upSQL string) error
	RollbackMigration(name, downSQL string) error
}
