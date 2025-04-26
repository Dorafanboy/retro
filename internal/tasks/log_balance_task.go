package tasks

import (
	"context"
	"fmt"
	"time"

	"retro/internal/evm"
	"retro/internal/logger"
	"retro/internal/utils"
	"retro/internal/wallet"
)

// LogBalanceTask is a simple task that logs the wallet's balance.
type LogBalanceTask struct{}

// Run executes the log balance task.
func (t *LogBalanceTask) Run(ctx context.Context, w *wallet.Wallet, client evm.EVMClient, taskConfig map[string]interface{}) error {
	logger.Info("Запуск задачи: log_balance", "wallet", w.Address.Hex())

	// Создаем дочерний контекст с таймаутом от переданного ctx
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	balanceWei, err := client.GetBalance(callCtx, w.Address) // Используем callCtx
	if err != nil {
		logger.Error("Не удалось получить баланс", "wallet", w.Address.Hex(), "error", err)
		return fmt.Errorf("ошибка получения баланса: %w", err)
	}

	balanceEtherStr := utils.FromWei(balanceWei)

	logger.Success("Баланс получен", "wallet", w.Address.Hex(), "balance_eth", balanceEtherStr)
	return nil
}

// init registers the task when the package is imported.
func init() {
	err := RegisterTask("log_balance", &LogBalanceTask{})
	if err != nil {
		// Using panic here because registration failure during init is critical
		panic(fmt.Sprintf("Failed to register task log_balance: %v", err))
	}
}
