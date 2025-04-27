package app

import (
	"context"
	"errors"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"retro/internal/config"
	"retro/internal/logger"
	"retro/internal/processor"
	"retro/internal/storage"
	"retro/internal/types"
	"retro/internal/utils"
	"retro/internal/wallet"
)

// Application holds the core application logic and dependencies.
type Application struct {
	cfg          *config.Config
	wallets      []*wallet.Wallet
	wg           *sync.WaitGroup
	txLogger     storage.TransactionLogger
	stateStorage storage.StateStorage
	log          logger.Logger
}

// NewApplication creates a new Application instance.
func NewApplication(cfg *config.Config, wallets []*wallet.Wallet, wg *sync.WaitGroup, txLogger storage.TransactionLogger, stateStorage storage.StateStorage, log logger.Logger) *Application {
	// Условие перемешивания будет перенесено в Run
	// if cfg.Wallets.ProcessOrder == types.OrderRandom {
	// 	log.Info("Перемешивание порядка кошельков...")
	// 	rand.Shuffle(len(wallets), func(i, j int) {
	// 		wallets[i], wallets[j] = wallets[j], wallets[i]
	// 	})
	// }
	return &Application{
		cfg:          cfg,
		wallets:      wallets,
		wg:           wg,
		txLogger:     txLogger,
		stateStorage: stateStorage,
		log:          log,
	}
}

// Run starts the main application logic loop, processing wallets.
// It handles sequential or parallel execution based on config.
func (a *Application) Run(ctx context.Context) {
	// --- Начальная настройка и логика возобновления (без изменений) ---
	processedWallets := a.wallets
	originalWalletCount := len(processedWallets)
	lastCompletedIndex := -1
	shouldShuffle := (a.cfg.Wallets.ProcessOrder == types.OrderRandom)

	if a.cfg.State.ResumeEnabled {
		a.log.Info("Проверка состояния для возобновления...")
		stateValue, err := a.stateStorage.GetState(ctx, "last_completed_wallet_index")
		if err != nil {
			if errors.Is(err, storage.ErrStateNotFound) {
				a.log.Info("Сохраненное состояние не найдено, начинаем с начала.")
			} else {
				a.log.Error("Ошибка чтения состояния из хранилища, начинаем с начала.", "error", err)
			}
		} else {
			index, convErr := strconv.Atoi(stateValue)
			if convErr != nil {
				a.log.Error("Ошибка конвертации сохраненного индекса, начинаем с начала.", "value", stateValue, "error", convErr)
			} else {
				lastCompletedIndex = index
				a.log.Info("Обнаружено сохраненное состояние.", "last_completed_wallet_index", lastCompletedIndex)
			}
		}

		if lastCompletedIndex >= 0 {
			startIndex := lastCompletedIndex + 1
			if startIndex < originalWalletCount {
				processedWallets = a.wallets[startIndex:]
				a.log.Info("Возобновление работы.", "start_index", startIndex, "wallets_to_process", len(processedWallets), "wallets_skipped", startIndex)
			} else {
				processedWallets = []*wallet.Wallet{}
				a.log.Info("Все кошельки уже были обработаны в предыдущем сеансе.")
			}
			// При возобновлении всегда работаем последовательно
			if shouldShuffle {
				a.log.Warn("Возобновление состояния включено, process_order: random будет проигнорирован. Обработка продолжится последовательно.")
				shouldShuffle = false
			}
		}
	} else {
		a.log.Info("Возобновление состояния отключено.")
	}

	if shouldShuffle {
		a.log.Info("Перемешивание порядка кошельков...")
		rand.Shuffle(len(processedWallets), func(i, j int) {
			processedWallets[i], processedWallets[j] = processedWallets[j], processedWallets[i]
		})
	}

	if len(processedWallets) == 0 {
		a.log.Info("Нет кошельков для обработки в этом сеансе.")
		return
	}

	// --- Определение режима (последовательный или параллельный) ---
	numWorkers := a.cfg.Concurrency.MaxParallelWallets
	isSequential := false
	if numWorkers <= 0 {
		numWorkers = 1
		isSequential = true
		a.log.Warn("MaxParallelWallets <= 0, используется последовательный режим (1 воркер).", "configured_value", a.cfg.Concurrency.MaxParallelWallets)
	} else if numWorkers == 1 {
		isSequential = true
		a.log.Info("MaxParallelWallets = 1, используется последовательный режим.")
	} else {
		if numWorkers > len(processedWallets) {
			numWorkers = len(processedWallets)
			a.log.Info("Запрошено больше воркеров, чем кошельков, используется количество кошельков.", "requested", a.cfg.Concurrency.MaxParallelWallets, "using", numWorkers)
		}
		a.log.Info("Начало обработки кошельков в параллельном режиме", "count", len(processedWallets), "workers", numWorkers)
	}

	// --- Выполнение ---
	if isSequential {
		a.runSequentially(ctx, processedWallets)
	} else {
		a.runParallel(ctx, processedWallets, numWorkers)
	}

	a.log.Info("Завершение основного потока Application.Run.")
	// wg.Wait() и финальные логи остаются в main.go
}

// runSequentially processes wallets one by one in the main goroutine.
func (a *Application) runSequentially(ctx context.Context, walletsToProcess []*wallet.Wallet) {
	totalWalletsInRun := len(walletsToProcess)
	a.log.Info("Запуск последовательной обработки кошельков", "count", totalWalletsInRun)

	for i, w := range walletsToProcess {
		// Находим оригинальный индекс для сохранения состояния
		originalIndex := -1
		for oi, ow := range a.wallets {
			if ow.Address == w.Address {
				originalIndex = oi
				break
			}
		}
		if originalIndex == -1 {
			a.log.Error("Не удалось найти оригинальный индекс для кошелька, пропускаем.", "address", w.Address.Hex())
			continue
		}

		// Проверяем контекст перед каждой итерацией
		select {
		case <-ctx.Done():
			a.log.Warn("Последовательная обработка прервана (контекст отменен).", "lastProcessedOriginalIndex", originalIndex-1)
			return
		default:
		}

		// Передаем i+1 как currentNum и totalWalletsInRun как totalNum
		proc := processor.NewProcessor(a.cfg, w, originalIndex, i+1, totalWalletsInRun, a.txLogger, a.log)
		err := proc.Process(ctx)

		// Обрабатываем результат и сохраняем состояние
		if err == nil {
			// Успех
			if a.cfg.State.ResumeEnabled {
				a.log.Debug("Кошелек успешно обработан (последовательно), попытка сохранения состояния.", "originalIndex", originalIndex)
				setStateCtx, setStateCancel := context.WithTimeout(context.Background(), 10*time.Second)
				if stateErr := a.stateStorage.SetState(setStateCtx, "last_completed_wallet_index", strconv.Itoa(originalIndex)); stateErr != nil {
					a.log.Error("Ошибка сохранения состояния (последовательно)", "originalIndex", originalIndex, "error", stateErr)
				}
				setStateCancel()
			}
		} else {
			// Ошибка или отмена контекста во время обработки
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				a.log.Warn("Обработка кошелька прервана контекстом (последовательно).", "originalIndex", originalIndex, "error", err)
			} else {
				a.log.Error("Ошибка обработки кошелька (последовательно).", "originalIndex", originalIndex, "error", err)
			}
			// Прерываем дальнейшую обработку при ошибке?
			a.log.Warn("Прерываем последовательную обработку из-за ошибки/отмены.")
			return
		}

		// Пауза *после* успешной обработки, если не последний
		if i < len(walletsToProcess)-1 {
			delayDuration, delayErr := utils.RandomDuration(a.cfg.Delay.BetweenAccounts)
			if delayErr != nil {
				a.log.Error("Ошибка получения времени задержки между кошельками (последовательно)", "err", delayErr)
			} else {
				a.log.Info("Пауза перед следующим кошельком (последовательно)", "duration", delayDuration)
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
func (a *Application) runParallel(ctx context.Context, walletsToProcess []*wallet.Wallet, numWorkers int) {
	totalWalletsInRun := len(walletsToProcess)
	a.log.Info("Запуск параллельной обработки кошельков", "count", totalWalletsInRun, "workers", numWorkers)

	semaphore := make(chan struct{}, numWorkers)
	type result struct { // Определение result перенесено сюда
		originalIndex int
		err           error
	}
	resultsChan := make(chan result, len(walletsToProcess))

	// Горутина для сбора результатов и сохранения состояния
	go func() {
		processedCount := 0
		highestCompletedIndex := -1 // Отслеживаем максимальный индекс

		for processedCount < len(walletsToProcess) {
			select {
			case res := <-resultsChan:
				processedCount++
				if res.err == nil {
					// Успешное завершение кошелька
					a.log.Debug("Кошелек успешно обработан.", "originalIndex", res.originalIndex)

					// Сохраняем состояние, только если этот индекс больше предыдущего сохраненного
					if res.originalIndex > highestCompletedIndex {
						highestCompletedIndex = res.originalIndex // Обновляем максимум
						a.log.Debug("Новый максимальный успешно обработанный индекс.", "highestIndex", highestCompletedIndex)

						// Сохраняем актуальный максимальный индекс асинхронно
						if a.cfg.State.ResumeEnabled {
							// Фиксируем текущее значение highestCompletedIndex для передачи в горутину
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
							}(currentIndexToSave) // Передаем зафиксированное значение
						}
					}
				} else {
					// Обработка кошелька завершилась с ошибкой (или была отменена)
					if errors.Is(res.err, context.Canceled) || errors.Is(res.err, context.DeadlineExceeded) {
						a.log.Warn("Обработка кошелька была прервана контекстом.", "originalIndex", res.originalIndex, "error", res.err)
					} else {
						a.log.Error("Обработка кошелька завершилась с ошибкой.", "originalIndex", res.originalIndex, "error", res.err)
					}
					// Не сохраняем состояние, если была ошибка
				}
			case <-ctx.Done():
				a.log.Warn("Основной контекст отменен, прекращаем ожидание результатов.")
				return // Выходим из горутины обработки результатов
			}
		}
		close(resultsChan)
		a.log.Info("Все результаты обработки кошельков получены.")
	}()

	// Запуск горутин-воркеров с ограничением семафором
	for i, w := range walletsToProcess {
		originalIndex := -1
		for oi, ow := range a.wallets {
			if ow.Address == w.Address {
				originalIndex = oi
				break
			}
		}
		if originalIndex == -1 {
			a.log.Error("Не удалось найти оригинальный индекс для кошелька, пропускаем.", "address", w.Address.Hex())
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
		// Передаем i+1 как currentNum и totalWalletsInRun как totalNum
		go func(currentWallet *wallet.Wallet, walletLoopIndex int, walletOriginalIndex int, currentNum int, totalNum int) {
			defer func() { <-semaphore }()
			defer a.wg.Done()

			// Переменная для хранения результата этой горутины
			var processErr error

			// Запускаем вложенную функцию, чтобы можно было использовать defer для отправки результата
			func() {
				defer func() {
					// Отправляем результат в канал независимо от того, была ошибка или нет
					resultsChan <- result{originalIndex: walletOriginalIndex, err: processErr}
				}()

				select {
				case <-ctx.Done():
					a.log.Warn("Обработка кошелька пропущена (контекст отменен перед стартом)", "wIdx", walletOriginalIndex, "addr", currentWallet.Address.Hex())
					processErr = ctx.Err() // Сохраняем ошибку контекста
					return
				default:
				}

				// Используем переданные currentNum и totalNum
				proc := processor.NewProcessor(a.cfg, currentWallet, walletOriginalIndex, currentNum, totalNum, a.txLogger, a.log)
				processErr = proc.Process(ctx)

				// --- Пауза после успешной обработки ---
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
							// Не возвращаем ошибку, просто выходим из паузы
						}
					}
				}
				// --- Конец паузы ---
			}()
		}(w, i, originalIndex, i+1, totalWalletsInRun)
	}

endParallelLoop:
	a.log.Info("Цикл запуска параллельных горутин завершен.")
	// Ожидание завершения всех запущенных горутин произойдет через wg.Wait() в main
}
