package app

import (
	"context"
	"errors"
	"strconv"
	"time"

	"retro/internal/keyloader"
	"retro/internal/processor"
	"retro/internal/utils"
)

// runProcessing determines the execution mode (sequential/parallel) and starts processing.
func (a *Application) runProcessing(ctx context.Context, keysToProcess []*keyloader.LoadedKey) {
	numWorkers := a.cfg.Concurrency.MaxParallelWallets
	isSequential := false
	if numWorkers <= 0 {
		numWorkers = 1 // Ensure at least one worker
		isSequential = true
		a.log.Warn("MaxParallelWallets <= 0, используется последовательный режим (1 воркер).",
			"configured_value", a.cfg.Concurrency.MaxParallelWallets)
	} else if numWorkers == 1 {
		isSequential = true
		a.log.Info("MaxParallelWallets = 1, используется последовательный режим.")
	} else {
		if numWorkers > len(keysToProcess) {
			numWorkers = len(keysToProcess) // Don't use more workers than keys
			a.log.Info("Запрошено больше воркеров, чем ключей, используется количество ключей.",
				"requested", a.cfg.Concurrency.MaxParallelWallets, "using", numWorkers)
		}
		a.log.Info("Начало обработки ключей в параллельном режиме",
			"count", len(keysToProcess), "workers", numWorkers)
	}

	if isSequential {
		a.runSequentially(ctx, keysToProcess)
	} else {
		a.runParallel(ctx, keysToProcess, numWorkers)
	}
}

// runSequentially processes wallets one by one in the main goroutine.
func (a *Application) runSequentially(ctx context.Context, keysToProcess []*keyloader.LoadedKey) {
	totalWalletsInRun := len(keysToProcess)
	a.log.Info("Запуск последовательной обработки кошельков", "count", totalWalletsInRun)

	for i, key := range keysToProcess {
		originalIndex := -1
		for oi, originalKey := range a.wallets {
			if originalKey.Address == key.Address {
				originalIndex = oi
				break
			}
		}
		if originalIndex == -1 {
			a.log.Error("Не удалось найти оригинальный индекс для ключа, пропускаем.",
				"address", key.Address.Hex())
			continue
		}

		select {
		case <-ctx.Done():
			a.log.Warn("Последовательная обработка прервана (контекст отменен).",
				"lastProcessedOriginalIndex", originalIndex-1)
			return
		default:
		}

		proc := processor.NewProcessor(a.cfg, key, originalIndex, i+1, totalWalletsInRun, a.txLogger, a.log)
		err := proc.Process(ctx)

		if err == nil {
			// Успех
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
		} else {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				a.log.Warn("Обработка кошелька прервана контекстом (последовательно).",
					"originalIndex", originalIndex, "error", err)
			} else {
				a.log.Error("Ошибка обработки кошелька (последовательно).",
					"originalIndex", originalIndex, "error", err)
			}
			a.log.Warn("Прерываем последовательную обработку из-за ошибки/отмены.")
			return
		}

		if i < len(keysToProcess)-1 {
			delayDuration, delayErr := utils.RandomDuration(a.cfg.Delay.BetweenAccounts)
			if delayErr != nil {
				a.log.Error("Ошибка получения времени задержки между кошельками (последовательно)",
					"err", delayErr)
			} else {
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

// runParallel handles processing wallets concurrently using worker goroutines.
func (a *Application) runParallel(ctx context.Context, keysToProcess []*keyloader.LoadedKey, numWorkers int) {
	totalWalletsInRun := len(keysToProcess)
	a.log.Info("Запуск параллельной обработки кошельков", "count", totalWalletsInRun, "workers", numWorkers)

	semaphore := make(chan struct{}, numWorkers)
	type result struct {
		originalIndex int
		err           error
	}
	resultsChan := make(chan result, len(keysToProcess))

	go func() {
		processedCount := 0
		highestCompletedIndex := -1

		for processedCount < len(keysToProcess) {
			select {
			case res := <-resultsChan:
				processedCount++
				if res.err == nil {
					a.log.Debug("Кошелек успешно обработан.", "originalIndex", res.originalIndex)

					if res.originalIndex > highestCompletedIndex {
						highestCompletedIndex = res.originalIndex
						a.log.Debug("Новый максимальный успешно обработанный индекс.", "highestIndex", highestCompletedIndex)

						if a.cfg.State.ResumeEnabled {
							currentIndexToSave := highestCompletedIndex
							go func(indexValueToSave int) {
								setStateCtx, setStateCancel := context.WithTimeout(context.Background(), 10*time.Second)
								defer setStateCancel()
								err := a.stateStorage.SetState(setStateCtx, "last_completed_wallet_index", strconv.Itoa(indexValueToSave))
								if err != nil {
									a.log.Error("Ошибка асинхронного сохранения состояния (new highest)", "originalIndex", indexValueToSave, "error", err)
								} else {
									a.log.Debug("State saved to DB (new highest)", "key", "last_completed_wallet_index", "value", indexValueToSave)
								}
							}(currentIndexToSave)
						}
					}
				} else {
					if errors.Is(res.err, context.Canceled) || errors.Is(res.err, context.DeadlineExceeded) {
						a.log.Warn("Обработка кошелька была прервана контекстом.", "originalIndex", res.originalIndex, "error", res.err)
					} else {
						a.log.Error("Обработка кошелька завершилась с ошибкой.", "originalIndex", res.originalIndex, "error", res.err)
					}
				}
			case <-ctx.Done():
				a.log.Warn("Основной контекст отменен, прекращаем ожидание результатов.")
				return
			}
		}
		close(resultsChan)
		a.log.Info("Все результаты обработки кошельков получены.")
	}()

	for i, key := range keysToProcess {
		originalIndex := -1
		for oi, originalKey := range a.wallets {
			if originalKey.Address == key.Address {
				originalIndex = oi
				break
			}
		}
		if originalIndex == -1 {
			a.log.Error("Не удалось найти оригинальный индекс для ключа, пропускаем.", "address", key.Address.Hex())
			continue
		}

		select {
		case <-ctx.Done():
			a.log.Warn("Параллельная обработка прервана (контекст отменен).", "lastProcessedOriginalIndex", originalIndex-1)
			goto endParallelLoop
		default:
		}

		a.log.Debug("Ожидание свободного слота воркера...", "originalIndex", originalIndex, "workers_busy", len(semaphore), "workers_total", numWorkers)
		semaphore <- struct{}{}
		a.log.Debug("Слот воркера получен, запуск горутины.", "originalIndex", originalIndex)

		a.wg.Add(1)
		go func(currentKey *keyloader.LoadedKey, walletLoopIndex int, walletOriginalIndex int, currentNum int, totalNum int) {
			defer func() { <-semaphore }()
			defer a.wg.Done()

			var processErr error

			func() {
				defer func() {
					resultsChan <- result{originalIndex: walletOriginalIndex, err: processErr}
				}()

				select {
				case <-ctx.Done():
					a.log.Warn("Обработка кошелька пропущена (контекст отменен перед стартом)", "wIdx", walletOriginalIndex, "addr", currentKey.Address.Hex())
					processErr = ctx.Err()
					return
				default:
				}

				proc := processor.NewProcessor(a.cfg, currentKey, walletOriginalIndex, currentNum, totalNum, a.txLogger, a.log)
				processErr = proc.Process(ctx)

				if processErr == nil {
					delayDuration, delayErr := utils.RandomDuration(a.cfg.Delay.BetweenAccounts)
					if delayErr != nil {
						a.log.Error("Ошибка получения времени задержки между аккаунтами (в воркере)", "wIdx", walletOriginalIndex, "err", delayErr)
					} else if delayDuration > 0 {
						a.log.Info("Пауза воркера после обработки аккаунта", "wIdx", walletOriginalIndex, "duration", delayDuration)
						select {
						case <-time.After(delayDuration):
						case <-ctx.Done():
							a.log.Warn("Пауза воркера прервана (контекст отменен)", "wIdx", walletOriginalIndex)
						}
					}
				}
			}()
		}(key, i, originalIndex, i+1, totalWalletsInRun)
	}

endParallelLoop:
	a.log.Info("Цикл запуска параллельных горутин завершен.")
}
