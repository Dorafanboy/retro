package processor

import (
	"context"
	"errors"
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

// ErrNoValidTasksSelected используется селектором, можно убрать или оставить, если используется еще где-то.
// var ErrNoValidTasksSelected = errors.New("no valid and active tasks selected for the wallet")

// Processor encapsulates the logic for processing a single wallet.
// Переименовано в Processor и сделано публичным
type Processor struct {
	cfg          *config.Config
	wallet       *wallet.Wallet
	walletIndex  int
	taskSelector *selector.Selector // Используем тип из нового пакета
	taskExecutor *executor.Executor // Используем тип из нового пакета
	txLogger     storage.TransactionLogger
}

// NewProcessor creates a new Processor instance.
// Переименовано в NewProcessor и сделано публичным
func NewProcessor(cfg *config.Config, w *wallet.Wallet, index int, txLogger storage.TransactionLogger) *Processor {
	taskSelector := selector.NewSelector(cfg) // Используем конструктор из нового пакета
	taskExecutor := executor.NewExecutor(cfg) // Используем конструктор из нового пакета

	return &Processor{
		cfg:          cfg,
		wallet:       w,
		walletIndex:  index,
		taskSelector: taskSelector,
		taskExecutor: taskExecutor,
		txLogger:     txLogger,
	}
}

// Process processes one wallet: selects tasks, executes them with retries and delays.
// Метод сделан публичным
func (p *Processor) Process(ctx context.Context) {
	logger.InfoWithBlankLine("-------------------- Начало обработки кошелька --------------------",
		"wIdx", p.walletIndex+1,
		"addr", p.wallet.Address.Hex())

	// Используем taskSelector из структуры
	selectedTasks, err := p.taskSelector.SelectTasks()
	if err != nil {
		// Используем ошибку из пакета selector
		if errors.Is(err, selector.ErrNoValidTasksSelected) {
			logger.Warn("Для кошелька не выбрано ни одной задачи, пропускаем",
				"wIdx", p.walletIndex+1,
				"addr", p.wallet.Address.Hex())
		} else {
			logger.Error("Ошибка выбора задач для кошелька, пропускаем",
				"err", err,
				"wIdx", p.walletIndex+1,
				"addr", p.wallet.Address.Hex())
		}
		return
	}

	logger.Info("Задачи для выполнения",
		"count", len(selectedTasks),
		"order", p.cfg.Actions.TaskOrder,
		"wIdx", p.walletIndex+1,
		"addr", p.wallet.Address.Hex())

	for taskIndex, taskEntry := range selectedTasks {
		select {
		case <-ctx.Done():
			logger.Warn("Обработка прервана (контекст отменен перед задачей)",
				"task", taskEntry.Name,
				"wIdx", p.walletIndex+1,
				"addr", p.wallet.Address.Hex())
			return
		default:
		}

		logger.InfoWithBlankLine("------ Начало задачи ------",
			"taskIdx", taskIndex+1,
			"task", taskEntry.Name,
			"net", taskEntry.Network,
			"wIdx", p.walletIndex+1,
			"addr", p.wallet.Address.Hex())

		var client *evm.Client
		var err error
		var runner tasks.TaskRunner

		// Создаем новый экземпляр задачи через конструктор
		runner, err = tasks.NewTask(taskEntry.Name)
		if err != nil {
			// Проверяем, является ли ошибка ошибкой 'конструктор не найден'
			if errors.Is(err, tasks.ErrTaskConstructorNotFound) {
				logger.Error("Конструктор задачи не найден в реестре, пропуск",
					"task", taskEntry.Name,
					"wIdx", p.walletIndex+1,
					"addr", p.wallet.Address.Hex())
			} else {
				logger.Error("Не удалось создать runner задачи, пропуск",
					"task", taskEntry.Name,
					"err", err,
					"wIdx", p.walletIndex+1,
					"addr", p.wallet.Address.Hex())
			}
			continue
		}

		if taskEntry.Network != "any" {
			rpcUrls, ok := p.cfg.RPCNodes[taskEntry.Network]
			if !ok || len(rpcUrls) == 0 {
				logger.Error("Не найдены RPC URL для сети, пропуск задачи",
					"task", taskEntry.Name,
					"net", taskEntry.Network,
					"wIdx", p.walletIndex+1,
					"addr", p.wallet.Address.Hex())
				continue
			}

			logger.Debug("Создание EVM клиента",
				"net", taskEntry.Network,
				"wIdx", p.walletIndex+1,
				"addr", p.wallet.Address.Hex())
			client, err = evm.NewClient(ctx, rpcUrls)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					logger.Warn("Создание EVM клиента прервано (контекст)",
						"task", taskEntry.Name,
						"err", err,
						"wIdx", p.walletIndex+1,
						"addr", p.wallet.Address.Hex())
					return
				} else if errors.Is(err, evm.ErrEvmClientCreationFailed) || errors.Is(err, evm.ErrNoRpcUrlsProvided) {
					logger.Error("Не удалось создать EVM клиент (нет RPC/ошибка), пропуск",
						"task", taskEntry.Name,
						"net", taskEntry.Network,
						"err", err,
						"wIdx", p.walletIndex+1,
						"addr", p.wallet.Address.Hex())
				} else {
					logger.Error("Непредвиденная ошибка при создании EVM клиента, пропуск",
						"task", taskEntry.Name,
						"net", taskEntry.Network,
						"err", err,
						"wIdx", p.walletIndex+1,
						"addr", p.wallet.Address.Hex())
				}
				continue
			}
		} else {
			logger.Debug("Пропуск создания EVM клиента для задачи с сетью 'any'",
				"task", taskEntry.Name,
				"wIdx", p.walletIndex+1)
		}

		// Используем taskExecutor из структуры
		executionErr := p.taskExecutor.ExecuteTaskWithRetries(ctx, p.wallet, client, taskEntry, runner)

		// Формируем и логируем запись о транзакции/задаче
		record := storage.TransactionRecord{
			Timestamp:     time.Now(),
			WalletAddress: p.wallet.Address.Hex(),
			TaskName:      taskEntry.Name, // Тип types.TaskName совпадает с storage.TransactionRecord
			Network:       taskEntry.Network,
			// TxHash:      "", // Пока не получаем хэш от задач
		}
		if executionErr != nil {
			record.Status = types.TxStatusFailed
			record.Error = executionErr.Error()
		} else {
			record.Status = types.TxStatusSuccess
		}

		// Используем txLogger из структуры
		if logErr := p.txLogger.LogTransaction(ctx, record); logErr != nil {
			logger.Error("Не удалось записать лог транзакции в БД",
				"task", taskEntry.Name,
				"err", logErr,
				"wIdx", p.walletIndex+1,
				"addr", p.wallet.Address.Hex())
		}

		if client != nil {
			// defer client.Close() // Перенесено
		}
		logger.InfoWithBlankLine("------ Конец задачи ------",
			"task", taskEntry.Name,
			"wIdx", p.walletIndex+1,
			"addr", p.wallet.Address.Hex())

		if taskIndex < len(selectedTasks)-1 {
			select {
			case <-ctx.Done():
				logger.Warn("Обработка прервана (контекст отменен перед задержкой)",
					"wIdx", p.walletIndex+1,
					"addr", p.wallet.Address.Hex())
				return
			default:
			}
			actionDelayDuration, delayErr := utils.RandomDuration(p.cfg.Delay.BetweenActions)
			if delayErr != nil {
				logger.Error("Ошибка получения времени задержки между задачами",
					"err", delayErr,
					"wIdx", p.walletIndex+1,
					"addr", p.wallet.Address.Hex())
			} else {
				logger.Info("Пауза перед следующей задачей",
					"duration", actionDelayDuration,
					"wIdx", p.walletIndex+1,
					"addr", p.wallet.Address.Hex())
				// Заменяем time.Sleep на select с проверкой контекста
				select {
				case <-time.After(actionDelayDuration):
					// Пауза завершена
				case <-ctx.Done():
					logger.Warn("Задержка между задачами прервана (контекст отменен)",
						"wIdx", p.walletIndex+1,
						"addr", p.wallet.Address.Hex())
					return // Выходим из Process для этого кошелька
				}
			}
		}
	}

	logger.InfoWithBlankLine("-------------------- Конец обработки кошелька ---------------------",
		"wIdx", p.walletIndex+1,
		"addr", p.wallet.Address.Hex())
}
