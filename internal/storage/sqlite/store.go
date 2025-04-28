package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"retro/internal/logger"
	"retro/internal/storage"

	_ "github.com/mattn/go-sqlite3"
)

// store implements storage.TransactionLogger and storage.StateStorage using SQLite.
type store struct {
	db  *sql.DB
	log logger.Logger
}

// NewStore initializes the database schema (tables) on an existing SQLite connection.
func NewStore(
	log logger.Logger,
	db *sql.DB, // Expect a ready connection
) (storage.TransactionLogger, storage.StateStorage, error) {
	ctx := context.Background()

	log.Info("Initializing SQLite schema (tables)...")
	if _, err := db.ExecContext(ctx, storage.CreateTxTableSQL); err != nil {
		return nil, nil, fmt.Errorf("failed to create transactions table in sqlite: %w", err)
	}
	log.Info("Table 'transactions' initialized successfully (or already existed).")

	if _, err := db.ExecContext(ctx, storage.CreateStateTableSQL); err != nil {
		return nil, nil, fmt.Errorf("failed to create application_state table in sqlite: %w", err)
	}
	log.Info("Table 'application_state' initialized successfully (or already existed).")

	log.Success("SQLite schema initialized.")
	s := &store{db: db, log: log}
	return s, s, nil
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
