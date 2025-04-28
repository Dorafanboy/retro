package tasks

import (
	"context"

	"retro/internal/evm"
	// "retro/internal/wallet" // No longer needed here directly
)

// TaskRunner defines the interface for any task that can be executed.
type TaskRunner interface {
	// Run executes the task logic using an EVM signer for potential on-chain actions.
	Run(ctx context.Context, signer *evm.Signer, client evm.EVMClient, taskConfig map[string]interface{}) error
}
