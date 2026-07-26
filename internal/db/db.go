package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const databaseFile = "jovepoxy.db"
const legacyDatabaseFile = "opencode2api.db"

// Open creates the data directory, configures SQLite, and applies all migrations.
func Open(ctx context.Context, dataDir string) (*sql.DB, error) {
	if dataDir == "" {
		return nil, errors.New("database data directory is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	if err := migrateLegacyDatabaseFile(dataDir); err != nil {
		return nil, err
	}
	database, err := sql.Open("sqlite", databaseDSN(filepath.Join(dataDir, databaseFile)))
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	if err := database.PingContext(ctx); err != nil {
		return nil, closeWithError(database, fmt.Errorf("connect sqlite database: %w", err))
	}
	if err := Migrate(ctx, database); err != nil {
		return nil, closeWithError(database, err)
	}
	return database, nil
}

// migrateLegacyDatabaseFile renames pre-rebrand SQLite files once when present.
func migrateLegacyDatabaseFile(dataDir string) error {
	next := filepath.Join(dataDir, databaseFile)
	if _, err := os.Stat(next); err == nil {
		return nil
	}
	legacy := filepath.Join(dataDir, legacyDatabaseFile)
	if _, err := os.Stat(legacy); err != nil {
		return nil
	}
	if err := os.Rename(legacy, next); err != nil {
		return fmt.Errorf("rename legacy database: %w", err)
	}
	// Best-effort companion WAL/SHM rename (ignore missing).
	_ = os.Rename(legacy+"-wal", next+"-wal")
	_ = os.Rename(legacy+"-shm", next+"-shm")
	return nil
}

func databaseDSN(path string) string {
	return "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=secure_delete(ON)"
}

func closeWithError(database *sql.DB, operationErr error) error {
	if closeErr := database.Close(); closeErr != nil {
		return errors.Join(operationErr, fmt.Errorf("close sqlite database: %w", closeErr))
	}
	return operationErr
}
