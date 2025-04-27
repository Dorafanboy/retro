package app

import (
	"context"
	"math/rand"
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
	cfg      *config.Config
	wallets  []*wallet.Wallet
	wg       *sync.WaitGroup
	txLogger storage.TransactionLogger
}

// NewApplication creates a new Application instance.
func NewApplication(cfg *config.Config, wallets []*wallet.Wallet, wg *sync.WaitGroup, txLogger storage.TransactionLogger) *Application {
	if cfg.Wallets.ProcessOrder == types.OrderRandom {
		logger.Info("Перемешивание порядка кошельков...")
		rand.Shuffle(len(wallets), func(i, j int) {
			wallets[i], wallets[j] = wallets[j], wallets[i]
		})
	}
	return &Application{
		cfg:      cfg,
		wallets:  wallets,
		wg:       wg,
		txLogger: txLogger,
	}
}

// Run starts the main application logic loop, processing wallets concurrently.
func (a *Application) Run(ctx context.Context) {
	logger.Info("Начало обработки кошельков", "count", len(a.wallets))

	for i, w := range a.wallets {
		a.wg.Add(1)
		go func(currentWallet *wallet.Wallet, walletIndex int) {
			defer a.wg.Done()

			select {
			case <-ctx.Done():
				logger.Warn("Обработка кошелька пропущена (контекст отменен перед стартом)", "wIdx", walletIndex, "addr", currentWallet.Address.Hex())
				return
			default:
			}

			proc := processor.NewProcessor(a.cfg, currentWallet, walletIndex, a.txLogger)
			proc.Process(ctx)

			if walletIndex < len(a.wallets)-1 {
				delayDuration, err := utils.RandomDuration(a.cfg.Delay.BetweenAccounts)
				if err != nil {
					logger.Error("Ошибка получения времени задержки между кошельками", "err", err, "wIdx", walletIndex, "addr", currentWallet.Address.Hex())
				} else {
					logger.Info("Пауза перед следующим кошельком", "duration", delayDuration, "wIdx", walletIndex, "addr", currentWallet.Address.Hex())
					select {
					case <-time.After(delayDuration):
					case <-ctx.Done():
						logger.Warn("Задержка между кошельками прервана (контекст отменен)", "wIdx", walletIndex, "addr", currentWallet.Address.Hex())
						return
					}
				}
			}
		}(w, i)
	}

	logger.Info("Все горутины обработки кошельков запущены.")
}
