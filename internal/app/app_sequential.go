package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"retro/internal/keyloader"
	"retro/internal/processor"
	"retro/internal/storage"
	"retro/internal/utils"
)

// processSingleWalletSequentially handles the processing of a single wallet in sequential mode.
func (a *Application) processSingleWalletSequentially(ctx context.Context, key *keyloader.LoadedKey, originalIndex, currentNum, totalNum int) error {
	a.log.Debug("Начало обработки одного кошелька (последовательно)",
		"origIdx", originalIndex, "num", fmt.Sprintf("%d/%d", currentNum, totalNum), "addr", key.Address.Hex())

	proc := processor.NewProcessor(a.cfg, key, originalIndex, currentNum, totalNum, a.txLogger, a.log)
	err := proc.Process(ctx)

	if err == nil {
		if a.cfg.State.ResumeEnabled {
			a.log.Debug("Кошелек успешно обработан (последовательно), попытка сохранения состояния.",
				"originalIndex", originalIndex)
			setStateCtx, setStateCancel := context.WithTimeout(context.Background(), 10*time.Second)
			if stateErr := a.stateStorage.SetState(setStateCtx, "last_completed_wallet_index", strconv.Itoa(originalIndex)); stateErr != nil {
				a.log.Error("Ошибка сохранения состояния (последовательно)",
					"originalIndex", originalIndex, "error", stateErr)
			}
			setStateCancel()
		}
		return nil
	} else {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			a.log.Warn("Обработка кошелька прервана контекстом (последовательно).",
				"originalIndex", originalIndex, "error", err)
		} else {
			a.log.Error("Ошибка обработки кошелька (последовательно).",
				"originalIndex", originalIndex, "error", err)
		}
		return err
	}
}

// runSequentially processes wallets one by one in the main goroutine.
func (a *Application) runSequentially(ctx context.Context, keysToProcess []*keyloader.LoadedKey) {
	totalWalletsInRun := len(keysToProcess)
	a.log.Info("Запуск последовательной обработки кошельков", "count", totalWalletsInRun)

	lastCompletedIndex := -1
	if a.cfg.State.ResumeEnabled {
		loadStateCtx, loadStateCancel := context.WithTimeout(context.Background(), 10*time.Second)
		lastIndexStr, loadErr := a.stateStorage.GetState(loadStateCtx, "last_completed_wallet_index")
		loadStateCancel()
		if loadErr == nil && lastIndexStr != "" {
			if idx, convErr := strconv.Atoi(lastIndexStr); convErr == nil {
				lastCompletedIndex = idx
				a.log.Info("Возобновление последовательной обработки.", "startFromIndex", lastCompletedIndex+1)
			} else {
				a.log.Warn("Не удалось конвертировать загруженное состояние в число (последовательно).", "value", lastIndexStr, "error", convErr)
			}
		} else if loadErr != nil && !errors.Is(loadErr, storage.ErrStateNotFound) {
			a.log.Error("Ошибка загрузки состояния (последовательно).", "error", loadErr)
		}
	}

	for i, key := range keysToProcess {
		originalIndex, findErr := a.findOriginalIndex(key.Address)
		if findErr != nil {
			a.log.Error("Не удалось найти оригинальный индекс для ключа, пропускаем.",
				"address", key.Address.Hex(), "error", findErr)
			continue
		}

		if a.cfg.State.ResumeEnabled && originalIndex <= lastCompletedIndex {
			a.log.Debug("Пропуск уже обработанного кошелька (последовательно).",
				"originalIndex", originalIndex, "lastCompletedIndex", lastCompletedIndex)
			continue
		}

		select {
		case <-ctx.Done():
			a.log.Warn("Последовательная обработка прервана (контекст отменен) перед обработкой кошелька.",
				"walletIndex", originalIndex)
			return
		default:
		}

		err := a.processSingleWalletSequentially(ctx, key, originalIndex, i+1, totalWalletsInRun)
		if err != nil {
			a.log.Warn("Прерываем последовательную обработку из-за ошибки/отмены в кошельке.", "walletIndex", originalIndex)
			return
		}

		if i < len(keysToProcess)-1 {
			delayDuration, delayErr := utils.RandomDuration(a.cfg.Delay.BetweenAccounts)
			if delayErr != nil {
				a.log.Error("Ошибка получения времени задержки между кошельками (последовательно)",
					"err", delayErr)
			} else if delayDuration > 0 {
				a.log.Info("Пауза перед следующим кошельком (последовательно)",
					"duration", delayDuration)
				select {
				case <-time.After(delayDuration):
				case <-ctx.Done():
					a.log.Warn("Задержка между кошельками прервана (контекст отменен, последовательно)")
					return
				}
			}
		}
	}
	a.log.Info("Последовательная обработка всех кошельков завершена.")
}
