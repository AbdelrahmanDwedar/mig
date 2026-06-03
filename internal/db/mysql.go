package db

import (
	"database/sql"
	"fmt"
	"github.com/AbdelrahmanDwedar/mig/internal/config"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

type MySQLDriver struct {
	db     *sql.DB
	config *config.DatabaseConfig
}

func (d *MySQLDriver) Connect() error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		d.config.User, d.config.Password, d.config.Host, d.config.Port, d.config.DBName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	d.db = db
	return nil
}

func (d *MySQLDriver) Close() error { return d.db.Close() }

func (d *MySQLDriver) EnsureMigrationsTable() error {
	_, err := d.db.Exec("CREATE TABLE IF NOT EXISTS _migrations (uuid CHAR(36) PRIMARY KEY, migration VARCHAR(255), batch INTEGER)")
	return err
}

func (d *MySQLDriver) GetAppliedMigrations() ([]string, error) {
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

func (d *MySQLDriver) ApplyMigration(name, upSQL string) error {
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

func (d *MySQLDriver) RollbackMigration(name, downSQL string) error {
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
