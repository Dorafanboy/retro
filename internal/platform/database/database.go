package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"retro/internal/logger"
	"retro/internal/storage"
	"retro/internal/storage/noop"
	"retro/internal/storage/postgres"
	"retro/internal/storage/sqlite"
	"retro/internal/types"

	"github.com/jackc/pgx/v5/pgxpool"

	_ "github.com/mattn/go-sqlite3"
)

var (
	ErrUnsupportedDBType       = errors.New("unsupported database type specified")
	ErrMissingConnectionString = errors.New("database connection string is missing")
)

// NewStorage initializes and returns the configured storage implementation(s).
func NewStorage(
	ctx context.Context,
	log logger.Logger,
	dbType types.DBType,
	connStr string,
	poolMaxConnsStr string,
) (storage.TransactionLogger, storage.StateStorage, error) {
	switch dbType {
	case types.Postgres:
		if connStr == "" {
			return nil, nil, fmt.Errorf("для PostgreSQL: %w", ErrMissingConnectionString)
		}
		log.Info("Установка соединения с PostgreSQL...")
		pool, err := setupPostgresConnection(ctx, log, connStr, poolMaxConnsStr)
		if err != nil {
			return nil, nil, fmt.Errorf("ошибка установки соединения с PostgreSQL: %w", err)
		}
		log.Info("Соединение с PostgreSQL установлено. Инициализация хранилища...")
		return postgres.NewStore(pool, log)

	case types.SQLite:
		if connStr == "" {
			return nil, nil, fmt.Errorf("для SQLite: %w", ErrMissingConnectionString)
		}
		log.Info("Установка соединения с SQLite...")
		db, err := setupSQLiteConnection(ctx, log, connStr)
		if err != nil {
			return nil, nil, fmt.Errorf("ошибка установки соединения с SQLite: %w", err)
		}
		log.Info("Соединение с SQLite установлено. Инициализация хранилища...")
		return sqlite.NewStore(log, db)

	case types.None, "":
		log.Info("Логгирование транзакций и сохранение состояния в БД отключены.")
		noopStore := noop.NewStore()
		return noopStore, noopStore, nil

	default:
		return nil, nil, fmt.Errorf("%w: %s (ожидается '%s', '%s' или '%s')",
			ErrUnsupportedDBType, dbType, types.Postgres, types.SQLite, types.None)
	}
}

// setupPostgresConnection handles parsing DSN, configuring, creating, and pinging a PostgreSQL connection pool.
func setupPostgresConnection(
	ctx context.Context,
	log logger.Logger,
	connectionString string,
	maxConnsStr string,
) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse postgres connection string: %w", err)
	}

	if maxConnsStr != "" {
		maxConns, convErr := strconv.Atoi(maxConnsStr)
		if convErr != nil {
			log.Warn("Invalid DB_POOL_MAX_CONNS value, using default",
				"value", maxConnsStr, "error", convErr)
		} else if maxConns > 0 {
			config.MaxConns = int32(maxConns)
			log.Info("Setting max DB connections", "count", config.MaxConns)
		}
	}

	log.Debug("Creating PostgreSQL connection pool...")
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create postgres connection pool: %w", err)
	}

	defer func() {
		if err != nil {
			log.Debug("Closing connection pool due to setup error...")
			pool.Close()
		}
	}()

	log.Debug("Pinging PostgreSQL database...")
	err = pool.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to ping postgres database: %w", err)
	}

	log.Success("Successfully connected to PostgreSQL and pinged.")
	return pool, nil
}

// setupSQLiteConnection handles directory creation, opening, and pinging an SQLite database connection.
func setupSQLiteConnection(
	ctx context.Context,
	log logger.Logger,
	dbPath string,
) (*sql.DB, error) {
	log.Debug("Ensuring directory exists for SQLite database...", "path", dbPath)
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create directory '%s' for sqlite db: %w", dir, err)
	}

	connStr := fmt.Sprintf("%s?_journal=WAL&_busy_timeout=5000", dbPath)
	log.Debug("Opening SQLite database connection...", "connection_string", connStr)
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %s: %w", dbPath, err)
	}

	defer func() {
		if err != nil {
			log.Debug("Closing SQLite connection due to setup error...")
			_ = db.Close()
		}
	}()

	log.Debug("Pinging SQLite database...")
	if err = db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite database at %s: %w", dbPath, err)
	}

	log.Success("Successfully connected to SQLite and pinged.", "path", dbPath)
	return db, nil
}
