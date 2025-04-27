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

	logInstance := logger.NewColorLogger()

	defer func() {
		if r := recover(); r != nil {
			logInstance.Fatal("Критическая ошибка (panic)", "error", r)
		}
	}()

	rand.Seed(time.Now().UnixNano())
	flag.Parse()

	logInstance.Info("Запуск Retro Template...")

	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go gracefulShutdown(cancel, logInstance)

	logInstance.Info("Загрузка конфигурации...", "path", *configPath)
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		if errors.Is(err, config.ErrConfigNotFound) { // TODO: избавиться от столькоих ошибок мб просто питаь оишбку и все, подумать
			logInstance.Fatal("Файл конфигурации не найден", "path", *configPath, "error", err)
		} else if errors.Is(err, config.ErrConfigParseFailed) {
			logInstance.Fatal("Ошибка парсинга файла конфигурации (проверьте YAML синтаксис)", "path", *configPath, "error", err)
		} else {
			logInstance.Fatal("Не удалось прочитать файл конфигурации", "path", *configPath, "error", err)
		}
	}
	logInstance.Info("Конфигурация успешно загружена", "max_parallel", cfg.Concurrency.MaxParallelWallets)

	txLogger, stateStorage, err := database.NewStorage(ctx, logInstance, cfg.Database.Type, cfg.Database.ConnectionString, cfg.Database.PoolMaxConns)
	if err != nil {
		if errors.Is(err, database.ErrUnsupportedDBType) || errors.Is(err, database.ErrMissingConnectionString) {
			logInstance.Fatal("Ошибка конфигурации хранилища данных", "db_type", cfg.Database.Type, "error", err)
		} else {
			logInstance.Fatal("Не удалось инициализировать хранилище данных", "db_type", cfg.Database.Type, "error", err)
		}
	}

	defer func() {
		logInstance.Debug("Closing transaction logger...")
		if err := txLogger.Close(); err != nil {
			logInstance.Error("Ошибка закрытия логгера транзакций", "error", err)
		}
	}()

	defer func() {
		logInstance.Debug("Closing state storage...")
		if stateStorage != nil {
			if err := stateStorage.Close(); err != nil {
				logInstance.Error("Ошибка закрытия хранилища состояния", "error", err)
			}
		}
	}()

	defer cancel()

	logInstance.Info("Загрузка кошельков...", "path", *walletsPath)
	wallets, err := wallet.LoadWallets(*walletsPath, logInstance)
	if err != nil {
		if errors.Is(err, wallet.ErrWalletsFileNotFound) {
			logInstance.Fatal("Файл кошельков не найден", "path", *walletsPath, "error", err)
		} else if errors.Is(err, wallet.ErrNoValidKeysFound) {
			logInstance.Fatal("В файле кошельков не найдено валидных ключей", "path", *walletsPath, "error", err)
		} else {
			logInstance.Fatal("Не удалось прочитать файл кошельков", "path", *walletsPath, "error", err)
		}
	}
	logInstance.Info("Кошельки успешно загружены", "count", len(wallets))

	bootstrap.RegisterTasksFromConfig(cfg, logInstance)

	appInstance := app.NewApplication(cfg, wallets, &wg, txLogger, stateStorage, logInstance)

	appInstance.Run(ctx)

	select {
	case <-ctx.Done():
		logInstance.Warn("Контекст был отменен.")
	default:
	}

	logInstance.Info("Retro Template ожидает завершения операций перед выходом...")
	wg.Wait()
	logInstance.Info("Retro Template завершил работу.")
}

// gracefulShutdown handles termination signals.
func gracefulShutdown(cancel context.CancelFunc, log logger.Logger) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-signalChan
	log.Warn("Получен сигнал завершения", "signal", sig.String())
	log.Warn("Инициируется плавная остановка... Отменяем контекст.")
	cancel()

	log.Info("Graceful shutdown: сигнал обработан, контекст отменен.")
}
