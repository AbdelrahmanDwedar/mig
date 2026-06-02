package db

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/AbdelrahmanDwedar/mig/internal/config"
)

type PostgresDriver struct {
	db     *sql.DB
	config *config.DatabaseConfig
}

func (d *PostgresDriver) Connect() error {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		d.config.Host, d.config.Port, d.config.User, d.config.Password, d.config.DBName)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	d.db = db
	return nil
}

func (d *PostgresDriver) Close() error { return d.db.Close() }

func (d *PostgresDriver) EnsureMigrationsTable() error {
	_, err := d.db.Exec("CREATE TABLE IF NOT EXISTS _migrations (uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(), migration TEXT, batch INTEGER)")
	return err
}

func (d *PostgresDriver) GetAppliedMigrations() ([]string, error) {
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

func (d *PostgresDriver) ApplyMigration(name, upSQL string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(upSQL); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec("INSERT INTO _migrations (migration, batch) VALUES ($1, $2)", name, 1); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}


func (d *PostgresDriver) RollbackMigration(name, downSQL string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(downSQL); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec("DELETE FROM _migrations WHERE migration = $1", name); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}
