package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"retro_template/internal/config"
	"retro_template/internal/evm"
	"retro_template/internal/logger"
	"retro_template/internal/storage"
	"retro_template/internal/storage/postgres"
	"retro_template/internal/storage/sqlite"
	"retro_template/internal/tasks"
	"retro_template/internal/utils"
	"retro_template/internal/wallet"

	"github.com/joho/godotenv"

	_ "retro_template/internal/tasks"

	_ "github.com/mattn/go-sqlite3"
)

var (
	configPath  = flag.String("config", "config/config.yml", "Path to the configuration file")
	walletsPath = flag.String("wallets", "local/private_keys.txt", "Path to the private keys file")
)

func main() {
	// Load .env file. Ignore error if it doesn't exist.
	_ = godotenv.Load()

	defer func() {
		if r := recover(); r != nil {
			logger.Fatal("Критическая ошибка (panic)", "error", r)
		}
	}()

	flag.Parse()

	logger.InfoWithBlankLine("Запуск Retro Template...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go gracefulShutdown(cancel)

	// Initialize Transaction Logger based on environment variables
	txLogger, err := initTransactionLogger(ctx)
	if err != nil {
		logger.Fatal("Failed to initialize transaction logger", "error", err)
	}
	defer func() {
		if err := txLogger.Close(); err != nil {
			logger.Error("Error closing transaction logger", "error", err)
		}
	}()

	logger.Info("Загрузка конфигурации...", "path", *configPath)
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.Fatal("Не удалось загрузить конфигурацию", "error", err)
	}
	logger.InfoWithBlankLine("Конфигурация успешно загружена", "max_parallel", cfg.Concurrency.MaxParallelWallets)

	logger.Info("Загрузка кошельков...", "path", *walletsPath)
	wallets, err := wallet.LoadWallets(*walletsPath)
	if err != nil {
		logger.Fatal("Не удалось загрузить кошельки", "error", err)
	}
	logger.SuccessWithBlankLine("Кошельки успешно загружены", "count", len(wallets))

	registeredTasks := tasks.ListTasks()
	logger.Info("Зарегистрированные задачи", "tasks", registeredTasks)
	logger.Debug("Реестр задач инициализирован")

	// Перемешиваем кошельки, если указан случайный порядок
	if cfg.Wallets.ProcessOrder == "random" {
		logger.Info("Перемешивание порядка кошельков...")
		rand.Shuffle(len(wallets), func(i, j int) {
			wallets[i], wallets[j] = wallets[j], wallets[i]
		})
	}

	logger.Info("Начало обработки кошельков", "count", len(wallets))
	// Основной цикл по кошелькам
	for i, w := range wallets {
		logger.Info("----------------------------------------------------------------")
		logger.Info("Обработка кошелька", "index", i+1, "address", w.Address.Hex())

		// 4. Выбираем задачи для текущего кошелька
		selectedTasks, err := selectTasksForWallet(cfg, cfg.Tasks, registeredTasks)
		if err != nil {
			logger.Error("Ошибка выбора задач для кошелька, пропускаем", "wallet", w.Address.Hex(), "error", err)
			continue // Переходим к следующему кошельку
		}
		if len(selectedTasks) == 0 {
			logger.Warn("Для кошелька не выбрано ни одной задачи, пропускаем", "wallet", w.Address.Hex())
			continue
		}

		logger.Info("Задачи для выполнения", "wallet", w.Address.Hex(), "count", len(selectedTasks), "tasks", getTaskNames(selectedTasks))

		// 5. Цикл по выбранным задачам
		for taskIndex, taskEntry := range selectedTasks {
			logger.Info("------ Начало задачи ------", "task_index", taskIndex+1, "task_name", taskEntry.Name, "network", taskEntry.Network)

			// 6. Получаем TaskRunner из реестра
			runner, err := tasks.GetTask(taskEntry.Name)
			if err != nil {
				logger.Error("Не удалось получить runner для задачи, пропуск", "task_name", taskEntry.Name, "error", err)
				continue // Пропускаем эту задачу
			}

			// 7. Получаем RPC URL для нужной сети
			rpcUrls, ok := cfg.RPCNodes[taskEntry.Network]
			if !ok || len(rpcUrls) == 0 {
				logger.Error("Не найдены RPC URL для сети задачи, пропуск", "task_name", taskEntry.Name, "network", taskEntry.Network)
				continue
			}

			// 8. Создаем EVM клиент
			logger.Debug("Создание EVM клиента", "network", taskEntry.Network)
			client, err := evm.NewClient(ctx, rpcUrls) // Передаем главный контекст
			if err != nil {
				logger.Error("Не удалось создать EVM клиент, пропуск задачи", "task_name", taskEntry.Name, "network", taskEntry.Network, "error", err)
				continue
			}
			// Закрываем клиент после выполнения задачи (или ретраев)
			defer client.Close()

			// 9. Цикл ретраев для выполнения задачи
			var taskErr error
			success := false
			for attempt := 1; attempt <= cfg.Delay.BetweenRetries.Attempts; attempt++ {
				logger.Debug("Попытка выполнения задачи", "task_name", taskEntry.Name, "attempt", attempt)
				// 10. Вызываем Run, передавая главный контекст
				taskErr = runner.Run(ctx, w, client, taskEntry.Params)
				if taskErr == nil {
					success = true
					logger.Success("Задача успешно выполнена", "task_name", taskEntry.Name, "attempt", attempt)
					break // Выходим из цикла ретраев при успехе
				}
				logger.Warn("Ошибка выполнения задачи, попытка повтора",
					"task_name", taskEntry.Name,
					"attempt", attempt,
					"max_attempts", cfg.Delay.BetweenRetries.Attempts,
					"error", taskErr)

				// 11. Задержка перед следующей попыткой (если это не последняя)
				if attempt < cfg.Delay.BetweenRetries.Attempts {
					retryDelayDuration, delayErr := utils.RandomDuration(cfg.Delay.BetweenRetries.Delay)
					if delayErr != nil {
						logger.Error("Ошибка получения времени задержки между попытками", "error", delayErr)
					} else {
						logger.Info("Пауза перед следующей попыткой", "duration", retryDelayDuration)
						time.Sleep(retryDelayDuration)
					}
				}
			}

			// 12. Логируем окончательный результат задачи
			if !success {
				logger.Error("Задача не выполнена после всех попыток", "task_name", taskEntry.Name, "error", taskErr)
				// TODO: Возможно, добавить логику обработки постоянной ошибки (например, запись в файл)
				// Применяем задержку после ошибки (если она настроена)
				if cfg.Delay.AfterError.Min > 0 || cfg.Delay.AfterError.Max > 0 {
					afterErrorDelay, delayErr := utils.RandomDuration(cfg.Delay.AfterError)
					if delayErr != nil {
						logger.Error("Ошибка получения времени задержки после ошибки", "error", delayErr)
					} else {
						logger.Info("Пауза после ошибки задачи", "duration", afterErrorDelay)
						time.Sleep(afterErrorDelay)
					}
				}
			}

			logger.Info("------ Конец задачи ------", "task_name", taskEntry.Name)

			// 13. Применяем задержку между действиями (если это не последняя задача)
			if taskIndex < len(selectedTasks)-1 {
				actionDelayDuration, delayErr := utils.RandomDuration(cfg.Delay.BetweenActions)
				if delayErr != nil {
					logger.Error("Ошибка получения времени задержки между действиями", "error", delayErr)
				} else {
					logger.Info("Пауза перед следующей задачей", "duration", actionDelayDuration)
					time.Sleep(actionDelayDuration)
				}
			}
		} // Конец цикла по задачам

		// 14. Применяем задержку между кошельками (если это не последний)
		if i < len(wallets)-1 {
			delayDuration, err := utils.RandomDuration(cfg.Delay.BetweenAccounts)
			if err != nil {
				logger.Error("Ошибка получения времени задержки между кошельками", "error", err)
			} else {
				logger.Info("Пауза перед следующим кошельком", "duration", delayDuration)
				time.Sleep(delayDuration)
			}
		}
		logger.Info("----------------------------------------------------------------")
	}

	logger.Debug("Распределение задач по воркерам (TODO)")

	logger.Debug("Ожидание завершения всех задач (TODO)")

	logger.Debug("Вывод итоговой статистики (TODO)")

	select {
	case <-ctx.Done():
		logger.Warn("Контекст был отменен до завершения основной логики.")
	default:
		// Просто продолжаем
	}

	logger.SuccessWithBlankLine("Retro Template завершил работу.")
}

// selectTasksForWallet выбирает задачи для выполнения одним кошельком на основе конфигурации.
func selectTasksForWallet(cfg *config.Config, allTaskConfigs []config.TaskConfigEntry, registeredTaskNames []string) ([]config.TaskConfigEntry, error) {

	// Проверяем режим явной последовательности (Режим 3)
	if len(cfg.Actions.ExplicitTaskSequence) > 0 {
		logger.Debug("Используется режим явной последовательности задач")
		var selected []config.TaskConfigEntry
		taskMap := make(map[string]config.TaskConfigEntry)
		for _, taskCfg := range allTaskConfigs {
			if taskCfg.Enabled {
				taskMap[taskCfg.Name] = taskCfg
			}
		}

		for _, taskName := range cfg.Actions.ExplicitTaskSequence {
			taskCfg, exists := taskMap[taskName]
			if !exists {
				logger.Warn("Задача из явной последовательности не найдена или отключена в конфиге", "task_name", taskName)
				continue
			}
			// Проверяем, зарегистрирован ли такой runner
			if !isTaskRegistered(taskName, registeredTaskNames) {
				logger.Warn("Задача из явной последовательности не зарегистрирована в коде", "task_name", taskName)
				continue
			}
			selected = append(selected, taskCfg)
		}
		if len(selected) == 0 {
			return nil, fmt.Errorf("в явной последовательности не найдено ни одной валидной и активной задачи")
		}
		logger.Info("Выбраны задачи из явной последовательности", "count", len(selected))
		return selected, nil
	}

	// Режимы 1 и 2: Случайный выбор
	logger.Debug("Используется режим случайного выбора задач")

	// Фильтруем активные и зарегистрированные задачи
	availableTasks := make([]config.TaskConfigEntry, 0)
	for _, taskCfg := range allTaskConfigs {
		if taskCfg.Enabled && isTaskRegistered(taskCfg.Name, registeredTaskNames) {
			availableTasks = append(availableTasks, taskCfg)
		}
	}

	if len(availableTasks) == 0 {
		return nil, fmt.Errorf("не найдено ни одной активной и зарегистрированной задачи в конфигурации")
	}

	// Выбираем количество задач
	minTasks := cfg.Actions.ActionsPerAccount.Min
	maxTasks := cfg.Actions.ActionsPerAccount.Max
	if maxTasks <= 0 || maxTasks > len(availableTasks) {
		maxTasks = len(availableTasks) // Не больше, чем доступно
	}
	if minTasks <= 0 {
		minTasks = 1 // Хотя бы одну, если есть доступные
	}
	if minTasks > maxTasks {
		minTasks = maxTasks // Минимум не может быть больше максимума
	}
	numTasksToSelect := utils.RandomIntInRange(minTasks, maxTasks)
	logger.Info("Будет выбрано задач", "count", numTasksToSelect, "min", minTasks, "max", maxTasks)

	// Перемешиваем доступные задачи
	rand.Shuffle(len(availableTasks), func(i, j int) {
		availableTasks[i], availableTasks[j] = availableTasks[j], availableTasks[i]
	})

	// Берем первые N задач
	selected := availableTasks[:numTasksToSelect]

	// Сортируем, если нужен последовательный порядок (Режим 2)
	if cfg.Actions.TaskOrder == "sequential" {
		logger.Debug("Сортировка выбранных задач по порядку из конфига")
		// Создаем мапу для быстрой сортировки по исходному индексу
		originalIndex := make(map[string]int)
		for idx, taskCfg := range allTaskConfigs {
			originalIndex[taskCfg.Name] = idx
		}
		sort.SliceStable(selected, func(i, j int) bool {
			return originalIndex[selected[i].Name] < originalIndex[selected[j].Name]
		})
	} else {
		logger.Debug("Порядок выполнения задач - случайный")
		// Уже перемешано, ничего делать не нужно
	}

	return selected, nil
}

// isTaskRegistered проверяет, есть ли имя задачи в списке зарегистрированных
func isTaskRegistered(name string, registeredNames []string) bool {
	for _, registeredName := range registeredNames {
		if name == registeredName {
			return true
		}
	}
	return false
}

func gracefulShutdown(cancel context.CancelFunc) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-signalChan
	logger.Warn("Получен сигнал завершения", "signal", sig.String())
	logger.Warn("Инициируется плавная остановка...")
	cancel()

	time.Sleep(2 * time.Second)
	logger.Info("Приложение остановлено.")
	os.Exit(0)
}

// getTaskNames - хелпер для логирования имен задач
func getTaskNames(tasks []config.TaskConfigEntry) []string {
	names := make([]string, len(tasks))
	for i, task := range tasks {
		names[i] = task.Name
	}
	return names
}

// initTransactionLogger initializes the transaction logger based on ENV variables.
func initTransactionLogger(ctx context.Context) (storage.TransactionLogger, error) {
	dbType := os.Getenv("DB_TYPE")
	connStr := os.Getenv("DB_CONNECTION_STRING")
	maxConnsStr := os.Getenv("DB_POOL_MAX_CONNS")

	var txLogger storage.TransactionLogger
	var err error

	switch dbType {
	case "postgres":
		if connStr == "" {
			return nil, errors.New("DB_CONNECTION_STRING must be set when DB_TYPE is postgres")
		}
		logger.Info("Инициализация логгера транзакций PostgreSQL...")
		txLogger, err = postgres.NewStore(ctx, connStr, maxConnsStr)
	case "sqlite":
		if connStr == "" {
			return nil, errors.New("DB_CONNECTION_STRING (file path) must be set when DB_TYPE is sqlite")
		}
		logger.Info("Инициализация логгера транзакций SQLite...")
		txLogger, err = sqlite.NewStore(ctx, connStr)
	case "none", "":
		logger.Info("Логгирование транзакций в БД отключено (DB_TYPE='none' или не задан).")
		txLogger = storage.NewNoOpStorage()
	default:
		err = fmt.Errorf("неподдерживаемый тип БД: %s (укажите 'postgres', 'sqlite' или 'none' в DB_TYPE)", dbType)
	}

	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации логгера транзакций: %w", err)
	}

	return txLogger, nil
}
