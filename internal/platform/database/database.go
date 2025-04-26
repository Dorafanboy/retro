package database

import (
	"context"
	"errors"
	"fmt"

	"retro/internal/logger"
	"retro/internal/storage"
	"retro/internal/storage/noop"
	"retro/internal/storage/postgres"
	"retro/internal/storage/sqlite"
	"retro/internal/types"
)

var (
	// ErrUnsupportedDBType indicates that the provided database type is not supported.
	ErrUnsupportedDBType = errors.New("unsupported database type specified")
	// ErrDBConnectionFailed indicates that the attempt to connect to the database failed.
	ErrDBConnectionFailed = errors.New("database connection failed")
	// ErrMissingConnectionString indicates that the database connection string was not provided.
	ErrMissingConnectionString = errors.New("database connection string is missing")
)

// NewTransactionLogger создает экземпляр TransactionLogger на основе переданных параметров.
func NewTransactionLogger(ctx context.Context, dbType types.DBType, connStr, maxConnsStr string) (storage.TransactionLogger, error) {
	var txLogger storage.TransactionLogger
	var err error

	switch dbType {
	case types.Postgres:
		if connStr == "" {
			return nil, fmt.Errorf("для PostgreSQL: %w", ErrMissingConnectionString)
		}
		logger.Info("Инициализация логгера транзакций PostgreSQL...")
		txLogger, err = postgres.NewStore(ctx, connStr, maxConnsStr)
		if err != nil {
			return nil, fmt.Errorf("ошибка подключения к PostgreSQL: %w: %w", ErrDBConnectionFailed, err)
		}
	case types.SQLite:
		if connStr == "" {
			return nil, fmt.Errorf("для SQLite: %w", ErrMissingConnectionString)
		}
		logger.Info("Инициализация логгера транзакций SQLite...")
		txLogger, err = sqlite.NewStore(ctx, connStr)
		if err != nil {
			return nil, fmt.Errorf("ошибка подключения к SQLite: %w: %w", ErrDBConnectionFailed, err)
		}
	case types.None, "":
		logger.Info("Логгирование транзакций в БД отключено.")
		txLogger = noop.NewStore()
	default:
		return nil, fmt.Errorf("%w: %s (ожидается '%s', '%s' или '%s')",
			ErrUnsupportedDBType, dbType, types.Postgres, types.SQLite, types.None)
	}

	return txLogger, nil
}
