package processor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"retro/internal/config"
	"retro/internal/evm"
	"retro/internal/executor"
	"retro/internal/logger"
	"retro/internal/selector"
	"retro/internal/storage"
	"retro/internal/tasks"
	"retro/internal/types"
	"retro/internal/utils"
	"retro/internal/wallet"
)

// Processor encapsulates the logic for processing a single wallet.
type Processor struct {
	cfg              *config.Config
	wallet           *wallet.Wallet
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
	w *wallet.Wallet,
	originalIndex int,
	currentNum int,
	totalNum int,
	txLogger storage.TransactionLogger,
	log logger.Logger,
) *Processor {
	taskSelector := selector.NewSelector(cfg, log)
	taskExecutor := executor.NewExecutor(cfg, log)

	return &Processor{
		cfg:              cfg,
		wallet:           w,
		walletIndex:      originalIndex,
		currentWalletNum: currentNum,
		totalWalletsNum:  totalNum,
		taskSelector:     taskSelector,
		taskExecutor:     taskExecutor,
		txLogger:         txLogger,
		log:              log,
	}
}

// Process processes one wallet: selects tasks, executes them with retries and delays.
func (p *Processor) Process(ctx context.Context) error {
	walletProgress := fmt.Sprintf("%d/%d", p.currentWalletNum, p.totalWalletsNum)
	p.log.InfoWithBlankLine("-------------------- Начало обработки кошелька --------------------",
		"wallet", walletProgress, "origIdx", p.walletIndex, "addr", p.wallet.Address.Hex())

	selectedTasks, err := p.taskSelector.SelectTasks()
	if err != nil {
		if errors.Is(err, selector.ErrNoValidTasksSelected) {
			p.log.Warn("Для кошелька не выбрано ни одной задачи, пропускаем",
				"wallet", walletProgress, "addr", p.wallet.Address.Hex())
		} else {
			p.log.Error("Ошибка выбора задач для кошелька, пропускаем",
				"err", err, "wallet", walletProgress, "addr", p.wallet.Address.Hex())
		}
		return err
	}

	totalTasks := len(selectedTasks)
	p.log.Info("Задачи для выполнения",
		"count", totalTasks, "order", p.cfg.Actions.TaskOrder,
		"wallet", walletProgress, "addr", p.wallet.Address.Hex())

	var finalError error

	for taskIndex, taskEntry := range selectedTasks {
		taskProgress := fmt.Sprintf("%d/%d", taskIndex+1, totalTasks)
		select {
		case <-ctx.Done():
			p.log.Warn("Обработка прервана (контекст отменен перед задачей)",
				"task", taskEntry.Name, "taskNum", taskProgress,
				"wallet", walletProgress, "addr", p.wallet.Address.Hex())
			return ctx.Err()
		default:
		}

		p.log.InfoWithBlankLine("------ Начало задачи ------", "taskNum", taskProgress,
			"task", taskEntry.Name, "net", taskEntry.Network,
			"wallet", walletProgress, "addr", p.wallet.Address.Hex())

		var client *evm.Client
		var err error
		var runner tasks.TaskRunner

		runner, err = tasks.NewTask(taskEntry.Name, p.log)
		if err != nil {
			if errors.Is(err, tasks.ErrTaskConstructorNotFound) {
				p.log.Error("Конструктор задачи не найден в реестре, пропуск", "task", taskEntry.Name,
					"wallet", walletProgress, "addr", p.wallet.Address.Hex())
			} else {
				p.log.Error("Не удалось создать runner задачи, пропуск",
					"task", taskEntry.Name, "err", err,
					"wallet", walletProgress, "addr", p.wallet.Address.Hex())
			}
			if finalError == nil {
				finalError = err
			}
			continue
		}

		if taskEntry.Network != "any" {
			rpcUrls, ok := p.cfg.RPCNodes[taskEntry.Network]
			if !ok || len(rpcUrls) == 0 {
				p.log.Error("Не найдены RPC URL для сети, пропуск задачи",
					"task", taskEntry.Name, "net", taskEntry.Network,
					"wallet", walletProgress, "addr", p.wallet.Address.Hex())
				continue
			}

			p.log.Debug("Создание EVM клиента", "net", taskEntry.Network,
				"wallet", walletProgress, "addr", p.wallet.Address.Hex())
			client, err = evm.NewClient(ctx, p.log, rpcUrls)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					p.log.Warn("Создание EVM клиента прервано (контекст)",
						"task", taskEntry.Name, "err", err,
						"wallet", walletProgress, "addr", p.wallet.Address.Hex())
					return err
				} else if errors.Is(err, evm.ErrEvmClientCreationFailed) || errors.Is(err, evm.ErrNoRpcUrlsProvided) {
					p.log.Error("Не удалось создать EVM клиент (нет RPC/ошибка), пропуск", "task", taskEntry.Name,
						"net", taskEntry.Network, "err", err,
						"wallet", walletProgress, "addr", p.wallet.Address.Hex())
				} else {
					p.log.Error("Непредвиденная ошибка при создании EVM клиента, пропуск", "task", taskEntry.Name,
						"net", taskEntry.Network, "err", err,
						"wallet", walletProgress, "addr", p.wallet.Address.Hex())
				}
				if finalError == nil {
					finalError = err
				}
				continue
			}

			defer func() { //TODO: defer не нравится в цикле
				if client != nil {
					client.Close()
				}
			}()
		} else {
			p.log.Debug("Пропуск создания EVM клиента для задачи с сетью 'any'",
				"task", taskEntry.Name, "wallet", walletProgress)
		}

		executionErr := p.taskExecutor.ExecuteTaskWithRetries(ctx, p.wallet, client, taskEntry, runner)

		record := storage.TransactionRecord{
			Timestamp:     time.Now().Truncate(time.Second),
			WalletAddress: p.wallet.Address.Hex(),
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
			p.log.Error("Не удалось записать лог транзакции в БД (возможно, не связано с отменой основного ctx)",
				"task", taskEntry.Name, "err", logDbErr,
				"wallet", walletProgress, "addr", p.wallet.Address.Hex())
		}
		logTxCancel()

		if executionErr != nil {
			if finalError == nil {
				finalError = executionErr
			}
		}
		p.log.InfoWithBlankLine("------ Конец задачи ------", "taskNum", taskProgress,
			"task", taskEntry.Name, "wallet", walletProgress,
			"status", record.Status, "addr", p.wallet.Address.Hex())

		if taskIndex < totalTasks-1 {
			select {
			case <-ctx.Done():
				p.log.Warn("Обработка прервана (контекст отменен перед задержкой между задачами)",
					"wallet", walletProgress, "addr", p.wallet.Address.Hex())
				return ctx.Err()
			default:
			}
			actionDelayDuration, delayErr := utils.RandomDuration(p.cfg.Delay.BetweenActions)
			if delayErr != nil {
				p.log.Error("Ошибка получения времени задержки между задачами", "err", delayErr,
					"wallet", walletProgress, "addr", p.wallet.Address.Hex())
			} else {
				p.log.Info("Пауза перед следующей задачей", "duration", actionDelayDuration,
					"wallet", walletProgress, "addr", p.wallet.Address.Hex())
				select {
				case <-time.After(actionDelayDuration):
				case <-ctx.Done():
					p.log.Warn("Задержка между задачами прервана (контекст отменен)",
						"wallet", walletProgress, "addr", p.wallet.Address.Hex())
					return ctx.Err()
				}
			}
		}
	}

	p.log.InfoWithBlankLine("-------------------- Конец обработки кошелька ---------------------",
		"wallet", walletProgress, "addr", p.wallet.Address.Hex(),
		"status", map[bool]string{finalError == nil: "Success", finalError != nil: "Failed"}[true])

	return finalError
}
