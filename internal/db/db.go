package db

type Driver interface {
	Connect() error
	Close() error
	EnsureMigrationsTable() error
	GetAppliedMigrations() ([]string, error)
	ApplyMigration(name, upSQL string) error
	RollbackMigration(name, downSQL string) error
}
