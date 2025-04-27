package noop

import (
	"context"

	"retro/internal/storage"
)

// NoOpStorage is an implementation of TransactionLogger that does nothing.
// Useful when database logging is disabled.
type noOpStorage struct{}

// Compile-time checks to ensure noOpStorage implements both interfaces.
var _ storage.TransactionLogger = (*noOpStorage)(nil)
var _ storage.StateStorage = (*noOpStorage)(nil)

// NewStore creates a new no-operation storage instance.
// It now returns the concrete type which implements both interfaces.
func NewStore() *noOpStorage {
	return &noOpStorage{}
}

// LogTransaction does nothing.
func (s *noOpStorage) LogTransaction(ctx context.Context, record storage.TransactionRecord) error {
	// No operation
	return nil
}

// Close does nothing.
func (s *noOpStorage) Close() error {
	// No operation
	return nil
}

// GetState always returns ErrStateNotFound for the NoOp store.
func (s *noOpStorage) GetState(ctx context.Context, key string) (string, error) {
	return "", storage.ErrStateNotFound
}

// SetState does nothing for the NoOp store.
func (s *noOpStorage) SetState(ctx context.Context, key, value string) error {
	return nil // No operation, always successful
}
