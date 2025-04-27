package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"retro/internal/logger"
	"retro/internal/storage"

	_ "github.com/mattn/go-sqlite3"
)

// store implements storage.TransactionLogger and storage.StateStorage using SQLite.
type store struct {
	db  *sql.DB
	log logger.Logger
}

const createTxTableSQL = `
CREATE TABLE IF NOT EXISTS transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME NOT NULL,
    wallet_address TEXT NOT NULL,
    task_name TEXT NOT NULL,
    network TEXT NOT NULL,
    tx_hash TEXT,
    status TEXT NOT NULL,
    error_message TEXT
);`

const createStateTableSQL = `
CREATE TABLE IF NOT EXISTS application_state (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);`

// NewStore creates a new SQLite transaction logger and state storage.
func NewStore(ctx context.Context, log logger.Logger, dbPath string) (storage.TransactionLogger, storage.StateStorage, error) {
	log.Info("Initializing SQLite database...", "path", dbPath)

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, nil, fmt.Errorf("failed to create directory for sqlite db %s: %w", dir, err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open sqlite database at %s: %w", dbPath, err)
	}

	// Defer closing the DB if initialization fails
	defer func() {
		if err != nil && db != nil {
			db.Close()
		}
	}()

	if err = db.PingContext(ctx); err != nil {
		// db is closed by defer
		return nil, nil, fmt.Errorf("failed to ping sqlite database at %s: %w", dbPath, err)
	}

	// Create transactions table
	if _, err = db.ExecContext(ctx, createTxTableSQL); err != nil {
		// db is closed by defer
		return nil, nil, fmt.Errorf("failed to create transactions table: %w", err)
	}
	log.Info("Table 'transactions' initialized successfully (or already existed).")

	// Create application_state table
	if _, err = db.ExecContext(ctx, createStateTableSQL); err != nil {
		// db is closed by defer
		return nil, nil, fmt.Errorf("failed to create application_state table: %w", err)
	}
	log.Info("Table 'application_state' initialized successfully (or already existed).")

	log.Success("SQLite database initialized successfully.", "path", dbPath)
	s := &store{db: db, log: log}
	return s, s, nil // Return store instance for both interfaces
}

// LogTransaction saves a transaction record to the SQLite database.
func (s *store) LogTransaction(ctx context.Context, record storage.TransactionRecord) error {
	query := `INSERT INTO transactions (timestamp, wallet_address, task_name, network, tx_hash, status, error_message)
               VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		record.Timestamp,
		record.WalletAddress,
		string(record.TaskName),
		record.Network,
		record.TxHash,
		string(record.Status),
		record.Error,
	)

	if err != nil {
		s.log.Error("Failed to insert transaction log into SQLite DB", "error", err,
			"wallet", record.WalletAddress, "task", record.TaskName)
		return fmt.Errorf("failed to execute insert query in sqlite: %w", err)
	}
	s.log.Debug("Transaction log saved to SQLite DB", "wallet", record.WalletAddress, "task", record.TaskName, "status", record.Status)
	return nil
}

// GetState retrieves a value from the application_state table.
func (s *store) GetState(ctx context.Context, key string) (string, error) {
	query := `SELECT value FROM application_state WHERE key = ?`
	var value string
	err := s.db.QueryRowContext(ctx, query, key).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", storage.ErrStateNotFound // Используем нашу ошибку
		}
		s.log.Error("Failed to query state from SQLite DB", "key", key, "error", err)
		return "", fmt.Errorf("failed to query state from sqlite for key '%s': %w", key, err)
	}
	return value, nil
}

// SetState saves or updates a key-value pair in the application_state table.
func (s *store) SetState(ctx context.Context, key, value string) error {
	query := `INSERT INTO application_state (key, value)
	           VALUES (?, ?)
	           ON CONFLICT (key) DO UPDATE SET value = excluded.value`
	_, err := s.db.ExecContext(ctx, query, key, value)
	if err != nil {
		s.log.Error("Failed to set state in SQLite DB", "key", key, "error", err)
		return fmt.Errorf("failed to set state in sqlite for key '%s': %w", key, err)
	}
	s.log.Debug("State saved to SQLite DB", "key", key)
	return nil
}

// Close closes the database connection.
func (s *store) Close() error {
	s.log.Info("Closing SQLite database connection...")
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
