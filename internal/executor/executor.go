package executor

import (
	"context"
	"time"

	"retro/internal/config"
	"retro/internal/evm"
	"retro/internal/logger"
	"retro/internal/tasks"
	"retro/internal/utils"
)

// Executor is responsible for executing a single task with retries logic.
type Executor struct {
	cfg *config.Config
	log logger.Logger
}

// NewExecutor creates a new Executor instance.
func NewExecutor(cfg *config.Config, log logger.Logger) *Executor {
	return &Executor{
		cfg: cfg,
		log: log,
	}
}

// ExecuteTaskWithRetries executes a single task with retries logic.
func (e *Executor) ExecuteTaskWithRetries(
	ctx context.Context,
	signer *evm.Signer,
	client evm.EVMClient,
	taskEntry config.TaskConfigEntry,
	runner tasks.TaskRunner,
) error {
	var taskErr error
	success := false
	maxAttempts := e.cfg.Delay.BetweenRetries.Attempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	walletAddress := signer.Address()

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		e.log.Debug(
			"Попытка выполнения задачи", "task", taskEntry.Name,
			"attempt", attempt, "wallet", walletAddress.Hex())
		taskErr = runner.Run(ctx, signer, client, taskEntry.Params)
		if taskErr == nil {
			success = true
			e.log.SuccessWithBlankLine("Задача успешно выполнена", "task", taskEntry.Name,
				"attempt", attempt, "wallet", walletAddress.Hex())
			break
		}
		e.log.Warn("Ошибка выполнения задачи, попытка повтора", "task", taskEntry.Name,
			"attempt", attempt, "maxAttempts", maxAttempts,
			"err", taskErr, "wallet", walletAddress.Hex())

		if attempt < maxAttempts {
			retryDelayDuration, delayErr := utils.RandomDuration(e.cfg.Delay.BetweenRetries.Delay)
			if delayErr != nil {
				e.log.Error("Ошибка получения времени задержки между попытками",
					"err", delayErr, "wallet", walletAddress.Hex())
			} else {
				e.log.Info("Пауза перед следующей попыткой",
					"duration", retryDelayDuration, "wallet", walletAddress.Hex())
				select {
				case <-time.After(retryDelayDuration):
				case <-ctx.Done():
					e.log.Warn("Задержка между попытками прервана (контекст отменен)",
						"task", taskEntry.Name, "wallet", walletAddress.Hex())
					return taskErr
				}
			}
		}
	}

	if !success {
		e.log.ErrorWithBlankLine("Задача не выполнена после всех попыток", "task", taskEntry.Name,
			"err", taskErr, "wallet", walletAddress.Hex())
		if e.cfg.Delay.AfterError.Min > 0 || e.cfg.Delay.AfterError.Max > 0 {
			afterErrorDelay, delayErr := utils.RandomDuration(e.cfg.Delay.AfterError)
			if delayErr != nil {
				e.log.Error("Ошибка получения времени задержки после ошибки",
					"err", delayErr, "wallet", walletAddress.Hex())
			} else {
				e.log.Info("Пауза после ошибки задачи",
					"duration", afterErrorDelay, "wallet", walletAddress.Hex())
				select {
				case <-time.After(afterErrorDelay):
				case <-ctx.Done():
					e.log.Warn("Задержка после ошибки прервана (контекст отменен)",
						"task", taskEntry.Name, "wallet", walletAddress.Hex())
					return taskErr
				}
			}
		}
	}

	return taskErr
}
