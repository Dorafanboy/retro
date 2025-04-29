package app

import (
	"context"
	"fmt"

	"retro/internal/keyloader"

	"github.com/ethereum/go-ethereum/common"
)

// runProcessing determines the execution mode (sequential/parallel) and starts processing.
func (a *Application) runProcessing(ctx context.Context, keysToProcess []*keyloader.LoadedKey) {
	numWorkers := a.cfg.Concurrency.MaxParallelWallets
	isSequential := false
	if numWorkers <= 0 {
		numWorkers = 1
		isSequential = true
		a.log.Warn("MaxParallelWallets <= 0, используется последовательный режим (1 воркер).",
			"configured_value", a.cfg.Concurrency.MaxParallelWallets)
	} else if numWorkers == 1 {
		isSequential = true
		a.log.Info("MaxParallelWallets = 1, используется последовательный режим.")
	} else {
		if numWorkers > len(keysToProcess) {
			numWorkers = len(keysToProcess)
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

// findOriginalIndex searches for the original index of a key by its address.
func (a *Application) findOriginalIndex(keyAddress common.Address) (int, error) {
	for oi, originalKey := range a.wallets {
		if originalKey.Address == keyAddress {
			return oi, nil
		}
	}
	return -1, fmt.Errorf("оригинальный индекс для адреса %s не найден", keyAddress.Hex())
}
