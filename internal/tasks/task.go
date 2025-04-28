package tasks

import (
	"context"

	"retro/internal/evm"
	"retro/internal/wallet"
)

// TaskRunner defines the interface for any task that can be executed.
type TaskRunner interface {
	// Run executes the task logic.
	Run(ctx context.Context, w *wallet.Wallet, client evm.EVMClient, taskConfig map[string]interface{}) error
}
