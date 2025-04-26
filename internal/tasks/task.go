package tasks

import (
	"context"
	"retro/internal/evm"
	"retro/internal/wallet"
)

// TaskRunner defines the interface for any task that can be executed.
type TaskRunner interface {
	// Run executes the task logic.
	// It receives the main context, the wallet to use,
	// the evm client (interface) for the target network,
	// and task-specific configuration parameters.
	Run(ctx context.Context, w *wallet.Wallet, client evm.EVMClient, taskConfig map[string]interface{}) error
}

// TODO: Define TaskRegistry here later for registering and retrieving TaskRunners by name.
