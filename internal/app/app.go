package app

import (
	"context"
	"sync"

	"retro/internal/config"
	"retro/internal/keyloader"
	"retro/internal/logger"
	"retro/internal/storage"
)

// Application holds the core application logic and dependencies.
type Application struct {
	cfg          *config.Config
	wallets      []*keyloader.LoadedKey
	wg           *sync.WaitGroup
	txLogger     storage.TransactionLogger
	stateStorage storage.StateStorage
	log          logger.Logger
}

// NewApplication creates a new Application instance.
func NewApplication(
	cfg *config.Config,
	wallets []*keyloader.LoadedKey,
	wg *sync.WaitGroup,
	txLogger storage.TransactionLogger,
	stateStorage storage.StateStorage,
	log logger.Logger,
) *Application {
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
func (a *Application) Run(ctx context.Context) {
	keysToProcess, err := a.prepareWalletsToProcess(ctx)
	if err != nil {
		a.log.Error("Ошибка подготовки списка ключей для обработки, завершение работы.", "error", err)
		return
	}

	if len(keysToProcess) == 0 {
		a.log.Info("Нет ключей для обработки в этом сеансе.")
		return
	}

	a.runProcessing(ctx, keysToProcess)

	a.log.Info("Завершение основного потока Application.Run.")
}
