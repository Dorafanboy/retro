package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"retro_template/internal/logger"
	"retro_template/internal/storage"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// store implements storage.TransactionLogger using SQLite.
type store struct {
	db *sql.DB
}

const createTableSQL = `
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

// NewStore creates a new SQLite transaction logger.
// It opens the database file at dbPath and ensures the necessary table exists.
func NewStore(ctx context.Context, dbPath string) (storage.TransactionLogger, error) {
	logger.Info("Initializing SQLite database...", "path", dbPath)

	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create directory for sqlite db %s: %w", dir, err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_busy_timeout=5000") // Enable WAL for better concurrency
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %s: %w", dbPath, err)
	}

	// Check connection
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping sqlite database at %s: %w", dbPath, err)
	}

	// Create table if it doesn't exist
	if _, err = db.ExecContext(ctx, createTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create transactions table: %w", err)
	}

	logger.Success("SQLite database initialized successfully.", "path", dbPath)
	return &store{db: db}, nil
}

// LogTransaction saves a transaction record to the SQLite database.
func (s *store) LogTransaction(ctx context.Context, record storage.TransactionRecord) error {
	query := `INSERT INTO transactions (timestamp, wallet_address, task_name, network, tx_hash, status, error_message)
               VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		record.Timestamp,
		record.WalletAddress,
		record.TaskName,
		record.Network,
		record.TxHash,
		record.Status,
		record.Error,
	)

	if err != nil {
		logger.Error("Failed to insert transaction log into SQLite DB", "error", err)
		return fmt.Errorf("failed to execute insert query in sqlite: %w", err)
	}
	logger.Debug("Transaction log saved to SQLite DB", "wallet", record.WalletAddress, "task", record.TaskName, "status", record.Status)
	return nil
}

// Close closes the database connection.
func (s *store) Close() error {
	if s.db != nil {
		logger.Info("Closing SQLite database connection...")
		return s.db.Close()
	}
	return nil
}
