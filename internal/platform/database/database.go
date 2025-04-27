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

// NewStorage initializes and returns the configured storage implementation(s).
// It returns both a TransactionLogger and a StateStorage.
// If dbType is "none", both returned interfaces will be non-nil (noop implementations).
func NewStorage(ctx context.Context, log logger.Logger, dbType types.DBType, connStr string, poolMaxConnsStr string) (storage.TransactionLogger, storage.StateStorage, error) {
	var txLogger storage.TransactionLogger
	var stateStorage storage.StateStorage
	var err error

	switch dbType {
	case types.Postgres:
		if connStr == "" {
			return nil, nil, fmt.Errorf("для PostgreSQL: %w", ErrMissingConnectionString)
		}
		log.Info("Инициализация хранилища PostgreSQL...")
		txLogger, stateStorage, err = postgres.NewStore(ctx, log, connStr, poolMaxConnsStr)
		if err != nil {
			return nil, nil, fmt.Errorf("ошибка подключения к PostgreSQL: %w: %w", ErrDBConnectionFailed, err)
		}
	case types.SQLite:
		if connStr == "" {
			return nil, nil, fmt.Errorf("для SQLite: %w", ErrMissingConnectionString)
		}
		log.Info("Инициализация хранилища SQLite...")
		txLogger, stateStorage, err = sqlite.NewStore(ctx, log, connStr)
		if err != nil {
			return nil, nil, fmt.Errorf("ошибка подключения к SQLite: %w: %w", ErrDBConnectionFailed, err)
		}
	case types.None, "":
		log.Info("Логгирование транзакций и сохранение состояния в БД отключены.")
		noopStore := noop.NewStore()
		txLogger = noopStore
		stateStorage = noopStore
	default:
		return nil, nil, fmt.Errorf("%w: %s (ожидается '%s', '%s' или '%s')",
			ErrUnsupportedDBType, dbType, types.Postgres, types.SQLite, types.None)
	}

	return txLogger, stateStorage, nil
}
