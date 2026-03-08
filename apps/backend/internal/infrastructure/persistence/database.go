// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Package persistence provides the database connection factory.
// All repository implementations live in internal/domain/repositories.
package persistence

import (
	"database/sql"
	"fmt"

	glebarez_sqlite "github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Database wraps the GORM connection and exposes the underlying *sql.DB for
// go-migrate and any other component that needs the raw driver.
type Database struct {
	db     *sql.DB
	gormDB *gorm.DB
	driver string
}

// Open opens a database connection, initialises GORM, and returns a Database
// wrapper. All three drivers (sqlite, postgres, mysql) work with CGO_ENABLED=0.
func Open(driver, dsn string) (*Database, error) {
	gormCfg := &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	}

	var gormDB *gorm.DB
	var err error

	switch driver {
	case "sqlite", "sqlite3":
		gormDB, err = gorm.Open(glebarez_sqlite.Open(dsn), gormCfg)
	case "postgres", "pgx":
		gormDB, err = gorm.Open(postgres.Open(dsn), gormCfg)
	case "mysql":
		gormDB, err = gorm.Open(mysql.Open(dsn), gormCfg)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
	if err != nil {
		return nil, fmt.Errorf("gorm.Open: %w", err)
	}

	rawDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("gormDB.DB(): %w", err)
	}

	return &Database{db: rawDB, gormDB: gormDB, driver: driver}, nil
}

func (d *Database) DB() *sql.DB      { return d.db }
func (d *Database) GormDB() *gorm.DB { return d.gormDB }
func (d *Database) Close() error     { return d.db.Close() }
func (d *Database) Driver() string   { return d.driver }
