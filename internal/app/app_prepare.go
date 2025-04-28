package app

import (
	"context"
	"errors"
	"math/rand"
	"strconv"

	"retro/internal/keyloader"
	"retro/internal/storage"
	"retro/internal/types"
)

// prepareWalletsToProcess determines the list of wallets to process based on resume state and shuffling.
func (a *Application) prepareWalletsToProcess(ctx context.Context) ([]*keyloader.LoadedKey, error) {
	processedWallets := a.wallets // Start with all wallets
	originalWalletCount := len(processedWallets)
	lastCompletedIndex := -1
	shouldShuffle := a.cfg.Wallets.ProcessOrder == types.OrderRandom

	if a.cfg.State.ResumeEnabled {
		a.log.Info("Проверка состояния для возобновления...")
		stateValue, err := a.stateStorage.GetState(ctx, "last_completed_wallet_index")
		if err != nil {
			if errors.Is(err, storage.ErrStateNotFound) {
				a.log.Info("Сохраненное состояние не найдено, начинаем с начала.")
				// No error, just means start from the beginning
			} else {
				// Actual error reading state
				a.log.Error("Ошибка чтения состояния из хранилища, начинаем с начала.", "error", err)
				// Treat as non-resumable, but maybe log it? Decide if this should stop execution.
				// For now, we continue without resume, but return the error might be better.
				// return nil, fmt.Errorf("ошибка чтения состояния: %w", err) // Option: Stop execution
			}
		} else {
			index, convErr := strconv.Atoi(stateValue)
			if convErr != nil {
				a.log.Error("Ошибка конвертации сохраненного индекса, начинаем с начала.",
					"value", stateValue, "error", convErr)
				// Treat as non-resumable
			} else {
				lastCompletedIndex = index
				a.log.Info("Обнаружено сохраненное состояние.",
					"last_completed_wallet_index", lastCompletedIndex)
			}
		}

		if lastCompletedIndex >= 0 {
			startIndex := lastCompletedIndex + 1
			if startIndex < originalWalletCount {
				processedWallets = a.wallets[startIndex:]
				a.log.Info("Возобновление работы.", "start_index", startIndex,
					"wallets_to_process", len(processedWallets), "wallets_skipped", startIndex)
			} else {
				processedWallets = []*keyloader.LoadedKey{} // All wallets were processed
				a.log.Info("Все кошельки уже были обработаны в предыдущем сеансе.")
			}
			// If resuming, shuffling is disabled to maintain order
			if shouldShuffle {
				a.log.Warn("Возобновление состояния включено, process_order: random будет проигнорирован. Обработка продолжится последовательно.")
				shouldShuffle = false
			}
		}
	} else {
		a.log.Info("Возобновление состояния отключено.")
	}

	if shouldShuffle && len(processedWallets) > 1 {
		a.log.Info("Перемешивание порядка кошельков...", "count", len(processedWallets))
		rand.Shuffle(len(processedWallets), func(i, j int) {
			processedWallets[i], processedWallets[j] = processedWallets[j], processedWallets[i]
		})
	}

	return processedWallets, nil
}
