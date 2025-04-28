package tasks

import (
	"context"
	"fmt"
	"time"

	"retro/internal/evm"
	"retro/internal/logger"
	"retro/internal/utils"
	// "retro/internal/wallet" // No longer needed
)

// LogBalanceTask is a simple task that logs the wallet's balance.
type LogBalanceTask struct {
	log logger.Logger
}

// Run executes the log balance task.
// It implements the TaskRunner interface.
func (t *LogBalanceTask) Run(ctx context.Context, signer *evm.Signer, client evm.EVMClient, taskConfig map[string]interface{}) error {
	// Use signer.Address() method
	walletAddress := signer.Address()
	t.log.Info("Запуск задачи: log_balance", "wallet", walletAddress.Hex())

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	balanceWei, err := client.GetBalance(callCtx, walletAddress)
	if err != nil {
		t.log.Error("Не удалось получить баланс", "wallet", walletAddress.Hex(), "error", err)
		return fmt.Errorf("ошибка получения баланса: %w", err)
	}

	balanceEtherStr := utils.FromWei(balanceWei)

	t.log.Success("Баланс получен", "wallet", walletAddress.Hex(), "balance_eth", balanceEtherStr)
	return nil
}

// NewLogBalanceTask creates a new instance of LogBalanceTask.
func NewLogBalanceTask(log logger.Logger) TaskRunner {
	return &LogBalanceTask{log: log}
}
