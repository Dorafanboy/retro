package app

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

// TaskExecutor is responsible for executing a single task with retrace logic.
type TaskExecutor struct {
	cfg *config.Config
}

// newTaskExecutor creates a new TaskExecutor instance.
func newTaskExecutor(cfg *config.Config) *TaskExecutor {
	return &TaskExecutor{
		cfg: cfg,
	}
}

// ExecuteTaskWithRetries executes a single task with retries logic.
func (te *TaskExecutor) ExecuteTaskWithRetries(
	ctx context.Context,
	wallet *wallet.Wallet,
	client evm.EVMClient,
	taskEntry config.TaskConfigEntry,
	runner tasks.TaskRunner,
) error {
	var taskErr error
	success := false
	maxAttempts := te.cfg.Delay.BetweenRetries.Attempts
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
			retryDelayDuration, delayErr := utils.RandomDuration(te.cfg.Delay.BetweenRetries.Delay)
			if delayErr != nil {
				logger.Error("Ошибка получения времени задержки между попытками",
					"err", delayErr,
					"wallet", wallet.Address.Hex())
			} else {
				logger.Info("Пауза перед следующей попыткой",
					"duration", retryDelayDuration,
					"wallet", wallet.Address.Hex())
				time.Sleep(retryDelayDuration)
			}
		}
	}

	if !success {
		logger.ErrorWithBlankLine("Задача не выполнена после всех попыток",
			"task", taskEntry.Name,
			"err", taskErr,
			"wallet", wallet.Address.Hex())
		if te.cfg.Delay.AfterError.Min > 0 || te.cfg.Delay.AfterError.Max > 0 {
			afterErrorDelay, delayErr := utils.RandomDuration(te.cfg.Delay.AfterError)
			if delayErr != nil {
				logger.Error("Ошибка получения времени задержки после ошибки",
					"err", delayErr,
					"wallet", wallet.Address.Hex())
			} else {
				logger.Info("Пауза после ошибки задачи",
					"duration", afterErrorDelay,
					"wallet", wallet.Address.Hex())
				time.Sleep(afterErrorDelay)
			}
		}
	}

	return taskErr
}
