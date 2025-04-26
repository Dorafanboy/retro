package app

import (
	"context"
	"errors"
	"time"

	"retro/internal/config"
	"retro/internal/evm"
	"retro/internal/logger"
	"retro/internal/tasks"
	"retro/internal/utils"
	"retro/internal/wallet"
)

var (
	ErrNoValidTasksSelected = errors.New("no valid and active tasks selected for the wallet")
)

// WalletProcessor encapsulates the logic for processing a single wallet.
type WalletProcessor struct {
	cfg          *config.Config
	wallet       *wallet.Wallet
	walletIndex  int
	taskSelector *TaskSelector
	taskExecutor *TaskExecutor
}

// newWalletProcessor creates a new WalletProcessor instance.
func newWalletProcessor(cfg *config.Config, w *wallet.Wallet, index int, registeredTaskNames []string) *WalletProcessor {
	taskSelector := newTaskSelector(cfg, registeredTaskNames)
	taskExecutor := newTaskExecutor(cfg)

	return &WalletProcessor{
		cfg:          cfg,
		wallet:       w,
		walletIndex:  index,
		taskSelector: taskSelector,
		taskExecutor: taskExecutor,
	}
}

// Process processes one wallet: selects tasks, executes them with retries and delays.
func (wp *WalletProcessor) Process(ctx context.Context) {
	logger.Info("-------------------- Начало обработки кошелька --------------------",
		"wIdx", wp.walletIndex+1,
		"addr", wp.wallet.Address.Hex())

	selectedTasks, err := wp.taskSelector.SelectTasks()
	if err != nil {
		if errors.Is(err, ErrNoValidTasksSelected) {
			logger.Warn("Для кошелька не выбрано ни одной задачи, пропускаем",
				"wIdx", wp.walletIndex+1,
				"addr", wp.wallet.Address.Hex())
		} else {
			logger.Error("Ошибка выбора задач для кошелька, пропускаем",
				"err", err,
				"wIdx", wp.walletIndex+1,
				"addr", wp.wallet.Address.Hex())
		}
		return
	}

	logger.Info("Задачи для выполнения",
		"count", len(selectedTasks),
		"order", wp.cfg.Actions.TaskOrder,
		"wIdx", wp.walletIndex+1,
		"addr", wp.wallet.Address.Hex())

	for taskIndex, taskEntry := range selectedTasks {
		select {
		case <-ctx.Done():
			logger.Warn("Обработка прервана (контекст отменен перед задачей)",
				"task", taskEntry.Name,
				"wIdx", wp.walletIndex+1,
				"addr", wp.wallet.Address.Hex())
			return
		default:
		}

		logger.Info("------ Начало задачи ------",
			"taskIdx", taskIndex+1,
			"task", taskEntry.Name,
			"net", taskEntry.Network,
			"wIdx", wp.walletIndex+1,
			"addr", wp.wallet.Address.Hex())

		runner, err := tasks.GetTask(taskEntry.Name)
		if err != nil {
			if errors.Is(err, tasks.ErrTaskNotFound) {
				logger.Error("Задача не найдена в реестре, пропуск",
					"task", taskEntry.Name,
					"err", err,
					"wIdx", wp.walletIndex+1,
					"addr", wp.wallet.Address.Hex())
			} else {
				logger.Error("Не удалось получить runner задачи, пропуск",
					"task", taskEntry.Name,
					"err", err,
					"wIdx", wp.walletIndex+1,
					"addr", wp.wallet.Address.Hex())
			}
			continue
		}

		rpcUrls, ok := wp.cfg.RPCNodes[taskEntry.Network]
		if !ok || len(rpcUrls) == 0 {
			logger.Error("Не найдены RPC URL для сети, пропуск задачи",
				"task", taskEntry.Name,
				"net", taskEntry.Network,
				"wIdx", wp.walletIndex+1,
				"addr", wp.wallet.Address.Hex())
			continue
		}

		logger.Debug("Создание EVM клиента",
			"net", taskEntry.Network,
			"wIdx", wp.walletIndex+1,
			"addr", wp.wallet.Address.Hex())
		client, err := evm.NewClient(ctx, rpcUrls)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				logger.Warn("Создание EVM клиента прервано (контекст)",
					"task", taskEntry.Name,
					"err", err,
					"wIdx", wp.walletIndex+1,
					"addr", wp.wallet.Address.Hex())
				return
			} else if errors.Is(err, evm.ErrEvmClientCreationFailed) || errors.Is(err, evm.ErrNoRpcUrlsProvided) {
				logger.Error("Не удалось создать EVM клиент (нет RPC/ошибка), пропуск",
					"task", taskEntry.Name,
					"net", taskEntry.Network,
					"err", err,
					"wIdx", wp.walletIndex+1,
					"addr", wp.wallet.Address.Hex())
			} else {
				logger.Error("Непредвиденная ошибка при создании EVM клиента, пропуск",
					"task", taskEntry.Name,
					"net", taskEntry.Network,
					"err", err,
					"wIdx", wp.walletIndex+1,
					"addr", wp.wallet.Address.Hex())
			}
			continue
		}

		wp.taskExecutor.ExecuteTaskWithRetries(ctx, wp.wallet, client, taskEntry, runner)
		client.Close()
		logger.Info("------ Конец задачи ------",
			"task", taskEntry.Name,
			"wIdx", wp.walletIndex+1,
			"addr", wp.wallet.Address.Hex())

		if taskIndex < len(selectedTasks)-1 {
			select {
			case <-ctx.Done():
				logger.Warn("Обработка прервана (контекст отменен перед задержкой)",
					"wIdx", wp.walletIndex+1,
					"addr", wp.wallet.Address.Hex())
				return
			default:
			}
			actionDelayDuration, delayErr := utils.RandomDuration(wp.cfg.Delay.BetweenActions)
			if delayErr != nil {
				logger.Error("Ошибка получения времени задержки между задачами",
					"err", delayErr,
					"wIdx", wp.walletIndex+1,
					"addr", wp.wallet.Address.Hex())
			} else {
				logger.Info("Пауза перед следующей задачей",
					"duration", actionDelayDuration,
					"wIdx", wp.walletIndex+1,
					"addr", wp.wallet.Address.Hex())
				time.Sleep(actionDelayDuration)
			}
		}
	}

	logger.Info("-------------------- Конец обработки кошелька ---------------------",
		"wIdx", wp.walletIndex+1,
		"addr", wp.wallet.Address.Hex())
}
