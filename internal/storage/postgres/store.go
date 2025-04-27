package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"retro/internal/logger"
	"retro/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// store implements the storage.TransactionLogger and storage.StateStorage interfaces using PostgreSQL.
type store struct {
	pool *pgxpool.Pool
	log  logger.Logger
}

const createTxTableSQL = `
CREATE TABLE IF NOT EXISTS transactions (
    id SERIAL PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL,
    wallet_address VARCHAR(42) NOT NULL,
    task_name VARCHAR(255) NOT NULL,
    network VARCHAR(255) NOT NULL,
    tx_hash VARCHAR(66),
    status VARCHAR(50) NOT NULL,
    error_message TEXT
);`

const createStateTableSQL = `
CREATE TABLE IF NOT EXISTS application_state (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);`

// NewStore creates a new PostgreSQL transaction logger and state storage.
func NewStore(ctx context.Context, log logger.Logger, connectionString string, maxConnsStr string) (storage.TransactionLogger, storage.StateStorage, error) {
	config, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse connection string: %w", err)
	}

	if maxConnsStr != "" {
		maxConns, err := strconv.Atoi(maxConnsStr)
		if err != nil {
			log.Warn("Invalid DB_POOL_MAX_CONNS value, using default", "value", maxConnsStr, "error", err)
		} else if maxConns > 0 {
			config.MaxConns = int32(maxConns)
			log.Info("Setting max DB connections", "count", config.MaxConns)
		}
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Defer closing the pool if initialization fails after this point
	defer func() {
		if err != nil && pool != nil {
			pool.Close()
		}
	}()

	err = pool.Ping(ctx)
	if err != nil {
		// pool is closed by defer
		return nil, nil, fmt.Errorf("unable to ping database: %w", err)
	}

	// Create transactions table if it doesn't exist
	if _, err = pool.Exec(ctx, createTxTableSQL); err != nil {
		// pool is closed by defer
		return nil, nil, fmt.Errorf("failed to create transactions table: %w", err)
	}
	log.Info("Table 'transactions' initialized successfully (or already existed).")

	// Create application_state table if it doesn't exist
	if _, err = pool.Exec(ctx, createStateTableSQL); err != nil {
		// pool is closed by defer
		return nil, nil, fmt.Errorf("failed to create application_state table: %w", err)
	}
	log.Info("Table 'application_state' initialized successfully (or already existed).")

	log.Success("Successfully connected to PostgreSQL.")
	s := &store{pool: pool, log: log}
	return s, s, nil // Return store instance for both interfaces
}

// LogTransaction saves a transaction record to the 'transactions' table.
func (s *store) LogTransaction(ctx context.Context, record storage.TransactionRecord) error {
	query := `INSERT INTO transactions (timestamp, wallet_address, task_name, network, tx_hash, status, error_message) 
	           VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := s.pool.Exec(ctx, query,
		record.Timestamp,
		record.WalletAddress,
		string(record.TaskName),
		record.Network,
		record.TxHash,
		string(record.Status),
		record.Error,
	)

	if err != nil {
		s.log.Error("Failed to insert transaction log into DB", "error", err, "wallet", record.WalletAddress, "task", record.TaskName)
		return fmt.Errorf("failed to execute insert query: %w", err)
	}
	s.log.Debug("Transaction log saved to DB", "wallet", record.WalletAddress, "task", record.TaskName, "status", record.Status)
	return nil
}

// GetState retrieves a value from the application_state table.
func (s *store) GetState(ctx context.Context, key string) (string, error) {
	query := `SELECT value FROM application_state WHERE key = $1`
	var value string
	err := s.pool.QueryRow(ctx, query, key).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", storage.ErrStateNotFound // Используем нашу ошибку
		}
		s.log.Error("Failed to query state from DB", "key", key, "error", err)
		return "", fmt.Errorf("failed to query state for key '%s': %w", key, err)
	}
	return value, nil
}

// SetState saves or updates a key-value pair in the application_state table.
func (s *store) SetState(ctx context.Context, key, value string) error {
	query := `INSERT INTO application_state (key, value)
	           VALUES ($1, $2)
	           ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`
	_, err := s.pool.Exec(ctx, query, key, value)
	if err != nil {
		s.log.Error("Failed to set state in DB", "key", key, "error", err)
		return fmt.Errorf("failed to set state for key '%s': %w", key, err)
	}
	s.log.Debug("State saved to DB", "key", key)
	return nil
}

// Close closes the database connection pool.
func (s *store) Close() error {
	if s.pool != nil {
		s.log.Info("Closing PostgreSQL connection pool...")
		s.pool.Close()
	}
	return nil
}
