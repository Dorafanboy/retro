package dummy

import (
	"context"
	"time"

	"retro/internal/evm"
	"retro/internal/logger"
	"retro/internal/tasks"
	"retro/internal/utils"
	// "retro/internal/wallet" // No longer needed
)

// DummyTask - это простая задача-заглушка для тестирования.
type DummyTask struct {
	log logger.Logger
}

// Убедимся, что DummyTask реализует интерфейс TaskRunner
var _ tasks.TaskRunner = (*DummyTask)(nil)

// NewTask создает новый экземпляр DummyTask.
func NewTask(log logger.Logger) tasks.TaskRunner {
	return &DummyTask{log: log}
}

// Run выполняет логику задачи-заглушки.
func (dt *DummyTask) Run(ctx context.Context, signer *evm.Signer, client evm.EVMClient, params map[string]interface{}) error {
	walletAddress := signer.Address()
	dt.log.Info("Начало выполнения задачи-заглушки (DummyTask)", "wallet", walletAddress.Hex())

	// Имитация работы
	delay := time.Duration(utils.RandomIntInRange(1, 3)) * time.Second
	dt.log.Debug("DummyTask: имитация работы...", "delay", delay, "wallet", walletAddress.Hex())
	time.Sleep(delay)

	// Можно добавить сюда логику для генерации фейкового хэша транзакции,
	// если логгер транзакций ожидает его для записи в БД.
	// txHash := common.HexToHash(fmt.Sprintf("0x%x", time.Now().UnixNano()))

	dt.log.Success("Задача-заглушка (DummyTask) успешно завершена", "wallet", walletAddress.Hex())
	return nil // Возвращаем nil, имитируя успешное выполнение
}
