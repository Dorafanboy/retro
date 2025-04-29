package app

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"retro/internal/keyloader"
	"retro/internal/processor"
	"retro/internal/storage"
	"retro/internal/utils"
)

// result is used in the channel for parallel processing results.
type result struct {
	originalIndex int
	err           error
}

// processWalletWorker is the main function for a single parallel worker goroutine.
func (a *Application) processWalletWorker(
	ctx context.Context,
	key *keyloader.LoadedKey,
	originalIndex int,
	currentNum int,
	totalNum int,
	resultsChan chan<- result,
	semaphore chan struct{},
	wg *sync.WaitGroup,
) {
	defer func() {
		semaphore <- struct{}{}
		wg.Done()
		a.log.Debug("Слот воркера освобожден.", "wIdx", originalIndex)
	}()

	var processErr error
	defer func() {
		resultsChan <- result{originalIndex: originalIndex, err: processErr}
	}()

	select {
	case <-ctx.Done():
		a.log.Warn("Обработка кошелька пропущена воркером (контекст отменен перед стартом)",
			"wIdx", originalIndex, "addr", key.Address.Hex())
		processErr = ctx.Err()
		return
	default:
	}

	a.log.Debug("Воркер начинает обработку кошелька.", "wIdx", originalIndex, "addr", key.Address.Hex())
	proc := processor.NewProcessor(a.cfg, key, originalIndex, currentNum, totalNum, a.txLogger, a.log)
	processErr = proc.Process(ctx)

	if processErr == nil {
		delayDuration, delayErr := utils.RandomDuration(a.cfg.Delay.BetweenAccounts)
		if delayErr != nil {
			a.log.Error("Ошибка получения времени задержки между аккаунтами (в воркере)",
				"wIdx", originalIndex, "err", delayErr)
		} else if delayDuration > 0 {
			a.log.Info("Пауза воркера после обработки аккаунта", "wIdx", originalIndex, "duration", delayDuration)
			select {
			case <-time.After(delayDuration):
			case <-ctx.Done():
				a.log.Warn("Пауза воркера прервана (контекст отменен)", "wIdx", originalIndex)
			}
		}
	} else {
		a.log.Debug("Воркер завершил обработку кошелька с ошибкой.", "wIdx", originalIndex, "err", processErr)
	}
}

// handleParallelResults listens on the results channel and processes completed wallet results.
func (a *Application) handleParallelResults(ctx context.Context, resultsChan <-chan result, totalKeys int) {
	processedCount := 0
	highestCompletedIndex := -1

	if a.cfg.State.ResumeEnabled {
		loadStateCtx, loadStateCancel := context.WithTimeout(context.Background(), 10*time.Second)
		lastIndexStr, loadErr := a.stateStorage.GetState(loadStateCtx, "last_completed_wallet_index")
		loadStateCancel()
		if loadErr == nil && lastIndexStr != "" {
			if idx, convErr := strconv.Atoi(lastIndexStr); convErr == nil {
				highestCompletedIndex = idx
				a.log.Debug("Загружено предыдущее состояние.", "lastCompletedIndex", highestCompletedIndex)
			} else {
				a.log.Warn("Не удалось конвертировать загруженное состояние в число.", "value", lastIndexStr, "error", convErr)
			}
		} else if loadErr != nil && !errors.Is(loadErr, storage.ErrStateNotFound) {
			a.log.Error("Ошибка загрузки состояния.", "error", loadErr)
		}
	}

	for processedCount < totalKeys {
		select {
		case res, ok := <-resultsChan:
			if !ok {
				a.log.Info("Канал результатов закрыт. Завершение обработки результатов.")
				return
			}
			processedCount++

			if res.err == nil {
				a.log.Debug("Кошелек успешно обработан (получен результат).", "originalIndex", res.originalIndex)
				if res.originalIndex > highestCompletedIndex {
					newHighest := res.originalIndex
					a.log.Debug("Новый максимальный успешно обработанный индекс.", "newHighestIndex", newHighest, "previousHighest", highestCompletedIndex)
					highestCompletedIndex = newHighest
					if a.cfg.State.ResumeEnabled {
						currentIndexToSave := highestCompletedIndex
						go func(indexValueToSave int) {
							setStateCtx, setStateCancel := context.WithTimeout(context.Background(), 10*time.Second)
							defer setStateCancel()
							err := a.stateStorage.SetState(setStateCtx, "last_completed_wallet_index", strconv.Itoa(indexValueToSave))
							if err != nil {
								a.log.Error("Ошибка асинхронного сохранения состояния (new highest)",
									"originalIndex", indexValueToSave, "error", err)
							} else {
								a.log.Debug("Состояние сохранено в БД (новый максимальный индекс).", "key", "last_completed_wallet_index", "value", indexValueToSave)
							}
						}(currentIndexToSave)
					}
				}
			} else {
				if errors.Is(res.err, context.Canceled) || errors.Is(res.err, context.DeadlineExceeded) {
					a.log.Warn("Обработка кошелька была прервана контекстом (получен результат).", "originalIndex", res.originalIndex, "error", res.err)
				} else {
					a.log.Error("Обработка кошелька завершилась с ошибкой (получен результат).", "originalIndex", res.originalIndex, "error", res.err)
				}
			}
		case <-ctx.Done():
			a.log.Warn("Основной контекст отменен, прекращаем ожидание результатов.", "processedCount", processedCount, "totalKeys", totalKeys)
			return
		}
	}
	a.log.Info("Все ожидаемые результаты обработки кошельков получены.", "processedCount", processedCount)
}

// runParallel handles processing wallets concurrently using worker goroutines.
func (a *Application) runParallel(ctx context.Context, keysToProcess []*keyloader.LoadedKey, numWorkers int) {
	totalWalletsInRun := len(keysToProcess)
	a.log.Info("Запуск параллельной обработки кошельков", "count", totalWalletsInRun, "workers", numWorkers)

	semaphore := make(chan struct{}, numWorkers)
	for i := 0; i < numWorkers; i++ {
		semaphore <- struct{}{}
	}

	resultsChan := make(chan result, len(keysToProcess))

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.handleParallelResults(ctx, resultsChan, totalWalletsInRun)
	}()

	for i, key := range keysToProcess {
		originalIndex, findErr := a.findOriginalIndex(key.Address)
		if findErr != nil {
			a.log.Error("Не удалось найти оригинальный индекс для ключа в runParallel, пропускаем.",
				"address", key.Address.Hex(), "error", findErr)
			continue
		}

		select {
		case <-ctx.Done():
			a.log.Warn("Параллельная обработка прервана (контекст отменен) перед запуском воркера.",
				"lastAttemptedOriginalIndex", originalIndex)
			goto endParallelLoop
		default:
		}

		a.log.Debug("Ожидание свободного слота воркера...", "wIdx", originalIndex)
		select {
		case <-semaphore:
			a.log.Debug("Слот воркера получен, запуск горутины.", "wIdx", originalIndex)
			a.wg.Add(1)
			go a.processWalletWorker(ctx, key, originalIndex, i+1, totalWalletsInRun, resultsChan, semaphore, a.wg)
		case <-ctx.Done():
			a.log.Warn("Параллельная обработка прервана (контекст отменен) во время ожидания слота воркера.",
				"lastAttemptedOriginalIndex", originalIndex)
			goto endParallelLoop
		}
	}

endParallelLoop:
	a.log.Info("Цикл запуска воркеров завершен. Ожидание завершения всех активных воркеров и обработки результатов...")
}
