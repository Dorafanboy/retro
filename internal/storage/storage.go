package storage

import (
	"context"
	"errors"
	"time"

	"retro/internal/types"
)

// TransactionRecord represents information about an executed transaction attempt.
type TransactionRecord struct {
	Timestamp     time.Time      `json:"timestamp"`
	WalletAddress string         `json:"wallet_address"`
	TaskName      types.TaskName `json:"task_name"`
	Network       string         `json:"network"`
	TxHash        string         `json:"tx_hash,omitempty"`
	Status        types.TxStatus `json:"status"`
	Error         string         `json:"error,omitempty"`
}

// TransactionLogger defines the interface for storing transaction history.
type TransactionLogger interface {
	// LogTransaction saves a record of an attempted or completed transaction.
	LogTransaction(ctx context.Context, record TransactionRecord) error
	// Close closes any underlying resources (like database connections).
	Close() error
}

var ErrStateNotFound = errors.New("state key not found")

// StateStorage defines the interface for reading and writing application state.
type StateStorage interface {
	// GetState retrieves the value associated with a key.
	GetState(ctx context.Context, key string) (string, error)
	// SetState saves a key-value pair.
	SetState(ctx context.Context, key, value string) error
	// Close releases any resources used by the storage.
	Close() error
}
