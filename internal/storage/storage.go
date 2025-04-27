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
	TxHash        string         `json:"tx_hash,omitempty"` // Can be empty on pre-send errors
	Status        types.TxStatus `json:"status"`            // Use the custom TxStatus type
	Error         string         `json:"error,omitempty"`   // Error message if Status is not "Success"
}

// TransactionLogger defines the interface for storing transaction history.
// Implementations could be Postgres, NoOp, etc.
type TransactionLogger interface {
	// LogTransaction saves a record of an attempted or completed transaction.
	LogTransaction(ctx context.Context, record TransactionRecord) error
	// Close closes any underlying resources (like database connections).
	Close() error
}

// ErrStateNotFound indicates that the requested state key was not found.
var ErrStateNotFound = errors.New("state key not found")

// StateStorage defines the interface for reading and writing application state.
type StateStorage interface {
	// GetState retrieves the value associated with a key.
	// Returns ErrStateNotFound if the key does not exist.
	GetState(ctx context.Context, key string) (string, error)

	// SetState saves a key-value pair.
	SetState(ctx context.Context, key, value string) error

	// Close releases any resources used by the storage.
	// It might be the same underlying connection as TransactionLogger.
	Close() error
}
