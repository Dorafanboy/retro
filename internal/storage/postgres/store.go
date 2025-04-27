package postgres

import (
	"context"
	"fmt"
	"strconv"

	"retro/internal/logger"
	"retro/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

// store implements the storage.TransactionLogger interface using PostgreSQL.
type store struct {
	pool *pgxpool.Pool
}

const createTableSQL = `
CREATE TABLE IF NOT EXISTS transactions (
    id SERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    wallet_address VARCHAR(42) NOT NULL,
    task_name VARCHAR(255) NOT NULL,
    network VARCHAR(255) NOT NULL,
    tx_hash VARCHAR(66),
    status VARCHAR(50) NOT NULL,
    error_message TEXT
);`

// NewStore creates a new PostgreSQL transaction logger.
func NewStore(ctx context.Context, connectionString string, maxConnsStr string) (storage.TransactionLogger, error) {
	config, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse connection string: %w", err)
	}

	if maxConnsStr != "" {
		maxConns, err := strconv.Atoi(maxConnsStr)
		if err != nil {
			logger.Warn("Invalid DB_POOL_MAX_CONNS value, using default", "value", maxConnsStr, "error", err)
		} else if maxConns > 0 {
			config.MaxConns = int32(maxConns)
			logger.Info("Setting max DB connections", "count", config.MaxConns)
		}
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
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
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	// Create table if it doesn't exist
	if _, err = pool.Exec(ctx, createTableSQL); err != nil {
		// pool is closed by defer
		return nil, fmt.Errorf("failed to create transactions table: %w", err)
	}
	logger.Info("Table 'transactions' initialized successfully (or already existed).")

	logger.Success("Successfully connected to PostgreSQL.")
	return &store{pool: pool}, nil // Return nil error on success
}

// LogTransaction saves a transaction record to the 'transactions' table.
func (s *store) LogTransaction(ctx context.Context, record storage.TransactionRecord) error {
	query := `INSERT INTO transactions (timestamp, wallet_address, task_name, network, tx_hash, status, error_message) 
	           VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := s.pool.Exec(ctx, query,
		record.Timestamp,
		record.WalletAddress,
		record.TaskName,
		record.Network,
		record.TxHash,
		record.Status,
		record.Error,
	)

	if err != nil {
		logger.Error("Failed to insert transaction log into DB", "error", err)
		return fmt.Errorf("failed to execute insert query: %w", err)
	}
	logger.Debug("Transaction log saved to DB", "wallet", record.WalletAddress, "task", record.TaskName, "status", record.Status)
	return nil
}

// Close closes the database connection pool.
func (s *store) Close() error {
	if s.pool != nil {
		logger.Info("Closing PostgreSQL connection pool...")
		s.pool.Close()
	}
	return nil
}
