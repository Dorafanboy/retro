package app

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"retro/internal/config"
	"retro/internal/logger"
	"retro/internal/types"
	"retro/internal/utils"
	"retro/internal/wallet"
)

// Application holds the core application logic and dependencies.
type Application struct {
	cfg                 *config.Config
	wallets             []*wallet.Wallet
	registeredTaskNames []string
	wg                  *sync.WaitGroup
}

// NewApplication creates a new Application instance.
func NewApplication(cfg *config.Config, wallets []*wallet.Wallet, registeredTaskNames []string, wg *sync.WaitGroup) *Application {
	if cfg.Wallets.ProcessOrder == types.OrderRandom {
		logger.Info("Перемешивание порядка кошельков...")
		rand.Shuffle(len(wallets), func(i, j int) {
			wallets[i], wallets[j] = wallets[j], wallets[i]
		})
	}
	return &Application{
		cfg:                 cfg,
		wallets:             wallets,
		registeredTaskNames: registeredTaskNames,
		wg:                  wg,
	}
}

// Run starts the main application logic loop, processing wallets concurrently.
func (a *Application) Run(ctx context.Context) {
	logger.Info("Начало обработки кошельков", "count", len(a.wallets))

	for i, w := range a.wallets {
		a.wg.Add(1)
		processor := newWalletProcessor(a.cfg, w, i, a.registeredTaskNames)

		go func(p *WalletProcessor, walletIndex int) {
			defer a.wg.Done()

			select {
			case <-ctx.Done():
				logger.Warn("Обработка кошелька пропущена (контекст отменен перед стартом)")
				return
			default:
			}

			p.Process(ctx)

			if walletIndex < len(a.wallets)-1 {
				select {
				case <-ctx.Done():
					logger.Warn("Задержка между кошельками пропущена (контекст отменен)")
					return
				default:
				}
				delayDuration, err := utils.RandomDuration(a.cfg.Delay.BetweenAccounts)
				if err != nil {
					logger.Error("Ошибка получения времени задержки между кошельками", "err", err)
				} else {
					logger.Info("Пауза перед следующим кошельком", "duration", delayDuration)
					time.Sleep(delayDuration)
				}
			}
		}(processor, i)
	}

	logger.Info("Все горутины обработки кошельков запущены.")
}
