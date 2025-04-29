package processor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"retro/internal/config"
	"retro/internal/evm"
	"retro/internal/storage"
	"retro/internal/tasks"
	"retro/internal/types"
	"retro/internal/utils"
)

// getEvmClientForTask creates an EVM client for the specified network if needed.
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
		return nil, nil, err
	}

	client, err := p.getEvmClientForTask(ctx, taskEntry.Network)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			p.log.Warn("Создание EVM клиента прервано (контекст)",
				"task", taskEntry.Name, "err", err,
				"wallet", walletProgress, "addr", walletAddress.Hex())
			return nil, runner, err
		}
		p.log.Error("Не удалось получить EVM клиент, пропуск задачи",
			"task", taskEntry.Name, "net", taskEntry.Network, "err", err,
			"wallet", walletProgress, "addr", walletAddress.Hex())
		return nil, runner, err
	}

	return client, runner, nil
}

// executeAndLogTask executes the task using the executor, closes the client, and logs the transaction.
func (p *Processor) executeAndLogTask(ctx context.Context, taskEntry config.TaskConfigEntry, runner tasks.TaskRunner, client *evm.Client, walletProgress string) error {
	executionErr := p.taskExecutor.ExecuteTaskWithRetries(ctx, p.signer, client, taskEntry, runner)

	if client != nil {
		client.Close()
		p.log.Debug("EVM клиент закрыт", "task", taskEntry.Name, "net", taskEntry.Network, "wallet", walletProgress)
	}

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
		p.log.Error("Не удалось записать лог транзакции в БД",
			"task", taskEntry.Name, "err", logDbErr,
			"wallet", walletProgress, "addr", p.signer.Address().Hex())
	}
	logTxCancel()

	return executionErr
}

// performInterTaskDelay handles the delay between tasks.
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
		return nil
	}

	if actionDelayDuration <= 0 {
		return nil // No delay configured or needed
	}

	p.log.Info("Пауза перед следующей задачей", "duration", actionDelayDuration,
		"wallet", walletProgress, "addr", walletAddress.Hex())
	select {
	case <-time.After(actionDelayDuration):
		return nil
	case <-ctx.Done():
		p.log.Warn("Задержка между задачами прервана (контекст отменен)",
			"wallet", walletProgress, "addr", walletAddress.Hex())
		return ctx.Err()
	}
}
