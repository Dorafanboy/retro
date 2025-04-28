package processor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"retro/internal/config"
	"retro/internal/evm"
	"retro/internal/executor"
	"retro/internal/keyloader"
	"retro/internal/logger"
	"retro/internal/selector"
	"retro/internal/storage"
	"retro/internal/tasks"
	"retro/internal/types"
	"retro/internal/utils"
)

// Processor encapsulates the logic for processing a single wallet.
type Processor struct {
	cfg              *config.Config
	signer           *evm.Signer
	walletIndex      int
	currentWalletNum int
	totalWalletsNum  int
	taskSelector     *selector.Selector
	taskExecutor     *executor.Executor
	txLogger         storage.TransactionLogger
	log              logger.Logger
}

// NewProcessor creates a new Processor instance.
func NewProcessor(
	cfg *config.Config,
	key *keyloader.LoadedKey,
	originalIndex int,
	currentNum int,
	totalNum int,
	txLogger storage.TransactionLogger,
	log logger.Logger,
) *Processor {
	taskSelector := selector.NewSelector(cfg, log)
	taskExecutor := executor.NewExecutor(cfg, log)
	signer := evm.NewSigner(key.PrivateKey)

	return &Processor{
		cfg:              cfg,
		signer:           signer,
		walletIndex:      originalIndex,
		currentWalletNum: currentNum,
		totalWalletsNum:  totalNum,
		taskSelector:     taskSelector,
		taskExecutor:     taskExecutor,
		txLogger:         txLogger,
		log:              log,
	}
}

// getEvmClientForTask creates an EVM client for the specified network if needed.
// It returns nil, nil if the network is "any".
func (p *Processor) getEvmClientForTask(ctx context.Context, network string) (*evm.Client, error) {
	if network == "any" {
		p.log.Debug("Пропуск создания EVM клиента для сети 'any'")
		return nil, nil
	}

	rpcUrls, ok := p.cfg.RPCNodes[network]
	if !ok || len(rpcUrls) == 0 {
		return nil, fmt.Errorf("не найдены RPC URL для сети %s", network)
	}

	p.log.Debug("Создание EVM клиента", "net", network)
	client, err := evm.NewClient(ctx, p.log, rpcUrls)
	if err != nil {
		// Возвращаем исходную ошибку, чтобы внешний код мог ее правильно обработать (включая ошибки контекста)
		return nil, fmt.Errorf("ошибка создания EVM клиента для сети %s: %w", network, err)
	}

	return client, nil
}

// prepareTask handles the setup required before executing a task.
// It creates the task runner and gets the EVM client.
// Returns the client, runner, and a potential error.
// If context is canceled during client creation, it returns the context error.
func (p *Processor) prepareTask(ctx context.Context, taskEntry config.TaskConfigEntry, walletProgress string) (*evm.Client, tasks.TaskRunner, error) {
	walletAddress := p.signer.Address()
	runner, err := tasks.NewTask(taskEntry.Name, p.log)
	if err != nil {
		if errors.Is(err, tasks.ErrTaskConstructorNotFound) {
			p.log.Error("Конструктор задачи не найден в реестре, пропуск", "task", taskEntry.Name,
				"wallet", walletProgress, "addr", walletAddress.Hex())
		} else {
			p.log.Error("Не удалось создать runner задачи, пропуск",
				"task", taskEntry.Name, "err", err,
				"wallet", walletProgress, "addr", walletAddress.Hex())
		}
		return nil, nil, err // Return the error to skip this task
	}

	client, err := p.getEvmClientForTask(ctx, taskEntry.Network)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			p.log.Warn("Создание EVM клиента прервано (контекст)",
				"task", taskEntry.Name, "err", err,
				"wallet", walletProgress, "addr", walletAddress.Hex())
			return nil, runner, err // Return context error to stop processing
		}
		// Other client errors (no RPC, connection error)
		p.log.Error("Не удалось получить EVM клиент, пропуск задачи",
			"task", taskEntry.Name, "net", taskEntry.Network, "err", err,
			"wallet", walletProgress, "addr", walletAddress.Hex())
		return nil, runner, err // Return error to skip this task
	}

	return client, runner, nil
}

// executeAndLogTask executes the task using the executor, closes the client, and logs the transaction.
// Returns the error from the task execution itself.
func (p *Processor) executeAndLogTask(ctx context.Context, taskEntry config.TaskConfigEntry, runner tasks.TaskRunner, client *evm.Client, walletProgress string) error {
	executionErr := p.taskExecutor.ExecuteTaskWithRetries(ctx, p.signer, client, taskEntry, runner)

	// Close client immediately after use
	if client != nil {
		client.Close()
		p.log.Debug("EVM клиент закрыт", "task", taskEntry.Name, "net", taskEntry.Network, "wallet", walletProgress)
	}

	// Log transaction result
	record := storage.TransactionRecord{
		Timestamp:     time.Now().Truncate(time.Second),
		WalletAddress: p.signer.Address().Hex(),
		TaskName:      taskEntry.Name,
		Network:       taskEntry.Network,
	}
	if executionErr != nil {
		record.Status = types.TxStatusFailed
		record.Error = executionErr.Error()
	} else {
		record.Status = types.TxStatusSuccess
	}

	logTxCtx, logTxCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if logDbErr := p.txLogger.LogTransaction(logTxCtx, record); logDbErr != nil {
		// Log the DB logging error, but don't return it as the primary task error
		p.log.Error("Не удалось записать лог транзакции в БД",
			"task", taskEntry.Name, "err", logDbErr,
			"wallet", walletProgress, "addr", p.signer.Address().Hex())
	}
	logTxCancel()

	return executionErr // Return the result of the task execution
}

// performInterTaskDelay handles the delay between tasks.
// Returns an error if the context is cancelled during the delay.
func (p *Processor) performInterTaskDelay(ctx context.Context, walletProgress string) error {
	walletAddress := p.signer.Address()
	select {
	case <-ctx.Done():
		p.log.Warn("Обработка прервана (контекст отменен перед задержкой между задачами)",
			"wallet", walletProgress, "addr", walletAddress.Hex())
		return ctx.Err()
	default:
	}

	actionDelayDuration, delayErr := utils.RandomDuration(p.cfg.Delay.BetweenActions)
	if delayErr != nil {
		p.log.Error("Ошибка получения времени задержки между задачами", "err", delayErr,
			"wallet", walletProgress, "addr", walletAddress.Hex())
		// Continue without delay if duration calculation fails
		return nil
	}

	if actionDelayDuration <= 0 {
		return nil // No delay configured or needed
	}

	p.log.Info("Пауза перед следующей задачей", "duration", actionDelayDuration,
		"wallet", walletProgress, "addr", walletAddress.Hex())
	select {
	case <-time.After(actionDelayDuration):
		return nil // Delay completed successfully
	case <-ctx.Done():
		p.log.Warn("Задержка между задачами прервана (контекст отменен)",
			"wallet", walletProgress, "addr", walletAddress.Hex())
		return ctx.Err() // Return context error
	}
}

// Process processes one wallet: selects tasks, executes them with retries and delays.
func (p *Processor) Process(ctx context.Context) error {
	walletAddress := p.signer.Address()
	walletProgress := fmt.Sprintf("%d/%d", p.currentWalletNum, p.totalWalletsNum)
	p.log.InfoWithBlankLine("-------------------- Начало обработки кошелька --------------------",
		"wallet", walletProgress, "origIdx", p.walletIndex, "addr", walletAddress.Hex())

	selectedTasks, err := p.taskSelector.SelectTasks()
	if err != nil {
		if errors.Is(err, selector.ErrNoValidTasksSelected) {
			p.log.Warn("Для кошелька не выбрано ни одной задачи, пропускаем",
				"wallet", walletProgress, "addr", walletAddress.Hex())
			// No tasks selected is not an error for the overall process
			return nil
		} else {
			p.log.Error("Ошибка выбора задач для кошелька, пропускаем",
				"err", err, "wallet", walletProgress, "addr", walletAddress.Hex())
		}
		// Return error if task selection itself failed
		return err
	}

	totalTasks := len(selectedTasks)
	p.log.Info("Задачи для выполнения",
		"count", totalTasks, "order", p.cfg.Actions.TaskOrder,
		"wallet", walletProgress, "addr", walletAddress.Hex())

	var finalError error // Keep track of the first error encountered during task execution

	for taskIndex, taskEntry := range selectedTasks {
		taskProgress := fmt.Sprintf("%d/%d", taskIndex+1, totalTasks)

		// 1. Check context before starting task processing
		select {
		case <-ctx.Done():
			p.log.Warn("Обработка прервана (контекст отменен перед задачей)",
				"task", taskEntry.Name, "taskNum", taskProgress,
				"wallet", walletProgress, "addr", walletAddress.Hex())
			return ctx.Err()
		default:
		}

		p.log.InfoWithBlankLine("------ Начало задачи ------", "taskNum", taskProgress,
			"task", taskEntry.Name, "net", taskEntry.Network,
			"wallet", walletProgress, "addr", walletAddress.Hex())

		// 2. Prepare task (get runner and client)
		client, runner, prepareErr := p.prepareTask(ctx, taskEntry, walletProgress)
		if prepareErr != nil {
			// If context was canceled during preparation, stop everything
			if errors.Is(prepareErr, context.Canceled) || errors.Is(prepareErr, context.DeadlineExceeded) {
				return prepareErr
			}
			// Other preparation errors (runner not found, non-context client error) mean skip this task
			if finalError == nil { // Store the first error encountered
				finalError = prepareErr
			}
			p.log.InfoWithBlankLine("------ Конец задачи (пропущена из-за ошибки подготовки) ------", "taskNum", taskProgress,
				"task", taskEntry.Name, "wallet", walletProgress, "addr", walletAddress.Hex())
			continue // Skip to the next task
		}

		// 3. Execute task and handle result
		executionErr := p.executeAndLogTask(ctx, taskEntry, runner, client, walletProgress)
		if executionErr != nil {
			p.log.Error("Ошибка выполнения задачи",
				"task", taskEntry.Name, "taskNum", taskProgress, "err", executionErr,
				"wallet", walletProgress, "addr", walletAddress.Hex())
			if finalError == nil { // Store the first error encountered
				finalError = executionErr
			}
			// Decide whether to continue based on config?
			// For now, continue to the next task even if one fails.
		} else {
			p.log.Success("Задача успешно выполнена", "task", taskEntry.Name, "taskNum", taskProgress,
				"wallet", walletProgress, "addr", walletAddress.Hex())
		}

		p.log.InfoWithBlankLine("------ Конец задачи ------", "taskNum", taskProgress,
			"task", taskEntry.Name, "wallet", walletProgress, "addr", walletAddress.Hex())

		// 4. Delay between tasks if not the last task
		if taskIndex < totalTasks-1 {
			if delayErr := p.performInterTaskDelay(ctx, walletProgress); delayErr != nil {
				// If context was cancelled during delay, return the error
				return delayErr
			}
		}
	}

	p.log.InfoWithBlankLine("-------------------- Конец обработки кошелька --------------------",
		"wallet", walletProgress, "addr", walletAddress.Hex())

	// Return the first error encountered during task execution, or nil if all succeeded
	return finalError
}
