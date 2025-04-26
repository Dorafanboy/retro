package main

import (
	"context"
	"errors"
	"flag"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"retro/internal/app"
	"retro/internal/config"
	"retro/internal/logger"
	"retro/internal/platform/database"
	"retro/internal/tasks"
	"retro/internal/wallet"

	"github.com/joho/godotenv"

	_ "retro/internal/tasks"
	_ "retro/internal/tasks/dummy"

	_ "github.com/mattn/go-sqlite3"
)

var (
	configPath  = flag.String("config", "config/config.yml", "Path to the configuration file")
	walletsPath = flag.String("wallets", "local/data/private_keys.txt", "Path to the private keys file")
)

func main() {
	_ = godotenv.Load()

	defer func() {
		if r := recover(); r != nil {
			logger.Fatal("Критическая ошибка (panic)", "error", r)
		}
	}()

	rand.Seed(time.Now().UnixNano())
	flag.Parse()

	logger.Info("Запуск Retro Template...")

	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go gracefulShutdown(cancel)

	logger.Info("Загрузка конфигурации...", "path", *configPath)
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		if errors.Is(err, config.ErrConfigNotFound) {
			logger.Fatal("Файл конфигурации не найден", "path", *configPath, "error", err)
		} else if errors.Is(err, config.ErrConfigParseFailed) {
			logger.Fatal("Ошибка парсинга файла конфигурации (проверьте YAML синтаксис)", "path", *configPath, "error", err)
		} else {
			logger.Fatal("Не удалось прочитать файл конфигурации", "path", *configPath, "error", err)
		}
	}
	logger.Info("Конфигурация успешно загружена", "max_parallel", cfg.Concurrency.MaxParallelWallets)

	txLogger, err := database.NewTransactionLogger(ctx, cfg.Database.Type, cfg.Database.ConnectionString, cfg.Database.PoolMaxConns)
	if err != nil {
		if errors.Is(err, database.ErrUnsupportedDBType) || errors.Is(err, database.ErrMissingConnectionString) {
			logger.Fatal("Ошибка конфигурации логгера транзакций", "db_type", cfg.Database.Type, "error", err)
		} else {
			logger.Fatal("Не удалось инициализировать логгер транзакций", "db_type", cfg.Database.Type, "error", err)
		}
	}
	defer func() {
		if err := txLogger.Close(); err != nil {
			logger.Error("Ошибка закрытия логгера транзакций", "error", err)
		}
	}()

	logger.Info("Загрузка кошельков...", "path", *walletsPath)
	wallets, err := wallet.LoadWallets(*walletsPath)
	if err != nil {
		if errors.Is(err, wallet.ErrWalletsFileNotFound) {
			logger.Fatal("Файл кошельков не найден", "path", *walletsPath, "error", err)
		} else if errors.Is(err, wallet.ErrNoValidKeysFound) {
			logger.Fatal("В файле кошельков не найдено валидных ключей", "path", *walletsPath, "error", err)
		} else {
			logger.Fatal("Не удалось прочитать файл кошельков", "path", *walletsPath, "error", err)
		}
	}
	logger.Info("Кошельки успешно загружены", "count", len(wallets))

	registeredTasks := tasks.ListTasks()
	logger.Info("Зарегистрированные задачи", "tasks", registeredTasks)

	appInstance := app.NewApplication(cfg, wallets, registeredTasks, &wg)

	appInstance.Run(ctx)

	select {
	case <-ctx.Done():
		logger.Warn("Контекст был отменен.")
	default:
	}

	logger.Info("Retro Template ожидает завершения операций перед выходом...")
	wg.Wait()
	logger.Info("Retro Template завершил работу.")
}

// gracefulShutdown handles termination signals.
func gracefulShutdown(cancel context.CancelFunc) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-signalChan
	logger.Warn("Получен сигнал завершения", "signal", sig.String())
	logger.Warn("Инициируется плавная остановка... Отменяем контекст.")
	cancel()

	logger.Info("Graceful shutdown: сигнал обработан, контекст отменен.")
}
