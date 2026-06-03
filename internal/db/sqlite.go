package db

import (
	"database/sql"
	"github.com/AbdelrahmanDwedar/mig/internal/config"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type SQLiteDriver struct {
	db     *sql.DB
	config *config.DatabaseConfig
}

func (d *SQLiteDriver) Connect() error {
	db, err := sql.Open("sqlite3", d.config.DBName)
	if err != nil {
		return err
	}
	d.db = db
	return nil
}

func (d *SQLiteDriver) Close() error {
	return d.db.Close()
}

func (d *SQLiteDriver) EnsureMigrationsTable() error {
	_, err := d.db.Exec("CREATE TABLE IF NOT EXISTS _migrations (uuid CHAR(36) PRIMARY KEY, migration TEXT, batch INTEGER)")
	return err
}

func (d *SQLiteDriver) GetAppliedMigrations() ([]string, error) {
	rows, err := d.db.Query("SELECT migration FROM _migrations ORDER BY batch DESC, migration ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

func (d *SQLiteDriver) ApplyMigration(name, upSQL string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(upSQL); err != nil {
		tx.Rollback()
		return err
	}

	id := uuid.New().String()
	if _, err := tx.Exec("INSERT INTO _migrations (uuid, migration, batch) VALUES (?, ?, ?)", id, name, 1); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (d *SQLiteDriver) RollbackMigration(name, downSQL string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(downSQL); err != nil {
		tx.Rollback()
		return err
	}

	if _, err := tx.Exec("DELETE FROM _migrations WHERE migration = ?", name); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
