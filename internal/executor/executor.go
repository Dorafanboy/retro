package executor

import (
	"context"
	"time"

	"retro/internal/config"
	"retro/internal/evm"
	"retro/internal/logger"
	"retro/internal/tasks"
	"retro/internal/utils"
	"retro/internal/wallet"
)

// Executor is responsible for executing a single task with retries logic.
// Переименовано в Executor и сделано публичным
type Executor struct {
	cfg *config.Config
}

// NewExecutor creates a new Executor instance.
// Переименовано в NewExecutor и сделано публичным
func NewExecutor(cfg *config.Config) *Executor {
	return &Executor{
		cfg: cfg,
	}
}

// ExecuteTaskWithRetries executes a single task with retries logic.
// Метод сделан публичным
func (e *Executor) ExecuteTaskWithRetries(
	ctx context.Context,
	wallet *wallet.Wallet,
	client evm.EVMClient,
	taskEntry config.TaskConfigEntry,
	runner tasks.TaskRunner,
) error {
	var taskErr error
	success := false
	// Используем e.cfg вместо te.cfg
	maxAttempts := e.cfg.Delay.BetweenRetries.Attempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		logger.Debug(
			"Попытка выполнения задачи",
			"task", taskEntry.Name,
			"attempt", attempt,
			"wallet", wallet.Address.Hex())
		taskErr = runner.Run(ctx, wallet, client, taskEntry.Params)
		if taskErr == nil {
			success = true
			logger.SuccessWithBlankLine("Задача успешно выполнена",
				"task", taskEntry.Name,
				"attempt", attempt,
				"wallet", wallet.Address.Hex())
			break
		}
		logger.Warn("Ошибка выполнения задачи, попытка повтора",
			"task", taskEntry.Name,
			"attempt", attempt,
			"maxAttempts", maxAttempts,
			"err", taskErr,
			"wallet", wallet.Address.Hex())

		if attempt < maxAttempts {
			// Используем e.cfg вместо te.cfg
			retryDelayDuration, delayErr := utils.RandomDuration(e.cfg.Delay.BetweenRetries.Delay)
			if delayErr != nil {
				logger.Error("Ошибка получения времени задержки между попытками",
					"err", delayErr,
					"wallet", wallet.Address.Hex())
			} else {
				logger.Info("Пауза перед следующей попыткой",
					"duration", retryDelayDuration,
					"wallet", wallet.Address.Hex())
				select {
				case <-time.After(retryDelayDuration):
					// Пауза завершена, продолжаем ретрай
				case <-ctx.Done():
					logger.Warn("Задержка между попытками прервана (контекст отменен)",
						"task", taskEntry.Name,
						"wallet", wallet.Address.Hex())
					return taskErr // Возвращаем последнюю ошибку
				}
			}
		}
	}

	if !success {
		logger.ErrorWithBlankLine("Задача не выполнена после всех попыток",
			"task", taskEntry.Name,
			"err", taskErr,
			"wallet", wallet.Address.Hex())
		// Используем e.cfg вместо te.cfg
		if e.cfg.Delay.AfterError.Min > 0 || e.cfg.Delay.AfterError.Max > 0 {
			afterErrorDelay, delayErr := utils.RandomDuration(e.cfg.Delay.AfterError)
			if delayErr != nil {
				logger.Error("Ошибка получения времени задержки после ошибки",
					"err", delayErr,
					"wallet", wallet.Address.Hex())
			} else {
				logger.Info("Пауза после ошибки задачи",
					"duration", afterErrorDelay,
					"wallet", wallet.Address.Hex())
				// !! ВАЖНО: Здесь остался time.Sleep. Заменяем на select
				select {
				case <-time.After(afterErrorDelay):
					// Пауза завершена
				case <-ctx.Done():
					logger.Warn("Задержка после ошибки прервана (контекст отменен)",
						"task", taskEntry.Name,
						"wallet", wallet.Address.Hex())
					// Возвращаем ошибку, так как задача уже не удалась
					return taskErr
				}
			}
		}
	}

	return taskErr
}
