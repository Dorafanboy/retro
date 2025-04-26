package app

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"retro_template/internal/config"
	"retro_template/internal/evm"
	"retro_template/internal/logger"
	"retro_template/internal/tasks"
	"retro_template/internal/types"
	"retro_template/internal/utils"
	"retro_template/internal/wallet"
)

var (
	// ErrNoValidTasksSelected indicates that no enabled and registered tasks were found.
	ErrNoValidTasksSelected = errors.New("no valid and active tasks selected for the wallet")
)

// Application holds the core application logic and dependencies.
type Application struct {
	cfg                 *config.Config
	wallets             []*wallet.Wallet
	registeredTaskNames []string
	wg                  *sync.WaitGroup
}

// NewApplication creates a new Application instance.
func NewApplication(cfg *config.Config, wallets []*wallet.Wallet, registeredTaskNames []string, wg *sync.WaitGroup) *Application {
	if cfg.Wallets.ProcessOrder == types.OrderRandom {
		logger.Info("Перемешивание порядка кошельков...")
		rand.Shuffle(len(wallets), func(i, j int) {
			wallets[i], wallets[j] = wallets[j], wallets[i]
		})
	}
	return &Application{
		cfg:                 cfg,
		wallets:             wallets,
		registeredTaskNames: registeredTaskNames,
		wg:                  wg,
	}
}

// Run starts the main application logic loop, processing wallets concurrently.
func (a *Application) Run(ctx context.Context) {
	logger.Info("Начало обработки кошельков", "count", len(a.wallets))

	for i, w := range a.wallets {
		a.wg.Add(1)
		go func(walletInstance *wallet.Wallet, walletIndex int) {
			defer a.wg.Done()

			select {
			case <-ctx.Done():
				logger.Warn("Обработка кошелька пропущена из-за отмены контекста", "index", walletIndex+1, "address", walletInstance.Address.Hex())
				return
			default:
			}

			logger.Info("----------------------------------------------------------------")
			logger.Info("Обработка кошелька", "index", walletIndex+1, "address", walletInstance.Address.Hex())

			selectedTasks, err := a.selectTasksForWallet()
			if err != nil {
				if errors.Is(err, ErrNoValidTasksSelected) {
					logger.Warn("Для кошелька не выбрано ни одной задачи, пропускаем", "wallet", walletInstance.Address.Hex())
				} else {
					logger.Error("Ошибка выбора задач для кошелька, пропускаем", "wallet", walletInstance.Address.Hex(), "error", err)
				}
				return
			}

			logger.Info("Задачи для выполнения", "wallet", walletInstance.Address.Hex(), "count", len(selectedTasks), "tasks", getTaskNames(selectedTasks))

			for taskIndex, taskEntry := range selectedTasks {
				select {
				case <-ctx.Done():
					logger.Warn("Выполнение задачи прервано из-за отмены контекста", "task_name", taskEntry.Name, "wallet", walletInstance.Address.Hex())
					return
				default:
				}

				logger.Info("------ Начало задачи ------", "task_index", taskIndex+1, "task_name", taskEntry.Name, "network", taskEntry.Network)

				runner, err := tasks.GetTask(taskEntry.Name)
				if err != nil {
					if errors.Is(err, tasks.ErrTaskNotFound) {
						logger.Error("Задача не найдена в реестре, пропуск задачи", "task_name", taskEntry.Name, "error", err)
					} else {
						logger.Error("Не удалось получить runner для задачи, пропуск задачи", "task_name", taskEntry.Name, "error", err)
					}
					continue
				}

				rpcUrls, ok := a.cfg.RPCNodes[taskEntry.Network]
				if !ok || len(rpcUrls) == 0 {
					logger.Error("Не найдены RPC URL для сети задачи, пропуск задачи", "task_name", taskEntry.Name, "network", taskEntry.Network)
					continue
				}

				logger.Debug("Создание EVM клиента", "network", taskEntry.Network)
				client, err := evm.NewClient(ctx, rpcUrls)
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						logger.Warn("Создание EVM клиента прервано из-за отмены контекста", "task_name", taskEntry.Name, "error", err)
						return
					} else if errors.Is(err, evm.ErrEvmClientCreationFailed) || errors.Is(err, evm.ErrNoRpcUrlsProvided) {
						logger.Error("Не удалось создать EVM клиент (нет доступных RPC или ошибка подключения), пропуск задачи", "task_name", taskEntry.Name, "network", taskEntry.Network, "error", err)
					} else {
						logger.Error("Непредвиденная ошибка при создании EVM клиента, пропуск задачи", "task_name", taskEntry.Name, "network", taskEntry.Network, "error", err)
					}
					continue
				}

				a.executeTaskWithRetries(ctx, walletInstance, client, taskEntry, runner)
				client.Close()
				logger.Info("------ Конец задачи ------", "task_name", taskEntry.Name)

				if taskIndex < len(selectedTasks)-1 {
					select {
					case <-ctx.Done():
						logger.Warn("Задержка между задачами пропущена из-за отмены контекста", "wallet", walletInstance.Address.Hex())
						return
					default:
					}
					actionDelayDuration, delayErr := utils.RandomDuration(a.cfg.Delay.BetweenActions)
					if delayErr != nil {
						logger.Error("Ошибка получения времени задержки между действиями", "error", delayErr)
					} else {
						logger.Info("Пауза перед следующей задачей", "duration", actionDelayDuration)
						time.Sleep(actionDelayDuration)
					}
				}
			}

			logger.Info("Обработка кошелька завершена", "index", walletIndex+1, "address", walletInstance.Address.Hex())

			if walletIndex < len(a.wallets)-1 {
				select {
				case <-ctx.Done():
					logger.Warn("Задержка между кошельками пропущена из-за отмены контекста")
					return
				default:
					delayDuration, err := utils.RandomDuration(a.cfg.Delay.BetweenAccounts)
					if err != nil {
						logger.Error("Ошибка получения времени задержки между кошельками", "error", err)
					} else {
						logger.Info("Пауза перед следующим кошельком", "duration", delayDuration)
						time.Sleep(delayDuration)
					}
				}
			}
			logger.Info("----------------------------------------------------------------")
		}(w, i)
	}

	logger.Info("Все горутины обработки кошельков запущены.")
}

// executeTaskWithRetries выполняет одну задачу с логикой ретраев.
func (a *Application) executeTaskWithRetries(ctx context.Context, w *wallet.Wallet, client *evm.Client, taskEntry config.TaskConfigEntry, runner tasks.TaskRunner) {
	var taskErr error
	success := false
	for attempt := 1; attempt <= a.cfg.Delay.BetweenRetries.Attempts; attempt++ {
		logger.Debug("Попытка выполнения задачи", "task_name", taskEntry.Name, "attempt", attempt)
		taskErr = runner.Run(ctx, w, client, taskEntry.Params)
		if taskErr == nil {
			success = true
			logger.Success("Задача успешно выполнена", "task_name", taskEntry.Name, "attempt", attempt)
			break
		}
		logger.Warn("Ошибка выполнения задачи, попытка повтора",
			"task_name", taskEntry.Name,
			"attempt", attempt,
			"max_attempts", a.cfg.Delay.BetweenRetries.Attempts,
			"error", taskErr)

		if attempt < a.cfg.Delay.BetweenRetries.Attempts {
			retryDelayDuration, delayErr := utils.RandomDuration(a.cfg.Delay.BetweenRetries.Delay)
			if delayErr != nil {
				logger.Error("Ошибка получения времени задержки между попытками", "error", delayErr)
			} else {
				logger.Info("Пауза перед следующей попыткой", "duration", retryDelayDuration)
				time.Sleep(retryDelayDuration)
			}
		}
	}

	if !success {
		logger.Error("Задача не выполнена после всех попыток", "task_name", taskEntry.Name, "error", taskErr)
		if a.cfg.Delay.AfterError.Min > 0 || a.cfg.Delay.AfterError.Max > 0 {
			afterErrorDelay, delayErr := utils.RandomDuration(a.cfg.Delay.AfterError)
			if delayErr != nil {
				logger.Error("Ошибка получения времени задержки после ошибки", "error", delayErr)
			} else {
				logger.Info("Пауза после ошибки задачи", "duration", afterErrorDelay)
				time.Sleep(afterErrorDelay)
			}
		}
	}
}

// selectTasksForWallet выбирает задачи для выполнения одним кошельком на основе конфигурации.
func (a *Application) selectTasksForWallet() ([]config.TaskConfigEntry, error) {
	if len(a.cfg.Actions.ExplicitTaskSequence) > 0 {
		logger.Debug("Используется режим явной последовательности задач")
		var selected []config.TaskConfigEntry
		taskMap := make(map[string]config.TaskConfigEntry)
		for _, taskCfg := range a.cfg.Tasks {
			if taskCfg.Enabled {
				taskMap[taskCfg.Name] = taskCfg
			}
		}

		for _, taskName := range a.cfg.Actions.ExplicitTaskSequence {
			taskCfg, exists := taskMap[taskName]
			if !exists {
				logger.Warn("Задача из явной последовательности не найдена или отключена в конфиге", "task_name", taskName)
				continue
			}
			if !isTaskRegistered(taskName, a.registeredTaskNames) {
				logger.Warn("Задача из явной последовательности не зарегистрирована в коде", "task_name", taskName)
				continue
			}
			selected = append(selected, taskCfg)
		}
		if len(selected) == 0 {
			return nil, fmt.Errorf("в явной последовательности %w", ErrNoValidTasksSelected)
		}
		logger.Info("Выбраны задачи из явной последовательности", "count", len(selected))
		return selected, nil
	}

	logger.Debug("Используется режим случайного выбора задач")

	availableTasks := make([]config.TaskConfigEntry, 0)
	for _, taskCfg := range a.cfg.Tasks {
		if taskCfg.Enabled && isTaskRegistered(taskCfg.Name, a.registeredTaskNames) {
			availableTasks = append(availableTasks, taskCfg)
		}
	}

	if len(availableTasks) == 0 {
		return nil, fmt.Errorf("в конфигурации %w", ErrNoValidTasksSelected)
	}

	minTasks := a.cfg.Actions.ActionsPerAccount.Min
	maxTasks := a.cfg.Actions.ActionsPerAccount.Max
	if maxTasks <= 0 || maxTasks > len(availableTasks) {
		maxTasks = len(availableTasks)
	}
	if minTasks <= 0 {
		minTasks = 1
	}
	if minTasks > maxTasks {
		minTasks = maxTasks
	}
	numTasksToSelect := utils.RandomIntInRange(minTasks, maxTasks)
	logger.Info("Будет выбрано задач", "count", numTasksToSelect, "min", minTasks, "max", maxTasks)

	rand.Shuffle(len(availableTasks), func(i, j int) {
		availableTasks[i], availableTasks[j] = availableTasks[j], availableTasks[i]
	})

	selected := availableTasks[:numTasksToSelect]

	if a.cfg.Actions.TaskOrder == config.TaskOrderSequential {
		logger.Debug("Сортировка выбранных задач по порядку из конфига")
		originalIndex := make(map[string]int)
		for idx, taskCfg := range a.cfg.Tasks {
			originalIndex[taskCfg.Name] = idx
		}
		sort.SliceStable(selected, func(i, j int) bool {
			return originalIndex[selected[i].Name] < originalIndex[selected[j].Name]
		})
	} else {
		logger.Debug("Порядок выполнения задач - случайный")
	}

	return selected, nil
}

// isTaskRegistered проверяет, есть ли имя задачи в списке зарегистрированных.
func isTaskRegistered(name string, registeredNames []string) bool {
	for _, registeredName := range registeredNames {
		if name == registeredName {
			return true
		}
	}
	return false
}

// getTaskNames - хелпер для логирования имен задач.
func getTaskNames(tasks []config.TaskConfigEntry) []string {
	names := make([]string, len(tasks))
	for i, task := range tasks {
		names[i] = task.Name
	}
	return names
}
