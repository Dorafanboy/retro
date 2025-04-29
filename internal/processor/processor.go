package processor

import (
	"context"
	"errors"
	"fmt"

	"retro/internal/config"
	"retro/internal/evm"
	"retro/internal/executor"
	"retro/internal/keyloader"
	"retro/internal/logger"
	"retro/internal/selector"
	"retro/internal/storage"
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

// Process processes one wallet: selects tasks and delegates the execution loop.
func (p *Processor) Process(ctx context.Context) error {
	walletAddress := p.signer.Address()
	walletProgress := fmt.Sprintf("%d/%d", p.currentWalletNum, p.totalWalletsNum)
	p.log.InfoWithBlankLine("-------------------- Начало обработки кошелька --------------------",
		"wallet", walletProgress, "origIdx", p.walletIndex, "addr", walletAddress.Hex())

	selectedTasks, err := p.taskSelector.SelectTasks()
	if err != nil {
		if errors.Is(err, selector.ErrNoValidTasksSelected) {
			p.log.Warn("Для кошелька не выбрано ни одной валидной задачи, пропускаем.",
				"wallet", walletProgress, "addr", walletAddress.Hex())
			return nil
		} else {
			p.log.Error("Ошибка выбора задач для кошелька, обработка прервана.",
				"err", err, "wallet", walletProgress, "addr", walletAddress.Hex())
			return fmt.Errorf("ошибка выбора задач: %w", err)
		}
	}

	totalTasks := len(selectedTasks)
	if totalTasks == 0 {
		p.log.Warn("Список выбранных задач пуст после фильтрации селектором, пропускаем кошелек.",
			"wallet", walletProgress, "addr", walletAddress.Hex())
		return nil // Не является ошибкой
	}
	p.log.Info("Задачи для выполнения",
		"count", totalTasks, "order", p.cfg.Actions.TaskOrder,
		"wallet", walletProgress, "addr", walletAddress.Hex())

	loopErr := p.processTaskLoop(ctx, selectedTasks, walletProgress)

	if loopErr != nil {
		if errors.Is(loopErr, context.Canceled) || errors.Is(loopErr, context.DeadlineExceeded) {
			p.log.Warn("Обработка кошелька прервана (контекст отменен во время выполнения задач).",
				"wallet", walletProgress, "addr", walletAddress.Hex(), "error", loopErr)
		} else {
			p.log.Error("Обработка кошелька завершилась с ошибкой.",
				"wallet", walletProgress, "addr", walletAddress.Hex(), "error", loopErr)
		}
	} else {
		p.log.Info("Обработка кошелька успешно завершена.",
			"wallet", walletProgress, "addr", walletAddress.Hex())
	}

	p.log.InfoWithBlankLine("-------------------- Конец обработки кошелька --------------------",
		"wallet", walletProgress, "addr", walletAddress.Hex())

	return loopErr
}

// processTaskLoop iterates through the selected tasks and processes each one.
func (p *Processor) processTaskLoop(ctx context.Context, selectedTasks []config.TaskConfigEntry, walletProgress string) error {
	walletAddress := p.signer.Address()
	totalTasks := len(selectedTasks)
	var firstError error

	for taskIndex, taskEntry := range selectedTasks {
		taskProgress := fmt.Sprintf("%d/%d", taskIndex+1, totalTasks)

		select {
		case <-ctx.Done():
			p.log.Warn("Обработка прервана (контекст отменен перед задачей в цикле)",
				"task", taskEntry.Name, "taskNum", taskProgress,
				"wallet", walletProgress, "addr", walletAddress.Hex())
			return ctx.Err()
		default:
		}

		p.log.InfoWithBlankLine("------ Начало задачи ------", "taskNum", taskProgress,
			"task", taskEntry.Name, "net", taskEntry.Network,
			"wallet", walletProgress, "addr", walletAddress.Hex())

		client, runner, prepareErr := p.prepareTask(ctx, taskEntry, walletProgress)
		if prepareErr != nil {
			if errors.Is(prepareErr, context.Canceled) || errors.Is(prepareErr, context.DeadlineExceeded) {
				return prepareErr
			}
			if firstError == nil {
				firstError = prepareErr
			}
			p.log.Warn("Задача пропущена из-за ошибки подготовки.", "taskNum", taskProgress,
				"task", taskEntry.Name, "err", prepareErr, "wallet", walletProgress, "addr", walletAddress.Hex())
			p.log.InfoWithBlankLine("------ Конец задачи (пропущена) ------", "taskNum", taskProgress,
				"task", taskEntry.Name, "wallet", walletProgress, "addr", walletAddress.Hex())
			continue
		}

		executionErr := p.executeAndLogTask(ctx, taskEntry, runner, client, walletProgress)
		if executionErr != nil {
			p.log.Error("Ошибка выполнения задачи",
				"task", taskEntry.Name, "taskNum", taskProgress, "err", executionErr,
				"wallet", walletProgress, "addr", walletAddress.Hex())
			if errors.Is(executionErr, context.Canceled) || errors.Is(executionErr, context.DeadlineExceeded) {
				return executionErr
			}
			if firstError == nil {
				firstError = executionErr
			}
		} else {
			p.log.Success("Задача успешно выполнена", "task", taskEntry.Name, "taskNum", taskProgress,
				"wallet", walletProgress, "addr", walletAddress.Hex())
		}

		p.log.InfoWithBlankLine("------ Конец задачи ------", "taskNum", taskProgress,
			"task", taskEntry.Name, "wallet", walletProgress, "addr", walletAddress.Hex())

		if taskIndex < totalTasks-1 {
			if delayErr := p.performInterTaskDelay(ctx, walletProgress); delayErr != nil {
				return delayErr
			}
		}
	}

	return firstError
}
