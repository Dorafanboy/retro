package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"retro/internal/app"
	"retro/internal/bootstrap"
	"retro/internal/config"
	"retro/internal/logger"
	"retro/internal/platform/database"
	"retro/internal/wallet"

	"github.com/joho/godotenv"

	_ "github.com/mattn/go-sqlite3"
)

var (
	configPath  = flag.String("config", "config/config.yml", "Path to the configuration file")
	walletsPath = flag.String("wallets", "local/data/private_keys.txt", "Path to the private keys file")
)

func main() {
	_ = godotenv.Load()

	colorLogger := logger.NewColorLogger()

	defer func() {
		if r := recover(); r != nil {
			colorLogger.Fatal("Критическая ошибка (panic)", "error", r)
		}
	}()

	rand.Seed(time.Now().UnixNano())
	flag.Parse()

	colorLogger.Info("Запуск Retro Template...")

	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())

	colorLogger.Info("Загрузка конфигурации...", "path", *configPath)
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		if errors.Is(err, config.ErrConfigNotFound) {
			colorLogger.Fatal("Файл конфигурации не найден", "path", *configPath, "error", err)
		} else if errors.Is(err, config.ErrConfigParseFailed) {
			colorLogger.Fatal("Ошибка парсинга файла конфигурации (проверьте YAML синтаксис)",
				"path", *configPath, "error", err)
		} else {
			colorLogger.Fatal("Не удалось прочитать файл конфигурации",
				"path", *configPath, "error", err)
		}
	}
	colorLogger.Info("Конфигурация успешно загружена", "max_parallel", cfg.Concurrency.MaxParallelWallets)

	txLogger, stateStorage, err := database.NewStorage(
		ctx,
		colorLogger,
		cfg.Database.Type,
		cfg.Database.ConnectionString,
		cfg.Database.PoolMaxConns,
	)
	if err != nil {
		if errors.Is(err, database.ErrUnsupportedDBType) || errors.Is(err, database.ErrMissingConnectionString) {
			colorLogger.Fatal("Ошибка конфигурации хранилища данных",
				"db_type", cfg.Database.Type, "error", err)
		} else {
			colorLogger.Fatal("Не удалось инициализировать хранилище данных",
				"db_type", cfg.Database.Type, "error", err)
		}
	}

	colorLogger.Info("Загрузка кошельков...", "path", *walletsPath)
	wallets, err := wallet.LoadWallets(*walletsPath, colorLogger)
	if err != nil {
		if errors.Is(err, wallet.ErrWalletsFileNotFound) {
			colorLogger.Fatal("Файл кошельков не найден", "path", *walletsPath, "error", err)
		} else if errors.Is(err, wallet.ErrNoValidKeysFound) {
			colorLogger.Fatal("В файле кошельков не найдено валидных ключей",
				"path", *walletsPath, "error", err)
		} else {
			colorLogger.Fatal("Не удалось прочитать файл кошельков",
				"path", *walletsPath, "error", err)
		}
	}
	colorLogger.Info("Кошельки успешно загружены", "count", len(wallets))

	bootstrap.RegisterTasksFromConfig(cfg, colorLogger)

	appInstance := app.NewApplication(cfg, wallets, &wg, txLogger, stateStorage, colorLogger)

	go gracefulShutdown(cancel, colorLogger, txLogger, stateStorage)

	appInstance.Run(ctx)

	select {
	case <-ctx.Done():
		colorLogger.Warn("Контекст был отменен.")
	default:
	}

	colorLogger.Info("Retro Template ожидает завершения операций перед выходом...")
	wg.Wait()
	colorLogger.Info("Retro Template завершил работу.")
}

// gracefulShutdown handles termination signals and cleans up resources.
func gracefulShutdown(cancel context.CancelFunc, log logger.Logger, closers ...io.Closer) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-signalChan
	log.Warn("Получен сигнал завершения", "signal", sig.String())
	log.Warn("Инициируется плавная остановка... Отменяем контекст.")
	cancel()

	log.Info("Graceful shutdown: закрытие ресурсов...")

	for i, closer := range closers {
		if closer != nil {
			log.Debug("Closing resource...", "index", i+1)
			if err := closer.Close(); err != nil {
				log.Error("Ошибка закрытия ресурса при остановке", "index", i+1, "error", err)
			} else {
				log.Debug("Resource closed.", "index", i+1)
			}
		}
	}

	log.Info("Graceful shutdown: сигнал обработан, контекст отменен, ресурсы закрыты.")
}
