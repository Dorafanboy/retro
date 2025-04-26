package types

// WalletProcessOrder defines the possible orders for processing wallets.
type WalletProcessOrder string

// TaskOrder defines the possible orders for executing tasks within a wallet.
type TaskOrder string

// Common execution order constants.
const (
	OrderRandom     WalletProcessOrder = "random"
	OrderSequential WalletProcessOrder = "sequential"

	TaskOrderRandom     TaskOrder = "random"
	TaskOrderSequential TaskOrder = "sequential"
)
