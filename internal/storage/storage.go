package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"devlog/internal/errors"
)

const (
	DefaultQueryTimeout     = 5 * time.Second
	DefaultQueryTimeoutLong = 10 * time.Second
	DefaultMaxOpenConns     = 5
	DefaultMaxIdleConns     = 5
	DefaultConnMaxLifetime  = 0 // 0 means connections are reused forever
	DefaultDirPermissions   = 0755
)

type Storage struct {
	db *sql.DB
}

type stdoutMigrationLogger struct{}

func (stdoutMigrationLogger) Printf(format string, v ...interface{}) {
	fmt.Printf(format+"\n", v...)
}

func New(dbPath string) (*Storage, error) {
	_, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("database does not exist at %s (run with --init to create)", dbPath)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, errors.WrapStorage("open database", err)
	}

	db.SetMaxOpenConns(DefaultMaxOpenConns)
	db.SetMaxIdleConns(DefaultMaxIdleConns)
	db.SetConnMaxLifetime(DefaultConnMaxLifetime)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, errors.WrapStorage("enable WAL mode", err)
	}

	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		db.Close()
		return nil, errors.WrapStorage("set synchronous mode", err)
	}

	if err := RunMigrations(db, nil); err != nil {
		db.Close()
		return nil, errors.WrapStorage("run migrations", err)
	}

	if _, err := db.Exec("PRAGMA optimize"); err != nil {
		db.Close()
		return nil, errors.WrapStorage("optimize database", err)
	}

	return &Storage{
		db: db,
	}, nil
}

func InitDB(dbPath string) error {
	if _, err := os.Stat(dbPath); err == nil {
		return fmt.Errorf("database already exists at %s", dbPath)
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, DefaultDirPermissions); err != nil {
		return errors.WrapStorage("create database directory", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return errors.WrapStorage("create database", err)
	}
	defer db.Close()

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return errors.WrapStorage("enable WAL mode", err)
	}

	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		return errors.WrapStorage("set synchronous mode", err)
	}

	if err := RunMigrations(db, &stdoutMigrationLogger{}); err != nil {
		return errors.WrapStorage("run migrations", err)
	}

	fmt.Printf("Created database at %s\n", dbPath)
	return nil
}

func (s *Storage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
