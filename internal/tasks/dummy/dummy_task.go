package dummy

import (
	"context"
	"time"

	"retro/internal/evm"
	"retro/internal/logger"
	"retro/internal/tasks"
	"retro/internal/utils"
	"retro/internal/wallet"
)

// DummyTask - это простая задача-заглушка для тестирования.
type DummyTask struct{}

// Убедимся, что DummyTask реализует интерфейс TaskRunner
var _ tasks.TaskRunner = (*DummyTask)(nil)

// NewTask создает новый экземпляр DummyTask.
func NewTask() tasks.TaskRunner {
	return &DummyTask{}
}

// Run выполняет логику задачи-заглушки.
func (dt *DummyTask) Run(ctx context.Context, w *wallet.Wallet, client evm.EVMClient, params map[string]interface{}) error {
	logger.Info("Начало выполнения задачи-заглушки (DummyTask)", "wallet", w.Address.Hex())

	// Имитация работы
	delay := time.Duration(utils.RandomIntInRange(1, 3)) * time.Second
	logger.Debug("DummyTask: имитация работы...", "delay", delay, "wallet", w.Address.Hex())
	time.Sleep(delay)

	// Можно добавить сюда логику для генерации фейкового хэша транзакции,
	// если логгер транзакций ожидает его для записи в БД.
	// txHash := common.HexToHash(fmt.Sprintf("0x%x", time.Now().UnixNano()))

	logger.Success("Задача-заглушка (DummyTask) успешно завершена", "wallet", w.Address.Hex())
	return nil // Возвращаем nil, имитируя успешное выполнение
}
