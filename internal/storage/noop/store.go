package noop

import (
	"context"
	"retro_template/internal/storage"
)

// NoOpStorage is an implementation of TransactionLogger that does nothing.
// Useful when database logging is disabled.
type noOpStorage struct{}

// NewStore creates a new no-operation storage logger.
func NewStore() storage.TransactionLogger {
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
