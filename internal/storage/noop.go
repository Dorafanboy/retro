package storage

import "context"

// NoOpStorage is an implementation of TransactionLogger that does nothing.
// Useful when database logging is disabled.
type noOpStorage struct{}

// NewNoOpStorage creates a new no-operation storage logger.
func NewNoOpStorage() TransactionLogger {
	return &noOpStorage{}
}

// LogTransaction does nothing.
func (s *noOpStorage) LogTransaction(ctx context.Context, record TransactionRecord) error {
	// No operation
	return nil
}

// Close does nothing.
func (s *noOpStorage) Close() error {
	// No operation
	return nil
}
