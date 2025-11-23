package db

import (
	"embed"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func (db *DB) Migrate() error {
	db.log.Debug("running migration")
	files, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return err
	}
	driver, err := pgx.WithInstance(db.conn.DB, &pgx.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance("myiofs", files, "mypg", driver)
	if err != nil {
		return err
	}

	err = m.Up()

	if err != nil {
		if err != migrate.ErrNoChange {
			db.log.Error("migration failed", "error", err)
			return err
		}
		db.log.Debug("migration did not change anything")
	}

	db.log.Debug("migration finished")
	return nil
}

func (d *DB) CleanMigrations() error {
	d.log.Debug("cleaning migrations")

	driver, err := pgx.WithInstance(d.conn.DB, &pgx.Config{})
	if err != nil {
		return err
	}

	source, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return err
	}

	err = m.Force(1)
	if err != nil {
		d.log.Warn("force version failed", "error", err)
	}

	d.log.Debug("migrations cleaned")
	return nil
}
